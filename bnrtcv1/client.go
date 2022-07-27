package bnrtcv1

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/guabee/bnrtc/util"
	"github.com/pion/webrtc/v3"
)

const Version = "1.0.0"

type OfferInfo struct {
	Offer   *webrtc.SessionDescription
	NatName string
	Version string
}

func IsSameVersion(removeVersion, localVersion string) bool {
	return removeVersion == localVersion
}

func ParseOfferInfo(postForm url.Values) (*OfferInfo, error) {
	offerJSONs, found := postForm["offer"]
	if !found || len(offerJSONs) == 0 {
		return nil, errors.New("no found offer in post form")
	}

	natNames, found := postForm["natName"]
	if !found || len(natNames) == 0 {
		return nil, errors.New("no found natName in post form")
	}

	version, found := postForm["version"]
	if !found || len(version) == 0 {
		return nil, errors.New("no found version in post form")
	}

	offer := &webrtc.SessionDescription{}
	err := json.Unmarshal([]byte(offerJSONs[0]), offer)
	if err != nil {
		return nil, err
	}

	return &OfferInfo{
		Offer:   offer,
		NatName: natNames[0],
		Version: version[0],
	}, nil
}

type Bnrtc1Client struct {
	Peer             *Bnrtc1Peer
	Config           *Bnrtc1ClientConfig
	rns              *RegisteredNatDuplexs
	natDuplexFactory *NatDuplexFactory
	connManager      *ConnectionManager
	signalOption     InternetPeerOptions
}
type Bnrtc1ClientConfig struct {
	Name            string `json:"name"`
	LocalNatName    string `json:"localNatName"`
	ConnectionLimit int    `json:"connectionLimit"`
	HoldHooks       bool   `json:"holdHooks"`
}
type IpInfo struct {
	Version  IPVersion `json:"version"`
	Address  string    `json:"address"`
	IsPublic bool      `json:"isPublic"`
}

func NewClient(peer *Bnrtc1Peer, config *Bnrtc1ClientConfig) (*Bnrtc1Client, error) {
	return newClientWithCustomRns(config, peer, nil)
}
func newClientWithCustomRns(config *Bnrtc1ClientConfig, peer *Bnrtc1Peer, rns *RegisteredNatDuplexs) (*Bnrtc1Client, error) {
	if config == nil {
		config = &Bnrtc1ClientConfig{}
	}
	if config.LocalNatName == "" {
		config.LocalNatName = config.Name
	}
	if config.ConnectionLimit == 0 {
		config.ConnectionLimit = 1024
	}

	if peer == nil {
		peer = &Bnrtc1Peer{}
	}
	if rns == nil {
		rns = newRegisteredNatDuplexs(config.LocalNatName, config.HoldHooks)
	}
	client := &Bnrtc1Client{
		Peer:             peer,
		Config:           config,
		rns:              rns,
		natDuplexFactory: NewNatDuplexFactory(),
		connManager:      newConnectionManager(config.LocalNatName, config.ConnectionLimit),
	}

	client.connManager.SetOfferHandler(func(offerInfo *OfferInfo) (res []byte, err error) {
		serviceInfo, err := client.GetServiceInfo(client.signalOption.Address, client.signalOption.Port, client.Config.LocalNatName)
		if err != nil {
			return nil, err
		}
		pcConfig := serviceInfo.ToPeerConnectionConfig(client.signalOption.Address)
		return handlerOffer(client, client.rns, pcConfig, offerInfo)
	})

	return client, nil
}

func (client *Bnrtc1Client) Start() {
	client.natDuplexFactory.Start()
	client.connManager.Start()
}

func (client *Bnrtc1Client) Stop() {
	client.connManager.Close()
	client.natDuplexFactory.Stop()
	client.rns.CloseAllNatDuplex()
}

func selectPeer(client *Bnrtc1Client, selectedPeer *InternetPeerOptions) (*PeerItem, error) {
	if selectedPeer == nil {
		peerCount := len(client.Peer.peerList)
		if peerCount == 0 {
			return nil, errors.New("no peer")
		}
		randomIndex := rand.Intn(peerCount)
		randomPeer := client.Peer.peerList[randomIndex]
		return randomPeer, nil
	}
	for _, peer := range client.Peer.peerList {
		if peer.Address == selectedPeer.Address {
			if selectedPeer.Port == 0 { // port == 0 == defaultPort
				return peer, nil
			}
			if selectedPeer.Port == int32(peer.Port) {
				return peer, nil
			}
		}
	}
	return nil, fmt.Errorf("no found peer %s:%d", selectedPeer.Address, selectedPeer.Port)
}

