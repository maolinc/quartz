package quartz

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

const (
	Repeat   = -1
	canceled = 1
)

var longDelay = nowNano() + time.Hour*24*365*12

type ExecuteFunc func(ctx context.Context)

type taskOption func(t *task)

type task struct {
	expire     time.Duration
	absExpire  time.Duration
	execFunc   ExecuteFunc
	ctx        context.Context
	cancelFunc context.CancelFunc

	maxExec   int64
	execCount int64
	offset    int64
	state     int32 // canceled:1
	id        int64
}

func (t *task) GetDelay() time.Duration {
	return t.expire
}
func (t *task) IsCycle() bool {
	return t.maxExec == Repeat
}
func (t *task) Exec() {
	t.execFunc(t.ctx)
}

type node struct {
	next *node
	prev *node
	task *task
}

type nodeList struct {
	root      *node
	expire    time.Duration
	absExpire time.Duration
	len       int64
	lock      sync.RWMutex
}

func (l *nodeList) GetDelay() time.Duration {
	return l.absExpire - nowNano()
}
func (l *nodeList) IsCycle() bool {
	return false
}
func (l *nodeList) Exec() {

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
	root := &node{}
	root.prev = root
	root.next = root
	return &nodeList{
		root:      root,
		len:       0,
		lock:      sync.RWMutex{},
		absExpire: longDelay,
	}
}

func (l *nodeList) length() int64 {
	return atomic.LoadInt64(&l.len)
}

func (l *nodeList) add(task *task) {
	l.lock.Lock()
	defer l.lock.Unlock()
	newNode := &node{task: task}
	newNode.prev = l.root.prev
	l.root.prev.next = newNode
	l.root.prev = newNode
	newNode.next = l.root
	atomic.AddInt64(&l.len, 1)
}

func (l *nodeList) poll() *task {
	if l.length() == 0 {
		return nil
	}
	l.lock.Lock()
	defer l.lock.Unlock()
	tail := l.root.prev
	l.root.prev = tail.prev
	tail.prev.next = l.root
	atomic.AddInt64(&l.len, -1)
	return tail.task
}

func (l *nodeList) foreach(fn func(task *task)) {
	l.lock.RLock()
	defer l.lock.RUnlock()
	for e := l.root.next; e != nil && e != l.root; e = e.next {
		fn(e.task)
	}
}

func (l *nodeList) removeAll(fn func(expire time.Duration, task *task)) {
	l.lock.Lock()
	if l.length() == 0 {
		return
	}
	expir := l.absExpire
	for n := l.root.next; n != nil && n != l.root; n = n.next {
		fn(expir, n.task)
		n.prev = nil
	}
	l.root = &node{}
	l.root.next = l.root
	l.root.prev = l.root
	l.len = 0
	l.absExpire = longDelay
	l.lock.Unlock()
}

func initNodeList(size int) []*nodeList {
	ns := make([]*nodeList, 0, size)
	for i := 0; i < size; i++ {
		ns = append(ns, newNodeList())
	}
	return ns
}

func (l *nodeList) setExpire(expire int64) bool {
	return atomic.SwapInt64((*int64)(&l.expire), expire) != expire
}

func (l *nodeList) getExpire() time.Duration {
	return time.Duration(atomic.LoadInt64((*int64)(&l.expire)))
}

func (l *nodeList) setAbsExpire(absExpire int64) bool {
	return atomic.SwapInt64((*int64)(&l.absExpire), absExpire) != absExpire
}

func (l *nodeList) getAbsExpire() time.Duration {
	return time.Duration(atomic.LoadInt64((*int64)(&l.absExpire)))
}

func (t *task) Stop() {
	t.cancelFunc()
	t.canceled()
}

func (t *task) isCanceled() bool {
	return t.state&canceled != 0
}

func (t *task) canceled() {
	t.state |= canceled
}

func (t *task) isExec() bool {
	if t.isCanceled() {
		return false
	}
	if t.maxExec == Repeat {
		return true
	}
	if t.execCount >= t.maxExec {
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
		t.maxExec = Repeat
	}
}
