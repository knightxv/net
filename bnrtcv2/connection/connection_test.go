package connection_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/guabee/bnrtc/bnrtcv1"
	"github.com/guabee/bnrtc/bnrtcv2/connection"
	"github.com/guabee/bnrtc/bnrtcv2/message"
	"github.com/guabee/bnrtc/buffer"
)

type MockConnectionNatDuplex struct {
	cbs bnrtcv1.NatDuplexHooks
}

func (d *MockConnectionNatDuplex) SendBytes(data []byte) error {
	d.cbs.OnMessage(data, false)
	return nil
}
func (d *MockConnectionNatDuplex) Close() error {
	d.cbs.OnClose()
	return nil
}
func (d *MockConnectionNatDuplex) BindHooks(cbs bnrtcv1.NatDuplexHooks) {
	d.cbs = cbs
}

func (d *MockConnectionNatDuplex) GetUrl() string {
	return ""
}

func (d *MockConnectionNatDuplex) GetRemoteNatName() string {
	return "EAXDkaEJgWKMM61KYz2dYU1RfuxbB8Ma"
}

func TestConnection(t *testing.T) {
	conn := connection.NewConnection(&MockConnectionNatDuplex{})
	conn.OnMessage(func(data []byte) {

		now_time := time.Now()
		t.Log("Onmessage 1", now_time)
		t.Log("Onmessage 1", conn.LastDataTime)
		if now_time == conn.LastDataTime {
			t.Error("time error")
		}
		e := []byte{1, 2, 3}
		msg := message.FromBytes(data)
		if !bytes.Equal(msg.Buffer.Data(), e) {
			t.Error("data error1")
		}
		t.Log("Onmessage 1")
	})

	conn.OnMessage(func(data []byte) {
		now_time := time.Now()
		if now_time != conn.LastDataTime {
			t.Error("time error2")
		}
		e := []byte{1, 2, 3}
		msg := message.FromBytes(data)
		if !bytes.Equal(msg.Buffer.Data(), e) {
			t.Error("data error2")
		}
		t.Log("Onmessage 2")
	})

	conn.OnClose(func() {
		t.Log("Onclose 1")
	})

	conn.OnClose(func() {
		t.Log("Onclose 2")
	})

	bytes := []byte{1, 2, 3}
	_ = conn.Send((&message.Message{DstAddr: "EAXDkaEJgWKMM61KYz2dYU1RfuxbB8Ma", Dport: "dport", Buffer: buffer.FromBytes(bytes)}).GetBytes())
	conn.Close()
}
