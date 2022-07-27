package net

import (
	"context"
	"sort"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
	"github.com/guabee/bnrtc/util"
)

const (
	StaticInfoCheckIntervalDefault = 300 * time.Second
)

type IConnectionManager interface {
	Connect(addr *transport.TransportAddress) error
	IsConnected(deviceId deviceid.DeviceId) bool
	GetConnectionLimit() int
	GetConnectionSize() int
}

type DeviceStatisItem struct {
	deviceId string
	count    uint64
}

type DirectChannelManagerOption struct {
	StatisManager           *StatisManager
	Device                  IDevice
	ConnectionManager       IConnectionManager
	StaticInfoCheckInterval time.Duration
}

type DirectChannelManager struct {
	connectionManager       IConnectionManager
	device                  IDevice
	deviceStatisMap         *util.SafeMap // [string]*DeviceStatisItem
	topSendDevices          *util.SafeMap // map[string]string
	staticInfoCheckInterval time.Duration
}

func NewDirectChannelManager(ctx context.Context, option *DirectChannelManagerOption) *DirectChannelManager {
	m := &DirectChannelManager{
		connectionManager:       option.ConnectionManager,
		device:                  option.Device,
		deviceStatisMap:         util.NewSafeMap(), // make(map[string]*DeviceStatisItem),
		topSendDevices:          util.NewSafeMap(), // make(map[string]string),
		staticInfoCheckInterval: option.StaticInfoCheckInterval,
	}

	if m.staticInfoCheckInterval <= 0 {
		m.staticInfoCheckInterval = StaticInfoCheckIntervalDefault
	}

	option.StatisManager.OnChange(m.onStatisChange)

	go m.updateDeviceStatisWorker(ctx)
	return m
}

func (m *DirectChannelManager) onStatisChange(deviceId string, statis *Statis, bytes uint64, isSend bool) {
	// only care send
	if !isSend {
		return
	}

	// ignore topnetwork device
	devid := deviceid.FromString(deviceId)
	if devid.IsTopNetwork() {
		return
	}

	item, found := m.deviceStatisMap.Get(deviceId)
	if !found {
		item = &DeviceStatisItem{
			deviceId: deviceId,
			count:    0,
		}
		m.deviceStatisMap.Set(deviceId, item)
	}

	item.(*DeviceStatisItem).count++
}

func (m *DirectChannelManager) CheckChannel(address *transport.TransportAddress, force bool) error {
	// ignore topnetwork device
	if address.DeviceId.IsTopNetwork() {
		return nil
	}

	if m.connectionManager.IsConnected(address.DeviceId) {
		return nil
	}

	// ignore ipv6 address
	if !address.IsIPv4() {
		return nil
	}

	if !force {
		if !m.topSendDevices.Has(address.DeviceId.String()) && m.connectionManager.GetConnectionSize() >= m.connectionManager.GetConnectionLimit() {
			return nil
		}
	}

	return m.connectionManager.Connect(address)
}

func (m *DirectChannelManager) updateDeviceStatisWorker(ctx context.Context) {
	t := time.NewTicker(m.staticInfoCheckInterval)
	for {
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
			m.doUpdateDeviceStatis()
		}
	}
}

func (m *DirectChannelManager) doUpdateDeviceStatis() {
	deviceStatisMap := m.deviceStatisMap
	if deviceStatisMap.Size() == 0 {
		return // no change
	}
	m.deviceStatisMap = util.NewSafeMap()

	items := make([]*DeviceStatisItem, deviceStatisMap.Size())
	i := 0
	for _, item := range deviceStatisMap.Items() {
		items[i] = item.(*DeviceStatisItem)
		i++
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].count > items[j].count
	})

	limit := m.connectionManager.GetConnectionLimit()
	if len(items) > limit {
		items = items[:limit]
	}

	topSendDevices := util.NewSafeMap()
	for _, item := range items {
		topSendDevices.Set(item.deviceId, item.deviceId)
	}

	m.topSendDevices = topSendDevices
}

// another implement use Heap
// type DeviceStatisItemHeap []*DeviceStatisItem

// func (h DeviceStatisItemHeap) Len() int {
// 	return len(h)
// }

// func (h DeviceStatisItemHeap) Less(i, j int) bool {
// 	return h[i].count < h[j].count
// }

// func (h DeviceStatisItemHeap) Swap(i, j int) {
// 	h[i], h[j] = h[j], h[i]
// }

// func (h *DeviceStatisItemHeap) Push(x interface{}) {
// 	*h = append(*h, x.(*DeviceStatisItem))
// }

// func (h *DeviceStatisItemHeap) Pop() interface{} {
// 	old := *h
// 	n := len(old)
// 	x := old[n-1]
// 	*h = old[0 : n-1]
// 	return x
// }
