package quartz

import (
	"context"
	"sync"
)

type HandlerFunc func(ctx context.Context)

type TaskPool struct {
	wg          sync.WaitGroup
	taskChannel chan HandlerFunc
	maxThread   int
	cancel      context.CancelFunc
}

func NewTaskPool() *TaskPool {
	ctx, cancelFunc := context.WithCancel(context.Background())
	tp := &TaskPool{
		wg:          sync.WaitGroup{},
		taskChannel: make(chan HandlerFunc),
		maxThread:   10,
		cancel:      cancelFunc,
	}

	for i := 0; i < tp.maxThread; i++ {
		go func() {
			for {
				select {
				case fn := <-tp.taskChannel:
					fn(ctx)
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	return tp
}

func (t *TaskPool) Submit(handlerFunc HandlerFunc) {
	t.taskChannel <- handlerFunc
}

func (t *TaskPool) Stop() {
	t.cancel()
	close(t.taskChannel)
}
