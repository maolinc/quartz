## quartz
使用时间轮实现的调度框架; 零依赖性

## 使用
```shell
go get -u github.com/maolinc/quartz
```
```go
// 创建调度器，tick间隔周期， wheelSize时间轮格子
scheduler := NewTimingWheel(tick time.Duration, wheelSize int64)
```

1. 单次任务
```go
scheduler := NewTimingWheel(time.Millisecond*10, 60)
scheduler.Run()
result := scheduler.AfterFunc(time.Second, func(ctx context.Context) {
    // todo
})
// 停止任务
result.Stop()
```

2. 周期任务
```go
scheduler := NewTimingWheel(time.Millisecond*10, 60)
scheduler.Run()
// 任务1
result := scheduler.ScheduleFunc(time.Second, func(ctx context.Context) {
    // todo
})
// 任务2
result := scheduler.ScheduleFunc(time.Second, func(ctx context.Context) {
    // todo
})
// 暂停
result.Pause()
// 继续
result.Continue()
// 停止任务，只会停止任务1，任务停止后对任务在操作便失效
result.Stop()

// 停止所有任务
scheduler.Stop()
```

## 架构
![无标题](https://github.com/maolinc/quartz/assets/82015883/bb78c4b4-698a-4834-b000-7bb79d109adb)

使用延时队列解决空转性能消耗
所有任务都在链表中，任务的添加和删除O(1)
