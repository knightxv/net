package util

import (
	"errors"
	"sort"
	"sync"
)

type SafeSlice struct {
	lock  sync.RWMutex
	limit int
	sl    []interface{}
}

func NewSafeLimitSlice(limit int) *SafeSlice {
	return &SafeSlice{
		sl:    make([]interface{}, 0),
		limit: limit,
	}
}

func NewSafeSlice(size int) *SafeSlice {
	return &SafeSlice{
		sl:    make([]interface{}, size),
		limit: -1,
	}
}

func (s *SafeSlice) Get(idx int) (interface{}, bool) {
	s.lock.RLock()
	if idx >= len(s.sl) {
		s.lock.RUnlock()
		return nil, false
	}
	val := s.sl[idx]
	s.lock.RUnlock()
	return val, true
}

func (s *SafeSlice) Set(idx int, v interface{}) error {
	s.lock.Lock()
	if idx >= len(s.sl) {
		s.lock.Unlock()
		return errors.New("index out of range")
	}
	s.sl[idx] = v
	s.lock.Unlock()
	return nil
}

func (s *SafeSlice) Append(v interface{}) error {
	s.lock.Lock()
	if s.limit > 0 && len(s.sl) >= s.limit {
		s.lock.Unlock()
		return errors.New("limit reached")
	}
	s.sl = append(s.sl, v)
	s.lock.Unlock()
	return nil
}

func (s *SafeSlice) Delete(idx int) error {
	s.lock.Lock()
	if idx >= len(s.sl) {
		s.lock.Unlock()
		return errors.New("index out of range")
	}

	s.sl = append(s.sl[0:idx], s.sl[idx+1:]...)
	s.lock.Unlock()
	return nil
}

func (s *SafeSlice) Clear() {
	s.lock.Lock()
	s.sl = make([]interface{}, 0)
	s.lock.Unlock()
}

func (s *SafeSlice) Size() int {
	s.lock.RLock()
	l := len(s.sl)
	s.lock.RUnlock()
	return l
}

func (s *SafeSlice) Items() []interface{} {
	s.lock.RLock()
	n := make([]interface{}, len(s.sl))
	copy(n, s.sl)
	s.lock.RUnlock()
	return n
}

func (s *SafeSlice) Sort(compare func(i, j interface{}) bool) {
	s.lock.Lock()
	sliceCompare := func(i, j int) bool {
		return compare(s.sl[i], s.sl[j])
	}

	sort.Slice(s.sl, sliceCompare)
	s.lock.Unlock()
}
