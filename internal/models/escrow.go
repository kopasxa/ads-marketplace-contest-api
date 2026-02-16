package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	EscrowStatusAwaiting = "awaiting"
	EscrowStatusFunded   = "funded"
	EscrowStatusReleased = "released"
	EscrowStatusRefunded = "refunded"
)

type EscrowLedger struct {
	ID                 uuid.UUID  `json:"id"`
	DealID             uuid.UUID  `json:"deal_id"`
	DepositExpectedTON string     `json:"deposit_expected_ton"`
	DepositAddress     string     `json:"deposit_address"`
	DepositMemo        string     `json:"deposit_memo"`
	FundedAt           *time.Time `json:"funded_at,omitempty"`
	FundingTxHash      *string    `json:"funding_tx_hash,omitempty"`
	PayerAddress       *string    `json:"payer_address,omitempty"`
	ReleaseAmountTON   *string    `json:"release_amount_ton,omitempty"`
	ReleaseTxHash      *string    `json:"release_tx_hash,omitempty"`
	RefundedAt         *time.Time `json:"refunded_at,omitempty"`
	RefundTxHash       *string    `json:"refund_tx_hash,omitempty"`
	Status             string     `json:"status"`
}
