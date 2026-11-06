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

// MA3.7c (X1), moved by MA4.6 (plan 8.6): the owner's stories are searched
// only through the Texture custodian's guarded search, choir.SearchTexture,
// under a texture.search lease the research desk's Texture child is issued.
// Failure modes pinned: the world corpus search (SearchWireCorpus) still
// answers the owner's stories; the research desk's own run is served
// SearchTexture; the Texture child's search returns a body, an unlabelled hit
// or an unrecorded query; the lease outlives the child.
func TestTextureSearchIsTheTextureChildsUnderItsLease(t *testing.T) {
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
	rec := types.RunRecord{RunID: "run-ma46-search", AgentID: research, OwnerID: owner, ComputerID: f.computer,
		ChannelID: doc, AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research, State: types.RunRunning}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatal(err)
	}
	exec := toolregistry.ExecutionContext{RunID: rec.RunID, AgentID: rec.AgentID, OwnerID: owner, ChannelID: doc, ComputerID: f.computer,
		Profile: agentprofile.Research, Role: agentprofile.Research, RunRecord: &rec}
	cell := func(cellCtx context.Context, role, src string) string {
		t.Helper()
		tool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), role)
		payload, _ := json.Marshal(map[string]any{"source": src, "timeout_ms": 15000})
		out, err := tool.Func(toolregistry.WithExecutionContext(cellCtx, exec), payload)
		if err != nil {
			t.Fatalf("cell: %v\n%s", err, out)
		}
		if strings.Contains(out, "SECRET-TAIL-MARKER") {
			t.Fatalf("a document body reached the cell: %s", out)
		}
		var c struct {
			Stdout string `json:"stdout"`
		}
		if err := json.Unmarshal([]byte(out), &c); err != nil {
			t.Fatalf("decode cell output %q: %v", out, err)
		}
		return c.Stdout
	}
	object := func(stdout, prefix string) map[string]any {
		t.Helper()
		i := strings.Index(stdout, prefix)
		j := strings.LastIndex(stdout, "}")
		if i < 0 || j < i {
			t.Fatalf("no result object in cell output: %s", stdout)
		}
		var got map[string]any
		if err := json.Unmarshal([]byte(stdout[i:j+1]), &got); err != nil {
			t.Fatalf("decode %q: %v", stdout, err)
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
	textureEvents := func() int {
		evs, err := s.ListEvents(ctx, rec.RunID, 100)
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
	parentCtx := objectgraph.WithLeases(withDeskReader(ctx, agentprofile.Research), rt.leaseAuthority(), rec.RunID)

	// The research desk's own run: its world corpus search carries no
	// stories, and its SearchTexture is refused by the custodian (no lease).
	objectgraph.ResetReaderGateCounts()
	got := object(cell(parentCtx, agentprofile.Research, `r, err := choir.SearchWireCorpus("traffic schedule", 25)
fmt.Println(r, err)`), "{\"capture_count\"")
	for _, key := range []string{"results", "results_note", "count"} {
		if _, has := got[key]; has {
			t.Errorf("the world corpus search answered the owner's stories (%s): %v", key, got)
		}
	}
	out := cell(parentCtx, agentprofile.Research, `r, err := choir.SearchTexture("traffic schedule", 5)
fmt.Println(string(r), err)`)
	if !strings.Contains(out, "texture.search needs a lease") || strings.Contains(out, doc) {
		t.Errorf("the research desk's own run searched Texture: %s", out)
	}
	if c := crossings(); len(c) != 0 {
		t.Errorf("a refused search counted: %v", c)
	}
	if n := textureEvents(); n != 0 {
		t.Errorf("a refused search was recorded as a query: %d", n)
	}

	// The Texture child, under the lease the custodian issues it: refs and
	// bounded labelled snippets, one recorded query, nothing counted.
	childCtx, end, err := rt.researchTextureChildContext(researchChildReadContext(ctx, agentprofile.Research), exec, researchLineage{ownerID: owner}, "k1")
	if err != nil {
		t.Fatal(err)
	}
	objectgraph.ResetReaderGateCounts()
	got = object(cell(childCtx, yaegikernel.SessionRoleResearchTexture, `r, err := choir.SearchTexture("traffic schedule", 25)
fmt.Println(string(r), err)`), "{\"count\"")
	results, _ := got["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("expected the Harbor story under the lease: %v", got)
	}
	hit := results[0].(map[string]any)
	if hit["ref"] != doc || hit["label"] != "owner_private:texture" {
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
	if n := textureEvents(); n != 1 {
		t.Errorf("the query was not recorded once as an event: %d", n)
	}
	live := rt.leases.Live(owner)
	if len(live) != 1 || live[0].Holder != yaegikernel.SessionRoleResearchTexture || live[0].Uses == 0 {
		t.Errorf("the lease is not the Texture child's, or its use was not counted: %+v", live)
	}
	if !rt.runLabelSet(rec.RunID).Has("owner_private:texture") {
		t.Errorf("the search did not label the run it works for: %v", rt.runLabelSet(rec.RunID))
	}

	// The lease ends with the child: a later search is refused.
	end()
	if live := rt.leases.Live(owner); len(live) != 0 {
		t.Errorf("the Texture child's lease outlived it: %+v", live)
	}
	out = cell(childCtx, yaegikernel.SessionRoleResearchTexture, `r, err := choir.SearchTexture("traffic schedule", 5)
fmt.Println(string(r), err)`)
	if !strings.Contains(out, "texture.search needs a lease") {
		t.Errorf("an ended lease still served Texture: %s", out)
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

// Devin r1 on MA3.7c, MA4.6: a research child shares its parent's run id, so
// the web child must not read the Texture child's texture.search lease, nor the
// parent its own; only the Texture child's context is served.
func TestOnlyTheTextureChildHoldsTheTextureSearchLease(t *testing.T) {
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
	parent := withDeskReader(objectgraph.WithLeases(ctx, rt.leaseAuthority(), rec.RunID), agentprofile.Research)
	exec := toolregistry.ExecutionContext{RunID: rec.RunID, OwnerID: owner, ComputerID: f.computer, AgentID: rec.AgentID}
	textureChild, end, err := rt.researchTextureChildContext(researchChildReadContext(parent, agentprofile.Research), exec, researchLineage{ownerID: owner}, "k1")
	if err != nil {
		t.Fatal(err)
	}
	defer end()
	if hits, err := s.TextureSearchUnderLease(textureChild, owner, "lanterns", 5); err != nil || len(hits) != 1 {
		t.Fatalf("the Texture child's search: %v %v", hits, err)
	}
	if hits, err := s.TextureSearchUnderLease(parent, owner, "lanterns", 5); !errors.Is(err, store.ErrTextureSearchNoLease) || len(hits) != 0 {
		t.Fatalf("the research parent searched Texture on its child's lease: %v %v", hits, err)
	}
	web := researchChildReadContext(parent, agentprofile.Research)
	if hits, err := s.TextureSearchUnderLease(web, owner, "lanterns", 5); !errors.Is(err, store.ErrTextureSearchNoLease) || len(hits) != 0 {
		t.Fatalf("a web child searched Texture on its sibling's lease: %v %v", hits, err)
	}
	webLeased := objectgraph.WithLeases(withDeskReader(ctx, yaegikernel.SessionRoleResearchWeb), rt.leaseAuthority(), rec.RunID)
	if hits, err := s.TextureSearchUnderLease(webLeased, owner, "lanterns", 5); !errors.Is(err, store.ErrTextureSearchNoLease) || len(hits) != 0 {
		t.Fatalf("a reader declared as the web child searched Texture: %v %v", hits, err)
	}
}

// A SearchTexture result labels the run with each hit's owner-private label
// and the custodian's own; a Research result labels it by each child's role:
// the Texture child's Report is owner-private, the web child's the world's.
func TestTextureChildReadsTaintTheRun(t *testing.T) {
	got, ok := egressReadLabels(yaegikernel.ActionTextureSearch, nil, json.RawMessage(`{"results":[{"ref":"d","label":"owner_private:texture"}]}`))
	if !ok || !got.Has("owner_private:texture") || got.Has(types.LabelWorldUntrusted) {
		t.Errorf("a Texture search did not taint the run as owner-private Texture text only: %v", got)
	}
	got, _ = egressReadLabels(yaegikernel.ActionTextureSearch, nil, json.RawMessage(`{"results":[]}`))
	if !got.Has("owner_private:texture") {
		t.Errorf("an empty Texture search lost the custodian's label: %v", got)
	}
	got, _ = egressReadLabels(yaegikernel.ActionSearchWireCorpus, nil, json.RawMessage(`{"captures":[]}`))
	if got.Has("owner_private:texture") || !got.Has(types.LabelWorldUntrusted) {
		t.Errorf("a world corpus search is not the world's: %v", got)
	}
	web, _ := json.Marshal([]yaegikernel.ResearchChildResult{{Role: yaegikernel.SessionRoleResearchWeb}})
	got, _ = egressReadLabels(yaegikernel.ActionResearch, nil, web)
	if got.Has("owner_private:texture") || !got.Has(types.LabelWorldUntrusted) {
		t.Errorf("a web child's result is not the world's: %v", got)
	}
	both, _ := json.Marshal([]yaegikernel.ResearchChildResult{{Role: yaegikernel.SessionRoleResearchWeb}, {Role: yaegikernel.SessionRoleResearchTexture}})
	got, _ = egressReadLabels(yaegikernel.ActionResearch, nil, both)
	if !got.Has("owner_private:texture") || !got.Has(types.LabelWorldUntrusted) {
		t.Errorf("a Texture child's result did not label the parent: %v", got)
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
	child := yaegikernel.SessionRoleResearchTexture + ":k-bounds"
	granted, err := rt.leases.IssueTextureSearch(ctx, owner, f.computer, rec.RunID, child)
	if err != nil {
		t.Fatal(err)
	}
	rctx := withDeskReader(objectgraph.WithLeases(ctx, rt.leaseAuthority(), rec.RunID), yaegikernel.SessionRoleResearchTexture)
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
	again, err := rt.leases.IssueTextureSearch(ctx, owner, f.computer, rec.RunID, child)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.TextureSearchUnderLease(rctx, owner, "beacon", 1); err != nil {
		t.Fatalf("a fresh lease %s is refused: %v", again.LeaseID, err)
	}
}

// leasedResearchCtx is a research run's context for a corpus search test that
// is about something else than the lease: the run reads as the research desk
// with a stub authority that admits any grant, which the world corpus search
// never asks for (MA4.6); the lease service is tested in internal/lease and
// above.
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
