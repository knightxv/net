package broadcast

import (
	"fmt"

	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
)

type IDHT interface {
	IterateBucketIdx(filter func(*device_info.DeviceInfo) bool, callback func(int, int, *device_info.DeviceInfo) error)
	IterateClosestNode(targetId deviceid.DeviceId, filter func(*device_info.DeviceInfo) bool, callback func(*device_info.DeviceInfo) error) error
}

type IBroadcasrStrategy interface {
	Broadcast(*buffer.Buffer) error
	HandleDataMessage(msg *transport.BroatcastMessage)
	SetDeviceInfo(*device_info.DeviceInfo)
}

type Option func(option *BroadcastCtrl)

func WithPubSub(pubsub *util.PubSub) Option {
	return func(m *BroadcastCtrl) {
		m.Pubsub = pubsub
	}
}

func WithStrategy(strategy IBroadcasrStrategy) Option {
	return func(m *BroadcastCtrl) {
		m.strategy = strategy
	}
}

type BroadcastCtrl struct {
	Id       deviceid.DeviceId
	Info     *device_info.DeviceInfo
	Pubsub   *util.PubSub
	strategy IBroadcasrStrategy
	hook     func(msg *transport.BroatcastMessage)
}

func NewBroadcastCtrl(info *device_info.DeviceInfo, transportManager transport.ITransportManager, dht IDHT, options ...Option) *BroadcastCtrl {
	bc := &BroadcastCtrl{
		Info:     info,
		Id:       info.Id,
		strategy: NewDefaultBroadcastStrategy2(info, transportManager, dht),
		Pubsub:   util.DefaultPubSub(),
	}

	for _, option := range options {
		option(bc)
	}

	_, err := bc.Pubsub.SubscribeFunc("broadcast", rpc.DeviceInfoChangeTopic, func(psmt util.PubSubMsgType) {
		info := psmt.(*device_info.DeviceInfo)
		bc.Info = info
		bc.Id = info.Id
		bc.strategy.SetDeviceInfo(info)
	})
	if err != nil {
		fmt.Printf("broadcast subscribe device info change topic error: %s\n", err)
	}

	go func() {
		for msg := range transportManager.GetBroadcastDataMsgChan() {
			if msg.IsExpired() {
				continue
			}
			if bc.hook != nil {
				bc.hook(msg)
			}
			bc.strategy.HandleDataMessage(msg)
		}
	}()
	return bc
}

func (bc *BroadcastCtrl) AddMessageHook(hook func(msg *transport.BroatcastMessage)) {
	bc.hook = hook
}

func (bc *BroadcastCtrl) Broadcast(buf *buffer.Buffer) error {
	return bc.strategy.Broadcast(buf)
}
