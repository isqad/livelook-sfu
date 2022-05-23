package rpc

import "encoding/json"

type SubscribeStreamRpc struct {
	jsonRpcHead
	Params interface{} `json:"params"`
}

func NewSubscribeStreamRpc() *SubscribeStreamRpc {
	return &SubscribeStreamRpc{
		jsonRpcHead: jsonRpcHead{
			Version: jsonRpcVersion,
			Method:  SubscribeStreamMethod,
		},
		Params: nil,
	}
}

func (r SubscribeStreamRpc) GetMethod() Method {
	return r.Method
}

func (r SubscribeStreamRpc) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}
