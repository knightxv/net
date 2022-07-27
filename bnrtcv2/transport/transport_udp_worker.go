package transport

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
)

type UdpTransportType uint8

const (
	UdpTransportPing UdpTransportType = iota
	UdpTransportPong
	UdpTransportData
	UdpTransportAck
	UdpVersionMagic uint32 = 0x18273645
)

var (
	/*
	* | magic  |  type  |  msgid | needAck |   devid   |
	* | uint32 |  uint8 | uint832|  uint8  |  devidLen |
	 */
	UdpHeaderMagicSize   = 4
	UdpHeaderTypeSize    = 1
	UdpHeaderMsgIdSize   = 4
	UdpHeaderNeedAckSize = 1
	UdpHeaderDevIdSize   = int(buffer.DataLenSize(deviceid.ZeroDeviceId))
	UdpHeaderAckMsgSize  = UdpHeaderMagicSize + UdpHeaderTypeSize + UdpHeaderMsgIdSize
	UdpHeaderDataMsgSize = UdpHeaderMagicSize + UdpHeaderTypeSize + UdpHeaderMsgIdSize + UdpHeaderNeedAckSize + UdpHeaderDevIdSize
	UdpHeaderMsgSizeMin  = UdpHeaderMagicSize + UdpHeaderTypeSize + UdpHeaderMsgIdSize
)

func (t UdpTransportType) String() string {
	switch t {
	case UdpTransportData:
		return "data"
	case UdpTransportAck:
		return "ack"
	case UdpTransportPing:
		return "ping"
	case UdpTransportPong:
		return "pong"
	default:
		return "unknown"
	}
}

const UdpTransportAckTimeoutDefault = 10 * time.Second
const UDPMessageRingSize = 1024
const UDPAddressExpireTime = 30 * time.Second

type UDPMessage struct {
	Address *TransportAddress
	Buf     *buffer.Buffer
	Origin  *[]byte
	NeedAck bool
	MsgId   uint32
}

type addressItem struct {
	id      string
	addr    *net.UDPAddr
	expired time.Time
}

type UDPEvent = uint8

const (
	UDPEventRequestResolved = iota
	UDPEventRequestTimeout
	UDPEventRequestFailed
	UDPEventPingReqResolved
	UDPEventPingReqTimeout
	UDPEventPingReqFailed
)

type UdpTransportWorker struct {
	DevId          deviceid.DeviceId
	sendConn       *net.UDPConn
	recvConn       *net.UDPConn
	msgHandler     TransportMessageHandler
	msgChan        chan *UDPMessage
	msgPool        sync.Pool
	stopChan       chan struct{}
	addressList    *util.SafeLinkMap
	needAck        bool
	isClosed       bool
	ackTimeout     time.Duration
	waitPongIds    *util.SafeMap
	reqrespManager *util.ReqRespManager
	mutex          sync.RWMutex
	logger         *log.Entry
}

func newUdpTransportWorker(sendConn, recvConn *net.UDPConn, logger *log.Entry, ack bool, ackTimeout time.Duration) *UdpTransportWorker {
	t := &UdpTransportWorker{
		sendConn: sendConn,
		recvConn: recvConn,
		msgChan:  make(chan *UDPMessage, 1024*10),
		msgPool: sync.Pool{
			New: func() interface{} {
				d := make([]byte, MaxMessageSize)
				return &d
			},
		},
		stopChan:    make(chan struct{}),
		addressList: util.NewSafeLinkMap(),
		needAck:     ack,
		ackTimeout:  ackTimeout,
		waitPongIds: util.NewSafeMap(),
		logger:      logger,
	}

	if ackTimeout <= 0 {
		t.ackTimeout = UdpTransportAckTimeoutDefault
	}
	t.reqrespManager = util.NewReqRespManager(util.WithReqRespTimeout(t.ackTimeout))
	t.run()
	return t
}

func (t *UdpTransportWorker) resolvePing(ip net.IP) {
	items := t.waitPongIds.Items()
	t.waitPongIds.Clear()
	for _, _item := range items {
		id := _item.(uint32)
		t.reqrespManager.ResolveReq(id, ip)
	}
}

