package textureowner

// Texture re-cast a trajectory's original spec over the owner's later
// revision (docs/design/texture-spec-follows-owner-2026-10-25.md; evidence
// run 230c5a4e, revision ee41afb6 on b@c). The request turn read owner intent
// from the head revision's metadata only: owner_prompt is per-revision and
// seed_prompt is durable, so one appagent revision after an owner revise
// dropped the owner's direction and left the original spec labelled "The
// owner's request". Ways the fix can fail, written before the code:
//
//  1. An appagent head after an owner revise still drops the direction.
//  2. Directions print out of order, so an older one reads as current.
//  3. The seed still reads as the owner's whole request when a later
//     direction exists.
//  4. The head's own direction prints twice.
//  5. With no directions the turn changes for every other document.
//  6. The walk loops on a parent cycle or runs unbounded on a long story.
//  7. A missing parent fails the walk instead of stopping it.
//  8. A huge or numerous set of directions bloats the prompt.
//  9. Switch off still changes the turn.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	directionsSeed   = "Replace the readers. Delete the old readers: remove PdfApp.svelte and EpubApp.svelte."
	directionsOwner  = "Keep PdfApp and EpubApp beside the Reader; do not delete them. The Reader opens PDF and EPUB by default."
	originalLabel    = "The owner's original request (the owner's later directions below override it wherever they differ):\n"
	laterLabelPrefix = "The owner's later directions, oldest first."
)

func directionsMeta(t *testing.T, fields map[string]any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// startDirectionsLifecycle opens a task trajectory whose seed revision is the
// owner's request, with an open Texture work item.
func startDirectionsLifecycle(t *testing.T, s *store.Store, suffix string) types.StartLifecycleRequest {
	t.Helper()
	docID := "doc-directions-" + suffix
	req := types.StartLifecycleRequest{
		OwnerID: "user-1", ComputerID: "autoputer-test", CommandID: "start-directions-" + suffix,
		TrajectoryID: "trajectory-directions-" + suffix, Kind: types.TrajectoryKindTask,
		SubjectRefs:     map[string]string{"artifact": "texture://document/" + docID, "doc_id": docID},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work-directions-" + suffix, Objective: "build the Reader", AssignedAgentID: "texture:" + docID},
		InitialDocument: types.Document{DocID: docID, Title: "Reader"},
		InitialRevision: types.Revision{
			RevisionID: "revision-directions-seed-" + suffix, AuthorKind: types.AuthorUser, AuthorLabel: "user-1",
			BodyDoc:  observationBodyDoc(directionsSeed),
			Metadata: directionsMeta(t, map[string]any{"input_origin": textureInputOriginUserPrompt, "seed_prompt": directionsSeed}),
		},
		Agent: types.AgentRecord{AgentID: "texture:" + docID, Profile: "texture", Role: "texture", ChannelID: docID},
	}
	req.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(req)
	if _, err := s.StartLifecycle(context.Background(), req); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	return req
}

// commitAgentHead commits an appagent revision on the head the way Texture's
// writer leaves one: the durable keys carried, no owner_prompt.
func commitAgentHead(t *testing.T, s *store.Store, start types.StartLifecycleRequest, revisionID, text string) types.Revision {
	t.Helper()
	ctx := context.Background()
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	head := snapshot.HeadRevision
	rev := types.Revision{
		RevisionID: revisionID, DocID: start.InitialDocument.DocID, OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		TrajectoryID: start.TrajectoryID, AuthorKind: types.AuthorAppAgent, AuthorLabel: "appagent",
		BodyDoc:          observationBodyDoc(text),
		Metadata:         carryForwardDurableTextureMetadata(directionsMeta(t, map[string]any{"source": "texture_cell"}), head.Metadata),
		ParentRevisionID: head.RevisionID,
	}
	command := types.CommitLifecycleArtifactHeadRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "agent-head:" + revisionID,
		TrajectoryID: start.TrajectoryID, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID: head.RevisionID, Revision: rev,
	}
	command.CommandDigest, err = store.ComputeCommitLifecycleArtifactHeadWithSourceGraphDigest(command, store.TextureSourceGraphWriteSet{})
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	result, err := s.CommitLifecycleArtifactHeadWithSourceGraph(ctx, command, store.TextureSourceGraphWriteSet{})
	if err != nil || result.Revision == nil {
		t.Fatalf("commit agent head: %+v %v", result, err)
	}
	return *result.Revision
}

