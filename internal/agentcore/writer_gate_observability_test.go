package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// ARCH-04 observe mode: the writer gate's violation counts are the oracle for
// the observe-to-enforce flip, so they must be readable without SSH, on the
// product observability route (review non-blocking finding; standing question
// 9). Ways it can fail: the counts are absent from the route; the mode is not
// named, so a reader cannot tell observe counts from an off gate's silence;
// the route mutates or resets the counts.
func TestGuestObservabilityServesWriterGateCounts(t *testing.T) {
	restore := objectgraph.SetWriterGateMode(objectgraph.WriterGateObserve)
	defer restore()
	objectgraph.ResetWriterGateCounts()
	defer objectgraph.ResetWriterGateCounts()
	// One undeclared write of a kind with a declared writer, through the gate.
	kind := objectgraph.ObjectKind("choir.texture_decision")
	id, err := objectgraph.BuildCanonicalID(kind, "user:gate", objectgraph.StableSuffixFromKey("observability"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	meta := json.RawMessage(`{}`)
	object := objectgraph.Object{CanonicalID: id, ObjectKind: kind, OwnerID: "user:gate", ContentHash: objectgraph.ContentHash(kind, nil, meta), Metadata: meta, CreatedAt: now, UpdatedAt: now}
	if err := objectgraph.CheckWriteAs(context.Background(), []objectgraph.Object{object}, nil); err != nil {
		t.Fatalf("observe mode refused a write: %v", err)
	}
	want := objectgraph.WriterGateCounts()
	if len(want) == 0 {
		t.Fatal("fixture: the undeclared write was not counted")
	}

	rt, _ := testRuntime(t)
	read := func() map[string]any {
		req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
		req.Header.Set("X-Authenticated-User", "owner-writer-gate")
		rec := httptest.NewRecorder()
		(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("observability = %d %s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body
	}
	body := read()
	gate, ok := body["objectgraph_writer_gate"].(map[string]any)
	if !ok {
		t.Fatalf("observability has no objectgraph_writer_gate: %v", body)
	}
	if gate["mode"] != "observe" {
		t.Fatalf("writer gate mode = %v, want observe", gate["mode"])
	}
	counts, _ := gate["counts"].(map[string]any)
	for key, n := range want {
		if got, _ := counts[key].(float64); uint64(got) != n {
			t.Fatalf("count %q = %v, want %d (all: %v)", key, counts[key], n, counts)
		}
	}
	read()
	if after := objectgraph.WriterGateCounts(); len(after) != len(want) {
		t.Fatalf("reading the route changed the counts: %v -> %v", want, after)
	}
}
