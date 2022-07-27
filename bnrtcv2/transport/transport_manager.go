package transport

import (
	"fmt"
	"net"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/util"

	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/fragmentation"
)

const (
	MaxMessageSize            = 1200
	MaxFragmentChunkSize      = 1000
	MaxFragmentSize           = 100 * 1024 * 1024 // 100M
	MessageExpiredTimeSeconds = 10
	MessgaeHistorySize        = 1000
)

type TransportType uint8

const TransportTypeNum = 2
const (
	DirectType TransportType = iota
	ForwardType
	DirectAndForwardType
)

type TransportMessageHandler func(address *TransportAddress, buf *buffer.Buffer) error

type ITransport interface {
	Name() string
	Type() TransportType
	Send(addr *TransportAddress, buf *buffer.Buffer, ignoreAck bool) error
	OnMessage(handler TransportMessageHandler)
	IsTargetReachable(addr *TransportAddress) bool
	SetDeviceId(id deviceid.DeviceId)
	GetExternalIp(addr *TransportAddress) (net.IP, error)
	SetExternalIp(ip net.IP)
	Close()
}

type ITransportManager interface {
	AddTransport(transport ITransport)
	SendCtrlMsg(addr *TransportAddress, buf *buffer.Buffer) error
	SendDataMsg(addr *TransportAddress, buf *buffer.Buffer) error
	SendBroadcastMsg(addr *TransportAddress, buf *buffer.Buffer, extra []byte) error
	BroadcastDataMsg(addr *TransportAddress, buf *buffer.Buffer, extra []byte) error
	GetCtrlMsgChan() <-chan *Message
	GetDataMsgChan() <-chan *Message
	GetBroadcastDataMsgChan() <-chan *BroatcastMessage
	SetDeviceId(id deviceid.DeviceId)
	GetExternalIp(addr *TransportAddress) (net.IP, error)
	IsTargetReachable(addr *TransportAddress) bool
}

type MessageHandler func(msg *Message)

type TransportManager struct {
	DevId                deviceid.DeviceId
	PubSub               *util.PubSub
	ExternalIp           net.IP
	transports           [TransportTypeNum][]ITransport
	frag                 *fragmentation.Fragmentation
	stopChan             chan struct{}
	msgHistoryMap        *util.SafeLinkMap
	addressTransportMap  *util.SafeMap // map[string]ITransport
	ctrlMsgChan          chan *Message
	dataMsgChan          chan *Message
	broadcastDataMsgChan chan *BroatcastMessage
	forwardMsgChan       chan *Message
	logger               *log.Entry
}

type TransportManagerOption func(*TransportManager)

func WithDevidOption(id deviceid.DeviceId) TransportManagerOption {
	return func(t *TransportManager) {
		t.DevId = id
	}
}

func WithPubSubOption(pubsub *util.PubSub) TransportManagerOption {
	return func(t *TransportManager) {
		t.PubSub = pubsub
	}
}

func WithExternalIp(ip string) TransportManagerOption {
	return func(m *TransportManager) {
		m.ExternalIp = net.ParseIP(ip)
	}
}

func NewTransportManager(options ...TransportManagerOption) *TransportManager {
	t := &TransportManager{
		PubSub: util.DefaultPubSub(),
		frag: fragmentation.NewFragmentation(&fragmentation.FragmentOption{
			MaxFragmentSize:      MaxFragmentSize,
			MaxFragmentChunkSize: MaxFragmentChunkSize,
			ExpiredTimeSeconds:   MessageExpiredTimeSeconds,
		}),
		stopChan:             make(chan struct{}),
		msgHistoryMap:        util.NewSafeLimitLinkMap(MessgaeHistorySize),
		addressTransportMap:  util.NewSafeMap(),
		ctrlMsgChan:          make(chan *Message, 1024),
		dataMsgChan:          make(chan *Message, 1024),
		broadcastDataMsgChan: make(chan *BroatcastMessage, 1024),
		forwardMsgChan:       make(chan *Message, 1024),
		logger:               log.NewLoggerEntry("transport-manager"),
	}

	for _, option := range options {
		option(t)
	}

	_, err := t.PubSub.SubscribeFunc("transport-manager", rpc.DeviceIdChangeTopic, func(msg util.PubSubMsgType) {
		devId := msg.(deviceid.DeviceId)
		t.SetDeviceId(devId)
	})
	if err != nil {
		t.logger.Errorf("subscribe device id change failed, error=%s", err)
	}

	t.run()
	return t
}

