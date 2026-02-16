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

type ChannelHandler struct {
	channelService *services.ChannelService
	log            *zap.Logger
}

func NewChannelHandler(channelService *services.ChannelService, log *zap.Logger) *ChannelHandler {
	return &ChannelHandler{channelService: channelService, log: log}
}

func (h *ChannelHandler) CreateChannel(c *fiber.Ctx) error {
	var req dto.CreateChannelRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid request"})
	}
	if req.Username == "" {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "username is required"})
	}

	userID := middleware.GetUserID(c)
	ch, err := h.channelService.CreateChannel(c.Context(), req.Username, userID)
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(dto.SuccessResponse{OK: true, Data: ch})
}

func (h *ChannelHandler) MyChannels(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	channels, err := h.channelService.GetMyChannels(c.Context(), userID)
	if err != nil {
		h.log.Error("get my channels failed", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: "internal error"})
	}

	return c.JSON(dto.SuccessResponse{OK: true, Data: channels})
}

func (h *ChannelHandler) GetChannel(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid channel id"})
	}

	ch, err := h.channelService.GetChannel(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(dto.ErrorResponse{Error: "channel not found"})
	}

	return c.JSON(dto.SuccessResponse{OK: true, Data: ch})
}

func (h *ChannelHandler) SearchChannels(c *fiber.Ctx) error {
	filter := repositories.ChannelFilter{
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
	if v := c.Query("min_subscribers"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.MinSubscribers = &n
		}
	}
	if v := c.Query("max_subscribers"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.MaxSubscribers = &n
		}
	}
	if v := c.Query("min_avg_views"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.MinAvgViews = &n
		}
	}
	if v := c.Query("status"); v != "" {
		filter.Status = &v
	}

	channels, err := h.channelService.SearchChannels(c.Context(), filter)
	if err != nil {
		h.log.Error("search channels failed", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: "internal error"})
	}

	return c.JSON(dto.SuccessResponse{OK: true, Data: channels})
}

func (h *ChannelHandler) InviteBot(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid channel id"})
	}

	instructions, err := h.channelService.GetBotInviteLink(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.JSON(dto.BotInviteResponse{Instructions: instructions})
}

func (h *ChannelHandler) AddManager(c *fiber.Ctx) error {
	channelID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid channel id"})
	}

	var req dto.AddManagerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid request"})
	}

	actorID := middleware.GetUserID(c)
	if err := h.channelService.AddManager(c.Context(), channelID, actorID, req.TelegramUserID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.JSON(dto.SuccessResponse{OK: true})
}

func (h *ChannelHandler) GetAdmins(c *fiber.Ctx) error {
	channelID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid channel id"})
	}

	admins, err := h.channelService.GetAdmins(c.Context(), channelID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.JSON(dto.SuccessResponse{OK: true, Data: admins})
}

func (h *ChannelHandler) UpdateListing(c *fiber.Ctx) error {
	channelID, err := uuid.Parse(c.Params("channelId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid channel id"})
	}

	var req dto.UpdateListingRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid request"})
	}

	listing := &models.ChannelListing{
		ChannelID:      channelID,
		FormatsEnabled: []string{models.AdFormatPost}, // default
	}
	if req.Status != nil {
		listing.Status = *req.Status
	} else {
		listing.Status = "draft"
	}
	if req.PricingJSON != nil {
		listing.PricingJSON = req.PricingJSON
	}
	if req.MinLeadTimeMinutes != nil {
		listing.MinLeadTimeMinutes = *req.MinLeadTimeMinutes
	}
	listing.Description = req.Description
	listing.Category = req.Category
	listing.Language = req.Language

	// Структурированные цены по формату
	listing.PricePostTON = req.PricePostTON
	listing.PriceRepostTON = req.PriceRepostTON
	listing.PriceStoryTON = req.PriceStoryTON

	// Включённые форматы
	if len(req.FormatsEnabled) > 0 {
		// Валидация форматов
		for _, f := range req.FormatsEnabled {
			if !models.IsValidAdFormat(f) {
				return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{
					Error: "invalid format: " + f + " (allowed: post, repost, story)",
				})
			}
		}
		listing.FormatsEnabled = req.FormatsEnabled
	}

	// Hold period по формату
	if req.HoldHoursPost != nil {
		listing.HoldHoursPost = *req.HoldHoursPost
	} else {
		listing.HoldHoursPost = 24
	}
	if req.HoldHoursRepost != nil {
		listing.HoldHoursRepost = *req.HoldHoursRepost
	} else {
		listing.HoldHoursRepost = 24
	}
	if req.HoldHoursStory != nil {
		listing.HoldHoursStory = *req.HoldHoursStory
	}

	// Авто-одобрение
	if req.AutoAccept != nil {
		listing.AutoAccept = *req.AutoAccept
	}

	actorID := middleware.GetUserID(c)
	if err := h.channelService.UpsertListing(c.Context(), channelID, actorID, listing); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.JSON(dto.SuccessResponse{OK: true, Data: listing})
}

func (h *ChannelHandler) GetListing(c *fiber.Ctx) error {
	channelID, err := uuid.Parse(c.Params("channelId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid channel id"})
	}

	listing, err := h.channelService.GetListing(c.Context(), channelID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(dto.ErrorResponse{Error: "listing not found"})
	}

	return c.JSON(dto.SuccessResponse{OK: true, Data: listing})
}

func (h *ChannelHandler) GetStats(c *fiber.Ctx) error {
	channelID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid channel id"})
	}

	stats, err := h.channelService.GetChannelStats(c.Context(), channelID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(dto.ErrorResponse{Error: "stats not found"})
	}

	return c.JSON(dto.SuccessResponse{OK: true, Data: stats})
}

func (h *ChannelHandler) ExploreChannels(c *fiber.Ctx) error {
	filter := repositories.ChannelFilter{
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
	if v := c.Query("min_subscribers"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.MinSubscribers = &n
		}
	}
	if v := c.Query("max_subscribers"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.MaxSubscribers = &n
		}
	}
	if v := c.Query("min_avg_views"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.MinAvgViews = &n
		}
	}
	if v := c.Query("status"); v != "" {
		filter.Status = &v
	}
	if v := c.Query("category"); v != "" {
		filter.Category = &v
	}
	if v := c.Query("language"); v != "" {
		filter.Language = &v
	}

	channels, err := h.channelService.ExploreChannels(c.Context(), filter)
	if err != nil {
		h.log.Error("explore channels failed", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: "internal error"})
	}

	return c.JSON(dto.SuccessResponse{OK: true, Data: channels})
}
