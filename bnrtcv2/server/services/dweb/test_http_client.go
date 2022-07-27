package dweb

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/guabee/bnrtc/bnrtcv2/client"
	"github.com/guabee/bnrtc/bnrtcv2/message"
	"github.com/guabee/bnrtc/buffer"
)

type BnrtcHttpClient struct {
	msgChan chan *client.DataMsgEvt
}

func NewBnrtcHttpClient() IBnrtcClient {
	return &BnrtcHttpClient{make(chan *client.DataMsgEvt)}
}
func (c *BnrtcHttpClient) Send(src, dst, dPort, devid string, data []byte) error {
	reqUrl := "http://localhost:3000?address=" + dst + "&dPort=" + dPort
	res, err := http.Post(reqUrl, "application/octet-stream", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("BnrtcHttpClient send error: %s", err.Error())
		return err
	}
	result, _ := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	c.msgChan <- &client.DataMsgEvt{SrcAddr: src, DstAddr: dst, Dport: dPort, DstDevId: devid, Buffer: buffer.FromBytes(result)}
	return nil
}
func (c *BnrtcHttpClient) OnMessage(dport string, handler client.HandlerFunc) error {
	go func() {
		for {
			msgEvt := <-c.msgChan
			handler((*message.Message)(msgEvt))
		}
	}()
	return nil
}
func (c *BnrtcHttpClient) OffMessage(dPort string, handler client.HandlerFunc) error {
	return nil
}
func (c *BnrtcHttpClient) Close() {
}
