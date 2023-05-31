package quartz

import (
	"fmt"
	"testing"
	"time"
)

type item struct {
	expire time.Duration
	cycle  bool
	fn     func()
}

func (i *item) GetDelay() time.Duration {
	return i.expire
}

func (i *item) Cycle() bool {
	return i.cycle
}

func TestQueue(t *testing.T) {
	queue := NewDelayQueue()
	go func() {
		func() {
			for {
				select {
				case it, ok := <-queue.Poll():
					if !ok {
						return
					}
					it.(*item).fn()
				}
			}
		}()
		fmt.Println(queue.Len())
	}()

	queue.Offer(&item{
		expire: time.Millisecond * 10,
		cycle:  false,
		fn: func() {
			fmt.Println("ok")
		},
	})

	for i := 0; i < 1000; i++ {
		queue.Offer(&item{
			expire: time.Millisecond * 10,
			cycle:  false,
			fn: func() {

			},
		})
	}

	time.Sleep(time.Second * 2)
	queue.Close()
	time.Sleep(time.Second * 1)
}
