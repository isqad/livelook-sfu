package bot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/isqad/livelook-sfu/internal/eventbus/rpc"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/ivfreader"
	"golang.org/x/net/publicsuffix"
)

const (
	videoFileName = "video.ivf"
)

type Bot struct {
	serverHost    string
	client        *http.Client
	cookieJar     *cookiejar.Jar
	websocketConn *websocket.Conn

	peerConnection *webrtc.PeerConnection

	lock              sync.Mutex
	pendingCandidates []webrtc.ICECandidateInit
}

func New(host, login, password string) (*Bot, error) {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	u, err := url.Parse(fmt.Sprintf("https://%s/admin/login", host))
	if err != nil {
		return nil, err
	}

	// Try to auth
	resp, err := httpClient.PostForm(
		u.String(),
		url.Values{"email": []string{login}, "password": []string{password}},
	)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	bot := &Bot{
		serverHost: host,
		client:     httpClient,
		cookieJar:  jar,
	}

	return bot, nil
}

func (bot *Bot) Close() {
	bot.client.CloseIdleConnections()

	if bot.peerConnection != nil {
		bot.peerConnection.Close()
	}

	if bot.websocketConn != nil {
		bot.websocketConn.Close()
	}
}

func (bot *Bot) Start() error {
	defer bot.Close()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	dialer := &websocket.Dialer{
		Jar:              bot.cookieJar,
		HandshakeTimeout: 45 * time.Second,
	}

	c, resp, err := dialer.Dial(fmt.Sprintf("wss://%s/api/v1/ws", bot.serverHost), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()

	bot.websocketConn = c

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			if err := bot.readRPC(c); err != nil {
				log.Printf("read error: %v", err)
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return nil
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				return err
			}

			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return nil
		}
	}
}

func (bot *Bot) readRPC(conn *websocket.Conn) error {
	_, message, err := conn.ReadMessage()
	if err != nil {
		return err
	}

	reader := bytes.NewReader(message)
	p, err := rpc.RpcFromReader(reader)
	if err != nil {
		return err
	}

	log.Printf("parsed RPC: %v", p)

	switch p.GetMethod() {
	case rpc.JoinMethod:
		go func() {
			if err := bot.createPeerConnection(); err != nil {
				log.Printf("create peer conn error: %v", err)
			}
		}()
	case rpc.ICECandidateMethod:
		msg, ok := p.(*rpc.ICECandidateRpc)
		if !ok {
			return errors.New("can't convert to ICECandidateRpc")
		}
		if err := bot.addICECandidate(msg.Params.ICECandidateInit); err != nil {
			return err
		}
	case rpc.SDPAnswerMethod:
		msg, ok := p.(*rpc.SDPRpc)
		if !ok {
			return errors.New("can't convert to SDPRpc")
		}

		if err := bot.setRemoteDescription(msg.Params.SessionDescription); err != nil {
			return err
		}
	default:
		log.Println("unknown type")
	}

	return nil
}

func (bot *Bot) createPeerConnection() error {
	var err error
	// Create a new RTCPeerConnection
	bot.peerConnection, err = webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		return err
	}

	iceConnectedCtx, iceConnectedCtxCancel := context.WithCancel(context.Background())
	bot.peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			// All candidates are gathered
			return
		}

		// Send ICE candidate
		p, err := rpc.NewICECandidateRpc(candidate.ToJSON(), rpc.Publisher).ToJSON()
		if err != nil {
			log.Printf("error form ICE candidate: %v", err)
		}

		if err := bot.websocketConn.WriteMessage(websocket.TextMessage, p); err != nil {
			log.Printf("error send ICE candidate: %v", err)
		}
	})
	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	bot.peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.Printf("Connection State has changed %s \n", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateConnected {
			iceConnectedCtxCancel()
		}
	})

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	bot.peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		log.Printf("Peer Connection State has changed: %s\n", s.String())

		if s == webrtc.PeerConnectionStateFailed {
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			fmt.Println("Peer Connection has gone to failed exiting")
			os.Exit(0)
		}
	})

	// Create a video track
	videoTrack, videoTrackErr := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "pion")
	if videoTrackErr != nil {
		return videoTrackErr
	}

	rtpSender, videoTrackErr := bot.peerConnection.AddTrack(videoTrack)
	if videoTrackErr != nil {
		return videoTrackErr
	}

	// Read incoming RTCP packets
	// Before these packets are returned they are processed by interceptors. For things
	// like NACK this needs to be called.
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	// Create Offer
	offer, err := bot.peerConnection.CreateOffer(nil)
	if err != nil {
		return err
	}

	if err := bot.peerConnection.SetLocalDescription(offer); err != nil {
		return err
	}

	// Send offer
	p, err := rpc.NewSDPOfferRpc(&offer, rpc.Publisher).ToJSON()
	if err != nil {
		return err
	}

	if err := bot.websocketConn.WriteMessage(websocket.TextMessage, p); err != nil {
		return err
	}

	// Open a IVF file and start reading using our IVFReader
	file, ivfErr := os.Open(videoFileName)
	if ivfErr != nil {
		return ivfErr
	}

	ivf, header, ivfErr := ivfreader.NewWith(file)
	if ivfErr != nil {
		return ivfErr
	}

	<-iceConnectedCtx.Done()

	log.Println("start main loop")

	// Send our video file frame at a time. Pace our sending so we send it at the same speed it should be played back as.
	// This isn't required since the video is timestamped, but we will such much higher loss if we send all at once.
	//
	// It is important to use a time.Ticker instead of time.Sleep because
	// * avoids accumulating skew, just calling time.Sleep didn't compensate for the time spent parsing the data
	// * works around latency issues with Sleep (see https://github.com/golang/go/issues/44343)
	ticker := time.NewTicker(time.Millisecond * time.Duration((float32(header.TimebaseNumerator)/float32(header.TimebaseDenominator))*1000))
	for ; true; <-ticker.C {
		frame, _, ivfErr := ivf.ParseNextFrame()
		if errors.Is(ivfErr, io.EOF) {
			fmt.Printf("All video frames parsed and sent")
			os.Exit(0)
		}

		if ivfErr != nil {
			return ivfErr
		}

		if ivfErr = videoTrack.WriteSample(media.Sample{Data: frame, Duration: time.Second}); ivfErr != nil {
			return ivfErr
		}
	}

	return nil
}

func (bot *Bot) addICECandidate(candidate webrtc.ICECandidateInit) error {
	desc := bot.peerConnection.RemoteDescription()
	if desc != nil {
		bot.peerConnection.AddICECandidate(candidate)
		return nil
	}

	bot.lock.Lock()
	defer bot.lock.Unlock()

	bot.pendingCandidates = append(bot.pendingCandidates, candidate)

	return nil
}

func (bot *Bot) setRemoteDescription(sdp webrtc.SessionDescription) error {
	if err := bot.peerConnection.SetRemoteDescription(sdp); err != nil {
		return err
	}

	bot.lock.Lock()
	defer bot.lock.Unlock()

	for _, candidate := range bot.pendingCandidates {
		if err := bot.peerConnection.AddICECandidate(candidate); err != nil {
			return err
		}
	}

	bot.pendingCandidates = make([]webrtc.ICECandidateInit, 0)

	return nil
}