func (t *TransportManager) addDigestToHistory(digest uint32, sender deviceid.DeviceId) uint32 {
	t.msgHistoryMap.Set(digest, sender)
	return digest
}

func (t *TransportManager) getDigestSender(digest uint32) deviceid.DeviceId {
	id, found := t.msgHistoryMap.Get(digest)
	if found {
		return id.(deviceid.DeviceId)
	} else {
		return deviceid.EmptyDeviceId
	}
}

func (t *TransportManager) doSend(transport ITransport, mt MessageType, addr *TransportAddress, fragmentDatas []*buffer.Buffer) (err error) {
	for _, fragmentData := range fragmentDatas {
		err = transport.Send(addr, fragmentData, mt == MessageCtrl)
		if err != nil {
			t.logger.Debugf("send direct to %s failed, error=%s", addr, err)
			return err
		}
	}
	return nil
}

func (t *TransportManager) packetize(mt MessageType, addr *TransportAddress, buf *buffer.Buffer, extra []byte) []*buffer.Buffer {
	fragmentDatas := t.frag.Packetize(buf.Data())
	fragmentBuffers := make([]*buffer.Buffer, len(fragmentDatas))
	for i, fragmentData := range fragmentDatas {
		buf := fragmentData.GetBuffer()
		if mt == MessageBroadcast {
			fragmentBuffers[i] = (&BroatcastMessage{Version: MessageVersion, Mt: mt, Source: t.DevId, Digest: t.addDigestToHistory(fragmentData.Digest, t.DevId), Extra: extra, Buffer: buf}).toBuffer()
		} else {
			fragmentBuffers[i] = (&Message{Version: MessageVersion, Mt: mt, Target: addr.DeviceId, Buffer: buf}).toBuffer()
		}
	}
	return fragmentBuffers
}

func (t *TransportManager) send(mt MessageType, addr *TransportAddress, fragmentBuffers []*buffer.Buffer) (err error) {
	transport, isCached := t.getTransport(DirectType, addr, nil)
	if transport == nil {
		return fmt.Errorf("not direct transport for %s", addr)
	}

	err = t.doSend(transport, mt, addr, fragmentBuffers)
	if err != nil {
		t.addressTransportMap.Delete(addr.String())
		if isCached {
			// try to get new transport when the cached one failed
			newTransport, _ := t.getTransport(DirectType, addr, func(tp ITransport) bool {
				return tp != transport
			})
			if newTransport != nil {
				return t.doSend(newTransport, mt, addr, fragmentBuffers)
			}
		}
	}

	return
}

func (t *TransportManager) AddTransport(transport ITransport) {
	tt := transport.Type()
	if tt == DirectType || tt == DirectAndForwardType {
		t.transports[DirectType] = append(t.transports[DirectType], transport)
	}
	if tt == ForwardType || tt == DirectAndForwardType {
		t.transports[ForwardType] = append(t.transports[ForwardType], transport)
	}
	transport.SetDeviceId(t.DevId)
	if t.ExternalIp != nil {
		transport.SetExternalIp(t.ExternalIp)
	}
	transport.OnMessage(t.onMessage)
}

func (t *TransportManager) SendCtrlMsg(addr *TransportAddress, buf *buffer.Buffer) error {
	if addr == nil {
		return fmt.Errorf("nil addr")
	}
	fragmentBuffers := t.packetize(MessageCtrl, addr, buf, nil)
	return t.send(MessageCtrl, addr, fragmentBuffers)
}