func (t *UdpTransportWorker) sendRequestWithData(tt UdpTransportType, needAck bool, addr *TransportAddress, buf *buffer.Buffer) (data interface{}, err error) {
	t.logger.Debugf("send request %s to %s, needAck: %t", tt, addr, needAck)
	udpAddr := addr.Addr
	if t.IsConnected(addr.DeviceId) && addr.Addr.Port == 0 {
		addrItem, found := t.addressList.Get(addr.DeviceId.String())
		if found {
			udpAddr = *addrItem.(*addressItem).addr
		}
	}

	msgId := uint32(0)
	var req *util.Request = nil
	if needAck {
		req = t.reqrespManager.GetReq()
		msgId = req.Id()
		if tt == UdpTransportPing {
			t.waitPongIds.Set(msgId, msgId)
		}
	}

	// | magic | type  |  msgid | needAck  |  devid  |
	buf.Put(t.DevId)
	buf.PutBool(needAck)
	buf.PutU32(msgId)
	buf.PutU8(uint8(tt))
	buf.PutU32(UdpVersionMagic)

	_, err = t.sendConn.WriteToUDP(buf.Data(), &udpAddr)
	if err != nil {
		t.logger.Errorf("write failed to %s, error=%s", &udpAddr, err)
		if req != nil {
			if tt == UdpTransportPing {
				t.waitPongIds.Delete(msgId)
			}
			t.reqrespManager.CancelReq(msgId)
		}
		return nil, err
	}

	if req != nil {
		resp := req.WaitResult()
		if resp != nil && resp.IsSuccess() {
			data = resp.Data()
			return data, nil
		} else {
			err = resp.Error()
		}
	}
	if err != nil {
		t.logger.Debugf("send request %s to %s failed, error=%s", tt, &udpAddr, err)
	}
	return nil, err
}

func (t *UdpTransportWorker) sendRequest(tt UdpTransportType, needAck bool, addr *TransportAddress, buf *buffer.Buffer) (err error) {
	_, err = t.sendRequestWithData(tt, needAck, addr, buf)
	return err
}

func (t *UdpTransportWorker) SendAck(msgId uint32, addr *TransportAddress) (err error) {
	// | magic | type  |  msgid |
	t.logger.Debugf("send ack %d to %s", msgId, addr)
	buf := buffer.NewBuffer(MaxMessageSize, nil)
	buf.PutU32(msgId)
	buf.PutU8(uint8(UdpTransportAck))
	buf.PutU32(UdpVersionMagic)

	_, err = t.sendConn.WriteToUDP(buf.Data(), &addr.Addr)
	if err != nil {
		t.logger.Errorf("write failed to %+v, error=%s", addr, err)
	}

	return err
}

func (t *UdpTransportWorker) Send(addr *TransportAddress, buf *buffer.Buffer, ignoreAck bool) (err error) {
	return t.sendRequest(UdpTransportData, (!ignoreAck) && t.needAck, addr, buf)
}

func (t *UdpTransportWorker) OnMessage(handler TransportMessageHandler) {
	t.msgHandler = handler
}

func (t *UdpTransportWorker) GetExternalIp(addr *TransportAddress) (net.IP, error) {
	data, err := t.sendRequestWithData(UdpTransportPing, true, addr, buffer.NewBuffer(0, nil))
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, errors.New("no response")
	}
	return data.(net.IP), nil
}

func (t *UdpTransportWorker) IsConnected(id deviceid.DeviceId) bool {
	if !id.IsValid() {
		return false
	}
	return t.addressList.Has(id.String())
}

func (t *UdpTransportWorker) SetDeviceId(id deviceid.DeviceId) {
	if !t.DevId.Equal(id) {
		t.DevId = id
		t.logger = t.logger.WithField("deviceId", id)
	}
}

func (t *UdpTransportWorker) Close() {
	t.mutex.Lock()
	close(t.stopChan)
	t.isClosed = true
	t.reqrespManager.Close()
	t.mutex.Unlock()
}

func (t *UdpTransportWorker) IsClosed() bool {
	t.mutex.RLock()
	isClosed := t.isClosed
	t.mutex.RUnlock()
	return isClosed
}

