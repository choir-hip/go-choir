package objectgraph

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
)

// maxGateSites bounds the distinct "kind|outcome|site" keys a gate keeps;
// further new keys are counted under gateSitesOverflowKey.
const (
	maxGateSites         = 64
	gateSitesOverflowKey = "overflow"
)

// gateSites is a bounded tally of where gate violations came from: the first
// stack frame outside the graph and the store. Counts by kind say a read was
// undeclared; the site says whose read it was.
type gateSites struct {
	mu     sync.Mutex
	counts map[string]uint64
}

func (g *gateSites) tally(kind ObjectKind, outcome string) {
	key := fmt.Sprintf("%s|%s|%s", kind, outcome, callerSite())
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.counts == nil {
		g.counts = map[string]uint64{}
	}
	if _, seen := g.counts[key]; !seen && len(g.counts) >= maxGateSites {
		key = gateSitesOverflowKey
	}
	g.counts[key]++
}

func (g *gateSites) snapshot() map[string]uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make(map[string]uint64, len(g.counts))
	for k, v := range g.counts {
		out[k] = v
	}
	return out
}

func (g *gateSites) reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counts = nil
}

// callerSite names the first frame outside internal/objectgraph and
// internal/store as "function (repo/relative/file.go:line)". Test files in
// those packages count as callers. Runs on the violation path only.
func callerSite() string {
	pcs := make([]uintptr, 48)
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		f, more := frames.Next()
		inGraph := strings.Contains(f.File, "/internal/objectgraph/") || strings.Contains(f.File, "/internal/store/")
		if f.File != "" && (!inGraph || strings.HasSuffix(f.File, "_test.go")) && !strings.HasPrefix(f.Function, "runtime.") {
			return fmt.Sprintf("%s (%s:%d)", f.Function, repoRelative(f.File), f.Line)
		}
		if !more {
			return "unknown"
		}
	}
}

func repoRelative(file string) string {
	for _, marker := range []string{"/internal/", "/cmd/"} {
		if i := strings.LastIndex(file, marker); i >= 0 {
			return file[i+1:]
		}
	}
	if i := strings.LastIndex(file, "/"); i >= 0 {
		return file[i+1:]
	}
	return file
}

var (
	readerGateSites gateSites
	writerGateSites gateSites
)

// ReaderGateSites returns the reader gate's violation sites, keyed
// "kind|outcome|function (file:line)".
func ReaderGateSites() map[string]uint64 { return readerGateSites.snapshot() }

// WriterGateSites is ReaderGateSites for the writer gate.
func WriterGateSites() map[string]uint64 { return writerGateSites.snapshot() }
