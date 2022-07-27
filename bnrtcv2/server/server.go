package server

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	conf "github.com/guabee/bnrtc/bnrtcv2/config"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/server/iservice"
	"github.com/guabee/bnrtc/bnrtcv2/server/services"
	"github.com/guabee/bnrtc/util"
)

type ITransportAddress interface {
	HasIp() bool
	IsIPv4() bool
	IsPublic() bool
	IsLocal() bool
	String() string
}

type INetController interface {
	Connect(address string) error
	AddPeers(hosts []string)
	DelPeers(hosts []string)
	GetPeers() []string
	GetAddressInfo(address string) (AddressInfo, error)
	IsOnline(address string) bool
	BindAddress(address string)
	UnbindAddress(address string)
	GetAddresses() []string
	GetDefaultAddress() string
	AddFavorite(address string)
	DelFavorite(address string)
	GetFavorites() []string
}

type IWSChannel interface {
	Send(wsMessage []byte)
	Close()
}

type IClientManager interface {
	CreateWsChannel(conn *websocket.Conn) IWSChannel
	CreateLocalChannel(name string) iservice.ILocalChannel
}

const (
	UserConfigFileNameDefault = "user.json"
)

type Endpoint struct {
	Name string `json:"name"`
	Id   string `json:"id"`
}

type AddressInfo struct {
	Address   string      `json:"address"`
	Endpoints []*Endpoint `json:"endpoints"`
}

type Bnrtc2ServerConfig struct {
	HttpPort int `json:"httpPort"`
}

type Bnrtc2HttpServerOptions func(s *Bnrtc2HttpServer)

func WithServices(servicesMap map[string]conf.OptionMap) Bnrtc2HttpServerOptions {
	return func(m *Bnrtc2HttpServer) {
		m.servicesMap = servicesMap
	}
}

func WithUserconfig(filePath string, load bool) Bnrtc2HttpServerOptions {
	return func(server *Bnrtc2HttpServer) {
		server.userConfigReaderWriter = conf.UserConfigReaderWriter{FileName: filePath}
		server.loadUserConfig = load
	}
}

func WithRPCCenter(c *util.RPCCenter) Bnrtc2HttpServerOptions {
	return func(m *Bnrtc2HttpServer) {
		m.RPCCenter = c
	}
}

func WithPubSub(pubsub *util.PubSub) Bnrtc2HttpServerOptions {
	return func(m *Bnrtc2HttpServer) {
		m.Pubsub = pubsub
	}
}

func WithServiceManagerOptions(options *conf.OptionMap) Bnrtc2HttpServerOptions {
	return func(m *Bnrtc2HttpServer) {
		m.serviceManagerOptions = options
	}
}

type Bnrtc2HttpServer struct {
	Config                 *Bnrtc2ServerConfig
	RPCCenter              *util.RPCCenter
	Pubsub                 *util.PubSub
	websocketUpgrader      *websocket.Upgrader
	clientManager          IClientManager
	serviceManager         iservice.IServiceManager
	netController          INetController
	serviceManagerOptions  *conf.OptionMap
	servicesMap            map[string]conf.OptionMap
	device                 iservice.IDevice
	handlers               map[string]iservice.Bnrtc2ServiceHandler
	userConfigReaderWriter conf.UserConfigReaderWriter
	loadUserConfig         bool
	logger                 *log.Entry
}

func NewBnrtc2Server(config *Bnrtc2ServerConfig, device iservice.IDevice, netController INetController, clientManager IClientManager, opts ...Bnrtc2HttpServerOptions) (*Bnrtc2HttpServer, error) {
	server := &Bnrtc2HttpServer{
		Config:    config,
		RPCCenter: util.DefaultRPCCenter(),
		Pubsub:    util.DefaultPubSub(),
		websocketUpgrader: &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { // allow any origin
				return true
			},
		},
		device:                 device,
		handlers:               make(map[string]iservice.Bnrtc2ServiceHandler),
		clientManager:          clientManager,
		netController:          netController,
		userConfigReaderWriter: conf.UserConfigReaderWriter{FileName: UserConfigFileNameDefault},
		logger:                 log.NewLoggerEntry("server"),
	}
	for _, opt := range opts {
		opt(server)
	}
	server.serviceManager = services.NewServiceManager(server, server.serviceManagerOptions, server.servicesMap)

	if server.loadUserConfig {
		server.LoadUserConfig()
	}

	go server.startLocalHttpService()

	server.serviceManager.Start()
	return server, nil
}

