package transport_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/guabee/bnrtc/bnrtcv1"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
)

func newDeviceIdPubSub(id deviceid.DeviceId) *util.PubSub {
	pubsub := util.NewPubSub()
	pubsub.Publish(rpc.DeviceIdChangeTopic, id)
	return pubsub
}

func newRPCCenter(handler func() *transport.TransportAddress) *util.RPCCenter {
	center := util.NewRPCCenter()
	center.Regist(rpc.GetSignalServerAddrRpcName, func(params ...util.RPCParamType) util.RPCResultType {
		return handler()
	})
	return center
}

func getPeerServiceOptions(id deviceid.DeviceId, port int32) *bnrtcv1.Bnrtc1ServerOptions {
	return &bnrtcv1.Bnrtc1ServerOptions{
		Name:              id.NodeId(),
		LocalNatName:      id.NodeId(),
		HttpPort:          port,
		StunPort:          port + 1,
		DisableTurnServer: true,
		DisableStunServer: false,
	}
}

type ConnetctionManagerContainer struct {
	*transport.Bnrtc1ConnectionManager
	DevId deviceid.DeviceId
	Addr  *transport.TransportAddress
}

func newConnectionManagerContainer(port int, signalAddr *transport.TransportAddress, options ...transport.Bnrtc1ManagerOption) *ConnetctionManagerContainer {
	id := deviceid.NewDeviceId(signalAddr == nil, []byte{127, 0, 0, 1}, nil)
	fmt.Println("id:", id, id.NodeId(), "port:", port)
	options = append(options, transport.WithPubSub(newDeviceIdPubSub(id)))
	if signalAddr != nil {
		options = append(options, transport.WithRPCCenter(newRPCCenter(func() *transport.TransportAddress {
			return signalAddr
		})))
	}

	m := transport.NewBnrtc1Manager(
		id,
		getPeerServiceOptions(id, int32(port)),
		options...,
	).GetConnectionManager()
	var addr *transport.TransportAddress
	if signalAddr != nil {
		addr = transport.NewAddress(id, &signalAddr.Addr)
	} else {
		addr = transport.NewAddress(id, &net.UDPAddr{
			IP:   net.IPv4(127, 0, 0, 1),
			Port: port,
		})
	}

	return &ConnetctionManagerContainer{
		Bnrtc1ConnectionManager: m,
		DevId:                   id,
		Addr:                    addr,
	}
}

type SignalManagerContainer struct {
	*transport.Bnrtc1SignalManager
	DevId deviceid.DeviceId
	Addr  *transport.TransportAddress
}

func newSignalManagerContainer(port int, signalAddr *transport.TransportAddress, options ...transport.Bnrtc1ManagerOption) *SignalManagerContainer {
	id := deviceid.NewDeviceId(signalAddr == nil, []byte{127, 0, 0, 1}, nil)
	fmt.Println("id:", id, id.NodeId(), "port:", port)
	options = append(options, transport.WithPubSub(newDeviceIdPubSub(id)))
	if signalAddr != nil {
		options = append(options, transport.WithRPCCenter(newRPCCenter(func() *transport.TransportAddress {
			return signalAddr
		})))
	}

	m := transport.NewBnrtc1Manager(
		id,
		getPeerServiceOptions(id, int32(port)),
		options...,
	).GetSignalManager()
	var addr *transport.TransportAddress
	if signalAddr != nil {
		addr = transport.NewAddress(id, &signalAddr.Addr)
	} else {
		addr = transport.NewAddress(id, &net.UDPAddr{
			IP:   net.IPv4(127, 0, 0, 1),
			Port: port,
		})
	}

	return &SignalManagerContainer{
		Bnrtc1SignalManager: m,
		DevId:               id,
		Addr:                addr,
	}
}

