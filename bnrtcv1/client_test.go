package bnrtcv1

import (
	"log"
	"testing"
)

type TestRnsHooks struct {
	// rns RegisteredNatDuplex
	duplexChan chan *NatDuplex
}

func (hook *TestRnsHooks) OnNatDuplexConnected(natDuplex *NatDuplex) {
	log.Println("server p2p connected", natDuplex.DuplexKey)
	hook.duplexChan <- natDuplex
}
func (hook *TestRnsHooks) OnNatDuplexDisConnected(natDuplex *NatDuplex) {
	log.Println("server p2p disconnected", natDuplex.DuplexKey)
}

// type ClientHook struct {
// 	msgChan chan string
// }
type TestNatDuplexHooks struct {
	msgChan chan string
}

func (l *TestNatDuplexHooks) OnMessage(data []byte, isString bool) {
	l.msgChan <- string(data)
}
func (l *TestNatDuplexHooks) OnClose() {

}
func (l *TestNatDuplexHooks) OnRemoteNameChanged(oldName, newName string) {

}

// func (hook *ClientHook) OnNatDuplexConnected(natDuplex *NatDuplex) {
// 	log.Println("client p2p connected", natDuplex.DuplexKey)
// 	_, err := natDuplex.getIoDc()
// 	if err != nil {
// 		panic(err)
// 	}
// 	natDuplex.BindHooks(&ClientHookDcListener{
// 		msgChan: hook.msgChan,
// 	})

// }
// func (hook *ClientHook) OnNatDuplexDisConnected(natDuplex *NatDuplex) {
// 	log.Println("client p2p disconnected", natDuplex.DuplexKey)
// 	hook.msgChan <- "disconnected"
// }
func TestRegisterServer(t *testing.T) {
	server, err := NewServer(&Bnrtc1ServerOptions{Name: "ss", HttpPort: 29103, TurnPort: 29302, StunPort: 29403}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = server.Start()

	clientA, _ := NewClient(nil, &Bnrtc1ClientConfig{Name: "ca"})
	clientA.Start()
	// 建立直连
	_, err = clientA.ConnectSignalServer(&InternetPeerOptions{
		Address: server.Config.PublicIp,
		Port:    int32(server.Config.HttpPort),
	})
	if err != nil {
		t.Fatal(err)
	}
	clientAHook := &TestRnsHooks{
		duplexChan: make(chan *NatDuplex),
	}

	if err := clientA.BindClientRnsHooks(clientAHook); err != nil {
		t.Fatal(err)
	}

	// 客户端B
	clientB, _ := NewClient(nil, &Bnrtc1ClientConfig{Name: "cb", HoldHooks: true})
	clientB.Start()
	// 使用穿透服务连接到客户端A
	for {
		// time.Sleep(20000) // 确保ca已经注册到ss了
		duplexNat2, err := clientB.ConnectNatPeer(&InternetPeerOptions{
			Address: server.Config.PublicIp,
			Port:    int32(server.Config.HttpPort),
		}, "ca")
		if err != nil {
			t.Log("no found ca~~,try~~")
			continue
		}
		_ = duplexNat2.SendText("hi")
		_ = duplexNat2.SendText("gaubee")
		t.Logf("%s sended msg", duplexNat2.DuplexKey)
		break
	}

	duplexNat_cA1 := <-clientAHook.duplexChan
	log.Printf("duplexNat_cA1.ioDc: %v\n", duplexNat_cA1.ioDc)
	hook := &TestNatDuplexHooks{
		msgChan: make(chan string, 1),
	}
	duplexNat_cA1.BindHooks(hook)

	// duplexNat2.SendText("mmm")
	// duplexNat2.SendText("777")

	// t.Logf("duplex(%s(%s)).hooks: %v", duplexNat1.DuplexKey, duplexNat1.LocalNatName, duplexNat1.hooks)

	resMsg1 := <-hook.msgChan
	t.Logf("got msg resMsg1 %s", resMsg1)
	resMsg2 := <-hook.msgChan
	t.Logf("got msg resMsg2 %s", resMsg2)
	if resMsg1 != "hi" || resMsg2 != "gaubee" {
		t.Fatal(resMsg1 + resMsg2)
	}
}

func TestErrorChan(t *testing.T) {
	cc := make(chan error, 1)
	go func() {
		cc <- nil
		close(cc)
	}()

	t.Logf("res %v", <-cc)
	t.Logf("res %v", <-cc)
}
