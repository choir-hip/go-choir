// Package fanout is the one bounded fan-out every Jev call site shares
// (docs/evidence/jev-measurement-2026-11-10.md, "Build: failure list"): run n
// independent tasks with at most limit in flight and hand back each task's
// own result in input order.
package fanout

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// Result is one task's outcome. A task that never started (the context ended
// first) carries the context's error.
type Result[T any] struct {
	Value T
	Err   error
}

// Do runs fn(ctx, i) for i in [0, n) with at most limit tasks at once and
// returns the n results indexed by i, never by arrival. One task's error never
// cancels or discards another's: the caller keeps every answer and decides
// what a failed slot means. A panic in a task becomes that slot's error. Do
// returns only after every started task has returned; once ctx is done no
// further task starts. A task is never retried.
func Do[T any](ctx context.Context, limit, n int, fn func(ctx context.Context, i int) (T, error)) []Result[T] {
	results := make([]Result[T], max(n, 0))
	if n <= 0 {
		return results
	}
	workers := min(max(limit, 1), n)
	var next atomic.Int64
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1)) - 1
				if i >= n {
					return
				}
				if err := ctx.Err(); err != nil {
					results[i].Err = err
					continue
				}
				results[i] = run(ctx, i, fn)
			}
		}()
	}
	wg.Wait()
	return results
}

func run[T any](ctx context.Context, i int, fn func(ctx context.Context, i int) (T, error)) (r Result[T]) {
	defer func() {
		if p := recover(); p != nil {
			r = Result[T]{Err: fmt.Errorf("fanout: task %d panicked: %v", i, p)}
		}
	}()
	r.Value, r.Err = fn(ctx, i)
	return r
}
