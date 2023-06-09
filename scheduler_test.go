package quartz

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestScheduler(t *testing.T) {
	scheduler := NewTimingWheel(time.Millisecond*10, 20)
	scheduler.Run()

	for i := 1; i < 5000000; i++ {
		scheduler.ScheduleFunc(time.Millisecond*10*time.Duration(i), func(ctx context.Context) {

		})
	}
	st := time.Now()
	_ = scheduler.ScheduleFunc(time.Millisecond*200, func(ctx context.Context) {
		ed := time.Now()
		fmt.Println(ed.UnixMilli() - st.UnixMilli() - 200)
		st = ed
	})

	time.Sleep(time.Second * 2)
	//scheduler.Stop()
	for {

	}
}

func TestScheduler2(t *testing.T) {
	scheduler := NewTimingWheel(time.Millisecond*10, 60)
	scheduler.Run()

	var k int64 = 1
	for i := 1; i < 10000000; i++ {
		if i%1000 == 0 {
			k++
		}
		scheduler.ScheduleFunc(time.Millisecond*10*time.Duration(k), func(ctx context.Context) {

		})
	}

	st := time.Now()
	scheduler.ScheduleFunc(time.Millisecond*200, func(ctx context.Context) {
		ed := time.Now()
		fmt.Println(ed.UnixMilli() - st.UnixMilli() - 200)
		st = ed
	})

	for {

	}
}
