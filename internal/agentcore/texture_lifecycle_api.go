package agentcore

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TextureActorToolLoopBudgetSpend is the persisted spend carried across
// activations of one resident Texture actor.
type TextureActorToolLoopBudgetSpend struct {
	SourceRunID        string
	ProviderCalls      int
	InputTokens        int
	OutputTokens       int
	ObservedUsageEvent bool
}

type checkedActorDispatchState struct {
	mu     sync.Mutex
	errors map[string]error
}

var checkedActorDispatches sync.Map // map[*Runtime]*checkedActorDispatchState

// SetCheckedDispatchActor installs the actor substrate while retaining the
// synchronous result of each exact dispatch. This lets Store projection callers
// fail when the cold actor occurrence append failed instead of trusting a log.
func (rt *Runtime) SetCheckedDispatchActor(fn func(context.Context, string, string, string, string, string, string, string) error) {
	state := &checkedActorDispatchState{errors: make(map[string]error)}
	checkedActorDispatches.Store(rt, state)
	rt.SetDispatchActor(func(ctx context.Context, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string) error {
		err := fn(ctx, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID)
		key := strings.TrimSpace(kind) + "\x00" + strings.TrimSpace(content)
		state.mu.Lock()
		if err != nil {
			state.errors[key] = err
		} else {
			delete(state.errors, key)
		}
		state.mu.Unlock()
		return err
	})
}

// CheckedActorDispatchError returns the exact most recent synchronous append
// error for a checked dispatch identity.
func (rt *Runtime) CheckedActorDispatchError(kind, content string) error {
	raw, ok := checkedActorDispatches.Load(rt)
	if !ok {
		return nil
	}
	state := raw.(*checkedActorDispatchState)
	key := strings.TrimSpace(kind) + "\x00" + strings.TrimSpace(content)
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.errors[key]
}

// DispatchActor sends a concrete actor-runtime message through the configured
// actor execution substrate.
func (rt *Runtime) DispatchActor(ctx context.Context, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string) error {
	if rt == nil || rt.dispatchActor == nil {
		return fmt.Errorf("runtime: actor dispatch unavailable")
	}
	return rt.dispatchActor(ctx, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID)
}

// ErrActivationOccurrenceMustRemainUnprocessed means execution durably
// passivated the same exact run for retry. Acknowledging its current mailbox
// occurrence would lose the only non-colliding retry authority.
var ErrActivationOccurrenceMustRemainUnprocessed = errors.New("activation occurrence must remain unprocessed")

// ExecuteActivationSyncChecked preserves the actor-log acknowledgement boundary.
// A transient provider-admission failure whose durable retry passivation could
// not be persisted is returned to the actor handler so the exact occurrence
// remains unprocessed.
func (rt *Runtime) ExecuteActivationSyncChecked(ctx context.Context, rec *types.RunRecord) error {
	rt.ExecuteActivationSync(ctx, rec)
	if rec == nil {
		return fmt.Errorf("runtime: activation returned no run")
	}
	if message := metadataStringValue(rec.Metadata, activationRetryableErrorMetadata); message != "" {
		return errors.New(message)
	}
	if rec.State == types.RunPassivated {
		reason := metadataStringValue(rec.Metadata, "passivated_reason")
		if reason == runtimeInjectionAppendFailurePassivationReason || reason == lifecycleResearcherAdmissionRetryReason {
			return fmt.Errorf("%w: %s", ErrActivationOccurrenceMustRemainUnprocessed, reason)
		}
	}
	return nil
}

// TextureActorParkIdle returns the configured resident Texture idle interval.
func (rt *Runtime) TextureActorParkIdle() time.Duration {
	if rt == nil {
		return 0
	}
	return rt.cfg.TextureActorParkIdle
}

// TextureComputerID returns the runtime autoputer identity used for durable actor records.
func (rt *Runtime) TextureComputerID() string {
	if rt == nil {
		return ""
	}
	return rt.cfg.ComputerID
}

// TextureActiveRunByAgent returns the latest executing lifecycle run for one computer-scoped actor.
func (rt *Runtime) TextureActiveRunByAgent(ctx context.Context, ownerID, computerID, agentID string) (types.RunRecord, bool, error) {
	if rt == nil || rt.store == nil {
		return types.RunRecord{}, false, nil
	}
	ownerID = strings.TrimSpace(ownerID)
	computerID = strings.TrimSpace(computerID)
	agentID = strings.TrimSpace(agentID)
	if ownerID == "" || computerID == "" || agentID == "" {
		return types.RunRecord{}, false, nil
	}
	rec, err := rt.store.GetLatestActiveLifecycleRunByAgent(ctx, ownerID, computerID, agentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return types.RunRecord{}, false, nil
		}
		return types.RunRecord{}, false, err
	}
	if rec.State == types.RunBlocked {
		return types.RunRecord{}, false, nil
	}
	return rec, true, nil
}

