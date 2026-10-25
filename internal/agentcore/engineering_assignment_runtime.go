package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/vocabmigrate"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

const (
	// engineeringAssignmentMemoryMax holds the cmd/autoputer link (measured
	// peak 1.50 GiB stripped, 1.80 GiB with DWARF; the 1 GiB budget made
	// the drill's link thrash for 14+ minutes) and fits the default 4 GiB
	// guest's admission budget (3/4 of MemTotal) with one engineering
	// capsule live at a time
	// (docs/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md, B5).
	engineeringAssignmentMemoryMax = int64(2 << 30)
	// engineeringAssignmentMemoryRequest is the capsule's doctrine request
	// (RES-04, internal/capsule/capsule_memory.go): memory.high under the
	// doctrine policy, so it holds the DWARF link peak; admission is the
	// same 2 GiB as before, and the ceiling becomes twice it.
	engineeringAssignmentMemoryRequest = engineeringAssignmentMemoryMax
	engineeringAssignmentCPUQuota      = int64(100000)
	engineeringAssignmentPidsMax       = int64(256)
	// coManagementAssignmentDeadline is the fail-closed scheduling deadline for one
	// live assignment (I26): a bound assignment that has not reached a terminal
	// disposition by this bound is cancelled with an expired reason. The
	// underlying execution request stays pending and retryable — the scheduler
	// may re-admit it later; expiry fails the assignment, never the work.
	engineeringAssignmentDeadline = 6 * time.Hour
	// assignedEngineeringFateWatchdogDelay is the fate-transition watchdog delay:
	// after a committed disposition strands the terminal saga, the watchdog
	// re-drives the continuation without waiting for a worker cell or a
	// restart. Longer than an in-flight commit needs, shorter than the
	// deadline backstop.
	assignedEngineeringFateWatchdogDelay = 5 * time.Minute
	// engineeringActivationGrace bounds the bind→activate window: a bound run
	// that stays pending past the grace never got its dispatch (crash between
	// Bind and activate, or a lost mailbox write), so the assignment's capsule
	// reads dead and the trajectory reconcile repairs instead of waiting for
	// the six-hour deadline.
	engineeringActivationGrace = 5 * time.Minute
)