func TestTransportBnrtc1ConnectionManager(t *testing.T) {
	// peer2 <---> peer1(SignalServer) <---> peer3
	m1 := newConnectionManagerContainer(4444, nil)
	m2 := newConnectionManagerContainer(5555, m1.Addr)
	m3 := newConnectionManagerContainer(6666, m1.Addr)
	connectionConnectedCnt := 0
	connectionDisconnectedCnt := 0
	connectionHandler := func(ids map[string]bool, conn transport.IConnection) {
		connectionConnectedCnt++
		_, found := ids[conn.Name()]
		if !found {
			t.Errorf("wrong conn id %s expected %v ", conn.Name(), ids)
		} else {
			delete(ids, conn.Name())
		}
	}
	disConnectionHandler := func(ids map[string]bool, conn transport.IConnection) {
		connectionDisconnectedCnt++
		_, found := ids[conn.Name()]
		if !found {
			t.Errorf("wrong disconn id %s expected %v ", conn.Name(), ids)
		} else {
			delete(ids, conn.Name())
		}
	}
	genPeerIds := func(ids []string, exclude string) map[string]bool {
		m := make(map[string]bool)
		for _, id := range ids {
			if id != exclude {
				m[id] = true
			}
		}
		return m
	}

	ids := []string{m1.DevId.NodeId(), m2.DevId.NodeId(), m3.DevId.NodeId()}
	ms := [](*ConnetctionManagerContainer){m1, m2, m3}
	for _, m := range ms {
		realM := m
		m.OnConnection(func(conn transport.IConnection) {
			ids := genPeerIds(ids, realM.DevId.NodeId())
			fmt.Println("conn:", conn.Name(), realM.DevId.NodeId(), ids)
			connectionHandler(ids, conn)
		})
		m.OnDisconnection(func(conn transport.IConnection) {
			ids := genPeerIds(ids, realM.DevId.NodeId())
			fmt.Println("disconn:", conn.Name(), realM.DevId.NodeId(), ids)
			disConnectionHandler(ids, conn)
		})
	}

	time.Sleep(4 * time.Second)
	if m2.IsConnected(m3.DevId) {
		t.Error("wrong connection state")
	}
	if m2.IsConnected(m1.DevId) {
		t.Error("wrong connection state")
	}

	_ = m2.Connect(m3.Addr)
	if !m2.IsConnected(m3.DevId) {
		t.Error("wrong connection state")
	}
	_ = m2.Connect(m1.Addr)
	if m2.IsConnected(m1.DevId) {
		t.Error("wrong connection state")
	}
	m2.Disconnect(m1.DevId)
	m2.Disconnect(m3.DevId)

	time.Sleep(1 * time.Second)
	if m2.IsConnected(m1.DevId) {
		t.Error("wrong connection state")
	}

	if m2.IsConnected(m3.DevId) {
		t.Error("wrong connection state")
	}

	// check
	if connectionConnectedCnt != 2 {
		t.Errorf("wrong connection connected count %d", connectionConnectedCnt)
	}
	if connectionDisconnectedCnt != 2 {
		t.Errorf("wrong connection disconnected count %d", connectionDisconnectedCnt)
	}
}

func TestTransportBnrtc1ManagerLimitAndExpired(t *testing.T) {
	// peer1 <-> peer0 <--> peer2-peer9
	port := 5050
	connectionCnt := 10
	expireConnectionCnt := 0
	m0 := newConnectionManagerContainer(port, nil)
	var m1Addr *transport.TransportAddress = nil
	cnt := 0
	newConnectionManager := func() {
		m := newConnectionManagerContainer(port+cnt+1, m0.Addr,
			transport.WithConnectionLimit(connectionCnt),
			transport.WithConnectionExpired(10*time.Second),
			transport.WithCheckSignalServerTime(time.Second),
		)
		if m1Addr == nil {
			m1Addr = m.Addr
			m.OnDisconnection(func(conn transport.IConnection) {
				expireConnectionCnt++
			})
		} else {
			time.Sleep(50 * time.Millisecond)
			_ = m.Connect(m1Addr)
		}
		cnt++
	}

	firstConnectCnt := connectionCnt / 2
	for i := 0; i < firstConnectCnt+1; i++ {
		newConnectionManager()
	}

	go func() {
		time.Sleep(5 * time.Second)
		for i := 0; i < connectionCnt-firstConnectCnt; i++ {
			newConnectionManager()
		}
	}()

	time.Sleep(12 * time.Second)
	if expireConnectionCnt != firstConnectCnt {
		t.Errorf("wrong expire connection count %d", expireConnectionCnt)
	}

	time.Sleep(6 * time.Second)
	if expireConnectionCnt != connectionCnt {
		t.Errorf("wrong expire connection count %d", expireConnectionCnt)
	}
}

