package kaddht

import (
	"fmt"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/guabee/bnrtc/bnrtcv2/dht/idht"
	"github.com/guabee/bnrtc/bnrtcv2/log"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
)

const MessageChannelSize = 1024

type networkMessageResponse struct {
	ch        chan interface{}
	tm        messageType
	recevier  *Node
	id        int64
	timestamp time.Time
	done      bool
}

func (res *networkMessageResponse) WaitResult() *message {
	if res == nil || res.ch == nil {
		return nil
	}
	data, done := <-res.ch
	if !done || data == nil {
		return nil
	} else {
		return data.(*message)
	}
}

func (res *networkMessageResponse) ResultChannel() <-chan interface{} {
	if res == nil {
		return nil
	}

	return res.ch
}

func (res *networkMessageResponse) Close() {
	if res != nil {
		if res.ch != nil {
			close(res.ch)
			res.ch = nil
		}
		res.done = true
	}
}

type networking interface {
	sendMessage(*message, *messageResponseOptions) (*networkMessageResponse, error)
	getMessageChan() <-chan (*message)
	getIterateMessageChan() <-chan (*message)
	run()
	close() error
	cancelResponse(*networkMessageResponse)
	SetNode(*Node)
	SetLogger(*log.Entry)
}

type networkingEvent uint8

const (
	networkingEvent_AddPendingResponse networkingEvent = iota
	networkingEvent_DeletePendingResponse
	networkingEvent_AnswerPendingResponse
)

func (e networkingEvent) String() string {
	switch e {
	case networkingEvent_AddPendingResponse:
		return "AddPendingResponse"
	case networkingEvent_DeletePendingResponse:
		return "DeletePendingResponse"
	case networkingEvent_AnswerPendingResponse:
		return "AnswerPendingResponse"
	default:
		return fmt.Sprintf("Unknown(%d)", e)
	}
}

type networkingEventData struct {
	ev   networkingEvent
	data interface{}
}

type networkingMessageData struct {
	msg              *message
	expectedResponse *networkMessageResponse
}

type realNetworking struct {
	transport       idht.ITransportManager
	self            *Node
	sendChan        chan (*networkingMessageData)
	msgChan         messageChannel
	iteratorMsgChan messageChannel
	eventChan       chan (*networkingEventData)
	ctxManager      *util.ContextManager
	responseMap     *util.SafeLinkMap
	msgCounter      int64
	logger          *log.Entry
}

func newRealNetworking(transport idht.ITransportManager, self *Node, logger *log.Entry) *realNetworking {
	return &realNetworking{
		transport:       transport,
		self:            self,
		sendChan:        make(chan (*networkingMessageData), MessageChannelSize),
		msgChan:         *newMessageChannel(MessageChannelSize),
		iteratorMsgChan: *newMessageChannel(MessageChannelSize),
		eventChan:       make(chan *networkingEventData, 64),
		ctxManager:      util.NewContextManager(),
		responseMap:     util.NewSafeLinkMap(),
		logger:          logger,
	}
}

func (rn *realNetworking) getMessageChan() <-chan (*message) {
	return rn.msgChan.GetMsgChannel()
}

func (rn *realNetworking) getIterateMessageChan() <-chan (*message) {
	return rn.iteratorMsgChan.GetMsgChannel()
}

func (rn *realNetworking) sendMessage(msg *message, options *messageResponseOptions) (*networkMessageResponse, error) {
	expectedResponse := (*networkMessageResponse)(nil)
	if options != nil && options.timeout != 0 { // expected response
		expectedResponse = &networkMessageResponse{
			tm:        msg.Type,
			ch:        make(chan interface{}, 1),
			recevier:  msg.Receiver,
			timestamp: time.Now().Add(options.timeout),
		}
	}

	data := &networkingMessageData{
		msg:              msg,
		expectedResponse: expectedResponse,
	}
	select {
	case rn.sendChan <- data:
		return expectedResponse, nil
	default:
		expectedResponse.Close()
		return nil, fmt.Errorf("Send message channel is full")
	}
}

func (rn *realNetworking) cancelResponse(res *networkMessageResponse) {
	if res == nil {
		return
	}

	rn.eventChan <- &networkingEventData{
		ev:   networkingEvent_DeletePendingResponse,
		data: res,
	}
}

func (rn *realNetworking) close() error {
	rn.ctxManager.CancelWaitAll()
	rn.msgChan.Close()
	rn.iteratorMsgChan.Close()
	rn.logger.Debug("Closed")
	return nil
}

func (rn *realNetworking) checkExpiredResponseWorker(ctx *util.Context) {
	t := time.NewTicker(time.Second)
	for {
		select {
		case <-t.C:
			for _, _resp := range rn.responseMap.Items() {
				resp := _resp.(*networkMessageResponse)
				if time.Since(resp.timestamp) > 0 {
					rn.cancelResponse(resp)
				} else {
					break
				}
			}
		case <-ctx.Done():
			t.Stop()
			ctx.Success()
			return
		}
	}
}

func (rn *realNetworking) eventWorker(ctx *util.Context) {
	for {
		select {
		case ev := <-rn.eventChan:
			rn.handleEvent(ev)
		case <-ctx.Done():
			// clear all pending responses
			for _, resp := range rn.responseMap.Items() {
				resp.(*networkMessageResponse).Close()
			}
			rn.responseMap.Clear()
			close(rn.eventChan)
			ctx.Success()
			return
		}
	}
}

