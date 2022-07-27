package backpressure

/*
 throttle controller based on count
*/

import (
	"fmt"
	"sync"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/message"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
)

const (
	gapTimeBPInfoSend      = time.Millisecond * 50 // time_ms between each local bp info sending
	gapTimeBPInfoReq       = time.Millisecond * 50 // time_ms between each local bp info sending
	waitForBPinfoRespTime  = time.Second * 10
	recvOverLoadCountLimit = 10
)

type CountBPController struct {
	LocalDeviceId    string
	RemoteDeviceId   string
	dport            string
	chanUserDataSend chan *message.Message
	chanCtrlDataSend chan *message.Message
	chanUserDataRecv chan *message.Message
	readyToSendChan  chan bool
	readyToRecvChan  chan bool
	onSendMessage    func(msg *message.Message)
	onRecvMessage    func(msg *message.Message)
	wg               sync.WaitGroup
	logger           *log.Entry
	msgId            uint64
	currentId        uint64 // current id being proceeded

	// remote
	remoteSizeAvailable   uint32
	remoteCapacity        uint32
	hasRemoteBPInfo       bool // true on initial bp info received
	hasLocalBPInfoSent    bool // true on bp info sent
	waitForBPinfoRespChan chan struct{}
	remoteReadyOnce       sync.Once

	// callbacks
	closeRequested bool
	cbClosed       func()
	closeLock      sync.Mutex

	// strategy variables
	recvFrameCounter      uint64
	sendFrameCounter      uint64
	lastTimeBPInfoReqSent time.Time
	lastTimeBPInfoSent    time.Time
	countPerBPReq         uint32
	lastAliveTime         time.Time
	recvOverLoadCount     uint32
}

func newCountBPController(localDeviceId, remoteDeviceId, dport string, capRecv uint32, logger *log.Entry) *CountBPController {
	ctrl := &CountBPController{
		LocalDeviceId:         localDeviceId,
		RemoteDeviceId:        remoteDeviceId,
		dport:                 dport,
		wg:                    sync.WaitGroup{},
		lastAliveTime:         time.Now(),
		countPerBPReq:         capRecv / 10,
		waitForBPinfoRespChan: make(chan struct{}),
		chanCtrlDataSend:      make(chan *message.Message, 1024),
		chanUserDataRecv:      make(chan *message.Message, capRecv),
		readyToSendChan:       make(chan bool),
		readyToRecvChan:       make(chan bool),
	}

	ctrl.logger = logger.WithField("bpctl(count)", fmt.Sprintf("[ %s<->%s:%s ]", localDeviceId, remoteDeviceId, dport))
	util.GoWithWaitGroup(&ctrl.wg, ctrl.comsumeSendCtrlQueue)
	util.GoWithWaitGroup(&ctrl.wg, ctrl.comsumeRecvQueue)

	ctrl.logger.Infoln("bp controller created")

	return ctrl
}

func (c *CountBPController) LastAlive() time.Time {
	return c.lastAliveTime
}

func (c *CountBPController) waitForBPInfoResp() bool {
	t := time.NewTimer(time.Duration(waitForBPinfoRespTime))
	for {
		select {
		case <-t.C:
			return false
		case <-c.waitForBPinfoRespChan:
			t.Stop()
			return true
		}
	}
}

func (c *CountBPController) SendEnqueue(msg *message.Message) error {
	if c.closeRequested {
		return fmt.Errorf("controller is closing")
	}

	if msg.SrcDevId != c.LocalDeviceId {
		return fmt.Errorf("controller src device id not match")
	}

	if msg.DstDevId != c.RemoteDeviceId {
		return fmt.Errorf("controller dest device id not match")
	}

	if !c.hasRemoteBPInfo {
		c.sendBPInfoReq()
		ready := c.waitForBPInfoResp()
		if !ready {
			c.logger.Warnln("discard due to not ready")
			return fmt.Errorf("bp not ready")
		}
	}

	select {
	case c.chanUserDataSend <- msg:
		return nil
	default:
		return fmt.Errorf("bp queue is full")
	}
}