// assignedEngineeringDeadline returns the I26 fail-closed bound-assignment
// deadline. CHOIR_ASSIGNMENT_DEADLINE (a Go duration) overrides the default for
// staging probes and focused testing; it must be constant for a process's
// lifetime so the arm-time not-before and the fire-time expiry check agree.
func assignedEngineeringDeadline() time.Duration {
	if v := strings.TrimSpace(os.Getenv("CHOIR_ASSIGNMENT_DEADLINE")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return engineeringAssignmentDeadline
}

type assignmentCapsuleRuntime interface {
	Spawn(context.Context, capsule.SpawnSpec) (*capsule.Capsule, error)
	MintCapabilityHandle(string, capsule.AgentRole, string, string, time.Duration, string) (*capsule.Capability, error)
	RevokeCapability(string, string) error
	ForceDestroy(context.Context, string) error
	ExtractGranted(context.Context, string, string) ([]capsule.FileChange, error)
	ResolveGrantedWorktreeDigest(context.Context, string, string) (string, error)
	ResolveExecutionReceipts([]string) ([]capsule.ExecutionReceipt, error)
	AssignmentHandle(string, string) (string, error)
	InspectCapsuleRaw(string) (*capsule.CapsuleDiagnostics, error)
	HasCapsule(string) bool
	CleanupOrphanedCapsule(context.Context, string) error
	PersistRevocationReceipt(string, string, string, string) (capsule.CapsuleRevocationReceipt, error)
}

type AssignedEngineeringStart struct {
	Assignment types.EngineeringAssignment
	Run        types.RunRecord
	Replay     bool
}

// overlayIDNamedInObjective reports the model_policy_overlay_id value assigned
// in prose inside an objective (model_policy_overlay_id=<id>), or "" when the
// objective names none. A bare mention without an assignment is not a naming.
// The id charset mirrors the overlay loader ([A-Za-z0-9_-]); a malformed
// assignment is skipped in favour of a later well-formed one.
func overlayIDNamedInObjective(objective string) string {
	const key = "model_policy_overlay_id"
	named := ""
	rest := objective
	for {
		i := strings.Index(rest, key)
		if i < 0 {
			return named
		}
		after := strings.TrimSpace(rest[i+len(key):])
		rest = after
		if !strings.HasPrefix(after, "=") {
			continue
		}
		after = strings.TrimSpace(strings.TrimPrefix(after, "="))
		after = strings.Trim(after, "\"'")
		end := strings.IndexFunc(after, func(r rune) bool {
			return !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-')
		})
		var id string
		if end < 0 {
			id = after
		} else {
			id = after[:end]
		}
		if id != "" && len(id) <= 96 {
			named = id
		}
	}
}

// deterministicDocumentAssignmentCapability derives the opaque capability for
// an assignment. Determinism is required for resume: an open but unbound
// assignment must re-derive the exact capability its binding digest
// committed, or the bind receipt can never be replayed after a crash between
// open and bind.
func deterministicDocumentAssignmentCapability(assignmentID string, attempt uint64) string {
	return "assignment-cap-" + strings.TrimPrefix(objectgraph.SHA256([]byte(assignmentID+"\x00cap\x00"+fmt.Sprint(attempt))), "sha256:")
}

// assignmentSpawnRequest is what the spawn saga needs beyond the committed
// binding: the objective the run is prompted with and an optional model
// policy overlay.
type assignmentSpawnRequest struct {
	Objective            string
	ModelPolicyOverlayID string
}

// engineeringCastSpec is the spec management passes with a Cast to
// engineering (C1e D4): parent names the record id of the Texture's Cast the
// assignment serves (as choir.Records() shows it); network "forbidden"
// withholds the L2 ecosystem egress.
type engineeringCastSpec struct {
	Parent string `json:"parent"`
}

// engineeringCastPlan is a management cell's Cast to engineering, admitted
// before anything commits (C1e D4): the assignment id, the binding and the
// Cast record addressed to engineering:<assignmentID>.
type engineeringCastPlan struct {
	AssignmentID string
	Binding      types.EngineeringAssignmentBinding
	Objective    string
	Cast         types.CommitmentRecord
}

// planEngineeringCast runs D4's checks for one staged Cast to engineering
// from a management records turn, before anything commits: the Cast names
// its parent, the Texture Cast that opened management's current round with
// texture:<doc>; that round is open; the document names the trajectory; and
// the capsule source preflights. The assignment id derives from the Cast's
// pre-commit record id, the owner, the computer and the trajectory, so the
// Cast is addressed to engineering:<assignmentID> when it is minted. A
// failure refuses the cell and nothing is written.
func (rt *Runtime) planEngineeringCast(ctx context.Context, scope ReductionScope, caster *types.RunRecord, in yaegikernel.StagedIntent) (engineeringCastPlan, error) {
	if rt == nil || rt.store == nil || caster == nil {
		return engineeringCastPlan{}, fmt.Errorf("engineering cast: runtime authority unavailable")
	}
	if to := strings.TrimSpace(in.ToDesk); to != agentprofile.Engineering {
		return engineeringCastPlan{}, fmt.Errorf("engineering cast: address \"engineering\"; each Cast opens a new assignment with its own conversation (use Note to speak into an open one), not %q", to)
	}
	objective := strings.TrimSpace(in.Objective)
	if objective == "" {
		return engineeringCastPlan{}, fmt.Errorf("engineering cast: an objective is required")
	}
	var spec engineeringCastSpec
	if raw := strings.TrimSpace(in.Statement); raw != "" && raw != "null" {
		if err := json.Unmarshal([]byte(raw), &spec); err != nil {
			return engineeringCastPlan{}, fmt.Errorf("engineering cast: spec is not JSON: %w", err)
		}
	}
	parentID := strings.TrimSpace(spec.Parent)
	if parentID == "" {
		return engineeringCastPlan{}, fmt.Errorf("engineering cast: name the Texture request it serves: pass spec {\"parent\": r.RecordID} with the record id of that Texture's Cast from choir.Records()")
	}
	ownerID, computerID := strings.TrimSpace(scope.OwnerID), strings.TrimSpace(scope.ComputerID)
	management := strings.TrimSpace(scope.DeskAgentID)
	parent, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, parentID)
	if err != nil {
		return engineeringCastPlan{}, fmt.Errorf("engineering cast: parent %s: %w", parentID, err)
	}
	if !isTextureCastTo(parent, management) {
		return engineeringCastPlan{}, fmt.Errorf("engineering cast: parent %s is not a Texture's Cast in %s", parentID, management)
	}
	head, err := rt.store.ConversationState(ctx, ownerID, computerID, parent.ConversationID)
	if err != nil {
		return engineeringCastPlan{}, fmt.Errorf("engineering cast: conversation %s: %w", parent.ConversationID, err)
	}
	if !head.Open || head.Round != parent.Round {
		return engineeringCastPlan{}, fmt.Errorf("engineering cast: the round %s opened (round %d of %s) is not open", parentID, parent.Round, parent.ConversationID)
	}
	// R5-2 (docs/evidence/selfdev-trace-reviews-2026-10-25.md round 5): a
	// request whose assignment is still open gets no second one. A Texture
	// re-cast into the same round (a correction, "continue the repair") is
	// for the bound engineer, as a Note (P1); a second assignment would
	// start from the base beside it.
	if err := rt.refuseCastOntoOpenAssignment(ctx, ownerID, computerID, parent.ConversationID, parent.Round); err != nil {
		return engineeringCastPlan{}, err
	}
	docID := strings.TrimPrefix(parent.SenderInbox, agentprofile.Texture+":")
	document, err := rt.store.GetLifecycleDocument(ctx, ownerID, computerID, docID)
	if err != nil {
		return engineeringCastPlan{}, fmt.Errorf("engineering cast: document %s: %w", docID, err)
	}
	trajectoryID := strings.TrimSpace(document.TrajectoryID)
	if trajectoryID == "" {
		return engineeringCastPlan{}, fmt.Errorf("engineering cast: document %s names no trajectory", docID)
	}
	if rt.capsuleExecutor == nil {
		return engineeringCastPlan{}, fmt.Errorf("engineering cast: capsule authority unavailable")
	}
	recordID := commitmentRecordForIntent(scope, in).RecordID
	assignmentID := "assignment-" + computerevent.DigestBytes([]byte(strings.Join([]string{
		vocabmigrate.IdentitySeedDelegatedCastRequestV1, ownerID, computerID, trajectoryID, recordID,
	}, "\x00")))[:24]
	preflightStarted := time.Now()
	// U1: the capsule seeds from the repository's HEAD, so HEAD must be the
	// effective release's source before it is read (and a stack on a moved
	// base refuses here, visibly, rather than building on the old base).
	if err := rt.sourceProjectionCastGate(ctx); err != nil {
		return engineeringCastPlan{}, fmt.Errorf("engineering cast: %w", err)
	}
	preflight, err := rt.capsuleExecutor.PreflightSourceSnapshot(ctx, "")
	if err != nil {
		return engineeringCastPlan{}, fmt.Errorf("engineering cast: preflight capsule source: %w", err)
	}
	preflightTook := time.Since(preflightStarted)
	subjectDigest := "sha256:" + strings.TrimPrefix(preflight.SubjectDigest, "sha256:")
	const attempt = uint64(1)
	opaque := deterministicDocumentAssignmentCapability(assignmentID, attempt)
	parentDecisionID := "decision:" + objectgraph.SHA256([]byte(strings.Join([]string{
		vocabmigrate.IdentitySeedDelegatedDecisionV1, ownerID, computerID, management, trajectoryID, parentID, recordID, caster.RunID,
	}, "\x00")))
	scopeBytes, err := json.Marshal(struct {
		Intent string `json:"intent"`
		Run    string `json:"run"`
		Parent string `json:"parent"`
		Seed   string `json:"seed"`
	}{objective, caster.RunID, parentID, scope.CellID + ":" + in.LocalID})
	if err != nil {
		return engineeringCastPlan{}, err
	}
	assignedAgentID := agentprofile.Engineering + ":" + assignmentID
	binding := types.EngineeringAssignmentBinding{
		OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
		ParentAgentID: management, ParentRunID: caster.RunID, ParentDecisionID: parentDecisionID,
		ParentControlID: recordID, AssignedAgentID: assignedAgentID,
		Kind: types.EngineeringAssignmentImplementation, Attempt: attempt,
		ScopeDigest: objectgraph.SHA256(scopeBytes),
		// The parent Texture Cast's full text is bound in, so engineering's
		// view of the request (engineeringRequestView) is checked against
		// what this Cast committed to.
		RequestDigest: engineeringRequestDigest(objective, parentID, recordID,
			parentRequestDigest(parent.SenderInbox, parent.Directive.Objective, parent.Directive.Body)),
		CapabilityDigest:      store.DigestEngineeringOpaqueCapability(opaque),
		ExecutionHandleDigest: objectgraph.SHA256([]byte(opaque)), SubjectDigest: subjectDigest,
		SourceArtifactRef: preflight.ArtifactRef,
		Writable:          true, CapsuleID: "capsule-" + strings.TrimPrefix(uuid.NewSHA1(uuid.NameSpaceOID, []byte(assignmentID+"\x00"+fmt.Sprint(attempt))).String(), "-"),
		NetworkMode:          engineeringCastNetwork(in.Statement, rt.capsuleExecutor.ClaudeDelegateStatus().Available),
		FilesystemMode:       types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
		CastAuthority:        types.EngineeringCastAuthorityRecords,
		ParentConversationID: parent.ConversationID, ParentRound: parent.Round,
		AssignedConversationID: types.InboxConversationID(management, assignedAgentID),
	}
	if err := binding.Validate(); err != nil {
		return engineeringCastPlan{}, fmt.Errorf("engineering cast: %w", err)
	}
	// The cast's preflight runs inside the casting activation.
	logAssignmentSagaPhase(binding.CapsuleID, assignmentID, "cast_preflight", time.Now().Add(-preflightTook))
	cast, err := inboxRecordForIntent(scope, in, assignedAgentID, false)
	if err != nil {
		return engineeringCastPlan{}, err
	}
	cast.ParentID = parentID
	return engineeringCastPlan{AssignmentID: assignmentID, Binding: binding, Objective: objective, Cast: cast}, nil
}

