package models

import (
	"time"

	"github.com/google/uuid"
)

type UserWallet struct {
	ID              uuid.UUID  `json:"id"`
	UserID          uuid.UUID  `json:"user_id"`
	Address         string     `json:"address"`          // raw: 0:<hex>
	AddressFriendly string     `json:"address_friendly"` // EQ.../UQ...
	Network         string     `json:"network"`          // mainnet/testnet
	PublicKey       string     `json:"public_key"`       // hex
	ProofPayload    string     `json:"-"`
	ProofSignature  string     `json:"-"`
	ProofTimestamp  int64      `json:"-"`
	ProofDomain     string     `json:"-"`
	Verified        bool       `json:"verified"`
	ConnectedAt     time.Time  `json:"connected_at"`
	DisconnectedAt  *time.Time `json:"disconnected_at,omitempty"`
	IsActive        bool       `json:"is_active"`
}

type TonProofPayload struct {
	ID        uuid.UUID `json:"id"`
	Payload   string    `json:"payload"`
	UserID    *uuid.UUID `json:"-"`
	CreatedAt time.Time `json:"-"`
	ExpiresAt time.Time `json:"-"`
	Used      bool      `json:"-"`
}
