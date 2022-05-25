package rpc

import (
	"encoding/json"

	"github.com/isqad/livelook-sfu/internal/core"
)

type SubscribeParams struct {
	UserID core.UserSessionID `json:"user_id"`
}

type SubscribeStreamRpc struct {
	jsonRpcHead
	Params SubscribeParams `json:"params"`
}

func NewSubscribeStreamRpc(userID core.UserSessionID) *SubscribeStreamRpc {
	return &SubscribeStreamRpc{
		jsonRpcHead: jsonRpcHead{
			Version: jsonRpcVersion,
			Method:  SubscribeStreamMethod,
		},
		Params: SubscribeParams{userID},
	}
}

func (r SubscribeStreamRpc) GetMethod() Method {
	return r.Method
}

func (r SubscribeStreamRpc) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}
