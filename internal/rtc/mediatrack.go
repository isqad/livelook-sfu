package rtc

import (
	"time"

	"github.com/isqad/livelook-sfu/internal/buffer"
	"github.com/pion/webrtc/v3"
	"github.com/rs/zerolog/log"
)

type MediaTrackID string

type MediaTrackParams struct {
	BufferFactory *buffer.Factory
}

type MediaTrack struct {
	ID MediaTrackID

	buffer *buffer.Buffer

	MediaTrackParams
}

func NewMediaTrack(params MediaTrackParams) *MediaTrack {
	return &MediaTrack{
		MediaTrackParams: params,
	}
}

func (t *MediaTrack) AddReceiver(track *webrtc.TrackRemote, rtpReceiver *webrtc.RTPReceiver) {
	buff, rtcpReader := t.MediaTrackParams.BufferFactory.GetBufferPair(uint32(track.SSRC()))

	if buff == nil || rtcpReader == nil {
		log.Error().Msg("could not retrive buffer pair")
		return
	}

	t.buffer = buff

	// rtcpReader.OnPacket

	buff.Bind(track.Codec().RTPCodecCapability)

	go t.forwardRTP()
}

func (t *MediaTrack) forwardRTP() {
	for {

		time.Sleep(10 * time.Millisecond)
	}
}

func (t *MediaTrack) Close() {
	log.Debug().Str("service", "participant").Str("ID", string(t.ID)).Msg("TODO: close exists MediaTrack")
}
