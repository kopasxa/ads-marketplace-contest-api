package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ads-marketplace/backend/internal/config"
	"github.com/ads-marketplace/backend/internal/db"
	"github.com/ads-marketplace/backend/internal/models"
	"github.com/ads-marketplace/backend/internal/repositories"
	"github.com/ads-marketplace/backend/internal/services"
	"github.com/ads-marketplace/backend/internal/statsparser"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	cfg := config.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.NewPostgresPool(ctx, cfg.PostgresDSN, log)
	if err != nil {
		log.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer pool.Close()

	rdb, err := db.NewRedisClient(ctx, cfg.RedisURL, log)
	if err != nil {
		log.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer rdb.Close()

	channelRepo := repositories.NewChannelRepo(pool)
	parser := statsparser.NewParser(cfg.TMEFetchTimeoutMS, cfg.TMEFetchMaxRetries, log)
	userbotClient := services.NewUserbotClient(cfg.UserbotInternalURL, log)

	// Check userbot availability on startup
	userbotAvailable := userbotClient.IsAvailable(ctx)
	if userbotAvailable {
		log.Info("userbot service is available — will use for channels with userbot_status=active")
	} else {
		log.Warn("userbot service is not available — falling back to t.me parser for all channels")
	}

	log.Info("stats fetcher started", zap.Duration("interval", cfg.StatsRefreshInterval))

	// Initial run
	runStatsRefresh(ctx, channelRepo, parser, userbotClient, rdb, cfg, log)

	ticker := time.NewTicker(cfg.StatsRefreshInterval)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			runStatsRefresh(ctx, channelRepo, parser, userbotClient, rdb, cfg, log)
		case <-sigCh:
			log.Info("shutting down stats fetcher")
			cancel()
			return
		case <-ctx.Done():
			return
		}
	}
}

func runStatsRefresh(
	ctx context.Context,
	channelRepo *repositories.ChannelRepo,
	parser *statsparser.Parser,
	userbotClient *services.UserbotClient,
	rdb *redis.Client,
	cfg *config.Config,
	log *zap.Logger,
) {
	channels, err := channelRepo.GetActiveChannelsWithRecentUsers(ctx)
	if err != nil {
		log.Error("failed to get active channels", zap.Error(err))
		return
	}

	log.Info("refreshing stats", zap.Int("channels", len(channels)))

	// Check userbot availability once per refresh cycle
	userbotAvailable := userbotClient.IsAvailable(ctx)

	for _, ch := range channels {
		// Rate limit check
		rlKey := fmt.Sprintf("rl:stats:%s", ch.Username)
		if rdb.Exists(ctx, rlKey).Val() > 0 {
			continue
		}
		rdb.Set(ctx, rlKey, "1", cfg.StatsRefreshInterval)

		// Check cache
		cacheKey := fmt.Sprintf("stats:%s", ch.Username)
		if rdb.Exists(ctx, cacheKey).Val() > 0 {
			continue
		}

		var snapshot *models.ChannelStatsSnapshot

		// Try userbot first if available and channel has active userbot
		if userbotAvailable && ch.UserbotStatus == "active" {
			snapshot = tryUserbotStats(ctx, userbotClient, ch, log)
		}

		// Fallback to t.me parser
		if snapshot == nil {
			snapshot = tryParserStats(ctx, parser, ch, log)
		}

		if snapshot == nil {
			continue
		}

		// Compute growth from previous snapshots
		computeGrowth(ctx, channelRepo, snapshot, log)

		if err := channelRepo.InsertStatsSnapshot(ctx, snapshot); err != nil {
			log.Error("failed to save stats snapshot", zap.String("channel", ch.Username), zap.Error(err))
			continue
		}

		// Cache
		cacheData, _ := json.Marshal(snapshot)
		rdb.Set(ctx, cacheKey, string(cacheData), cfg.StatsRefreshInterval)

		log.Info("stats updated",
			zap.String("channel", ch.Username),
			zap.String("source", snapshot.Source),
			zap.Intp("subscribers", snapshot.Subscribers),
			zap.Intp("avg_views", snapshot.AvgViews20),
		)

		// Small delay between requests to avoid rate limiting
		time.Sleep(2 * time.Second)
	}
}

func tryUserbotStats(ctx context.Context, client *services.UserbotClient, ch models.Channel, log *zap.Logger) *models.ChannelStatsSnapshot {
	stats, err := client.GetStatsByUsername(ctx, ch.Username)
	if err != nil {
		log.Warn("userbot stats failed, will fallback to parser",
			zap.String("channel", ch.Username),
			zap.Error(err),
		)
		return nil
	}

	rawJSON, _ := json.Marshal(stats)
	snapshot := &models.ChannelStatsSnapshot{
		ChannelID:     ch.ID,
		Subscribers:   stats.Subscribers,
		VerifiedBadge: stats.Verified,
		AvgViews20:    stats.AvgViews20,
		RawJSON:       json.RawMessage(rawJSON),
		Source:        "userbot",
		MembersOnline: stats.MembersOnline,
		AdminsCount:   stats.AdminsCount,
		PostsCount:    stats.PostsCount,
		// GetBroadcastStats fields
		ViewsPerPost:                stats.ViewsPerPost,
		SharesPerPost:               stats.SharesPerPost,
		EnabledNotificationsPercent: stats.EnabledNotificationsPercent,
		ERPercent:                   stats.ERPercent,
	}
	return snapshot
}

func tryParserStats(ctx context.Context, parser *statsparser.Parser, ch models.Channel, log *zap.Logger) *models.ChannelStatsSnapshot {
	stats, err := parser.FetchAndParse(ctx, ch.Username)
	if err != nil {
		log.Warn("t.me parser stats failed",
			zap.String("channel", ch.Username),
			zap.Error(err),
		)
		return nil
	}

	rawJSON, _ := json.Marshal(stats)
	var lastPostID *int64
	if len(stats.LastPosts) > 0 {
		id := stats.LastPosts[len(stats.LastPosts)-1].MessageID
		lastPostID = &id
	}

	snapshot := &models.ChannelStatsSnapshot{
		ChannelID:     ch.ID,
		Subscribers:   stats.Subscribers,
		VerifiedBadge: stats.VerifiedBadge,
		AvgViews20:    stats.AvgViewsLast20,
		LastPostID:    lastPostID,
		RawJSON:       json.RawMessage(rawJSON),
		Source:        "tme_parser",
	}
	return snapshot
}

func computeGrowth(ctx context.Context, channelRepo *repositories.ChannelRepo, snapshot *models.ChannelStatsSnapshot, log *zap.Logger) {
	if snapshot.Subscribers == nil {
		return
	}

	// Get previous stats to compute growth
	prev, err := channelRepo.GetLatestStats(ctx, snapshot.ChannelID)
	if err != nil || prev == nil || prev.Subscribers == nil {
		return
	}

	// Simple growth: current - previous (snapshot-to-snapshot)
	growth := *snapshot.Subscribers - *prev.Subscribers
	snapshot.Growth7d = &growth
}
