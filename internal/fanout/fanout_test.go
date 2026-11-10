package fanout

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Ways the helper can fail, written before the code (docs/evidence/
// jev-measurement-2026-11-10.md, "Build: failure list", Fan-out): results in
// arrival order, a failed task discarding the others, goroutine leaks on
// cancel, a panic killing the process, a retry that duplicates a judgment,
// bad bounds, and more tasks in flight than the limit.

func TestDoKeepsInputOrder(t *testing.T) {
	got := Do(context.Background(), 8, 20, func(_ context.Context, i int) (int, error) {
		time.Sleep(time.Duration(20-i) * time.Millisecond) // later indexes finish first
		return i * 10, nil
	})
	for i, r := range got {
		if r.Err != nil || r.Value != i*10 {
			t.Fatalf("slot %d = %+v", i, r)
		}
	}
}

func TestDoPartialFailureKeepsOthers(t *testing.T) {
	boom := errors.New("boom")
	got := Do(context.Background(), 4, 6, func(_ context.Context, i int) (string, error) {
		if i == 2 {
			return "", boom
		}
		return "ok", nil
	})
	for i, r := range got {
		if i == 2 {
			if !errors.Is(r.Err, boom) {
				t.Fatalf("slot 2 = %+v", r)
			}
			continue
		}
		if r.Err != nil || r.Value != "ok" {
			t.Fatalf("slot %d lost to a sibling's failure: %+v", i, r)
		}
	}
}

func TestDoRespectsLimit(t *testing.T) {
	var cur, peak atomic.Int64
	Do(context.Background(), 3, 30, func(_ context.Context, _ int) (struct{}, error) {
		n := cur.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		cur.Add(-1)
		return struct{}{}, nil
	})
	if peak.Load() > 3 || peak.Load() < 2 {
		t.Fatalf("peak concurrency = %d, want 2..3", peak.Load())
	}
}

func TestDoCancelStartsNoMore(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var started atomic.Int64
	got := Do(ctx, 2, 50, func(ctx context.Context, i int) (int, error) {
		started.Add(1)
		if i == 1 {
			cancel()
		}
		time.Sleep(5 * time.Millisecond)
		return i, nil
	})
	if started.Load() >= 50 {
		t.Fatalf("all %d tasks started after cancel", started.Load())
	}
	cancelled := 0
	for _, r := range got {
		if errors.Is(r.Err, context.Canceled) {
			cancelled++
		}
	}
	if cancelled == 0 {
		t.Fatal("no slot reports the cancel")
	}
}

func TestDoNoGoroutineLeak(t *testing.T) {
	before := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	Do(ctx, 8, 100, func(context.Context, int) (int, error) { return 0, nil })
	Do(context.Background(), 8, 100, func(context.Context, int) (int, error) { return 0, nil })
	time.Sleep(20 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > before+1 {
		t.Fatalf("goroutines %d -> %d", before, after)
	}
}

func TestDoPanicIsAnError(t *testing.T) {
	got := Do(context.Background(), 2, 3, func(_ context.Context, i int) (int, error) {
		if i == 1 {
			panic("bad")
		}
		return i, nil
	})
	if got[1].Err == nil || got[0].Err != nil || got[2].Err != nil {
		t.Fatalf("results = %+v", got)
	}
}

func TestDoNeverRetries(t *testing.T) {
	var mu sync.Mutex
	calls := map[int]int{}
	Do(context.Background(), 4, 10, func(_ context.Context, i int) (int, error) {
		mu.Lock()
		calls[i]++
		mu.Unlock()
		return 0, errors.New("fail")
	})
	for i := 0; i < 10; i++ {
		if calls[i] != 1 {
			t.Fatalf("task %d ran %d times", i, calls[i])
		}
	}
}

func TestDoBounds(t *testing.T) {
	if got := Do(context.Background(), 0, 0, func(context.Context, int) (int, error) { return 0, nil }); len(got) != 0 {
		t.Fatalf("n=0 gave %d results", len(got))
	}
	if got := Do(context.Background(), -5, 3, func(_ context.Context, i int) (int, error) { return i, nil }); len(got) != 3 || got[2].Value != 2 {
		t.Fatalf("limit<1 = %+v", got)
	}
	if got := Do(context.Background(), 100, 2, func(_ context.Context, i int) (int, error) { return i, nil }); len(got) != 2 {
		t.Fatalf("limit>n = %+v", got)
	}
}
