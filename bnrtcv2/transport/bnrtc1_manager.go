package transport

import (
	"sync"
	"time"

	"github.com/guabee/bnrtc/bnrtcv1"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/util"
)

type Bnrtc1Manager struct {
	Pubsub                         *util.PubSub
	RPCCenter                      *util.RPCCenter
	devId                          deviceid.DeviceId
	peerServiceBuilder             IBnrtc1ServiceBuilder
	initPeerServiceOnce            sync.Once
	ConnectionExpireTime           time.Duration
	ConnectionLimit                int
	CheckConnectionExpiredInterval time.Duration
	signalManager                  *Bnrtc1SignalManager
	connectionManager              *Bnrtc1ConnectionManager
	onServiceHandler               func(service IBnrtc1Service)
	logger                         *log.Entry
}

type Bnrtc1ManagerOption func(*Bnrtc1Manager)

func WithPubSub(pubsub *util.PubSub) Bnrtc1ManagerOption {
	return func(m *Bnrtc1Manager) {
		m.Pubsub = pubsub
	}
}

func WithRPCCenter(rpcCenter *util.RPCCenter) Bnrtc1ManagerOption {
	return func(m *Bnrtc1Manager) {
		m.RPCCenter = rpcCenter
	}
}

func WithConnectionLimit(limit int) Bnrtc1ManagerOption {
	return func(m *Bnrtc1Manager) {
		if limit > 0 && limit < ConnectionLimitMax {
			m.ConnectionLimit = limit
		}
	}
}

func WithConnectionExpired(expiredTime time.Duration) Bnrtc1ManagerOption {
	return func(m *Bnrtc1Manager) {
		if expiredTime > 0 {
			m.ConnectionExpireTime = expiredTime
		}
	}
}

func WithCheckSignalServerTime(checkTime time.Duration) Bnrtc1ManagerOption {
	return func(m *Bnrtc1Manager) {
		if checkTime > 0 {
			m.CheckConnectionExpiredInterval = checkTime
		}
	}
}

func NewBnrtc1Manager(devid deviceid.DeviceId, bnrtc1ServerOptions *bnrtcv1.Bnrtc1ServerOptions, options ...Bnrtc1ManagerOption) *Bnrtc1Manager {
	m := &Bnrtc1Manager{
		devId:     devid,
		Pubsub:    util.DefaultPubSub(),
		RPCCenter: util.DefaultRPCCenter(),
		logger:    log.NewLoggerEntry("transport-bnrtc1-manager"),
	}

	for _, option := range options {
		option(m)
	}

	server, err := bnrtcv1.NewServer(bnrtc1ServerOptions, nil)
	if err != nil {
		m.logger.Errorf("create bnrtc1 server failed: %s", err)
		return nil
	}

	err = server.Start()
	if err != nil {
		m.logger.Errorf("start bnrtc1 server failed: %s", err)
		return nil
	}
	bnrtc1Server := NewBnrtc1ServerWrapper(server)
	m.connectionManager = NewBnrtc1ConnectionManager(
		bnrtc1Server.GetPeerService(),
		m.Pubsub,
		m.RPCCenter,
		Bnrtc1ConnectionManagerConfig{
			ConnectionExpireTime:           m.ConnectionExpireTime,
			ConnectionLimit:                m.ConnectionLimit,
			CheckConnectionExpiredInterval: m.CheckConnectionExpiredInterval,
		},
	)
	m.signalManager = NewBnrtc1SignalManager(
		bnrtc1Server.GetSignalService(),
		m.Pubsub,
		m.RPCCenter,
	)

	_, err = m.Pubsub.SubscribeFunc("bnrtc1-manager", rpc.DeviceIdChangeTopic, func(msg util.PubSubMsgType) {
		devId := msg.(deviceid.DeviceId)
		m.devId = devId
		m.logger = m.logger.WithField("deviceid", m.devId)
	})
	if err != nil {
		m.logger.Errorf("subscribe deviceid change failed: %s", err)
	}

	return m
}

func (m *Bnrtc1Manager) GetSignalManager() *Bnrtc1SignalManager {
	return m.signalManager
}

func (m *Bnrtc1Manager) GetConnectionManager() *Bnrtc1ConnectionManager {
	return m.connectionManager
}

func (m *Bnrtc1Manager) IsConnected(deviceId deviceid.DeviceId) bool {
	return m.connectionManager.IsConnected(deviceId) || m.signalManager.IsConnected(deviceId)
}

func (m *Bnrtc1Manager) Connect(addr *TransportAddress) error {
	return m.connectionManager.Connect(addr)
}

func (m *Bnrtc1Manager) GetConnectionLimit() int {
	return m.connectionManager.GetConnectionLimit()
}

func (m *Bnrtc1Manager) GetConnectionSize() int {
	return m.connectionManager.GetConnectionSize()
}
