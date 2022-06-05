package rtc

import (
	"strings"

	"github.com/isqad/livelook-sfu/internal/config"
	"github.com/isqad/livelook-sfu/internal/eventbus/rpc"
	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v3"
)

func createMediaEngine(
	enabledCodecs []config.CodecSpec,
	directionConfig config.DirectionConfig,
	target rpc.SignalingTarget,
) (*webrtc.MediaEngine, *interceptor.Registry, error) {
	mediaEngine := &webrtc.MediaEngine{}
	if err := registerCodecs(mediaEngine, enabledCodecs, directionConfig.RTCPFeedback); err != nil {
		return nil, nil, err
	}

	if err := registerHeaderExtensions(mediaEngine, directionConfig.RTPHeaderExtension); err != nil {
		return nil, nil, err
	}

	ir := &interceptor.Registry{}
	if target == rpc.Publisher {
		// Use the default set of Interceptors
		if err := webrtc.RegisterDefaultInterceptors(mediaEngine, ir); err != nil {
			return nil, nil, err
		}
	}
	// Receiver is not implemented for now

	return mediaEngine, ir, nil

	// if params.Target == rpc.Receiver {
	// 	isSendSideBWE := false
	// 	for _, ext := range directionConfig.RTPHeaderExtension.Video {
	// 		if ext == sdp.TransportCCURI {
	// 			isSendSideBWE = true
	// 			break
	// 		}
	// 	}
	// 	for _, ext := range directionConfig.RTPHeaderExtension.Audio {
	// 		if ext == sdp.TransportCCURI {
	// 			isSendSideBWE = true
	// 			break
	// 		}
	// 	}

	// 	if isSendSideBWE {
	// 		gf, err := cc.NewInterceptor(func() (cc.BandwidthEstimator, error) {
	// 			return gcc.NewSendSideBWE(
	// 				gcc.SendSideBWEInitialBitrate(1*1000*1000),
	// 				gcc.SendSideBWEPacer(gcc.NewNoOpPacer()),
	// 			)
	// 		})
	// 		if err == nil {
	// 			gf.OnNewPeerConnection(func(id string, estimator cc.BandwidthEstimator) {
	// 				if onBandwidthEstimator != nil {
	// 					onBandwidthEstimator(estimator)
	// 				}
	// 			})
	// 			ir.Add(gf)

	// 			tf, err := twcc.NewHeaderExtensionInterceptor()
	// 			if err == nil {
	// 				ir.Add(tf)
	// 			}
	// 		}
	// 	}
	// }
}

func registerCodecs(
	mediaEngine *webrtc.MediaEngine,
	enabledCodecs []config.CodecSpec,
	rtcpFeedback config.RTCPFeedbackConfig,
) error {
	opusCodec := webrtc.RTPCodecCapability{
		MimeType:     webrtc.MimeTypeOpus,
		ClockRate:    48000,
		Channels:     1,
		SDPFmtpLine:  "minptime=10;useinbandfec=1",
		RTCPFeedback: rtcpFeedback.Audio,
	}
	if isCodecEnabled(enabledCodecs, opusCodec) {
		if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
			RTPCodecCapability: opusCodec,
			PayloadType:        111,
		}, webrtc.RTPCodecTypeAudio); err != nil {
			return err
		}
	}

	for _, codec := range []webrtc.RTPCodecParameters{
		{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType:     webrtc.MimeTypeVP8,
				ClockRate:    90000,
				RTCPFeedback: rtcpFeedback.Video,
			},
			PayloadType: 96,
		},
		{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType:     webrtc.MimeTypeVP9,
				ClockRate:    90000,
				SDPFmtpLine:  "profile-id=0",
				RTCPFeedback: rtcpFeedback.Video,
			},
			PayloadType: 98,
		},
		{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType:     webrtc.MimeTypeVP9,
				ClockRate:    90000,
				SDPFmtpLine:  "profile-id=1",
				RTCPFeedback: rtcpFeedback.Video,
			},
			PayloadType: 100,
		},
		{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType:     webrtc.MimeTypeH264,
				ClockRate:    90000,
				SDPFmtpLine:  "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
				RTCPFeedback: rtcpFeedback.Video,
			},
			PayloadType: 125,
		},
		{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType:     webrtc.MimeTypeH264,
				ClockRate:    90000,
				SDPFmtpLine:  "level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42e01f",
				RTCPFeedback: rtcpFeedback.Video,
			},
			PayloadType: 108,
		},
		{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType:     webrtc.MimeTypeH264,
				ClockRate:    90000,
				SDPFmtpLine:  "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=640032",
				RTCPFeedback: rtcpFeedback.Video,
			},
			PayloadType: 123,
		},
		{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType:     webrtc.MimeTypeAV1,
				ClockRate:    90000,
				RTCPFeedback: rtcpFeedback.Video,
			},
			PayloadType: 35,
		},
	} {
		if isCodecEnabled(enabledCodecs, codec.RTPCodecCapability) {
			if err := mediaEngine.RegisterCodec(codec, webrtc.RTPCodecTypeVideo); err != nil {
				return err
			}
		}
	}

	return nil
}

func registerHeaderExtensions(me *webrtc.MediaEngine, rtpHeaderExtension config.RTPHeaderExtensionConfig) error {
	for _, extension := range rtpHeaderExtension.Video {
		if err := me.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: extension}, webrtc.RTPCodecTypeVideo); err != nil {
			return err
		}
	}

	for _, extension := range rtpHeaderExtension.Audio {
		if err := me.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: extension}, webrtc.RTPCodecTypeAudio); err != nil {
			return err
		}
	}

	return nil
}

func isCodecEnabled(codecs []config.CodecSpec, cap webrtc.RTPCodecCapability) bool {
	for _, codec := range codecs {
		if !strings.EqualFold(codec.Mime, cap.MimeType) {
			continue
		}
		if codec.FmtpLine == "" || strings.EqualFold(codec.FmtpLine, cap.SDPFmtpLine) {
			return true
		}
	}
	return false
}
