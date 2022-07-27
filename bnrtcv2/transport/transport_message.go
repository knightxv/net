package transport

import (
	"errors"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"

	"github.com/guabee/bnrtc/buffer"
)

const MessageVersion = 0x02000004
const MessageExpiredInterval = 30 * time.Second

type MessageType uint8

const (
	MessageCtrl MessageType = iota
	MessageData
	MessageBroadcast
)

/*
* MessageCtrl/MessageData
* ---------------------------------------
* |   u32   |  u8   | deviceId |  bytes |
* ---------------------------------------
* | version | type  | targetId |  data  |
* ---------------------------------------
*
* MessageBroadcast
* -------------------------------------------------------
* |   u32   |  u8   | bytes |  u32   | deviceId | bytes |
* -------------------------------------------------------
* | version | type  | extra | digest | sourceId |  data |
* -------------------------------------------------------
*
**/

var (
	MessageHeaderSizeMin       uint32 = 5 // version(4) + msgType(1)
	DeviceIdLen                       = buffer.DataLenSize(deviceid.EmptyDeviceId)
	MessageHeaderSize                 = MessageHeaderSizeMin + DeviceIdLen     // msgType(1)
	BroadcastMessageHeaderSize        = MessageHeaderSizeMin + 4 + DeviceIdLen // msgType(1) + extra(0) + digest(4) + deviceId(n)
	VersionOffset              uint32 = 0
	TypeOffset                 uint32 = 4
)

type Message struct {
	Version     uint32
	Mt          MessageType
	IsBroadcast bool
	Target      deviceid.DeviceId
	Sender      *TransportAddress
	Buffer      *buffer.Buffer
	Timestamp   time.Time
}

type BroatcastMessage struct {
	Version   uint32
	Mt        MessageType
	Extra     []byte
	Digest    uint32
	Source    deviceid.DeviceId
	Sender    *TransportAddress
	Buffer    *buffer.Buffer
	Timestamp time.Time
}

func NewMessage(sender *TransportAddress, target deviceid.DeviceId, buf *buffer.Buffer) *Message {
	return &Message{
		Sender:    sender,
		Target:    target,
		Timestamp: time.Now(),
		Buffer:    buf,
	}
}

func (m *Message) IsExpired() bool {
	return time.Since(m.Timestamp) > MessageExpiredInterval
}

func (m *Message) FromBuffer(buf *buffer.Buffer) (err error) {
	if buf.Len() < uint32(MessageHeaderSize) {
		return errors.New("invalid message header")
	}
	m.Version = buf.PullU32()
	m.Mt = MessageType(buf.PullU8())
	m.Target, err = buf.PullWithError()
	if err != nil {
		return err
	}
	m.Buffer = buf.Buffer()
	m.Timestamp = time.Now()
	return nil
}

func (m *Message) GetBuffer() *buffer.Buffer {
	return m.Buffer
}

func (m *Message) toBuffer() *buffer.Buffer {
	return m.Buffer.Put(m.Target).PutU8(uint8(m.Mt)).PutU32(m.Version)
}

func (m *Message) ToData() []byte {
	return m.toBuffer().Data()
}

func NewBroadcastMessage(sender *TransportAddress) *BroatcastMessage {
	return &BroatcastMessage{
		Sender:    sender,
		Timestamp: time.Now(),
	}
}

func (m *BroatcastMessage) IsExpired() bool {
	return time.Since(m.Timestamp) > MessageExpiredInterval
}

func (m *BroatcastMessage) FromBuffer(buf *buffer.Buffer) (err error) {
	if buf.Len() < uint32(BroadcastMessageHeaderSize) {
		return errors.New("invalid message header")
	}
	m.Version = buf.PullU32()
	m.Mt = MessageType(buf.PullU8())
	extra, err := buf.PullWithError()
	if err != nil {
		return err
	}
	m.Extra = append([]byte(nil), extra...)
	m.Digest = buf.PullU32()
	source, err := buf.PullWithError()
	if err != nil {
		return err
	}
	m.Source = append([]byte(nil), source...)
	m.Buffer = buf.Buffer()
	m.Timestamp = time.Now()
	return nil
}

func (m *BroatcastMessage) Clone() *BroatcastMessage {
	return &BroatcastMessage{
		Version:   m.Version,
		Mt:        m.Mt,
		Extra:     append([]byte(nil), m.Extra...),
		Digest:    m.Digest,
		Source:    m.Source,
		Sender:    m.Sender,
		Buffer:    m.Buffer.Clone(),
		Timestamp: m.Timestamp,
	}
}

func (m *BroatcastMessage) GetBuffer() *buffer.Buffer {
	return m.Buffer
}

func (m *BroatcastMessage) toBuffer() *buffer.Buffer {
	return m.Buffer.Put(m.Source).PutU32(m.Digest).Put(m.Extra).PutU8(uint8(m.Mt)).PutU32(m.Version)
}

func (m *BroatcastMessage) ToData() []byte {
	return m.toBuffer().Data()
}

func (m *BroatcastMessage) ToMessage(mt MessageType) *Message {
	return &Message{
		Version:     m.Version,
		Mt:          mt,
		IsBroadcast: true,
		Target:      deviceid.EmptyDeviceId,
		Sender:      m.Sender,
		Buffer:      m.Buffer.Clone(),
		Timestamp:   m.Timestamp,
	}
}
