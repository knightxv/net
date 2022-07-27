package dweb

import (
	"encoding/json"
	"log"

	"github.com/guabee/bnrtc/bnrtcv2/client"
	"github.com/guabee/bnrtc/bnrtcv2/message"
	"github.com/guabee/bnrtc/buffer"
)

type BnrtcTestClient struct {
	msgChan chan *client.DataMsgEvt
}

func NewBnrtcTestClient() IBnrtcClient {
	return &BnrtcTestClient{make(chan *client.DataMsgEvt)}
}
func (c *BnrtcTestClient) Send(src, dst, dPort, devid string, bytes []byte) error {
	c.msgChan <- &client.DataMsgEvt{SrcAddr: src, DstAddr: dst, Dport: dPort, DstDevId: devid, Buffer: buffer.FromBytes(bytes)}
	return nil
}
func (c *BnrtcTestClient) OnMessage(dport string, handler client.HandlerFunc) error {
	go func() {
		for {
			msgEvt := <-c.msgChan
			var req HttpProxyRequest
			err := json.Unmarshal(msgEvt.Buffer.Data(), &req)
			if err != nil {
				log.Println(err.Error())
				return
			}
			var res HttpProxyResponse
			if req.Path == "/" {
				resHeader := map[string]string{"text/html": "max-age=0"}
				res = HttpProxyResponse{req.ReqId, 200, resHeader, []byte("<div style=\"color: red\">hah</div>")}
			} else {
				res = HttpProxyResponse{ReqId: req.ReqId, StatusCode: 404}
			}
			data, _ := encodeDwebResponce(&res)
			msgEvt.Buffer = buffer.FromBytes(data)
			handler((*message.Message)(msgEvt))
		}
	}()
	return nil
}
func (c *BnrtcTestClient) OffMessage(dPort string, handler client.HandlerFunc) error {
	return nil
}
func (c *BnrtcTestClient) Close() {}
