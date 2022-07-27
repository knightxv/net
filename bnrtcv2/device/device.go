package device

import (
	"fmt"
	"net"
	"sync"

	"github.com/guabee/bnrtc/bnrtcv2/broadcast"
	"github.com/guabee/bnrtc/bnrtcv2/config"
	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/dht/idht"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
)

type IBroadcastCtrl interface {
	Broadcast(buf *buffer.Buffer) error
}

type IConnectionManager interface {
	Connect(addr *transport.TransportAddress) error
}

type Device struct {
	Id                    deviceid.DeviceId
	ExternalIp            net.IP
	Pubsub                *util.PubSub
	RPCCenter             *util.RPCCenter
	DeviceInfo            *device_info.DeviceInfo
	Dht                   idht.IDHT
	TransportManager      transport.ITransportManager
	ConnectionManager     IConnectionManager
	Peers                 map[string]*transport.TransportAddress
	messageHandler        transport.MessageHandler
	onAddPeerHandler      func()
	logger                *log.Entry
	BroadcastCtrl         IBroadcastCtrl
	dhtMt                 sync.Mutex
	DHTMemoryStoreOptions *config.DHTMemoryStoreOptions
}

type Option func(option *Device)

func WithDeviceId(id deviceid.DeviceId) Option {
	return func(option *Device) {
		option.Id = id
	}
}

func WithExternalIp(ip string) Option {
	return func(m *Device) {
		m.ExternalIp = net.ParseIP(ip)
	}
}

func WithDHTMemoryStoreOptions(options *config.DHTMemoryStoreOptions) Option {
	return func(m *Device) {
		m.DHTMemoryStoreOptions = options
	}
}

func WithPubSub(pubsub *util.PubSub) Option {
	return func(m *Device) {
		m.Pubsub = pubsub
	}
}

func WithRPCCenter(c *util.RPCCenter) Option {
	return func(m *Device) {
		m.RPCCenter = c
	}
}

func NewDevice(transportManager transport.ITransportManager, bnrtc1ConnectionManager IConnectionManager, dhtBuilder func(*idht.DHTOption) idht.IDHT, options ...Option) *Device {
	device := &Device{
		TransportManager:  transportManager,
		ConnectionManager: bnrtc1ConnectionManager,
		Id:                deviceid.EmptyDeviceId,
		Pubsub:            util.DefaultPubSub(),
		RPCCenter:         util.DefaultRPCCenter(),
		Peers:             make(map[string]*transport.TransportAddress),
		logger:            log.NewLoggerEntry("device"),
	}

	for _, option := range options {
		option(device)
	}

	var once sync.Once
	_, err := device.Pubsub.SubscribeFunc("device", rpc.DeviceInfoChangeTopic, func(msg util.PubSubMsgType) {
		info := msg.(*device_info.DeviceInfo)
		if info == nil {
			return
		}
		device.DeviceInfo = info
		once.Do(func() {
			device.dhtMt.Lock()
			device.Dht = dhtBuilder(&idht.DHTOption{
				DeviceInfo:            device.DeviceInfo,
				Peers:                 device.GetPeerAddresses(),
				TransportManager:      device.TransportManager,
				PubSub:                device.Pubsub,
				DHTMemoryStoreOptions: device.DHTMemoryStoreOptions,
			})
			device.dhtMt.Unlock()

			device.BroadcastCtrl = broadcast.NewBroadcastCtrl(
				info,
				transportManager,
				device.Dht,
				broadcast.WithPubSub(device.Pubsub),
			)
		})
		device.Dht.SetDeviceInfo(info)

		if !device.Id.Equal(info.Id) {
			device.Id = info.Id
			device.logger = device.logger.WithField("deviceId", device.Id)
		}
	})

	if err != nil {
		device.logger.Errorf("subscribe %s failed: %s", rpc.DeviceInfoChangeTopic, err)
	}

	go func() {
		for msg := range transportManager.GetDataMsgChan() {
			if msg.IsExpired() {
				continue
			}
			device.handleMessage(msg)
		}
	}()
	device.onAddPeerHandler = func() {
		device.RPCCenter.Regist(rpc.GetExternalIpRpcName, func(params ...util.RPCParamType) util.RPCResultType {
			return device.GetExternalIp()
		})
		device.RPCCenter.Regist(rpc.GetSignalServerAddrRpcName, func(params ...util.RPCParamType) util.RPCResultType {
			return device.GetNearestDeviceAddress()
		})
		device.onAddPeerHandler = nil
	}
	return device
}

func (d *Device) GetDeviceInfo() *device_info.DeviceInfo {
	return d.DeviceInfo
}

func (d *Device) GetDeviceId() deviceid.DeviceId {
	return d.Id
}

func (d *Device) ChooseMatchedAddress(infos []*device_info.DeviceInfo, devId deviceid.DeviceId) (*transport.TransportAddress, error) {
	return d.DeviceInfo.ChooseMatchedAddress(infos, devId, d.TransportManager)
}

func (d *Device) Send(addr *transport.TransportAddress, buf *buffer.Buffer) error {
	return d.TransportManager.SendDataMsg(addr, buf)
}

func (d *Device) SendByInfo(info *device_info.DeviceInfo, buf *buffer.Buffer) error {
	if info == nil {
		return fmt.Errorf("info is nil")
	}

	addr := transport.NewAddress(info.Id, nil)
	if d.TransportManager.IsTargetReachable(addr) {
		return d.Send(addr, buf)
	}

	ta, err := d.DeviceInfo.ChooseMatchedAddressByNet([]*device_info.DeviceInfo{info}, info.Id)
	if err != nil {
		return err
	}

	return d.Send(ta, buf)
}

