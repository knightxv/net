package dchat

import (
	"fmt"

	"github.com/guabee/bnrtc/buffer"
)

type messageType uint8

const (
	messageType_data = uint8(1)
	messageType_ack  = uint8(2)
)

const (
	messageType_ev_online messageType = iota
	messageType_ev_offline
	messageType_ev_add_friends
	messageType_ev_del_friends
	messageType_ev_message
	messageType_ev_keepalive
	messageType_ev_online_ack
)

func (t messageType) String() string {
	switch t {
	case messageType_ev_online:
		return "online"
	case messageType_ev_offline:
		return "offline"
	case messageType_ev_add_friends:
		return "add_friends"
	case messageType_ev_del_friends:
		return "del_friends"
	case messageType_ev_message:
		return "message"
	case messageType_ev_keepalive:
		return "keepalive"
	case messageType_ev_online_ack:
		return "online_ack"
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}

type DchatMessage struct {
	Type     uint8
	MsgId    uint32
	Magic    uint32
	Buffer   *buffer.Buffer
	address  string
	devid    string
	isPushed bool
}

type DchatDataMessage struct {
	Type     messageType
	Buffer   *buffer.Buffer
	isPushed bool
}

func getStringArray(buf *buffer.Buffer) []string {
	array := make([]string, 0)
	for {
		str, err := buf.PullStringWithError()
		if err != nil {
			break
		}

		if str == "" {
			continue
		}

		array = append(array, str)
	}

	return array
}

func stringArrayToBufferData(arr []string) []byte {
	if len(arr) == 0 {
		return nil
	}
	buf := buffer.NewBuffer(0, nil)
	for _, str := range arr {
		buf.PushString(str)
	}
	return buf.Data()
}

func byteArrayToBufferData(arr [][]byte) []byte {
	if len(arr) == 0 {
		return nil
	}
	buf := buffer.NewBuffer(0, nil)
	for _, data := range arr {
		buf.Push(data)
	}
	return buf.Data()
}

func NewDchatMessage(tm uint8, msgId, magic uint32, address, devid string, buf *buffer.Buffer) *DchatMessage {
	return &DchatMessage{
		Type:    tm,
		MsgId:   msgId,
		Magic:   magic,
		Buffer:  buf,
		address: address,
		devid:   devid,
	}
}

func DchatMessageFromBuffer(buf *buffer.Buffer) *DchatMessage {
	if buf.Len() < 9 {
		return nil
	}

	tm := buf.PullU8()
	msgId := buf.PullU32()
	magic := buf.PullU32()
	return NewDchatMessage(tm, msgId, magic, "", "", buf.Buffer())
}

func (msg *DchatMessage) String() string {
	return fmt.Sprintf("Type(%d), MsgId(%d), Magic(%d), Address(%s), DevId(%s)", msg.Type, msg.MsgId, msg.Magic, msg.address, msg.devid)
}

func (msg *DchatMessage) GetBuffer() *buffer.Buffer {
	if msg.isPushed {
		return msg.Buffer
	}
	msg.isPushed = true
	if msg.Buffer == nil {
		msg.Buffer = buffer.NewBuffer(0, nil)
	}

	msg.Buffer.PutU32(msg.Magic)
	msg.Buffer.PutU32(msg.MsgId)
	msg.Buffer.PutU8(msg.Type)
	return msg.Buffer
}

func NewDchatDataMessage(tm messageType, buf *buffer.Buffer) *DchatDataMessage {
	return &DchatDataMessage{
		Type:   tm,
		Buffer: buf.Buffer(),
	}
}

func DchatDataMessageFromBuffer(buf *buffer.Buffer) *DchatDataMessage {
	if buf.Len() < 1 {
		return nil
	}

	tm := messageType(buf.PullU8())
	return NewDchatDataMessage(tm, buf)
}

func (msg *DchatDataMessage) String() string {
	return fmt.Sprintf("Type(%s)", msg.Type)
}

func (msg *DchatDataMessage) GetBuffer() *buffer.Buffer {
	if msg.isPushed {
		return msg.Buffer
	}
	msg.isPushed = true
	if msg.Buffer == nil {
		msg.Buffer = buffer.NewBuffer(0, nil)
	}
	msg.Buffer.PutU8(uint8(msg.Type))
	return msg.Buffer
}