func (c *CountBPController) RecvEnqueue(msg *message.Message) error {
	if c.closeRequested {
		return fmt.Errorf("controller is closing")
	}

	f, err := DecodeBPFrame(msg.Buffer)
	if err != nil {
		c.logger.Errorf("error decodnig bp frame info with len= %d", msg.Buffer.Len())
		return err
	}

	c.lastAliveTime = time.Now()
	c.logger.Tracef("received msg type= %d", f.MsgType)
	switch f.MsgType {
	case MT_BPInfoReq:
		{
			c.logger.Debugln("received MT_BPInfoReq")
			c.sendLocalBPInfo(true)
		}
	case MT_BPInfoResp:
		{
			c.logger.Debugln("received MT_BPInfoResp")
			info, err := DecodePayloadBPInfo(f.Buf)
			if err != nil {
				c.logger.Errorf("error decodnig bp resp info with len= %d", f.Buf.Len())
				return err
			}
			// update remote bp info
			if !c.hasRemoteBPInfo {
				c.remoteReadyOnce.Do(func() {
					c.remoteCapacity = info.Capacity
					c.remoteSizeAvailable = info.SizeAvailable
					c.chanUserDataSend = make(chan *message.Message, c.remoteCapacity)
					util.GoWithWaitGroup(&c.wg, c.comsumeSendQueue)
					c.hasRemoteBPInfo = true
					close(c.waitForBPinfoRespChan)
					c.logger.Infof("bp ready")
				})
			} else {
				if c.msgId >= info.CurrentId {
					diff := uint32(c.msgId - info.CurrentId)
					c.remoteSizeAvailable = info.SizeAvailable - diff
				} else {
					c.remoteSizeAvailable = info.SizeAvailable
				}
			}
			c.logger.Debugf("current BP size %d, queue available %d", c.remoteSizeAvailable, c.remoteCapacity-uint32(len(c.chanUserDataSend)))
		}
	case MT_UserData:
		{
			// got use data message before bp info request is due to restart a new controller, just send local bp info
			if !c.hasLocalBPInfoSent {
				c.sendLocalBPInfo(true)
			}
			payload, err := DecodePayloadUserData(f.Buf)
			if err != nil {
				c.logger.Errorf("error decodnig userData with len= %d", f.Buf.Len())
				return err
			}
			remaining := cap(c.chanUserDataRecv) - len(c.chanUserDataRecv)
			if remaining > 0 {
				c.chanUserDataRecv <- msg
				c.currentId = payload.Id
				_cap := cap(c.chanUserDataRecv)
				availableRate := float32(remaining) / float32(_cap)
				if availableRate < 0.1 {
					c.sendLocalBPInfo(false)
				} else {
					c.recvOverLoadCount = 0
				}
			} else {
				c.sendLocalBPInfo(true) // stop send please, i can't process more
				c.recvOverLoadCount++
				if c.recvOverLoadCount > recvOverLoadCountLimit { // close the controller if recv queue is overload
					c.logger.Warnln("recv queue is overload, close the controller")
					c.Close()
				}
			}
		}
	case MT_BPClose:
		{
			c.logger.Info("close controller by remote")
			c.close()
		}
	}
	return nil
}

func (c *CountBPController) close() {
	c.closeLock.Lock()
	if c.closeRequested {
		c.closeLock.Unlock()
		return
	}
	c.closeRequested = true
	cbClosedHandler := c.cbClosed
	c.closeLock.Unlock()

	if cbClosedHandler != nil {
		cbClosedHandler()
	}

	if c.chanUserDataSend != nil {
		close(c.chanUserDataSend)
	}
	close(c.chanUserDataRecv)
	close(c.chanCtrlDataSend)
	c.wg.Wait()
	c.logger.Infoln("bp controller closed")
}

func (c *CountBPController) Close() {
	c.closeLock.Lock()
	if !c.closeRequested {
		c.sendBPClose()
	}
	c.closeLock.Unlock()

	c.close()
}

func (c *CountBPController) OnClose(cb func()) {
	c.closeLock.Lock()
	if !c.closeRequested {
		c.cbClosed = cb
		c.closeLock.Unlock()
		return
	}
	c.closeLock.Unlock()

	if cb != nil {
		cb()
	}
}

func (c *CountBPController) newUserDataFrame(buf *buffer.Buffer) *BPFrame {
	pld := &PayloadUserData{
		Id:  c.sendFrameCounter,
		Buf: buf,
	}
	payload := pld.Encode()
	f := &BPFrame{
		MsgType: MT_UserData,
		Buf:     payload,
	}
	return f
}

func (c *CountBPController) newBPInfoReqFrame() *BPFrame {
	f := &BPFrame{
		MsgType: MT_BPInfoReq,
		Buf:     buffer.NewBuffer(0, nil),
	}
	return f
}

