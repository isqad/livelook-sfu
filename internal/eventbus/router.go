package eventbus

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"

	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/pion/webrtc/v3"
)

var (
	errConvertIceCandidate  = errors.New("can't convert to ice candidate")
	errConvertSession       = errors.New("can't convert to session")
	errConvertAddRemotePeer = errors.New("can't convert to add_remote_peer rpc")
	errPeerNotFound         = errors.New("can't find peer")
	errUndefinedMethod      = errors.New("undefined method")
)

// Router - Внутренний маршрутиризатор RPC-вызовов
// Его задача подписаться на события redis pub/sub и вызывать определенные колбеки сервера
type Router struct {
	EventsSubscriber Subscriber
	subscription     *Subscription

	onAddICECandidate       func(core.UserSessionID, *webrtc.ICECandidateInit) error
	onCreateOrUpdateSession func(core.UserSessionID, *core.Session) error
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
	log.Println("start router")

	go func() {
		// If the Go channel
		// is blocked full for 30 seconds the message is dropped.
		channel := router.subscription.Channel()

		for msg := range channel {
			userID, rpc, err := parseRpc(msg.Payload)
			if err != nil {
				log.Printf("router: error: %v", err)
				continue
			}

			switch rpc.GetMethod() {
			case ICECandidateMethod:
				_, ok := rpc.(*ICECandidateRpc)
				if !ok {
					log.Printf("router: error: %v", errConvertIceCandidate)
					continue
				}

				// if err := router.onAddICECandidate(userID, rpc.Params); err != nil {
				// 	log.Printf("router: error add ice candidate: %v", err)
				// }
			case CreateSessionMethod:
				rpc, ok := rpc.(*CreateSessionRpc)
				if !ok {
					log.Printf("router: error: %v", errConvertSession)
					continue
				}

				if err := router.onCreateOrUpdateSession(userID, rpc.Params); err != nil {
					log.Printf("router: error save session: %v", err)
				}
			// case CloseSessionMethod:
			// 	if err := c.closeSessionPeer(userID); err != nil {
			// 		log.Printf("commutator: error close session: %v", err)
			// 	}
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
				log.Printf("router: error: %v, %v", errUndefinedMethod, rpc.GetMethod())
			}
		}
	}()
}

func parseRpc(payload string) (core.UserSessionID, Rpc, error) {
	serverMessage := make(map[string]interface{})
	if err := json.Unmarshal([]byte(payload), &serverMessage); err != nil {
		log.Printf("router: error: %v", err)
		return "", nil, err
	}

	strUserID, ok := serverMessage["user_id"].(string)
	if !ok {
		err := errors.New("can't get user id")
		log.Printf("router: error: %v", err)
		return "", nil, err
	}

	rawRpc, err := json.Marshal(serverMessage["rpc"])
	if err != nil {
		log.Printf("router: error: %v", err)
		return "", nil, err
	}

	reader := bytes.NewReader(rawRpc)
	rpc, err := RpcFromReader(reader)
	if err != nil {
		log.Printf("router: error: %v", err)
		return "", nil, err
	}
	return core.UserSessionID(strUserID), rpc, nil
}

func (router *Router) OnAddICECandidate(callback func(core.UserSessionID, *webrtc.ICECandidateInit) error) {
	router.onAddICECandidate = callback
}

func (router *Router) OnCreateOrUpdateSession(callback func(core.UserSessionID, *core.Session) error) {
	router.onCreateOrUpdateSession = callback
}
