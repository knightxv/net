package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/guabee/bnrtc/bnrtcv1"
	"github.com/guabee/bnrtc/bnrtcv2/address"
	"github.com/guabee/bnrtc/bnrtcv2/backpressure"
	"github.com/guabee/bnrtc/bnrtcv2/client"
	conf "github.com/guabee/bnrtc/bnrtcv2/config"
	"github.com/guabee/bnrtc/bnrtcv2/device"
	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/dht"
	"github.com/guabee/bnrtc/bnrtcv2/group"
	"github.com/guabee/bnrtc/bnrtcv2/net"
	"github.com/guabee/bnrtc/bnrtcv2/server"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
	"github.com/guabee/bnrtc/util"

	// services
	_ "github.com/guabee/bnrtc/bnrtcv2/server/services/dchat"
	_ "github.com/guabee/bnrtc/bnrtcv2/server/services/dweb"
)

func getBnrtc1ServiceOptions(id deviceid.DeviceId, config *conf.Config) *bnrtcv1.Bnrtc1ServerOptions {
	return &bnrtcv1.Bnrtc1ServerOptions{
		Name:              config.Name,
		PublicIp:          config.ExternalIp,
		LocalNatName:      id.NodeId(),
		ConnectionLimit:   config.SignalConnectionLimit,
		HttpPort:          int32(config.ServicePort),
		StunPort:          int32(config.Bnrtc1StunPort),
		TurnPort:          int32(config.Bnrtc1TurnPort),
		DisableTurnServer: config.DisableTurnServer,
		DisableStunServer: config.DisableStunServer,
		DefaultTurnUrls:   config.DefaultTurnUrls,
		DefaultStunUrls:   config.DefaultStunUrls,
	}
}

func newTransportManager(bnrtc1Manager *transport.Bnrtc1Manager, devid deviceid.DeviceId, address string, port int, needAck bool, ackTimeout time.Duration, pubsub *util.PubSub, externalIp string) transport.ITransportManager {
	udpTransport, err := transport.NewUdpTransport(transport.UdpTransportOption{
		Address:    address,
		Port:       port,
		NeedAck:    needAck,
		AckTimeout: ackTimeout,
	})
	if err != nil {
		fmt.Println("initial udpTransport failed", err)
		return nil
	}

	signalTransport, err := transport.NewSignalTransport(bnrtc1Manager.GetSignalManager())
	if err != nil {
		fmt.Println("initial bnrtc1Transport failed", err)
		return nil
	}
	bnrtc1Transport, err := transport.NewBnrtc1Transport(bnrtc1Manager.GetConnectionManager())
	if err != nil {
		fmt.Println("initial bnrtc1Transport failed", err)
		return nil
	}
	transportManager := transport.NewTransportManager(transport.WithDevidOption(devid), transport.WithPubSubOption(pubsub), transport.WithExternalIp(externalIp))
	transportManager.AddTransport(signalTransport)
	transportManager.AddTransport(bnrtc1Transport)
	transportManager.AddTransport(udpTransport)
	return transportManager
}

func getPath(dirPath string, fileName string) string {
	if dirPath == "" {
		return fileName
	} else {
		return fmt.Sprintf("%s/%s", dirPath, fileName)
	}
}

func StartServer(config conf.Config) (*server.Bnrtc2HttpServer, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	pubsub := util.NewPubSub()
	rpcCenter := util.NewRPCCenter()
	idFileName := config.Name + ".id"
	deviceIdManagerOptions := []deviceid.Option{deviceid.WithPubSub(pubsub), deviceid.WithExternalIp(config.ExternalIp), deviceid.WithRPCCenter(rpcCenter), deviceid.WithFilePath(getPath(config.DirPath, idFileName))}
	if config.DevId != "" {
		deviceIdManagerOptions = append(deviceIdManagerOptions, deviceid.WithDeviceId(deviceid.FromString(config.DevId)))
	}

	if config.ForceTopNetwork {
		deviceIdManagerOptions = append(deviceIdManagerOptions, deviceid.WithForceTopNetwork())
	}
	deviceIdManager := deviceid.NewDeviceIdManager(deviceIdManagerOptions...)
	// get current devid
	configDevId := deviceIdManager.GetDeviceId(true)
	device_info.NewDeviceInfoManage(
		config.Name,
		device_info.WithDeviceId(configDevId),
		device_info.WithPort(int(config.ServicePort)),
		device_info.WithDisableIpv6(config.DisableIpv6),
		device_info.WithDisableLan(config.DisableLan),
		device_info.WithPubSub(pubsub),
		device_info.WithRPCCenter(rpcCenter),
		device_info.WithExternalIp(config.ExternalIp),
	)
	bnrtc1Manager := transport.NewBnrtc1Manager(
		configDevId,
		getBnrtc1ServiceOptions(configDevId, &config),
		transport.WithPubSub(pubsub),
		transport.WithRPCCenter(rpcCenter),
		transport.WithConnectionLimit(config.ConnectionLimit),
		transport.WithConnectionExpired(config.ConnectionExpired),
	)
	transportManager := newTransportManager(bnrtc1Manager, configDevId, config.Host, int(config.ServicePort), config.EnableUdpAck, config.UdpAckTimeout, pubsub, config.ExternalIp)
	device := device.NewDevice(
		transportManager,
		bnrtc1Manager,
		dht.NewDHT,
		device.WithPubSub(pubsub),
		device.WithRPCCenter(rpcCenter),
		device.WithExternalIp(config.ExternalIp),
		device.WithDHTMemoryStoreOptions(config.DHTMemoryStoreOptions),
	)

	groupManager := group.NewManager(device, group.WithPubSub(pubsub))
	addressManager := address.NewManager(device, groupManager, address.WithPubSub(pubsub))
	bpManager := backpressure.NewManager(
		backpressure.WithDeviceId(configDevId.String()),
		backpressure.WithBpType(backpressure.BPCType(config.BpType)),
		backpressure.WithLimit(config.BackPressureLimit),
	)
	netController := net.NewNetController(ctx, &net.NetControllerConfig{
		Device:            device,
		AddressManager:    addressManager,
		GroupManager:      groupManager,
		BpManager:         bpManager,
		ConnectionManager: bnrtc1Manager,
	}, net.WithDevid(configDevId), net.WithPubSub(pubsub), net.WithRPCCenter(rpcCenter))

	clientManager := client.NewClientManger(netController)

	userConfigFileName := config.Name + "-user.json"
	server, err := server.NewBnrtc2Server(
		&server.Bnrtc2ServerConfig{
			HttpPort: config.Port,
		},
		device,
		netController,
		clientManager,
		server.WithServiceManagerOptions(&config.ServiceManager),
		server.WithRPCCenter(rpcCenter),
		server.WithPubSub(pubsub),
		server.WithServices(config.Services),
		server.WithUserconfig(getPath(config.DirPath, userConfigFileName), conf.GetBool(util.BNRTC_HAS_EXCEPTION_KEY)),
	)

	if err != nil {
		fmt.Println("NewBnrtc2Server failed", err)
		panic(err)
	}

	server.BindAddresses(config.Addresses)
	if len(config.Peers) == 0 {
		config.Peers = []string{fmt.Sprintf("127.0.0.1:%d", config.ServicePort)}
	}

	server.AddPeers(config.Peers)
	return server, cancel
}