// refuseCastOntoOpenAssignment refuses an engineering Cast for a Texture
// round that already has a non-terminal assignment, and names the bound
// engineer's inbox to Note instead.
func (rt *Runtime) refuseCastOntoOpenAssignment(ctx context.Context, ownerID, computerID, conversation string, round int) error {
	children, err := rt.store.ListEngineeringAssignmentsByParentRound(ctx, ownerID, computerID, conversation, round)
	if err != nil {
		return fmt.Errorf("engineering cast: assignments of round %d of %s: %w", round, conversation, err)
	}
	for _, child := range children {
		if child.Disposition.Terminal() {
			continue
		}
		inbox := strings.TrimSpace(child.Binding.AssignedAgentID)
		return fmt.Errorf("engineering cast: assignment %s for this request is still %s, so no second assignment opens; send the change to its engineer with choir.Note(%q, <the change in full>) and wait for its final Report before casting again", child.AssignmentID, child.Disposition, inbox)
	}
	return nil
}

// openDelegatedCastAssignment is the one mint of an engineering assignment
// (C1e D1): it commits the casting cell's whole inbox command (records,
// cursor, verdicts) in the same transaction as the assignment row and its
// binding (D4). The spawn/bind/activate saga is not driven here: the open
// commit's actor-wake outbox carries the spawn obligation
// (delegated_assignment_spawn_deadline). A replayed cell replays the open.
func (rt *Runtime) openDelegatedCastAssignment(ctx context.Context, plan engineeringCastPlan, records types.CommitInboxRecordsRequest) (types.EngineeringAssignmentCommandResult, error) {
	open := types.OpenEngineeringAssignmentRequest{
		CommandID: "engineering-open:" + plan.AssignmentID, AssignmentID: plan.AssignmentID, Binding: plan.Binding,
		AssignedAgent: types.AgentRecord{AgentID: plan.Binding.AssignedAgentID}, Records: records,
	}
	var err error
	if open.CommandDigest, err = store.ComputeOpenEngineeringAssignmentDigest(open); err != nil {
		return types.EngineeringAssignmentCommandResult{}, err
	}
	return rt.store.OpenEngineeringAssignment(ctx, open)
}