// wakeAfterOwnerReviseAndAgentRevision reproduces 230c5a4e's inputs: seed
// request, owner revise, one Texture revision, then a progress-relay wake.
func wakeAfterOwnerReviseAndAgentRevision(t *testing.T, suffix string, switchOn bool) string {
	t.Helper()
	core, handler := testAPISetup(t)
	core.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	s := core.Store()
	start := startDirectionsLifecycle(t, s, suffix)
	if switchOn {
		if err := s.PutComputerSetting(t.Context(), store.ComputerSetting{OwnerID: start.OwnerID, ComputerID: start.ComputerID, Key: store.SettingTextureOwnerDirections, Value: true}); err != nil {
			t.Fatal(err)
		}
	}
	docID := start.InitialDocument.DocID
	revise := postOwnerInstruction(t, handler, "/api/texture/documents/"+docID+"/revise", start.OwnerID, "keep-"+suffix, directionsOwner, start.InitialRevision.RevisionID)
	if revise.Code != http.StatusAccepted {
		t.Fatalf("owner revise status=%d body=%s", revise.Code, revise.Body.String())
	}
	commitAgentHead(t, s, start, "revision-directions-agent-"+suffix, "The plan: retain both readers and make the Reader the default.")
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
	return rec.Prompt
}

// Failure mode 1 end to end: the owner's direction reaches a wake whose head
// is a Texture revision, after the original request and above it.
func TestOwnerDirectionReachesTheWakeAfterAnAgentRevision(t *testing.T) {
	prompt := wakeAfterOwnerReviseAndAgentRevision(t, "on", true)
	seedAt := strings.Index(prompt, originalLabel+directionsSeed)
	ownerAt := strings.Index(prompt, directionsOwner)
	if seedAt < 0 || ownerAt < 0 || ownerAt < seedAt {
		t.Fatalf("the owner's direction does not follow the original request in the wake (seed %d, owner %d):\n%s", seedAt, ownerAt, prompt)
	}
	if !strings.Contains(prompt, laterLabelPrefix) || !strings.Contains(prompt, "never undo") {
		t.Fatalf("the wake does not rank the later direction over the original request:\n%s", prompt)
	}
	if strings.Contains(prompt, "The owner's request:\n") {
		t.Fatalf("the original request is still labelled as the owner's whole request:\n%s", prompt)
	}
}

// Failure mode 9: off, the wake is what it was (the defect reproduces).
func TestOwnerDirectionsSwitchOffKeepsTheWakeUnchanged(t *testing.T) {
	prompt := wakeAfterOwnerReviseAndAgentRevision(t, "off", false)
	if !strings.Contains(prompt, "The owner's request:\n"+directionsSeed) || strings.Contains(prompt, directionsOwner) {
		t.Fatalf("switch off changed the wake:\n%s", prompt)
	}
}

func userDirectionRevision(id, parent, text string, at time.Time) types.Revision {
	raw, _ := json.Marshal(map[string]any{"input_origin": textureInputOriginUserPrompt, "owner_prompt": text, "seed_prompt": directionsSeed})
	return types.Revision{RevisionID: id, ParentRevisionID: parent, AuthorKind: types.AuthorUser, Metadata: raw, CreatedAt: at}
}

func agentRevision(id, parent string) types.Revision {
	raw, _ := json.Marshal(map[string]any{"seed_prompt": directionsSeed})
	return types.Revision{RevisionID: id, ParentRevisionID: parent, AuthorKind: types.AuthorAppAgent, Metadata: raw}
}

type revisionChain map[string]types.Revision

func (c revisionChain) load(calls *int) func(string) (types.Revision, error) {
	return func(id string) (types.Revision, error) {
		*calls++
		rev, ok := c[id]
		if !ok {
			return types.Revision{}, errors.New("not found")
		}
		return rev, nil
	}
}

func (c revisionChain) add(revs ...types.Revision) revisionChain {
	for _, rev := range revs {
		c[rev.RevisionID] = rev
	}
	return c
}

// Failure modes 2 and 7: oldest first, and a missing ancestor ends the walk.
func TestOwnerDirectionsCollectOldestFirstAndStopAtAMissingParent(t *testing.T) {
	t0 := time.Date(2026, 10, 25, 2, 14, 24, 0, time.UTC)
	chain := revisionChain{}.add(
		userDirectionRevision("u1", "gone", "hold", t0),
		agentRevision("a1", "u1"),
		userDirectionRevision("u2", "a1", directionsOwner, t0.Add(45*time.Minute)),
		agentRevision("a2", "u2"),
		agentRevision("head", "a2"),
	)
	calls := 0
	got := collectTextureOwnerDirections(chain["head"], nil, chain.load(&calls))
	if len(got) != 2 || got[0].RevisionID != "u1" || got[0].Text != "hold" || got[1].RevisionID != "u2" || got[1].Text != directionsOwner || !got[1].At.Equal(t0.Add(45*time.Minute)) {
		t.Fatalf("directions = %+v; want u1 (hold) then u2", got)
	}
}