// TextureChannelHasGroundedHistory reports whether grounded worker actors have
// already delivered a channel message before the optional boundary.
func (rt *Runtime) TextureChannelHasGroundedHistory(ctx context.Context, ownerID, channelID string, before time.Time) (bool, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return false, nil
	}
	runs, err := rt.ListRunsByChannel(ctx, ownerID, channelID, 500)
	if err != nil {
		return false, err
	}
	groundedRunIDs := make(map[string]struct{})
	for _, run := range runs {
		if !before.IsZero() && !run.CreatedAt.Before(before) {
			continue
		}
		switch agentProfileForRun(&run) {
		case agentprofile.Researcher, agentprofile.Super, agentprofile.CoSuper:
			groundedRunIDs[run.RunID] = struct{}{}
		}
	}
	if len(groundedRunIDs) == 0 {
		return false, nil
	}
	messages, err := rt.store.ListChannelMessages(ctx, ownerID, channelID, 0, 500)
	if err != nil {
		return false, err
	}
	for _, message := range messages {
		if !before.IsZero() && !message.Timestamp.Before(before) {
			continue
		}
		if _, ok := groundedRunIDs[strings.TrimSpace(message.FromRunID)]; ok {
			return true, nil
		}
	}
	return false, nil
}

// LatestTextureActorToolLoopBudgetSpend loads durable provider/tool-loop spend
// from the previous activation of the same actor.
func (rt *Runtime) LatestTextureActorToolLoopBudgetSpend(ctx context.Context, ownerID, agentID string) (TextureActorToolLoopBudgetSpend, bool, error) {
	var spend TextureActorToolLoopBudgetSpend
	if rt == nil || rt.store == nil {
		return spend, false, nil
	}
	ownerID = strings.TrimSpace(ownerID)
	agentID = strings.TrimSpace(agentID)
	if ownerID == "" || agentID == "" {
		return spend, false, nil
	}
	sourceRunID, _, err := rt.store.LatestActorRunMemoryEntries(ctx, ownerID, rt.TextureComputerID(), agentID, "")
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return spend, false, nil
		}
		return spend, false, err
	}
	spend.SourceRunID = sourceRunID
	eventsForRun, err := rt.store.ListEvents(ctx, sourceRunID, 5000)
	if err != nil {
		return spend, false, err
	}
	providerCallsFromPreflight := 0
	for _, event := range eventsForRun {
		if event.Kind != types.EventRunProgress {
			continue
		}
		switch event.Phase {
		case "provider_call":
			providerCallsFromPreflight++
		case "tool_loop_budget_usage", "tool_loop_budget":
			var payload map[string]any
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				continue
			}
			spend.ObservedUsageEvent = true
			if value := metadataIntValue(payload, "provider_calls"); value > spend.ProviderCalls {
				spend.ProviderCalls = value
			}
			if value := metadataIntValue(payload, "input_tokens"); value > spend.InputTokens {
				spend.InputTokens = value
			}
			if value := metadataIntValue(payload, "output_tokens"); value > spend.OutputTokens {
				spend.OutputTokens = value
			}
		}
	}
	if spend.ProviderCalls == 0 && providerCallsFromPreflight > 0 {
		spend.ProviderCalls = providerCallsFromPreflight
	}
	return spend, true, nil
}

// TextureActorOccurrenceVersion is the canonical mailbox identity format for
// Store-owned Texture producer reports and owner instructions. The encoded
// form is a versioned, length-prefixed tuple rather than an authored delimiter
// string, so every field participates injectively in actor-log deduplication.
const TextureActorOccurrenceVersion = "choir:texture-actor-occurrence:v1"

const (
	TextureActorOccurrenceProducerReport   = "producer_report"
	TextureActorOccurrenceOwnerInstruction = "owner_instruction"
)

