package kaddht

import (
	"testing"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/stretchr/testify/assert"
)

func TestNetMessage(t *testing.T) {
	netMsgInit()
	sender := device_info.BuildDeviceInfo(newTransportAddress(newID(ID_TOP_NETWORK, ""), 3000))
	receiver := device_info.BuildDeviceInfo(newTransportAddress(newID(ID_TOP_NETWORK, ""), 3001))

	msg := newMessage(messageTypePing, NewNode(sender), NewNode(receiver), nil)
	data, err := serializeMessage(msg)
	if err != nil {
		t.Fatal(err)
	}
	msg2, err := deserializeMessage(data)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, msg, msg2)
}

func TestMessageChannel(t *testing.T) {
	num := 10
	mc := newMessageChannel(num)

	msg := newMessage(messageTypePing, nil, nil, nil)
	for i := 0; i < num*2; i++ {
		mc.Send(msg)
	}
	time.AfterFunc(1*time.Second, func() {
		mc.Close()
	})
	count := 0
	for m := range mc.GetMsgChannel() {
		count++
		assert.Equal(t, msg, m)
	}

	assert.Equal(t, num, count)
}
