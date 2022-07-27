package bnrtcv1

import (
	"errors"
	"net"
)

type Bnrtc1Peer struct {
	peerList []*PeerItem
}
type PeerItem struct {
	Version  IPVersion
	Address  string
	IsPublic bool
	Port     uint16
}

func peerItem2InternetPeer(peerItem *PeerItem) *InternetPeer {
	return &InternetPeer{
		Address:  peerItem.Address,
		Port:     int32(peerItem.Port),
		Version:  peerItem.Version,
		IsPublic: peerItem.IsPublic,
	}
}

type IPVersion = uint8

const (
	IPv4 IPVersion = 4
	IPv6 IPVersion = 6
)

type InternetPeerOptions struct {
	Address string `json:"address"`
	Port    int32  `json:"port"`
}
type InternetPeer struct {
	Address  string    `json:"address"`
	Port     int32     `json:"port"`
	IsPublic bool      `json:"isPublic"`
	Version  IPVersion `json:"version"`
}

func IsPublicIP(IP net.IP) bool {
	return IP.IsGlobalUnicast() && !IP.IsPrivate()
}

func (server *Bnrtc1Peer) AddPeer(peerOptions *InternetPeerOptions) (*InternetPeer, error) {
	peerAddress := peerOptions.Address
	addressInfo := net.ParseIP(peerAddress)
	addressLen := len(addressInfo)
	var addressVersion IPVersion

	if addressLen == net.IPv4len {
		addressVersion = IPv4
	} else if addressLen == net.IPv6len {
		addressVersion = IPv6
	} else {
		return nil, errors.New("unknown ip version: " + peerAddress)
	}

	var peerPort uint16 = 19103
	if peerOptions.Port > 0 && peerOptions.Port < 65536 {
		peerPort = uint16(peerOptions.Port)
	}
	// peer := &InternetPeer{}
	peerItem := &PeerItem{
		Address:  peerOptions.Address,
		Port:     peerPort,
		Version:  addressVersion,
		IsPublic: IsPublicIP(addressInfo),
	}
	server.peerList = append(server.peerList, peerItem)

	return peerItem2InternetPeer(peerItem), nil
}

// go mobile not support slice
func (server *Bnrtc1Peer) GetPeers(offset int, limit int) []*InternetPeer {
	peers := make([]*InternetPeer, 0)
	peerCount := len(server.peerList)
	if peerCount < (offset + 1) {
		return peers
	}
	endIndex := offset + limit
	if peerCount < (endIndex + 1) {
		endIndex = peerCount - 1
	}
	peers = make([]*InternetPeer, endIndex-offset)
	for i, peerItem := range server.peerList[offset:endIndex] {
		peers[i] = peerItem2InternetPeer(peerItem)
	}
	return peers
}

func (server *Bnrtc1Peer) GetItemAt(pos int) *InternetPeer {
	if pos >= 0 && pos < len(server.peerList) {
		return peerItem2InternetPeer(server.peerList[pos])
	}
	return nil
}

func (server *Bnrtc1Peer) CountPeers() int {
	return len(server.peerList)
}
