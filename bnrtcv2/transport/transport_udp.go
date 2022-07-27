package transport

import (
	"net"
	"strconv"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/buffer"
)

type UdpTransportOption struct {
	Address    string
	Port       int
	NeedAck    bool
	AckTimeout time.Duration
}

type UdpTransport struct {
	DevId         deviceid.DeviceId
	Address       string
	Port          int
	name          string
	transportType TransportType
	udpWorker     *UdpTransportWorker
	conn          *net.UDPConn
	logger        *log.Entry
}

func NewUdpTransport(option UdpTransportOption) (*UdpTransport, error) {
	t := &UdpTransport{
		Address:       option.Address,
		Port:          option.Port,
		name:          "UDP",
		transportType: DirectType,
		logger:        log.NewLoggerEntry("transport-udp"),
	}

	conn, err := t.listen("udp")
	if err != nil {
		return nil, err
	}

	t.Port = conn.LocalAddr().(*net.UDPAddr).Port
	t.udpWorker = newUdpTransportWorker(conn, conn, t.logger, option.NeedAck, option.AckTimeout)
	t.conn = conn
	return t, nil
}

func (t *UdpTransport) listen(proto string) (conn *net.UDPConn, err error) {
	t.logger.Debugf("Listening for peers on IP: %s port: %d Protocol=%s", t.Address, t.Port, proto)
	listener, err := net.ListenPacket(proto, t.Address+":"+strconv.Itoa(t.Port))
	if err != nil {
		t.logger.Debugf("Listen failed:%s", err)
		return
	}
	conn = listener.(*net.UDPConn)
	return
}

func (t *UdpTransport) Send(addr *TransportAddress, buf *buffer.Buffer, ignoreAck bool) (err error) {
	return t.udpWorker.Send(addr, buf, ignoreAck)
}

func (t *UdpTransport) OnMessage(handler TransportMessageHandler) {
	t.udpWorker.OnMessage(handler)
}

func (t *UdpTransport) Name() string {
	return t.name
}

func (t *UdpTransport) Type() TransportType {
	return t.transportType
}

func (t *UdpTransport) IsTargetReachable(addr *TransportAddress) bool {
	if t.udpWorker.IsConnected(addr.DeviceId) {
		return true
	}

	if addr.HasIp() {
		if addr.DeviceId.IsTopNetwork() { //top network is always reachable
			return true
		}

		if addr.IsPublic() || addr.IsLocal() {
			return true
		}

		if addr.DeviceId.IsEmpty() {
			return true // @fixme: if not devid, either it's reachable or not, just give a try
		}
	}
	return false
}

func (t *UdpTransport) SetDeviceId(id deviceid.DeviceId) {
	if !t.DevId.Equal(id) {
		t.DevId = id
		t.logger = t.logger.WithField("deviceId", id)
		t.udpWorker.SetDeviceId(id)
	}
}

func (t *UdpTransport) GetExternalIp(addr *TransportAddress) (net.IP, error) {
	return t.udpWorker.GetExternalIp(addr)
}

func (t *UdpTransport) SetExternalIp(ip net.IP) {

}

func (t *UdpTransport) Close() {
	t.conn.Close()
	t.udpWorker.Close()
}
