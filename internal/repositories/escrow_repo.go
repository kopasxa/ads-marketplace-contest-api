package repositories

import (
	"context"

	"github.com/ads-marketplace/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EscrowRepo struct {
	pool *pgxpool.Pool
}

func NewEscrowRepo(pool *pgxpool.Pool) *EscrowRepo {
	return &EscrowRepo{pool: pool}
}

func (r *EscrowRepo) Create(ctx context.Context, e *models.EscrowLedger) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO escrow_ledger (deal_id, deposit_expected_ton, deposit_address, deposit_memo, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, e.DealID, e.DepositExpectedTON, e.DepositAddress, e.DepositMemo, e.Status).Scan(&e.ID)
}

func (r *EscrowRepo) GetByDealID(ctx context.Context, dealID uuid.UUID) (*models.EscrowLedger, error) {
	var e models.EscrowLedger
	err := r.pool.QueryRow(ctx, `
		SELECT id, deal_id, deposit_expected_ton, deposit_address, deposit_memo,
		       funded_at, funding_tx_hash, payer_address,
		       release_amount_ton, release_tx_hash,
		       refunded_at, refund_tx_hash, status
		FROM escrow_ledger WHERE deal_id = $1
	`, dealID).Scan(&e.ID, &e.DealID, &e.DepositExpectedTON, &e.DepositAddress, &e.DepositMemo,
		&e.FundedAt, &e.FundingTxHash, &e.PayerAddress,
		&e.ReleaseAmountTON, &e.ReleaseTxHash,
		&e.RefundedAt, &e.RefundTxHash, &e.Status)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *EscrowRepo) GetByMemo(ctx context.Context, memo string) (*models.EscrowLedger, error) {
	var e models.EscrowLedger
	err := r.pool.QueryRow(ctx, `
		SELECT id, deal_id, deposit_expected_ton, deposit_address, deposit_memo,
		       funded_at, funding_tx_hash, payer_address,
		       release_amount_ton, release_tx_hash,
		       refunded_at, refund_tx_hash, status
		FROM escrow_ledger WHERE deposit_memo = $1
	`, memo).Scan(&e.ID, &e.DealID, &e.DepositExpectedTON, &e.DepositAddress, &e.DepositMemo,
		&e.FundedAt, &e.FundingTxHash, &e.PayerAddress,
		&e.ReleaseAmountTON, &e.ReleaseTxHash,
		&e.RefundedAt, &e.RefundTxHash, &e.Status)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *EscrowRepo) MarkFunded(ctx context.Context, dealID uuid.UUID, txHash, payerAddr string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE escrow_ledger SET status = 'funded', funded_at = now(), funding_tx_hash = $1, payer_address = $2
		WHERE deal_id = $3 AND status = 'awaiting'
	`, txHash, payerAddr, dealID)
	return err
}

func (r *EscrowRepo) MarkReleased(ctx context.Context, dealID uuid.UUID, amount, txHash string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE escrow_ledger SET status = 'released', release_amount_ton = $1, release_tx_hash = $2
		WHERE deal_id = $3 AND status = 'funded'
	`, amount, txHash, dealID)
	return err
}

func (r *EscrowRepo) MarkRefunded(ctx context.Context, dealID uuid.UUID, txHash string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE escrow_ledger SET status = 'refunded', refunded_at = now(), refund_tx_hash = $1
		WHERE deal_id = $2
	`, txHash, dealID)
	return err
}
