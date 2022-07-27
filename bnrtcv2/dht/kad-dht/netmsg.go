package kaddht

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/dht/idht"
)

type messageType uint8

const (
	messageTypePing messageType = iota
	messageTypeStore
	messageTypeFindNode
	messageTypeFindValue
	messageTypeJoinGroup
	messageTypeLeaveGroup
)

func (tm messageType) String() string {
	switch tm {
	case messageTypePing:
		return "Ping"
	case messageTypeStore:
		return "Store"
	case messageTypeFindNode:
		return "FindNode"
	case messageTypeFindValue:
		return "FindValue"
	case messageTypeJoinGroup:
		return "JoinGroup"
	case messageTypeLeaveGroup:
		return "LeaveGroup"
	default:
		return fmt.Sprintf("Unknown(%d)", tm)
	}
}

type message struct {
	Sender     *Node
	Receiver   *Node
	ID         int64
	Error      error
	Type       messageType
	IsResponse bool
	Data       interface{}
	Iterative  bool
}

type messageRequestOptions struct {
	Target    []byte
	Data      []byte
	Args      interface{}
	Iterative bool
	Timeout   time.Duration
}

type messageResponseOptions struct {
	timeout time.Duration
}

type messageOptionData struct {
	ID         int64
	Error      error
	Iterative  bool
	IsResponse bool
	Data       interface{}
}

func newMessage(t messageType, sender, receiver *Node, data *messageOptionData) *message {
	if data == nil {
		data = &messageOptionData{}
	}
	return &message{
		Sender:     sender,
		Receiver:   receiver,
		ID:         data.ID,
		Error:      data.Error,
		Type:       t,
		IsResponse: data.IsResponse,
		Data:       data.Data,
		Iterative:  data.Iterative,
	}
}

type queryDataFindNode struct {
	Target []byte
}

type queryDataFindValue struct {
	Target  []byte
	NoCache bool
}

type queryDataStore struct {
	Key        []byte
	Data       []byte
	Options    *idht.DHTStoreOptions
	Publishing bool // Whether or not we are the original publisher
}

type responseDataFindNode struct {
	Closest []*Node
}

type responseDataFindValue struct {
	Closest []*Node
	Data    *StoreData
}

type responseDataStore struct {
	Success bool
}

func netMsgInit() {
	gob.Register(&queryDataFindNode{})
	gob.Register(&queryDataFindValue{})
	gob.Register(&queryDataStore{})
	gob.Register(&responseDataFindNode{})
	gob.Register(&responseDataFindValue{})
	gob.Register(&responseDataStore{})
}

func serializeMessage(q *message) ([]byte, error) {
	var msgBuffer bytes.Buffer
	enc := gob.NewEncoder(&msgBuffer)
	err := enc.Encode(q)
	if err != nil {
		return nil, err
	}
	return msgBuffer.Bytes(), nil
}

func deserializeMessage(data []byte) (*message, error) {
	reader := bytes.NewBuffer(data)
	msg := &message{}
	dec := gob.NewDecoder(reader)

	err := dec.Decode(msg)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

type messageChannel struct {
	dataChannel chan *message
	size        int
}

func newMessageChannel(size int) *messageChannel {
	return &messageChannel{
		dataChannel: make(chan *message, size),
		size:        size,
	}
}

func (m *messageChannel) Send(msg *message) {
	if m.size == 0 {
		m.dataChannel <- msg
	} else {
		select {
		case m.dataChannel <- msg:
		default:
			<-m.dataChannel // discard oldest message
			m.dataChannel <- msg
		}
	}
}

func (m *messageChannel) GetMsgChannel() <-chan *message {
	return m.dataChannel
}

func (m *messageChannel) Close() {
	close(m.dataChannel)
}
