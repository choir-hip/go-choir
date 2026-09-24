package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The self-development join is an engineering-bound lifecycle document: the
// operation's directive commits as the initial owner-authored revision, and
// the revision occurrence is the cast the engineering desk consumes.
func TestSelfDevelopmentEngineeringDocJoinCommitsDirectiveRevision(t *testing.T) {
	ctx := context.Background()
	runtime, productStore := testRuntime(t)
	ownerID := "owner-selfdev-join"
	computerID := "computer-selfdev-join"
	runtime.cfg.ComputerID = computerID
	operation := selfdev.Operation{
		OperationID:       "selfdev-op-join-test",
		ComputerID:        computerID,
		TrajectoryID:      "trajectory-selfdev-join",
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("c", 64),
	}
	directive := "Author classic solitaire game engine"
	if err := runtime.ensureSelfDevelopmentEngineeringDoc(ctx, operation, ownerID, directive); err != nil {
		t.Fatal(err)
	}

	docID, revisionID, workID := selfDevelopmentTextureJoinIDs(ownerID, computerID, operation.OperationID)
	deskAgentID := engineeringDeskAgentID(docID)

	doc, err := productStore.GetLifecycleDocument(ctx, ownerID, computerID, docID)
	if err != nil {
		t.Fatalf("engineering document missing: %v", err)
	}
	if doc.TrajectoryID != operation.TrajectoryID {
		t.Fatalf("document trajectory %q != operation trajectory %q", doc.TrajectoryID, operation.TrajectoryID)
	}
	snapshot, err := productStore.GetLifecycleSnapshot(ctx, ownerID, computerID, operation.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	head := snapshot.HeadRevision
	if head.RevisionID != revisionID || head.AuthorKind != types.AuthorUser {
		t.Fatalf("head revision is not the committed owner directive: %+v", head)
	}
	if head.Content != directive {
		t.Fatalf("directive revision content %q != %q", head.Content, directive)
	}
	meta := map[string]any{}
	if err := json.Unmarshal(head.Metadata, &meta); err != nil {
		t.Fatal(err)
	}
	if meta["owner_prompt"] != directive || meta["self_development_operation_id"] != operation.OperationID {
		t.Fatalf("directive revision metadata missing owner_prompt/operation join: %#v", meta)
	}
	var deskWork *types.WorkItemRecord
	for i := range snapshot.WorkItems {
		if snapshot.WorkItems[i].WorkItemID == workID {
			copy := snapshot.WorkItems[i]
			deskWork = &copy
		}
	}
	if deskWork == nil || deskWork.AssignedAgentID != deskAgentID || deskWork.AuthorityProfile != agentprofile.Engineering {
		t.Fatalf("engineering desk work item missing or misbound: %+v", deskWork)
	}
	agent, err := productStore.GetAgentByScope(ctx, ownerID, computerID, deskAgentID)
	if err != nil || agent.Profile != agentprofile.Engineering || agent.ChannelID != docID {
		t.Fatalf("engineering desk agent missing or misbound: %+v err=%v", agent, err)
	}
}

// The join is idempotent: a replayed start replays the committed lifecycle
// rather than minting a second document or revision.
func TestSelfDevelopmentEngineeringDocJoinReplaysCommittedStart(t *testing.T) {
	ctx := context.Background()
	runtime, productStore := testRuntime(t)
	ownerID := "owner-selfdev-replay"
	computerID := "computer-selfdev-replay"
	runtime.cfg.ComputerID = computerID
	operation := selfdev.Operation{
		OperationID:       "selfdev-op-replay-test",
		ComputerID:        computerID,
		TrajectoryID:      "trajectory-selfdev-replay",
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("d", 64),
	}
	if err := runtime.ensureSelfDevelopmentEngineeringDoc(ctx, operation, ownerID, "first directive"); err != nil {
		t.Fatal(err)
	}
	if err := runtime.ensureSelfDevelopmentEngineeringDoc(ctx, operation, ownerID, "first directive"); err != nil {
		t.Fatalf("replayed join failed: %v", err)
	}
	docID, revisionID, _ := selfDevelopmentTextureJoinIDs(ownerID, computerID, operation.OperationID)
	snapshot, err := productStore.GetLifecycleSnapshot(ctx, ownerID, computerID, operation.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.HeadRevision.RevisionID != revisionID {
		t.Fatalf("replayed join advanced the head: %+v", snapshot.HeadRevision)
	}
	doc, err := productStore.GetLifecycleDocument(ctx, ownerID, computerID, docID)
	if err != nil || doc.CurrentRevisionID != revisionID {
		t.Fatalf("replayed join mutated the document: %+v err=%v", doc, err)
	}
}
