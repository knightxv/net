package dchat

import (
	"reflect"
	"testing"
	"time"

	"bou.ke/monkey"
	"github.com/guabee/bnrtc/bnrtcv2/client"
	"github.com/guabee/bnrtc/bnrtcv2/server/iservice"
	"github.com/guabee/bnrtc/bnrtcv2/store"
	"github.com/guabee/bnrtc/buffer"
)

type MockLocalChannel struct {
}

func (c *MockLocalChannel) Send(src string, dst string, dport string, devid string, data []byte) error {
	return nil
}
func (c *MockLocalChannel) Multicast(src string, dst string, dport string, devid string, data []byte) error {
	return nil
}
func (c *MockLocalChannel) OnMessage(dport string, handler client.HandlerFunc) error {
	return nil
}
func (c *MockLocalChannel) OffMessage(dport string, handler client.HandlerFunc) error {
	return nil
}

func (c *MockLocalChannel) OnClosed(name string, handler func()) error {
	return nil
}

func (c *MockLocalChannel) Close() {
}

type MockServiceManager struct {
}

func (m *MockServiceManager) Start()                                           {}
func (m *MockServiceManager) BindAddress(address string)                       {}
func (m *MockServiceManager) GetAddress() string                               { return "" }
func (m *MockServiceManager) RegistService(info *iservice.ServiceInfo) error   { return nil }
func (m *MockServiceManager) UnregistService(info *iservice.ServiceInfo) error { return nil }
func (m *MockServiceManager) GetServiceInfo(serviceName string) (iservice.ServiceInfo, error) {
	return iservice.ServiceInfo{}, nil
}

type MockMsg struct {
	tm   messageType
	data []string
}

type MsgContainer struct {
	msgs []*MockMsg
}

func (c *MsgContainer) Push(tm messageType, data []string) {
	c.msgs = append(c.msgs, &MockMsg{tm, data})
}

func (c *MsgContainer) Pop() *MockMsg {
	if len(c.msgs) == 0 {
		return nil
	}
	msg := c.msgs[0]
	c.msgs = c.msgs[1:]
	return msg
}

type MockStorage struct {
	storage map[string][]byte
}

func newMockStorage() *MockStorage {
	return &MockStorage{
		storage: make(map[string][]byte),
	}
}

func (s *MockStorage) Open(path string) error {
	return nil
}

func (s *MockStorage) Load(prefix string, handler store.IStoreLoadHandler) error {
	return nil
}

func (s *MockStorage) Put(items []*store.StoreItem) error {
	for _, item := range items {
		s.storage[string(item.Key)] = item.Val
	}
	return nil
}

func (s *MockStorage) PutOne(key []byte, val []byte) error {
	s.storage[string(key)] = val
	return nil
}

func (s *MockStorage) Delete(keys []store.StoreKey) error {
	for _, key := range keys {
		delete(s.storage, string(key))
	}
	return nil

}

func (s *MockStorage) DeleteOne(key []byte) error {
	delete(s.storage, string(key))
	return nil
}

func (s *MockStorage) Clear(prefix string) error {
	s.storage = make(map[string][]byte)
	return nil
}

func (s *MockStorage) Close() error {
	return nil
}

type DchatServiceContainer struct {
	service *DchatService
}

func NewDchatServiceContainer() *DchatServiceContainer {
	s := &DchatServiceContainer{
		service: NewDchatService(&MockLocalChannel{}, newMockStorage(), &MockServiceManager{}),
	}
	go s.service.Run()
	return s
}

func (c *DchatServiceContainer) Stop() {
	c.service.Stop()
}

func (c *DchatServiceContainer) recvMsg(tm messageType, address string, devid string, data []byte) {
	dataMsg := NewDchatDataMessage(tm, buffer.FromBytes(data))
	msg := NewDchatMessage(messageType_data, 0, 0, address, devid, dataMsg.GetBuffer())
	c.service.handleMessage(msg)
}

func monkeyMock(callback func(mt messageType, address string, devid string, data []byte)) {
	// mask DchatService.sendDataMessage()
	var s *DchatService // Has to be a pointer
	monkey.PatchInstanceMethod(reflect.TypeOf(s), "SendDataMessage", func(_ *DchatService, mt messageType, dst string, devid string, data []byte) {
		callback(mt, dst, devid, data)
	})
}

