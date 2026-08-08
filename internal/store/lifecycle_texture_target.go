package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// LifecycleTextureControlTargetRequest is the runtime-derived identity envelope
// checked before ApplyTextureTurn may include a downward lifecycle control.
// TargetWorkItemID is required for an existing Researcher continuation. It may
// be empty for the persistent-Super opener, which creates the target work in the
// same future reducer transaction.
type LifecycleTextureControlTargetRequest struct {
	OwnerID          string
	ComputerID       string
	DocumentID       string
	TrajectoryID     string
	CallerAgentID    string
	CallerRunID      string
	TargetAgentID    string
	TargetWorkItemID string
}

// LifecycleTextureControlTargetBinding contains only store-proven authority.
// A nil TargetWorkItem is valid only for a persistent-Super opener.
type LifecycleTextureControlTargetBinding struct {
	Trajectory     types.TrajectoryRecord
	Document       types.Document
	CallerRun      types.RunRecord
	CallerAgent    types.AgentRecord
	TargetAgent    types.AgentRecord
	TargetRun      *types.RunRecord
	TargetWorkItem *types.WorkItemRecord
	TargetProfile  string
}

type lifecycleTextureControlTargetReader interface {
	GetLifecycleTrajectory(context.Context, string, string, string) (types.TrajectoryRecord, error)
	GetLifecycleDocument(context.Context, string, string, string) (types.Document, error)
	GetLifecycleRun(context.Context, string, string, string) (types.RunRecord, error)
	GetLifecycleWorkItem(context.Context, string, string, string) (types.WorkItemRecord, error)
	GetAgentByScope(context.Context, string, string, string) (types.AgentRecord, error)
}

// ValidateLifecycleTextureControlTarget fail-closed validates the caller and
// target side of a forthcoming atomic Texture turn. It performs no mutation,
// queues no packet, and wakes no actor.
func (s *Store) ValidateLifecycleTextureControlTarget(ctx context.Context, req LifecycleTextureControlTargetRequest) (LifecycleTextureControlTargetBinding, error) {
	if s == nil {
		return LifecycleTextureControlTargetBinding{}, fmt.Errorf("validate lifecycle Texture control target: store is unavailable")
	}
	return validateLifecycleTextureControlTarget(ctx, s, req)
}

