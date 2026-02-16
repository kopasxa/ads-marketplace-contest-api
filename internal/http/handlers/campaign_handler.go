package handlers

import (
	"strconv"

	"github.com/ads-marketplace/backend/internal/http/dto"
	"github.com/ads-marketplace/backend/internal/middleware"
	"github.com/ads-marketplace/backend/internal/models"
	"github.com/ads-marketplace/backend/internal/repositories"
	"github.com/ads-marketplace/backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type CampaignHandler struct {
	campaignService *services.CampaignService
	log             *zap.Logger
}

func NewCampaignHandler(campaignService *services.CampaignService, log *zap.Logger) *CampaignHandler {
	return &CampaignHandler{campaignService: campaignService, log: log}
}

func (h *CampaignHandler) CreateCampaign(c *fiber.Ctx) error {
	var req dto.CreateCampaignRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid request"})
	}

	if req.Title == "" || req.TargetAudience == "" || req.BudgetTON == "" {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "title, target_audience, and budget_ton are required"})
	}

	campaign := &models.Campaign{
		Title:          req.Title,
		TargetAudience: req.TargetAudience,
		KeyMessages:    req.KeyMessages,
		BudgetTON:      req.BudgetTON,
		PreferredDate:  req.PreferredDate,
	}

	userID := middleware.GetUserID(c)
	if err := h.campaignService.Create(c.Context(), userID, campaign); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(dto.SuccessResponse{OK: true, Data: campaign})
}

func (h *CampaignHandler) GetCampaign(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid campaign id"})
	}

	userID := middleware.GetUserID(c)
	campaign, err := h.campaignService.GetByID(c.Context(), id, userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(dto.ErrorResponse{Error: "campaign not found"})
	}

	return c.JSON(dto.SuccessResponse{OK: true, Data: campaign})
}

func (h *CampaignHandler) ListCampaigns(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	filter := repositories.CampaignFilter{
		Limit:  20,
		Offset: 0,
	}

	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Offset = n
		}
	}

	campaigns, err := h.campaignService.List(c.Context(), userID, filter)
	if err != nil {
		h.log.Error("list campaigns failed", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: "internal error"})
	}

	return c.JSON(dto.SuccessResponse{OK: true, Data: campaigns})
}

func (h *CampaignHandler) UpdateCampaign(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid campaign id"})
	}

	var req dto.UpdateCampaignRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid request"})
	}

	campaign := &models.Campaign{
		Title:          req.Title,
		TargetAudience: req.TargetAudience,
		KeyMessages:    req.KeyMessages,
		BudgetTON:      req.BudgetTON,
		PreferredDate:  req.PreferredDate,
		Status:         req.Status,
	}

	userID := middleware.GetUserID(c)
	if err := h.campaignService.Update(c.Context(), id, userID, campaign); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	updated, _ := h.campaignService.GetByID(c.Context(), id, userID)
	return c.JSON(dto.SuccessResponse{OK: true, Data: updated})
}

func (h *CampaignHandler) DeleteCampaign(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid campaign id"})
	}

	userID := middleware.GetUserID(c)
	if err := h.campaignService.Delete(c.Context(), id, userID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.JSON(dto.SuccessResponse{OK: true})
}