func (t *TransportManager) SendDataMsg(addr *TransportAddress, buf *buffer.Buffer) error {
	if addr == nil {
		return fmt.Errorf("nil addr")
	}
	fragmentBuffers := t.packetize(MessageData, addr, buf, nil)
	return t.send(MessageData, addr, fragmentBuffers)
}

func (t *TransportManager) SendBroadcastMsg(addr *TransportAddress, buf *buffer.Buffer, extra []byte) error {
	if addr == nil {
		return fmt.Errorf("nil addr")
	}
	fragmentBuffers := t.packetize(MessageBroadcast, addr, buf, extra)
	return t.send(MessageBroadcast, addr, fragmentBuffers)
}

func (t *TransportManager) BroadcastDataMsg(addr *TransportAddress, buf *buffer.Buffer, extra []byte) error {
	if addr == nil {
		return fmt.Errorf("nil addr")
	}
	// update extra info
	version := buf.PullU32() // version
	mt := buf.PullU8()       // msgType
	_ = buf.Pull()           // extra
	digest := buf.PeekU32(0) // digest
	source := buf.Peek(4)    // source
	sender := t.getDigestSender(digest)
	buf.Put(extra)
	buf.PutU8(mt)
	buf.PutU32(version)
	// ignore sender and source
	if addr.DeviceId.IsValid() && (addr.DeviceId.Equal(sender) || addr.DeviceId.Equal(source)) {
		return nil
	}

	return t.send(MessageType(mt), addr, []*buffer.Buffer{buf})
}

func (t *TransportManager) GetCtrlMsgChan() <-chan *Message {
	return t.ctrlMsgChan
}

func (t *TransportManager) GetDataMsgChan() <-chan *Message {
	return t.dataMsgChan
}

func (t *TransportManager) GetBroadcastDataMsgChan() <-chan *BroatcastMessage {
	return t.broadcastDataMsgChan
}

func (t *TransportManager) SetDeviceId(id deviceid.DeviceId) {
	if !t.DevId.Equal(id) {
		t.DevId = id
		t.logger = t.logger.WithField("deviceId", id)
		for _, transport := range t.transports[DirectType] {
			transport.SetDeviceId(id)
		}
		for _, transport := range t.transports[ForwardType] {
			transport.SetDeviceId(id)
		}
	}
}

func (t *TransportManager) GetExternalIp(addr *TransportAddress) (net.IP, error) {
	if t.ExternalIp != nil {
		return t.ExternalIp, nil
	}

	if addr == nil {
		return nil, fmt.Errorf("nil addr")
	}
	for _, transport := range t.transports[DirectType] {
		ip, err := transport.GetExternalIp(addr)
		if err == nil {
			t.SetExternalIp(ip)
			return ip, nil
		}
	}

	return nil, fmt.Errorf("not found external ip from %s", addr)
}

func (t *TransportManager) SetExternalIp(ip net.IP) {
	if ip == nil {
		return
	}
	for _, transport := range t.transports[DirectType] {
		transport.SetExternalIp(ip)
	}
}

func (t *TransportManager) IsTargetReachable(addr *TransportAddress) bool {
	if addr == nil {
		return false
	}
	transport, _ := t.getTransport(DirectType, addr, nil)
	return transport != nil
}

func (t *TransportManager) getTransport(tt TransportType, addr *TransportAddress, filter func(transport ITransport) bool) (ITransport, bool) {
	_transport, found := t.addressTransportMap.Get(addr.String())
	if found {
		transport := _transport.(ITransport)
		if filter == nil || filter(transport) {
			return transport, true
		}
	}

	for _, transport := range t.transports[tt] {
		if transport.IsTargetReachable(addr) {
			if filter == nil || filter(transport) {
				t.addressTransportMap.Set(addr.String(), transport)
				return transport, false
			}
		}
	}

	return nil, false
}

