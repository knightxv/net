package backpressure

import (
	"fmt"
	"sync"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/message"
)

type NopBPController struct {
	LocalDeviceId  string
	RemoteDeviceId string
	dport          string
	onSendMessage  func(msg *message.Message)
	onRecvMessage  func(msg *message.Message)
	isClosed       bool
	cbClosed       func()
	lock           sync.Mutex
	lastAliveTime  time.Time
	logger         *log.Entry
}

func newNopBPController(localDeviceId, remoteDeviceId, dport string, logger *log.Entry) *NopBPController {
	ctrl := &NopBPController{
		LocalDeviceId:  localDeviceId,
		RemoteDeviceId: remoteDeviceId,
		dport:          dport,
		lastAliveTime:  time.Now(),
	}

	ctrl.logger = logger.WithField("bpctl(nop)", fmt.Sprintf("[ %s<->%s:%s ]", localDeviceId, remoteDeviceId, dport))
	ctrl.logger.Info("bp controller created")
	return ctrl
}

func (c *NopBPController) LastAlive() time.Time {
	return c.lastAliveTime
}

func (c *NopBPController) SendEnqueue(msg *message.Message) error {
	if msg.SrcDevId != c.LocalDeviceId {
		return fmt.Errorf("bp controller src device id not match")
	}

	if msg.DstDevId != c.RemoteDeviceId {
		return fmt.Errorf("bp controller dest device id not match")
	}

	c.lastAliveTime = time.Now()
	if c.onSendMessage != nil {
		c.onSendMessage(msg)
		return nil
	} else {
		return fmt.Errorf("bp controller not set onSendMessage")
	}
}

func (c *NopBPController) RecvEnqueue(msg *message.Message) error {
	c.lastAliveTime = time.Now()
	if c.onRecvMessage != nil {
		c.onRecvMessage(msg)
		return nil
	} else {
		return fmt.Errorf("bp controller not set onRecvMessage")
	}
}

func (c *NopBPController) OnSendMessage(handler func(*message.Message)) {
	c.onSendMessage = handler
}

func (c *NopBPController) OnRecvMessage(handler func(*message.Message)) {
	c.onRecvMessage = handler
}

func (c *NopBPController) Close() {
	c.lock.Lock()
	if c.isClosed {
		c.lock.Unlock()
		return
	}
	c.isClosed = true
	cbClosedHandler := c.cbClosed
	c.lock.Unlock()

	if cbClosedHandler != nil {
		cbClosedHandler()
	}

	c.logger.Info("bp controller closed")
}

func (c *NopBPController) OnClose(cb func()) {
	c.lock.Lock()
	if !c.isClosed {
		c.cbClosed = cb
		c.lock.Unlock()
		return
	}
	c.lock.Unlock()

	if cb != nil {
		cb()
	}
}
