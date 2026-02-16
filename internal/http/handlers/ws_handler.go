package handlers

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/ads-marketplace/backend/internal/auth"
	"github.com/ads-marketplace/backend/internal/config"
	"github.com/ads-marketplace/backend/internal/events"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type WSHub struct {
	cfg         *config.Config
	subscriber  events.Subscriber
	log         *zap.Logger
	mu          sync.RWMutex
	connections map[uuid.UUID][]*websocket.Conn
}

func NewWSHub(cfg *config.Config, subscriber events.Subscriber, log *zap.Logger) *WSHub {
	return &WSHub{
		cfg:         cfg,
		subscriber:  subscriber,
		log:         log,
		connections: make(map[uuid.UUID][]*websocket.Conn),
	}
}

func (h *WSHub) Start(ctx context.Context) {
	_ = h.subscriber.Subscribe(ctx, "events:deal", func(event events.Event) {
		h.broadcast(event)
	})
}

func (h *WSHub) broadcast(event events.Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, conns := range h.connections {
		for _, conn := range conns {
			_ = conn.WriteMessage(websocket.TextMessage, data)
		}
	}
}

func (h *WSHub) SendToUser(userID uuid.UUID, event events.Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, conn := range h.connections[userID] {
		_ = conn.WriteMessage(websocket.TextMessage, data)
	}
}

// WSUpgradeMiddleware checks for websocket upgrade
func WSUpgradeMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	}
}

func (h *WSHub) HandleWS(conn *websocket.Conn) {
	// Extract token from query
	tokenStr := conn.Query("token")
	if tokenStr == "" {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"missing token"}`))
		conn.Close()
		return
	}

	claims, err := auth.ParseJWT(h.cfg.JWTSecret, tokenStr)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"invalid token"}`))
		conn.Close()
		return
	}

	userID := claims.UserID

	// Register
	h.mu.Lock()
	h.connections[userID] = append(h.connections[userID], conn)
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		conns := h.connections[userID]
		for i, c := range conns {
			if c == conn {
				h.connections[userID] = append(conns[:i], conns[i+1:]...)
				break
			}
		}
		if len(h.connections[userID]) == 0 {
			delete(h.connections, userID)
		}
		h.mu.Unlock()
		conn.Close()
	}()

	// Read loop (keep alive / pings)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}
