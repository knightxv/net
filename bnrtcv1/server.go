package bnrtcv1

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/guabee/bnrtc/bnrtcv1/stund"
	"github.com/guabee/bnrtc/util"
	"github.com/pion/turn/v2"
	"github.com/pion/webrtc/v3"

	"github.com/sebest/xff"
)

type Bnrtc1ServiceHandler func(r *http.Request, rw http.ResponseWriter) ([]byte, error)
type Bnrtc1ServiceHandlerType = string

const (
	Bnrtc1ServiceHandlerGet  Bnrtc1ServiceHandlerType = "GET"
	Bnrtc1ServiceHandlerPost Bnrtc1ServiceHandlerType = "POST"
)

type Bnrtc1Server struct {
	Peer              *Bnrtc1Peer
	Client            *Bnrtc1Client
	Config            *Bnrtc1ServerConfig
	httpSev           *http.Server
	turnSev           *turn.Server
	stunSev           *stund.Server
	handlers          map[string]map[string]Bnrtc1ServiceHandler
	websocketUpgrader *websocket.Upgrader
	rns               *RegisteredNatDuplexs
	connManager       *ConnectionManager
}

type Bnrtc1ServerConfig struct {
	Name             string   `json:"name"`
	LocalNatName     string   `json:"localNatName"`
	ConnectionLimit  int      `json:"connectionLimit"`
	PublicIp         string   `json:"publicIp"`
	HttpPort         uint16   `json:"httpPort"`
	TurnPort         uint16   `json:"turnPort"`
	StunPort         uint16   `json:"stunPort"`
	EnableHttpServer bool     `json:"enableHttpServer"`
	EnableTurnServer bool     `json:"enableTurnServer"`
	EnableStunServer bool     `json:"enableStunServer"`
	DefaultStunUrls  []string `json:"defaultStunUrls"`
	DefaultTurnUrls  []string `json:"defaultTurnUrls"`
	HoldHooks        bool     `json:"holdHooks"`
}
type Bnrtc1ServerOptions struct {
	Name              string   `json:"name"`
	LocalNatName      string   `json:"localNatName"`
	ConnectionLimit   int      `json:"connectionLimit"`
	SplitRns          bool     `json:"splitRns"` // the server and client use standalone rns instance.
	HoldHooks         bool     `json:"holdHooks"`
	PublicIp          string   `json:"publicIp"`
	HttpPort          int32    `json:"httpPort"`
	TurnPort          int32    `json:"turnPort"`
	StunPort          int32    `json:"stunPort"`
	DefaultStunUrls   []string `json:"defaultStunUrls"`
	DefaultTurnUrls   []string `json:"defaultTurnUrls"`
	DisableHttpServer bool     `json:"disableHttpServer"`
	DisableTurnServer bool     `json:"disableTurnServer"`
	DisableStunServer bool     `json:"disableStunServer"`
}

type RequestLike struct {
	URL      *url.URL
	Method   string
	PostForm url.Values
}

