package eventbus

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/pion/webrtc/v3"
)

const jsonRpcVersion = "2.0"

type Method string

const (
	ICECandidateMethod Method = "iceCandidate"
	SDPAnswerMethod    Method = "answer"
	SDPOfferMethod     Method = "offer"
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

var (
	ErrUnknownRpcType = errors.New("unknown RPC type")
	ErrMalformedRpc   = errors.New("malformed RPC")
)

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

		return NewICECandidateRpc(*c), nil
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
	default:
		return nil, ErrUnknownRpcType
	}
}

// SDP RPC
type SDPRpc struct {
	jsonRpcHead
	Params *webrtc.SessionDescription `json:"params"`
}

func NewSDPOfferRpc(sdp *webrtc.SessionDescription) SDPRpc {
	return SDPRpc{
		jsonRpcHead: jsonRpcHead{
			Version: jsonRpcVersion,
			Method:  SDPOfferMethod,
		},
		Params: sdp,
	}
}

func NewSDPAnswerRpc(sdp *webrtc.SessionDescription) SDPRpc {
	return SDPRpc{
		jsonRpcHead: jsonRpcHead{
			Version: jsonRpcVersion,
			Method:  SDPAnswerMethod,
		},
		Params: sdp,
	}
}

func (r SDPRpc) GetMethod() Method {
	return r.Method
}

func (r SDPRpc) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// ICE candidate RPC
type ICECandidateRpc struct {
	jsonRpcHead
	Params webrtc.ICECandidateInit `json:"params"`
}

func NewICECandidateRpc(candidate webrtc.ICECandidateInit) ICECandidateRpc {
	return ICECandidateRpc{
		jsonRpcHead: jsonRpcHead{
			Version: jsonRpcVersion,
			Method:  ICECandidateMethod,
		},
		Params: candidate,
	}
}

func (r ICECandidateRpc) GetMethod() Method {
	return r.Method
}

func (r ICECandidateRpc) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}