// TextureActorOccurrence is an authenticated pointer to exactly one canonical
// lifecycle row. Recovery fields are projections of canonical run-memory/head
// state; they create a new deterministic wake after an older occurrence was
// processed without settling its Store row, but never change trigger authority.
type TextureActorOccurrence struct {
	Version          string
	Kind             string
	OwnerID          string
	ComputerID       string
	TrajectoryID     string
	DocumentID       string
	TargetAgentID    string
	TargetWorkItemID string
	ProducerAgentID  string
	UpdateID         string
	ProducerUpdateID string
	ProducerWorkID   string
	InstructionID    string
	RequestID        string
	HeadRevisionID   string
	InstructionKind  string
	LifecycleVersion int64
	ReducerSeq       int64
	MessageSeq       int64
	RecoveryRunID    string
	RecoveryTailID   string
	RecoveryHeadID   string
	RecoveryMutation string
	// ResolvedTargetWorkItemID is populated only after Store authentication by
	// Texture ownership. It is deliberately excluded from the encoded identity.
	ResolvedTargetWorkItemID string `json:"-"`
}

func appendTextureOccurrenceField(dst []byte, value string) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(value)))
	return append(dst, value...)
}

func readTextureOccurrenceField(raw []byte, at *int) (string, error) {
	if *at >= len(raw) {
		return "", fmt.Errorf("texture actor occurrence: truncated field")
	}
	n, used := binary.Uvarint(raw[*at:])
	if used <= 0 {
		return "", fmt.Errorf("texture actor occurrence: invalid field length")
	}
	*at += used
	if n > uint64(len(raw)-*at) {
		return "", fmt.Errorf("texture actor occurrence: truncated field value")
	}
	value := string(raw[*at : *at+int(n)])
	*at += int(n)
	return value, nil
}

func (o TextureActorOccurrence) fields() []string {
	return []string{
		o.Version, o.Kind, o.OwnerID, o.ComputerID, o.TrajectoryID,
		o.DocumentID, o.TargetAgentID, o.TargetWorkItemID,
		o.ProducerAgentID, o.UpdateID, o.ProducerUpdateID, o.ProducerWorkID,
		o.InstructionID, o.RequestID, o.HeadRevisionID, o.InstructionKind,
		fmt.Sprintf("%d", o.LifecycleVersion), fmt.Sprintf("%d", o.ReducerSeq), fmt.Sprintf("%d", o.MessageSeq),
		o.RecoveryRunID, o.RecoveryTailID, o.RecoveryHeadID, o.RecoveryMutation,
	}
}

// EncodeTextureActorOccurrence returns the complete deterministic occurrence
// identity. It is safe to persist as actor.Update.Content.
func EncodeTextureActorOccurrence(o TextureActorOccurrence) (string, error) {
	o.Version = strings.TrimSpace(o.Version)
	if o.Version == "" {
		o.Version = TextureActorOccurrenceVersion
	}
	if o.Version != TextureActorOccurrenceVersion || strings.TrimSpace(o.Kind) == "" ||
		strings.TrimSpace(o.OwnerID) == "" || strings.TrimSpace(o.ComputerID) == "" ||
		strings.TrimSpace(o.TrajectoryID) == "" || strings.TrimSpace(o.TargetAgentID) == "" {
		return "", fmt.Errorf("texture actor occurrence: incomplete versioned scope")
	}
	raw := make([]byte, 0, 512)
	for _, field := range o.fields() {
		raw = appendTextureOccurrenceField(raw, strings.TrimSpace(field))
	}
	return "texture-occurrence:" + base64.RawURLEncoding.EncodeToString(raw), nil
}

