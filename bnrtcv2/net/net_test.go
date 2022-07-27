package net_test

import (
	"context"
	"errors"
	"fmt"
	inet "net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/message"
	"github.com/guabee/bnrtc/bnrtcv2/net"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/bnrtcv2/server"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
	"github.com/stretchr/testify/assert"
)

type ConnectionManagerMock struct {
	connectionSize  int
	connectionLimit int
	connections     *util.SafeLinkMap
}

func newConnectionManagerMock(limit int) net.IConnectionManager {
	return &ConnectionManagerMock{
		connectionLimit: limit,
		connections:     util.NewSafeLinkMap(),
	}
}

func (c *ConnectionManagerMock) Connect(addr *transport.TransportAddress) error {
	if c.connections.Size() >= c.connectionLimit {
		c.connections.Shift()
		c.connectionSize--
	}

	c.connections.Set(addr.DeviceId.String(), true)
	c.connectionSize++
	return nil
}

func (c *ConnectionManagerMock) IsConnected(deviceId deviceid.DeviceId) bool {
	_, found := c.connections.Get(deviceId.String())
	return found
}

func (c *ConnectionManagerMock) GetConnectionLimit() int {
	return c.connectionLimit
}

func (c *ConnectionManagerMock) GetConnectionSize() int {
	return c.connectionSize
}

type Record struct {
	SendCnt   uint64
	RecvCnt   uint64
	SendBytes uint64
	RecvBytes uint64
}

func TestStatis(t *testing.T) {
	m := net.NewStatisManager(context.Background())
	id1 := "id1"
	id2 := "id2"
	id3 := "id3"
	id4 := "id4"

	recordMap := make(map[string]*Record)
	m.OnChange(func(deviceId string, statis *net.Statis, bytes uint64, isSend bool) {
		r, found := recordMap[deviceId]
		if !found {
			r = &Record{}
			recordMap[deviceId] = r
		}

		if isSend {
			r.SendCnt++
			r.SendBytes += bytes
		} else {
			r.RecvCnt++
			r.RecvBytes += bytes
		}
	})

	count := uint64(10)
	bytes := uint64(0)
	for i := 0; i < int(count); i++ {
		bytes += uint64(i)
		m.RecordSend(id1, uint64(i))
		m.RecordRecv(id2, uint64(i))
		m.RecordSend(id3, uint64(i))
		m.RecordRecv(id3, uint64(i))
	}

	time.Sleep(2 * time.Second)
	s1 := m.GetStatis(id1)
	r1 := recordMap[id1]
	if s1.CountSent != r1.SendCnt || s1.CountReceived != r1.RecvCnt || s1.BytesSent != r1.SendBytes || s1.BytesReceived != r1.RecvBytes {
		t.Errorf("statis error %s: %+v != %+v", id1, r1, s1)
	}
	if s1.CountSent != count || s1.CountReceived != 0 || s1.BytesSent != bytes || s1.BytesReceived != 0 {
		t.Errorf("statis error %s: %+v != %+v", id1, r1, s1)
	}
	s2 := m.GetStatis(id2)
	r2 := recordMap[id2]
	if s2.CountSent != r2.SendCnt || s2.CountReceived != r2.RecvCnt || s2.BytesSent != r2.SendBytes || s2.BytesReceived != r2.RecvBytes {
		t.Errorf("statis error %s: %+v != %+v", id2, r2, s2)
	}
	if s2.CountSent != 0 || s2.CountReceived != count || s2.BytesSent != 0 || s2.BytesReceived != bytes {
		t.Errorf("statis error %s: %+v != %+v", id2, r2, s2)
	}

	sendCount := m.GetSendCount(id3)
	recvCount := m.GetRecvCount(id3)
	sendBytes := m.GetSendBytes(id3)
	recvBytes := m.GetRecvBytes(id3)
	r3 := recordMap[id3]
	if sendCount != r3.SendCnt || recvCount != r3.RecvCnt || sendBytes != r3.SendBytes || recvBytes != r3.RecvBytes {
		t.Errorf("statis error %s: %+v", id3, r3)
	}
	if sendCount != count || recvCount != count || sendBytes != bytes || recvBytes != bytes {
		t.Errorf("statis error %s: %+v", id3, r3)
	}

	s4 := m.GetStatis(id4)
	_, found := recordMap[id4]
	if s4 != nil || found {
		t.Errorf("statis found not exist item %s", id4)
	}

	m.Clear(id1)
	if m.GetStatis(id1) != nil {
		t.Errorf("clear error %s", id1)
	}

	m.ClearAll()
	if m.GetStatis(id2) != nil {
		t.Errorf("clear error %s", id2)
	}
}

