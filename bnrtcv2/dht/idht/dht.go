package idht

import (
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/config"
	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
)

type DHTStatus = string

const (
	DHTStatusReady        DHTStatus = "ready"
	DHTStatusConnected    DHTStatus = "connected"
	DHTStatusDisconnected DHTStatus = "disconnected"
)

type ITransportManager interface {
	SendCtrlMsg(addr *transport.TransportAddress, buf *buffer.Buffer) error
	GetCtrlMsgChan() <-chan *transport.Message
	IsTargetReachable(addr *transport.TransportAddress) bool
}

type DHTStoreOptions struct {
	TExpire     time.Duration `json:"TExpire"`
	TReplicate  time.Duration `json:"TReplicate"`
	NoCache     bool          `json:"NoCache"`
	NoReplicate bool          `json:"NoReplicate"`
}

type DHTOption struct {
	DeviceInfo            *device_info.DeviceInfo
	Peers                 []*transport.TransportAddress
	TransportManager      ITransportManager
	PubSub                *util.PubSub
	DHTMemoryStoreOptions *config.DHTMemoryStoreOptions
}

type IDHT interface {
	IsConnected() bool
	SetDeviceInfo(devInfo *device_info.DeviceInfo)
	IterateBucketIdx(filter func(*device_info.DeviceInfo) bool, apply func(int, int, *device_info.DeviceInfo) error)
	IterateClosestNode(targetId deviceid.DeviceId, filter func(*device_info.DeviceInfo) bool, apply func(*device_info.DeviceInfo) error) error
	Put(key deviceid.DataKey, data []byte, options *DHTStoreOptions) error
	Update(key deviceid.DataKey, oldData, newData []byte, options *DHTStoreOptions) error
	Get(key deviceid.DataKey, noCache bool) (data []byte, found bool, err error)
	Delete(key deviceid.DataKey) error
	AddPeer(info *device_info.DeviceInfo)
	GetClosestTopNetworkDeviceInfo() *device_info.DeviceInfo
	GetDeviceInfo(id deviceid.DeviceId) *device_info.DeviceInfo
	GetNeighborDeviceInfo(num int) []*device_info.DeviceInfo
}
