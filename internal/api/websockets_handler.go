package api

import (
	"bytes"
	"fmt"
	"log"
	"net/http"

	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/isqad/melody"
	"github.com/jmoiron/sqlx"
)

const (
	wsSubscriptionSessionKey = "subscription"
	wsUserIDSessionKey       = "userId"
)

func WebsocketsHandler(
	eventsSubscriber eventbus.Subscriber,
	db *sqlx.DB,
	websocket *melody.Melody, // replace to UserRepository
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := userFromRequest(db, r)
		if err != nil {
			log.Printf("can't get user from request context: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		subscription, err := eventsSubscriber.SubscribeClient(user.ID)
		if err != nil {
			log.Printf("can't subscribe the user to signaling channel: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		sessKeys := make(map[string]interface{})
		sessKeys[wsUserIDSessionKey] = user.ID
		sessKeys[wsSubscriptionSessionKey] = subscription

		websocket.HandleRequestWithKeys(w, r, sessKeys)
	}
}

func DisconnectHandler(eventsPublisher eventbus.Publisher) func(session *melody.Session) {
	return func(session *melody.Session) {
		defer closeWsSession(session)

		log.Println("close session of user")

		userID, err := getUserIDFromSession(session)
		if err != nil {
			log.Printf("extract subscription error: %v", err)
			return
		}

		rpc := eventbus.NewCloseSessionRpc()
		if err := eventsPublisher.PublishServer(eventbus.ServerMessage{UserID: userID, Rpc: rpc}); err != nil {
			log.Printf("publish server rpc error: %v", err)
		}

		subscription, err := getUserSubscription(session)
		if err != nil {
			log.Printf("extract subscription error: %v", err)
			return
		}
		err = subscription.Close()
		if err != nil {
			log.Printf("close subscription error: %v", err)
		}
	}
}

func ConnectHandler(session *melody.Session) {
	subscription, err := getUserSubscription(session)
	if err != nil {
		log.Printf("extract subscription error: %v", err)
		log.Printf("close session: %v", session.Close())
		return
	}

	go func() {
		ch := subscription.Channel()

		for msg := range ch {
			log.Printf("send message to websockets: %s", msg.Payload)
			session.Write([]byte(msg.Payload))
		}
	}()
}

func HandleMessage(eventsPublisher eventbus.Publisher) func(s *melody.Session, msg []byte) {
	return func(s *melody.Session, msg []byte) {
		userID, err := getUserIDFromSession(s)
		if err != nil {
			log.Printf("extract userID error: %v", err)
			return
		}

		reader := bytes.NewReader(msg)
		rpc, err := eventbus.RpcFromReader(reader)
		if err != nil {
			log.Printf("rpc parse error: %v", err)
			return
		}

		if err := eventsPublisher.PublishServer(eventbus.ServerMessage{UserID: userID, Rpc: rpc}); err != nil {
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

func getUserIDFromSession(s *melody.Session) (string, error) {
	userID, ok := s.Keys[wsUserIDSessionKey]
	if !ok {
		return "", fmt.Errorf("no sub for given session: %+v", s)
	}
	id, ok := userID.(string)
	if !ok {
		return "", fmt.Errorf("can't convert userID: %+v", userID)
	}
	return id, nil
}

func closeWsSession(session *melody.Session) {
	log.Printf("close session: %v", session.Close())
}
