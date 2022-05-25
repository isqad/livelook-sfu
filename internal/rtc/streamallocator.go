package rtc

import (
	"github.com/pion/interceptor/pkg/cc"
	"github.com/rs/zerolog/log"
)

type StreamAllocator struct {
	bwe cc.BandwidthEstimator
}

func NewStreamAllocator() *StreamAllocator {
	s := &StreamAllocator{}

	return s
}

func (s *StreamAllocator) SetBandwidthEstimator(bwe cc.BandwidthEstimator) {
	if bwe != nil {
		bwe.OnTargetBitrateChange(s.onTargetBitrateChange)
	}
	s.bwe = bwe
}

// called when target bitrate changes (send side bandwidth estimation)
func (s *StreamAllocator) onTargetBitrateChange(bitrate int) {
	log.Debug().Msgf("bitrate changed: %d", bitrate)
}