func TestDchat(t *testing.T) {
	addr1 := "addr1"
	addr2 := "addr2"
	addr3 := "addr3"

	devid1 := "devid1"
	devid2 := "devid2"
	devid3 := "devid3"
	devid4 := "devid4"
	devid5 := "devid5"

	// addr1: {endpoints: {devid1}, friends: {addr2, addr3}}
	// addr2: {endpoints: {devid2, devid4}, friends: {addr1}}
	// addr3: {endpoints: {devid3, devid5},  friends: {addr1 addr2}}
	expectMsgMap := map[string][]*MockMsg{}
	pushExpectMsg := func(addr string, devid string, tm messageType, data []string) {
		if _, ok := expectMsgMap[addr+devid]; !ok {
			expectMsgMap[addr+devid] = make([]*MockMsg, 0)
		}

		expectMsgMap[addr+devid] = append(expectMsgMap[addr+devid], &MockMsg{
			tm:   tm,
			data: data,
		})
	}

	popExpectMsg := func(addr string, devid string) *MockMsg {
		msgs := expectMsgMap[addr+devid]
		if len(msgs) == 0 {
			return nil
		}
		msg := msgs[0]
		expectMsgMap[addr+devid] = expectMsgMap[addr+devid][1:]
		return msg
	}

	monkeyMock(func(mt messageType, address, devid string, data []byte) {
		msg := popExpectMsg(address, devid)
		if msg == nil {
			t.Errorf("no message for %s:%s", address, devid)
			return
		}

		if msg.tm != mt {
			t.Errorf("message type mismatch: %d != %d", msg.tm, mt)
			return
		}

		if !reflect.DeepEqual(data, stringArrayToBufferData(msg.data)) {
			t.Errorf("message data mismatch: %v != %v", data, stringArrayToBufferData(msg.data))
			return
		}
	})

	service := NewDchatServiceContainer()
	pushExpectMsg(addr1, devid1, messageType_ev_online_ack, []string{addr1})
	service.recvMsg(messageType_ev_online, addr1, devid1, stringArrayToBufferData([]string{addr1}))

	// not msg
	service.recvMsg(messageType_ev_add_friends, addr1, devid1, stringArrayToBufferData([]string{addr2, addr3}))

	pushExpectMsg(addr1, "", messageType_ev_online, []string{addr2})
	pushExpectMsg(addr2, devid2, messageType_ev_online_ack, []string{addr2})
	service.recvMsg(messageType_ev_online, addr2, devid2, stringArrayToBufferData([]string{addr2}))

	pushExpectMsg(addr2, devid2, messageType_ev_online, []string{addr1})
	service.recvMsg(messageType_ev_add_friends, addr2, devid2, stringArrayToBufferData([]string{addr1}))

	pushExpectMsg(addr1, "", messageType_ev_online, []string{addr3})
	pushExpectMsg(addr3, devid3, messageType_ev_online_ack, []string{addr3})
	service.recvMsg(messageType_ev_online, addr3, devid3, stringArrayToBufferData([]string{addr3}))

	pushExpectMsg(addr3, devid3, messageType_ev_online, []string{addr1})
	service.recvMsg(messageType_ev_add_friends, addr3, devid3, stringArrayToBufferData([]string{addr1}))

	pushExpectMsg(addr2, devid4, messageType_ev_online_ack, []string{addr2})
	service.recvMsg(messageType_ev_online, addr2, devid4, stringArrayToBufferData([]string{addr2}))

	pushExpectMsg(addr2, devid4, messageType_ev_online, []string{addr1})
	service.recvMsg(messageType_ev_add_friends, addr2, devid4, stringArrayToBufferData([]string{addr1}))

	pushExpectMsg(addr3, devid5, messageType_ev_online_ack, []string{addr3})
	pushExpectMsg(addr1, "", messageType_ev_online, []string{addr3})
	service.recvMsg(messageType_ev_online, addr3, devid5, stringArrayToBufferData([]string{addr3}))

	pushExpectMsg(addr3, devid5, messageType_ev_online, []string{addr2})
	service.recvMsg(messageType_ev_add_friends, addr3, devid5, stringArrayToBufferData([]string{addr2}))

	// no msg
	service.recvMsg(messageType_ev_del_friends, addr1, devid1, stringArrayToBufferData([]string{addr2}))
	// no msg
	service.recvMsg(messageType_ev_del_friends, addr1, devid1, stringArrayToBufferData([]string{addr3}))

	pushExpectMsg(addr2, "", messageType_ev_offline, []string{addr1})
	pushExpectMsg(addr3, "", messageType_ev_offline, []string{addr1})
	service.recvMsg(messageType_ev_offline, addr1, devid1, stringArrayToBufferData([]string{addr1}))

	// no msg
	service.recvMsg(messageType_ev_offline, addr2, devid2, stringArrayToBufferData([]string{addr2}))

	// no msg
	service.recvMsg(messageType_ev_del_friends, addr3, devid3, stringArrayToBufferData([]string{addr1}))
	// no msg
	service.recvMsg(messageType_ev_del_friends, addr3, devid3, stringArrayToBufferData([]string{addr2}))

	service.recvMsg(messageType_ev_offline, addr3, devid3, stringArrayToBufferData([]string{addr3}))

	pushExpectMsg(addr2, "", messageType_ev_offline, []string{addr3})
	service.recvMsg(messageType_ev_offline, addr3, devid5, stringArrayToBufferData([]string{addr3}))

	// no msg
	service.recvMsg(messageType_ev_offline, addr2, devid4, stringArrayToBufferData([]string{addr2}))

}

func TestDchatExpired(t *testing.T) {
	addr1 := "addr1"
	devid1 := "devid1"
	addr2 := "addr2"
	devid2 := "devid2"

	DchatServiceAddressExpireInterval = time.Second * 3
	addr1_online := false
	addr2_online := false
	addr1_offline := false
	addr2_offline := false
	monkeyMock(func(mt messageType, address, devid string, data []byte) {
		if mt == messageType_ev_online_ack {
			if address == addr1 {
				addr1_online = true
			} else if address == addr2 {
				addr2_online = true
			}
		}
		if mt == messageType_ev_offline {
			if reflect.DeepEqual(data, stringArrayToBufferData([]string{addr1})) {
				addr1_offline = true
			} else if reflect.DeepEqual(data, stringArrayToBufferData([]string{addr2})) {
				addr2_offline = true
			}
		}
	})

	service := NewDchatServiceContainer()
	service.recvMsg(messageType_ev_online, addr1, devid1, stringArrayToBufferData([]string{addr1}))
	time.Sleep(time.Second * 1)
	service.recvMsg(messageType_ev_online, addr2, devid2, stringArrayToBufferData([]string{addr2}))
	service.recvMsg(messageType_ev_add_friends, addr2, devid2, stringArrayToBufferData([]string{addr1}))

	time.Sleep(time.Second * 10)
	if !addr1_online || !addr2_online || !addr1_offline || addr2_offline {
		t.Errorf("expect online and offline %v %v %v %v", addr1_online, addr2_online, addr1_offline, addr2_offline)
	}
}
