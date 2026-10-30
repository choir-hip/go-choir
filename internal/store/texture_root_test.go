package store

// PD3 S1: the Texture root desk is the master document's own Texture desk,
// named by an additive texture_root marker written by compare-and-set
// (docs/archive/definitions/choir-pd3-texture-root-desk-2026-10-11.md). Ways it can
// fail, written before the code:
//
//   - the switch is missing, unknown to the allowlist, or on by default;
//   - two ensure steps race and both claim a master (the CAS is the claim);
//   - a replacement advances the generation without naming the marker it
//     replaces, or a stale claim overwrites a newer one;
//   - the root writes a document other than the master, or another
//     document's Texture writes the master: the existing channel binding of
//     ApplyTextureTurn must refuse both, with no new check.

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestTextureRootDeskSwitchIsKnownAndOffByDefault(t *testing.T) {
	def, known := KnownComputerSettings[SettingTextureRootDesk]
	if !known || def {
		t.Fatalf("texture.root_desk known=%v default=%v, want known and off", known, def)
	}
	if SettingTextureRootDesk != "texture.root_desk" {
		t.Fatalf("switch key = %q", SettingTextureRootDesk)
	}
}

func TestTextureRootMarkerIsClaimedByCompareAndSet(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const owner, computer = "owner-root", "computer-root"
	if _, err := s.GetTextureRoot(ctx, owner, computer); !errors.Is(err, ErrNotFound) {
		t.Fatalf("marker before any claim: err=%v, want ErrNotFound", err)
	}

	// The race: eight ensure steps claim the first marker at once, each for
	// its own document. Exactly one wins; the others see a conflict.
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins, losses := 0, 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := s.ClaimTextureRoot(ctx, nil, TextureRoot{OwnerID: owner, ComputerID: computer, DocumentID: "master-" + string(rune('a'+i)), Generation: 1})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				wins++
			case errors.Is(err, ErrConcurrentStateChange):
				losses++
			default:
				t.Errorf("claim %d: unexpected err %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	if wins != 1 || losses != 7 {
		t.Fatalf("concurrent first claims: wins=%d losses=%d, want 1 and 7", wins, losses)
	}
	first, err := s.GetTextureRoot(ctx, owner, computer)
	if err != nil || first.Generation != 1 || first.DocumentID == "" {
		t.Fatalf("marker after the race = %+v, %v", first, err)
	}

	// Replacement names the marker it replaces and advances by one.
	next := TextureRoot{OwnerID: owner, ComputerID: computer, DocumentID: "master-next", Generation: 2, PredecessorDocumentID: first.DocumentID}
	if _, err := s.ClaimTextureRoot(ctx, nil, next); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("a claim that ignores the current marker: err=%v, want conflict", err)
	}
	skip := next
	skip.Generation = 3
	if _, err := s.ClaimTextureRoot(ctx, &first, skip); err == nil {
		t.Fatal("a claim that skips a generation was accepted")
	}
	claimed, err := s.ClaimTextureRoot(ctx, &first, next)
	if err != nil || claimed.Generation != 2 || claimed.DocumentID != "master-next" || claimed.PredecessorDocumentID != first.DocumentID {
		t.Fatalf("replacement claim = %+v, %v", claimed, err)
	}
	if _, err := s.ClaimTextureRoot(ctx, &first, TextureRoot{OwnerID: owner, ComputerID: computer, DocumentID: "master-stale", Generation: 2}); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("a stale claim overwrote a newer marker: err=%v", err)
	}
	got, err := s.GetTextureRoot(ctx, owner, computer)
	if err != nil || got.DocumentID != "master-next" || got.Generation != 2 {
		t.Fatalf("marker after replacement = %+v, %v", got, err)
	}
	if other, err := s.GetTextureRoot(ctx, owner, "computer-other"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("marker leaked to another computer: %+v, %v", other, err)
	}
}

