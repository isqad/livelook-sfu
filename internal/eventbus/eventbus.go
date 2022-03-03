package eventbus

import (
	"context"

	"github.com/go-redis/redis/v8"
)

type Channel string

const (
	ClientMessages Channel = "client_messages"
	ServerMessages Channel = "server_messages"
)

func (c Channel) buildChannel(userID string) string {
	return string(c) + ":" + userID
}

type Publisher interface {
	PublishClient(userID string, rpc Rpc) error
	PublishServer(userID string, rpc Rpc) error
}

type Subscriber interface {
	SubscribeClient(userID string) (*Subscription, error)
	SubscribeServer(userID string) (*Subscription, error)
}

type Subscription struct {
	pubsub *redis.PubSub
}

func (s *Subscription) Channel() <-chan *redis.Message {
	return s.pubsub.Channel()
}

func (s *Subscription) Close() error {
	return s.pubsub.Close()
}

type Eventbus struct {
	rdb *redis.Client
}

// RedisPubSub is factory for building Eventbus based on redis pubsub
func RedisPubSub(rdb *redis.Client) *Eventbus {
	return &Eventbus{rdb: rdb}
}

func (e *Eventbus) PublishClient(userID string, rpc Rpc) error {
	return e.publish(userID, rpc, ClientMessages)
}

func (e *Eventbus) PublishServer(userID string, rpc Rpc) error {
	return e.publish(userID, rpc, ServerMessages)
}

func (e *Eventbus) SubscribeClient(userID string) (*Subscription, error) {
	return e.subscribe(userID, ClientMessages)
}

func (e *Eventbus) SubscribeServer(userID string) (*Subscription, error) {
	return e.subscribe(userID, ServerMessages)
}

func (e *Eventbus) publish(userID string, rpc Rpc, ch Channel) error {
	msg, err := rpc.ToJSON()
	if err != nil {
		return err
	}
	return e.rdb.Publish(context.Background(), ch.buildChannel(userID), msg).Err()
}

func (e *Eventbus) subscribe(userID string, ch Channel) (*Subscription, error) {
	ctx := context.Background()
	// Subscribe user to messages
	pubsub := e.rdb.Subscribe(ctx, ch.buildChannel(userID))
	// Wait until subscription is created
	if _, err := pubsub.Receive(ctx); err != nil {
		return nil, err
	}

	return &Subscription{pubsub: pubsub}, nil
}
