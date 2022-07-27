package transport

import (
	"fmt"
	"net"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
)

type TransportAddress struct {
	DeviceId deviceid.DeviceId
	Addr     net.UDPAddr
}

func NewAddress(id deviceid.DeviceId, addr *net.UDPAddr) *TransportAddress {
	var newAddr net.UDPAddr
	if addr != nil {
		newAddr = *addr
	}

	return &TransportAddress{
		DeviceId: id,
		Addr:     newAddr,
	}
}

func AddressFromString(host string) *TransportAddress {
	addr, _ := net.ResolveUDPAddr("udp", host)
	return NewAddress(nil, addr)
}

func (t *TransportAddress) HasIp() bool {
	return t.Addr.IP != nil
}

func (t *TransportAddress) IsIPv4() bool {
	return t.Addr.IP.To4() != nil
}

func (t *TransportAddress) IsPublic() bool {
	return t.Addr.IP.IsGlobalUnicast() && !t.Addr.IP.IsPrivate()
}

const localAddrsCacheExpire = 5 * 60 * time.Second

var localAddrsCacheMap = make(map[string]struct{})
var localAddrsCacheTime = time.Now()

func isLocal(ip string) bool {
	_, found := localAddrsCacheMap[ip]
	return found
}

func rebuildLocalAddrsCache() {
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}

	newLocalAddrsCacheMap := make(map[string]struct{})
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				newLocalAddrsCacheMap[v.IP.String()] = struct{}{}
			case *net.IPAddr:
				newLocalAddrsCacheMap[v.IP.String()] = struct{}{}
			}
		}
	}

	localAddrsCacheMap = newLocalAddrsCacheMap
	localAddrsCacheTime = time.Now()
}

func (t *TransportAddress) IsLocal() bool {
	if t.Addr.IP.IsLoopback() {
		return true
	}

	if len(localAddrsCacheMap) == 0 || localAddrsCacheTime.Add(localAddrsCacheExpire).Before(time.Now()) {
		rebuildLocalAddrsCache()
	}

	return isLocal(t.Addr.IP.String())
}

func (t *TransportAddress) String() string {
	if t == nil {
		return ""
	}

	return fmt.Sprintf("%s:%s", t.DeviceId.String(), t.Addr.String())
}