func TestFavorite(t *testing.T) {
	f := net.NewFavoriteManager(context.Background(), &DeviceMock{}, newConnectionManagerMock(10))
	addr1 := "addr1"
	addr2 := "addr2"
	addr3 := "addr3"
	addr4 := "addr4"

	f.AddFavorite(addr1)
	f.AddFavorite(addr2)
	f.AddFavorite(addr3)

	if !f.HasFavorite(addr1) || !f.HasFavorite(addr2) || !f.HasFavorite(addr3) || f.HasFavorite(addr4) {
		t.Errorf("add error")
	}

	f.DelFavorite(addr1)
	if f.HasFavorite(addr1) || !f.HasFavorite(addr2) || !f.HasFavorite(addr3) || f.HasFavorite(addr4) {
		t.Errorf("del error")
	}

	addrs := f.GetFavorites()
	addrsMap := map[string]bool{addr2: true, addr3: true}
	if len(addrs) != 2 || !addrsMap[addrs[0]] || !addrsMap[addrs[1]] {
		t.Errorf("get all error")
	}
}

func TestDirectChannel(t *testing.T) {
	id1 := deviceid.NewDeviceId(true, []byte{1, 1, 1, 1}, nil)
	id2 := deviceid.NewDeviceId(false, []byte{127, 0, 0, 1}, nil)
	id3 := deviceid.NewDeviceId(false, []byte{127, 0, 0, 1}, nil)
	id4 := deviceid.NewDeviceId(false, []byte{127, 0, 0, 1}, nil)
	id5 := deviceid.NewDeviceId(false, []byte{127, 0, 0, 1}, nil)
	peer1Addr := transport.NewAddress(id1, &inet.UDPAddr{
		IP:   inet.IPv4(1, 1, 1, 1),
		Port: 8111,
	})
	peer2Addr := transport.NewAddress(id2, &inet.UDPAddr{
		IP:   inet.IPv4(1, 1, 1, 1),
		Port: 8222,
	})
	peer3Addr := transport.NewAddress(id3, &inet.UDPAddr{
		IP:   inet.IPv4(1, 1, 1, 1),
		Port: 8333,
	})
	peer4Addr := transport.NewAddress(id4, &inet.UDPAddr{
		IP:   inet.IPv4(1, 1, 1, 1),
		Port: 8444,
	})
	peer5Addr := transport.NewAddress(id5, &inet.UDPAddr{
		IP:   inet.IPv4(1, 1, 1, 1),
		Port: 8555,
	})

	sm := net.NewStatisManager(context.Background())
	cm := newConnectionManagerMock(2)
	m := net.NewDirectChannelManager(context.Background(), &net.DirectChannelManagerOption{
		StatisManager:           sm,
		ConnectionManager:       cm,
		Device:                  newDeviceMock(id1, false, func(addr *transport.TransportAddress) bool { return false }),
		StaticInfoCheckInterval: 2 * time.Second,
	})

	total := 10
	index := 1
	recordMessage := func(id deviceid.DeviceId) {
		idStr := id.String()
		for i := 0; i < index; i++ {
			sm.RecordSend(idStr, 1)
		}

		for i := 0; i < total-index; i++ {
			sm.RecordRecv(idStr, 1)
		}
		index++
	}

	recordMessage(id2)
	recordMessage(id3)
	recordMessage(id4)
	recordMessage(id1)

	time.Sleep(3 * time.Second)
	// check id4 > id3 > id2
	m.CheckChannel(peer1Addr, false)
	if cm.IsConnected(id1) {
		t.Errorf("peer1 must connected")
	}
	m.CheckChannel(peer1Addr, true)
	if cm.IsConnected(id1) {
		t.Errorf("peer1 must connected")
	}
	m.CheckChannel(peer2Addr, false)
	if !cm.IsConnected(id2) {
		t.Errorf("peer2 must connected")
	}
	m.CheckChannel(peer3Addr, false)
	if !cm.IsConnected(id3) {
		t.Errorf("peer3 must connected")
	}
	m.CheckChannel(peer4Addr, false)
	if !cm.IsConnected(id4) {
		t.Errorf("peer4 must connected")
	}
	m.CheckChannel(peer5Addr, false)
	if cm.IsConnected(id5) {
		t.Errorf("peer5 must not connected")
	}
	m.CheckChannel(peer5Addr, true)
	if !cm.IsConnected(id5) {
		t.Errorf("peer5 must connected")
	}

	recordMessage(id4)
	recordMessage(id3)
	recordMessage(id2)
	recordMessage(id1)
	time.Sleep(3 * time.Second)
	// prev connection have id4, id5; now check id2 > id3 > id4
	m.CheckChannel(peer4Addr, false)
	if !cm.IsConnected(id4) {
		t.Errorf("peer4 must connected")
	}
	m.CheckChannel(peer2Addr, true)
	if !cm.IsConnected(id2) {
		t.Errorf("peer2 must connected")
	}
	m.CheckChannel(peer3Addr, false)
	if !cm.IsConnected(id3) {
		t.Errorf("peer3 must connected")
	}

	m.CheckChannel(peer1Addr, false)
	if cm.IsConnected(id1) {
		t.Errorf("peer1 must not connected")
	}
	m.CheckChannel(peer4Addr, false)
	if cm.IsConnected(id4) {
		t.Errorf("peer4 must not connected")
	}
	m.CheckChannel(peer5Addr, false)
	if cm.IsConnected(id5) {
		t.Errorf("peer5 must not connected")
	}
}

