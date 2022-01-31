package eventbus

import (
	"github.com/go-redis/redis/v8"
)

// UserSub is user subscription
type UserSubscription struct {
	UserID string
	pubsub *redis.PubSub
}

func (s *UserSubscription) Channel() <-chan *redis.Message {
	return s.pubsub.Channel()
}

func (s *UserSubscription) Close() error {
	return s.pubsub.Close()
}
