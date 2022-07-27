package bnrtcv1

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/guabee/bnrtc/util"
)

type WsMessage struct {
	MessageType  MessageType
	Data         []byte
	CloseOnError bool
}

type Connection struct {
	conn           *websocket.Conn
	remoteName     string
	dataMsgHandler func(message []byte)
	ctrlMsgHandler func(conn *Connection, message []byte)
	closeHandlers  []func()
	msgChan        chan *WsMessage
}

func newConnection(conn *websocket.Conn, name string) *Connection {
	return &Connection{
		conn:          conn,
		remoteName:    name,
		closeHandlers: []func(){},
		msgChan:       make(chan *WsMessage, 10),
	}
}

func (c *Connection) Name() string {
	return c.remoteName
}

func (c *Connection) Send(data []byte) error {
	return c.SendDataMessage(data)
}

func (c *Connection) SendDataMessage(data []byte) error {
	dataMessage := NewDataMessage(data)
	dataMessageBytes, err := dataMessage.toBytes()
	if err != nil {
		return err
	}

	return c.sendWsMessage(MessageTypeData, dataMessageBytes, true)
}

func (c *Connection) SendCtrlMessage(data []byte) error {
	return c.sendWsMessage(MessageTypeCtrl, data, true)
}

func (c *Connection) sendWsMessage(messageType MessageType, data []byte, closeOnError bool) error {
	c.msgChan <- &WsMessage{
		MessageType:  messageType,
		Data:         data,
		CloseOnError: closeOnError,
	}
	return nil
}

func (c *Connection) recvWsMessage(data []byte) {
	messageHeader := &MessageHeader{}
	err := messageHeader.fromBytes(data)
	if err != nil {
		return
	}
	switch messageHeader.MsgType {
	case MessageTypeData:
		if c.dataMsgHandler != nil {
			dataMessage := &DataMessage{}
			err := dataMessage.fromBytes(data)
			if err == nil {
				c.dataMsgHandler(dataMessage.Data)
			}
		}
	case MessageTypeCtrl:
		if c.ctrlMsgHandler != nil {
			c.ctrlMsgHandler(c, data)
		}
	}
}

func (c *Connection) OnCtrlMessage(h func(conn *Connection, message []byte)) {
	c.ctrlMsgHandler = h
}

func (c *Connection) OnMessage(h func(message []byte)) {
	c.dataMsgHandler = h
}

func (c *Connection) SetRemoteName(name string) {
	c.remoteName = name
}

func (c *Connection) OnClose(h func()) {
	c.closeHandlers = append(c.closeHandlers, h)
}

func (c *Connection) onClose() {
	for _, h := range c.closeHandlers {
		h()
	}
}

func (c *Connection) recvMessageWorker() {
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		c.recvWsMessage(message)
	}
	c.Close()
}

func (c *Connection) sendMessageWorker() {
	for msg := range c.msgChan {
		err := c.conn.WriteMessage(websocket.BinaryMessage, msg.Data)
		if err != nil {
			if msg.CloseOnError {
				c.conn.Close()
				break
			}
		}

		if msg.MessageType == MessageTypeClose {
			break
		}
	}
}

func (c *Connection) Start() {
	go c.recvMessageWorker()
	go c.sendMessageWorker()
}

func (c *Connection) Close() {
	_ = c.sendWsMessage(MessageTypeClose, []byte{}, false)
	c.conn.Close()
	c.onClose()
}

type ConnectionManager struct {
	localNatName        string
	connections         *util.SafeLinkMap
	limit               int
	reqManager          *util.ReqRespManager
	offerHandler        func(offerInfo *OfferInfo) (res []byte, err error)
	onConnectionHandler func(conn *Connection)
	natRequestHandlers  map[string]func(conn *Connection, r *CtrlNatRequest)
}

func (m *ConnectionManager) AddNatRequestHandler(path string, hander func(conn *Connection, r *CtrlNatRequest)) {
	m.natRequestHandlers[path] = hander
}

func (m *ConnectionManager) SetOfferHandler(h func(offerInfo *OfferInfo) (res []byte, err error)) {
	m.offerHandler = h
}

func (m *ConnectionManager) OnConnection(h func(conn *Connection)) {
	m.onConnectionHandler = h
}

func (m *ConnectionManager) handleNatOfferRequest(conn *Connection, r *CtrlNatRequest) {
	var statusCode = 200
	var bytes []byte
	var err error
	if m.offerHandler == nil {
		err = errors.New("offer handler not set")
	} else {
		var offerInfo *OfferInfo
		offerInfo, err = ParseOfferInfo(r.PostForm)
		if err == nil {
			if !IsSameVersion(offerInfo.Version, Version) {
				err = fmt.Errorf("version not match, expected: %s, actual: %s", Version, offerInfo.Version)
			} else {
				bytes, err = m.offerHandler(offerInfo)
			}
		}
	}

	if err != nil {
		statusCode = 500
		bytes = []byte(err.Error())
	}

	natResponse := NewCtrlNatResponse(r.MsgId, bytes, statusCode)
	_ = m.SendNatResponse(conn, natResponse)
}

func (m *ConnectionManager) SendNatResponse(conn *Connection, natResponse *CtrlNatResponse) error {
	natResponseBytes, err := natResponse.toBytes()
	if err != nil {
		return err
	}

	return conn.SendCtrlMessage(natResponseBytes)
}

