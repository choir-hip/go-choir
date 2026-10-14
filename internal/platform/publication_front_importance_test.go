package platform

// Ways attaching front importance scores can fail:
//
//  1. A present, valid scores file attaches nothing, or attaches a score to
//     the wrong story.
//  2. A missing, corrupt, wrong-version or stale file changes the front
//     list (adds fields, errors, or reorders) instead of leaving it as it
//     was without scores.
//  3. A story revised after it was scored loses its score before the
//     scorer rescores it.
//  4. A rewritten file is not reread (the mtime cache serves old scores).
//  5. A section outside the four is attached, or a row with no section
//     (scored before sections existed) gains one.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func frontPageJSON(t *testing.T, svc *Service) (*FrontPage, string) {
	t.Helper()
	page, err := svc.ListFrontStories(context.Background(), 0, "")
	if err != nil {
		t.Fatalf("front: %v", err)
	}
	raw, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return page, string(raw)
}

func writeScores(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write scores: %v", err)
	}
}

func TestFrontImportanceAttachesScoresAndFallsBack(t *testing.T) {
	f := newFrontFixture(t)
	_, baseline := frontPageJSON(t, f.svc)
	page, _ := frontPageJSON(t, f.svc)
	ids := map[string]string{}
	for _, s := range page.Stories {
		ids[s.Title] = s.PublicationID
	}

	path := filepath.Join(t.TempDir(), "scores.json")
	f.svc.SetFrontImportancePath(path)

	// Missing file: unchanged output.
	if _, got := frontPageJSON(t, f.svc); got != baseline {
		t.Fatalf("missing file changed the front list:\n got %s\nwant %s", got, baseline)
	}

	// Present file: harbor scored at an older revision (failure mode 3),
	// bridge scored promo, canal unscored.
	writeScores(t, path, fmt.Sprintf(`{"version":1,"scored_at":"2026-10-14T00:00:00Z","scores":{
		%q:{"revision":1,"importance":8,"promo":false,"section":"world","scored_at":"2026-10-14T00:00:00Z"},
		%q:{"revision":1,"importance":2,"promo":true,"section":"sports","scored_at":"2026-10-14T00:00:00Z"},
		"pub-not-on-front":{"revision":1,"importance":9}}}`, ids["Harbor reopens"], ids["Bridge closes"]))
	page, _ = frontPageJSON(t, f.svc)
	got := map[string]FrontStory{}
	for _, s := range page.Stories {
		got[s.Title] = s
	}
	if got["Harbor reopens"].Importance != 8 || got["Harbor reopens"].Promo {
		t.Fatalf("harbor = %+v, want importance 8 not promo", got["Harbor reopens"])
	}
	if got["Bridge closes"].Importance != 2 || !got["Bridge closes"].Promo {
		t.Fatalf("bridge = %+v, want importance 2 promo", got["Bridge closes"])
	}
	if got["Harbor reopens"].Section != "world" || got["Bridge closes"].Section != "" || got["Canal dredged"].Section != "" {
		t.Fatalf("sections = %q/%q/%q, want world, none (not a section), none (unscored)",
			got["Harbor reopens"].Section, got["Bridge closes"].Section, got["Canal dredged"].Section)
	}
	if got["Canal dredged"].Importance != 0 || got["Canal dredged"].Promo {
		t.Fatalf("canal = %+v, want no score", got["Canal dredged"])
	}
	if fmt.Sprint(storyIDs(page)) != "[Harbor reopens Canal dredged Bridge closes]" {
		t.Fatalf("scores reordered the server list: %v", storyIDs(page))
	}

	// Rewritten file is reread (failure mode 4).
	writeScores(t, path, fmt.Sprintf(`{"version":1,"scores":{%q:{"revision":3,"importance":5,"promo":false}}}`, ids["Harbor reopens"]))
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, future, future)
	page, _ = frontPageJSON(t, f.svc)
	for _, s := range page.Stories {
		if s.Title == "Harbor reopens" && (s.Importance != 5 || s.Section != "") {
			t.Fatalf("rewritten file not reread: harbor = %+v", s)
		}
		if s.Title == "Bridge closes" && (s.Importance != 0 || s.Promo) {
			t.Fatalf("rewritten file kept old bridge score: %+v", s)
		}
	}

	// Corrupt, wrong version, out-of-range: unchanged output.
	for name, body := range map[string]string{
		"corrupt":      `{"version":1,"scores":{`,
		"version":      fmt.Sprintf(`{"version":2,"scores":{%q:{"revision":3,"importance":5}}}`, ids["Harbor reopens"]),
		"out-of-range": fmt.Sprintf(`{"version":1,"scores":{%q:{"revision":3,"importance":11}}}`, ids["Harbor reopens"]),
	} {
		writeScores(t, path, body)
		later := time.Now().Add(time.Duration(len(name)) * time.Second)
		_ = os.Chtimes(path, later, later)
		if _, got := frontPageJSON(t, f.svc); got != baseline {
			t.Fatalf("%s file changed the front list:\n got %s\nwant %s", name, got, baseline)
		}
	}

	// Stale file (older than a day): unchanged output.
	writeScores(t, path, fmt.Sprintf(`{"version":1,"scores":{%q:{"revision":3,"importance":5}}}`, ids["Harbor reopens"]))
	old := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(path, old, old)
	if _, got := frontPageJSON(t, f.svc); got != baseline {
		t.Fatalf("stale file changed the front list:\n got %s\nwant %s", got, baseline)
	}

	// Removed file: unchanged output.
	_ = os.Remove(path)
	if _, got := frontPageJSON(t, f.svc); got != baseline {
		t.Fatalf("removed file changed the front list")
	}
}
