package net

import (
	"context"
	"fmt"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/message"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/bnrtcv2/server"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
)

type IDevice interface {
	IsConnected() bool
	IsTargetReachable(addr *transport.TransportAddress) bool
	Send(addr *transport.TransportAddress, buf *buffer.Buffer) error
	Broadcast(buf *buffer.Buffer) error
	OnMessage(transport.MessageHandler)
	AddPeers(ips []string)
	GetPeers() []string
	GetNeighborDeviceAddresses(num int) []*transport.TransportAddress
}
type IGroupManager interface {
	HandlerGroupMessage(msg *message.Message) (next bool)
}

type IAddressManager interface {
	SyncData(address string, buf *buffer.Buffer) error
	SetUserDataHandler(handler func(buf *buffer.Buffer))
	GetDefaultAddress() string
	GetTransportAddress(address string, devid string, flush bool) *transport.TransportAddress
	BindAddress(address string) bool
	UnbindAddress(address string) bool
	GetAddresses() []string
	IsLocalAddress(address string) bool
	GetAddressInfo(address string) (server.AddressInfo, error)
}

type BPControllerDirection int

const (
	BPControllerDirectionSend = iota
	BPControllerDirectionRecv
)

type IBPController interface {
	RecvEnqueue(msg *message.Message) error
	SendEnqueue(msg *message.Message) error
	OnSendMessage(func(*message.Message))
	OnRecvMessage(func(*message.Message))
	Close()
}

type IBpManager interface {
	OnController(handler func(ctrl IBPController))
	GetController(deviceId string, dport string, direction BPControllerDirection) (IBPController, error)
	SetDeviceId(string)
}

type MsgHandler = func(msg *message.Message)

type NetControllerConfig struct {
	Device            IDevice
	AddressManager    IAddressManager
	GroupManager      IGroupManager
	BpManager         IBpManager
	ConnectionManager IConnectionManager
}

type NetController struct {
	PubSub          *util.PubSub
	RPCCenter       *util.RPCCenter
	DevId           string
	device          IDevice
	addressManager  IAddressManager
	groupManager    IGroupManager
	favoriteManager *FavoriteManager
	dportMsgHandler *util.SafeMap //   map[string]MsgHandler dport->handler
	sendDports      *util.SafeMap //   map[string]MsgHandler dport->handler
	bpManager       IBpManager
	addrCache       *util.SafeLinkMap //   map[string]*transport.TransportAddress
	recvMsgHook     func(msg *message.Message) (next bool)
	logger          *log.Entry
}

type Option func(*NetController)

func WithDevid(id deviceid.DeviceId) Option {
	return func(t *NetController) {
		t.DevId = id.String()
	}
}

func WithPubSub(pubsub *util.PubSub) Option {
	return func(c *NetController) {
		c.PubSub = pubsub
	}
}

func WithRPCCenter(c *util.RPCCenter) Option {
	return func(m *NetController) {
		m.RPCCenter = c
	}
}

