package buffer

import (
	"io"
	"strings"
	"sync"
	"time"

	"github.com/gammazero/deque"
	"github.com/pion/webrtc/v3"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"
)

const (
	maxPktSize = 1500
)

type pendingPacket struct {
	arrivalTime int64
	packet      []byte
}

// Buffer служит для временного хранения RTP и RTCP пакетов перед их обработкой
type Buffer struct {
	sync.RWMutex
	bucket     *Bucket
	mediaSSRC  uint32
	extPackets deque.Deque
	videoPool  *sync.Pool
	audioPool  *sync.Pool
	mime       string

	pendingPackets []pendingPacket

	closed    atomic.Bool
	closeOnce sync.Once

	bound bool
}

func NewBuffer(ssrc uint32, vp, ap *sync.Pool) *Buffer {
	b := &Buffer{
		mediaSSRC: ssrc,
		videoPool: vp,
		audioPool: ap,
	}
	b.extPackets.SetMinCapacity(7)

	return b
}

func (b *Buffer) Close() error {
	b.Lock()
	defer b.Unlock()

	log.Debug().Str("service", "SRTP buffer").Msg("close buffer")

	b.closeOnce.Do(func() {
		b.closed.Store(true)
	})

	return nil
}

func (b *Buffer) Read(p []byte) (n int, err error) {
	// log.Debug().Str("service", "SRTP buffer").Msg("read packet")

	return 0, nil
}

// Write вызывается в webrtc/pion
// Помним, что пакеты могут "прилетать" в неверном порядке
func (b *Buffer) Write(p []byte) (n int, err error) {
	b.Lock()
	defer b.Unlock()

	if b.closed.Load() {
		err = io.EOF
		return
	}

	if !b.bound {
		packet := make([]byte, len(p))
		copy(packet, p)

		b.pendingPackets = append(b.pendingPackets, pendingPacket{
			packet:      packet,
			arrivalTime: time.Now().UnixNano(),
		})
		return
	}

	b.calc(p, time.Now().UnixNano())

	return
}

func (b *Buffer) Bind(codec webrtc.RTPCodecCapability) {
	b.Lock()
	defer b.Unlock()

	if b.bound {
		log.Error().Str("service", "SRTP buffer").Msg("already bound!")
		return
	}

	for _, pkt := range b.pendingPackets {
		b.calc(pkt.packet, pkt.arrivalTime)
	}

	b.mime = strings.ToLower(codec.MimeType)
	b.pendingPackets = nil
	b.bound = true
}

func (b *Buffer) calc(pkt []byte, arrivalTime int64) {
	log.Debug().Str("service", "SRTP buffer").Int64("arrivalTime", arrivalTime).Msg("calculate packet")
}
