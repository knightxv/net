package client

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/message"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
)

type WsEvent uint8

const (
	CLOSE WsEvent = 0
)

type WsClient struct {
	name              string
	MC                *MessageChannel
	WSC               *WSChannel
	conn              *websocket.Conn
	cm                *ClientManager
	sendDportHandlers *util.SafeMap // map[string]HandlerFunc
	recvDportHandlers *util.SafeMap // map[string]HandlerFunc
	closeCallback     *util.SafeMap // map[string]func()
	isClosed          bool
	logger            *log.Entry
}

type WSChannel struct {
	Name      string
	DataChan  chan []byte
	EventChan chan WsEvent
	IsClosed  bool
}

var wsClientId uint32 = 0

func NewWsClient(conn *websocket.Conn, clientManager *ClientManager) *WsClient {
	curClientId := atomic.AddUint32(&wsClientId, 1)
	name := fmt.Sprintf("WsClient-%d", curClientId)
	client := &WsClient{
		conn:              conn,
		name:              name,
		cm:                clientManager,
		sendDportHandlers: util.NewSafeMap(), //make(map[string]HandlerFunc),
		recvDportHandlers: util.NewSafeMap(), //make(map[string]HandlerFunc),
		closeCallback:     util.NewSafeMap(), //make(map[string]func()),
		MC: &MessageChannel{
			DataChan: make(chan IMsgEvt, 1024),
			IsClosed: false,
		},
		WSC: &WSChannel{
			DataChan:  make(chan []byte, 1024),
			EventChan: make(chan WsEvent),
		},
		isClosed: false,
		logger:   log.NewLoggerEntry("client-ws").WithField("client", name),
	}

	client.logger.Infof("new ws client %s", client.Name())
	return client
}

func (wc *WSChannel) Send(wsMessage []byte) {
	wc.DataChan <- wsMessage
}

func (wc *WSChannel) Close() {
	wc.EventChan <- CLOSE
}

func (c *WsClient) sendWsMessage(messageType int, data []byte, closeOnError bool) {
	err := c.conn.WriteMessage(messageType, data)

	if err != nil {
		c.logger.Errorf("send websocket message failed %s", err)
		if closeOnError {
			c.conn.Close()
		}
	}
}

func (c *WsClient) HandleMCData() {
	for _evt := range c.MC.DataChan {
		switch evt := _evt.(type) {
		case *CloseMsgEvt:
			c.logger.Debugln("websocket closed!!!")
			// c.sendWsMessage(websocket.CloseMessage, []byte{}, false)
			// c.conn.Close()
		case *DataMsgEvt:
			c.logger.Debugf("recv bp data %s(%s)->%s(%s):%s, isSync(%t), data(%s)", evt.SrcAddr, evt.SrcDevId, evt.DstAddr, evt.DstDevId, evt.Dport, evt.Sync, util.Truncate(evt.Buffer.Data(), 128))
			c.sendWsMessage(websocket.BinaryMessage, evt.EncodeData(), true)
		case *AckMsgEvt:
			c.logger.Debugf("send protocol ack, dport(%s), msgId(%d), status(%d)", evt.Dport, evt.MsgId, evt.StatusCode)
			c.sendWsMessage(websocket.BinaryMessage, evt.EncodeData(), true)
		}
	}
}

func (c *WsClient) HandleWSData() {
	for messageData := range c.WSC.DataChan {
		c.handleWsMessage(messageData)
	}
}

func (c *WsClient) HandleWSEvent() {
	for event := range c.WSC.EventChan {
		c.logger.Debugf("Handle WSChannel event %d", event)
		switch event {
		case CLOSE:
			data := []byte{uint8(MessageTypeClose)}
			c.WSC.DataChan <- data
		}
	}
}

func (c *WsClient) Start() {
	go c.HandleWSData()
	go c.HandleMCData()
	go c.HandleWSEvent()
}

func (c *WsClient) Close() {
	c.WSC.Close()
}

func (c *WsClient) close() {
	if c.isClosed {
		return
	}
	c.isClosed = true

	for _recvDport := range c.recvDportHandlers.Items() {
		recvDport := _recvDport.(string)
		_ = c.cm.UnbindDPort(recvDport, c, DirectionReceive)
	}

	for _sendDport := range c.sendDportHandlers.Items() {
		sendDport := _sendDport.(string)
		_ = c.cm.UnbindDPort(sendDport, c, DirectionSend)
	}

	for _, _callback := range c.closeCallback.Items() {
		callback := _callback.(func())
		if callback != nil {
			callback()
		}
	}

	c.MC.Send(&CloseMsgEvt{})
	c.MC.Close()
	close(c.WSC.DataChan)
	close(c.WSC.EventChan)
}

func (c *WsClient) Name() string {
	return c.name
}

func (c *WsClient) GetChannel() *WSChannel {
	return c.WSC
}

func (c *WsClient) Recv(msg *message.Message) {
	c.MC.Send((*DataMsgEvt)(msg.Copy()))
}

