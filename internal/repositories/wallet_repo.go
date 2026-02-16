package repositories

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/ads-marketplace/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WalletRepo struct {
	pool *pgxpool.Pool
}

func NewWalletRepo(pool *pgxpool.Pool) *WalletRepo {
	return &WalletRepo{pool: pool}
}

// --- Proof Payloads (nonce) ---

func (r *WalletRepo) CreateProofPayload(ctx context.Context, userID *uuid.UUID, ttl time.Duration) (*models.TonProofPayload, error) {
	payload := generateNonce(32)
	p := &models.TonProofPayload{
		Payload: payload,
		UserID:  userID,
	}

	err := r.pool.QueryRow(ctx, `
		INSERT INTO ton_proof_payloads (payload, user_id, expires_at)
		VALUES ($1, $2, now() + $3::interval)
		RETURNING id, created_at, expires_at
	`, payload, userID, ttl.String()).Scan(&p.ID, &p.CreatedAt, &p.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *WalletRepo) ConsumeProofPayload(ctx context.Context, payload string) (*models.TonProofPayload, error) {
	var p models.TonProofPayload
	err := r.pool.QueryRow(ctx, `
		UPDATE ton_proof_payloads
		SET used = true
		WHERE payload = $1 AND used = false AND expires_at > now()
		RETURNING id, payload, user_id, created_at, expires_at, used
	`, payload).Scan(&p.ID, &p.Payload, &p.UserID, &p.CreatedAt, &p.ExpiresAt, &p.Used)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// --- User Wallets ---

func (r *WalletRepo) ConnectWallet(ctx context.Context, w *models.UserWallet) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO user_wallets (
			user_id, address, address_friendly, network, public_key,
			proof_payload, proof_signature, proof_timestamp, proof_domain,
			verified, is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, true)
		ON CONFLICT (user_id, address) DO UPDATE SET
			public_key = EXCLUDED.public_key,
			proof_payload = EXCLUDED.proof_payload,
			proof_signature = EXCLUDED.proof_signature,
			proof_timestamp = EXCLUDED.proof_timestamp,
			proof_domain = EXCLUDED.proof_domain,
			verified = EXCLUDED.verified,
			is_active = true,
			disconnected_at = NULL,
			connected_at = now()
		RETURNING id, connected_at
	`, w.UserID, w.Address, w.AddressFriendly, w.Network, w.PublicKey,
		w.ProofPayload, w.ProofSignature, w.ProofTimestamp, w.ProofDomain,
		w.Verified,
	).Scan(&w.ID, &w.ConnectedAt)
}

func (r *WalletRepo) DeactivateAllWallets(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE user_wallets SET is_active = false, disconnected_at = now()
		WHERE user_id = $1 AND is_active = true
	`, userID)
	return err
}

func (r *WalletRepo) DisconnectWallet(ctx context.Context, userID uuid.UUID, walletID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE user_wallets SET is_active = false, disconnected_at = now()
		WHERE id = $1 AND user_id = $2
	`, walletID, userID)
	return err
}

func (r *WalletRepo) GetActiveWallet(ctx context.Context, userID uuid.UUID) (*models.UserWallet, error) {
	var w models.UserWallet
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, address, address_friendly, network, public_key,
		       proof_payload, proof_signature, proof_timestamp, proof_domain,
		       verified, connected_at, disconnected_at, is_active
		FROM user_wallets
		WHERE user_id = $1 AND is_active = true
		ORDER BY connected_at DESC LIMIT 1
	`, userID).Scan(
		&w.ID, &w.UserID, &w.Address, &w.AddressFriendly, &w.Network, &w.PublicKey,
		&w.ProofPayload, &w.ProofSignature, &w.ProofTimestamp, &w.ProofDomain,
		&w.Verified, &w.ConnectedAt, &w.DisconnectedAt, &w.IsActive,
	)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *WalletRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.UserWallet, error) {
	var w models.UserWallet
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, address, address_friendly, network, public_key,
		       verified, connected_at, disconnected_at, is_active
		FROM user_wallets WHERE id = $1
	`, id).Scan(
		&w.ID, &w.UserID, &w.Address, &w.AddressFriendly, &w.Network, &w.PublicKey,
		&w.Verified, &w.ConnectedAt, &w.DisconnectedAt, &w.IsActive,
	)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *WalletRepo) UpdateUserWalletAddress(ctx context.Context, userID uuid.UUID, address string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET wallet_address = $1 WHERE id = $2`, address, userID)
	return err
}

func (r *WalletRepo) ClearUserWalletAddress(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET wallet_address = NULL WHERE id = $1`, userID)
	return err
}

func generateNonce(bytes int) string {
	b := make([]byte, bytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
