package agentcore

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// The store_stalled holder log (docs/problems/paper-post-refresh-health-
// windows-2026-10-22.md, "Findings: an instrument that names the holder at
// the stall"). When /health's store reads miss their budget, the write
// pool's one connection is held by someone; engine_mutex_live cannot see a
// raw s.db caller, and a goroutine dump taken from outside 1-2 s later finds
// the holder gone. So the 503 branch, after its answer is written, takes one
// goroutine snapshot at once and logs the goroutines inside the Dolt engine
// that are not themselves waiting for a connection, with both pools' stats.
// At most once a minute, and bounded.
const (
	storeStallHolderEvery    = time.Minute
	storeStallHolderMax      = 16
	storeStallHolderLineMax  = 512
	storeStallHolderTotalMax = 8 << 10
	storeStallStackMax       = 8 << 20

	choirModulePrefix  = "github.com/yusefmosiah/go-choir/"
	engineModulePrefix = "github.com/dolthub/"
	connWaitFrame      = "database/sql.(*DB).conn("
)

// storeStallCapture rate-limits the holder log per runtime.
type storeStallCapture struct {
	mu   sync.Mutex
	last time.Time
}

// allow reports whether a capture may run at now, and if so records it.
func (c *storeStallCapture) allow(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.last.IsZero() && now.Sub(c.last) < storeStallHolderEvery {
		return false
	}
	c.last = now
	return true
}

// logStoreStallHolders snapshots every goroutine and logs, in one write, a
// header with both pools' stats and one line per engine goroutine.
func (rt *Runtime) logStoreStallHolders(unanswered []string) {
	dump, truncated := allGoroutineStacks(storeStallStackMax)
	lines, matched := storeStallHolderLines(dump)
	var pools *store.PoolStats
	if rt.store != nil {
		pools = rt.store.PoolStats()
	}
	if pools == nil {
		pools = &store.PoolStats{}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "store_stalled holders: computer=%s unanswered=%s engine_goroutines=%d shown=%d parked_in_conn=%d stack_truncated=%t write_pool{%s} read_pool{%s}",
		rt.cfg.ComputerID, strings.Join(unanswered, ","), matched, len(lines),
		strings.Count(dump, connWaitFrame), truncated, formatPoolStat(pools.Write), formatPoolStat(pools.Read))
	for _, line := range lines {
		b.WriteString("\n")
		b.WriteString(line)
	}
	log.Print(b.String())
}

func formatPoolStat(p store.PoolStat) string {
	return fmt.Sprintf("max_open=%d open=%d in_use=%d idle=%d wait_count=%d wait_ms=%d",
		p.MaxOpen, p.Open, p.InUse, p.Idle, p.WaitCount, p.WaitDurationMS)
}

// allGoroutineStacks is runtime.Stack of every goroutine, grown up to max.
func allGoroutineStacks(max int) (string, bool) {
	buf := make([]byte, 256<<10)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return string(buf[:n]), false
		}
		if len(buf) >= max {
			return string(buf[:n]), true
		}
		next := 2 * len(buf)
		if next > max {
			next = max
		}
		buf = make([]byte, next)
	}
}

// storeStallHolderLines picks, from a runtime.Stack dump, the goroutines
// with a Dolt or go-mysql-server frame that are not parked in
// database/sql.(*DB).conn, and renders each as one compact line: its first
// choir frame (or its choir creator) and its first two engine frames. Only
// goroutines with a choir frame or creator are shown: a connection is taken
// from the pool only by a choir caller, and the engine's own background
// goroutines (about eight per open store, parked for its lifetime) would
// fill the cap. At most storeStallHolderMax lines, each at most
// storeStallHolderLineMax bytes and all together at most
// storeStallHolderTotalMax. matched counts every engine goroutine, shown or
// not.
func storeStallHolderLines(dump string) (lines []string, matched int) {
	var withChoir []string
	for _, block := range strings.Split(dump, "\n\n") {
		line, choir, ok := storeStallHolderLine(block)
		if !ok {
			continue
		}
		matched++
		if choir {
			withChoir = append(withChoir, line)
		}
	}
	total := 0
	for _, line := range withChoir {
		if len(lines) == storeStallHolderMax || total+len(line) > storeStallHolderTotalMax {
			break
		}
		lines = append(lines, line)
		total += len(line)
	}
	return lines, matched
}

func storeStallHolderLine(block string) (line string, hasChoir, ok bool) {
	rows := strings.Split(strings.TrimSpace(block), "\n")
	if len(rows) < 2 || !strings.HasPrefix(rows[0], "goroutine ") {
		return "", false, false
	}
	head := strings.TrimSuffix(strings.TrimPrefix(rows[0], "goroutine "), ":")
	id, state, _ := strings.Cut(head, " ")
	state = strings.TrimSuffix(strings.TrimPrefix(state, "["), "]")
	var engine []string
	choir, creator := "", ""
	for _, row := range rows[1:] {
		if strings.HasPrefix(row, "\t") || row == "" {
			continue
		}
		if created, found := strings.CutPrefix(row, "created by "); found {
			fn, _, _ := strings.Cut(created, " in goroutine ")
			if strings.HasPrefix(fn, choirModulePrefix) {
				creator = compactFrame(fn)
			}
			continue
		}
		if strings.HasPrefix(row, connWaitFrame) {
			return "", false, false
		}
		switch {
		case strings.HasPrefix(row, engineModulePrefix):
			if len(engine) < 2 {
				engine = append(engine, compactFrame(row))
			}
		case strings.HasPrefix(row, choirModulePrefix):
			if choir == "" {
				choir = compactFrame(row)
			}
		}
	}
	if len(engine) == 0 {
		return "", false, false
	}
	hasChoir = choir != "" || creator != ""
	if choir == "" {
		choir = creator
	}
	line = fmt.Sprintf("store_stalled holder g=%s [%s] choir=%s engine=%s", id, state, choir, strings.Join(engine, " < "))
	if len(line) > storeStallHolderLineMax {
		line = line[:storeStallHolderLineMax]
	}
	return line, hasChoir, true
}

// compactFrame drops the module prefix and the argument list of a stack
// frame's function line.
func compactFrame(fn string) string {
	if strings.HasSuffix(fn, ")") {
		if i := strings.LastIndex(fn, "("); i > 0 {
			fn = fn[:i]
		}
	}
	fn = strings.TrimPrefix(fn, choirModulePrefix)
	return strings.TrimPrefix(fn, engineModulePrefix)
}
