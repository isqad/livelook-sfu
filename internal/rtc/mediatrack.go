package rtc

import (
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/rs/zerolog/log"
)

type MediaTrackID string

type MediaTrackParams struct{}

type MediaTrack struct {
	ID MediaTrackID
	MediaTrackParams
}

func NewMediaTrack(params MediaTrackParams) *MediaTrack {
	return &MediaTrack{
		MediaTrackParams: params,
	}
}

func (t *MediaTrack) AddReceiver(track *webrtc.TrackRemote, rtpReceiver *webrtc.RTPReceiver) {
	// go t.forwardRTP()
}

func (t *MediaTrack) forwardRTP() {
	for {

		time.Sleep(10 * time.Millisecond)
	}
}

func (t *MediaTrack) Close() {
	log.Debug().Str("service", "participant").Str("ID", string(t.ID)).Msg("TODO: close exists MediaTrack")
}
