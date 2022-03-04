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
	errConvertIceCandidate = errors.New("can't convert to ice candidate")
	errConvertSession      = errors.New("can't convert to session")
	errPeerNotFound        = errors.New("can't find peer")
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

	peers map[string]*peer
	mutex sync.RWMutex

	subscription *eventbus.Subscription
}

func NewCommutator(options Options) (*Commutator, error) {
	comm := &Commutator{
		Options: options,
		peers:   make(map[string]*peer),
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
			userID, rpc, err := c.parseRpc(msg.Payload)
			if err != nil {
				log.Printf("commutator: error: %v", err)
				continue
			}

			switch rpc.GetMethod() {
			case eventbus.ICECandidateMethod:
				rpc, ok := rpc.(*eventbus.ICECandidateRpc)
				log.Printf("%v", rpc)
				if !ok {
					log.Printf("commutator: error: %v", errConvertIceCandidate)
					continue
				}

				if err := c.addICECandidate(userID, rpc.Params); err != nil {
					log.Printf("commutator: error add ice candidate: %v", err)
				}
			case eventbus.CreateSessionMethod:
				rpc, ok := rpc.(*eventbus.CreateSessionRpc)
				if !ok {
					log.Printf("commutator: error: %v", errConvertSession)
					continue
				}

				if err := c.createOrUpdateSession(userID, rpc.Params); err != nil {
					log.Printf("commutator: error save session: %v", err)
				}
			case eventbus.CloseSessionMethod:
				if err := c.closeSessionPeer(userID); err != nil {
					log.Printf("commutator: error close session: %v", err)
				}
			default:
				log.Printf("commutator: error: %v, %v", errors.New("undefined method"), rpc.GetMethod())
			}
		}
	}()
}

func (c *Commutator) createOrUpdateSession(userID string, sessionData *core.Session) error {
	peer, err := c.findOrInitPeer(userID)
	if err != nil {
		return err
	}

	session, err := c.sessionStorage.Save(sessionData)
	if err != nil {
		return err
	}

	peer.session = session

	if err := peer.serRemoteDescription(*session.Sdp); err != nil {
		return err
	}

	answer, err := peer.createAnswer()
	if err != nil {
		return err
	}

	if err := c.EventsPublisher.PublishClient(userID, answer); err != nil {
		return err
	}

	return nil
}

func (c *Commutator) addICECandidate(userID string, candidate *webrtc.ICECandidateInit) error {
	peer, err := c.findOrInitPeer(userID)
	if err != nil {
		return err
	}

	return peer.addICECandidate(candidate)
}

func (c *Commutator) findOrInitPeer(userID string) (*peer, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	var p *peer
	p, ok := c.peers[userID]
	if !ok {
		p = &peer{
			iceCandidates: []*webrtc.ICECandidateInit{},
		}
		c.peers[userID] = p
	}

	if err := p.establishPeerConnection(c.EventsPublisher); err != nil {
		log.Printf("could not to establish peer connection, delete %s from peers map", userID)
		p.close()
		delete(c.peers, userID)

		return nil, err
	}

	return p, nil
}

func (c *Commutator) closeSessionPeer(userID string) error {
	peer, err := c.findPeer(userID)
	if err != nil {
		return err
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if err := peer.close(); err != nil {
		return err
	}

	delete(c.peers, userID)

	log.Printf("commutator: session closed for user: %s", userID)

	return nil
}

func (c *Commutator) findPeer(userID string) (*peer, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	p, ok := c.peers[userID]
	if !ok {
		return nil, errPeerNotFound
	}

	return p, nil
}

func (c *Commutator) parseRpc(payload string) (userID string, rpc eventbus.Rpc, err error) {
	serverMessage := make(map[string]interface{})
	if err := json.Unmarshal([]byte(payload), &serverMessage); err != nil {
		log.Printf("commutator: error: %v", err)
		return "", nil, err
	}

	userID, ok := serverMessage["user_id"].(string)
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
