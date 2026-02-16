package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ads-marketplace/backend/internal/config"
	"github.com/ads-marketplace/backend/internal/db"
	"github.com/ads-marketplace/backend/internal/events"
	"github.com/ads-marketplace/backend/internal/models"
	"github.com/ads-marketplace/backend/internal/repositories"
	"github.com/redis/go-redis/v9"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"go.uber.org/zap"
)

const (
	redisCursorLT  = "ton-indexer:cursor:lt"
	redisCursorHash = "ton-indexer:cursor:hash"
	redisProcessed = "ton-indexer:tx:"
	processedTTL   = 7 * 24 * time.Hour
	pollInterval   = 5 * time.Second
	txBatchSize    = 100
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	cfg := config.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.TONHotWalletAddress == "" {
		log.Fatal("TON_HOT_WALLET_ADDRESS is required")
	}

	hotWallet, err := address.ParseAddr(cfg.TONHotWalletAddress)
	if err != nil {
		log.Fatal("invalid TON_HOT_WALLET_ADDRESS", zap.String("addr", cfg.TONHotWalletAddress), zap.Error(err))
	}

	pool, err := db.NewPostgresPool(ctx, cfg.PostgresDSN, log)
	if err != nil {
		log.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer pool.Close()

	rdb, err := db.NewRedisClient(ctx, cfg.RedisURL, log)
	if err != nil {
		log.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer rdb.Close()

	escrowRepo := repositories.NewEscrowRepo(pool)
	dealRepo := repositories.NewDealRepo(pool)
	publisher := events.NewRedisPublisher(rdb, log)

	tonAPI, err := connectToTON(ctx, cfg, log)
	if err != nil {
		log.Fatal("failed to connect to TON network", zap.Error(err))
	}

	log.Info("TON indexer started",
		zap.String("hot_wallet", hotWallet.String()),
		zap.String("network", cfg.TONNetwork),
	)

	initCursor(ctx, tonAPI, hotWallet, rdb, log)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			if err := pollAndProcess(ctx, tonAPI, hotWallet, escrowRepo, dealRepo, publisher, rdb, log); err != nil {
				log.Error("poll cycle failed", zap.Error(err))
			}
		case <-sigCh:
			log.Info("shutting down TON indexer")
			cancel()
			return
		case <-ctx.Done():
			return
		}
	}
}

// connectToTON establishes a connection to the TON network.
// If LITE_SERVER_HOST + LITE_SERVER_KEY are set, connects to a specific lite server.
// Otherwise, auto-discovers lite servers from the global TON config based on TON_NETWORK.
func connectToTON(ctx context.Context, cfg *config.Config, log *zap.Logger) (ton.APIClientWrapped, error) {
	client := liteclient.NewConnectionPool()

	if cfg.LiteServerHost != "" && cfg.LiteServerKey != "" {
		addr := fmt.Sprintf("%s:%d", cfg.LiteServerHost, cfg.LiteServerPort)
		log.Info("connecting to lite server", zap.String("addr", addr))
		if err := client.AddConnection(ctx, addr, cfg.LiteServerKey); err != nil {
			return nil, fmt.Errorf("connect to lite server %s: %w", addr, err)
		}
	} else {
		var configURL string
		switch strings.ToLower(cfg.TONNetwork) {
		case "mainnet":
			configURL = "https://ton.org/global.config.json"
		default:
			configURL = "https://ton.org/testnet-global.config.json"
		}
		log.Info("connecting via global config", zap.String("url", configURL), zap.String("network", cfg.TONNetwork))
		if err := client.AddConnectionsFromConfigUrl(ctx, configURL); err != nil {
			return nil, fmt.Errorf("connect via config %s: %w", configURL, err)
		}
	}

	proofPolicy := ton.ProofCheckPolicyFast
	if strings.ToLower(cfg.TONNetwork) == "mainnet" {
		proofPolicy = ton.ProofCheckPolicySecure
	}

	api := ton.NewAPIClient(client, proofPolicy).WithRetry()
	return api, nil
}

// initCursor sets the initial cursor position on first run.
// On first run, it stores the current account LastTxLT so that only
// NEW transactions (arriving after startup) are processed.
func initCursor(ctx context.Context, api ton.APIClientWrapped, addr *address.Address, rdb *redis.Client, log *zap.Logger) {
	existing, _ := rdb.Get(ctx, redisCursorLT).Result()
	if existing != "" {
		log.Info("resuming from saved cursor", zap.String("lt", existing))
		return
	}

	block, err := api.CurrentMasterchainInfo(ctx)
	if err != nil {
		log.Warn("failed to get master block for cursor init", zap.Error(err))
		rdb.Set(ctx, redisCursorLT, "0", 0)
		return
	}

	account, err := api.GetAccount(ctx, block, addr)
	if err != nil {
		log.Warn("failed to get account for cursor init", zap.Error(err))
		rdb.Set(ctx, redisCursorLT, "0", 0)
		return
	}

	if account == nil || !account.IsActive || account.LastTxLT == 0 {
		log.Info("hot wallet not active yet, starting from LT=0")
		rdb.Set(ctx, redisCursorLT, "0", 0)
		return
	}

	saveCursor(ctx, rdb, account.LastTxLT, account.LastTxHash)
	log.Info("cursor initialized at current account state (skipping historical transactions)",
		zap.Uint64("lt", account.LastTxLT),
		zap.String("hash", hex.EncodeToString(account.LastTxHash)),
	)
}

