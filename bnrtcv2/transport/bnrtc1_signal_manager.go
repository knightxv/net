package transport

import (
	"errors"
	"net"
	"time"

	"github.com/guabee/bnrtc/bnrtcv1"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/util"
)

const (
	SignalServerChangeDelayIntervalDefault = 5 * time.Second
	ConnectSignalServerIntervalDefault     = 30 * time.Second
)

type ISignalConnection interface {
	Name() string
	Send(data []byte) error
	OnMessage(func(message []byte))
	OnClose(func())
	Close()
}

type Bnrtc1SignalManager struct {
	ConnectSignalServerInterval time.Duration
	Pubsub                      *util.PubSub
	RPCCenter                   *util.RPCCenter
	signalService               ISignalService
	connections                 *util.SafeLinkMap /* map[string]Connection  devid-> Connection */
	stopChan                    chan bool
	eventChan                   chan struct{}
	isSignalServer              bool
	signalServerAddress         *TransportAddress
	onConnectionCallback        func(conn ISignalConnection)
	onDisconnectionCallback     func(conn ISignalConnection)
	logger                      *log.Entry
}

func NewBnrtc1SignalManager(signalService ISignalService, pubsub *util.PubSub, rpcCenter *util.RPCCenter) *Bnrtc1SignalManager {
	m := &Bnrtc1SignalManager{
		Pubsub:                      pubsub,
		RPCCenter:                   rpcCenter,
		signalService:               signalService,
		connections:                 util.NewSafeLinkMap(),
		stopChan:                    make(chan bool),
		eventChan:                   make(chan struct{}, 2),
		isSignalServer:              false,
		ConnectSignalServerInterval: ConnectSignalServerIntervalDefault,
		logger:                      log.NewLoggerEntry("transport-bnrtc1-manager"),
	}

	_, err := pubsub.SubscribeFunc("transport-bnrtc1-signal-manager", rpc.DeviceIdChangeTopic, func(msg util.PubSubMsgType) {
		devId := msg.(deviceid.DeviceId)
		m.logger = m.logger.WithField("deviceid", devId)
		if devId.IsTopNetwork() {
			m.isSignalServer = true
		} else {
			m.isSignalServer = false
			m.eventChan <- struct{}{}
		}
	})
	if err != nil {
		m.logger.Errorf("subscribe deviceid change failed: %s", err)
	}

	rpcCenter.OnRegist(rpc.GetSignalServerAddrRpcName, func(params ...util.RPCParamType) util.RPCResultType {
		if !m.isSignalServer {
			m.eventChan <- struct{}{}
		}
		return nil
	})

	signalService.OnConnection(func(conn ISignalConnection) {
		conn.OnClose(func() {
			m.logger.Infof("signal connection %s closed", conn.Name())
			m.connections.Delete(conn.Name())
			m.onDisconnection(conn)
		})
		m.logger.Infof("signal connection %s added", conn.Name())
		m.connections.Set(conn.Name(), conn)
		m.onConnection(conn)
	})

	m.Start()
	return m
}

func (m *Bnrtc1SignalManager) OnConnection(f func(conn ISignalConnection)) {
	m.onConnectionCallback = f
}

func (m *Bnrtc1SignalManager) onConnection(conn ISignalConnection) {
	handler := m.onConnectionCallback
	if handler != nil {
		handler(conn)
	}
}

func (m *Bnrtc1SignalManager) OnDisconnection(f func(conn ISignalConnection)) {
	m.onDisconnectionCallback = f
}

func (m *Bnrtc1SignalManager) onDisconnection(conn ISignalConnection) {
	handler := m.onDisconnectionCallback
	if handler != nil {
		handler(conn)
	}
}

func (m *Bnrtc1SignalManager) GetSignalServerAddress() *TransportAddress {
	return m.signalServerAddress
}

func (m *Bnrtc1SignalManager) Start() {
	go m.connectSignalServerWorker()
}

func (m *Bnrtc1SignalManager) Close() {
	m.stopChan <- true
}

func (m *Bnrtc1SignalManager) connectSignalServerWorker() {
	t := time.NewTicker(m.ConnectSignalServerInterval)
	connectTask := util.NewSingleTonTask(util.BindingFunc(m.doConnectSignalServer), util.AllowPending())
OUT:
	for {
		select {
		case <-t.C:
			connectTask.Run()
		case <-m.eventChan:
			connectTask.Run()
		case <-m.stopChan:
			close(m.eventChan)
			t.Stop()
			break OUT
		}
	}
}

func (m *Bnrtc1SignalManager) doConnectSignalServer() {
	if m.isSignalServer {
		return
	}

	// not need to change signal server when already connected
	conn := m.GetSignalConnection()
	if conn != nil {
		return
	}

	_signalServerAddr, err := m.RPCCenter.Call(rpc.GetSignalServerAddrRpcName)
	if err != nil {
		m.logger.Debugf("not signal server address getter")
		return
	}
	signalServerAddr := _signalServerAddr.(*TransportAddress)
	if signalServerAddr == nil {
		m.logger.Debugf("signal server address is nil")
		return
	}
	peerOptions := &bnrtcv1.InternetPeerOptions{
		Address: signalServerAddr.Addr.IP.String(),
		Port:    int32(signalServerAddr.Addr.Port),
	}
	conn, err = m.signalService.ConnectSignalServer(peerOptions)
	if err != nil {
		m.logger.Errorf("connect signal server %s failed, err %s", signalServerAddr, err)
		return
	}
	m.logger.Infof("connect signal server %s success", signalServerAddr)
	if m.signalServerAddress.String() != signalServerAddr.String() {
		m.signalServerAddress = signalServerAddr
		m.Pubsub.Publish(rpc.Bnrtc1SignalServerChangeTopic, m.signalServerAddress)
	}
	conn.OnClose(func() {
		m.signalServerAddress = nil
		m.Pubsub.Publish(rpc.Bnrtc1SignalServerChangeTopic, m.signalServerAddress)
		m.eventChan <- struct{}{}
		m.logger.Infof("signal server %s closed", signalServerAddr)
	})
}

func (m *Bnrtc1SignalManager) GetSignalConnection() ISignalConnection {
	if m.isSignalServer {
		return nil
	}

	_, conn, found := m.connections.Front()
	if !found {
		return nil
	}

	return conn.(ISignalConnection)
}

func (m *Bnrtc1SignalManager) CloseAllConnections() {
	for _, conn := range m.connections.Items() {
		conn.(ISignalConnection).Close()
	}
}

func (m *Bnrtc1SignalManager) GetConnection(devid deviceid.DeviceId) (ISignalConnection, error) {
	conn, found := m.connections.Get(devid.NodeId())
	if found {
		return conn.(ISignalConnection), nil
	}

	return nil, errors.New("not found")
}

func (m *Bnrtc1SignalManager) IsConnected(devid deviceid.DeviceId) bool {
	_, err := m.GetConnection(devid)
	return err == nil
}

func (m *Bnrtc1SignalManager) GetExternalIp(host string) (net.IP, error) {
	ip, err := m.signalService.GetLocalIp(host)
	if err != nil {
		m.logger.Debugf("get external ip failed: %s", err)
		return nil, err
	}

	return net.ParseIP(ip), nil
}

func (m *Bnrtc1SignalManager) SetExternalIp(ip net.IP) {
	if ip != nil {
		m.signalService.SetLocalIp(ip.String())
	}
}
