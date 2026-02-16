package middleware

import (
	"strings"

	"github.com/ads-marketplace/backend/internal/auth"
	"github.com/ads-marketplace/backend/internal/config"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	CtxUserID         = "user_id"
	CtxTelegramUserID = "telegram_user_id"
)

func AuthMiddleware(cfg *config.Config, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authorization header"})
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenStr == authHeader {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid authorization format"})
		}

		claims, err := auth.ParseJWT(cfg.JWTSecret, tokenStr)
		if err != nil {
			log.Debug("jwt parse error", zap.Error(err))
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired token"})
		}

		c.Locals(CtxUserID, claims.UserID)
		c.Locals(CtxTelegramUserID, claims.TelegramUserID)

		return c.Next()
	}
}

func GetUserID(c *fiber.Ctx) uuid.UUID {
	id, _ := c.Locals(CtxUserID).(uuid.UUID)
	return id
}

func GetTelegramUserID(c *fiber.Ctx) int64 {
	id, _ := c.Locals(CtxTelegramUserID).(int64)
	return id
}

// AdminMiddleware requires admin telegram IDs
func AdminMiddleware(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		telegramID := GetTelegramUserID(c)
		if !cfg.IsAdmin(telegramID) && !cfg.IsSupport(telegramID) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "admin access required"})
		}
		return c.Next()
	}
}
