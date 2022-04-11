package sfu

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"sync"

	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/jmoiron/sqlx"
	"github.com/pion/webrtc/v3"
)

var (
	errConvertIceCandidate  = errors.New("can't convert to ice candidate")
	errConvertSession       = errors.New("can't convert to session")
	errConvertAddRemotePeer = errors.New("can't convert to add_remote_peer rpc")
	errPeerNotFound         = errors.New("can't find peer")
	errUndefinedMethod      = errors.New("undefined method")
)

// Options is options of the sfu
type Options struct {
	DB               *sqlx.DB
	EventsPublisher  eventbus.Publisher
	EventsSubscriber eventbus.Subscriber
}

type Commutator struct {
	Options
	sessionStorage core.SessionsDBStorer

	peers     map[core.UserSessionID]*peer
	peersLock sync.RWMutex

	subscription *eventbus.Subscription
}

func NewCommutator(options Options) (*Commutator, error) {
	comm := &Commutator{
		Options: options,
		peers:   make(map[core.UserSessionID]*peer),
	}
	subscription, err := comm.EventsSubscriber.SubscribeServer()
	if err != nil {
		return nil, err
	}

	comm.subscription = subscription
	comm.sessionStorage = core.NewSessionsRepository(options.DB)

	return comm, nil
}

func (c *Commutator) Start() {
	go func() {
		// If the Go channel
		// is blocked full for 30 seconds the message is dropped.
		channel := c.subscription.Channel()

		for msg := range channel {
			_, rpc, err := c.parseRpc(msg.Payload)
			if err != nil {
				log.Printf("commutator: error: %v", err)
				continue
			}

			switch rpc.GetMethod() {
			// case eventbus.ICECandidateMethod:
			// 	rpc, ok := rpc.(*eventbus.ICECandidateRpc)
			// 	if !ok {
			// 		log.Printf("commutator: error: %v", errConvertIceCandidate)
			// 		continue
			// 	}

			// 	if err := c.addICECandidate(userID, rpc.Params); err != nil {
			// 		log.Printf("commutator: error add ice candidate: %v", err)
			// 	}
			// case eventbus.CreateSessionMethod:
			// 	rpc, ok := rpc.(*eventbus.CreateSessionRpc)
			// 	if !ok {
			// 		log.Printf("commutator: error: %v", errConvertSession)
			// 		continue
			// 	}

			// 	if err := c.createOrUpdateSession(userID, rpc.Params); err != nil {
			// 		log.Printf("commutator: error save session: %v", err)
			// 	}
			// case eventbus.CloseSessionMethod:
			// 	if err := c.closeSessionPeer(userID); err != nil {
			// 		log.Printf("commutator: error close session: %v", err)
			// 	}
			// case eventbus.RenegotiationMethod:
			// 	rpc, ok := rpc.(*eventbus.RenegotiationRpc)
			// 	if !ok {
			// 		log.Printf("commutator: error: %v", errConvertSession)
			// 		continue
			// 	}

			// 	if err := c.renogotiation(userID, rpc.Params); err != nil {
			// 		log.Printf("commutator: renegotiation error: %v", err)
			// 	}
			// case eventbus.StartStreamMethod:
			// 	if err := c.allowStreaming(userID); err != nil {
			// 		log.Printf("commutator: error allowing streaming: %v", err)
			// 	}
			// case eventbus.StopStreamMethod:
			// 	if err := c.disallowStreaming(userID); err != nil {
			// 		log.Printf("commutator: error disallowing streaming: %v", err)
			// 	}
			// case eventbus.AddRemotePeerMethod:
			// 	rpc, ok := rpc.(*eventbus.AddRemotePeerRpc)
			// 	if !ok {
			// 		log.Printf("commutator: error: %v", errConvertAddRemotePeer)
			// 		continue
			// 	}

			// 	if err := c.addRemotePeer(userID, rpc.Params["user_id"]); err != nil {
			// 		log.Printf("commutator: error on add remote peer: %v", err)
			// 	}
			default:
				log.Printf("commutator: error: %v, %v", errUndefinedMethod, rpc.GetMethod())
			}
		}
	}()
}

func (c *Commutator) addRemotePeer(userID core.UserSessionID, remoteUserID core.UserSessionID) error {
	peer, err := c.findPeer(userID)
	if err != nil {
		return err
	}
	remotePeer, err := c.findPeer(remoteUserID)
	if err != nil {
		return err
	}

	return remotePeer.addRemotePeer(peer)
}

