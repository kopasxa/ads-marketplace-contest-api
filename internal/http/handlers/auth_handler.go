package handlers

import (
	"encoding/json"

	"github.com/ads-marketplace/backend/internal/auth"
	"github.com/ads-marketplace/backend/internal/config"
	"github.com/ads-marketplace/backend/internal/http/dto"
	"github.com/ads-marketplace/backend/internal/repositories"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type AuthHandler struct {
	userRepo *repositories.UserRepo
	cfg      *config.Config
	log      *zap.Logger
}

func NewAuthHandler(userRepo *repositories.UserRepo, cfg *config.Config, log *zap.Logger) *AuthHandler {
	return &AuthHandler{userRepo: userRepo, cfg: cfg, log: log}
}

func (h *AuthHandler) TelegramAuth(c *fiber.Ctx) error {
	var req dto.AuthTelegramRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid request body"})
	}

	if req.InitData == "" {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "init_data is required"})
	}

	vals, err := auth.ValidateTelegramWebAppData(req.InitData, h.cfg.WebAppSecret, h.cfg.InitDataMaxAge)
	if err != nil {
		h.log.Debug("telegram auth validation failed", zap.Error(err))
		return c.Status(fiber.StatusUnauthorized).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	// Parse user from initData
	userJSON := vals.Get("user")
	if userJSON == "" {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "user data missing from init_data"})
	}

	var tgUser struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	if err := json.Unmarshal([]byte(userJSON), &tgUser); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid user data"})
	}

	var username, firstName, lastName *string
	if tgUser.Username != "" {
		username = &tgUser.Username
	}
	if tgUser.FirstName != "" {
		firstName = &tgUser.FirstName
	}
	if tgUser.LastName != "" {
		lastName = &tgUser.LastName
	}

	user, err := h.userRepo.UpsertByTelegramID(c.Context(), tgUser.ID, username, firstName, lastName)
	if err != nil {
		h.log.Error("failed to upsert user", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: "internal server error"})
	}

	token, err := auth.GenerateJWT(h.cfg.JWTSecret, user.ID, user.TelegramUserID, h.cfg.JWTExpiration)
	if err != nil {
		h.log.Error("failed to generate jwt", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: "internal server error"})
	}

	return c.JSON(dto.AuthResponse{
		Token: token,
		User:  user,
	})
}
