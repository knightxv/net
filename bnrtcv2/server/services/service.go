package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	conf "github.com/guabee/bnrtc/bnrtcv2/config"
	"github.com/guabee/bnrtc/bnrtcv2/dht/idht"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/bnrtcv2/server/iservice"

	"github.com/guabee/bnrtc/util"
)

const (
	ServiceManagerName = "service-manager"
)

var (
	ServiceInfoPublishIntervalDefault = 60 * time.Second
)

var serviceFactoryMap = make(map[string]iservice.IServiceFactory)

func RegisterServiceFactory(serviceFactory iservice.IServiceFactory) {
	name := serviceFactory.Name()
	if _, ok := serviceFactoryMap[name]; !ok {
		serviceFactoryMap[name] = serviceFactory
	}
}

func GetServiceFactory(name string) (iservice.IServiceFactory, error) {
	serviceFactory, ok := serviceFactoryMap[name]
	if !ok {
		return nil, fmt.Errorf("service %s not found", name)
	}

	return serviceFactory, nil
}

type ServiceManager struct {
	device          iservice.IDevice
	pubsub          *util.PubSub
	rpcCenter       *util.RPCCenter
	server          iservice.IServer
	address         string
	services        map[string]iservice.IService
	serviceInfos    map[string]iservice.ServiceInfo
	servicesMap     map[string]conf.OptionMap
	IsStart         bool
	publishInterval time.Duration
	stopChan        chan struct{}
	forceChan       chan string
	logger          *log.Entry
}

func NewServiceManager(server iservice.IServer, options *conf.OptionMap, servicesMap map[string]conf.OptionMap) *ServiceManager {
	m := &ServiceManager{
		device:          server.GetDevice(),
		services:        make(map[string]iservice.IService),
		serviceInfos:    make(map[string]iservice.ServiceInfo),
		servicesMap:     make(map[string]conf.OptionMap),
		IsStart:         false,
		stopChan:        make(chan struct{}, 1),
		forceChan:       make(chan string, 1),
		pubsub:          server.GetPubSub(),
		rpcCenter:       server.GetRPCCenter(),
		server:          server,
		publishInterval: ServiceInfoPublishIntervalDefault,
		logger:          log.NewLoggerEntry("service-manager"),
	}

	if servicesMap != nil {
		m.servicesMap = servicesMap
	}

	// services.dchat.publish_interval
	publishInterval := options.GetDuration("publish_interval")
	if publishInterval > 0 {
		m.publishInterval = publishInterval
	}

	return m
}

func (m *ServiceManager) initServices(servicesMap map[string]conf.OptionMap) {
	for name, options := range servicesMap {
		enable := options.GetBool("enabled", false)
		if !enable {
			continue
		}

		factory, err := GetServiceFactory(name)
		if err != nil {
			fmt.Printf("get service factory %s err: %s\n", name, err)
			continue
		}
		service, err := factory.GetInstance(m.server, options)
		if err != nil {
			fmt.Printf("create service %s err: %s\n", name, err)
			continue
		}
		_ = m.AddService(service)
	}
}

func (m *ServiceManager) publishServiceInfoWorker() {
	t := time.NewTicker(m.publishInterval)
	publishInfoServiceTask := util.NewSingleTonTask(util.BindingFunc(m.doPublishServiceInfo))
OUT:
	for {
		select {
		case <-t.C:
			publishInfoServiceTask.Run()
		case name := <-m.forceChan:
			if name != "" {
				info, ok := m.serviceInfos[name]
				if ok {
					m.publishServiceInfo(info)
				}
			} else {
				publishInfoServiceTask.Run()
			}
		case <-m.stopChan:
			t.Stop()
			close(m.forceChan)
			break OUT
		}
	}
}

func (m *ServiceManager) GetServiceInfo(serviceName string) (iservice.ServiceInfo, error) {
	localData, ok := m.serviceInfos[serviceName]
	if ok {
		return localData, nil
	}

	data, ok, err := m.server.GetDevice().GetData(iservice.GetServiceKey(serviceName), true)
	if err != nil {
		return iservice.ServiceInfo{}, err
	}

	if !ok {
		return iservice.ServiceInfo{}, fmt.Errorf("service info not found")
	}
	serviceInfo := iservice.ServiceInfo{}
	err = json.Unmarshal(data, &serviceInfo)
	if err != nil {
		return iservice.ServiceInfo{}, err
	}
	serviceInfo.AddressesStr = strings.Join(serviceInfo.Addresses, ",")
	return serviceInfo, nil
}

