package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const TickDuration = 50 * time.Millisecond

type Task struct {
	period    uint64
	next      uint64
	fn        func()
	cancelled atomic.Bool
}

func (t *Task) Cancel() {
	t.cancelled.Store(true)
}

type Scheduler struct {
	log   *slog.Logger
	mu    sync.Mutex
	tasks []*Task
	tick  atomic.Uint64
}

func New(log *slog.Logger) *Scheduler {
	return &Scheduler{log: log}
}

func (s *Scheduler) Tick() uint64 {
	return s.tick.Load()
}

func (s *Scheduler) Repeat(period int, fn func()) *Task {
	return s.add(uint64(max(1, period)), uint64(max(1, period)), fn)
}

func (s *Scheduler) After(delay int, fn func()) *Task {
	return s.add(0, uint64(max(1, delay)), fn)
}

func (s *Scheduler) add(period, delay uint64, fn func()) *Task {
	t := &Task{period: period, fn: fn}
	s.mu.Lock()
	t.next = s.tick.Load() + delay
	s.tasks = append(s.tasks, t)
	s.mu.Unlock()
	return t
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(TickDuration)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.step()
		}
	}
}

func (s *Scheduler) step() {
	now := s.tick.Add(1)
	s.mu.Lock()
	due := make([]*Task, 0, len(s.tasks))
	kept := s.tasks[:0]
	for _, t := range s.tasks {
		if t.cancelled.Load() {
			continue
		}
		if t.next <= now {
			due = append(due, t)
			if t.period == 0 {
				continue
			}
			t.next = now + t.period
		}
		kept = append(kept, t)
	}
	for i := len(kept); i < len(s.tasks); i++ {
		s.tasks[i] = nil
	}
	s.tasks = kept
	s.mu.Unlock()

	for _, t := range due {
		if !t.cancelled.Load() {
			s.call(t)
		}
	}
}

func (s *Scheduler) call(t *Task) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("scheduled task panicked", "err", fmt.Sprint(r))
		}
	}()
	t.fn()
}