func processHttpResponse(resp *http.Response, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		errMsg := fmt.Sprintf("req: %s, code: %d, body: %s", resp.Request.URL, resp.StatusCode, string(body))
		return nil, errors.New(errMsg)
	}
	if err != nil {
		return nil, err
	}
	return body, nil
}
func GetLocalIpAddressByHttp(host string) (*IpInfo, error) {
	fetchIpUrl := fmt.Sprintf("http://%s/ip", host)
	maybeIpBytes, err := processHttpResponse(http.Get(fetchIpUrl))
	if err != nil {
		return nil, err
	}

	maybeIp := string(maybeIpBytes)
	res := IpInfo{
		Address: maybeIp,
		Version: IPv4,
	}

	ipAddress := net.ParseIP(maybeIp)
	ipLen := len(ipAddress)
	if ipLen == net.IPv4len {
		res.Version = IPv4
	} else if ipLen == net.IPv6len {
		res.Version = IPv6
	} else {
		return nil, errors.New("unknown ip version: " + maybeIp)
	}

	return &res, nil
}
func (client *Bnrtc1Client) GetLocalIpAddress(peerOptions *InternetPeerOptions) (*IpInfo, error) {
	selectedPeer, err := selectPeer(client, peerOptions)
	if err != nil {
		return nil, err
	}
	return GetLocalIpAddressByHttp(fmt.Sprintf("%s:%d", selectedPeer.Address, selectedPeer.Port))
}

func (client *Bnrtc1Client) GetServiceInfo(address string, port int32, queryNatName string) (*ServiceInfo, error) {
	fetchServicesUrl := fmt.Sprintf("http://%s:%d/services", address, port)
	if queryNatName != "" {
		fetchServicesUrl = fmt.Sprintf("%s?queryNatName=%s", fetchServicesUrl, url.QueryEscape(queryNatName))
	}
	serviceInfoBytes, err := processHttpResponse(http.Get(fetchServicesUrl))
	if err != nil {
		return nil, err
	}

	serviceInfo := &ServiceInfo{}
	if err := json.Unmarshal(serviceInfoBytes, serviceInfo); err != nil {
		return nil, err
	}
	return serviceInfo, nil
}

type ServiceInfo struct {
	Name            string   `json:"name"`
	PublicIp        string   `json:"publicIp"`
	LocalNatName    string   `json:"localNatName"`
	TurnPort        int32    `json:"turnPort"`
	StunPort        int32    `json:"stunPort"`
	DefaultStunUrls []string `json:"defaultStunUrls"`
	DefaultTurnUrls []string `json:"defaultTurnUrls"`
	RegisteredNames []string `json:"registeredNames"`
	ConnectedNames  []string `json:"connectedNames"`
}

func (info *ServiceInfo) ToPeerConnectionConfig(address string) *webrtc.Configuration {
	pc_stun_urls := make([]string, len(info.DefaultStunUrls))
	copy(pc_stun_urls, info.DefaultStunUrls)
	if len(pc_stun_urls) != 0 {
		for i, stunUrl := range pc_stun_urls {
			stunUrl = strings.ReplaceAll(stunUrl, "0.0.0.0", address)
			stunUrl = strings.ReplaceAll(stunUrl, "127.0.0.1", address)
			pc_stun_urls[i] = stunUrl
		}
	} else if info.StunPort > 0 && info.StunPort < 65536 {
		pc_stun_urls = append(pc_stun_urls, fmt.Sprintf("stun:%s:%d", address, info.StunPort))
	}

	pc_turn_urls := make([]string, len(info.DefaultTurnUrls))
	copy(pc_turn_urls, info.DefaultTurnUrls)
	if len(pc_turn_urls) != 0 {
		for i, turnUrl := range pc_turn_urls {
			turnUrl = strings.ReplaceAll(turnUrl, "0.0.0.0", address)
			turnUrl = strings.ReplaceAll(turnUrl, "127.0.0.1", address)
			pc_turn_urls[i] = turnUrl
		}
	} else if info.TurnPort > 0 && info.TurnPort < 65536 {
		pc_turn_urls = append(pc_turn_urls, fmt.Sprintf("turn:%s:%d", address, info.TurnPort))
	}

	return &webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: pc_stun_urls,
			},
			{
				URLs:       pc_turn_urls,
				Username:   "ibt.bfchain",
				Credential: "ibt.bfchain",
			},
		},
	}
}