func validateLifecycleTextureControlTarget(ctx context.Context, reader lifecycleTextureControlTargetReader, req LifecycleTextureControlTargetRequest) (LifecycleTextureControlTargetBinding, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return LifecycleTextureControlTargetBinding{}, fmt.Errorf("validate lifecycle Texture control target: %w", err)
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.DocumentID = strings.TrimSpace(req.DocumentID)
	req.TrajectoryID = strings.TrimSpace(req.TrajectoryID)
	req.CallerAgentID = strings.TrimSpace(req.CallerAgentID)
	req.CallerRunID = strings.TrimSpace(req.CallerRunID)
	req.TargetAgentID = strings.TrimSpace(req.TargetAgentID)
	req.TargetWorkItemID = strings.TrimSpace(req.TargetWorkItemID)
	if req.DocumentID == "" || req.TrajectoryID == "" || req.CallerAgentID == "" || req.CallerRunID == "" || req.TargetAgentID == "" {
		return LifecycleTextureControlTargetBinding{}, invalidLifecycleTextureControlTarget("complete caller, document, trajectory, and target identity is required")
	}

	trajectory, err := reader.GetLifecycleTrajectory(ctx, ownerID, computerID, req.TrajectoryID)
	if err != nil {
		return LifecycleTextureControlTargetBinding{}, fmt.Errorf("validate lifecycle Texture control target: lookup trajectory: %w", err)
	}
	if trajectory.OwnerID != ownerID || trajectory.ComputerID != computerID || trajectory.TrajectoryID != req.TrajectoryID ||
		trajectory.Status != types.TrajectoryLive || strings.TrimSpace(trajectory.SubjectRefs["doc_id"]) != req.DocumentID {
		return LifecycleTextureControlTargetBinding{}, invalidLifecycleTextureControlTarget("trajectory does not bind the live document")
	}

	document, err := reader.GetLifecycleDocument(ctx, ownerID, computerID, req.DocumentID)
	if err != nil {
		return LifecycleTextureControlTargetBinding{}, fmt.Errorf("validate lifecycle Texture control target: lookup document: %w", err)
	}
	if document.DocID != req.DocumentID || document.OwnerID != ownerID || document.ComputerID != computerID || document.TrajectoryID != req.TrajectoryID {
		return LifecycleTextureControlTargetBinding{}, invalidLifecycleTextureControlTarget("document scope or trajectory binding mismatch")
	}

	callerRun, err := reader.GetLifecycleRun(ctx, ownerID, computerID, req.CallerRunID)
	if err != nil {
		return LifecycleTextureControlTargetBinding{}, fmt.Errorf("validate lifecycle Texture control target: lookup caller run: %w", err)
	}
	expectedTextureAgentID := agentprofile.Texture + ":" + req.DocumentID
	if callerRun.RunID != req.CallerRunID || callerRun.OwnerID != ownerID || callerRun.SandboxID != computerID ||
		callerRun.TrajectoryID != req.TrajectoryID || callerRun.AgentID != req.CallerAgentID || req.CallerAgentID != expectedTextureAgentID ||
		strings.TrimSpace(callerRun.AgentProfile) != agentprofile.Texture || strings.TrimSpace(callerRun.AgentRole) != agentprofile.Texture ||
		strings.TrimSpace(callerRun.ChannelID) != req.DocumentID || !callerRun.State.Active() {
		return LifecycleTextureControlTargetBinding{}, invalidLifecycleTextureControlTarget("caller run is not the active lifecycle Texture for the document")
	}

	callerAgent, err := reader.GetAgentByScope(ctx, ownerID, computerID, req.CallerAgentID)
	if err != nil {
		return LifecycleTextureControlTargetBinding{}, fmt.Errorf("validate lifecycle Texture control target: lookup caller agent: %w", err)
	}
	if callerAgent.AgentID != req.CallerAgentID || callerAgent.OwnerID != ownerID || callerAgent.ComputerID != computerID ||
		strings.TrimSpace(callerAgent.Profile) != agentprofile.Texture || strings.TrimSpace(callerAgent.Role) != agentprofile.Texture ||
		strings.TrimSpace(callerAgent.ChannelID) != req.DocumentID || callerAgent.LifecycleVersion <= 0 ||
		strings.TrimSpace(callerAgent.ActiveRunID) != req.CallerRunID {
		return LifecycleTextureControlTargetBinding{}, invalidLifecycleTextureControlTarget("caller agent does not own the current lifecycle Texture activation")
	}

	targetAgent, err := reader.GetAgentByScope(ctx, ownerID, computerID, req.TargetAgentID)
	if err != nil {
		return LifecycleTextureControlTargetBinding{}, fmt.Errorf("validate lifecycle Texture control target: lookup target agent: %w", err)
	}
	binding := LifecycleTextureControlTargetBinding{
		Trajectory: trajectory, Document: document, CallerRun: callerRun, CallerAgent: callerAgent, TargetAgent: targetAgent,
	}

	exactPersistentSuperID := agentprofile.Super + ":" + ownerID
	switch {
	case req.TargetAgentID == exactPersistentSuperID:
		if targetAgent.AgentID != exactPersistentSuperID || targetAgent.OwnerID != ownerID || targetAgent.ComputerID != computerID ||
			strings.TrimSpace(targetAgent.Profile) != agentprofile.Super || strings.TrimSpace(targetAgent.Role) != agentprofile.Super ||
			targetAgent.LifecycleVersion != 0 {
			return LifecycleTextureControlTargetBinding{}, invalidLifecycleTextureControlTarget("persistent Super target binding mismatch")
		}
		binding.TargetProfile = agentprofile.Super
	case strings.TrimSpace(targetAgent.Profile) == agentprofile.Researcher && strings.TrimSpace(targetAgent.Role) == agentprofile.Researcher:
		if targetAgent.AgentID != req.TargetAgentID || targetAgent.OwnerID != ownerID || targetAgent.ComputerID != computerID ||
			targetAgent.LifecycleVersion <= 0 || req.TargetWorkItemID == "" {
			return LifecycleTextureControlTargetBinding{}, invalidLifecycleTextureControlTarget("Researcher target requires lifecycle identity and an existing target work item")
		}
		binding.TargetProfile = agentprofile.Researcher
	default:
		return LifecycleTextureControlTargetBinding{}, invalidLifecycleTextureControlTarget("target must be a bound Researcher or the exact persistent Super")
	}

	if req.TargetWorkItemID != "" {
		work, err := reader.GetLifecycleWorkItem(ctx, ownerID, computerID, req.TargetWorkItemID)
		if err != nil {
			return LifecycleTextureControlTargetBinding{}, fmt.Errorf("validate lifecycle Texture control target: lookup target work: %w", err)
		}
		if work.WorkItemID != req.TargetWorkItemID || work.OwnerID != ownerID || work.ComputerID != computerID ||
			work.TrajectoryID != req.TrajectoryID || work.Status != types.WorkItemOpen ||
			strings.TrimSpace(work.AssignedAgentID) != req.TargetAgentID || strings.TrimSpace(work.AuthorityProfile) != binding.TargetProfile ||
			!lifecycleTargetWorkRequestedByTexture(work, req.CallerRunID, req.CallerAgentID) {
			return LifecycleTextureControlTargetBinding{}, invalidLifecycleTextureControlTarget("target work does not bind the open target obligation and caller provenance")
		}
		binding.TargetWorkItem = &work
	}
	if binding.TargetProfile == agentprofile.Researcher {
		activeRunID := strings.TrimSpace(targetAgent.ActiveRunID)
		if activeRunID != "" {
			targetRun, err := reader.GetLifecycleRun(ctx, ownerID, computerID, activeRunID)
			if err != nil {
				return LifecycleTextureControlTargetBinding{}, fmt.Errorf("validate lifecycle Texture control target: lookup target run: %w", err)
			}
			workItemIDs, bindingErr := lifecycleActivationWorkItemIDs(targetRun.Metadata)
			if bindingErr != nil || targetRun.RunID != activeRunID || targetRun.OwnerID != ownerID || targetRun.SandboxID != computerID ||
				targetRun.TrajectoryID != req.TrajectoryID || targetRun.AgentID != req.TargetAgentID ||
				strings.TrimSpace(targetRun.AgentProfile) != agentprofile.Researcher || strings.TrimSpace(targetRun.AgentRole) != agentprofile.Researcher ||
				!targetRun.State.Active() || !containsLifecycleIdentity(workItemIDs, req.TargetWorkItemID) {
				return LifecycleTextureControlTargetBinding{}, invalidLifecycleTextureControlTarget("active Researcher run does not bind the target work")
			}
			binding.TargetRun = &targetRun
		}
	}
	return binding, nil
}

