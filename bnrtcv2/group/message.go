package group

import (
	"fmt"

	"github.com/guabee/bnrtc/buffer"
)

type messageType uint8

const groupMessageDport = "_group_"

func getGroupMessagePort() string {
	return groupMessageDport
}

func IsGroupMessagePort(port string) bool {
	return groupMessageDport == port
}

const (
	messageType_ping messageType = iota
	messageType_pong
	messageType_memberInfo
	messageType_memberInfo_ack
	messageType_groupInfo
	messageType_userdata
	messageType_close
)

func (t messageType) String() string {
	switch t {
	case messageType_ping:
		return "ping"
	case messageType_pong:
		return "pong"
	case messageType_memberInfo:
		return "memberInfo"
	case messageType_memberInfo_ack:
		return "memberInfoAck"
	case messageType_groupInfo:
		return "groupInfo"
	case messageType_userdata:
		return "userdata"
	case messageType_close:
		return "close"
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}

type GroupMessage struct {
	Type     messageType
	Seq      uint64
	Id       GroupId
	MemberId MemberId
	Buffer   *buffer.Buffer
}

func (manager *GroupManager) newGroupMessage(tm messageType, id GroupId, buf *buffer.Buffer) *GroupMessage {
	manager.msgSeq++
	return &GroupMessage{
		Type:     tm,
		Seq:      manager.msgSeq,
		Id:       id,
		MemberId: manager.getLocalId(),
		Buffer:   buf,
	}
}

func GroupMessageFromBuffer(buf *buffer.Buffer) *GroupMessage {
	if buf.Len() < 9 {
		return nil
	}

	tm := messageType(buf.PullU8())
	seq := buf.PullU64()
	id := buf.PullString()
	memberId := buf.Pull()

	msg := &GroupMessage{
		Type:     tm,
		Seq:      seq,
		Id:       id,
		MemberId: memberId,
		Buffer:   buf.Buffer(),
	}

	return msg
}

func (msg *GroupMessage) GetBuffer() *buffer.Buffer {
	msg.Buffer.Put([]byte(msg.MemberId))
	msg.Buffer.PutString(msg.Id)
	msg.Buffer.PutU64(msg.Seq)
	msg.Buffer.PutU8(uint8(msg.Type))
	return msg.Buffer
}