// DecodeTextureActorOccurrence decodes the injective tuple and rejects trailing
// or malformed fields. Callers must still reload and compare the canonical row.
func DecodeTextureActorOccurrence(content string) (TextureActorOccurrence, error) {
	var o TextureActorOccurrence
	const prefix = "texture-occurrence:"
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, prefix) {
		return o, fmt.Errorf("texture actor occurrence: unsupported identity")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(content, prefix))
	if err != nil {
		return o, fmt.Errorf("texture actor occurrence: decode: %w", err)
	}
	fields := make([]string, 23)
	at := 0
	for i := range fields {
		fields[i], err = readTextureOccurrenceField(raw, &at)
		if err != nil {
			return o, err
		}
	}
	if at != len(raw) {
		return o, fmt.Errorf("texture actor occurrence: trailing bytes")
	}
	parseInt := func(value string) (int64, error) {
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil || strconv.FormatInt(n, 10) != value {
			return 0, fmt.Errorf("noncanonical decimal %q", value)
		}
		return n, nil
	}
	o = TextureActorOccurrence{
		Version: fields[0], Kind: fields[1], OwnerID: fields[2], ComputerID: fields[3], TrajectoryID: fields[4],
		DocumentID: fields[5], TargetAgentID: fields[6], TargetWorkItemID: fields[7], ProducerAgentID: fields[8],
		UpdateID: fields[9], ProducerUpdateID: fields[10], ProducerWorkID: fields[11], InstructionID: fields[12],
		RequestID: fields[13], HeadRevisionID: fields[14], InstructionKind: fields[15], RecoveryRunID: fields[19],
		RecoveryTailID: fields[20], RecoveryHeadID: fields[21], RecoveryMutation: fields[22],
	}
	if o.LifecycleVersion, err = parseInt(fields[16]); err != nil {
		return TextureActorOccurrence{}, fmt.Errorf("texture actor occurrence: lifecycle version: %w", err)
	}
	if o.ReducerSeq, err = parseInt(fields[17]); err != nil {
		return TextureActorOccurrence{}, fmt.Errorf("texture actor occurrence: reducer sequence: %w", err)
	}
	if o.MessageSeq, err = parseInt(fields[18]); err != nil {
		return TextureActorOccurrence{}, fmt.Errorf("texture actor occurrence: message sequence: %w", err)
	}
	if o.Version != TextureActorOccurrenceVersion {
		return TextureActorOccurrence{}, fmt.Errorf("texture actor occurrence: unsupported version %q", o.Version)
	}
	canonical, canonicalErr := EncodeTextureActorOccurrence(o)
	if canonicalErr != nil || canonical != content {
		return TextureActorOccurrence{}, fmt.Errorf("texture actor occurrence: noncanonical encoding")
	}
	return o, nil
}

func TextureProducerReportOccurrence(update types.CoagentSourcePacket) (TextureActorOccurrence, error) {
	o := TextureActorOccurrence{
		Version: TextureActorOccurrenceVersion, Kind: TextureActorOccurrenceProducerReport,
		OwnerID: update.OwnerID, ComputerID: update.ComputerID, TrajectoryID: update.TrajectoryID,
		DocumentID: update.ChannelID, TargetAgentID: update.TargetAgentID, TargetWorkItemID: update.TargetWorkItemID, ProducerAgentID: update.AgentID,
		UpdateID: update.UpdateID, ProducerUpdateID: update.ProducerUpdateID,
		ProducerWorkID:   firstNonEmpty(update.ProducerWorkItemID, update.WorkItemID),
		LifecycleVersion: update.LifecycleVersion, ReducerSeq: update.ReducerSeq, MessageSeq: update.MessageSeq,
	}
	if update.Direction != types.LifecyclePacketDirectionProducerReport || o.UpdateID == "" || o.ProducerUpdateID == "" || o.ProducerWorkID == "" {
		return TextureActorOccurrence{}, fmt.Errorf("texture producer occurrence: incomplete canonical producer report")
	}
	return o, nil
}

func TextureOwnerInstructionOccurrence(instruction types.LifecycleOwnerInstruction) (TextureActorOccurrence, error) {
	o := TextureActorOccurrence{
		Version: TextureActorOccurrenceVersion, Kind: TextureActorOccurrenceOwnerInstruction,
		OwnerID: instruction.OwnerID, ComputerID: instruction.ComputerID, TrajectoryID: instruction.TrajectoryID,
		DocumentID: instruction.DocumentID, TargetAgentID: instruction.TargetAgentID, TargetWorkItemID: instruction.TargetWorkItemID,
		InstructionID: instruction.InstructionID, RequestID: instruction.RequestID, HeadRevisionID: instruction.HeadRevisionID,
		InstructionKind: string(instruction.Kind), LifecycleVersion: instruction.LifecycleVersion, ReducerSeq: instruction.ReducerSeq,
	}
	if instruction.Status != types.LifecycleOwnerInstructionPending || o.InstructionID == "" || o.RequestID == "" || o.HeadRevisionID == "" {
		return TextureActorOccurrence{}, fmt.Errorf("texture owner occurrence: incomplete canonical pending instruction")
	}
	return o, nil
}

// TextureRecoveryOccurrence advances actor-log identity using only canonical
// Store projections. It retains the exact base trigger fields.
func TextureRecoveryOccurrence(base TextureActorOccurrence, runID, tailID, headID, mutation string) TextureActorOccurrence {
	base.RecoveryRunID = strings.TrimSpace(runID)
	base.RecoveryTailID = strings.TrimSpace(tailID)
	base.RecoveryHeadID = strings.TrimSpace(headID)
	base.RecoveryMutation = strings.TrimSpace(mutation)
	return base
}

