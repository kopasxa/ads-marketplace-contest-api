package models

import (
	"time"

	"github.com/google/uuid"
)

type Channel struct {
	ID             uuid.UUID  `json:"id"`
	TelegramChatID *int64     `json:"telegram_chat_id,omitempty"`
	Username       string     `json:"username"`
	Title          *string    `json:"title,omitempty"`
	AddedByUserID  *uuid.UUID `json:"added_by_user_id,omitempty"`
	BotStatus      string     `json:"bot_status"`
	UserbotStatus  string     `json:"userbot_status"` // none/pending/active/failed/removed
	BotAddedAt     *time.Time `json:"bot_added_at,omitempty"`
	BotRemovedAt   *time.Time `json:"bot_removed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type ChannelMember struct {
	ID               uuid.UUID  `json:"id"`
	ChannelID        uuid.UUID  `json:"channel_id"`
	UserID           uuid.UUID  `json:"user_id"`
	Role             string     `json:"role"` // owner / manager
	CanPost          bool       `json:"can_post"`
	LastAdminCheckAt *time.Time `json:"last_admin_check_at,omitempty"`
}

// Ad format types
const (
	AdFormatPost   = "post"
	AdFormatRepost = "repost"
	AdFormatStory  = "story"
)

var AllAdFormats = []string{AdFormatPost, AdFormatRepost, AdFormatStory}

func IsValidAdFormat(f string) bool {
	for _, af := range AllAdFormats {
		if af == f {
			return true
		}
	}
	return false
}

type ChannelListing struct {
	ID                 uuid.UUID `json:"id"`
	ChannelID          uuid.UUID `json:"channel_id"`
	Status             string    `json:"status"` // draft/active/paused
	PricingJSON        any       `json:"pricing_json,omitempty"`
	// Структурированные цены за формат (TON)
	PricePostTON       *string   `json:"price_post_ton,omitempty"`
	PriceRepostTON     *string   `json:"price_repost_ton,omitempty"`
	PriceStoryTON      *string   `json:"price_story_ton,omitempty"`
	FormatsEnabled     []string  `json:"formats_enabled"` // ["post", "repost", "story"]
	MinLeadTimeMinutes int       `json:"min_lead_time_minutes"`
	Description        *string   `json:"description,omitempty"`
	Category           *string   `json:"category,omitempty"`
	Language           *string   `json:"language,omitempty"`
	// Hold period по формату (часы)
	HoldHoursPost      int       `json:"hold_hours_post"`
	HoldHoursRepost    int       `json:"hold_hours_repost"`
	HoldHoursStory     int       `json:"hold_hours_story"`
	AutoAccept         bool      `json:"auto_accept"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// GetPriceForFormat возвращает цену для указанного формата.
func (l *ChannelListing) GetPriceForFormat(format string) *string {
	switch format {
	case AdFormatPost:
		return l.PricePostTON
	case AdFormatRepost:
		return l.PriceRepostTON
	case AdFormatStory:
		return l.PriceStoryTON
	default:
		return nil
	}
}

// GetHoldHoursForFormat возвращает hold period (в часах) для формата.
func (l *ChannelListing) GetHoldHoursForFormat(format string) int {
	switch format {
	case AdFormatPost:
		return l.HoldHoursPost
	case AdFormatRepost:
		return l.HoldHoursRepost
	case AdFormatStory:
		return l.HoldHoursStory
	default:
		return l.HoldHoursPost
	}
}

// IsFormatEnabled проверяет, включён ли формат.
func (l *ChannelListing) IsFormatEnabled(format string) bool {
	for _, f := range l.FormatsEnabled {
		if f == format {
			return true
		}
	}
	return false
}

type ChannelStatsSnapshot struct {
	ID            uuid.UUID `json:"id"`
	ChannelID     uuid.UUID `json:"channel_id"`
	FetchedAt     time.Time `json:"fetched_at"`
	Subscribers   *int      `json:"subscribers,omitempty"`
	VerifiedBadge bool      `json:"verified_badge"`
	AvgViews20    *int      `json:"avg_views_20,omitempty"`
	LastPostID    *int64    `json:"last_post_id,omitempty"`
	PremiumCount  *int      `json:"premium_count,omitempty"`
	RawJSON       any       `json:"raw_json,omitempty"`
	// Extended fields (from userbot)
	Source        string `json:"source"`         // "tme_parser" or "userbot"
	MembersOnline *int   `json:"members_online,omitempty"`
	AdminsCount   *int   `json:"admins_count,omitempty"`
	Growth7d      *int   `json:"growth_7d,omitempty"`
	Growth30d     *int   `json:"growth_30d,omitempty"`
	PostsCount    *int   `json:"posts_count,omitempty"`
	// Fields from GetBroadcastStats
	ViewsPerPost                *float64 `json:"views_per_post,omitempty"`
	SharesPerPost               *float64 `json:"shares_per_post,omitempty"`
	EnabledNotificationsPercent *float64 `json:"enabled_notifications_percent,omitempty"`
	ERPercent                   *float64 `json:"er_percent,omitempty"`
}
