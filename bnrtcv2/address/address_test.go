package address_test

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/address"
	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/dht/idht"
	"github.com/guabee/bnrtc/bnrtcv2/group"
	"github.com/guabee/bnrtc/bnrtcv2/message"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
)

func newDeviceInfoChangePubSub(info *device_info.DeviceInfo) *util.PubSub {
	pb := util.NewPubSub()
	pb.Publish(rpc.DeviceIdChangeTopic, info.Id)
	pb.Publish(rpc.DeviceInfoChangeTopic, info)
	return pb
}

type DeviceMock struct {
	DisableSend      bool
	deviceInfo       *device_info.DeviceInfo
	TransportManager transport.ITransportManager
	store            map[string][]byte
	messageChannels  map[string]chan (*buffer.Buffer)
}

func (d *DeviceMock) ChooseMatchedAddress(infos []*device_info.DeviceInfo, devId deviceid.DeviceId) (*transport.TransportAddress, error) {
	return d.deviceInfo.ChooseMatchedAddress(infos, devId, d.TransportManager)
}

func (d *DeviceMock) GetDeviceId() deviceid.DeviceId {
	return d.deviceInfo.Id
}

func (d *DeviceMock) GetDeviceInfoById(id deviceid.DeviceId) *device_info.DeviceInfo {
	return nil
}

func (d *DeviceMock) Send(addr *transport.TransportAddress, buf *buffer.Buffer) error {
	if !d.DisableSend {
		return d.nomalSend(addr, buf)
	} else {
		return errors.New("send error")
	}
}

func (d *DeviceMock) SendByInfo(info *device_info.DeviceInfo, buf *buffer.Buffer) error {
	if !d.DisableSend {
		return d.nomalSend(transport.NewAddress(info.Id, nil), buf)
	} else {
		return errors.New("send error")
	}
}

func (d *DeviceMock) nomalSend(addr *transport.TransportAddress, buf *buffer.Buffer) error {
	deviceLock.RLock()
	channel, found := d.messageChannels[addr.DeviceId.String()]
	deviceLock.RUnlock()

	if !found {
		return errors.New("send error")
	}
	channel <- buf
	return nil
}

func (d *DeviceMock) PutData(key deviceid.DataKey, data []byte, options *idht.DHTStoreOptions) error {
	deviceLock.Lock()
	d.store[string(key)] = data
	deviceLock.Unlock()
	return nil
}
func (d *DeviceMock) GetData(key deviceid.DataKey, noCache bool) ([]byte, bool, error) {
	deviceLock.RLock()
	data, found := d.store[string(key)]
	deviceLock.RUnlock()

	return data, found, nil
}

func (d *DeviceMock) UpdateData(key deviceid.DataKey, oldData, newData []byte, options *idht.DHTStoreOptions) error {
	data, found, err := d.GetData(key, false)
	if err != nil {
		return err
	}

	if !found {
		// if not found and oldData is nil, then we can do a put
		if oldData == nil {
			return d.PutData(key, newData, options)
		} else {
			return fmt.Errorf("key %s not exists", key)
		}
	}

	if !bytes.Equal(data, oldData) {
		return fmt.Errorf("update data error")
	}
	return d.PutData(key, newData, options)
}

func (d *DeviceMock) DeleteData(key deviceid.DataKey) error {
	deviceLock.Lock()
	delete(d.store, string(key))
	deviceLock.Unlock()
	return nil
}

func (d *DeviceMock) IsConnected() bool {
	return true
}

type TransportManagerMock struct {
}

func (d *TransportManagerMock) BroadcastDataMsg(addr *transport.TransportAddress, buf *buffer.Buffer, extra []byte) error {
	return nil
}
func (d *TransportManagerMock) AddTransport(transport transport.ITransport) {}
func (d *TransportManagerMock) SendCtrlMsg(addr *transport.TransportAddress, buf *buffer.Buffer) error {

	return nil
}
func (d *TransportManagerMock) SendDataMsg(addr *transport.TransportAddress, buf *buffer.Buffer) error {
	return nil
}
func (d *TransportManagerMock) SendBroadcastMsg(addr *transport.TransportAddress, buf *buffer.Buffer, extra []byte) error {
	return nil
}

