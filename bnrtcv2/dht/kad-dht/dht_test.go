package kaddht

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
	"github.com/stretchr/testify/suite"
)

var transportIdMsgChannel = util.NewSafeMap()
var transportAddressMsgChannel = util.NewSafeMap()
var IdIndex int32 = 0
var portIndex int32 = 3000
var allDHts = make(map[string]*DHTContainer)
var MaxNodeNumPerBucketTest = 8

type TransportManagerMock struct {
	Id                        deviceid.DeviceId
	NetAddr                   *net.UDPAddr
	TransportAddress          *transport.TransportAddress
	TransportAddressWithoutId *transport.TransportAddress
	Port                      int
	filter                    func(addr *transport.TransportAddress) bool
	msgChan                   chan *transport.Message
}

type ID_TYPE string

const (
	ID_TOP_NETWORK = "top"
	ID_SUB_NETWORK = "sub"
	ID_RANDOM      = "random"
)

func getMsgChannel(addr *transport.TransportAddress) chan<- *transport.Message {
	ch, found := transportIdMsgChannel.Get(addr.DeviceId.String())
	if found {
		return ch.(chan *transport.Message)
	}

	ch, found = transportAddressMsgChannel.Get(addr.Addr.String())
	if found {
		return ch.(chan *transport.Message)
	}

	return nil
}

func (t *TransportManagerMock) SendCtrlMsg(addr *transport.TransportAddress, buf *buffer.Buffer) error {
	ch := getMsgChannel(addr)
	if ch != nil {
		msg := &transport.Message{
			Target: t.Id,
			Buffer: buf,
		}
		// fmt.Printf("%s Sending msg to %s\n", t.Id, addr)
		ch <- msg
		return nil
	} else {
		return fmt.Errorf("channel for address %s not found", addr)
	}
}

func (t *TransportManagerMock) GetCtrlMsgChan() <-chan *transport.Message {
	return t.msgChan
}

func (t *TransportManagerMock) AddAddress(Addr *net.UDPAddr) {
	transportAddressMsgChannel.Set(Addr.String(), t.msgChan)
}

func (t *TransportManagerMock) SetAddrFilter(filter func(addr *transport.TransportAddress) bool) {
	t.filter = filter
}
func (t *TransportManagerMock) IsTargetReachable(addr *transport.TransportAddress) bool {
	if t.filter != nil {
		return t.filter(addr)
	}

	if addr.IsPublic() || addr.IsLocal() {
		return true
	} else if strings.HasPrefix(addr.Addr.IP.String(), "192.168.1") {
		return true
	}
	return false
}

func getPort() int {
	return int(atomic.AddInt32(&portIndex, 1))
}

func newID(tt ID_TYPE, ip string) deviceid.DeviceId {
	index := atomic.AddInt32(&IdIndex, 1)
	idRandom := make([]byte, deviceid.NodeIdLength)
	idRandom[14] = byte(index & 0xff)
	idRandom[13] = byte((index >> 8) & 0xff)
	idGroup := net.ParseIP(ip).To4()
	isTop := false
	if tt == ID_TOP_NETWORK {
		isTop = true
	} else if tt == ID_SUB_NETWORK {
		isTop = false
	} else {
		isTop = (index % 2) == 0
	}

	return deviceid.NewDeviceId(isTop, idGroup, idRandom)
}

func newTransportManagerMockWithIp(tt ID_TYPE, ip string) *TransportManagerMock {
	port := getPort()
	t := &TransportManagerMock{
		Id:      newID(tt, ip),
		Port:    port,
		NetAddr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port},
		msgChan: make(chan *transport.Message, 1024),
	}
	t.TransportAddress = transport.NewAddress(t.Id, t.NetAddr)
	t.TransportAddressWithoutId = transport.NewAddress(nil, t.NetAddr)
	transportIdMsgChannel.Set(t.Id.String(), t.msgChan)
	transportAddressMsgChannel.Set(t.NetAddr.String(), t.msgChan)

	return t
}

