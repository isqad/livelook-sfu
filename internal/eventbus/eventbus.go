package eventbus

import (
	"context"
	"log"

	"github.com/go-redis/redis/v8"
)

type Publisher interface {
	Publish(userID string, message interface{}) error
}

type Subscriber interface {
	SubscribeUser(userID string) (*UserSubscription, error)
}

type Eventbus struct {
	rdb *redis.Client
}

// RedisPubSub is factory for building Eventbus based on redis pubsub
func RedisPubSub(rdb *redis.Client) *Eventbus {
	return &Eventbus{rdb: rdb}
}

func (e *Eventbus) Publish(userID string, message interface{}) error {
	log.Printf("Publish: %v, %v", userID, message)
	return e.rdb.Publish(context.Background(), userMessagesChannel(userID), message).Err()
}

func (e *Eventbus) SubscribeUser(userID string) (*UserSubscription, error) {
	ctx := context.Background()
	// Subscribe user to messages
	pubsub := e.rdb.Subscribe(ctx, userMessagesChannel(userID))
	// Wait until subscription is created
	if _, err := pubsub.Receive(ctx); err != nil {
		return nil, err
	}

	return &UserSubscription{UserID: userID, pubsub: pubsub}, nil
}

func userMessagesChannel(userID string) string {
	return "messages:" + userID
}
