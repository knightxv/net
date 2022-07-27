package transport_test

import (
	"net"
	"testing"

	"github.com/guabee/bnrtc/bnrtcv2/transport"
)

func TestAddress(t *testing.T) {
	peer1Addr := transport.NewAddress(nil, &net.UDPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 0,
	})
	peer2Addr := transport.NewAddress(nil, &net.UDPAddr{
		IP:   net.IPv4(1, 1, 1, 1),
		Port: 0,
	})
	if !peer1Addr.IsLocal() {
		t.Error("Failed to check local address")
	}
	if peer2Addr.IsLocal() {
		t.Error("Failed to check local address")
	}
	if peer1Addr.IsPublic() {
		t.Error("Failed to check public address")
	}
	if !peer2Addr.IsPublic() {
		t.Error("Failed to check public address")
	}
	if !peer1Addr.IsIPv4() {
		t.Error("Failed to check ipv4 address")
	}
	if peer2Addr.String() != ":"+peer2Addr.Addr.String() {
		t.Error("Failed to check address string")
	}
	if transport.AddressFromString(peer1Addr.Addr.String()).String() != peer1Addr.String() {
		t.Error("Failed to check address string")
	}
}