func NewNetController(ctx context.Context, config *NetControllerConfig, options ...Option) *NetController {
	ctrl := &NetController{
		PubSub:          util.DefaultPubSub(),
		RPCCenter:       util.DefaultRPCCenter(),
		device:          config.Device,
		addressManager:  config.AddressManager,
		groupManager:    config.GroupManager,
		favoriteManager: NewFavoriteManager(ctx, config.Device, config.ConnectionManager),
		dportMsgHandler: util.NewSafeMap(),
		sendDports:      util.NewSafeMap(),
		bpManager:       config.BpManager,
		addrCache:       util.NewSafeLimitLinkMap(1024), //  make(map[string]*transport.TransportAddress),
		logger:          log.NewLoggerEntry("net"),
	}

	for _, opt := range options {
		opt(ctrl)
	}

	ctrl.bpManager.OnController(func(c IBPController) {
		c.OnSendMessage(ctrl.doSend)
		c.OnRecvMessage(ctrl.doRecv)
	})

	ctrl.device.OnMessage(func(data *transport.Message) {
		senderId := data.Sender.DeviceId.String()
		if !ctrl.addrCache.Has(senderId) {
			// update when not exist
			ctrl.addrCache.Set(senderId, data.Sender)
		}

		buf := data.GetBuffer()
		size := buf.Len()
		msg := message.FromBuffer(buf)

		if msg == nil {
			ctrl.logger.Errorf("discard message for invalid message")
			return
		}
		ctrl.logger.Debugf("on message, %s(%s)->%s(%s):%s ", msg.SrcAddr, msg.SrcDevId, msg.DstAddr, msg.DstDevId, msg.Dport)
		recvMsgHook := ctrl.recvMsgHook
		if recvMsgHook != nil {
			next := recvMsgHook(msg.Clone())
			if !next {
				return
			}
		}

		if msg.SrcDevId == "" {
			ctrl.logger.Errorf("discard message for empty source device")
			return
		}
		if msg.SrcDevId == ctrl.DevId {
			ctrl.logger.Errorf("discard message from myself")
			return // not allow to send to self
		}
		if !ctrl.IsSendPort(msg.Dport) && !ctrl.IsRecvPort(msg.Dport) {
			ctrl.logger.Errorf("discard message for not dport %s registered", msg.Dport)
			return
		}

		bpc, err := ctrl.bpManager.GetController(msg.SrcDevId, msg.Dport, BPControllerDirectionRecv)
		if err != nil {
			ctrl.logger.Errorf("get controller failed, %v", err)
			return
		}

		ctrl.favoriteManager.RecordRecv(msg.SrcDevId, uint64(size))
		err = bpc.RecvEnqueue(msg)
		if err != nil {
			ctrl.logger.Debugf("enqueue message failed, %s", err)
		}
	})

	_, err := ctrl.PubSub.SubscribeFunc("net-controller", rpc.DeviceIdChangeTopic, func(msg util.PubSubMsgType) {
		devid := msg.(deviceid.DeviceId)
		if ctrl.DevId != devid.String() {
			ctrl.DevId = devid.String()
			ctrl.logger = ctrl.logger.WithField("deviceId", ctrl.DevId)
			ctrl.bpManager.SetDeviceId(ctrl.DevId)
		}
	})
	if err != nil {
		ctrl.logger.Debugf("subscribe device id change failed, %s", err)
	}

	ctrl.HookRecvMessage(ctrl.groupManager.HandlerGroupMessage)
	ctrl.addressManager.SetUserDataHandler(ctrl.handleSyncMsg)
	return ctrl
}

func (c *NetController) HookRecvMessage(handler func(msg *message.Message) (next bool)) {
	c.recvMsgHook = handler
}

func (c *NetController) GetDevice() IDevice {
	return c.device
}

func (c *NetController) doSend(msg *message.Message) {
	c.logger.Debugf("do send message, %s(%s)->%s(%s):%s ", msg.SrcAddr, msg.SrcDevId, msg.DstAddr, msg.DstDevId, msg.Dport)
	var addr *transport.TransportAddress
	_addr, found := c.addrCache.Get(msg.DstDevId)
	if !found {
		addr = c.addressManager.GetTransportAddress(msg.DstAddr, msg.DstDevId, false)
		if addr == nil {
			c.logger.Errorf("get transport address failed, %s(%s)", msg.DstAddr, msg.DstDevId)
			return
		}
	} else {
		addr = _addr.(*transport.TransportAddress)
	}
	buf := msg.GetBuffer()
	size := buf.Len()
	err := c.device.Send(addr, buf)
	if err != nil {
		addr = c.addressManager.GetTransportAddress(msg.DstAddr, msg.DstDevId, true)
		if addr == nil {
			c.addrCache.Delete(msg.DstDevId)
			c.logger.Errorf("get transportAddress error, %s(%s), %s", msg.DstAddr, msg.DstDevId, err)
			return
		}
		// try again with new address
		err = c.device.Send(addr, buf)
		if err != nil {
			c.addrCache.Delete(msg.DstDevId)
			c.logger.Errorf("send message error, %s %s", addr, err)
			return
		}
		c.addrCache.Set(msg.DstDevId, addr) // add new address to cache
	}
	c.favoriteManager.RecordSend(msg.DstDevId, uint64(size))
}

func (c *NetController) handleMessage(msg *message.Message) {
	handler, found := c.dportMsgHandler.Get(msg.Dport)
	if found {
		handler.(MsgHandler)(msg)
	} else {
		c.logger.Warnf("discard msg due to not handler for dport %s", msg.Dport)
	}
}

