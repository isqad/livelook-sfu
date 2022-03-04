package eventbus

import (
	"context"
	"encoding/json"

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

type ServerMessage struct {
	UserID string `json:"user_id"`
	Rpc    Rpc    `json:"rpc"`
}

type Publisher interface {
	PublishClient(userID string, rpc Rpc) error
	PublishServer(message ServerMessage) error
}

type Subscriber interface {
	SubscribeClient(userID string) (*Subscription, error)
	SubscribeServer() (*Subscription, error)
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
	msg, err := rpc.ToJSON()
	if err != nil {
		return err
	}
	return e.rdb.Publish(context.Background(), ClientMessages.buildChannel(userID), msg).Err()
}

func (e *Eventbus) PublishServer(message ServerMessage) error {
	msg, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return e.rdb.Publish(context.Background(), string(ServerMessages), msg).Err()
}

func (e *Eventbus) SubscribeClient(userID string) (*Subscription, error) {
	ctx := context.Background()
	// Subscribe user to messages
	pubsub := e.rdb.Subscribe(ctx, ClientMessages.buildChannel(userID))
	// Wait until subscription is created
	if _, err := pubsub.Receive(ctx); err != nil {
		return nil, err
	}

	return &Subscription{pubsub: pubsub}, nil
}

func (e *Eventbus) SubscribeServer() (*Subscription, error) {
	ctx := context.Background()
	// Subscribe user to messages
	pubsub := e.rdb.Subscribe(ctx, string(ServerMessages))
	// Wait until subscription is created
	if _, err := pubsub.Receive(ctx); err != nil {
		return nil, err
	}

	return &Subscription{pubsub: pubsub}, nil
}
