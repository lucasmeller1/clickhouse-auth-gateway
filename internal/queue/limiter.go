package limiter

import (
	"context"
)

type ExportLimiter struct {
	slots chan struct{}
}

func NewExportLimiter(maxConcurrent int) *ExportLimiter {
	return &ExportLimiter{
		slots: make(chan struct{}, maxConcurrent),
	}
}

// Acquire blocks until a slot is available or context is cancelled.
func (l *ExportLimiter) Acquire(ctx context.Context) error {
	select {
	case l.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release frees one slot.
func (l *ExportLimiter) Release() {
	select {
	case <-l.slots:
	default:
	}
}

// Active returns number of occupied slots
func (l *ExportLimiter) Active() int {
	return len(l.slots)
}

// Capacity returns max slots
func (l *ExportLimiter) Capacity() int {
	return cap(l.slots)
}