func (c *CountBPController) newBPInfoRespFrame() *BPFrame {
	capacity := cap(c.chanUserDataRecv)
	sizeAvailable := capacity - len(c.chanUserDataRecv)
	info := &PayloadBPInfo{
		CurrentId:     c.currentId,
		SizeAvailable: uint32(sizeAvailable),
		Capacity:      uint32(capacity),
	}
	payload := info.Encode()
	f := &BPFrame{
		MsgType: MT_BPInfoResp,
		Buf:     payload,
	}
	return f
}

func (c *CountBPController) newBPCloseFrame() *BPFrame {
	f := &BPFrame{
		MsgType: MT_BPClose,
		Buf:     buffer.NewBuffer(0, nil),
	}
	return f
}

func (c *CountBPController) sendCtrlMessage(msg *message.Message) {
	c.chanCtrlDataSend <- msg
}

func (c *CountBPController) sendBPInfoReq() {
	if time.Since(c.lastTimeBPInfoReqSent) < gapTimeBPInfoReq {
		return
	}
	c.lastTimeBPInfoReqSent = time.Now()

	buf := c.newBPInfoReqFrame().Encode()
	c.sendCtrlMessage(message.NewMessage(c.dport, "", "", c.LocalDeviceId, c.RemoteDeviceId, false, buf))
	c.logger.Debugln("bp info req sent")
}

func (c *CountBPController) sendLocalBPInfo(force bool) {
	if !force && time.Since(c.lastTimeBPInfoSent) < gapTimeBPInfoSend {
		return
	}

	buf := c.newBPInfoRespFrame().Encode()
	c.sendCtrlMessage(message.NewMessage(c.dport, "", "", c.LocalDeviceId, c.RemoteDeviceId, false, buf))
	c.lastTimeBPInfoSent = time.Now()
	c.hasLocalBPInfoSent = true
	c.logger.Debugln("local bp info sent")
}

func (c *CountBPController) sendBPClose() {
	buf := c.newBPCloseFrame().Encode()
	c.sendCtrlMessage(message.NewMessage(c.dport, "", "", c.LocalDeviceId, c.RemoteDeviceId, false, buf))
	c.logger.Debugln("bp close sent")
}

func (c *CountBPController) OnRecvMessage(handler func(*message.Message)) {
	oldHandler := c.onRecvMessage
	c.onRecvMessage = handler
	if oldHandler == nil {
		close(c.readyToRecvChan)
	}
}

func (c *CountBPController) OnSendMessage(handler func(*message.Message)) {
	oldHandler := c.onSendMessage
	c.onSendMessage = handler
	if oldHandler == nil {
		close(c.readyToSendChan)
	}
}

func (c *CountBPController) comsumeSendQueue() {
	<-c.readyToSendChan
	c.logger.Debugln("comsumeSendQueue ready")
	for {
		if c.closeRequested {
			break
		}

		availableRate := float32(c.remoteSizeAvailable) / float32(c.remoteCapacity)
		if availableRate < 0.01 {
			time.Sleep(time.Millisecond * 1000)
		} else if availableRate < 0.05 {
			time.Sleep(time.Millisecond * 800)
		} else if availableRate < 0.1 {
			time.Sleep(time.Millisecond * 500)
		}
		msg, more := <-c.chanUserDataSend
		if !more {
			break
		}

		payload := c.newUserDataFrame(msg.Buffer)
		msg.Buffer = payload.Encode()
		c.lastAliveTime = time.Now()
		c.sendFrameCounter++
		c.onSendMessage(msg)
	}
}

func (c *CountBPController) comsumeSendCtrlQueue() {
	<-c.readyToSendChan
	c.logger.Debugln("comsumeSendCtrlQueue ready")

	for {
		if c.closeRequested {
			break
		}

		msg, more := <-c.chanCtrlDataSend
		if !more {
			break
		}

		c.onSendMessage(msg)
	}
}

func (c *CountBPController) comsumeRecvQueue() {
	<-c.readyToRecvChan
	c.logger.Debugln("comsumeRecvQueue ready")

	for {
		if c.closeRequested {
			break
		}
		msg, more := <-c.chanUserDataRecv
		if !more {
			break
		}
		c.recvFrameCounter++
		if c.recvFrameCounter%uint64(c.countPerBPReq) == 0 {
			c.sendLocalBPInfo(false)
		}
		c.onRecvMessage(msg)
	}
}
