package transport

import (
	"sync"

	"github.com/guabee/bnrtc/bnrtcv1"
	"github.com/guabee/bnrtc/bnrtcv2/connection"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
)

type ISignalService interface {
	ConnectSignalServer(peerOptions *bnrtcv1.InternetPeerOptions) (ISignalConnection, error)
	OnConnection(f func(conn ISignalConnection))
	GetLocalIp(host string) (string, error)
	SetLocalIp(ip string)
}

type IPeerService interface {
	LinkNatPeer(peerOptions *bnrtcv1.InternetPeerOptions, remoteNatName string) (connection.INatDuplex, error)
	IsOnline(devid string) bool
	GetServicePort() uint16
	BindEventHook(hook bnrtcv1.RegisteredNatDuplexsHooks) error
}

type IBnrtc1Service interface {
	ISignalService
	IPeerService
}

type IBnrtc1ServiceBuilder func(devId deviceid.DeviceId) IBnrtc1Service

type Bnrtc1ServerWrapper struct {
	server *bnrtcv1.Bnrtc1Server
}

var bnrtc1Service IBnrtc1Service = nil
var initBnrtc1ServiceOnce sync.Once

func NewBnrtc1ServerWrapper(server *bnrtcv1.Bnrtc1Server) *Bnrtc1ServerWrapper {
	return &Bnrtc1ServerWrapper{
		server: server,
	}
}

func (w *Bnrtc1ServerWrapper) GetSignalService() ISignalService {
	return w
}
func (w *Bnrtc1ServerWrapper) GetPeerService() IPeerService {
	return w
}

func (w *Bnrtc1ServerWrapper) ConnectSignalServer(peerOptions *bnrtcv1.InternetPeerOptions) (ISignalConnection, error) {
	return w.server.Client.ConnectSignalServer(peerOptions)
}

func (w *Bnrtc1ServerWrapper) OnConnection(f func(conn ISignalConnection)) {
	w.server.Client.OnConnection(func(conn *bnrtcv1.Connection) {
		f(conn)
	})
	w.server.OnConnection(func(conn *bnrtcv1.Connection) {
		f(conn)
	})
}

func (w *Bnrtc1ServerWrapper) LinkNatPeer(peerOptions *bnrtcv1.InternetPeerOptions, remoteNatName string) (connection.INatDuplex, error) {
	return w.server.Client.LinkNatPeer(peerOptions, remoteNatName)
}
func (w *Bnrtc1ServerWrapper) IsOnline(address string) bool {
	return w.server.IsOnline(address)
}
func (w *Bnrtc1ServerWrapper) GetLocalIp(host string) (string, error) {
	ip, err := bnrtcv1.GetLocalIpAddressByHttp(host)
	if err != nil {
		return "", err
	}
	w.SetLocalIp(ip.Address)
	return ip.Address, nil
}
func (w *Bnrtc1ServerWrapper) SetLocalIp(ip string) {
	w.server.SetPublicIp(ip)
}

func (w *Bnrtc1ServerWrapper) GetServicePort() uint16 {
	return w.server.Config.HttpPort
}

func (w *Bnrtc1ServerWrapper) SetLocalNatName(address string) {
	w.server.SetLocalNatName(address)
}

func (w *Bnrtc1ServerWrapper) BindEventHook(hook bnrtcv1.RegisteredNatDuplexsHooks) error {
	return w.server.BindServerRnsHooks(hook)
}
