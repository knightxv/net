package transport_test

import (
	"net"
	"testing"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/transport"
	"github.com/guabee/bnrtc/buffer"
)

func TestTransportUDPInfo(t *testing.T) {
	tran, _ := transport.NewUdpTransport(transport.UdpTransportOption{
		Address: "0.0.0.0",
		Port:    7777,
	})

	if tran == nil {
		t.Error("Failed to create transport")
		return
	}
	if tran.Port != 7777 {
		t.Error("Failed to get port")
	}
	if tran.Name() != "UDP" {
		t.Error("Failed to get transport name")
	}
	if tran.Type() != transport.DirectType {
		t.Error("Failed to get transport type")
	}
}

func TestTransportUDPSend(t *testing.T) {
	t1, _ := transport.NewUdpTransport(transport.UdpTransportOption{
		Address:    "0.0.0.0",
		Port:       9999,
		NeedAck:    true,
		AckTimeout: time.Second * 2,
	})
	t1.SetDeviceId(deviceid.NewDeviceId(true, []byte{192, 168, 0, 1}, nil))
	t2, _ := transport.NewUdpTransport(transport.UdpTransportOption{
		Address: "0.0.0.0",
		Port:    8888,
	})
	t2.SetDeviceId(deviceid.NewDeviceId(false, []byte{192, 168, 0, 1}, nil))

	t1Addr := transport.NewAddress(nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9999})
	t2Addr := transport.NewAddress(nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8888})
	t3Addr := transport.NewAddress(nil, &net.UDPAddr{IP: net.IPv4(192, 168, 0, 1), Port: 8888})

	t1Msg := []byte("Hello")
	t2Msg := []byte("World")
	t1SendCnt := 0
	t1RecvCnt := 0
	t2SendCnt := 0
	t2RecvCnt := 0

	t1.OnMessage(func(address *transport.TransportAddress, buf *buffer.Buffer) error {
		if string(buf.Data()) != string(t2Msg) {
			t.Error("Failed to receive message")
		}
		t1RecvCnt++
		return nil
	})

	t2.OnMessage(func(address *transport.TransportAddress, buf *buffer.Buffer) error {
		if string(buf.Data()) != string(t1Msg) {
			t.Error("Failed to receive message")
		}
		t2RecvCnt++
		return nil
	})
	err := t1.Send(t2Addr, buffer.FromBytes(t1Msg), false)
	if err != nil {
		t.Error("Failed to send message")
	}
	t1SendCnt++
	err = t2.Send(t1Addr, buffer.FromBytes(t2Msg), false)
	if err != nil {
		t.Error("Failed to send message")
	}
	t2SendCnt++

	time.Sleep(2 * time.Second)
	if t1SendCnt != 1 || t1RecvCnt != 1 || t2SendCnt != 1 || t2RecvCnt != 1 {
		t.Error("Failed to send message")
	}

	if !t1.IsTargetReachable(t2Addr) {
		t.Error("Failed to check reachable")
	}

	if !t1.IsTargetReachable(t3Addr) {
		t.Error("Failed to check reachable")
	}

	ip, err := t1.GetExternalIp(t2Addr)
	if err != nil {
		t.Error("Failed to get external ip")
	}
	if ip.String() != "127.0.0.1" {
		t.Error("Failed to get external ip")
	}

	t1.Close()
}
