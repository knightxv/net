package address

import (
	"fmt"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/group"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/bnrtcv2/server"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
)

const (
	RepublishIntervalDefault  = 5 * 60 * time.Second
	DataExpireIntervalDefault = 6 * 60 * time.Second
	NoCache                   = true
	Cache                     = false
	TransportAddressCacheSize = 1024
)

type IDevice interface {
	ChooseMatchedAddress(infos []*device_info.DeviceInfo, devId deviceid.DeviceId) (*transport.TransportAddress, error)
	GetDeviceInfoById(id deviceid.DeviceId) *device_info.DeviceInfo
}

type AddressInfo struct {
	Address string
	Devices []*device_info.DeviceInfo
}

type AddressManager struct {
	device                          IDevice
	groupManager                    *group.GroupManager
	Pubsub                          *util.PubSub
	RepublishInterval               time.Duration
	DataExpireInterval              time.Duration
	addresses                       *util.SafeSliceMap
	defaultAddress                  string
	logger                          *log.Entry
	userDataHandler                 func(*buffer.Buffer)
	transportAddressCacheByAddress  *util.SafeLinkMap
	transportAddressCacheByDeviceId *util.SafeLinkMap
}

type Option func(*AddressManager)

func WithPubSub(pubsub *util.PubSub) Option {
	return func(m *AddressManager) {
		m.Pubsub = pubsub
	}
}

func WithRepublishTime(t time.Duration) Option {
	return func(m *AddressManager) {
		if t > 0 {
			m.RepublishInterval = t
		}
	}
}

func WithDataExpireTime(t time.Duration) Option {
	return func(m *AddressManager) {
		if t > 0 {
			m.DataExpireInterval = t
		}
	}
}

func isValidAddress(address string) bool {
	return address != ""
}

func GetGroupId(address string) group.GroupId {
	return "_a_" + address
}

func NewManager(device IDevice, groupManager *group.GroupManager, options ...Option) *AddressManager {
	manager := &AddressManager{
		device:                          device,
		groupManager:                    groupManager,
		addresses:                       util.NewSafeSliceMap(),
		logger:                          log.NewLoggerEntry("address"),
		Pubsub:                          util.DefaultPubSub(),
		RepublishInterval:               RepublishIntervalDefault,
		DataExpireInterval:              DataExpireIntervalDefault,
		transportAddressCacheByAddress:  util.NewSafeLimitLinkMap(TransportAddressCacheSize),
		transportAddressCacheByDeviceId: util.NewSafeLimitLinkMap(TransportAddressCacheSize),
	}

	for _, option := range options {
		option(manager)
	}

	_, err := manager.Pubsub.SubscribeFunc("address", rpc.DeviceIdChangeTopic, func(msg util.PubSubMsgType) {
		id := msg.(deviceid.DeviceId)
		manager.logger = manager.logger.WithField("device", id.String())
	})
	if err != nil {
		manager.logger.Errorf("subscribe deviceid change failed: %s", err)
	}

	return manager
}

func (manager *AddressManager) GetDefaultAddress() string {
	return manager.defaultAddress
}

func (manager *AddressManager) getAddressInfo(address string, flush bool) (*AddressInfo, error) {
	info, err := manager.groupManager.GetGroupInfo(GetGroupId(address), flush)
	if err != nil {
		return nil, err
	}

	addressInfo := &AddressInfo{
		Address: address,
		Devices: []*device_info.DeviceInfo{info.Leader.GetMeta()},
	}
	for _, member := range info.Members {
		addressInfo.Devices = append(addressInfo.Devices, member.GetMeta())
	}

	return addressInfo, nil
}

func (manager *AddressManager) GetAddressInfo(address string) (info server.AddressInfo, err error) {
	inf, err := manager.getAddressInfo(address, true)
	if err != nil {
		return
	}
	info.Address = inf.Address
	for _, device := range inf.Devices {
		info.Endpoints = append(info.Endpoints, &server.Endpoint{
			Id:   device.Id.String(),
			Name: device.Name,
		})
	}
	return
}

