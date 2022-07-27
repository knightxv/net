package util_test

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/guabee/bnrtc/util"
)

func assertPanic(t *testing.T, f func()) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	f()
}

func TestPubSub(t *testing.T) {
	input := "hello"
	wg := &sync.WaitGroup{}
	num := 10

	wg.Add(num)
	for i := 0; i < num; i++ {
		topic := fmt.Sprintf("topic-%d", i)
		name := fmt.Sprintf("subscriber-%d", i)
		isPublished := false
		var unsubscribe util.PubSubCancelFunc

		go func(topic, name string) {
			time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
			var cnt int32

			unsubscribe, _ = util.SubscribeFunc(name+"2", topic, func(msg util.PubSubMsgType) {
				if msg.(string) != input {
					t.Errorf("Expected hello, got %s", msg.(string))
				}
				atomic.AddInt32(&cnt, 1)
			})

			time.Sleep(time.Second)
			if cnt != 1 {
				t.Errorf("Expected 1 messages, got %d", cnt)
			}
			wg.Done()
		}(topic, name)

		go func(topic string) {
			_, _ = util.SubscribeFuncWait(topic+"wait", topic, func(msg util.PubSubMsgType) {
				if msg.(string) != input {
					t.Errorf("Expected hello, got %s", msg.(string))
				}
			})
			if !isPublished {
				t.Errorf("Expected published")
			}
		}(topic)

		go func(topic string) {
			time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
			util.Publish(topic, input)
			isPublished = true
		}(topic)

		go func() {
			time.Sleep(2 * time.Second)
			unsubscribe()
		}()
	}

	wg.Wait()
}

func TestRPCCenter(t *testing.T) {
	name := "test"
	input := "hello"
	wg := &sync.WaitGroup{}
	wg.Add(3)
	go func() {
		time.Sleep(100 * time.Millisecond)
		util.RPCRegist(name, func(params ...util.RPCParamType) util.RPCResultType {
			return input
		})
		wg.Done()
	}()

	go func() {
		_, err := util.RPCCall(name)
		if err == nil {
			t.Errorf("Expected error")
			return
		}

		util.RPCOnRegist(name, func(params ...util.RPCParamType) util.RPCResultType {
			res, err := util.RPCCall(name)
			if err != nil {
				t.Errorf("Expected no error, got %s", err)
			}
			if res.(string) != input {
				t.Errorf("Expected %s, got %s", input, res)
			}
			wg.Done()
			return nil
		})

		time.Sleep(200 * time.Millisecond)
		res, err := util.RPCCall(name)
		if err != nil {
			t.Errorf("Expected no error, got %s", err)
		}
		if res.(string) != input {
			t.Errorf("Expected %s, got %s", input, res)
		}
		wg.Done()
	}()
	wg.Wait()
}

func TestBitSet(t *testing.T) {
	b := util.NewBitSet(100)

	b.Set(1)
	b.Set(99)

	if b.LowestSetBit() != 1 {
		t.Errorf("LowestSetBit() returned %d, expected 1", b.LowestSetBit())
	}
	if b.HighestSetBit() != 99 {
		t.Errorf("HighestSetBit() returned %d, expected 99", b.HighestSetBit())
	}

	if !b.IsSet(1) {
		t.Error("Expected 1 to be set")
	}

	if !b.IsSet(99) {
		t.Error("Expected 99 to be set")
	}

	if b.IsSet(20) {
		t.Error("Expected 20 to not be set")
	}

	b.Clear(1)
	if b.IsSet(1) {
		t.Error("Expected 1 to be clear")
	}

	assertPanic(t, func() {
		b.Set(100)
	})
}

