package main

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/bootstrap"
	conf "github.com/guabee/bnrtc/bnrtcv2/config"

	"github.com/guabee/bnrtc/bnrtcv2/client"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/message"
	"github.com/stretchr/testify/assert"
)

type Record struct {
	SendCnt   int32
	RecvCnt   int32
	SendBytes int32
	RecvBytes int32
}

type Statis struct {
	controllerRecordMap map[string]*Record
	addressRecordMap    map[string]*Record
	dportRecordMap      map[string]*Record
}

func newStatis() *Statis {
	return &Statis{
		controllerRecordMap: make(map[string]*Record),
		addressRecordMap:    make(map[string]*Record),
		dportRecordMap:      make(map[string]*Record),
	}
}

func (s *Statis) RecordSend(dport, address, name string, bytes int) {
	atomic.AddInt32(&s.controllerRecordMap[name].SendCnt, 1)
	atomic.AddInt32(&s.controllerRecordMap[name].SendBytes, int32(bytes))
	atomic.AddInt32(&s.addressRecordMap[address].SendCnt, 1)
	atomic.AddInt32(&s.addressRecordMap[address].SendBytes, int32(bytes))
	atomic.AddInt32(&s.dportRecordMap[dport].SendCnt, 1)
	atomic.AddInt32(&s.dportRecordMap[dport].SendBytes, int32(bytes))
}

func (s *Statis) RecordRecv(dport, address, name string, bytes int) {
	atomic.AddInt32(&s.controllerRecordMap[name].RecvCnt, 1)
	atomic.AddInt32(&s.controllerRecordMap[name].RecvBytes, int32(bytes))
	atomic.AddInt32(&s.addressRecordMap[address].RecvCnt, 1)
	atomic.AddInt32(&s.addressRecordMap[address].RecvBytes, int32(bytes))
	atomic.AddInt32(&s.dportRecordMap[dport].RecvCnt, 1)
	atomic.AddInt32(&s.dportRecordMap[dport].RecvBytes, int32(bytes))
}

var recordStatis = newStatis()
var expectedStatis = newStatis()

func newController(t *testing.T, devid deviceid.DeviceId, port, servicePort int, addresses, peers, dports []string) client.ILocalChannel {
	config := conf.GetDefaultConfig()
	name := devid.String()
	config.Name = name
	config.Port = port
	config.ServicePort = servicePort
	config.ForceTopNetwork = devid.IsTopNetwork()
	config.Addresses = addresses
	config.Peers = peers
	config.DevId = name
	config.DefaultStunUrls = nil
	config.DefaultTurnUrls = nil
	server, _ := bootstrap.StartServer(config)
	c := server.NewLocalChannel(name)
	recordStatis.controllerRecordMap[name] = &Record{}
	expectedStatis.controllerRecordMap[name] = &Record{}
	addressMap := make(map[string]string)
	for _, address := range addresses {
		if recordStatis.addressRecordMap[address] == nil {
			recordStatis.addressRecordMap[address] = &Record{}
			expectedStatis.addressRecordMap[address] = &Record{}
		}
		addressMap[address] = address
	}
	for _, dport := range dports {
		if recordStatis.dportRecordMap[dport] == nil {
			recordStatis.dportRecordMap[dport] = &Record{}
			expectedStatis.dportRecordMap[dport] = &Record{}
		}
		_ = c.OnMessage(dport, func(msgEvt *message.Message) {
			if addressMap[msgEvt.DstAddr] == "" {
				t.Errorf("%s: unexpected address %s", name, msgEvt.DstAddr)
				return
			}
			t.Logf("%s: recv %s, %s %s %s", name, msgEvt.Dport, msgEvt.DstAddr, msgEvt.DstDevId, string(msgEvt.Buffer.Data()))
			recordStatis.RecordSend(msgEvt.Dport, msgEvt.SrcAddr, msgEvt.SrcDevId, len(msgEvt.Buffer.Data()))
			recordStatis.RecordRecv(msgEvt.Dport, msgEvt.DstAddr, name, len(msgEvt.Buffer.Data()))
		})
	}
	for _, address := range addresses {
		if recordStatis.addressRecordMap[address] == nil {
			recordStatis.addressRecordMap[address] = &Record{}
			expectedStatis.addressRecordMap[address] = &Record{}
		}
	}

	return c
}

func TestServer(t *testing.T) {
	peers := []string{"127.0.0.1:7771"}
	devid1 := deviceid.NewDeviceId(true, []byte{192, 168, 0, 1}, nil)
	devid2 := deviceid.NewDeviceId(false, []byte{192, 168, 0, 1}, nil)
	devid3 := deviceid.NewDeviceId(false, []byte{192, 168, 0, 1}, nil)
	devid1Str := devid1.String()
	// devid2Str := devid2.String()
	// devid3Str := devid3.String()
	addr1 := "a1"
	addr2 := "a2"
	addr3 := "a3"
	addr4 := "a4"
	dport1 := "p1"
	dport2 := "p2"
	dport3 := "p3"

	c1 := newController(t, devid1, 7770, 7771, []string{addr1, addr2}, peers, []string{dport1, dport2, dport3})
	_ = newController(t, devid2, 7780, 7781, []string{addr1, addr3}, peers, []string{dport1, dport2, dport3})
	_ = newController(t, devid3, 7790, 7791, []string{addr4}, peers, []string{dport1, dport2, dport3})

	// wait for all controllers to be ready
	time.Sleep(10 * time.Second)

	data := []byte("hello")
	dataLen := len(data)
	_ = c1.Send(addr1, addr2, dport1, "", data)
	expectedStatis.RecordSend(dport1, addr1, devid1Str, dataLen)
	expectedStatis.RecordRecv(dport1, addr2, devid1Str, dataLen)

	// c1.Send(addr1, addr3, dport1, "", data)
	// expectedStatis.RecordSend(dport1, addr1, devid1Str, dataLen)
	// expectedStatis.RecordRecv(dport1, addr3, devid2Str, dataLen)

	// c1.Send(addr1, addr4, dport1, "", data)
	// expectedStatis.RecordSend(dport1, addr1, devid1Str, dataLen)
	// expectedStatis.RecordRecv(dport1, addr4, devid3Str, dataLen)

	// wait for all messages to be sent
	time.Sleep(1 * time.Second)
	assert.Equal(t, expectedStatis, recordStatis)
}
