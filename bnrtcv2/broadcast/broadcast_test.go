package broadcast_test

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"bou.ke/monkey"
	"github.com/guabee/bnrtc/bnrtcv2/broadcast"
	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	kaddht "github.com/guabee/bnrtc/bnrtcv2/dht/kad-dht"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
)

var transportMap = util.NewSafeLinkMap()
var ctrlMap = util.NewSafeLinkMap()
var allCtrls []*BroadcastCtrlContainer

type TransportMessageMock struct {
	SrcId  deviceid.DeviceId
	Buffer *buffer.Buffer
}

type TransportMock struct {
	Id             deviceid.DeviceId
	NetAddr        *net.UDPAddr
	msgChan        chan *TransportMessageMock
	messageHandler transport.TransportMessageHandler
	sendHook       func(src, dst deviceid.DeviceId)
}

func newTransportMock(id deviceid.DeviceId, netAddr *net.UDPAddr) *TransportMock {
	t := &TransportMock{
		Id:      id,
		NetAddr: netAddr,
		msgChan: make(chan *TransportMessageMock, 1024),
	}
	transportMap.Set(id.String(), t)
	go func() {
		for msg := range t.msgChan {
			if t.messageHandler != nil {
				t.messageHandler(transport.NewAddress(msg.SrcId, nil), msg.Buffer)
			}
		}
	}()
	return t
}

func (t *TransportMock) Name() string {
	return "mock"
}

func (t *TransportMock) Type() transport.TransportType {
	return transport.DirectAndForwardType
}

func (t *TransportMock) Send(addr *transport.TransportAddress, buf *buffer.Buffer, ignoreAck bool) error {
	dest, found := transportMap.Get(addr.DeviceId.String())
	if !found {
		return fmt.Errorf("destination not found")
	}

	if t.sendHook != nil {
		t.sendHook(t.Id, addr.DeviceId)
	}

	dest.(*TransportMock).msgChan <- &TransportMessageMock{
		SrcId:  t.Id,
		Buffer: buf,
	}
	return nil
}

func (t *TransportMock) AddSendHook(hook func(src, dst deviceid.DeviceId)) {
	t.sendHook = hook
}

func (t *TransportMock) OnMessage(handler transport.TransportMessageHandler) {
	t.messageHandler = handler
}

func (t *TransportMock) IsTargetReachable(addr *transport.TransportAddress) bool {
	return true
}

func (t *TransportMock) SetDeviceId(id deviceid.DeviceId) {
	t.Id = id
}

func (t *TransportMock) GetExternalIp(addr *transport.TransportAddress) (net.IP, error) {
	return t.NetAddr.IP, nil
}
func (t *TransportMock) SetExternalIp(net.IP) {}

func (t *TransportMock) Close() {
	close(t.msgChan)
	transportMap.Delete(t.Id.String())
}

func addNode(dht interface{}, deviceInfo *device_info.DeviceInfo) {
	dht.(*kaddht.DHT).AddNode(kaddht.NewNode(deviceInfo), !deviceInfo.Id.IsTopNetwork())
}

type BroadcastCtrlContainer struct {
	*broadcast.BroadcastCtrl
	Dht *kaddht.DHT
}

func newDeviceInfo(top bool, ip net.IP, randomValue []byte) *device_info.DeviceInfo {
	if ip == nil {
		ip = randomPublicIp()
	}

	id := deviceid.NewDeviceId(top, ip.To4(), randomValue)
	addr := transport.NewAddress(id, &net.UDPAddr{
		IP:   ip,
		Port: 9999,
	})
	return device_info.BuildDeviceInfo(addr)
}

func newDeviceInfoChangePubSub(info *device_info.DeviceInfo) *util.PubSub {
	pb := util.NewPubSub()
	pb.Publish(rpc.DeviceInfoChangeTopic, info)
	return pb
}

type RecvMsgHookBuilder = func(ctrl *BroadcastCtrlContainer) func(msg *transport.BroatcastMessage)

var testData = ""