// resumeDelegatedCastAssignment re-drives the spawn/bind saga for an
// assignment whose durable open committed but whose bind never landed.
// Caller MUST hold engineeringAssignmentOpenMu and pass a freshly re-read
// assignment. The deterministic capability re-derives the exact digest the
// binding committed.
func (rt *Runtime) resumeDelegatedCastAssignment(ctx context.Context, assignment types.EngineeringAssignment, req assignmentSpawnRequest) (AssignedEngineeringStart, error) {
	if rt == nil || rt.store == nil {
		return AssignedEngineeringStart{}, fmt.Errorf("assigned Engineering runtime unavailable")
	}
	// Admission comes first, before the capsule authority, the preflight and
	// anything that could cancel: a request that has to wait stays open and
	// unbound, and the live assignment is never touched. It reads only the
	// assignment rows. Expiry comes before admission, whatever the slot.
	phaseStarted := time.Now()
	if err := rt.expireOverdueEngineeringRequests(ctx, assignment); err != nil {
		return AssignedEngineeringStart{}, err
	}
	if err := rt.admitEngineeringAssignment(ctx, assignment); err != nil {
		return AssignedEngineeringStart{}, err
	}
	phaseStarted = logAssignmentSagaPhase(assignment.Binding.CapsuleID, assignment.AssignmentID, "admission", phaseStarted)
	if rt.capsuleExecutor == nil {
		return AssignedEngineeringStart{}, fmt.Errorf("assigned Engineering capsule authority unavailable")
	}
	preflight, err := rt.capsuleExecutor.PreflightSourceSnapshot(ctx, assignment.Binding.SourceArtifactRef)
	if err != nil {
		return AssignedEngineeringStart{}, fmt.Errorf("preflight resumed assignment subject: %w", err)
	}
	logAssignmentSagaPhase(assignment.Binding.CapsuleID, assignment.AssignmentID, "resume_preflight", phaseStarted)
	if preflight.ArtifactRef != assignment.Binding.SourceArtifactRef ||
		"sha256:"+strings.TrimPrefix(preflight.SubjectDigest, "sha256:") != assignment.Binding.SubjectDigest {
		return AssignedEngineeringStart{}, fmt.Errorf("resumed assignment subject drifted from the committed binding")
	}
	opaque := deterministicDocumentAssignmentCapability(assignment.AssignmentID, assignment.Binding.Attempt)
	if store.DigestEngineeringOpaqueCapability(opaque) != assignment.Binding.CapabilityDigest {
		return AssignedEngineeringStart{}, fmt.Errorf("resumed assignment capability does not match the committed binding")
	}
	return rt.spawnBindActivateAssignment(ctx, assignment, preflight, opaque, req)
}

// assignmentObjective is the committed objective of a records assignment:
// the objective on management's Cast that opened it (C1e D1).
func (rt *Runtime) assignmentObjective(ctx context.Context, assignment types.EngineeringAssignment) (string, error) {
	cast, err := rt.store.GetCommitmentRecord(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.ParentControlID)
	if err != nil {
		return "", fmt.Errorf("load the Cast that opened %s: %w", assignment.AssignmentID, err)
	}
	if cast == nil || cast.Directive == nil || strings.TrimSpace(cast.Directive.Objective) == "" {
		return "", fmt.Errorf("the Cast that opened %s carries no objective", assignment.AssignmentID)
	}
	return strings.TrimSpace(cast.Directive.Objective), nil
}

// buildAssignedEngineeringRun builds the engineering run a records
// assignment binds (C1e): its parent fields come from the binding, it names
// its engineering conversation and the parent round it serves, and it
// carries no work items.
func buildAssignedEngineeringRun(assignment types.EngineeringAssignment, runID, objective, sourceArtifactRef string) types.RunRecord {
	binding := assignment.Binding
	ownerID, computerID, trajectoryID := binding.OwnerID, binding.ComputerID, binding.TrajectoryID
	assignmentID, attempt := assignment.AssignmentID, binding.Attempt
	agentID, capsuleID := binding.AssignedAgentID, binding.CapsuleID
	return types.RunRecord{
		RunID: runID, AgentID: agentID, ChannelID: agentID, RequestedByRunID: binding.ParentRunID, TrajectoryID: trajectoryID,
		AgentProfile: agentprofile.Engineering, AgentRole: agentprofile.Engineering, OwnerID: ownerID, ComputerID: computerID,
		State: types.RunPending, Prompt: objective,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Engineering, runMetadataAgentRole: agentprofile.Engineering, runMetadataAgentID: agentID,
			runMetadataTrajectoryID: trajectoryID,
			"requested_by_agent_id": binding.ParentAgentID, "requested_by_profile": store.EngineeringParentProfileForBinding(binding),
			"assignment_id": assignmentID, "assignment_attempt": attempt, "assignment_kind": string(binding.Kind),
			runMetadataEngineeringSlot: "implementation",
			"assigned_conversation_id": binding.AssignedConversationID, "parent_conversation_id": binding.ParentConversationID,
			"parent_round": binding.ParentRound, "capsule_id": capsuleID, "capsule_network_mode": binding.NetworkMode,
			"parent_decision_id": binding.ParentDecisionID, "parent_control_id": binding.ParentControlID,
			"scope_digest": binding.ScopeDigest, "request_digest": binding.RequestDigest,
			"capability_digest": binding.CapabilityDigest, "execution_handle_digest": binding.ExecutionHandleDigest, "subject_digest": binding.SubjectDigest,
			"source_artifact_ref": sourceArtifactRef, "source_candidate_id": binding.SourceCandidateID,
		},
	}
}

