package models

import (
	"time"

	"github.com/google/uuid"
)

type Campaign struct {
	ID               uuid.UUID  `json:"id"`
	AdvertiserUserID uuid.UUID  `json:"advertiser_user_id"`
	Title            string     `json:"title"`
	TargetAudience   string     `json:"target_audience"`
	KeyMessages      *string    `json:"key_messages,omitempty"`
	BudgetTON        string     `json:"budget_ton"`
	PreferredDate    *time.Time `json:"preferred_date,omitempty"`
	Status           string     `json:"status"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