func (server *Bnrtc2HttpServer) GetDevice() iservice.IDevice {
	return server.device
}

func (server *Bnrtc2HttpServer) GetServiceManager() iservice.IServiceManager {
	return server.serviceManager
}

func (server *Bnrtc2HttpServer) GetRPCCenter() *util.RPCCenter {
	return server.RPCCenter
}

func (server *Bnrtc2HttpServer) GetPubSub() *util.PubSub {
	return server.Pubsub
}

func (server *Bnrtc2HttpServer) startLocalHttpService() {
	addr := fmt.Sprintf("127.0.0.1:%d", server.Config.HttpPort)
	server.LoadLocalHanders()
	server.logger.Infof("bnrtcv2 local http server started %s", addr)
	err := http.ListenAndServe(addr, server)
	if err != nil {
		server.logger.Panicf("bnrtcv2 local http server start %s failed %s", addr, err)
		os.Exit(1)
	}
}

func (server *Bnrtc2HttpServer) SaveUserConfig() {
	config := conf.UserConfig{
		Peers:     server.GetPeers(),
		Addresses: server.GetAddresses(),
		Favorites: server.GetFavorites(),
	}
	server.userConfigReaderWriter.Write(config)
}

func (server *Bnrtc2HttpServer) LoadUserConfig() {
	config := server.userConfigReaderWriter.Read()
	server.bindAddresses(config.Addresses, false)
	server.addFavorites(config.Favorites, false)
	server.addPeers(config.Peers, false)
}

func (server *Bnrtc2HttpServer) bindAddresses(addresses []string, save bool) {
	for _, address := range addresses {
		server.netController.BindAddress(address)
	}

	server.serviceManager.BindAddress(server.GetDefaultAddress())
	if save {
		server.SaveUserConfig()
	}
}

func (server *Bnrtc2HttpServer) BindAddresses(addresses []string) {
	server.bindAddresses(addresses, true)
}

func (server *Bnrtc2HttpServer) UnbindAddresses(addresses []string) {
	for _, address := range addresses {
		server.netController.UnbindAddress(address)
	}
	server.serviceManager.BindAddress(server.GetDefaultAddress())
	server.SaveUserConfig()
}

func (server *Bnrtc2HttpServer) GetAddresses() []string {
	return server.netController.GetAddresses()
}

func (server *Bnrtc2HttpServer) GetDefaultAddress() string {
	return server.netController.GetDefaultAddress()
}

func (server *Bnrtc2HttpServer) addFavorites(addresses []string, save bool) {
	for _, address := range addresses {
		server.netController.AddFavorite(address)
	}
	if save {
		server.SaveUserConfig()
	}
}

func (server *Bnrtc2HttpServer) AddFavorites(addresses []string) {
	server.addFavorites(addresses, true)
}

func (server *Bnrtc2HttpServer) DelFavorites(addresses []string) {
	for _, address := range addresses {
		server.netController.DelFavorite(address)
	}
	server.SaveUserConfig()
}

func (server *Bnrtc2HttpServer) GetFavorites() []string {
	return server.netController.GetFavorites()
}

func (server *Bnrtc2HttpServer) addPeers(peers []string, save bool) {
	server.netController.AddPeers(peers)
	if save {
		server.SaveUserConfig()
	}
}

func (server *Bnrtc2HttpServer) AddPeers(peers []string) {
	server.addPeers(peers, true)
}

func (server *Bnrtc2HttpServer) DelPeers(peers []string) {
	server.netController.DelPeers(peers)
	server.SaveUserConfig()
}

func (server *Bnrtc2HttpServer) GetPeers() []string {
	return server.netController.GetPeers()
}

func (server *Bnrtc2HttpServer) GetAddressInfo(address string) (info AddressInfo, err error) {
	return server.netController.GetAddressInfo(address)
}

func (server *Bnrtc2HttpServer) GetServiceInfo(serviceName string) (iservice.ServiceInfo, error) {
	if server.serviceManager == nil {
		return iservice.ServiceInfo{}, fmt.Errorf("service manager not init")
	}
	return server.serviceManager.GetServiceInfo(serviceName)
}

