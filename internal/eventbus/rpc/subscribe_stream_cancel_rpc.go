package rpc

import (
	"encoding/json"

	"github.com/isqad/livelook-sfu/internal/core"
)

type SubscribeStreamCancelRpc struct {
	jsonRpcHead
	Params SubscribeParams `json:"params"`
}

func NewSubscribeStreamCancelRpc(userID core.UserSessionID) *SubscribeStreamCancelRpc {
	return &SubscribeStreamCancelRpc{
		jsonRpcHead: jsonRpcHead{
			Version: jsonRpcVersion,
			Method:  SubscribeStreamCancelMethod,
		},
		Params: SubscribeParams{userID},
	}
}

func (r SubscribeStreamCancelRpc) GetMethod() Method {
	return r.Method
}

func (r SubscribeStreamCancelRpc) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}
