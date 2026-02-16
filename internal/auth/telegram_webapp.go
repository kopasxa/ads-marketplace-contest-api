package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultInitDataTTL — максимальный возраст auth_date initData.
	// Telegram не навязывает конкретный TTL, но рекомендует проверять свежесть.
	// Для mini-app разумно 5 минут — initData генерируется при каждом открытии.
	DefaultInitDataTTL = 5 * time.Minute
)

// ValidateTelegramWebAppData validates initData from Telegram WebApp.
// https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app
//
// maxAge — максимально допустимый возраст auth_date. Если <= 0, используется DefaultInitDataTTL.
func ValidateTelegramWebAppData(initData string, botToken string, maxAge time.Duration) (url.Values, error) {
	if maxAge <= 0 {
		maxAge = DefaultInitDataTTL
	}

	vals, err := url.ParseQuery(initData)
	if err != nil {
		return nil, fmt.Errorf("invalid initData format: %w", err)
	}

	receivedHash := vals.Get("hash")
	if receivedHash == "" {
		return nil, fmt.Errorf("hash is missing from initData")
	}

	// ---- Проверяем auth_date (свежесть) ----
	authDateStr := vals.Get("auth_date")
	if authDateStr == "" {
		return nil, fmt.Errorf("auth_date is missing from initData")
	}
	authDateUnix, err := strconv.ParseInt(authDateStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("auth_date is not a valid unix timestamp")
	}
	authDate := time.Unix(authDateUnix, 0)
	if time.Since(authDate) > maxAge {
		return nil, fmt.Errorf("initData expired: auth_date is %s old (max %s)", time.Since(authDate).Round(time.Second), maxAge)
	}
	// Защита от auth_date из будущего (clock skew макс. 1 мин)
	if authDate.After(time.Now().Add(1 * time.Minute)) {
		return nil, fmt.Errorf("auth_date is in the future")
	}

	// ---- Проверяем HMAC-SHA256 подпись ----
	var pairs []string
	for key, values := range vals {
		if key == "hash" {
			continue
		}
		for _, v := range values {
			pairs = append(pairs, fmt.Sprintf("%s=%s", key, v))
		}
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(pairs, "\n")

	// secret_key = HMAC-SHA256("WebAppData", bot_token)
	secretKey := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	hash := hmacSHA256(secretKey, []byte(dataCheckString))
	calculatedHash := hex.EncodeToString(hash)

	if calculatedHash != receivedHash {
		return nil, fmt.Errorf("invalid hash: data integrity check failed")
	}

	return vals, nil
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
