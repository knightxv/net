package broadcast

import (
	"fmt"

	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	kaddht "github.com/guabee/bnrtc/bnrtcv2/dht/kad-dht"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
	"github.com/guabee/bnrtc/buffer"
)

type DefaultBroadcastStrategy2 struct {
	DefaultBroadcastStrategy
	Id               deviceid.DeviceId
	Info             *device_info.DeviceInfo
	dht              IDHT
	transportManager transport.ITransportManager
}

type StrategyOption2 func(s *DefaultBroadcastStrategy2)

func NewDefaultBroadcastStrategy2(
	info *device_info.DeviceInfo,
	transportManager transport.ITransportManager,
	dht IDHT,
	options ...StrategyOption2,
) *DefaultBroadcastStrategy2 {
	bs := &DefaultBroadcastStrategy2{
		Info:             info,
		Id:               info.Id,
		dht:              dht,
		transportManager: transportManager,
	}

	for _, option := range options {
		option(bs)
	}

	return bs
}

func (bs *DefaultBroadcastStrategy2) SetDeviceInfo(info *device_info.DeviceInfo) {
	bs.Info = info
	bs.Id = info.Id
}

func (bs *DefaultBroadcastStrategy2) Broadcast(buf *buffer.Buffer) error {
	bs.IterateBroadcast(0, func(addr *transport.TransportAddress) error {
		return bs.transportManager.SendBroadcastMsg(
			addr,
			buf.Clone(),
			[]byte{1},
		)
	})
	bs.dht.IterateBucketIdx(
		func(di *device_info.DeviceInfo) bool {
			return !di.Id.IsTopNetwork()
		},
		func(idx int, bucketCnt int, info *device_info.DeviceInfo) error {
			addr, err := bs.Info.ChooseMatchedAddress([]*device_info.DeviceInfo{info}, info.Id, bs.transportManager)
			if err != nil {
				return err
			}
			return bs.transportManager.SendBroadcastMsg(addr, buf.Clone(), nil)
		},
	)
	return nil
}

func (bs *DefaultBroadcastStrategy2) HandleDataMessage(msg *transport.BroatcastMessage) {
	if bs.Id.IsTopNetwork() {
		// broadcast to top net
		index := bs.Id.HighestDifferBit(msg.Sender.DeviceId) + 1
		bs.IterateBroadcast(index, func(addr *transport.TransportAddress) error {
			return bs.transportManager.BroadcastDataMsg(
				addr,
				msg.Buffer.Clone(),
				nil,
			)
		})
	}
	// broadcast to sub net
	bs.dht.IterateBucketIdx(
		func(di *device_info.DeviceInfo) bool {
			if di.Id.IsTopNetwork() {
				return false
			}
			if bs.Id.IsTopNetwork() {
				return true
			}
			if msg.Sender.DeviceId.Equal(di.Id) {
				return false
			}
			return di.Id.SameGroupId(di.Id)
		},
		func(idx int, bucketCnt int, info *device_info.DeviceInfo) error {
			addr, err := bs.Info.ChooseMatchedAddress([]*device_info.DeviceInfo{info}, info.Id, bs.transportManager)
			if err != nil {
				return err
			}
			bs.transportManager.SendBroadcastMsg(addr, msg.Buffer.Clone(), nil)
			return fmt.Errorf("broadcast all sub net node")
		},
	)
}

func (bs *DefaultBroadcastStrategy2) IterateBroadcast(
	ttl int,
	sendHook func(addr *transport.TransportAddress) error,
) {
	mirrorOrigin := bs.Id.ToTopNetwork()
	for {
		if ttl < 0 || ttl > kaddht.DeviceIDBits-1 {
			return
		}
		targetID := mirrorOrigin.BitwiseInversion(ttl)
		err := bs.broadcastImageTarget(
			targetID,
			ttl,
			sendHook,
			func(ta *transport.TransportAddress) {
				ttl = mirrorOrigin.HighestDifferBit(ta.DeviceId) + 1
			},
		)
		// fmt.Printf("mirrorOrigin: %s, ttl:%d\n", mirrorOrigin.String(), ttl)
		if err != nil {
			ttl += 1
		}
	}
}

func (bs *DefaultBroadcastStrategy2) broadcastImageTarget(
	targetID deviceid.DeviceId,
	ttl int,
	sendHook func(addr *transport.TransportAddress) error,
	callback func(*transport.TransportAddress),
) error {
	return bs.dht.IterateClosestNode(
		targetID,
		func(di *device_info.DeviceInfo) bool {
			if bs.Id.Compare(di.Id) == 0 {
				return false
			}
			return targetID.HighestDifferBit(di.Id) > ttl
		},
		func(info *device_info.DeviceInfo) error {
			addr, err := bs.Info.ChooseMatchedAddress(
				[]*device_info.DeviceInfo{info},
				info.Id,
				bs.transportManager,
			)
			if err != nil {
				return err
			}
			broadcastError := sendHook(addr)
			if broadcastError == nil {
				callback(addr)
			}
			return broadcastError
		},
	)
}
