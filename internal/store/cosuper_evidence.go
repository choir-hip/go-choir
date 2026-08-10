package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

var (
	ErrCoSuperEvidenceCorrupt  = errors.New("capsule evidence authority is corrupt or ambiguous")
	ErrCoSuperEvidenceTooLarge = errors.New("capsule evidence exceeds bounded projection")
)

const (
	CoSuperCapsuleEvidenceSchemaV1 = "choir.co_super_capsule_evidence/v1"
	coSuperEvidenceMaxObjects      = 20000
	coSuperEvidenceMaxReports      = 256
	coSuperEvidenceMaxCandidates   = 256
	coSuperEvidenceMaxExecutions   = 1024
	coSuperEvidenceMaxFateSteps    = 64
	coSuperEvidenceMaxCommands     = 256
	coSuperEvidenceMaxOutputs      = 256
	coSuperEvidenceMaxMutations    = 256
	coSuperEvidenceMaxEncodedBytes = 2 << 20
)

type CoSuperEvidenceAssignment struct {
	AssignmentID       string                             `json:"assignment_id"`
	Attempt            uint64                             `json:"attempt"`
	OwnerID            string                             `json:"owner_id"`
	ComputerID         string                             `json:"computer_id"`
	TrajectoryID       string                             `json:"trajectory_id"`
	ParentAgentID      string                             `json:"parent_agent_id"`
	ParentRunID        string                             `json:"parent_loop_id"`
	ParentDecisionID   string                             `json:"parent_decision_id"`
	ParentControlID    string                             `json:"parent_control_id"`
	ParentWorkItemID   string                             `json:"parent_work_item_id"`
	AssignedWorkItemID string                             `json:"assigned_work_item_id"`
	AssignedAgentID    string                             `json:"assigned_agent_id"`
	AssignmentKind     types.CoSuperAssignmentKind        `json:"assignment_kind"`
	Disposition        types.CoSuperAssignmentDisposition `json:"disposition"`
	CapsuleDisposition types.CoSuperCapsuleDisposition    `json:"capsule_disposition"`
	BoundRunID         string                             `json:"bound_loop_id,omitempty"`
	ScopeDigest        string                             `json:"scope_digest"`
	RequestDigest      string                             `json:"request_digest"`
	SubjectDigest      string                             `json:"subject_digest"`
	SourceCandidateID  string                             `json:"source_candidate_id,omitempty"`
	Writable           bool                               `json:"writable"`
	CapsuleID          string                             `json:"capsule_id"`
	NetworkMode        string                             `json:"network_mode"`
	FilesystemMode     string                             `json:"filesystem_mode"`
	LifecycleVersion   int64                              `json:"lifecycle_version"`
	CreatedAt          time.Time                          `json:"created_at"`
	UpdatedAt          time.Time                          `json:"updated_at"`
	TerminalAt         *time.Time                         `json:"terminal_at,omitempty"`
}

type CoSuperEvidenceCommand struct {
	CommandID     string `json:"command_id"`
	CommandDigest string `json:"command_digest"`
	ExitCode      int    `json:"exit_code"`
}

type CoSuperEvidenceOutput struct {
	OutputID string `json:"output_id"`
	Kind     string `json:"kind"`
	Digest   string `json:"digest"`
}

type CoSuperEvidenceMutation struct {
	MutationID          string `json:"mutation_id"`
	Kind                string `json:"kind"`
	BeforeDigest        string `json:"before_digest"`
	AfterDigest         string `json:"after_digest"`
	SubjectBytesChanged bool   `json:"subject_bytes_changed"`
}

type CoSuperEvidenceReport struct {
	ReportID                 string                            `json:"report_id"`
	AssignmentID             string                            `json:"assignment_id"`
	Attempt                  uint64                            `json:"attempt"`
	RunID                    string                            `json:"loop_id"`
	AssignedAgentID          string                            `json:"assigned_agent_id"`
	Result                   types.CoSuperAssignmentResultKind `json:"result"`
	Verdict                  types.CoSuperAssignmentVerdict    `json:"verdict"`
	ObservedSubjectDigest    string                            `json:"observed_subject_digest"`
	Commands                 []CoSuperEvidenceCommand          `json:"commands"`
	Outputs                  []CoSuperEvidenceOutput           `json:"outputs"`
	Mutations                []CoSuperEvidenceMutation         `json:"mutations"`
	Late                     bool                              `json:"late"`
	CertifiesOriginalSubject bool                              `json:"certifies_original_subject"`
	CandidateSubjectDigest   string                            `json:"candidate_subject_digest,omitempty"`
	CandidateID              string                            `json:"candidate_id,omitempty"`
	ReducerSeq               int64                             `json:"reducer_seq"`
	CreatedAt                time.Time                         `json:"created_at"`
}

