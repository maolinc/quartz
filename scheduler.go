package quartz

import "time"

type Result interface {
	Stop()
}

// Scheduler 定时器接口
type Scheduler interface {
	// AfterFunc 一次性定时器
	AfterFunc(expire time.Duration, callback ExecuteFunc) Result

	// ScheduleFunc 周期性定时器
	ScheduleFunc(expire time.Duration, callback ExecuteFunc) Result

	// Run 运行
	Run()

	// Stop 停止所有定时器
	Stop()
}
