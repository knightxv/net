package iservice

import (
	"net/http"

	conf "github.com/guabee/bnrtc/bnrtcv2/config"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/dht/idht"
	"github.com/guabee/bnrtc/bnrtcv2/message"
	"github.com/guabee/bnrtc/util"
)

const ServiceKey = "services"

type ServiceInfo struct {
	Name         string   `json:"name"`
	Addresses    []string `json:"addresses"`
	ServiceDport string   `json:"service_dport"`
	ClientDport  string   `json:"client_dport"`
	Signature    string   `json:"signature"`
	Version      string   `json:"version"`
	ABI          string   `json:"abi"`
	AddressesStr string   `json:"-"`
}

type Bnrtc2ServiceHandler func(http.ResponseWriter, *http.Request) ([]byte, error)

type ILocalChannel interface {
	Send(src string, dst string, dport string, devid string, data []byte) error
	Multicast(src string, dst string, dport string, devid string, data []byte) error
	OnMessage(dport string, handler func(msg *message.Message)) error
	OffMessage(dport string, handler func(msg *message.Message)) error
	OnClosed(name string, handler func()) error
	Close()
}

type IDevice interface {
	IsConnected() bool
	GetData(key deviceid.DataKey, noCache bool) ([]byte, bool, error)
	PutData(key deviceid.DataKey, data []byte, options *idht.DHTStoreOptions) error
	UpdateData(key deviceid.DataKey, oldData, newData []byte, options *idht.DHTStoreOptions) error
	DeleteData(key deviceid.DataKey) error
}

type IServer interface {
	NewLocalChannel(name string) ILocalChannel
	AddServicesHandler(name string, path string, handler Bnrtc2ServiceHandler) error
	GetDevice() IDevice
	GetRPCCenter() *util.RPCCenter
	GetPubSub() *util.PubSub
	GetServiceManager() IServiceManager
}

type IService interface {
	Name() string
	Start() error
	Stop() error
}

type IServiceFactory interface {
	GetInstance(server IServer, options conf.OptionMap) (IService, error)
	Name() string
}

type IServiceManager interface {
	Start()
	RegistService(info *ServiceInfo) error
	UnregistService(info *ServiceInfo) error
	GetServiceInfo(serviceName string) (ServiceInfo, error)
	BindAddress(address string)
	GetAddress() string
}

func GetServiceKey(address string) deviceid.DataKey {
	return deviceid.KeyGen(util.Str2bytes("_s_" + address))
}