func (client *Bnrtc1Client) SetLocalNatName(natName string) {
	if natName == client.Config.LocalNatName {
		return
	}
	client.Config.LocalNatName = natName
	client.rns.SetLocalNatName(natName)
	client.connManager.SetLocalNatName(natName)
}

func (client *Bnrtc1Client) ConnectSignalServer(peerOptions *InternetPeerOptions) (*Connection, error) {
	serviceInfo, err := client.GetServiceInfo(peerOptions.Address, peerOptions.Port, "")
	if err != nil {
		return nil, err
	}

	wsUrl := fmt.Sprintf("ws://%s:%d/websocket?natName=%s&version=%s", peerOptions.Address, peerOptions.Port, client.Config.LocalNatName, Version)
	c, _, err := websocket.DefaultDialer.Dial(wsUrl, nil)
	if err != nil {
		return nil, err
	}
	conn, err := client.connManager.NewConnection(serviceInfo.LocalNatName, c)
	if err != nil {
		return nil, err
	}
	client.signalOption = *peerOptions
	conn.Start()
	return conn, nil
}

func (client *Bnrtc1Client) OnConnection(f func(conn *Connection)) {
	client.connManager.OnConnection(f)
}

func contains(s []string, searchIterm string) bool {
	sort.Strings(s)
	i := sort.SearchStrings(s, searchIterm)
	return i < len(s) && s[i] == searchIterm
}
func (client *Bnrtc1Client) ConnectNatPeer(peerOptions *InternetPeerOptions, remoteNatName string) (*NatDuplex, error) {
	return client._connectNatPeer(peerOptions, remoteNatName, false)
}

// like ConnectNatPeer, but reuse the connection if exist
func (client *Bnrtc1Client) LinkNatPeer(peerOptions *InternetPeerOptions, remoteNatName string) (*NatDuplex, error) {
	return client._connectNatPeer(peerOptions, remoteNatName, true)
}

/// Use NAT to Connect Registered Peer
func (client *Bnrtc1Client) _connectNatPeer(peerOptions *InternetPeerOptions, remoteNatName string, reuse bool) (*NatDuplex, error) {
	localNatName := client.Config.LocalNatName
	serviceInfo, err := client.GetServiceInfo(peerOptions.Address, peerOptions.Port, remoteNatName)
	if err != nil {
		return nil, err
	}
	if serviceInfo.LocalNatName == remoteNatName {
		return nil, fmt.Errorf("no allow connect signal server name: %s", remoteNatName)
	}

	if !contains(serviceInfo.RegisteredNames, remoteNatName) {
		return nil, fmt.Errorf("no found registried nat name: %s", remoteNatName)
	}

	pcConfigGetter := func() (*webrtc.Configuration, error) {
		return serviceInfo.ToPeerConnectionConfig(peerOptions.Address), nil
		// pcConfig, err := getPcConfig(serviceInfo, peerOptions.Address)
		// if err != nil {
		// 	return pcConfig, err
		// }

		// pcConfig.ICETransportPolicy = webrtc.NewICETransportPolicy("relay")
		// return pcConfig, nil
	}
	o2aHrefGetter := func(postData url.Values) string {
		postData.Set("method", "offer2answer")
		postData.Set("natServerName", remoteNatName)
		return fmt.Sprintf("http://%s:%d/nat", peerOptions.Address, peerOptions.Port)
	}
	source := fmt.Sprintf("bnrtc1:%s:%d/%s", peerOptions.Address, peerOptions.Port, remoteNatName)

	return client._connectDuplex(reuse, source, localNatName, remoteNatName, pcConfigGetter, o2aHrefGetter)
}

