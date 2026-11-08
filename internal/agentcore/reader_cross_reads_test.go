package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The cross reads of the custody section (plan section 8.4) that live in
// agentcore, pinned as counted crossings (MA3.3). A built-in kind only
// observes, so each read works; the counters read it as the desk that crossed.
// MA3.7 resolves them, and each test is its refusal target.

func readerDelta(key string) func() uint64 {
	before := objectgraph.ReaderGateCounts()[key]
	return func() uint64 { return objectgraph.ReaderGateCounts()[key] - before }
}

// X7 (MA3.7a): management's ProductAPI reaches the Texture list route and
// nothing else under /api/texture/. The list answers a desk from the story
// index Texture publishes, so the route's reads are no crossing into
// Texture's documents; a document, revision or history route is refused (a
// story's content comes through a Cast to Texture).
func TestProductAPITextureListByManagementCrossesNothing(t *testing.T) {
	rt, st := testRuntime(t)
	owner, computer := "owner-x7", rt.cfg.ComputerID
	openTestStory(t, st, owner, computer, "Harbor reopens")
	canonical := server.NewServer("x7-reader-test", "0")
	canonical.HandleFunc("/api/texture/documents", func(w http.ResponseWriter, r *http.Request) {
		// A handler that read Texture's documents on the request context
		// would be the crossing; this one lists from the index.
		if _, err := st.ListStoryIndex(r.Context(), owner, computer, time.Time{}, "", 0); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusOK)
	})
	rt.SetProductAPIHandler(canonical, "")
	objectgraph.ResetReaderGateCounts()
	ctx := withDeskReader(context.Background(), agentprofile.Management)
	if _, err := rt.productAPIRequest(ctx, owner, "", "GET", "/api/texture/documents", nil); err != nil {
		t.Fatal(err)
	}
	for key, n := range objectgraph.ReaderGateCounts() {
		if n > 0 && strings.Contains(key, "|"+string(objectgraph.WriterManagement)+"|") {
			t.Errorf("management's list counted %q = %d", key, n)
		}
	}
	for _, path := range []string{
		"/api/texture/documents/doc-1", "/api/texture/documents/doc-1/revisions", "/api/texture/documents/doc-1/history",
		"/api/texture/revisions/rev-1", "/api/texture/documents/doc-1/export",
	} {
		if _, err := rt.productAPIRequest(ctx, owner, "", "GET", path, nil); err == nil || !strings.Contains(err.Error(), "Cast to the Texture desk") {
			t.Errorf("GET %s: err = %v; want refused toward a Cast to Texture", path, err)
		}
	}
}

// X8 (MA3.7a): management's choir.Stories reads the story index, and the
// reads count nothing against Texture's documents, revisions or library head.
func TestStoriesByManagementReadsTheIndexNotTextureKinds(t *testing.T) {
	ctx := withDeskReader(context.Background(), agentprofile.Management)
	rt, st := testRuntime(t)
	owner, computer := "owner-x8", rt.cfg.ComputerID
	storeIntakeLabelsOn(t, st, owner, computer)
	openTestStory(t, st, owner, computer, "Harbor reopens after the storm")
	objectgraph.ResetReaderGateCounts()
	exec := toolregistry.ExecutionContext{OwnerID: owner, ComputerID: computer, RunID: "run-x8", Role: agentprofile.Management, Profile: agentprofile.Management}
	raw, err := rt.storiesEgress(ctx, exec.Role, exec, nil)
	if err != nil {
		t.Fatal(err)
	}
	var page yaegikernel.StoriesPage
	if err := json.Unmarshal(raw, &page); err != nil || len(page.Stories) != 1 || page.Stories[0].Headline != "Harbor reopens after the storm" {
		t.Fatalf("stories = %s, %v", raw, err)
	}
	requireNoManagementTextureRead(t)
}

func requireNoManagementTextureRead(t *testing.T) {
	t.Helper()
	for _, kind := range []string{"choir.texture_document", "choir.texture_revision", "choir.story_library_head"} {
		key := kind + "|" + string(objectgraph.WriterManagement) + "|wrong_reader"
		if n := objectgraph.ReaderGateCounts()[key]; n != 0 {
			t.Errorf("counter %q = %d; management must read the story index, not %s", key, n, kind)
		}
	}
}

// X9 (MA3.7a): the intake labeler's candidates come from the story index.
func TestIntakeLabelerReadsTheStoryIndexNotTextureKinds(t *testing.T) {
	ctx := context.Background()
	rt, st := testRuntime(t)
	owner, computer, sourceID := "owner-x9", rt.cfg.ComputerID, "rss:x9"
	storeIntakeLabelsOn(t, st, owner, computer)
	pope := openTestStory(t, st, owner, computer, "Pope has lung surgery")
	captured := []sourcegraph.CapturedItem{{CaptureID: "choir.web_capture:x9-1", Title: "Papst operiert", URL: "https://x9.example/1", Language: "de", Summary: "Vatikan", Outlet: "DW"}}
	rt.SetCandidateJudge(&fakeIntakeJudge{answers: map[string]string{"Papst operiert": "Pope has lung surgery"}})
	objectgraph.ResetReaderGateCounts()
	if labelled, _ := rt.labelCaptures(ctx, owner, computer, sourceID, captured); labelled != 1 {
		t.Fatalf("labelled %d", labelled)
	}
	labels, err := st.CaptureLabels(ctx, owner, computer, "choir.web_capture:x9-1")
	if err != nil || len(labels) != 1 || labels[0].EventDocID != pope {
		t.Fatalf("labels = %+v, %v; want the story from the index", labels, err)
	}
	requireNoManagementTextureRead(t)
}

// X18 (MA3.7b): research's get_run_memory_entry is self-scoped: a run reads
// its own run's entries, and an entry of another run reads as absent. The
// entry kind is a commons kind, so nothing is counted either way.
func TestRunMemoryEntryIsSelfScopedToTheRun(t *testing.T) {
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
	objectgraph.ResetReaderGateCounts()
	call := func(entryID string) (string, error) {
		c := toolregistry.WithExecutionContext(withDeskReader(ctx, agentprofile.Research), toolregistry.ExecutionContext{OwnerID: "owner-x18", RunID: "run-x18-self"})
		return tool.Func(c, json.RawMessage(`{"entry_id":"`+entryID+`"}`))
	}
	if out, err := call("x18-own"); err != nil || !strings.Contains(out, "x18-own") {
		t.Fatalf("the run's own entry: %q, %v", out, err)
	}
	if out, err := call("x18-other"); err == nil || strings.Contains(out, `\"b\"`) {
		t.Fatalf("another run's entry was served: %q, %v", out, err)
	}
	if counts := objectgraph.ReaderGateCounts(); len(counts) != 0 {
		t.Fatalf("reader counts = %v; want none", counts)
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
	// Capture authority (enforce flip section 7.2) reads by rule, uncounted;
	// a desk without it is still counted until the flip.
	for _, desk := range []string{agentprofile.Research, agentprofile.Management} {
		if n := crossing(desk, private); n != 0 {
			t.Errorf("%s: a desk with capture authority was counted: %d", desk, n)
		}
	}
	if n := crossing(agentprofile.Engineering, private); n == 0 {
		t.Error("engineering: an owner-private capture was not counted")
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

func storeIntakeLabelsOn(t *testing.T, s *store.Store, owner, computer string) {
	t.Helper()
	if err := s.PutComputerSetting(context.Background(), store.ComputerSetting{OwnerID: owner, ComputerID: computer, Key: store.SettingIntakeLabels, Value: true}); err != nil {
		t.Fatal(err)
	}
}
