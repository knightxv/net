package device_info_test

import (
	"net"
	"testing"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
)

type TransportManagerMock struct {
}

func (d *TransportManagerMock) AddTransport(transport transport.ITransport) {}
func (d *TransportManagerMock) SendCtrlMsg(addr *transport.TransportAddress, buf *buffer.Buffer) error {
	return nil
}
func (d *TransportManagerMock) SendDataMsg(addr *transport.TransportAddress, buf *buffer.Buffer) error {
	return nil
}
func (d *TransportManagerMock) BroadcastDataMsg(addr *transport.TransportAddress, buf *buffer.Buffer, isSource bool) error {
	return nil
}
func (d *TransportManagerMock) SetDeviceId(id deviceid.DeviceId) {}
func (d *TransportManagerMock) GetExternalIp(addr *transport.TransportAddress) (net.IP, error) {
	return nil, nil
}
func (d *TransportManagerMock) IsTargetReachable(addr *transport.TransportAddress) bool {
	if addr.IsPublic() || addr.IsLocal() {
		return true
	} else if addr.Addr.IP.Equal(net.IPv4(192, 168, 1, 1)) {
		return true
	}
	return false
}

func NewDeviceInfo(ipstr string, randomValue []byte) *device_info.DeviceInfo {
	ip := net.ParseIP(ipstr)

	isPublic := ip.IsGlobalUnicast() && !ip.IsPrivate()
	id := deviceid.NewDeviceId(isPublic, ip.To4(), randomValue)
	addr := transport.NewAddress(id, &net.UDPAddr{
		IP:   ip,
		Port: 9999,
	})

	return device_info.BuildDeviceInfo(addr)
}

func TestDeviceInfo(t *testing.T) {
	info1 := NewDeviceInfo("192.168.1.1", nil)
	info2 := NewDeviceInfo("192.168.1.2", nil)
	info3 := NewDeviceInfo("1.1.1.1", nil)
	info4 := NewDeviceInfo("192.168.1.1", info1.Id.NodeIdValue())

	if !info1.Equal(info4) {
		t.Errorf("DeviceInfo equal failed")
	}

	bytes := info1.GetBytes()
	if !device_info.FromBytes(bytes).Equal(info1) {
		t.Errorf("DeviceInfo from bytes failed")
	}

	addr, err := info2.ChooseMatchedAddress([]*device_info.DeviceInfo{info1}, deviceid.EmptyDeviceId, &TransportManagerMock{})
	if err != nil {
		t.Errorf("DeviceInfo choose matched address failed")
	}
	if addr.Addr.IP.String() != "192.168.1.1" {
		t.Errorf("DeviceInfo choose matched address failed")
	}

	addr2, err := info2.ChooseMatchedAddress([]*device_info.DeviceInfo{info3}, deviceid.EmptyDeviceId, &TransportManagerMock{})
	if err != nil {
		t.Errorf("DeviceInfo choose matched address failed")
	}
	if addr2.Addr.IP.String() != "1.1.1.1" {
		t.Errorf("DeviceInfo choose matched address failed")
	}
}

func TestManager(t *testing.T) {
	devID := deviceid.NewDeviceId(true, []byte{192, 168, 1, 1}, nil)
	manager := device_info.NewDeviceInfoManage("test", device_info.WithPort(10000))
	done := make(chan bool)

	_, _ = manager.Pubsub.SubscribeFunc("test", rpc.DeviceInfoChangeTopic, func(msg util.PubSubMsgType) {
		info := msg.(*device_info.DeviceInfo)
		if !info.Equal(manager.GetDeviceInfo()) {
			t.Errorf("DeviceInfo failed")
		}
		close(done)
	})

	go func() {
		manager.Pubsub.Publish(rpc.Bnrtc1SignalServerChangeTopic, transport.NewAddress(devID, &net.UDPAddr{
			IP:   net.IPv4(192, 168, 1, 1),
			Port: 10000,
		}))
		time.Sleep(time.Second)
		manager.Pubsub.Publish(rpc.DeviceIdChangeTopic, devID)
	}()

	<-done
}
