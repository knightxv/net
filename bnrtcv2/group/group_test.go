package group_test

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

func (d *DeviceMock) SendByInfo(info *device_info.DeviceInfo, buf *buffer.Buffer) error {
	if !d.DisableSend {
		return d.nomalSend(info.Id, buf)
	} else {
		return errors.New("send error")
	}
}

func (d *DeviceMock) nomalSend(id deviceid.DeviceId, buf *buffer.Buffer) error {
	deviceLock.RLock()
	channel, found := d.messageChannels[id.String()]
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
	Id           string
	groupManager *group.GroupManager
	Channel      chan *buffer.Buffer
	Device       *DeviceMock
	stopChan     chan bool
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
	c := &ManagerContainer{
		Id: id,
		groupManager: group.NewManager(
			deviceMock,
			group.WithCheckInfoTime(2*time.Second),
			group.WithCheckLeaderTime(5*time.Second),
			group.WithKeepAliveTime(2*time.Second),
			group.WithPubSub(newDeviceInfoChangePubSub(info)),
		),
		Channel:  channel,
		Device:   deviceMock,
		stopChan: make(chan bool),
	}
	time.Sleep(10 * time.Millisecond)

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
				c.groupManager.HandlerGroupMessage(msg)
			}
		}
	}()
	return c
}

func (c *ManagerContainer) Close() {
	close(c.stopChan)
	c.groupManager.Close()
}

func (c *ManagerContainer) GetData(data string) []byte {
	return util.Str2bytes(data)
}

func TestGroupManagerInfo(t *testing.T) {
	c1 := newManagerContainer()
	c2 := newManagerContainer()
	c3 := newManagerContainer()
	group1 := "**Group1**"
	group2 := "**Group2**"
	group3 := "**Group3**"
	group4 := "**Group4**"

	// c1: addr1, addr2, addr4; c2: addr1, addr3; c3: addr1, addr2
	c1.groupManager.JoinGroup(group1, c1.GetData("c1"))
	c1.groupManager.JoinGroup(group2, c1.GetData("c1"))
	c2.groupManager.JoinGroup(group3, c2.GetData("c2"))
	// wait leader
	time.Sleep(3 * time.Second)

	c1.groupManager.JoinGroup(group4, c1.GetData("c1"))
	c2.groupManager.JoinGroup(group1, c2.GetData("c2"))
	c3.groupManager.JoinGroup(group1, c3.GetData("c3"))
	c3.groupManager.JoinGroup(group2, c3.GetData("c3"))

	// check
	time.Sleep(4 * time.Second)
	if c1.groupManager.GroupNum() != 3 || c2.groupManager.GroupNum() != 2 || c3.groupManager.GroupNum() != 2 {
		t.Errorf("group len error %d %d %d", c1.groupManager.GroupNum(), c2.groupManager.GroupNum(), c3.groupManager.GroupNum())
	}
	info1, _ := c1.groupManager.GetGroupInfo(group1, false)
	info2, _ := c2.groupManager.GetGroupInfo(group1, false)
	info3, _ := c3.groupManager.GetGroupInfo(group1, false)
	if info1.Id != group1 || info2.Id != group1 || info3.Id != group1 {
		t.Errorf("group %s info error %s %s %s", group1, info1.Id, info2.Id, info3.Id)
	}
	if !info1.GetLeaderId().Equal(deviceid.FromString(c1.Id)) || !info1.GetLeaderId().Equal(info2.GetLeaderId()) || !info1.GetLeaderId().Equal(info3.GetLeaderId()) {
		t.Errorf("group %s leader error %s %s %s", group1, info1.GetLeaderId(), info2.GetLeaderId(), info3.GetLeaderId())
	}
	if len(info1.Members) != 2 || len(info2.Members) != 2 || len(info3.Members) != 2 {
		t.Errorf("group %s member len error %d %d %d", group1, len(info1.Members), len(info2.Members), len(info3.Members))
	}
	info, _ := c1.groupManager.GetGroupInfo(group2, false)
	if info.Id != group2 || !info.GetLeaderId().Equal(deviceid.FromString(c1.Id)) || len(info.Members) != 1 {
		t.Errorf("group %s info error %s %d", group2, info.GetLeaderId(), len(info.Members))
	}
	info, _ = c1.groupManager.GetGroupInfo(group3, false)
	if info.Id != group3 || !info.GetLeaderId().Equal(deviceid.FromString(c2.Id)) || len(info.Members) != 0 {
		t.Errorf("group %s info error %s %d", group2, info.GetLeaderId(), len(info.Members))
	}
	info, _ = c1.groupManager.GetGroupInfo(group4, false)
	if info.Id != group4 || !info.GetLeaderId().Equal(deviceid.FromString(c1.Id)) || len(info.Members) != 0 {
		t.Errorf("group %s info error %s %d", group4, info.GetLeaderId(), len(info.Members))
	}
	info, err := c1.groupManager.GetGroupInfo("NOExist", false)
	if err == nil || info != nil {
		t.Errorf("get no exist address error %s %s", err, info)
	}

	t.Log("info test end")

}

