package util

import (
	"errors"
	"sync"
	"time"
)

type ConditionOption func(*Condition)

func WithConditionTimeout(timeout time.Duration) ConditionOption {
	return func(c *Condition) {
		c.timeout = timeout
	}
}

func WithConditionWaitCount(waiters int) ConditionOption {
	return func(c *Condition) {
		c.waiters = waiters
	}
}

type Condition struct {
	waiters int
	lock    sync.Mutex
	done    bool
	result  chan error
	timeout time.Duration
	timer   *time.Timer
}

func NewCondition(options ...ConditionOption) *Condition {
	c := &Condition{
		result:  make(chan error, 1),
		waiters: 1,
	}

	for _, option := range options {
		option(c)
	}

	if c.timeout > 0 {
		c.timer = time.AfterFunc(c.timeout, func() {
			c.Signal(errors.New("timeout"))
		})
	}
	return c
}

func (c *Condition) Wait() (err error) {
	err = <-c.result
	return
}

func (c *Condition) Signal(err error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.done {
		return
	}
	if err != nil {
		c.result <- err
		close(c.result)
		c.done = true
	} else {
		c.waiters--
		if c.waiters == 0 {
			if c.timer != nil {
				c.timer.Stop()
			}
			close(c.result)
			c.done = true
		}
	}
}
