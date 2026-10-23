package store

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The slow-read line of ListRunMemoryEntriesForAgent. Ways this could fail,
// written first: a fast call logs; a slow call logs without its site or its
// rows; the rate limit lets every slow call through (a stall would then log
// hundreds a minute) or swallows the first; the suppressed count is lost, so
// the line cannot say how many slow calls it stands for.
func TestRunMemoryAgentListSlowLine(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, err := s.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
			RunID: "run-slow-1", OwnerID: "owner-1", AgentID: "agent-desk", Kind: "rlm_inbox_cursor",
			Details: map[string]any{"channel_id": "ch-1", "cursor": int64(i + 1)},
		}); err != nil {
			t.Fatal(err)
		}
	}
	var (
		mu    sync.Mutex
		lines []string
	)
	s.runMemoryAgentList.logf = func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		lines = append(lines, fmt.Sprintf(format, args...))
	}
	read := func(site string) {
		t.Helper()
		got, err := s.ListRunMemoryEntriesForAgent(WithRunMemoryReadSite(ctx, site), "owner-1", "agent-desk", []string{"rlm_inbox_cursor"})
		if err != nil || len(got) != 3 {
			t.Fatalf("list: %d rows, %v; want 3", len(got), err)
		}
	}

	// Default threshold: a read of three rows is far under 250 ms.
	before := s.RunMemoryAgentListCalls()
	read("desk_cell_start")
	if len(lines) != 0 {
		t.Fatalf("a fast read logged: %q", lines)
	}
	if got := s.RunMemoryAgentListCalls() - before; got != 1 {
		t.Fatalf("call counter moved by %d, want 1", got)
	}

	// Threshold of one nanosecond: every call is slow.
	s.runMemoryAgentList.slowAfter = time.Nanosecond
	read("desk_cell_start")
	read("commit_recovery")
	read("capsule_cell_start")
	if len(lines) != 1 {
		t.Fatalf("three slow reads in a minute logged %d lines, want 1: %q", len(lines), lines)
	}
	for _, want := range []string{"run_memory_agent_list slow", "site=desk_cell_start", "agent=agent-desk", "kinds=1", "rows_returned=3",
		"conn_wait_ms=", "first_row_ms=", "iterate_ms=", "elapsed_ms=", "calls_since_open=2", "slow_suppressed=0"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("line lacks %q: %s", want, lines[0])
		}
	}

	// Past the rate limit, the next slow read logs and reports the two it stood for.
	o := &s.runMemoryAgentList
	o.mu.Lock()
	o.last = time.Now().Add(-2 * runMemorySlowEvery)
	o.mu.Unlock()
	read("commit_recovery")
	if len(lines) != 2 {
		t.Fatalf("after the window, %d lines, want 2: %q", len(lines), lines)
	}
	if !strings.Contains(lines[1], "site=commit_recovery") || !strings.Contains(lines[1], "slow_suppressed=2") {
		t.Errorf("second line = %s", lines[1])
	}

	// An unlabelled caller is named as such.
	o.mu.Lock()
	o.last = time.Now().Add(-2 * runMemorySlowEvery)
	o.mu.Unlock()
	if _, err := s.ListRunMemoryEntriesForAgent(ctx, "owner-1", "agent-desk", nil); err != nil {
		t.Fatal(err)
	}
	if got := lines[len(lines)-1]; !strings.Contains(got, "site=unlabelled") || !strings.Contains(got, "kinds=0") {
		t.Errorf("unlabelled line = %s", got)
	}
}
