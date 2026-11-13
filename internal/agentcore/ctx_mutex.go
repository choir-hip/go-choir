package agentcore

import (
	"context"
	"sync"
)

// ctxMutex is a mutex whose waiter gives up with its own context. The zero
// value is ready to use.
type ctxMutex struct {
	once sync.Once
	ch   chan struct{}
}

func (m *ctxMutex) init() { m.once.Do(func() { m.ch = make(chan struct{}, 1) }) }

// lock takes the mutex, or reports false if ctx ended first.
func (m *ctxMutex) lock(ctx context.Context) bool {
	m.init()
	select {
	case m.ch <- struct{}{}:
		return true
	default:
	}
	select {
	case m.ch <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (m *ctxMutex) unlock() { <-m.ch }
