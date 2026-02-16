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
	"testing"
	"time"
)

// helper: собирает initData с валидным hash и заданным auth_date
func buildInitData(botToken string, authDate time.Time, extra map[string]string) string {
	params := url.Values{}
	params.Set("auth_date", strconv.FormatInt(authDate.Unix(), 10))
	for k, v := range extra {
		params.Set(k, v)
	}

	var pairs []string
	for key, values := range params {
		for _, v := range values {
			pairs = append(pairs, fmt.Sprintf("%s=%s", key, v))
		}
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(pairs, "\n")

	secretKey := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	hash := hmacSHA256(secretKey, []byte(dataCheckString))
	params.Set("hash", hex.EncodeToString(hash))

	return params.Encode()
}

func TestValidateTelegramWebAppData_ValidHash(t *testing.T) {
	botToken := "test-bot-token-12345"

	initData := buildInitData(botToken, time.Now().Add(-30*time.Second), map[string]string{
		"query_id": "test_query_id",
		"user":     `{"id":123456,"first_name":"Test","username":"testuser"}`,
	})

	result, err := ValidateTelegramWebAppData(initData, botToken, 5*time.Minute)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Get("query_id") != "test_query_id" {
		t.Errorf("expected query_id=test_query_id, got %s", result.Get("query_id"))
	}
}

func TestValidateTelegramWebAppData_ExpiredAuthDate(t *testing.T) {
	botToken := "test-bot-token-12345"

	// auth_date 10 минут назад, maxAge = 5 мин → expired
	initData := buildInitData(botToken, time.Now().Add(-10*time.Minute), map[string]string{
		"user": `{"id":123456}`,
	})

	_, err := ValidateTelegramWebAppData(initData, botToken, 5*time.Minute)
	if err == nil {
		t.Fatal("expected error for expired initData")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected 'expired' in error, got: %s", err.Error())
	}
}

func TestValidateTelegramWebAppData_FutureAuthDate(t *testing.T) {
	botToken := "test-bot-token-12345"

	// auth_date 5 минут в будущем → rejected
	initData := buildInitData(botToken, time.Now().Add(5*time.Minute), map[string]string{
		"user": `{"id":123456}`,
	})

	_, err := ValidateTelegramWebAppData(initData, botToken, 5*time.Minute)
	if err == nil {
		t.Fatal("expected error for future auth_date")
	}
	if !strings.Contains(err.Error(), "future") {
		t.Errorf("expected 'future' in error, got: %s", err.Error())
	}
}

func TestValidateTelegramWebAppData_DefaultMaxAge(t *testing.T) {
	botToken := "test-bot-token-12345"

	// auth_date свежий, maxAge = 0 → должен использоваться DefaultInitDataTTL (5 мин)
	initData := buildInitData(botToken, time.Now().Add(-10*time.Second), map[string]string{
		"user": `{"id":123456}`,
	})

	_, err := ValidateTelegramWebAppData(initData, botToken, 0)
	if err != nil {
		t.Fatalf("expected no error with default maxAge, got: %v", err)
	}
}

func TestValidateTelegramWebAppData_InvalidHash(t *testing.T) {
	params := url.Values{}
	params.Set("auth_date", strconv.FormatInt(time.Now().Unix(), 10))
	params.Set("user", `{"id":123456}`)
	params.Set("hash", "invalidhash")

	_, err := ValidateTelegramWebAppData(params.Encode(), "test-bot-token-12345", 5*time.Minute)
	if err == nil {
		t.Fatal("expected error for invalid hash")
	}
}

func TestValidateTelegramWebAppData_MissingHash(t *testing.T) {
	params := url.Values{}
	params.Set("auth_date", strconv.FormatInt(time.Now().Unix(), 10))

	_, err := ValidateTelegramWebAppData(params.Encode(), "token", 5*time.Minute)
	if err == nil {
		t.Fatal("expected error for missing hash")
	}
}

func TestValidateTelegramWebAppData_MissingAuthDate(t *testing.T) {
	params := url.Values{}
	params.Set("user", `{"id":123456}`)
	params.Set("hash", "somehash")

	_, err := ValidateTelegramWebAppData(params.Encode(), "token", 5*time.Minute)
	if err == nil {
		t.Fatal("expected error for missing auth_date")
	}
}

func TestHmacSHA256(t *testing.T) {
	key := []byte("test-key")
	data := []byte("test-data")

	result := hmacSHA256(key, data)

	h := hmac.New(sha256.New, key)
	h.Write(data)
	expected := h.Sum(nil)

	if !hmac.Equal(result, expected) {
		t.Error("hmacSHA256 result doesn't match expected")
	}
}
