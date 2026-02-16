package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const CtxRequestID = "request_id"

func RequestIDMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		reqID := c.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}
		c.Locals(CtxRequestID, reqID)
		c.Set("X-Request-ID", reqID)
		return c.Next()
	}
}
