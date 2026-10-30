package platform

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"
	"testing"
)

// Opening an article cost about a second of server time on every request
// (docs/archive/problems/article-open-slow-store-b-scans-2026-10-16.md). A slow
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

// The reader polls the bare route every minute only to learn whether a
// newer version exists, and each poll rebuilt the whole bundle (up to
// 394 KB on the paper). The nav view answers with the route, the
// publication and the nav alone. Ways it could fail: its nav differs from
// the full bundle's; it still carries the text, manifest or checks; it
// serves a route the full resolve refuses.
func TestNavViewAnswersWithoutTheBundle(t *testing.T) {
	svc, _ := newVersionTestService(t)
	v1 := publishRevision(t, svc, 1, "rev-a", "Harbor opens.")
	publishRevision(t, svc, 2, "rev-b", "Harbor opens at dawn.")

	full := readBundle(t, svc, v1.PermalinkPath)
	nav, err := svc.GetPublicationNavByRoute(context.Background(), v1.PermalinkPath)
	if err != nil {
		t.Fatalf("nav view: %v", err)
	}
	if nav.Nav != full.Nav || nav.Nav.Latest != 2 || nav.Route != full.Route || nav.Publication != full.Publication {
		t.Fatalf("nav view %+v %+v %+v, full %+v %+v %+v", nav.Nav, nav.Route, nav.Publication, full.Nav, full.Route, full.Publication)
	}
	if nav.Payload.Text != "" || len(nav.Payload.Manifest) != 0 || nav.Artifact.Content != "" || len(nav.Checks.Citations) != 0 {
		t.Fatalf("nav view carries the bundle: %+v", nav)
	}
	if _, err := svc.GetPublicationNavByRoute(context.Background(), v1.RoutePath+"/v9"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("nav view of a missing version: %v", err)
	}
	if _, err := svc.GetPublicationNavByRoute(context.Background(), "/pub/texture/no-such-story"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("nav view of a missing route: %v", err)
	}
}
