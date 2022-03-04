package sfu

import (
	"errors"
	"log"

	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/pion/webrtc/v3"
)

var (
	errConnectionNotInitialized = errors.New("connection is not initialized")
)

type peer struct {
	session    *core.Session
	connection *webrtc.PeerConnection

	iceCandidates []*webrtc.ICECandidateInit
}

func (p *peer) establishPeerConnection(eventsPublisher eventbus.Publisher) error {
	peerConnection, err := webrtc.NewPeerConnection(peerConnectionConfig)
	if err != nil {
		return err
	}

	peerConnection.OnICECandidate(p.onICECandidate(eventsPublisher))

	if _, err := peerConnection.AddTransceiverFromKind(
		webrtc.RTPCodecTypeAudio,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendrecv},
	); err != nil {
		return err
	}
	if _, err := peerConnection.AddTransceiverFromKind(
		webrtc.RTPCodecTypeVideo,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendrecv},
	); err != nil {
		return err
	}

	p.connection = peerConnection

	return nil
}

func (p *peer) serRemoteDescription(sdp webrtc.SessionDescription) error {
	if p.connection == nil {
		return errConnectionNotInitialized
	}

	return p.connection.SetRemoteDescription(sdp)
}

func (p *peer) addICECandidate(candidate *webrtc.ICECandidateInit) error {
	if p.connection == nil {
		return errConnectionNotInitialized
	}

	p.iceCandidates = append(p.iceCandidates, candidate)

	if p.connection.CurrentRemoteDescription() == nil {
		return nil
	}

	defer p.clearCandidates()

	for _, c := range p.iceCandidates {
		iceCandidate := *c

		if err := p.connection.AddICECandidate(iceCandidate); err != nil {
			return err
		}
	}

	return nil
}

func (p *peer) clearCandidates() {
	p.iceCandidates = []*webrtc.ICECandidateInit{}
}

func (p *peer) createAnswer() (*eventbus.SDPRpc, error) {
	if p.connection == nil {
		return nil, errConnectionNotInitialized
	}

	answer, err := p.connection.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	err = p.connection.SetLocalDescription(answer)
	if err != nil {
		return nil, err
	}

	rpc := eventbus.NewSDPAnswerRpc(p.connection.LocalDescription())

	return rpc, nil
}

func (p *peer) close() error {
	if p.connection == nil {
		return errConnectionNotInitialized
	}

	return p.connection.Close()
}

func (p *peer) onICECandidate(eventsPublisher eventbus.Publisher) func(*webrtc.ICECandidate) {
	return func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			log.Println("No more ICE candidates")
			return
		}

		candidateInit := candidate.ToJSON()
		rpc := eventbus.NewICECandidateRpc(&candidateInit)

		if err := eventsPublisher.PublishClient(p.session.UserID, rpc); err != nil {
			log.Printf("onICECandidate: error %v", err)
			return
		}
	}
}
