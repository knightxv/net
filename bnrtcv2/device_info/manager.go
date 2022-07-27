package device_info

import (
	"net"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
	"github.com/guabee/bnrtc/util"
)

type Option func(option *DeviceInfoManager)

func WithDeviceId(id deviceid.DeviceId) Option {
	return func(option *DeviceInfoManager) {
		option.Id = id
	}
}

func WithPubSub(pubsub *util.PubSub) Option {
	return func(m *DeviceInfoManager) {
		m.Pubsub = pubsub
	}
}

func WithRPCCenter(rpcCenter *util.RPCCenter) Option {
	return func(m *DeviceInfoManager) {
		m.RPCCenter = rpcCenter
	}
}

func WithPort(port int) Option {
	return func(option *DeviceInfoManager) {
		option.Port = port
	}
}

func WithDisableLan(disable bool) Option {
	return func(option *DeviceInfoManager) {
		option.DisableLan = disable
	}
}

func WithDisableIpv6(disable bool) Option {
	return func(option *DeviceInfoManager) {
		option.DisableIpv6 = disable
	}
}

func WithExternalIp(ip string) Option {
	return func(m *DeviceInfoManager) {
		m.ExternalIp = net.ParseIP(ip)
	}
}

type DeviceInfoManager struct {
	Id                    deviceid.DeviceId
	Name                  string
	Pubsub                *util.PubSub
	RPCCenter             *util.RPCCenter
	ExternalIp            net.IP
	Port                  int
	DisableLan            bool
	DisableIpv6           bool
	deviceInfo            *DeviceInfo
	bnrtc1Interface       *Interface
	ipv4Nets              []*net.IPNet
	ipv6Nets              []*net.IPNet
	lanNets               []*net.IPNet
	refershDeviceInfoTask *util.SingleTonTask
	logger                *log.Entry
}

func NewDeviceInfoManage(name string, option ...Option) *DeviceInfoManager {
	m := &DeviceInfoManager{
		Name:      name,
		Pubsub:    util.DefaultPubSub(),
		RPCCenter: util.DefaultRPCCenter(),
		ipv4Nets:  make([]*net.IPNet, 0),
		ipv6Nets:  make([]*net.IPNet, 0),
		lanNets:   make([]*net.IPNet, 0),
		logger:    log.NewLoggerEntry("device-info"),
	}
	m.refershDeviceInfoTask = util.NewSingleTonTask(util.BindingFunc(m.refreshDeviceInfo), util.AllowPending())

	for _, opt := range option {
		opt(m)
	}

	_, err := m.Pubsub.SubscribeFunc("device-info-bnrtc1-iface", rpc.Bnrtc1SignalServerChangeTopic, func(msg util.PubSubMsgType) {
		m.updateBnrtc1Interface(msg.(*transport.TransportAddress))
	})
	if err != nil {
		m.logger.Errorf("subscribe device-info-bnrtc1-iface failed: %s", err)
	}

	_, err = m.Pubsub.SubscribeFunc("device-info-id", rpc.DeviceIdChangeTopic, func(msg util.PubSubMsgType) {
		id := msg.(deviceid.DeviceId)
		m.Id = id
		m.logger = m.logger.WithField("deviceid", id)
		m.refershDeviceInfoTask.Run()
	})
	if err != nil {
		m.logger.Errorf("subscribe device-info-id failed: %s", err)
	}

	m.RPCCenter.OnRegist(rpc.GetExternalIpRpcName, func(params ...util.RPCParamType) util.RPCResultType {
		m.refershDeviceInfoTask.Run()
		return nil
	})

	m.logger.Info("device info manager created")
	return m
}

func (m *DeviceInfoManager) GetDeviceInfo() *DeviceInfo {
	return m.deviceInfo
}

func (m *DeviceInfoManager) onDeviceInfoChange(info *DeviceInfo) {
	m.Pubsub.Publish(rpc.DeviceInfoChangeTopic, info)
}

func (m *DeviceInfoManager) updateBnrtc1Interface(addr *transport.TransportAddress) {
	m.logger.Infof("bnrtc1 interface changed to %+v", addr)
	if addr == nil {
		m.bnrtc1Interface = nil
	} else {
		m.bnrtc1Interface = &Interface{IP: addr.Addr.IP, Port: addr.Addr.Port, Mask: addr.Addr.IP.DefaultMask()}
	}

	m.refershDeviceInfoTask.Run()
}

func (m *DeviceInfoManager) refreshLocalInterfaceInfo() {
	// refresh local interface info
	_, ipv6List, lanList := deviceid.GetLocalIpNetList()
	// m.ipv4Nets = ipv4List
	m.ipv4Nets = []*net.IPNet{}
	m.ipv6Nets = ipv6List
	m.lanNets = lanList
}

func (m *DeviceInfoManager) refreshDeviceInfo() {
	if m.Id.IsEmpty() {
		return
	}

	m.refreshLocalInterfaceInfo()

	if m.ExternalIp != nil {
		// if m.ExternalIp.IsGlobalUnicast() && !m.ExternalIp.IsPrivate() {
		m.ipv4Nets = append(m.ipv4Nets, &net.IPNet{IP: m.ExternalIp, Mask: m.ExternalIp.DefaultMask()})
		// }
	} else if m.Id.IsTopNetwork() {
		// get ExternalIp
		_ip, err := m.RPCCenter.Call(rpc.GetExternalIpRpcName)
		if err == nil {
			ip := _ip.(net.IP)
			if ip.IsGlobalUnicast() && !ip.IsPrivate() {
				m.ipv4Nets = append(m.ipv4Nets, &net.IPNet{IP: ip, Mask: ip.DefaultMask()})
			}
		}
	}

	info := &DeviceInfo{
		Name:      m.Name,
		Id:        m.Id,
		Iterfaces: make([][]*Interface, InterfaceTypeMax),
		Port:      m.Port,
	}

	if len(m.ipv4Nets) != 0 {
		for _, ip := range m.ipv4Nets {
			info.Iterfaces[Ipv4] = append(info.Iterfaces[Ipv4], &Interface{IP: ip.IP, Port: m.Port, Mask: ip.Mask})
		}
	}

	if !m.DisableIpv6 && len(m.ipv6Nets) != 0 {
		for _, ip := range m.ipv6Nets {
			info.Iterfaces[Ipv6] = append(info.Iterfaces[Ipv6], &Interface{IP: ip.IP, Port: m.Port, Mask: ip.Mask})
		}
	}

	if m.bnrtc1Interface != nil {
		info.Iterfaces[Bnrtc1] = append(info.Iterfaces[Bnrtc1], m.bnrtc1Interface)
	}

	if !m.DisableLan && len(m.lanNets) != 0 {
		for _, ip := range m.lanNets {
			info.Iterfaces[Lan] = append(info.Iterfaces[Lan], &Interface{IP: ip.IP, Port: m.Port, Mask: ip.Mask})
		}
	}

	m.logger.Infof("device info changed %+v->%+v", m.deviceInfo, info)

	if !info.HaveIterface() {
		// no interface, ignore
		return
	}

	m.deviceInfo = info
	m.onDeviceInfoChange(info)
}
