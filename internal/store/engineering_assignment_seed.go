package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// EngineeringAssignmentSeed is the records authority (C1e D4) used by
// assignment command tests and runtime restart tests: a live trajectory, the
// document that names it, the persistent management run that casts, and a
// Texture's Cast that opened round ParentRound of management's conversation
// with texture:<doc>.
type EngineeringAssignmentSeed struct {
	OwnerID, ComputerID, TrajectoryID string
	DocID, TextureInbox               string
	ParentAgentID, ParentRunID        string
	ParentDecisionID                  string
	ParentConversationID              string
	ParentRound                       int
	// TextureCastID is the record id of the Texture's Cast an engineering
	// Cast names as its parent.
	TextureCastID    string
	AssignmentIDs    []string
	AssignedAgentIDs []string
	AssignedRunIDs   []string
}

// SeedEngineeringAssignmentAuthority installs the records authority required to
// open assigned Engineering work: count assignment identities are derived,
// none is opened.
func SeedEngineeringAssignmentAuthority(s *Store, ownerID, computerID string, count int) (EngineeringAssignmentSeed, error) {
	return SeedEngineeringAssignmentAuthorityForDocument(s, ownerID, computerID, "assignment", count)
}

// SeedEngineeringAssignmentAuthorityForDocument is the seed for one more
// document of the same owner: its own trajectory, document and Texture
// Cast into the shared management desk, so two documents' requests can
// each hold an assignment (C1e failure item 3). key names the document;
// "assignment" is the default seed.
func SeedEngineeringAssignmentAuthorityForDocument(s *Store, ownerID, computerID, key string, count int) (EngineeringAssignmentSeed, error) {
	return SeedEngineeringAssignmentAuthorityWithCastBody(s, ownerID, computerID, key, count, "")
}