func (m *ServiceManager) publishServiceInfo(info iservice.ServiceInfo) error {
	if len(info.Addresses) == 0 {
		if m.address != "" {
			info.Addresses = []string{m.address} // use current addresses
		} else {
			// no address, no need to publish
			return fmt.Errorf("service %s info no addresses", info.Name)
		}
	}

	infoData, err := json.Marshal(info)
	if err != nil {
		return err
	}
	dataExpiredInterval := m.publishInterval * 5
	return m.device.PutData(iservice.GetServiceKey(info.Name), infoData, &idht.DHTStoreOptions{
		TExpire:     dataExpiredInterval,
		NoReplicate: true,
	})

}

func (m *ServiceManager) doPublishServiceInfo() {
	for _, info := range m.serviceInfos {
		err := m.publishServiceInfo(info)
		if err != nil {
			m.logger.Errorf("publish service %s failed: %s", info.Name, err)
		}
	}
}

func (m *ServiceManager) RegistService(info *iservice.ServiceInfo) error {
	// override service info
	// todo: check if service signature is valid
	m.serviceInfos[info.Name] = *info
	m.forceChan <- info.Name
	return nil

}
func (m *ServiceManager) UnregistService(info *iservice.ServiceInfo) error {
	// todo: check if service signature is valid
	delete(m.serviceInfos, info.Name)
	// todo: remove service info from dht
	return nil
}

func (m *ServiceManager) startService(s iservice.IService) {
	err := s.Start()
	if err != nil {
		m.logger.Errorf("start service %s failed: %s", s.Name(), err)
	}
}

func (m *ServiceManager) stopService(s iservice.IService) {
	err := s.Stop()
	if err != nil {
		m.logger.Errorf("stop service %s failed: %s", s.Name(), err)
	}
}

func (m *ServiceManager) Start() {
	if m.IsStart {
		return
	}

	m.initServices(m.servicesMap)
	_, err := m.pubsub.SubscribeFunc(ServiceManagerName, rpc.DHTStatusChangeTopic, func(msg util.PubSubMsgType) {
		status := msg.(idht.DHTStatus)
		if status == idht.DHTStatusReady || status == idht.DHTStatusConnected {
			m.forceChan <- ""
		}
	})
	if err != nil {
		m.logger.Errorf("subscribe dht status failed: %s", err)
	}

	for _, s := range m.services {
		m.logger.Infof("start service %s", s.Name())
		go m.startService(s)
	}

	go m.publishServiceInfoWorker()
	m.IsStart = true
}

func (m *ServiceManager) Stop() {
	if !m.IsStart {
		return
	}
	m.IsStart = false
	for _, s := range m.services {
		m.logger.Infof("stop service %s", s.Name())
		go m.stopService(s)
	}

	m.stopChan <- struct{}{}
}

func (m *ServiceManager) AddService(s iservice.IService) error {
	name := s.Name()
	if name == "" {
		return fmt.Errorf("service name is empty")
	}

	if _, ok := m.services[name]; ok {
		return fmt.Errorf("service %s already exists", name)
	}
	m.services[name] = s
	if m.IsStart {
		m.logger.Infof("start service %s", s.Name())
		go m.startService(s)
	}
	return nil
}

func (m *ServiceManager) DeleteService(s iservice.IService) error {
	name := s.Name()
	if name == "" {
		return fmt.Errorf("service name is empty")
	}

	if service, ok := m.services[name]; ok {
		m.logger.Infof("stop service %s", s.Name())
		service.Stop()
		delete(m.services, name)
	}
	return nil
}

func (m *ServiceManager) BindAddress(address string) {
	if m.address == address {
		return
	}

	m.address = address
	if address != "" {
		m.forceChan <- ""
	}
}

func (m *ServiceManager) GetAddress() string {
	return m.address
}
