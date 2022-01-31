package eventbus

import (
	"context"

	"github.com/go-redis/redis/v8"
)

type Publisher interface {
	Publish(channel string, message interface{}) error
}

type Eventbus struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Eventbus {
	return &Eventbus{rdb: rdb}
}

func (e *Eventbus) Publish(channel string, message interface{}) error {
	return e.rdb.Publish(context.Background(), channel, message).Err()
}

func (e *Eventbus) SubscribeUser(userID string) (*UserSubscription, error) {
	ctx := context.Background()
	// Subscribe user to messages
	pubsub := e.rdb.Subscribe(ctx, "messages:"+userID)
	// Wait until subscription is created
	if _, err := pubsub.Receive(ctx); err != nil {
		return nil, err
	}

	return &UserSubscription{UserID: userID, pubsub: pubsub}, nil
}