func (t *UdpTransportWorker) onMessage(msg *UDPMessage) {
	if t.msgHandler != nil {
		buf := msg.Buf.Copy()
		t.msgPool.Put(msg.Origin) // @fixme without copy but Put it when the message is handled
		msg.Buf = nil             // clear it
		err := t.msgHandler(msg.Address, buf)
		if err == nil && msg.NeedAck {
			_ = t.SendAck(msg.MsgId, msg.Address)
		}
	}
}

func (t *UdpTransportWorker) run() {
	t.reqrespManager.Start()
	go t.readMessage()
	go t.clearExpiredAddress()
	go t.handleMessage()
}

func (t *UdpTransportWorker) handleMessage() {
	for msg := range t.msgChan {
		if msg == nil {
			return
		}
		t.onMessage(msg)
	}
}

func (t *UdpTransportWorker) clearExpiredAddress() {
	timer := time.NewTicker(time.Second * 5)
	for {
		select {
		case <-timer.C:
			for _, _item := range t.addressList.Items() {
				item := _item.(*addressItem)
				if time.Now().After(item.expired) {
					t.addressList.Delete(item.addr)
				} else {
					break
				}
			}
		case <-t.stopChan:
			timer.Stop()
			return
		}
	}
}

// Read from UDP socket, writes slice of byte into channel.
func (t *UdpTransportWorker) readMessage() {
	var msgItem *[]byte = nil
	defer close(t.msgChan)

Exit:
	for {

		select {
		case <-t.stopChan:
			break Exit
		default:
		}

		if msgItem == nil {
			msgItem = t.msgPool.Get().(*[]byte)
		}
		recvData := *msgItem
		n, addr, err := t.recvConn.ReadFromUDP(recvData)
		if t.IsClosed() {
			break Exit
		}

		if err != nil {
			t.logger.Debugf("readResponse error:%s", err)
			continue
		}
		if n == MaxMessageSize || n < UdpHeaderMsgSizeMin {
			t.logger.Debugf("Warning. Received packet with len %d, some data may have been discarded", n)
			continue
		}
		// | magic | type  |  msgid | needAck  |  devid  |
		buf := buffer.FromBytes(recvData[:n])
		magic := buf.PullU32()
		if magic != UdpVersionMagic {
			continue
		}

		tt := UdpTransportType(buf.PullU8())
		msgId := buf.PullU32()
		t.logger.Debugf("recv request %s from %s", tt, addr)
		if tt == UdpTransportAck {
			if n < UdpHeaderAckMsgSize {
				continue
			}
			t.logger.Debugf("recv Ack id %d from %s", msgId, addr)
			t.reqrespManager.ResolveReq(msgId, nil)
		} else {
			if n < UdpHeaderDataMsgSize {
				continue
			}
			needAck := buf.PullBool()
			_devid, err := buf.PullWithSize(deviceid.DeviceIdLength)
			if err != nil {
				continue
			}

			devid := deviceid.DeviceId(_devid)
			t.logger.Debugf("recv request %s id %d from %s %s needAck %t", tt, msgId, devid, addr, needAck)
			if devid.IsValid() {
				idStr := devid.String()
				t.addressList.Set(idStr, &addressItem{id: idStr, addr: addr, expired: time.Now().Add(UDPAddressExpireTime)})
			}

			switch tt {
			case UdpTransportPing:
				err = t.sendRequest(UdpTransportPong, false, NewAddress(nil, addr), buffer.FromBytes(addr.IP))
				if err != nil {
					t.logger.Debugf("sendRequest error:%s", err)
				}
			case UdpTransportPong:
				if needAck {
					t.SendAck(msgId, NewAddress(nil, addr))
				}
				t.resolvePing(buf.DataCopy())
			case UdpTransportData:
				msg := &UDPMessage{
					Address: NewAddress(devid, addr),
					Buf:     buf,
					Origin:  msgItem,
					NeedAck: needAck,
					MsgId:   msgId,
				}
				select {
				case t.msgChan <- msg:
					msgItem = nil // msgitem data is now in channel, don't reuse it
				case <-t.stopChan:
					break Exit
				default:
					// enqueue failed, discard it
				}
			}
		}
	}

	if msgItem != nil {
		t.msgPool.Put(msgItem)
	}
}