func TestSafeLinkMap(t *testing.T) {
	m := util.NewSafeLinkMap()
	key1 := "key1"
	key2 := "key2"
	key3 := "key3"
	key4 := "key4"
	value1 := "value1"
	value2 := "value2"
	value3 := "value3"
	value4 := "value4"

	m.Set(key1, value1)
	m.Set(key2, value2)
	m.Set(key3, value3)
	m.Set(key4, value4)

	key0 := "key0"
	_, ok := m.Get(key0)
	if ok {
		t.Error("Expected key0 to not be found")
	}

	v1, ok := m.Get(key1)
	if !ok || v1.(string) != value1 {
		t.Errorf("Expected %s, got %s", value1, v1.(string))
	}

	if m.Has(key0) || !m.Has(key1) {
		t.Error("Expected key0 and key1 to be found")
	}

	if m.Size() != 4 {
		t.Errorf("Expected size to be 3, got %d", m.Size())
	}

	m.Delete(key1)
	if m.Size() != 3 {
		t.Errorf("Expected size to be 2, got %d", m.Size())
	}
	m.Set(key1, value1)

	k, v, ok := m.Front()
	if !ok || k.(string) != key2 || v.(string) != value2 {
		t.Errorf("Expected key2, got %s", v.(string))
	}

	k, v, ok = m.Shift()
	if !ok || k.(string) != key2 || v.(string) != value2 {
		t.Errorf("Expected key2, got %s", v.(string))
	}

	k, v, ok = m.Back()
	if !ok || k.(string) != key1 || v.(string) != value1 {
		t.Errorf("Expected key1, got %s", v.(string))
	}

	k, v, ok = m.PopBack()
	if !ok || k.(string) != key1 || v.(string) != value1 {
		t.Errorf("Expected key1, got %s", v.(string))
	}

	m.MoveToBack(key3)

	for i, item := range m.Items() {
		if i == 0 {
			if item.(string) != value4 {
				t.Errorf("Expected key4, got %s", item.(string))
			}
		} else if i == 1 {
			if item.(string) != value3 {
				t.Errorf("Expected key3, got %s", item.(string))
			}
		} else {
			t.Errorf("Expected null, got %s", item.(string))
		}
	}

	m.Update(key4, value4)
	k, v, ok = m.Front()
	if !ok || k.(string) != key4 || v.(string) != value4 {
		t.Errorf("Expected key4, got %s", v.(string))
	}

	m.Set(key4, value4)
	k, v, ok = m.Back()
	if !ok || k.(string) != key4 || v.(string) != value4 {
		t.Errorf("Expected key4, got %s", v.(string))
	}

	m.Clear()
	if m.Size() != 0 {
		t.Errorf("Expected size to be 0, got %d", m.Size())
	}
}

func TestSafeMap(t *testing.T) {
	m := util.NewSafeMap()
	key1 := "key1"
	key2 := "key2"
	key3 := "key3"
	value1 := "value1"
	value2 := "value2"
	value3 := "value3"

	m.Set(key1, value1)
	m.Set(key2, value2)
	m.Set(key3, value3)

	key0 := "key0"
	_, ok := m.Get(key0)
	if ok {
		t.Error("Expected key0 to not be found")
	}

	v1, ok := m.Get(key1)
	if !ok || v1.(string) != value1 {
		t.Errorf("Expected %s, got %s", value1, v1.(string))
	}

	if m.Has(key0) || !m.Has(key1) {
		t.Error("Expected key0 and key1 to be found")
	}

	if m.Size() != 3 {
		t.Errorf("Expected size to be 3, got %d", m.Size())
	}

	m.Delete(key1)
	if m.Size() != 2 {
		t.Errorf("Expected size to be 2, got %d", m.Size())
	}

	allKeys := []string{key2, key3}
	for key := range m.Items() {
		for i, k := range allKeys {
			if k == key {
				// Found!
				allKeys = append(allKeys[:i], allKeys[i+1:]...)
			}
		}
	}
	if len(allKeys) != 0 {
		t.Errorf("Expected all keys to be found, got %v", allKeys)
	}

	m.Clear()
	if m.Size() != 0 {
		t.Errorf("Expected size to be 0, got %d", m.Size())
	}
}

