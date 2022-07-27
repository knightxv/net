package buffer

import (
	"encoding/binary"
	"errors"

	"github.com/guabee/bnrtc/util"
)

type WriteDirection int8

const (
	PUT  WriteDirection = -1
	PUSH WriteDirection = 1
)
const HeadReserveSize = 256
const MaxBufferLen = ^uint32(0)
const ShrinkRatio = 0.75

const (
	InvalidU8  = ^uint8(0)
	InvalidU16 = ^uint16(0)
	InvalidU32 = ^uint32(0)
	InvalidU64 = ^uint64(0)
)

func nextpow2(n uint32) uint32 {
	if n >= MaxBufferLen { // MaxUint32
		return n
	}

	n = n - 1 // 这个操作决定当n刚好是2的次幂值时，取n，如果没有，则取2n
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n += 1
	return n
}

type BufferData struct {
	buffer  []byte
	readers int32
}

func (d *BufferData) Len() int {
	return len(d.buffer)
}

func (d *BufferData) Cap() int {
	return cap(d.buffer)
}

type Buffer struct {
	peeker     func(size uint32, offset uint32) ([]byte, error)
	data       *BufferData
	dataOffset uint32
	endOffset  uint32
	isFreezed  bool
}

func IsValidU8(v uint8) bool {
	return v != InvalidU8
}

func IsValidU16(v uint16) bool {
	return v != InvalidU16
}

func IsValidU32(v uint32) bool {
	return v != InvalidU32
}

func IsValidU64(v uint64) bool {
	return v != InvalidU64
}

// read only
func FromBytes(data []byte) *Buffer {
	return &Buffer{
		data: &BufferData{
			buffer:  data,
			readers: 1,
		},
		dataOffset: 0,
		endOffset:  uint32(len(data)),
		isFreezed:  true,
	}
}

func DataLenSize(data []byte) uint32 {
	return uint32(len(data) + 4) // 4 for len field
}

func StringLenSize(str string) uint32 {
	data := util.Str2bytes(str)
	return uint32(len(data) + 4) // 4 for len field
}

func NewBuffer(size uint32, data []byte) *Buffer {
	dataLen := uint32(len(data))
	if dataLen > 0 && size < dataLen {
		size = dataLen
	}

	bufLen := nextpow2(size + HeadReserveSize)
	buf := &Buffer{
		data: &BufferData{
			buffer:  make([]byte, bufLen),
			readers: 1,
		},
		dataOffset: HeadReserveSize,
		endOffset:  HeadReserveSize,
		isFreezed:  false,
	}

	if dataLen > 0 {
		copy(buf.data.buffer[buf.endOffset:], data)
		buf.endOffset += dataLen
	}

	return buf
}

func (buf *Buffer) DataView() []byte {
	if buf == nil {
		return nil
	}
	// not copy, just view
	return buf.data.buffer[buf.dataOffset:buf.endOffset]
}

// copy it, release the delay slice buffer
func (buf *Buffer) DataCopy() []byte {
	if buf == nil {
		return nil
	}
	d := make([]byte, buf.Len())
	copy(d, buf.data.buffer[buf.dataOffset:buf.endOffset])
	return d
}

func (buf *Buffer) Data() []byte {
	if buf.inLowUsage() {
		return buf.DataCopy()
	} else {
		return buf.DataView()
	}
}

func (buf *Buffer) Buffer() *Buffer {
	if buf.inLowUsage() {
		return FromBytes(buf.DataCopy())
	}

	return buf
}

// clone buffer is read(pull/peek) only
func (buf *Buffer) Clone() *Buffer {
	if buf == nil {
		return nil
	}

	newBuf := &Buffer{
		data:       buf.data,
		dataOffset: buf.dataOffset,
		endOffset:  buf.endOffset,
		isFreezed:  buf.isFreezed,
	}
	newBuf.data.readers += 1
	return newBuf
}

func (buf *Buffer) Copy() *Buffer {
	return NewBuffer(buf.Len(), buf.DataView())
}

func (buf *Buffer) Len() uint32 {
	if buf == nil {
		return 0
	}
	return buf.endOffset - buf.dataOffset
}

func (buf *Buffer) Cap() uint32 {
	if buf == nil {
		return 0
	}
	return uint32(buf.data.Cap())
}

func (buf *Buffer) inLowUsage() bool {
	return float64(buf.Len()) < float64(buf.Cap())*ShrinkRatio
}

func (buf *Buffer) Shrink() *Buffer {
	if buf == nil {
		return nil
	}
	return FromBytes(buf.DataCopy())
}

