package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

type Config struct {
	// Database
	PostgresDSN string
	RedisURL    string

	// Bot
	BotToken      string
	BotInternalURL string

	// TON
	TONHotWalletAddress    string
	TONNetwork             string // mainnet/testnet
	LiteServerHost         string
	LiteServerPort         int
	LiteServerKey          string
	TONProofAllowedDomains []string // домены, разрешённые в TON Proof

	// Platform
	PlatformFeeBPS    int
	HoldPeriodSeconds int

	// Admin
	AdminTelegramIDs   []int64
	SupportTelegramIDs []int64

	// Deal timeouts
	DealTimeoutSubmittedSeconds int
	DealTimeoutAcceptedSeconds  int
	DealTimeoutCreativeSeconds  int
	DealTimeoutPaymentSeconds   int

	// Stats
	TMEFetchTimeoutMS    int
	TMEFetchMaxRetries   int
	StatsRefreshInterval time.Duration
	StatsActiveWindow    time.Duration

	// Userbot
	UserbotInternalURL string

	// Auth
	WebAppSecret   string
	JWTSecret      string
	JWTExpiration  time.Duration // время жизни JWT токена
	InitDataMaxAge time.Duration // макс. возраст auth_date из Telegram initData

	// Server
	APIPort    string
	WorkerPort string
}

func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		PostgresDSN:    getEnv("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/ads_marketplace?sslmode=disable"),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379/0"),
		BotToken:       getEnv("BOT_TOKEN", ""),
		BotInternalURL: getEnv("BOT_INTERNAL_URL", "http://localhost:8081"),

		TONHotWalletAddress:    getEnv("TON_HOT_WALLET_ADDRESS", ""),
		TONNetwork:             getEnv("TON_NETWORK", "testnet"),
		LiteServerHost:         getEnv("LITE_SERVER_HOST", ""),
		LiteServerPort:         getEnvInt("LITE_SERVER_PORT", 4443),
		LiteServerKey:          getEnv("LITE_SERVER_KEY", ""),
		TONProofAllowedDomains: parseDomainList(getEnv("TON_PROOF_ALLOWED_DOMAINS", "")),

		PlatformFeeBPS:    getEnvInt("PLATFORM_FEE_BPS", 300),
		HoldPeriodSeconds: getEnvInt("HOLD_PERIOD_SECONDS", 3600),

		AdminTelegramIDs:   parseIDList(getEnv("ADMIN_TELEGRAM_IDS", "")),
		SupportTelegramIDs: parseIDList(getEnv("SUPPORT_TELEGRAM_IDS", "")),

		DealTimeoutSubmittedSeconds: getEnvInt("DEAL_TIMEOUT_SUBMITTED_SECONDS", 86400),
		DealTimeoutAcceptedSeconds:  getEnvInt("DEAL_TIMEOUT_ACCEPTED_SECONDS", 86400),
		DealTimeoutCreativeSeconds:  getEnvInt("DEAL_TIMEOUT_CREATIVE_SECONDS", 172800),
		DealTimeoutPaymentSeconds:   getEnvInt("DEAL_TIMEOUT_PAYMENT_SECONDS", 3600),

		TMEFetchTimeoutMS:  getEnvInt("TME_FETCH_TIMEOUT_MS", 10000),
		TMEFetchMaxRetries: getEnvInt("TME_FETCH_MAX_RETRIES", 3),
		StatsRefreshInterval: time.Duration(getEnvInt("STATS_REFRESH_INTERVAL_HOURS", 6)) * time.Hour,
		StatsActiveWindow:    time.Duration(getEnvInt("STATS_ACTIVE_WINDOW_HOURS", 48)) * time.Hour,

		UserbotInternalURL: getEnv("USERBOT_INTERNAL_URL", "http://localhost:8082"),

		WebAppSecret:   getEnv("WEBAPP_SECRET", ""),
		JWTSecret:      getEnv("JWT_SECRET", "change-me-in-production"),
		JWTExpiration:  time.Duration(getEnvInt("JWT_EXPIRATION_HOURS", 24)) * time.Hour,
		InitDataMaxAge: time.Duration(getEnvInt("INIT_DATA_MAX_AGE_SECONDS", 300)) * time.Second, // 5 мин по умолчанию

		APIPort:    getEnv("API_PORT", "3000"),
		WorkerPort: getEnv("WORKER_PORT", "3001"),
	}

	if cfg.WebAppSecret == "" && cfg.BotToken != "" {
		cfg.WebAppSecret = cfg.BotToken
	}

	return cfg
}

func (c *Config) IsAdmin(telegramID int64) bool {
	for _, id := range c.AdminTelegramIDs {
		if id == telegramID {
			return true
		}
	}
	return false
}

func (c *Config) IsSupport(telegramID int64) bool {
	for _, id := range c.SupportTelegramIDs {
		if id == telegramID {
			return true
		}
	}
	return false
}

func (c *Config) Validate(log *zap.Logger) {
	if c.BotToken == "" {
		log.Warn("BOT_TOKEN is not set")
	}
	if c.JWTSecret == "change-me-in-production" {
		log.Warn("JWT_SECRET is default, change in production")
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	s := os.Getenv(key)
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return v
}

func parseDomainList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var domains []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			domains = append(domains, p)
		}
	}
	return domains
}

func parseIDList(s string) []int64 {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.ParseInt(p, 10, 64)
		if err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}
