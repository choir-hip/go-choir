package platform

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// slowResolveThreshold is the resolve time above which corpusd logs each
// step's time. Opening an article cost about a second on every request
// (docs/archive/problems/article-open-slow-store-b-scans-2026-10-16.md).
var slowResolveThreshold = 300 * time.Millisecond

// resolveSteps times the steps of one publication resolve.
type resolveSteps struct {
	start, last time.Time
	steps       []string
}

func newResolveSteps() *resolveSteps {
	now := time.Now()
	return &resolveSteps{start: now, last: now}
}

// mark records the time since the previous mark as the named step.
func (r *resolveSteps) mark(name string) {
	now := time.Now()
	r.steps = append(r.steps, fmt.Sprintf("%s=%dms", name, now.Sub(r.last).Milliseconds()))
	r.last = now
}

// logIfSlow logs the steps when the whole resolve took the threshold or
// longer; a resolve that failed partway logs the steps it reached.
func (r *resolveSteps) logIfSlow(routePath string) {
	total := time.Since(r.start)
	if total < slowResolveThreshold {
		return
	}
	log.Printf("platform bundle: resolve %s %s total=%dms", routePath, strings.Join(r.steps, " "), total.Milliseconds())
}