func (t *TransportManager) forwardMessage(buf *buffer.Buffer, addr *TransportAddress) error {
	// @todo limit the forward message count/bytes
	transport, isCached := t.getTransport(ForwardType, addr, nil)
	if transport == nil {
		return fmt.Errorf("no forward transport for %s found", addr)
	}
	err := transport.Send(addr, buf, true)
	if err != nil {
		t.addressTransportMap.Delete(addr.String())
		if isCached {
			// try to get new transport when the cached one failed
			newTransport, _ := t.getTransport(ForwardType, addr, func(tp ITransport) bool {
				return tp != transport
			})
			if newTransport != nil {
				return newTransport.Send(addr, buf, true)
			}
		}
	}

	return err
}

func (t *TransportManager) run() {
	go t.forwardMessageWorker()
}

func (t *TransportManager) forwardMessageWorker() {
	for msg := range t.forwardMsgChan {
		if msg.IsExpired() {
			continue
		}

		err := t.forwardMessage(msg.Buffer, NewAddress(msg.Target, nil))
		if err != nil {
			t.logger.Debugf("Received message to %s, not to me %s ,forward it failed error=%s", msg.Target, t.DevId, err)
		}
	}
}

func (t *TransportManager) handleMessage(msg *Message) {
	packet := t.frag.UnPacketize(msg.Buffer.Data())
	if packet == nil {
		return
	}

	msg.Buffer = buffer.FromBytes(packet)
	switch msg.Mt {
	case MessageCtrl:
		select {
		case t.ctrlMsgChan <- msg:
		default:
		}
	case MessageData:
		select {
		case t.dataMsgChan <- msg:
		default:
		}
	default:
		t.logger.Debugf("Unknown message type %d", msg.Mt)
	}
}

func (t *TransportManager) handleBroadcastMessage(msg *BroatcastMessage, data *Message) {
	// enquene broadcast message and then handle it as a data message
	select {
	case t.broadcastDataMsgChan <- msg:
	default:
	}
	t.handleMessage(data)
}

func (t *TransportManager) onMessage(addr *TransportAddress, buf *buffer.Buffer) error {
	if buf.Len() < MessageHeaderSizeMin {
		return fmt.Errorf("invalid message header")
	}

	version := buf.PeekU32(VersionOffset)
	if version != MessageVersion {
		t.logger.Debugf("Unknown message version %d <=> %d", version, MessageVersion)
		return fmt.Errorf("Unknown message version")
	}

	mt := MessageType(buf.PeekU8(TypeOffset))
	switch mt {
	case MessageCtrl:
		fallthrough
	case MessageData:
		msg := NewMessage(addr, nil, nil)
		err := msg.FromBuffer(buf.Clone())
		if err != nil {
			t.logger.Errorf("Failed to parse message from %s: %s", addr, err)
			return err
		}
		if !msg.Target.IsEmpty() && !t.DevId.SameNodeId(msg.Target) {
			if !msg.Target.IsValid() {
				return fmt.Errorf("invalid target device id")
			}
			select {
			case t.forwardMsgChan <- NewMessage(addr, msg.Target, buf.Clone()):
			default:
			}
		} else {
			t.handleMessage(msg)
		}
	case MessageBroadcast:
		msg := NewBroadcastMessage(addr)
		err := msg.FromBuffer(buf.Clone())
		if err != nil {
			t.logger.Errorf("Failed to parse message from %s: %s", addr, err)
			return err
		}

		if t.msgHistoryMap.Has(msg.Digest) {
			return nil
		}

		bMsg := msg.Clone()
		bMsg.Buffer = buf
		t.addDigestToHistory(msg.Digest, msg.Sender.DeviceId)
		t.handleBroadcastMessage(bMsg, msg.ToMessage(MessageData))
	}
	return nil
}
