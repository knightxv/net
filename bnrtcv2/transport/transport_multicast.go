package transport

import (
	"net"
	"strconv"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/buffer"
)

// @warning: udp multicast not always enabled, not product used
const (
	MulticastAddress = "224.0.0.1"
	MaxDatagramSize  = 8192
)

type MulticastTransportOption struct {
	Port       int
	NeedAck    bool
	AckTimeout time.Duration
}

type MulticastTransport struct {
	DevId         deviceid.DeviceId
	Port          int
	name          string
	addr          *net.UDPAddr
	transportType TransportType
	udpWorker     *UdpTransportWorker
	conn          *net.UDPConn
	logger        *log.Entry
}

func NewMulticastTransport(option MulticastTransportOption) (*MulticastTransport, error) {
	t := &MulticastTransport{
		Port:          option.Port,
		name:          "Multicast",
		transportType: DirectType,
		logger:        log.NewLoggerEntry("transport-multicast-udp"),
	}

	recvConn, err := t.listen("udp")
	if err != nil {
		return nil, err
	}
	t.conn = recvConn
	t.udpWorker = newUdpTransportWorker(recvConn, recvConn, t.logger, option.NeedAck, option.AckTimeout)
	return t, nil
}

func (t *MulticastTransport) listen(proto string) (conn *net.UDPConn, err error) {
	t.logger.Debugf("Listening for peers on IP: %s port: %d Protocol=%s", MulticastAddress, t.Port, proto)
	addr, err := net.ResolveUDPAddr("udp", MulticastAddress+":"+strconv.Itoa(t.Port))
	if err != nil {
		t.logger.Errorf("Failed to resolve UDP address: %s", err)
		return
	}

	conn, err = net.ListenMulticastUDP(proto, nil, addr)
	if err != nil {
		t.logger.Errorf("Listen failed:%s", err)
		return
	}
	err = conn.SetReadBuffer(MaxDatagramSize)
	if err != nil {
		t.logger.Errorf("Failed to set read buffer: %s", err)
	}
	t.addr = addr
	return
}

func (t *MulticastTransport) Send(addr *TransportAddress, buf *buffer.Buffer, ignoreAck bool) (err error) {
	return t.udpWorker.Send(NewAddress(nil, t.addr), buf, ignoreAck)
}

func (t *MulticastTransport) OnMessage(handler TransportMessageHandler) {
	t.udpWorker.OnMessage(handler)
}

func (t *MulticastTransport) Name() string {
	return t.name
}

func (t *MulticastTransport) Type() TransportType {
	return t.transportType
}

func (t *MulticastTransport) IsTargetReachable(addr *TransportAddress) bool {
	// @todo
	return false
}

func (t *MulticastTransport) SetDeviceId(id deviceid.DeviceId) {
	if !t.DevId.Equal(id) {
		t.DevId = id
		t.logger = t.logger.WithField("deviceId", id)
		t.udpWorker.SetDeviceId(id)
	}
}

func (t *MulticastTransport) GetExternalIp(addr *TransportAddress) (net.IP, error) {
	return t.udpWorker.GetExternalIp(addr)
}

func (t *MulticastTransport) Close() {
	t.conn.Close()
	t.udpWorker.Close()
}
