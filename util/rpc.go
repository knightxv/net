package util

import (
	"fmt"
	"reflect"
)

type (
	RPCResultType interface{}
	RPCParamType  interface{}
	RPCHandler    func(params ...RPCParamType) RPCResultType
)

type RPCOption func(*RPCCenter)

type RPCCenter struct {
	handlers          *SafeMap
	onRegistCallbacks *SafeMap
}

func NewRPCCenter(options ...RPCOption) *RPCCenter {
	rpc := &RPCCenter{
		handlers:          NewSafeMap(),
		onRegistCallbacks: NewSafeMap(),
	}
	for _, option := range options {
		option(rpc)
	}
	return rpc
}

func (rpc *RPCCenter) Regist(name string, handler RPCHandler) {
	if handler == nil {
		return
	}
	rpc.handlers.Set(name, handler)
	callbacks, found := rpc.onRegistCallbacks.Get(name)
	if found {
		for _, callback := range callbacks.(*SafeSlice).Items() {
			go callback.(RPCHandler)()
		}
	}
}

func (rpc *RPCCenter) Unregist(name string, handler RPCHandler) {
	oldHandler, found := rpc.handlers.Get(name)
	if found {
		sf1 := reflect.ValueOf(oldHandler)
		sf2 := reflect.ValueOf(handler)
		if sf1.Pointer() == sf2.Pointer() {
			rpc.handlers.Delete(name)
		}
	}
}
func (rpc *RPCCenter) Call(name string, params ...RPCParamType) (res RPCResultType, err error) {
	handler, found := rpc.handlers.Get(name)
	if !found {
		err = fmt.Errorf("handler %s not found", name)
		return
	}

	res = handler.(RPCHandler)(params...)
	return
}

func (rpc *RPCCenter) OnRegist(name string, handler RPCHandler) {
	callbasks, found := rpc.onRegistCallbacks.Get(name)
	if !found {
		callbasks = NewSafeSlice(0)
		rpc.onRegistCallbacks.Set(name, callbasks)
	}
	callbasks.(*SafeSlice).Append(handler)
	_, found = rpc.handlers.Get(name)
	if found {
		handler()
	}
}

var RPCCenterDefault = NewRPCCenter()

func DefaultRPCCenter() *RPCCenter { return RPCCenterDefault }

func RPCRegist(name string, handler RPCHandler) {
	RPCCenterDefault.Regist(name, handler)
}

func RPCUnregist(name string, handler RPCHandler) {
	RPCCenterDefault.Unregist(name, handler)
}

func RPCCall(name string) (res RPCResultType, err error) {
	return RPCCenterDefault.Call(name)
}

func RPCOnRegist(name string, handler RPCHandler) {
	RPCCenterDefault.OnRegist(name, handler)
}
