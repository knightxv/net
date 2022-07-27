package transport_test

import (
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
)

type PeerMock struct {
	devId     deviceid.DeviceId
	addr      *net.UDPAddr
	sendCnt   int
	recvCnt   int
	onMessage func(id deviceid.DeviceId, buf *buffer.Buffer)
}

var peerMockMap = make(map[string]*PeerMock)

func newPeerMock(devId deviceid.DeviceId, addr *net.UDPAddr) *PeerMock {
	peer := &PeerMock{
		devId: devId,
		addr:  addr,
	}

	peerMockMap[devId.String()] = peer
	return peer
}

func (p *PeerMock) Address() *transport.TransportAddress {
	return transport.NewAddress(p.devId, p.addr)
}

func (p *PeerMock) OnMessage(handler func(id deviceid.DeviceId, buf *buffer.Buffer)) {
	p.onMessage = handler
}

func (p *PeerMock) Send(peer *PeerMock, buf *buffer.Buffer) {
	p.sendCnt++
	peer.Recv(p.devId, buf)
}

func (p *PeerMock) Recv(source deviceid.DeviceId, buf *buffer.Buffer) {
	p.recvCnt++
	if p.onMessage != nil {
		p.onMessage(source, buf)
	}
}

func (p *PeerMock) SetDeviceId(id deviceid.DeviceId) {
	p.devId = id
}

type TransportMock struct {
	name        string
	local       *PeerMock
	tt          transport.TransportType
	peerMap     map[string]*PeerMock
	peerAddrMap map[string]*PeerMock
	onMessage   transport.TransportMessageHandler
}

func NewTransportMock(name string, local *PeerMock, tt transport.TransportType) *TransportMock {
	t := &TransportMock{
		name:        name,
		local:       local,
		tt:          tt,
		peerMap:     make(map[string]*PeerMock),
		peerAddrMap: make(map[string]*PeerMock),
	}
	t.local.OnMessage(func(id deviceid.DeviceId, buf *buffer.Buffer) {
		if t.onMessage != nil {
			t.onMessage(&transport.TransportAddress{
				DeviceId: id,
			}, buf)
		}
	})
	return t
}
func (t *TransportMock) AddPeer(peer *PeerMock) {
	t.peerMap[peer.devId.String()] = peer
	t.peerAddrMap[peer.addr.String()] = peer
}

func (t *TransportMock) Name() string                  { return t.name }
func (t *TransportMock) Type() transport.TransportType { return t.tt }
func (t *TransportMock) Send(addr *transport.TransportAddress, buf *buffer.Buffer, ignoreAck bool) error {
	peer, ok := t.peerMap[addr.DeviceId.String()]
	if !ok {
		peer, ok = t.peerAddrMap[addr.Addr.String()]
		if !ok {
			return errors.New("peer not found")
		}
	}

	t.local.Send(peer, buf)
	return nil
}

func (t *TransportMock) OnMessage(handler transport.TransportMessageHandler) {
	t.onMessage = handler
}

func (t *TransportMock) IsTargetReachable(addr *transport.TransportAddress) bool {
	_, ok := t.peerMap[addr.DeviceId.String()]
	if ok {
		return true
	}
	_, ok = t.peerAddrMap[addr.Addr.String()]
	return ok
}

func (t *TransportMock) SetDeviceId(id deviceid.DeviceId) {
	t.local.SetDeviceId(id)
}

func (m *TransportMock) GetExternalIp(addr *transport.TransportAddress) (net.IP, error) {
	return net.IPv4(1, 1, 1, 1), nil
}
func (m *TransportMock) SetExternalIp(ip net.IP) {}

func (m *TransportMock) Close() {}

type statis struct {
	sendCtrlCnt      int32
	sendDataCnt      int32
	recvCtrlCnt      int32
	recvDataCnt      int32
	recvBroadcastCnt int32
}

func (s *statis) record(isCtrl bool, isSend bool) {
	if isCtrl {
		if isSend {
			atomic.AddInt32(&s.sendCtrlCnt, 1)
		} else {
			atomic.AddInt32(&s.recvCtrlCnt, 1)
		}
	} else {
		if isSend {
			atomic.AddInt32(&s.sendDataCnt, 1)
		} else {
			atomic.AddInt32(&s.recvDataCnt, 1)
		}
	}
}

func (s *statis) recordBroadcast() {
	atomic.AddInt32(&s.recvBroadcastCnt, 1)
}

type TransportManagerContainer struct {
	devId   deviceid.DeviceId
	manager *transport.TransportManager
	statis  *statis
	peers   []*PeerMock
}