func TestTransportBnrtc1lSend(t *testing.T) {
	// peer2 <---> peer1(SignalServer) <--> peer3
	m1 := newConnectionManagerContainer(5444, nil)
	m2 := newConnectionManagerContainer(6444, m1.Addr)
	m3 := newConnectionManagerContainer(7444, m1.Addr)
	type MsgInfo struct {
		Transport *transport.Bnrtc1Transport
		Manager   *ConnetctionManagerContainer
		Msg       []byte
		SendCnt   int
		RecvCnt   int
	}

	t2, _ := transport.NewBnrtc1Transport(m2.Bnrtc1ConnectionManager)
	t3, _ := transport.NewBnrtc1Transport(m3.Bnrtc1ConnectionManager)
	var msgInfos []*MsgInfo = []*MsgInfo{
		{Transport: t2, Manager: m2, Msg: []byte("msg2")},
		{Transport: t3, Manager: m3, Msg: []byte("msg3")},
	}
	for _, info := range msgInfos {
		realInfo := info
		info.Transport.OnMessage(func(addr *transport.TransportAddress, buf *buffer.Buffer) error {
			if string(buf.Data()) != string(realInfo.Msg) {
				t.Errorf("%s Failed to recv message", realInfo.Manager.DevId)
			}
			realInfo.RecvCnt++
			return nil
		})
	}

	time.Sleep(3 * time.Second)
	m2.Connect(m3.Addr)

	if t2.Name() != "bnrtc1" {
		t.Error("Failed to get transport name")
	}
	if t2.Type() != transport.DirectType {
		t.Error("Failed to get transport type")
	}

	time.Sleep(4 * time.Second)
	for _, sender := range msgInfos {
		for _, receiver := range msgInfos {
			if sender == receiver {
				continue
			}
			err := sender.Transport.Send(receiver.Manager.Addr, buffer.FromBytes(receiver.Msg), false)
			if err != nil {
				t.Errorf("%s Failed to send message to %s, error: %s", sender.Manager.DevId, receiver.Manager.DevId, err.Error())
			} else {
				sender.SendCnt++
			}
		}
	}

	time.Sleep(2 * time.Second)
	for _, info := range msgInfos {
		if info.SendCnt != 1 {
			t.Errorf("%s Failed to send message %d", info.Manager.DevId, info.SendCnt)
		}
		if info.RecvCnt != 1 {
			t.Errorf("%s Failed to recv message %d", info.Manager.DevId, info.RecvCnt)
		}
	}
}

func TestTransportSignalSend(t *testing.T) {
	// peer2 <---> peer1(SignalServer) <--> peer3
	m1 := newSignalManagerContainer(6666, nil)
	m2 := newSignalManagerContainer(7666, m1.Addr)
	m3 := newSignalManagerContainer(8666, m1.Addr)
	type MsgInfo struct {
		Transport *transport.SignalTransport
		Manager   *SignalManagerContainer
		Msg       []byte
		SendCnt   int
		RecvCnt   int
	}

	t1, _ := transport.NewSignalTransport(m1.Bnrtc1SignalManager)
	t2, _ := transport.NewSignalTransport(m2.Bnrtc1SignalManager)
	t3, _ := transport.NewSignalTransport(m3.Bnrtc1SignalManager)
	var msgInfos []*MsgInfo = []*MsgInfo{
		{Transport: t1, Manager: m1, Msg: []byte("msg1")},
		{Transport: t2, Manager: m2, Msg: []byte("msg2")},
		{Transport: t3, Manager: m3, Msg: []byte("msg3")},
	}
	for _, info := range msgInfos {
		realInfo := info
		info.Transport.OnMessage(func(addr *transport.TransportAddress, buf *buffer.Buffer) error {
			if string(buf.Data()) != string(realInfo.Msg) {
				t.Errorf("%s Failed to recv message", realInfo.Manager.DevId)
			}
			realInfo.RecvCnt++
			return nil
		})
	}

	time.Sleep(3 * time.Second)
	if t2.Name() != "signal" {
		t.Error("Failed to get transport name")
	}
	if t2.Type() != transport.DirectAndForwardType {
		t.Error("Failed to get transport type")
	}

	time.Sleep(4 * time.Second)
	for _, sender := range msgInfos {
		for _, receiver := range msgInfos {
			if sender == receiver {
				continue
			}

			err := sender.Transport.Send(receiver.Manager.Addr, buffer.FromBytes(receiver.Msg), false)
			if err == nil {
				sender.SendCnt++
			}
		}
	}

	time.Sleep(2 * time.Second)
	for _, info := range msgInfos {
		if info.Transport == t1 {
			if info.SendCnt != 2 {
				t.Errorf("%s Failed to send message %d", info.Manager.DevId, info.SendCnt)
			}
			if info.RecvCnt != 2 {
				t.Errorf("%s Failed to recv message %d", info.Manager.DevId, info.RecvCnt)
			}
		} else {
			if info.SendCnt != 1 {
				t.Errorf("%s Failed to send message %d", info.Manager.DevId, info.SendCnt)
			}
			if info.RecvCnt != 1 {
				t.Errorf("%s Failed to recv message %d", info.Manager.DevId, info.RecvCnt)
			}
		}
	}
}