const LifecycleResearcherAdmissionRecoveryPrefix = "lifecycle-researcher-admission-recovery:v1:"

type LifecycleResearcherAdmissionRecoveryControl struct {
	UpdateID         string
	LifecycleVersion int64
	ReducerSeq       int64
}

type LifecycleResearcherAdmissionRecoveryOccurrence struct {
	OwnerID, ComputerID, TrajectoryID, AgentID, RunID, LogicalKey, SourceAgentID string
	Controls                                                                     []LifecycleResearcherAdmissionRecoveryControl
}

func EncodeLifecycleResearcherAdmissionRecovery(o LifecycleResearcherAdmissionRecoveryOccurrence) (string, error) {
	fields := []string{o.OwnerID, o.ComputerID, o.TrajectoryID, o.AgentID, o.RunID, o.LogicalKey, o.SourceAgentID, fmt.Sprintf("%d", len(o.Controls))}
	for _, control := range o.Controls {
		fields = append(fields, control.UpdateID, fmt.Sprintf("%d", control.LifecycleVersion), fmt.Sprintf("%d", control.ReducerSeq))
	}
	for _, field := range fields[:7] {
		if strings.TrimSpace(field) == "" {
			return "", fmt.Errorf("lifecycle Researcher recovery occurrence: incomplete scope")
		}
	}
	if len(o.Controls) == 0 {
		return "", fmt.Errorf("lifecycle Researcher recovery occurrence: controls are required")
	}
	raw := make([]byte, 0, 512)
	for _, field := range fields {
		raw = appendTextureOccurrenceField(raw, strings.TrimSpace(field))
	}
	return LifecycleResearcherAdmissionRecoveryPrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeLifecycleResearcherAdmissionRecovery(content string) (LifecycleResearcherAdmissionRecoveryOccurrence, error) {
	var out LifecycleResearcherAdmissionRecoveryOccurrence
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, LifecycleResearcherAdmissionRecoveryPrefix) {
		return out, fmt.Errorf("lifecycle Researcher recovery occurrence: unsupported identity")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(content, LifecycleResearcherAdmissionRecoveryPrefix))
	if err != nil {
		return out, fmt.Errorf("lifecycle Researcher recovery occurrence: decode: %w", err)
	}
	at := 0
	fields := make([]string, 8)
	for i := range fields {
		fields[i], err = readTextureOccurrenceField(raw, &at)
		if err != nil {
			return out, err
		}
	}
	count64, countErr := strconv.ParseInt(fields[7], 10, 32)
	if countErr != nil || strconv.FormatInt(count64, 10) != fields[7] || count64 <= 0 || count64 > 10000 {
		return out, fmt.Errorf("lifecycle Researcher recovery occurrence: invalid control count")
	}
	count := int(count64)
	out = LifecycleResearcherAdmissionRecoveryOccurrence{OwnerID: fields[0], ComputerID: fields[1], TrajectoryID: fields[2], AgentID: fields[3], RunID: fields[4], LogicalKey: fields[5], SourceAgentID: fields[6], Controls: make([]LifecycleResearcherAdmissionRecoveryControl, count)}
	for i := 0; i < count; i++ {
		updateID, fieldErr := readTextureOccurrenceField(raw, &at)
		if fieldErr != nil {
			return LifecycleResearcherAdmissionRecoveryOccurrence{}, fieldErr
		}
		versionRaw, fieldErr := readTextureOccurrenceField(raw, &at)
		if fieldErr != nil {
			return LifecycleResearcherAdmissionRecoveryOccurrence{}, fieldErr
		}
		seqRaw, fieldErr := readTextureOccurrenceField(raw, &at)
		if fieldErr != nil {
			return LifecycleResearcherAdmissionRecoveryOccurrence{}, fieldErr
		}
		version, versionErr := strconv.ParseInt(versionRaw, 10, 64)
		if versionErr != nil || strconv.FormatInt(version, 10) != versionRaw || version <= 0 {
			return LifecycleResearcherAdmissionRecoveryOccurrence{}, fmt.Errorf("lifecycle Researcher recovery occurrence: invalid lifecycle version")
		}
		seq, seqErr := strconv.ParseInt(seqRaw, 10, 64)
		if seqErr != nil || strconv.FormatInt(seq, 10) != seqRaw || seq <= 0 {
			return LifecycleResearcherAdmissionRecoveryOccurrence{}, fmt.Errorf("lifecycle Researcher recovery occurrence: invalid reducer sequence")
		}
		if strings.TrimSpace(updateID) == "" {
			return LifecycleResearcherAdmissionRecoveryOccurrence{}, fmt.Errorf("lifecycle Researcher recovery occurrence: empty update id")
		}
		out.Controls[i] = LifecycleResearcherAdmissionRecoveryControl{UpdateID: strings.TrimSpace(updateID), LifecycleVersion: version, ReducerSeq: seq}
	}
	if at != len(raw) {
		return LifecycleResearcherAdmissionRecoveryOccurrence{}, fmt.Errorf("lifecycle Researcher recovery occurrence: trailing bytes")
	}
	canonical, canonicalErr := EncodeLifecycleResearcherAdmissionRecovery(out)
	if canonicalErr != nil || canonical != content {
		return LifecycleResearcherAdmissionRecoveryOccurrence{}, fmt.Errorf("lifecycle Researcher recovery occurrence: noncanonical encoding")
	}
	return out, nil
}

