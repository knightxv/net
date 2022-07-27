package util

import (
	"context"
)

type WorkerFunc func(ctx *Context)

type Context struct {
	ctx    context.Context
	cancel context.CancelFunc
	wait   chan struct{}
}

func (c *Context) Cancel() {
	c.cancel()
}

func (c *Context) CancelWait() {
	c.cancel()
	<-c.wait
}

func (c *Context) Done() <-chan struct{} {
	return c.ctx.Done()
}

func (c *Context) Success() {
	c.wait <- struct{}{}
	c.cancel()
}

type ContextManager struct {
	ctx      context.Context
	contexts *SafeSlice
}

func NewContextManager() *ContextManager {
	return &ContextManager{
		ctx:      context.Background(),
		contexts: NewSafeSlice(0),
	}
}

func (c *ContextManager) RunWorker(worker WorkerFunc) {
	ctx, cancel := context.WithCancel(c.ctx)
	ctxx := &Context{
		ctx:    ctx,
		cancel: cancel,
		wait:   make(chan struct{}, 1),
	}
	c.contexts.Append(ctxx)
	go worker(ctxx)
}

func (c *ContextManager) CancelAll() {
	for _, ctx := range c.contexts.Items() {
		ctx.(*Context).Cancel()
	}
}

func (c *ContextManager) CancelWaitAll() {
	for _, ctx := range c.contexts.Items() {
		ctx.(*Context).CancelWait()
	}
}
