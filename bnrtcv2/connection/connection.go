package connection

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/guabee/bnrtc/bnrtcv1"
	"github.com/guabee/bnrtc/bnrtcv2/log"
)

const Bnrtc1Protocol = "bnrtc1:"

type CloseHandler func()
type NameChangedHandler func(oldName, newName string)

type INatDuplex interface {
	SendBytes(data []byte) error
	Close() error
	BindHooks(cbs bnrtcv1.NatDuplexHooks)
	GetUrl() string
	GetRemoteNatName() string
}
type NatduplexHook struct {
	conn *Connection
}

func (hook *NatduplexHook) OnMessage(data []byte, isString bool) {
	if !isString {
		hook.conn.HandleOnMessage(data)
	}
}

func (hook *NatduplexHook) OnRemoteNameChanged(old, new string) {
	hook.conn.HandleRemoteNameChanged(old, new)
}

func (hook *NatduplexHook) OnClose() {
	hook.conn.HandleOnClose()
}

type Connection struct {
	name                 string
	Url                  string
	LastDataTime         time.Time
	NatDuplex            INatDuplex
	onCloseHandlers      []CloseHandler
	onMessageHandler     func([]byte)
	onNameChangedHandler NameChangedHandler
	onAliveHandler       func()
	mu                   sync.RWMutex
	isClosed             bool
	isHooked             bool
	logger               *log.Entry
}

var connectionLogger = log.NewLoggerEntry("connection")

func NewConnection(natDuplex INatDuplex) *Connection {
	conn := &Connection{
		name:         natDuplex.GetRemoteNatName(),
		NatDuplex:    natDuplex,
		LastDataTime: time.Now(),
		isClosed:     false,
	}
	url := natDuplex.GetUrl()
	if strings.HasPrefix(url, Bnrtc1Protocol) {
		conn.Url = url
	}

	conn.logger = connectionLogger.WithField("connection", conn.name)
	return conn
}

func (conn *Connection) addHook() {
	if conn.isHooked {
		return
	}

	hook := &NatduplexHook{
		conn: conn,
	}

	conn.NatDuplex.BindHooks(hook)
	conn.logger.Debugf("Connection %s bind hook", conn.name)

	conn.isHooked = true
}

func (conn *Connection) Name() string {
	return conn.name
}

func (conn *Connection) GetPeerAddr() *net.UDPAddr {
	if conn.Url == "" {
		return nil
	}
	addr, _ := net.ResolveUDPAddr("udp", strings.Replace(conn.Url, Bnrtc1Protocol, "", 1))
	return addr
}

func (conn *Connection) HandleOnMessage(data []byte) {
	if conn.isClosed {
		conn.logger.Errorf("Connection %s recv message when closed", conn.name)
		return
	}

	conn.LastDataTime = time.Now()
	if conn.onAliveHandler != nil {
		conn.onAliveHandler()
	}
	handler := conn.onMessageHandler
	if handler == nil {
		conn.logger.Errorf("Connection %s has no message handler", conn.name)
		return
	}
	conn.logger.Debugf("Connection %s recv message len %d", conn.name, len(data))
	handler(data)
}

func (conn *Connection) OnAlive(f func()) {
	conn.onAliveHandler = f
}

func (conn *Connection) OnMessage(f func([]byte)) {
	conn.mu.Lock()
	conn.onMessageHandler = f
	conn.addHook()
	conn.mu.Unlock()
}

func (conn *Connection) HandleRemoteNameChanged(oldName, newName string) {
	if conn.isClosed {
		conn.logger.Errorf("Connection %s is closed", conn.name)
		return
	}

	if newName == "" {
		return
	}

	if conn.name != newName {
		conn.name = newName
		conn.logger.Infof("Connection %s name changed from %s to %s", conn.name, oldName, newName)
		handler := conn.onNameChangedHandler
		if handler != nil {
			handler(conn.name, newName)
		}
	}
}

func (conn *Connection) OnNameChanged(f NameChangedHandler) {
	conn.mu.Lock()
	conn.onNameChangedHandler = f
	conn.mu.Unlock()
}

func (conn *Connection) OnClose(f CloseHandler) {
	conn.mu.Lock()
	conn.logger.Debugf("Connection %s add close handler", conn.name)
	if conn.isClosed {
		if f != nil {
			f()
		}
	} else {
		conn.onCloseHandlers = append(conn.onCloseHandlers, f)
	}
	conn.mu.Unlock()
}

func (conn *Connection) HandleOnClose() {
	conn.mu.RLock()
	handlers := conn.onCloseHandlers

	if !conn.isClosed {
		conn.logger.Debugf("Connection %s handler close", conn.name)
		conn.isClosed = true
		for _, handler := range handlers {
			if handler != nil {
				handler()
			}
		}
	}
	conn.mu.RUnlock()
}

func (conn *Connection) Close() error {
	conn.logger.Debugf("Connection %s close", conn.name)
	err := conn.NatDuplex.Close()
	conn.HandleOnClose()
	return err
}

func (conn *Connection) Send(data []byte) error {
	if conn.isClosed {
		conn.logger.Errorf("Connection %s send message when closed", conn.name)
		return fmt.Errorf("Connection %s send message when closed", conn.name)
	}

	conn.LastDataTime = time.Now()
	if conn.onAliveHandler != nil {
		conn.onAliveHandler()
	}

	conn.logger.Debugf("Connection %s send message len %d", conn.name, len(data))
	return conn.NatDuplex.SendBytes(data)
}