func (c *NetController) handleSyncMsg(buf *buffer.Buffer) {
	msg := message.FromBuffer(buf)
	if msg == nil {
		return
	}

	if !c.isToMe(msg) {
		return
	}

	msg.Sync = true // force to sync
	c.handleMessage(msg.Clone())
}

func (c *NetController) doRecv(msg *message.Message) {
	c.logger.Debugf("do recv message, %s(%s)->%s(%s):%s ", msg.SrcAddr, msg.SrcDevId, msg.DstAddr, msg.DstDevId, msg.Dport)
	if !c.isToMe(msg) {
		c.logger.Warnf("discard msg due to not to me (%s)%s", msg.DstAddr, msg.DstDevId)
		return
	}

	cloneMsg := msg.Clone()
	if msg.Sync {
		go func() {
			err := c.sync(msg.SrcAddr, msg.DstAddr, msg.Dport, msg.Buffer)
			if err != nil {
				c.logger.Debugf("sync failed, %s", err)
			}
		}()
	}
	cloneMsg.Sync = false // set false for the original receiver
	// @todo use goroutine
	c.handleMessage(cloneMsg)
}

func (c *NetController) send(src, dst, dport, devid string, buf *buffer.Buffer, sync bool) error {
	if c.DevId == "" {
		c.logger.Errorf("send message error, device id is empty")
		return fmt.Errorf("controller have empty device id")
	}

	if src == "" {
		src = c.addressManager.GetDefaultAddress()
	} else if !c.addressManager.IsLocalAddress(src) {
		return fmt.Errorf("source address %s is not local", src)
	}
	if src == "" || (dst == "" && devid == "") || dport == "" || buf.Len() == 0 {
		return fmt.Errorf("invalid info src(%s), dst(%s), dport(%s), bufLen(%d)", src, dst, dport, buf.Len())
	}

	if src == dst && (devid == "" || devid == c.DevId) {
		return fmt.Errorf("src and dst can not be same")
	}

	targetDevId := devid
	if (devid == "" || devid == c.DevId) && c.addressManager.IsLocalAddress(dst) {
		targetDevId = c.DevId
		msg := message.NewMessage(dport, src, dst, c.DevId, targetDevId, sync, buf)
		c.doRecv(msg)
		return nil
	}

	if !c.device.IsConnected() {
		return fmt.Errorf("device not connected yet")
	}

	addr := (*transport.TransportAddress)(nil)
	if targetDevId != "" {
		// try to get address from cache
		_addr, found := c.addrCache.Get(targetDevId)
		if found {
			addr = _addr.(*transport.TransportAddress)
		}
	}

	if addr == nil {
		// no cache, get address from address manager
		addr = c.addressManager.GetTransportAddress(dst, targetDevId, false)
		if addr == nil {
			return fmt.Errorf("not route for address(%s:%s)", devid, dst)
		}
	}

	targetDevId = addr.DeviceId.String()
	if targetDevId == c.DevId {
		// got address target device is self, but local not have this address
		// which means the addressInfo is not synced yet, try get address again with flush
		addr = c.addressManager.GetTransportAddress(dst, targetDevId, true)
		if addr == nil {
			return fmt.Errorf("not route for address(%s:%s)", devid, dst)
		}
		targetDevId = addr.DeviceId.String()
		if targetDevId == c.DevId {
			return fmt.Errorf("not route for address(%s:%s)", devid, dst)
		}
	}

	c.addrCache.Set(targetDevId, addr)
	_ = c.favoriteManager.CheckChannel(addr, dst)

	msg := message.NewMessage(dport, src, dst, c.DevId, targetDevId, sync, buf)
	bpc, err := c.bpManager.GetController(targetDevId, dport, BPControllerDirectionSend)
	if err == nil {
		err = bpc.SendEnqueue(msg)
	}
	if err != nil {
		c.logger.Errorf("send message error, %v", err)
	}
	return err
}

func (c *NetController) Send(src, dst, dport, devid string, buf *buffer.Buffer) error {
	return c.send(src, dst, dport, devid, buf, false)
}

func (c *NetController) SendToAll(src, dst, dport, devid string, buf *buffer.Buffer) error {
	return c.send(src, dst, dport, devid, buf, true)
}

