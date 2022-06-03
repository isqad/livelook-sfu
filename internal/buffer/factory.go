package buffer

import (
	"io"
	"sync"

	"github.com/pion/transport/packetio"
)

type Factory struct {
	sync.RWMutex
	videoPool   *sync.Pool
	audioPool   *sync.Pool
	rtpBuffers  map[uint32]*Buffer
	rtcpReaders map[uint32]*RTCPReader
}

func NewBufferFactory(trackingPackets int) *Factory {
	return &Factory{
		videoPool: &sync.Pool{
			New: func() interface{} {
				b := make([]byte, trackingPackets*maxPktSize)
				return &b
			},
		},
		audioPool: &sync.Pool{
			New: func() interface{} {
				b := make([]byte, maxPktSize*25)
				return &b
			},
		},
		rtpBuffers:  make(map[uint32]*Buffer),
		rtcpReaders: make(map[uint32]*RTCPReader),
	}
}

// GetOrNew соответствует интерфейсу func(packetType packetio.BufferPacketType, ssrc uint32) io.ReadWriteCloser
// ссылка на эту функцию будет передана в webrtc.SettingEngine:
// https://github.com/pion/webrtc/blob/dc31439c934d9851b2d1e51515e699b946b2598d/settingengine.go#L66
// а далее данная функция будет использоваться в srtp и srtcp для создания буферов пакетов RTP и RTPC соотв.:
// https://github.com/pion/srtp/blob/3c34651fa0c6de900bdc91062e7ccb5992409643/stream_srtp.go#L53
// https://github.com/pion/srtp/blob/82008b58b1e7be7a0cb834270caafacc7ba53509/stream_srtcp.go#L117
func (f *Factory) GetOrNew(packetType packetio.BufferPacketType, ssrc uint32) io.ReadWriteCloser {
	f.Lock()
	defer f.Unlock()

	switch packetType {
	case packetio.RTCPBufferPacket:
		if reader, ok := f.rtcpReaders[ssrc]; ok {
			return reader
		}

		reader := NewRTCPReader(ssrc)

		f.rtcpReaders[ssrc] = reader

		return reader
	case packetio.RTPBufferPacket:
		if reader, ok := f.rtpBuffers[ssrc]; ok {
			return reader
		}

		reader := NewBuffer(ssrc, f.videoPool, f.audioPool)

		f.rtpBuffers[ssrc] = reader

		return reader
	}

	return nil
}

func (f *Factory) GetBufferPair(ssrc uint32) (*Buffer, *RTCPReader) {
	f.RLock()
	defer f.RUnlock()

	return f.rtpBuffers[ssrc], f.rtcpReaders[ssrc]
}