func (rn *realNetworking) handleEvent(ev *networkingEventData) {
	rn.logger.Debugf("handler Event: %s", ev.ev)
	switch ev.ev {
	case networkingEvent_AddPendingResponse:
		resp := ev.data.(*networkMessageResponse)
		if !resp.done {
			rn.logger.Debugf("AddPendingResponse id: %d", resp.id)
			oldResp, found := rn.responseMap.Get(resp.id)
			if found {
				oldResp.(*networkMessageResponse).Close() // close the channel if already exists
			}
			rn.responseMap.Set(resp.id, resp)
		}
	case networkingEvent_DeletePendingResponse:
		resp := ev.data.(*networkMessageResponse)
		rn.logger.Debugf("DeletePendingResponse id: %d", resp.id)
		rn.responseMap.Delete(resp.id)
		resp.Close()
	case networkingEvent_AnswerPendingResponse:
		msg := ev.data.(*message)
		id := msg.ID
		rn.logger.Debugf("AnswerPendingResponse id: %d", id)
		_resp, found := rn.responseMap.Get(id)
		if found {
			resp := _resp.(*networkMessageResponse)
			if msg.Sender.Equal(resp.recevier, resp.tm == messageTypePing) {
				resp.ch <- msg
				resp.Close()
				rn.responseMap.Delete(id)
			}
		}
	}
}

func (rn *realNetworking) receiveMessageWorker(ctx *util.Context) {
	for {
		select {
		case tmMsg := <-rn.transport.GetCtrlMsgChan():
			if tmMsg == nil { // transport closed
				ctx.Success()
				return
			}
			if tmMsg.IsExpired() {
				continue
			}

			data := tmMsg.GetBuffer().Data()
			msg, err := deserializeMessage(data)
			if err != nil {
				rn.logger.Errorf("Failed to deserialize message: %s", err.Error())
				continue
			}

			rn.handleReceiveMessage(msg)
		case <-ctx.Done():
			ctx.Success()
			return
		}
	}
}

func (rn *realNetworking) handleReceiveMessage(msg *message) {
	rn.logger.Debugf("ReceiveMessage %s form %s, isResponse: %t", msg.Type, msg.Sender.Id, msg.IsResponse)
	isPing := msg.Type == messageTypePing

	if !rn.self.Equal(msg.Receiver, isPing) {
		rn.logger.Errorf("Received message with invalid receiver %s", msg.Receiver.Id)
		return
	}

	if msg.IsResponse {
		rn.eventChan <- &networkingEventData{
			ev:   networkingEvent_AnswerPendingResponse,
			data: msg,
		}
	} else {
		assertion := false
		switch msg.Type {
		case messageTypeFindNode:
			_, assertion = msg.Data.(*queryDataFindNode)
		case messageTypeFindValue:
			_, assertion = msg.Data.(*queryDataFindValue)
		case messageTypeStore:
			_, assertion = msg.Data.(*queryDataStore)
		default:
			assertion = true
		}

		if !assertion {
			rn.logger.Errorf("Received bad message %s from %+v", msg.Type, msg.Sender)
			return
		}
		if msg.Iterative {
			rn.iteratorMsgChan.Send(msg)
		} else {
			rn.msgChan.Send(msg)
		}
	}
}

func (rn *realNetworking) sendMessageWorker(ctx *util.Context) {
	for {
		select {
		case msg := <-rn.sendChan:
			rn.handleSendMessage(msg)
		case <-ctx.Done():
			close(rn.sendChan)
			ctx.Success()
			return
		}
	}
}

func (rn *realNetworking) handleSendMessage(data *networkingMessageData) {
	msg := data.msg
	expectedResponse := data.expectedResponse

	if !msg.IsResponse {
		rn.msgCounter++
		msg.ID = rn.msgCounter
	}
	rn.logger.Debugf("SendMessage %s to %s: Id %d, isResponse: %t", msg.Type, msg.Receiver.Id, msg.ID, msg.IsResponse)

	// add pending before sending
	if expectedResponse != nil {
		expectedResponse.id = msg.ID
		rn.eventChan <- &networkingEventData{
			ev:   networkingEvent_AddPendingResponse,
			data: expectedResponse,
		}
	}

	value, err := serializeMessage(msg)
	if err != nil {
		rn.cancelResponse(expectedResponse)
		rn.logger.Errorf("Failed to serialize message %s", err.Error())
		return
	}

	target := msg.Receiver
	tsAddress, err := rn.self.ChooseMatchedAddress([]*device_info.DeviceInfo{target.GetDeviceInfo()}, target.Id, rn.transport)
	if err != nil {
		rn.cancelResponse(expectedResponse)
		rn.logger.Errorf("Failed to choose address for %s: %s", target.Id, err.Error())
		return
	}

	err = rn.transport.SendCtrlMsg(tsAddress, buffer.FromBytes(value))
	if err != nil {
		rn.cancelResponse(expectedResponse)
		rn.logger.Errorf("Failed to send message %s to %s: %s", msg.Type, target.Id, err.Error())
		return
	}
}

func (rn *realNetworking) run() {
	rn.ctxManager.RunWorker(rn.receiveMessageWorker)
	rn.ctxManager.RunWorker(rn.sendMessageWorker)
	rn.ctxManager.RunWorker(rn.checkExpiredResponseWorker)
	rn.ctxManager.RunWorker(rn.eventWorker)
}

func (rn *realNetworking) SetNode(n *Node) {
	rn.self = n
}

func (rn *realNetworking) SetLogger(logger *log.Entry) {
	rn.logger = logger
}
