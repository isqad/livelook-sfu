package sfu

import (
	"context"

	"github.com/go-redis/redis/v8"
)

type EventBusPublisher interface {
	Publish(channel string, message interface{}) error
}

type EventBus struct {
	rdb *redis.Client
}

func NewEventBus(rdb *redis.Client) *EventBus {
	return &EventBus{rdb: rdb}
}

func (e *EventBus) Publish(channel string, message interface{}) error {
	return e.rdb.Publish(context.Background(), channel, message).Err()
}
