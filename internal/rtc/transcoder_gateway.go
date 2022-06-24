package rtc

// TranscoderGateway is interface for interact with transcoder
//
// Here is creating SDP and sending signal to run a transcoder (i.e. ffmpeg)
// Also here is allocating UDP ports for every enabled codecs
// then by OnTrack event it forwards RTP packets to transcoder via UDP
type TranscoderGateway struct{}