func containsLifecycleIdentity(values []string, wanted string) bool {
	wanted = strings.TrimSpace(wanted)
	for _, value := range values {
		if strings.TrimSpace(value) == wanted {
			return true
		}
	}
	return false
}

func lifecycleTargetWorkRequestedByTexture(work types.WorkItemRecord, callerRunID, callerAgentID string) bool {
	return strings.TrimSpace(work.CreatedByRunID) == strings.TrimSpace(callerRunID) &&
		workDetailString(work.Details, "requested_by_profile") == agentprofile.Texture &&
		workDetailString(work.Details, "requested_by_agent_id") == strings.TrimSpace(callerAgentID) &&
		workDetailString(work.Details, "requested_by_run_id") == strings.TrimSpace(callerRunID)
}

func workDetailString(details map[string]any, key string) string {
	value, _ := details[key].(string)
	return strings.TrimSpace(value)
}

// ResolveLifecyclePacketWorkBindings returns the single producer-work alias and
// target work selected by a packet direction. WorkItemID remains a read alias
// for legacy producer reports; it is never a second writable producer field.
func ResolveLifecyclePacketWorkBindings(packet types.CoagentSourcePacket) (producerWorkItemID, targetWorkItemID string, err error) {
	legacyProducer := strings.TrimSpace(packet.WorkItemID)
	producer := strings.TrimSpace(packet.ProducerWorkItemID)
	target := strings.TrimSpace(packet.TargetWorkItemID)
	if legacyProducer != "" && producer != "" && legacyProducer != producer {
		return "", "", fmt.Errorf("lifecycle packet producer work aliases disagree: %w", ErrLifecycleInvalidTransition)
	}
	if producer == "" {
		producer = legacyProducer
	}
	switch packet.Direction {
	case "":
		if packet.ProducerWorkItemID != "" || target != "" {
			return "", "", fmt.Errorf("legacy lifecycle packet cannot carry direction-specific work: %w", ErrLifecycleInvalidTransition)
		}
		return producer, "", nil
	case types.LifecyclePacketDirectionProducerReport:
		if target != "" {
			return "", "", fmt.Errorf("producer report cannot carry target work: %w", ErrLifecycleInvalidTransition)
		}
		return producer, "", nil
	case types.LifecyclePacketDirectionControl:
		if producer != "" || target == "" || packet.WorkDisposition != "" {
			return "", "", fmt.Errorf("lifecycle control requires only target work: %w", ErrLifecycleInvalidTransition)
		}
		return "", target, nil
	default:
		return "", "", fmt.Errorf("unknown lifecycle packet direction %q: %w", packet.Direction, ErrLifecycleInvalidTransition)
	}
}

func invalidLifecycleTextureControlTarget(reason string) error {
	return fmt.Errorf("validate lifecycle Texture control target: %s: %w", reason, ErrLifecycleInvalidTransition)
}
