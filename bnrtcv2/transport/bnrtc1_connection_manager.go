package transport

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/guabee/bnrtc/bnrtcv1"
	"github.com/guabee/bnrtc/bnrtcv2/connection"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/util"
)

const (
	CheckConnectionExpiredIntervalDefault = 5 * 60 * time.Second
	ConnectionExpireTimeDefault           = 30 * 60 * time.Second
	ConnectionLimitDefault                = 1000
	ConnectionLimitMax                    = 20000
)

type IConnection interface {
	Name() string
	Send(data []byte) error
	OnMessage(f func(data []byte))
}

type Bnrtc1ConnectionManager struct {
	Pubsub                         *util.PubSub
	RPCCenter                      *util.RPCCenter
	ConnectionLimit                int
	CheckConnectionExpiredInterval time.Duration
	ConnectionExpireTime           time.Duration
	peerService                    IPeerService
	connections                    *util.SafeLinkMap /* map[string]Connection  devid-> Connection */
	stopChan                       chan bool
	onConnectionCallback           func(conn IConnection)
	onDisconnectionCallback        func(conn IConnection)
	logger                         *log.Entry
	connectLock                    sync.Mutex
}

type ConnectionEventHook struct {
	manager *Bnrtc1ConnectionManager
}

func (hook *ConnectionEventHook) OnNatDuplexConnected(natDuplex *bnrtcv1.NatDuplex) {
	if strings.HasPrefix(natDuplex.Source, "bnrtc1") {
		hook.manager.logger.Infof("connected peer %s", natDuplex.RemoteNatName)
	} else {
		hook.manager.logger.Infof("peer %s connected", natDuplex.RemoteNatName)
		conn := connection.NewConnection(natDuplex)
		hook.manager.addConnection(conn)
	}
}

func (hook *ConnectionEventHook) OnNatDuplexDisConnected(natDuplex *bnrtcv1.NatDuplex) {
	hook.manager.logger.Infof("peer %s disconnected", natDuplex.RemoteNatName)
	hook.manager.disconnectByNodeId(natDuplex.RemoteNatName)
}

type Bnrtc1ConnectionManagerConfig struct {
	ConnectionLimit                int
	CheckConnectionExpiredInterval time.Duration
	ConnectionExpireTime           time.Duration
}

func NewBnrtc1ConnectionManager(peerService IPeerService, pubsub *util.PubSub, rpcCenter *util.RPCCenter, config Bnrtc1ConnectionManagerConfig) *Bnrtc1ConnectionManager {
	m := &Bnrtc1ConnectionManager{
		Pubsub:                         pubsub,
		RPCCenter:                      rpcCenter,
		peerService:                    peerService,
		connections:                    util.NewSafeLinkMap(),
		stopChan:                       make(chan bool),
		CheckConnectionExpiredInterval: CheckConnectionExpiredIntervalDefault,
		ConnectionExpireTime:           ConnectionExpireTimeDefault,
		ConnectionLimit:                ConnectionLimitDefault,
		logger:                         log.NewLoggerEntry("transport-bnrtc1-connection-manager"),
	}

	if config.ConnectionLimit > 0 {
		m.ConnectionLimit = config.ConnectionLimit
	}
	if config.CheckConnectionExpiredInterval > 0 {
		m.CheckConnectionExpiredInterval = config.CheckConnectionExpiredInterval
	}
	if config.ConnectionExpireTime > 0 {
		m.ConnectionExpireTime = config.ConnectionExpireTime
	}

	_, err := pubsub.SubscribeFunc("transport-bnrtc1-connection-manager", rpc.DeviceIdChangeTopic, func(msg util.PubSubMsgType) {
		devId := msg.(deviceid.DeviceId)
		m.logger = m.logger.WithField("deviceid", devId)
	})
	if err != nil {
		m.logger.Errorf("subscribe deviceid change failed: %s", err)
	}

	rpcCenter.Regist(rpc.ConnectPeerRpcName, func(params ...util.RPCParamType) util.RPCResultType {
		if len(params) != 1 {
			return nil
		}
		addr := params[0].(*TransportAddress)
		if m.IsConnected(addr.DeviceId) {
			return true
		}

		err := m.Connect(addr)
		return err == nil
	})
	m.peerService.BindEventHook(&ConnectionEventHook{manager: m})
	m.Start()
	return m
}

func (m *Bnrtc1ConnectionManager) OnConnection(f func(conn IConnection)) {
	m.onConnectionCallback = f
}

func (m *Bnrtc1ConnectionManager) onConnection(conn IConnection) {
	handler := m.onConnectionCallback
	if handler != nil {
		handler(conn)
	}
}

func (m *Bnrtc1ConnectionManager) OnDisconnection(f func(conn IConnection)) {
	m.onDisconnectionCallback = f
}

func (m *Bnrtc1ConnectionManager) onDisconnection(conn IConnection) {
	handler := m.onDisconnectionCallback
	if handler != nil {
		handler(conn)
	}
}

