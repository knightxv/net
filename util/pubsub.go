package util

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	BufferSizeDefault = 10
	CacheLastDefault  = true
	TimeoutDefault    = 5 * time.Second
)

type (
	PubSubMsgType      interface{}
	PubSubMsgChannel   <-chan PubSubMsgType
	PubSubCancelFunc   func()
	PubSubCallbackFunc func(PubSubMsgType)
)

type Subscriber struct {
	name     string
	topic    string
	callback PubSubCallbackFunc
}

func newSubscriber(name, topic string, callback PubSubCallbackFunc) *Subscriber {
	return &Subscriber{
		name:     name,
		topic:    topic,
		callback: callback,
	}
}

type Publisher struct {
	topic *Topic
}

func (p *Publisher) Publish(msg PubSubMsgType) {
	p.topic.publish(msg)
}

type Topic struct {
	Id         string
	subcribers *SafeMap
	cacheLast  bool
	cacheMsg   PubSubMsgType
	hasCache   bool
	msgChan    chan PubSubMsgType
}

func newTopic(ctx context.Context, id string, cache bool) *Topic {
	topic := &Topic{
		Id:         id,
		subcribers: NewSafeMap(),
		cacheLast:  cache,
		msgChan:    make(chan PubSubMsgType, BufferSizeDefault),
	}

	go func() {
		for {
			select {
			case msg := <-topic.msgChan:
				for _, _sub := range topic.subcribers.Items() {
					sub := _sub.(*Subscriber)
					sub.callback(msg)
				}
			case <-ctx.Done():
				topic.close()
			}
		}
	}()

	return topic
}

func (t *Topic) publish(msg PubSubMsgType) {
	if t.cacheLast {
		t.cacheMsg = msg
		t.hasCache = true
	}

	if t.subcribers.Size() > 0 {
		t.msgChan <- msg
	}
}

func (t *Topic) addSubcriber(sub *Subscriber) bool {
	if t.subcribers.Has(sub.name) {
		return false
	}

	t.subcribers.Set(sub.name, sub)
	if t.cacheLast && t.hasCache {
		sub.callback(t.cacheMsg)
	}

	return true
}

func (t *Topic) delSubcriber(sub *Subscriber) {
	t.subcribers.Delete(sub.name)
}

func (t *Topic) close() {
	t.subcribers.Clear()
	close(t.msgChan)
}

type PubSubOption func(*PubSub)

func WithCacheLast(cached bool) PubSubOption {
	return func(ps *PubSub) {
		ps.CacheLast = cached
	}
}

type PubSub struct {
	topics    *SafeMap
	CacheLast bool
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewPubSub(options ...PubSubOption) *PubSub {
	ctx, cancel := context.WithCancel(context.Background())
	ps := &PubSub{
		topics:    NewSafeMap(),
		CacheLast: CacheLastDefault,
		ctx:       ctx,
		cancel:    cancel,
	}

	for _, option := range options {
		option(ps)
	}
	return ps
}

func (ps *PubSub) getTopic(id string) *Topic {
	_topicItem, found := ps.topics.Get(id)
	if found {
		return _topicItem.(*Topic)
	}

	topic := newTopic(ps.ctx, id, ps.CacheLast)
	ps.topics.Set(id, topic)
	return topic
}

func (ps *PubSub) Subscribe(name string, topicId string, callback PubSubCallbackFunc) (*Subscriber, error) {
	topic := ps.getTopic(topicId)
	sub := newSubscriber(name, topicId, callback)
	if !topic.addSubcriber(sub) {
		return nil, fmt.Errorf("subscriber %s already exists", name)
	}

	return sub, nil
}

func (ps *PubSub) subscribeFunc(name string, topicId string, handler func(PubSubMsgType), wg *sync.WaitGroup) (PubSubCancelFunc, error) {
	if handler == nil {
		return nil, fmt.Errorf("handler is nil")
	}

	if wg != nil {
		wg.Add(1)
	}
	sub, err := ps.Subscribe(name, topicId, func(psmt PubSubMsgType) {
		handler(psmt)
		if wg != nil {
			wg.Done()
			wg = nil
		}
	})
	if err != nil {
		return nil, err
	}
	return func() { ps.unsubscribe(sub) }, nil
}

func (ps *PubSub) SubscribeFunc(name string, topicId string, handler func(PubSubMsgType)) (PubSubCancelFunc, error) {
	return ps.subscribeFunc(name, topicId, handler, nil)
}

func (ps *PubSub) SubscribeFuncWait(name string, topicId string, handler func(PubSubMsgType)) (cancel PubSubCancelFunc, err error) {
	wg := &sync.WaitGroup{}
	cancel, err = ps.subscribeFunc(name, topicId, handler, wg)
	wg.Wait()
	return
}

func (ps *PubSub) unsubscribe(sub *Subscriber) {
	topic := ps.getTopic(sub.topic)
	topic.delSubcriber(sub)
}

func (ps *PubSub) Publish(topicId string, msg PubSubMsgType) {
	topic := ps.getTopic(topicId)
	topic.publish(msg)
}

func (ps *PubSub) Close() {
	ps.cancel()
}

var PubSubDefault = NewPubSub()

func DefaultPubSub() *PubSub { return PubSubDefault }

func Publish(topicId string, msg PubSubMsgType) {
	PubSubDefault.Publish(topicId, msg)
}

func SubscribeFunc(name string, topicId string, handler PubSubCallbackFunc) (PubSubCancelFunc, error) {
	return PubSubDefault.SubscribeFunc(name, topicId, handler)
}

func SubscribeFuncWait(name string, topicId string, handler PubSubCallbackFunc) (PubSubCancelFunc, error) {
	return PubSubDefault.SubscribeFuncWait(name, topicId, handler)
}
