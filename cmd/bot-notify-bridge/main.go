package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ads-marketplace/backend/internal/config"
	"github.com/ads-marketplace/backend/internal/db"
	"github.com/ads-marketplace/backend/internal/events"
	"go.uber.org/zap"
)

// Bot Notify Bridge — optional small Go service that subscribes to
// Redis events and forwards notifications to the Python bot service.

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	cfg := config.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rdb, err := db.NewRedisClient(ctx, cfg.RedisURL, log)
	if err != nil {
		log.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer rdb.Close()

	subscriber := events.NewRedisSubscriber(rdb, log)

	log.Info("bot-notify-bridge started")

	// Subscribe to deal events and forward to bot
	_ = subscriber.Subscribe(ctx, "events:deal", func(event events.Event) {
		log.Info("forwarding event to bot", zap.String("type", event.Type))
		forwardToBot(cfg.BotInternalURL, event, log)
	})

	_ = subscriber.Subscribe(ctx, "events:bot", func(event events.Event) {
		log.Info("forwarding bot event", zap.String("type", event.Type))
		forwardToBot(cfg.BotInternalURL, event, log)
	})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Info("shutting down bot-notify-bridge")
	cancel()
}

func forwardToBot(baseURL string, event events.Event, log *zap.Logger) {
	// Extract notification data from event payload
	telegramUserID, ok := event.Payload["telegram_user_id"]
	if !ok {
		return
	}

	text, _ := event.Payload["text"].(string)
	if text == "" {
		text = fmt.Sprintf("Event: %s", event.Type)
	}

	body, _ := json.Marshal(map[string]any{
		"telegram_user_id": telegramUserID,
		"text":             text,
	})

	url := fmt.Sprintf("%s/internal/notify", strings.TrimRight(baseURL, "/"))
	resp, err := http.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		log.Warn("failed to forward notification", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Warn("bot notification returned non-200", zap.Int("status", resp.StatusCode))
	}
}
