package device_test

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/device"
	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/dht/idht"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
)

type TransportManagerMock struct {
	Id                   deviceid.DeviceId
	filter               func(addr *transport.TransportAddress) bool
	dataMsgChan          chan *transport.Message
	ctrlMsgChan          chan *transport.Message
	broadcastDataMsgChan chan *transport.BroatcastMessage
	ip                   net.IP
}

func newTransportManagerMock(Id deviceid.DeviceId) *TransportManagerMock {
	return &TransportManagerMock{
		Id:                   Id,
		dataMsgChan:          make(chan *transport.Message, 10),
		ctrlMsgChan:          make(chan *transport.Message, 10),
		broadcastDataMsgChan: make(chan *transport.BroatcastMessage, 10),
	}
}
func (d *TransportManagerMock) AddTransport(transport transport.ITransport) {}
func (d *TransportManagerMock) SendCtrlMsg(addr *transport.TransportAddress, buf *buffer.Buffer) error {
	d.ctrlMsgChan <- &transport.Message{
		Target:    d.Id,
		Buffer:    buf,
		Timestamp: time.Now(),
	}

	return nil
}
func (d *TransportManagerMock) SendDataMsg(addr *transport.TransportAddress, buf *buffer.Buffer) error {
	d.dataMsgChan <- &transport.Message{
		Target:    d.Id,
		Buffer:    buf,
		Timestamp: time.Now(),
	}
	return nil
}

func (d *TransportManagerMock) SendBroadcastMsg(addr *transport.TransportAddress, buf *buffer.Buffer, extra []byte) error {
	return nil
}

func (d *TransportManagerMock) BroadcastDataMsg(addr *transport.TransportAddress, buf *buffer.Buffer, extra []byte) error {
	return nil
}

func (d *TransportManagerMock) GetCtrlMsgChan() <-chan *transport.Message {
	return d.ctrlMsgChan
}

func (d *TransportManagerMock) GetDataMsgChan() <-chan *transport.Message {
	return d.dataMsgChan
}

func (d *TransportManagerMock) GetBroadcastDataMsgChan() <-chan *transport.BroatcastMessage {
	return d.broadcastDataMsgChan
}

func (d *TransportManagerMock) SetDeviceId(id deviceid.DeviceId) {}

func (d *TransportManagerMock) SetExternalIp(ip net.IP) {
	d.ip = ip
}

func (d *TransportManagerMock) GetExternalIp(addr *transport.TransportAddress) (net.IP, error) {
	return d.ip, nil
}
func (d *TransportManagerMock) SetAddrFilter(filter func(addr *transport.TransportAddress) bool) {
	d.filter = filter
}
func (d *TransportManagerMock) IsTargetReachable(addr *transport.TransportAddress) bool {
	if d.filter != nil {
		return d.filter(addr)
	}

	if addr.IsPublic() || addr.IsLocal() {
		return true
	} else if strings.HasPrefix(addr.Addr.IP.String(), "192.168.1") {
		return true
	}
	return false
}

func NewDeviceInfo(ipstr string, randomValue []byte) *device_info.DeviceInfo {
	ip := net.ParseIP(ipstr)

	info := &device_info.DeviceInfo{
		Id:        deviceid.NewDeviceId(true, ip.To4(), randomValue),
		Iterfaces: make([][]*device_info.Interface, device_info.InterfaceTypeMax),
	}
	if ip.IsPrivate() {
		info.Iterfaces[device_info.Lan] = append(info.Iterfaces[device_info.Lan], &device_info.Interface{
			IP:   ip,
			Port: 9999,
			Mask: net.IPv4Mask(255, 255, 255, 0),
		})
	} else {
		info.Iterfaces[device_info.Ipv4] = append(info.Iterfaces[device_info.Ipv4], &device_info.Interface{
			IP:   ip,
			Port: 9999,
			Mask: net.IPv4Mask(255, 255, 255, 0),
		})
	}
	return info
}

type DHTMock struct {
	DeviceInfo *device_info.DeviceInfo
	Peers      []*device_info.DeviceInfo
	Store      map[string][]byte
}

func (d *DHTMock) RefreshStore(key deviceid.DataKey) error {
	return nil
}

func (d *DHTMock) SetDeviceInfo(devInfo *device_info.DeviceInfo) {
	d.DeviceInfo = devInfo
}
func (d *DHTMock) GetDeviceInfo(id deviceid.DeviceId) *device_info.DeviceInfo {
	return nil
}
func (d *DHTMock) IterateBucketIdx(filter func(*device_info.DeviceInfo) bool, callback func(int, int, *device_info.DeviceInfo) error) {
}
func (d *DHTMock) IterateClosestNode(targetId deviceid.DeviceId, filter func(*device_info.DeviceInfo) bool, callback func(*device_info.DeviceInfo) error) error {
	return nil
}

func (d *DHTMock) Put(key deviceid.DataKey, data []byte, options *idht.DHTStoreOptions) error {
	d.Store[key.String()] = data
	return nil
}

func (d *DHTMock) Get(key deviceid.DataKey, noCache bool) (data []byte, found bool, err error) {
	data, found = d.Store[key.String()]
	return
}

func (d *DHTMock) Update(key deviceid.DataKey, oldData, newData []byte, options *idht.DHTStoreOptions) (err error) {
	v, found := d.Store[key.String()]
	if !found {
		if oldData == nil {
			return d.Put(key, newData, options)
		} else {
			return fmt.Errorf("not found")
		}
	}
	if !bytes.Equal(oldData, v) {
		return fmt.Errorf("old data not equal")
	}

	return d.Put(key, newData, options)
}

func (d *DHTMock) Delete(key deviceid.DataKey) error {
	delete(d.Store, key.String())
	return nil
}