var deviceMessageChannelMap = make(map[string]chan *transport.Message)

type DeviceMock struct {
	devId           deviceid.DeviceId
	msgChan         chan *transport.Message
	isConnected     bool
	isReachableFunc func(addr *transport.TransportAddress) bool
}

func newDeviceMock(id deviceid.DeviceId, isConnected bool, isReachableFunc func(addr *transport.TransportAddress) bool) *DeviceMock {
	d := &DeviceMock{
		devId:           id,
		msgChan:         make(chan *transport.Message, 10),
		isConnected:     isConnected,
		isReachableFunc: isReachableFunc,
	}

	deviceMessageChannelMap[id.String()] = d.msgChan
	return d
}

func deviceSend(addr *transport.TransportAddress, buf *buffer.Buffer) error {
	c := deviceMessageChannelMap[addr.DeviceId.String()]
	if c == nil {
		return errors.New("no channel")
	}
	c <- &transport.Message{
		Target: addr.DeviceId,
		Sender: addr,
		Buffer: buf,
	}

	return nil
}

func (d *DeviceMock) Send(addr *transport.TransportAddress, buf *buffer.Buffer) error {
	return deviceSend(addr, buf)
}

func (d *DeviceMock) Broadcast(buf *buffer.Buffer) error {
	return nil
}

func (d *DeviceMock) OnMessage(handler transport.MessageHandler) {
	go func() {
		for msg := range d.msgChan {
			handler(msg)
		}
	}()
}
func (d *DeviceMock) AddPeers(ips []string) {
}

func (d *DeviceMock) GetPeers() []string { return nil }

func (d *DeviceMock) IsConnected() bool {
	return d.isConnected
}

func (d *DeviceMock) IsTargetReachable(addr *transport.TransportAddress) bool {
	return d.isReachableFunc == nil || d.isReachableFunc(addr)
}

func (d *DeviceMock) GetNeighborDeviceAddresses(num int) []*transport.TransportAddress {
	return nil
}

const syncDportPrefix = "sync"

type GroupManagerMock struct {
	messageHandler func(buf *buffer.Buffer)
}

func (g *GroupManagerMock) HandlerGroupMessage(msg *message.Message) (next bool) {
	if strings.HasPrefix(msg.Dport, syncDportPrefix) {
		if g.messageHandler != nil {
			msg.Dport = strings.TrimPrefix(msg.Dport, syncDportPrefix)
			g.messageHandler(msg.GetBuffer())
		}
		return false
	}
	return true
}

func (g *GroupManagerMock) OnMessage(h func(buf *buffer.Buffer)) {
	g.messageHandler = h
}

type AddressManagerMock struct {
	id                     string
	defaultAddress         string
	defaultAddressDeviceId map[string]string
	addresses              map[string]string
	addressMap             map[string]map[string]*transport.TransportAddress
	usedataHandler         func(buf *buffer.Buffer)
	syncDportPrefix        string
}