func newBroadcastCtrlContainer(top bool, ip net.IP, randomValue []byte, bucketCnt int, sendMsgHook func(src, dst deviceid.DeviceId), recvMsgHookBuilder RecvMsgHookBuilder) *BroadcastCtrlContainer {
	info := newDeviceInfo(top, ip, randomValue)
	t := newTransportMock(info.Id, &info.ToTransportAddress().Addr)

	t.AddSendHook(sendMsgHook)
	pb := newDeviceInfoChangePubSub(info)
	tm := transport.NewTransportManager(transport.WithDevidOption(info.Id), transport.WithPubSubOption(pb))
	tm.AddTransport(t)

	dht, _ := kaddht.NewDHT(&kaddht.Options{
		ID:                  info.Id,
		DeviceInfo:          info,
		BootstrapNodes:      nil,
		AutoRun:             false,
		MaxNodeNumPerBucket: bucketCnt,
	}, tm)

	ctrl := &BroadcastCtrlContainer{
		BroadcastCtrl: broadcast.NewBroadcastCtrl(info, tm, dht, broadcast.WithPubSub(pb)),
		Dht:           dht,
	}
	ctrl.AddMessageHook(recvMsgHookBuilder(ctrl))

	go func() {
		for msg := range tm.GetDataMsgChan() {
			if msg.IsBroadcast {
				// if string(msg.Buffer.Data()) != testData {
				// 	panic("invalid data")
				// }
				fmt.Printf("(%s) -- broadcast--> %s\n", msg.Sender.String(), ctrl.Id.String())
			} else {
				panic("not broadcast")
			}
		}
	}()

	return ctrl
}

func randomPublicIp() net.IP {
	p1 := byte(rand.Intn(0xff))
	p2 := byte(rand.Intn(0xff))
	p3 := byte(rand.Intn(0xff))
	p4 := byte(rand.Intn(0xff))

	ip := net.IPv4(p1, p2, p3, p4)
	if ip.IsGlobalUnicast() && !ip.IsPrivate() {
		return ip
	} else {
		return randomPublicIp()
	}
}

var once sync.Once

func monkeyMock() {
	once.Do(func() {
		// mask SingleTonTask.Run()
		var s *util.SingleTonTask // Has to be a pointer
		monkey.PatchInstanceMethod(reflect.TypeOf(s), "RunWaitWith", func(*util.SingleTonTask, func()) {
			// do Nothing
		})
		var d *kaddht.DHT // Has to be a pointer
		monkey.PatchInstanceMethod(reflect.TypeOf(d), "CheckAndRemoveNode", func(_ *kaddht.DHT, b *kaddht.Bucket) bool {
			return true
		})

		monkey.PatchInstanceMethod(
			reflect.TypeOf(d),
			"IterateClosestNode", func(
				mdht *kaddht.DHT,
				targetId deviceid.DeviceId,
				filter func(*device_info.DeviceInfo) bool,
				callback func(*device_info.DeviceInfo) error) error {
				{
					closestNodeDHT := mdht
					for {
						sl := closestNodeDHT.GetClosestContacts(10, targetId, func(n *kaddht.Node) bool {
							return filter(n.DeviceInfo)
						})
						if len(sl.Nodes()) == 0 {
							break
						}
						closestNode := sl.Nodes()[0]
						if closestNode.Id.GetDistance(targetId).Cmp(closestNodeDHT.DeviceInfo.Id.GetDistance(targetId)) > -1 {
							break
						}
						ctrl, found := ctrlMap.Get(closestNode.Id.String())
						if !found {
							panic("can not found device")
						}
						closestNodeDHT = ctrl.(*BroadcastCtrlContainer).Dht
					}
					if closestNodeDHT.DeviceInfo.Id.Equal(mdht.DeviceInfo.Id) {
						return fmt.Errorf("can not found device")
					}
					callback(closestNodeDHT.DeviceInfo)
					return nil
				}
				// Ideal state

				// {
				// 	var closestNode *device_info.DeviceInfo
				// 	for _, ctrl := range allCtrls {
				// 		if filter != nil && !filter(ctrl.Dht.DeviceInfo) {
				// 			continue
				// 		}
				// 		if closestNode == nil {
				// 			closestNode = ctrl.Dht.DeviceInfo
				// 		} else {
				// 			if ctrl.Id.GetDistance(targetId).Cmp(closestNode.Id.GetDistance(targetId)) == -1 {
				// 				closestNode = ctrl.Dht.DeviceInfo
				// 			}
				// 		}
				// 	}
				// 	if closestNode == nil {
				// 		return fmt.Errorf("can not found device")
				// 	}
				// 	callback(closestNode)
				// 	return nil
				// }
			},
		)

	})
}