// engineeringCapsuleNetwork picks a new assignment's network mode. Owner
// direction (2026-10-10, "don't hinder agency"): implementation capsules get
// L2 ecosystem egress through the executor's proxy unless the caster
// withheld it; verification stays offline because the verifier rebuilds from
// the frozen bundle (engineering-network-grants-2026-10-10.md §4.0b).
func engineeringCapsuleNetwork(kind types.EngineeringAssignmentKind, withheld bool) string {
	if kind == types.EngineeringAssignmentImplementation && !withheld {
		return types.EngineeringCapsuleNetworkEcosystemProxy
	}
	return types.EngineeringCapsuleNetworkForbidden
}

// engineeringCastNetwork is an implementation Cast's network mode. On the
// computer whose Claude delegate is available it is the claude mode (model
// API and Go module proxy only, docs/design/claude-delegate-slice1-2026-10-14.md)
// unless the Cast withholds the network or asks for "ecosystem_proxy"
// explicitly; everywhere else it is engineeringCapsuleNetwork's choice.
func engineeringCastNetwork(spec string, delegateAvailable bool) string {
	mode := engineeringCapsuleNetwork(types.EngineeringAssignmentImplementation, castWithholdsNetwork(spec))
	if mode == types.EngineeringCapsuleNetworkEcosystemProxy && delegateAvailable && !castAsksForEcosystem(spec) {
		return types.EngineeringCapsuleNetworkClaude
	}
	return mode
}

// castAsksForEcosystem reads an explicit "network": "ecosystem_proxy" (or
// "l2"): the Cast wants npm, GitHub or PyPI, which the claude mode lacks.
func castAsksForEcosystem(spec string) bool {
	var fields map[string]any
	if json.Unmarshal([]byte(spec), &fields) != nil {
		return false
	}
	value, _ := fields["network"].(string)
	switch strings.ToLower(strings.TrimSpace(value)) {
	case types.EngineeringCapsuleNetworkEcosystemProxy, "l2":
		return true
	}
	return false
}

// castWithholdsNetwork reads a Cast spec's "network" field. Only an explicit
// "forbidden", "none" or "off" (or false) withholds; anything else keeps the
// default, and a spec can never widen past L2.
func castWithholdsNetwork(spec string) bool {
	var fields map[string]any
	if json.Unmarshal([]byte(spec), &fields) != nil {
		return false
	}
	switch value := fields["network"].(type) {
	case bool:
		return !value
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case types.EngineeringCapsuleNetworkForbidden, types.EngineeringCapsuleNetworkNone, "off":
			return true
		}
	}
	return false
}

