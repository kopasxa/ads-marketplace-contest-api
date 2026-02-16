package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ads-marketplace/backend/internal/config"
	"github.com/ads-marketplace/backend/internal/db"
	"github.com/ads-marketplace/backend/internal/events"
	apphttp "github.com/ads-marketplace/backend/internal/http"
	"github.com/ads-marketplace/backend/internal/http/handlers"
	"github.com/ads-marketplace/backend/internal/repositories"
	"github.com/ads-marketplace/backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	cfg := config.Load()
	cfg.Validate(log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Database
	pool, err := db.NewPostgresPool(ctx, cfg.PostgresDSN, log)
	if err != nil {
		log.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer pool.Close()

	// Run migrations
	if err := db.RunMigrations(ctx, pool, "migrations", log); err != nil {
		log.Fatal("failed to run migrations", zap.Error(err))
	}

	// Redis
	rdb, err := db.NewRedisClient(ctx, cfg.RedisURL, log)
	if err != nil {
		log.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer rdb.Close()

	// Repositories
	userRepo := repositories.NewUserRepo(pool)
	channelRepo := repositories.NewChannelRepo(pool)
	dealRepo := repositories.NewDealRepo(pool)
	escrowRepo := repositories.NewEscrowRepo(pool)
	auditRepo := repositories.NewAuditRepo(pool)
	withdrawRepo := repositories.NewWithdrawRepo(pool)
	walletRepo := repositories.NewWalletRepo(pool)
	campaignRepo := repositories.NewCampaignRepo(pool)

	// Events
	publisher := events.NewRedisPublisher(rdb, log)
	subscriber := events.NewRedisSubscriber(rdb, log)

	// Services
	botClient := services.NewBotClient(cfg.BotInternalURL, log)
	dealService := services.NewDealService(dealRepo, channelRepo, escrowRepo, auditRepo, withdrawRepo, walletRepo, botClient, publisher, cfg, log)
	channelService := services.NewChannelService(channelRepo, userRepo, auditRepo, botClient, cfg, log)
	walletService := services.NewWalletService(walletRepo, auditRepo, cfg, log)
	campaignService := services.NewCampaignService(campaignRepo, auditRepo, log)

	// Handlers
	authHandler := handlers.NewAuthHandler(userRepo, cfg, log)
	userHandler := handlers.NewUserHandler(userRepo, log)
	channelHandler := handlers.NewChannelHandler(channelService, log)
	dealHandler := handlers.NewDealHandler(dealService, log)
	walletHandler := handlers.NewWalletHandler(walletService, log)
	campaignHandler := handlers.NewCampaignHandler(campaignService, log)
	wsHub := handlers.NewWSHub(cfg, subscriber, log)

	// Start WS hub
	wsHub.Start(ctx)

	// Fiber app
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"error": err.Error()})
		},
	})

	apphttp.SetupRouter(app, cfg, log, rdb, authHandler, userHandler, channelHandler, dealHandler, walletHandler, campaignHandler, wsHub)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Info("shutting down...")
		cancel()
		_ = app.Shutdown()
	}()

	addr := fmt.Sprintf(":%s", cfg.APIPort)
	log.Info("starting API server", zap.String("addr", addr))
	if err := app.Listen(addr); err != nil {
		log.Fatal("server error", zap.Error(err))
	}
}
