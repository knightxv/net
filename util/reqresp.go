package util

import (
	"errors"
	"sync/atomic"
	"time"
)

const (
	DefaultReqRespTimeout              = 30 * time.Second
	DefaultReqRespCheckExpiredInterval = 2 * time.Second
	InvalidReqId                       = ^uint32(0)
)

type ResponseCode int

const (
	ReqRespTimeout ResponseCode = iota
	ReqRespCanceled
	ReqRespResolved
	ReqRespError
)

type Response struct {
	reqId uint32
	code  ResponseCode
	data  interface{}
}

func (r *Response) IsSuccess() bool {
	return r.code == ReqRespResolved
}

func (r *Response) Id() uint32 {
	return r.reqId
}

func (r *Response) Code() ResponseCode {
	return r.code
}

func (r *Response) Data() interface{} {
	return r.data
}

func (r *Response) Error() error {
	switch r.code {
	case ReqRespResolved:
		return nil
	case ReqRespTimeout:
		return errors.New("request timeout")
	case ReqRespCanceled:
		return errors.New("request canceled")
	case ReqRespError:
		return errors.New("request error")
	default:
		return errors.New("unknown error")
	}
}

type Event struct {
	controller *ReqRespConroller
	reqId      uint32
	code       ResponseCode
	data       interface{}
}

type RequestCallback func(*Response)

type Request struct {
	id          uint32
	ch          chan *Response
	callback    RequestCallback
	expiredTime time.Time
}

func (req *Request) WaitResult() *Response {
	if req == nil || req.ch == nil {
		return &Response{
			reqId: req.id,
			code:  ReqRespError,
		}
	}
	return <-req.ch
}

func (req *Request) setResult(code ResponseCode, data interface{}) {
	if req != nil {
		resp := &Response{
			reqId: req.id,
			code:  code,
			data:  data,
		}

		if req.callback != nil {
			req.callback(resp)
		} else if req.ch != nil {
			req.ch <- resp
			close(req.ch)
		}
	}
}

func (req *Request) Id() uint32 {
	if req == nil {
		return InvalidReqId
	}
	return req.id
}

type ReqRespManagerOption func(*ReqRespManager)

func WithReqRespCheckInterval(interval time.Duration) ReqRespManagerOption {
	return func(m *ReqRespManager) {
		if interval > 0 {
			m.checkInterval = interval
		}
	}
}

func WithReqRespTimeout(timeout time.Duration) ReqRespManagerOption {
	return func(m *ReqRespManager) {
		if timeout > 0 {
			m.timeout = timeout
		}
	}
}

type ReqRespManager struct {
	timeout           time.Duration
	checkInterval     time.Duration
	controllers       *SafeMap
	defaultController *ReqRespConroller
	msgId             uint32
	stopChan          chan struct{}
	eventChan         chan *Event
	isClosed          int32
}

type ReqRespConroller struct {
	name    string
	manager *ReqRespManager
	reqs    *SafeLinkMap
}

func (m *ReqRespManager) NewController(name string) *ReqRespConroller {
	c, ok := m.controllers.Get(name)
	if ok {
		return c.(*ReqRespConroller)
	}

	newController := &ReqRespConroller{
		name:    name,
		manager: m,
		reqs:    NewSafeLinkMap(),
	}
	m.controllers.Set(name, newController)
	return newController
}

func (c *ReqRespConroller) timeoutReq(id uint32) {
	c.manager.eventChan <- &Event{
		controller: c,
		reqId:      id,
		code:       ReqRespTimeout,
	}
}

func (c *ReqRespConroller) CancelReq(id uint32) {
	c.manager.eventChan <- &Event{
		controller: c,
		reqId:      id,
		code:       ReqRespCanceled,
	}
}

func (c *ReqRespConroller) ResolveReq(id uint32, data interface{}) {
	c.manager.eventChan <- &Event{
		controller: c,
		reqId:      id,
		code:       ReqRespResolved,
		data:       data,
	}
}

func (c *ReqRespConroller) Close() {
	for _, req := range c.reqs.Items() {
		c.CancelReq(req.(*Request).id)
	}

	c.manager.controllers.Delete(c.name)
}

