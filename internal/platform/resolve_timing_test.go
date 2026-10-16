package platform

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

// Opening an article cost about a second of server time on every request
// (docs/problems/article-open-slow-store-b-scans-2026-10-16.md). A slow
// resolve logs the time of each step, so the fix aims at measured costs.
//
// Ways this could fail, written first: a slow resolve logs nothing; the
// line misses a step, so its time is unaccounted; a fast resolve logs too,
// flooding the journal.
func TestSlowResolveLogsEachStep(t *testing.T) {
	svc, _ := newVersionTestService(t)
	v1 := publishRevision(t, svc, 1, "rev-a", "Harbor opens.")

	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	prevSlow := slowResolveThreshold
	t.Cleanup(func() { slowResolveThreshold = prevSlow })

	slowResolveThreshold = 0
	readBundle(t, svc, v1.RoutePath)
	line := buf.String()
	if !strings.Contains(line, "platform bundle: resolve "+v1.RoutePath) {
		t.Fatalf("slow resolve logged %q", line)
	}
	for _, step := range []string{"route=", "versions=", "version=", "manifest=", "blob=", "blocks=", "spans=", "citations=", "policy=", "entities=", "transclusions=", "provenance=", "overlay=", "total="} {
		if !strings.Contains(line, step) {
			t.Fatalf("slow resolve line %q misses %s", line, step)
		}
	}

	buf.Reset()
	slowResolveThreshold = 1 << 62
	readBundle(t, svc, v1.RoutePath)
	if strings.Contains(buf.String(), "platform bundle: resolve") {
		t.Fatalf("fast resolve logged %q", buf.String())
	}
}