// ErrInvalidLifecycleProducerReportAuthority denotes a proved durable canonical
// mismatch. Store/CAS/read failures are deliberately not wrapped with it.
var ErrInvalidLifecycleProducerReportAuthority = errors.New("invalid lifecycle producer report authority")

func invalidLifecycleProducerReportAuthority(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidLifecycleProducerReportAuthority, fmt.Sprintf(format, args...))
}

// ValidateLifecycleProducerReportAuthority proves that an upward lifecycle
// report came from the exact Store-owned run/work and, for lifecycle
// Researchers/CoSupers, from a fingerprinted Texture-control activation. The
// legacy QueueLifecycleUpdate projection may omit ControlBindingID for these
// roles, so the run fingerprint and canonical delivered-control ledger are the
// binding authority.
func (rt *Runtime) ValidateLifecycleProducerReportAuthority(ctx context.Context, report types.CoagentSourcePacket) error {
	if rt == nil || rt.store == nil || report.Direction != types.LifecyclePacketDirectionProducerReport || report.SourceRunID == "" || report.ProducerWorkItemID == "" {
		return invalidLifecycleProducerReportAuthority("producer report authority is incomplete")
	}
	run, err := rt.GetRun(ctx, report.SourceRunID, report.OwnerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return invalidLifecycleProducerReportAuthority("producer report source run is missing")
		}
		return fmt.Errorf("load producer report source run: %w", err)
	}
	profile := agentprofile.Canonical(run.AgentProfile)
	trajectoryBound := run.TrajectoryID == report.TrajectoryID
	if profile == agentprofile.Super {
		trajectoryBound = run.TrajectoryID == "" && metadataStringValue(run.Metadata, "assignment_trajectory_id") == report.TrajectoryID
	}
	if run.RunID != report.SourceRunID || run.OwnerID != report.OwnerID || run.ComputerID != report.ComputerID || run.AgentID != report.AgentID || !trajectoryBound || run.ChannelID != report.ChannelID || !lifecycleControlWorkIDsForRun(run)[report.ProducerWorkItemID] {
		return invalidLifecycleProducerReportAuthority("producer report source run authority mismatch")
	}
	if profile == agentprofile.Super {
		if run.AgentID != persistentSuperAgentID(report.OwnerID) || report.ControlBindingID == "" || report.TargetWorkItemID == "" {
			return invalidLifecycleProducerReportAuthority("persistent Super report lacks exact control binding authority")
		}
	}
	if profile == agentprofile.CoSuper {
		assignmentID := metadataStringValue(run.Metadata, "assignment_id")
		attempt := uint64(metadataIntValue(run.Metadata, "assignment_attempt"))
		assignment, assignmentErr := rt.store.GetCoSuperAssignment(ctx, report.OwnerID, report.ComputerID, assignmentID, attempt)
		if assignmentErr != nil {
			if errors.Is(assignmentErr, store.ErrNotFound) {
				return invalidLifecycleProducerReportAuthority("CoSuper source assignment is missing")
			}
			return fmt.Errorf("load CoSuper source assignment: %w", assignmentErr)
		}
		if assignment.LifecycleVersion <= 0 || assignment.Binding.OwnerID != report.OwnerID || assignment.Binding.ComputerID != report.ComputerID || assignment.Binding.TrajectoryID != report.TrajectoryID || assignment.Binding.AssignedAgentID != report.AgentID || assignment.Binding.AssignedWorkItemID != report.ProducerWorkItemID || assignment.BoundRunID != report.SourceRunID {
			return invalidLifecycleProducerReportAuthority("CoSuper report source assignment authority mismatch")
		}
	}
	if profile == agentprofile.Researcher {
		if metadataStringValue(run.Metadata, "request_source") != "lifecycle_texture_control" || metadataStringValue(run.Metadata, lifecycleLogicalActivationKeyMetadata) == "" {
			return invalidLifecycleProducerReportAuthority("producer report source run has no lifecycle Texture-control fingerprint")
		}
		versions, versionErr := lifecycleActivationVersionsForRun(run)
		if versionErr != nil || len(versions) == 0 {
			return invalidLifecycleProducerReportAuthority("producer report source run has no exact activation versions")
		}
		bound := false
		for _, version := range versions {
			if version.TargetWorkItemID == report.ProducerWorkItemID && (report.ControlBindingID == "" || version.UpdateID == report.ControlBindingID) {
				bound = true
			}
		}
		if !bound {
			return invalidLifecycleProducerReportAuthority("producer report work is not bound by its lifecycle control fingerprint")
		}
		delivered, deliveryErr := rt.lifecycleRunHasCanonicalControlDelivery(ctx, run)
		if deliveryErr != nil {
			return fmt.Errorf("load canonical control delivery: %w", deliveryErr)
		}
		if !delivered {
			return invalidLifecycleProducerReportAuthority("producer report source run has no canonical control delivery")
		}
	}
	return nil
}

