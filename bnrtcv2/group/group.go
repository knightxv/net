package group

import (
	"fmt"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/dht/idht"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/message"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
)

const (
	CheckInfoIntervalDefault   = 5 * time.Second
	KeepAliveIntervalDefault   = 5 * time.Second
	KeepAliveTimeoutDefault    = 3 * KeepAliveIntervalDefault
	CheckLeaderIntervalDefault = 30 * time.Second
	RepublishIntervalDefault   = 5 * 60 * time.Second
	DataExpireIntervalDefault  = 6 * 60 * time.Second
	NoCache                    = true
	Cache                      = false
	ModuleName                 = "group"
)

type IDevice interface {
	IsConnected() bool
	SendByInfo(info *device_info.DeviceInfo, buf *buffer.Buffer) error
	UpdateData(key deviceid.DataKey, oldData, newData []byte, options *idht.DHTStoreOptions) error
	GetData(key deviceid.DataKey, noCache bool) ([]byte, bool, error)
	DeleteData(key deviceid.DataKey) error
}

type GroupEvent uint8

const (
	GroupEventLocalInfoChange GroupEvent = iota
	GroupEventAddGroup
	GroupEventDeleteGroup
	GroupEventNetworkReady
	GroupEventDataMessage
)

func (event GroupEvent) String() string {
	switch event {
	case GroupEventLocalInfoChange:
		return "LocalInfoChange"
	case GroupEventAddGroup:
		return "AddGroup"
	case GroupEventDeleteGroup:
		return "DeleteGroup"
	case GroupEventNetworkReady:
		return "NetworkReady"
	case GroupEventDataMessage:
		return "DataMessage"
	default:
		return "Unknown"
	}
}

type GroupEventData struct {
	event GroupEvent
	data  interface{}
}

type GroupItemConfig struct {
	RepublishInterval  time.Duration
	DataExpireInterval time.Duration
}

type GroupManager struct {
	Pubsub              *util.PubSub
	CheckInfoInterval   time.Duration
	CheckLeaderInterval time.Duration
	KeepAliveInterval   time.Duration
	KeepAliveTimeout    time.Duration
	RepublishInterval   time.Duration
	DataExpireInterval  time.Duration
	localInfo           *MemberMeta
	device              IDevice
	groups              *util.SafeSliceMap /* groupId-> *GroupItem */
	groupInfoMap        *util.SafeMap      // map[string]*AddressInfo
	logger              *log.Entry
	msgChan             chan *GroupMessage
	eventChan           chan *GroupEventData
	stopChan            chan struct{}
	msgSeq              uint64
}

type Option func(*GroupManager)

func WithPubSub(pubsub *util.PubSub) Option {
	return func(m *GroupManager) {
		m.Pubsub = pubsub
	}
}

func WithCheckInfoTime(t time.Duration) Option {
	return func(m *GroupManager) {
		if t > 0 {
			m.CheckInfoInterval = t
		}
	}
}

func WithCheckLeaderTime(t time.Duration) Option {
	return func(m *GroupManager) {
		if t > 0 {
			m.CheckLeaderInterval = t
		}
	}
}

func WithKeepAliveTime(t time.Duration) Option {
	return func(m *GroupManager) {
		if t > 0 {
			m.KeepAliveInterval = t
			m.KeepAliveTimeout = 3 * t
		}
	}
}

func WithRepublishTime(t time.Duration) Option {
	return func(m *GroupManager) {
		if t > 0 {
			m.RepublishInterval = t
		}
	}
}

func WithDataExpireTime(t time.Duration) Option {
	return func(m *GroupManager) {
		if t > 0 {
			m.DataExpireInterval = t
		}
	}
}

func GetGroupKey(groupId GroupId) deviceid.DataKey {
	// @todo cache it
	return deviceid.KeyGen(util.Str2bytes(fmt.Sprintf("_g_%s", groupId)))
}

func NewManager(device IDevice, options ...Option) *GroupManager {
	manager := &GroupManager{
		device:              device,
		localInfo:           &EmptyMemberMeta,
		groups:              util.NewSafeSliceMap(),
		groupInfoMap:        util.NewSafeMap(),
		msgChan:             make(chan *GroupMessage, 10),
		eventChan:           make(chan *GroupEventData, 10),
		stopChan:            make(chan struct{}),
		logger:              log.NewLoggerEntry(ModuleName),
		Pubsub:              util.DefaultPubSub(),
		CheckInfoInterval:   CheckInfoIntervalDefault,
		CheckLeaderInterval: CheckLeaderIntervalDefault,
		KeepAliveInterval:   KeepAliveIntervalDefault,
		KeepAliveTimeout:    KeepAliveTimeoutDefault,
		RepublishInterval:   RepublishIntervalDefault,
		DataExpireInterval:  DataExpireIntervalDefault,
	}

	for _, option := range options {
		option(manager)
	}

	_, err := manager.Pubsub.SubscribeFunc(ModuleName, rpc.DeviceInfoChangeTopic, func(msg util.PubSubMsgType) {
		info := msg.(*device_info.DeviceInfo)
		oldInfo := manager.localInfo
		manager.localInfo = info.Clone()
		manager.logger = manager.logger.WithField("deviceid", info.Id)
		manager.eventChan <- &GroupEventData{
			event: GroupEventLocalInfoChange,
			data:  oldInfo,
		}
	})
	if err != nil {
		manager.logger.Debugf("subscribe device id change failed, %s", err)
	}

	_, err = manager.Pubsub.SubscribeFunc(ModuleName, rpc.DHTStatusChangeTopic, func(msg util.PubSubMsgType) {
		status := msg.(idht.DHTStatus)
		if status == idht.DHTStatusConnected {
			manager.eventChan <- &GroupEventData{
				event: GroupEventNetworkReady,
			}
		}
	})
	if err != nil {
		manager.logger.Errorf("subscribe dht status failed: %s", err)
	}

	go manager.run()
	return manager
}