// SeedEngineeringAssignmentAuthorityWithCastBody is the seed whose Texture
// Cast carries castBody as its packet (the directive body), such as the
// root's Cast naming a rebase notice (U4).
func SeedEngineeringAssignmentAuthorityWithCastBody(s *Store, ownerID, computerID, key string, count int, castBody string) (EngineeringAssignmentSeed, error) {
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID, computerID, key = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(key)
	f := EngineeringAssignmentSeed{
		OwnerID: ownerID, ComputerID: computerID, TrajectoryID: "trajectory-" + key,
		DocID: "document-" + key, TextureInbox: "texture:document-" + key,
		ParentAgentID: "management:" + ownerID, ParentRunID: "run-management-assignment",
		ParentDecisionID: "decision:" + objectgraph.SHA256([]byte("decision-"+key)),
		TextureCastID:    "texture-cell:Cast:" + key,
	}
	f.ParentConversationID = types.InboxConversationID(f.ParentAgentID, f.TextureInbox)
	trajectory := types.TrajectoryRecord{
		TrajectoryID: f.TrajectoryID, OwnerID: f.OwnerID, ComputerID: f.ComputerID,
		Kind: types.TrajectoryKindTask, Status: types.TrajectoryLive,
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequiredSubjectRefs: []string{"subject"}},
		SubjectRefs:    map[string]string{"subject": "artifact:subject", "doc_id": f.DocID}, LifecycleVersion: 1, ReducerSeq: 1, CreatedAt: now, UpdatedAt: now,
	}
	document := types.Document{DocID: f.DocID, OwnerID: f.OwnerID, ComputerID: f.ComputerID, TrajectoryID: f.TrajectoryID,
		Title: "Assignment authority", CurrentRevisionID: "revision-assignment", CreatedAt: now, UpdatedAt: now}
	revision := types.Revision{RevisionID: "revision-assignment", DocID: document.DocID, OwnerID: f.OwnerID, ComputerID: f.ComputerID, AuthorKind: types.AuthorAppAgent, AuthorLabel: "Choir", Content: "assignment authority", CreatedAt: now}
	parentAgent := types.AgentRecord{
		AgentID: f.ParentAgentID, OwnerID: f.OwnerID, ComputerID: f.ComputerID,
		Profile: "management", Role: "management", ChannelID: f.ParentAgentID, ActiveRunID: f.ParentRunID,
		LifecycleVersion: 0, CreatedAt: now, UpdatedAt: now,
	}
	parentRun := types.RunRecord{
		RunID: f.ParentRunID, AgentID: f.ParentAgentID, ChannelID: f.ParentAgentID,
		AgentProfile: "management", AgentRole: "management", OwnerID: f.OwnerID, ComputerID: f.ComputerID,
		State: types.RunRunning, Prompt: "coordinate", CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{"request_source": "inbox_record"},
	}
	must := func(kind objectgraph.ObjectKind, key string, body any, metadata map[string]any) (objectgraph.Object, error) {
		return lifecycleObject(kind, f.OwnerID, f.ComputerID, key, body, metadata, now, now)
	}
	trajObj, err := must(ogKindTrajectory, f.TrajectoryID, trajectory, lifecycleMetadata("trajectory_id", f.TrajectoryID, f.ComputerID, f.TrajectoryID, 1))
	if err != nil {
		return EngineeringAssignmentSeed{}, err
	}
	agentObj, err := must(ogKindAgent, f.ParentAgentID, parentAgent, map[string]any{"agent_id": f.ParentAgentID, "computer_id": f.ComputerID})
	if err != nil {
		return EngineeringAssignmentSeed{}, err
	}
	docObj, err := must(ogKindTexDoc, document.DocID, document, map[string]any{"doc_id": document.DocID, "computer_id": f.ComputerID})
	if err != nil {
		return EngineeringAssignmentSeed{}, err
	}
	revObj, err := must(ogKindTexRev, revision.RevisionID, revision, map[string]any{"revision_id": revision.RevisionID, "doc_id": document.DocID, "computer_id": f.ComputerID})
	if err != nil {
		return EngineeringAssignmentSeed{}, err
	}
	// A second document's seed shares the management desk and its run.
	_, parentErr := s.GetRunByOwnerOG(ctx, f.OwnerID, f.ParentRunID)
	objects := []objectgraph.Object{trajObj, docObj, revObj}
	if parentErr != nil {
		objects = append(objects, agentObj)
	}
	if err := s.ogStore.PutBatch(ctx, objectgraph.Batch{Objects: objects}); err != nil {
		return EngineeringAssignmentSeed{}, err
	}
	if parentErr != nil {
		if err := s.CreateRunOG(ctx, parentRun); err != nil {
			return EngineeringAssignmentSeed{}, err
		}
	}
	textureCast := TextureCastRecord(f.TextureCastID, f.TextureInbox, f.ParentAgentID, "change the platform")
	textureCast.Directive.Body = castBody
	cast := types.CommitInboxRecordsRequest{OwnerID: f.OwnerID, ComputerID: f.ComputerID, CommandID: "seed-texture-cast-" + key,
		Records: []types.CommitmentRecord{textureCast}}
	if cast.CommandDigest, err = ComputeCommitInboxRecordsDigest(cast); err != nil {
		return EngineeringAssignmentSeed{}, err
	}
	receipt, err := s.CommitInboxRecords(ctx, cast)
	if err != nil {
		return EngineeringAssignmentSeed{}, err
	}
	f.ParentRound = receipt.Records[0].Round
	prefix := "assignment-"
	if key != "assignment" {
		prefix = "assignment-" + key + "-"
	}
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("%s%02d", prefix, i)
		f.AssignmentIDs = append(f.AssignmentIDs, id)
		f.AssignedAgentIDs = append(f.AssignedAgentIDs, "engineering:"+id)
		f.AssignedRunIDs = append(f.AssignedRunIDs, "run:"+id)
	}
	return f, nil
}

// TextureCastRecord is a Texture's Cast to management, as a Texture turn
// commits it.
func TextureCastRecord(recordID, textureInbox, managementInbox, objective string) types.CommitmentRecord {
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: recordID, Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: objective},
		Cast:      &types.CommitmentCast{}, Inbox: managementInbox, SenderInbox: textureInbox, Addressee: managementInbox,
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{AgentID: textureInbox},
	}
}

// EngineeringCastRecord is management's Cast that opens one assignment's
// engineering conversation (C1e D4): addressed to engineering:<assignment>,
// naming the Texture's Cast it serves as its parent.
func EngineeringCastRecord(recordID, managementInbox, assignmentID, parentRecordID, objective string) types.CommitmentRecord {
	inbox := "engineering:" + assignmentID
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: recordID, Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: objective},
		Cast:      &types.CommitmentCast{}, Inbox: inbox, SenderInbox: managementInbox, Addressee: inbox,
		ParentID: parentRecordID, EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{AgentID: managementInbox},
	}
}

