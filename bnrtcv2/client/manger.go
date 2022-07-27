package client

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/bnrtcv2/message"
	"github.com/guabee/bnrtc/bnrtcv2/server"
	"github.com/guabee/bnrtc/bnrtcv2/server/iservice"
	"github.com/guabee/bnrtc/buffer"
)

//js-send-message-type

type Direction uint8

const (
	DirectionSend    Direction = 0
	DirectionReceive Direction = 1
)

type HandlerFunc = func(msg *message.Message)

type IClient interface {
	Name() string
	Recv(msg *message.Message)
	Start()
}

type IMsgEvt interface {
	EncodeData() []byte
}

type MessageChannel struct {
	Name     string
	DataChan chan IMsgEvt
	IsClosed bool
	Lock     sync.Mutex
}

func (mc *MessageChannel) Send(evt IMsgEvt) {
	mc.Lock.Lock()
	if !mc.IsClosed {
		mc.DataChan <- evt
	}
	mc.Lock.Unlock()
}

func (mc *MessageChannel) Close() {
	mc.Lock.Lock()
	close(mc.DataChan)
	mc.IsClosed = true
	mc.Lock.Unlock()
}

type INetController interface {
	Send(src, dst, dport, devid string, buf *buffer.Buffer) error
	SendToAll(src, dst, dport, devid string, buf *buffer.Buffer) error
	Sync(address, dport string, buf *buffer.Buffer) error
	Broadcast(src, dport string, buf *buffer.Buffer) error
	SendToNeighbors(num int, src, dport string, buf *buffer.Buffer) error
	OnMessage(dport string, f func(msg *message.Message))
	AddSendPort(dport string)
	RemoveSendPort(dport string)
}

type ClientManager struct {
	NetController   INetController
	sendClients     map[string]IClient
	recvClients     map[string]IClient
	sendClientsLock sync.Mutex
	recvClientsLock sync.Mutex
	logger          *log.Entry
}

func NewClientManger(netController INetController) *ClientManager {
	cm := &ClientManager{
		NetController: netController,
		sendClients:   make(map[string]IClient),
		recvClients:   make(map[string]IClient),
		logger:        log.NewLoggerEntry("client-manager"),
	}

	return cm
}

func (cm *ClientManager) BindDport(dport string, client IClient, direction Direction) error {
	if strings.HasPrefix(dport, "_") {
		return fmt.Errorf("dport '%s' with prefix '_' is reserved", dport)
	}

	if direction == DirectionSend {
		cm.sendClientsLock.Lock()
		defer cm.sendClientsLock.Unlock()
		_, found := cm.sendClients[dport]
		if found {
			return fmt.Errorf("send dport '%s' is already bound", dport)
		}
		cm.sendClients[dport] = client
		cm.NetController.AddSendPort(dport)
		cm.logger.Infof("bind send dport(%s) <-> client(%s)", dport, client.Name())
	} else if direction == DirectionReceive {
		cm.recvClientsLock.Lock()
		defer cm.recvClientsLock.Unlock()
		_, found := cm.recvClients[dport]
		if found {
			return fmt.Errorf("recv dport '%s' is already bound", dport)
		}
		cm.recvClients[dport] = client
		cm.NetController.OnMessage(dport, client.Recv)
		cm.logger.Infof("bind recv dport(%s) <-> client(%s)", dport, client.Name())
	}
	return nil
}

func (cm *ClientManager) UnbindDPort(dport string, c IClient, direction Direction) error {
	if direction == DirectionSend {
		cm.sendClientsLock.Lock()
		defer cm.sendClientsLock.Unlock()
		client, found := cm.sendClients[dport]
		if !found {
			return fmt.Errorf("send dport '%s' not bound yet", dport)
		}
		if client != c {
			return fmt.Errorf("send dport '%s' is bound by others", dport)
		}
		delete(cm.sendClients, dport)
		cm.NetController.RemoveSendPort(dport)
		cm.logger.Infof("unbind send dport(%s) <-> client(%s)", dport, client.Name())
	} else if direction == DirectionReceive {
		cm.recvClientsLock.Lock()
		defer cm.recvClientsLock.Unlock()
		client, found := cm.recvClients[dport]
		if !found {
			return fmt.Errorf("recv dport '%s' not bound yet", dport)
		}
		if client != c {
			return fmt.Errorf("recv dport '%s' is bound by others", dport)
		}
		delete(cm.recvClients, dport)
		cm.NetController.OnMessage(dport, nil)
		cm.logger.Infof("unbind recv dport(%s) <-> client(%s)", dport, client.Name())
	}
	return nil
}

func (cm *ClientManager) CreateWsClient(conn *websocket.Conn) *WsClient {
	mc := NewWsClient(conn, cm)
	mc.Start()
	return mc
}

func (cm *ClientManager) CreateWsChannel(conn *websocket.Conn) server.IWSChannel {
	mc := NewWsClient(conn, cm)
	mc.Start()
	return mc.GetChannel()
}

func (cm *ClientManager) CreateLocalClient(name string) *LocalClient {
	lc := NewLocalClient(name, cm)
	lc.Start()
	return lc
}

func (cm *ClientManager) CreateLocalChannel(name string) iservice.ILocalChannel {
	lc := NewLocalClient(name, cm)
	lc.Start()
	return lc.GetChannel()
}

func (cm *ClientManager) Send(src, dst, dport, devid string, buf *buffer.Buffer) error {
	return cm.NetController.Send(src, dst, dport, devid, buf)
}

func (cm *ClientManager) Multicast(src, dst, dport, devid string, buf *buffer.Buffer) error {
	return cm.NetController.SendToAll(src, dst, dport, devid, buf)
}

func (cm *ClientManager) Broadcast(src, dport string, buf *buffer.Buffer) error {
	return cm.NetController.Broadcast(src, dport, buf)
}

func (cm *ClientManager) Sync(src, dport string, buf *buffer.Buffer) error {
	return cm.NetController.Sync(src, dport, buf)
}

func (cm *ClientManager) NeighborCast(num int, src, dport string, buf *buffer.Buffer) error {
	return cm.NetController.SendToNeighbors(num, src, dport, buf)
}
