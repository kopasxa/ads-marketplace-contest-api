package events

import "context"

// Event types
const (
	EventDealStatusChanged = "deal_status_changed"
	EventBotNotification   = "bot_notification"
	EventPaymentReceived   = "payment_received"
)

type Event struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

type Publisher interface {
	Publish(ctx context.Context, stream string, event Event) error
}

type Subscriber interface {
	Subscribe(ctx context.Context, stream string, handler func(Event)) error
}