// SeedEngineeringOpenRequest builds the records open of seed assignment
// index: management's Cast record composed into the open, the binding the
// runtime would derive, and the capability digest of capability.
func SeedEngineeringOpenRequest(f EngineeringAssignmentSeed, index int, capability string) types.OpenEngineeringAssignmentRequest {
	assignmentID := f.AssignmentIDs[index]
	castID := "management-cell:Cast:" + assignmentID
	subject := objectgraph.SHA256([]byte("subject:" + assignmentID))
	binding := types.EngineeringAssignmentBinding{
		OwnerID: f.OwnerID, ComputerID: f.ComputerID, TrajectoryID: f.TrajectoryID,
		ParentAgentID: f.ParentAgentID, ParentRunID: f.ParentRunID,
		ParentDecisionID: "decision:" + objectgraph.SHA256([]byte("decision:"+assignmentID)), ParentControlID: castID,
		AssignedAgentID: f.AssignedAgentIDs[index],
		Kind:            types.EngineeringAssignmentImplementation, Attempt: 1,
		ScopeDigest: objectgraph.SHA256([]byte("scope:" + assignmentID)), RequestDigest: objectgraph.SHA256([]byte("request:" + assignmentID)),
		CapabilityDigest: DigestEngineeringOpaqueCapability(capability), ExecutionHandleDigest: objectgraph.SHA256([]byte(capability)),
		SubjectDigest: subject, SourceArtifactRef: "capsule-source-git:commit:" + subject,
		Writable: true, CapsuleID: "capsule-" + assignmentID,
		NetworkMode:          types.EngineeringCapsuleNetworkForbidden,
		FilesystemMode:       types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
		CastAuthority:        types.EngineeringCastAuthorityRecords,
		ParentConversationID: f.ParentConversationID, ParentRound: f.ParentRound,
	}
	binding.AssignedConversationID = types.InboxConversationID(binding.ParentAgentID, binding.AssignedAgentID)
	records := types.CommitInboxRecordsRequest{OwnerID: f.OwnerID, ComputerID: f.ComputerID, CommandID: "cell-records:management-cell-" + assignmentID,
		Records: []types.CommitmentRecord{EngineeringCastRecord(castID, f.ParentAgentID, assignmentID, f.TextureCastID, "bounded delegated assignment")}}
	records.CommandDigest, _ = ComputeCommitInboxRecordsDigest(records)
	req := types.OpenEngineeringAssignmentRequest{
		CommandID: "command-open-" + assignmentID, AssignmentID: assignmentID, Binding: binding,
		AssignedAgent: types.AgentRecord{AgentID: binding.AssignedAgentID}, Records: records,
	}
	req.CommandDigest, _ = ComputeOpenEngineeringAssignmentDigest(req)
	return req
}

// SeedEngineeringRun is the pending engineering run a records binding
// accepts at bind (validateEngineeringAssignmentRun).
func SeedEngineeringRun(open types.OpenEngineeringAssignmentRequest, runID string) types.RunRecord {
	b := open.Binding
	return types.RunRecord{
		RunID: runID, AgentID: b.AssignedAgentID, ChannelID: b.AssignedAgentID,
		RequestedByRunID: b.ParentRunID, TrajectoryID: b.TrajectoryID,
		AgentProfile: "engineering", AgentRole: "engineering", OwnerID: b.OwnerID, ComputerID: b.ComputerID,
		State: types.RunPending, Prompt: "bounded delegated assignment",
		Metadata: map[string]any{
			"requested_by_agent_id": b.ParentAgentID, "requested_by_profile": "management",
			"assignment_id": open.AssignmentID, "assignment_attempt": b.Attempt, "assignment_kind": string(b.Kind),
			"assigned_conversation_id": b.AssignedConversationID, "parent_conversation_id": b.ParentConversationID, "parent_round": b.ParentRound,
			"parent_decision_id": b.ParentDecisionID, "parent_control_id": b.ParentControlID,
			"capsule_id": b.CapsuleID, "scope_digest": b.ScopeDigest, "request_digest": b.RequestDigest,
			"capability_digest": b.CapabilityDigest, "execution_handle_digest": b.ExecutionHandleDigest,
			"subject_digest": b.SubjectDigest, "source_artifact_ref": b.SourceArtifactRef, "source_candidate_id": b.SourceCandidateID,
		},
	}
}

// SeedEngineeringBindRequest binds a seed open to runID.
func SeedEngineeringBindRequest(open types.OpenEngineeringAssignmentRequest, runID, capability string) types.BindEngineeringAssignmentRequest {
	req := types.BindEngineeringAssignmentRequest{
		CommandID: "command-bind-" + open.AssignmentID,
		OwnerID:   open.Binding.OwnerID, ComputerID: open.Binding.ComputerID, AssignmentID: open.AssignmentID,
		Attempt: open.Binding.Attempt, ExpectedLifecycleVersion: 1, RunID: runID, Run: SeedEngineeringRun(open, runID),
		OpaqueCapability: capability, CapsuleID: open.Binding.CapsuleID,
	}
	req.CommandDigest, _ = ComputeBindEngineeringAssignmentDigest(req)
	return req
}