func newAddressManagerMock(id string, group *GroupManagerMock) *AddressManagerMock {
	m := &AddressManagerMock{
		id:                     id,
		defaultAddressDeviceId: make(map[string]string),
		addresses:              make(map[string]string),
		addressMap:             make(map[string]map[string]*transport.TransportAddress),
		syncDportPrefix:        syncDportPrefix,
	}

	group.OnMessage(func(buf *buffer.Buffer) {
		if m.usedataHandler != nil {
			m.usedataHandler(buf)
		}
	})

	return m
}

func (m *AddressManagerMock) SyncData(address string, buf *buffer.Buffer) error {
	msg := message.FromBuffer(buf)
	if msg == nil {
		return errors.New("invalid message")
	}

	addresses := m.addressMap[address]
	msg.Sync = true
	msg.DstDevId = ""
	msg.Dport = m.syncDportPrefix + msg.Dport
	for id, ta := range addresses {
		if id != m.id {
			return deviceSend(ta, msg.GetBuffer())
		}
	}
	return nil
}

func (m *AddressManagerMock) SetUserDataHandler(handler func(buf *buffer.Buffer)) {
	m.usedataHandler = handler
}

func (m *AddressManagerMock) GetDefaultAddress() string {
	return m.defaultAddress
}
func (m *AddressManagerMock) addTransportAddress(address string, devid string) {
	if m.addressMap[address] == nil {
		m.addressMap[address] = make(map[string]*transport.TransportAddress)
	}
	m.addressMap[address][devid] = transport.NewAddress(deviceid.FromString(devid), nil)
	m.defaultAddressDeviceId[address] = devid
}

func (m *AddressManagerMock) GetTransportAddress(address string, devid string, flush bool) *transport.TransportAddress {
	a, found := m.addressMap[address]
	if !found {
		return nil
	}
	if devid == "" {
		devid = m.defaultAddressDeviceId[address]
	}
	addr, found := a[devid]
	if !found {
		return nil
	}

	return addr
}

func (m *AddressManagerMock) BindAddress(address string) bool {
	m.addresses[address] = address
	m.defaultAddress = address
	return true
}

func (m *AddressManagerMock) UnbindAddress(address string) bool {
	delete(m.addresses, address)
	delete(m.addressMap, address)
	m.defaultAddress = ""
	for addr := range m.addresses {
		m.defaultAddress = addr
	}
	return true
}
func (m *AddressManagerMock) GetAddresses() []string { return nil }

func (m *AddressManagerMock) IsLocalAddress(address string) bool {
	return m.addresses[address] != ""
}
func (m *AddressManagerMock) GetAddressInfo(address string) (info server.AddressInfo, err error) {
	return
}

type BPControllerMock struct {
	localId       string
	remoteId      string
	dport         string
	onSendMessage func(msg *message.Message)
	onRecvMessage func(msg *message.Message)
	msgChan       chan *message.Message
}

func newBPControllerMock(localId, remoteId, dport string) *BPControllerMock {
	ctrl := &BPControllerMock{
		localId:  localId,
		remoteId: remoteId,
		dport:    dport,
		msgChan:  make(chan *message.Message),
	}
	return ctrl
}

func (c *BPControllerMock) String() string {
	return c.localId + ":" + c.remoteId + ":" + c.dport
}

func (c *BPControllerMock) PeerString() string {
	return c.remoteId + ":" + c.localId + ":" + c.dport
}

func (c *BPControllerMock) LastAlive() time.Time {
	return time.Now()
}

func (c *BPControllerMock) SendEnqueue(msg *message.Message) error {
	if c.onSendMessage != nil {
		c.onSendMessage(msg)
		return nil
	}
	return errors.New("no handler")
}

func (c *BPControllerMock) RecvEnqueue(msg *message.Message) error {
	if c.onRecvMessage != nil {
		c.onRecvMessage(msg)
		return nil
	}

	return errors.New("no handler")
}

func (c *BPControllerMock) OnSendMessage(handler func(*message.Message)) {
	c.onSendMessage = handler
}

func (c *BPControllerMock) OnRecvMessage(handler func(*message.Message)) {
	c.onRecvMessage = handler
}

func (c *BPControllerMock) Close() {
}

func (c *BPControllerMock) OnClose(cb func()) {
}

type BPManagerMock struct {
	id           string
	controllers  map[string]net.IBPController
	onController func(controller net.IBPController)
}

