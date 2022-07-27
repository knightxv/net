package dweb

import (
	"testing"
	"time"

	"github.com/guabee/bnrtc/util"
)

func TestDwebServer(t *testing.T) {
	server := NewDwebServer(NewBnrtcTestClient())

	done := make(chan bool)
	time.AfterFunc(time.Second*2, func() {
		server.Stop()
		done <- true
	})
	_ = server.Start()
	<-done
}

func TestHttpServer(t *testing.T) {
	server := NewDwebServer(NewBnrtcHttpClient())
	done := make(chan bool)
	time.AfterFunc(time.Second*2, func() {
		server.Stop()
		done <- true
	})
	_ = server.Start()
	<-done
}

func TestHexAddress(t *testing.T) {
	hexAdd, _ := util.ToHexAddress("2et8jGb14fAfzQBNgLRHcwVxLd4Pif1mn")
	print(hexAdd)
	originAdd, _ := util.FromHexAddress(hexAdd)
	print(originAdd)
}