func newTransportManagerContainer(devid deviceid.DeviceId) *TransportManagerContainer {
	c := &TransportManagerContainer{
		devId:   devid,
		manager: transport.NewTransportManager(transport.WithPubSubOption(util.NewPubSub())),
		statis:  &statis{},
		peers:   make([]*PeerMock, 0),
	}
	go func() {
		for {
			select {
			case msg := <-c.manager.GetCtrlMsgChan():
				if msg != nil {
					c.statis.record(true, false)
				}
			case msg := <-c.manager.GetDataMsgChan():
				if msg != nil {
					if msg.IsBroadcast {
						c.statis.recordBroadcast()
					} else {
						c.statis.record(false, false)
					}
				}

			case msg := <-c.manager.GetBroadcastDataMsgChan():
				if msg != nil {
					for _, peer := range c.peers {
						_ = c.manager.BroadcastDataMsg(peer.Address(), msg.Buffer.Clone(), nil)
					}
				}
			}
		}
	}()
	c.manager.SetDeviceId(devid)

	return c
}

func (c *TransportManagerContainer) send(addr *transport.TransportAddress, buf *buffer.Buffer, isCtrl bool) error {
	var err error
	if isCtrl {
		err = c.manager.SendCtrlMsg(addr, buf)
	} else {
		err = c.manager.SendDataMsg(addr, buf)
	}
	if err == nil {
		c.statis.record(isCtrl, true)
	}

	return err
}

func (c *TransportManagerContainer) Send(addr *transport.TransportAddress, buf *buffer.Buffer, isCtrl bool) error {
	return c.send(addr, buf, isCtrl)
}

func (c *TransportManagerContainer) Broadcast(addr *transport.TransportAddress, buf *buffer.Buffer) error {
	err := c.manager.SendBroadcastMsg(addr, buf, nil)
	if err == nil {
		c.statis.record(false, true)
	}

	return err
}

func (c *TransportManagerContainer) AddPeer(peer *PeerMock) {
	c.peers = append(c.peers, peer)
}

func TestTransportManager(t *testing.T) {
	t.Log("TestTransportManager")

	// peer1<---->peer2<---->peer3
	peer1Net := &net.UDPAddr{IP: net.IPv4(192, 168, 0, 1), Port: 9999}
	peer2Net := &net.UDPAddr{IP: net.IPv4(192, 168, 0, 2), Port: 9999}
	peer3Net := &net.UDPAddr{IP: net.IPv4(192, 168, 0, 3), Port: 9999}
	peer1Id := deviceid.NewDeviceId(true, []byte{192, 168, 0, 1}, nil)
	peer2Id := deviceid.NewDeviceId(true, []byte{192, 168, 0, 2}, nil)
	peer3Id := deviceid.NewDeviceId(true, []byte{192, 168, 0, 3}, nil)

	peer1Addr1 := transport.NewAddress(peer1Id, peer1Net)
	peer1Addr2 := transport.NewAddress(peer1Id, peer2Net)
	peer2Addr1 := transport.NewAddress(peer2Id, peer2Net)
	peer3Addr1 := transport.NewAddress(peer3Id, peer3Net)
	peer3Addr2 := transport.NewAddress(peer3Id, peer2Net)

	peer1 := newPeerMock(peer1Id, peer1Net)
	peer2 := newPeerMock(peer2Id, peer2Net)
	peer3 := newPeerMock(peer3Id, peer3Net)

	// peer1 transport
	p1t1 := NewTransportMock("p1t1", peer1, transport.DirectType)
	p1t1.AddPeer(peer2)

	// peer2 transport
	p2t1 := NewTransportMock("p2t1", peer2, transport.DirectType)
	p2t1.AddPeer(peer1)
	p2t1.AddPeer(peer3)
	p2t2 := NewTransportMock("p2t2", peer2, transport.ForwardType)
	p2t2.AddPeer(peer1)
	p2t2.AddPeer(peer3)

	// peer3 transport
	p3t1 := NewTransportMock("p3t1", peer3, transport.DirectType)
	p3t1.AddPeer(peer2)

	// peer1 manager
	p1m := newTransportManagerContainer(peer1Id)
	p1m.manager.AddTransport(p1t1)

	// peer2 manager
	p2m := newTransportManagerContainer(peer2Id)
	p2m.manager.AddTransport(p2t1)
	p2m.manager.AddTransport(p2t2)

	// peer3 manager
	p3m := newTransportManagerContainer(peer3Id)
	p3m.manager.AddTransport(p3t1)

	// peer1 Send
	msg := buffer.FromBytes([]byte{1, 2, 3})
	_ = p1m.Send(peer2Addr1, msg, true)
	_ = p1m.Send(peer2Addr1, msg, false)
	_ = p1m.Send(peer3Addr1, msg, true)
	_ = p1m.Send(peer3Addr1, msg, false)
	_ = p1m.Send(peer3Addr2, msg, true)
	_ = p1m.Send(peer3Addr2, msg, false)

	// peer2 Send
	_ = p2m.Send(peer1Addr1, msg, true)
	_ = p2m.Send(peer1Addr1, msg, false)
	_ = p2m.Send(peer3Addr1, msg, true)
	_ = p2m.Send(peer3Addr1, msg, false)

	// peer3 Send
	_ = p3m.Send(peer1Addr1, msg, true)
	_ = p3m.Send(peer1Addr1, msg, false)
	_ = p3m.Send(peer1Addr2, msg, true)
	_ = p3m.Send(peer1Addr2, msg, false)
	_ = p3m.Send(peer2Addr1, msg, true)
	_ = p3m.Send(peer2Addr1, msg, false)

	time.Sleep(2 * time.Second)

	// check peer1
	if p1m.statis.sendCtrlCnt != 2 {
		t.Errorf("peer1 send ctrl count error, expect 2, but %d", p1m.statis.sendCtrlCnt)
	}
	if p1m.statis.sendDataCnt != 2 {
		t.Errorf("peer1 send data count error, expect 2, but %d", p1m.statis.sendDataCnt)
	}
	if p1m.statis.recvCtrlCnt != 2 {
		t.Errorf("peer1 recv ctrl count error, expect 2, but %d", p1m.statis.recvCtrlCnt)
	}
	if p1m.statis.recvDataCnt != 2 {
		t.Errorf("peer1 recv data count error, expect 2, but %d", p1m.statis.recvDataCnt)
	}
	if peer1.sendCnt != 4 {
		t.Errorf("peer1 send count error, expect 4, but %d", peer1.sendCnt)
	}
	if peer1.recvCnt != 4 {
		t.Errorf("peer1 recv count error, expect 4, but %d", peer1.recvCnt)
	}

	// check peer1
	if p2m.statis.sendCtrlCnt != 2 {
		t.Errorf("peer2 send ctrl count error, expect 2, but %d", p2m.statis.sendCtrlCnt)
	}
	if p2m.statis.sendDataCnt != 2 {
		t.Errorf("peer2 send data count error, expect 2, but %d", p2m.statis.sendDataCnt)
	}
	if p2m.statis.recvCtrlCnt != 2 {
		t.Errorf("peer2 recv ctrl count error, expect 2, but %d", p2m.statis.recvCtrlCnt)
	}
	if p2m.statis.recvDataCnt != 2 {
		t.Errorf("peer2 recv data count error, expect 2, but %d", p2m.statis.recvDataCnt)
	}
	if peer2.sendCnt != 8 {
		t.Errorf("peer2 send count error, expect 8, but %d", peer2.sendCnt)
	}
	if peer2.recvCnt != 8 {
		t.Errorf("peer2 recv count error, expect 8, but %d", peer2.recvCnt)
	}

	// check peer3
	if p3m.statis.sendCtrlCnt != 2 {
		t.Errorf("peer3 send ctrl count error, expect 2, but %d", p3m.statis.sendCtrlCnt)
	}
	if p3m.statis.sendDataCnt != 2 {
		t.Errorf("peer3 send data count error, expect 2, but %d", p3m.statis.sendDataCnt)
	}
	if p3m.statis.recvCtrlCnt != 2 {
		t.Errorf("peer3 recv ctrl count error, expect 2, but %d", p3m.statis.recvCtrlCnt)
	}
	if p3m.statis.recvDataCnt != 2 {
		t.Errorf("peer3 recv data count error, expect 2, but %d", p3m.statis.recvDataCnt)
	}
	if peer3.sendCnt != 4 {
		t.Errorf("peer3 send count error, expect 4, but %d", peer3.sendCnt)
	}
	if peer3.recvCnt != 4 {
		t.Errorf("peer3 recv count error, expect 4, but %d", peer3.recvCnt)
	}
}