func (manager *GroupManager) getGroupItem(id GroupId) *GroupItem {
	item, found := manager.groups.Get(id)
	if found {
		return item.(*GroupItem)
	}
	return nil
}

func (manager *GroupManager) getLocalInfo() *MemberMeta {
	return manager.localInfo
}

func (manager *GroupManager) getLocalId() MemberId {
	return manager.localInfo.Id
}

func (manager *GroupManager) setLocalGroupInfo(groupId GroupId, info *GroupInfo) {
	manager.groupInfoMap.Set(groupId, info)
}

func (manager *GroupManager) getLocalGroupInfo(groupId GroupId) *GroupInfo {
	info, found := manager.groupInfoMap.Get(groupId)
	if !found {
		return nil
	}

	return info.(*GroupInfo)
}

func (manager *GroupManager) GetGroupInfo(groupId GroupId, flush bool) (*GroupInfo, error) {
	// if local not found, retrive from remote
	info := manager.getLocalGroupInfo(groupId)
	if info != nil {
		return info, nil
	}

	info, found, err := manager.retriveGroup(groupId, flush)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, fmt.Errorf("group %s info not found", groupId)
	}

	return info, nil
}

func (manager *GroupManager) GroupNum() int {
	return manager.groups.Size()
}

func (manager *GroupManager) JoinGroup(groupId GroupId, data []byte, options ...GroupItemOption) bool {
	item := manager.getGroupItem(groupId)
	if item == nil {
		item = newGroupItem(groupId, NewMemberData(manager.getLocalInfo(), data), options...)
		manager.groups.Set(groupId, item)
	} else {
		item.LocalInfo.Data.UserData = data
	}
	manager.logger.Infof("join group %s", groupId)
	manager.eventChan <- &GroupEventData{
		event: GroupEventAddGroup,
		data:  groupId,
	}
	return true
}

func (manager *GroupManager) LeaveGroup(groupId GroupId) bool {
	item := manager.getGroupItem(groupId)
	if item == nil {
		return true
	}

	manager.groups.Delete(groupId)
	manager.groupInfoMap.Delete(groupId)
	manager.logger.Infof("leave group %s", groupId)
	manager.eventChan <- &GroupEventData{
		event: GroupEventDeleteGroup,
		data:  item,
	}
	return true
}

func (manager *GroupManager) SetGroupUserDataHandler(groupId GroupId, handler func(groupId GroupId, buf *buffer.Buffer)) bool {
	item := manager.getGroupItem(groupId)
	if item == nil {
		return false
	}

	item.UserDataHandler = handler
	return true
}

func (manager *GroupManager) handerUserData(item *GroupItem, srcId deviceid.DeviceId, buf *buffer.Buffer) {
	handler := item.UserDataHandler
	if handler != nil {
		handler(item.Id, buf.Clone())
	}

	// if i am leader, sync to members
	if item.IsLeader(manager.getLocalId()) {
		filter := func(id deviceid.DeviceId) bool {
			return !id.Equal(srcId)
		}

		manager.sendToMembersWithFilter(messageType_userdata, item, filter, buf)
	}
}

func (manager *GroupManager) SendGroupData(groupId GroupId, buf *buffer.Buffer) error {
	addrItem := manager.getGroupItem(groupId)
	if addrItem == nil {
		return fmt.Errorf("groupId %s not found", groupId)
	}

	if addrItem.IsLeader(manager.getLocalId()) {
		manager.sendToMembers(messageType_userdata, addrItem, buf)
	} else {
		manager.sendToLeader(messageType_userdata, addrItem, buf)
	}

	return nil
}

func (manager *GroupManager) iterateGroupItems(callback func(item *GroupItem)) {
	if callback == nil {
		return
	}
	for _, item := range manager.groups.Items() {
		callback(item.(*GroupItem))
	}
}

func (manager *GroupManager) Close() {
	close(manager.stopChan)
}

func (manager *GroupManager) HandlerGroupMessage(msg *message.Message) (next bool) {
	if !IsGroupMessagePort(msg.Dport) {
		return true
	}

	next = false
	if msg.DstDevId != manager.getLocalId().String() {
		manager.logger.Debugf("deviceid dst %s not local", msg.DstDevId)
		return
	}
	if msg.DstDevId == msg.SrcDevId {
		manager.logger.Debugf("deviceid source %s local", msg.SrcDevId)
		return
	}

	groupMsg := GroupMessageFromBuffer(msg.Buffer)
	if groupMsg == nil {
		manager.logger.Debugf("group message from buffer failed")
		return
	}

	manager.msgChan <- groupMsg
	return
}
