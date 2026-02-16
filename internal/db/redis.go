package db

import (
	"context"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func NewRedisClient(ctx context.Context, url string, log *zap.Logger) (*redis.Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	log.Info("redis connected", zap.String("addr", opts.Addr))
	return client, nil
}