// Failure mode 4 at the walk: the head's own direction is not collected.
func TestOwnerDirectionsExcludeTheHeadsOwnDirection(t *testing.T) {
	chain := revisionChain{}.add(
		userDirectionRevision("u1", "", "hold", time.Time{}),
		agentRevision("a1", "u1"),
		userDirectionRevision("head", "a1", directionsOwner, time.Time{}),
	)
	calls := 0
	prev := chain["a1"]
	got := collectTextureOwnerDirections(chain["head"], &prev, chain.load(&calls))
	if len(got) != 1 || got[0].RevisionID != "u1" {
		t.Fatalf("directions = %+v; want only u1", got)
	}
	if calls != 1 {
		t.Fatalf("walk loaded %d revisions; want 1 (the loaded parent is reused)", calls)
	}
}

// Failure modes 6 and 8: a cycle and a long chain end within the bound, and
// the kept count and text length are capped.
func TestOwnerDirectionsWalkIsBounded(t *testing.T) {
	cycle := revisionChain{}.add(agentRevision("x", "y"), agentRevision("y", "x"))
	calls := 0
	if got := collectTextureOwnerDirections(cycle["x"], nil, cycle.load(&calls)); len(got) != 0 || calls > textureOwnerDirectionWalkLimit {
		t.Fatalf("cycle: directions=%d loads=%d", len(got), calls)
	}

	long := revisionChain{}
	parent := ""
	for i := 0; i < 3*textureOwnerDirectionWalkLimit; i++ {
		id := fmt.Sprintf("r%03d", i)
		long.add(userDirectionRevision(id, parent, strings.Repeat("d", 2*textureOwnerDirectionMaxChars)+id, time.Time{}))
		parent = id
	}
	head := agentRevision("head", parent)
	calls = 0
	got := collectTextureOwnerDirections(head, nil, long.load(&calls))
	if calls > textureOwnerDirectionWalkLimit {
		t.Fatalf("long chain: %d loads; bound is %d", calls, textureOwnerDirectionWalkLimit)
	}
	if len(got) != textureOwnerDirectionKeep {
		t.Fatalf("long chain kept %d directions; want %d", len(got), textureOwnerDirectionKeep)
	}
	if last := got[len(got)-1]; last.RevisionID != parent {
		t.Fatalf("the newest direction is %s; want %s", last.RevisionID, parent)
	}
	for _, d := range got {
		if len(d.Text) > textureOwnerDirectionMaxChars {
			t.Fatalf("direction %s is %d chars; cap is %d", d.RevisionID, len(d.Text), textureOwnerDirectionMaxChars)
		}
	}
}

// Failure modes 3, 4 and 5 at the builder.
func TestWakePromptRanksOwnerDirectionsAboveTheOriginalRequest(t *testing.T) {
	seedMeta := map[string]any{"seed_prompt": directionsSeed}
	head := agentRevision("head", "a1")
	parent := agentRevision("a1", "u1")

	// 5: no directions, byte-identical to the turn without the projection.
	if a, b := buildAgentRevisionRequest(head, &parent, seedMeta, textureAgentRevisionRequest{}, "", nil),
		buildAgentRevisionRequest(head, &parent, seedMeta, textureAgentRevisionRequest{}, "", []textureOwnerDirection{}); a != b || !strings.Contains(a, "The owner's request:\n"+directionsSeed) {
		t.Fatalf("no directions changed the turn:\n%s\n---\n%s", a, b)
	}

	// 2 and 3: two directions, oldest first, under the ranking rule.
	at := time.Date(2026, 10, 25, 2, 59, 28, 0, time.UTC)
	directions := []textureOwnerDirection{{RevisionID: "u0", Text: "hold"}, {RevisionID: "u1", Text: directionsOwner, At: at}}
	got := buildAgentRevisionRequest(head, &parent, seedMeta, textureAgentRevisionRequest{}, "", directions)
	seedAt, holdAt, ownerAt := strings.Index(got, originalLabel+directionsSeed), strings.Index(got, "hold"), strings.Index(got, directionsOwner)
	if seedAt < 0 || holdAt < seedAt || ownerAt < holdAt || !strings.Contains(got, "2026-10-25T02:59:28Z") {
		t.Fatalf("directions out of order or unlabelled (seed %d, hold %d, owner %d):\n%s", seedAt, holdAt, ownerAt, got)
	}

	// 4: an owner head prints its direction once, after the earlier ones.
	ownerHead := userDirectionRevision("head", "a1", directionsOwner, at)
	headMeta := decodeRevisionMetadata(ownerHead.Metadata)
	got = buildAgentRevisionRequest(ownerHead, &parent, headMeta, textureAgentRevisionRequest{}, "", []textureOwnerDirection{{RevisionID: "u0", Text: "hold"}})
	if strings.Count(got, directionsOwner) != 1 || !strings.Contains(got, originalLabel) || strings.Index(got, "hold") > strings.Index(got, "The owner's direction for this revision:\n"+directionsOwner) {
		t.Fatalf("owner head direction printed wrong:\n%s", got)
	}
}
