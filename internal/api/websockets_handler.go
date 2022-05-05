package api

import (
	"bytes"
	"fmt"
	"log"
	"net/http"

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
			log.Printf("can't get user from request context: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		subscription, err := eventsSubscriber.SubscribeClient(core.UserSessionID(user.ID))
		if err != nil {
			log.Printf("can't subscribe the user to signaling channel: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		sessKeys := make(map[string]interface{})
		sessKeys[wsUserSessionKey] = user
		sessKeys[wsSubscriptionSessionKey] = subscription

		if err := websocket.HandleRequestWithKeys(w, r, sessKeys); err != nil {
			log.Printf("can't handle request: %v", err)
		}
	}
}

func DisconnectHandler(eventsPublisher eventbus.Publisher) func(session *melody.Session) {
	return func(session *melody.Session) {
		defer closeWsSession(session)

		user, err := getUserFromSession(session)
		if err != nil {
			log.Printf("extract subscription error: %v", err)
			return
		}

		rpc := rpc.NewCloseSessionRpc()
		if err := eventsPublisher.PublishServer(eventbus.ServerMessage{UserID: user.ID, Rpc: rpc}); err != nil {
			log.Printf("publish server rpc error: %v", err)
		}

		subscription, err := getUserSubscription(session)
		if err != nil {
			log.Printf("extract subscription error: %v", err)
			return
		}
		if err := subscription.Close(); err != nil {
			log.Printf("close subscription error: %v", err)
		}
	}
}

func ConnectHandler(eventsPublisher eventbus.Publisher) func(session *melody.Session) {
	return func(session *melody.Session) {
		subscription, err := getUserSubscription(session)
		if err != nil {
			log.Printf("extract subscription error: %v", err)
			return
		}

		user, err := getUserFromSession(session)
		if err != nil {
			log.Printf("extract user error: %v", err)
			return
		}

		ready := make(chan struct{})

		go func() {
			ch := subscription.Channel()

			close(ready)
			for msg := range ch {
				if err := session.Write([]byte(msg.Payload)); err != nil {
					// there's only session closed error can be
					log.Printf("ws write error: %v", err)
					return
				}
			}
		}()

		<-ready

		msg := eventbus.ServerMessage{
			UserID: user.ID,
			Rpc:    rpc.NewJoinRpc(),
		}

		if err := eventsPublisher.PublishServer(msg); err != nil {
			log.Printf("publishing failed due server error: %v", err)
			return
		}
	}
}

func HandleMessage(eventsPublisher eventbus.Publisher) func(s *melody.Session, msg []byte) {
	return func(s *melody.Session, msg []byte) {
		user, err := getUserFromSession(s)
		if err != nil {
			log.Printf("extract user error: %v", err)
			return
		}

		reader := bytes.NewReader(msg)
		rpc, err := rpc.RpcFromReader(reader)
		if err != nil {
			log.Printf("rpc parse error: %v", err)
			return
		}

		if err := eventsPublisher.PublishServer(eventbus.ServerMessage{UserID: user.ID, Rpc: rpc}); err != nil {
			log.Printf("publish server rpc error: %v", err)
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
		log.Printf("close session: %v", session.Close())
	}
}
