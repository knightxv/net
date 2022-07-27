package fragmentation

import (
	"math/rand"
	"testing"
	"time"
)

func checksum(data []byte) byte {
	var c byte = data[0]
	for i := 1; i < len(data); i++ {
		c ^= data[i]
	}
	return c
}
func packAndUnpack(t *testing.T, _len int32) {

	t.Logf("testing packetize with length= %d", _len)
	frag := NewFragmentation(&FragmentOption{
		MaxFragmentChunkSize: 1000,
		ExpiredTimeSeconds:   30,
	})

	data := make([]byte, _len+1)
	for i := range data {
		data[i] = byte(rand.Int())
	}
	fd := frag.Packetize(data)
	cs := checksum(data)

	data2 := make([]byte, _len+10)
	for i := range data2 {
		data2[i] = byte(rand.Int())
	}
	fd2 := frag.Packetize(data2)
	cs2 := checksum(data2)

	allfd := append(fd, fd2...)

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(allfd), func(i, j int) {
		allfd[i], allfd[j] = allfd[j], allfd[i]
	})
	cnt := 0
	for _, v := range allfd {
		recv := frag.UnPacketize(v.GetBytes())
		if recv != nil {
			cr := checksum(recv)
			if cr != cs && cr != cs2 {
				t.Fail()
			}
			if len(recv) != len(data) && len(recv) != len(data2) {
				t.Fail()
			}
			cnt++
		}
	}

	if cnt != 2 {
		t.Fail()
	}
}

func TestPacketizeUnchunked(t *testing.T) {
	t.Log("testing packet unchunked")
	packAndUnpack(t, 10)
}
func TestPacketizeChunked(t *testing.T) {
	t.Log("testing packet chunked")
	packAndUnpack(t, 5000)
}

func TestPacketizeChunked2(t *testing.T) {
	t.Log("testing packet chunked")
	packAndUnpack(t, 12000)
}

func TestPacketizeChunked3(t *testing.T) {
	t.Log("testing packet chunked")
	packAndUnpack(t, 1300)
}
