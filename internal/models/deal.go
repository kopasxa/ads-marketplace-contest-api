package models

import (
	"time"

	"github.com/google/uuid"
)

// Deal statuses
const (
	DealStatusDraft                   = "draft"
	DealStatusSubmitted               = "submitted"
	DealStatusRejected                = "rejected"
	DealStatusAccepted                = "accepted"
	DealStatusAwaitingPayment         = "awaiting_payment"
	DealStatusFunded                  = "funded"
	DealStatusCreativePending         = "creative_pending"
	DealStatusCreativeSubmitted       = "creative_submitted"
	DealStatusCreativeChangesRequested = "creative_changes_requested"
	DealStatusCreativeApproved        = "creative_approved"
	DealStatusScheduled               = "scheduled"
	DealStatusPosted                  = "posted"
	DealStatusHoldVerification        = "hold_verification"
	DealStatusHoldVerificationFailed  = "hold_verification_failed"
	DealStatusCompleted               = "completed"
	DealStatusRefunded                = "refunded"
	DealStatusCancelled               = "cancelled"
)

// Valid state transitions: from -> []to
var ValidDealTransitions = map[string][]string{
	DealStatusDraft:                    {DealStatusSubmitted, DealStatusCancelled},
	DealStatusSubmitted:                {DealStatusAccepted, DealStatusRejected, DealStatusCancelled},
	DealStatusRejected:                 {},
	DealStatusAccepted:                 {DealStatusAwaitingPayment, DealStatusCancelled},
	DealStatusAwaitingPayment:          {DealStatusFunded, DealStatusCancelled},
	DealStatusFunded:                   {DealStatusCreativePending, DealStatusCancelled},
	DealStatusCreativePending:          {DealStatusCreativeSubmitted, DealStatusCancelled},
	DealStatusCreativeSubmitted:        {DealStatusCreativeApproved, DealStatusCreativeChangesRequested},
	DealStatusCreativeChangesRequested: {DealStatusCreativeSubmitted, DealStatusCancelled},
	DealStatusCreativeApproved:         {DealStatusScheduled, DealStatusPosted},
	DealStatusScheduled:                {DealStatusPosted, DealStatusCancelled},
	DealStatusPosted:                   {DealStatusHoldVerification},
	DealStatusHoldVerification:         {DealStatusCompleted, DealStatusHoldVerificationFailed},
	DealStatusHoldVerificationFailed:   {DealStatusRefunded},
	DealStatusCompleted:                {},
	DealStatusRefunded:                 {},
	DealStatusCancelled:                {DealStatusRefunded},
}

func IsValidTransition(from, to string) bool {
	allowed, ok := ValidDealTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

type Deal struct {
	ID                uuid.UUID  `json:"id"`
	ChannelID         uuid.UUID  `json:"channel_id"`
	AdvertiserUserID  uuid.UUID  `json:"advertiser_user_id"`
	Status            string     `json:"status"`
	AdFormat          string     `json:"ad_format"` // post / repost / story
	Brief             *string    `json:"brief,omitempty"`
	ScheduledAt       *time.Time `json:"scheduled_at,omitempty"`
	PriceTON          string     `json:"price_ton"` // numeric as string
	PlatformFeeBPS    int        `json:"platform_fee_bps"`
	HoldPeriodSeconds int        `json:"hold_period_seconds"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// DealWithChannel embeds Deal and adds channel info to avoid N+1 queries.
type DealWithChannel struct {
	Deal
	ChannelTitle    *string `json:"channel_title,omitempty"`
	ChannelUsername *string `json:"channel_username,omitempty"`
}

type DealCreative struct {
	ID                      uuid.UUID `json:"id"`
	DealID                  uuid.UUID `json:"deal_id"`
	Version                 int       `json:"version"`
	OwnerComposedText       *string   `json:"owner_composed_text,omitempty"`
	AdvertiserMaterialsText *string   `json:"advertiser_materials_text,omitempty"`
	Status                  string    `json:"status"`
	// Формат-специфичные поля
	RepostFromChatID        *int64    `json:"repost_from_chat_id,omitempty"`
	RepostFromMsgID         *int64    `json:"repost_from_msg_id,omitempty"`
	RepostFromURL           *string   `json:"repost_from_url,omitempty"`
	MediaURLs               any       `json:"media_urls,omitempty"`  // []string
	ButtonsJSON             any       `json:"buttons_json,omitempty"` // [{text, url}]
	CreatedAt               time.Time `json:"created_at"`
}

type DealPost struct {
	ID                uuid.UUID  `json:"id"`
	DealID            uuid.UUID  `json:"deal_id"`
	TelegramMessageID *int64     `json:"telegram_message_id,omitempty"`
	TelegramChatID    *int64     `json:"telegram_chat_id,omitempty"`
	PostURL           *string    `json:"post_url,omitempty"`
	ContentHash       *string    `json:"content_hash,omitempty"`
	PostedAt          *time.Time `json:"posted_at,omitempty"`
	LastCheckedAt     *time.Time `json:"last_checked_at,omitempty"`
	IsDeleted         bool       `json:"is_deleted"`
	IsEdited          bool       `json:"is_edited"`
	AdFormat          *string    `json:"ad_format,omitempty"`
	StoryExpiresAt    *time.Time `json:"story_expires_at,omitempty"`
}
