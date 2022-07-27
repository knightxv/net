package bnrtcv1

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/guabee/bnrtc/util"
	"github.com/pion/webrtc/v3"
)

type NatDuplexRole int8

const (
	NatDuplexRoleOffer NatDuplexRole = iota + 1
	NatDuplexRoleAnswer
)

const (
	ioChannelLabel   = "io"
	ctrlChannelLabel = "ctrl"
)

type NatDuplexFactory struct {
	reqManager *util.ReqRespManager
	started    bool
}

func NewNatDuplexFactory() *NatDuplexFactory {
	return &NatDuplexFactory{
		reqManager: util.NewReqRespManager(util.WithReqRespTimeout(30 * time.Second)),
	}
}

func (f *NatDuplexFactory) Start() {
	if f.started {
		return
	}
	f.started = true
	f.reqManager.Start()
}

func (f *NatDuplexFactory) Stop() {
	if !f.started {
		return
	}
	f.started = false
	f.reqManager.Close()
}

func (f *NatDuplexFactory) New(pc_config *webrtc.Configuration, source string, localNatName string, remoteNatName string, role NatDuplexRole, duplexKey string) (*NatDuplex, error) {
	if !f.started {
		return nil, errors.New("NatDuplexFactory not started")
	}

	peerConnection, err := webrtc.NewPeerConnection(*pc_config)
	if err != nil {
		return nil, err
	}

	d := &NatDuplex{
		pc:             peerConnection,
		Source:         source,
		LocalNatName:   localNatName,
		RemoteNatName:  remoteNatName,
		Role:           role,
		DuplexKey:      duplexKey,
		_ioReady:       make(chan error, 1),
		_ctrlReady:     make(chan error, 1),
		ioMessageSlice: util.NewSafeLimitSlice(1024),
		reqController:  f.reqManager.NewController(duplexKey),
	}
	d.autoSaveDataChannel()
	return d, nil
}

type NatDuplex struct {
	Source                  string        `json:"source"`
	LocalNatName            string        `json:"localNatName"`
	RemoteNatName           string        `json:"remoteNatName"`
	DuplexKey               string        `json:"duplexKey"`
	Role                    NatDuplexRole `json:"role"`
	pc                      *webrtc.PeerConnection
	_ioReady                chan error
	_ctrlReady              chan error
	hooks                   NatDuplexHooks
	ioDc                    *webrtc.DataChannel
	ctrlDc                  *webrtc.DataChannel
	ioMessageSlice          *util.SafeSlice
	onRemoteNameChangedhook func(oldname, newName string)
	reqController           *util.ReqRespConroller
}

func (natDuplex *NatDuplex) WaitReady() error {
	readyCond := util.NewCondition(util.WithConditionTimeout(5*time.Second), util.WithConditionWaitCount(2))
	ctx, cancel := context.WithCancel(context.Background())
	go func(ctx context.Context) {
		ioRready := natDuplex._ioReady
		ctrlReady := natDuplex._ctrlReady
		for {
			select {
			case err := <-ioRready:
				readyCond.Signal(err)
				if err != nil {
					return
				}
				ioRready = nil // prevent double signal
			case err := <-ctrlReady:
				readyCond.Signal(err)
				if err != nil {
					return
				}
				ctrlReady = nil // prevent double signal
			case <-ctx.Done():
				return
			}
		}
	}(ctx)
	err := readyCond.Wait()
	cancel()
	return err
}

func (natDuplex *NatDuplex) OnRemoteNameChanged(handle func(oldname, newName string)) {
	natDuplex.onRemoteNameChangedhook = handle
}

func (natDuplex *NatDuplex) onRemoteNameChanged(newName string) {
	oldName := natDuplex.RemoteNatName
	if newName != oldName {
		natDuplex.RemoteNatName = newName
		handle := natDuplex.onRemoteNameChangedhook
		if handle != nil {
			handle(oldName, newName)
		}

		hooks := natDuplex.hooks
		if hooks != nil {
			hooks.OnRemoteNameChanged(oldName, newName)
		}
	}
}

func afterDataChannelOpened(dc *webrtc.DataChannel) error {
	state := dc.ReadyState()
	if state == webrtc.DataChannelStateClosed || state == webrtc.DataChannelStateClosing {
		return fmt.Errorf("dataChannel(%s) is %s", dc.Label(), state.String())
	}

	afterDcOpenedCond := util.NewCondition(util.WithConditionTimeout(time.Second * 5))
	dc.OnOpen(func() {
		afterDcOpenedCond.Signal(nil)
	})
	dc.OnError(func(err error) {
		afterDcOpenedCond.Signal(err)
	})
	return afterDcOpenedCond.Wait()
}

func (natDuplex *NatDuplex) CreateDataChannels() {
	natDuplex.createDataChannel(ioChannelLabel, natDuplex._ioReady)
	natDuplex.createDataChannel(ctrlChannelLabel, natDuplex._ctrlReady)
}

func (natDuplex *NatDuplex) createDataChannel(label string, ready chan<- error) {
	dc, err := natDuplex.pc.CreateDataChannel(label, nil)
	if err != nil {
		ready <- err
		close(ready)
		return
	}
	natDuplex.saveDataChannel(dc)
}

