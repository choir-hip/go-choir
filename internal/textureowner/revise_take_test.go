package textureowner

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// fakeVersions is the platform's version read in tests.
type fakeVersions struct {
	versions map[string]PublicationVersion
	err      error
	calls    int
}

func (f *fakeVersions) ReadPublicationVersion(_ context.Context, id string) (PublicationVersion, error) {
	f.calls++
	if f.err != nil {
		return PublicationVersion{}, f.err
	}
	v, ok := f.versions[id]
	if !ok {
		return PublicationVersion{}, ErrSourceNotReadable
	}
	return v, nil
}

func testVersion(id, body string) PublicationVersion {
	return PublicationVersion{
		PublicationVersionID: id, OwnerID: "paper-owner", AuthorLabel: "Autopaper", Title: "Harbor",
		RoutePath: "/pub/texture/harbor/v1", ManifestHash: "manifest-" + id, Content: body,
	}
}

func sourceOf(v PublicationVersion) map[string]string {
	return map[string]string{"publication_version_id": v.PublicationVersionID, "manifest_hash": v.ManifestHash}
}

func TestTakeWritesTheSourceBodyWithNoWake(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	wakes := countWakes(core)
	v := testVersion("pub-a-v1", "Harbor opens at dawn.")
	handler.PublicationVersions = &fakeVersions{versions: map[string]PublicationVersion{v.PublicationVersionID: v}}
	doc := start.InitialDocument.DocID
	w := postRevise(t, handler, doc, start.OwnerID, map[string]any{"client_request_id": "take-1", "sources": []any{sourceOf(v)}})
	if w.Code != http.StatusAccepted {
		t.Fatalf("take status=%d body=%s", w.Code, w.Body.String())
	}
	receipt := decodeReceipt(t, w)
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	head := snapshot.HeadRevision
	if head.RevisionID != receipt.RevisionID || head.AuthorKind != types.AuthorUser || head.Content != v.Content || head.ParentRevisionID != start.InitialRevision.RevisionID {
		t.Fatalf("head=%+v, want a user revision of the source body on the old head", head)
	}
	var prov types.Provenance
	if err := json.Unmarshal(head.Provenance, &prov); err != nil || len(prov.ParentLinks) != 2 {
		t.Fatalf("provenance=%s err=%v", head.Provenance, err)
	}
	var base, source *types.ParentLink
	for i := range prov.ParentLinks {
		switch prov.ParentLinks[i].Kind {
		case types.ParentLinkBase:
			base = &prov.ParentLinks[i]
		case types.ParentLinkSource:
			source = &prov.ParentLinks[i]
		}
	}
	if base == nil || base.RevisionID != start.InitialRevision.RevisionID || source == nil || source.PublicationVersionID != v.PublicationVersionID || source.ManifestHash != v.ManifestHash || source.AuthorLabel != "Autopaper" {
		t.Fatalf("links=%+v", prov.ParentLinks)
	}
	if got := wakes(); got != 0 {
		t.Fatalf("a take woke the desk %d times", got)
	}

	// Replay: the same request is the same receipt; another source under the
	// same request id is a conflict (R7); neither writes.
	again := postRevise(t, handler, doc, start.OwnerID, map[string]any{"client_request_id": "take-1", "sources": []any{sourceOf(v)}})
	if again.Code != http.StatusAccepted || !decodeReceipt(t, again).Replay {
		t.Fatalf("replay status=%d body=%s", again.Code, again.Body.String())
	}
	other := testVersion("pub-b-v1", "Another story.")
	handler.PublicationVersions.(*fakeVersions).versions[other.PublicationVersionID] = other
	conflict := postRevise(t, handler, doc, start.OwnerID, map[string]any{"client_request_id": "take-1", "sources": []any{sourceOf(other)}})
	if conflict.Code != http.StatusConflict {
		t.Fatalf("other source under the same id status=%d body=%s, want 409", conflict.Code, conflict.Body.String())
	}
	after, _ := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if after.HeadRevision.RevisionID != receipt.RevisionID {
		t.Fatalf("replay or conflict wrote: head=%s", after.HeadRevision.RevisionID)
	}
}