func (d *DHTMock) AddPeer(info *device_info.DeviceInfo) {
	d.Peers = append(d.Peers, info)
}
func (d *DHTMock) GetClosestTopNetworkDeviceInfo() *device_info.DeviceInfo {
	return nil
}

func (d *DHTMock) IsConnected() bool {
	return true
}

func (d *DHTMock) GetNeighborDeviceInfo(num int) []*device_info.DeviceInfo {
	return nil
}

func NewDHTMock(option *idht.DHTOption) idht.IDHT {
	dht := &DHTMock{
		DeviceInfo: option.DeviceInfo,
		Store:      make(map[string][]byte),
	}

	for _, peer := range option.Peers {
		info := device_info.BuildDeviceInfo(peer)
		if info != nil {
			dht.AddPeer(info)
		}
	}
	return dht
}

type ConnectionManagerMock struct {
}

func (d *ConnectionManagerMock) Connect(ta *transport.TransportAddress) error { return nil }

func TestNewDevice(t *testing.T) {
	info := NewDeviceInfo("192.168.1.1", nil)
	id := info.Id

	pubsub := util.DefaultPubSub()
	pubsub.Publish(rpc.DeviceInfoChangeTopic, info)
	pubsub.Publish(rpc.DeviceIdChangeTopic, id)
	tm := newTransportManagerMock(id)
	device := device.NewDevice(tm, &ConnectionManagerMock{}, NewDHTMock, device.WithPubSub(pubsub))
	onDeviceIdChanged := false
	_, _ = device.Pubsub.SubscribeFunc("TestNewDevice", rpc.DeviceIdChangeTopic, func(msg util.PubSubMsgType) {
		devId := msg.(deviceid.DeviceId)
		onDeviceIdChanged = true
		if !device.GetDeviceId().Equal(devId) {
			t.Errorf("device id is not correct%s<-> %s", device.GetDeviceId(), devId)
		}
	})

	time.Sleep(time.Second)
	if !onDeviceIdChanged {
		t.Errorf("device id is not correct")
	}

	if !device.GetDeviceId().Equal(id) {
		t.Errorf("device id is not correct")
	}

	info1 := NewDeviceInfo("192.168.0.1", nil)
	info2 := NewDeviceInfo("1.1.1.1", nil)
	info3 := NewDeviceInfo("192.168.1.2", nil)
	info4 := NewDeviceInfo("1.1.1.2", nil)
	info5 := NewDeviceInfo("192.168.0.2", nil)
	id1 := info1.Id
	id2 := info2.Id
	id3 := info3.Id
	infos1 := []*device_info.DeviceInfo{info1, info2, info3, info4, info5}
	infos2 := []*device_info.DeviceInfo{info5, info4, info3, info2, info1}
	tm.SetAddrFilter(func(addr *transport.TransportAddress) bool {
		if addr.DeviceId.Equal(id1) {
			return true
		}

		if addr.IsPublic() || addr.IsLocal() {
			return true
		}

		if strings.HasPrefix(addr.Addr.IP.String(), "192.168.1") {
			return true
		}
		return false
	})

	addr1, err := device.ChooseMatchedAddress(infos1, nil)
	if err != nil || addr1 == nil {
		t.Errorf("device id is not correct")
	}

	addr1, err = device.ChooseMatchedAddress(infos2, nil)
	if err != nil || addr1 == nil {
		t.Errorf("device id is not correct")
	}

	addr2, err := device.ChooseMatchedAddress(infos1, id2)
	if err != nil || addr2 == nil || !addr2.DeviceId.Equal(id2) {
		t.Errorf("device id is not correct")
	}

	addr3, err := device.ChooseMatchedAddress(infos2, id3)
	if err != nil || addr3 == nil || !addr3.DeviceId.Equal(id3) {
		t.Errorf("device id is not correct")
	}

	_, err = device.ChooseMatchedAddress(infos2, id)
	if err == nil {
		t.Errorf("device id is not correct")
	}

	msgStr := "hello"
	msgReceived := false
	device.OnMessage(func(msg *transport.Message) {
		msgReceived = true
		if !msg.Target.Equal(id) || util.Bytes2str(msg.GetBuffer().Data()) != msgStr {
			t.Errorf("message is not correct")
		}
	})
	buf := buffer.FromBytes([]byte(msgStr))
	_ = device.Send(addr1, buf)
	time.Sleep(time.Second)
	if !msgReceived {
		t.Errorf("message is not correct")
	}

	data1 := []byte("world")
	data2 := []byte("world2")
	data3 := []byte("world3")
	key1 := deviceid.DataKey(data1)
	key2 := deviceid.DataKey(data2)
	key3 := deviceid.DataKey(data3)

	_ = device.PutData(key1, data1, nil)
	_ = device.PutData(key2, data2, nil)
	data_1, found, err := device.GetData(key1, false)
	if err != nil || !found || !bytes.Equal(data_1, data1) {
		t.Errorf("data is not correct")
	}
	data_2, found, err := device.GetData(key2, false)
	if err != nil || !found || !bytes.Equal(data_2, data2) {
		t.Errorf("data is not correct")
	}
	_, found, err = device.GetData(key3, false)
	if err != nil || found {
		t.Errorf("data is not correct")
	}
	_ = device.DeleteData(key1)
	_, found, err = device.GetData(key1, false)
	if err != nil || found {
		t.Errorf("data is not correct")
	}

	ip := net.ParseIP("2.2.2.2")
	tm.SetExternalIp(ip)
	if device.GetExternalIp() != nil {
		t.Errorf("external ip is not correct")
	}
	device.AddPeers([]string{"1.1.1.1:9999"})
	if !device.GetExternalIp().Equal(ip) {
		t.Errorf("external ip is not correct")
	}
}
