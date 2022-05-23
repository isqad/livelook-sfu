package eventbus

import (
	"github.com/go-redis/redis/v8"
	"github.com/isqad/livelook-sfu/internal/core"
)

type MockSubscriber struct {
	ServerSubscribed bool
	ClientSubscribed bool
	Bus              RedisBus
}

func NewMockSubscriber(bus RedisBus) *MockSubscriber {
	return &MockSubscriber{
		Bus: bus,
	}
}

func (s *MockSubscriber) SubscribeServer() (RedisBus, error) {
	s.ServerSubscribed = true

	return s.Bus, nil
}

func (s *MockSubscriber) SubscribeClient(userID core.UserSessionID) (RedisBus, error) {
	return s.Bus, nil
}

type MockBus struct {
	Messages chan *redis.Message
}

func NewMockBus() *MockBus {
	return &MockBus{Messages: make(chan *redis.Message)}
}

func (b *MockBus) Channel() <-chan *redis.Message {
	return b.Messages
}

func (b *MockBus) Close() error {
	close(b.Messages)
	return nil
}
