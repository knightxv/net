package util

import (
	"sync"
)

type TaskState = int32

const (
	Idle TaskState = iota
	Running
	Pending
)

type SingleTonTaskOption func(*SingleTonTask)

func AllowPending() SingleTonTaskOption {
	return func(s *SingleTonTask) {
		s.allowPending = true
	}
}

func BindingFunc(f func()) SingleTonTaskOption {
	return func(s *SingleTonTask) {
		s.taskFunc = f
	}
}

type SingleTonTask struct {
	state        TaskState
	allowPending bool
	taskFunc     func()
	mt           sync.Mutex
}

func NewSingleTonTask(opts ...SingleTonTaskOption) *SingleTonTask {
	t := &SingleTonTask{
		allowPending: false,
		taskFunc:     nil,
		state:        Idle,
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

func (s *SingleTonTask) IsRunning() bool {
	return s.state == Running
}

func (s *SingleTonTask) run(f func(), sync bool) {
	s.mt.Lock()
	if !s.allowPending && s.state != Idle {
		s.mt.Unlock()
		return
	} else if s.allowPending && s.state == Pending {
		s.mt.Unlock()
		return
	}

	if s.state == Idle {
		s.state = Running
		s.mt.Unlock()
		worker := func() {
			f()
			s.mt.Lock()
			if s.state == Pending {
				s.state = Idle
				s.mt.Unlock()
				s.run(f, sync)
			} else {
				s.state = Idle
				s.mt.Unlock()
			}
		}
		if sync {
			worker()
		} else {
			go worker()
		}
	} else if s.state == Running {
		s.state = Pending
		s.mt.Unlock()
	}
}

//go:noinline
func (s *SingleTonTask) RunWaitWith(f func()) {
	s.run(f, true)
}

//go:noinline
func (s *SingleTonTask) RunWait() {
	if s.taskFunc == nil {
		return
	}

	s.RunWaitWith(s.taskFunc)
}

//go:noinline
func (s *SingleTonTask) RunWith(f func()) {
	s.run(f, false)
}

//go:noinline
func (s *SingleTonTask) Run() {
	if s.taskFunc == nil {
		return
	}

	s.RunWith(s.taskFunc)
}
