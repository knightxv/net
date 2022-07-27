package bnrtcv1

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/pion/webrtc/v3"
)

type PublicIp string

func TestHttpServer(t *testing.T) {
	t.Log("test http server")
	// publicIp := "172.30.93.161"
	// disabled := false
	// enabled := true
	options := Bnrtc1ServerOptions{
		// PublicIp:          &publicIp,
		DisableStunServer: true,
		DisableTurnServer: true,
		DisableHttpServer: false,
	}
	server, _ := NewServer(&options, nil)
	if err := server.Start(); err != nil {
		t.Error(err)
	}

	// log.Println("start servers")
	if resp, err := http.Get(`http://127.0.0.1:` + strconv.Itoa(int(server.Config.HttpPort)) + `/ip`); err != nil {
		t.Error(err)
	} else {
		if ipbytes, err := io.ReadAll(resp.Body); err != nil {
			t.Error(err)
		} else {
			if string(ipbytes) != "127.0.0.1" {
				t.Error(string(ipbytes))
			}
		}
	}

	_ = server.Stop()
}

func TestTurnServer(t *testing.T) {
	t.Log("test turn server")
	// publicIp := "172.30.93.161"
	// disabled := false
	// enabled := true
	options := Bnrtc1ServerOptions{
		// PublicIp:          &publicIp,
		DisableStunServer: true,
		DisableTurnServer: false,
		DisableHttpServer: true,
	}
	server, _ := NewServer(&options, nil)
	if err := server.Start(); err != nil {
		t.Error(err)
	}

	rtc_config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{fmt.Sprintf("turn:%s:%d", server.Config.PublicIp, server.Config.TurnPort)},
				// URLs: []string{"stun:stun1.l.google.com:19302"},
				Username:   "gaubee",
				Credential: "pwd",
			},
		},
	}
	p1, err := webrtc.NewPeerConnection(rtc_config)
	if err != nil {
		t.Error(err)
	}
	c1, _ := p1.CreateDataChannel("test turn", nil)
	ch := make(chan interface{}, 1)
	c1.OnOpen(func() {
		_ = c1.SendText("success")
	})

	p2, _ := webrtc.NewPeerConnection(rtc_config)
	p2.OnDataChannel(func(c2 *webrtc.DataChannel) {
		c2.OnMessage(func(msg webrtc.DataChannelMessage) {
			ch <- string(msg.Data)
		})
	})
	p2.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		if pcs == webrtc.PeerConnectionStateFailed {
			ch <- "fail"
			t.Fail()
		}
	})

	offer, _ := p1.CreateOffer(nil)
	_ = p1.SetLocalDescription(offer)

	_ = p2.SetRemoteDescription(offer)
	answer, _ := p2.CreateAnswer(nil)
	_ = p2.SetLocalDescription(answer)

	<-webrtc.GatheringCompletePromise(p2)

	p2l := p2.LocalDescription()
	_ = p1.SetRemoteDescription(*p2l)

	<-ch
}

func TestStunServer(t *testing.T) {
	t.Log("test stun server")
	options := Bnrtc1ServerOptions{
		// PublicIp:          "172.30.93.161",
		DisableStunServer: false,
		DisableTurnServer: true,
		DisableHttpServer: true,
	}
	server, _ := NewServer(&options, nil)
	if err := server.Start(); err != nil {
		t.Error(err)
	}

	rtc_config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{fmt.Sprintf("stun:%s:%d", server.Config.PublicIp, server.Config.StunPort)},
			},
		},
	}
	p1, err := webrtc.NewPeerConnection(rtc_config)
	if err != nil {
		t.Error(err)
	}
	c1, _ := p1.CreateDataChannel("test stun", nil)
	ch := make(chan interface{}, 1)
	c1.OnOpen(func() {
		_ = c1.SendText("success")
	})

	p2, _ := webrtc.NewPeerConnection(rtc_config)
	p2.OnDataChannel(func(c2 *webrtc.DataChannel) {
		c2.OnMessage(func(msg webrtc.DataChannelMessage) {
			ch <- string(msg.Data)
		})
	})
	p2.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		if pcs == webrtc.PeerConnectionStateFailed {
			ch <- "fail"
			t.Fail()
		}
	})

	offer, _ := p1.CreateOffer(nil)
	_ = p1.SetLocalDescription(offer)
	_ = p2.SetRemoteDescription(offer)
	answer, _ := p2.CreateAnswer(nil)
	_ = p2.SetLocalDescription(answer)

	<-webrtc.GatheringCompletePromise(p2)

	p2l := p2.LocalDescription()
	_ = p1.SetRemoteDescription(*p2l)

	<-ch
}
