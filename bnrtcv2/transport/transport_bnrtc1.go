package transport

import (
	"fmt"
	"net"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/buffer"
)

const Bnrtc1VersionMagic uint32 = 0x18273645

type Bnrtc1Transport struct {
	DevId             deviceid.DeviceId
	name              string
	transportType     TransportType
	manager           *Bnrtc1ConnectionManager
	onMessageCallback TransportMessageHandler
	logger            *log.Entry
}

func NewBnrtc1Transport(manager *Bnrtc1ConnectionManager) (*Bnrtc1Transport, error) {
	if manager == nil {
		return nil, fmt.Errorf("nil peerService")
	}

	transport := &Bnrtc1Transport{
		name:          "bnrtc1",
		manager:       manager,
		transportType: DirectType,
		logger:        log.NewLoggerEntry("transport-bnrtc1"),
	}

	manager.OnConnection(func(conn IConnection) {
		conn.OnMessage(func(data []byte) {
			transport.onMessage(buffer.FromBytes(data))
		})
	})
	return transport, nil
}

func (t *Bnrtc1Transport) Name() string {
	return t.name
}

func (t *Bnrtc1Transport) Type() TransportType {
	return t.transportType
}

func (t *Bnrtc1Transport) IsTargetReachable(addr *TransportAddress) bool {
	return t.manager.IsConnected(addr.DeviceId)
}

func (t *Bnrtc1Transport) SetDeviceId(id deviceid.DeviceId) {
	if !t.DevId.Equal(id) {
		t.DevId = id
		t.logger = t.logger.WithField("deviceId", id)
	}
}

func (t *Bnrtc1Transport) Send(addr *TransportAddress, buf *buffer.Buffer, ignoreAck bool) error {
	conn, err := t.manager.GetConnection(addr.DeviceId)
	if err != nil {
		return fmt.Errorf("connect %s failed", addr)
	}
	buf.Put(t.DevId)
	buf.PutU32(Bnrtc1VersionMagic)

	return conn.Send(buf.Data())
}

func (t *Bnrtc1Transport) OnMessage(handler TransportMessageHandler) {
	t.onMessageCallback = handler
}

func (t *Bnrtc1Transport) onMessage(buf *buffer.Buffer) {
	magic := buf.PullU32()
	if magic != Bnrtc1VersionMagic {
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

func (t *Bnrtc1Transport) Close() {
	t.onMessageCallback = nil
}

func (t *Bnrtc1Transport) GetExternalIp(addr *TransportAddress) (net.IP, error) {
	return nil, fmt.Errorf("not implemented")
}

func (t *Bnrtc1Transport) SetExternalIp(ip net.IP) {

}
