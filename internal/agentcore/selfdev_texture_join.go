package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func selfDevelopmentOperationIDFromPacketSources(sources []types.CoagentPacketSource) string {
	for _, source := range sources {
		key, value := splitTypedWorkerUpdateRef(source.Target.URI)
		if key == "operation" && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func selfDevelopmentTextureJoinIDs(ownerID, computerID, operationID string) (docID, revisionID, workID string) {
	key := strings.Join([]string{"choir:texture:self-development", ownerID, computerID, operationID}, ":")
	docID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+":document")).String()
	revisionID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+":revision:v0")).String()
	workID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(key+":work:engineering")).String()
	return docID, revisionID, workID
}

// ensureSelfDevelopmentEngineeringDoc creates the engineering-bound lifecycle
// document for one self-development operation and commits the directive
// revision that is the operation's cast. The document's lifecycle trajectory
// IS the operation's trajectory, so the operation store resolves it by
// trajectory. The revision occurrence opens the implementation assignment on
// the document channel — no Management run mediates the opener.
func (rt *Runtime) ensureSelfDevelopmentEngineeringDoc(ctx context.Context, operation selfdev.Operation, ownerID, prompt string) error {
	if rt == nil || rt.store == nil {
		return fmt.Errorf("start self-development run: store authority unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if ownerID == "" || computerID == "" {
		return fmt.Errorf("start self-development run: owner and computer identity are required")
	}
	if strings.TrimSpace(operation.ComputerID) != computerID {
		return fmt.Errorf("start self-development run: operation computer does not match this runtime")
	}
	trajectoryID := strings.TrimSpace(operation.TrajectoryID)
	if trajectoryID == "" {
		return fmt.Errorf("start self-development run: operation lacks trajectory binding")
	}
	docID, revisionID, workID := selfDevelopmentTextureJoinIDs(ownerID, computerID, operation.OperationID)
	deskAgentID := engineeringDeskAgentID(docID)
	directive := strings.TrimSpace(prompt)
	if directive == "" {
		directive = selfDevelopmentRewakeFallbackPrompt
	}
	now := time.Now().UTC()
	joinMeta, _ := json.Marshal(map[string]any{
		"input_origin":                  "user_prompt",
		"owner_prompt":                  directive,
		"self_development_operation_id": operation.OperationID,
	})
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "start:selfdev-engineering:" + operation.OperationID,
		TrajectoryID: trajectoryID, Kind: types.TrajectoryKindDocument,
		SubjectRefs:    map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:    types.WorkItemRecord{WorkItemID: workID, Objective: directive, AssignedAgentID: deskAgentID, AuthorityProfile: agentprofile.Engineering},
		InitialDocument: types.Document{
			DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
			Title: "Self-development operation " + operation.OperationID, CreatedAt: now, UpdatedAt: now,
		},
		InitialRevision: types.Revision{
			RevisionID: revisionID, DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
			AuthorKind: types.AuthorUser, AuthorLabel: ownerID,
			Content: directive, Metadata: joinMeta, CreatedAt: now,
		},
		Agent: types.AgentRecord{
			AgentID: deskAgentID, OwnerID: ownerID, ComputerID: computerID,
			Profile: agentprofile.Engineering, Role: agentprofile.Engineering, ChannelID: docID, CreatedAt: now, UpdatedAt: now,
		},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	result, err := rt.store.StartLifecycle(ctx, start)
	if err != nil {
		return fmt.Errorf("start self-development engineering lifecycle: %w", err)
	}
	if result.Revision == nil {
		return fmt.Errorf("start self-development engineering lifecycle: revision unavailable")
	}
	if !rt.DispatchActorActive() {
		return fmt.Errorf("self-development engineering document committed but actor dispatch unavailable")
	}
	reducerSeq := int64(0)
	for _, event := range result.Events {
		if event.Kind == types.LifecycleArtifactHeadAdvanced &&
			len(event.ArtifactRefs) >= 2 && strings.TrimSpace(event.ArtifactRefs[1]) == result.Revision.RevisionID {
			reducerSeq = event.ReducerSeq
		}
	}
	requestID := "owner-request-selfdev-" + operation.OperationID
	occurrence, occErr := DocumentRevisionOccurrence(*result.Revision, agentprofile.Engineering, requestID, result.Trajectory.LifecycleVersion, reducerSeq)
	if occErr != nil {
		return fmt.Errorf("build self-development engineering revision occurrence: %w", occErr)
	}
	content, encErr := EncodeTextureActorOccurrence(occurrence)
	if encErr != nil {
		return fmt.Errorf("encode self-development engineering revision occurrence: %w", encErr)
	}
	if err := rt.DispatchActor(ctx, ownerID, computerID, deskAgentID, "coagent_result",
		content, trajectoryID, "owner:"+ownerID); err != nil {
		return fmt.Errorf("wake self-development engineering desk for owner revision: %w", err)
	}
	return nil
}

const selfDevelopmentRewakeFallbackPrompt = "Author, freeze, and propose the bound self-development operation."