func newTransportManagerMock(tt ID_TYPE) *TransportManagerMock {
	return newTransportManagerMockWithIp(tt, "")
}

func newDhtListWithIp(num int, tt ID_TYPE, ip string, peers []*transport.TransportAddress) []*DHTContainer {
	dhts := []*DHTContainer{}
	for i := 0; i < num; i++ {
		tm := newTransportManagerMockWithIp(tt, ip)
		dht := newDHT(tm, peers)
		dhts = append(dhts, dht)
	}

	return dhts
}

func newDhtList(num int, tt ID_TYPE, peers []*transport.TransportAddress) []*DHTContainer {
	return newDhtListWithIp(num, tt, "", peers)
}

func newTransportAddress(id deviceid.DeviceId, port int) *transport.TransportAddress {
	return transport.NewAddress(id, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
}

type DHTContainer struct {
	*DHT
}

func (c *DHTContainer) Close() error {
	delete(allDHts, c.Self.Id.String())
	return c.DHT.Close()
}

func (c *DHTContainer) FindTarget(target *DHTContainer) bool {

	_, nodes, _ := c.iterate(iterateFindNode, target.Self.Id, nil, nil)
	if len(nodes) == 0 {
		return false
	}

	if !nodes[0].Id.Equal(target.Self.Id) {
		return false
	}

	return true
}

func newDHT(tm *TransportManagerMock, peers []*transport.TransportAddress) *DHTContainer {
	nodes := []*Node{}
	for _, addr := range peers {
		nodes = append(nodes, NewNode(device_info.BuildDeviceInfo(addr)))
	}
	pubsub := util.NewPubSub()
	pubsub.SubscribeFunc("dwadaw", rpc.DHTStatusChangeTopic, func(msg util.PubSubMsgType) {
		return
	})

	dht, _ := NewDHT(&Options{
		ID:                  tm.Id,
		DeviceInfo:          device_info.BuildDeviceInfo(tm.TransportAddress),
		BootstrapNodes:      nodes,
		TMsgTimeout:         2 * time.Second,
		TCheckBootstrap:     1 * time.Second,
		TUpdateHashtable:    1 * time.Second,
		TUPdateClosestNode:  1 * time.Second,
		MaxNodeNumPerBucket: MaxNodeNumPerBucketTest,
		AutoRun:             true,
		PubSub:              pubsub,
	}, tm)

	dhtc := &DHTContainer{
		DHT: dht,
	}

	allDHts[tm.Id.String()] = dhtc
	time.Sleep(200 * time.Millisecond)
	return dhtc
}

type DHTTestSuit struct {
	suite.Suite
}

func (s *DHTTestSuit) SetupTest() {
	IdIndex = 0
	portIndex = 3000
}

func (s *DHTTestSuit) TearDownTest() {
	wg := sync.WaitGroup{}
	wg.Add(len(allDHts))
	for _, dht := range allDHts {
		go func(d *DHTContainer) {
			d.Close()
			wg.Done()
		}(dht)
	}
	allDHts = make(map[string]*DHTContainer)

	for _, v := range transportIdMsgChannel.Items() {
		close(v.(chan *transport.Message))
	}
	transportIdMsgChannel.Clear()
	transportAddressMsgChannel.Clear()
	wg.Wait()
}

func TestDHTSuit(t *testing.T) {
	suite.Run(t, new(DHTTestSuit))
}

// Creates ten DHTs and bootstraps each with the previous
// at the end all should know about each other
func (s *DHTTestSuit) TestBootstrapTenNodes() {
	dhts := []*DHTContainer{}
	prevTm := &TransportManagerMock{
		Port: 2999,
	}
	num := 10
	for i := 0; i < num; i++ {
		tm := newTransportManagerMock(ID_TOP_NETWORK)
		addr := (*transport.TransportAddress)(nil)
		// bootstrap node with and without id
		if i%2 == 0 {
			// not id
			addr = newTransportAddress(nil, prevTm.Port)
		} else if i%3 == 0 {
			// right id
			addr = newTransportAddress(prevTm.Id, prevTm.Port)
		} else {
			// wrong id
			addr = newTransportAddress(tm.Id, prevTm.Port)
		}
		dhts = append(dhts, newDHT(tm, []*transport.TransportAddress{addr}))
		prevTm = tm
	}

	time.Sleep(time.Duration(num) * time.Second)
	for _, dht := range dhts {
		s.Assert().Equal(num-1, dht.NumNodes(false))
	}
}

// Create two DHTs have them connect and bootstrap, then close. Repeat
// 10 times to ensure that we can use the same IP and port without EADDRINUSE
// errors.
func (s *DHTTestSuit) TestReconnect() {
	for i := 0; i < 10; i++ {
		tm1 := newTransportManagerMock(ID_TOP_NETWORK)
		tm2 := newTransportManagerMock(ID_TOP_NETWORK)

		dht1 := newDHT(tm1, nil)
		dht2 := newDHT(tm2, []*transport.TransportAddress{tm1.TransportAddressWithoutId})

		done := make(chan bool)
		go func() {
			time.Sleep(100 * time.Millisecond)
			err := dht2.Close()
			s.Assert().NoError(err)
			err = dht1.Close()
			s.Assert().NoError(err)
			done <- true
		}()

		time.Sleep(100 * time.Millisecond)
		s.Assert().Equal(1, dht1.NumNodes(false))
		s.Assert().Equal(1, dht2.NumNodes(false))
		<-done
	}
}

// Create two DHTs and have them connect. Send a store message with 1mb
// payload from one node to another. Ensure that the other node now has
// this data in its store.
func (s *DHTTestSuit) TestStoreAndFindValue() {
	tm1 := newTransportManagerMock(ID_TOP_NETWORK)
	dht1 := newDHT(tm1, nil)
	dhts := newDhtList(10, ID_TOP_NETWORK, []*transport.TransportAddress{tm1.TransportAddressWithoutId})
	time.Sleep(5 * time.Second)

	payload := [10]byte{}
	key := deviceid.KeyGen(payload[:])
	err := dht1.Store(key, payload[:], nil)
	s.Assert().NoError(err)
	time.Sleep(5 * time.Second)

	num := dht1.store.KeyNum()
	for _, dht := range dhts {
		num += dht.store.KeyNum()
	}
	s.Assert().Contains([]int{3, 4}, num) // 3 for dht1 is one of closest node, 4 for the other dhts
	for _, dht := range dhts {
		value, exists, err := dht.Get(key, false)
		s.Assert().NoError(err)
		s.Assert().Equal(true, exists)
		s.Assert().Equal(0, bytes.Compare(payload[:], value))
	}
}

func (s *DHTTestSuit) TestGroupTableBucket() {
	tm1 := newTransportManagerMockWithIp(ID_TOP_NETWORK, "1.1.1.1")
	dht1 := newDHT(tm1, nil)

	newDhtListWithIp(MaxNodeNumPerBucketTest+2, ID_TOP_NETWORK, "2.2.2.2", []*transport.TransportAddress{tm1.TransportAddressWithoutId})
	newDhtListWithIp(MaxNodeNumPerBucketTest+2, ID_SUB_NETWORK, "3.3.3.3", []*transport.TransportAddress{tm1.TransportAddressWithoutId})

	time.Sleep(5 * time.Second)
	s.Assert().Equal(MaxNodeNumPerBucketTest, dht1.NumNodes(false))
}

func (s *DHTTestSuit) TestTopSubNet() {
	tm1 := newTransportManagerMock(ID_TOP_NETWORK)
	dht1 := newDHT(tm1, nil)
	id1 := dht1.Self.Id
	dhts1 := newDhtList(10, ID_SUB_NETWORK, []*transport.TransportAddress{tm1.TransportAddressWithoutId})
	tm2 := newTransportManagerMock(ID_TOP_NETWORK)
	dht2 := newDHT(tm2, []*transport.TransportAddress{tm1.TransportAddressWithoutId})
	id2 := dht2.Self.Id
	dhts2 := newDhtList(10, ID_SUB_NETWORK, []*transport.TransportAddress{tm2.TransportAddressWithoutId})

	time.Sleep(5 * time.Second)

	for _, dht := range dhts1 {
		node := dht.ht.getNode(id1)
		s.Assert().NotEmpty(node)
		node2 := dht.ht.getNode(id2)
		s.Assert().NotEmpty(node2)
	}
	for _, dht := range dhts2 {
		node := dht.ht.getNode(id1)
		s.Assert().NotEmpty(node)
		node2 := dht.ht.getNode(id2)
		s.Assert().NotEmpty(node2)
	}
}

func (s *DHTTestSuit) TestHashGroupTable() {
	tm1 := newTransportManagerMockWithIp(ID_TOP_NETWORK, "1.1.1.1")
	dht1 := newDHT(tm1, nil)

	tm11 := newTransportManagerMockWithIp(ID_SUB_NETWORK, "2.1.1.1")
	dht11 := newDHT(tm11, []*transport.TransportAddress{tm1.TransportAddressWithoutId})
	tm12 := newTransportManagerMockWithIp(ID_SUB_NETWORK, "2.1.1.1")
	dht12 := newDHT(tm12, []*transport.TransportAddress{tm1.TransportAddressWithoutId})
	tm13 := newTransportManagerMockWithIp(ID_SUB_NETWORK, "1.3.3.3")
	dht13 := newDHT(tm13, []*transport.TransportAddress{tm1.TransportAddressWithoutId})
	tm14 := newTransportManagerMockWithIp(ID_SUB_NETWORK, "1.3.3.3")
	dht14 := newDHT(tm14, []*transport.TransportAddress{tm1.TransportAddressWithoutId})

	tm2 := newTransportManagerMockWithIp(ID_TOP_NETWORK, "2.2.2.2")
	dht2 := newDHT(tm2, []*transport.TransportAddress{tm1.TransportAddressWithoutId})
	tm21 := newTransportManagerMockWithIp(ID_SUB_NETWORK, "1.2.2.2")
	dht21 := newDHT(tm21, []*transport.TransportAddress{tm2.TransportAddressWithoutId})
	tm22 := newTransportManagerMockWithIp(ID_SUB_NETWORK, "1.2.2.2")
	dht22 := newDHT(tm22, []*transport.TransportAddress{tm2.TransportAddressWithoutId})

	time.Sleep(10 * time.Second)

	s.Assert().NotEmpty(dht1.ht.getNode(dht2.Self.Id))
	s.Assert().NotEmpty(dht1.gt.getNode(dht21.Self.Id))
	s.Assert().NotEmpty(dht1.gt.getNode(dht22.Self.Id))
	s.Assert().NotEmpty(dht1.gt.getNode(dht13.Self.Id))
	s.Assert().NotEmpty(dht1.gt.getNode(dht14.Self.Id))
	s.Assert().Empty(dht1.gt.getNode(dht11.Self.Id))
	s.Assert().Empty(dht1.gt.getNode(dht12.Self.Id))
	s.Assert().Equal(1, dht1.NumNodes(true))
	s.Assert().Equal(5, dht1.NumNodes(false))

	s.Assert().NotEmpty(dht2.ht.getNode(dht1.Self.Id))
	s.Assert().NotEmpty(dht2.gt.getNode(dht11.Self.Id))
	s.Assert().NotEmpty(dht2.gt.getNode(dht12.Self.Id))
	s.Assert().Empty(dht2.gt.getNode(dht13.Self.Id))
	s.Assert().Empty(dht2.gt.getNode(dht14.Self.Id))
	s.Assert().Empty(dht2.gt.getNode(dht21.Self.Id))
	s.Assert().Empty(dht2.gt.getNode(dht22.Self.Id))
	s.Assert().Equal(1, dht2.NumNodes(true))
	s.Assert().Equal(3, dht2.NumNodes(false))

	s.Assert().NotEmpty(dht11.ht.getNode(dht1.Self.Id))
	s.Assert().NotEmpty(dht11.ht.getNode(dht2.Self.Id))
	s.Assert().NotEmpty(dht11.gt.getNode(dht12.Self.Id))
	s.Assert().Empty(dht11.gt.getNode(dht13.Self.Id))
	s.Assert().Empty(dht11.gt.getNode(dht14.Self.Id))
	s.Assert().Empty(dht11.gt.getNode(dht21.Self.Id))
	s.Assert().Empty(dht11.gt.getNode(dht22.Self.Id))
	s.Assert().Equal(2, dht11.NumNodes(true))
	s.Assert().Equal(3, dht11.NumNodes(false))

	s.Assert().NotEmpty(dht12.ht.getNode(dht1.Self.Id))
	s.Assert().NotEmpty(dht12.ht.getNode(dht2.Self.Id))
	s.Assert().NotEmpty(dht12.gt.getNode(dht11.Self.Id))
	s.Assert().Empty(dht11.gt.getNode(dht13.Self.Id))
	s.Assert().Empty(dht11.gt.getNode(dht14.Self.Id))
	s.Assert().Empty(dht12.ht.getNode(dht21.Self.Id))
	s.Assert().Empty(dht12.ht.getNode(dht22.Self.Id))
	s.Assert().Equal(2, dht12.NumNodes(true))
	s.Assert().Equal(3, dht12.NumNodes(false))

	s.Assert().NotEmpty(dht13.ht.getNode(dht1.Self.Id))
	s.Assert().NotEmpty(dht13.ht.getNode(dht2.Self.Id))
	s.Assert().NotEmpty(dht13.gt.getNode(dht14.Self.Id))
	s.Assert().Empty(dht13.gt.getNode(dht11.Self.Id))
	s.Assert().Empty(dht13.gt.getNode(dht22.Self.Id))
	s.Assert().Empty(dht13.ht.getNode(dht21.Self.Id))
	s.Assert().Empty(dht13.ht.getNode(dht22.Self.Id))
	s.Assert().Equal(2, dht13.NumNodes(true))
	s.Assert().Equal(3, dht13.NumNodes(false))

	s.Assert().NotEmpty(dht21.ht.getNode(dht1.Self.Id))
	s.Assert().NotEmpty(dht21.ht.getNode(dht2.Self.Id))
	s.Assert().NotEmpty(dht21.gt.getNode(dht22.Self.Id))
	s.Assert().Empty(dht21.ht.getNode(dht11.Self.Id))
	s.Assert().Empty(dht21.ht.getNode(dht12.Self.Id))
	s.Assert().Empty(dht21.ht.getNode(dht13.Self.Id))
	s.Assert().Empty(dht21.ht.getNode(dht14.Self.Id))
	s.Assert().Equal(2, dht21.NumNodes(true))
	s.Assert().Equal(3, dht21.NumNodes(false))

	s.Assert().NotEmpty(dht22.ht.getNode(dht1.Self.Id))
	s.Assert().NotEmpty(dht22.ht.getNode(dht2.Self.Id))
	s.Assert().NotEmpty(dht22.gt.getNode(dht21.Self.Id))
	s.Assert().Empty(dht22.ht.getNode(dht11.Self.Id))
	s.Assert().Empty(dht22.ht.getNode(dht12.Self.Id))
	s.Assert().Empty(dht22.ht.getNode(dht13.Self.Id))
	s.Assert().Empty(dht22.ht.getNode(dht14.Self.Id))
	s.Assert().Equal(2, dht22.NumNodes(true))
	s.Assert().Equal(3, dht22.NumNodes(false))
}

func (s *DHTTestSuit) TestSendMessage() {
	tm1 := newTransportManagerMockWithIp(ID_TOP_NETWORK, "1.1.1.1")
	dht1 := newDHT(tm1, nil)

	tm11 := newTransportManagerMockWithIp(ID_SUB_NETWORK, "1.2.2.2")
	dht11 := newDHT(tm11, []*transport.TransportAddress{tm1.TransportAddressWithoutId})
	tm12 := newTransportManagerMockWithIp(ID_SUB_NETWORK, "1.2.2.2")
	dht12 := newDHT(tm12, []*transport.TransportAddress{tm1.TransportAddressWithoutId})
	tm13 := newTransportManagerMockWithIp(ID_SUB_NETWORK, "1.3.3.3")
	dht13 := newDHT(tm13, []*transport.TransportAddress{tm1.TransportAddressWithoutId})

	tm2 := newTransportManagerMockWithIp(ID_TOP_NETWORK, "2.2.2.2")
	dht2 := newDHT(tm2, []*transport.TransportAddress{tm1.TransportAddressWithoutId})
	tm21 := newTransportManagerMockWithIp(ID_SUB_NETWORK, "2.1.1.1")
	dht21 := newDHT(tm21, []*transport.TransportAddress{tm2.TransportAddressWithoutId})
	tm22 := newTransportManagerMockWithIp(ID_SUB_NETWORK, "2.1.1.1")
	dht22 := newDHT(tm22, []*transport.TransportAddress{tm2.TransportAddressWithoutId})

	time.Sleep(10 * time.Second)

	s.Assert().True(dht1.FindTarget(dht2))
	s.Assert().True(dht1.FindTarget(dht11))
	s.Assert().True(dht1.FindTarget(dht21))

	s.Assert().True(dht2.FindTarget(dht1))
	s.Assert().True(dht2.FindTarget(dht12))
	s.Assert().True(dht2.FindTarget(dht22))

	s.Assert().True(dht11.FindTarget(dht1))
	s.Assert().True(dht11.FindTarget(dht2))
	s.Assert().True(dht11.FindTarget(dht12))
	s.Assert().True(dht11.FindTarget(dht13))

	s.Assert().True(dht22.FindTarget(dht1))
	s.Assert().True(dht22.FindTarget(dht2))
	s.Assert().True(dht22.FindTarget(dht21))
}

func (s *DHTTestSuit) TestDataStore() {
	tm1 := newTransportManagerMockWithIp(ID_TOP_NETWORK, "1.1.1.1")
	dht1 := newDHT(tm1, nil)
	tm2 := newTransportManagerMockWithIp(ID_SUB_NETWORK, "1.2.2.2")
	dht2 := newDHT(tm2, []*transport.TransportAddress{tm1.TransportAddressWithoutId})

	time.Sleep(5 * time.Second)
	key1 := deviceid.KeyGen([]byte("key1"))
	key2 := deviceid.KeyGen([]byte("key2"))
	value1 := []byte("value1")
	value2 := []byte("value2")

	err := dht1.Put(key1, value1, nil)
	s.Assert().Equal(err, nil)
	err = dht2.Put(key2, value2, nil)
	s.Assert().Equal(err, nil)

	data, found, err := dht1.Get(key1, true)
	s.Assert().Equal(err, nil)
	s.Assert().True(found)
	s.Assert().Equal(data, value1)

	data, found, err = dht2.Get(key1, true)
	s.Assert().Equal(err, nil)
	s.Assert().True(found)
	s.Assert().Equal(data, value1)

	data, found, err = dht1.Get(key2, true)
	s.Assert().Equal(err, nil)
	s.Assert().True(found)
	s.Assert().Equal(data, value2)

	data, found, err = dht2.Get(key2, true)
	s.Assert().Equal(err, nil)
	s.Assert().True(found)
	s.Assert().Equal(data, value2)

	err = dht1.Delete(key1)
	s.Assert().Equal(err, nil)

	_, found, err = dht1.Get(key1, true)
	s.Assert().Equal(err, nil)
	s.Assert().False(found)

	_, found, err = dht2.Get(key1, true)
	s.Assert().Equal(err, nil)
	s.Assert().False(found)
}
