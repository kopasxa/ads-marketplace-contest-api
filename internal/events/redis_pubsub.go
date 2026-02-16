package events

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type RedisPublisher struct {
	client *redis.Client
	log    *zap.Logger
}

func NewRedisPublisher(client *redis.Client, log *zap.Logger) *RedisPublisher {
	return &RedisPublisher{client: client, log: log}
}

func (p *RedisPublisher) Publish(ctx context.Context, stream string, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.client.Publish(ctx, stream, string(data)).Err()
}

type RedisSubscriber struct {
	client *redis.Client
	log    *zap.Logger
}

func NewRedisSubscriber(client *redis.Client, log *zap.Logger) *RedisSubscriber {
	return &RedisSubscriber{client: client, log: log}
}

func (s *RedisSubscriber) Subscribe(ctx context.Context, stream string, handler func(Event)) error {
	pubsub := s.client.Subscribe(ctx, stream)
	ch := pubsub.Channel()

	go func() {
		defer pubsub.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var event Event
				if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
					s.log.Error("failed to unmarshal event", zap.Error(err))
					continue
				}
				handler(event)
			}
		}
	}()

	return nil
}
