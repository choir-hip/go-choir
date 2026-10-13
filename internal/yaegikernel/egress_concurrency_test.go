package yaegikernel

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"
)

// echoEgressHost answers every source search with the query it was asked, so a
// cell can tell whether each caller got its own answer. The first query is
// slow, which is the case the cross-wire probe caught: the reply to "a"
// arrived last and was taken by whichever caller read next
// (problems/parallel-choir-calls-cross-wire-results-2026-10-12.md).
func echoEgressHost(ctx context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
	var p SourceSearchPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	if p.Query == "a" {
		time.Sleep(150 * time.Millisecond)
	}
	return json.Marshal(map[string]string{"query": p.Query})
}

func researchWorker(t *testing.T) *DeskSessionWorker {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("desk session worker spawn requires unix socketpair")
	}
	cfg := deskWorkerConfig(t)
	cfg.Session.Role = SessionRoleResearch
	cfg.Egress = echoEgressHost
	w, err := SpawnDeskSessionWorker(cfg)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	t.Cleanup(w.Close)
	return w
}

// TestEgressConcurrentCallersGetOwnResults runs eight goroutines in one cell,
// each calling choir.SourceSearch with its own query, against a real worker
// process. Every caller must get the answer to its own query.
func TestEgressConcurrentCallersGetOwnResults(t *testing.T) {
	w := researchWorker(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	res, err := w.Eval(ctx, `import "sort"
results := make(chan string, 8)
for _, q := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
	q := q
	go func() {
		r, err := choir.SourceSearch(q, 1)
		if err != nil {
			results <- q + " ERR " + err.Error()
			return
		}
		results <- q + " " + string(r)
	}()
}
got := []string{}
for i := 0; i < 8; i++ {
	got = append(got, <-results)
}
sort.Strings(got)
fmt.Println(strings.Join(got, "\n"))
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("cell error: %s", res.Error)
	}
	lines := strings.Split(strings.TrimSpace(res.Stdout), "\n")
	if len(lines) != 8 {
		t.Fatalf("want 8 results, got %d: %q", len(lines), res.Stdout)
	}
	for _, line := range lines {
		q, body, _ := strings.Cut(line, " ")
		want := `{"query":"` + q + `"}`
		if body != want {
			t.Errorf("caller %q got %s, want %s", q, body, want)
		}
	}
	// The worker stays usable: the routing left no frame behind.
	next, err := w.Eval(ctx, `fmt.Println("next")`)
	if err != nil || next.Error != "" || strings.TrimSpace(next.Stdout) != "next" {
		t.Fatalf("next cell after concurrent egress: err=%v res=%#v", err, next)
	}
}

// TestEgressLateCallerGetsCleanError starts a goroutine that calls
// choir.SourceSearch after its cell has returned. It must get an error naming
// the reason, never another caller's result, and the next cell's own egress
// and result frames must be untouched.
func TestEgressLateCallerGetsCleanError(t *testing.T) {
	w := researchWorker(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	first, err := w.Eval(ctx, `import "time"
late := make(chan string, 1)
go func() {
	time.Sleep(100 * time.Millisecond)
	r, err := choir.SourceSearch("late", 1)
	if err != nil {
		late <- "ERR " + err.Error()
		return
	}
	late <- "OK " + string(r)
}()
`)
	if err != nil || first.Error != "" {
		t.Fatalf("first cell: err=%v res=%#v", err, first)
	}
	time.Sleep(300 * time.Millisecond)
	second, err := w.Eval(ctx, `
r, err := choir.SourceSearch("second", 1)
fmt.Println("own", string(r), err)
fmt.Println("late", <-late)
`)
	if err != nil {
		t.Fatalf("second cell: %v", err)
	}
	if second.Error != "" {
		t.Fatalf("second cell error: %s", second.Error)
	}
	if !strings.Contains(second.Stdout, `own {"query":"second"} <nil>`) {
		t.Errorf("second cell's own call lost its result: %q", second.Stdout)
	}
	lateLine := ""
	for _, line := range strings.Split(second.Stdout, "\n") {
		if strings.HasPrefix(line, "late ") {
			lateLine = line
		}
	}
	if !strings.HasPrefix(lateLine, "late ERR ") || !strings.Contains(lateLine, "after its cell") {
		t.Errorf("late caller must get a clean error naming the cell boundary, got %q", lateLine)
	}
}
