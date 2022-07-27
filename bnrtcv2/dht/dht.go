package dht

import (
	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/guabee/bnrtc/bnrtcv2/dht/idht"
	kaddht "github.com/guabee/bnrtc/bnrtcv2/dht/kad-dht"
)

func NewDHT(option *idht.DHTOption) idht.IDHT {
	var nodes []*kaddht.Node
	for _, peer := range option.Peers {
		info := device_info.BuildDeviceInfo(peer)
		if info != nil {
			nodes = append(nodes, kaddht.NewNode(info))
		}
	}

	dht, err := kaddht.NewDHT(
		&kaddht.Options{
			ID:                    option.DeviceInfo.Id,
			DeviceInfo:            option.DeviceInfo,
			BootstrapNodes:        nodes,
			AutoRun:               true,
			PubSub:                option.PubSub,
			DHTMemoryStoreOptions: option.DHTMemoryStoreOptions,
		},
		option.TransportManager,
	)
	if err != nil {
		panic(err)
	}

	return dht
}