func TestReviseRefusesUnknownOrMismatchedSource(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	wakes := countWakes(core)
	v := testVersion("pub-a-v1", "Harbor opens.")
	fake := &fakeVersions{versions: map[string]PublicationVersion{v.PublicationVersionID: v}}
	handler.PublicationVersions = fake
	doc := start.InitialDocument.DocID
	forged := sourceOf(v)
	forged["manifest_hash"] = "forged"
	for name, c := range map[string]struct {
		src  map[string]string
		want int
	}{
		"unknown version (R1)":                 {map[string]string{"publication_version_id": "pub-nope-v1", "manifest_hash": "x"}, http.StatusUnprocessableEntity},
		"wrong manifest hash (R1)":             {forged, http.StatusUnprocessableEntity},
		"a local revision id as a source (R2)": {map[string]string{"publication_version_id": start.InitialRevision.RevisionID, "manifest_hash": "x"}, http.StatusUnprocessableEntity},
	} {
		for _, text := range []string{"", "merge this in"} {
			body := map[string]any{"client_request_id": "bad-" + name + text, "sources": []any{c.src}}
			if text != "" {
				body["text"] = text
			}
			if w := postRevise(t, handler, doc, start.OwnerID, body); w.Code != c.want {
				t.Errorf("%s text=%q: status=%d body=%s, want %d", name, text, w.Code, w.Body.String(), c.want)
			}
		}
	}
	// R9: the platform down is a 503 and nothing commits.
	fake.err = errors.New("connection refused")
	if w := postRevise(t, handler, doc, start.OwnerID, map[string]any{"client_request_id": "down", "sources": []any{sourceOf(v)}}); w.Code != http.StatusServiceUnavailable {
		t.Errorf("platform down: status=%d body=%s, want 503", w.Code, w.Body.String())
	}
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || snapshot.HeadRevision.RevisionID != start.InitialRevision.RevisionID {
		t.Fatalf("a refused source moved the head: %+v err=%v", snapshot.HeadRevision, err)
	}
	if wakes() != 0 {
		t.Fatal("a refused source woke the desk")
	}
}

func TestTakeRefusesPrivateEmbeds(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	v := testVersion("pub-e-v1", "Evidence.")
	v.BodyDoc = json.RawMessage(`{"schema":"choir.texture_doc.v1","doc":{"type":"doc","content":[{"type":"embed","attrs":{"id":"e1","kind":"capsule_evidence","target":{"digest":"` + string(make([]byte, 0)) + `0000000000000000000000000000000000000000000000000000000000000000","operation":"op"}}}]}}`)
	handler.PublicationVersions = &fakeVersions{versions: map[string]PublicationVersion{v.PublicationVersionID: v}}
	w := postRevise(t, handler, start.InitialDocument.DocID, start.OwnerID, map[string]any{"client_request_id": "embed", "sources": []any{sourceOf(v)}})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s, want 422", w.Code, w.Body.String())
	}
}

func TestTakeOnSettledDocumentContinuesThenCommits(t *testing.T) {
	core, handler := testAPISetup(t)
	s := core.Store()
	start := startObservationLifecycle(t, s, func(req *types.StartLifecycleRequest) {
		req.InitialRevision.Metadata = json.RawMessage(`{"trajectory_id":"` + req.TrajectoryID + `"}`)
	})
	docID := start.InitialDocument.DocID
	wakes := countWakes(core)
	settleDocumentTrajectory(t, s, start.OwnerID, start.ComputerID, docID, start.TrajectoryID)
	v := testVersion("pub-s-v1", "Taken onto a settled document.")
	handler.PublicationVersions = &fakeVersions{versions: map[string]PublicationVersion{v.PublicationVersionID: v}}
	body := map[string]any{"client_request_id": "take-settled", "sources": []any{sourceOf(v)}}
	w := postRevise(t, handler, docID, start.OwnerID, body)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	receipt := decodeReceipt(t, w)
	doc, err := s.GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, docID)
	if err != nil || doc.TrajectoryID == start.TrajectoryID || doc.CurrentRevisionID != receipt.RevisionID {
		t.Fatalf("doc=%+v err=%v, want the take as head on a new trajectory", doc, err)
	}
	if wakes() != 0 {
		t.Fatal("a take on a settled document woke the desk")
	}
	// A retry finds the head it wrote, though the trajectory (so the derived
	// revision id) changed.
	again := postRevise(t, handler, docID, start.OwnerID, body)
	if again.Code != http.StatusAccepted || !decodeReceipt(t, again).Replay {
		t.Fatalf("retry status=%d body=%s, want a replay", again.Code, again.Body.String())
	}
}