func (c *NetController) SendToNeighbors(num int, src, dport string, buf *buffer.Buffer) error {
	addresses := c.device.GetNeighborDeviceAddresses(num)
	for _, addr := range addresses {
		_ = c.send(src, "", dport, addr.DeviceId.String(), buf, false)
	}
	return nil
}

func (c *NetController) sync(src, dst, dport string, buf *buffer.Buffer) error {
	if dport == "" || buf.Len() == 0 {
		return fmt.Errorf("invalid info dport(%s), bufLen(%d)", dport, buf.Len())
	}

	msg := message.NewMessage(dport, src, dst, c.DevId, "", true, buf)
	return c.addressManager.SyncData(dst, msg.GetBuffer())
}

func (c *NetController) Sync(address, dport string, buf *buffer.Buffer) error {
	if address == "" {
		address = c.addressManager.GetDefaultAddress()
	} else if !c.addressManager.IsLocalAddress(address) {
		return fmt.Errorf("source address %s is not local", address)
	}
	return c.sync(address, address, dport, buf)
}

func (c *NetController) Broadcast(src, dport string, buf *buffer.Buffer) error {
	if src == "" || dport == "" || buf.Len() == 0 {
		return fmt.Errorf("invalid info src(%s), dport(%s), bufLen(%d)", src, dport, buf.Len())
	}

	msg := message.NewMessage(dport, src, "", c.DevId, "", false, buf)
	err := c.device.Broadcast(msg.GetBuffer())
	if err != nil {
		return err
	}
	return nil
}

func (c *NetController) isToMe(msg *message.Message) bool {
	if msg.DstDevId == "" {
		return true
	}

	if msg.DstDevId != c.DevId {
		return false
	}

	return c.addressManager.IsLocalAddress(msg.DstAddr)
}

func (c *NetController) AddSendPort(dport string) {
	c.sendDports.Set(dport, struct{}{})
}

func (c *NetController) RemoveSendPort(dport string) {
	c.sendDports.Delete(dport)
}

func (c *NetController) IsSendPort(dport string) bool {
	_, found := c.sendDports.Get(dport)
	return found
}

func (c *NetController) OnMessage(dport string, f MsgHandler) {
	if f == nil {
		c.dportMsgHandler.Delete(dport)
	} else {
		c.dportMsgHandler.Set(dport, f)
	}
}

func (c *NetController) IsRecvPort(dport string) bool {
	_, found := c.dportMsgHandler.Get(dport)
	return found
}

func (c *NetController) GetTransportAddress(address string) *transport.TransportAddress {
	return c.addressManager.GetTransportAddress(address, "", true)
}

func (c *NetController) AddPeers(hosts []string) {
	c.device.AddPeers(hosts)
}

func (c *NetController) DelPeers(hosts []string) {
	// do nothing
}

func (c *NetController) GetPeers() []string {
	return c.device.GetPeers()
}
func (c *NetController) GetAddressInfo(address string) (server.AddressInfo, error) {
	return c.addressManager.GetAddressInfo(address)
}

func (c *NetController) IsOnline(address string) bool {
	return c.addressManager.GetTransportAddress(address, "", true) != nil
}

func (c *NetController) BindAddress(address string) {
	c.addressManager.BindAddress(address)
}

func (c *NetController) UnbindAddress(address string) {
	c.addressManager.UnbindAddress(address)
}

func (c *NetController) GetAddresses() []string {
	return c.addressManager.GetAddresses()
}

func (c *NetController) GetDefaultAddress() string {
	return c.addressManager.GetDefaultAddress()
}

func (c *NetController) AddFavorite(address string) {
	c.favoriteManager.AddFavorite(address)
}

func (c *NetController) DelFavorite(address string) {
	c.favoriteManager.DelFavorite(address)
}

func (c *NetController) GetFavorites() []string {
	return c.favoriteManager.GetFavorites()
}

func (c *NetController) Connect(address string) error {
	addr := c.GetTransportAddress(address)
	if addr == nil {
		return fmt.Errorf("not found address(%s)", address)
	}

	_, err := c.RPCCenter.Call(rpc.ConnectPeerRpcName, addr)
	return err
}