// spawnBindActivateAssignment is the shared post-open saga: spawn the capsule,
// mint the capability, bind the run, wake the actor. Every failure path cancels
// the durable open so a stranded assignment never blocks a later cast.
func (rt *Runtime) spawnBindActivateAssignment(ctx context.Context, assignment types.EngineeringAssignment, preflight capsule.SourcePreflight, opaque string, req assignmentSpawnRequest) (AssignedEngineeringStart, error) {
	binding := assignment.Binding
	ownerID, computerID := binding.OwnerID, binding.ComputerID
	assignmentID, attempt := assignment.AssignmentID, binding.Attempt
	capsuleID := binding.CapsuleID
	runID := "run:" + assignmentID
	cancelOpen := func(cause error) error {
		current, loadErr := rt.store.GetEngineeringAssignment(context.Background(), ownerID, computerID, assignmentID, attempt)
		if loadErr == nil && !current.Disposition.Terminal() {
			cancel := types.CancelEngineeringAssignmentRequest{CommandID: fmt.Sprintf("engineering-open-failed:%s:%d", assignmentID, attempt), OwnerID: ownerID, ComputerID: computerID,
				AssignmentID: assignmentID, Attempt: attempt, ExpectedLifecycleVersion: current.LifecycleVersion, Reason: cause.Error()}
			cancel.CommandDigest, _ = store.ComputeCancelEngineeringAssignmentDigest(cancel)
			if _, cancelErr := rt.store.CancelEngineeringAssignment(context.Background(), cancel); cancelErr != nil && (errors.Is(cancelErr, store.ErrLifecycleCommandConflict) || errors.Is(cancelErr, store.ErrEngineeringAssignmentInvalid)) {
				// A concurrent reconciler already cancelled or advanced the
				// assignment; terminal/adopted state is convergence.
				if reloaded, reloadErr := rt.store.GetEngineeringAssignment(context.Background(), ownerID, computerID, assignmentID, attempt); reloadErr == nil && reloaded.Disposition.Terminal() {
					return cause
				}
				return fmt.Errorf("%w (cancel stranded open: %v)", cause, cancelErr)
			}
		}
		return cause
	}
	phaseStarted := time.Now()
	spawnCtx, cancelSpawn := context.WithTimeout(ctx, 90*time.Second)
	defer cancelSpawn()
	memory, cpu, pids := engineeringAssignmentMemoryMax, engineeringAssignmentCPUQuota, engineeringAssignmentPidsMax
	claude := binding.NetworkMode == types.EngineeringCapsuleNetworkClaude
	if claude {
		// Claude Code beside go test (docs/design/claude-delegate-slice1-2026-10-14.md §3).
		memory, cpu, pids = capsule.ClaudeCapsuleSize(rt.capsuleExecutor.MemoryBudget(), memory)
	}
	// The request is the size chosen above, a Claude capsule's included:
	// doctrine's memory.high never sits below what the capsule used to get.
	spec := capsule.SpawnSpec{CapsuleID: capsuleID, OwnerRunID: runID,
		MemoryMax: memory, MemoryRequest: memory, CpuQuota: cpu, CpuPeriod: 100000, PidsMax: pids,
		WorkingDir: "/workspace/platform", Tier: capsule.TierMedium,
		SourceArtifactRef: preflight.ArtifactRef, ExpectedSubjectDigest: preflight.SubjectDigest,
		Egress:         binding.NetworkMode == types.EngineeringCapsuleNetworkEcosystemProxy || claude,
		ClaudeDelegate: claude}
	// P8: the engineering assignment on a release adoption's trajectory
	// gets the verified patch at /selfdev/adoption, re-verified now (A8);
	// any other trajectory gets nothing (A9). Bytes that changed since
	// verification refuse the spawn.
	adoptionDir, adoptionErr := rt.adoptionInputDir(ctx, ownerID, computerID, binding.TrajectoryID)
	if adoptionErr != nil {
		return AssignedEngineeringStart{}, cancelOpen(fmt.Errorf("adoption input: %w", adoptionErr))
	}
	spec.AdoptionDir = adoptionDir
	phaseStarted = logAssignmentSagaPhase(capsuleID, assignmentID, "adoption_input", phaseStarted)
	// Option A (failure 15, problem 1): an earlier failed round's work on this
	// trajectory is offered as an inert patch the engineer may apply in this
	// fresh capsule. Never fatal: a refusal is recorded and the capsule starts
	// from the clean base tree.
	spec.CarriedPatchDir = carriedPatchDirForSpawn(rt.offerCarriedPatchForAssignment(ctx, assignment, preflight.ArtifactRef))
	phaseStarted = logAssignmentSagaPhase(capsuleID, assignmentID, "carried_patch", phaseStarted)
	// One live assignment capsule at a time: prior capsules are reclaimed
	// here, in the saga, so the open itself writes nothing but its own
	// transaction (C1e D4).
	if err := rt.reclaimTerminalAssignmentCapsules(ctx, types.RunRecord{OwnerID: ownerID, ComputerID: computerID}, assignmentID); err != nil {
		if errors.Is(err, ErrEngineeringAdmissionBusy) {
			// Defense in depth: admission already ran under the same mutex. A
			// request that has to wait is never cancelled here.
			return AssignedEngineeringStart{}, err
		}
		return AssignedEngineeringStart{}, cancelOpen(fmt.Errorf("reclaim terminal capsules: %w", err))
	}
	phaseStarted = logAssignmentSagaPhase(capsuleID, assignmentID, "reclaim", phaseStarted)
	var created *capsule.Capsule
	if rt.capsuleExecutor.HasCapsule(capsuleID) {
		// Resume after a crash between spawn and bind: the capsule already
		// exists; verify it is still active rather than respawning.
		diagnostics, diagErr := rt.capsuleExecutor.InspectCapsuleRaw(capsuleID)
		if diagErr != nil || diagnostics == nil {
			return AssignedEngineeringStart{}, cancelOpen(fmt.Errorf("resumed assignment capsule is not inspectable: %v", diagErr))
		}
		created = &capsule.Capsule{ID: capsuleID, State: capsule.StateActive}
	} else {
		var spawnErr error
		created, spawnErr = rt.capsuleExecutor.Spawn(spawnCtx, spec)
		if spawnErr != nil {
			return AssignedEngineeringStart{}, cancelOpen(fmt.Errorf("spawn assigned capsule after durable open: %w", spawnErr))
		}
	}
	cleanupCapsule := func(cause error) error {
		current, loadErr := rt.store.GetEngineeringAssignment(context.Background(), ownerID, computerID, assignmentID, attempt)
		if loadErr != nil {
			return fmt.Errorf("%w (load opened assignment for capsule cleanup: %v)", cause, loadErr)
		}
		intent := "capsule-revoke-intent:" + objectgraph.SHA256([]byte(current.AssignmentID+"\x00pre-bind\x00"+cause.Error()))
		requested := current
		if current.CapsuleDisposition != types.EngineeringCapsuleRevokeRequested && current.CapsuleDisposition != types.EngineeringCapsuleRevoked {
			var fateErr error
			result, fateErr := rt.store.SetEngineeringCapsuleDisposition(context.Background(), engineeringFateRequest(current, types.EngineeringCapsuleRevokeRequested, intent, ""))
			if fateErr != nil {
				if !errors.Is(fateErr, store.ErrLifecycleCommandConflict) && !errors.Is(fateErr, store.ErrEngineeringAssignmentInvalid) {
					return fmt.Errorf("%w (persist pre-bind capsule revoke intent: %v)", cause, fateErr)
				}
				// A competing cleanup already recorded a revoke intent; the
				// recorded intent is the durable authority — adopt it.
				reloaded, reloadErr := rt.store.GetEngineeringAssignment(context.Background(), ownerID, computerID, assignmentID, attempt)
				if reloadErr != nil || reloaded.CapsuleDisposition != types.EngineeringCapsuleRevokeRequested && reloaded.CapsuleDisposition != types.EngineeringCapsuleRevoked {
					return fmt.Errorf("%w (persist pre-bind capsule revoke intent: %v)", cause, fateErr)
				}
				requested = reloaded
				intent = strings.TrimSpace(reloaded.CapsuleIntentRef)
			} else {
				requested = result.Assignment
			}
		} else if existing := strings.TrimSpace(current.CapsuleIntentRef); existing != "" {
			intent = existing
		}
		_ = rt.capsuleExecutor.RevokeCapability(runID, opaque)
		if rt.capsuleExecutor.HasCapsule(capsuleID) {
			if destroyErr := rt.capsuleExecutor.ForceDestroy(context.Background(), capsuleID); destroyErr != nil {
				return fmt.Errorf("%w (destroy after pre-bind revoke intent: %v)", cause, destroyErr)
			}
		}
		if rt.capsuleExecutor.HasCapsule(capsuleID) {
			return fmt.Errorf("%w (pre-bind capsule continued after executor acknowledgement)", cause)
		}
		acked := requested
		if requested.CapsuleDisposition != types.EngineeringCapsuleRevoked {
			receipt, receiptErr := rt.capsuleExecutor.PersistRevocationReceipt(runID, requested.Binding.CapabilityDigest, capsuleID, intent)
			if receiptErr != nil {
				return fmt.Errorf("%w (persist structured pre-bind revoke acknowledgement: %v)", cause, receiptErr)
			}
			fateAck, fateAckErr := engineeringFateAckRequest(requested, types.EngineeringCapsuleRevoked, intent, receipt.ReceiptRef, "", "", receipt.OccurredAt, receipt.CapsuleAbsent)
			if fateAckErr != nil {
				return fmt.Errorf("%w (invalid revoke receipt occurred_at: %v)", cause, fateAckErr)
			}
			result, fateErr := rt.store.SetEngineeringCapsuleDisposition(context.Background(), fateAck)
			if fateErr != nil {
				if !errors.Is(fateErr, store.ErrLifecycleCommandConflict) && !errors.Is(fateErr, store.ErrEngineeringAssignmentInvalid) {
					return fmt.Errorf("%w (persist pre-bind capsule revoke acknowledgement: %v)", cause, fateErr)
				}
				reloaded, reloadErr := rt.store.GetEngineeringAssignment(context.Background(), ownerID, computerID, assignmentID, attempt)
				if reloadErr != nil || reloaded.CapsuleDisposition != types.EngineeringCapsuleRevoked {
					return fmt.Errorf("%w (persist pre-bind capsule revoke acknowledgement: %v)", cause, fateErr)
				}
				acked = reloaded
			} else {
				acked = result.Assignment
			}
		}
		if acked.Disposition.Terminal() {
			return cause
		}
		cancel := types.CancelEngineeringAssignmentRequest{CommandID: "engineering-open-failed:" + assignmentID, OwnerID: ownerID, ComputerID: computerID,
			AssignmentID: assignmentID, Attempt: attempt, ExpectedLifecycleVersion: acked.LifecycleVersion, Reason: cause.Error()}
		cancel.CommandDigest, _ = store.ComputeCancelEngineeringAssignmentDigest(cancel)
		if _, cancelErr := rt.store.CancelEngineeringAssignment(context.Background(), cancel); cancelErr != nil {
			if !errors.Is(cancelErr, store.ErrLifecycleCommandConflict) && !errors.Is(cancelErr, store.ErrEngineeringAssignmentInvalid) {
				return fmt.Errorf("%w (cancel pre-bind assignment after revoke ack: %v)", cause, cancelErr)
			}
			reloaded, reloadErr := rt.store.GetEngineeringAssignment(context.Background(), ownerID, computerID, assignmentID, attempt)
			if reloadErr != nil || !reloaded.Disposition.Terminal() {
				return fmt.Errorf("%w (cancel pre-bind assignment after revoke ack: %v)", cause, cancelErr)
			}
		}
		return cause
	}
	if created.ID != capsuleID || created.State != capsule.StateActive {
		return AssignedEngineeringStart{}, cleanupCapsule(fmt.Errorf("assigned capsule acknowledgement mismatch"))
	}
	phaseStarted = logAssignmentSagaPhase(capsuleID, assignmentID, "spawn", phaseStarted)
	spawnedAt := time.Now().UTC()
	slot := "implementation"
	capability, err := rt.capsuleExecutor.MintCapabilityHandle(runID, capsule.RoleEngineering, capsuleID, opaque, 24*time.Hour, slot)
	grantedAt := time.Now().UTC()
	if err != nil {
		return AssignedEngineeringStart{}, cleanupCapsule(fmt.Errorf("mint exact assignment capability: %w", err))
	}
	compiledVerbs := make([]string, 0, len(capsule.RoleVerbSets[capsule.RoleEngineering]))
	for verb, allowed := range capsule.RoleVerbSets[capsule.RoleEngineering] {
		if allowed {
			compiledVerbs = append(compiledVerbs, verb)
		}
	}
	slices.Sort(compiledVerbs)
	actualVerbs := make([]string, 0, len(capability.Verbs))
	for verb, allowed := range capability.Verbs {
		if !allowed {
			return AssignedEngineeringStart{}, cleanupCapsule(fmt.Errorf("minted assignment capability contains a disabled verb"))
		}
		actualVerbs = append(actualVerbs, verb)
	}
	slices.Sort(actualVerbs)
	if capability.AgentRole != capsule.RoleEngineering || capability.AgentRunID != runID || capability.CapsuleID != capsuleID || capability.TargetCapsule != capsuleID ||
		capability.Handle != opaque || capability.Slot != slot || !slices.Equal(actualVerbs, compiledVerbs) || len(capability.ExternalAccess) != 0 || strings.TrimSpace(capability.KeyID) == "" ||
		len(capability.Signature) == 0 || !capability.ExpiresAt.After(grantedAt) || capability.ExpiresAt.After(grantedAt.Add(24*time.Hour+time.Second)) {
		return AssignedEngineeringStart{}, cleanupCapsule(fmt.Errorf("minted assignment capability acknowledgement mismatch"))
	}
	capabilityBytes, err := json.Marshal(capability)
	if err != nil {
		return AssignedEngineeringStart{}, cleanupCapsule(fmt.Errorf("digest minted assignment capability: %w", err))
	}
	grantAttestation := &types.EngineeringGrantPolicyAttestation{
		Role: string(capability.AgentRole), GrantedVerbs: actualVerbs,
		VerbSetDigest:          store.ComputeEngineeringGrantVerbSetDigest(actualVerbs),
		PolicyDigest:           store.ComputeEngineeringGrantPolicyDigest(string(capability.AgentRole), actualVerbs, binding.NetworkMode, binding.FilesystemMode, binding.Writable),
		SignedCapabilityDigest: objectgraph.SHA256(capabilityBytes), SpawnAcknowledged: true, ActiveAcknowledged: true, GrantAcknowledged: true,
		SpawnedAt: spawnedAt, GrantedAt: grantedAt,
	}
	run := buildAssignedEngineeringRun(assignment, runID, req.Objective, preflight.ArtifactRef)
	if overlay := strings.TrimSpace(req.ModelPolicyOverlayID); overlay != "" {
		run.Metadata[modelpolicy.MetadataPolicyOverlayID] = overlay
	}
	run.Metadata = rt.modelPolicy.EnrichMetadata(ctx, ownerID, agentprofile.Engineering, run.Metadata)
	// Fail closed on policy errors like the Texture eval path: an unknown or
	// unresolvable overlay must not silently fall back to the default model.
	if policyErr := metadataStringValue(run.Metadata, modelpolicy.MetadataPolicyError); policyErr != "" {
		return AssignedEngineeringStart{}, cleanupCapsule(fmt.Errorf("model policy overlay did not resolve: %s", policyErr))
	}
	if model := metadataStringValue(run.Metadata, modelpolicy.MetadataModel); model != "" {
		run.Metadata[runMetadataModel] = model
	}
	bind := types.BindEngineeringAssignmentRequest{
		CommandID: "engineering-bind:" + assignmentID, OwnerID: ownerID, ComputerID: computerID,
		AssignmentID: assignmentID, Attempt: attempt, ExpectedLifecycleVersion: assignment.LifecycleVersion,
		RunID: runID, Run: run, OpaqueCapability: opaque, CapsuleID: capsuleID, GrantPolicyAttestation: grantAttestation,
	}
	bind.CommandDigest, err = store.ComputeBindEngineeringAssignmentDigest(bind)
	if err != nil {
		return AssignedEngineeringStart{}, cleanupCapsule(err)
	}
	bound, err := rt.store.BindEngineeringAssignment(ctx, bind)
	if err != nil {
		return AssignedEngineeringStart{}, cleanupCapsule(fmt.Errorf("bind assigned Engineering activation: %w", err))
	}
	phaseStarted = logAssignmentSagaPhase(capsuleID, assignmentID, "bind", phaseStarted)
	rt.armAssignedEngineeringDeadline(bound.Assignment)
	// The lifecycle Bind receipt is durable before this actor wake. No generic
	// lifecycle-Management refusal is removed and no pre-cutover mailbox is written.
	rt.activate(&run)
	logAssignmentSagaPhase(capsuleID, assignmentID, "activate", phaseStarted)
	return AssignedEngineeringStart{Assignment: bound.Assignment, Run: run}, nil
}

