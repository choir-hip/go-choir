package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// MA4.6 (plan section 8.6): research is a parent with two child roles. The
// failure modes, written before the code:
//
//	R1 a child spec with no role, or an unknown one, runs with the wrong
//	   authority (an unknown role must be refused, never run as web);
//	R2 the web child can search the owner's stories (it holds the Texture
//	   child's lease, or SearchTexture is on its surface), so Texture-derived
//	   text reaches the child that composes web queries;
//	R3 the Texture child can reach the web (world.read on its surface);
//	R4 a child call is not a Cast carrying the parent's typed prediction, its
//	   Report is not recorded, or the fan-out is not scored (MA1.5);
//	R5 the Texture child's Report is not labelled owner_private:texture, so the
//	   parent run does not carry the label into its later sinks;
//	R6 the Texture child's lease outlives the child, or is held by the parent;
//	R7 the research desk's own run still holds texture.search.
//
// The frame check half of R2 and R3 is TestFrameRoleResearchChildrenAreNarrowerAndDisjoint
// (internal/yaegikernel); the lease half of R6 and R7 is
// TestOnlyTheTextureChildHoldsTheTextureSearchLease and
// TestTextureSearchIsTheTextureChildsUnderItsLease.

const ma46Marker = "MARKER-OWNER-PRIVATE-QUAY"

// ma46ChildProvider scripts each child by its role's preamble: one cell that
// uses its own grant and tries the other child's, then findings that quote
// the cell's output. It keeps every web-child request for the transcript check.
type ma46ChildProvider struct {
	provideriface.Provider
	mu  sync.Mutex
	web []string
}

func (p *ma46ChildProvider) CallWithTools(_ context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	raw := rawMessagesForTest(req.Messages)
	texture := strings.Contains(req.System, "You are the Texture child")
	if !texture {
		p.mu.Lock()
		p.web = append(p.web, req.System+"\n"+raw)
		p.mu.Unlock()
	}
	// Cell 1 uses the child's own grant; cell 2 tries the other child's verb
	// (a separate cell, since a verb missing from the surface fails the cell
	// at compile time).
	own, other := "r, err := choir.SearchTexture(\"lanterns\", 5)\nfmt.Println(string(r), err)", "w, err := choir.SearchWireCorpus(\"quay\", 5)\nfmt.Println(string(w), err)"
	if !texture {
		own, other = "fmt.Println(\"web child cell\")", "r, err := choir.SearchTexture(\"quay\", 5)\nfmt.Println(string(r), err)"
	}
	for _, step := range []struct{ id, src string }{{"call-ma46-own", own}, {"call-ma46-other", other}} {
		if !strings.Contains(raw, step.id) {
			args, _ := json.Marshal(map[string]any{"source": step.src})
			return &provideriface.ToolLoopResponse{StopReason: "tool_use", Model: "child-model",
				ToolCalls: []types.ToolCall{{ID: step.id, Name: "desk_go_eval", Arguments: args}}}, nil
		}
	}
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Model: "child-model",
		Text: "findings: " + raw[strings.Index(raw, "call-ma46-own"):]}, nil
}

