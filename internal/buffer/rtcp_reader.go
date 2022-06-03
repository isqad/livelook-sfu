package buffer

import "github.com/rs/zerolog/log"

type RTCPReader struct {
	ssrc uint32
}

func NewRTCPReader(ssrc uint32) *RTCPReader {
	return &RTCPReader{ssrc: ssrc}
}

func (r *RTCPReader) Close() error {
	log.Debug().Str("service", "SRTCP buffer").Msg("close")
	return nil
}

func (r *RTCPReader) Read(p []byte) (n int, err error) {
	log.Debug().Str("service", "SRTCP buffer").Msg("read packet")

	return 0, nil
}

func (r *RTCPReader) Write(p []byte) (n int, err error) {
	log.Debug().Str("service", "SRTCP buffer").Msg("write packet")
	return 0, nil
}
