package eventbus

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/rs/zerolog/log"

	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/eventbus/rpc"
)

var (
	errConvertIceCandidate = errors.New("can't convert to ice candidate")
	errConvertOffer        = errors.New("can't convert to offer")
	errConvertJoin         = errors.New("can't convert to join")
	errUndefinedMethod     = errors.New("undefined method")
)

// Router - Внутренний маршрутиризатор RPC-вызовов
// Его задача подписаться на события redis pub/sub и вызывать определенные колбеки сервера
type Router struct {
	EventsSubscriber Subscriber
	subscription     RedisBus

	stop    chan struct{}
	stopped chan struct{}

	onAddICECandidate func(core.UserSessionID, rpc.ICECandidateParams) error
	onOffer           func(core.UserSessionID, rpc.SDPParams) error
	onJoin            func(core.UserSessionID) error
	onCloseSession    func(core.UserSessionID) error
	onPublishStream   func(core.UserSessionID) error
	onStopStream      func(core.UserSessionID) error
}

func NewRouter(sub Subscriber) (*Router, error) {
	router := &Router{
		EventsSubscriber: sub,
		stop:             make(chan struct{}),
		stopped:          make(chan struct{}),
	}
	subscription, err := router.EventsSubscriber.SubscribeServer()
	if err != nil {
		return nil, err
	}
	router.subscription = subscription

	return router, nil
}

func (router *Router) Start() chan struct{} {
	started := make(chan struct{})

	go func() {
		log.Debug().Str("service", "router").Msg("started")

		// If the Go channel
		// is blocked full for 30 seconds the message is dropped.
		channel := router.subscription.Channel()

		close(started)
		for {
			select {
			case msg := <-channel:
				payload := []byte(msg.Payload)

				userID, r, err := parseRpc(payload)
				if err != nil {
					log.Error().Err(err).Str("service", "router").Interface("payload", payload).Msg("can't parse RPC")
					continue
				}

				switch r.GetMethod() {
				case rpc.ICECandidateMethod:
					msg, ok := r.(*rpc.ICECandidateRpc)
					if !ok {
						log.Error().Err(errConvertIceCandidate).Str("service", "router").Msg("")
						continue
					}

					if err := router.onAddICECandidate(userID, msg.Params); err != nil {
						log.Error().Err(err).Str("service", "router").Msg("router: error add ice candidate")
					}
				case rpc.JoinMethod:
					_, ok := r.(*rpc.JoinRpc)
					if !ok {
						log.Error().Err(errConvertJoin).Str("service", "router").Msg("")
						continue
					}

					if err := router.onJoin(userID); err != nil {
						log.Error().Err(err).Str("service", "router").Msg("error occured in onJoin")
					}
				case rpc.SDPOfferMethod:
					msg, ok := r.(*rpc.SDPRpc)
					if !ok {
						log.Error().Err(errConvertOffer).Str("service", "router").Msg("")
						continue
					}

					if err := router.onOffer(userID, msg.Params); err != nil {
						log.Error().Err(err).Str("service", "router").Msg("error occured in onOffer")
					}
				case rpc.CloseSessionMethod:
					if err := router.onCloseSession(userID); err != nil {
						log.Error().Err(err).Str("service", "router").Msg("close session error")
					}
				case rpc.PublishStreamMethod:
					if err := router.onPublishStream(userID); err != nil {
						log.Error().Err(err).Str("service", "router").Msg("publish stream error")
					}
				case rpc.PublishStreamStopMethod:
					if err := router.onStopStream(userID); err != nil {
						log.Error().Err(err).Str("service", "router").Msg("stop stream error")
					}
				default:
					log.Error().Err(errUndefinedMethod).Str("rpcMethod", string(r.GetMethod())).Str("service", "router").Msg("")
				}
			case <-router.stop:
				if err := router.subscription.Close(); err != nil {
					log.Error().Err(err).Str("service", "router").Msg("close subscription errored")
				}

				close(router.stopped)

				log.Debug().Str("service", "router").Msg("stopped")
				return
			}
		}
	}()

	return started
}

func (router *Router) Stop() chan struct{} {
	router.stop <- struct{}{}

	return router.stopped
}

func parseRpc(payload []byte) (core.UserSessionID, rpc.Rpc, error) {
	serverMessage := &ServerMessage{}

	if err := json.Unmarshal(payload, serverMessage); err != nil {
		return "", nil, err
	}

	userID := serverMessage.UserID
	rawRpc := serverMessage.Message

	reader := bytes.NewReader(rawRpc)
	rpc, err := rpc.RpcFromReader(reader)
	if err != nil {
		return "", nil, err
	}

	return core.UserSessionID(userID), rpc, nil
}

func (router *Router) OnAddICECandidate(callback func(core.UserSessionID, rpc.ICECandidateParams) error) {
	router.onAddICECandidate = callback
}

func (router *Router) OnJoin(callback func(core.UserSessionID) error) {
	router.onJoin = callback
}

func (router *Router) OnOffer(callback func(core.UserSessionID, rpc.SDPParams) error) {
	router.onOffer = callback
}

func (router *Router) OnCloseSession(callback func(core.UserSessionID) error) {
	router.onCloseSession = callback
}

func (router *Router) OnPublishStream(callback func(core.UserSessionID) error) {
	router.onPublishStream = callback
}

func (router *Router) OnStopStream(callback func(core.UserSessionID) error) {
	router.onStopStream = callback
}