// ErrInvalidLifecycleResearcherRecovery marks a durable malformed or foreign
// structured recovery occurrence. Operational Store/read/not-ready errors must
// remain unprocessed and retry only on a distinct wake/restart.
var ErrInvalidLifecycleResearcherRecovery = errors.New("invalid lifecycle Researcher recovery occurrence")

func invalidLifecycleResearcherRecovery(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidLifecycleResearcherRecovery, fmt.Sprintf(format, args...))
}

func (rt *Runtime) lifecycleResearcherAdmissionRecoveryControls(ctx context.Context, rec *types.RunRecord) ([]types.CoagentSourcePacket, error) {
	delivered, err := rt.listPendingLifecyclePacketsDeliveredToRun(ctx, rec)
	if err != nil || len(delivered) > 0 {
		return delivered, err
	}
	versions, err := lifecycleActivationVersionsForRun(rec)
	if err != nil || len(versions) == 0 {
		return nil, invalidLifecycleResearcherRecovery("recovery has no exact activation versions: %v", err)
	}
	pending, err := rt.store.ListAllPendingLifecycleUpdates(ctx, rec.OwnerID, rec.ComputerID, rec.AgentID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]types.CoagentSourcePacket, len(pending))
	for _, control := range pending {
		byID[control.UpdateID] = control
	}
	controls := make([]types.CoagentSourcePacket, 0, len(versions))
	workByID := make(map[string]types.WorkItemRecord, len(versions))
	for _, version := range versions {
		control, ok := byID[version.UpdateID]
		if !ok || control.TrajectoryID != rec.TrajectoryID || control.TargetAgentID != rec.AgentID || control.TargetWorkItemID != version.TargetWorkItemID ||
			control.Direction != types.LifecyclePacketDirectionControl || control.Disposition != types.UpdatePending || control.DeliveredAt != nil || control.DeliveredToRunID != "" || control.LifecycleVersion != version.ControlLifecycleVersion {
			return nil, invalidLifecycleResearcherRecovery("pending control %q is not exact", version.UpdateID)
		}
		work, workErr := rt.store.GetLifecycleWorkItem(ctx, rec.OwnerID, rec.ComputerID, version.TargetWorkItemID)
		if workErr != nil || work.Status != types.WorkItemOpen || work.AssignedAgentID != rec.AgentID || work.TrajectoryID != rec.TrajectoryID || work.LifecycleVersion != version.WorkLifecycleVersion {
			if workErr != nil {
				if errors.Is(workErr, store.ErrNotFound) {
					return nil, invalidLifecycleResearcherRecovery("work %q is missing", version.TargetWorkItemID)
				}
				return nil, workErr
			}
			return nil, invalidLifecycleResearcherRecovery("work %q is not exact", version.TargetWorkItemID)
		}
		controls = append(controls, control)
		workByID[work.WorkItemID] = work
	}
	logical, failed, _, err := lifecycleActivationKeys(rec.OwnerID, rec.ComputerID, rec.TrajectoryID, rec.AgentID, metadataStringValue(rec.Metadata, lifecycleActivationBuildMetadata), controls, workByID)
	if err != nil || logical != metadataStringValue(rec.Metadata, lifecycleLogicalActivationKeyMetadata) || failed != metadataStringValue(rec.Metadata, lifecycleFailedAttemptKeyMetadata) {
		return nil, invalidLifecycleResearcherRecovery("pending fingerprint mismatch")
	}
	return controls, nil
}

