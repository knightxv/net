package dchat

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/client"
	conf "github.com/guabee/bnrtc/bnrtcv2/config"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/message"
	"github.com/guabee/bnrtc/bnrtcv2/server/iservice"
	"github.com/guabee/bnrtc/bnrtcv2/server/services"
	"github.com/guabee/bnrtc/bnrtcv2/store"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
)

const (
	DchatServiceName                  = "dchat"
	DchatServiceServerDport           = "dchat-service-server-dport"
	DchatServiceClientDport           = "dchat-service-client-dport"
	DchatServiceStoreMessagePrefix    = "messages-"
	DchatServiceStoreOnlineUserPrefix = "users-"
)

var (
	DchatServiceMessageLimit          uint32 = 1024
	DchatServiceAddressExpireInterval        = 180 * time.Second
	DchatServiceMessageTimeout               = 30 * time.Second
	DchatServiceBindingAddresses             = []string{}
)

func init() {
	services.RegisterServiceFactory(&DchatServiceFactory{})
}

type DchatServiceEvent = uint8

const (
	DchatServiceEventPushMessageResult DchatServiceEvent = iota
)

type DchatServiceEventPushMessageResultData struct {
	success bool
	address string
	keys    []store.StoreKey
}

type DchatServiceEventData struct {
	tm   DchatServiceEvent
	data interface{}
}

type DchatService struct {
	channel          client.ILocalChannel
	serviceManager   iservice.IServiceManager
	serviceInfo      *iservice.ServiceInfo
	onlineUsers      *UserManager
	statusListeners  *ListenerManager
	storageManager   *StorageManager
	stopChan         chan struct{}
	msgChan          chan *DchatMessage
	eventChan        chan *DchatServiceEventData
	magic            uint32
	reqManager       *util.ReqRespManager
	storePath        string
	store            store.IStore
	logger           *log.Entry
	inPushingMessage map[string]bool
}

type DchatServiceFactory struct {
	once    sync.Once
	service *DchatService
}

func (f *DchatServiceFactory) Name() string {
	return DchatServiceName
}

func (f *DchatServiceFactory) LoadOptions(options conf.OptionMap) {
	// services.dchat.limit
	limit := options.GetInt("limit")
	if limit > 0 {
		DchatServiceMessageLimit = uint32(limit)
	}

	// services.dchat.address_expired
	addressExpired := options.GetDuration("address_expired")
	if addressExpired > 0 {
		DchatServiceAddressExpireInterval = addressExpired
	}

	// services.dchat.message_timeout
	messageTimeout := options.GetDuration("message_timeout")
	if messageTimeout > 0 {
		DchatServiceMessageTimeout = messageTimeout
	}

	// services.dchat.addresses
	DchatServiceBindingAddresses = options.GetStringSlice("addresses")
}

func (f *DchatServiceFactory) GetInstance(server iservice.IServer, options conf.OptionMap) (iservice.IService, error) {
	f.once.Do(func() {
		channel := server.NewLocalChannel(DchatServiceName)
		serviceManager := server.GetServiceManager()
		f.LoadOptions(options)
		store := store.NewLevelStore()
		f.service = NewDchatService(channel, store, serviceManager)
	})
	return f.service, nil
}

var serviceId uint32 = 0

func NewDchatService(channel client.ILocalChannel, store store.IStore, serviceManager iservice.IServiceManager) *DchatService {
	s := &DchatService{
		channel:        channel,
		serviceManager: serviceManager,
		serviceInfo: &iservice.ServiceInfo{
			Name:         DchatServiceName,
			Addresses:    DchatServiceBindingAddresses,
			ServiceDport: DchatServiceServerDport,
			ClientDport:  DchatServiceClientDport,
		},
		statusListeners:  NewListenerManager(),
		storageManager:   NewStorageManager(store),
		logger:           log.NewLoggerEntry(DchatServiceName),
		msgChan:          make(chan *DchatMessage, 10240),
		eventChan:        make(chan *DchatServiceEventData, 10),
		stopChan:         make(chan struct{}),
		magic:            util.RandomUint32(),
		reqManager:       util.NewReqRespManager(util.WithReqRespTimeout(DchatServiceMessageTimeout)),
		storePath:        fmt.Sprintf("%s-db-%s-%d", DchatServiceName, conf.BuildName, atomic.AddUint32(&serviceId, 1)),
		store:            store,
		inPushingMessage: make(map[string]bool),
	}
	s.onlineUsers = NewUserManager(s.store, s.doOffline)
	return s
}

