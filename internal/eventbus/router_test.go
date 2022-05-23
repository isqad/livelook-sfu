package eventbus

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/go-redis/redis/v8"
	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/eventbus/rpc"
	"github.com/stretchr/testify/assert"
)

const (
	mockUserSessionID = core.UserSessionID("0c4038d6-da68-11ec-9d64-0242ac120002")
)

type MockCallbacks struct {
	JoinCallbackFired            bool
	AddICECandidateCallbackFired bool
	OnOfferFired                 bool
	OnPublishStreamFired         bool
	OnStopStreamFired            bool
}

func (m *MockCallbacks) JoinMockCallback(userID core.UserSessionID) error {
	m.JoinCallbackFired = true

	return nil
}
func (m *MockCallbacks) OnICECandidate(userID core.UserSessionID, candidate rpc.ICECandidateParams) error {
	m.AddICECandidateCallbackFired = true

	return nil
}

func (m *MockCallbacks) OnOffer(userID core.UserSessionID, sdp rpc.SDPParams) error {
	m.OnOfferFired = true

	return nil
}

func (m *MockCallbacks) OnPublishStream(userID core.UserSessionID) error {
	m.OnPublishStreamFired = true

	return nil
}

func (m *MockCallbacks) OnStopStream(userID core.UserSessionID) error {
	m.OnStopStreamFired = true

	return nil
}

func TestNewRouter(t *testing.T) {
	mockBus := NewMockBus()
	defer mockBus.Close()

	s := NewMockSubscriber(mockBus)

	_, err := NewRouter(s)
	assert.Nil(t, err)

	assert.Equal(t, true, s.ServerSubscribed)
	assert.Equal(t, false, s.ClientSubscribed)
}

func TestParseRpc(t *testing.T) {
	payload, err := mockServerMessagePayload(rpc.JoinMethod, "null")
	assert.Nil(t, err)

	uid, r, err := parseRpc(payload)
	assert.Nil(t, err)

	assert.Equal(t, mockUserSessionID, uid)
	assert.Equal(t, rpc.JoinMethod, r.GetMethod())
}

func TestOnJoin(t *testing.T) {
	payload, err := mockServerMessagePayload(rpc.JoinMethod, "null")
	assert.Nil(t, err)

	callbacks := &MockCallbacks{}

	mockBus := NewMockBus()

	s := NewMockSubscriber(mockBus)
	router, err := NewRouter(s)
	assert.Nil(t, err)

	router.OnJoin(callbacks.JoinMockCallback)

	<-router.Start()
	msg := &redis.Message{Payload: string(payload[:])}
	mockBus.Messages <- msg
	<-router.Stop()

	assert.Equal(t, true, callbacks.JoinCallbackFired)
}

func TestOnAddICECandidate(t *testing.T) {
	payload, err := mockServerMessagePayload(rpc.ICECandidateMethod, "{}")
	assert.Nil(t, err)

	callbacks := &MockCallbacks{}

	mockBus := NewMockBus()

	s := NewMockSubscriber(mockBus)
	router, err := NewRouter(s)
	assert.Nil(t, err)

	router.OnAddICECandidate(callbacks.OnICECandidate)

	<-router.Start()
	msg := &redis.Message{Payload: string(payload[:])}
	mockBus.Messages <- msg
	<-router.Stop()

	assert.Equal(t, true, callbacks.AddICECandidateCallbackFired)
}

func TestOnOffer(t *testing.T) {
	payload, err := mockServerMessagePayload(rpc.SDPOfferMethod, "{}")
	assert.Nil(t, err)

	callbacks := &MockCallbacks{}

	mockBus := NewMockBus()

	s := NewMockSubscriber(mockBus)
	router, err := NewRouter(s)
	assert.Nil(t, err)

	router.OnOffer(callbacks.OnOffer)

	<-router.Start()
	msg := &redis.Message{Payload: string(payload[:])}
	mockBus.Messages <- msg
	<-router.Stop()

	assert.Equal(t, true, callbacks.OnOfferFired)
}

func TestOnPublishStream(t *testing.T) {
	payload, err := mockServerMessagePayload(rpc.PublishStreamMethod, "{}")
	assert.Nil(t, err)

	callbacks := &MockCallbacks{}

	mockBus := NewMockBus()

	s := NewMockSubscriber(mockBus)
	router, err := NewRouter(s)
	assert.Nil(t, err)

	router.OnPublishStream(callbacks.OnPublishStream)

	<-router.Start()
	msg := &redis.Message{Payload: string(payload[:])}
	mockBus.Messages <- msg
	<-router.Stop()

	assert.Equal(t, true, callbacks.OnPublishStreamFired)
}

func TestOnStopStream(t *testing.T) {
	payload, err := mockServerMessagePayload(rpc.PublishStreamStopMethod, "{}")
	assert.Nil(t, err)

	callbacks := &MockCallbacks{}

	mockBus := NewMockBus()

	s := NewMockSubscriber(mockBus)
	router, err := NewRouter(s)
	assert.Nil(t, err)

	router.OnStopStream(callbacks.OnStopStream)

	<-router.Start()
	msg := &redis.Message{Payload: string(payload[:])}
	mockBus.Messages <- msg
	<-router.Stop()

	assert.Equal(t, true, callbacks.OnStopStreamFired)
}

func mockServerMessagePayload(method rpc.Method, params string) ([]byte, error) {
	rpcBytes := []byte(fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"%s","params":%s}`,
		string(method),
		params,
	))

	serverMessage := &ServerMessage{
		UserID:  mockUserSessionID,
		Message: rpcBytes,
	}

	return json.Marshal(serverMessage)
}