func (m *Bnrtc1ConnectionManager) GetConnectionLimit() int {
	return m.ConnectionLimit
}

func (m *Bnrtc1ConnectionManager) GetConnectionSize() int {
	return m.connections.Size()
}

func (m *Bnrtc1ConnectionManager) Start() {
	go m.clearExpiredConnectionWorker()
}

func (m *Bnrtc1ConnectionManager) Close() {
	m.stopChan <- true
}

func (m *Bnrtc1ConnectionManager) clearExpiredConnectionWorker() {
	m.logger.Debugln("clearExpiredConnectionWorker start")

	t := time.NewTicker(m.CheckConnectionExpiredInterval)
OUT:
	for {
		select {
		case <-t.C:
			now := time.Now()
			count := 0
			for {
				_, _conn, ok := m.connections.Front()
				if !ok {
					break
				}
				conn := _conn.(*connection.Connection)
				if now.Sub(conn.LastDataTime) > m.ConnectionExpireTime {
					count += 1
					conn.Close()
				} else {
					break
				}
			}
			m.logger.Debugf("clearExpiredConnectionWorker clear %d expired connection", count)
		case <-m.stopChan:
			t.Stop()
			break OUT
		}
	}
	m.logger.Debugln("clearExpiredConnectionWorker stop")
}

func (m *Bnrtc1ConnectionManager) addConnection(conn *connection.Connection) *connection.Connection {
	oldConnection := m.getConnectionByNodeId(conn.Name())
	if oldConnection != nil {
		conn.Close()
		return oldConnection
	}

	conn.OnClose(func() {
		m.logger.Infof("connection %s closed", conn.Name())
		m.onDisconnection(conn)
		m.connections.Delete(conn.Name())
	})

	conn.OnAlive(func() {
		m.connections.MoveToBack(conn.Name())
	})

	conn.OnNameChanged(func(oldName, newName string) {
		m.connections.Delete(oldName)
		m.connections.Set(newName, conn)
	})

	m.connections.Set(conn.Name(), conn)
	if m.connections.Size() > m.ConnectionLimit {
		_, _conn, ok := m.connections.Shift()
		if ok {
			conn := _conn.(*connection.Connection)
			m.logger.Infof("close connection %s due to limit %d", conn.Name(), m.ConnectionLimit)
			conn.Close()
		}
	}

	m.onConnection(conn)
	return conn
}

func (m *Bnrtc1ConnectionManager) connect(address string, port int, devid deviceid.DeviceId) (*connection.Connection, error) {
	if m.peerService == nil {
		return nil, fmt.Errorf("peer service is nil")
	}

	if isLocal(address) && port == int(m.peerService.GetServicePort()) {
		return nil, fmt.Errorf("can not connect to self")
	}

	peerOptions := &bnrtcv1.InternetPeerOptions{
		Address: address,
		Port:    int32(port),
	}

	natDuplex, err := m.peerService.LinkNatPeer(peerOptions, devid.NodeId())
	if err != nil {
		m.logger.Errorf("link nat peer %s failed, err %s", devid, err)
		return nil, err
	}

	conn := connection.NewConnection(natDuplex)
	m.logger.Infof("connect to peer %s:%d/%s -> %s success", address, port, devid, conn.Name())
	return conn, nil
}

func (m *Bnrtc1ConnectionManager) getConnectionByNodeId(nodeId string) *connection.Connection {
	_conn, found := m.connections.Get(nodeId)
	if found {
		return _conn.(*connection.Connection)
	}

	return nil
}

func (m *Bnrtc1ConnectionManager) GetConnection(devid deviceid.DeviceId) (IConnection, error) {
	conn := m.getConnectionByNodeId(devid.NodeId())
	if conn == nil {
		return nil, fmt.Errorf("connection %s not found", devid)
	} else {
		return conn, nil
	}
}

func (m *Bnrtc1ConnectionManager) IsConnected(devid deviceid.DeviceId) bool {
	_, err := m.GetConnection(devid)
	return err == nil
}

func (m *Bnrtc1ConnectionManager) Connect(addr *TransportAddress) error {
	m.connectLock.Lock()
	defer m.connectLock.Unlock()
	_, err := m.GetConnection(addr.DeviceId)
	if err == nil {
		return nil
	}

	conn, err := m.connect(addr.Addr.IP.String(), addr.Addr.Port, addr.DeviceId)
	if err != nil {
		return err
	}

	m.addConnection(conn)
	return nil
}

func (m *Bnrtc1ConnectionManager) disconnectByNodeId(nodeid string) {
	conn := m.getConnectionByNodeId(nodeid)
	if conn != nil {
		conn.Close()
	}
}

func (m *Bnrtc1ConnectionManager) Disconnect(devid deviceid.DeviceId) {
	m.disconnectByNodeId(devid.NodeId())
}