func TestSafeSlice(t *testing.T) {
	s := util.NewSafeSlice(0)
	value1 := "value1"
	value2 := "value2"
	value3 := "value3"
	value4 := "value4"

	s.Append(value1)
	s.Append(value2)
	s.Append(value3)

	if s.Size() != 3 {
		t.Errorf("Expected size to be 3, got %d", s.Size())
	}

	v, ok := s.Get(0)
	if !ok || v.(string) != value1 {
		t.Errorf("Expected %s, got %s", value1, v.(string))
	}

	_, ok = s.Get(3)
	if ok {
		t.Error("Expected index 3 to not be found")
	}

	s.Delete(1)
	if s.Size() != 2 {
		t.Errorf("Expected size to be 2, got %d", s.Size())
	}

	s.Set(0, value4)

	for i, v := range s.Items() {
		if i == 0 {
			if v.(string) != value4 {
				t.Errorf("Expected %s, got %s", value4, v.(string))
			}
		} else if i == 1 {
			if v.(string) != value3 {
				t.Errorf("Expected %s, got %s", value3, v.(string))
			}
		} else {
			t.Errorf("Expected null, got %s", v.(string))
		}
	}

	s.Sort(func(a, b interface{}) bool {
		return a.(string) < b.(string)
	})

	for i, v := range s.Items() {
		if i == 0 {
			if v.(string) != value3 {
				t.Errorf("Expected %s, got %s", value3, v.(string))
			}
		} else if i == 1 {
			if v.(string) != value4 {
				t.Errorf("Expected %s, got %s", value4, v.(string))
			}
		} else {
			t.Errorf("Expected null, got %s", v.(string))
		}
	}

	s.Clear()
	if s.Size() != 0 {
		t.Errorf("Expected size to be 0, got %d", s.Size())
	}
}

func TestRing(t *testing.T) {
	ringSize := 3
	messageSize := 10
	ring := util.NewRing(ringSize, messageSize)

	l := make([]*util.RingMessage, ringSize)
	for i := 0; i < ringSize; i++ {
		if ring.Cap() != ringSize {
			t.Errorf("Expected ring capacity to be %d, got %d", ringSize, ring.Cap())
		}
		if ring.Len() != i {
			t.Errorf("Expected ring length to be %d, got %d", i, ring.Len())
		}
		msg := ring.Get()
		l[i] = msg
		data := msg.Bytes()
		if cap(data) != messageSize {
			t.Errorf("Expected message size to be %d, got %d", messageSize, cap(data))
		}

		if len(data) != messageSize {
			t.Errorf("Expected message size to be %d, got %d", messageSize, len(data))
		}

		if ring.Cap() != ringSize {
			t.Errorf("Expected ring capacity to be %d, got %d", ringSize, ring.Cap())
		}
		if ring.Len() != i+1 {
			t.Errorf("Expected ring capacity to be %d, got %d", i+1, ring.Len())
		}
	}
	gotMsg := false
	time.AfterFunc(2*time.Second, func() {
		msg := l[0]
		ring.Put(msg)
	})
	time.AfterFunc(1*time.Second, func() {
		if gotMsg {
			t.Error("Expected to not get message")
		}
	})
	msg := ring.Get()
	data := msg.Bytes()
	gotMsg = true
	if cap(data) != messageSize {
		t.Errorf("Expected message size to be %d, got %d", messageSize, cap(data))
	}

	if len(data) != messageSize {
		t.Errorf("Expected message size to be %d, got %d", messageSize, cap(data))
	}

	if msg != l[0] {
		t.Errorf("Expected message to be %v, got %v", l[0], msg)
	}

	if ring.Cap() != ringSize {
		t.Errorf("Expected ring capacity to be %d, got %d", ringSize, ring.Cap())
	}
	if ring.Len() != ringSize {
		t.Errorf("Expected ring capacity to be %d, got %d", ringSize, ring.Len())
	}

	ring.Put(msg)
	ring.Put(l[1])
	ring.Put(l[2])

	for i := 0; i < ringSize; i++ {
		if ring.Cap() != ringSize {
			t.Errorf("Expected ring capacity to be %d, got %d", ringSize, ring.Cap())
		}
		if ring.Len() != i {
			t.Errorf("Expected ring length to be %d, got %d", i, ring.Len())
		}
		msg := ring.Get()
		data := msg.Bytes()
		if cap(data) != messageSize {
			t.Errorf("Expected message size to be %d, got %d", messageSize, cap(data))
		}

		if len(data) != messageSize {
			t.Errorf("Expected message size to be %d, got %d", messageSize, len(data))
		}

		if ring.Cap() != ringSize {
			t.Errorf("Expected ring capacity to be %d, got %d", ringSize, ring.Cap())
		}
		if ring.Len() != i+1 {
			t.Errorf("Expected ring capacity to be %d, got %d", i+1, ring.Len())
		}
	}
}

