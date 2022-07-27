package transport

import (
	"fmt"
	"net"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/buffer"
)

const SignalVersionMagic uint32 = 0x18273645

type SignalTransport struct {
	DevId             deviceid.DeviceId
	name              string
	transportType     TransportType
	manager           *Bnrtc1SignalManager
	onMessageCallback TransportMessageHandler
	logger            *log.Entry
}

func NewSignalTransport(manager *Bnrtc1SignalManager) (*SignalTransport, error) {
	if manager == nil {
		return nil, fmt.Errorf("nil peerService")
	}

	transport := &SignalTransport{
		name:          "signal",
		transportType: DirectAndForwardType,
		manager:       manager,
		logger:        log.NewLoggerEntry("transport-signal"),
	}

	manager.OnConnection(func(conn ISignalConnection) {
		conn.OnMessage(func(data []byte) {
			transport.onMessage(buffer.FromBytes(data))
		})
	})
	return transport, nil
}

func (t *SignalTransport) Name() string {
	return t.name
}

func (t *SignalTransport) Type() TransportType {
	return t.transportType
}

func (t *SignalTransport) IsTargetReachable(addr *TransportAddress) bool {
	return t.manager.IsConnected(addr.DeviceId)
}

func (t *SignalTransport) SetDeviceId(id deviceid.DeviceId) {
	if !t.DevId.Equal(id) {
		t.DevId = id
		t.logger = t.logger.WithField("deviceId", id)
	}
}

func (t *SignalTransport) Send(addr *TransportAddress, buf *buffer.Buffer, ignoreAck bool) error {
	conn, err := t.manager.GetConnection(addr.DeviceId)
	if err != nil {
		return fmt.Errorf("get connection %s failed", addr)
	}
	buf.Put(t.DevId)
	buf.PutU32(SignalVersionMagic)

	return conn.Send(buf.Data())
}

func (t *SignalTransport) OnMessage(handler TransportMessageHandler) {
	t.onMessageCallback = handler
}

func (t *SignalTransport) onMessage(buf *buffer.Buffer) {
	magic := buf.PullU32()
	if magic != SignalVersionMagic {
		return
	}

	devid, err := buf.PullWithError()
	if err != nil {
		return
	}

	handler := t.onMessageCallback
	if handler != nil {
		handler(NewAddress(devid, nil), buf)
	}
}

func (t *SignalTransport) Close() {
	t.onMessageCallback = nil
}

func (t *SignalTransport) GetExternalIp(addr *TransportAddress) (net.IP, error) {
	if !addr.HasIp() {
		addr = t.manager.GetSignalServerAddress()
	}
	if addr != nil {
		return t.manager.GetExternalIp(addr.Addr.String())
	}
	return nil, fmt.Errorf("no signal server address")
}

func (t *SignalTransport) SetExternalIp(ip net.IP) {
	t.manager.SetExternalIp(ip)
}
