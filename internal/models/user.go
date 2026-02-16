package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID             uuid.UUID `json:"id"`
	TelegramUserID int64     `json:"telegram_user_id"`
	Username       *string   `json:"username,omitempty"`
	FirstName      *string   `json:"first_name,omitempty"`
	LastName       *string   `json:"last_name,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	LastActiveAt   time.Time `json:"last_active_at"`
}