func (d *TransportManagerMock) SetDeviceId(id deviceid.DeviceId)          {}
func (d *TransportManagerMock) GetCtrlMsgChan() <-chan *transport.Message { return nil }
func (d *TransportManagerMock) GetDataMsgChan() <-chan *transport.Message { return nil }
func (d *TransportManagerMock) GetBroadcastDataMsgChan() <-chan *transport.BroatcastMessage {
	return nil
}

func (d *TransportManagerMock) GetExternalIp(addr *transport.TransportAddress) (net.IP, error) {
	return nil, nil
}
func (d *TransportManagerMock) IsTargetReachable(addr *transport.TransportAddress) bool {
	if addr.IsPublic() || addr.IsLocal() {
		return true
	} else if addr.Addr.IP.Equal(net.IPv4(192, 168, 1, 1)) {
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

type ManagerContainer struct {
	Id             string
	addressManager *address.AddressManager
	Channel        chan *buffer.Buffer
	Device         *DeviceMock
	stopChan       chan bool
}

var IdIndex int32 = 0
var deviceStore = make(map[string][]byte)
var deviceMessageChannels = make(map[string]chan (*buffer.Buffer))
var deviceLock sync.RWMutex

func newManagerContainer() *ManagerContainer {
	index := atomic.AddInt32(&IdIndex, 1)
	idRandom := make([]byte, 15)
	idRandom[14] = byte(index & 0xff)
	idRandom[13] = byte((index >> 8) & 0xff)
	info := NewDeviceInfo("192.168.1.1", idRandom)
	channel := make(chan *buffer.Buffer)
	deviceMock := &DeviceMock{
		deviceInfo:       info,
		TransportManager: &TransportManagerMock{},
		store:            deviceStore,
		messageChannels:  deviceMessageChannels,
	}

	id := info.Id.String()
	subpub := newDeviceInfoChangePubSub(info)
	groupManager := group.NewManager(
		deviceMock,
		group.WithCheckInfoTime(2*time.Second),
		group.WithCheckLeaderTime(5*time.Second),
		group.WithKeepAliveTime(2*time.Second),
		group.WithPubSub(subpub),
	)
	time.Sleep(10 * time.Millisecond)
	c := &ManagerContainer{
		Id: id,
		addressManager: address.NewManager(
			deviceMock,
			groupManager,
			address.WithPubSub(subpub),
		),
		Channel:  channel,
		Device:   deviceMock,
		stopChan: make(chan bool),
	}

	deviceLock.Lock()
	deviceMessageChannels[id] = channel
	deviceLock.Unlock()

	go func() {
		for {
			select {
			case <-c.stopChan:
				deviceLock.Lock()
				delete(deviceMessageChannels, id)
				deviceLock.Unlock()
				return
			case buf := <-channel:
				msg := message.FromBuffer(buf)
				groupManager.HandlerGroupMessage(msg)
			}
		}
	}()
	return c
}

func (c *ManagerContainer) Close() {
	close(c.stopChan)
}

func TestManagerInfo(t *testing.T) {
	m := newManagerContainer()
	addr1 := "addr1"
	addr2 := "addr2"
	addr3 := "addr3"
	addr4 := "addr4"

	m.addressManager.BindAddress(addr1)
	m.addressManager.BindAddress(addr2)
	m.addressManager.BindAddress(addr3)
	m.addressManager.BindAddress(addr4)

	if len(m.addressManager.GetAddresses()) != 4 {
		t.Errorf("address size %d not correct", 4)
	}

	if !m.addressManager.IsLocalAddress(addr1) {
		t.Errorf("addr1 is not local")
	}

	m.addressManager.UnbindAddress(addr1)
	if len(m.addressManager.GetAddresses()) != 3 {
		t.Errorf("address size %d not correct", 3)
	}

	if m.addressManager.IsLocalAddress(addr1) {
		t.Errorf("addr1 is local")
	}

	if m.addressManager.GetDefaultAddress() != addr4 {
		t.Errorf("default address %s not correct", addr4)
	}

	m.addressManager.UnbindAddress(addr4)
	if m.addressManager.GetDefaultAddress() == "" || m.addressManager.GetDefaultAddress() == addr4 {
		t.Errorf("default address %s not correct", m.addressManager.GetDefaultAddress())
	}
	time.Sleep(1 * time.Second)
}

func TestAddressInfo(t *testing.T) {
	m1 := newManagerContainer()
	m2 := newManagerContainer()
	addr1 := "addr1"
	addr2 := "addr2"

	m1.addressManager.BindAddress(addr1)
	m1.addressManager.BindAddress(addr2)
	time.Sleep(3 * time.Second)
	m2.addressManager.BindAddress(addr1)

	time.Sleep(3 * time.Second)

	info1, _ := m1.addressManager.GetAddressInfo(addr1)
	info2, _ := m1.addressManager.GetAddressInfo(addr2)
	info3, _ := m2.addressManager.GetAddressInfo(addr1)
	info4, _ := m2.addressManager.GetAddressInfo(addr2)
	if info1.Address != addr1 || len(info1.Endpoints) != 2 {
		t.Errorf("address info %s %d not correct", info1.Address, len(info1.Endpoints))
	}
	if info2.Address != addr2 || len(info2.Endpoints) != 1 {
		t.Errorf("address info %s %d not correct", info2.Address, len(info2.Endpoints))
	}
	if info3.Address != addr1 || len(info3.Endpoints) != 2 {
		t.Errorf("address info %s %d not correct", info3.Address, len(info3.Endpoints))
	}
	if info4.Address != addr2 || len(info4.Endpoints) != 1 {
		t.Errorf("address info %s %d not correct", info4.Address, len(info4.Endpoints))
	}

	ta := m2.addressManager.GetTransportAddress(addr1, "", false)
	if ta == nil {
		t.Errorf("GetTransportAddress error")
	}
}

func TestSyncData(t *testing.T) {
	var l []*ManagerContainer
	num := 5
	sendAddr := "**Send-UserData**"
	recvAddr := "**Recv-UserData**"
	dport := "userdata"
	data := "userdata"
	srcId := deviceid.NewDeviceId(true, nil, nil).String()
	var count int32 = 0
	userDataHandler := func(buf *buffer.Buffer) {
		msg := message.FromBuffer(buf)
		if msg == nil {
			return
		}
		atomic.AddInt32(&count, 1)
		if msg.Dport != dport || msg.SrcAddr != sendAddr || msg.DstAddr != recvAddr || msg.SrcDevId != srcId || msg.DstDevId != "" || !msg.Sync {
			t.Errorf("userdata error %s %s %s %s %s %t", msg.Dport, msg.SrcAddr, msg.DstAddr, msg.SrcDevId, msg.DstDevId, msg.Sync)
		}

		if string(msg.Buffer.Data()) != data {
			t.Errorf("userdata error %s", string(msg.Buffer.Data()))
		}
	}

	for i := 0; i < num; i++ {
		c := newManagerContainer()
		c.addressManager.BindAddress(recvAddr)
		c.addressManager.SetUserDataHandler(userDataHandler)
		if i == 0 {
			time.Sleep(3 * time.Second)
		}
		l = append(l, c)
	}
	time.Sleep(5 * time.Second)

	c0 := l[0]
	msg := message.NewMessage(dport, sendAddr, recvAddr, srcId, "", true, buffer.FromBytes([]byte(data)))
	err := c0.addressManager.SyncData(recvAddr, msg.GetBuffer())
	if err != nil {
		t.Errorf("userdata error %s", err)
	}
	time.Sleep(3 * time.Second)

	if atomic.LoadInt32(&count) != int32(num-1) {
		t.Errorf("userdata error %d", count)
	}

	atomic.StoreInt32(&count, 0)

	c1 := l[1]
	msg2 := message.NewMessage(dport, sendAddr, recvAddr, srcId, "", true, buffer.FromBytes([]byte(data)))
	err2 := c1.addressManager.SyncData(recvAddr, msg2.GetBuffer())
	if err2 != nil {
		t.Errorf("userdata error %s", err2)
	}
	time.Sleep(3 * time.Second)

	if atomic.LoadInt32(&count) != int32(num-1) {
		t.Errorf("userdata error %d", count)
	}
}
