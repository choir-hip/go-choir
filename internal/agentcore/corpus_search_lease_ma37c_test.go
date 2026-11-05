package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// MA3.7c (X1): the Texture part of research's corpus search is the Texture
// custodian's guarded search under a texture.search lease the research run
// holds. Without the lease the Texture part is omitted and nothing is counted;
// with it the cell gets ranked refs and bounded, labelled snippets and no body.
func TestCorpusSearchTextureHalfNeedsTheLease(t *testing.T) {
	f := newDelegationFixture(t, "owner-ma37c-search")
	ctx := context.Background()
	rt, s := f.rt, f.s
	owner, doc := "owner-ma37c-texture", "doc-ma37c"
	now := time.Now().UTC()
	body := "# Harbor\n\nThe harbor reopened. " + strings.Repeat("harbor traffic resumed under the new schedule. ", 60) + "SECRET-TAIL-MARKER"
	if err := s.CreateDocument(ctx, types.Document{DocID: doc, OwnerID: owner, Title: "Harbor reopens", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRevision(ctx, types.Revision{RevisionID: "rev-ma37c-1", DocID: doc, OwnerID: owner, AuthorKind: types.AuthorUser,
		Content: body, Citations: json.RawMessage("[]"), CreatedAt: now}); err != nil {
		t.Fatal(err)
	}

	research := "research:" + doc
	newRun := func(id string) (types.RunRecord, toolregistry.ExecutionContext) {
		rec := types.RunRecord{RunID: id, AgentID: research, OwnerID: owner, ComputerID: f.computer,
			ChannelID: doc, AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research, State: types.RunRunning}
		if err := s.CreateRun(ctx, rec); err != nil {
			t.Fatal(err)
		}
		return rec, toolregistry.ExecutionContext{RunID: rec.RunID, AgentID: rec.AgentID, OwnerID: owner, ChannelID: doc, ComputerID: f.computer,
			Profile: agentprofile.Research, Role: agentprofile.Research, RunRecord: &rec}
	}
	search := func(exec toolregistry.ExecutionContext, query string) map[string]any {
		t.Helper()
		tool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Research)
		payload, _ := json.Marshal(map[string]any{"source": `r, err := choir.SearchWireCorpus("` + query + `", 25)
fmt.Println(r, err)`, "timeout_ms": 15000})
		cellCtx := withDeskReader(withDeskWriter(ctx, exec.Profile), exec.Profile)
		cellCtx = objectgraph.WithLeases(cellCtx, rt.leaseAuthority(), exec.RunID)
		out, err := tool.Func(toolregistry.WithExecutionContext(cellCtx, exec), payload)
		if err != nil {
			t.Fatalf("cell: %v\n%s", err, out)
		}
		if strings.Contains(out, "SECRET-TAIL-MARKER") {
			t.Fatalf("a document body reached the cell: %s", out)
		}
		var cell struct {
			Stdout string `json:"stdout"`
		}
		if err := json.Unmarshal([]byte(out), &cell); err != nil {
			t.Fatalf("decode cell output %q: %v", out, err)
		}
		i := strings.Index(cell.Stdout, "{\"capture_count\"")
		j := strings.LastIndex(cell.Stdout, "}")
		if i < 0 || j < i {
			t.Fatalf("no result object in cell output: %s", out)
		}
		var got map[string]any
		if err := json.Unmarshal([]byte(cell.Stdout[i:j+1]), &got); err != nil {
			t.Fatalf("decode %q: %v", out, err)
		}
		return got
	}
	crossings := func() []string {
		var out []string
		for key, n := range objectgraph.ReaderGateCounts() {
			if strings.Contains(key, "|undeclared|") {
				continue // the fixture's own seeding
			}
			out = append(out, key+" x"+countStr(n))
		}
		return out
	}
	texturEvents := func(runID string) int {
		evs, err := s.ListEvents(ctx, runID, 100)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, e := range evs {
			if e.Kind == types.EventTextureSearchQueried {
				n++
			}
		}
		return n
	}

	// Without a lease: no Texture results, nothing counted.
	_, noLease := newRun("run-ma37c-nolease")
	objectgraph.ResetReaderGateCounts()
	got := search(noLease, "traffic schedule")
	if c, _ := got["count"].(float64); c != 0 {
		t.Errorf("a run without the lease got Texture results: %v", got["results"])
	}
	if note, _ := got["results_note"].(string); !strings.Contains(note, "texture.search") {
		t.Errorf("the omitted Texture part is not explained: %v", got)
	}
	if c := crossings(); len(c) != 0 {
		t.Errorf("a search without the lease counted: %v", c)
	}
	if n := texturEvents("run-ma37c-nolease"); n != 0 {
		t.Errorf("a refused search was recorded as a query: %d", n)
	}

	// With the lease: refs and bounded labelled snippets, nothing counted.
	rec, withLease := newRun("run-ma37c-lease")
	granted, err := rt.leases.IssueTextureSearch(ctx, owner, f.computer, rec.RunID, rec.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	objectgraph.ResetReaderGateCounts()
	got = search(withLease, "traffic schedule")
	results, _ := got["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("expected the Harbor story under the lease: %v", got)
	}
	hit := results[0].(map[string]any)
	if hit["doc_id"] != doc || hit["label"] != "owner_private:texture" {
		t.Errorf("hit is not a labelled ref: %v", hit)
	}
	snippet, _ := hit["snippet"].(string)
	if snippet == "" || len([]rune(snippet)) > store.TextureSearchSnippetRunes+1 {
		t.Errorf("snippet is empty or unbounded (%d runes)", len([]rune(snippet)))
	}
	if _, has := hit["content"]; has {
		t.Errorf("a body field is present: %v", hit)
	}
	if c := crossings(); len(c) != 0 {
		t.Errorf("a search under the lease counted: %v", c)
	}
	if n := texturEvents(rec.RunID); n != 1 {
		t.Errorf("the query was not recorded once as an event: %d", n)
	}
	live := rt.leases.Live(owner)
	if len(live) != 1 || live[0].LeaseID != granted.LeaseID || live[0].Uses == 0 {
		t.Errorf("the lease use was not counted: %+v", live)
	}

	// The lease ends with its run: a later search by the same run omits Texture.
	rt.endRunLeases(ctx, rec.RunID, "run ended")
	got = search(withLease, "traffic schedule")
	if c, _ := got["count"].(float64); c != 0 {
		t.Errorf("an ended lease still served Texture: %v", got["results"])
	}
}

func seedStoryForSearch(t *testing.T, s *store.Store, owner, doc, title, body string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	if err := s.CreateDocument(ctx, types.Document{DocID: doc, OwnerID: owner, Title: title, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRevision(ctx, types.Revision{RevisionID: "rev-" + doc, DocID: doc, OwnerID: owner, AuthorKind: types.AuthorUser,
		Content: body, Citations: json.RawMessage("[]"), CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
}

// Devin r1 on MA3.7c: a research child shares its parent's run id, so it must
// not read the parent's texture.search lease. The child's context holds no
// lease; the same search on the parent's context is served.
func TestResearchChildDoesNotHoldTheParentsTextureSearchLease(t *testing.T) {
	f := newDelegationFixture(t, "owner-ma37c-child")
	ctx := context.Background()
	rt, s := f.rt, f.s
	owner := "owner-ma37c-child-texture"
	seedStoryForSearch(t, s, owner, "doc-child", "Quay", "dockside lanterns brightened every evening")
	rec := types.RunRecord{RunID: "run-child-parent", AgentID: "research:doc-child", OwnerID: owner, ComputerID: f.computer,
		AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research, State: types.RunRunning}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.leases.IssueTextureSearch(ctx, owner, f.computer, rec.RunID, rec.AgentID); err != nil {
		t.Fatal(err)
	}
	parent := withDeskReader(objectgraph.WithLeases(ctx, rt.leaseAuthority(), rec.RunID), agentprofile.Research)
	if hits, err := s.TextureSearchUnderLease(parent, owner, "lanterns", 5); err != nil || len(hits) != 1 {
		t.Fatalf("the parent's search: %v %v", hits, err)
	}
	child := researchChildReadContext(parent, agentprofile.Research)
	if hits, err := s.TextureSearchUnderLease(child, owner, "lanterns", 5); !errors.Is(err, store.ErrTextureSearchNoLease) || len(hits) != 0 {
		t.Fatalf("a research child searched Texture on its parent's lease: %v %v", hits, err)
	}
}

// A SearchWireCorpus result that carries labelled Texture hits folds the
// label into the run's labels; a result with none carries the world label only.
func TestCorpusSearchHitsTaintTheRun(t *testing.T) {
	with := json.RawMessage(`{"results":[{"doc_id":"d","label":"owner_private:texture"}]}`)
	got, ok := egressReadLabels(yaegikernel.ActionSearchWireCorpus, nil, with)
	if !ok || !got.Has("owner_private:texture") || !got.Has(types.LabelWorldUntrusted) {
		t.Errorf("labelled hits did not taint the run: %v", got)
	}
	got, _ = egressReadLabels(yaegikernel.ActionSearchWireCorpus, nil, json.RawMessage(`{"results":[]}`))
	if got.Has("owner_private:texture") {
		t.Errorf("an empty result tainted the run: %v", got)
	}
}

// The bounds: a request for more than ten hits is clamped to ten, and the 41st
// served query of one lease is refused. A refused (or unserved) query does not
// spend the budget.
func TestTextureSearchClampsHitsAndBoundsQueriesPerLease(t *testing.T) {
	f := newDelegationFixture(t, "owner-ma37c-bounds")
	ctx := context.Background()
	rt, s := f.rt, f.s
	owner := "owner-ma37c-bounds-texture"
	for i := 0; i < 12; i++ {
		seedStoryForSearch(t, s, owner, fmt.Sprintf("doc-b%02d", i), fmt.Sprintf("Story %d", i), "beacon signal lamps")
	}
	rec := types.RunRecord{RunID: "run-bounds", AgentID: "research:doc-b", OwnerID: owner, ComputerID: f.computer,
		AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research, State: types.RunRunning}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatal(err)
	}
	granted, err := rt.leases.IssueTextureSearch(ctx, owner, f.computer, rec.RunID, rec.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	rctx := withDeskReader(objectgraph.WithLeases(ctx, rt.leaseAuthority(), rec.RunID), agentprofile.Research)
	hits, err := s.TextureSearchUnderLease(rctx, owner, "beacon", 11)
	if err != nil || len(hits) != store.TextureSearchMaxResults {
		t.Fatalf("11 requested: got %d hits, %v (want %d)", len(hits), err, store.TextureSearchMaxResults)
	}
	for i := 1; i < store.TextureSearchMaxQueriesPerLease; i++ {
		if _, err := s.TextureSearchUnderLease(rctx, owner, "beacon", 1); err != nil {
			t.Fatalf("query %d refused: %v", i+1, err)
		}
	}
	if _, err := s.TextureSearchUnderLease(rctx, owner, "beacon", 1); !errors.Is(err, store.ErrTextureSearchRate) {
		t.Fatalf("the 41st query was not refused: %v", err)
	}
	// The count is dropped with the lease: a new lease starts at zero.
	rt.endRunLeases(ctx, rec.RunID, "run ended")
	if granted.LeaseID == "" {
		t.Fatal("no lease id")
	}
	again, err := rt.leases.IssueTextureSearch(ctx, owner, f.computer, rec.RunID, rec.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.TextureSearchUnderLease(rctx, owner, "beacon", 1); err != nil {
		t.Fatalf("a fresh lease %s is refused: %v", again.LeaseID, err)
	}
}

// leasedResearchCtx is a research run's context for a corpus search test that
// is about something else than the lease: the run reads as the research desk
// and holds the texture.search grant (a stub authority; the lease service is
// tested in internal/lease and above).
func leasedResearchCtx(ctx context.Context) context.Context {
	return objectgraph.WithLeases(objectgraph.WithReader(ctx, objectgraph.WriterResearch), grantEverything{}, "research-run")
}

type grantEverything struct{}

func (grantEverything) AdmitsKind(objectgraph.Writer, string, objectgraph.ObjectKind) bool {
	return false
}
func (grantEverything) AdmitsCapture(objectgraph.Writer, string, string, []string) bool { return false }
func (grantEverything) AdmitsGrant(objectgraph.Writer, string, string) (string, bool) {
	return "lease-stub", true
}
func (grantEverything) AdmitsGrantRef(objectgraph.Writer, string, string, string) (string, bool) {
	return "lease-stub", true
}
