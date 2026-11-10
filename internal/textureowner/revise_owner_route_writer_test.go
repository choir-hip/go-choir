package textureowner

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// An owner-mode publication cites a web capture and carries the capture's
// reader snapshot in its entity. Every owner-route commit that copies such an
// entity into a document must be made by the entity kind's registered writer,
// whichever document state the take lands on. The requests run with the writer
// gate enforced and the vocabulary cutover on, as on a computer.
const ownerModeEntities = `[{"source_entity_id":"src_1","target":{"kind":"web_url","uri":"https://example.test/captured","id":"obj:choir.web_capture:owner:key-2","metadata":{"capture_doc":"obj:choir.web_capture:owner:key-2","capture_version":"sha256:cc"}},"selectors":[{"kind":"whole_resource"}],"display":{"mode":"numbered_ref","title":"Captured page"},"evidence":{"state":"available","open_surface":"source"},"reader_snapshot":{"text":"The captured page says the harbor opens at dawn."},"provenance":{"created_by":"texture_write"}}]`

func ownerModeVersion(id string) PublicationVersion {
	v := publishedShapeVersion(id)
	v.SourceEntities = []byte(ownerModeEntities)
	return v
}

func ownerRoutePathSetup(t *testing.T) (*agentcore.Runtime, *Handler, PublicationVersion) {
	t.Helper()
	core, handler := testAPISetup(t)
	if _, err := core.Store().MigrateVocabularyToV2(t.Context()); err != nil {
		t.Fatalf("cutover: %v", err)
	}
	v := ownerModeVersion("pub-owner-mode-v5")
	handler.PublicationVersions = &fakeVersions{versions: map[string]PublicationVersion{v.PublicationVersionID: v}}
	return core, handler, v
}

// withProjection takes v once into the fixture document to learn the text the
// store derives for its body, which a fork's first revision must equal.
func withProjection(t *testing.T, core *agentcore.Runtime, handler *Handler, v PublicationVersion) PublicationVersion {
	t.Helper()
	start := startObservationLifecycle(t, core.Store())
	w := postRevise(t, handler, start.InitialDocument.DocID, start.OwnerID, map[string]any{"client_request_id": "t-read", "sources": []any{sourceOf(v)}})
	if w.Code != http.StatusAccepted {
		t.Fatalf("read take status=%d body=%s", w.Code, w.Body.String())
	}
	reader := objectgraph.WithReaderIfAbsent(context.Background(), objectgraph.WriterOwner)
	taken, err := core.Store().GetLifecycleRevision(reader, start.OwnerID, start.ComputerID, decodeReceipt(t, w).RevisionID)
	if err != nil || taken.Content == "" {
		t.Fatalf("taken=%+v err=%v", taken, err)
	}
	v.Content = taken.Content
	handler.PublicationVersions = &fakeVersions{versions: map[string]PublicationVersion{v.PublicationVersionID: v}}
	return v
}

func TestOwnerModeVersionCommitsOnEveryOwnerRoutePath(t *testing.T) {
	t.Run("take into a live document", func(t *testing.T) {
		core, handler, v := ownerRoutePathSetup(t)
		start := startObservationLifecycle(t, core.Store())
		t.Cleanup(objectgraph.SetModeForTest(objectgraph.WriterGateEnforce))
		w := postRevise(t, handler, start.InitialDocument.DocID, start.OwnerID, map[string]any{"client_request_id": "t-live", "sources": []any{sourceOf(v)}})
		if w.Code != http.StatusAccepted {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})
	t.Run("take into a settled document", func(t *testing.T) {
		core, handler, v := ownerRoutePathSetup(t)
		start := startObservationLifecycle(t, core.Store(), func(req *types.StartLifecycleRequest) {
			req.InitialRevision.Metadata = json.RawMessage(`{"trajectory_id":"` + req.TrajectoryID + `"}`)
		})
		settleDocumentTrajectory(t, core.Store(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID, start.TrajectoryID)
		t.Cleanup(objectgraph.SetModeForTest(objectgraph.WriterGateEnforce))
		w := postRevise(t, handler, start.InitialDocument.DocID, start.OwnerID, map[string]any{"client_request_id": "t-settled", "sources": []any{sourceOf(v)}})
		if w.Code != http.StatusAccepted {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})
	t.Run("take twice, the second finds the entity already written", func(t *testing.T) {
		core, handler, v := ownerRoutePathSetup(t)
		start := startObservationLifecycle(t, core.Store())
		t.Cleanup(objectgraph.SetModeForTest(objectgraph.WriterGateEnforce))
		for _, id := range []string{"t-1", "t-2"} {
			w := postRevise(t, handler, start.InitialDocument.DocID, start.OwnerID, map[string]any{"client_request_id": id, "sources": []any{sourceOf(v)}})
			if w.Code != http.StatusAccepted {
				t.Fatalf("%s status=%d body=%s", id, w.Code, w.Body.String())
			}
		}
	})
	t.Run("verbatim fork", func(t *testing.T) {
		core, handler, v := ownerRoutePathSetup(t)
		v = withProjection(t, core, handler, v)
		t.Cleanup(objectgraph.SetModeForTest(objectgraph.WriterGateEnforce))
		if w := postFork(t, handler, "reader-2", map[string]any{"client_request_id": "f-1", "source": sourceOf(v)}); w.Code != http.StatusAccepted {
			t.Fatalf("fork status=%d body=%s", w.Code, w.Body.String())
		}
	})
	t.Run("fork with text", func(t *testing.T) {
		core, handler, v := ownerRoutePathSetup(t)
		v = withProjection(t, core, handler, v)
		t.Cleanup(objectgraph.SetModeForTest(objectgraph.WriterGateEnforce))
		if w := postFork(t, handler, "reader-3", map[string]any{"client_request_id": "f-2", "text": "answer it", "source": sourceOf(v)}); w.Code != http.StatusAccepted {
			t.Fatalf("fork with text status=%d body=%s", w.Code, w.Body.String())
		}
	})
}
