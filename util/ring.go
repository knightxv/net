package util

type RingMessage struct {
	Data []byte
}

func (m *RingMessage) Bytes() []byte {
	return m.Data
}

type MessageRing struct {
	buffer      chan *RingMessage
	ringSize    int
	messageSize int
}

func NewRing(ringSize int, messageSize int) *MessageRing {
	ring := &MessageRing{
		buffer:      make(chan *RingMessage, ringSize),
		ringSize:    ringSize,
		messageSize: messageSize,
	}
	for i := 0; i < ringSize; i++ {
		ring.buffer <- &RingMessage{
			Data: make([]byte, messageSize),
		}
	}
	return ring
}

func (r *MessageRing) Get() *RingMessage {
	return <-r.buffer
}

func (r *MessageRing) Put(msg *RingMessage) {
	r.buffer <- msg
}

func (r *MessageRing) Cap() int {
	return r.ringSize
}

func (r *MessageRing) Len() int {
	return r.ringSize - len(r.buffer)
}
