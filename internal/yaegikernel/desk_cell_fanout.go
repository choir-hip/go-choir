package yaegikernel

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Goroutine fan-out probe (yellow). The deployed evidence for the yaegi fork
// (go.mod replace to github.com/choir-hip/yaegi; upstream traefik/yaegi#1752
// and #1739; docs/problems/yaegi-goroutine-closure-race-2026-11-06.md) and a
// standing regression check: one interpreted cell runs in every self-test
// desk worker, the same session-worker path desk cells take (process or
// gVisor, whichever the worker is confined by), and fans out goroutines.
//
// Failure modes, written first:
//  1. The race returns: an interpreted function literal run as a goroutine
//     panics ("call of nil function", a nil dereference) or corrupts the
//     sum. The cell recovers the panic and prints it; the probe reports
//     ok=false with that text. A panic outside the cell's recover (a
//     goroutine's own) kills the worker: the transport error is reported.
//  2. A lost update: total != 64*63/2 after a round. Reported with the round.
//  3. A deadlock or livelock: the probe's context (fanOutTimeout) expires and
//     the worker is closed by the self-test; reported as the transport error.
//  4. "sync" missing from the desk's import set: the cell fails to compile;
//     reported as the cell error, not as a pass.
//  5. The cell prints nothing or the wrong marker: not ok.
//  6. The worker is unreachable or the probe never ran: the judge refuses a
//     serving worker whose receipt has no fan-out entry.
const (
	fanOutRounds     = 50
	fanOutGoroutines = 64
	fanOutTimeout    = 20 * time.Second
	fanOutMarker     = "FANOUT_OK"
)

// DeskCellFanOut is the probe's receipt entry.
type DeskCellFanOut struct {
	OK         bool   `json:"ok"`
	Rounds     int    `json:"rounds"`
	Goroutines int    `json:"goroutines"`
	DurationMS int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

var fanOutCell = fmt.Sprintf(`import (
	"fmt"
	"sync"
)

fanOut := func() (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprint("panic: ", r)
		}
	}()
	const rounds, n = %d, %d
	want := n * (n - 1) / 2
	for r := 0; r < rounds; r++ {
		var wg sync.WaitGroup
		var mu sync.Mutex
		total := 0
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(k int) {
				defer wg.Done()
				mu.Lock()
				total += k
				mu.Unlock()
			}(i)
		}
		wg.Wait()
		if total != want {
			return fmt.Sprintf("round %%d: total %%d, want %%d", r, total, want)
		}
	}
	// One round where every goroutine calls a shared local closure.
	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0
	add := func(k int) {
		mu.Lock()
		total += k
		mu.Unlock()
	}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			add(k)
		}(i)
	}
	wg.Wait()
	if total != want {
		return fmt.Sprintf("shared closure: total %%d, want %%d", total, want)
	}
	return ""
}

if msg := fanOut(); msg != "" {
	fmt.Println("FANOUT_FAIL", msg)
} else {
	fmt.Println("%s", %d, "rounds", %d, "goroutines")
}
`, fanOutRounds, fanOutGoroutines, fanOutMarker, fanOutRounds, fanOutGoroutines)

// runDeskCellFanOut runs the fan-out cell through eval (a worker's Eval, in
// production) and reports. It never panics and bounds the call by
// fanOutTimeout.
func runDeskCellFanOut(ctx context.Context, eval func(context.Context, string) (SessionResult, error)) *DeskCellFanOut {
	out := &DeskCellFanOut{Rounds: fanOutRounds, Goroutines: fanOutGoroutines}
	cctx, cancel := context.WithTimeout(ctx, fanOutTimeout)
	defer cancel()
	start := time.Now()
	res, err := eval(cctx, fanOutCell)
	out.DurationMS = time.Since(start).Milliseconds()
	switch {
	case err != nil:
		out.Error = "transport: " + err.Error()
	case res.Error != "":
		out.Error = "cell error: " + res.Error
	case strings.Contains(res.Stdout, "FANOUT_FAIL"):
		out.Error = strings.TrimSpace(res.Stdout)
	case !strings.Contains(res.Stdout, fanOutMarker):
		out.Error = "no result marker: " + strings.TrimSpace(res.Stdout)
	default:
		out.OK = true
	}
	return out
}
