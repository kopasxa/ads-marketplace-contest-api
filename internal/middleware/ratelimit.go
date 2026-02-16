package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

func RateLimitMiddleware(rdb *redis.Client, limit int, window time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := fmt.Sprintf("rl:%s:%s", c.Path(), c.IP())

		ctx := context.Background()
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			return c.Next() // fail open
		}

		if count == 1 {
			rdb.Expire(ctx, key, window)
		}

		if count > int64(limit) {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "rate limit exceeded",
			})
		}

		return c.Next()
	}
}