func (server *Bnrtc2HttpServer) GetServiceAddresses(serviceName string) []string {
	serviceInfo, err := server.GetServiceInfo(serviceName)
	if err != nil {
		return nil
	}

	return serviceInfo.Addresses
}

func (server *Bnrtc2HttpServer) ConnectAddress(address string) error {
	return server.netController.Connect(address)
}

func (server *Bnrtc2HttpServer) RegistService(serviveInfo *iservice.ServiceInfo) error {
	return server.serviceManager.RegistService(serviveInfo)
}

func (server *Bnrtc2HttpServer) UnregistService(serviveInfo *iservice.ServiceInfo) error {
	return server.serviceManager.UnregistService(serviveInfo)
}

func (server *Bnrtc2HttpServer) IsOnline(address string) bool {
	return server.netController.IsOnline(address)
}

func (server *Bnrtc2HttpServer) SetLogLevel(name string, level log.Level) {
	log.SetLogLevel(name, level)
}

func (server *Bnrtc2HttpServer) NewLocalChannel(name string) iservice.ILocalChannel {
	return server.clientManager.CreateLocalChannel(name)
}

func (server *Bnrtc2HttpServer) NewWsChannel(conn *websocket.Conn) IWSChannel {
	return server.clientManager.CreateWsChannel(conn)
}

func (server *Bnrtc2HttpServer) addHandler(path string, h iservice.Bnrtc2ServiceHandler) {
	server.handlers[path] = h
}

func (server *Bnrtc2HttpServer) AddServicesHandler(name string, path string, h iservice.Bnrtc2ServiceHandler) error {
	servicePath := fmt.Sprintf("/bnrtc2/server/service/%s/%s", name, path)
	server.addHandler(servicePath, h)
	return nil
}

func (server *Bnrtc2HttpServer) LoadLocalHanders() {
	server.addHandler("/bnrtc2/server/isOnline", server.HandleIsOnlineRequest)
	server.addHandler("/bnrtc2/server/serverConfig", server.HandleConfigInfo)
	server.addHandler("/bnrtc2/server/connect", server.HandleConnectRequest)
	server.addHandler("/bnrtc2/server/addPeers", server.HandleAddPeersRequest)
	server.addHandler("/bnrtc2/server/deletePeers", server.HandleDeletePeersRequest)
	server.addHandler("/bnrtc2/server/bindAddress", server.HandleBindAddressRequest)
	server.addHandler("/bnrtc2/server/unbindAddress", server.HandleUnbindAddressRequest)
	server.addHandler("/bnrtc2/server/addFavorites", server.HandleAddFavoritesRequest)
	server.addHandler("/bnrtc2/server/delFavorites", server.HandleDelFavoritesRequest)
	server.addHandler("/bnrtc2/server/getServiceAddresses", server.HandleGetServiceAddressesRequest)
	server.addHandler("/bnrtc2/server/connectAddress", server.HandleConnectAddressRequest)
	server.addHandler("/bnrtc2/server/registService", server.HandleRegistServiceRequest)
	server.addHandler("/bnrtc2/server/unregistService", server.HandleUnregistServiceRequest)
	server.addHandler("/bnrtc2/server/getServiceInfo", server.HandleGetServiceInfoRequest)
	server.addHandler("/bnrtc2/server/getAddressInfo", server.HandleGetAddressInfoRequest)

	// debugs
	server.addHandler("/debug/addresses", server.HandleDebugAddresses)
	server.addHandler("/debug/peers", server.HandleDebugPeers)
	server.addHandler("/debug/favorites", server.HandleDebugFavorites)
}

func (handler *Bnrtc2HttpServer) ServeHTTP(wr http.ResponseWriter, r *http.Request) {
	handler.logger.Debugln("r.URL.Path:" + r.URL.String())
	path := r.URL.Path
	var res []byte = nil
	var err error = nil
	statusCode := 404
	wr.Header().Set("Access-Control-Allow-Origin", "*")
	h, ok := handler.handlers[path]
	if !ok {
		handler.logger.Debugf("handler for %s not found", path)
	} else {
		statusCode = 200
		res, err = h(wr, r)
		if err != nil {
			statusCode = 502
			res = []byte(err.Error())
		}
	}

	if statusCode != 200 {
		wr.WriteHeader(statusCode)
	}
	if res != nil {
		_, err := wr.Write(res)
		if err != nil {
			handler.logger.Errorln(err)
			wr.WriteHeader(500)
		}
	}
}
