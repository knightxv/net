package backpressure_test

import (
	"testing"
	"time"

	bp "github.com/guabee/bnrtc/bnrtcv2/backpressure"
	"github.com/guabee/bnrtc/bnrtcv2/message"
	"github.com/guabee/bnrtc/bnrtcv2/net"
	"github.com/guabee/bnrtc/buffer"
)

type ConnMock struct {
	localId string
	peerId  string
	dport   string
	done    chan bool
	sendCnt int
	recvCnt int
	cap     int
	manager *bp.BPManager
}

func newConnMock(id string, peer string, dport string, t bp.BPCType, cap int) *ConnMock {
	return &ConnMock{
		localId: id,
		peerId:  peer,
		dport:   dport,
		done:    make(chan bool),
		cap:     cap,
		manager: bp.NewManager(
			bp.WithCapacity(uint32(cap)),
			bp.WithBpType(t),
			bp.WithDeviceId(id),
		),
	}
}

func (c *ConnMock) sendMock(msg *message.Message) {
	c.sendCnt++
	ctrl, _ := c.manager.GetController(c.peerId, c.dport, bp.BPControllerDirectionSend)
	_ = ctrl.RecvEnqueue(msg)
}

func (c *ConnMock) recvMock(msg *message.Message) {
	time.Sleep(100 * time.Microsecond)
	c.recvCnt++
	if c.recvCnt == c.cap {
		c.done <- true
	}
}

func makeConnPair(id1, id2, dport string, t bp.BPCType, cap int) (*ConnMock, *ConnMock) {
	conn1 := newConnMock(id1, id2, dport, t, cap)
	conn2 := newConnMock(id2, id1, dport, t, cap)

	conn1.manager.OnController(func(c net.IBPController) {
		c.OnSendMessage(conn1.sendMock)
		c.OnRecvMessage(conn1.recvMock)
	})

	conn2.manager.OnController(func(c net.IBPController) {
		c.OnSendMessage(conn2.sendMock)
		c.OnRecvMessage(conn2.recvMock)
	})
	return conn1, conn2
}

func testBackpressure(t *testing.T, bpt bp.BPCType) {
	cap := 1024
	dport := "dport"
	id1 := "1"
	id2 := "2"
	conn1, conn2 := makeConnPair(id1, id2, dport, bpt, cap)

	go (func() {
		i := 0
		for i < 2*cap {
			time.Sleep(50 * time.Microsecond)
			ctrl1, _ := conn1.manager.GetController(id2, dport, bp.BPControllerDirectionSend)
			ctrl2, _ := conn2.manager.GetController(id1, dport, bp.BPControllerDirectionSend)
			_ = ctrl1.SendEnqueue(message.NewMessage(dport, "", "", id1, id2, false, buffer.FromBytes([]byte("1"))))
			_ = ctrl2.SendEnqueue(message.NewMessage(dport, "", "", id2, id1, false, buffer.FromBytes([]byte("2"))))
			i++
			if i%30 == 0 {
				ctrl1.Close()
			} else if i%30 == 10 {
				ctrl2.Close()
			} else if i%30 == 20 {
				ctrl1.Close()
				ctrl2.Close()
			}
		}
	})()

	c1Done := false
	c2Done := false
	for {

		select {
		case <-conn1.done:
			c1Done = true
		case <-conn2.done:
			c2Done = true
		}
		if c1Done && c2Done {
			break
		}
	}
}

func TestCountBackpressure(t *testing.T) {
	testBackpressure(t, bp.BPCTypeCount)
}

func TestNopBackpressure(t *testing.T) {
	testBackpressure(t, bp.BPCTypeNop)
}