func (s *DchatService) Name() string {
	return DchatServiceName
}

func (s *DchatService) getAddress() string {
	if len(DchatServiceBindingAddresses) > 0 {
		return DchatServiceBindingAddresses[0]
	}

	return s.serviceManager.GetAddress()
}

func (s *DchatService) initStore() error {
	err := s.store.Open(s.storePath)
	if err != nil {
		return err
	}

	err = s.store.Load(DchatServiceStoreMessagePrefix, func(key, val []byte) error {
		strKey := string(key)
		keys := strings.Split(strKey, "-")
		if len(keys) != 3 {
			return nil
		}

		address := keys[1]
		idx, err := strconv.ParseUint(keys[2], 10, 32)
		if err != nil {
			idx = 0
		}
		if address == "" {
			return nil
		}

		s.storageManager.VirtualPush(address, uint32(idx))
		return nil
	})

	if err != nil {
		return err
	}

	err = s.store.Load(DchatServiceStoreOnlineUserPrefix, func(key, val []byte) error {
		strKey := string(key)
		keys := strings.Split(strKey, "-")
		if len(keys) != 2 {
			return nil
		}

		address := keys[1]
		if address == "" {
			return nil
		}

		user := NewUser(address, s.store)
		err = json.Unmarshal(val, user)
		if err != nil {
			return nil
		}

		if user.IsExpired() {
			return nil
		}
		s.onlineUsers.SetUser(address, user)
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *DchatService) Start() error {
	err := s.channel.OnMessage(DchatServiceServerDport, s.onDportMessage)
	if err != nil {
		return err
	}

	err = s.serviceManager.RegistService(s.serviceInfo)
	if err != nil {
		return err
	}

	err = s.initStore()
	if err != nil {
		s.logger.Errorf("init store failed: %s", err)
	}

	s.reqManager.Start()
	go s.Run()
	return nil
}

func (s *DchatService) Stop() error {
	close(s.stopChan)
	s.stopChan = nil
	s.reqManager.Close()
	err := s.channel.OffMessage(DchatServiceServerDport, s.onDportMessage)
	return err
}

func (s *DchatService) isClosed() bool {
	return s.stopChan == nil
}

func (s *DchatService) checkMsgChan() bool {
	if s.isClosed() {
		if s.msgChan != nil {
			close(s.msgChan)
			s.msgChan = nil
		}
	}

	return s.msgChan != nil
}

func (s *DchatService) confirmAckMessage(id, magic uint32) {
	if magic != s.magic {
		return
	}
	s.reqManager.ResolveReq(id, nil)
}

func (s *DchatService) sendAckMessage(msg *DchatMessage) error {
	ackMsg := NewDchatMessage(messageType_ack, msg.MsgId, msg.Magic, msg.address, msg.devid, nil)
	return s.channel.Send(s.getAddress(), msg.address, DchatServiceClientDport, msg.devid, ackMsg.GetBuffer().Data())
}

func (s *DchatService) sendDataMessage(mt messageType, dst string, devid string, data []byte, id uint32) error {
	s.logger.Debugf("%s send %s data msg: %s", s.getAddress(), DchatServiceClientDport, mt)
	dataMsg := NewDchatDataMessage(mt, buffer.FromBytes(data))
	msg := NewDchatMessage(messageType_data, id, s.magic, dst, devid, dataMsg.GetBuffer())
	return s.channel.Multicast(s.getAddress(), msg.address, DchatServiceClientDport, msg.devid, msg.GetBuffer().Data())
}

//go:noinline
func (s *DchatService) SendDataMessage(mt messageType, dst string, devid string, data []byte) {
	_ = s.sendDataMessage(mt, dst, devid, data, s.reqManager.GetReqId())
}

//go:noinline
func (s *DchatService) SendDataMessageCallback(mt messageType, dst string, devid string, data []byte, callback util.RequestCallback) {
	req := s.reqManager.GetCallbackReq(callback)
	err := s.sendDataMessage(mt, dst, devid, data, req.Id())
	if err != nil {
		s.reqManager.CancelReq(req.Id())
	}
}

func (s *DchatService) Run() {
	t := time.NewTicker(5 * time.Second)
	for {
		select {
		case msg := <-s.msgChan:
			if msg != nil {
				s.handleMessage(msg)
			}
		case event := <-s.eventChan:
			if event != nil {
				s.handleEvent(event)
			}
		case <-t.C:
			for {
				_, _user, ok := s.onlineUsers.users.Front()
				if !ok {
					break
				}
				user := _user.(*User)
				if user.IsExpired() {
					s.onOffline(user.Address, "")
				} else {
					break
				}
			}
		case <-s.stopChan:
			t.Stop()
			return
		}
	}
}

func (s *DchatService) handleEvent(ev *DchatServiceEventData) {
	switch ev.tm {
	case DchatServiceEventPushMessageResult:
		r := ev.data.(*DchatServiceEventPushMessageResultData)
		if r.success {
			// clear and remove
			s.storageManager.Delete(r.address, r.keys)
		}
		delete(s.inPushingMessage, r.address)
	}
}

func (s *DchatService) handleMessage(msg *DchatMessage) {
	s.logger.Debugf("handle message form(%s/%s): type(%d) msgid(%d) magic(%d)", msg.address, msg.devid, msg.Type, msg.MsgId, msg.Magic)
	s.onOnline(msg.address, msg.devid)
	switch msg.Type {
	case messageType_ack:
		s.confirmAckMessage(msg.MsgId, msg.Magic)
	case messageType_data:
		err := s.sendAckMessage(msg)
		if err != nil {
			s.logger.Errorf("send ack message error: %s", err.Error())
		}
		dataMsg := DchatDataMessageFromBuffer(msg.Buffer)
		s.handleDataMessage(msg.address, msg.devid, dataMsg)
	}
}

func (s *DchatService) handleDataMessage(address string, devid string, msg *DchatDataMessage) {
	s.logger.Debugf("handle data message: %s form %s", msg.Type, address)
	switch msg.Type {
	case messageType_ev_online:
		// already take care online logic, do nothing here
	case messageType_ev_offline:
		s.onOffline(address, devid)
	case messageType_ev_add_friends:
		friends := getStringArray(msg.Buffer)
		s.onAddFriends(address, devid, friends)
	case messageType_ev_del_friends:
		friends := getStringArray(msg.Buffer)
		s.onDelFriends(address, devid, friends)
	case messageType_ev_message:
		dst := msg.Buffer.PeekString(0)
		s.logger.Debugf("save message : %s->%s", address, dst)
		if dst == "" {
			s.logger.Errorf("dst is empty")
			return
		}
		s.onMessage(address, devid, dst, msg.Buffer.DataCopy())
	case messageType_ev_keepalive:
		addressess := getStringArray(msg.Buffer)
		s.onKeepalive(address, devid, addressess)
	default:
		s.logger.Errorf("unknown event: %s", msg.Type)
	}
}

func (s *DchatService) doOffline(address string, friends map[string]struct{}) {
	listener := s.statusListeners.GetListener(address)
	if listener != nil {
		for subscriber := range listener.GetSubscribers() {
			s.SendDataMessage(messageType_ev_offline, subscriber, "", stringArrayToBufferData([]string{address}))
		}
	}

	for friend := range friends {
		s.statusListeners.DelSubscriber(friend, address)
	}
}

func (s *DchatService) onOnline(address string, devid string) {
	user := s.onlineUsers.GetUser(address)
	if user == nil {
		s.logger.Debugf("online: %s", address)
		// send online ack
		s.SendDataMessage(messageType_ev_online_ack, address, devid, stringArrayToBufferData([]string{address}))

		listener := s.statusListeners.GetListener(address)
		if listener != nil {
			for subscriber := range listener.GetSubscribers() {
				if s.onlineUsers.HasUser(subscriber) {
					s.logger.Debugf("send online: %s -> %s", address, subscriber)
					s.SendDataMessage(messageType_ev_online, subscriber, "", stringArrayToBufferData([]string{address}))
				}
			}
		}
	} else {
		if !user.HasEndPoint(devid) {
			s.logger.Debugf("online: %s", address)
			// send online ack
			s.SendDataMessage(messageType_ev_online_ack, address, devid, stringArrayToBufferData([]string{address}))
		}
	}

	s.onlineUsers.AddUser(address, devid)
	s.pushMessage(address, devid)
}

func (s *DchatService) onOffline(address, devid string) {
	if devid == "" {
		s.onlineUsers.ClearUser(address)
	} else {
		s.onlineUsers.DelUser(address, devid)
	}
}

func (s *DchatService) onAddFriends(address string, devid string, friends []string) {
	s.logger.Debugf("add friends: %+v", friends)
	s.onlineUsers.AddFriends(address, devid, friends)
	onlineFriends := make([]string, 0)
	for _, friend := range friends {
		s.statusListeners.AddSubscriber(friend, address)
		if s.onlineUsers.HasUser(friend) {
			onlineFriends = append(onlineFriends, friend)
		}
	}

	if len(onlineFriends) != 0 {
		s.SendDataMessage(messageType_ev_online, address, devid, stringArrayToBufferData(onlineFriends))
	}
}

func (s *DchatService) onDelFriends(address, devid string, friends []string) {
	s.logger.Debugf("del friends: %+v", friends)
	s.onlineUsers.DelFriends(devid, devid, friends)
	for _, friend := range friends {
		s.statusListeners.DelSubscriber(friend, address)
	}
}

func (s *DchatService) onKeepalive(address string, devid string, addresses []string) {
	for _, addr := range addresses {
		s.onOnline(addr, devid)
	}
}

func (s *DchatService) onMessage(address, devid string, dst string, data []byte) {
	if data == nil {
		return
	}

	// @todo limit sender message count
	s.storageManager.Push(dst, data)
	if s.onlineUsers.HasUser(dst) {
		// if dst is online, PushMessage to dst
		s.pushMessage(dst, "" /* devid */)
		s.SendDataMessage(messageType_ev_online, address, devid, stringArrayToBufferData([]string{dst}))
	}
}

func (s *DchatService) pushMessage(address string, devid string) {
	_, found := s.inPushingMessage[address]
	if found { // is pushing message
		return
	}
	keys, messages := s.storageManager.GetMessages(address)
	if len(messages) == 0 {
		return
	}

	s.logger.Debugf("push message: %s", address)
	s.inPushingMessage[address] = true
	messageData := byteArrayToBufferData(messages)
	s.SendDataMessageCallback(messageType_ev_message, address, devid, messageData, func(r *util.Response) {
		s.eventChan <- &DchatServiceEventData{
			tm: DchatServiceEventPushMessageResult,
			data: &DchatServiceEventPushMessageResultData{
				success: r.IsSuccess(),
				address: address,
				keys:    keys,
			},
		}
	})
}

func (s *DchatService) onDportMessage(msg *message.Message) {
	s.logger.Debugf("recv, src: %s, dst: %s, DPort: %s, dataLength: %d", msg.SrcAddr, msg.DstAddr, msg.Dport, len(msg.Buffer.Data()))
	if msg.DstAddr != s.getAddress() {
		s.logger.Errorf("dst address is not match, dst: %s, address: %s", msg.DstAddr, s.getAddress())
		return
	}

	dportMsg := DchatMessageFromBuffer(msg.Buffer)
	if dportMsg == nil {
		s.logger.Errorf("dchat message is invalid")
		return
	}

	if s.checkMsgChan() {
		dportMsg.address = msg.SrcAddr
		dportMsg.devid = msg.SrcDevId
		select {
		case s.msgChan <- dportMsg:
		default:
		}
	}
}