func NewServer(options *Bnrtc1ServerOptions, client *Bnrtc1Client) (*Bnrtc1Server, error) {
	config := &Bnrtc1ServerConfig{
		PublicIp:         "127.0.0.1",
		HttpPort:         19103,
		TurnPort:         19403,
		StunPort:         19302,
		EnableHttpServer: true,
		EnableTurnServer: true,
		EnableStunServer: true,
		DefaultStunUrls:  make([]string, 0),
		DefaultTurnUrls:  make([]string, 0),
		ConnectionLimit:  1024,
	}
	var rns *RegisteredNatDuplexs
	var splitRns bool

	if options != nil {
		if options.PublicIp != "" {
			config.PublicIp = options.PublicIp
		}
		if options.Name != "" {
			config.Name = options.Name
			config.LocalNatName = config.Name // LocalNatName也使用Name，后面有配置则覆盖
		}
		if options.LocalNatName != "" {
			config.LocalNatName = options.LocalNatName
		}
		if options.ConnectionLimit > 0 {
			config.ConnectionLimit = options.ConnectionLimit
		}
		if options.HttpPort > 0 && options.HttpPort < 65536 {
			config.HttpPort = uint16(options.HttpPort)
		}
		if options.TurnPort > 0 && options.TurnPort < 65536 {
			config.TurnPort = uint16(options.TurnPort)
		}
		if options.StunPort > 0 && options.StunPort < 65536 {
			config.StunPort = uint16(options.StunPort)
		}
		if options.DisableHttpServer {
			config.EnableHttpServer = false
		}
		if options.DisableTurnServer {
			config.EnableTurnServer = false
		}
		if options.DisableStunServer {
			config.EnableStunServer = false
		}
		if options.HoldHooks {
			config.HoldHooks = options.HoldHooks
		}
		if options.DefaultStunUrls != nil {
			config.DefaultStunUrls = options.DefaultStunUrls
		}
		if options.DefaultTurnUrls != nil {
			config.DefaultTurnUrls = options.DefaultTurnUrls
		}
		splitRns = options.SplitRns
	}
	if client == nil {
		rns = newRegisteredNatDuplexs(config.LocalNatName, config.HoldHooks)
		var _client *Bnrtc1Client
		var err error

		clientConfig := &Bnrtc1ClientConfig{
			Name:            config.Name,
			ConnectionLimit: config.ConnectionLimit,
			LocalNatName:    config.LocalNatName,
		}

		if splitRns {
			_client, err = NewClient(nil, clientConfig)
		} else {
			_client, err = newClientWithCustomRns(clientConfig, nil, rns)
		}
		if err != nil {
			return nil, err
		}
		client = _client
	} else {
		if splitRns {
			rns = newRegisteredNatDuplexs(config.LocalNatName, config.HoldHooks)
		} else {
			rns = client.rns
		}
	}

	server := &Bnrtc1Server{
		Config:   config,
		Peer:     client.Peer,
		Client:   client,
		rns:      rns,
		handlers: make(map[string]map[string]Bnrtc1ServiceHandler),
		websocketUpgrader: &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { // allow any origin
				return true
			},
		},
		connManager: newConnectionManager(config.LocalNatName, config.ConnectionLimit),
	}
	server.handlers[Bnrtc1ServiceHandlerGet] = make(map[string]Bnrtc1ServiceHandler)
	server.handlers[Bnrtc1ServiceHandlerPost] = make(map[string]Bnrtc1ServiceHandler)
	server.LoadLocalHanders()
	return server, nil
}

func handlerOffer(client *Bnrtc1Client, rns *RegisteredNatDuplexs, pc_config *webrtc.Configuration, offerInfo *OfferInfo) ([]byte, error) {
	localNatName := client.Config.LocalNatName
	remoteNatName := offerInfo.NatName
	duplexKey := fmt.Sprintf("%s->%s", remoteNatName, localNatName)

	if rns.HasNatDuplex(remoteNatName) {
		return nil, fmt.Errorf("already exist duplex: %s", duplexKey)
	}
	natDuplex, err := client.natDuplexFactory.New(pc_config, "offer2answer", localNatName, remoteNatName, NatDuplexRoleAnswer, duplexKey)
	if err != nil {
		return nil, err
	}

	natDuplex.OnRemoteNameChanged(func(oldname, newName string) {
		err := rns.ChangeKey(oldname, natDuplex)
		if err != nil {
			log.Printf("change natDuplex %s key (%s->%s) error: %s\n", natDuplex.RemoteNatName, oldname, newName, err)
		}
	})

	peerConnection := natDuplex.pc
	peerConnection.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		if pcs == webrtc.PeerConnectionStateDisconnected || pcs == webrtc.PeerConnectionStateFailed || pcs == webrtc.PeerConnectionStateClosed {
			natDuplex.onClose()
			if err := rns.Delete(natDuplex); err == nil {
				go rns._emitOnNatDuplexDisConnected(natDuplex)
			}
		} else if pcs == webrtc.PeerConnectionStateConnected {
			if err := rns.Add(natDuplex); err == nil {
				go func() {
					if err := natDuplex.WaitReady(); err == nil {
						rns._emitOnNatDuplexConnected(natDuplex)
					} else {
						peerConnection.Close()
						log.Println(err)
					}
				}()
			} else {
				peerConnection.Close()
			}
		}
	})

	readyCond := util.NewCondition(util.WithConditionTimeout(30 * time.Second))
	var localDescBytes []byte = nil
	go func() {
		err = peerConnection.SetRemoteDescription(*offerInfo.Offer)
		if err != nil {
			readyCond.Signal(err)
			return
		}
		answer, err := peerConnection.CreateAnswer(nil)
		if err != nil {
			readyCond.Signal(err)
			return
		}
		err = peerConnection.SetLocalDescription(answer)
		if err != nil {
			readyCond.Signal(err)
			return
		}

		<-webrtc.GatheringCompletePromise(peerConnection)

		localDesc := peerConnection.LocalDescription()
		localDescBytes, err = json.Marshal(localDesc)
		readyCond.Signal(err)
	}()

	err = readyCond.Wait()
	if err != nil {
		peerConnection.Close()
		return nil, err
	} else {
		return localDescBytes, nil
	}
}

