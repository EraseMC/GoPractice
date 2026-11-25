package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"math"
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

	statMu  sync.Mutex
	last    time.Time
	tps     [20]float64
	use     [20]float64
	samples int
}

type Stats struct {
	TPS, TickUsage, AverageTPS, AverageTickUsage float64
}

func (s *Scheduler) Stats() Stats {
	s.statMu.Lock()
	defer s.statMu.Unlock()
	if s.samples == 0 {
		return Stats{TPS: 20, AverageTPS: 20}
	}
	cur := (s.samples - 1) % len(s.tps)
	n := min(s.samples, len(s.tps))
	var sumTPS, sumUse float64
	for i := 0; i < n; i++ {
		sumTPS += s.tps[i]
		sumUse += s.use[i]
	}
	return Stats{
		TPS:              round2(s.tps[cur]),
		TickUsage:        round2(s.use[cur] * 100),
		AverageTPS:       round2(sumTPS / float64(n)),
		AverageTickUsage: round2(sumUse / float64(n) * 100),
	}
}

func (s *Scheduler) record(start time.Time, work time.Duration) {
	s.statMu.Lock()
	defer s.statMu.Unlock()
	interval := TickDuration.Seconds()
	if !s.last.IsZero() {
		interval = start.Sub(s.last).Seconds()
	}
	s.last = start
	i := s.samples % len(s.tps)
	s.tps[i] = min(20, 1/max(0.001, interval))
	s.use[i] = min(1, work.Seconds()/TickDuration.Seconds())
	s.samples++
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
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
			start := time.Now()
			s.step()
			s.record(start, time.Since(start))
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