func (natDuplex *NatDuplex) saveIoDataChannel(dc *webrtc.DataChannel, dcReady chan<- error) error {
	if natDuplex.ioDc != nil {
		return fmt.Errorf("%s(%s) already exist channel %s", natDuplex.LocalNatName, natDuplex.DuplexKey, natDuplex.ioDc.Label())
	}
	natDuplex.ioDc = dc
	release := func() {
		natDuplex.ioDc = nil
		natDuplex.pc.Close()
	}
	go func() {
		openErr := afterDataChannelOpened(dc)
		if dcReady != nil {
			dcReady <- openErr
			close(dcReady)
		}
		if openErr != nil {
			release()
		}
	}()

	dc.OnClose(release)

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		hooks := natDuplex.hooks
		if hooks != nil {
			hooks.OnMessage(msg.Data, msg.IsString)
		} else {
			_ = natDuplex.ioMessageSlice.Append(msg)
		}
	})

	log.Printf("%s(%s) save dataChannel %s", natDuplex.LocalNatName, natDuplex.DuplexKey, dc.Label())
	return nil
}

func (natDuplex *NatDuplex) saveCtrlDataChannel(dc *webrtc.DataChannel, dcReady chan<- error) error {
	if natDuplex.ctrlDc != nil {
		return fmt.Errorf("already exist channel %s", natDuplex.ctrlDc.Label())
	}

	natDuplex.ctrlDc = dc
	release := func() {
		natDuplex.ctrlDc = nil
		natDuplex.pc.Close()
	}

	go func() {
		openErr := afterDataChannelOpened(dc)
		if dcReady != nil {
			dcReady <- openErr
			close(dcReady)
		}
		if openErr != nil {
			release()
		}
	}()
	dc.OnClose(release)
	/// 作为中介，转发数据
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		ctrlMessage := &CtrlMessageHeader{}
		err := ctrlMessage.fromBytes(msg.Data)
		if err != nil {
			log.Println(err)
			return
		}
		if ctrlMessage.IsReq {
			switch ctrlMessage.CtrlMessageType {
			case CtrlMessageSetNatName:
				natNameMessage := &CtrlNatName{}
				err := natNameMessage.fromBytes(msg.Data)
				if err != nil {
					log.Println(err)
					return
				}
				natDuplex.onRemoteNameChanged(natNameMessage.Name)
			}
		}
	})
	return nil
}

func (natDuplex *NatDuplex) saveDataChannel(dc *webrtc.DataChannel) {
	var err error
	var ready chan<- error = nil
	switch dc.Label() {
	case ioChannelLabel:
		ready = natDuplex._ioReady
		err = natDuplex.saveIoDataChannel(dc, ready)
	case ctrlChannelLabel:
		ready = natDuplex._ctrlReady
		err = natDuplex.saveCtrlDataChannel(dc, ready)
	default:
		err = fmt.Errorf("unknown dataChannel label %s", dc.Label())
	}
	if err != nil {
		log.Println(err)
		if ready != nil {
			ready <- err
			close(ready)
		}
	}
}

func (natDuplex *NatDuplex) autoSaveDataChannel() {
	natDuplex.pc.OnDataChannel(natDuplex.saveDataChannel)
}

func (natDuplex *NatDuplex) getCtrlDc() *webrtc.DataChannel {
	return natDuplex.ctrlDc
}

func (natDuplex *NatDuplex) getIoDc() *webrtc.DataChannel {
	return natDuplex.ioDc
}

func (natDuplex *NatDuplex) GetUrl() string {
	return natDuplex.Source
}

func (natDuplex *NatDuplex) GetLocalNatName() string {
	return natDuplex.LocalNatName
}

func (natDuplex *NatDuplex) GetRemoteNatName() string {
	return natDuplex.RemoteNatName
}

func (natDuplex *NatDuplex) ChangeNatName(newName string) error {
	dc := natDuplex.getCtrlDc()
	if dc == nil {
		return errors.New("no ctrl dataChannel")
	}

	msg := NewCtrlNatNameMessage(newName)
	data, err := msg.toBytes()
	if err != nil {
		return err
	}

	return dc.Send(data)
}

func (natDuplex *NatDuplex) Send(data []byte, isString bool) error {
	if isString {
		return natDuplex.SendText(string(data))
	} else {
		return natDuplex.SendBytes(data)
	}
}

func (natDuplex *NatDuplex) SendBytes(data []byte) error {
	dc := natDuplex.getIoDc()
	if dc == nil {
		return errors.New("no io dataChannel")
	}

	return dc.Send(data)
}

func (natDuplex *NatDuplex) SendText(text string) error {
	dc := natDuplex.getIoDc()
	if dc == nil {
		return errors.New("no io dataChannel")
	}
	return dc.SendText(text)
}

func (natDuplex *NatDuplex) Close() error {
	return natDuplex.pc.Close()
}

func (natDuplex *NatDuplex) onClose() {
	natDuplex.reqController.Close()
	if natDuplex.hooks != nil {
		natDuplex.hooks.OnClose()
	}
}

type NatDuplexHooks interface {
	OnMessage(data []byte, isString bool)
	OnRemoteNameChanged(oldName, newName string)
	OnClose()
}

func (natDuplex *NatDuplex) BindHooks(cbs NatDuplexHooks) {
	if cbs == nil {
		return
	}
	natDuplex.hooks = cbs
	for _, _msg := range natDuplex.ioMessageSlice.Items() {
		msg := _msg.(webrtc.DataChannelMessage)
		cbs.OnMessage(msg.Data, msg.IsString)
	}
	natDuplex.ioMessageSlice.Clear()
}
