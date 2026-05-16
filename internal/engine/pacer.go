package engine

import (
	"context"
	"sync"
	"time"
)

type pacer interface {
	Wait(context.Context) error
}

type noopPacer struct{}

func (noopPacer) Wait(context.Context) error {
	return nil
}

type qpsPacer struct {
	mu       sync.Mutex
	next     time.Time
	interval time.Duration
}

func newPacer(qps float64) pacer {
	if qps <= 0 {
		return noopPacer{}
	}

	interval := time.Duration(float64(time.Second) / qps)
	if interval <= 0 {
		interval = time.Nanosecond
	}

	return &qpsPacer{interval: interval}
}

func (p *qpsPacer) Wait(ctx context.Context) error {
	p.mu.Lock()
	now := time.Now()
	if p.next.IsZero() || now.After(p.next) {
		p.next = now
	}
	scheduled := p.next
	p.next = p.next.Add(p.interval)
	p.mu.Unlock()

	delay := time.Until(scheduled)
	if delay <= 0 {
		return ctx.Err()
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