func (buf *Buffer) extendBuffer(size uint32, direction WriteDirection) {
	oldData := buf.DataView()
	oldDataLen := buf.Len()
	buf.data.readers -= 1
	newLen := nextpow2(size + oldDataLen + HeadReserveSize)
	buf.data = &BufferData{
		buffer:  make([]byte, newLen),
		readers: 1,
	}
	if direction == PUSH {
		buf.dataOffset = HeadReserveSize
		copy(buf.data.buffer[HeadReserveSize:], oldData)
		buf.endOffset = HeadReserveSize + oldDataLen
	} else {
		buf.dataOffset = HeadReserveSize + size
		copy(buf.data.buffer[buf.dataOffset:], oldData)
		buf.endOffset = HeadReserveSize + size + oldDataLen
	}
	buf.isFreezed = false
}

func (buf *Buffer) checkSpace(size uint32, direction WriteDirection) error {
	if size >= MaxBufferLen-buf.Len() {
		return errors.New("buffer is full")
	}

	if buf.isFreezed || buf.data.readers > 1 {
		buf.extendBuffer(size, direction)
	} else {
		if direction == PUT {
			if size > buf.dataOffset {
				buf.extendBuffer(size, direction)
			}
		} else if direction == PUSH {
			if buf.endOffset+size > uint32(len(buf.data.buffer)) {
				buf.extendBuffer(size, direction)
			}
		}
	}

	return nil
}

func (buf *Buffer) put(data []byte) *Buffer {
	dataLen := uint32(len(data))
	err := buf.checkSpace(dataLen, PUT)
	if err == nil {
		buf.dataOffset -= dataLen
		copy(buf.data.buffer[buf.dataOffset:], data)
	}
	return buf
}

func (buf *Buffer) Put(data []byte) *Buffer {
	buf.put(data)
	buf.PutU32(uint32(len(data)))
	return buf
}

func (buf *Buffer) PutBool(value bool) *Buffer {
	var v uint8 = 0
	if value {
		v = 1
	}
	buf.PutU8(v)
	return buf
}

func (buf *Buffer) PutU8(value uint8) *Buffer {
	data := []byte{value}
	buf.put(data)
	return buf
}

func (buf *Buffer) PutU16(value uint16) *Buffer {
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, value)
	buf.put(data)
	return buf
}

func (buf *Buffer) PutU32(value uint32) *Buffer {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, value)
	buf.put(data)
	return buf
}

func (buf *Buffer) PutU64(value uint64) *Buffer {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, value)
	buf.put(data)
	return buf
}

func (buf *Buffer) PutString(str string) *Buffer {
	data := util.Str2bytes(str)
	buf.Put(data)
	return buf
}

func (buf *Buffer) push(data []byte) *Buffer {
	dataLen := uint32(len(data))
	err := buf.checkSpace(dataLen, PUSH)
	if err == nil {
		copy(buf.data.buffer[buf.endOffset:], data)
		buf.endOffset += dataLen
	}
	return buf
}

func (buf *Buffer) Push(data []byte) *Buffer {
	buf.PushU32(uint32(len(data)))
	buf.push(data)
	return buf
}

func (buf *Buffer) PushBool(value bool) *Buffer {
	var v uint8 = 0
	if value {
		v = 1
	}
	buf.PushU8(v)
	return buf
}

func (buf *Buffer) PushU8(value uint8) *Buffer {
	data := []byte{value}
	buf.push(data)
	return buf
}

func (buf *Buffer) PushU16(value uint16) *Buffer {
	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, value)
	buf.push(data)
	return buf
}

func (buf *Buffer) PushU32(value uint32) *Buffer {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, value)
	buf.push(data)
	return buf
}

func (buf *Buffer) PushU64(value uint64) *Buffer {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, value)
	buf.push(data)
	return buf
}

func (buf *Buffer) PushString(str string) *Buffer {
	data := util.Str2bytes(str)
	buf.Push(data)
	return buf
}

func (buf *Buffer) _peek(size uint32, offset uint32) ([]byte, error) {
	bufLen := buf.endOffset - buf.dataOffset - offset
	if size > bufLen {
		return nil, errors.New("no enough data")
	}

	return buf.data.buffer[buf.dataOffset+offset : buf.dataOffset+offset+size], nil
}

func (buf *Buffer) peek(size uint32, offset uint32) ([]byte, error) {
	if buf.peeker == nil {
		return buf._peek(size, offset)
	} else {
		return buf.peeker(size, offset)
	}
}

func (buf *Buffer) PeekWithError(offset uint32) ([]byte, error) {
	dataLen, err := buf.peekU32(offset)
	if err != nil {
		return nil, err
	}
	if dataLen > buf.Len()-offset-4 {
		return nil, errors.New("no enough data")
	}

	return buf.peek(dataLen, offset+4)
}

func (buf *Buffer) PeekWithSize(size uint32, offset uint32) ([]byte, error) {
	dataLen := buf.PeekU32(offset)
	if dataLen != size {
		return nil, errors.New("size not match")
	}
	return buf.peek(dataLen, offset+4)
}

func (buf *Buffer) Peek(offset uint32) []byte {
	dataLen, err := buf.peekU32(offset)
	if err != nil {
		return nil
	}

	data, _ := buf.peek(dataLen, offset+4)
	return data
}

