package backpressure

import (
	"fmt"

	"github.com/guabee/bnrtc/buffer"
)

type xMessageType byte

const (
	MT_UserData   xMessageType = 0
	MT_BPInfoReq  xMessageType = 1
	MT_BPInfoResp xMessageType = 2
	MT_BPClose    xMessageType = 3
)

type BPFrame struct {
	MsgType xMessageType
	Buf     *buffer.Buffer
}

type PayloadBPInfo struct {
	CurrentId     uint64 // current message ID being handled
	SizeAvailable uint32 // the size available
	Capacity      uint32 // capacity
}
type PayloadUserData struct {
	Id  uint64 // message ID，self accumulated，rolled over at ^uint32(0)
	Buf *buffer.Buffer
}

/*

Basically the message is consists of a sequence of HLV unit (see hlv.go),

little endian is used

BPFrame:

each field will be Encoded with HLV unit

+---------+---------+
|   Type  | Payload |
+---------+---------+

Data structure when type = MT_UserData:
+-----------+-----------+
|     Id    |    Data   |
+-----------+-----------+


Data structure when type = MT_BPInfoReq:  (None)

Data structure when type = MT_BPInfoResp:
+-----------+-----------+------------+
|   CurId   | SizeAvail |  Capacity  |
+-----------+-----------+------------+

*/

func (f *BPFrame) Encode() *buffer.Buffer {
	return EncodeBPFrame(f)
}

func (v *PayloadBPInfo) Encode() *buffer.Buffer {
	return EncodePayloadBPInfo(v)
}
func (v *PayloadUserData) Encode() *buffer.Buffer {
	return EncodePayloadUserData(v)
}

func EncodeBPFrame(v *BPFrame) *buffer.Buffer {
	v.Buf.PutU8(uint8(v.MsgType))
	return v.Buf
}

func DecodeBPFrame(buf *buffer.Buffer) (*BPFrame, error) {
	if buf.Len() < 1 {
		return nil, fmt.Errorf("invalid length")
	}
	mt := xMessageType(buf.PullU8())

	return &BPFrame{MsgType: mt, Buf: buf}, nil
}

func EncodePayloadBPInfo(v *PayloadBPInfo) *buffer.Buffer {
	buf := buffer.NewBuffer(0, nil)
	buf.PutU32(v.Capacity)
	buf.PutU32(v.SizeAvailable)
	buf.PutU64(v.CurrentId)

	return buf
}
func DecodePayloadBPInfo(buf *buffer.Buffer) (*PayloadBPInfo, error) {
	if buf.Len() != 16 {
		return nil, fmt.Errorf("invalid length")
	}
	currentId := buf.PullU64()
	sizeAvailable := buf.PullU32()
	capacity := buf.PullU32()
	return &PayloadBPInfo{CurrentId: currentId, SizeAvailable: sizeAvailable, Capacity: capacity}, nil
}

func EncodePayloadUserData(v *PayloadUserData) *buffer.Buffer {
	v.Buf.PutU64(v.Id)
	return v.Buf
}

func DecodePayloadUserData(buf *buffer.Buffer) (*PayloadUserData, error) {
	if buf.Len() < 8 {
		return nil, fmt.Errorf("invalid length")
	}

	id := buf.PullU64()
	return &PayloadUserData{Id: id, Buf: buf}, nil
}
