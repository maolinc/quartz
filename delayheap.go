package quartz

import (
	"container/heap"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

const longTick = time.Hour * 24 * 365

type Delay interface {
	GetDelay() time.Duration
	IsCycle() bool
	Exec()
}

type element struct {
	e         Delay
	absExpire time.Duration
	index     int
}

type h []*element

type Option func(d *DelayQueue)

type DelayQueue struct {
	// minimum error range
	offset time.Duration
	ticker *time.Timer
	heap   h
	ctx    context.Context
	cancel context.CancelFunc
	// expireChannel
	out        chan Delay
	in         chan struct{}
	lock       sync.RWMutex
	state      uint32
	nextExpire time.Duration
}

func NewDelayQueue(ops ...Option) *DelayQueue {
	ctx, cancelFunc := context.WithCancel(context.Background())

	inChannel := make(chan struct{})
	outChannel := make(chan Delay)
	d := &DelayQueue{
		offset:     time.Millisecond * 10,
		ticker:     time.NewTimer(longTick),
		heap:       make(h, 0, 16),
		ctx:        ctx,
		cancel:     cancelFunc,
		out:        outChannel,
		in:         inChannel,
		nextExpire: longTick,
	}

	for _, op := range ops {
		op(d)
	}

	go d.run()

	return d
}

func (d *DelayQueue) run() {
	var e *element
	var delay time.Duration

	for {
		d.lock.Lock()
		e, delay = d.heap.peekAndShift()

		if e != nil {
			d.lock.Unlock()
			select {
			case d.out <- e.e:
				continue
			case <-d.ctx.Done():
				goto exit
			}
		}

		atomic.StoreUint32(&d.state, 1)
		d.lock.Unlock()

		if delay == 0 {
			//delay = longTick
			select {
			case <-d.in:
				continue
			case <-d.ctx.Done():
				goto exit
			}
		}

		select {
		case <-time.After(delay):
			if atomic.SwapUint32(&d.state, 0) == 0 {
				<-d.in
			}
			continue
		case <-d.in:
			continue
		case <-d.ctx.Done():
			goto exit
		}
	}

exit:
	atomic.SwapUint32(&d.state, 1)
	close(d.in)
	close(d.out)
}

func (d *DelayQueue) Poll() <-chan Delay {
	return d.out
}

func (d *DelayQueue) Push(e Delay) {
	de := e.GetDelay()
	if e.GetDelay() <= 0 {
		de = d.offset
	}
	et := &element{
		e:         e,
		absExpire: nowNano() + de,
	}

	d.lock.Lock()
	defer d.lock.Unlock()

	if d.len() > 0 {
		top := d.heap[0]
		if top.absExpire > et.absExpire {
			if atomic.CompareAndSwapUint32(&d.state, 1, 0) {
				d.in <- struct{}{}
			}
		}
	} else {
		if atomic.CompareAndSwapUint32(&d.state, 1, 0) {
			d.in <- struct{}{}
		}
	}

	heap.Push(&d.heap, et)
}

func (d *DelayQueue) Pop() (any, bool) {
	d.lock.Lock()
	defer d.lock.Unlock()
	if d.heap.Len() >= 0 {
		et := heap.Pop(&d.heap)
		return et.(*element).e, true
	}
	return nil, false
}

func (d *DelayQueue) Len() int {
	d.lock.RLocker()
	defer d.lock.RUnlock()
	return len(d.heap)
}

func (d *DelayQueue) len() int {
	return len(d.heap)
}

func (d *DelayQueue) Top() (any, bool) {
	d.lock.RLocker()
	defer d.lock.RUnlock()
	if len(d.heap) > 0 {
		return d.heap[0].e, true
	}
	return nil, false
}

func (d *DelayQueue) top() (*element, bool) {
	if len(d.heap) > 0 {
		return d.heap[0], true
	}
	return nil, false
}

func (d *DelayQueue) Close() {
	d.cancel()
}

func (m *h) peekAndShift() (*element, time.Duration) {
	if m.Len() == 0 {
		return nil, 0
	}

	now := nowNano()
	e := (*m)[0]
	if e.absExpire > now {
		return nil, e.absExpire - now
	}
	if e.e.IsCycle() {
		e.absExpire += e.e.GetDelay()
		heap.Fix(m, 0)
	} else {
		heap.Remove(m, 0)
	}

	return e, 0
}

func (m h) Len() int {
	return len(m)
}

func (m h) Less(i, j int) bool {
	return m[i].absExpire < m[j].absExpire
}

func (m h) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
	m[i].index = i
	m[j].index = j
}

func (m *h) Push(x any) {
	//*m = append(*m, x.(*element))
	//lastIndex := len(*m) - 1
	//(*m)[lastIndex].index = lastIndex
	n := len(*m)
	c := cap(*m)
	if n+1 > c {
		npq := make(h, n, c*2)
		copy(npq, *m)
		*m = npq
	}
	*m = (*m)[0 : n+1]
	item := x.(*element)
	item.index = n
	(*m)[n] = item
}

func (m *h) Pop() any {
	//old := *m
	//n := len(old)
	//x := old[n-1]
	//x.index = -1
	//*m = old[0 : n-1]
	//return x
	n := len(*m)
	c := cap(*m)
	if n < (c/2) && c > 25 {
		npq := make(h, n, c/2)
		copy(npq, *m)
		*m = npq
	}
	item := (*m)[n-1]
	item.index = -1
	*m = (*m)[0 : n-1]
	return item
}

func WithOffset(offset time.Duration) Option {
	return func(d *DelayQueue) {
		d.offset = offset
	}
}

func WithTaskChannel(num int) Option {
	return func(d *DelayQueue) {
		d.out = make(chan Delay, num)
	}
}

func nowNano() time.Duration {
	return time.Duration(time.Now().UTC().UnixNano())
}
