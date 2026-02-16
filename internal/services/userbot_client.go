package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// UserbotClient communicates with the Pyrogram userbot internal API.
type UserbotClient struct {
	baseURL    string
	httpClient *http.Client
	log        *zap.Logger
}

func NewUserbotClient(baseURL string, log *zap.Logger) *UserbotClient {
	return &UserbotClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		log: log,
	}
}

// UserbotStats represents rich channel statistics from the userbot.
type UserbotStats struct {
	Subscribers   *int    `json:"subscribers"`
	AdminsCount   *int    `json:"admins_count"`
	MembersOnline *int    `json:"members_online"`
	PostsCount    *int    `json:"posts_count"`
	Verified      bool    `json:"verified"`
	Title         *string `json:"title"`
	Username      *string `json:"username"`
	Description   *string `json:"description"`
	AvgViews20    *int    `json:"avg_views_20"`
	Growth7d      *int    `json:"growth_7d"`
	Growth30d     *int    `json:"growth_30d"`
	FetchedAt     string  `json:"fetched_at"`
	Source        string  `json:"source"`
	// Extended fields from GetBroadcastStats
	ViewsPerPost                *float64 `json:"views_per_post"`
	SharesPerPost               *float64 `json:"shares_per_post"`
	EnabledNotificationsPercent *float64 `json:"enabled_notifications_percent"`
	ERPercent                   *float64 `json:"er_percent"`
}

// UserbotMe represents the userbot's own Telegram info.
type UserbotMe struct {
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

// IsAvailable checks if the userbot service is reachable and connected.
func (c *UserbotClient) IsAvailable(ctx context.Context) bool {
	url := fmt.Sprintf("%s/health", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var result struct {
		Status    string `json:"status"`
		Connected bool   `json:"connected"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}
	return result.Connected
}

// GetMe returns the userbot's Telegram account info.
func (c *UserbotClient) GetMe(ctx context.Context) (*UserbotMe, error) {
	url := fmt.Sprintf("%s/internal/me", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userbot service unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userbot returned %d: %s", resp.StatusCode, string(body))
	}

	var me UserbotMe
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		return nil, err
	}
	return &me, nil
}

// GetStatsByUsername collects channel stats via the userbot by username.
func (c *UserbotClient) GetStatsByUsername(ctx context.Context, username string) (*UserbotStats, error) {
	url := fmt.Sprintf("%s/internal/stats/by-username/%s", c.baseURL, username)
	return c.fetchStats(ctx, url)
}

// GetStatsByChatID collects channel stats via the userbot by telegram chat_id.
func (c *UserbotClient) GetStatsByChatID(ctx context.Context, chatID int64) (*UserbotStats, error) {
	url := fmt.Sprintf("%s/internal/stats/by-chat-id/%d", c.baseURL, chatID)
	return c.fetchStats(ctx, url)
}

func (c *UserbotClient) fetchStats(ctx context.Context, url string) (*UserbotStats, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userbot service unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userbot returned %d: %s", resp.StatusCode, string(body))
	}

	var stats UserbotStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, err
	}
	return &stats, nil
}