// startTextureRootFixtureDocument starts one lifecycle Texture document with
// its own texture:<doc> desk and an active run, the shape the create path
// gives every document.
func startTextureRootFixtureDocument(t *testing.T, s *Store, docID string) (types.StartLifecycleRequest, types.RunRecord) {
	t.Helper()
	ctx := context.Background()
	start := lifecycleStartFixture()
	start.CommandID = "command-start-" + docID
	start.TrajectoryID = "trajectory-" + docID
	start.Kind = types.TrajectoryKindDocument
	start.InitialWork.WorkItemID = "work-" + docID
	start.InitialWork.AssignedAgentID = "texture:" + docID
	start.InitialWork.AuthorityProfile = "texture"
	start.InitialDocument.DocID = docID
	start.InitialRevision.RevisionID = "revision-" + docID
	start.InitialRevision.Content = "Initial " + docID
	start.InitialRevision.BodyDoc = lifecycleStructuredBodyDoc(docID, start.InitialRevision.RevisionID, start.InitialRevision.Content)
	start.Agent.AgentID = "texture:" + docID
	start.Agent.ChannelID = docID
	start.SubjectRefs = map[string]string{"doc_id": docID, "artifact": "texture://documents/" + docID}
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start %s: %v", docID, err)
	}
	run := lifecycleRunFixture(start, "run-"+docID, types.RunRunning)
	run.ChannelID = docID
	project := types.ReplaceLifecycleActivationRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "project-" + docID,
		TrajectoryID: start.TrajectoryID, AgentID: start.Agent.AgentID, Run: run,
	}
	project.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ReplaceLifecycleActivation(ctx, project); err != nil {
		t.Fatalf("project %s run: %v", docID, err)
	}
	return start, run
}

// textureRootTurn is a no-change turn by caller on the target document.
func textureRootTurn(t *testing.T, s *Store, target types.StartLifecycleRequest, caller types.RunRecord, callerStart types.StartLifecycleRequest, commandID string) types.ApplyTextureTurnRequest {
	t.Helper()
	req := textureTurnBaseRequest(t, s, target, caller, types.TextureTurnNoSemanticChange)
	agent, err := s.GetAgentByScope(context.Background(), callerStart.OwnerID, callerStart.ComputerID, caller.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	req.CommandID = commandID
	req.ExpectedCallerLifecycleVersion = agent.LifecycleVersion
	req.CallerWorkItemID = ""
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	return req
}

// The red invariant: per-document authority is not widened to the root. The
// root is texture:<master>, an ordinary lifecycle Texture desk, so the two
// refusals come from the channel binding every Texture turn already has.
func TestTextureRootRefusalsComeFromTheChannelBinding(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	master, rootRun := startTextureRootFixtureDocument(t, s, "doc-master")
	other, otherRun := startTextureRootFixtureDocument(t, s, "doc-other")
	if _, err := s.ClaimTextureRoot(ctx, nil, TextureRoot{OwnerID: master.OwnerID, ComputerID: master.ComputerID, DocumentID: master.InitialDocument.DocID, Generation: 1}); err != nil {
		t.Fatalf("claim the marker: %v", err)
	}

	versions := map[string]int64{}
	for _, start := range []types.StartLifecycleRequest{master, other} {
		trajectory, err := s.GetLifecycleTrajectory(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
		if err != nil {
			t.Fatal(err)
		}
		versions[start.TrajectoryID] = trajectory.LifecycleVersion
	}
	rootOnOther := textureRootTurn(t, s, other, rootRun, master, "root-writes-other")
	if _, err := s.ApplyTextureTurn(ctx, rootOnOther); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("texture:<master> wrote another document: err=%v, want the channel binding's refusal", err)
	}
	otherOnMaster := textureRootTurn(t, s, master, otherRun, other, "other-writes-master")
	if _, err := s.ApplyTextureTurn(ctx, otherOnMaster); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("texture:<other> wrote the master document: err=%v, want the channel binding's refusal", err)
	}
	for _, start := range []types.StartLifecycleRequest{master, other} {
		trajectory, err := s.GetLifecycleTrajectory(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
		if err != nil || trajectory.LifecycleVersion != versions[start.TrajectoryID] {
			t.Fatalf("a refused turn moved %s: %+v, %v", start.TrajectoryID, trajectory, err)
		}
	}

	// Each desk still writes its own document.
	rootOnMaster := textureRootTurn(t, s, master, rootRun, master, "root-writes-master")
	if _, err := s.ApplyTextureTurn(ctx, rootOnMaster); err != nil {
		t.Fatalf("texture:<master> refused on the master: %v", err)
	}
	otherOnOther := textureRootTurn(t, s, other, otherRun, other, "other-writes-other")
	if _, err := s.ApplyTextureTurn(ctx, otherOnOther); err != nil {
		t.Fatalf("texture:<other> refused on its own document: %v", err)
	}
}