func (server *Bnrtc1Server) handlerIp(r *http.Request, rw http.ResponseWriter) ([]byte, error) {
	address := xff.GetRemoteAddr(r)
	ip, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	return []byte(ip), nil
}

func (server *Bnrtc1Server) handlerServices(r *http.Request, rw http.ResponseWriter) ([]byte, error) {
	query := r.URL.Query()
	queryNatName := query.Get("queryNatName")
	serviceInfo := server.GetServiceInfo(queryNatName)
	bytes, err := json.Marshal(serviceInfo)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func (server *Bnrtc1Server) handlerOffer2answer(r *http.Request, wr http.ResponseWriter) (res []byte, err error) {
	offerInfo, err := ParseOfferInfo(r.PostForm)
	if err != nil {
		return nil, err
	}

	if !IsSameVersion(offerInfo.Version, Version) {
		return nil, fmt.Errorf("nat version mismatch %s <--> %s", offerInfo.Version, Version)
	}

	serviceInfo := server.GetServiceInfo(server.Config.LocalNatName)
	pcConfig := serviceInfo.ToPeerConnectionConfig(server.Config.PublicIp)

	return handlerOffer(server.Client, server.rns, pcConfig, offerInfo)
}

func (server *Bnrtc1Server) handlerNat(r *http.Request, rw http.ResponseWriter) ([]byte, error) {
	natServerName := r.PostForm.Get("natServerName")
	if len(natServerName) == 0 {
		return nil, errors.New("no found nat-server-name")
	}

	method := r.PostForm.Get("method")
	if len(method) == 0 {
		return nil, errors.New("no found method")
	}

	conn := server.connManager.GetConnection(natServerName)
	if conn == nil {
		return nil, fmt.Errorf("not found %s", natServerName)
	}
	/// 构建出请求
	subUrl := *r.URL
	subUrl.Path = "/" + method
	natRequest := NewCtrlNatRequest(subUrl, r.Method, r.PostForm)
	natResponse, err := server.connManager.SendNatRequest(conn, natRequest, true)
	if err != nil {
		return nil, err
	}

	if natResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%d: %s", natResponse.StatusCode, natResponse.Content)
	}

	return natResponse.Content, nil
}

func (server *Bnrtc1Server) OnConnection(f func(conn *Connection)) {
	server.connManager.OnConnection(f)
}

func (server *Bnrtc1Server) upgradeToConn(upgrader *websocket.Upgrader, r *http.Request, wr http.ResponseWriter) error {
	if server.connManager.IsFull() {
		return errors.New("server is full")
	}

	query := r.URL.Query()
	natName := query.Get("natName")
	if len(natName) == 0 {
		return errors.New("no found natName in query")
	}

	version := query.Get("version")
	if !IsSameVersion(version, Version) {
		return fmt.Errorf("nat version mismatch %s <--> %s", version, Version)
	}
	conn, err := upgrader.Upgrade(wr, r, nil)
	if err != nil {
		return err
	}
	connection, err := server.connManager.NewConnection(natName, conn)
	if err != nil {
		conn.Close()
	} else {
		connection.Start()
	}
	return nil
}