func newBPManagerMock(id string) *BPManagerMock {
	return &BPManagerMock{
		id:          id,
		controllers: make(map[string]net.IBPController),
	}
}

func (m *BPManagerMock) OnController(handler func(ctrl net.IBPController)) {
	m.onController = handler
}

func (m *BPManagerMock) GetController(deviceId string, dport string, _direction net.BPControllerDirection) (net.IBPController, error) {
	name := m.id + ":" + deviceId + ":" + dport
	ctrl, found := m.controllers[name]
	if found {
		return ctrl, nil
	}

	ctrl1 := newBPControllerMock(m.id, deviceId, dport)
	m.controllers[ctrl1.String()] = ctrl1
	if m.onController != nil {
		m.onController(ctrl1)
	}

	if m.id != deviceId {
		ctrl2 := newBPControllerMock(deviceId, m.id, dport)
		m.controllers[ctrl2.String()] = ctrl2
		if m.onController != nil {
			m.onController(ctrl2)
		}
	}

	return ctrl1, nil
}
func (m *BPManagerMock) SetDeviceId(id string) {
	m.id = id
}

func newDeviceIdPubSub(id deviceid.DeviceId) *util.PubSub {
	pubsub := util.NewPubSub()
	pubsub.Publish(rpc.DeviceIdChangeTopic, id)
	return pubsub
}

type netControllerContainer struct {
	Id            string
	AddrManager   *AddressManagerMock
	NetController *net.NetController
}

func newNetControllerContainer(id deviceid.DeviceId) *netControllerContainer {
	device := newDeviceMock(id, true, nil)
	group := &GroupManagerMock{}
	addrManager := newAddressManagerMock(id.String(), group)
	bpManager := newBPManagerMock(id.String())
	connManager := newConnectionManagerMock(10)
	return &netControllerContainer{
		Id:          id.String(),
		AddrManager: addrManager,
		NetController: net.NewNetController(context.Background(), &net.NetControllerConfig{
			GroupManager:      group,
			Device:            device,
			AddressManager:    addrManager,
			BpManager:         bpManager,
			ConnectionManager: connManager,
		}, net.WithDevid(id), net.WithPubSub(newDeviceIdPubSub(id))),
	}
}

func (c *netControllerContainer) Send(src, dst, dport, devid string, buf *buffer.Buffer) error {
	return c.NetController.Send(src, dst, dport, devid, buf)
}

func (c *netControllerContainer) OnMessage(dport string, f func(c *netControllerContainer, msg *message.Message)) {
	c.NetController.OnMessage(dport, func(msg *message.Message) {
		f(c, msg)
	})
}

func (c *netControllerContainer) BindAddress(address string) {
	c.NetController.BindAddress(address)
}
func (c *netControllerContainer) UnbindAddress(address string) {
	c.NetController.UnbindAddress(address)
}

type MessageStatisItem struct {
	sendCnt int64
	recvCnt int64
}

type MessageStatis struct {
	DportMessageCnt   map[string]*MessageStatisItem
	AddressMessageCnt map[string]*MessageStatisItem
	DeviceMessageCnt  map[string]*MessageStatisItem
	Total             MessageStatisItem
}

func newMessageStatis(dports, addrs, devids []string) *MessageStatis {
	statis := &MessageStatis{
		DportMessageCnt:   make(map[string]*MessageStatisItem),
		AddressMessageCnt: make(map[string]*MessageStatisItem),
		DeviceMessageCnt:  make(map[string]*MessageStatisItem),
	}

	for _, dport := range dports {
		statis.DportMessageCnt[dport] = &MessageStatisItem{}
	}
	for _, addr := range addrs {
		statis.AddressMessageCnt[addr] = &MessageStatisItem{}
	}
	for _, devid := range devids {
		statis.DeviceMessageCnt[devid] = &MessageStatisItem{}
	}
	return statis
}

func (statis *MessageStatis) Record(dport, addr, devid string, isSent bool) {
	if isSent {
		atomic.AddInt64(&statis.Total.sendCnt, 1)
		atomic.AddInt64(&statis.DportMessageCnt[dport].sendCnt, 1)
		atomic.AddInt64(&statis.AddressMessageCnt[addr].sendCnt, 1)
		atomic.AddInt64(&statis.DeviceMessageCnt[devid].sendCnt, 1)
	} else {
		atomic.AddInt64(&statis.Total.recvCnt, 1)
		atomic.AddInt64(&statis.DportMessageCnt[dport].recvCnt, 1)
		atomic.AddInt64(&statis.AddressMessageCnt[addr].recvCnt, 1)
		atomic.AddInt64(&statis.DeviceMessageCnt[devid].recvCnt, 1)
	}
}

