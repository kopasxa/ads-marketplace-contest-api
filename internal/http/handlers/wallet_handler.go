package handlers

import (
	"github.com/ads-marketplace/backend/internal/http/dto"
	"github.com/ads-marketplace/backend/internal/middleware"
	"github.com/ads-marketplace/backend/internal/services"
	"github.com/ads-marketplace/backend/internal/ton"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type WalletHandler struct {
	walletService *services.WalletService
	log           *zap.Logger
}

func NewWalletHandler(walletService *services.WalletService, log *zap.Logger) *WalletHandler {
	return &WalletHandler{walletService: walletService, log: log}
}

// GeneratePayload создаёт nonce для TON Proof.
// POST /me/wallet/proof-payload
func (h *WalletHandler) GeneratePayload(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	payload, err := h.walletService.GeneratePayload(c.Context(), &userID)
	if err != nil {
		h.log.Error("failed to generate proof payload", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: "internal error"})
	}
	return c.JSON(fiber.Map{"payload": payload})
}

// ConnectWallet подключает кошелёк после проверки TON Proof.
// POST /me/wallet/connect
func (h *WalletHandler) ConnectWallet(c *fiber.Ctx) error {
	var req struct {
		Address         string    `json:"address"`
		AddressFriendly string    `json:"address_friendly"`
		Network         string    `json:"network"`
		PublicKey       string    `json:"public_key"`
		Proof           ton.Proof `json:"proof"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "invalid request body"})
	}

	if req.Address == "" || req.PublicKey == "" || req.Proof.Signature == "" {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: "address, public_key, and proof.signature are required"})
	}
	if req.AddressFriendly == "" {
		req.AddressFriendly = req.Address
	}
	if req.Network == "" {
		req.Network = "mainnet"
	}

	userID := middleware.GetUserID(c)
	wallet, err := h.walletService.ConnectWallet(c.Context(), userID, services.ConnectWalletRequest{
		Address:         req.Address,
		AddressFriendly: req.AddressFriendly,
		Network:         req.Network,
		PublicKey:       req.PublicKey,
		Proof:           req.Proof,
	})
	if err != nil {
		h.log.Debug("wallet connect failed", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(dto.ErrorResponse{Error: err.Error()})
	}

	return c.JSON(dto.SuccessResponse{OK: true, Data: wallet})
}

// DisconnectWallet отключает кошелёк.
// DELETE /me/wallet
func (h *WalletHandler) DisconnectWallet(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if err := h.walletService.DisconnectWallet(c.Context(), userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{Error: "failed to disconnect wallet"})
	}
	return c.JSON(dto.SuccessResponse{OK: true})
}

// GetWallet возвращает активный кошелёк.
// GET /me/wallet
func (h *WalletHandler) GetWallet(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	wallet, err := h.walletService.GetActiveWallet(c.Context(), userID)
	if err != nil {
		return c.JSON(dto.SuccessResponse{OK: true, Data: nil})
	}
	return c.JSON(dto.SuccessResponse{OK: true, Data: wallet})
}
