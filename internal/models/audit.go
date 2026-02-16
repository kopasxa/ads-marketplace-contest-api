package models

import (
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID          uuid.UUID  `json:"id"`
	ActorUserID *uuid.UUID `json:"actor_user_id,omitempty"`
	ActorType   string     `json:"actor_type"` // user/admin/system/bot
	Action      string     `json:"action"`
	EntityType  string     `json:"entity_type"`
	EntityID    *uuid.UUID `json:"entity_id,omitempty"`
	Meta        any        `json:"meta,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}