func (c *WsClient) addDPort(dport string) {
	c.recvDportHandlers.Set(dport, nil)
	c.logger.Debugf("add dport %s", dport)
}

func (c *WsClient) deleteDport(dport string) {
	c.recvDportHandlers.Delete(dport)
	c.logger.Debugf("delete dport %s", dport)
}

func (c *WsClient) handleWsMessage(data []byte) {
	wsData := DecodeWSData(buffer.FromBytes(data))
	if wsData == nil {
		c.logger.Errorf("decode ws data failed")
		return
	}
	c.logger.Debugf("receive ws data type(%s)", wsData.mt)
	if wsData.mt == MessageTypeClose {
		c.close()
		return
	} else {
		msg := wsData.msg
		c.logger.Debugf("receive ws data type(%s) %s->%s(%s):%s, msgId(%d), data(%s)", wsData.mt, msg.SrcAddr,
			msg.DstAddr, msg.DstDevId, msg.Dport, wsData.msgId, util.Truncate(msg.Buffer.Data(), 128))
		switch wsData.mt {
		case MessageTypeData:
			fallthrough
		case MessageTypeBroadcast:
			fallthrough
		case MessageTypeMulticast:
			fallthrough
		case MessageTypeSync:
			fallthrough
		case MessageTypeNeighborCast:
			err := c.data(wsData)
			if err != nil {
				c.logger.Errorf("handle ws Data %s->%s:%s, data(%s) error(%s)", msg.SrcAddr, msg.DstAddr, msg.Dport, util.Truncate(msg.Buffer.Data(), 64), err)
			}
		case MessageTypeBindDport:
			err := c.bindDport(msg.Dport, wsData.msgId)
			if err != nil {
				c.logger.Errorf("handle ws BindDport dport(%s), data(%s) error(%s)", msg.Dport, util.Truncate(msg.Buffer.Data(), 64), err)
			}
		case MessageTypeUnbindDport:
			err := c.unbindDport(msg.Dport, wsData.msgId)
			if err != nil {
				c.logger.Errorf("handle ws UnBindDport dport(%s), data(%s) error(%s)", msg.Dport, util.Truncate(msg.Buffer.Data(), 64), err)
			}
		case MessageTypeClose:
			c.close()
		}
	}
}

func (c *WsClient) bindDport(dport string, msgId uint32) error {
	err := c.cm.BindDport(dport, c, DirectionReceive)
	if err != nil {
		c.MC.Send(&AckMsgEvt{Dport: dport, MsgId: msgId, StatusCode: evt_dport_bound})
		return err
	}
	c.MC.Send(&AckMsgEvt{Dport: dport, MsgId: msgId, StatusCode: evt_success})
	c.addDPort(dport)
	return nil
}

func (c *WsClient) unbindDport(dport string, msgId uint32) error {
	err := c.cm.UnbindDPort(dport, c, DirectionReceive)
	if err != nil {
		c.MC.Send(&AckMsgEvt{Dport: dport, MsgId: msgId, StatusCode: evt_dport_unbound})
		return err
	}
	c.MC.Send(&AckMsgEvt{Dport: dport, MsgId: msgId, StatusCode: evt_success})
	c.deleteDport(dport)
	return nil
}

func (c *WsClient) data(data *WSData) error {
	msg := data.msg
	err := c.checkDport(msg.Dport)
	if err != nil {
		c.logger.Errorf("bindDport send dport %s failed %s", msg.Dport, err)
		c.MC.Send(&AckMsgEvt{Dport: msg.Dport, MsgId: data.msgId, StatusCode: evt_dport_bound})
		return err
	}

	switch data.mt {
	case MessageTypeData:
		err = c.cm.Send(msg.SrcAddr, msg.DstAddr, msg.Dport, msg.DstDevId, msg.Buffer)
	case MessageTypeMulticast:
		err = c.cm.Multicast(msg.SrcAddr, msg.DstAddr, msg.Dport, msg.DstDevId, msg.Buffer)
	case MessageTypeBroadcast:
		err = c.cm.Broadcast(msg.SrcAddr, msg.Dport, msg.Buffer)
	case MessageTypeSync:
		err = c.cm.Sync(msg.SrcAddr, msg.Dport, msg.Buffer)
	case MessageTypeNeighborCast:
		err = c.cm.NeighborCast(int(data.neighbors), msg.SrcAddr, msg.Dport, msg.Buffer)
	}

	if err != nil {
		c.MC.Send(&AckMsgEvt{Dport: msg.Dport, MsgId: data.msgId, StatusCode: evt_send_failed})
		return err
	}
	c.MC.Send(&AckMsgEvt{Dport: msg.Dport, MsgId: data.msgId, StatusCode: evt_success})
	return nil
}

func (c *WsClient) checkDport(dport string) error {
	_, found := c.sendDportHandlers.Get(dport)
	if !found {
		err := c.cm.BindDport(dport, c, DirectionSend)
		if err != nil {
			return errors.New("this port is already bound")
		}
		c.sendDportHandlers.Set(dport, nil)
	}

	return nil
}