func (client *Bnrtc1Client) _connectDuplex(reuse bool, source string, localNatName string, remoteNatName string, pcConfigGetter func() (*webrtc.Configuration, error), o2HrefGetter func(postData url.Values) string) (*NatDuplex, error) {
	duplexKey := fmt.Sprintf("%s->%s", localNatName, remoteNatName)
	if client.rns.HasNatDuplex(remoteNatName) {
		if reuse {
			return client.rns.GetNatduplex(remoteNatName)
		}
		return nil, fmt.Errorf("client(%s) already has natDuplex(%s)", client.Config.Name, duplexKey)
	}

	pc_config, err := pcConfigGetter()
	if err != nil {
		return nil, err
	}

	/// 创建natDuplex
	natDuplex, err := client.natDuplexFactory.New(pc_config, source, localNatName, remoteNatName, NatDuplexRoleOffer, duplexKey)
	if err != nil {
		return nil, err
	}
	natDuplex.CreateDataChannels()
	pc := natDuplex.pc
	pcConnectedChan := make(chan error, 1)
	dataChannelReadyChan := make(chan error, 1)

	/// 注册与销毁
	pc.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		if pcs == webrtc.PeerConnectionStateDisconnected || pcs == webrtc.PeerConnectionStateFailed || pcs == webrtc.PeerConnectionStateClosed {
			pcConnectedChan <- errors.New(pcs.String())
			natDuplex.onClose()
			if err := client.rns.Delete(natDuplex); err == nil {
				go client.rns._emitOnNatDuplexDisConnected(natDuplex)
			}
		} else if pcs == webrtc.PeerConnectionStateConnected {
			if err := client.rns.Add(natDuplex); err == nil {
				pcConnectedChan <- nil
				go func() {
					if err := natDuplex.WaitReady(); err == nil {
						client.rns._emitOnNatDuplexConnected(natDuplex)
					} else {
						pc.Close()
						log.Println(err)
					}
					dataChannelReadyChan <- err
				}()
			} else {
				pcConnectedChan <- err
				pc.Close()
			}
		}
	})

	readyCond := util.NewCondition(util.WithConditionTimeout(60 * time.Second))
	go func() {
		offer, err := pc.CreateOffer(nil)
		if err != nil {
			readyCond.Signal(err)
			return
		}
		if err := pc.SetLocalDescription(offer); err != nil {
			readyCond.Signal(err)
			return
		}

		gatherComplete := webrtc.GatheringCompletePromise(pc)
		<-gatherComplete

		if bytes, err := json.Marshal(*pc.LocalDescription()); err != nil {
			readyCond.Signal(err)
			return
		} else {
			postData := url.Values{
				"offer":   {string(bytes)},
				"natName": {localNatName},
				"version": {Version},
			}
			offer2answerHref := o2HrefGetter(postData)
			answerBytes, err := processHttpResponse((&http.Client{Timeout: 60 * time.Second}).PostForm(offer2answerHref, postData))
			if err != nil {
				readyCond.Signal(err)
				return
			}

			answer := &webrtc.SessionDescription{}
			if err := json.Unmarshal(answerBytes, answer); err != nil {
				readyCond.Signal(err)
				return
			}
			if err := pc.SetRemoteDescription(*answer); err != nil {
				readyCond.Signal(err)
				return
			}
		}
		/// 等待dataChannel打开，如果握手成果，那么就意味着nat穿透完成。这个dataChannel会用来替代offer2answer等接口服务
		pcOpenedRes := <-pcConnectedChan
		if pcOpenedRes != nil {
			if client.rns.HasNatDuplex(remoteNatName) {
				if reuse {
					natDuplex, err = client.rns.GetNatduplex(remoteNatName)
					if err != nil {
						readyCond.Signal(err)
					} else {
						readyCond.Signal(nil)
					}
				} else {
					readyCond.Signal(fmt.Errorf("client(%s) already has natDuplex(%s)", client.Config.Name, duplexKey))
				}
			} else {
				readyCond.Signal(pcOpenedRes)
			}
			return
		}

		readyCond.Signal(<-dataChannelReadyChan)
	}()

	err = readyCond.Wait()
	if err != nil {
		pc.Close()
		return nil, err
	} else {
		return natDuplex, nil
	}
}

func (client *Bnrtc1Client) GetConnectedNatDuplexCount() int {
	return client.rns.Size()
}

func (client *Bnrtc1Client) BindClientRnsHooks(hooks RegisteredNatDuplexsHooks) error {
	client.rns.BindRnsHooks(hooks)
	return nil
}

func (client *Bnrtc1Client) GetConnectedNatDuplexByRemoteNatName(remoteNatName string) (*NatDuplex, error) {
	if client.rns == nil {
		return nil, errors.New("need init client first")
	}
	return client.rns.GetNatduplex(remoteNatName)
}