func TestGroupManagerOnlyLeader(t *testing.T) {
	c1 := newManagerContainer()
	group := "**GroupOnlyLeader**"

	c1.groupManager.JoinGroup(group, c1.GetData("c1"))
	time.Sleep(3 * time.Second)
	info, _ := c1.groupManager.GetGroupInfo(group, false)
	if info.GetLeader() == nil || len(info.Members) != 0 {
		t.Errorf("address %s devices len error %d", group, len(info.Members))
	}
	time.Sleep(10 * time.Second)
	info, _ = c1.groupManager.GetGroupInfo(group, false)
	if info.GetLeader() == nil || len(info.Members) != 0 {
		t.Errorf("address %s devices len error %d", group, len(info.Members))
	}
	c1.groupManager.LeaveGroup(group)
	time.Sleep(3 * time.Second)
	info, err := c1.groupManager.GetGroupInfo(group, false)
	if err == nil || info != nil {
		t.Errorf("get no exist address error %s %s", err, info)
	}
}

func TestGroupManagerMemberUnbind(t *testing.T) {
	c1 := newManagerContainer()
	c2 := newManagerContainer()
	c3 := newManagerContainer()
	group := "**GroupMemberUnbind**"

	c1.groupManager.JoinGroup(group, c1.GetData("c1"))
	time.Sleep(3 * time.Second)
	c2.groupManager.JoinGroup(group, c2.GetData("c2"))
	c3.groupManager.JoinGroup(group, c3.GetData("c3"))
	time.Sleep(3 * time.Second)

	c2.groupManager.LeaveGroup(group)
	time.Sleep(2 * time.Second)
	info1, _ := c1.groupManager.GetGroupInfo(group, false)
	info2, _ := c2.groupManager.GetGroupInfo(group, false)
	info3, _ := c3.groupManager.GetGroupInfo(group, false)
	if len(info1.Members) != 1 || len(info2.Members) != 1 || len(info3.Members) != 1 {
		t.Errorf("address %s devices len error %d %d %d", group, len(info1.Members), len(info2.Members), len(info3.Members))
	}

	c2.groupManager.JoinGroup(group, c2.GetData("c2"))
	time.Sleep(2 * time.Second)
	info1, _ = c1.groupManager.GetGroupInfo(group, false)
	info2, _ = c2.groupManager.GetGroupInfo(group, false)
	info3, _ = c3.groupManager.GetGroupInfo(group, false)
	if len(info1.Members) != 2 || len(info2.Members) != 2 || len(info3.Members) != 2 {
		t.Errorf("address %s devices len error %d %d %d", group, len(info1.Members), len(info2.Members), len(info3.Members))
	}
}