func (statis *MessageStatis) String() string {
	str := "Count: " + fmt.Sprintf("send:%d recv:%d\n", statis.Total.sendCnt, statis.Total.recvCnt)
	str += "DPORT:\n"
	for dport, item := range statis.DportMessageCnt {
		str += fmt.Sprintf("dport:%s send:%d recv:%d\n", dport, item.sendCnt, item.recvCnt)
	}
	str += "ADDRESS:\n"
	for addr, item := range statis.AddressMessageCnt {
		str += fmt.Sprintf("addr:%s send:%d recv:%d\n", addr, item.sendCnt, item.recvCnt)
	}
	str += "DEVICE:\n"
	for devid, item := range statis.DeviceMessageCnt {
		str += fmt.Sprintf("devid:%s send:%d recv:%d\n", devid, item.sendCnt, item.recvCnt)
	}

	return str
}

func TestController(t *testing.T) {
	addr1 := "addr1"
	addr2 := "addr2"
	addr3 := "addr3"
	addr4 := "addr4"
	addr5 := "addr5"
	dport1 := "dport1"
	dport2 := "dport2"
	dport3 := "dport3"
	dport4 := "dport4"
	peer1Id := deviceid.NewDeviceId(false, []byte{127, 0, 0, 1}, nil)
	peer1IdStr := peer1Id.String()
	peer2Id := deviceid.NewDeviceId(false, []byte{127, 0, 0, 1}, nil)
	peer2IdStr := peer2Id.String()
	peer3Id := deviceid.NewDeviceId(false, []byte{127, 0, 0, 1}, nil)
	peer3IdStr := peer3Id.String()

	addTransportAddresses := func(n *netControllerContainer) {
		n.AddrManager.addTransportAddress(addr1, peer1IdStr)
		n.AddrManager.addTransportAddress(addr1, peer3IdStr)
		n.AddrManager.addTransportAddress(addr2, peer1IdStr)
		n.AddrManager.addTransportAddress(addr3, peer2IdStr)
		n.AddrManager.addTransportAddress(addr3, peer3IdStr)
		n.AddrManager.addTransportAddress(addr4, peer2IdStr)
		n.AddrManager.addTransportAddress(addr5, peer3IdStr)
	}

	n1 := newNetControllerContainer(peer1Id)
	n2 := newNetControllerContainer(peer2Id)
	n3 := newNetControllerContainer(peer3Id)
	time.Sleep(time.Millisecond * 100)

	// know all transportAddress
	addTransportAddresses(n1)
	addTransportAddresses(n2)
	addTransportAddresses(n3)
	messageStatis := newMessageStatis([]string{dport1, dport2, dport3, dport4}, []string{addr1, addr2, addr3, addr4, addr5}, []string{peer1IdStr, peer2IdStr, peer3IdStr})
	expectMessageStatis := newMessageStatis([]string{dport1, dport2, dport3, dport4}, []string{addr1, addr2, addr3, addr4, addr5}, []string{peer1IdStr, peer2IdStr, peer3IdStr})
	msgHandler := func(c *netControllerContainer, msg *message.Message) {
		messageStatis.Record(msg.Dport, msg.DstAddr, c.Id, false)
	}
	//n1: dport1 dport2
	//n2: dport1 dport3
	//n3: dport2 dport3 dport4
	n1.OnMessage(dport1, msgHandler)
	n1.OnMessage(dport2, msgHandler)
	n2.OnMessage(dport1, msgHandler)
	n2.OnMessage(dport3, msgHandler)
	n3.OnMessage(dport2, msgHandler)
	n3.OnMessage(dport3, msgHandler)
	n3.OnMessage(dport4, msgHandler)

	// device1: addr1 addr2
	// device2: addr3 addr4
	// device3: addr1 addr3 addr5
	n1.BindAddress(addr1)
	n1.BindAddress(addr2)
	n2.BindAddress(addr3)
	n2.BindAddress(addr4)
	n3.BindAddress(addr1)
	n3.BindAddress(addr3)
	n3.BindAddress(addr5)

	send := func(c *netControllerContainer, src, dst, dport, devid string) {
		buf := buffer.FromBytes([]byte("hello"))
		err := c.Send(src, dst, dport, devid, buf)
		if err != nil {
			fmt.Println("err", err)
			return
		}
		if src == "" {
			src = c.AddrManager.GetDefaultAddress()
		}

		messageStatis.Record(dport, src, c.Id, true)
	}

	//device1: dport1 dport2              addr1 addr2
	//device2: dport1 dport3              addr3 addr4
	//device3: dport2 dport3 dport4       addr1 addr3 addr5
	send(n1, addr1, addr2, dport1, "")
	expectMessageStatis.Record(dport1, addr1, peer1IdStr, true)
	expectMessageStatis.Record(dport1, addr2, peer1IdStr, false)
	send(n1, addr2, addr2, dport1, "") // failed
	send(n1, "", addr2, dport1, "")    // failed
	send(n1, addr1, addr1, dport1, "") //failed
	send(n1, addr1, addr1, dport1, peer3IdStr)
	expectMessageStatis.Record(dport1, addr1, peer1IdStr, true)
	send(n1, addr1, addr3, dport2, "")
	expectMessageStatis.Record(dport2, addr1, peer1IdStr, true)
	expectMessageStatis.Record(dport2, addr3, peer3IdStr, false)
	send(n1, addr1, addr4, dport2, "")
	expectMessageStatis.Record(dport2, addr1, peer1IdStr, true)
	send(n1, addr1, addr5, dport2, "")
	expectMessageStatis.Record(dport2, addr1, peer1IdStr, true)
	expectMessageStatis.Record(dport2, addr5, peer3IdStr, false)
	send(n1, addr1, addr3, dport3, "")
	expectMessageStatis.Record(dport3, addr1, peer1IdStr, true)
	expectMessageStatis.Record(dport3, addr3, peer3IdStr, false)
	send(n1, addr1, addr4, dport4, "")
	expectMessageStatis.Record(dport4, addr1, peer1IdStr, true)
	send(n1, addr1, addr5, dport4, "")
	expectMessageStatis.Record(dport4, addr1, peer1IdStr, true)
	expectMessageStatis.Record(dport4, addr5, peer3IdStr, false)

	// test SendtoAll
	sendToAll := func(c *netControllerContainer, src, dst, dport, devid string) {
		buf := buffer.FromBytes([]byte("hello"))
		err := c.NetController.SendToAll(src, dst, dport, devid, buf)
		if err != nil {
			return
		}
		if src == "" {
			src = c.AddrManager.GetDefaultAddress()
		}

		messageStatis.Record(dport, src, c.Id, true)
	}
	sendToAll(n1, addr1, addr3, dport3, "")
	expectMessageStatis.Record(dport3, addr1, peer1IdStr, true)
	expectMessageStatis.Record(dport3, addr3, peer2IdStr, false)
	expectMessageStatis.Record(dport3, addr3, peer3IdStr, false)
	sendToAll(n1, addr1, addr3, dport3, peer2IdStr)
	expectMessageStatis.Record(dport3, addr1, peer1IdStr, true)
	expectMessageStatis.Record(dport3, addr3, peer2IdStr, false)
	expectMessageStatis.Record(dport3, addr3, peer3IdStr, false)
	sendToAll(n1, addr1, addr3, dport3, peer3IdStr)
	expectMessageStatis.Record(dport3, addr1, peer1IdStr, true)
	expectMessageStatis.Record(dport3, addr3, peer2IdStr, false)
	expectMessageStatis.Record(dport3, addr3, peer3IdStr, false)

	// test Sync
	syncData := func(c *netControllerContainer, address, dport string) {
		buf := buffer.FromBytes([]byte("hello"))
		err := c.NetController.Sync(address, dport, buf)
		if err != nil {
			return
		}
	}
	syncData(n1, addr1, dport2)
	expectMessageStatis.Record(dport2, addr1, peer3IdStr, false)
	syncData(n2, addr3, dport3)
	expectMessageStatis.Record(dport3, addr3, peer3IdStr, false)
	syncData(n3, addr1, dport2)
	expectMessageStatis.Record(dport2, addr1, peer1IdStr, false)

	//check
	time.Sleep(5 * time.Second)
	t.Logf("expectMessageStatis: %s\n", expectMessageStatis.String())
	t.Logf("messageStatis: %s\n", messageStatis.String())
	assert.Equal(t, expectMessageStatis, messageStatis)
}
