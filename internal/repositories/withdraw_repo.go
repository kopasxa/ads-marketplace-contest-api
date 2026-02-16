package repositories

import (
	"context"

	"github.com/ads-marketplace/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WithdrawRepo struct {
	pool *pgxpool.Pool
}

func NewWithdrawRepo(pool *pgxpool.Pool) *WithdrawRepo {
	return &WithdrawRepo{pool: pool}
}

func (r *WithdrawRepo) Upsert(ctx context.Context, w *models.WithdrawWallet) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO withdraw_wallets (channel_id, owner_user_id, wallet_address)
		VALUES ($1, $2, $3)
		ON CONFLICT (channel_id) DO UPDATE SET
			wallet_address = EXCLUDED.wallet_address,
			owner_user_id = EXCLUDED.owner_user_id,
			updated_at = now()
		RETURNING id, created_at, updated_at
	`, w.ChannelID, w.OwnerUserID, w.WalletAddress).Scan(&w.ID, &w.CreatedAt, &w.UpdatedAt)
}

func (r *WithdrawRepo) GetByChannel(ctx context.Context, channelID uuid.UUID) (*models.WithdrawWallet, error) {
	var w models.WithdrawWallet
	err := r.pool.QueryRow(ctx, `
		SELECT id, channel_id, owner_user_id, wallet_address, created_at, updated_at
		FROM withdraw_wallets WHERE channel_id = $1
	`, channelID).Scan(&w.ID, &w.ChannelID, &w.OwnerUserID, &w.WalletAddress, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}
