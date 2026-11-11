package textureowner

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure list: edition_owner.go (D1-D4).

func getEdition(f *algoFixture, user string, mutate ...func(*http.Request)) (int, map[string]any, string) {
	w := f.call(http.MethodGet, "/api/texture/edition", nil, append([]func(*http.Request){func(r *http.Request) { r.Header.Set("X-Authenticated-User", user) }}, mutate...)...)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out, w.Header().Get("Cache-Control")
}

func TestEditionRouteFindsTheOwnersEdition(t *testing.T) {
	f := newAlgoFixture(t, "x")
	if code, got, _ := getEdition(f, "user-1"); code != http.StatusOK || got["found"] == true { // D3
		t.Fatalf("no edition: status=%d answer=%v", code, got)
	}
	_ = startTextDoc(t, f.core.Store(), "doc-edition", "  my   EDITION ", "# What I follow\n\n- Ukraine\n")
	_ = startTextDoc(t, f.core.Store(), "doc-other", "Interview, 11 Nov", "talk")
	code, got, cc := getEdition(f, "user-1")
	if code != http.StatusOK || got["found"] != true || got["doc_id"] != "doc-edition" || got["revision_id"] == "" {
		t.Fatalf("status=%d answer=%v", code, got)
	}
	if cc != "no-store" {
		t.Fatalf("cache-control=%q", cc)
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "Ukraine") { // D4
		t.Fatalf("D4: text leaked: %s", raw)
	}
	if _, got, _ := getEdition(f, "user-2"); got["found"] == true { // D1
		t.Fatalf("D1: user-2 saw %v", got)
	}
}

// settleAndArchive settles a startTextDoc document's trajectory (a live one
// cannot be archived), then archives the document.
func settleAndArchive(t *testing.T, f *algoFixture, docID string) {
	t.Helper()
	ctx, s := context.Background(), f.core.Store()
	owner, computer, traj := "user-1", "autoputer-test", "trajectory-"+docID
	snap, err := s.GetLifecycleSnapshot(ctx, owner, computer, traj)
	if err != nil {
		t.Fatal(err)
	}
	for _, work := range snap.WorkItems {
		if work.Status != types.WorkItemOpen {
			continue
		}
		req := types.SettleLifecycleWorkRequest{OwnerID: owner, ComputerID: computer, CommandID: "settle-work:" + work.WorkItemID, TrajectoryID: traj,
			WorkItemID: work.WorkItemID, ActingAgentID: "texture:" + docID, ResultRef: snap.Document.CurrentRevisionID}
		req.CommandDigest, _ = store.ComputeSettleLifecycleWorkDigest(req)
		if _, err := s.SettleLifecycleWork(ctx, req); err != nil {
			t.Fatalf("settle work: %v", err)
		}
	}
	snap, err = s.GetLifecycleSnapshot(ctx, owner, computer, traj)
	if err != nil {
		t.Fatal(err)
	}
	req := types.SettleLifecycleTrajectoryRequest{OwnerID: owner, ComputerID: computer, CommandID: "settle:" + traj, TrajectoryID: traj,
		ExpectedLifecycleVersion: snap.Trajectory.LifecycleVersion, ExpectedHeadRevisionID: snap.Document.CurrentRevisionID}
	req.CommandDigest, _ = store.ComputeSettleLifecycleTrajectoryDigest(req)
	if _, err := s.SettleLifecycleTrajectory(ctx, req); err != nil {
		t.Fatalf("settle trajectory: %v", err)
	}
	if _, err := s.ArchiveTextureDocumentAuthority(ctx, docID, owner, computer); err != nil {
		t.Fatalf("archive: %v", err)
	}
}

func TestEditionRouteDuplicatesLatestWinsAndArchivedIsSkipped(t *testing.T) { // D1, D2
	f := newAlgoFixture(t, "x")
	_ = startTextDoc(t, f.core.Store(), "doc-edition-old", "My edition", "old")
	time.Sleep(20 * time.Millisecond)
	_ = startTextDoc(t, f.core.Store(), "doc-edition-new", "My Edition", "new")
	for i := 0; i < 3; i++ { // stable across reads
		if _, got, _ := getEdition(f, "user-1"); got["doc_id"] != "doc-edition-new" {
			t.Fatalf("D2: read %d answered %v, want the most recently updated", i, got)
		}
	}
	settleAndArchive(t, f, "doc-edition-new")
	if _, got, _ := getEdition(f, "user-1"); got["doc_id"] != "doc-edition-old" {
		t.Fatalf("D1: archived edition not skipped: %v", got)
	}
	settleAndArchive(t, f, "doc-edition-old")
	if _, got, _ := getEdition(f, "user-1"); got["found"] == true {
		t.Fatalf("D1: only archived editions, answered %v", got)
	}
}

func TestEditionRouteIsOwnerOnly(t *testing.T) {
	f := newAlgoFixture(t, "x")
	_ = startTextDoc(t, f.core.Store(), "doc-edition", "My edition", "x")
	if code, _, _ := getEdition(f, "user-1", func(r *http.Request) { r.RemoteAddr = "" }); code != http.StatusForbidden {
		t.Fatalf("in-process status=%d", code)
	}
	if code, _, _ := getEdition(f, ""); code != http.StatusUnauthorized {
		t.Fatalf("anonymous status=%d", code)
	}
	if w := f.call(http.MethodPost, "/api/texture/edition", map[string]string{}); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("post status=%d", w.Code)
	}
}
