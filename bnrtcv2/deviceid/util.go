package deviceid

import (
	"bytes"
	"net"
	"sort"

	"github.com/btcsuite/btcutil/base58"
)

// func base64Encode(data []byte) string {
// 	return base64.StdEncoding.EncodeToString(data)
// }

// func base64Decode(data string) []byte {
// 	dec, err := base64.StdEncoding.DecodeString(data)
// 	if err != nil {
// 		return nil
// 	}
// 	return dec
// }

func base58Encode(data []byte) string {
	return base58.Encode(data)
}

func base58Decode(data string) []byte {
	return base58.Decode(data)
}

const (
	Ipv4   = 0
	Ipv6   = 1
	Bnrtc1 = 2
	Lan    = 3
)

func GetLocalIpNetList() (ipv4 []*net.IPNet, ipv6 []*net.IPNet, lan []*net.IPNet) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, nil, nil
	}
	ipv4List := make([]*net.IPNet, 0)
	ipv6List := make([]*net.IPNet, 0)
	lanList := make([]*net.IPNet, 0)
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			ip := ipnet.IP
			if ip.IsGlobalUnicast() {
				if ip.To4() != nil {
					if !ip.IsPrivate() {
						ipv4List = append(ipv4List, ipnet)
					} else {
						lanList = append(lanList, ipnet)
					}
				} else if ip.To16() != nil {
					ipv6List = append(ipv6List, ipnet)
				}
			}
		}
	}
	sort.Slice(ipv4List, func(i, j int) bool {
		return bytes.Compare(ipv4List[i].IP, ipv4List[j].IP) < 0
	})

	sort.Slice(ipv6List, func(i, j int) bool {
		return bytes.Compare(ipv6List[i].IP, ipv6List[j].IP) < 0
	})

	sort.Slice(lanList, func(i, j int) bool {
		return bytes.Compare(lanList[i].IP, lanList[j].IP) < 0
	})

	return ipv4List, ipv6List, lanList
}