func TestBroadcast(t *testing.T) {
	t.Log("TestTransportManager")
	num := 30

	newPeers := func(num int) []*PeerMock {
		peers := make([]*PeerMock, num)
		for i := 0; i < num; i++ {
			peerNet := &net.UDPAddr{IP: net.IPv4(192, 168, 0, 1), Port: 9999}
			peerId := deviceid.NewDeviceId(true, []byte{192, 168, 0, 1}, nil)
			peer := newPeerMock(peerId, peerNet)
			peers[i] = peer
		}

		return peers
	}

	newTransportManagerContainers := func(peers []*PeerMock) []*TransportManagerContainer {
		n := len(peers)
		mcs := make([]*TransportManagerContainer, n)
		peerNum := n / 4
		if peerNum == 0 {
			peerNum = 1
		}
		for i := 0; i < n; i++ {
			me := peers[i]
			t := NewTransportMock("mock", me, transport.DirectType)
			p := newTransportManagerContainer(me.devId)
			p.manager.AddTransport(t)
			for j := 0; j < peerNum; j++ {
				peer := peers[(i+j+1)%n]
				t.AddPeer(peer)
				p.AddPeer(peer)
			}
			mcs[i] = p
		}
		return mcs
	}

	peers := newPeers(num)
	managers := newTransportManagerContainers(peers)
	data := [2000]byte{1, 2, 3}
	msg := buffer.FromBytes(data[:])

	source := managers[0]
	for _, peer := range source.peers {
		_ = source.Broadcast(peer.Address(), msg.Clone())
	}

	time.Sleep(5 * time.Second)
	for i, manager := range managers {
		if i != 0 {
			if manager.statis.recvBroadcastCnt != 1 {
				t.Errorf("manager %s recv broadcast count error, expect 1, but %d", manager.devId, manager.statis.recvBroadcastCnt)
			}
		}
	}
}