func loadCursorLT(ctx context.Context, rdb *redis.Client) uint64 {
	val, err := rdb.Get(ctx, redisCursorLT).Result()
	if err != nil || val == "" {
		return 0
	}
	lt, _ := strconv.ParseUint(val, 10, 64)
	return lt
}

func loadCursorHash(ctx context.Context, rdb *redis.Client) []byte {
	val, err := rdb.Get(ctx, redisCursorHash).Result()
	if err != nil || val == "" {
		return nil
	}
	hash, _ := hex.DecodeString(val)
	return hash
}

func saveCursor(ctx context.Context, rdb *redis.Client, lt uint64, hash []byte) {
	rdb.Set(ctx, redisCursorLT, strconv.FormatUint(lt, 10), 0)
	rdb.Set(ctx, redisCursorHash, hex.EncodeToString(hash), 0)
}

// pollAndProcess runs a single poll cycle:
// 1. Get the account's latest state
// 2. Fetch all transactions newer than the cursor
// 3. Process incoming TON transfers
// 4. Update the cursor
func pollAndProcess(
	ctx context.Context,
	api ton.APIClientWrapped,
	addr *address.Address,
	escrowRepo *repositories.EscrowRepo,
	dealRepo *repositories.DealRepo,
	publisher events.Publisher,
	rdb *redis.Client,
	log *zap.Logger,
) error {
	cursorLT := loadCursorLT(ctx, rdb)

	block, err := api.CurrentMasterchainInfo(ctx)
	if err != nil {
		return fmt.Errorf("get master block: %w", err)
	}

	account, err := api.GetAccount(ctx, block, addr)
	if err != nil {
		return fmt.Errorf("get account: %w", err)
	}

	if account == nil || !account.IsActive || account.LastTxLT == 0 {
		return nil
	}

	if account.LastTxLT <= cursorLT {
		return nil
	}

	newTxs, err := fetchNewTransactions(ctx, api, addr, account, cursorLT)
	if err != nil {
		return fmt.Errorf("fetch transactions: %w", err)
	}

	if len(newTxs) > 0 {
		log.Info("found new transactions", zap.Int("count", len(newTxs)))
		for _, tx := range newTxs {
			processIncomingTx(ctx, tx, escrowRepo, dealRepo, publisher, rdb, log)
		}
	}

	saveCursor(ctx, rdb, account.LastTxLT, account.LastTxHash)
	return nil
}

// fetchNewTransactions retrieves all transactions with LT > cursorLT.
// ListTransactions returns results oldest-first; we paginate backwards
// until we reach the cursor, then return in chronological order.
func fetchNewTransactions(
	ctx context.Context,
	api ton.APIClientWrapped,
	addr *address.Address,
	account *tlb.Account,
	cursorLT uint64,
) ([]*tlb.Transaction, error) {
	var allTxs []*tlb.Transaction

	lt := account.LastTxLT
	hash := account.LastTxHash

	for {
		txs, err := api.ListTransactions(ctx, addr, uint32(txBatchSize), lt, hash)
		if err != nil {
			return nil, fmt.Errorf("list transactions (lt=%d): %w", lt, err)
		}
		if len(txs) == 0 {
			break
		}

		reachedCursor := false
		for _, tx := range txs {
			if tx.LT <= cursorLT {
				reachedCursor = true
				continue
			}
			allTxs = append(allTxs, tx)
		}

		if reachedCursor || len(txs) < txBatchSize {
			break
		}

		oldest := txs[0]
		if oldest.PrevTxLT == 0 {
			break
		}
		lt = oldest.PrevTxLT
		hash = oldest.PrevTxHash
	}

	sort.Slice(allTxs, func(i, j int) bool {
		return allTxs[i].LT < allTxs[j].LT
	})

	return allTxs, nil
}