func (server *Bnrtc1Server) HandleWebsocketConnect(r *http.Request, wr http.ResponseWriter) ([]byte, error) {
	err := server.upgradeToConn(server.websocketUpgrader, r, wr)
	return nil, err
}

func (server *Bnrtc1Server) addHandler(t Bnrtc1ServiceHandlerType, path string, h Bnrtc1ServiceHandler) {
	server.handlers[t][path] = h
}

func (server *Bnrtc1Server) LoadLocalHanders() {
	server.addHandler(Bnrtc1ServiceHandlerGet, "/ip", server.handlerIp)
	server.addHandler(Bnrtc1ServiceHandlerGet, "/services", server.handlerServices)
	server.addHandler(Bnrtc1ServiceHandlerGet, "/websocket", server.HandleWebsocketConnect)
	server.addHandler(Bnrtc1ServiceHandlerPost, "/offer2answer", server.handlerOffer2answer)
	server.addHandler(Bnrtc1ServiceHandlerPost, "/nat", server.handlerNat)
}

func (server *Bnrtc1Server) GetPublicIp() string {
	if server.Config.PublicIp == "" || server.Config.PublicIp == "127.0.0.1" {
		ipInfo, err := server.Client.GetLocalIpAddress(nil)
		if err != nil {
			return "127.0.0.1"
		}
		server.Config.PublicIp = ipInfo.Address
	}
	return server.Config.PublicIp
}

func (server *Bnrtc1Server) SetPublicIp(ip string) {
	server.Config.PublicIp = ip
}

func (server *Bnrtc1Server) IsOnline(natName string) bool {
	return server.rns.HasNatDuplex(natName)
}

func (server *Bnrtc1Server) SetLocalNatName(natName string) {
	if natName == server.Config.LocalNatName {
		return
	}
	server.Config.LocalNatName = natName
	server.connManager.SetLocalNatName(natName)
	if server.rns != server.Client.rns {
		// if not SplitRns, server.rns is client.rns
		server.rns.SetLocalNatName(natName)
	}

	server.Client.SetLocalNatName(natName)
}

func (server *Bnrtc1Server) GetServiceInfo(queryNatName string) *ServiceInfo {
	serviceInfo := &ServiceInfo{
		Name:         server.Client.Config.Name,
		LocalNatName: server.Client.Config.LocalNatName,
	}
	if server.Config.EnableStunServer {
		serviceInfo.StunPort = int32(server.Config.StunPort)
	}
	if server.Config.EnableTurnServer {
		serviceInfo.TurnPort = int32(server.Config.TurnPort)
	}
	if len(server.Config.DefaultStunUrls) > 0 {
		serviceInfo.DefaultStunUrls = make([]string, len(server.Config.DefaultStunUrls))
		copy(serviceInfo.DefaultStunUrls, server.Config.DefaultStunUrls)
	}
	if len(server.Config.DefaultTurnUrls) > 0 {
		serviceInfo.DefaultTurnUrls = make([]string, len(server.Config.DefaultTurnUrls))
		copy(serviceInfo.DefaultTurnUrls, server.Config.DefaultTurnUrls)
	}

	if queryNatName == "" { // for all
		serviceInfo.ConnectedNames = server.rns.GetNames(func(natDuplex *NatDuplex) bool {
			return true
		})

		serviceInfo.RegisteredNames = server.connManager.GetNames(nil)
	} else { // for queryNatName
		_, err := server.rns.GetNatduplex(queryNatName)
		if err == nil {
			serviceInfo.ConnectedNames = []string{queryNatName}
		}
		if server.connManager.HasConnection(queryNatName) {
			serviceInfo.RegisteredNames = []string{queryNatName}
		}
	}

	return serviceInfo
}