func TestBroadcast(t *testing.T) {
	monkeyMock()

	var totalSendCnt int32
	var coverCount int32
	graph := newBroadcastGraph()

	sendMsgHook := func(src, dst deviceid.DeviceId) {
		graph.AddPair(src.String(), dst.String(), true)
		atomic.AddInt32(&totalSendCnt, 1)
	}

	recvMsgHookBuilder := func(ctrl *BroadcastCtrlContainer) func(msg *transport.BroatcastMessage) {
		return func(msg *transport.BroatcastMessage) {
			atomic.AddInt32(&coverCount, 1)
			graph.AddPair(msg.Sender.String(), ctrl.Id.String(), false)
		}
	}

	const sourceNum = 1
	const nodeNum = 15
	const topNetworkNum = sourceNum + nodeNum
	const subNetworkNum = sourceNum + nodeNum*6
	const totalNum = topNetworkNum + subNetworkNum

	ctrl0 := newBroadcastCtrlContainer(true, nil, nil, 20, sendMsgHook, recvMsgHookBuilder)
	ctrl1 := newBroadcastCtrlContainer(false, nil, nil, 20, sendMsgHook, recvMsgHookBuilder)
	topCtrls := [topNetworkNum]*BroadcastCtrlContainer{ctrl0}
	subCtrls := [subNetworkNum]*BroadcastCtrlContainer{ctrl1}
	for i := 0; i < nodeNum; i++ {
		topCtrls[sourceNum+i] = newBroadcastCtrlContainer(true, nil, nil, 20, sendMsgHook, recvMsgHookBuilder)
	}
	for i := 0; i < nodeNum; i++ {
		// group 1
		ip := randomPublicIp()
		subCtrls[sourceNum+0*nodeNum+i] = newBroadcastCtrlContainer(false, ip, nil, 20, sendMsgHook, recvMsgHookBuilder)
		subCtrls[sourceNum+1*nodeNum+i] = newBroadcastCtrlContainer(false, ip, nil, 20, sendMsgHook, recvMsgHookBuilder) // for same group
		subCtrls[sourceNum+2*nodeNum+i] = newBroadcastCtrlContainer(false, ip, nil, 20, sendMsgHook, recvMsgHookBuilder) // for same group
		// group 2
		ip2 := randomPublicIp()
		subCtrls[sourceNum+3*nodeNum+i] = newBroadcastCtrlContainer(false, ip2, nil, 20, sendMsgHook, recvMsgHookBuilder)
		subCtrls[sourceNum+4*nodeNum+i] = newBroadcastCtrlContainer(false, ip2, nil, 20, sendMsgHook, recvMsgHookBuilder) // for same group
		subCtrls[sourceNum+5*nodeNum+i] = newBroadcastCtrlContainer(false, ip2, nil, 20, sendMsgHook, recvMsgHookBuilder) // for same group

	}

	// wait for device ready
	time.Sleep(time.Second)

	addNode(ctrl0.Dht, ctrl1.Info)
	addNode(ctrl1.Dht, ctrl0.Info)
	for i := 0; i < topNetworkNum; i++ {
		src := topCtrls[i]
		for j := 0; j < topNetworkNum; j++ {
			if i != j {
				addNode(src.Dht, topCtrls[j].Info)
			}
		}
		if i > 0 {
			addNode(src.Dht, subCtrls[sourceNum+0*nodeNum+i-1].Info)
			addNode(src.Dht, subCtrls[sourceNum+1*nodeNum+i-1].Info)
			addNode(src.Dht, subCtrls[sourceNum+2*nodeNum+i-1].Info)
			addNode(src.Dht, subCtrls[sourceNum+3*nodeNum+i-1].Info)
			addNode(src.Dht, subCtrls[sourceNum+4*nodeNum+i-1].Info)
			addNode(src.Dht, subCtrls[sourceNum+5*nodeNum+i-1].Info)
		}
	}
	for i := 0; i < subNetworkNum; i++ {
		src := subCtrls[i]
		for k := 0; k < topNetworkNum; k++ {
			addNode(src.Dht, topCtrls[k].Info)
		}
		for j := 0; j < subNetworkNum; j++ {
			if i != j {
				addNode(src.Dht, subCtrls[j].Info)
			}
		}
	}

	allCtrls = append(topCtrls[:], subCtrls[:]...)
	stringBuilder := strings.Builder{}
	for _, ctrl := range allCtrls {
		ctrlMap.Set(ctrl.Id.String(), ctrl)
		ctrl.Dht.DumpNodes(&stringBuilder)
		stringBuilder.WriteString("\n\n")
	}
	_ = os.WriteFile("./nodes.txt", util.Str2bytes(stringBuilder.String()), 0644)

	fmt.Printf("TopnetworkNode %s broadcast:\n", ctrl0.Id)
	testData = "hello"
	_ = ctrl0.Broadcast(buffer.NewBuffer(0, []byte(testData)))
	time.Sleep(2 * time.Second)
	graph.Render("./topnetwork.html")
	if atomic.LoadInt32(&coverCount) != int32(totalNum-1) {
		t.Errorf("cover count: %d != %d", atomic.LoadInt32(&coverCount), totalNum-1)
	}
	t.Logf("TopnetworkNode broadcast cover count: %d, total send count: %d, ratio: %f", atomic.LoadInt32(&coverCount), atomic.LoadInt32(&totalSendCnt), float64(atomic.LoadInt32(&coverCount)*100)/float64(atomic.LoadInt32(&totalSendCnt)))

	atomic.StoreInt32(&totalSendCnt, 0)
	atomic.StoreInt32(&coverCount, 0)
	graph.Reset()
	fmt.Printf("SubnetworkNode %s broadcast:\n", ctrl1.Id)
	testData = "world"
	_ = ctrl1.Broadcast(buffer.NewBuffer(0, []byte(testData)))
	time.Sleep(2 * time.Second)
	graph.Render("./subnetwork.html")
	if atomic.LoadInt32(&coverCount) < int32(totalNum-1) {
		t.Errorf("cover count: %d != %d", atomic.LoadInt32(&coverCount), totalNum-1)
	}
	t.Logf("SubnetworkNode broadcast cover count: %d, total send count: %d, ratio: %f", atomic.LoadInt32(&coverCount), atomic.LoadInt32(&totalSendCnt), float64(atomic.LoadInt32(&coverCount)*100)/float64(atomic.LoadInt32(&totalSendCnt)))
}
func TestBroadcastTop(t *testing.T) {
	monkeyMock()

	var totalSendCnt int32
	var coverCount int32

	sendMsgHook := func(src, dst deviceid.DeviceId) {
		atomic.AddInt32(&totalSendCnt, 1)
	}

	recvMsgHookBuilder := func(ctrl *BroadcastCtrlContainer) func(msg *transport.BroatcastMessage) {
		return func(msg *transport.BroatcastMessage) {
			atomic.AddInt32(&coverCount, 1)
		}
	}

	const sourceNum = 1
	const nodeNum = 1000
	const topNetworkNum = sourceNum + nodeNum
	ctrl0 := newBroadcastCtrlContainer(true, nil, nil, 2, sendMsgHook, recvMsgHookBuilder)
	topCtrls := [topNetworkNum]*BroadcastCtrlContainer{ctrl0}
	for i := 0; i < nodeNum; i++ {
		topCtrls[sourceNum+i] = newBroadcastCtrlContainer(true, nil, nil, 2, sendMsgHook, recvMsgHookBuilder)
	}
	// wait for device ready
	time.Sleep(time.Second)

	for i := 0; i < topNetworkNum; i++ {
		src := topCtrls[i]
		for j := 0; j < topNetworkNum; j++ {
			if i != j {
				addNode(src.Dht, topCtrls[j].Info)
			}
		}
	}
	allCtrls = topCtrls[:]
	stringBuilder := strings.Builder{}
	for _, ctrl := range topCtrls {
		ctrlMap.Set(ctrl.Id.String(), ctrl)
		ctrl.Dht.DumpNodes(&stringBuilder)
		stringBuilder.WriteString("\n\n")
	}
	_ = os.WriteFile("./nodes-top.txt", util.Str2bytes(stringBuilder.String()), 0644)

	fmt.Printf("TopnetworkNode %s broadcast:\n", ctrl0.Id)
	testData = "hello world"
	_ = ctrl0.Broadcast(buffer.NewBuffer(0, []byte(testData)))
	time.Sleep(nodeNum / 100 * 2 * time.Second)
	if atomic.LoadInt32(&coverCount) != int32(nodeNum) {
		t.Errorf("cover count: %d != %d", atomic.LoadInt32(&coverCount), nodeNum)
	}
	t.Logf("TopnetworkNode broadcast cover count: %d, total send count: %d, ratio: %f", atomic.LoadInt32(&coverCount), atomic.LoadInt32(&totalSendCnt), float64(atomic.LoadInt32(&coverCount)*100)/float64(atomic.LoadInt32(&totalSendCnt)))
}
