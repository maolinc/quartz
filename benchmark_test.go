package quartz

import (
	"context"
	"testing"
	"time"
)

func genD(i int) time.Duration {
	return time.Duration(i) * 30 * time.Millisecond
}

func Benchmark_quartz(b *testing.B) {

	scheduler := NewTimingWheel(time.Millisecond*10, 256)
	scheduler.Run()
	//defer scheduler.Stop()
	b.ResetTimer()

	cases := []struct {
		name string
		N    int // the data size (i.e. number of existing timers)
	}{
		{"1m", 1000000},
		//{"5m", 5000000},
		//{"10m", 10000000},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			res := make([]Result, c.N)
			for i := 0; i < len(res); i++ {
				res[i] = scheduler.AfterFunc(genD(i), func(ctx context.Context) {

				})
			}
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				scheduler.AfterFunc(time.Second, func(ctx context.Context) {

				}).Stop()
			}

			b.StopTimer()

			for i := 0; i < len(res); i++ {
				res[i].Stop()
			}
		})
	}
	scheduler.Stop()
}

func Benchmark_Stdlib(b *testing.B) {
	cases := []struct {
		name string
		N    int // the data size (i.e. number of existing timers)
	}{
		{"1m", 1000000},
		//{"5m", 5000000},
		//{"10m", 10000000},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			base := make([]*time.Timer, c.N)
			for i := 0; i < len(base); i++ {
				base[i] = time.AfterFunc(genD(i), func() {})
			}
			//b.ResetTimer()
			//
			//for i := 0; i < b.N; i++ {
			//	time.AfterFunc(time.Second, func() {}).Stop()
			//}

			b.StopTimer()
			for i := 0; i < len(base); i++ {
				base[i].Stop()
			}
		})
	}
}
