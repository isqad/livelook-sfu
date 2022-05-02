package eventbus

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/rs/zerolog/log"

	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/pion/webrtc/v3"
)

var (
	errConvertIceCandidate  = errors.New("can't convert to ice candidate")
	errConvertOffer         = errors.New("can't convert to offer")
	errConvertJoin          = errors.New("can't convert to join")
	errConvertAddRemotePeer = errors.New("can't convert to add_remote_peer rpc")
	errPeerNotFound         = errors.New("can't find peer")
	errUndefinedMethod      = errors.New("undefined method")
)

// Router - Внутренний маршрутиризатор RPC-вызовов
// Его задача подписаться на события redis pub/sub и вызывать определенные колбеки сервера
type Router struct {
	EventsSubscriber Subscriber
	subscription     *Subscription

	onAddICECandidate func(core.UserSessionID, *webrtc.ICECandidateInit) error
	onOffer           func(core.UserSessionID, *webrtc.SessionDescription) error
	onJoin            func(core.UserSessionID) error
	onCloseSession    func(core.UserSessionID) error
}

func NewRouter(sub Subscriber) (*Router, error) {
	router := &Router{
		EventsSubscriber: sub,
	}
	subscription, err := router.EventsSubscriber.SubscribeServer()
	if err != nil {
		return nil, err
	}
	router.subscription = subscription

	return router, nil
}

func (router *Router) Start() {
	log.Debug().Str("service", "router").Msg("start")

	go func() {
		// If the Go channel
		// is blocked full for 30 seconds the message is dropped.
		channel := router.subscription.Channel()

		for msg := range channel {
			userID, rpc, err := parseRpc(msg.Payload)
			if err != nil {
				log.Error().Err(err).Str("service", "router").Msg("")
				continue
			}

			switch rpc.GetMethod() {
			case ICECandidateMethod:
				msg, ok := rpc.(*ICECandidateRpc)
				if !ok {
					log.Error().Err(errConvertIceCandidate).Str("service", "router").Msg("")
					continue
				}

				if err := router.onAddICECandidate(userID, msg.Params); err != nil {
					log.Error().Err(err).Str("service", "router").Msg("router: error add ice candidate")
				}
			case JoinMethod:
				_, ok := rpc.(*JoinRpc)
				if !ok {
					log.Error().Err(errConvertJoin).Str("service", "router").Msg("")
					continue
				}

				if err := router.onJoin(userID); err != nil {
					log.Error().Err(err).Str("service", "router").Msg("error occured in onJoin")
				}
			case SDPOfferMethod:
				msg, ok := rpc.(*SDPRpc)
				if !ok {
					log.Error().Err(errConvertOffer).Str("service", "router").Msg("")
					continue
				}

				if err := router.onOffer(userID, msg.Params); err != nil {
					log.Error().Err(err).Str("service", "router").Msg("error occured in onOffer")
				}
			case CloseSessionMethod:
				if err := router.onCloseSession(userID); err != nil {
					log.Error().Err(err).Str("service", "router").Msg("close session error")
				}
			// case RenegotiationMethod:
			// 	rpc, ok := rpc.(*eventbus.RenegotiationRpc)
			// 	if !ok {
			// 		log.Printf("commutator: error: %v", errConvertSession)
			// 		continue
			// 	}

			// 	if err := c.renogotiation(userID, rpc.Params); err != nil {
			// 		log.Printf("commutator: renegotiation error: %v", err)
			// 	}
			// case StartStreamMethod:
			// 	if err := c.allowStreaming(userID); err != nil {
			// 		log.Printf("commutator: error allowing streaming: %v", err)
			// 	}
			// case StopStreamMethod:
			// 	if err := c.disallowStreaming(userID); err != nil {
			// 		log.Printf("commutator: error disallowing streaming: %v", err)
			// 	}
			// case AddRemotePeerMethod:
			// 	rpc, ok := rpc.(*AddRemotePeerRpc)
			// 	if !ok {
			// 		log.Printf("commutator: error: %v", errConvertAddRemotePeer)
			// 		continue
			// 	}

			// 	if err := c.addRemotePeer(userID, rpc.Params["user_id"]); err != nil {
			// 		log.Printf("commutator: error on add remote peer: %v", err)
			// 	}
			default:
				log.Error().Err(errUndefinedMethod).Str("rpcMethod", string(rpc.GetMethod())).Str("service", "router").Msg("")
			}
		}
	}()
}

func parseRpc(payload string) (core.UserSessionID, Rpc, error) {
	serverMessage := make(map[string]interface{})
	if err := json.Unmarshal([]byte(payload), &serverMessage); err != nil {
		log.Error().Err(err).Str("service", "router").Msg("")
		return "", nil, err
	}

	strUserID, ok := serverMessage["user_id"].(string)
	if !ok {
		err := errors.New("can't get user id")
		log.Error().Interface("serverMessage", serverMessage).Str("service", "router").Err(err).Msg("")
		return "", nil, errors.New("can't get user id")
	}

	rawRpc, err := json.Marshal(serverMessage["rpc"])
	if err != nil {
		log.Error().Err(err).Str("service", "router").Msg("")
		return "", nil, err
	}

	reader := bytes.NewReader(rawRpc)
	rpc, err := RpcFromReader(reader)
	if err != nil {
		log.Error().Err(err).Str("service", "router").Msg("")
		return "", nil, err
	}
	return core.UserSessionID(strUserID), rpc, nil
}

func (router *Router) OnAddICECandidate(callback func(core.UserSessionID, *webrtc.ICECandidateInit) error) {
	router.onAddICECandidate = callback
}

func (router *Router) OnJoin(callback func(core.UserSessionID) error) {
	router.onJoin = callback
}

func (router *Router) OnOffer(callback func(core.UserSessionID, *webrtc.SessionDescription) error) {
	router.onOffer = callback
}

func (router *Router) OnCloseSession(callback func(core.UserSessionID) error) {
	router.onCloseSession = callback
}
