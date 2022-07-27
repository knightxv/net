package device_info

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
)

const (
	InterfaceTypeMin = 0
	Ipv4             = 0
	Ipv6             = 1
	Bnrtc1           = 2
	Lan              = 3
	InterfaceTypeMax = 4
)

func interfaceTypeToString(t int) string {
	switch t {
	case Ipv4:
		return "ipv4"
	case Ipv6:
		return "ipv6"
	case Bnrtc1:
		return "bnrtc1"
	case Lan:
		return "lan"
	default:
		return "unknown"
	}
}

type ITransportManager interface {
	IsTargetReachable(addr *transport.TransportAddress) bool
}

type Interface struct {
	IP   net.IP
	Port int
	Mask net.IPMask // network mask
}

type DeviceInfo struct {
	Name      string
	Id        deviceid.DeviceId
	Iterfaces [][]*Interface
	Port      int
}

func FromBytes(data []byte) *DeviceInfo {
	var d DeviceInfo
	err := json.Unmarshal(data, &d)
	if err != nil {
		return nil
	} else {
		return &d
	}
}

func BuildDeviceInfo(addr *transport.TransportAddress) *DeviceInfo {
	if addr == nil {
		return nil
	}

	info := &DeviceInfo{
		Id:        addr.DeviceId,
		Port:      addr.Addr.Port,
		Iterfaces: make([][]*Interface, InterfaceTypeMax),
	}
	ip := addr.Addr.IP
	inter := &Interface{IP: ip, Port: addr.Addr.Port, Mask: addr.Addr.IP.DefaultMask()}
	if addr.IsPublic() {
		if addr.IsIPv4() {
			info.Iterfaces[Ipv4] = append(info.Iterfaces[Ipv4], inter)
		} else {
			info.Iterfaces[Ipv6] = append(info.Iterfaces[Ipv6], inter)
		}
	} else if addr.IsIPv4() {
		info.Iterfaces[Lan] = append(info.Iterfaces[Lan], inter)
	}
	return info
}

// only got the first matched address
func (d *DeviceInfo) ToTransportAddress() *transport.TransportAddress {
	for t := InterfaceTypeMin; t < InterfaceTypeMax; t++ {
		// ignore bnrtc1 as it is not our ip
		if t == Bnrtc1 {
			continue
		}
		size := len(d.Iterfaces[t])
		if size != 0 {
			idx := rand.Intn(size)
			inter := d.Iterfaces[t][idx]
			return transport.NewAddress(d.Id, &net.UDPAddr{IP: inter.IP, Port: inter.Port})
		}
	}

	return nil
}

func (d *DeviceInfo) Equal(another *DeviceInfo) bool {
	return bytes.Equal(d.GetBytes(), another.GetBytes())
}

func (d *DeviceInfo) Clone() *DeviceInfo {
	info := &DeviceInfo{
		Name:      d.Name,
		Id:        d.Id,
		Port:      d.Port,
		Iterfaces: make([][]*Interface, InterfaceTypeMax),
	}

	for t := InterfaceTypeMin; t < InterfaceTypeMax; t++ {
		size := len(d.Iterfaces[t])
		if size != 0 {
			info.Iterfaces[t] = make([]*Interface, size)
			for i := 0; i < size; i++ {
				inter := d.Iterfaces[t][i]
				info.Iterfaces[t][i] = &Interface{IP: inter.IP, Port: inter.Port, Mask: inter.Mask}
			}
		}
	}

	return info
}

func (d *DeviceInfo) GetBytes() []byte {
	bytes, _ := json.Marshal(d)
	return bytes
}

func (d *DeviceInfo) String() string {
	buf := strings.Builder{}
	buf.WriteString(fmt.Sprintf("Name: %s, ", d.Name))
	buf.WriteString(fmt.Sprintf("Id: %s, ", d.Id))
	buf.WriteString(fmt.Sprintf("Port: %d, ", d.Port))
	for t := InterfaceTypeMin; t < InterfaceTypeMax; t++ {
		if len(d.Iterfaces[t]) == 0 {
			continue
		}
		buf.WriteString(fmt.Sprintf("%s: ", interfaceTypeToString(t)))
		for i, inter := range d.Iterfaces[t] {
			if i == 0 {
				buf.WriteString(fmt.Sprintf("%s:%d", inter.IP, inter.Port))
			} else {
				buf.WriteString(fmt.Sprintf(", %s:%d", inter.IP, inter.Port))
			}
		}
	}
	return buf.String()
}

func (d *DeviceInfo) IsValid() bool {
	if d == nil {
		return false
	}

	if d.Id.IsEmpty() {
		return false
	}

	if !d.HaveIterface() {
		return false
	}
	return true
}

