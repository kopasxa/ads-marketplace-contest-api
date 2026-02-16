package dto

import "time"

type AuthTelegramRequest struct {
	InitData string `json:"init_data"`
}

type CreateChannelRequest struct {
	Username string `json:"username"`
}

type AddManagerRequest struct {
	TelegramUserID int64 `json:"telegram_user_id"`
}

type UpdateListingRequest struct {
	Status             *string  `json:"status,omitempty"`
	PricingJSON        any      `json:"pricing_json,omitempty"` // legacy compatibility
	PricePostTON       *string  `json:"price_post_ton,omitempty"`
	PriceRepostTON     *string  `json:"price_repost_ton,omitempty"`
	PriceStoryTON      *string  `json:"price_story_ton,omitempty"`
	FormatsEnabled     []string `json:"formats_enabled,omitempty"` // ["post","repost","story"]
	MinLeadTimeMinutes *int     `json:"min_lead_time_minutes,omitempty"`
	Description        *string  `json:"description,omitempty"`
	Category           *string  `json:"category,omitempty"`
	Language           *string  `json:"language,omitempty"`
	HoldHoursPost      *int     `json:"hold_hours_post,omitempty"`
	HoldHoursRepost    *int     `json:"hold_hours_repost,omitempty"`
	HoldHoursStory     *int     `json:"hold_hours_story,omitempty"`
	AutoAccept         *bool    `json:"auto_accept,omitempty"`
}

type CreateDealRequest struct {
	ChannelID   string     `json:"channel_id"`
	AdFormat    string     `json:"ad_format"` // post / repost / story
	Brief       *string    `json:"brief,omitempty"`
	PriceTON    string     `json:"price_ton,omitempty"` // если пусто — берём из листинга
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
}

type SubmitCreativeRequest struct {
	Text          string   `json:"text"`
	RepostFromURL *string  `json:"repost_from_url,omitempty"`
	MediaURLs     []string `json:"media_urls,omitempty"`
	ButtonsJSON   any      `json:"buttons_json,omitempty"`
}

type RequestCreativeChangesRequest struct {
	Feedback *string `json:"feedback,omitempty"`
}

type MarkManualPostRequest struct {
	PostURL string `json:"post_url"`
}

type SetWithdrawWalletRequest struct {
	WalletAddress string `json:"wallet_address"`
}

// Campaigns

type CreateCampaignRequest struct {
	Title          string     `json:"title"`
	TargetAudience string     `json:"target_audience"`
	KeyMessages    *string    `json:"key_messages,omitempty"`
	BudgetTON      string     `json:"budget_ton"`
	PreferredDate  *time.Time `json:"preferred_date,omitempty"`
}

type UpdateCampaignRequest struct {
	Title          string     `json:"title"`
	TargetAudience string     `json:"target_audience"`
	KeyMessages    *string    `json:"key_messages,omitempty"`
	BudgetTON      string     `json:"budget_ton"`
	PreferredDate  *time.Time `json:"preferred_date,omitempty"`
	Status         string     `json:"status,omitempty"`
}
