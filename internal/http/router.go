package http

import (
	"time"

	"github.com/ads-marketplace/backend/internal/config"
	"github.com/ads-marketplace/backend/internal/http/handlers"
	"github.com/ads-marketplace/backend/internal/middleware"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func SetupRouter(
	app *fiber.App,
	cfg *config.Config,
	log *zap.Logger,
	rdb *redis.Client,
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	channelHandler *handlers.ChannelHandler,
	dealHandler *handlers.DealHandler,
	walletHandler *handlers.WalletHandler,
	campaignHandler *handlers.CampaignHandler,
	wsHub *handlers.WSHub,
) {
	// Global middleware
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Request-ID",
	}))
	app.Use(middleware.RequestIDMiddleware())
	app.Use(middleware.LoggerMiddleware(log))

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	api := app.Group("/api/v1")

	// Auth (public)
	api.Post("/auth/telegram", authHandler.TelegramAuth)

	// Rate-limited public endpoints
	api.Use(middleware.RateLimitMiddleware(rdb, 100, time.Minute))

	// Meta (public, no auth required)
	metaHandler := handlers.NewMetaHandler()
	api.Get("/meta/categories", metaHandler.GetCategories)
	api.Get("/meta/languages", metaHandler.GetLanguages)

	// Protected endpoints
	protected := api.Group("", middleware.AuthMiddleware(cfg, log))

	// User
	protected.Get("/me", userHandler.GetMe)
	protected.Post("/me/ping", userHandler.Ping)

	// Wallet (TON Connect + Proof)
	protected.Post("/me/wallet/proof-payload", walletHandler.GeneratePayload)
	protected.Post("/me/wallet/connect", walletHandler.ConnectWallet)
	protected.Delete("/me/wallet", walletHandler.DisconnectWallet)
	protected.Get("/me/wallet", walletHandler.GetWallet)

	// Channels
	protected.Post("/channels", channelHandler.CreateChannel)
	protected.Get("/channels/my", channelHandler.MyChannels)
	protected.Get("/channels", channelHandler.SearchChannels)
	protected.Get("/channels/:id", channelHandler.GetChannel)
	protected.Get("/channels/:id/stats", channelHandler.GetStats)
	protected.Post("/channels/:id/invite-bot", channelHandler.InviteBot)
	protected.Post("/channels/:id/managers", channelHandler.AddManager)
	protected.Get("/channels/:id/admins", channelHandler.GetAdmins)

	// Explore (enriched channels with stats + listing)
	protected.Get("/explore/channels", channelHandler.ExploreChannels)

	// Listings
	protected.Put("/listings/:channelId", channelHandler.UpdateListing)
	protected.Get("/listings/:channelId", channelHandler.GetListing)

	// Campaigns
	protected.Post("/campaigns", campaignHandler.CreateCampaign)
	protected.Get("/campaigns", campaignHandler.ListCampaigns)
	protected.Get("/campaigns/:id", campaignHandler.GetCampaign)
	protected.Put("/campaigns/:id", campaignHandler.UpdateCampaign)
	protected.Delete("/campaigns/:id", campaignHandler.DeleteCampaign)

	// Deals
	protected.Post("/deals", dealHandler.CreateDeal)
	protected.Get("/deals", dealHandler.ListDeals)
	protected.Get("/deals/:id", dealHandler.GetDeal)
	protected.Post("/deals/:id/submit", dealHandler.SubmitDeal)
	protected.Post("/deals/:id/accept", dealHandler.AcceptDeal)
	protected.Post("/deals/:id/reject", dealHandler.RejectDeal)
	protected.Post("/deals/:id/cancel", dealHandler.CancelDeal)
	protected.Get("/deals/:id/creative", dealHandler.GetCreative)
	protected.Post("/deals/:id/creative", dealHandler.SubmitCreative)
	protected.Post("/deals/:id/creative/approve", dealHandler.ApproveCreative)
	protected.Post("/deals/:id/creative/request-changes", dealHandler.RequestCreativeChanges)
	protected.Get("/deals/:id/events", dealHandler.GetDealEvents)
	protected.Post("/deals/:id/post/mark-manual", dealHandler.MarkManualPost)
	protected.Post("/deals/:id/finance/set-withdraw-wallet", dealHandler.SetWithdrawWallet)
	protected.Get("/deals/:id/payment", dealHandler.GetPaymentInfo)

	// WebSocket
	app.Use("/ws", handlers.WSUpgradeMiddleware())
	app.Get("/ws", websocket.New(wsHub.HandleWS))
}