// processIncomingTx handles a single incoming TON transfer:
// extracts the memo, matches it to an escrow record, verifies the amount,
// and updates escrow + deal status.
func processIncomingTx(
	ctx context.Context,
	tx *tlb.Transaction,
	escrowRepo *repositories.EscrowRepo,
	dealRepo *repositories.DealRepo,
	publisher events.Publisher,
	rdb *redis.Client,
	log *zap.Logger,
) {
	if tx.IO.In == nil {
		return
	}

	inMsg, ok := tx.IO.In.Msg.(*tlb.InternalMessage)
	if !ok || inMsg == nil {
		return
	}

	if inMsg.Bounced {
		return
	}

	if inMsg.Amount.Nano().Sign() <= 0 {
		return
	}

	comment := extractComment(inMsg)
	if comment == "" {
		log.Debug("transfer without memo, skipping",
			zap.Uint64("lt", tx.LT),
			zap.String("from", inMsg.SrcAddr.String()),
			zap.String("amount", inMsg.Amount.String()),
		)
		return
	}

	// Idempotency: skip if already processed
	txKey := fmt.Sprintf("%s%d", redisProcessed, tx.LT)
	if rdb.Exists(ctx, txKey).Val() > 0 {
		return
	}

	memo := strings.TrimSpace(comment)

	log.Info("incoming payment detected",
		zap.Uint64("lt", tx.LT),
		zap.String("from", inMsg.SrcAddr.String()),
		zap.String("amount", inMsg.Amount.String()),
		zap.String("memo", memo),
	)

	escrow, err := escrowRepo.GetByMemo(ctx, memo)
	if err != nil {
		log.Debug("no escrow found for memo", zap.String("memo", memo))
		rdb.Set(ctx, txKey, "no_escrow", processedTTL)
		return
	}

	if escrow.Status != models.EscrowStatusAwaiting {
		log.Debug("escrow not in awaiting status",
			zap.String("memo", memo),
			zap.String("deal_id", escrow.DealID.String()),
			zap.String("status", escrow.Status),
		)
		rdb.Set(ctx, txKey, "skip:"+escrow.Status, processedTTL)
		return
	}

	// Verify payment amount
	expectedNano, err := parseTONToNano(escrow.DepositExpectedTON)
	if err != nil {
		log.Error("invalid expected amount in escrow",
			zap.String("deal_id", escrow.DealID.String()),
			zap.String("expected_ton", escrow.DepositExpectedTON),
			zap.Error(err),
		)
		return
	}

	receivedNano := inMsg.Amount.Nano()
	if receivedNano.Cmp(expectedNano) < 0 {
		log.Warn("insufficient payment — amount below expected",
			zap.String("deal_id", escrow.DealID.String()),
			zap.String("received", inMsg.Amount.String()),
			zap.String("expected", escrow.DepositExpectedTON),
			zap.String("memo", memo),
		)
		// Don't mark as processed: the user may send the remainder
		return
	}

	// Mark escrow funded
	txRef := strconv.FormatUint(tx.LT, 10)
	fromAddr := inMsg.SrcAddr.String()

	if err := escrowRepo.MarkFunded(ctx, escrow.DealID, txRef, fromAddr); err != nil {
		log.Error("failed to mark escrow funded",
			zap.String("deal_id", escrow.DealID.String()),
			zap.Error(err),
		)
		return
	}

	// Advance deal status
	if err := dealRepo.UpdateStatus(ctx, escrow.DealID, models.DealStatusFunded); err != nil {
		log.Error("failed to update deal status to funded",
			zap.String("deal_id", escrow.DealID.String()),
			zap.Error(err),
		)
		return
	}

	// Publish event for bot notifications / websocket
	_ = publisher.Publish(ctx, "events:deal", events.Event{
		Type: events.EventPaymentReceived,
		Payload: map[string]any{
			"deal_id":    escrow.DealID.String(),
			"tx_lt":      tx.LT,
			"amount_ton": inMsg.Amount.String(),
			"from":       fromAddr,
			"memo":       memo,
		},
	})

	rdb.Set(ctx, txKey, "funded:"+escrow.DealID.String(), processedTTL)

	log.Info("payment processed — deal funded",
		zap.String("deal_id", escrow.DealID.String()),
		zap.Uint64("tx_lt", tx.LT),
		zap.String("amount", inMsg.Amount.String()),
		zap.String("from", fromAddr),
		zap.String("memo", memo),
	)
}

// extractComment parses a text comment from an InternalMessage body.
// TON text comments have opcode 0x00000000 followed by UTF-8 text.
func extractComment(inMsg *tlb.InternalMessage) string {
	body := inMsg.Body
	if body == nil {
		return ""
	}

	slice := body.BeginParse()
	if slice.BitsLeft() < 32 {
		return ""
	}

	op, err := slice.LoadUInt(32)
	if err != nil || op != 0 {
		return ""
	}

	remaining := slice.BitsLeft()
	if remaining < 8 {
		return ""
	}

	data, err := slice.LoadSlice(remaining)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}

// parseTONToNano converts a decimal TON string (e.g. "5.5") to nanoTON (*big.Int).
// 1 TON = 1_000_000_000 nanoTON.
func parseTONToNano(tonStr string) (*big.Int, error) {
	tonStr = strings.TrimSpace(tonStr)
	if tonStr == "" {
		return nil, fmt.Errorf("empty TON amount")
	}

	parts := strings.Split(tonStr, ".")
	if len(parts) > 2 {
		return nil, fmt.Errorf("invalid TON amount: %s", tonStr)
	}

	whole := parts[0]
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}

	if len(frac) > 9 {
		frac = frac[:9]
	}
	for len(frac) < 9 {
		frac += "0"
	}

	nano, ok := new(big.Int).SetString(whole+frac, 10)
	if !ok {
		return nil, fmt.Errorf("invalid TON amount: %s", tonStr)
	}
	return nano, nil
}