func (c *Commutator) createOrUpdateSession(userID core.UserSessionID, sessionData *core.Session) error {
	peer, err := c.findOrInitPeer(userID)
	if err != nil {
		return err
	}

	// TODO: move to API
	session, err := c.sessionStorage.Save(sessionData)
	if err != nil {
		return err
	}

	if err := peer.setRemoteDescription(*session.Sdp); err != nil {
		return err
	}

	answer, err := peer.createAnswer()
	if err != nil {
		return err
	}

	if err := c.EventsPublisher.PublishClient(core.UserSessionID(userID), answer); err != nil {
		return err
	}

	return nil
}

func (c *Commutator) addICECandidate(userID core.UserSessionID, candidate *webrtc.ICECandidateInit) error {
	peer, err := c.findOrInitPeer(userID)
	if err != nil {
		return err
	}

	return peer.addICECandidate(candidate)
}

func (c *Commutator) allowStreaming(userID core.UserSessionID) error {
	peer, err := c.findOrInitPeer(userID)
	if err != nil {
		return err
	}

	peer.streamingAllowed = true

	return nil
}

func (c *Commutator) disallowStreaming(userID core.UserSessionID) error {
	peer, err := c.findOrInitPeer(userID)
	if err != nil {
		return err
	}

	peer.streamingAllowed = false

	return nil
}

func (c *Commutator) renogotiation(userID core.UserSessionID, sdp *webrtc.SessionDescription) error {
	log.Println("commutator: renegotiation")

	peer, err := c.findOrInitPeer(userID)
	if err != nil {
		return err
	}

	if err := peer.setRemoteDescription(*sdp); err != nil {
		return err
	}

	answerRpc, err := peer.createAnswer()
	if err != nil {
		return err
	}

	return c.EventsPublisher.PublishClient(userID, answerRpc)
}

func (c *Commutator) findOrInitPeer(userID core.UserSessionID) (*peer, error) {
	p, err := c.findPeer(userID)
	if err != nil && err != errPeerNotFound {
		return nil, err
	}

	if err == errPeerNotFound {
		p = &peer{
			streamingAllowed: false,
			iceCandidates:    []*webrtc.ICECandidateInit{},
			remotePeers:      []*peer{},
			closeChan:        make(chan struct{}),
			closed:           make(chan struct{}),
			stopTracks:       make(chan struct{}),
			stopped:          make(chan struct{}),
			stopParentTracks: make(chan struct{}, 1),
			userID:           userID,
		}
		go p.listenAndAccept()

		c.peersLock.Lock()
		defer c.peersLock.Unlock()

		c.peers[userID] = p

		if err := p.establishPeerConnection(c.EventsPublisher); err != nil {
			log.Printf("could not to establish peer connection, delete %s from peers map", userID)
			<-p.close()
			delete(c.peers, userID)

			return nil, err
		}
	}

	return p, nil
}

func (c *Commutator) findPeer(userID core.UserSessionID) (*peer, error) {
	c.peersLock.RLock()
	defer c.peersLock.RUnlock()

	p, ok := c.peers[userID]
	if !ok {
		return nil, errPeerNotFound
	}

	return p, nil
}

func (c *Commutator) closeSessionPeer(userID core.UserSessionID) error {
	peer, err := c.findPeer(userID)
	if err != nil {
		return err
	}
	// Wait until closed
	<-peer.close()

	c.peersLock.Lock()
	delete(c.peers, userID)
	c.peersLock.Unlock()

	// if err := c.sessionStorage.SetOffline(userID); err != nil {
	// 	return err
	// }

	log.Printf("commutator: session closed for user: %s", userID)

	return nil
}

func (c *Commutator) parseRpc(payload string) (userID core.UserSessionID, rpc eventbus.Rpc, err error) {
	serverMessage := make(map[string]interface{})
	if err := json.Unmarshal([]byte(payload), &serverMessage); err != nil {
		log.Printf("commutator: error: %v", err)
		return "", nil, err
	}

	userID, ok := serverMessage["user_id"].(core.UserSessionID)
	if !ok {
		err := errors.New("can't get user id")
		log.Printf("commutator: error: %v", err)
		return "", nil, err
	}

	rawRpc, err := json.Marshal(serverMessage["rpc"])
	if err != nil {
		log.Printf("commutator: error: %v", err)
		return "", nil, err
	}

	reader := bytes.NewReader(rawRpc)
	rpc, err = eventbus.RpcFromReader(reader)
	if err != nil {
		log.Printf("commutator: error: %v", err)
		return "", nil, err
	}
	return userID, rpc, nil
}
