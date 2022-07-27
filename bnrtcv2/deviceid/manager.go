package deviceid

import (
	"io/ioutil"
	"net"
	"sync"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/util"
)

const (
	UpdateIntervalDefault = 60 * time.Second
)

type ExternalIpGetter func() net.IP

type IDeviceIdManager interface {
	GetDeviceId(wait bool) DeviceId
}

type DeviceIdManager struct {
	DeviceId           DeviceId
	PubSub             *util.PubSub
	RPCCenter          *util.RPCCenter
	UpdateInterval     time.Duration
	IdFilePath         string
	ForceTopNetwork    bool
	useConfig          bool
	ExternalIp         net.IP
	updateDeviceIdTask *util.SingleTonTask
	stopChan           chan struct{}
	waitChan           chan struct{}
	forceChan          chan struct{}
	once               sync.Once
	logger             *log.Entry
}

type Option func(*DeviceIdManager)

func WithDeviceId(id DeviceId) Option {
	return func(m *DeviceIdManager) {
		m.DeviceId = id
		m.useConfig = true
	}
}

func WithExternalIp(ip string) Option {
	return func(m *DeviceIdManager) {
		m.ExternalIp = net.ParseIP(ip)
	}
}

func WithPubSub(pubsub *util.PubSub) Option {
	return func(m *DeviceIdManager) {
		m.PubSub = pubsub
	}
}

func WithRPCCenter(rpcCenter *util.RPCCenter) Option {
	return func(m *DeviceIdManager) {
		m.RPCCenter = rpcCenter
	}
}

func WithUpdateTime(t time.Duration) Option {
	return func(m *DeviceIdManager) {
		if t > 0 {
			m.UpdateInterval = t
		}
	}
}

func WithFilePath(path string) Option {
	return func(m *DeviceIdManager) {
		m.IdFilePath = path
	}
}

func WithForceTopNetwork() Option {
	return func(m *DeviceIdManager) {
		m.ForceTopNetwork = true
	}
}

func NewDeviceIdManager(options ...Option) *DeviceIdManager {
	m := &DeviceIdManager{
		PubSub:         util.DefaultPubSub(),
		RPCCenter:      util.DefaultRPCCenter(),
		UpdateInterval: UpdateIntervalDefault,
		useConfig:      false,
		stopChan:       make(chan struct{}),
		forceChan:      make(chan struct{}, 1),
	}
	for _, opt := range options {
		opt(m)
	}
	m.updateDeviceIdTask = util.NewSingleTonTask(util.BindingFunc(m.doUpdateDeviceId), util.AllowPending())
	m.logger = log.NewLoggerEntry("deviceid-manager").WithField("deviceid", m.DeviceId)
	if !m.useConfig {
		if m.ExternalIp == nil {
			m.RPCCenter.OnRegist(rpc.GetExternalIpRpcName, func(params ...util.RPCParamType) util.RPCResultType {
				m.forceChan <- struct{}{}
				return nil
			})
		}
		m.updateDeviceIdTask.RunWait()
		go m.updateDeviceIdWorker()
	} else {
		m.PubSub.Publish(rpc.DeviceIdChangeTopic, m.DeviceId)
		m.logger.Infof("use config device id %s", m.DeviceId)
	}
	return m
}

func (m *DeviceIdManager) GetDeviceId(wait bool) DeviceId {
	if m.DeviceId.IsEmpty() {
		if !wait {
			return nil
		}

		m.once.Do(func() {
			m.waitChan = make(chan struct{})
		})
		<-m.waitChan
	}
	return m.DeviceId
}

func (m *DeviceIdManager) updateDeviceIdWorker() {
	t := time.NewTicker(m.UpdateInterval)

OUT:
	for {
		select {
		case <-t.C:
			m.updateDeviceIdTask.Run()
		case <-m.forceChan:
			m.updateDeviceIdTask.Run()
		case <-m.stopChan:
			t.Stop()
			close(m.forceChan)
			break OUT
		}
	}
}

func (m *DeviceIdManager) getExternalIp() net.IP {
	ipv4list, _, lanlist := GetLocalIpNetList()
	// use first public ipv4 as external ip
	for _, ip := range ipv4list {
		return ip.IP
	}

	// if not public ipv4, try get external ip
	_ip, err := m.RPCCenter.Call(rpc.GetExternalIpRpcName)
	if err == nil {
		ip := _ip.(net.IP)
		if ip.IsGlobalUnicast() && !ip.IsPrivate() {
			return ip
		}
	}

	// if get external ip failed, try use lan address
	for _, ip := range lanlist {
		return ip.IP
	}

	return nil

}

func (m *DeviceIdManager) doUpdateDeviceId() {
	topNetwork := m.ForceTopNetwork
	externalIp := m.ExternalIp
	if externalIp == nil {
		externalIp = m.getExternalIp()
		if externalIp == nil {
			return
		}
	}

	changed := m.generateId(topNetwork, externalIp.To4())

	if m.waitChan != nil {
		close(m.waitChan)
	}

	if changed {
		m.logger = m.logger.WithField("deviceid", m.DeviceId)
		m.PubSub.Publish(rpc.DeviceIdChangeTopic, m.DeviceId)
	}
}

func (m *DeviceIdManager) Destroy() {
	close(m.stopChan)
}

func (m *DeviceIdManager) generateId(topNetwork bool, groupId []byte) (changed bool) {
	if !m.DeviceId.IsEmpty() && m.DeviceId.SameGroupId(groupId) && (m.DeviceId.IsTopNetwork() == topNetwork) {
		return false
	}
	random := m.readRandomId()
	newDeviceId := NewDeviceId(topNetwork, groupId, random)
	m.logger.Infof("device id change %s -> %s", m.DeviceId, newDeviceId)
	m.DeviceId = newDeviceId
	m.writeRandomId()
	return true
}

func (m *DeviceIdManager) writeRandomId() {
	if m.IdFilePath != "" {
		err := ioutil.WriteFile(m.IdFilePath, m.DeviceId.NodeIdValue(), 0666)
		if err != nil {
			m.logger.Errorf("write random id failed: %s", err)
		} else {
			m.logger.Infof("write random id to %s success", m.IdFilePath)
		}
	}
}

func (m *DeviceIdManager) readRandomId() []byte {
	if m.IdFilePath != "" {
		data, err := ioutil.ReadFile(m.IdFilePath)
		if err != nil {
			m.logger.Debugf("read random id from file %s err %s", m.IdFilePath, err.Error())
			return nil
		}
		m.logger.Infof("read random id form %s success", m.IdFilePath)
		if len(data) == NodeIdLength {
			return data
		}
	}

	return nil
}
