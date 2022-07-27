package bnrtcv1

import (
	"fmt"
	"log"
	"sync"

	"github.com/guabee/bnrtc/util"
)

type RegisteredNatDuplexs struct {
	localNatName string
	_duplexMap   map[string]*NatDuplex
	hooks        RegisteredNatDuplexsHooks
	_hooksHolder _RegisteredNatDuplexsHooksHolder
	// _hooksBinding chan RegisteredNatDuplexsHooks
	lock sync.RWMutex
}

type RegisteredNatDuplexsHooks interface {
	OnNatDuplexConnected(*NatDuplex)
	OnNatDuplexDisConnected(*NatDuplex)
}

/// 托管Hooks的参数，如果DoHold==true的话
type _RegisteredNatDuplexsHooksHolder struct {
	DoHold        bool
	HoldHookTasks *util.SafeSlice // []*_HoldHookTask
}
type _HoldTask int

const (
	_HoldTask_OnNatDuplexConnected    _HoldTask = iota + 1
	_HoldTask_OnNatDuplexDisConnected _HoldTask = iota + 1
)

type _HoldHookTask struct {
	taskType  _HoldTask
	natDuplex *NatDuplex
}

func newRegisteredNatDuplexs(localNatName string, holdHooks bool) *RegisteredNatDuplexs {
	return &RegisteredNatDuplexs{
		localNatName: localNatName,
		_duplexMap:   make(map[string]*NatDuplex),
		_hooksHolder: _RegisteredNatDuplexsHooksHolder{
			DoHold:        holdHooks,
			HoldHookTasks: util.NewSafeSlice(0),
		},
	}
}

func (rns *RegisteredNatDuplexs) BindRnsHooks(hooks RegisteredNatDuplexsHooks) {
	rns.hooks = hooks
	if hooks != nil {
		rns._ClearAndEmitHoldHooks(hooks)
	}
}

func (rns *RegisteredNatDuplexs) _ClearAndEmitHoldHooks(hooks RegisteredNatDuplexsHooks) {
	/// 清空缓存
	holdHooks := rns._hooksHolder.HoldHookTasks.Items()
	rns._hooksHolder.HoldHookTasks.Clear()
	for _, ht := range holdHooks {
		if rns.hooks == hooks {
			hookTask := ht.(*_HoldHookTask)
			if hookTask.taskType == _HoldTask_OnNatDuplexConnected {
				hooks.OnNatDuplexConnected(hookTask.natDuplex)
			} else if hookTask.taskType == _HoldTask_OnNatDuplexDisConnected {
				hooks.OnNatDuplexDisConnected(hookTask.natDuplex)
			}
		}
	}
}

func (rns *RegisteredNatDuplexs) _emitOnNatDuplexConnected(natDuplex *NatDuplex) {
	hooks := rns.hooks
	if hooks == nil {
		if rns._hooksHolder.DoHold {
			rns._hooksHolder.HoldHookTasks.Append(&_HoldHookTask{
				taskType:  _HoldTask_OnNatDuplexConnected,
				natDuplex: natDuplex,
			})
		}
	} else {
		hooks.OnNatDuplexConnected(natDuplex)
	}
}
func (rns *RegisteredNatDuplexs) _emitOnNatDuplexDisConnected(natDuplex *NatDuplex) {
	hooks := rns.hooks
	if hooks == nil {
		if rns._hooksHolder.DoHold {
			rns._hooksHolder.HoldHookTasks.Append(&_HoldHookTask{
				taskType:  _HoldTask_OnNatDuplexDisConnected,
				natDuplex: natDuplex,
			})
		}
	} else {
		hooks.OnNatDuplexDisConnected(natDuplex)
	}
}

func (rns *RegisteredNatDuplexs) CloseAllNatDuplex() {
	rns.lock.RLock()
	for _, natDuplex := range rns._duplexMap {
		natDuplex.Close()
	}
	rns.lock.RUnlock()
	rns.Clear()
}

func (rns *RegisteredNatDuplexs) Clear() {
	rns.lock.Lock()
	defer rns.lock.Unlock()
	rns._duplexMap = make(map[string]*NatDuplex)
	rns._hooksHolder.HoldHookTasks.Clear()
}

func (rns *RegisteredNatDuplexs) ChangeKey(oldKey string, natDuplex *NatDuplex) error {
	rns.lock.Lock()
	defer rns.lock.Unlock()

	duplex, found := rns._duplexMap[oldKey]
	if !found {
		return fmt.Errorf("nas(%s) do not have natDuplex(%s)", rns.localNatName, oldKey)
	}
	if duplex != natDuplex {
		return fmt.Errorf("natDuplex(%s) no belong to nas(%s)", natDuplex.DuplexKey, rns.localNatName)
	}

	delete(rns._duplexMap, oldKey)

	_, found = rns._duplexMap[natDuplex.RemoteNatName]
	if found {
		natDuplex.Close() // if already exist, close it
		return fmt.Errorf("nas(%s) already have natDuplex(%s)", rns.localNatName, natDuplex.RemoteNatName)
	}

	rns._duplexMap[natDuplex.RemoteNatName] = natDuplex
	return nil
}

