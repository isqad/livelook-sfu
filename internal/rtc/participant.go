package rtc

import (
	"log"

	"github.com/isqad/livelook-sfu/internal/config"
	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/pion/webrtc/v3"
)

type Participant struct {
	ID        core.UserSessionID
	publisher *PCTransport

	sink eventbus.Publisher
}

func NewParticipant(
	userID core.UserSessionID,
	sink eventbus.Publisher,
	enabledCodecs []config.CodecSpec,
	rtcConf *config.WebRTCConfig,
) (*Participant, error) {
	var err error

	p := &Participant{
		ID:   userID,
		sink: sink,
	}

	p.publisher, err = NewPCTransport(TransportParams{
		EnabledCodecs: enabledCodecs,
		Config:        rtcConf,
	})
	if err != nil {
		return nil, err
	}

	p.publisher.pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		log.Printf("publisher: new ICE candidate: %v", candidate)

		if err := p.sendICECandidate(candidate); err != nil {
			log.Printf("publisher: error on send ICE candidate %v", err)
		}
	})

	return p, nil
}

func (p *Participant) sendICECandidate(candidate *webrtc.ICECandidate) error {
	if candidate == nil {
		return nil
	}

	candidateInit := candidate.ToJSON()
	rpc := eventbus.NewICECandidateRpc(&candidateInit)

	if err := p.sink.PublishClient(p.ID, rpc); err != nil {
		return err
	}

	return nil
}

func (p *Participant) HandleOffer(sdp webrtc.SessionDescription) error {
	log.Println("handle offer")

	if err := p.publisher.pc.SetRemoteDescription(sdp); err != nil {
		return err
	}

	answer, err := p.publisher.pc.CreateAnswer(nil)
	if err != nil {
		return err
	}

	log.Printf("answer: %v", answer)

	err = p.publisher.pc.SetLocalDescription(answer)
	if err != nil {
		return err
	}

	rpc := eventbus.NewSDPAnswerRpc(p.publisher.pc.LocalDescription())

	p.sink.PublishClient(p.ID, rpc)

	return nil
}
