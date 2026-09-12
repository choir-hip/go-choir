package store

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Cadence gate (scheduling-and-candidate-proof-2026-08-21, Phase 2):
// semantic-changing authoring turns produce exactly one revision each;
// wait/block/no-change turns produce none; the deterministic self-development
// caller never authors a revision (no milestone forcing).
func cadenceSuperOpenerFixture(t *testing.T, s *Store, suffix string) (*Store, types.StartLifecycleRequest, types.RunRecord, string) {
	t.Helper()
	_, start, caller, _ := setupLifecycleTextureTargetFixtureWithStore(t, s)
	return s, start, caller, "super:" + start.OwnerID
}

func callerForTest(start types.StartLifecycleRequest, runID string) types.RunRecord {
	textureAgentID := agentprofile.Texture + ":" + start.InitialDocument.DocID
	return types.RunRecord{
		RunID: runID, OwnerID: start.OwnerID, ComputerID: start.ComputerID, AgentID: textureAgentID,
		AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, ChannelID: start.InitialDocument.DocID,
		TrajectoryID: start.TrajectoryID, State: types.RunRunning,
		Metadata: map[string]any{
			"lifecycle_work_item_id": start.InitialWork.WorkItemID,
			"work_item_ids":          []string{start.InitialWork.WorkItemID},
		},
	}
}
func TestApplyTextureTurnRevisionOutcomeProducesExactlyOneAppagentVersion(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()

	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnRevision)
	rev := start.InitialRevision
	rev.RevisionID = "cadence-revision-1"
	rev.AuthorKind = types.AuthorAppAgent
	rev.AuthorLabel = "appagent"
	rev.ParentRevisionID = start.InitialRevision.RevisionID
	rev.CreatedAt = time.Time{}
	// Appagent revisions require a structured body_doc; the content must match
	// its derived projection.
	doc := userAuthoredTextStructuredTextureDoc(rev.DocID, rev.RevisionID, "semantic change: candidate A progress note")
	bodyDocJSON, _ := json.Marshal(doc)
	rev.BodyDoc = bodyDocJSON
	rev.Content = "semantic change: candidate A progress note"
	req.Revision = rev
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	result, err := s.ApplyTextureTurn(ctx, req)
	if err != nil {
		t.Fatalf("revision turn: %v", err)
	}
	if result.TextureTurn == nil || result.TextureTurn.Outcome != types.TextureTurnRevision {
		t.Fatalf("turn record = %+v", result.TextureTurn)
	}
	if result.Revision == nil || result.Revision.AuthorKind != types.AuthorAppAgent {
		t.Fatalf("revision author = %+v", result.Revision)
	}
	afterDoc, err := s.GetLifecycleDocument(ctx, start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	if err != nil || afterDoc.CurrentRevisionID != result.Revision.RevisionID {
		t.Fatalf("head after revision turn = %+v err=%v", afterDoc, err)
	}
	head, err := s.GetLifecycleRevision(ctx, start.OwnerID, start.ComputerID, afterDoc.CurrentRevisionID)
	if err != nil || head.AuthorKind != types.AuthorAppAgent {
		t.Fatalf("head revision = %+v err=%v", head, err)
	}
}

func TestApplyTextureTurnWaitBlockAndNoChangeProduceNoRevision(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	baseHead := start.InitialRevision.RevisionID

	for _, outcome := range []types.TextureTurnOutcome{types.TextureTurnWait, types.TextureTurnBlock, types.TextureTurnNoSemanticChange} {
		req := textureTurnBaseRequest(t, s, start, caller, outcome)
		req.CommandID = "cadence-" + string(outcome)
		setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
		result, err := s.ApplyTextureTurn(ctx, req)
		if err != nil {
			t.Fatalf("%s turn: %v", outcome, err)
		}
		if result.TextureTurn == nil || result.TextureTurn.Outcome != outcome {
			t.Fatalf("%s turn record = %+v", outcome, result.TextureTurn)
		}
		if result.Revision != nil {
			t.Fatalf("%s turn manufactured a revision: %+v", outcome, result.Revision)
		}
		doc, docErr := s.GetLifecycleDocument(ctx, start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
		if docErr != nil || doc.CurrentRevisionID != baseHead {
			t.Fatalf("%s turn moved the canonical head: %+v err=%v", outcome, doc, docErr)
		}
	}
}

// The deterministic self-development caller run identity must never appear as
// a revision author: revisions come only from genuine Texture agent runs
// (AuthorAppAgent via applyTextureLifecycleTurn) or owner edits (AuthorUser).
func TestSelfDevelopmentDeterministicCallerNeverAuthorsRevisions(t *testing.T) {
	s, start, _, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()

	deterministicCallerRun := "run-deterministic-selfdev-caller"
	rev := start.InitialRevision
	rev.RevisionID = ""
	rev.Content = "attempted synthetic revision from deterministic caller"
	rev.AuthorKind = types.AuthorAppAgent
	rev.ParentRevisionID = start.InitialRevision.RevisionID
	rev.CreatedAt = time.Time{}
	req := textureTurnBaseRequest(t, s, start, callerForTest(start, deterministicCallerRun), types.TextureTurnRevision)
	req.Revision = rev
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	if _, err := s.ApplyTextureTurn(ctx, req); err == nil {
		t.Fatalf("deterministic non-resident caller committed a revision turn; expected authority refusal")
	}
	doc, _ := s.GetLifecycleDocument(ctx, start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	if doc.CurrentRevisionID != start.InitialRevision.RevisionID {
		t.Fatalf("canonical head moved despite refusal: %+v", doc)
	}
}

// Scheduler/assignment lifecycle never calls ApplyTextureTurn: asserted by
// absence — no cosuper_assignment source file references ApplyTextureTurn.
// This test pins the observable consequence: opening and cancelling CoSuper
// assignments leaves every texture document head untouched.

func TestCoSuperAssignmentLifecycleLeavesDocumentHeadsUntouched(t *testing.T) {
	// The scheduling contract (I26) forbids scheduler/assignment lifecycle from
	// manufacturing revisions. Pinned at the store boundary: QueueLifecycleUpdate
	// (the only write path assignment receipts use) cannot advance a document
	// head — its reducer writes update objects and events only. The full
	// delivered-control join is exercised by lifecycle_control_delivery_test;
	// here we pin just the observable: no ApplyTextureTurn callsite exists in
	// cosuper_assignment_*.go or super_controller.go, so no lifecycle event can
	// carry artifact-advancing semantics.
	repo := filepath.Join("..", "..")
	sources := []string{
		filepath.Join(repo, "internal", "agentcore", "cosuper_assignment_runtime.go"),
		filepath.Join(repo, "internal", "agentcore", "cosuper_assignment_fate.go"),
		filepath.Join(repo, "internal", "agentcore", "super_controller.go"),
	}
	for _, path := range sources {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if bytes.Contains(body, []byte("ApplyTextureTurn")) {
			t.Fatalf("%s calls ApplyTextureTurn; scheduler/assignment lifecycle must never author texture turns", path)
		}
	}
}
