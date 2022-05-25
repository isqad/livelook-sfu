package config

import (
	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v3"
)

type CongestionControlProbeMode string

const (
	CongestionControlProbeModePadding CongestionControlProbeMode = "padding"
	CongestionControlProbeModeMedia   CongestionControlProbeMode = "media"

	frameMarking = "urn:ietf:params:rtp-hdrext:framemarking"
)

var DefaultStunServers = []string{
	"stun:stun.l.google.com:19302",
	"stun:stun1.l.google.com:19302",
}

type Config struct {
	Peer PeerConfig
	RTC  RTCConfig
}

type RTCConfig struct {
	ICEPortRangeStart uint32
	ICEPortRangeEnd   uint32
	Interfaces        InterfacesConfig
	CongestionControl CongestionControlConfig
}

type CodecSpec struct {
	Mime     string
	FmtpLine string
}

type WebRTCConfig struct {
	Configuration webrtc.Configuration
	SettingEngine webrtc.SettingEngine
	Publisher     DirectionConfig
	Subscriber    DirectionConfig
}

type RTPHeaderExtensionConfig struct {
	Audio []string
	Video []string
}

type RTCPFeedbackConfig struct {
	Audio []webrtc.RTCPFeedback
	Video []webrtc.RTCPFeedback
}

type DirectionConfig struct {
	RTPHeaderExtension RTPHeaderExtensionConfig
	RTCPFeedback       RTCPFeedbackConfig
}

type InterfacesConfig struct {
	Includes []string
}

type PeerConfig struct {
	EnabledCodecs []CodecSpec
}

type CongestionControlConfig struct {
	Enabled            bool
	AllowPause         bool
	UseSendSideBWE     bool
	ProbeMode          CongestionControlProbeMode
	MinChannelCapacity int64
}

func NewConfig() *Config {
	// TODO: extract to yaml
	conf := &Config{
		RTC: RTCConfig{
			ICEPortRangeStart: 50000,
			ICEPortRangeEnd:   60000,
			Interfaces: InterfacesConfig{
				Includes: []string{"wlp0s20u9", "enp3s0"},
			},
			CongestionControl: CongestionControlConfig{
				Enabled:    true,
				AllowPause: false,
				ProbeMode:  CongestionControlProbeModePadding,
			},
		},
		Peer: PeerConfig{
			EnabledCodecs: []CodecSpec{
				{Mime: webrtc.MimeTypeOpus},
				{Mime: webrtc.MimeTypeVP8},
			},
		},
	}

	return conf
}

func NewWebRTCConfig(config *Config) (*WebRTCConfig, error) {
	rtcConf := config.RTC

	c := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: DefaultStunServers,
			},
		},
		SDPSemantics: webrtc.SDPSemanticsUnifiedPlan,
	}
	s := webrtc.SettingEngine{
		// LoggerFactory: logging.NewLoggerFactory(logger.GetLogger()),
	}

	// when set to true, attempts to discover the host's public IP via STUN
	// this is useful for cloud environments such as AWS & Google where hosts have an internal IP
	// that maps to an external one
	// if conf.RTC.UseExternalIP && externalIP != "" {
	// 	s.SetNAT1To1IPs([]string{externalIP}, webrtc.ICECandidateTypeHost)
	// }

	networkTypes := make([]webrtc.NetworkType, 0, 4)
	// Use only UDP
	networkTypes = append(networkTypes,
		webrtc.NetworkTypeUDP4, webrtc.NetworkTypeUDP6,
	)
	// TODO: configure it
	if err := s.SetEphemeralUDPPortRange(uint16(config.RTC.ICEPortRangeStart), uint16(config.RTC.ICEPortRangeEnd)); err != nil {
		return nil, err
	}
	s.SetNetworkTypes(networkTypes)

	// publisher configuration
	publisherConfig := DirectionConfig{
		RTPHeaderExtension: RTPHeaderExtensionConfig{
			Audio: []string{
				sdp.SDESMidURI,
				sdp.SDESRTPStreamIDURI,
				sdp.AudioLevelURI,
			},
			Video: []string{
				sdp.SDESMidURI,
				sdp.SDESRTPStreamIDURI,
				sdp.TransportCCURI,
				frameMarking,
			},
		},
		RTCPFeedback: RTCPFeedbackConfig{
			Video: []webrtc.RTCPFeedback{
				{Type: webrtc.TypeRTCPFBGoogREMB},
				{Type: webrtc.TypeRTCPFBTransportCC},
				{Type: webrtc.TypeRTCPFBCCM, Parameter: "fir"},
				{Type: webrtc.TypeRTCPFBNACK},
				{Type: webrtc.TypeRTCPFBNACK, Parameter: "pli"},
			},
		},
	}

	// subscriber configuration
	subscriberConfig := DirectionConfig{
		RTCPFeedback: RTCPFeedbackConfig{
			Video: []webrtc.RTCPFeedback{
				{Type: webrtc.TypeRTCPFBCCM, Parameter: "fir"},
				{Type: webrtc.TypeRTCPFBNACK},
				{Type: webrtc.TypeRTCPFBNACK, Parameter: "pli"},
			},
		},
	}

	if rtcConf.CongestionControl.UseSendSideBWE {
		subscriberConfig.RTPHeaderExtension.Video = append(
			subscriberConfig.RTPHeaderExtension.Video,
			sdp.TransportCCURI,
		)
		subscriberConfig.RTCPFeedback.Video = append(
			subscriberConfig.RTCPFeedback.Video,
			webrtc.RTCPFeedback{Type: webrtc.TypeRTCPFBTransportCC},
		)
	} else {
		// By default set this RTP extensions
		// See the https://webrtc.googlesource.com/src/+/main/docs/native-code/rtp-hdrext/index.md
		subscriberConfig.RTPHeaderExtension.Video = append(
			subscriberConfig.RTPHeaderExtension.Video,
			sdp.ABSSendTimeURI,
		)

		subscriberConfig.RTCPFeedback.Video = append(
			subscriberConfig.RTCPFeedback.Video,
			webrtc.RTCPFeedback{Type: webrtc.TypeRTCPFBGoogREMB},
		)
	}

	// Filter interfaces
	if len(rtcConf.Interfaces.Includes) != 0 {
		includes := rtcConf.Interfaces.Includes
		s.SetInterfaceFilter(func(s string) bool {
			// filter by include interfaces
			if len(includes) > 0 {
				for _, iface := range includes {
					if iface == s {
						return true
					}
				}
				return false
			}
			return true
		})
	}

	return &WebRTCConfig{
		Configuration: c,
		SettingEngine: s,
		Publisher:     publisherConfig,
		Subscriber:    subscriberConfig,
	}, nil

}
