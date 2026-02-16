package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ads-marketplace/backend/internal/config"
	"github.com/ads-marketplace/backend/internal/db"
	"github.com/ads-marketplace/backend/internal/events"
	"github.com/ads-marketplace/backend/internal/models"
	"github.com/ads-marketplace/backend/internal/repositories"
	"github.com/ads-marketplace/backend/internal/services"
	"github.com/ads-marketplace/backend/internal/statsparser"
	"go.uber.org/zap"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	cfg := config.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	// Repos
	dealRepo := repositories.NewDealRepo(pool)
	channelRepo := repositories.NewChannelRepo(pool)
	escrowRepo := repositories.NewEscrowRepo(pool)
	auditRepo := repositories.NewAuditRepo(pool)
	withdrawRepo := repositories.NewWithdrawRepo(pool)
	walletRepo := repositories.NewWalletRepo(pool)

	// Services
	publisher := events.NewRedisPublisher(rdb, log)
	botClient := services.NewBotClient(cfg.BotInternalURL, log)
	dealService := services.NewDealService(dealRepo, channelRepo, escrowRepo, auditRepo, withdrawRepo, walletRepo, botClient, publisher, cfg, log)
	parser := statsparser.NewParser(cfg.TMEFetchTimeoutMS, cfg.TMEFetchMaxRetries, log)

	log.Info("worker started")

	// Run jobs on tickers
	timeoutTicker := time.NewTicker(2 * time.Minute)
	holdTicker := time.NewTicker(1 * time.Minute)
	postMonitorTicker := time.NewTicker(5 * time.Minute)
	defer timeoutTicker.Stop()
	defer holdTicker.Stop()
	defer postMonitorTicker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-timeoutTicker.C:
			runDealTimeouts(ctx, dealRepo, dealService, cfg, log)
		case <-holdTicker.C:
			runHoldRelease(ctx, dealRepo, dealService, log)
		case <-postMonitorTicker.C:
			runPostMonitoring(ctx, dealRepo, channelRepo, parser, dealService, log)
		case <-sigCh:
			log.Info("shutting down worker")
			cancel()
			return
		case <-ctx.Done():
			return
		}
	}
}

func runDealTimeouts(ctx context.Context, dealRepo *repositories.DealRepo, dealService *services.DealService, cfg *config.Config, log *zap.Logger) {
	timeouts := map[string]int{
		models.DealStatusSubmitted:         cfg.DealTimeoutSubmittedSeconds,
		models.DealStatusAwaitingPayment:   cfg.DealTimeoutPaymentSeconds,
		models.DealStatusCreativeSubmitted: cfg.DealTimeoutCreativeSeconds,
	}

	for status, timeout := range timeouts {
		deals, err := dealRepo.GetTimedOutDeals(ctx, status, timeout)
		if err != nil {
			log.Error("failed to get timed out deals", zap.String("status", status), zap.Error(err))
			continue
		}

		for _, deal := range deals {
			log.Info("auto-cancelling timed out deal",
				zap.String("deal_id", deal.ID.String()),
				zap.String("status", deal.Status),
			)
			if err := dealService.CancelDeal(ctx, deal.ID, deal.AdvertiserUserID); err != nil {
				log.Error("failed to cancel deal", zap.String("deal_id", deal.ID.String()), zap.Error(err))
			}
		}
	}
}

func runHoldRelease(ctx context.Context, dealRepo *repositories.DealRepo, dealService *services.DealService, log *zap.Logger) {
	deals, err := dealRepo.GetPostedDealsInHold(ctx)
	if err != nil {
		log.Error("failed to get deals for hold release", zap.Error(err))
		return
	}

	for _, deal := range deals {
		log.Info("releasing funds for deal", zap.String("deal_id", deal.ID.String()))
		if err := dealService.ReleaseFunds(ctx, deal.ID); err != nil {
			log.Error("failed to release funds", zap.String("deal_id", deal.ID.String()), zap.Error(err))
		}
	}
}

func runPostMonitoring(ctx context.Context, dealRepo *repositories.DealRepo, channelRepo *repositories.ChannelRepo, parser *statsparser.Parser, dealService *services.DealService, log *zap.Logger) {
	// Get all deals in hold_verification
	deals, err := dealRepo.List(ctx, repositories.DealFilter{
		Status: strPtr(models.DealStatusHoldVerification),
		Limit:  100,
	})
	if err != nil {
		log.Error("failed to get deals in hold", zap.Error(err))
		return
	}

	for _, deal := range deals {
		post, err := dealRepo.GetPost(ctx, deal.ID)
		if err != nil {
			continue
		}
		if post.IsDeleted {
			continue
		}

		ch, err := channelRepo.GetByID(ctx, deal.ChannelID)
		if err != nil {
			continue
		}

		// Check via HTML parsing
		if post.TelegramMessageID != nil {
			text, exists, err := parser.FetchPostContent(ctx, ch.Username, *post.TelegramMessageID)
			if err != nil {
				log.Warn("failed to check post", zap.Error(err))
				continue
			}

			if !exists {
				log.Warn("post deleted detected",
					zap.String("deal_id", deal.ID.String()),
					zap.Int64("message_id", *post.TelegramMessageID),
				)
				_ = dealRepo.UpdatePostFlags(ctx, deal.ID, true, false)
				_ = dealService.RefundDeal(ctx, deal.ID)
				continue
			}

			// Check for edits by comparing content hash
			if post.ContentHash != nil && text != "" {
				currentHash := sha256Hex(text)
				if currentHash != *post.ContentHash {
					log.Warn("post edited detected",
						zap.String("deal_id", deal.ID.String()),
					)
					_ = dealRepo.UpdatePostFlags(ctx, deal.ID, false, true)
					// For MVP: notify but don't auto-refund on edits
				}
			}
		} else if post.PostURL != nil {
			// Manual post — try to parse from URL
			// Extract message ID from URL like https://t.me/username/123
			// For now just check if page exists
			_, exists, err := parser.FetchPostContent(ctx, ch.Username, 0)
			if err != nil || !exists {
				log.Warn("manual post might be deleted", zap.String("deal_id", deal.ID.String()))
			}
		}

		time.Sleep(1 * time.Second) // rate limiting
	}
}

func strPtr(s string) *string {
	return &s
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}