func (buf *Buffer) PeekBool(offset uint32) bool {
	data, err := buf.peek(1, offset)
	if err != nil {
		return false
	}
	return data[0] != 0
}

func (buf *Buffer) PeekU8(offset uint32) uint8 {
	data, err := buf.peek(1, offset)
	if err != nil {
		return InvalidU8
	}
	return data[0]
}

func (buf *Buffer) PeekU16(offset uint32) uint16 {
	data, err := buf.peek(2, offset)
	if err != nil {
		return InvalidU16
	}
	return binary.BigEndian.Uint16(data)
}

func (buf *Buffer) peekU32NoError(offset uint32) uint32 {
	return binary.BigEndian.Uint32(buf.data.buffer[buf.dataOffset+offset : buf.dataOffset+offset+4])
}

func (buf *Buffer) peekU32(offset uint32) (uint32, error) {
	data, err := buf.peek(4, offset)
	if err != nil {
		return InvalidU32, err
	}
	return binary.BigEndian.Uint32(data), nil
}

func (buf *Buffer) PeekU32(offset uint32) uint32 {
	value, _ := buf.peekU32(offset)
	return value
}

func (buf *Buffer) PeekU64(offset uint32) uint64 {
	data, err := buf.peek(8, offset)
	if err != nil {
		return InvalidU64
	}

	return binary.BigEndian.Uint64(data)
}

func (buf *Buffer) PeekString(offset uint32) string {
	dataLen, err := buf.peekU32(offset)
	if err != nil {
		return ""
	}

	data, err := buf.peek(dataLen, 4+offset)
	if err != nil {
		return ""
	}

	return util.Bytes2str(data)
}

func (buf *Buffer) consume(size uint32) {
	buf.dataOffset += size
}

func (buf *Buffer) pull(size uint32) []byte {
	data := make([]byte, size)
	copy(data, buf.data.buffer[buf.dataOffset:buf.dataOffset+size])
	buf.consume(size)
	return data
}

func (buf *Buffer) PullWithSize(size uint32) ([]byte, error) {
	_, err := buf.PeekWithSize(size, 0)
	if err != nil {
		return nil, err
	}
	buf.pullU32()
	return buf.pull(size), nil
}

func (buf *Buffer) PullWithError() ([]byte, error) {
	_, err := buf.PeekWithError(0)
	if err != nil {
		return nil, err
	}
	dataLen := buf.pullU32()
	return buf.pull(dataLen), nil
}

func (buf *Buffer) Pull() []byte {
	data := buf.Peek(0)
	if data == nil {
		return nil
	}

	dataLen := buf.pullU32()
	return buf.pull(dataLen)
}

func (buf *Buffer) PullBool() bool {
	data, err := buf.peek(1, 0)
	if err != nil {
		return false
	}
	buf.consume(1)
	return data[0] != 0
}

func (buf *Buffer) PullU8() uint8 {
	data, err := buf.peek(1, 0)
	if err != nil {
		return InvalidU8
	}
	buf.consume(1)
	return data[0]
}

func (buf *Buffer) PullU16() uint16 {
	data, err := buf.peek(2, 0)
	if err != nil {
		return InvalidU16
	}
	buf.consume(2)
	return binary.BigEndian.Uint16(data)
}

func (buf *Buffer) pullU32() uint32 {
	value := buf.peekU32NoError(0)
	buf.consume(4)
	return value
}

func (buf *Buffer) PullU32() uint32 {
	data, err := buf.peek(4, 0)
	if err != nil {
		return InvalidU32
	}
	buf.consume(4)
	return binary.BigEndian.Uint32(data)
}

func (buf *Buffer) PullU64() uint64 {
	data, err := buf.peek(8, 0)
	if err != nil {
		return InvalidU64
	}
	buf.consume(8)
	return binary.BigEndian.Uint64(data)
}

func (buf *Buffer) PullString() string {
	data := buf.Pull()
	if data == nil {
		return ""
	}

	return util.Bytes2str(data)
}

func (buf *Buffer) PullStringWithError() (string, error) {
	data, err := buf.PullWithError()
	if err != nil {
		return "", err
	}

	return util.Bytes2str(data), nil
}

func (buf *Buffer) setPeeker(f func(size uint32, offset uint32) ([]byte, error)) {
	buf.peeker = f
}

type BufferWithError struct {
	*Buffer
	Err error
}

func NewBufferWithError(buf *Buffer) *BufferWithError {
	buffer := &BufferWithError{
		Buffer: buf,
		Err:    nil,
	}

	buf.setPeeker(func(size uint32, offset uint32) ([]byte, error) {
		if buffer.Err != nil {
			return nil, buffer.Err
		}
		data, err := buf._peek(size, offset)
		if err != nil {
			buffer.Err = err
		}
		return data, err
	})

	return buffer
}

func BufferWithErrorFromBytes(data []byte) *BufferWithError {
	return NewBufferWithError(FromBytes(data))
}

func (buf *BufferWithError) IsError() bool {
	return buf.Err != nil
}
