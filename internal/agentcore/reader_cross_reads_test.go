package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The cross reads of the custody section (plan section 8.4) that live in
// agentcore, pinned as counted crossings (MA3.3). A built-in kind only
// observes, so each read works; the counters read it as the desk that crossed.
// MA3.7 resolves them, and each test is its refusal target.

func readerDelta(key string) func() uint64 {
	before := objectgraph.ReaderGateCounts()[key]
	return func() uint64 { return objectgraph.ReaderGateCounts()[key] - before }
}

// X7: management's ProductAPI GET /api/texture/ reaches the Texture routes on
// the run's context, so the route's store reads declare management.
func TestProductAPITextureReadByManagementCountsAsACrossing(t *testing.T) {
	rt, _ := testRuntime(t)
	canonical := server.NewServer("x7-reader-test", "0")
	canonical.HandleFunc("/api/texture/documents", func(w http.ResponseWriter, r *http.Request) {
		// The route's own store read, on the request context.
		_, _ = rt.store.SearchDocuments(r.Context(), "anything", "owner-x7", 1)
		w.WriteHeader(http.StatusOK)
	})
	rt.SetProductAPIHandler(canonical, "")
	key := "choir.texture_document|" + string(objectgraph.WriterManagement) + "|wrong_reader"
	moved := readerDelta(key)

	run := func(ctx context.Context) {
		t.Helper()
		if _, err := rt.productAPIRequest(ctx, "owner-x7", "", "GET", "/api/texture/documents", nil); err != nil {
			t.Fatal(err)
		}
	}
	run(withDeskReader(context.Background(), agentprofile.Texture))
	run(objectgraph.WithReader(context.Background(), objectgraph.WriterOwner))
	if n := moved(); n != 0 {
		t.Fatalf("texture's and the owner's reads were counted as management's: %d", n)
	}
	run(withDeskReader(context.Background(), agentprofile.Management))
	if moved() == 0 {
		t.Fatalf("counter %q did not move (counts: %v)", key, objectgraph.ReaderGateCounts())
	}
}

// X18: research's get_run_memory_entry reads any run's entry by id. The
// entry kind is a commons kind in its own table, so the scope check is the
// tool's: an entry of another run counts as the desk reading it.
func TestRunMemoryEntryOfAnotherRunCountsAsACrossing(t *testing.T) {
	rt, st := testRuntime(t)
	ctx := context.Background()
	for _, e := range []types.RunMemoryEntry{
		{EntryID: "x18-own", RunID: "run-x18-self", OwnerID: "owner-x18", AgentID: "agent-x18", Kind: types.RunMemoryEntryMessage, Role: "user", Message: json.RawMessage(`{"role":"user","content":"a"}`)},
		{EntryID: "x18-other", RunID: "run-x18-other", OwnerID: "owner-x18", AgentID: "agent-x18", Kind: types.RunMemoryEntryMessage, Role: "user", Message: json.RawMessage(`{"role":"user","content":"b"}`)},
	} {
		if _, err := st.AppendRunMemoryEntry(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	tool := newGetRunMemoryEntryTool(rt)
	key := "choir.run_memory_entry|" + string(objectgraph.WriterResearch) + "|wrong_reader"
	moved := readerDelta(key)
	call := func(entryID string) {
		t.Helper()
		c := toolregistry.WithExecutionContext(withDeskReader(ctx, agentprofile.Research), toolregistry.ExecutionContext{OwnerID: "owner-x18", RunID: "run-x18-self"})
		if _, err := tool.Func(c, json.RawMessage(`{"entry_id":"`+entryID+`"}`)); err != nil {
			t.Fatal(err)
		}
	}
	call("x18-own")
	if n := moved(); n != 0 {
		t.Fatalf("the run's own entry was counted: %d", n)
	}
	call("x18-other")
	if moved() == 0 {
		t.Fatalf("counter %q did not move (counts: %v)", key, objectgraph.ReaderGateCounts())
	}
}

// ReadCapture (MA3.3): the frame handler decides from the capture record. A
// world capture is served under captures.read to every desk; a capture whose
// record carries an owner-private label is counted for a desk that is not the
// custodian of that kind, and a label comes from the record, never the cell.
func TestReadCaptureResolvesLabelsFromTheCaptureRecord(t *testing.T) {
	world := json.RawMessage(`{"capture_id":"c1","labels":["world_untrusted"]}`)
	private := json.RawMessage(`{"capture_id":"c2","labels":["owner_private:texture_document","world_untrusted"]}`)
	unlabelled := json.RawMessage(`{"capture_id":"c3"}`)

	if got := captureRecordLabels(private); !got.OwnerPrivate() || !got.Untrusted() {
		t.Fatalf("record labels = %v", got)
	}
	if got := captureRecordLabels(unlabelled); len(got) != 0 {
		t.Fatalf("an unlabelled record carries labels: %v", got)
	}
	// The run taint is the floor every capture read carries plus the record's.
	labels, reads := egressReadLabels("read_capture", nil, private)
	if !reads || !labels.Has("owner_private:texture_document") || !labels.Has(types.OwnerPrivateLabel("capture")) {
		t.Fatalf("read_capture labels = %v", labels)
	}

	crossing := func(desk string, result json.RawMessage) uint64 {
		w, _ := objectgraph.DeskWriter(desk)
		key := "choir.web_capture|" + string(w) + "|wrong_reader"
		moved := readerDelta(key)
		gateCaptureRead(context.Background(), desk, result)
		return moved()
	}
	for _, desk := range []string{agentprofile.Texture, agentprofile.Research, agentprofile.Management} {
		if n := crossing(desk, world); n != 0 {
			t.Errorf("%s: a world capture was counted: %d", desk, n)
		}
		if n := crossing(desk, unlabelled); n != 0 {
			t.Errorf("%s: an unlabelled capture was counted: %d", desk, n)
		}
	}
	if n := crossing(agentprofile.Texture, private); n != 0 {
		t.Errorf("the custodian of the label's kind was counted: %d", n)
	}
	for _, desk := range []string{agentprofile.Research, agentprofile.Management} {
		if n := crossing(desk, private); n == 0 {
			t.Errorf("%s: an owner-private capture was not counted", desk)
		}
	}
	// The owner and the kernel are admitted.
	if !captureServedTo(objectgraph.WriterOwner, true, captureRecordLabels(private)) || !captureServedTo(objectgraph.WriterKernel, true, captureRecordLabels(private)) {
		t.Error("the owner or the kernel was refused an owner-private capture")
	}
	// A label naming a kind no desk is custodian of serves no desk.
	if captureServedTo(objectgraph.WriterTexture, true, types.NewLabelSet("owner_private:nonesuch")) {
		t.Error("a label with no custodian was served to a desk")
	}
}
