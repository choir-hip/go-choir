package textureowner

// Texture holds its trajectory (docs/design/texture-holds-the-trajectory-2026-10-25.md).
// With texture.hold_trajectory on, a Texture run is marked to continue its
// earlier conversation, and the request turn shows the owner's original
// request even when it was the document's first body rather than a seed:
// on reader-4 (trajectory ac0cb480) the 8484-character request was revision
// 62d2e3e6's body, Texture's first revision replaced it with a summary, and
// three later turns asked management to find it. Ways it can fail, written
// before the code (the note's numbering):
//
//  1. Off changes the run or the turn.
//  9. The original request is missing when it was the first body and
//     Texture rewrote it.
//  10. The original request prints twice when a seed exists.

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const holdRequest = "Replace the PDF and EPUB readers with one Reader app package. Quality bar: the Reader renders the sample PDF's first page in the capture. Do not change package.json. HOLD-REQUEST-END"

// startBodyRequestLifecycle opens a trajectory the way reader-4's was: the
// owner's request is the first revision's body and no revision carries a
// seed_prompt.
func startBodyRequestLifecycle(t *testing.T, s *store.Store, suffix string) types.StartLifecycleRequest {
	t.Helper()
	docID := "doc-hold-" + suffix
	req := types.StartLifecycleRequest{
		OwnerID: "user-1", ComputerID: "autoputer-test", CommandID: "start-hold-" + suffix,
		TrajectoryID: "trajectory-hold-" + suffix, Kind: types.TrajectoryKindTask,
		SubjectRefs:     map[string]string{"artifact": "texture://document/" + docID, "doc_id": docID},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work-hold-" + suffix, Objective: holdRequest, AssignedAgentID: "texture:" + docID},
		InitialDocument: types.Document{DocID: docID, Title: "Reader"},
		InitialRevision: types.Revision{
			RevisionID: "revision-hold-root-" + suffix, AuthorKind: types.AuthorUser, AuthorLabel: "user-1",
			BodyDoc: observationBodyDoc(holdRequest),
		},
		Agent: types.AgentRecord{AgentID: "texture:" + docID, Profile: "texture", Role: "texture", ChannelID: docID},
	}
	req.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(req)
	if _, err := s.StartLifecycle(context.Background(), req); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	return req
}

type holdSwitches struct{ hold, directions bool }

// wakeAfterTheRequestWasRewritten: the request as the first body, Texture's
// summary over it, an owner correction, a Texture revision, then a report
// wake (reader-4 at 06:16).
func wakeAfterTheRequestWasRewritten(t *testing.T, suffix string, on holdSwitches) *types.RunRecord {
	t.Helper()
	core, handler := testAPISetup(t)
	core.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	s := core.Store()
	start := startBodyRequestLifecycle(t, s, suffix)
	for key, value := range map[string]bool{store.SettingTextureHoldTrajectory: on.hold, store.SettingTextureOwnerDirections: on.directions} {
		if !value {
			continue
		}
		if err := s.PutComputerSetting(t.Context(), store.ComputerSetting{OwnerID: start.OwnerID, ComputerID: start.ComputerID, Key: key, Value: true}); err != nil {
			t.Fatal(err)
		}
	}
	docID := start.InitialDocument.DocID
	summary := commitAgentHead(t, s, start, "revision-hold-summary-"+suffix, "A Reader replaces both readers; engineering will build it.")
	revise := postOwnerInstruction(t, handler, "/api/texture/documents/"+docID+"/revise", start.OwnerID, "correct-"+suffix, directionsOwner, summary.RevisionID)
	if revise.Code != http.StatusAccepted {
		t.Fatalf("owner revise status=%d body=%s", revise.Code, revise.Body.String())
	}
	commitAgentHead(t, s, start, "revision-hold-agent-"+suffix, "The Reader is the default; the old readers stay.")
	doc, err := s.GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, docID)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := handler.submitTextureAgentRevisionRun(t.Context(), doc, start.OwnerID, textureAgentRevisionRequest{
		Intent: "integrate_execution_findings", Provenance: map[string]any{"request_source": "inbox_record"},
	}, 0)
	if err != nil || rec == nil {
		t.Fatalf("submit Texture run: %+v %v", rec, err)
	}
	return rec
}

// Failure mode 9, with and without the owner-directions walk.
func TestHoldTrajectoryShowsTheOriginalRequestAfterTextureRewroteIt(t *testing.T) {
	rec := wakeAfterTheRequestWasRewritten(t, "both", holdSwitches{hold: true, directions: true})
	prompt := rec.Prompt
	reqAt, ownerAt := strings.Index(prompt, originalLabel+holdRequest), strings.Index(prompt, directionsOwner)
	if reqAt < 0 || ownerAt < reqAt {
		t.Fatalf("the wake lacks the original request above the owner's correction (request %d, owner %d):\n%s", reqAt, ownerAt, prompt)
	}
	if strings.Count(prompt, "HOLD-REQUEST-END") != 1 {
		t.Fatalf("the original request prints %d times:\n%s", strings.Count(prompt, "HOLD-REQUEST-END"), prompt)
	}
	if rec.Metadata["actor_rewarm_carry_conversation"] != true {
		t.Fatalf("the run is not marked to carry its conversation: %+v", rec.Metadata)
	}

	alone := wakeAfterTheRequestWasRewritten(t, "hold", holdSwitches{hold: true}).Prompt
	if !strings.Contains(alone, "The owner's request:\n"+holdRequest) {
		t.Fatalf("hold alone does not show the original request:\n%s", alone)
	}
}

// Failure mode 1: off, nothing changes (the defect reproduces).
func TestHoldTrajectoryOffLeavesTheRunUnchanged(t *testing.T) {
	rec := wakeAfterTheRequestWasRewritten(t, "off", holdSwitches{directions: true})
	if strings.Contains(rec.Prompt, "HOLD-REQUEST-END") {
		t.Fatalf("switch off printed the original body:\n%s", rec.Prompt)
	}
	if _, ok := rec.Metadata["actor_rewarm_carry_conversation"]; ok {
		t.Fatalf("switch off marked the run: %+v", rec.Metadata)
	}
}

// Failure mode 10: with a seed, the first body is not printed again.
func TestHoldTrajectoryKeepsTheSeedAsTheOnlyRequest(t *testing.T) {
	core, handler := testAPISetup(t)
	core.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	s := core.Store()
	start := startDirectionsLifecycle(t, s, "seeded")
	if err := s.PutComputerSetting(t.Context(), store.ComputerSetting{OwnerID: start.OwnerID, ComputerID: start.ComputerID, Key: store.SettingTextureHoldTrajectory, Value: true}); err != nil {
		t.Fatal(err)
	}
	commitAgentHead(t, s, start, "revision-seeded-agent", "The plan.")
	doc, err := s.GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := handler.submitTextureAgentRevisionRun(t.Context(), doc, start.OwnerID, textureAgentRevisionRequest{
		Intent: "integrate_execution_findings", Provenance: map[string]any{"request_source": "inbox_record"},
	}, 0)
	if err != nil || rec == nil {
		t.Fatalf("submit Texture run: %+v %v", rec, err)
	}
	if n := strings.Count(rec.Prompt, directionsSeed); n != 1 {
		t.Fatalf("the seeded request prints %d times:\n%s", n, rec.Prompt)
	}
}
