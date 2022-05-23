package rpc

import "encoding/json"

type SubscribeStreamCancelRpc struct {
	jsonRpcHead
	Params interface{} `json:"params"`
}

func NewSubscribeStreamCancelRpc() *SubscribeStreamCancelRpc {
	return &SubscribeStreamCancelRpc{
		jsonRpcHead: jsonRpcHead{
			Version: jsonRpcVersion,
			Method:  SubscribeStreamCancelMethod,
		},
		Params: nil,
	}
}

func (r SubscribeStreamCancelRpc) GetMethod() Method {
	return r.Method
}

func (r SubscribeStreamCancelRpc) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}
