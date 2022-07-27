package util

import (
	"container/list"
	"sync"
)

type Locker interface {
	Lock()
	Unlock()
	RLock()
	RUnlock()
}

type NullLocker struct {
}

func (n *NullLocker) Lock()    {}
func (n *NullLocker) Unlock()  {}
func (n *NullLocker) RLock()   {}
func (n *NullLocker) RUnlock() {}

type ListItem struct {
	Key   interface{}
	Value interface{}
}

type SafeLinkMap struct {
	lock  Locker
	list  *list.List
	mp    map[interface{}]*list.Element
	limit int
	count int
}

func NewSafeLimitLinkMap(size int) *SafeLinkMap {
	return &SafeLinkMap{
		lock:  &sync.RWMutex{},
		list:  list.New(),
		mp:    make(map[interface{}]*list.Element),
		limit: size,
	}
}

func NewSafeLinkMap() *SafeLinkMap {
	return &SafeLinkMap{
		lock:  &sync.RWMutex{},
		list:  list.New(),
		mp:    make(map[interface{}]*list.Element),
		limit: -1,
	}
}

func NewLinkMap() *SafeLinkMap {
	return &SafeLinkMap{
		lock:  &NullLocker{},
		list:  list.New(),
		mp:    make(map[interface{}]*list.Element),
		limit: -1,
	}
}

func (l *SafeLinkMap) Get(k interface{}) (interface{}, bool) {
	l.lock.RLock()
	e, ok := l.mp[k]
	if !ok {
		l.lock.RUnlock()
		return nil, false
	}
	val := e.Value.(*ListItem).Value
	l.lock.RUnlock()
	return val, true
}

func (l *SafeLinkMap) add(k, v interface{}) {
	if l.count == l.limit {
		l.shift()
	}
	e := l.list.PushBack(&ListItem{k, v})
	l.mp[k] = e
	l.count++
}

func (l *SafeLinkMap) Add(k, v interface{}) {
	l.lock.Lock()
	e, ok := l.mp[k]
	if ok {
		l.remove(e)
	}
	l.add(k, v)
	l.lock.Unlock()
}

func (l *SafeLinkMap) Set(k, v interface{}) {
	l.Add(k, v)
}

func (l *SafeLinkMap) Update(k, v interface{}) {
	l.lock.Lock()
	e, ok := l.mp[k]
	if ok {
		e.Value = &ListItem{k, v}
	} else {
		l.add(k, v)
	}
	l.lock.Unlock()
}

func (l *SafeLinkMap) Has(k interface{}) bool {
	l.lock.RLock()
	_, ok := l.mp[k]
	l.lock.RUnlock()
	return ok
}

func (l *SafeLinkMap) remove(e *list.Element) (key, value interface{}) {
	val := l.list.Remove(e)
	item := val.(*ListItem)
	key = item.Key
	value = item.Value
	delete(l.mp, key)
	l.count--
	return
}

func (l *SafeLinkMap) shift() (key, value interface{}, found bool) {
	e := l.list.Front()
	if e == nil {
		found = false
	} else {
		key, value = l.remove(e)
		found = true
	}
	return
}

func (l *SafeLinkMap) Shift() (key, value interface{}, found bool) {
	l.lock.Lock()
	key, value, found = l.shift()
	l.lock.Unlock()
	return
}

func (l *SafeLinkMap) Front() (key, value interface{}, found bool) {
	l.lock.RLock()
	e := l.list.Front()
	if e == nil {
		found = false
	} else {
		item := e.Value.(*ListItem)
		key = item.Key
		value = item.Value
		found = true
	}
	l.lock.RUnlock()
	return
}

func (l *SafeLinkMap) PopBack() (key, value interface{}, found bool) {
	l.lock.RLock()
	e := l.list.Back()
	if e == nil {
		found = false
	} else {
		key, value = l.remove(e)
		found = true
	}
	l.lock.RUnlock()
	return
}

func (l *SafeLinkMap) Back() (key, value interface{}, found bool) {
	l.lock.RLock()
	e := l.list.Back()
	if e == nil {
		found = false
	} else {
		item := e.Value.(*ListItem)
		key = item.Key
		value = item.Value
		found = true
	}
	l.lock.RUnlock()
	return
}

func (l *SafeLinkMap) MoveToBack(k interface{}) {
	l.lock.Lock()
	e, ok := l.mp[k]
	if ok {
		l.list.MoveToBack(e)
	}
	l.lock.Unlock()
}

func (l *SafeLinkMap) Delete(k interface{}) (value interface{}, found bool) {
	l.lock.Lock()
	e, ok := l.mp[k]
	var v interface{} = nil
	if ok {
		delete(l.mp, k)
		val := l.list.Remove(e)
		v = val.(*ListItem).Value
		l.count--
	}
	l.lock.Unlock()
	return v, ok
}

func (l *SafeLinkMap) Size() int {
	l.lock.RLock()
	size := l.count
	l.lock.RUnlock()
	return size
}

func (l *SafeLinkMap) Clear() {
	l.lock.Lock()
	l.list = list.New()
	l.mp = make(map[interface{}]*list.Element)
	l.count = 0
	l.lock.Unlock()
}

func (l *SafeLinkMap) Items() []interface{} {
	l.lock.RLock()
	n := make([]interface{}, l.list.Len())
	i := 0
	for e := l.list.Front(); e != nil; e = e.Next() {
		n[i] = e.Value.(*ListItem).Value
		i++
	}
	l.lock.RUnlock()
	return n
}

func (l *SafeLinkMap) RevertItems() []interface{} {
	l.lock.RLock()
	n := make([]interface{}, l.list.Len())
	i := 0
	for e := l.list.Back(); e != nil; e = e.Prev() {
		n[i] = e.Value.(*ListItem).Value
		i++
	}
	l.lock.RUnlock()
	return n
}