func TestResearchFansOutToAWebChildAndATextureChild(t *testing.T) {
	f := newSubCallFixture(t, nil)
	provider := &ma46ChildProvider{}
	f.rt.provider = provider
	seedStoryForSearch(t, f.s, f.owner, "doc-quay", "Quay", "dockside lanterns brightened every evening "+ma46Marker)

	// R7: the research desk's own run holds no texture.search lease.
	if live := f.rt.leases.Live(f.owner); len(live) != 0 {
		t.Fatalf("leases before the fan-out: %+v", live)
	}
	q := []types.TypedQuestion{subCallYesNo(0.6)}
	results := f.research(t, []yaegikernel.ResearchChild{
		{Objective: "web: harbor reopening news this week", Questions: q, PSuccess: fp(0.6), Weight: fp(0.5)},
		{Objective: "texture: what the owner has written about the quay", Role: yaegikernel.ResearchChildTexture, Questions: q, PSuccess: fp(0.6), Weight: fp(0.5)},
		{Objective: "an unknown role", Role: "mail"},
	})
	f.rt.waitSubCallJudge()
	if len(results) != 3 {
		t.Fatalf("results: %+v", results)
	}
	web, tex, unknown := results[0], results[1], results[2]

	// R1: roles. An empty role is the web child; an unknown one is refused
	// before any Cast or model call.
	if web.Role != yaegikernel.SessionRoleResearchWeb || tex.Role != yaegikernel.SessionRoleResearchTexture {
		t.Fatalf("roles: web %q, texture %q", web.Role, tex.Role)
	}
	if unknown.Error == "" || unknown.CastID != "" || unknown.Findings != "" || !strings.Contains(unknown.Error, `"mail"`) {
		t.Fatalf("an unknown role was not refused: %+v", unknown)
	}

	// R3, R2 (surface): each child uses its own grant and the other's verb is
	// not on its surface.
	if !strings.Contains(tex.Findings, ma46Marker) || !strings.Contains(tex.Findings, "owner_private:texture") {
		t.Fatalf("the Texture child did not search the owner's stories: %+v", tex)
	}
	if !strings.Contains(tex.Findings, "choir has no SearchWireCorpus") || strings.Contains(tex.Findings, "capture_count") {
		t.Fatalf("the Texture child reached the world corpus: %s", tex.Findings)
	}
	if !strings.Contains(web.Findings, "choir has no SearchTexture") || strings.Contains(web.Findings, ma46Marker) {
		t.Fatalf("the web child reached SearchTexture: %s", web.Findings)
	}

	// R2 (transcript): no Texture-derived text reached the web child.
	provider.mu.Lock()
	transcripts := append([]string(nil), provider.web...)
	provider.mu.Unlock()
	if len(transcripts) < 3 {
		t.Fatalf("the web child made %d model calls", len(transcripts))
	}
	for _, tr := range transcripts {
		if strings.Contains(tr, ma46Marker) || strings.Contains(tr, "dockside") || strings.Contains(tr, "doc-quay") {
			t.Fatalf("Texture-derived text reached the web child's context:\n%s", tr)
		}
	}

	// R4: each child call is a Cast with the parent's typed prediction, its
	// Report is recorded and linked, and the fan-out is scored by one judge
	// call whose settlement names each Cast.
	for _, r := range []yaegikernel.ResearchChildResult{web, tex} {
		if r.CastID == "" || r.ReportID == "" {
			t.Fatalf("child %s has no Cast or Report: %+v", r.Role, r)
		}
		cast, report := f.record(t, r.CastID), f.record(t, r.ReportID)
		if !cast.IsTyped() || cast.IsWeightZero() || cast.PSuccess == nil || *cast.PSuccess != 0.6 {
			t.Fatalf("child %s: the Cast does not carry the parent's typed prediction: %+v", r.Role, cast)
		}
		if cast.SubCall.Role != r.Role || report.SubCall.Role != r.Role || report.ParentID != cast.RecordID {
			t.Fatalf("child %s: Cast/Report roles %q/%q, parent %q", r.Role, cast.SubCall.Role, report.SubCall.Role, report.ParentID)
		}
		if !strings.HasPrefix(report.Provenance.AgentID, r.Role+":") {
			t.Fatalf("child %s: the Report is not the child's: %+v", r.Role, report.Provenance)
		}
		if !f.exists(types.SubCallSettlementRecordID(r.CastID, types.SubCallJudgeScorerID)) {
			t.Fatalf("child %s: the Report was not scored", r.Role)
		}
	}
	if n := f.judge.callCount(); n != 1 {
		t.Fatalf("judge calls = %d, want 1 for the fan-out", n)
	}

	// R5: the Texture child's Report is owner-private, and the parent's run
	// carries the label.
	if report := f.record(t, tex.ReportID); !types.NewLabelSet(report.Provenance.Labels...).Has(types.OwnerPrivateLabel(deskmanifest.Texture)) {
		t.Fatalf("the Texture child's Report is not labelled: %v", report.Provenance.Labels)
	}
	if !f.rt.runLabelSet(f.parent.RunID).Has(types.OwnerPrivateLabel(deskmanifest.Texture)) {
		t.Fatalf("the parent run does not carry the Texture label: %v", f.rt.runLabelSet(f.parent.RunID))
	}

	// R6: the lease was the Texture child's, and ended with it.
	if live := f.rt.leases.Live(f.owner); len(live) != 0 {
		t.Fatalf("a lease outlived the fan-out: %+v", live)
	}
	all, err := f.rt.leases.All(context.Background(), f.owner, f.computer)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].Holder != deskmanifest.ResearchTexture || all[0].HolderRunID != f.parent.RunID || all[0].Uses == 0 {
		t.Fatalf("leases: want one used texture.search lease held by the Texture child for the parent run: %+v", all)
	}
}