func TestSingleTonTask(t *testing.T) {
	num := 10
	count := 0

	cmd := func() {
		count++
		time.Sleep(1 * time.Second)
	}
	task := util.NewSingleTonTask()

	for i := 0; i < num; i++ {
		go func() {
			task.RunWith(cmd)
		}()
		time.Sleep(510 * time.Millisecond)
	}

	if count != num/2 {
		t.Errorf("Expected count to be %d, got %d", num/2, count)
	}

	num2 := 10
	count2 := 0

	cmd2 := func() {
		count2++
		time.Sleep(1 * time.Second)
	}
	task2 := util.NewSingleTonTask(util.AllowPending(), util.BindingFunc(cmd2))

	for i := 0; i < num2; i++ {
		go func() {
			task2.Run()
		}()
		time.Sleep(50 * time.Millisecond)
	}

	time.Sleep(3 * time.Second)
	task2.Run()
	time.Sleep(2 * time.Second)

	if count2 != 3 {
		t.Errorf("Expected count to be %d, got %d", 3, count2)
	}
}

func TestSelectN(t *testing.T) {
	waiter := func(ch chan interface{}, value int, timeout time.Duration) {
		<-time.After(timeout)
		ch <- value
	}
	getReqChans := func(num int) []interface{} {
		chs := make([]interface{}, num)
		for i := 0; i < num; i++ {
			ch := make(chan interface{})
			go waiter(ch, i, time.Second)
			chs[i] = ch
		}
		return chs
	}
	num := 127
	reqChans := getReqChans(num)
	ctx, cancel := context.WithCancel(context.Background())
	resultChan := make(chan interface{})
	go util.SelectN(ctx, reqChans, resultChan)
	for _result := range resultChan {
		if _result != nil {
			fmt.Println(_result.(int))
		}
	}
	cancel()

	reqChans2 := getReqChans(num)
	ctx2, cancel2 := context.WithCancel(context.Background())
	resultChan2 := make(chan interface{})
	go util.SelectN(ctx2, reqChans2, resultChan2)
	for _result := range resultChan2 {
		if _result != nil {
			fmt.Println(_result.(int))
		}
		cancel2()
	}
	cancel2()
	<-time.After(4 * time.Second)
}

func TestChannelSelecter(t *testing.T) {
	waiter := func(ch chan interface{}, value int, timeout time.Duration) {
		<-time.After(timeout)
		ch <- value
	}
	getReqChans := func(num int) []interface{} {
		chs := make([]interface{}, num)
		for i := 0; i < num; i++ {
			ch := make(chan interface{})
			go waiter(ch, i, time.Second)
			chs[i] = ch
		}
		return chs
	}
	num := 127
	reqChans := getReqChans(num)
	ctx, cancel := context.WithCancel(context.Background())
	resultChan := make(chan interface{})
	util.NewChannelSelector(reqChans).Select(ctx, resultChan)
	for _result := range resultChan {
		if _result != nil {
			fmt.Println(_result.(int))
		}
	}
	cancel()

	reqChans2 := getReqChans(num)
	ctx2, cancel2 := context.WithCancel(context.Background())
	resultChan2 := make(chan interface{})
	util.NewChannelSelector(reqChans2).Select(ctx2, resultChan2)
	for _result := range resultChan2 {
		if _result != nil {
			fmt.Println(_result.(int))
		}
		cancel2()
	}
	cancel2()
	<-time.After(2 * time.Second)
}

func TestOthers(t *testing.T) {
	address := "2et8jGb14fAfzQBNgLRHcwVxLd4Pif1mn"

	// Str2bytes & Bytes2str
	byteData := util.Str2bytes(address)
	if util.Bytes2str(byteData) != address {
		t.Error("Expected address to be equal to address")
	}

	// ToHexAddress & FromHexAddress
	hex, err := util.ToHexAddress(address)
	if err != nil {
		t.Error(err)
	}

	value, err := util.FromHexAddress(hex)
	if err != nil {
		t.Error(err)
	}

	if value != address {
		t.Error("Expected address to be equal to address")
	}

	// Truncate
	if !bytes.Equal(util.Truncate([]byte("1234567890123456789012345678901234567890"), 5), []byte("12345")) {
		t.Error("Expected bytes to be equal to bytes")
	}

	// HttpGet
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Hello, World!"))
	})

	go func() {
		_ = http.ListenAndServe(":9393", nil)
	}()

	data, err := util.HttpGet("localhost:9393", "/", url.Values{})
	if err != nil {
		t.Error(err)
	}

	if string(data) != "Hello, World!" {
		t.Error("Expected data to be equal to data")
	}
}