func (server *Bnrtc1Server) Start() error {
	if server.Config.EnableStunServer && server.stunSev == nil {
		udpListener, err := net.ListenPacket("udp4", "0.0.0.0:"+strconv.Itoa(int(server.Config.StunPort)))
		if err != nil {
			return err
		}
		stunSev := stund.Server{
			Conn: udpListener,
		}
		server.stunSev = &stunSev
		log.Println("start stun server", server.Config.StunPort)
		go func() {
			err := stunSev.Serve()
			if err != nil {
				log.Println("stun server error", err)
			}
		}()
	}

	if server.Config.EnableTurnServer && server.turnSev == nil {
		udpListener, err := net.ListenPacket("udp4", "0.0.0.0:"+strconv.Itoa(int(server.Config.TurnPort)))
		if err != nil {
			return err
		}
		if server.Config.PublicIp == "" {
			ipInfo, err := server.Client.GetLocalIpAddress(nil)
			if err != nil {
				return err
			}
			server.Config.PublicIp = ipInfo.Address
		}
		log.Println("start turn server", server.Config.TurnPort)
		turnSev, err := turn.NewServer(turn.ServerConfig{
			Realm: "ibt.bfchain",
			AuthHandler: func(username, realm string, srcAddr net.Addr) (key []byte, ok bool) {
				/**
				 * @TODO 使用非对称加密算法
				 */
				return turn.GenerateAuthKey(realm, realm, realm), true
			},
			PacketConnConfigs: []turn.PacketConnConfig{
				{
					PacketConn: udpListener,
					RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
						RelayAddress: net.ParseIP(server.GetPublicIp()), // Claim that we are listening on IP passed by user (This should be your Public IP)
						Address:      "0.0.0.0",                         // But actually be listening on every interface
					},
				},
			},
		})
		if err != nil {
			return err
		}
		server.turnSev = turnSev
	}

	if server.Config.EnableHttpServer && server.httpSev == nil {
		handler := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			var res []byte
			var err error
			var statusCode = 200
			path := r.URL.Path
			if r.Method == Bnrtc1ServiceHandlerGet || r.Method == Bnrtc1ServiceHandlerPost {
				h, ok := server.handlers[r.Method][path]
				if !ok {
					err = fmt.Errorf("handler for %s not found", path)
				} else {
					statusCode = 200
					if r.Method == Bnrtc1ServiceHandlerPost {
						_ = r.ParseForm()
					}
					res, err = h(r, rw)
					if err != nil {
						statusCode = 502
						res = []byte(err.Error())
					}
				}
			} else {
				err = fmt.Errorf("method %s not supported", r.Method)
			}

			if err != nil {
				rw.WriteHeader(500)
				_, err = rw.Write([]byte(err.Error()))
				if err != nil {
					log.Println(err)
				}
			} else if res != nil {
				rw.WriteHeader(statusCode)
				_, err = rw.Write(res)
				if err != nil {
					log.Println(err)
				}
			}
		})

		httpSev := http.Server{Addr: ":" + strconv.Itoa(int(server.Config.HttpPort)), Handler: handler}
		log.Println("start http server", server.Config.HttpPort)
		go func() {
			err := httpSev.ListenAndServe()
			if err != nil {
				log.Println(err)
			}
		}()
		server.httpSev = &httpSev
	}
	server.connManager.Start()
	server.Client.Start()
	return nil
}

func (server *Bnrtc1Server) Stop() error {
	server.Client.Stop()
	if server.httpSev != nil {
		if err := server.httpSev.Close(); err != nil {
			return err
		}
		server.httpSev = nil
		// 如果只是停止监听，这里不会销毁server.rns中的连接
	}
	if server.turnSev != nil {
		if err := server.turnSev.Close(); err != nil {
			return err
		}
		server.turnSev = nil
	}
	if server.stunSev != nil {
		if err := server.stunSev.Close(); err != nil {
			return err
		}
		server.stunSev = nil
	}
	server.connManager.Close()
	return nil
}
func (server *Bnrtc1Server) Destroy() error {
	err := server.Stop()
	if err != nil {
		return err
	}
	server.rns.CloseAllNatDuplex()
	return nil
}

func (server *Bnrtc1Server) BindServerRnsHooks(hooks RegisteredNatDuplexsHooks) error {
	if server.rns == nil {
		return errors.New("need init server first")
	}
	server.rns.BindRnsHooks(hooks)
	return nil
}

func (server *Bnrtc1Server) GetConnectedNatDuplexByRemoteNatName(remoteNatName string) (*NatDuplex, error) {
	if server.rns == nil {
		return nil, errors.New("need init server first")
	}
	return server.rns.GetNatduplex(remoteNatName)
}