// ResolveLifecycleResearcherAdmissionRecovery authenticates a distinct restart
// wake without relying on actor snapshot memory. Terminal/disposed control fate
// is a zero-provider acknowledgement; live pending authority returns the exact
// canonical run for synchronous execution.
func (rt *Runtime) ResolveLifecycleResearcherAdmissionRecovery(ctx context.Context, ownerID, computerID, agentID, content, trajectoryID, fromAgentID string) (*types.RunRecord, bool, error) {
	o, err := DecodeLifecycleResearcherAdmissionRecovery(content)
	if err != nil {
		return nil, false, invalidLifecycleResearcherRecovery("decode: %v", err)
	}
	if o.OwnerID != strings.TrimSpace(ownerID) || o.ComputerID != strings.TrimSpace(computerID) || o.AgentID != strings.TrimSpace(agentID) ||
		o.TrajectoryID != strings.TrimSpace(trajectoryID) || o.SourceAgentID != strings.TrimSpace(fromAgentID) {
		return nil, false, invalidLifecycleResearcherRecovery("envelope mismatch")
	}
	trajectory, err := rt.store.GetLifecycleTrajectory(ctx, o.OwnerID, o.ComputerID, o.TrajectoryID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, false, invalidLifecycleResearcherRecovery("trajectory is missing")
		}
		return nil, false, fmt.Errorf("load lifecycle Researcher recovery trajectory: %w", err)
	}
	if trajectory.Status != types.TrajectoryLive {
		return nil, true, nil
	}
	if _, cancelErr := rt.store.GetLifecycleCancellationIntent(ctx, o.OwnerID, o.ComputerID, o.TrajectoryID); cancelErr == nil {
		return nil, true, nil
	} else if !errors.Is(cancelErr, store.ErrNotFound) {
		return nil, false, fmt.Errorf("load lifecycle Researcher recovery cancellation intent: %w", cancelErr)
	}
	rec, err := rt.store.GetLifecycleRun(ctx, o.OwnerID, o.ComputerID, o.RunID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, false, invalidLifecycleResearcherRecovery("run is missing")
		}
		return nil, false, fmt.Errorf("load lifecycle Researcher recovery run: %w", err)
	}
	if rec.State.Terminal() {
		return nil, true, nil
	}
	if rec.State == types.RunPassivated && (metadataStringValue(rec.Metadata, "passivated_reason") == lifecycleResearcherAdmissionRetryReason || metadataStringValue(rec.Metadata, "passivated_reason") == runtimeInjectionAppendFailurePassivationReason) {
		return nil, false, fmt.Errorf("lifecycle Researcher recovery run is awaiting boot projection")
	}
	if rec.OwnerID != o.OwnerID || rec.ComputerID != o.ComputerID || rec.TrajectoryID != o.TrajectoryID || rec.AgentID != o.AgentID ||
		(rec.State != types.RunPending && rec.State != types.RunRunning) || metadataStringValue(rec.Metadata, lifecycleLogicalActivationKeyMetadata) != o.LogicalKey {
		return nil, false, invalidLifecycleResearcherRecovery("run authority mismatch")
	}
	controls, err := rt.lifecycleResearcherAdmissionRecoveryControls(ctx, &rec)
	if err != nil {
		return nil, false, fmt.Errorf("load lifecycle Researcher recovery controls: %w", err)
	}
	if len(controls) == 0 {
		return nil, true, nil
	}
	if len(controls) != len(o.Controls) {
		return nil, false, invalidLifecycleResearcherRecovery("control set changed")
	}
	for i, control := range controls {
		expected := o.Controls[i]
		if control.UpdateID != expected.UpdateID || control.LifecycleVersion != expected.LifecycleVersion || control.ReducerSeq != expected.ReducerSeq || control.AgentID != o.SourceAgentID || control.TargetAgentID != o.AgentID {
			return nil, false, invalidLifecycleResearcherRecovery("control authority mismatch")
		}
	}
	return &rec, false, nil
}