func (m *ConnectionManager) SendNatRequest(conn *Connection, natRequest *CtrlNatRequest, expectResponse bool) (*CtrlNatResponse, error) {
	var req *util.Request = nil

	if expectResponse {
		req = m.reqManager.GetReq()
		natRequest.MsgId = req.Id()
		defer func() {
			if req != nil {
				m.reqManager.CancelReq(req.Id())
			}
		}()
	}

	natRequestBytes, err := natRequest.toBytes()
	if err != nil {
		return nil, err
	}

	err = conn.SendCtrlMessage(natRequestBytes)
	if err != nil {
		return nil, err
	}

	if req != nil {
		res := req.WaitResult()
		req = nil // reset it for ignore defer cancel
		if res != nil && res.IsSuccess() {
			natResponse := res.Data().(*CtrlNatResponse)
			return natResponse, nil
		} else {
			return nil, res.Error()
		}
	} else {
		return nil, nil
	}
}

func newConnectionManager(localNatName string, limit int) *ConnectionManager {
	m := &ConnectionManager{
		localNatName:       localNatName,
		limit:              limit,
		connections:        util.NewSafeLinkMap(),
		reqManager:         util.NewReqRespManager(util.WithReqRespTimeout(30 * time.Second)),
		natRequestHandlers: make(map[string]func(conn *Connection, r *CtrlNatRequest)),
	}
	m.AddNatRequestHandler("/offer2answer", m.handleNatOfferRequest)
	return m
}

func (m *ConnectionManager) NewConnection(name string, conn *websocket.Conn) (*Connection, error) {
	if m.HasConnection(name) {
		return nil, fmt.Errorf("connection(%s) already exist", name)
	}
	if m.connections.Size() >= m.limit {
		return nil, fmt.Errorf("connection limit(%d) reached", m.limit)
	}

	c := newConnection(conn, name)
	m.connections.Add(name, c)

	c.OnCtrlMessage(m.onMessage)

	c.OnClose(func() {
		m.connections.Delete(name)
		fmt.Printf("ws peer %s closed\n", name)
	})
	m.onConnection(c)
	fmt.Printf("ws peer %s connected\n", name)

	return c, nil
}

func (m *ConnectionManager) handleNatMessage(conn *Connection, natReq *CtrlNatRequest) {
	path := natReq.URL.Path
	h, ok := m.natRequestHandlers[path]
	if !ok {
		return
	}
	h(conn, natReq)
}

func (m *ConnectionManager) onConnection(conn *Connection) {
	if m.onConnectionHandler != nil {
		m.onConnectionHandler(conn)
	}
}

func (m *ConnectionManager) onMessage(conn *Connection, message []byte) {
	ctrlMessage := &CtrlMessageHeader{}
	err := ctrlMessage.fromBytes(message)
	if err != nil {
		return
	}
	if ctrlMessage.IsReq {
		switch ctrlMessage.CtrlMessageType {
		case CtrlMessageSetNatName:
			natNameMessage := &CtrlNatName{}
			err := natNameMessage.fromBytes(message)
			if err != nil {
				return
			}
			conn.SetRemoteName(natNameMessage.Name)
		case CtrlMessageNatSend:
			natRequest := &CtrlNatRequest{}
			err := natRequest.fromBytes(message)
			if err != nil {
				return
			}
			m.handleNatMessage(conn, natRequest)
		}
	} else {
		switch ctrlMessage.CtrlMessageType {
		case CtrlMessageNatSend:
			natResponse := &CtrlNatResponse{}
			err := natResponse.fromBytes(message)
			if err != nil {
				return
			}
			m.reqManager.ResolveReq(ctrlMessage.MsgId, natResponse)
		}
	}
}

func (m *ConnectionManager) GetConnection(name string) *Connection {
	conn, found := m.connections.Get(name)
	if !found {
		return nil
	}
	return conn.(*Connection)
}

func (m *ConnectionManager) IsFull() bool {
	return m.connections.Size() >= m.limit
}

func (m *ConnectionManager) HasConnection(name string) bool {
	return m.GetConnection(name) != nil
}

func (m *ConnectionManager) Start() {
	m.reqManager.Start()
}

func (m *ConnectionManager) Close() {
	for _, _conn := range m.connections.Items() {
		conn := _conn.(*Connection)
		conn.Close()
	}
	m.reqManager.Close()
}

func (m *ConnectionManager) GetNames(filter func(conn *Connection) bool) []string {
	names := make([]string, 0)
	for _, _conn := range m.connections.Items() {
		conn := _conn.(*Connection)
		if filter == nil || filter(conn) {
			names = append(names, conn.remoteName)
		}
	}
	return names
}

func (m *ConnectionManager) SetLocalNatName(localNatName string) {
	if m.localNatName == localNatName {
		return
	}
	m.localNatName = localNatName
	msg := NewCtrlNatNameMessage(localNatName)
	data, err := msg.toBytes()
	if err != nil {
		return
	}

	for _, _conn := range m.connections.Items() {
		conn := _conn.(*Connection)
		if conn.remoteName == localNatName {
			conn.Close() // if have same name, close it
		} else {
			err := conn.SendCtrlMessage(data)
			if err != nil {
				log.Printf("change connection(%s) localNatName (%s) failed: %s\n", conn.remoteName, localNatName, err)
			}
		}
	}
}
