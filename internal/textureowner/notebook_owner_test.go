package textureowner

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/notebook"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure list: notebook_owner.go (N1-N7).

func nbAddOp(id, text string) notebook.Op {
	return notebook.Op{Op: "add", Section: "learned", ID: id, Text: text}
}

func TestNotebookOwner(t *testing.T) {
	f := newAlgoFixture(t, "x")
	ctx := context.Background()
	h := f.handler
	const owner, desk = "user-1", "desk:autoradio"
	modelCalls := f.model.Load()

	// N1: only the Autoradio desk holds the grants.
	if _, err := h.NotebookRead(ctx, owner, "desk:research"); err == nil {
		t.Error("(N1) research read the notebook")
	}
	if _, err := h.NotebookApply(ctx, owner, "desk:management", notebook.Patch{PatchID: "p0"}); err == nil {
		t.Error("(N1) management patched the notebook")
	}
	if st, err := h.NotebookRead(ctx, owner, desk); err != nil || st.Found {
		t.Fatalf("no notebook yet: %+v %v", st, err)
	}

	// N7: the first write creates it, titled My notebook, written by the desk.
	res, err := h.NotebookApply(ctx, owner, desk, notebook.Patch{PatchID: "p1", Ops: []notebook.Op{nbAddOp("L1", "Short pieces are heard.")}})
	if err != nil || res.Status != notebook.StatusApplied || res.DocID == "" {
		t.Fatalf("(N7) create: %+v %v", res, err)
	}
	st, err := h.NotebookRead(ctx, owner, desk)
	if err != nil || !st.Found || !st.Head.IsDesk() || !strings.Contains(st.Head.Text, "- Short pieces are heard. [L1 · heard 0 of 0]") || !strings.Contains(st.Head.Text, "## What you asked for") {
		t.Fatalf("(N7) read: %+v %v", st, err)
	}
	if _, got, _ := getEdition(f, owner); got["doc_id"] != res.DocID {
		t.Errorf("the edition route does not find the notebook: %v", got)
	}

	// A second patch on the head; a replay; a stale base; a refused one.
	res2, err := h.NotebookApply(ctx, owner, desk, notebook.Patch{PatchID: "p2", BaseRevisionID: res.RevisionID, Ops: []notebook.Op{nbAddOp("L2", "Second.")}, Counts: notebook.Counts{"L1": {3, 4}}})
	if err != nil || res2.Status != notebook.StatusApplied || res2.RevisionID == res.RevisionID {
		t.Fatalf("second: %+v %v", res2, err)
	}
	if rep, _ := h.NotebookApply(ctx, owner, desk, notebook.Patch{PatchID: "p2", BaseRevisionID: res.RevisionID, Ops: []notebook.Op{nbAddOp("L2", "Second.")}}); rep.Status != notebook.StatusApplied || rep.RevisionID != res2.RevisionID { // N4
		t.Errorf("(N4) replay: %+v", rep)
	}
	if st, _ := h.NotebookRead(ctx, owner, desk); !strings.Contains(st.Head.Text, "[L1 · heard 3 of 4]") || len(st.Chain) != 1 {
		t.Errorf("counts or chain: %q %d", st.Head.Text, len(st.Chain))
	}
	if stale, _ := h.NotebookApply(ctx, owner, desk, notebook.Patch{PatchID: "p3", BaseRevisionID: res.RevisionID, Ops: []notebook.Op{nbAddOp("L3", "Stale.")}}); stale.Status != notebook.StatusStale { // N3
		t.Errorf("(N3) stale: %+v", stale)
	}
	var big []notebook.Op
	for i := 0; i < 10; i++ {
		big = append(big, notebook.Op{Op: "tap", Ref: "tap:e", Text: strings.Repeat("word ", 90)})
	}
	if ref, _ := h.NotebookApply(ctx, owner, desk, notebook.Patch{PatchID: "p4", BaseRevisionID: res2.RevisionID, Ops: big}); ref.Status != notebook.StatusRefused { // N5
		t.Errorf("(N5) over the page: %+v", ref)
	}

	// The listener's edit is an owner revision; the chain shows it after the desk's.
	head, err := h.Store.GetLifecycleRevision(ctx, owner, "autoputer-test", res2.RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	if head.AuthorLabel != notebook.DeskLabel || head.AuthorKind != "appagent" {
		t.Errorf("author %s %s", head.AuthorKind, head.AuthorLabel)
	}

	// N6: no revision of the notebook wakes the desk's model.
	if rec, err := h.ReconcileAgentWake(ctx, owner, res.DocID); rec != nil || err != nil {
		t.Errorf("(N6) wake: %v %v", rec, err)
	}
	if f.model.Load() != modelCalls {
		t.Errorf("(N6) the model was called")
	}
}

// N2: a document titled My edition is adopted: retitled, not duplicated, and
// the listener's revision stays in the chain under the desk's.
func TestNotebookAdoptsTheEdition(t *testing.T) {
	f := newAlgoFixture(t, "x")
	ctx := context.Background()
	_ = startTextDoc(t, f.core.Store(), "doc-edition", "My edition", "# What I follow\n\n- Ukraine\n")
	st, err := f.handler.NotebookRead(ctx, "user-1", "desk:autoradio")
	if err != nil || !st.Found || st.DocID != "doc-edition" || st.Head.IsDesk() {
		t.Fatalf("read: %+v %v", st, err)
	}
	res, err := f.handler.NotebookApply(ctx, "user-1", "desk:autoradio", notebook.Patch{PatchID: "a1", BaseRevisionID: st.Head.RevisionID, Ops: []notebook.Op{nbAddOp("L1", "X.")}})
	if err != nil || res.Status != notebook.StatusApplied || res.DocID != "doc-edition" {
		t.Fatalf("(N2) %+v %v", res, err)
	}
	doc, err := f.core.Store().GetLifecycleDocument(ctx, "user-1", "autoputer-test", "doc-edition")
	if err != nil || doc.Title != "My notebook" {
		t.Errorf("(N2) title %q %v", doc.Title, err)
	}
	if a := notebook.Parse(st.Head.Text).AllLines(); len(a) < 1 || a[0].Section != notebook.SecAsked || a[0].Line.Text != "Ukraine" {
		t.Errorf("(N2) the edition's own line is not the listener's: %+v", a)
	}
	st, _ = f.handler.NotebookRead(ctx, "user-1", "desk:autoradio")
	if len(st.Chain) != 1 || !st.Chain[0].IsDesk() || !strings.Contains(st.Head.Text, "Ukraine") {
		t.Errorf("(N2) chain %+v\n%s", st.Chain, st.Head.Text)
	}
	if _, got, _ := getEdition(f, "user-1"); got["doc_id"] != "doc-edition" {
		t.Errorf("(N2) the edition route: %v", got)
	}
}

// N4: a document a model agent wrote is not adopted, and an archived notebook
// is reported and never written to.
func TestNotebookDoesNotAdoptAModelsDocumentOrWriteToAnArchivedOne(t *testing.T) {
	f := newAlgoFixture(t, "x")
	ctx := context.Background()
	body, _, projected, err := markdownLineageStructuredRevision("doc-model", "rev-doc-model-0", "# My notebook\n\n- hello\n", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	startObservationLifecycle(t, f.core.Store(), func(r *types.StartLifecycleRequest) {
		r.CommandID, r.TrajectoryID = "start-doc-model", "trajectory-doc-model"
		r.SubjectRefs = map[string]string{"artifact": "texture://document/doc-model", "doc_id": "doc-model"}
		r.InitialWork.WorkItemID = "work-doc-model"
		r.InitialDocument = types.Document{DocID: "doc-model", Title: "My notebook"}
		r.InitialRevision = types.Revision{RevisionID: "rev-doc-model-0", AuthorKind: types.AuthorAppAgent, AuthorLabel: "texture", Content: projected, BodyDoc: body}
		r.Agent = types.AgentRecord{AgentID: "texture:doc-model", Profile: "texture", Role: "texture", ChannelID: "doc-model"}
	})
	st, err := f.handler.NotebookRead(ctx, "user-1", "desk:autoradio")
	if err != nil || st.Found {
		t.Fatalf("(N4) a model's document was adopted: %+v %v", st, err)
	}
	// The listener's own, archived: reported, nothing written.
	_ = startTextDoc(t, f.core.Store(), "doc-mine", "My notebook", "# My notebook\n")
	settleAndArchive(t, f, "doc-mine")
	st, err = f.handler.NotebookRead(ctx, "user-1", "desk:autoradio")
	if err != nil || st.Found || !st.Archived {
		t.Fatalf("(N4) archived: %+v %v", st, err)
	}
	res, err := f.handler.NotebookApply(ctx, "user-1", "desk:autoradio", notebook.Patch{PatchID: "z1", Ops: []notebook.Op{nbAddOp("L1", "X.")}})
	if err != nil || res.Status != notebook.StatusRefused || !strings.Contains(res.Reason, "archived") {
		t.Errorf("(N4) write to an archived notebook: %+v %v", res, err)
	}
	docs, _ := f.core.Store().ListDocumentsByScope(ctx, "user-1", "autoputer-test", 100)
	for _, d := range docs {
		if d.DocID != "doc-model" && d.DocID != "doc-mine" && strings.Contains(d.Title, "notebook") {
			t.Errorf("(N4) a second notebook was made: %s", d.DocID)
		}
	}
}