type CoSuperEvidenceCandidate struct {
	CandidateID           string    `json:"candidate_id"`
	AssignmentID          string    `json:"assignment_id"`
	Attempt               uint64    `json:"attempt"`
	OriginalSubjectDigest string    `json:"original_subject_digest"`
	SubjectDigest         string    `json:"subject_digest"`
	SourceReportID        string    `json:"source_report_id"`
	ReducerSeq            int64     `json:"reducer_seq"`
	CreatedAt             time.Time `json:"created_at"`
}

type CoSuperEvidenceGrant struct {
	AttestationRef         string    `json:"attestation_ref"`
	Role                   string    `json:"role"`
	GrantedVerbs           []string  `json:"granted_verbs"`
	VerbSetDigest          string    `json:"verb_set_digest"`
	PolicyDigest           string    `json:"policy_digest"`
	SignedCapabilityDigest string    `json:"signed_capability_digest"`
	RunID                  string    `json:"loop_id"`
	CapsuleID              string    `json:"capsule_id"`
	TargetCapsule          string    `json:"target_capsule"`
	NetworkMode            string    `json:"network_mode"`
	FilesystemMode         string    `json:"filesystem_mode"`
	Writable               bool      `json:"writable"`
	SpawnAcknowledged      bool      `json:"spawn_acknowledged"`
	ActiveAcknowledged     bool      `json:"active_acknowledged"`
	GrantAcknowledged      bool      `json:"grant_acknowledged"`
	SpawnedAt              time.Time `json:"spawned_at"`
	GrantedAt              time.Time `json:"granted_at"`
	BindEventID            string    `json:"bind_event_id"`
	ReducerSeq             int64     `json:"reducer_seq"`
	RecordedAt             time.Time `json:"recorded_at"`
}

type CoSuperEvidenceExecution struct {
	AttestationRef      string    `json:"attestation_ref"`
	ReportID            string    `json:"report_id"`
	CommandID           string    `json:"command_id"`
	CommandDigest       string    `json:"command_digest"`
	RunID               string    `json:"loop_id"`
	CapsuleID           string    `json:"capsule_id"`
	ExitCode            int       `json:"exit_code"`
	StdoutDigest        string    `json:"stdout_digest"`
	StderrDigest        string    `json:"stderr_digest"`
	SourceSubjectDigest string    `json:"source_subject_digest"`
	FinalSubjectDigest  string    `json:"final_subject_digest"`
	WorktreeDigest      string    `json:"worktree_digest"`
	Granted             bool      `json:"granted"`
	Frozen              bool      `json:"frozen"`
	OccurredAt          time.Time `json:"occurred_at"`
	ReportEventID       string    `json:"report_event_id"`
	ReducerSeq          int64     `json:"reducer_seq"`
	RecordedAt          time.Time `json:"recorded_at"`
}

type CoSuperEvidenceFateStep struct {
	StepRef             string                          `json:"step_ref"`
	Disposition         types.CoSuperCapsuleDisposition `json:"capsule_disposition"`
	RunID               string                          `json:"loop_id,omitempty"`
	CapsuleID           string                          `json:"capsule_id"`
	EventID             string                          `json:"event_id"`
	ReducerSeq          int64                           `json:"reducer_seq"`
	IntentRef           string                          `json:"intent_ref"`
	AckRef              string                          `json:"ack_ref,omitempty"`
	SourceSubjectDigest string                          `json:"source_subject_digest,omitempty"`
	FinalSubjectDigest  string                          `json:"final_subject_digest,omitempty"`
	CapsuleAbsent       bool                            `json:"capsule_absent,omitempty"`
	OccurredAt          time.Time                       `json:"occurred_at"`
	RecordedAt          time.Time                       `json:"recorded_at"`
}

type CoSuperEvidenceVerifierContract struct {
	Schema                 string `json:"schema"`
	Version                string `json:"version"`
	PhysicalIsolationClaim bool   `json:"physical_isolation_claim"`
	EffectsEnabled         bool   `json:"effects_enabled"`
}

