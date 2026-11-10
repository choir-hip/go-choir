package agentcore

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/problems/unsettled-obligations-cluster-2026-11-09.md, "Build: failure
// list", the boot leg. The crash boot cancelled six b@c Texture runs and left
// their work items open. Ways the leg can fail, written before it:
//   - it runs at a boot with a planned-restart marker, closing work the
//     marker-backed resume owns;
//   - it takes another desk's work item;
//   - it takes the work of a run that started after the boot closed the old
//     ones;
//   - it has no bound, so a computer with thousands of open items holds the
//     boot;
//   - it resumes, dispatches or wakes anything (it only asks the Texture
//     owner to close).

type recordedCloses struct {
	mu  sync.Mutex
	ids []string
}

func (r *recordedCloses) close(_ context.Context, w types.WorkItemRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ids = append(r.ids, w.WorkItemID)
	return nil
}

func (r *recordedCloses) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := append([]string(nil), r.ids...)
	sort.Strings(out)
	return out
}

// seedBootTextureWork starts a document lifecycle with one open work item
// assigned to agentID under profile, and returns the work item id.
func seedBootTextureWork(t *testing.T, s *store.Store, ownerID, computerID, docID, agentID, profile string) string {
	t.Helper()
	now := time.Now().UTC()
	workID := "boot-work:" + docID
	req := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "boot-work-start:" + docID,
		TrajectoryID: "trajectory-boot-work-" + docID, Kind: types.TrajectoryKindDocument,
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		InitialWork:     types.WorkItemRecord{WorkItemID: workID, Objective: "answer the owner", AssignedAgentID: agentID, AuthorityProfile: profile},
		InitialDocument: types.Document{DocID: docID, Title: "Boot work " + docID},
		InitialRevision: types.Revision{RevisionID: "boot-work-revision:" + docID, AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: "Write about the harbor."},
		Agent:           types.AgentRecord{AgentID: agentID, OwnerID: ownerID, ComputerID: computerID, Profile: profile, Role: profile, ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	req.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(req)
	if _, err := s.StartLifecycle(context.Background(), req); err != nil {
		t.Fatalf("start lifecycle %s: %v", docID, err)
	}
	return workID
}

func TestCrashBootAsksToCloseOpenTextureWorkOnlyForTexture(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	computer := rt.TextureComputerID()
	a := seedBootTextureWork(t, s, "owner-boot-work", computer, "doc-a", currentTextureAgentID("doc-a"), "texture")
	b := seedBootTextureWork(t, s, "owner-boot-work", computer, "doc-b", currentTextureAgentID("doc-b"), "texture")
	// Another desk's open work item, assigned to an engineering agent.
	seedBootTextureWork(t, s, "owner-boot-work", computer, "doc-c", "engineering:doc-c", "engineering")
	var closes recordedCloses
	rt.SetUnservedTextureCloser(closes.close)

	rt.closeOpenObligationsAtBoot(ctx)

	got, want := closes.seen(), []string{a, b}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("crash boot asked to close %v, want exactly the Texture work %v", got, want)
	}
	// The leg resumes and dispatches nothing: the work items are still the
	// closer's to settle, and no run was started for them.
	for _, id := range []string{"texture:doc-a", "texture:doc-b"} {
		if run, live, err := rt.activeRunByAgent(ctx, "owner-boot-work", id); err != nil || live {
			t.Fatalf("a run exists for %s after the crash boot: %+v, %v", id, run, err)
		}
	}
}

func TestCrashBootSkipsTextureWorkWithALiveRun(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	computer := rt.TextureComputerID()
	seedBootTextureWork(t, s, "owner-boot-live", computer, "doc-live", currentTextureAgentID("doc-live"), "texture")
	idle := seedBootTextureWork(t, s, "owner-boot-live", computer, "doc-idle", currentTextureAgentID("doc-idle"), "texture")
	now := time.Now().UTC()
	run := types.RunRecord{
		RunID: "run-after-boot", OwnerID: "owner-boot-live", ComputerID: computer, AgentID: currentTextureAgentID("doc-live"),
		AgentProfile: "texture", AgentRole: "texture", ChannelID: "doc-live", TrajectoryID: "trajectory-boot-work-doc-live", State: types.RunRunning, CreatedAt: now, UpdatedAt: now,
	}
	project := types.ReplaceLifecycleActivationRequest{OwnerID: "owner-boot-live", ComputerID: computer, CommandID: "project-live", TrajectoryID: run.TrajectoryID, AgentID: run.AgentID, Run: run}
	project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ReplaceLifecycleActivation(ctx, project); err != nil {
		t.Fatalf("project the live run: %v", err)
	}
	var closes recordedCloses
	rt.SetUnservedTextureCloser(closes.close)

	// Called alone, after the pre-boot runs were closed: this run started
	// since and serves its work.
	rt.closeUnservedTextureWorkAfterCrash(ctx)

	if got := closes.seen(); len(got) != 1 || got[0] != idle {
		t.Fatalf("closed %v, want only the work with no live run (%s)", got, idle)
	}
}

func TestCrashBootClosureIsBounded(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	computer := rt.TextureComputerID()
	for _, doc := range []string{"d1", "d2", "d3", "d4", "d5"} {
		seedBootTextureWork(t, s, "owner-boot-bound", computer, "doc-"+doc, currentTextureAgentID("doc-"+doc), "texture")
	}
	old := bootUnservedTextureCloseCap
	bootUnservedTextureCloseCap = 2
	defer func() { bootUnservedTextureCloseCap = old }()
	var closes recordedCloses
	rt.SetUnservedTextureCloser(closes.close)

	rt.closeUnservedTextureWorkAfterCrash(ctx)

	if got := closes.seen(); len(got) != 2 {
		t.Fatalf("asked to close %d items, want the bound of 2: %v", len(got), got)
	}
}

// The marker decides, in Start: a planned restart passivates its runs for the
// one resume and never reaches the crash closure; a markerless boot does.
func TestBootClosesTextureWorkOnlyWithoutAPlannedRestartMarker(t *testing.T) {
	for _, planned := range []bool{true, false} {
		rt, s := testRuntime(t)
		computer := rt.TextureComputerID()
		work := seedBootTextureWork(t, s, "owner-boot-marker", computer, "doc-marker", currentTextureAgentID("doc-marker"), "texture")
		var closes recordedCloses
		rt.SetUnservedTextureCloser(closes.close)
		rt.bootRestartPlanned = planned

		done := make(chan struct{})
		go func() {
			defer close(done)
			rt.Start(context.Background())
		}()
		select {
		case <-done:
		case <-time.After(60 * time.Second):
			t.Fatal("boot did not finish")
		}

		got := closes.seen()
		switch {
		case planned && len(got) != 0:
			t.Fatalf("a boot with a planned-restart marker closed Texture work %v: the resume owns it", got)
		case !planned && (len(got) != 1 || got[0] != work):
			t.Fatalf("a markerless boot asked to close %v, want [%s]", got, work)
		}
	}
}
