package util

import (
	"sync"
)

type mapItem struct {
	data interface{}
	idx  int
}

type sliceItem struct {
	data interface{}
	key  interface{}
}

type SafeSliceMap struct {
	lock sync.RWMutex
	mp   map[interface{}]*mapItem
	sl   []*sliceItem
}

func NewSafeSliceMap() *SafeSliceMap {
	return &SafeSliceMap{
		mp: make(map[interface{}]*mapItem),
		sl: make([]*sliceItem, 0),
	}
}

func (m *SafeSliceMap) Get(key interface{}) (interface{}, bool) {
	m.lock.RLock()
	v, found := m.mp[key]
	m.lock.RUnlock()
	if !found {
		return nil, false
	}
	return v.data, found
}

func (m *SafeSliceMap) Set(key, v interface{}) {
	m.lock.Lock()
	old, found := m.mp[key]
	if found {
		m.mp[key] = &mapItem{
			data: v,
			idx:  old.idx,
		}
		m.sl[old.idx] = &sliceItem{
			data: v,
			key:  key,
		}
	} else {
		m.mp[key] = &mapItem{
			data: v,
			idx:  len(m.sl),
		}
		m.sl = append(m.sl, &sliceItem{key: key, data: v})
	}
	m.lock.Unlock()
}

func (m *SafeSliceMap) Has(key interface{}) bool {
	m.lock.RLock()
	_, found := m.mp[key]
	m.lock.RUnlock()
	return found
}

func (m *SafeSliceMap) Delete(key interface{}) {
	m.lock.Lock()
	old, found := m.mp[key]
	if found {
		if old.idx == len(m.sl) {
			m.sl = m.sl[0 : len(m.sl)-1]
		} else {
			m.sl = append(m.sl[0:old.idx], m.sl[old.idx+1:]...)
		}
		delete(m.mp, key)
	}
	m.lock.Unlock()
}

func (m *SafeSliceMap) Clear() {
	m.lock.Lock()
	m.mp = make(map[interface{}]*mapItem)
	m.sl = make([]*sliceItem, 0)
	m.lock.Unlock()
}

func (m *SafeSliceMap) Size() int {
	m.lock.RLock()
	l := len(m.sl)
	m.lock.RUnlock()
	return l
}

func (m *SafeSliceMap) Items() []interface{} {
	m.lock.RLock()
	n := make([]interface{}, len(m.sl))
	for i, v := range m.sl {
		n[i] = v.data
	}
	m.lock.RUnlock()
	return n
}

func (m *SafeSliceMap) GetAt(idx int) (interface{}, bool) {
	m.lock.RLock()
	if idx >= len(m.sl) {
		m.lock.RUnlock()
		return nil, false
	}
	val := m.sl[idx].data
	m.lock.RUnlock()
	return val, true
}

func (m *SafeSliceMap) DeleteAt(idx int) {
	m.lock.Lock()
	if idx < len(m.sl) {
		old := m.sl[idx]
		m.sl = append(m.sl[0:idx], m.sl[idx+1:]...)
		delete(m.mp, old.key)
	}
	m.lock.Unlock()
}
