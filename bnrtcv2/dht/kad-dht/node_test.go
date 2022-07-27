package kaddht

import (
	"testing"

	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/stretchr/testify/assert"
)

func TestShortList(t *testing.T) {
	nl := newShortList(getIDWithValues(0))
	n1 := NewNode(&device_info.DeviceInfo{Id: getZerodIDWithNthByte(4, 1)})
	n2 := NewNode(&device_info.DeviceInfo{Id: getZerodIDWithNthByte(3, 1)})
	n3 := NewNode(&device_info.DeviceInfo{Id: getZerodIDWithNthByte(2, 1)})
	n4 := NewNode(&device_info.DeviceInfo{Id: getZerodIDWithNthByte(1, 1)})

	nl.AppendUnique([]*Node{n3, n2, n4, n1})

	nl.Sort()

	assert.Equal(t, n1, nl.At(0))
	assert.Equal(t, n2, nl.At(1))
	assert.Equal(t, n3, nl.At(2))
	assert.Equal(t, n4, nl.At(3))
}

func getZerodIDWithNthByte(n int, v byte) []byte {
	id := getIDWithValues(0)
	id[n] = v
	return id
}

func getIDWithValues(b byte) []byte {
	return []byte{b, b, b, b, b, b, b, b, b, b, b, b, b, b, b, b, b, b, b, b}
}