func TestGroupManagerMemberSendError(t *testing.T) {
	c1 := newManagerContainer()
	c2 := newManagerContainer()
	c3 := newManagerContainer()
	group := "**GroupMemberSendError**"

	c1.groupManager.JoinGroup(group, c1.GetData("c1"))
	time.Sleep(3 * time.Second)
	c2.groupManager.JoinGroup(group, c2.GetData("c2"))
	c3.groupManager.JoinGroup(group, c3.GetData("c3"))
	time.Sleep(3 * time.Second)

	c2.Device.DisableSend = true
	time.Sleep(12 * time.Second)
	info1, _ := c1.groupManager.GetGroupInfo(group, false)
	info2, _ := c2.groupManager.GetGroupInfo(group, false)
	info3, _ := c3.groupManager.GetGroupInfo(group, false)
	if len(info1.Members) != 1 || len(info2.Members) != 1 || len(info3.Members) != 1 {
		t.Errorf("address %s devices len error %d %d %d", group, len(info1.Members), len(info2.Members), len(info3.Members))
	}

	c2.Device.DisableSend = false
	time.Sleep(12 * time.Second)
	info1, _ = c1.groupManager.GetGroupInfo(group, false)
	info2, _ = c2.groupManager.GetGroupInfo(group, false)
	info3, _ = c3.groupManager.GetGroupInfo(group, false)
	if !info1.GetLeaderId().Equal(info2.GetLeaderId()) || !info1.GetLeaderId().Equal(info3.GetLeaderId()) {
		t.Errorf("address %s leader error %s %s %s", group, info1.GetLeaderId(), info2.GetLeaderId(), info3.GetLeaderId())
	}
	if len(info1.Members) != 2 || len(info2.Members) != 2 || len(info3.Members) != 2 {
		t.Errorf("address %s devices len error %d %d %d", group, len(info1.Members), len(info2.Members), len(info3.Members))
	}
}

func TestGroupManagerLeaderUnbind(t *testing.T) {
	c1 := newManagerContainer()
	c2 := newManagerContainer()
	c3 := newManagerContainer()
	group := "**GroupLeaderUnbind**"

	c1.groupManager.JoinGroup(group, c1.GetData("c1"))
	time.Sleep(3 * time.Second)
	c2.groupManager.JoinGroup(group, c2.GetData("c2"))
	c3.groupManager.JoinGroup(group, c3.GetData("c3"))
	time.Sleep(3 * time.Second)

	c1.groupManager.LeaveGroup(group)
	time.Sleep(16 * time.Second)
	info1, _ := c1.groupManager.GetGroupInfo(group, false)
	info2, _ := c2.groupManager.GetGroupInfo(group, false)
	info3, _ := c3.groupManager.GetGroupInfo(group, false)
	if info1.GetLeaderId().Equal(deviceid.FromString(c1.Id)) || !info1.GetLeaderId().Equal(info2.GetLeaderId()) || !info1.GetLeaderId().Equal(info3.GetLeaderId()) {
		t.Errorf("address %s info error %s %d", group, info1.GetLeaderId(), len(info1.Members))
	}

	if len(info1.Members) != 1 || len(info2.Members) != 1 || len(info3.Members) != 1 {
		t.Errorf("address %s devices len error %d %d %d", group, len(info1.Members), len(info2.Members), len(info3.Members))
	}

	leaderId := info1.GetLeaderId()
	leader := (*ManagerContainer)(nil)
	other := (*ManagerContainer)(nil)
	if leaderId.Equal(deviceid.FromString(c2.Id)) {
		leader = c2
		other = c3
	} else if leaderId.Equal(deviceid.FromString(c3.Id)) {
		leader = c3
		other = c2
	}
	if leader == nil || other == nil {
		t.Errorf("address %s leader error %s", group, leaderId)
		return
	}

	c1.groupManager.JoinGroup(group, c1.GetData("c1"))
	time.Sleep(4 * time.Second)
	info1, _ = c1.groupManager.GetGroupInfo(group, false)
	info2, _ = c2.groupManager.GetGroupInfo(group, false)
	info3, _ = c3.groupManager.GetGroupInfo(group, false)
	if len(info1.Members) != 2 || len(info2.Members) != 2 || len(info3.Members) != 2 {
		t.Errorf("address %s devices len error %d %d %d", group, len(info1.Members), len(info2.Members), len(info3.Members))
	}
}

