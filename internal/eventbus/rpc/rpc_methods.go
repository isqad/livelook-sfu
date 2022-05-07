package rpc

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"

	"github.com/pion/webrtc/v3"

	"github.com/rs/zerolog/log"
)

const jsonRpcVersion = "2.0"

type Method string

const (
	JoinMethod                  Method = "join"
	ICECandidateMethod          Method = "iceCandidate"
	SDPOfferMethod              Method = "offer"
	SDPAnswerMethod             Method = "answer"
	CloseSessionMethod          Method = "close_session"
	PublishStreamMethod         Method = "publish"
	PublishStreamStopMethod     Method = "publishStop"
	SubscribeStreamMethod       Method = "subscribe"
	SubscribeStreamCancelMethod Method = "subscribeCancel"
)

var (
	ErrUnknownRpcType = errors.New("unknown RPC type")
	ErrMalformedRpc   = errors.New("malformed RPC")
)

type Rpc interface {
	GetMethod() Method
	ToJSON() ([]byte, error)
}

type jsonRpcHead struct {
	Version string `json:"jsonrpc"`
	Method  Method `json:"method"`
}

type jsonRpc struct {
	jsonRpcHead
	Params map[string]interface{} `json:"params"`
}

func RpcFromReader(reader io.Reader) (Rpc, error) {
	rpc := &jsonRpc{}

	err := json.NewDecoder(reader).Decode(rpc)
	if err != nil {
		return nil, err
	}

	params, err := json.Marshal(rpc.Params)
	if err != nil {
		return nil, err
	}

	switch rpc.Method {
	case ICECandidateMethod:
		c := &webrtc.ICECandidateInit{}
		if err := json.Unmarshal(params, c); err != nil {
			return nil, err
		}

		return NewICECandidateRpc(c), nil
	case SDPAnswerMethod:
		sdp := &webrtc.SessionDescription{}
		if err := json.Unmarshal(params, sdp); err != nil {
			return nil, err
		}

		return NewSDPAnswerRpc(sdp), nil
	case SDPOfferMethod:
		sdp := &webrtc.SessionDescription{}

		if err := json.Unmarshal(params, sdp); err != nil {
			return nil, err
		}

		// Unzip sdp
		log.Debug().Interface("sdp", sdp.SDP).Msg("decode offer SDP")

		gzdata, err := base64.StdEncoding.DecodeString(sdp.SDP)
		if err != nil {
			return nil, err
		}

		zr, err := gzip.NewReader(bytes.NewReader(gzdata))
		if err != nil {
			return nil, err
		}

		data, err := ioutil.ReadAll(zr)
		if err != nil {
			return nil, err
		}

		sdp.SDP = string(data)

		return NewSDPOfferRpc(sdp), nil
	case CloseSessionMethod:
		return NewCloseSessionRpc(), nil
	case PublishStreamMethod:
		return NewStartStreamRpc(), nil
	case PublishStreamStopMethod:
		return NewStopStreamRpc(), nil
	case JoinMethod:
		return NewJoinRpc(), nil
	default:
		return nil, ErrUnknownRpcType
	}
}
