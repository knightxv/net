package client

import (
	"github.com/guabee/bnrtc/bnrtcv2/message"
	"github.com/guabee/bnrtc/buffer"
)

type StatusCode uint8

const (
	evt_success     StatusCode = 0
	evt_send_failed StatusCode = 1
	// evt_offline        StatusCode = 2
	evt_dport_bound   StatusCode = 3
	evt_dport_unbound StatusCode = 4
	// evt_connect_failed StatusCode = 5
	// evt_format_err     StatusCode = 6
)

type CloseMsgEvt struct {
}

func (evt *CloseMsgEvt) EncodeData() []byte {
	return nil
}

type DataMsgEvt message.Message

func (evt *DataMsgEvt) EncodeData() []byte {
	mcData := &MCData{
		msg: (*message.Message)(evt),
	}

	if evt.Sync {
		mcData.mt = MessageTypeSync
	} else {
		mcData.mt = MessageTypeData
	}

	return EncodeMCData(mcData).Data()
}

type AckMsgEvt struct {
	Dport      string
	MsgId      uint32
	StatusCode StatusCode
}

func (evt *AckMsgEvt) EncodeData() []byte {
	buf := buffer.NewBuffer(0, nil)
	buf.PushU32(evt.MsgId)
	buf.PushU8(uint8(evt.StatusCode))

	mcData := &MCData{
		mt: MessageTypeACK,
		msg: &message.Message{
			Dport:  evt.Dport,
			Buffer: buf,
		},
	}
	return EncodeMCData(mcData).Data()
}

type MessageType uint8

const (
	MessageTypeBindDport MessageType = iota
	MessageTypeUnbindDport
	MessageTypeData
	MessageTypeMulticast
	MessageTypeBroadcast
	MessageTypeSync
	MessageTypeACK
	MessageTypeNeighborCast

	// internal message type
	MessageTypeClose
)

func (t MessageType) String() string {
	switch t {
	case MessageTypeBindDport:
		return "MessageTypeBindDport"
	case MessageTypeUnbindDport:
		return "MessageTypeUnbindDport"
	case MessageTypeData:
		return "MessageTypeData"
	case MessageTypeMulticast:
		return "MessageTypeMulticast"
	case MessageTypeBroadcast:
		return "MessageTypeBroadcast"
	case MessageTypeSync:
		return "MessageTypeSync"
	case MessageTypeACK:
		return "MessageTypeACK"
	case MessageTypeNeighborCast:
		return "MessageTypeNeighborCast"
	case MessageTypeClose:
		return "MessageTypeClose"
	default:
		return "MessageTypeUnknown"
	}
}

type WSData struct {
	mt        MessageType
	msgId     uint32
	neighbors uint32
	msg       *message.Message
}

type MCData struct {
	mt  MessageType
	msg *message.Message
}

func DecodeWSData(buf *buffer.Buffer) (data *WSData) {
	/*
	* MessageTypeClose
	* --------
	* |  u8  |
	* --------
	* | type |
	* --------
	* MessageTypeAck
	* -------------------
	* |  u8   |  uint32 |
	* -------------------
	* | type  |  msgId  |
	* -------------------
	* MessageTypeData
	* ---------------------------
	* |  u8   |  uint32 | bytes |
	* ---------------------------
	* | type  |  msgId  | data  |
	* ---------------------------
	* MessageTypeNeighborCast
	* --------------------------------------
	* |  u8   |  uint32 |  uint32  | bytes |
	* --------------------------------------
	* | type  |  msgId  | neighors | data  |
	* --------------------------------------

	**/
	if buf.Len() < 1 { // type
		return nil
	}

	mt := MessageType(buf.PullU8())
	if mt == MessageTypeClose {
		return &WSData{mt: MessageTypeClose}
	}

	if buf.Len() < 4 { // msgId
		return nil
	}
	msgId := buf.PullU32()
	if mt == MessageTypeACK {
		return &WSData{mt: mt}
	}

	if buf.Len() < 4 { // neighbors
		return nil
	}
	neighbors := uint32(0)
	if mt == MessageTypeNeighborCast {
		neighbors = buf.PullU32()
	}

	msg := message.FromBuffer(buf)
	if msg == nil {
		return nil
	}
	return &WSData{mt: mt, neighbors: neighbors, msgId: msgId, msg: msg}
}

func EncodeMCData(data *MCData) *buffer.Buffer {
	/*
	 *  MessageTypeData / MessageTypeACK
	 * ----------------
	 * |  u8  | bytes |
	 * ----------------
	 * | type |  data |
	 * ----------------
	 **/
	var buf *buffer.Buffer
	if data.msg == nil {
		buf = buffer.NewBuffer(0, nil)
	} else {
		buf = data.msg.GetBuffer()
	}

	buf.PutU8(uint8(data.mt))
	return buf
}