func (d *DeviceInfo) haveIterface(tt int) bool {
	return len(d.Iterfaces[tt]) > 0
}

func (d *DeviceInfo) HaveIterface() bool {
	for _, interfaces := range d.Iterfaces {
		if len(interfaces) > 0 {
			return true
		}
	}
	return false
}

func (d *DeviceInfo) supportIpv4() bool {
	return d.haveIterface(Ipv4) || d.haveIterface(Lan)
}

func (d *DeviceInfo) supportIpv6() bool {
	return d.haveIterface(Ipv6)
}

func (d *DeviceInfo) ChooseMatchedAddress(infos []*DeviceInfo, devId deviceid.DeviceId, transportManager ITransportManager) (*transport.TransportAddress, error) {
	if len(infos) == 0 {
		return nil, errors.New("no device info")
	}

	if transportManager != nil {
		addr := ChooseMatchedAddressByDeviceId(infos, devId, transportManager)
		if addr != nil {
			return addr, nil
		}
	}

	return d.ChooseMatchedAddressByNet(infos, devId)

}

func ChooseMatchedAddressByDeviceId(infos []*DeviceInfo, devId deviceid.DeviceId, transportManager ITransportManager) *transport.TransportAddress {
	if !devId.IsEmpty() {
		addr := transport.NewAddress(devId, nil)
		if transportManager.IsTargetReachable(addr) {
			return addr
		}
	} else {
		// random the info order
		newInfos := make([]*DeviceInfo, 0, len(infos))
		copy(newInfos, infos)
		rand.Shuffle(len(newInfos), func(i, j int) {
			newInfos[i], newInfos[j] = newInfos[j], newInfos[i]
		})
		for _, info := range newInfos {
			if info.Id.IsEmpty() {
				continue
			}
			addr := transport.NewAddress(info.Id, nil)
			if transportManager.IsTargetReachable(addr) {
				return addr
			}
		}
	}

	return nil
}

func (d *DeviceInfo) ChooseMatchedAddressByNet(infos []*DeviceInfo, devId deviceid.DeviceId) (*transport.TransportAddress, error) {
	if len(infos) == 0 {
		return nil, errors.New("empty infos")
	}

	type InterfaceWithId struct {
		*Interface
		Id deviceid.DeviceId
	}

	var AddressLists [InterfaceTypeMax][]*InterfaceWithId

	getInterfaces := func(info *DeviceInfo, tt int) []*InterfaceWithId {
		size := len(info.Iterfaces[tt])
		interfaces := make([]*InterfaceWithId, size)
		for i := 0; i < size; i++ {
			interfaces[i] = &InterfaceWithId{
				Interface: info.Iterfaces[tt][i],
				Id:        info.Id,
			}
		}
		return interfaces
	}

	updateInfoList := func(info *DeviceInfo, tt int) {
		if len(info.Iterfaces[tt]) > 0 {
			interfaces := getInterfaces(info, tt)
			AddressLists[tt] = append(AddressLists[tt], interfaces...)
		}
	}

	for _, info := range infos {
		if info == nil {
			continue
		}
		if devId.IsEmpty() || info.Id.Equal(devId) {
			for t := InterfaceTypeMin; t < InterfaceTypeMax; t++ {
				updateInfoList(info, t)
			}
		}
	}

	getAddressByType := func(tt int, supported bool) *transport.TransportAddress {
		size := len(AddressLists[tt])
		if size > 0 && supported {
			idx := rand.Intn(size)
			inter := AddressLists[tt][idx]
			return transport.NewAddress(inter.Id, &net.UDPAddr{IP: inter.IP, Port: inter.Port})
		}

		return nil
	}

	if addr := getAddressByType(Ipv4, d.supportIpv4()); addr != nil {
		return addr, nil
	}
	if addr := getAddressByType(Ipv6, d.supportIpv6()); addr != nil {
		return addr, nil
	}
	if addr := getAddressByType(Bnrtc1, d.supportIpv4()); addr != nil {
		return addr, nil
	}
	// lan address must check in same local network
	if d.supportIpv4() {
		localInterface := getInterfaces(d, Lan)
		for _, inter := range AddressLists[Lan] {
			for _, local := range localInterface {
				if inter.IP.Mask(inter.Mask).Equal(local.IP.Mask(local.Mask)) {
					addr := transport.NewAddress(inter.Id, &net.UDPAddr{IP: inter.IP, Port: inter.Port})
					return addr, nil
				}
			}
		}
	}

	// note: when choose lan interface, we failed by the same local network check
	// now there is no other type of interfaces available, we just try pick one of them
	if addr := getAddressByType(Lan, d.supportIpv4()); addr != nil {
		return addr, nil
	}

	return nil, errors.New("no matched Address for Network")
}
