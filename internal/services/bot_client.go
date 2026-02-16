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

// BotClient communicates with the Python bot internal API.
type BotClient struct {
	baseURL    string
	httpClient *http.Client
	log        *zap.Logger
}

func NewBotClient(baseURL string, log *zap.Logger) *BotClient {
	return &BotClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		log: log,
	}
}

type AdminInfo struct {
	TelegramUserID  int64  `json:"telegram_user_id"`
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	CanPostMessages bool   `json:"can_post_messages"`
	IsOwner         bool   `json:"is_owner"`
}

func (c *BotClient) GetAdmins(ctx context.Context, channelUsername string) ([]AdminInfo, error) {
	url := fmt.Sprintf("%s/internal/channels/%s/admins", c.baseURL, channelUsername)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bot service unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bot service returned %d: %s", resp.StatusCode, string(body))
	}

	var admins []AdminInfo
	if err := json.NewDecoder(resp.Body).Decode(&admins); err != nil {
		return nil, err
	}
	return admins, nil
}

type CheckAdminResult struct {
	IsAdmin         bool `json:"is_admin"`
	CanPostMessages bool `json:"can_post_messages"`
}

func (c *BotClient) CheckAdmin(ctx context.Context, channelUsername string, telegramUserID int64) (*CheckAdminResult, error) {
	url := fmt.Sprintf("%s/internal/channels/%s/check_admin?telegram_user_id=%d", c.baseURL, channelUsername, telegramUserID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bot service unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bot service returned %d: %s", resp.StatusCode, string(body))
	}

	var result CheckAdminResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

type PostRequest struct {
	DealID      string `json:"deal_id"`
	ChatID      int64  `json:"chat_id"`
	Text        string `json:"text"`
	ScheduledAt string `json:"scheduled_at,omitempty"`
}

type PostResult struct {
	MessageID int64  `json:"message_id"`
	ChatID    int64  `json:"chat_id"`
	PostURL   string `json:"post_url"`
}

func (c *BotClient) PostToDeal(ctx context.Context, req PostRequest) (*PostResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/internal/deals/%s/post", c.baseURL, req.DealID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("bot service unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bot service returned %d: %s", resp.StatusCode, string(b))
	}

	var result PostResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *BotClient) SendNotification(ctx context.Context, telegramUserID int64, text string) error {
	body, _ := json.Marshal(map[string]any{
		"telegram_user_id": telegramUserID,
		"text":             text,
	})

	url := fmt.Sprintf("%s/internal/notify", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Warn("failed to send bot notification", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.log.Warn("bot notification failed", zap.Int("status", resp.StatusCode))
	}
	return nil
}
