package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/guabee/bnrtc/bnrtcv2/server/iservice"
)

type BoolResult struct {
	Result bool `json:"result"`
}

func marshalBoolResult(result bool) ([]byte, error) {
	return json.Marshal(BoolResult{
		Result: result,
	})
}

func (server *Bnrtc2HttpServer) upgradeToConn(upgrader *websocket.Upgrader, wr http.ResponseWriter, r *http.Request) error {
	conn, err := upgrader.Upgrade(wr, r, nil)
	if err != nil {
		return err
	}
	wsChannel := server.NewWsChannel(conn)
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			server.logger.Debugln("read:", err)
			break
		}
		wsChannel.Send(message)
	}
	wsChannel.Close()
	return nil
}

func (server *Bnrtc2HttpServer) HandleConnectRequest(wr http.ResponseWriter, r *http.Request) (res []byte, err error) {
	err = server.upgradeToConn(server.websocketUpgrader, wr, r)
	return
}

func (server *Bnrtc2HttpServer) HandleIsOnlineRequest(wr http.ResponseWriter, r *http.Request) (res []byte, err error) {
	query := r.URL.Query()
	address := query.Get("address")
	isOnline := server.IsOnline(address)
	return marshalBoolResult(isOnline)
}

func (server *Bnrtc2HttpServer) HandleAddPeersRequest(wr http.ResponseWriter, r *http.Request) (res []byte, err error) {
	query := r.URL.Query()
	hosts := query.Get("hosts")
	hostList := strings.Split(hosts, ",")
	server.AddPeers(hostList)
	return marshalBoolResult(true)
}

func (server *Bnrtc2HttpServer) HandleDeletePeersRequest(wr http.ResponseWriter, r *http.Request) (res []byte, err error) {
	query := r.URL.Query()
	hosts := query.Get("hosts")
	hostList := strings.Split(hosts, ",")
	server.DelPeers(hostList)
	return marshalBoolResult(true)

}

func (server *Bnrtc2HttpServer) HandleBindAddressRequest(wr http.ResponseWriter, r *http.Request) (res []byte, err error) {
	query := r.URL.Query()
	address := query.Get("address")
	server.logger.Debugln("address", address)
	server.BindAddresses([]string{address})
	return marshalBoolResult(true)
}

func (server *Bnrtc2HttpServer) HandleUnbindAddressRequest(wr http.ResponseWriter, r *http.Request) (res []byte, err error) {
	query := r.URL.Query()
	address := query.Get("address")
	server.logger.Debugln("address", address)
	server.UnbindAddresses([]string{address})
	return marshalBoolResult(true)
}

func (server *Bnrtc2HttpServer) HandleAddFavoritesRequest(wr http.ResponseWriter, r *http.Request) (res []byte, err error) {
	query := r.URL.Query()
	addresses := query.Get("addresses")
	addressList := strings.Split(addresses, ",")
	server.AddFavorites(addressList)
	return marshalBoolResult(true)
}

func (server *Bnrtc2HttpServer) HandleDelFavoritesRequest(wr http.ResponseWriter, r *http.Request) (res []byte, err error) {
	query := r.URL.Query()
	addresses := query.Get("addresses")
	addressList := strings.Split(addresses, ",")
	server.DelFavorites(addressList)
	return marshalBoolResult(true)
}

func (server *Bnrtc2HttpServer) HandleGetServiceAddressesRequest(wr http.ResponseWriter, r *http.Request) (res []byte, err error) {
	query := r.URL.Query()
	serviceName := query.Get("service")
	addresses := server.GetServiceAddresses(serviceName)
	return json.Marshal(addresses)
}

func (server *Bnrtc2HttpServer) HandleConnectAddressRequest(wr http.ResponseWriter, r *http.Request) (res []byte, err error) {
	query := r.URL.Query()
	address := query.Get("address")
	err = server.ConnectAddress(address)
	if err == nil {
		return marshalBoolResult(true)
	}
	return
}

func (server *Bnrtc2HttpServer) HandleRegistServiceRequest(wr http.ResponseWriter, r *http.Request) (res []byte, err error) {
	query := r.URL.Query()
	name := query.Get("name")
	addresses := query.Get("addresses")
	addressList := strings.Split(addresses, ",")
	serviceDport := query.Get("service_dport")
	clientDport := query.Get("client_dport")
	signature := query.Get("signature")
	serviceInfo := &iservice.ServiceInfo{
		Name:         name,
		Addresses:    addressList,
		ServiceDport: serviceDport,
		ClientDport:  clientDport,
		Signature:    signature,
		AddressesStr: addresses,
	}

	e := server.RegistService(serviceInfo)
	return marshalBoolResult(e == nil)
}

func (server *Bnrtc2HttpServer) HandleUnregistServiceRequest(wr http.ResponseWriter, r *http.Request) (res []byte, err error) {
	query := r.URL.Query()
	name := query.Get("name")
	signature := query.Get("signature")
	serviceInfo := &iservice.ServiceInfo{
		Name:      name,
		Signature: signature,
	}
	e := server.UnregistService(serviceInfo)
	return marshalBoolResult(e == nil)
}

func (server *Bnrtc2HttpServer) HandleGetServiceInfoRequest(wr http.ResponseWriter, r *http.Request) (res []byte, err error) {
	query := r.URL.Query()
	serviceName := query.Get("service")
	info, err := server.GetServiceInfo(serviceName)
	if err != nil {
		return nil, err
	}

	return json.Marshal(info)
}

func (server *Bnrtc2HttpServer) HandleGetAddressInfoRequest(wr http.ResponseWriter, r *http.Request) (res []byte, err error) {
	query := r.URL.Query()
	address := query.Get("address")
	info, err := server.GetAddressInfo(address)
	if err != nil {
		return nil, err
	}

	return json.Marshal(info)
}

func (server *Bnrtc2HttpServer) HandleConfigInfo(wr http.ResponseWriter, r *http.Request) (res []byte, err error) {
	confInfo := server.Config
	return json.Marshal(confInfo)
}

func (server *Bnrtc2HttpServer) HandleDebugAddresses(wr http.ResponseWriter, r *http.Request) (res []byte, err error) {
	addresses := server.GetAddresses()
	return json.Marshal(addresses)
}

func (server *Bnrtc2HttpServer) HandleDebugPeers(wr http.ResponseWriter, r *http.Request) (res []byte, err error) {
	peers := server.GetPeers()
	return json.Marshal(peers)
}

func (server *Bnrtc2HttpServer) HandleDebugFavorites(wr http.ResponseWriter, r *http.Request) (res []byte, err error) {
	favorites := server.GetFavorites()
	return json.Marshal(favorites)
}