func (d *Device) OnMessage(handler transport.MessageHandler) {
	d.messageHandler = handler
}

func (d *Device) handleMessage(msg *transport.Message) {
	handler := d.messageHandler
	if handler != nil {
		handler(msg)
	}
}

func (d *Device) Broadcast(buf *buffer.Buffer) error {
	if d.BroadcastCtrl == nil {
		return fmt.Errorf("broadcast ctrl is nil")
	}

	return d.BroadcastCtrl.Broadcast(buf)
}

func (d *Device) IsConnected() bool {
	if d.Dht == nil {
		return false
	}

	return d.Dht.IsConnected()
}

func (d *Device) IsTargetReachable(addr *transport.TransportAddress) bool {
	return d.TransportManager.IsTargetReachable(addr)
}

func (d *Device) GetData(key deviceid.DataKey, noCache bool) ([]byte, bool, error) {
	if key.IsEmpty() {
		return nil, false, fmt.Errorf("key is empty")
	}

	if d.Dht == nil {
		return nil, false, fmt.Errorf("dht is nil")
	}

	return d.Dht.Get(key, noCache)
}

func (d *Device) PutData(key deviceid.DataKey, data []byte, options *idht.DHTStoreOptions) error {
	if key.IsEmpty() {
		key = deviceid.KeyGen(data)
	}

	if d.Dht == nil {
		return fmt.Errorf("dht is nil")
	}

	return d.Dht.Put(key, data, options)
}

func (d *Device) UpdateData(key deviceid.DataKey, oldData, newData []byte, options *idht.DHTStoreOptions) error {
	if key.IsEmpty() {
		return fmt.Errorf("key is empty for update data")
	}

	if d.Dht == nil {
		return fmt.Errorf("dht is nil")
	}

	return d.Dht.Update(key, oldData, newData, options)
}

func (d *Device) DeleteData(key deviceid.DataKey) error {
	if d.Dht == nil {
		return fmt.Errorf("dht is nil")
	}
	return d.Dht.Delete(key)
}

func (d *Device) AddPeers(ips []string) {
	d.logger.Infof("AddPeers: %s", ips)
	for _, ip := range ips {
		udpAddr, err := net.ResolveUDPAddr("udp", ip)
		if err != nil {
			continue
		}
		transportAddr := transport.NewAddress(nil, udpAddr)
		d.Peers[ip] = transportAddr
		d.dhtMt.Lock()
		if d.Dht != nil {
			info := device_info.BuildDeviceInfo(transportAddr)
			if info != nil {
				d.Dht.AddPeer(info)
			}
		}
		d.dhtMt.Unlock()
	}
	if len(d.Peers) > 0 && d.onAddPeerHandler != nil {
		d.onAddPeerHandler()
	}
}

func (d *Device) GetPeers() []string {
	peers := make([]string, 0)
	for peer := range d.Peers {
		peers = append(peers, peer)
	}
	return peers
}

func (d *Device) GetPeerAddresses() []*transport.TransportAddress {
	peers := make([]*transport.TransportAddress, 0)
	for _, peer := range d.Peers {
		peers = append(peers, peer)
	}
	return peers
}

func (d *Device) GetDeviceInfoById(id deviceid.DeviceId) *device_info.DeviceInfo {
	if d.Dht == nil {
		return nil
	}

	return d.Dht.GetDeviceInfo(id)
}

func (d *Device) GetNearestDeviceAddress() *transport.TransportAddress {
	if d.Dht == nil {
		for _, peer := range d.Peers {
			return peer
		}
		return nil
	}

	addr := (*transport.TransportAddress)(nil)
	info := d.Dht.GetClosestTopNetworkDeviceInfo()
	if info == nil {
		for _, peer := range d.Peers {
			return peer
		}
	} else {
		addr, _ = d.DeviceInfo.ChooseMatchedAddress([]*device_info.DeviceInfo{info}, nil, nil)
	}

	return addr
}

func (d *Device) GetNeighborDeviceAddresses(num int) []*transport.TransportAddress {
	if d.Dht == nil {
		return nil
	}

	addrs := ([]*transport.TransportAddress)(nil)
	infos := d.Dht.GetNeighborDeviceInfo(num)
	for _, info := range infos {
		addr, err := d.DeviceInfo.ChooseMatchedAddress([]*device_info.DeviceInfo{info}, nil, nil)
		if err == nil {
			addrs = append(addrs, addr)
		}
	}

	return addrs
}

func (d *Device) GetExternalIp() net.IP {
	if d.ExternalIp != nil {
		return d.ExternalIp
	}

	// if d.Dht != nil {
	// 	// from closest device
	// 	info := d.Dht.GetClosestTopNetworkDeviceInfo()
	// 	if info != nil {
	// 		addr, _ := d.ChooseMatchedAddress([]*device_info.DeviceInfo{info}, nil)
	// 		ip, err := d.TransportManager.GetExternalIp(addr)
	// 		if err == nil {
	// 			return ip
	// 		}
	// 	}
	// }

	// from peers
	for _, peer := range d.Peers {
		peerIp := peer.Addr.IP
		if peerIp.IsGlobalUnicast() && !peerIp.IsPrivate() {
			ip, err := d.TransportManager.GetExternalIp(peer)
			if err != nil {
				continue
			}
			if ip != nil {
				return ip
			}
		}
	}

	// not public ip peer, use local ip peer
	for _, peer := range d.Peers {
		peerIp := peer.Addr.IP
		if peerIp.IsLoopback() || peerIp.IsPrivate() {
			ip, err := d.TransportManager.GetExternalIp(peer)
			if err != nil {
				continue
			}
			if ip != nil {
				return ip
			}
		}
	}

	return nil
}
