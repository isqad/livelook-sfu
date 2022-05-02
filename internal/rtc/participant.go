package rtc

import (
	"github.com/rs/zerolog/log"

	"github.com/isqad/livelook-sfu/internal/config"
	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/eventbus"
	"github.com/isqad/livelook-sfu/internal/telemetry"
	"github.com/pion/webrtc/v3"
)

const (
	ReliableDataChannel = "_reliable"
)

type Participant struct {
	ID         core.UserSessionID
	publisher  *PCTransport
	reliableDC *webrtc.DataChannel
	// subscriber *PCTransport

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
		log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Interface("candidate", candidate).Msg("send ICE candidate")

		if err := p.sendICECandidate(candidate); err != nil {
			log.Error().Err(err).Str("service", "participant").Str("ID", string(p.ID)).Msg("error on send ICE candidate")
		}
	})
	p.publisher.pc.OnConnectionStateChange(p.handlePrimaryStateChange)
	p.publisher.pc.OnDataChannel(p.onDataChannel)
	p.publisher.pc.OnTrack(p.onMediaTrack)

	return p, nil
}

func (p *Participant) AddICECandidate(candidate *webrtc.ICECandidateInit) error {
	return p.publisher.AddICECandidate(candidate)
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
	log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Interface("sdp", sdp).Msg("handle offer")

	if err := p.publisher.SetRemoteDescription(sdp); err != nil {
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

	rpc := eventbus.NewSDPAnswerRpc(p.publisher.pc.LocalDescription())
	if err := p.sink.PublishClient(p.ID, rpc); err != nil {
		return err
	}

	return nil
}

func (p *Participant) handlePrimaryStateChange(state webrtc.PeerConnectionState) {
	log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Interface("state", state).Msg("connection state changed")

	if state == webrtc.PeerConnectionStateConnected {
		telemetry.ServiceOperationCounter.WithLabelValues("ice_connection", "success", "").Add(1)
	} else if state == webrtc.PeerConnectionStateFailed {
		telemetry.ServiceOperationCounter.WithLabelValues("ice_connection", "error", "state_failed").Add(1)
		p.closeSignalConnection()
		p.Close()
	}
}

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

// when a new remoteTrack is created, creates a Track and adds it to room
func (p *Participant) onMediaTrack(track *webrtc.TrackRemote, rtpReceiver *webrtc.RTPReceiver) {
	log.Debug().Str("service", "participant").Str("ID", string(p.ID)).Msg("on media track")
	// if p.State() == livekit.ParticipantInfo_DISCONNECTED {
	// 	return
	// }

	// if !p.CanPublish() {
	// 	p.params.Logger.Warnw("no permission to publish mediaTrack", nil)
	// 	return
	// }

	// publishedTrack, isNewTrack := p.mediaTrackReceived(track, rtpReceiver)

	// if publishedTrack != nil {
	// 	p.params.Logger.Infow("mediaTrack published",
	// 		"kind", track.Kind().String(),
	// 		"trackID", publishedTrack.ID(),
	// 		"rid", track.RID(),
	// 		"SSRC", track.SSRC())
	// }
	// if !isNewTrack && publishedTrack != nil && p.IsReady() && p.onTrackUpdated != nil {
	// 	p.onTrackUpdated(p, publishedTrack)
	// }
}

// closes signal connection to notify client to resume/reconnect
func (p *Participant) closeSignalConnection() {
	// Need to send RPC to client
}

func (p *Participant) Close() {
	// Close peer connections without blocking participant close. If peer connections are gathering candidates
	// Close will block.
	go func() {
		p.publisher.Close()
		// p.subscriber.Close()
	}()
}
