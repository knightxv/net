package util

import (
	"fmt"
	"sync"
)

// CircularBuffer implements a circular buffer. It is a fixed size,
// and new writes overwrite older data, such that for a buffer
// of size N, for any amount of writes, only the last N bytes
// are retained.
type CircularBuffer struct {
	data        []byte
	size        int64
	writeCursor int64
	written     int64
	mt          sync.Mutex
}

// NewBuffer creates a new buffer of a given size. The size
// must be greater than 0.
func NewCircularBuffer(size int64) (*CircularBuffer, error) {
	if size <= 0 {
		return nil, fmt.Errorf("Size must be positive")
	}

	b := &CircularBuffer{
		size: size,
		data: make([]byte, size),
	}
	return b, nil
}

// Write writes up to len(buf) bytes to the internal ring,
// overriding older data if necessary.
func (b *CircularBuffer) Write(buf []byte) (int, error) {
	b.mt.Lock()
	// Account for total bytes written
	n := len(buf)
	b.written += int64(n)

	// If the buffer is larger than ours, then we only care
	// about the last size bytes anyways
	if int64(n) > b.size {
		buf = buf[int64(n)-b.size:]
	}

	// Copy in place
	remain := b.size - b.writeCursor
	copy(b.data[b.writeCursor:], buf)
	if int64(len(buf)) > remain {
		copy(b.data, buf[remain:])
	}

	// Update location of the cursor
	b.writeCursor = ((b.writeCursor + int64(len(buf))) % b.size)
	b.mt.Unlock()
	return n, nil
}

// Size returns the size of the buffer
func (b *CircularBuffer) Size() int64 {
	return b.size
}

// TotalWritten provides the total number of bytes written
func (b *CircularBuffer) TotalWritten() int64 {
	return b.written
}

// Bytes provides a slice of the bytes written. This
// slice should not be written to.
func (b *CircularBuffer) Bytes() []byte {
	b.mt.Lock()
	defer b.mt.Unlock()
	switch {
	case b.written >= b.size && b.writeCursor == 0:
		return b.data
	case b.written > b.size:
		out := make([]byte, b.size)
		copy(out, b.data[b.writeCursor:])
		copy(out[b.size-b.writeCursor:], b.data[:b.writeCursor])
		return out
	default:
		return b.data[:b.writeCursor]
	}
}

// Reset resets the buffer so it has no content.
func (b *CircularBuffer) Reset() {
	b.mt.Lock()
	defer b.mt.Unlock()
	b.written = 0
	b.writeCursor = 0
	b.written = 0
}

// String returns the contents of the buffer as a string
func (b *CircularBuffer) String() string {
	return string(b.Bytes())
}
