package scheduler

import (
	"context"
	"fmt"
	"time"
)

// Task 是一个被调度的后台作业。Fn 必须幂等：进程被杀后重跑是常态。
type Task struct {
	Name string
	Cron string
	Fn   func(ctx context.Context) error
}

type scheduled struct {
	Task
	spec    CronSpec
	lastRun time.Time
	lastErr error
}

// Scheduler 是零依赖的轻量调度器：一秒粒度扫描 + 区间补偿。
// 每次扫描检查 [lastTick+1s, now] 内是否有匹配秒，短时钟停顿（GC、挂起）不会漏触发。
type Scheduler struct {
	tasks []*scheduled
}

func New() *Scheduler { return &Scheduler{} }

// Add 注册任务；cron 表达式错误在启动时立刻暴露。
func (s *Scheduler) Add(name, cron string, fn func(ctx context.Context) error) error {
	spec, err := ParseCron(cron)
	if err != nil {
		return fmt.Errorf("任务 %s: %w", name, err)
	}
	s.tasks = append(s.tasks, &scheduled{Task: Task{Name: name, Cron: cron, Fn: fn}, spec: spec})
	return nil
}

// Run 阻塞运行直到 ctx 取消。任务panic被捕获并转换为错误，不拖垮整个调度循环。
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var lastTick time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			start := now.Truncate(time.Second)
			if lastTick.IsZero() {
				lastTick = start.Add(-time.Second)
			}
			s.scan(ctx, lastTick.Add(time.Second), start)
			lastTick = start
		}
	}
}

func (s *Scheduler) scan(ctx context.Context, from, to time.Time) {
	for sec := from; !sec.After(to); sec = sec.Add(time.Second) {
		for _, t := range s.tasks {
			if !t.spec.Matches(sec) {
				continue
			}
			if !t.lastRun.Before(sec) {
				continue
			}
			t.lastRun = sec
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.lastErr = fmt.Errorf("panic: %v", r)
						logTask(t.Name, t.lastErr)
					}
				}()
				if err := t.Fn(ctx); err != nil {
					t.lastErr = err
					logTask(t.Name, err)
					return
				}
				t.lastErr = nil
				logTask(t.Name, nil)
			}()
		}
	}
}

// Status 暴露每个任务的上次执行情况，供管理端监控。
type Status struct {
	Name    string     `json:"name"`
	Cron    string     `json:"cron"`
	LastRun *time.Time `json:"last_run"`
	LastErr string     `json:"last_error"`
}

func (s *Scheduler) Status() []Status {
	out := make([]Status, 0, len(s.tasks))
	for _, t := range s.tasks {
		st := Status{Name: t.Name, Cron: t.Cron}
		if !t.lastRun.IsZero() {
			run := t.lastRun
			st.LastRun = &run
		}
		if t.lastErr != nil {
			st.LastErr = t.lastErr.Error()
		}
		out = append(out, st)
	}
	return out
}

func logTask(name string, err error) {
	ts := time.Now().Format("15:04:05")
	if err != nil {
		fmt.Printf("[%s] 调度任务 %s 失败：%v\n", ts, name, err)
		return
	}
	fmt.Printf("[%s] 调度任务 %s 完成\n", ts, name)
}
