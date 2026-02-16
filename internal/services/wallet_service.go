package services

import (
	"context"
	"fmt"
	"time"

	"github.com/ads-marketplace/backend/internal/config"
	"github.com/ads-marketplace/backend/internal/models"
	"github.com/ads-marketplace/backend/internal/repositories"
	"github.com/ads-marketplace/backend/internal/ton"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type WalletService struct {
	walletRepo *repositories.WalletRepo
	auditRepo  *repositories.AuditRepo
	cfg        *config.Config
	log        *zap.Logger
}

func NewWalletService(
	walletRepo *repositories.WalletRepo,
	auditRepo *repositories.AuditRepo,
	cfg *config.Config,
	log *zap.Logger,
) *WalletService {
	return &WalletService{
		walletRepo: walletRepo,
		auditRepo:  auditRepo,
		cfg:        cfg,
		log:        log,
	}
}

// GeneratePayload создаёт nonce для TON Proof.
// Клиент передаёт его в tonconnect при подключении кошелька.
func (s *WalletService) GeneratePayload(ctx context.Context, userID *uuid.UUID) (string, error) {
	ttl := 5 * time.Minute
	p, err := s.walletRepo.CreateProofPayload(ctx, userID, ttl)
	if err != nil {
		return "", fmt.Errorf("failed to create proof payload: %w", err)
	}
	return p.Payload, nil
}

// ConnectWallet проверяет TON Proof и привязывает кошелёк к пользователю.
type ConnectWalletRequest struct {
	Address         string    `json:"address"`          // raw: "0:abc..."
	AddressFriendly string    `json:"address_friendly"` // "EQA..." или "UQA..."
	Network         string    `json:"network"`          // "mainnet" / "testnet"
	PublicKey       string    `json:"public_key"`       // hex
	Proof           ton.Proof `json:"proof"`
}

func (s *WalletService) ConnectWallet(ctx context.Context, userID uuid.UUID, req ConnectWalletRequest) (*models.UserWallet, error) {
	// 1. Consume payload (nonce) — защита от replay
	_, err := s.walletRepo.ConsumeProofPayload(ctx, req.Proof.Payload)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired proof payload (nonce): %w", err)
	}

	// 2. Парсим raw address
	workchain, addrHash, err := ton.ParseRawAddress(req.Address)
	if err != nil {
		return nil, fmt.Errorf("invalid TON address: %w", err)
	}

	// 3. Проверяем network
	expectedNetwork := s.cfg.TONNetwork
	if req.Network != "" && req.Network != expectedNetwork {
		return nil, fmt.Errorf("network mismatch: expected %s, got %s", expectedNetwork, req.Network)
	}

	// 4. Верифицируем TON Proof подпись
	err = ton.VerifyProof(req.PublicKey, addrHash, workchain, req.Proof, s.cfg.TONProofAllowedDomains)
	if err != nil {
		return nil, fmt.Errorf("TON Proof verification failed: %w", err)
	}

	// 5. Деактивируем предыдущие кошельки этого юзера
	if err := s.walletRepo.DeactivateAllWallets(ctx, userID); err != nil {
		s.log.Warn("failed to deactivate old wallets", zap.Error(err))
	}

	// 6. Сохраняем новый кошелёк
	wallet := &models.UserWallet{
		UserID:          userID,
		Address:         req.Address,
		AddressFriendly: req.AddressFriendly,
		Network:         req.Network,
		PublicKey:       req.PublicKey,
		ProofPayload:    req.Proof.Payload,
		ProofSignature:  req.Proof.Signature,
		ProofTimestamp:  req.Proof.Timestamp,
		ProofDomain:     req.Proof.Domain.Value,
		Verified:        true,
		IsActive:        true,
	}

	if err := s.walletRepo.ConnectWallet(ctx, wallet); err != nil {
		return nil, fmt.Errorf("failed to save wallet: %w", err)
	}

	// 7. Обновляем кеш в users.wallet_address
	_ = s.walletRepo.UpdateUserWalletAddress(ctx, userID, req.AddressFriendly)

	// 8. Audit log
	_ = s.auditRepo.Log(ctx, models.AuditLog{
		ActorUserID: &userID,
		ActorType:   "user",
		Action:      "wallet_connected",
		EntityType:  "user_wallet",
		EntityID:    &wallet.ID,
		Meta:        map[string]any{"address": req.AddressFriendly, "network": req.Network},
	})

	s.log.Info("wallet connected",
		zap.String("user_id", userID.String()),
		zap.String("address", req.AddressFriendly),
	)

	return wallet, nil
}

// DisconnectWallet отключает активный кошелёк пользователя.
func (s *WalletService) DisconnectWallet(ctx context.Context, userID uuid.UUID) error {
	if err := s.walletRepo.DeactivateAllWallets(ctx, userID); err != nil {
		return err
	}
	_ = s.walletRepo.ClearUserWalletAddress(ctx, userID)

	_ = s.auditRepo.Log(ctx, models.AuditLog{
		ActorUserID: &userID,
		ActorType:   "user",
		Action:      "wallet_disconnected",
		EntityType:  "user",
		EntityID:    &userID,
	})

	return nil
}

// GetActiveWallet возвращает текущий активный кошелёк.
func (s *WalletService) GetActiveWallet(ctx context.Context, userID uuid.UUID) (*models.UserWallet, error) {
	return s.walletRepo.GetActiveWallet(ctx, userID)
}
