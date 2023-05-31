package quartz

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

const (
	repeat          = -1
	canceled uint32 = 1
	pause    uint32 = 2
)

var longDelay = nowNano() + time.Hour*24*365*12

type ExecuteFunc func(ctx context.Context)

type taskOption func(t *task)

type task struct {
	next *task
	prev *task

	expire     time.Duration
	absExpire  time.Duration
	execFunc   ExecuteFunc
	ctx        context.Context
	cancelFunc context.CancelFunc

	maxExec   int64
	execCount int64
	state     uint32 // canceled:1

	bucket *nodeList
}

type nodeList struct {
	root      *task
	expire    time.Duration
	absExpire time.Duration
	len       int64
	lock      sync.RWMutex
}

func (l *nodeList) GetDelay() time.Duration {
	return l.getAbsExpire() - nowNano()
}

func (l *nodeList) Cycle() bool {
	return false
}

func newTask(ctx context.Context, expire time.Duration, absExpire time.Duration, exec ExecuteFunc, ops ...taskOption) *task {
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	t := &task{
		ctx:        cancelCtx,
		cancelFunc: cancelFunc,
		expire:     expire,
		absExpire:  absExpire,
		execFunc:   exec,
	}

	for _, op := range ops {
		op(t)
	}

	return t
}

func newNodeList() *nodeList {
	root := &task{}
	root.prev = root
	root.next = root
	return &nodeList{
		root:      root,
		len:       0,
		absExpire: longDelay,
		lock:      sync.RWMutex{},
	}
}

func (l *nodeList) length() int64 {
	return atomic.LoadInt64(&l.len)
}

func (l *nodeList) add(task *task) {
	l.lock.Lock()
	defer l.lock.Unlock()

	task.prev = l.root.prev
	l.root.prev.next = task
	l.root.prev = task
	task.next = l.root

	task.bucket = l

	atomic.AddInt64(&l.len, 1)
	return
}

func (l *nodeList) remove(task *task) {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.doRemove(task)
}

func (l *nodeList) doRemove(task *task) {
	if task.bucket == l {
		task.next.prev = task.prev
		task.prev.next = task.next
		task.next = nil
		task.prev = nil
		task.bucket = nil
		atomic.AddInt64(&l.len, -1)
	}
}

func (l *nodeList) foreach(fn func(task *task)) {
	l.lock.RLock()
	defer l.lock.RUnlock()
	for e := l.root.next; e != nil && e != l.root; e = e.next {
		fn(e)
	}
}

func (l *nodeList) removeAll(fn func(task *task)) {
	if l.length() == 0 {
		return
	}
	l.lock.Lock()
	defer l.lock.Unlock()
	for n := l.root.next; n != nil && n != l.root; n = l.root.next {
		l.doRemove(n)
		fn(n)
	}

	l.setAbsExpire(int64(longDelay))
}

func initNodeList(size int) []*nodeList {
	ns := make([]*nodeList, 0, size)
	for i := 0; i < size; i++ {
		ns = append(ns, newNodeList())
	}
	return ns
}

func (l *nodeList) setAbsExpire(absExpire int64) bool {
	return atomic.SwapInt64((*int64)(&l.absExpire), absExpire) != absExpire
}

func (l *nodeList) getAbsExpire() time.Duration {
	return time.Duration(atomic.LoadInt64((*int64)(&l.absExpire)))
}

func (t *task) setState(state uint32) {
	atomic.StoreUint32(&t.state, state)
}

func (t *task) getState() uint32 {
	return atomic.LoadUint32(&t.state)
}

func (t *task) Stop() {
	t.cancelFunc()
	t.canceled()
}

func (t *task) isPause() bool {
	return t.getState()&pause != 0
}

func (t *task) Pause() {
	t.setState(t.getState() | pause)
}

func (t *task) Continue() {
	t.setState(t.getState() & (^pause))
}

func (t *task) isCanceled() bool {
	return t.getState()&canceled != 0
}

func (t *task) canceled() {
	//for t.bucket != nil {
	//	t.bucket.remove(t)
	//}
	t.setState(t.getState() | canceled)
}

func (t *task) isExecCount() bool {
	if t.maxExec == repeat {
		return true
	}
	if t.execCount >= t.maxExec {
		return false
	}
	return true
}

func (t *task) isExec() bool {
	if t.isCanceled() {
		return false
	}
	if !t.isExecCount() {
		return false
	}
	return true
}

func withTaskMaxExec(max int64) taskOption {
	return func(t *task) {
		t.maxExec = max
	}
}

func withTaskCycleExec() taskOption {
	return func(t *task) {
		t.maxExec = repeat
	}
}
