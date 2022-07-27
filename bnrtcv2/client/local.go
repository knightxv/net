package client

import (
	"errors"
	"fmt"

	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/message"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
)

type ILocalChannel interface {
	Send(src string, dst string, dport string, devid string, data []byte) error
	Multicast(src string, dst string, dport string, devid string, data []byte) error
	OnMessage(dport string, handler HandlerFunc) error
	OffMessage(dport string, handler HandlerFunc) error
	OnClosed(name string, handler func()) error
	Close()
}

type LocalClient struct {
	name              string
	MC                *MessageChannel
	cm                *ClientManager
	destoryCallback   *util.SafeMap //messageType[string]HandlerFunc
	sendDportHandlers *util.SafeMap //map[string]HandlerFunc
	recvDportHandlers *util.SafeMap //map[string]HandlerFunc
	IsClosed          bool
	logger            *log.Entry
}

type LocalChannel struct {
	dc *LocalClient
}

type LocalEventType struct {
	OPCode int
	DPort  string
	Func   HandlerFunc
}

var LocalClientId uint32 = 0

func NewLocalClient(clientName string, clientManager *ClientManager) *LocalClient {
	client := &LocalClient{
		name:              fmt.Sprintf("LocalClient-%s-%d", clientName, LocalClientId),
		recvDportHandlers: util.NewSafeMap(), // make(map[string]HandlerFunc),
		sendDportHandlers: util.NewSafeMap(), // make(map[string]HandlerFunc),
		destoryCallback:   util.NewSafeMap(), // make(map[string]HandlerFunc),
		cm:                clientManager,
		MC: &MessageChannel{
			Name:     fmt.Sprintf("LocalClient-%s-MCChannel-%d", clientName, LocalClientId),
			DataChan: make(chan IMsgEvt, 1024),
		},
		IsClosed: false,
		logger:   log.NewLoggerEntry("client-local").WithField("client", fmt.Sprintf("LocalClient-%d", wsClientId)),
	}
	LocalClientId += 1
	return client
}

func (c *LocalChannel) Send(src string, dst string, dport string, devid string, data []byte) error {
	return c.dc.Send(src, dst, dport, devid, data)
}

func (c *LocalChannel) Multicast(src string, dst string, dport string, devid string, data []byte) error {
	return c.dc.Multicast(src, dst, dport, devid, data)
}

func (c *LocalChannel) Broadcast(src string, dport string, data []byte) error {
	return c.dc.Broadcast(src, dport, data)
}

func (c *LocalChannel) Sync(src string, dport string, data []byte) error {
	return c.dc.Sync(src, dport, data)
}

func (c *LocalChannel) OffMessage(dport string, callback HandlerFunc) error {
	return c.dc.OffMessage(dport, callback)
}

func (c *LocalChannel) OnMessage(dport string, callback HandlerFunc) error {
	return c.dc.OnMessage(dport, callback)
}

func (c *LocalChannel) OnClosed(name string, callback func()) error {
	return c.dc.OnClosed(name, callback)
}

func (c *LocalChannel) Close() {
	c.dc.Close()
}

func (c *LocalClient) Recv(msg *message.Message) {
	if msg != nil {
		select {
		case c.MC.DataChan <- (*DataMsgEvt)(msg):
		default:
		}
	}
}

func (c *LocalClient) checkDport(dport string) error {
	_, found := c.sendDportHandlers.Get(dport)
	if !found {
		err := c.cm.BindDport(dport, c, DirectionSend)
		if err != nil {
			return fmt.Errorf("dport %s is already bound", dport)
		}
		c.sendDportHandlers.Set(dport, nil)
	}

	return nil
}

func (c *LocalClient) Send(src string, dst string, dport string, devid string, data []byte) error {
	err := c.checkDport(dport)
	if err != nil {
		return err
	}
	return c.cm.Send(src, dst, dport, devid, buffer.FromBytes(data))
}

func (c *LocalClient) Multicast(src string, dst string, dport string, devid string, data []byte) error {
	err := c.checkDport(dport)
	if err != nil {
		return err
	}
	return c.cm.Multicast(src, dst, dport, devid, buffer.FromBytes(data))
}

func (c *LocalClient) Broadcast(src string, dport string, data []byte) error {
	err := c.checkDport(dport)
	if err != nil {
		return err
	}
	return c.cm.Broadcast(src, dport, buffer.FromBytes(data))
}

func (c *LocalClient) Sync(src string, dport string, data []byte) error {
	err := c.checkDport(dport)
	if err != nil {
		return err
	}
	return c.cm.Sync(src, dport, buffer.FromBytes(data))
}

func (c *LocalClient) OnMessage(dport string, callback HandlerFunc) error {
	err := c.cm.BindDport(dport, c, DirectionReceive)
	if err != nil {
		c.logger.Errorf("%s Handle OnChanData BindDport failed %s", c.name, err)
		return err
	}
	c.recvDportHandlers.Set(dport, callback)
	return nil
}

func (c *LocalClient) OffMessage(dport string, callback HandlerFunc) error {
	err := c.cm.UnbindDPort(dport, c, DirectionReceive)
	if err != nil {
		c.logger.Errorf("%s Handle OnChanData UnbindDPort failed %s", c.name, err)
		return err
	}
	c.recvDportHandlers.Delete(dport)
	return nil
}

func (c *LocalClient) OnClosed(name string, callback func()) error {
	_, found := c.destoryCallback.Get(name)
	if found {
		return errors.New("the closed callback is register")
	}
	c.destoryCallback.Set(name, callback)
	return nil
}

func (c *LocalClient) Name() string {
	return c.name
}

func (c *LocalClient) GetChannel() ILocalChannel {
	return &LocalChannel{dc: c}
}

func (c *LocalClient) HandleMCData() {
	for _evt := range c.MC.DataChan {
		t := (*message.Message)(_evt.(*DataMsgEvt))
		c.logger.Debugf("%s Handle MCData %v", c.name, t)
		dPort := t.Dport
		handler, found := c.recvDportHandlers.Get(dPort)
		if found && handler != nil {
			handler.(HandlerFunc)(t)
		} else {
			c.logger.Debugf("%s Handle BPData dport %s not found", c.name, t.Dport)
		}
	}
}

func (c *LocalClient) Start() {
	go c.HandleMCData()
}

func (c *LocalClient) Close() {
	if c.IsClosed {
		return
	}

	for _recvDport := range c.recvDportHandlers.Items() {
		recvDport := _recvDport.(string)
		_ = c.cm.UnbindDPort(recvDport, c, DirectionReceive)
	}

	for _sendDport := range c.sendDportHandlers.Items() {
		sendDport := _sendDport.(string)
		_ = c.cm.UnbindDPort(sendDport, c, DirectionSend)
	}

	for _, _callback := range c.destoryCallback.Items() {
		callback := _callback.(HandlerFunc)
		if callback != nil {
			callback(nil)
		}
	}
	c.MC.Close()
	c.IsClosed = true
}
