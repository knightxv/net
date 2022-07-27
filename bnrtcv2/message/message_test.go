package message_test

import (
	"testing"

	"github.com/guabee/bnrtc/bnrtcv2/message"
	"github.com/guabee/bnrtc/buffer"
)

func TestMessage(t *testing.T) {
	dport := "dport"
	srcAddr := "srcAddr"
	dstAddr := "dstAddr"
	srcDevid := "srcDevid"
	dstDevid := "dstDevid"
	buf := buffer.FromBytes([]byte("hello"))

	msg1 := message.NewMessage(dport, srcAddr, dstAddr, srcDevid, dstDevid, false, buf)
	msg2 := msg1.Clone()
	msg2.Buffer = msg1.Buffer.Copy()
	buff := msg1.GetBuffer()
	bytes := msg2.GetBytes()
	msg3 := message.FromBuffer(buff)
	if msg3.Dport != dport || msg3.SrcAddr != srcAddr || msg3.DstAddr != dstAddr || msg3.SrcDevId != srcDevid || msg3.DstDevId != dstDevid || string(msg3.Buffer.Data()) != string(buf.Data()) {
		t.Error("Failed to build message from buffer")
	}
	msg4 := message.FromBytes(bytes)
	if msg4.Dport != dport || msg4.SrcAddr != srcAddr || msg4.DstAddr != dstAddr || msg4.SrcDevId != srcDevid || msg4.DstDevId != dstDevid || string(msg4.Buffer.Data()) != string(buf.Data()) {
		t.Error("Failed to build message from bytes")
	}
	msg5 := message.FromBytes([]byte{1, 2, 3})
	if msg5 != nil {
		t.Error("Failed to build message from bytes")
	}
}
