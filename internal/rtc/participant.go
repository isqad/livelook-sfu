package rtc

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/isqad/livelook-sfu/internal/eventbus/rpc"

	"github.com/isqad/livelook-sfu/internal/config"
	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/isqad/livelook-sfu/internal/telemetry"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

const (
	ReliableDataChannel = "_reliable"
)

type Participant struct {
	sync.Mutex

	ID              core.UserSessionID
	publisher       *PCTransport
	subscriber      *PCTransport
	reliableDC      *webrtc.DataChannel
	publishedTracks map[MediaTrackID]*MediaTrack
	sink            eventbus.Publisher
	rtcConf         *config.WebRTCConfig
}

func NewParticipant(
	userID core.UserSessionID,
	sink eventbus.Publisher,
	enabledCodecs []config.CodecSpec,
	rtcConf *config.WebRTCConfig,
) (*Participant, error) {
	var err error

	p := &Participant{
		ID:              userID,
		sink:            sink,
		rtcConf:         rtcConf,
		publishedTracks: make(map[MediaTrackID]*MediaTrack),
	}

	p.publisher, err = NewPCTransport(TransportParams{
		EnabledCodecs: enabledCodecs,
		Config:        rtcConf,
		Target:        rpc.Publisher,
	})
	if err != nil {
		return nil, err
	}

	p.publisher.pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Interface("candidate", candidate).Msg("send ICE candidate")

		if err := p.sendICECandidate(candidate, rpc.Publisher); err != nil {
			log.Error().Err(err).Str("service", "participant").Str("ID", string(p.ID)).Msg("error on send ICE candidate")
		}
	})
	p.publisher.pc.OnConnectionStateChange(p.handlePrimaryStateChange)
	p.publisher.pc.OnDataChannel(p.onDataChannel)
	p.publisher.pc.OnTrack(p.onMediaTrack)

	// p.subscriber, err = NewPCTransport(TransportParams{
	// 	EnabledCodecs: enabledCodecs,
	// 	Config:        rtcConf,
	// 	Target:        rpc.Receiver,
	// })
	// if err != nil {
	// 	return nil, err
	// }
	// p.subscriber.pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
	// 	if err := p.sendICECandidate(candidate, rpc.Receiver); err != nil {
	// 		log.Error().Err(err).Str("service", "participant").Str("ID", string(p.ID)).Msg("error on send ICE candidate to receiver")
	// 	}
	// })
	// p.subscriber.pc.OnConnectionStateChange(p.handleSecondaryStateChange)

	return p, nil
}

func (p *Participant) AddICECandidate(params rpc.ICECandidateParams) error {
	log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Str("target", string(params.Target)).Msg("add ICE candidate")

	if params.Target == rpc.Publisher {
		return p.publisher.AddICECandidate(params.ICECandidateInit)
	} else {
		return p.subscriber.AddICECandidate(params.ICECandidateInit)
	}
}

func (p *Participant) sendICECandidate(candidate *webrtc.ICECandidate, target rpc.SignalingTarget) error {
	if candidate == nil {
		return nil
	}

	candidateInit := candidate.ToJSON()
	rpc := rpc.NewICECandidateRpc(candidateInit, target)

	if err := p.sink.PublishClient(p.ID, rpc); err != nil {
		return err
	}

	return nil
}

func (p *Participant) HandleOffer(params rpc.SDPParams) error {
	log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Interface("params", params).Msg("handle offer")

	if params.Target == rpc.Publisher {
		if err := p.publisher.SetRemoteDescription(params.SessionDescription); err != nil {
			return err
		}

		answer, err := p.publisher.pc.CreateAnswer(nil)
		if err != nil {
			return err
		}

		log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Interface("sdp", answer).Msg("created answer")

		err = p.publisher.pc.SetLocalDescription(answer)
		if err != nil {
			return err
		}

		rpc := rpc.NewSDPAnswerRpc(p.publisher.pc.LocalDescription(), rpc.Publisher)
		if err := p.sink.PublishClient(p.ID, rpc); err != nil {
			return err
		}
	} else {
		log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Msg("handle offer of the receiver")
	}

	return nil
}

func (p *Participant) handlePrimaryStateChange(state webrtc.PeerConnectionState) {
	log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Str("state", state.String()).Msg("connection state changed")

	if state == webrtc.PeerConnectionStateConnected {
		telemetry.ServiceOperationCounter.WithLabelValues("ice_connection", "success", "").Add(1)
	} else if state == webrtc.PeerConnectionStateFailed {
		telemetry.ServiceOperationCounter.WithLabelValues("ice_connection", "error", "state_failed").Add(1)
		p.closeSignalConnection()
		p.Close()
	}
}

// func (p *Participant) handleSecondaryStateChange(state webrtc.PeerConnectionState) {
// 	log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Str("state", state.String()).Msg("secondary connection state changed")

// 	if state == webrtc.PeerConnectionStateConnected {
// 		telemetry.ServiceOperationCounter.WithLabelValues("ice_connection_receiver", "success", "").Add(1)
// 	} else if state == webrtc.PeerConnectionStateFailed {
// 		telemetry.ServiceOperationCounter.WithLabelValues("ice_connection_receiver", "error", "state_failed").Add(1)
// 		p.closeSignalConnection()
// 	}
// }

func (p *Participant) onDataChannel(dc *webrtc.DataChannel) {
	// if p.State() == livekit.ParticipantInfo_DISCONNECTED {
	// 	return
	// }

	switch dc.Label() {
	case ReliableDataChannel:
		p.reliableDC = dc
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			// Just ignore
		})
	// case LossyDataChannel:
	// 	p.lossyDC = dc
	// 	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
	// 		if p.CanPublishData() {
	// 			p.handleDataMessage(livekit.DataPacket_LOSSY, msg.Data)
	// 		}
	// 	})
	default:
		log.Error().Str("service", "participant").Str("ID", string(p.ID)).Str("label", dc.Label()).Msg("unsupported datachannel added")
	}
}

func (p *Participant) onMediaTrack(track *webrtc.TrackRemote, rtpReceiver *webrtc.RTPReceiver) {
	log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Msg("on media track")

	// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
	go func() {
		ticker := time.NewTicker(time.Second * 2)
		for range ticker.C {
			if rtcpErr := p.publisher.pc.WriteRTCP(
				[]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(track.SSRC())}},
			); rtcpErr != nil {
				log.Error().Err(rtcpErr).Str("service", "participant").Str("ID", string(p.ID)).Msg("")
			}
		}
	}()

	id := MediaTrackID(track.ID())
	mt, err := NewMediaTrack(id, track.Kind())
	if err != nil {
		log.Error().Err(err).Str("service", "participant").Str("ID", string(p.ID)).Msg("")
		return
	}

	p.Lock()
	p.publishedTracks[id] = mt
	p.Unlock()

	mt.ForwardRTP(track, rtpReceiver)
}

// closes signal connection to notify client to resume/reconnect
func (p *Participant) closeSignalConnection() {
	// Need to send RPC to client
}

func (p *Participant) Close() {
	log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Msg("close participant")

	p.Lock()
	defer p.Unlock()

	for _, t := range p.publishedTracks {
		t.Close()
		delete(p.publishedTracks, t.ID)
	}

	p.publishedTracks = make(map[MediaTrackID]*MediaTrack)
	// Close peer connections without blocking participant close. If peer connections are gathering candidates
	// Close will block.
	go func() {
		p.publisher.Close()
		// p.subscriber.Close()
	}()
}