func (c *ReqRespConroller) getReq(id uint32, f RequestCallback) *Request {
	if !c.manager.isRunning() {
		return nil
	}

	_, ok := c.reqs.Get(id)
	if ok {
		return nil
	}

	newReq := &Request{
		id:          id,
		callback:    f,
		expiredTime: time.Now().Add(c.manager.timeout),
	}

	if f == nil {
		newReq.ch = make(chan *Response, 1)
	}

	c.reqs.Set(id, newReq)
	return newReq
}

func (c *ReqRespConroller) GetReqId() uint32 {
	id := atomic.AddUint32(&c.manager.msgId, 1)
	if id == InvalidReqId {
		id = atomic.AddUint32(&c.manager.msgId, 1)
	}
	return id
}

func (c *ReqRespConroller) GetReq() *Request {
	return c.getReq(c.GetReqId(), nil)
}

func (c *ReqRespConroller) GetReqWithId(id uint32) *Request {
	if id == InvalidReqId {
		return nil
	}

	return c.getReq(id, nil)
}

func (c *ReqRespConroller) GetCallbackReq(f RequestCallback) *Request {
	return c.getReq(c.GetReqId(), f)
}

func (c *ReqRespConroller) GetCallbackReqWithId(id uint32, f RequestCallback) *Request {
	if id == InvalidReqId {
		return nil
	}

	return c.getReq(id, f)
}

func NewReqRespManager(options ...ReqRespManagerOption) *ReqRespManager {
	m := &ReqRespManager{
		timeout:       DefaultReqRespTimeout,
		checkInterval: DefaultReqRespCheckExpiredInterval,
		eventChan:     make(chan *Event, 1024),
		controllers:   NewSafeMap(),
		msgId:         0,
		isClosed:      1,
	}

	for _, option := range options {
		option(m)
	}
	m.defaultController = m.NewController("_default_")
	return m
}

func (m *ReqRespManager) isRunning() bool {
	return m.stopChan != nil
}

func (m *ReqRespManager) Start() {
	if m == nil {
		return
	}
	m.stopChan = make(chan struct{})
	atomic.StoreInt32(&m.isClosed, 0)
	go m.clearExpiredReqWorker()
	go m.handleEventWorker()
}

func (m *ReqRespManager) Close() {
	if m == nil {
		return
	}

	if atomic.CompareAndSwapInt32(&m.isClosed, 0, 1) {
		for _, controller := range m.controllers.Items() {
			controller.(*ReqRespConroller).Close()
		}
		close(m.stopChan)
	}
}

func (m *ReqRespManager) clearExpiredReqWorker() {
	t := time.NewTicker(m.checkInterval)
	for {
		select {
		case <-t.C:
			for _, controller := range m.controllers.Items() {
				c := controller.(*ReqRespConroller)
				for _, _req := range c.reqs.Items() {
					req := _req.(*Request)
					if time.Since(req.expiredTime) > 0 {
						c.timeoutReq(req.id)
					} else {
						break
					}
				}
			}
		case <-m.stopChan:
			t.Stop()
			return
		}
	}
}

func (m *ReqRespManager) handleEventWorker() {
	for {
		select {
		case event := <-m.eventChan:
			req, found := event.controller.reqs.Delete(event.reqId)
			if found {
				req.(*Request).setResult(event.code, event.data)
			}
		case <-m.stopChan:
			return
		}
	}
}

func (m *ReqRespManager) CancelReq(id uint32) {
	m.defaultController.CancelReq(id)
}

func (m *ReqRespManager) ResolveReq(id uint32, data interface{}) {
	m.defaultController.ResolveReq(id, data)
}

func (m *ReqRespManager) GetReqId() uint32 {
	return m.defaultController.GetReqId()
}

func (m *ReqRespManager) GetReq() *Request {
	return m.defaultController.GetReq()
}

func (m *ReqRespManager) GetReqWithId(id uint32) *Request {
	return m.defaultController.GetReqWithId(id)
}

func (m *ReqRespManager) GetCallbackReq(f RequestCallback) *Request {
	return m.defaultController.GetCallbackReq(f)
}

func (m *ReqRespManager) GetCallbackReqWithId(id uint32, f RequestCallback) *Request {
	return m.defaultController.GetCallbackReqWithId(id, f)
}
