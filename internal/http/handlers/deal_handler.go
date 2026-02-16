package handlers

import (
	"strconv"

	"github.com/ads-marketplace/backend/internal/http/dto"
	"github.com/ads-marketplace/backend/internal/middleware"
	"github.com/ads-marketplace/backend/internal/repositories"
	"github.com/ads-marketplace/backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type DealHandler struct {
	dealService *services.DealService
	log         *zap.Logger
}

func NewDealHandler(dealService *services.DealService, log *zap.Logger) *DealHandler {
	return &DealHandler{dealService: dealService, log: log}
}

func (h *DealHandler) CreateDeal(c *fiber.Ctx) error {
	var req dto.CreateDealRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid request"})
	}

	channelID, err := uuid.Parse(req.ChannelID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid channel_id"})
	}

	// ad_format обязателен
	if req.AdFormat == "" {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "ad_format is required (post, repost, story)"})
	}

	actorID := middleware.GetUserID(c)
	deal, err := h.dealService.CreateDeal(c.Context(), actorID, channelID, req.AdFormat, req.Brief, req.PriceTON, req.ScheduledAt)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(dto.SuccessResponse{OK: true, Data: deal})
}

func (h *DealHandler) GetDeal(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid deal id"})
	}

	deal, err := h.dealService.GetDeal(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(dto.ErrorResponse{Error: "deal not found"})
	}

	return c.JSON(dto.SuccessResponse{OK: true, Data: deal})
}

func (h *DealHandler) ListDeals(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	filter := repositories.DealFilter{
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
	if v := c.Query("status"); v != "" {
		filter.Status = &v
	}

	role := c.Query("role")
	switch role {
	case "advertiser":
		filter.AdvertiserUserID = &userID
	case "owner":
		filter.OwnerUserID = &userID
	default:
		// Return both — advertiser's own deals + channel deals they manage
		filter.AdvertiserUserID = &userID
	}

	deals, err := h.dealService.ListDeals(c.Context(), filter)
	if err != nil {
		h.log.Error("list deals failed", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: "internal error"})
	}

	return c.JSON(dto.SuccessResponse{OK: true, Data: deals})
}

func (h *DealHandler) AcceptDeal(c *fiber.Ctx) error {
	dealID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid deal id"})
	}

	actorID := middleware.GetUserID(c)
	if err := h.dealService.AcceptDeal(c.Context(), dealID, actorID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.JSON(dto.SuccessResponse{OK: true})
}

func (h *DealHandler) RejectDeal(c *fiber.Ctx) error {
	dealID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid deal id"})
	}

	actorID := middleware.GetUserID(c)
	if err := h.dealService.RejectDeal(c.Context(), dealID, actorID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.JSON(dto.SuccessResponse{OK: true})
}

func (h *DealHandler) CancelDeal(c *fiber.Ctx) error {
	dealID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid deal id"})
	}

	actorID := middleware.GetUserID(c)
	if err := h.dealService.CancelDeal(c.Context(), dealID, actorID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.JSON(dto.SuccessResponse{OK: true})
}

func (h *DealHandler) SubmitCreative(c *fiber.Ctx) error {
	dealID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid deal id"})
	}

	var req dto.SubmitCreativeRequest
	if err := c.BodyParser(&req); err != nil || req.Text == "" {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "text is required"})
	}

	input := services.SubmitCreativeInput{
		Text:          req.Text,
		RepostFromURL: req.RepostFromURL,
		MediaURLs:     req.MediaURLs,
		ButtonsJSON:   req.ButtonsJSON,
	}

	actorID := middleware.GetUserID(c)
	if err := h.dealService.SubmitCreative(c.Context(), dealID, actorID, input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.JSON(dto.SuccessResponse{OK: true})
}

func (h *DealHandler) ApproveCreative(c *fiber.Ctx) error {
	dealID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid deal id"})
	}

	actorID := middleware.GetUserID(c)
	if err := h.dealService.ApproveCreative(c.Context(), dealID, actorID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.JSON(dto.SuccessResponse{OK: true})
}

func (h *DealHandler) RequestCreativeChanges(c *fiber.Ctx) error {
	dealID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid deal id"})
	}

	var req dto.RequestCreativeChangesRequest
	_ = c.BodyParser(&req)

	actorID := middleware.GetUserID(c)
	if err := h.dealService.RequestCreativeChanges(c.Context(), dealID, actorID, req.Feedback); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.JSON(dto.SuccessResponse{OK: true})
}

func (h *DealHandler) GetCreative(c *fiber.Ctx) error {
	dealID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid deal id"})
	}

	creative, err := h.dealService.GetLatestCreative(c.Context(), dealID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(dto.ErrorResponse{Error: "creative not found"})
	}

	return c.JSON(dto.SuccessResponse{OK: true, Data: creative})
}

func (h *DealHandler) GetDealEvents(c *fiber.Ctx) error {
	dealID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid deal id"})
	}

	events, err := h.dealService.GetDealEvents(c.Context(), dealID)
	if err != nil {
		h.log.Error("get deal events failed", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: "internal error"})
	}

	return c.JSON(dto.SuccessResponse{OK: true, Data: events})
}

func (h *DealHandler) MarkManualPost(c *fiber.Ctx) error {
	dealID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid deal id"})
	}

	var req dto.MarkManualPostRequest
	if err := c.BodyParser(&req); err != nil || req.PostURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "post_url is required"})
	}

	actorID := middleware.GetUserID(c)
	if err := h.dealService.MarkManualPost(c.Context(), dealID, actorID, req.PostURL); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.JSON(dto.SuccessResponse{OK: true})
}

func (h *DealHandler) SetWithdrawWallet(c *fiber.Ctx) error {
	dealID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid deal id"})
	}

	var req dto.SetWithdrawWalletRequest
	if err := c.BodyParser(&req); err != nil || req.WalletAddress == "" {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "wallet_address is required"})
	}

	actorID := middleware.GetUserID(c)
	if err := h.dealService.SetWithdrawWallet(c.Context(), dealID, actorID, req.WalletAddress); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.JSON(dto.SuccessResponse{OK: true})
}

func (h *DealHandler) GetPaymentInfo(c *fiber.Ctx) error {
	dealID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid deal id"})
	}

	escrow, err := h.dealService.GetPaymentInfo(c.Context(), dealID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(dto.ErrorResponse{Error: "payment info not found"})
	}

	return c.JSON(dto.PaymentInfoResponse{
		DealID:        dealID.String(),
		WalletAddress: escrow.DepositAddress,
		Memo:          escrow.DepositMemo,
		AmountTON:     escrow.DepositExpectedTON,
		Status:        escrow.Status,
	})
}

func (h *DealHandler) SubmitDeal(c *fiber.Ctx) error {
	dealID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid deal id"})
	}

	actorID := middleware.GetUserID(c)
	if err := h.dealService.SubmitDeal(c.Context(), dealID, actorID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.JSON(dto.SuccessResponse{OK: true})
}
