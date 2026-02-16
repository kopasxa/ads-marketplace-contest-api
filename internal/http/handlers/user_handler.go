package handlers

import (
	"github.com/ads-marketplace/backend/internal/http/dto"
	"github.com/ads-marketplace/backend/internal/middleware"
	"github.com/ads-marketplace/backend/internal/repositories"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type UserHandler struct {
	userRepo *repositories.UserRepo
	log      *zap.Logger
}

func NewUserHandler(userRepo *repositories.UserRepo, log *zap.Logger) *UserHandler {
	return &UserHandler{userRepo: userRepo, log: log}
}

func (h *UserHandler) GetMe(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	user, err := h.userRepo.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(dto.ErrorResponse{Error: "user not found"})
	}
	return c.JSON(dto.SuccessResponse{OK: true, Data: user})
}

func (h *UserHandler) Ping(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if err := h.userRepo.UpdateLastActive(c.Context(), userID); err != nil {
		h.log.Error("failed to update last_active", zap.Error(err))
	}
	return c.JSON(dto.SuccessResponse{OK: true})
}
