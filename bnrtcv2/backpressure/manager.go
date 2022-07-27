package backpressure

import (
	"fmt"
	"sync"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/net"
)

// backpressure controller type
type BPCType int

const (
	BPCTypeNop   BPCType = -1 // no-op controller
	BPCTypeCount BPCType = 0  // controller of which throttle control is based on count (watermark)
)

const (
	ControllerLimitDefault        = 1000
	ControllerExpiredTime         = 5 * time.Minute
	ControllerCheckExpiredIterval = 1 * time.Minute
	CapacityRecvQueueDefault      = 1024
)

type BPControllerDirection = net.BPControllerDirection

const (
	BPControllerDirectionSend = net.BPControllerDirectionSend
	BPControllerDirectionRecv = net.BPControllerDirectionRecv
)

type IBPController interface {
	net.IBPController
	OnClose(func())
	Close()
	LastAlive() time.Time
}

type BPManager struct {
	Id                   string
	CapacityRecvQueue    uint32
	Type                 BPCType
	Limit                int
	controllers          map[string]IBPController // dport -> controller
	ctrlLock             sync.Mutex
	abortAllWork         chan struct{}
	onControllerCallback func(net.IBPController)
	logger               *log.Entry
}

type Option func(option *BPManager)

func WithDeviceId(deviceId string) Option {
	return func(mgr *BPManager) {
		mgr.Id = deviceId
	}
}

func WithCapacity(cap uint32) Option {
	return func(mgr *BPManager) {
		if cap > 0 {
			mgr.CapacityRecvQueue = cap
		}
	}
}

func WithBpType(t BPCType) Option {
	return func(mgr *BPManager) {
		mgr.Type = t
	}
}

func WithLimit(limit int) Option {
	return func(mgr *BPManager) {
		if limit > 0 {
			mgr.Limit = limit
		}
	}
}

func NewManager(options ...Option) *BPManager {
	mgr := &BPManager{
		controllers:       make(map[string]IBPController),
		CapacityRecvQueue: CapacityRecvQueueDefault,
		Type:              BPCTypeCount,
		Limit:             ControllerLimitDefault,
		abortAllWork:      make(chan struct{}),
		logger:            log.NewLoggerEntry("backpressure"),
	}

	for _, option := range options {
		option(mgr)
	}

	mgr.logger.Infoln("manager created")
	go mgr.clearExpiredControllerWorker()
	return mgr
}

func (mgr *BPManager) clearExpiredControllerWorker() {
	t := time.NewTicker(ControllerCheckExpiredIterval)
OUT:
	for {
		select {
		case <-t.C:
			var controllers []IBPController = nil
			now := time.Now()
			for _, ctrl := range mgr.controllers {
				if now.Sub(ctrl.LastAlive()) > ControllerExpiredTime {
					controllers = append(controllers, ctrl)
				}
			}
			for _, ctrl := range controllers {
				ctrl.Close()
			}

		case <-mgr.abortAllWork:
			t.Stop()
			break OUT
		}
	}
}

func (mgr *BPManager) SetDeviceId(deviceId string) {
	if mgr.Id != deviceId {
		mgr.Id = deviceId
		mgr.logger = mgr.logger.WithField("deviceId", deviceId)
		mgr.clear()
	}
}

func (mgr *BPManager) findController(deviceId string, dport string) (net.IBPController, bool) {
	name := deviceId + ":" + dport

	mgr.ctrlLock.Lock()
	defer mgr.ctrlLock.Unlock()
	ctrl, found := mgr.controllers[name]
	return ctrl, found
}

func (mgr *BPManager) GetController(deviceId string, dport string, direction net.BPControllerDirection) (net.IBPController, error) {
	if deviceId == "" || dport == "" {
		return nil, fmt.Errorf("invalid deviceId=%s, dport=%s", deviceId, dport)
	}
	name := deviceId + ":" + dport

	mgr.ctrlLock.Lock()
	defer mgr.ctrlLock.Unlock()

	ctrl, found := mgr.controllers[name]
	if found {
		return ctrl, nil
	}

	// disallow new received controller when touch limit
	if direction == net.BPControllerDirectionRecv {
		if len(mgr.controllers) >= mgr.Limit {
			return nil, fmt.Errorf("controllers exceed limit=%d, refuse", mgr.Limit)
		}
	}

	ctrl, err := mgr.newController(deviceId, dport, mgr.Type, mgr.CapacityRecvQueue)
	if err != nil {
		return nil, err
	}

	mgr.controllers[name] = ctrl
	mgr.logger.Infof("add ctrl %s", name)
	ctrl.OnClose(func() { // on controller closed
		mgr.ctrlLock.Lock()
		delete(mgr.controllers, name)
		mgr.ctrlLock.Unlock()
		mgr.logger.Infof("del ctrl %s", name)
	})

	if mgr.onControllerCallback != nil {
		mgr.onControllerCallback(ctrl)
	}

	return ctrl, nil
}

func (mgr *BPManager) CloseController(deviceId string, dport string) error {
	if deviceId == "" || dport == "" {
		return fmt.Errorf("invalid deviceId=%s, dport=%s", deviceId, dport)
	}

	ctrl, found := mgr.findController(deviceId, dport)
	if !found {
		return fmt.Errorf("controller not found")
	}
	ctrl.Close()
	return nil
}

func (mgr *BPManager) newController(deviceId string, dport string, bpcType BPCType, recvCapacity uint32) (IBPController, error) {
	var controller IBPController
	switch bpcType {
	case BPCTypeCount:
		controller = newCountBPController(mgr.Id, deviceId, dport, recvCapacity, mgr.logger)
	case BPCTypeNop:
		controller = newNopBPController(mgr.Id, deviceId, dport, mgr.logger)
	default:
		return nil, fmt.Errorf("invalid controller type= %d", bpcType)
	}
	return controller, nil
}

func (mgr *BPManager) OnController(handler func(ctrl net.IBPController)) {
	mgr.onControllerCallback = handler
}

func (mgr *BPManager) clear() {
	mgr.ctrlLock.Lock()
	defer mgr.ctrlLock.Unlock()
	if len(mgr.controllers) == 0 {
		return
	}

	for _, ctrl := range mgr.controllers {
		ctrl.OnClose(nil) // disable notify to avoid dead lock
	}

	for _, ctrl := range mgr.controllers {
		ctrl.Close()
	}
	mgr.controllers = make(map[string]IBPController)
}

func (mgr *BPManager) Destroy() {
	mgr.logger.Infof("destroy")
	mgr.clear()
	mgr.abortAllWork <- struct{}{}
}
