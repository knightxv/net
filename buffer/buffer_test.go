package buffer_test

import (
	"testing"
	"unsafe"

	"github.com/guabee/bnrtc/buffer"
	"github.com/stretchr/testify/assert"
)

func TestBuffer(t *testing.T) {
	u8 := uint8(10)
	u16 := uint16(65533)
	u32 := uint32(65566)
	u64 := uint64(^uint64(0))
	str := "我不是我，布拉all哆啦我来顶啦我来到了dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddwdadawdawdawdfergwtreh"
	data := make([]byte, 256)
	for i := range data {
		data[i] = 1
	}

	buf := buffer.NewBuffer(0, nil)

	buf.PutString(str)
	buf.Put(data)
	buf.PutU8(u8)
	buf.PutU16(u16)
	buf.PutU32(u32)
	buf.PutU64(u64)

	buf.PushString(str)
	buf.Push(data)
	buf.PushU8(u8)
	buf.PushU16(u16)
	buf.PushU32(u32)
	buf.PushU64(u64)

	u64_1 := buf.PullU64()
	if u64_1 != u64 {
		t.Error("u64 pull")
	}
	u32_1 := buf.PullU32()
	if u32_1 != u32 {
		t.Error("u32 pull")
	}
	u16_1 := buf.PullU16()
	if u16_1 != u16 {
		t.Error("u16 pull")
	}
	u8_1 := buf.PullU8()
	if u8_1 != u8 {
		t.Error("u8 pull")
	}
	data_1 := buf.Pull()
	for i, d := range data_1 {
		if d != data[i] {
			t.Error("data pull")
		}
	}
	str_1 := buf.PullString()
	if str_1 != str {
		t.Error("str pull")
	}
	str_2 := buf.PullString()
	if str_2 != str {
		t.Error("str2 pull")
	}
	data_2 := buf.Pull()
	for i, d := range data_2 {
		if d != data[i] {
			t.Error("data2 pull")
		}
	}
	u8_2 := buf.PullU8()
	if u8_2 != u8 {
		t.Error("u82 pull")
	}
	u16_2 := buf.PullU16()
	if u16_2 != u16 {
		t.Error("u162 pull")
	}
	u32_2 := buf.PullU32()
	if u32_2 != u32 {
		t.Error("u322 pull")
	}
	u64_2 := buf.PullU64()
	if u64_2 != u64 {
		t.Error("u642 pull")
	}
}

func TestClone(t *testing.T) {
	buf := buffer.NewBuffer(0, nil)
	buf.PutU8(1)
	buf2 := buf.Clone()
	assert.Equal(t, unsafe.Pointer(&buf.DataView()[0]), unsafe.Pointer(&buf2.DataView()[0]))
	buf.PutU8(2)
	assert.NotEqual(t, unsafe.Pointer(&buf.DataView()[0]), unsafe.Pointer(&buf2.DataView()[0]))
	oldPoint := unsafe.Pointer(&buf.DataView()[0])
	buf2.PutU8(3)
	assert.Equal(t, unsafe.Pointer(&buf.DataView()[0]), oldPoint)

	buf3 := buf.Clone()
	buf3.PutU8(4)
	assert.NotEqual(t, unsafe.Pointer(&buf.DataView()[1]), unsafe.Pointer(&buf3.DataView()[0]))
	buf.PutU8(5)

	assert.Equal(t, buf.PullU8(), uint8(5))
	assert.Equal(t, buf.PullU8(), uint8(2))
	assert.Equal(t, buf.PullU8(), uint8(1))
	assert.Equal(t, buf.PullU8(), ^uint8(0))
	assert.Equal(t, buf2.PullU8(), uint8(3))
	assert.Equal(t, buf2.PullU8(), uint8(1))
	assert.Equal(t, buf2.PullU8(), ^uint8(0))

	assert.Equal(t, buf3.PullU8(), uint8(4))
	assert.Equal(t, buf3.PullU8(), uint8(2))
	assert.Equal(t, buf3.PullU8(), uint8(1))
	assert.Equal(t, buf3.PullU8(), ^uint8(0))
}

func TestCopy(t *testing.T) {
	buf := buffer.NewBuffer(0, nil)
	buf.PutU8(1)
	buf2 := buf.Copy()
	assert.NotEqual(t, unsafe.Pointer(&buf.Data()[0]), unsafe.Pointer(&buf2.Data()[0]))

	buf2.PutU8(4)
	buf.PutU8(2)
	buf2.PushU8(5)
	buf.PushU8(3)
	assert.Equal(t, buf.PullU8(), uint8(2))
	assert.Equal(t, buf.PullU8(), uint8(1))
	assert.Equal(t, buf.PullU8(), uint8(3))
	assert.Equal(t, buf.PullU8(), ^uint8(0))
	assert.Equal(t, buf2.PullU8(), uint8(4))
	assert.Equal(t, buf2.PullU8(), uint8(1))
	assert.Equal(t, buf2.PullU8(), uint8(5))
	assert.Equal(t, buf2.PullU8(), ^uint8(0))
}

func TestData(t *testing.T) {
	buf := buffer.NewBuffer(0, nil)
	buf.PutU8(1)
	data := buf.Data()
	dataView := buf.DataView()
	assert.NotEqual(t, unsafe.Pointer(&data[0]), unsafe.Pointer(&dataView[0]))

	buf = buffer.NewBuffer(0, nil)
	d := make([]byte, 256*7/10)
	buf.Put(d)
	data = buf.Data()
	dataView = buf.DataView()
	assert.NotEqual(t, unsafe.Pointer(&data[0]), unsafe.Pointer(&dataView[0]))

	buf = buffer.NewBuffer(0, nil)
	d = make([]byte, 256*8/10)
	buf.Put(d)
	data = buf.Data()
	dataView = buf.DataView()
	assert.Equal(t, unsafe.Pointer(&data[0]), unsafe.Pointer(&dataView[0]))
}

func TestShrink(t *testing.T) {
	buf := buffer.NewBuffer(0, nil)
	buf.PutU8(1)
	buf = buf.Buffer()
	assert.Equal(t, int(buf.Len()), 1)

	buf = buffer.NewBuffer(0, nil)
	dl := 256 * 7 / 10
	d := make([]byte, dl)
	buf.Put(d)
	buf = buf.Buffer()
	assert.Equal(t, int(buf.Len()), dl+4)

	buf = buffer.NewBuffer(0, nil)
	dl = 256 * 8 / 10
	d = make([]byte, dl)
	buf.Put(d)
	buf = buf.Buffer()
	assert.NotEqual(t, int(buf.Len()), dl)
}

func TestBufferWithError(t *testing.T) {
	buf := buffer.NewBuffer(0, nil)
	buf.PutU8(1)
	buf.PutU8(2)

	errorBuffer := buffer.NewBufferWithError(buf)
	d := errorBuffer.PullU8()
	assert.Equal(t, d, uint8(2))
	assert.Equal(t, errorBuffer.IsError(), false)

	_ = errorBuffer.PullU16()
	assert.Equal(t, errorBuffer.IsError(), true)

}