func (rns *RegisteredNatDuplexs) Delete(natDuplex *NatDuplex) error {
	rns.lock.Lock()
	defer rns.lock.Unlock()
	duplex, found := rns._duplexMap[natDuplex.RemoteNatName]
	if !found {
		return fmt.Errorf("nas(%s) do not have natDuplex(%s)", rns.localNatName, natDuplex.RemoteNatName)
	}
	if duplex != natDuplex {
		return fmt.Errorf("natDuplex(%s) no belong to nas(%s)", natDuplex.DuplexKey, rns.localNatName)
	}
	delete(rns._duplexMap, natDuplex.RemoteNatName)

	// clear HoldHookTasks
	holdHooks := rns._hooksHolder.HoldHookTasks.Items()
	newHookTasks := util.NewSafeSlice(0)
	for _, hooktask := range holdHooks {
		hooktask := hooktask.(*_HoldHookTask)
		if hooktask.natDuplex != natDuplex {
			newHookTasks.Append(hooktask)
		}
	}
	rns._hooksHolder.HoldHookTasks = newHookTasks

	return nil
}

func (rns *RegisteredNatDuplexs) GetNatduplex(remoteNatName string) (*NatDuplex, error) {
	rns.lock.RLock()
	defer rns.lock.RUnlock()

	natDuplex, found := rns._duplexMap[remoteNatName]
	if found {
		return natDuplex, nil
	}
	return nil, fmt.Errorf("rns(%s) no found natDuplex(*->%s)", rns.localNatName, remoteNatName)
}

func (rns *RegisteredNatDuplexs) GetNames(filter func(natDuplex *NatDuplex) bool) []string {
	rns.lock.RLock()
	defer rns.lock.RUnlock()

	names := make([]string, 0)
	for _, natDuplex := range rns._duplexMap {
		if filter == nil || filter(natDuplex) {
			names = append(names, natDuplex.RemoteNatName)
		}
	}
	return names
}

func (rns *RegisteredNatDuplexs) HasNatDuplex(remoteNatName string) bool {
	rns.lock.RLock()
	defer rns.lock.RUnlock()

	return rns._duplexMap[remoteNatName] != nil
}

func (rns *RegisteredNatDuplexs) Belong(natDuplex *NatDuplex) bool {
	_natDuplex, err := rns.GetNatduplex(natDuplex.RemoteNatName)
	if err != nil {
		return false
	}
	return _natDuplex == natDuplex
}

func (rns *RegisteredNatDuplexs) Size() int {
	rns.lock.RLock()
	defer rns.lock.RUnlock()
	return len(rns._duplexMap)
}

func (rns *RegisteredNatDuplexs) Add(natDuplex *NatDuplex) error {
	rns.lock.Lock()
	defer rns.lock.Unlock()

	if natDuplex.RemoteNatName == "" || natDuplex.RemoteNatName == rns.localNatName {
		return fmt.Errorf("natDuplex(%s) remoteNatName is empty or equal to localNatName(%s)", natDuplex.RemoteNatName, rns.localNatName)
	}

	duplex, found := rns._duplexMap[natDuplex.RemoteNatName]
	if found {
		if duplex != natDuplex {
			return fmt.Errorf("rns(%s) already existed natDuplex(%s)", rns.localNatName, natDuplex.RemoteNatName)
		}
		return nil
	}
	rns._duplexMap[natDuplex.RemoteNatName] = natDuplex
	if natDuplex.LocalNatName != rns.localNatName {
		err := natDuplex.ChangeNatName(rns.localNatName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (rns *RegisteredNatDuplexs) SetLocalNatName(localNatName string) {
	rns.lock.Lock()
	defer rns.lock.Unlock()
	if rns.localNatName == localNatName {
		return
	}
	rns.localNatName = localNatName
	for _, natDuplex := range rns._duplexMap {
		if natDuplex.RemoteNatName == localNatName {
			natDuplex.Close() // if have same name, close it
		} else {
			err := natDuplex.ChangeNatName(rns.localNatName)
			if err != nil {
				log.Printf("change natDuplex(%s) localNatName (%s->%s) failed: %s\n", natDuplex.RemoteNatName, natDuplex.LocalNatName, rns.localNatName, err)
			}
		}
	}
}
