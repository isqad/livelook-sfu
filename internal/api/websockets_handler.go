package api

import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/isqad/livelook-sfu/internal/eventbus/rpc"
	"github.com/isqad/melody"
)

const (
	wsSubscriptionSessionKey = "subscription"
	wsUserSessionKey         = "current_user"
)

func WebsocketsHandler(
	eventsSubscriber eventbus.Subscriber,
	websocket *melody.Melody, // replace to UserRepository
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := userFromRequest(r)
		if err != nil {
			log.Error().Err(err).Str("service", "websockets").Msg("can't get the user from request context")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		subscription, err := eventsSubscriber.SubscribeClient(core.UserSessionID(user.ID))
		if err != nil {
			log.Error().Err(err).Str("service", "websockets").Msg("can't subscribe the user to signaling channel")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		sessKeys := make(map[string]interface{})
		sessKeys[wsUserSessionKey] = user
		sessKeys[wsSubscriptionSessionKey] = subscription

		if err := websocket.HandleRequestWithKeys(w, r, sessKeys); err != nil {
			log.Error().Err(err).Str("service", "websockets").Msg("can't handle request")
		}
	}
}

func DisconnectHandler(eventsPublisher eventbus.Publisher) func(session *melody.Session) {
	return func(session *melody.Session) {
		defer closeWsSession(session)

		user, err := getUserFromSession(session)
		if err != nil {
			log.Error().Err(err).Str("service", "websockets").Msg("extract user from session")
			return
		}

		subscription, err := getUserSubscription(session)
		if err != nil {
			log.Error().Err(err).Str("service", "websockets").Msg("extract subscription")
			return
		}
		if err := subscription.Close(); err != nil {
			log.Error().Err(err).Str("service", "websockets").Msg("close subscription")
			return
		}

		message, err := rpc.NewCloseSessionRpc().ToJSON()
		if err != nil {
			log.Error().Err(err).Str("service", "websockets").Msg("publish rpc")
			return
		}
		if err := eventsPublisher.PublishServer(eventbus.ServerMessage{UserID: user.ID, Message: message}); err != nil {
			log.Error().Err(err).Str("service", "websockets").Msg("publish rpc")
		}
	}
}

func ConnectHandler(eventsPublisher eventbus.Publisher) func(session *melody.Session) {
	return func(session *melody.Session) {
		subscription, err := getUserSubscription(session)
		if err != nil {
			log.Error().Err(err).Str("service", "websockets").Msg("extract subscription")
			closeWsSession(session)
			return
		}

		user, err := getUserFromSession(session)
		if err != nil {
			log.Error().Err(err).Str("service", "websockets").Msg("extract user error")
			subscription.Close()
			closeWsSession(session)
			return
		}

		ready := make(chan struct{})

		go func() {
			ch := subscription.Channel()

			close(ready)
			for msg := range ch {
				payload := msg.Payload

				if err := session.Write([]byte(payload)); err != nil {
					// there's only session closed error can be
					log.Error().Err(err).Str("service", "websockets").Str("userID", string(user.ID))
					return
				}
			}
		}()

		<-ready

		message, err := rpc.NewJoinRpc().ToJSON()
		if err != nil {
			log.Error().Err(err).Str("service", "websockets").Str("userID", string(user.ID))
			subscription.Close()
			closeWsSession(session)

			return
		}
		msg := eventbus.ServerMessage{
			UserID:  user.ID,
			Message: message,
		}

		if err := eventsPublisher.PublishServer(msg); err != nil {
			log.Error().Err(err).Str("service", "websockets").Str("userID", string(user.ID))
			subscription.Close()
			closeWsSession(session)
			return
		}
	}
}

func HandleMessage(eventsPublisher eventbus.Publisher) func(s *melody.Session, msg []byte) {
	return func(s *melody.Session, msg []byte) {
		user, err := getUserFromSession(s)
		if err != nil {
			log.Error().Err(err).Str("service", "websockets")
			closeWsSession(s)
			return
		}

		if err := eventsPublisher.PublishServer(eventbus.ServerMessage{UserID: user.ID, Message: msg}); err != nil {
			log.Error().Err(err).Str("service", "websockets").Str("userID", string(user.ID))
			closeWsSession(s)
		}
	}
}

func getUserSubscription(s *melody.Session) (*eventbus.Subscription, error) {
	userSub, ok := s.Keys[wsSubscriptionSessionKey]
	if !ok {
		return nil, fmt.Errorf("no sub for given session: %+v", s)
	}
	subscription, ok := userSub.(*eventbus.Subscription)
	if !ok {
		return nil, fmt.Errorf("can't convert userSub: %+v", userSub)
	}
	return subscription, nil
}

func getUserFromSession(s *melody.Session) (*core.User, error) {
	data, ok := s.Keys[wsUserSessionKey]
	if !ok {
		return nil, fmt.Errorf("no sub for given session: %+v", s)
	}

	user, ok := data.(*core.User)
	if !ok {
		return nil, fmt.Errorf("can't convert to user: %+v", user)
	}
	return user, nil
}

func closeWsSession(session *melody.Session) {
	if !session.IsClosed() {
		log.Error().Err(session.Close()).Str("service", "websockets").Msg("close session")
	}
}