func (manager *AddressManager) BindAddress(address string) bool {
	if !isValidAddress(address) {
		manager.logger.Errorf("bind invalid address %s", address)
		return false
	}

	manager.defaultAddress = address // always set to default address
	if manager.addresses.Has(address) {
		return true
	}

	manager.addresses.Set(address, address)
	groupId := GetGroupId(address)
	manager.groupManager.JoinGroup(
		groupId,
		nil,
		group.WithGroupDataExpireTime(manager.DataExpireInterval),
		group.WithGroupRepublishTime(manager.RepublishInterval),
	)
	manager.groupManager.SetGroupUserDataHandler(groupId, manager.handerUserData)
	manager.logger.Infof("bind address %s", address)
	return true
}

func (manager *AddressManager) UnbindAddress(address string) bool {
	if !isValidAddress(address) {
		manager.logger.Errorf("unbind invalid address %s", address)
		return false
	}

	if !manager.addresses.Has(address) {
		return true
	}

	manager.addresses.Delete(address)
	if address == manager.defaultAddress {
		manager.defaultAddress = ""
		for _, _address := range manager.addresses.Items() {
			manager.defaultAddress = _address.(string)
			break
		}
	}
	manager.logger.Infof("unbind address %s", address)
	manager.groupManager.LeaveGroup(GetGroupId(address))
	return true
}

func (manager *AddressManager) GetAddresses() []string {
	var addresses []string

	for _, address := range manager.addresses.Items() {
		addresses = append(addresses, address.(string))
	}
	return addresses
}

func (manager *AddressManager) IsLocalAddress(address string) bool {
	return manager.addresses.Has(address)
}

func (manager *AddressManager) SetUserDataHandler(handler func(buf *buffer.Buffer)) {
	manager.userDataHandler = handler
}

func (manager *AddressManager) handerUserData(groupId group.GroupId, buf *buffer.Buffer) {
	if manager.userDataHandler == nil {
		return
	}

	// ignore groupId
	manager.userDataHandler(buf)
}

func (manager *AddressManager) SyncData(address string, buf *buffer.Buffer) error {
	if !isValidAddress(address) {
		return fmt.Errorf("invalid address %s", address)
	}

	return manager.groupManager.SendGroupData(GetGroupId(address), buf)
}

func (manager *AddressManager) GetTransportAddress(address string, devid string, flush bool) *transport.TransportAddress {
	if address == "" && devid == "" {
		return nil
	}

	var deviceInfos []*device_info.DeviceInfo
	if address == "" && devid != "" {
		if !flush {
			_deviceInfos, found := manager.transportAddressCacheByDeviceId.Get(devid)
			if found {
				deviceInfos = _deviceInfos.([]*device_info.DeviceInfo)
			}
		}

		if deviceInfos == nil {
			deviceInfo := manager.device.GetDeviceInfoById(deviceid.FromString(devid))
			if deviceInfo == nil {
				manager.logger.Debugf("device %s info not found", devid)
				return nil
			}
			deviceInfos = []*device_info.DeviceInfo{deviceInfo}
			manager.transportAddressCacheByDeviceId.Set(devid, deviceInfos)
		}
	} else {
		if !flush {
			_deviceInfos, found := manager.transportAddressCacheByAddress.Get(address)
			if found {
				deviceInfos = _deviceInfos.([]*device_info.DeviceInfo)
			}
		}
		if deviceInfos == nil {
			addressInfo, err := manager.getAddressInfo(address, flush)
			if err != nil {
				manager.logger.Debugf("get transport address %s %s failed: %s", address, devid, err)
				return nil
			}
			deviceInfos = addressInfo.Devices
			manager.transportAddressCacheByAddress.Set(address, deviceInfos)
		}
	}

	id := deviceid.EmptyDeviceId
	if devid != "" {
		id = deviceid.FromString(devid)
	}

	addr, err := manager.device.ChooseMatchedAddress(deviceInfos, id)
	if err != nil {
		manager.logger.Debugf("choose matched transport address failed %s", err)
		return nil
	}

	return addr
}