type CoSuperCapsuleEvidence struct {
	Schema                 string                          `json:"schema"`
	Assignment             CoSuperEvidenceAssignment       `json:"assignment"`
	GrantPolicyAttestation *CoSuperEvidenceGrant           `json:"grant_policy_attestation,omitempty"`
	Reports                []CoSuperEvidenceReport         `json:"reports"`
	Candidates             []CoSuperEvidenceCandidate      `json:"candidates"`
	ExecutionAttestations  []CoSuperEvidenceExecution      `json:"execution_attestations"`
	CapsuleFateHistory     []CoSuperEvidenceFateStep       `json:"capsule_fate_history"`
	TextureSourceRefs      []string                        `json:"texture_source_refs"`
	SnapshotCursor         int64                           `json:"snapshot_cursor"`
	Watermark              int64                           `json:"watermark"`
	VerifierContract       CoSuperEvidenceVerifierContract `json:"verifier_contract"`
	EvidenceComplete       bool                            `json:"evidence_complete"`
	Deficits               []string                        `json:"deficits"`
}

func corruptEvidence(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrCoSuperEvidenceCorrupt, fmt.Sprintf(format, args...))
}

// GetCoSuperCapsuleEvidence reads the owner/computer ObjectGraph authority once
// and performs every trajectory/assignment/report/candidate/event join in
// memory. It never consults the executor or any receipt directory.
func (s *Store) GetCoSuperCapsuleEvidence(ctx context.Context, ownerID, computerID, trajectoryID, assignmentID string, attempt uint64) (CoSuperCapsuleEvidence, error) {
	ownerID, computerID, err := normalizeLifecycleScope(ownerID, computerID)
	if err != nil {
		return CoSuperCapsuleEvidence{}, err
	}
	trajectoryID, assignmentID = strings.TrimSpace(trajectoryID), strings.TrimSpace(assignmentID)
	if trajectoryID == "" || assignmentID == "" || attempt == 0 {
		return CoSuperCapsuleEvidence{}, ErrNotFound
	}
	graph := s.ogReadStore
	if graph == nil {
		graph = s.ogStore
	}
	if graph == nil {
		return CoSuperCapsuleEvidence{}, corruptEvidence("object graph unavailable")
	}
	objects, err := graph.ReadObjectSnapshot(ctx, ownerID, computerID)
	if err != nil {
		return CoSuperCapsuleEvidence{}, err
	}
	if len(objects) > coSuperEvidenceMaxObjects {
		return CoSuperCapsuleEvidence{}, ErrCoSuperEvidenceTooLarge
	}
	byID := make(map[string]objectgraph.Object, len(objects))
	for _, obj := range objects {
		if obj.Tombstone {
			continue
		}
		if obj.OwnerID != ownerID || obj.ComputerID != computerID || obj.CanonicalID == "" {
			return CoSuperCapsuleEvidence{}, corruptEvidence("snapshot scope or object identity mismatch")
		}
		if _, dup := byID[obj.CanonicalID]; dup {
			return CoSuperCapsuleEvidence{}, corruptEvidence("duplicate object identity")
		}
		byID[obj.CanonicalID] = obj
	}
	trajectoryCanonicalID, trajectoryIDErr := lifecycleCanonicalID(ogKindTrajectory, ownerID, computerID, trajectoryID)
	assignmentCanonicalID, assignmentIDErr := lifecycleCanonicalID(ogKindCoSuperAssignment, ownerID, computerID, coSuperAttemptKey(assignmentID, attempt))
	if trajectoryIDErr != nil || assignmentIDErr != nil {
		return CoSuperCapsuleEvidence{}, ErrNotFound
	}
	exactTrajectoryObj, trajectoryExists := byID[trajectoryCanonicalID]
	if !trajectoryExists {
		return CoSuperCapsuleEvidence{}, ErrNotFound
	}
	var exactTrajectory types.TrajectoryRecord
	if json.Unmarshal(exactTrajectoryObj.Body, &exactTrajectory) != nil || exactTrajectory.TrajectoryID != trajectoryID || exactTrajectory.OwnerID != ownerID || exactTrajectory.ComputerID != computerID || exactTrajectory.LifecycleVersion <= 0 {
		return CoSuperCapsuleEvidence{}, corruptEvidence("requested trajectory identity")
	}
	exactAssignmentObj, assignmentExists := byID[assignmentCanonicalID]
	if !assignmentExists {
		return CoSuperCapsuleEvidence{}, ErrNotFound
	}
	var exactAssignment types.CoSuperAssignment
	if json.Unmarshal(exactAssignmentObj.Body, &exactAssignment) != nil || exactAssignment.AssignmentID != assignmentID || exactAssignment.Binding.Attempt != attempt || exactAssignment.Binding.OwnerID != ownerID || exactAssignment.Binding.ComputerID != computerID || exactAssignment.Binding.TrajectoryID != trajectoryID {
		return CoSuperCapsuleEvidence{}, corruptEvidence("requested assignment identity")
	}
	trajectoryFound := false
	assignments := map[string]types.CoSuperAssignment{}
	events := map[string]types.LifecycleEvent{}
	reportObjects := map[string]objectgraph.Object{}
	candidateObjects := map[string]objectgraph.Object{}
	for _, obj := range objects {
		switch obj.ObjectKind {
		case ogKindTrajectory:
			var v types.TrajectoryRecord
			if json.Unmarshal(obj.Body, &v) != nil {
				if obj.CanonicalID == trajectoryCanonicalID {
					return CoSuperCapsuleEvidence{}, corruptEvidence("corrupt trajectory body")
				}
				continue
			}
			if v.TrajectoryID == trajectoryID {
				if obj.CanonicalID != trajectoryCanonicalID || trajectoryFound || v.OwnerID != ownerID || v.ComputerID != computerID || v.LifecycleVersion <= 0 {
					return CoSuperCapsuleEvidence{}, corruptEvidence("trajectory identity")
				}
				trajectoryFound = true
			}
		case ogKindCoSuperAssignment:
			var v types.CoSuperAssignment
			if json.Unmarshal(obj.Body, &v) != nil {
				if obj.CanonicalID == assignmentCanonicalID {
					return CoSuperCapsuleEvidence{}, corruptEvidence("corrupt assignment body")
				}
				continue
			}
			if v.Binding.TrajectoryID == trajectoryID {
				key := coSuperAttemptKey(v.AssignmentID, v.Binding.Attempt)
				if key == coSuperAttemptKey(assignmentID, attempt) && obj.CanonicalID != assignmentCanonicalID {
					return CoSuperCapsuleEvidence{}, corruptEvidence("noncanonical assignment identity")
				}
				if _, dup := assignments[key]; dup {
					return CoSuperCapsuleEvidence{}, corruptEvidence("duplicate assignment key")
				}
				assignments[key] = v
			}
		case ogKindLifecycleEvent:
			var v types.LifecycleEvent
			if json.Unmarshal(obj.Body, &v) != nil {
				continue
			}
			if v.TrajectoryID == trajectoryID {
				expected, buildErr := lifecycleCanonicalID(ogKindLifecycleEvent, ownerID, computerID, v.EventID)
				if buildErr != nil || expected != obj.CanonicalID || v.OwnerID != ownerID || v.ComputerID != computerID || v.EventID == "" || v.ReducerSeq <= 0 || strings.TrimSpace(v.CommandID) == "" || !types.ValidSHA256Digest(v.CommandDigest) {
					return CoSuperCapsuleEvidence{}, corruptEvidence("event canonical identity or scope")
				}
				if _, dup := events[v.EventID]; dup {
					return CoSuperCapsuleEvidence{}, corruptEvidence("duplicate event identity")
				}
				events[v.EventID] = v
			}
		case ogKindCoSuperReport:
			reportObjects[obj.CanonicalID] = obj
		case ogKindCoSuperCandidate:
			candidateObjects[obj.CanonicalID] = obj
		}
	}
	if !trajectoryFound {
		return CoSuperCapsuleEvidence{}, ErrNotFound
	}
	a, ok := assignments[coSuperAttemptKey(assignmentID, attempt)]
	if !ok || a.AssignmentID != exactAssignment.AssignmentID || a.Binding != exactAssignment.Binding {
		return CoSuperCapsuleEvidence{}, corruptEvidence("requested assignment snapshot join")
	}
	if err := a.Validate(); err != nil {
		return CoSuperCapsuleEvidence{}, corruptEvidence("assignment validation")
	}
	if a.GrantPolicyAttestation != nil {
		if err := validateGrantPolicyAttestation(*a.GrantPolicyAttestation, a); err != nil {
			return CoSuperCapsuleEvidence{}, corruptEvidence("grant validation")
		}
	}
	if err := validateFateHistory(a.CapsuleFateHistory, a); err != nil {
		return CoSuperCapsuleEvidence{}, corruptEvidence("fate validation")
	}
	eventList := make([]types.LifecycleEvent, 0, len(events))
	for _, event := range events {
		eventList = append(eventList, event)
	}
	sort.Slice(eventList, func(i, j int) bool {
		if eventList[i].ReducerSeq == eventList[j].ReducerSeq {
			return eventList[i].EventID < eventList[j].EventID
		}
		return eventList[i].ReducerSeq < eventList[j].ReducerSeq
	})
	watermark := int64(0)
	if len(eventList) > 0 {
		watermark = eventList[len(eventList)-1].ReducerSeq
	}
	seqFor := func(commandID string, kind types.LifecycleEventKind) (types.LifecycleEvent, error) {
		e, ok := events[commandID+":1"]
		if !ok || e.CommandID != commandID || e.Kind != kind || e.WorkItemID != a.Binding.AssignedWorkItemID {
			return types.LifecycleEvent{}, corruptEvidence("event join")
		}
		return e, nil
	}
	if a.GrantPolicyAttestation != nil {
		event, e := seqFor(a.GrantPolicyAttestation.BindCommandID, types.LifecycleCoSuperAssignmentBound)
		if e != nil || event.EventID != a.GrantPolicyAttestation.BindEventID || event.ReducerSeq != a.GrantPolicyAttestation.ReducerSeq {
			return CoSuperCapsuleEvidence{}, corruptEvidence("grant event join")
		}
	}
	for _, step := range a.CapsuleFateHistory {
		event, e := seqFor(step.CommandID, types.LifecycleCoSuperCapsuleDispositionSet)
		if e != nil || event.EventID != step.EventID || event.ReducerSeq != step.ReducerSeq {
			return CoSuperCapsuleEvidence{}, corruptEvidence("fate event join")
		}
	}
	if len(a.ReportRefs) > coSuperEvidenceMaxReports || len(a.CapsuleFateHistory) > coSuperEvidenceMaxFateSteps {
		return CoSuperCapsuleEvidence{}, ErrCoSuperEvidenceTooLarge
	}
	seenReportIDs := map[string]bool{}
	reportObjectsByID := map[string]objectgraph.Object{}
	for id, obj := range reportObjects {
		var r types.CoSuperAssignmentReport
		if json.Unmarshal(obj.Body, &r) == nil && r.OwnerID == ownerID && r.ComputerID == computerID && r.TrajectoryID == trajectoryID {
			expected, buildErr := lifecycleCanonicalID(ogKindCoSuperReport, ownerID, computerID, r.ReportID)
			if buildErr != nil || expected != id || seenReportIDs[r.ReportID] {
				return CoSuperCapsuleEvidence{}, corruptEvidence("duplicate or noncanonical report identity")
			}
			seenReportIDs[r.ReportID] = true
			reportObjectsByID[r.ReportID] = obj
		}
	}
	seenCandidateIDs := map[string]bool{}
	for id, obj := range candidateObjects {
		var c types.CoSuperSubjectCandidate
		if json.Unmarshal(obj.Body, &c) == nil && c.OwnerID == ownerID && c.ComputerID == computerID && c.TrajectoryID == trajectoryID {
			if c.CandidateID != id || seenCandidateIDs[c.CandidateID] {
				return CoSuperCapsuleEvidence{}, corruptEvidence("duplicate or noncanonical candidate identity")
			}
			seenCandidateIDs[c.CandidateID] = true
		}
	}
	declaredReportRefs := make(map[string]bool, len(a.ReportRefs))
	for _, ref := range a.ReportRefs {
		if declaredReportRefs[ref] {
			return CoSuperCapsuleEvidence{}, corruptEvidence("duplicate report ref")
		}
		declaredReportRefs[ref] = true
	}
	for id, obj := range reportObjects {
		var r types.CoSuperAssignmentReport
		if json.Unmarshal(obj.Body, &r) != nil {
			var metadata struct {
				AssignmentID string `json:"assignment_id"`
				Attempt      uint64 `json:"attempt"`
			}
			if json.Unmarshal(obj.Metadata, &metadata) == nil && metadata.AssignmentID == a.AssignmentID && metadata.Attempt == a.Binding.Attempt {
				return CoSuperCapsuleEvidence{}, corruptEvidence("corrupt report body")
			}
			continue
		}
		if r.OwnerID == ownerID && r.ComputerID == computerID && r.TrajectoryID == trajectoryID && r.AssignmentID == a.AssignmentID && r.Attempt == a.Binding.Attempt && !declaredReportRefs[id] {
			return CoSuperCapsuleEvidence{}, corruptEvidence("unjoined assignment report")
		}
	}
	type reportJoin struct {
		r     types.CoSuperAssignmentReport
		event types.LifecycleEvent
		ref   string
	}
	findReportEvent := func(ref string, kind types.LifecycleEventKind, workItemID string) (types.LifecycleEvent, error) {
		var matched *types.LifecycleEvent
		for i := range eventList {
			ev := eventList[i]
			count := 0
			for _, artifact := range ev.ArtifactRefs {
				if artifact == ref {
					count++
				}
			}
			if count > 1 {
				return types.LifecycleEvent{}, corruptEvidence("duplicate report artifact ref in event")
			}
			if count == 0 {
				continue
			}
			if ev.Kind != kind || ev.WorkItemID != workItemID {
				return types.LifecycleEvent{}, corruptEvidence("report event kind or work scope")
			}
			if matched != nil {
				return types.LifecycleEvent{}, corruptEvidence("ambiguous report event")
			}
			copy := ev
			matched = &copy
		}
		if matched == nil {
			return types.LifecycleEvent{}, corruptEvidence("report event missing")
		}
		return *matched, nil
	}
	joinedReports := make([]reportJoin, 0, len(a.ReportRefs))
	allReports := map[string]reportJoin{}
	seenRefs := map[string]bool{}
	for _, ref := range a.ReportRefs {
		if seenRefs[ref] {
			return CoSuperCapsuleEvidence{}, corruptEvidence("duplicate report ref")
		}
		seenRefs[ref] = true
		obj, found := reportObjects[ref]
		if !found {
			return CoSuperCapsuleEvidence{}, corruptEvidence("referenced report missing")
		}
		var r types.CoSuperAssignmentReport
		if json.Unmarshal(obj.Body, &r) != nil || r.ValidateAgainst(a) != nil || r.ReportID == "" {
			return CoSuperCapsuleEvidence{}, corruptEvidence("report validation")
		}
		kind := types.LifecycleCoSuperAssignmentReported
		if strings.HasPrefix(r.ReportID, "cancel-report:") {
			kind = types.LifecycleCoSuperAssignmentCancelled
		}
		if len(r.Commands) > coSuperEvidenceMaxCommands || len(r.Outputs) > coSuperEvidenceMaxOutputs || len(r.Mutations) > coSuperEvidenceMaxMutations {
			return CoSuperCapsuleEvidence{}, ErrCoSuperEvidenceTooLarge
		}
		event, eventErr := findReportEvent(ref, kind, a.Binding.AssignedWorkItemID)
		if eventErr != nil {
			return CoSuperCapsuleEvidence{}, eventErr
		}
		if (event.RunID != "" && event.RunID != r.RunID) || (event.AgentID != "" && event.AgentID != r.AssignedAgentID) {
			return CoSuperCapsuleEvidence{}, corruptEvidence("report event run or agent scope")
		}
		if len(r.ExecutionAttestations) > 0 {
			if err := validateExecutionAttestations(r.ExecutionAttestations, r, a); err != nil {
				return CoSuperCapsuleEvidence{}, corruptEvidence("execution validation")
			}
			for _, att := range r.ExecutionAttestations {
				if att.ReportCommandID != event.CommandID || att.ReportEventID != event.EventID || att.ReducerSeq != event.ReducerSeq {
					return CoSuperCapsuleEvidence{}, corruptEvidence("execution event join")
				}
			}
		}
		joinedReports = append(joinedReports, reportJoin{r, event, ref})
		allReports[r.ReportID] = reportJoin{r, event, ref}
	}
	sort.Slice(joinedReports, func(i, j int) bool {
		if joinedReports[i].event.ReducerSeq == joinedReports[j].event.ReducerSeq {
			return joinedReports[i].r.ReportID < joinedReports[j].r.ReportID
		}
		return joinedReports[i].event.ReducerSeq < joinedReports[j].event.ReducerSeq
	})
	candidateIDs := map[string]bool{}
	for _, j := range joinedReports {
		if j.r.CandidateID != "" {
			candidateIDs[j.r.CandidateID] = true
		}
	}
	if a.Binding.SourceCandidateID != "" {
		candidateIDs[a.Binding.SourceCandidateID] = true
	}
	for id, obj := range candidateObjects {
		var c types.CoSuperSubjectCandidate
		if json.Unmarshal(obj.Body, &c) == nil && c.OwnerID == ownerID && c.ComputerID == computerID && c.TrajectoryID == trajectoryID && c.AssignmentID == a.AssignmentID && c.Attempt == a.Binding.Attempt && !candidateIDs[id] {
			return CoSuperCapsuleEvidence{}, corruptEvidence("unjoined assignment candidate")
		}
	}
	if len(candidateIDs) > coSuperEvidenceMaxCandidates {
		return CoSuperCapsuleEvidence{}, ErrCoSuperEvidenceTooLarge
	}
	type candidateJoin struct {
		c   types.CoSuperSubjectCandidate
		seq int64
	}
	candidates := make([]candidateJoin, 0, len(candidateIDs))
	for id := range candidateIDs {
		obj, found := candidateObjects[id]
		if !found {
			return CoSuperCapsuleEvidence{}, corruptEvidence("referenced candidate missing")
		}
		var c types.CoSuperSubjectCandidate
		if json.Unmarshal(obj.Body, &c) != nil || c.CandidateID != id || c.OwnerID != ownerID || c.ComputerID != computerID || c.TrajectoryID != trajectoryID || !types.ValidSHA256Digest(c.OriginalSubjectDigest) || !types.ValidSHA256Digest(c.SubjectDigest) || c.ArtifactRef != "capsule-subject:"+c.SubjectDigest {
			return CoSuperCapsuleEvidence{}, corruptEvidence("candidate validation")
		}
		source, found := allReports[c.SourceReportID]
		if !found { // source candidate of a verification belongs to another exact implementation report in the same snapshot.
			for _, robj := range reportObjects {
				var rr types.CoSuperAssignmentReport
				if json.Unmarshal(robj.Body, &rr) == nil && rr.ReportID == c.SourceReportID && rr.OwnerID == ownerID && rr.ComputerID == computerID && rr.TrajectoryID == trajectoryID {
					srcA, ok := assignments[coSuperAttemptKey(rr.AssignmentID, rr.Attempt)]
					if !ok || srcA.Binding.Kind != types.CoSuperAssignmentImplementation || rr.ValidateAgainst(srcA) != nil {
						return CoSuperCapsuleEvidence{}, corruptEvidence("candidate source lineage")
					}
					sourceKind := types.LifecycleCoSuperAssignmentReported
					if strings.HasPrefix(rr.ReportID, "cancel-report:") {
						sourceKind = types.LifecycleCoSuperAssignmentCancelled
					}
					sourceEvent, sourceErr := findReportEvent(robj.CanonicalID, sourceKind, srcA.Binding.AssignedWorkItemID)
					if sourceErr != nil {
						return CoSuperCapsuleEvidence{}, sourceErr
					}
					source = reportJoin{rr, sourceEvent, robj.CanonicalID}
					found = true
					break
				}
			}
		}
		if !found || source.r.CandidateID != c.CandidateID || source.r.CandidateSubjectDigest != c.SubjectDigest || source.r.AssignmentID != c.AssignmentID || source.r.Attempt != c.Attempt {
			return CoSuperCapsuleEvidence{}, corruptEvidence("candidate report join")
		}
		if a.Binding.SourceCandidateID == id {
			impl := assignments[coSuperAttemptKey(c.AssignmentID, c.Attempt)]
			if impl.Binding.Kind != types.CoSuperAssignmentImplementation || impl.Binding.ParentAgentID != a.Binding.ParentAgentID || impl.Binding.ParentWorkItemID != a.Binding.ParentWorkItemID || a.Binding.SubjectDigest != c.SubjectDigest {
				return CoSuperCapsuleEvidence{}, corruptEvidence("implementation verifier join")
			}
		}
		candidates = append(candidates, candidateJoin{c, source.event.ReducerSeq})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].seq == candidates[j].seq {
			return candidates[i].c.CandidateID < candidates[j].c.CandidateID
		}
		return candidates[i].seq < candidates[j].seq
	})
	out := CoSuperCapsuleEvidence{Schema: CoSuperCapsuleEvidenceSchemaV1, Reports: []CoSuperEvidenceReport{}, Candidates: []CoSuperEvidenceCandidate{}, ExecutionAttestations: []CoSuperEvidenceExecution{}, CapsuleFateHistory: []CoSuperEvidenceFateStep{}, TextureSourceRefs: []string{}, SnapshotCursor: watermark, Watermark: watermark, VerifierContract: CoSuperEvidenceVerifierContract{Schema: "choir.co_super_capsule_evidence_verifier/v1", Version: "f1-incomplete", PhysicalIsolationClaim: false, EffectsEnabled: false}, EvidenceComplete: false}
	b := a.Binding
	out.Assignment = CoSuperEvidenceAssignment{a.AssignmentID, b.Attempt, b.OwnerID, b.ComputerID, b.TrajectoryID, b.ParentAgentID, b.ParentRunID, b.ParentDecisionID, b.ParentControlID, b.ParentWorkItemID, b.AssignedWorkItemID, b.AssignedAgentID, b.Kind, a.Disposition, a.CapsuleDisposition, a.BoundRunID, b.ScopeDigest, b.RequestDigest, b.SubjectDigest, b.SourceCandidateID, b.Writable, b.CapsuleID, b.NetworkMode, b.FilesystemMode, a.LifecycleVersion, a.CreatedAt, a.UpdatedAt, a.TerminalAt}
	if g := a.GrantPolicyAttestation; g != nil {
		out.GrantPolicyAttestation = &CoSuperEvidenceGrant{g.AttestationRef, g.Role, append([]string(nil), g.GrantedVerbs...), g.VerbSetDigest, g.PolicyDigest, g.SignedCapabilityDigest, g.RunID, g.CapsuleID, g.TargetCapsule, g.NetworkMode, g.FilesystemMode, g.Writable, g.SpawnAcknowledged, g.ActiveAcknowledged, g.GrantAcknowledged, g.SpawnedAt, g.GrantedAt, g.BindEventID, g.ReducerSeq, g.RecordedAt}
	}
	for _, j := range joinedReports {
		r := j.r
		pr := CoSuperEvidenceReport{ReportID: r.ReportID, AssignmentID: r.AssignmentID, Attempt: r.Attempt, RunID: r.RunID, AssignedAgentID: r.AssignedAgentID, Result: r.Result, Verdict: r.Verdict, ObservedSubjectDigest: r.ObservedSubjectDigest, Commands: []CoSuperEvidenceCommand{}, Outputs: []CoSuperEvidenceOutput{}, Mutations: []CoSuperEvidenceMutation{}, Late: r.Late, CertifiesOriginalSubject: r.CertifiesOriginalSubject, CandidateSubjectDigest: r.CandidateSubjectDigest, CandidateID: r.CandidateID, ReducerSeq: j.event.ReducerSeq, CreatedAt: r.CreatedAt}
		for _, v := range r.Commands {
			pr.Commands = append(pr.Commands, CoSuperEvidenceCommand{v.CommandID, v.CommandDigest, v.ExitCode})
		}
		for _, v := range r.Outputs {
			pr.Outputs = append(pr.Outputs, CoSuperEvidenceOutput{v.OutputID, v.Kind, v.Digest})
		}
		for _, v := range r.Mutations {
			pr.Mutations = append(pr.Mutations, CoSuperEvidenceMutation{v.MutationID, v.Kind, v.BeforeDigest, v.AfterDigest, v.SubjectBytesChanged})
		}
		out.Reports = append(out.Reports, pr)
		for _, x := range r.ExecutionAttestations {
			out.ExecutionAttestations = append(out.ExecutionAttestations, CoSuperEvidenceExecution{x.AttestationRef, x.ReportID, x.CommandID, x.CommandDigest, x.RunID, x.CapsuleID, x.ExitCode, x.StdoutDigest, x.StderrDigest, x.SourceSubjectDigest, x.FinalSubjectDigest, x.WorktreeDigest, x.Granted, x.Frozen, x.OccurredAt, x.ReportEventID, x.ReducerSeq, x.RecordedAt})
		}
	}
	if len(out.ExecutionAttestations) > coSuperEvidenceMaxExecutions {
		return CoSuperCapsuleEvidence{}, ErrCoSuperEvidenceTooLarge
	}
	sort.Slice(out.ExecutionAttestations, func(i, j int) bool {
		if out.ExecutionAttestations[i].ReducerSeq == out.ExecutionAttestations[j].ReducerSeq {
			return out.ExecutionAttestations[i].AttestationRef < out.ExecutionAttestations[j].AttestationRef
		}
		return out.ExecutionAttestations[i].ReducerSeq < out.ExecutionAttestations[j].ReducerSeq
	})
	for _, j := range candidates {
		c := j.c
		out.Candidates = append(out.Candidates, CoSuperEvidenceCandidate{c.CandidateID, c.AssignmentID, c.Attempt, c.OriginalSubjectDigest, c.SubjectDigest, c.SourceReportID, j.seq, c.CreatedAt})
	}
	for _, x := range a.CapsuleFateHistory {
		out.CapsuleFateHistory = append(out.CapsuleFateHistory, CoSuperEvidenceFateStep{x.StepRef, x.Disposition, x.RunID, x.CapsuleID, x.EventID, x.ReducerSeq, x.IntentRef, x.AckRef, x.SourceSubjectDigest, x.FinalSubjectDigest, x.CapsuleAbsent, x.OccurredAt, x.RecordedAt})
	}
	deficits := []string{"isolation_probe_missing", "texture_source_missing", "run_acceptance_gate_missing"}
	if a.GrantPolicyAttestation == nil {
		deficits = append(deficits, "grant_policy_attestation_missing")
	}
	commandCount := 0
	for _, r := range out.Reports {
		commandCount += len(r.Commands)
	}
	if len(out.ExecutionAttestations) < commandCount {
		deficits = append(deficits, "execution_attestation_missing")
	}
	if len(a.CapsuleFateHistory) == 0 {
		deficits = append(deficits, "capsule_fate_history_missing")
	}
	sort.Strings(deficits)
	out.Deficits = deficits
	encoded, err := json.Marshal(out)
	if err != nil {
		return CoSuperCapsuleEvidence{}, err
	}
	if len(encoded)+1 > coSuperEvidenceMaxEncodedBytes {
		return CoSuperCapsuleEvidence{}, ErrCoSuperEvidenceTooLarge
	}
	return out, nil
}
