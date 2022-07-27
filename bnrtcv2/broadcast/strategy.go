package broadcast

import (
	"encoding/binary"
	"fmt"

	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
	"github.com/guabee/bnrtc/buffer"
)

func uint32ToBytes(v uint32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, v)
	return buf
}

const (
	IterateBucketIdxRatioDefault = 0.3
	IterateBucketMaxCountDefault = 3
)

type DefaultBroadcastStrategy struct {
	Id                    deviceid.DeviceId
	Info                  *device_info.DeviceInfo
	dht                   IDHT
	transportManager      transport.ITransportManager
	IterateBucketIdxRatio float32
	IterateBucketMaxCount int
}

type StrategyOption func(s *DefaultBroadcastStrategy)

func WithRatio(ratio float32) StrategyOption {
	return func(s *DefaultBroadcastStrategy) {
		s.IterateBucketIdxRatio = ratio
	}
}

func WithMaxCount(count int) StrategyOption {
	return func(s *DefaultBroadcastStrategy) {
		s.IterateBucketMaxCount = count
	}
}

func NewDefaultBroadcastStrategy(info *device_info.DeviceInfo, transportManager transport.ITransportManager, dht IDHT, options ...StrategyOption) *DefaultBroadcastStrategy {
	bs := &DefaultBroadcastStrategy{
		Info:                  info,
		Id:                    info.Id,
		dht:                   dht,
		transportManager:      transportManager,
		IterateBucketIdxRatio: IterateBucketIdxRatioDefault,
		IterateBucketMaxCount: IterateBucketMaxCountDefault,
	}

	for _, option := range options {
		option(bs)
	}

	return bs
}

func (bs *DefaultBroadcastStrategy) SetDeviceInfo(info *device_info.DeviceInfo) {
	bs.Info = info
	bs.Id = info.Id
}

func (bs *DefaultBroadcastStrategy) Broadcast(buf *buffer.Buffer) error {
	/// broadcast for each buckets
	bs.dht.IterateBucketIdx(nil, func(idx int, bucketCnt int, info *device_info.DeviceInfo) error {
		addr, err := bs.Info.ChooseMatchedAddress([]*device_info.DeviceInfo{info}, info.Id, bs.transportManager)
		if err != nil {
			return err
		}
		return bs.transportManager.SendBroadcastMsg(addr, buf.Clone(), uint32ToBytes(uint32(bs.Id.HighestDifferBit(info.Id))))
	})
	return nil
}

func (bs *DefaultBroadcastStrategy) getMaxCount(bucketCnt int) int {
	max := int(float32(bucketCnt) * bs.IterateBucketIdxRatio)
	if max > bs.IterateBucketMaxCount {
		return bs.IterateBucketMaxCount
	} else {
		return max
	}
}

/// handler broadcast message
func (bs *DefaultBroadcastStrategy) HandleDataMessage(msg *transport.BroatcastMessage) {
	ttl := binary.BigEndian.Uint32(msg.Extra)
	buf := msg.Buffer
	isTopNet := bs.Id.IsTopNetwork()
	difbit := uint32(0)
	filter := func(info *device_info.DeviceInfo) bool {
		if !isTopNet {
			if bs.Id.InSameGroup(info.Id) {
				return true
			} else {
				return false // subnet not forward to topnet
			}
		} else {
			if !info.Id.IsTopNetwork() {
				return true
			}
		}

		difbit = uint32(bs.Id.HighestDifferBit(info.Id))

		return difbit > ttl
	}
	iterateBucketIndex := -1
	iterateBucketIdxCount := 0
	/// broadcast for each buckets
	bs.dht.IterateBucketIdx(filter, func(idx int, bucketCnt int, info *device_info.DeviceInfo) error {
		if idx != iterateBucketIndex {
			iterateBucketIndex = idx
			iterateBucketIdxCount = 0
		}
		addr, err := bs.Info.ChooseMatchedAddress([]*device_info.DeviceInfo{info}, info.Id, bs.transportManager)
		if err != nil {
			return err
		}
		broadcastError := bs.transportManager.BroadcastDataMsg(addr, buf.Clone(), uint32ToBytes(difbit))
		if broadcastError != nil {
			return broadcastError
		}
		iterateBucketIdxCount++
		if iterateBucketIdxCount >= bs.getMaxCount(bucketCnt) {
			iterateBucketIndex = -1
			return nil
		}
		return fmt.Errorf(
			"iterateBucketIdxCount(%d) Not reached IterateBucketIdxMinCount(%d)",
			iterateBucketIdxCount,
			bs.getMaxCount(bucketCnt),
		)
	})
}
