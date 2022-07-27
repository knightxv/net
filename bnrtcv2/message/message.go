package message

import (
	"github.com/guabee/bnrtc/buffer"
)

type IMessageHook interface {
	Send(msg *Message) error
	Recv(msg *Message) error
}

type MessageHandler func(msg *Message)

type MessageType = uint8

type Message struct {
	Dport    string
	SrcAddr  string
	DstAddr  string
	SrcDevId string
	DstDevId string
	Sync     bool
	Buffer   *buffer.Buffer
}

func NewMessage(dport, srcAddr, dstAddr, srcDevid, dstDevId string, sync bool, buffer *buffer.Buffer) *Message {
	return &Message{SrcAddr: srcAddr, DstAddr: dstAddr, Dport: dport, SrcDevId: srcDevid, DstDevId: dstDevId, Sync: sync, Buffer: buffer}
}

func FromBuffer(buf *buffer.Buffer) (msg *Message) {
	errorBuffer := buffer.NewBufferWithError(buf)
	dport := errorBuffer.PullString()
	srcAddr := errorBuffer.PullString()
	dstAddr := errorBuffer.PullString()
	srcDevid := errorBuffer.PullString()
	dstDevid := errorBuffer.PullString()
	sync := errorBuffer.PullBool()

	if errorBuffer.IsError() {
		return nil
	}
	msg = NewMessage(dport, srcAddr, dstAddr, srcDevid, dstDevid, sync, buf.Buffer())
	return
}

func FromBytes(data []byte) *Message {
	return FromBuffer(buffer.FromBytes(data))
}

func Clone(msg *Message) *Message {
	return NewMessage(msg.Dport, msg.SrcAddr, msg.DstAddr, msg.SrcDevId, msg.DstDevId, msg.Sync, msg.Buffer.Clone())
}

func (msg *Message) Clone() *Message {
	return Clone(msg)
}

func Copy(msg *Message) *Message {
	return NewMessage(msg.Dport, msg.SrcAddr, msg.DstAddr, msg.SrcDevId, msg.DstDevId, msg.Sync, msg.Buffer.Copy())
}

func (msg *Message) Copy() *Message {
	return Copy(msg)
}

func (msg *Message) GetBuffer() *buffer.Buffer {
	msg.Buffer.PutBool(msg.Sync)
	msg.Buffer.PutString(msg.DstDevId)
	msg.Buffer.PutString(msg.SrcDevId)
	msg.Buffer.PutString(msg.DstAddr)
	msg.Buffer.PutString(msg.SrcAddr)
	msg.Buffer.PutString(msg.Dport)
	return msg.Buffer
}

func (msg *Message) GetBytes() []byte {
	return msg.GetBuffer().Data()
}
