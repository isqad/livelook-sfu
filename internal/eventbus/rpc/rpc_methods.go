package rpc

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/pion/webrtc/v3"
)

const jsonRpcVersion = "2.0"

type Method string

const (
	JoinMethod         Method = "join"
	ICECandidateMethod Method = "iceCandidate"
	SDPOfferMethod     Method = "offer"
	SDPAnswerMethod    Method = "answer"
	CloseSessionMethod Method = "close_session"
	StartStreamMethod  Method = "start_stream"
	StopStreamMethod   Method = "stop_stream"
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

		return NewSDPOfferRpc(sdp), nil
	case CloseSessionMethod:
		return NewCloseSessionRpc(), nil
	case StartStreamMethod:
		return NewStartStreamRpc(), nil
	case StopStreamMethod:
		return NewStopStreamRpc(), nil
	case JoinMethod:
		return NewJoinRpc(), nil
	default:
		return nil, ErrUnknownRpcType
	}
}
