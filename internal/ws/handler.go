package ws

import (
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/isqad/melody"
)

func WsHandler(websocket *melody.Melody) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info().Msg("called WsHandler")
		// user, err := userFromRequest(r)
		// if err != nil {
		// 	log.Error().Err(err).Str("service", "websockets").Msg("can't get the user from request context")
		// 	w.WriteHeader(http.StatusBadRequest)
		// 	return
		// }

		// subscription, err := eventsSubscriber.SubscribeClient(core.UserSessionID(user.ID))
		// if err != nil {
		// 	log.Error().Err(err).Str("service", "websockets").Msg("can't subscribe the user to signaling channel")
		// 	w.WriteHeader(http.StatusInternalServerError)
		// 	return
		// }

		sessions := make(map[string]interface{})
		// sessKeys[wsUserSessionKey] = user
		// sessKeys[wsSubscriptionSessionKey] = subscription

		if err := websocket.HandleRequestWithKeys(w, r, sessions); err != nil {
			log.Error().Err(err).Str("service", "websockets").Msg("can't handle request")
		}
	}
}

func DisconnectHandler() func(session *melody.Session) {
	return func(session *melody.Session) {
		log.Info().Msg("called DisconnectHandler")
		// defer closeWsSession(session)

		// user, err := getUserFromSession(session)
		// if err != nil {
		// 	log.Error().Err(err).Str("service", "websockets").Msg("extract user from session")
		// 	return
		// }

		// subscription, err := getUserSubscription(session)
		// if err != nil {
		// 	log.Error().Err(err).Str("service", "websockets").Msg("extract subscription")
		// 	return
		// }
		// if err := subscription.Close(); err != nil {
		// 	log.Error().Err(err).Str("service", "websockets").Msg("close subscription")
		// 	return
		// }

		// message, err := rpc.NewCloseSessionRpc().ToJSON()
		// if err != nil {
		// 	log.Error().Err(err).Str("service", "websockets").Msg("publish rpc")
		// 	return
		// }
		// if err := eventsPublisher.PublishServer(eventbus.ServerMessage{UserID: user.ID, Message: message}); err != nil {
		// 	log.Error().Err(err).Str("service", "websockets").Msg("publish rpc")
		// }
	}
}

func ConnectHandler() func(session *melody.Session) {
	return func(session *melody.Session) {
		log.Info().Msg("called ConnectHandler")
		// subscription, err := getUserSubscription(session)
		// if err != nil {
		// 	log.Error().Err(err).Str("service", "websockets").Msg("extract subscription")
		// 	closeWsSession(session)
		// 	return
		// }

		// user, err := getUserFromSession(session)
		// if err != nil {
		// 	log.Error().Err(err).Str("service", "websockets").Msg("extract user error")
		// 	subscription.Close()
		// 	closeWsSession(session)
		// 	return
		// }

		// ready := make(chan struct{})

		// go func() {
		// 	ch := subscription.Channel()

		// 	close(ready)
		// 	for msg := range ch {
		// 		payload := msg.Payload

		// 		if err := session.Write([]byte(payload)); err != nil {
		// 			// there's only session closed error can be
		// 			log.Error().Err(err).Str("service", "websockets").Str("userID", string(user.ID))
		// 			return
		// 		}
		// 	}
		// }()

		// <-ready

		// message, err := rpc.NewJoinRpc().ToJSON()
		// if err != nil {
		// 	log.Error().Err(err).Str("service", "websockets").Str("userID", string(user.ID))
		// 	subscription.Close()
		// 	closeWsSession(session)

		// 	return
		// }
		// msg := eventbus.ServerMessage{
		// 	UserID:  user.ID,
		// 	Message: message,
		// }

		// if err := eventsPublisher.PublishServer(msg); err != nil {
		// 	log.Error().Err(err).Str("service", "websockets").Str("userID", string(user.ID))
		// 	subscription.Close()
		// 	closeWsSession(session)
		// 	return
		// }
	}
}

func HandleMessage() func(s *melody.Session, msg []byte) {
	return func(s *melody.Session, msg []byte) {
		log.Info().Msg("called HandleMessage")
		// user, err := getUserFromSession(s)
		// if err != nil {
		// 	log.Error().Err(err).Str("service", "websockets")
		// 	closeWsSession(s)
		// 	return
		// }

		// if err := eventsPublisher.PublishServer(eventbus.ServerMessage{UserID: user.ID, Message: msg}); err != nil {
		// 	log.Error().Err(err).Str("service", "websockets").Str("userID", string(user.ID))
		// 	closeWsSession(s)
		// }
	}
}
