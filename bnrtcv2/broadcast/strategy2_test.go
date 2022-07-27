package broadcast_test

import (
	"fmt"
	"testing"

	"github.com/guabee/bnrtc/util"
)

func TestStrategy2(t *testing.T) {
	// var a uint8 = 0x02
	// var b int = int(math.Pow(2, 1))
	// fmt.Printf("%d\n", b)
	// fmt.Printf("%08b\n", a)
	// fmt.Printf("%08b\n", b)
	// fmt.Printf("%08b\n", a^uint8(b))

	ttl := 12 + 1
	broadcastDeviceId := []byte{0, 10} // 1010
	receiveDeviceId := []byte{0, 14}   // 1110
	fmt.Printf(
		"broadcastDeviceId:%08b\n receiveDeviceId:%08b\n",
		broadcastDeviceId[1],
		receiveDeviceId[1],
	)
	for i := ttl + 1; i < len(broadcastDeviceId)*8; i++ {
		index := i / 8
		offset := i % 8
		xor := receiveDeviceId[index] ^ broadcastDeviceId[index]
		for j := offset; j < 8; j++ {
			if util.HasBit(xor, uint(j)) {
				continue
			}
			bitIndex := (receiveDeviceId[index] ^ (1 << (8 - j - 1)))

			fmt.Printf(
				"targetID:%08b\n ttl:%d\n i:%d\n j:%d\n",
				bitIndex,
				i+j,
				i,
				j,
			)
		}
	}
}