func TestGroupManagerLeaderSendError(t *testing.T) {
	c1 := newManagerContainer()
	c2 := newManagerContainer()
	c3 := newManagerContainer()
	group := "**AddrLeaderSendError**"

	c1.groupManager.JoinGroup(group, c1.GetData("c1"))
	time.Sleep(3 * time.Second)
	c2.groupManager.JoinGroup(group, c2.GetData("c2"))
	c3.groupManager.JoinGroup(group, c3.GetData("c3"))
	time.Sleep(3 * time.Second)

	c1.Device.DisableSend = true
	time.Sleep(16 * time.Second)
	info1, _ := c1.groupManager.GetGroupInfo(group, false)
	info2, _ := c2.groupManager.GetGroupInfo(group, false)
	info3, _ := c3.groupManager.GetGroupInfo(group, false)
	if info1.GetLeaderId().Equal(deviceid.FromString(c1.Id)) || !info1.GetLeaderId().Equal(info2.GetLeaderId()) || !info1.GetLeaderId().Equal(info3.GetLeaderId()) {
		t.Errorf("address %s info error %s %d", group, info1.GetLeaderId(), len(info1.Members))
	}
	if len(info1.Members) != 1 || len(info2.Members) != 1 || len(info3.Members) != 1 {
		t.Errorf("address %s devices len error %d %d %d", group, len(info1.Members), len(info2.Members), len(info3.Members))
	}
	c1.Device.DisableSend = false
	time.Sleep(16 * time.Second)
	info1, _ = c1.groupManager.GetGroupInfo(group, false)
	info2, _ = c2.groupManager.GetGroupInfo(group, false)
	info3, _ = c3.groupManager.GetGroupInfo(group, false)
	if len(info1.Members) != 2 || len(info2.Members) != 2 || len(info3.Members) != 2 {
		t.Errorf("address %s devices len error %d %d %d", group, len(info1.Members), len(info2.Members), len(info3.Members))
	}
}

func TestGroupManagerOutDate(t *testing.T) {
	c1 := newManagerContainer()
	c2 := newManagerContainer()
	c3 := newManagerContainer()
	c4 := newManagerContainer()
	c5 := newManagerContainer()

	group := "**AddrOutDate**"

	c1.groupManager.JoinGroup(group, c1.GetData("c1"))
	time.Sleep(3 * time.Second)
	c2.groupManager.JoinGroup(group, c2.GetData("c2"))
	c3.groupManager.JoinGroup(group, c3.GetData("c3"))
	time.Sleep(3 * time.Second)

	c1.Close()
	c2.Close()
	c3.Close()
	time.Sleep(5 * time.Second)
	c4.groupManager.JoinGroup(group, c4.GetData("c4"))
	c5.groupManager.JoinGroup(group, c5.GetData("c5"))
	time.Sleep(15 * time.Second)
	info1, _ := c4.groupManager.GetGroupInfo(group, false)
	info2, _ := c5.groupManager.GetGroupInfo(group, false)
	if !info1.GetLeaderId().Equal(deviceid.FromString(c4.Id)) && !info1.GetLeaderId().Equal(deviceid.FromString(c5.Id)) {
		t.Errorf("address %s info error %s", group, info1.GetLeaderId())
	}

	if !info1.GetLeaderId().Equal(info2.GetLeaderId()) {
		t.Errorf("address %s info error %s", group, info1.GetLeaderId())
	}
	if len(info1.Members) != 1 || len(info2.Members) != 1 {
		fmt.Printf("info1 %+v info2 %+v\n", info1.Members, info2.Members)
		t.Errorf("address %s devices len error %d %d", group, len(info1.Members), len(info2.Members))
	}
}

func TestUserData(t *testing.T) {
	var l []*ManagerContainer
	num := 5
	sendAddr := "**Send-UserData**"
	recvAddr := "**Recv-UserData**"
	groupName := "**Group-UserData**"
	dport := "userdata"
	data := "userdata"
	srcId := deviceid.NewDeviceId(true, nil, nil).String()
	var count int32 = 0
	userDataHandler := func(groupId group.GroupId, buf *buffer.Buffer) {
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
		c.groupManager.JoinGroup(groupName, c.GetData("c"))
		c.groupManager.SetGroupUserDataHandler(groupName, userDataHandler)
		if i == 0 {
			time.Sleep(3 * time.Second)
		}
		l = append(l, c)
	}
	time.Sleep(5 * time.Second)

	c0 := l[0]
	msg := message.NewMessage(dport, sendAddr, recvAddr, srcId, "", true, buffer.FromBytes([]byte(data)))
	err := c0.groupManager.SendGroupData(groupName, msg.GetBuffer())
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
	err2 := c1.groupManager.SendGroupData(groupName, msg2.GetBuffer())
	if err2 != nil {
		t.Errorf("userdata error %s", err2)
	}
	time.Sleep(3 * time.Second)

	if atomic.LoadInt32(&count) != int32(num-1) {
		t.Errorf("userdata error %d", count)
	}
}
