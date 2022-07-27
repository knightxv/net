package util

import (
	"context"
	"reflect"
	"sync"
)

func Select4(ctx context.Context, chans []interface{}, resultChan chan interface{}) {
	selectCaseSize := 4
	num := len(chans)
	if num == 0 {
		close(resultChan)
		return
	}

	if num > selectCaseSize {
		panic("Select4: too many channels")
	}

	var selectCase = make([]reflect.SelectCase, selectCaseSize+1)
	for i, ch := range chans {
		selectCase[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch),
		}
	}

	// pad with nil channel
	for i := num; i < selectCaseSize; i++ {
		selectCase[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(nil),
		}
	}
	// last is ctx channel
	selectCase[selectCaseSize] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(ctx.Done()),
	}

	finished := 0
	checkFinish := func(idx int) bool {
		selectCase[idx].Chan = reflect.ValueOf(nil) // block it
		finished++
		return finished == num
	}

	for {
		chosen, recv, _ := reflect.Select(selectCase)
		switch chosen {
		case 0:
			fallthrough
		case 1:
			fallthrough
		case 2:
			fallthrough
		case 3:
			if recv.IsNil() {
				resultChan <- nil
			} else {
				resultChan <- recv.Interface()
			}
			done := checkFinish(chosen)
			if done {
				close(resultChan)
				return
			}
		case 4: //ctx.Done()
			close(resultChan)
			return
		}
	}
}

func SelectN(ctx context.Context, chans []interface{}, resultChan chan interface{}) {
	num := len(chans)
	if num == 0 {
		close(resultChan)
		return
	}

	var selectCase = make([]reflect.SelectCase, num+1)
	for i, ch := range chans {
		selectCase[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch),
		}
	}

	// last is ctx channel
	selectCase[num] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(ctx.Done()),
	}

	finished := 0
	checkFinish := func(idx int) bool {
		selectCase[idx].Chan = reflect.ValueOf(nil) // block it
		finished++
		return finished == num
	}

	for {
		chosen, recv, _ := reflect.Select(selectCase)
		if chosen != num { //ctx.Done()
			if recv.IsNil() {
				resultChan <- nil
			} else {
				resultChan <- recv.Interface()
			}
			done := checkFinish(chosen)
			if done {
				close(resultChan)
				return
			}
		} else {
			close(resultChan)
			return
		}
	}
}

type ChannelSelector struct {
	chans []interface{}
	num   int32
}

func NewChannelSelector(chans []interface{}) *ChannelSelector {
	num := len(chans)
	if num == 0 {
		return nil
	}

	// copy chans
	chans2 := make([]interface{}, num)
	copy(chans2, chans)
	return &ChannelSelector{
		chans: chans2,
		num:   int32(num),
	}
}

func (c *ChannelSelector) Select(ctx context.Context, resultChan chan interface{}) {
	if c == nil {
		close(resultChan)
		return
	}

	i := 0
	var wg sync.WaitGroup
	for i < len(c.chans) {
		l := len(c.chans) - i
		wg.Add(1)
		switch {
		case l > 7:
			go c.select8(ctx, &wg, c.chans[i:i+8], resultChan)
			i += 8
		case l > 3:
			go c.select4(ctx, &wg, c.chans[i:i+4], resultChan)
			i += 4
		case l > 1:
			go c.select2(ctx, &wg, c.chans[i:i+2], resultChan)
			i += 2
		case l > 0:
			go c.select1(ctx, &wg, c.chans[i:i+1], resultChan)
			i += 1
		}
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()
}

func (c *ChannelSelector) checkFinish(chans []interface{}, idx int, pending *int, resultChan chan interface{}) bool {
	chans[idx] = chan interface{}(nil) // block it
	*pending -= 1
	return *pending == 0
}

func (c *ChannelSelector) select1(ctx context.Context, wg *sync.WaitGroup, chans []interface{}, resultChan chan interface{}) {
	left := 1
	var v interface{}
	defer wg.Done()
	for {
		select {
		case v = <-chans[0].(chan interface{}):
			resultChan <- v
			if c.checkFinish(chans, 0, &left, resultChan) {
				return
			}
		case <-ctx.Done():
			return
		}
	}

}

func (c *ChannelSelector) select2(ctx context.Context, wg *sync.WaitGroup, chans []interface{}, resultChan chan interface{}) {
	left := 2
	var v interface{}
	defer wg.Done()
	for {
		select {
		case v = <-chans[0].(chan interface{}):
			resultChan <- v
			if c.checkFinish(chans, 0, &left, resultChan) {
				return
			}
		case v = <-chans[1].(chan interface{}):
			resultChan <- v
			if c.checkFinish(chans, 1, &left, resultChan) {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *ChannelSelector) select4(ctx context.Context, wg *sync.WaitGroup, chans []interface{}, resultChan chan interface{}) {
	left := 4
	var v interface{}
	defer wg.Done()
	for {
		select {
		case v = <-chans[0].(chan interface{}):
			resultChan <- v
			if c.checkFinish(chans, 0, &left, resultChan) {
				return
			}
		case v = <-chans[1].(chan interface{}):
			resultChan <- v
			if c.checkFinish(chans, 1, &left, resultChan) {
				return
			}
		case v = <-chans[2].(chan interface{}):
			resultChan <- v
			if c.checkFinish(chans, 2, &left, resultChan) {
				return
			}
		case v = <-chans[3].(chan interface{}):
			resultChan <- v
			if c.checkFinish(chans, 3, &left, resultChan) {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *ChannelSelector) select8(ctx context.Context, wg *sync.WaitGroup, chans []interface{}, resultChan chan interface{}) {
	left := 8
	var v interface{}
	defer wg.Done()
	for {
		select {
		case v = <-chans[0].(chan interface{}):
			resultChan <- v
			if c.checkFinish(chans, 0, &left, resultChan) {
				return
			}
		case v = <-chans[1].(chan interface{}):
			resultChan <- v
			if c.checkFinish(chans, 1, &left, resultChan) {
				return
			}
		case v = <-chans[2].(chan interface{}):
			resultChan <- v
			if c.checkFinish(chans, 2, &left, resultChan) {
				return
			}
		case v = <-chans[3].(chan interface{}):
			resultChan <- v
			if c.checkFinish(chans, 3, &left, resultChan) {
				return
			}
		case v = <-chans[4].(chan interface{}):
			resultChan <- v
			if c.checkFinish(chans, 4, &left, resultChan) {
				return
			}
		case v = <-chans[5].(chan interface{}):
			resultChan <- v
			if c.checkFinish(chans, 5, &left, resultChan) {
				return
			}
		case v = <-chans[6].(chan interface{}):
			resultChan <- v
			if c.checkFinish(chans, 6, &left, resultChan) {
				return
			}
		case v = <-chans[7].(chan interface{}):
			resultChan <- v
			if c.checkFinish(chans, 7, &left, resultChan) {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