// logAssignmentSagaPhase logs one phase of an assignment's capsule start
// under the capsule id the executor's spawn lines use, and returns the time
// the next phase starts from. Log lines only
// (docs/problems/capsule-coldstart-18s-2026-10-22.md).
func logAssignmentSagaPhase(capsuleID, assignmentID, phase string, started time.Time) time.Time {
	now := time.Now()
	log.Printf("capsule %s saga phase=%s took=%s assignment=%s", capsuleID, phase, now.Sub(started).Round(time.Millisecond), assignmentID)
	return now
}

// reclaimTerminalAssignmentCapsules revokes the capsules that terminal
// assignments left behind, so a fresh capsule never spawns on a leaked
// admission budget. It never cancels live work: a non-terminal assignment that
// holds a capsule makes it refuse with the retryable busy error. Admission
// (admitEngineeringAssignment) has already refused the request by then, under
// the same mutex; this is the second line. Fail closed: a reclaim failure must
// not open a new assignment on a leaked budget.
//
// Until 2026-10-21 this function cancelled every other assignment that held a
// capsule, "superseded by a fresh implementation assignment", against the
// doctrine's FIFO admission with a retryable refusal (SDC ARCH-10,
// docs/problems/engineering-admission-supersedes-live-assignment-2026-10-21.md).
func (rt *Runtime) reclaimTerminalAssignmentCapsules(ctx context.Context, parent types.RunRecord, currentAssignmentID string) error {
	assignments, err := rt.store.ListEngineeringAssignmentsForComputer(ctx, parent.ComputerID)
	if err != nil {
		return err
	}
	for _, assignment := range assignments {
		// Only bound/live capsules hold admission budget. Unbound capsules were
		// never spawned and revoked capsules already released theirs.
		if assignment.AssignmentID == currentAssignmentID ||
			assignment.CapsuleDisposition == types.EngineeringCapsuleRevoked ||
			assignment.CapsuleDisposition == types.EngineeringCapsuleUnbound {
			continue
		}
		if !assignment.Disposition.Terminal() {
			return &engineeringAdmissionBusy{AssignmentID: currentAssignmentID, BlockedBy: assignment.AssignmentID, Why: "live"}
		}
		if assignment.BoundRunID == "" {
			// A terminal assignment that never bound a run has no capsule to
			// revoke through the fate path.
			continue
		}
		// A terminal assignment whose capsule was never revoked still holds
		// admission budget: revoke the capsule directly through the fate path.
		reclaimed, err := rt.revokeAssignedCapsule(ctx, assignment,
			"terminal assignment capsule budget reclaimed for fresh assignment")
		if err != nil {
			return fmt.Errorf("reclaim terminal %s (capsule %s): %w",
				assignment.AssignmentID, assignment.Binding.CapsuleID, err)
		}
		log.Printf("runtime: reclaimed terminal assignment capsule %s (capsule %s) disposition=%s",
			assignment.AssignmentID, assignment.Binding.CapsuleID, reclaimed.CapsuleDisposition)
	}
	return nil
}
