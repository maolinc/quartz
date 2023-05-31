package quartz

import "time"

type Result interface {
	// Stop Immediately stop running, the task will be deleted the next time it expires, lazy deletion.
	Stop()
	// Pause the task until resuming operation using Continue()
	Pause()
	// Continue Resuming paused tasks
	Continue()
}

// Scheduler interface
type Scheduler interface {
	// AfterFunc Single task, only executed once
	AfterFunc(expire time.Duration, callback ExecuteFunc) Result

	// ScheduleFunc Recurring tasks
	ScheduleFunc(expire time.Duration, callback ExecuteFunc) Result

	Run()

	// Stop Stopping scheduling will stop all tasks
	Stop()

	Size() int64
}
