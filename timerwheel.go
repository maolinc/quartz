package quartz

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
	"unsafe"
)

var _ Scheduler = (*TimingWheel)(nil)

type TimingWheel struct {
	state int32 //ensure heap concurrency security

	tick        time.Duration
	wheelSize   int64
	interval    time.Duration
	buckets     []*nodeList
	pointerTime time.Duration
	highWheel   unsafe.Pointer

	ctx           context.Context
	cancelFunc    context.CancelFunc
	queue         *DelayQueue // min heap
	currentTimeMs time.Duration

	level int
}

func NewTimingWheel(tick time.Duration, wheelSize int64) Scheduler {
	return NewTimingWheelWithContext(context.Background(), tick, wheelSize)
}

func NewTimingWheelWithContext(ctx context.Context, tick time.Duration, wheelSize int64) Scheduler {
	var start = nowNano()
	delayHeap := NewDelayQueue()
	tw := newTimingWheel(tick, wheelSize, start, delayHeap)
	tw.ctx, tw.cancelFunc = context.WithCancel(ctx)

	return tw
}

func newTimingWheel(tick time.Duration, wheelSize int64, startMs time.Duration, queue *DelayQueue) *TimingWheel {
	t := &TimingWheel{
		tick:          tick,
		wheelSize:     wheelSize,
		interval:      tick * time.Duration(wheelSize),
		buckets:       initNodeList(int(wheelSize)),
		pointerTime:   startMs - (startMs % tick),
		highWheel:     nil,
		currentTimeMs: startMs,
		ctx:           nil,
		queue:         queue,
		level:         1,
	}
	return t
}

func (t *TimingWheel) Run() {
	go func() {
		for {
			select {
			case e := <-t.queue.Poll():
				bucket := e.(*nodeList)
				t.advanceWheel(bucket.absExpire)
				bucket.removeAll(func(task *task) {
					// task moved form high to low
					t.addTimer(task)
				})
			case <-t.ctx.Done():
				t.queue.Close()
				return
			}
		}
	}()
}

func (t *TimingWheel) AfterFunc(expire time.Duration, executeFunc ExecuteFunc) Result {
	milli := nowNano()
	nTask := newTask(t.ctx, expire, milli+expire, executeFunc, withTaskMaxExec(1))
	t.addTimer(nTask)
	return nTask
}

func (t *TimingWheel) ScheduleFunc(expire time.Duration, executeFunc ExecuteFunc) Result {
	milli := nowNano()
	nTask := newTask(t.ctx, expire, milli+expire, executeFunc, withTaskCycleExec())
	t.addTimer(nTask)
	//t.queue.Push(nTask)
	return nTask
}

func (t *TimingWheel) Stop() {
	t.cancelFunc()
}

func (t *TimingWheel) addHighWheel() {
	// This method is rarely called
	if atomic.LoadPointer(&t.highWheel) == nil {
		ntw := newTimingWheel(t.interval, t.wheelSize, t.pointerTime, t.queue)
		ntw.level = t.level + 1
		atomic.CompareAndSwapPointer(&t.highWheel, nil, unsafe.Pointer(ntw))
	}
}

func (t *TimingWheel) add(task *task) bool {
	expire := task.absExpire
	if !task.isExec() {
		return false
	} else if expire < t.pointerTime+t.tick {
		//t.execTask(task)
		return false
	} else if expire < t.pointerTime+t.interval {
		// add bucket
		idx := expire / t.tick
		index := int64(idx) % t.wheelSize
		bucket := t.buckets[index]
		bucket.add(task)
		roundExpire := idx * t.tick
		if bucket.getAbsExpire() > roundExpire {
			if bucket.setAbsExpire(int64(roundExpire)) {
				t.queue.Offer(bucket)
			}
		}
		return true
	} else {
		t.addHighWheel()
		return (*TimingWheel)(t.highWheel).add(task)
	}
}

func (t *TimingWheel) addTimer(task *task) {
	if !t.add(task) {
		if task.isCanceled() {
			return
		}
		if !task.isExecCount() {
			return
		}

		go func() {
			if !task.isPause() {
				task.execCount++
				task.execFunc(task.ctx)
			}
			task.absExpire = task.expire + nowNano()
			t.addTimer(task)
		}()
	}
}

// Drive pointerTime forward
func (t *TimingWheel) advanceWheel(timeMs time.Duration) {
	if timeMs >= t.pointerTime+t.tick {
		t.pointerTime = timeMs - (timeMs % t.tick)
		if t.highWheel != nil {
			(*TimingWheel)(t.highWheel).advanceWheel(t.pointerTime)
		}
	}
}

func (t *TimingWheel) Size() int64 {
	var total int64 = 0
	for tw := t; tw != nil; tw = (*TimingWheel)(tw.highWheel) {
		for i := 0; i < len(tw.buckets); i++ {
			total += tw.buckets[i].length()
		}
	}
	return total
}

func (t *TimingWheel) getWheelDeep() int64 {
	var total int64
	for tw := t; tw != nil; tw = (*TimingWheel)(tw.highWheel) {
		total++
	}
	return total
}

func (t *TimingWheel) print() {
	var total int64 = 0
	i := 1
	for tw := t; tw != nil; tw = (*TimingWheel)(tw.highWheel) {
		fmt.Println("\n--------------------")
		fmt.Printf("第%d层: tick=%d, wheelSize=%d, interval=%d, pointerTime=%d",
			i, tw.tick, tw.wheelSize, tw.interval, tw.pointerTime)
		for j := range tw.buckets {
			list := tw.buckets[j]
			if list.length() == 0 {
				continue
			}
			total += list.length()
			fmt.Printf(" \n%d号bucket:", j+1)
			list.foreach(func(task *task) {
				fmt.Printf("%d,", task.expire)
			})
		}
		i++
	}
	fmt.Printf("\n总共task:%d \n", t.Size())
}
