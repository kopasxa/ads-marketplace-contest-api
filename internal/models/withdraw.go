package models

import (
	"time"

	"github.com/google/uuid"
)

type WithdrawWallet struct {
	ID            uuid.UUID `json:"id"`
	ChannelID     uuid.UUID `json:"channel_id"`
	OwnerUserID   uuid.UUID `json:"owner_user_id"`
	WalletAddress string    `json:"wallet_address"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
