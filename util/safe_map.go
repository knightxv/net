package util

import (
	"sync"
)

type SafeMap struct {
	lock sync.RWMutex
	mp   map[interface{}]interface{}
}

func NewSafeMap() *SafeMap {
	return &SafeMap{
		mp: make(map[interface{}]interface{}),
	}
}

func (m *SafeMap) Get(key interface{}) (interface{}, bool) {
	m.lock.RLock()
	v, ok := m.mp[key]
	m.lock.RUnlock()
	return v, ok
}

func (m *SafeMap) Set(key, v interface{}) {
	m.lock.Lock()
	m.mp[key] = v
	m.lock.Unlock()
}

func (m *SafeMap) Has(key interface{}) bool {
	m.lock.RLock()
	_, ok := m.mp[key]
	m.lock.RUnlock()
	return ok
}

func (m *SafeMap) Delete(key interface{}) {
	m.lock.Lock()
	delete(m.mp, key)
	m.lock.Unlock()
}

func (m *SafeMap) Clear() {
	m.lock.Lock()
	m.mp = make(map[interface{}]interface{})
	m.lock.Unlock()
}

func (m *SafeMap) Size() int {
	m.lock.RLock()
	l := len(m.mp)
	m.lock.RUnlock()
	return l
}

func (m *SafeMap) Items() map[interface{}]interface{} {
	m.lock.RLock()
	n := make(map[interface{}]interface{})
	for k, v := range m.mp {
		n[k] = v
	}
	m.lock.RUnlock()
	return n
}
