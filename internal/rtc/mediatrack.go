package rtc

import (
	"errors"
	"fmt"
	"net"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
	"github.com/rs/zerolog/log"
)

type udpConn struct {
	conn        *net.UDPConn
	port        int
	payloadType uint8
}

type MediaTrackID string

// TODO
// ffmpeg -protocol_whitelist file,udp,rtp -i rtp-forwarder.sdp -c:v libx264 -preset veryfast -crf 18 -b:v 3000k -maxrate 3000k -bufsize 6000k -pix_fmt yuv420p -g 30 -flags low_delay -hls_time 2 -hls_flags 'delete_segments' -hls_list_size 5 stream.m3u8
type MediaTrack struct {
	ID          MediaTrackID
	laddr       *net.UDPAddr
	forwardConn *udpConn
}

func NewMediaTrack(trackID MediaTrackID, mime webrtc.RTPCodecType) (*MediaTrack, error) {
	var err error

	mt := &MediaTrack{
		ID: trackID,
	}

	// Create a local addr
	if mt.laddr, err = net.ResolveUDPAddr("udp", "127.0.0.1:"); err != nil {
		return nil, err
	}

	if mime == webrtc.RTPCodecTypeAudio {
		mt.forwardConn = &udpConn{
			port: 4000, payloadType: 111,
		}
	} else {
		mt.forwardConn = &udpConn{
			port: 4002, payloadType: 96,
		}
	}

	return mt, nil
}

func (t *MediaTrack) ForwardRTP(track *webrtc.TrackRemote, rtpReceiver *webrtc.RTPReceiver) {
	log.Debug().Str("service", "MediaTrack").Str("ID", string(t.ID)).Msgf("forward %v", track.Kind().String())

	var (
		err   error
		raddr *net.UDPAddr
	)

	if raddr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", t.forwardConn.port)); err != nil {
		log.Error().Err(err).Str("service", "MediaTrack").Str("ID", string(t.ID)).Msg("")
		return
	}

	// Dial udp
	if t.forwardConn.conn, err = net.DialUDP("udp", t.laddr, raddr); err != nil {
		log.Error().Err(err).Str("service", "MediaTrack").Str("ID", string(t.ID)).Msg("")
		return
	}

	b := make([]byte, 1500)
	rtpPacket := &rtp.Packet{}

	for {
		// Read
		n, _, readErr := track.Read(b)
		if readErr != nil {
			log.Error().Err(readErr).Str("service", "MediaTrack").Str("ID", string(t.ID)).Msg("read track")
			return
		}

		// Unmarshal the packet and update the PayloadType
		if err = rtpPacket.Unmarshal(b[:n]); err != nil {
			log.Error().Err(err).Str("service", "MediaTrack").Str("ID", string(t.ID)).Msg("read track")
			return
		}
		rtpPacket.PayloadType = t.forwardConn.payloadType

		// Marshal into original buffer with updated PayloadType
		if n, err = rtpPacket.MarshalTo(b); err != nil {
			log.Error().Err(err).Str("service", "MediaTrack").Str("ID", string(t.ID)).Msg("read track")
			return
		}

		// Write
		if _, writeErr := t.forwardConn.conn.Write(b[:n]); writeErr != nil {
			// For this particular example, third party applications usually timeout after a short
			// amount of time during which the user doesn't have enough time to provide the answer
			// to the browser.
			// That's why, for this particular example, the user first needs to provide the answer
			// to the browser then open the third party application. Therefore we must not kill
			// the forward on "connection refused" errors
			var opError *net.OpError
			if errors.As(writeErr, &opError) && opError.Err.Error() == "write: connection refused" {
				continue
			}
			log.Error().Err(err).Str("service", "MediaTrack").Str("ID", string(t.ID)).Msg("write track")
			return
		}
	}
	// go t.forwardRTP()
}

func (t *MediaTrack) Close() {
	log.Debug().Str("service", "participant").Str("ID", string(t.ID)).Msg("TODO: close exists MediaTrack")

	if closeErr := t.forwardConn.conn.Close(); closeErr != nil {
		log.Error().Err(closeErr).Str("service", "MediaTrack").Str("ID", string(t.ID)).Msg("")
	}
}
