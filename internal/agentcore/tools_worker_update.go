package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func RegisterCoagentUpdateTools(registry *toolregistry.ToolRegistry, rt *Runtime) error {
	return registry.Register(newUpdateCoagentTool(rt))
}

type submitCoagentUpdateArgs struct {
	AgentID         string `json:"agent_id"`
	ChannelID       string `json:"channel_id,omitempty"`
	WorkItemID      string `json:"work_item_id,omitempty"`
	WorkDisposition string `json:"work_disposition,omitempty"`
	types.CoagentSourcePacketPayload
}

func newUpdateCoagentTool(rt *Runtime) toolregistry.Tool {
	return toolregistry.Tool{
		Name:        "update_coagent",
		Description: "Send one source packet to the explicit agent_id durably bound to this run. The target must be an allowed exact requester, owning parent, or assigned child in the same owner, computer, trajectory, and document scope; channel or metadata hints never select a target. Lifecycle Researcher, Processor, and Reconciler reports use the lifecycle backlog and require runtime call identity plus assigned work. Pre-cutover Super and CoSuper result/assignment paths use the legacy backlog only when durable run and assignment rows prove the relationship. Wake occurs only after commit.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"schema_version":   map[string]any{"type": "string", "enum": []string{types.CoagentSourcePacketSchemaV1}},
			"kind":             map[string]any{"type": "string", "enum": []string{"evidence_update", "execution_request", "execution_result", "blocker", "question", "proposal", "decision_request"}},
			"summary":          map[string]any{"type": "string"},
			"agent_id":         map[string]any{"type": "string", "description": "Required exact durable target agent id. It is never inferred from channel, caller, or requester metadata."},
			"work_item_id":     map[string]any{"type": "string", "description": "Assigned lifecycle work item addressed by this update. Required when the activation carries multiple work_item_ids; if the activation carries one item, omission selects that item."},
			"channel_id":       map[string]any{"type": "string", "description": "Optional equality assertion against the loaded target channel; never target authority."},
			"work_disposition": map[string]any{"type": "string", "enum": []string{"open", "completed"}, "description": "Optional native producer work consequence for lifecycle updates; omission preserves assigned work as open. Use completed only when this update fully satisfies that work."},
			"claims": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"claim_id":            map[string]any{"type": "string"},
						"text":                map[string]any{"type": "string"},
						"source_ids":          stringArraySchema(),
						"stance":              map[string]any{"type": "string", "enum": []string{"supports", "qualifies", "contradicts", "background"}},
						"recommended_surface": map[string]any{"type": "string", "enum": []string{"inline_ref", "block_embed", "source_panel", "decision_log"}},
					},
					"required":             []string{"text"},
					"additionalProperties": false,
				},
			},
			"sources": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"source_id": map[string]any{"type": "string"},
						"kind":      map[string]any{"type": "string", "enum": sourcecontract.SourceKindValues()},
						"target": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"uri":        map[string]any{"type": "string"},
								"title":      map[string]any{"type": "string"},
								"media_type": map[string]any{"type": "string"},
							},
							"required":             []string{"uri"},
							"additionalProperties": false,
						},
						"selectors": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"kind":   map[string]any{"type": "string", "enum": sourcecontract.SelectorKindValues()},
									"quote":  map[string]any{"type": "string"},
									"start":  map[string]any{"type": "integer"},
									"end":    map[string]any{"type": "integer"},
									"x":      map[string]any{"type": "number"},
									"y":      map[string]any{"type": "number"},
									"width":  map[string]any{"type": "number"},
									"height": map[string]any{"type": "number"},
								},
								"required":             []string{"kind"},
								"additionalProperties": false,
							},
						},
						"excerpt": map[string]any{
							"type":        "string",
							"description": "Bounded source text to show in Texture source_ref transclusions. Use this for the short source stub when the researcher has read source content.",
						},
						"reader_snapshot": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"text_content":        map[string]any{"type": "string"},
								"snapshot_kind":       map[string]any{"type": "string"},
								"media_type":          map[string]any{"type": "string"},
								"original_media_type": map[string]any{"type": "string"},
								"source_url":          map[string]any{"type": "string"},
								"access_scope":        map[string]any{"type": "string"},
								"truncated":           map[string]any{"type": "boolean"},
							},
							"additionalProperties": false,
						},
						"evidence": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"state":        map[string]any{"type": "string", "enum": []string{"available", "pending", "blocked", "unavailable"}},
								"confidence":   map[string]any{"type": "string", "enum": []string{"low", "medium", "high"}},
								"rights_scope": map[string]any{"type": "string"},
							},
							"additionalProperties": false,
						},
					},
					"required":             []string{"kind", "target"},
					"additionalProperties": false,
				},
			},
			"actions": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action_id": map[string]any{"type": "string"},
						"type":      map[string]any{"type": "string", "enum": []string{"run_command", "inspect_file", "produce_diff", "run_tests", "open_browser", "import_source", "revise_texture"}},
						"objective": map[string]any{"type": "string"},
						"inputs":    map[string]any{"type": "object"},
						"expected_sources": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"kind":     map[string]any{"type": "string", "enum": sourcecontract.SourceKindValues()},
									"required": map[string]any{"type": "boolean"},
								},
								"required":             []string{"kind"},
								"additionalProperties": false,
							},
						},
						"safety": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"mutation_class": map[string]any{"type": "string", "enum": []string{"green", "yellow", "orange", "red", "black"}},
								"network":        map[string]any{"type": "string", "enum": []string{"forbidden", "allowed", "required"}},
								"file_mutation":  map[string]any{"type": "string", "enum": []string{"forbidden", "allowed", "required"}},
							},
							"additionalProperties": false,
						},
					},
					"required":             []string{"type", "objective"},
					"additionalProperties": false,
				},
			},
			"questions": stringArraySchema(),
			"notes":     stringArraySchema(),
		}, []string{"schema_version", "kind", "summary", "agent_id"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			if err := rejectLegacyUpdateCoagentFields(raw); err != nil {
				return "", err
			}
			var in submitCoagentUpdateArgs
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode update_coagent args: %w", err)
			}
			packet := normalizeCoagentSourcePacketPayload(in.CoagentSourcePacketPayload)
			if err := validateCoagentSourcePacketPayload(packet); err != nil {
				return "", err
			}
			authority, err := resolveCoagentUpdateAuthority(ctx, rt, strings.TrimSpace(in.AgentID), strings.TrimSpace(in.WorkItemID))
			if err != nil {
				return "", err
			}
			if assertedChannel := strings.TrimSpace(in.ChannelID); assertedChannel != "" && assertedChannel != authority.target.ChannelID {
				return "", fmt.Errorf("update_coagent channel_id %q does not match loaded target %q channel %q", assertedChannel, authority.target.AgentID, authority.target.ChannelID)
			}

			workDisposition := strings.TrimSpace(in.WorkDisposition)
			update := types.CoagentSourcePacket{
				OwnerID:         authority.callerRun.OwnerID,
				ComputerID:      authority.callerRun.SandboxID,
				AgentID:         authority.callerRun.AgentID,
				TargetAgentID:   authority.target.AgentID,
				ChannelID:       authority.target.ChannelID,
				TrajectoryID:    authority.trajectoryID,
				Role:            authority.callerProfile,
				SourceRunID:     authority.callerRun.RunID,
				Packet:          packet,
				CreatedAt:       time.Now().UTC(),
				WorkDisposition: types.WorkItemStatus(workDisposition),
			}
			if authority.lifecycle {
				producerUpdateID, deriveErr := deriveLifecycleProducerUpdateID(toolregistry.ExecutionContextFrom(ctx), authority.callerRun)
				if deriveErr != nil {
					return "", deriveErr
				}
				if workDisposition == "" {
					workDisposition = string(types.WorkItemOpen)
					update.WorkDisposition = types.WorkItemOpen
				}
				update.ProducerUpdateID = producerUpdateID
				update.UpdateID = deriveLifecycleWorkerUpdateID(update, producerUpdateID)
			} else {
				if workDisposition != "" || strings.TrimSpace(in.WorkItemID) != "" {
					return "", fmt.Errorf("update_coagent pre-cutover update does not accept lifecycle producer/work fields")
				}
				update.UpdateID = deriveWorkerUpdateID(update)
			}
			update.Content = buildWorkerUpdateMessage(update)

			message := &types.ChannelMessage{
				ChannelID: update.ChannelID, From: update.SourceRunID,
				FromAgentID: update.AgentID, FromRunID: update.SourceRunID,
				ToAgentID: update.TargetAgentID, TrajectoryID: update.TrajectoryID,
				Role: update.Role, Content: update.Content, Timestamp: update.CreatedAt,
			}
			var stored types.CoagentSourcePacket
			var created bool
			if authority.lifecycle {
				payloadDigest, digestErr := store.ComputeLifecycleUpdatePayloadDigest(update.Packet, update.Content)
				if digestErr != nil {
					return "", digestErr
				}
				queue := types.QueueLifecycleUpdateRequest{
					OwnerID: update.OwnerID, ComputerID: update.ComputerID,
					CommandID: "lifecycle-queue:" + update.UpdateID, TrajectoryID: update.TrajectoryID,
					TargetAgentID: update.TargetAgentID, ProducerAgentID: update.AgentID,
					ProducerUpdateID: update.ProducerUpdateID, UpdateID: update.UpdateID,
					ChannelID: update.ChannelID, Role: update.Role, SourceRunID: update.SourceRunID,
					Packet: update.Packet, Content: update.Content, PayloadDigest: payloadDigest,
					WorkDisposition: types.WorkItemStatus(workDisposition), WorkItemID: authority.workItemID,
				}
				queue.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(queue)
				queued, queueErr := rt.store.QueueLifecycleUpdate(ctx, queue)
				if queueErr != nil {
					return "", fmt.Errorf("queue durable lifecycle update: %w", queueErr)
				}
				if queued.Update == nil {
					return "", fmt.Errorf("queue durable lifecycle update: reducer returned no update projection")
				}
				stored, created = *queued.Update, !queued.Replay
				if stored.Disposition == types.UpdatePending && created {
					rt.emitChannelMessageEvent(ctx, *message, update.OwnerID)
					// QueueLifecycleUpdate committed a new packet before the wake is attempted.
					rt.wakeUpdatedCoagent(ctx, stored)
				}
			} else {
				stored, created, err = rt.store.DispatchWorkerUpdate(ctx, update, message)
				if err != nil {
					return "", err
				}
			}
			if stored.Disposition == "" && !created {
				if err := validateExistingWorkerUpdate(stored, update); err != nil {
					return "", err
				}
			}
			if stored.Disposition == "" && created {
				rt.emitChannelMessageEvent(ctx, *message, update.OwnerID)
				// DispatchWorkerUpdate committed before the wake is attempted.
				rt.wakeUpdatedCoagent(ctx, stored)
			}

			return toolregistry.ResultJSON(map[string]any{
				"update_id": stored.UpdateID, "agent_id": stored.TargetAgentID,
				"channel_id": stored.ChannelID, "cursor": stored.MessageSeq,
				"trajectory_id": stored.TrajectoryID,
				"status":        map[bool]string{true: "submitted", false: "existing"}[created],
			})
		},
	}
}

type coagentUpdateAuthorityStore interface {
	GetAgentByScope(context.Context, string, string, string) (types.AgentRecord, error)
	GetLifecycleRun(context.Context, string, string, string) (types.RunRecord, error)
	GetRunByOwner(context.Context, string, string) (types.RunRecord, error)
	GetLifecycleTrajectory(context.Context, string, string, string) (types.TrajectoryRecord, error)
	GetLifecycleWorkItem(context.Context, string, string, string) (types.WorkItemRecord, error)
	GetTrajectory(context.Context, string, string) (types.TrajectoryRecord, error)
	GetWorkItem(context.Context, string, string) (types.WorkItemRecord, error)
	CoSuperSlotByAgentAndTrajectory(context.Context, string, string, string) (store.CoSuperSlotRecord, bool, error)
}

type coagentUpdateAuthority struct {
	callerRun     types.RunRecord
	callerAgent   types.AgentRecord
	target        types.AgentRecord
	callerProfile string
	targetProfile string
	trajectoryID  string
	workItemID    string
	lifecycle     bool
}

func resolveCoagentUpdateAuthority(ctx context.Context, rt *Runtime, explicitTargetAgentID, requestedWorkItemID string) (coagentUpdateAuthority, error) {
	if rt == nil || rt.store == nil {
		return coagentUpdateAuthority{}, fmt.Errorf("update_coagent authority store is unavailable")
	}
	return resolveCoagentUpdateAuthorityWithStore(ctx, rt, rt.store, explicitTargetAgentID, requestedWorkItemID)
}

func resolveCoagentUpdateAuthorityWithStore(ctx context.Context, rt *Runtime, authorityStore coagentUpdateAuthorityStore, explicitTargetAgentID, requestedWorkItemID string) (coagentUpdateAuthority, error) {
	var authority coagentUpdateAuthority
	if rt == nil || authorityStore == nil {
		return authority, fmt.Errorf("update_coagent authority store is unavailable")
	}
	execution := toolregistry.ExecutionContextFrom(ctx)
	if explicitTargetAgentID == "" {
		return authority, fmt.Errorf("update_coagent requires explicit agent_id; channel, caller, and requester hints are not target authority")
	}
	ownerID := strings.TrimSpace(execution.OwnerID)
	computerID := strings.TrimSpace(execution.SandboxID)
	callerAgentID := strings.TrimSpace(execution.AgentID)
	callerRunID := strings.TrimSpace(execution.RunID)
	if ownerID == "" || computerID == "" || callerAgentID == "" || callerRunID == "" || execution.RunRecord == nil {
		return authority, fmt.Errorf("update_coagent missing owner/computer/agent/run authority")
	}
	if configuredComputerID := strings.TrimSpace(rt.TextureSandboxID()); configuredComputerID == "" || configuredComputerID != computerID {
		return authority, fmt.Errorf("update_coagent caller computer does not match runtime computer")
	}

	target, err := authorityStore.GetAgentByScope(ctx, ownerID, computerID, explicitTargetAgentID)
	if err != nil {
		return authority, fmt.Errorf("resolve explicit update_coagent target: %w", err)
	}
	if strings.TrimSpace(target.AgentID) != explicitTargetAgentID || target.OwnerID != ownerID || target.ComputerID != computerID {
		return authority, fmt.Errorf("resolve explicit update_coagent target: durable scope mismatch")
	}
	targetProfile := agentprofile.Canonical(target.Profile)
	if targetProfile == "" || agentprofile.Canonical(target.Role) != targetProfile || strings.TrimSpace(target.ChannelID) == "" {
		return authority, fmt.Errorf("resolve explicit update_coagent target: profile, role, and channel must be canonical")
	}
	authority.target, authority.targetProfile = target, targetProfile

	callerAgent, err := authorityStore.GetAgentByScope(ctx, ownerID, computerID, callerAgentID)
	if err != nil {
		return authority, fmt.Errorf("resolve update_coagent caller agent: %w", err)
	}
	callerProfile := agentprofile.Canonical(callerAgent.Profile)
	if callerProfile == "" || agentprofile.Canonical(callerAgent.Role) != callerProfile {
		return authority, fmt.Errorf("resolve update_coagent caller agent: profile/role mismatch")
	}
	if executionProfile := agentprofile.Canonical(execution.Profile); executionProfile == "" || executionProfile != callerProfile {
		return authority, fmt.Errorf("resolve update_coagent caller agent: execution profile mismatch")
	}
	authority.callerAgent, authority.callerProfile = callerAgent, callerProfile
	if err := enforceCoagentUpdateAuthorityWithStore(ctx, rt, authorityStore, target, targetProfile); err != nil {
		return authority, err
	}

	lifecycleRun, lifecycleErr := authorityStore.GetLifecycleRun(ctx, ownerID, computerID, callerRunID)
	if lifecycleErr == nil {
		authority.lifecycle = true
		authority.callerRun = lifecycleRun
	} else {
		if !errors.Is(lifecycleErr, store.ErrNotFound) {
			return authority, fmt.Errorf("resolve lifecycle caller run: %w", lifecycleErr)
		}
		legacyRun, legacyErr := authorityStore.GetRunByOwner(ctx, ownerID, callerRunID)
		if legacyErr != nil {
			return authority, fmt.Errorf("resolve pre-cutover caller run after lifecycle miss: %w", legacyErr)
		}
		authority.callerRun = legacyRun
	}
	if err := validateLoadedCallerRun(execution, authority.callerRun, callerProfile, computerID); err != nil {
		return authority, err
	}
	authority.trajectoryID = strings.TrimSpace(trajectoryIDForRun(&authority.callerRun))

	if authority.lifecycle {
		if err := validateLifecycleCoagentUpdateAuthority(ctx, authorityStore, &authority, requestedWorkItemID); err != nil {
			return authority, err
		}
		return authority, nil
	}
	if err := validatePreCutoverCoagentUpdateAuthority(ctx, authorityStore, &authority); err != nil {
		return authority, err
	}
	return authority, nil
}

func validateLoadedCallerRun(execution toolregistry.ExecutionContext, run types.RunRecord, profile, computerID string) error {
	if run.RunID != strings.TrimSpace(execution.RunID) || run.AgentID != strings.TrimSpace(execution.AgentID) ||
		run.OwnerID != strings.TrimSpace(execution.OwnerID) || run.SandboxID != computerID ||
		agentprofile.Canonical(configuredAgentProfileForRun(&run)) != profile ||
		agentprofile.Canonical(agentRoleForRun(&run)) != profile {
		return fmt.Errorf("update_coagent durable caller run does not match execution identity")
	}
	provided := execution.RunRecord
	if provided == nil || provided.RunID != run.RunID || provided.AgentID != run.AgentID ||
		provided.OwnerID != run.OwnerID || provided.SandboxID != run.SandboxID ||
		strings.TrimSpace(trajectoryIDForRun(provided)) != strings.TrimSpace(trajectoryIDForRun(&run)) {
		return fmt.Errorf("update_coagent caller run context does not match durable run")
	}
	return nil
}

func enforceCoagentUpdateAuthority(ctx context.Context, rt *Runtime, target types.AgentRecord, targetProfile string) error {
	if rt == nil || rt.store == nil {
		return fmt.Errorf("update_coagent authority store is unavailable")
	}
	return enforceCoagentUpdateAuthorityWithStore(ctx, rt, rt.store, target, targetProfile)
}

func enforceCoagentUpdateAuthorityWithStore(ctx context.Context, rt *Runtime, authorityStore coagentUpdateAuthorityStore, target types.AgentRecord, targetProfile string) error {
	if rt == nil || authorityStore == nil {
		return fmt.Errorf("update_coagent authority store is unavailable")
	}
	execution := toolregistry.ExecutionContextFrom(ctx)
	ownerID := strings.TrimSpace(execution.OwnerID)
	computerID := strings.TrimSpace(execution.SandboxID)
	callerAgentID := strings.TrimSpace(execution.AgentID)
	callerRunID := strings.TrimSpace(execution.RunID)
	callerProfile := agentprofile.Canonical(execution.Profile)
	targetProfile = agentprofile.Canonical(targetProfile)
	if ownerID == "" || computerID == "" || callerAgentID == "" || callerRunID == "" || execution.RunRecord == nil {
		return fmt.Errorf("update_coagent missing owner/computer/agent/run authority")
	}
	if target.AgentID == "" || target.OwnerID != ownerID || target.ComputerID != computerID || targetProfile == "" {
		return fmt.Errorf("update_coagent target scope/profile is not authoritative")
	}
	if !agentprofile.CanMessage(callerProfile, targetProfile) {
		if targetProfile == agentprofile.Email {
			return fmt.Errorf("update_coagent %s cannot message %s; route owner intent through Texture request_email_draft artifact handoff", callerProfile, targetProfile)
		}
		return fmt.Errorf("update_coagent %s cannot message %s", callerProfile, targetProfile)
	}
	if callerProfile == agentprofile.Super && targetProfile == agentprofile.CoSuper {
		trajectoryID := strings.TrimSpace(trajectoryIDForRun(execution.RunRecord))
		if trajectoryID == "" {
			return fmt.Errorf("update_coagent super to co-super requires caller trajectory")
		}
		slot, found, err := authorityStore.CoSuperSlotByAgentAndTrajectory(ctx, ownerID, trajectoryID, target.AgentID)
		if err != nil {
			return fmt.Errorf("lookup co-super assignment slot: %w", err)
		}
		if !found || strings.TrimSpace(slot.RunID) == "" || strings.TrimSpace(slot.RequestedByRunID) != callerRunID {
			return fmt.Errorf("update_coagent co-super target is not assigned to the calling super")
		}
	}
	return nil
}

func validateLifecycleCoagentUpdateAuthority(ctx context.Context, authorityStore coagentUpdateAuthorityStore, authority *coagentUpdateAuthority, requestedWorkItemID string) error {
	if authority == nil {
		return fmt.Errorf("update_coagent lifecycle authority is missing")
	}
	switch authority.callerProfile {
	case agentprofile.Researcher, agentprofile.Processor, agentprofile.Reconciler:
	default:
		return fmt.Errorf("update_coagent lifecycle producer profile %s is not allowed", authority.callerProfile)
	}
	if authority.targetProfile != agentprofile.Texture || authority.target.LifecycleVersion <= 0 {
		return fmt.Errorf("update_coagent lifecycle report requires a lifecycle Texture target")
	}
	trajectoryID := strings.TrimSpace(authority.trajectoryID)
	if trajectoryID == "" {
		return fmt.Errorf("update_coagent lifecycle caller trajectory is required")
	}
	trajectory, err := authorityStore.GetLifecycleTrajectory(ctx, authority.callerRun.OwnerID, authority.callerRun.SandboxID, trajectoryID)
	if err != nil {
		return fmt.Errorf("resolve lifecycle caller trajectory: %w", err)
	}
	if trajectory.TrajectoryID != trajectoryID || trajectory.OwnerID != authority.callerRun.OwnerID || trajectory.ComputerID != authority.callerRun.SandboxID {
		return fmt.Errorf("update_coagent lifecycle trajectory scope mismatch")
	}
	docID := strings.TrimSpace(trajectory.SubjectRefs["doc_id"])
	if docID == "" || authority.target.AgentID != currentTextureAgentID(docID) || authority.target.ChannelID != docID {
		return fmt.Errorf("update_coagent lifecycle target does not match trajectory document")
	}
	parent, err := loadLifecycleRequesterRun(ctx, authorityStore, authority.callerRun, authority.target)
	if err != nil {
		return err
	}
	workItemID, err := lifecycleAssignedWorkItemID(authority.callerRun, requestedWorkItemID)
	if err != nil {
		return err
	}
	work, err := authorityStore.GetLifecycleWorkItem(ctx, authority.callerRun.OwnerID, authority.callerRun.SandboxID, workItemID)
	if err != nil {
		return fmt.Errorf("resolve lifecycle producer work: %w", err)
	}
	if work.OwnerID != authority.callerRun.OwnerID || work.ComputerID != authority.callerRun.SandboxID ||
		work.TrajectoryID != trajectoryID || work.Status != types.WorkItemOpen || work.AssignedAgentID != authority.callerRun.AgentID ||
		agentprofile.Canonical(work.AuthorityProfile) != authority.callerProfile {
		return fmt.Errorf("update_coagent lifecycle producer work binding mismatch")
	}
	if metadataStringValue(work.Details, "requested_by_agent_id") != authority.target.AgentID ||
		metadataStringValue(work.Details, "requested_by_run_id") != parent.RunID ||
		agentprofile.Canonical(metadataStringValue(work.Details, "requested_by_profile")) != agentprofile.Texture {
		return fmt.Errorf("update_coagent lifecycle work lacks exact requesting Texture provenance")
	}
	authority.workItemID = workItemID
	return nil
}

func loadLifecycleRequesterRun(ctx context.Context, authorityStore coagentUpdateAuthorityStore, caller types.RunRecord, target types.AgentRecord) (types.RunRecord, error) {
	requesterAgentID := metadataStringValue(caller.Metadata, "requested_by_agent_id")
	requesterProfile := agentprofile.Canonical(metadataStringValue(caller.Metadata, "requested_by_profile"))
	requesterRunID, err := exactRequesterRunID(caller)
	if err != nil {
		return types.RunRecord{}, err
	}
	if requesterAgentID != target.AgentID || requesterProfile != agentprofile.Texture {
		return types.RunRecord{}, fmt.Errorf("update_coagent lifecycle caller was not requested by the target Texture")
	}
	parent, err := authorityStore.GetLifecycleRun(ctx, caller.OwnerID, caller.SandboxID, requesterRunID)
	if err != nil {
		return types.RunRecord{}, fmt.Errorf("resolve requesting lifecycle Texture run: %w", err)
	}
	if parent.AgentID != target.AgentID || agentprofile.Canonical(configuredAgentProfileForRun(&parent)) != agentprofile.Texture ||
		agentprofile.Canonical(agentRoleForRun(&parent)) != agentprofile.Texture || strings.TrimSpace(trajectoryIDForRun(&parent)) != strings.TrimSpace(trajectoryIDForRun(&caller)) {
		return types.RunRecord{}, fmt.Errorf("update_coagent requesting lifecycle Texture run binding mismatch")
	}
	return parent, nil
}

func exactRequesterRunID(run types.RunRecord) (string, error) {
	values := []string{
		strings.TrimSpace(run.RequestedByRunID),
		strings.TrimSpace(metadataStringValue(run.Metadata, "requested_by_run_id")),
		strings.TrimSpace(metadataStringValue(run.Metadata, "requested_by")),
	}
	requesterRunID := ""
	for _, value := range values {
		if value == "" {
			continue
		}
		if requesterRunID != "" && value != requesterRunID {
			return "", fmt.Errorf("update_coagent requester run metadata conflicts")
		}
		requesterRunID = value
	}
	if requesterRunID == "" {
		return "", fmt.Errorf("update_coagent requester run binding is required")
	}
	return requesterRunID, nil
}

func lifecycleAssignedWorkItemID(run types.RunRecord, requestedWorkItemID string) (string, error) {
	assigned := metadataStringSlice(run.Metadata["work_item_ids"])
	singular := strings.TrimSpace(metadataStringValue(run.Metadata, "lifecycle_work_item_id"))
	if singular != "" {
		if len(assigned) > 0 && !slices.Contains(assigned, singular) {
			return "", fmt.Errorf("update_coagent lifecycle activation has inconsistent assigned work metadata")
		}
		if !slices.Contains(assigned, singular) {
			assigned = append(assigned, singular)
		}
	}
	requestedWorkItemID = strings.TrimSpace(requestedWorkItemID)
	if requestedWorkItemID == "" {
		if len(assigned) != 1 {
			return "", fmt.Errorf("update_coagent lifecycle activation requires one explicit assigned work binding")
		}
		return strings.TrimSpace(assigned[0]), nil
	}
	if !slices.Contains(assigned, requestedWorkItemID) {
		return "", fmt.Errorf("update_coagent work_item_id is not assigned to this lifecycle activation")
	}
	return requestedWorkItemID, nil
}

func validatePreCutoverCoagentUpdateAuthority(ctx context.Context, authorityStore coagentUpdateAuthorityStore, authority *coagentUpdateAuthority) error {
	if authority == nil {
		return fmt.Errorf("update_coagent pre-cutover authority is missing")
	}
	if authority.target.LifecycleVersion > 0 {
		return fmt.Errorf("update_coagent pre-cutover run cannot address a lifecycle target")
	}
	if authority.trajectoryID != "" {
		if _, err := authorityStore.GetLifecycleTrajectory(ctx, authority.callerRun.OwnerID, authority.callerRun.SandboxID, authority.trajectoryID); err == nil {
			return store.ErrLifecycleAuthorityRequired
		} else if !errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("check lifecycle trajectory before pre-cutover dispatch: %w", err)
		}
		trajectory, err := authorityStore.GetTrajectory(ctx, authority.callerRun.OwnerID, authority.trajectoryID)
		if err != nil {
			return fmt.Errorf("resolve pre-cutover trajectory: %w", err)
		}
		if computerID := strings.TrimSpace(trajectory.ComputerID); computerID != "" && computerID != authority.callerRun.SandboxID {
			return fmt.Errorf("update_coagent pre-cutover trajectory computer mismatch")
		}
		if channelID := strings.TrimSpace(trajectory.SubjectRefs["channel_id"]); channelID != "" && channelID != authority.target.ChannelID {
			return fmt.Errorf("update_coagent pre-cutover trajectory/document mismatch")
		}
	}

	if requester, err := loadLegacyRequesterRun(ctx, authorityStore, authority.callerRun, authority.target); err == nil {
		if authority.callerProfile == agentprofile.CoSuper {
			switch authority.targetProfile {
			case agentprofile.Texture:
				return validateCoSuperTextureResultPath(ctx, authorityStore, *authority)
			case agentprofile.Super:
				return validateCoSuperOwningSuper(ctx, authorityStore, *authority, requester)
			}
		}
		return nil
	}
	if authority.callerProfile == agentprofile.Super && (authority.targetProfile == agentprofile.Researcher || authority.targetProfile == agentprofile.CoSuper) {
		return validateLegacyOwnedChild(ctx, authorityStore, *authority)
	}
	if authority.callerProfile == agentprofile.CoSuper && authority.targetProfile == agentprofile.Texture {
		return validateCoSuperTextureResultPath(ctx, authorityStore, *authority)
	}
	return fmt.Errorf("update_coagent target is neither the exact requester nor an assigned child")
}

func loadLegacyRequesterRun(ctx context.Context, authorityStore coagentUpdateAuthorityStore, caller types.RunRecord, target types.AgentRecord) (types.RunRecord, error) {
	requesterRunID, err := exactRequesterRunID(caller)
	if err != nil {
		return types.RunRecord{}, err
	}
	if metadataStringValue(caller.Metadata, "requested_by_agent_id") != target.AgentID ||
		agentprofile.Canonical(metadataStringValue(caller.Metadata, "requested_by_profile")) != agentprofile.Canonical(target.Profile) {
		return types.RunRecord{}, fmt.Errorf("update_coagent target does not match caller requester metadata")
	}
	parent, err := loadScopedLegacyRun(ctx, authorityStore, caller.OwnerID, caller.SandboxID, requesterRunID)
	if err != nil {
		return types.RunRecord{}, fmt.Errorf("resolve pre-cutover requester run: %w", err)
	}
	if parent.AgentID != target.AgentID || agentprofile.Canonical(configuredAgentProfileForRun(&parent)) != agentprofile.Canonical(target.Profile) ||
		(parent.TrajectoryID != "" && caller.TrajectoryID != "" && parent.TrajectoryID != caller.TrajectoryID) {
		return types.RunRecord{}, fmt.Errorf("update_coagent pre-cutover requester identity mismatch")
	}
	return parent, nil
}

func loadScopedLegacyRun(ctx context.Context, authorityStore coagentUpdateAuthorityStore, ownerID, computerID, runID string) (types.RunRecord, error) {
	run, err := authorityStore.GetRunByOwner(ctx, ownerID, strings.TrimSpace(runID))
	if err != nil {
		return types.RunRecord{}, err
	}
	if run.OwnerID != ownerID || run.SandboxID != computerID || run.RunID != strings.TrimSpace(runID) {
		return types.RunRecord{}, fmt.Errorf("pre-cutover run scope mismatch")
	}
	return run, nil
}

func validateLegacyOwnedChild(ctx context.Context, authorityStore coagentUpdateAuthorityStore, authority coagentUpdateAuthority) error {
	targetRunID := strings.TrimSpace(authority.target.ActiveRunID)
	if targetRunID == "" {
		return fmt.Errorf("update_coagent assigned child has no durable active run")
	}
	child, err := loadScopedLegacyRun(ctx, authorityStore, authority.callerRun.OwnerID, authority.callerRun.SandboxID, targetRunID)
	if err != nil {
		return fmt.Errorf("resolve assigned child run: %w", err)
	}
	requesterRunID, err := exactRequesterRunID(child)
	if err != nil || requesterRunID != authority.callerRun.RunID || child.AgentID != authority.target.AgentID ||
		strings.TrimSpace(trajectoryIDForRun(&child)) != authority.trajectoryID {
		return fmt.Errorf("update_coagent child run is not owned by the calling super")
	}
	if authority.targetProfile == agentprofile.CoSuper {
		slot, found, err := authorityStore.CoSuperSlotByAgentAndTrajectory(ctx, authority.callerRun.OwnerID, authority.trajectoryID, authority.target.AgentID)
		if err != nil {
			return fmt.Errorf("lookup assigned co-super slot: %w", err)
		}
		if !found || slot.RunID != child.RunID || slot.RequestedByRunID != authority.callerRun.RunID {
			return fmt.Errorf("update_coagent co-super assignment does not match caller")
		}
	}
	workIDs := metadataStringSlice(child.Metadata["work_item_ids"])
	if singular := metadataStringValue(child.Metadata, "lifecycle_work_item_id"); singular != "" && !slices.Contains(workIDs, singular) {
		workIDs = append(workIDs, singular)
	}
	for _, workID := range workIDs {
		work, getErr := authorityStore.GetWorkItem(ctx, authority.callerRun.OwnerID, workID)
		if getErr != nil {
			return fmt.Errorf("resolve assigned child work: %w", getErr)
		}
		if work.AssignedAgentID == child.AgentID && work.CreatedByRunID == authority.callerRun.RunID && work.TrajectoryID == authority.trajectoryID && work.Status == types.WorkItemOpen {
			return nil
		}
	}
	return fmt.Errorf("update_coagent assigned child lacks exact open work provenance")
}

func validateCoSuperOwningSuper(ctx context.Context, authorityStore coagentUpdateAuthorityStore, authority coagentUpdateAuthority, owningSuper types.RunRecord) error {
	if authority.trajectoryID == "" || owningSuper.AgentID != authority.target.AgentID {
		return fmt.Errorf("update_coagent co-super owning Super binding mismatch")
	}
	slot, found, err := authorityStore.CoSuperSlotByAgentAndTrajectory(ctx, authority.callerRun.OwnerID, authority.trajectoryID, authority.callerRun.AgentID)
	if err != nil {
		return fmt.Errorf("lookup calling co-super assignment: %w", err)
	}
	if !found || slot.RunID != authority.callerRun.RunID || slot.RequestedByRunID != owningSuper.RunID {
		return fmt.Errorf("update_coagent calling co-super is not assigned to target Super")
	}
	return nil
}

func validateCoSuperTextureResultPath(ctx context.Context, authorityStore coagentUpdateAuthorityStore, authority coagentUpdateAuthority) error {
	if authority.callerProfile != agentprofile.CoSuper || authority.targetProfile != agentprofile.Texture || authority.trajectoryID == "" {
		return fmt.Errorf("update_coagent co-super result path is not applicable")
	}
	slot, found, err := authorityStore.CoSuperSlotByAgentAndTrajectory(ctx, authority.callerRun.OwnerID, authority.trajectoryID, authority.callerRun.AgentID)
	if err != nil {
		return fmt.Errorf("lookup calling co-super assignment: %w", err)
	}
	if !found || slot.RunID != authority.callerRun.RunID || strings.TrimSpace(slot.RequestedByRunID) == "" {
		return fmt.Errorf("update_coagent calling co-super lacks exact assignment")
	}
	owningSuper, err := loadScopedLegacyRun(ctx, authorityStore, authority.callerRun.OwnerID, authority.callerRun.SandboxID, slot.RequestedByRunID)
	if err != nil {
		return fmt.Errorf("resolve owning super run: %w", err)
	}
	if agentprofile.Canonical(configuredAgentProfileForRun(&owningSuper)) != agentprofile.Super {
		return fmt.Errorf("update_coagent co-super assignment owner is not Super")
	}
	if _, err := loadLegacyRequesterRun(ctx, authorityStore, owningSuper, authority.target); err != nil {
		return fmt.Errorf("resolve Texture requester through owning Super: %w", err)
	}
	return nil
}

func stringArraySchema() map[string]any {
	return map[string]any{
		"type":  "array",
		"items": map[string]any{"type": "string"},
	}
}

func workerUpdateEmpty(update types.CoagentSourcePacket) bool {
	return coagentPacketPayloadEmpty(update.Packet)
}

func buildWorkerUpdateMessage(update types.CoagentSourcePacket) string {
	packet := update.Packet
	var b strings.Builder
	b.WriteString("Coagent source packet ready.")
	if strings.TrimSpace(update.Role) != "" {
		b.WriteString("\nRole: ")
		b.WriteString(strings.TrimSpace(update.Role))
		b.WriteString(".")
	}
	b.WriteString("\nSchema: ")
	b.WriteString(packet.SchemaVersion)
	b.WriteString("\nKind: ")
	b.WriteString(packet.Kind)
	if strings.TrimSpace(packet.Summary) != "" {
		b.WriteString("\nSummary: ")
		b.WriteString(strings.TrimSpace(packet.Summary))
	}
	appendCoagentClaimSection(&b, packet.Claims)
	appendCoagentSourceSection(&b, packet.Sources)
	appendCoagentActionSection(&b, packet.Actions)
	appendWorkerUpdateSection(&b, "Questions", packet.Questions)
	appendWorkerUpdateSection(&b, "Notes", packet.Notes)
	return b.String()
}

func appendWorkerUpdateSection(b *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	b.WriteString("\n\n")
	b.WriteString(title)
	b.WriteString(":\n")
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(item)
		b.WriteString("\n")
	}
}

func appendCoagentClaimSection(b *strings.Builder, claims []types.CoagentPacketClaim) {
	if len(claims) == 0 {
		return
	}
	b.WriteString("\n\nClaims:\n")
	for _, claim := range claims {
		text := strings.TrimSpace(claim.Text)
		if text == "" {
			continue
		}
		b.WriteString("- ")
		if claim.ClaimID != "" {
			b.WriteString(claim.ClaimID)
			b.WriteString(": ")
		}
		b.WriteString(text)
		if len(claim.SourceIDs) > 0 {
			b.WriteString(" [sources: ")
			b.WriteString(strings.Join(claim.SourceIDs, ", "))
			b.WriteString("]")
		}
		if claim.Stance != "" {
			b.WriteString(" stance=")
			b.WriteString(claim.Stance)
		}
		b.WriteString("\n")
	}
}

func appendCoagentSourceSection(b *strings.Builder, sources []types.CoagentPacketSource) {
	if len(sources) == 0 {
		return
	}
	b.WriteString("\n\nSources:\n")
	for _, source := range sources {
		kind := strings.TrimSpace(source.Kind)
		uri := strings.TrimSpace(source.Target.URI)
		title := strings.TrimSpace(source.Target.Title)
		if kind == "" && uri == "" && title == "" {
			continue
		}
		b.WriteString("- ")
		if source.SourceID != "" {
			b.WriteString(source.SourceID)
			b.WriteString(": ")
		}
		b.WriteString(kind)
		if title != "" {
			b.WriteString(" ")
			b.WriteString(strconvQuote(title))
		}
		if uri != "" {
			b.WriteString(" <")
			b.WriteString(uri)
			b.WriteString(">")
		}
		if excerpt := strings.TrimSpace(source.Excerpt); excerpt != "" {
			b.WriteString(" excerpt=")
			b.WriteString(strconvQuote(truncateRunes(excerpt, 280)))
		} else if source.ReaderSnapshot != nil {
			if text := strings.TrimSpace(source.ReaderSnapshot.TextContent); text != "" {
				b.WriteString(" reader_snapshot=")
				b.WriteString(strconvQuote(truncateRunes(text, 280)))
			}
		}
		b.WriteString("\n")
	}
}

func strconvQuote(value string) string {
	encoded, err := json.Marshal(strings.TrimSpace(value))
	if err != nil {
		return strings.TrimSpace(value)
	}
	return string(encoded)
}

func appendCoagentActionSection(b *strings.Builder, actions []types.CoagentPacketAction) {
	if len(actions) == 0 {
		return
	}
	b.WriteString("\n\nActions:\n")
	for _, action := range actions {
		if strings.TrimSpace(action.Type) == "" && strings.TrimSpace(action.Objective) == "" {
			continue
		}
		b.WriteString("- ")
		if action.ActionID != "" {
			b.WriteString(action.ActionID)
			b.WriteString(": ")
		}
		b.WriteString(action.Type)
		if action.Objective != "" {
			b.WriteString(" - ")
			b.WriteString(action.Objective)
		}
		b.WriteString("\n")
	}
}

func deriveLifecycleProducerUpdateID(execution toolregistry.ExecutionContext, run types.RunRecord) (string, error) {
	callID := strings.TrimSpace(execution.ToolCallID)
	if callID == "" {
		return "", fmt.Errorf("update_coagent lifecycle delivery requires runtime tool_call_id authority")
	}
	if strings.TrimSpace(run.OwnerID) == "" || strings.TrimSpace(run.SandboxID) == "" || strings.TrimSpace(run.RunID) == "" ||
		execution.OwnerID != run.OwnerID || execution.SandboxID != run.SandboxID || execution.RunID != run.RunID {
		return "", fmt.Errorf("update_coagent cannot derive producer identity from mismatched run authority")
	}
	seed, _ := json.Marshal(struct {
		OwnerID    string `json:"owner_id"`
		ComputerID string `json:"computer_id"`
		RunID      string `json:"run_id"`
		ToolCallID string `json:"tool_call_id"`
	}{run.OwnerID, run.SandboxID, run.RunID, callID})
	sum := sha256.Sum256(seed)
	raw := append([]byte(nil), sum[:16]...)
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	id, err := uuid.FromBytes(raw)
	if err != nil {
		return "", fmt.Errorf("derive lifecycle producer identity: %w", err)
	}
	return id.String(), nil
}

func deriveLifecycleWorkerUpdateID(update types.CoagentSourcePacket, producerUpdateID string) string {
	payload := struct {
		OwnerID          string `json:"owner_id"`
		AgentID          string `json:"agent_id"`
		TargetAgentID    string `json:"target_agent_id"`
		TrajectoryID     string `json:"trajectory_id"`
		ProducerUpdateID string `json:"producer_update_id"`
	}{
		OwnerID: strings.TrimSpace(update.OwnerID), AgentID: strings.TrimSpace(update.AgentID),
		TargetAgentID: strings.TrimSpace(update.TargetAgentID), TrajectoryID: strings.TrimSpace(update.TrajectoryID),
		ProducerUpdateID: strings.TrimSpace(producerUpdateID),
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return "upd-" + hex.EncodeToString(sum[:])[:32]
}

func deriveWorkerUpdateID(update types.CoagentSourcePacket) string {
	payload := struct {
		OwnerID       string                           `json:"owner_id"`
		AgentID       string                           `json:"agent_id"`
		TargetAgentID string                           `json:"target_agent_id"`
		ChannelID     string                           `json:"channel_id"`
		TrajectoryID  string                           `json:"trajectory_id,omitempty"`
		Role          string                           `json:"role,omitempty"`
		Packet        types.CoagentSourcePacketPayload `json:"packet"`
	}{
		OwnerID:       strings.TrimSpace(update.OwnerID),
		AgentID:       strings.TrimSpace(update.AgentID),
		TargetAgentID: strings.TrimSpace(update.TargetAgentID),
		ChannelID:     strings.TrimSpace(update.ChannelID),
		TrajectoryID:  strings.TrimSpace(update.TrajectoryID),
		Role:          strings.TrimSpace(update.Role),
		Packet:        normalizeCoagentSourcePacketPayload(update.Packet),
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return "upd-" + hex.EncodeToString(sum[:])[:32]
}

func validateExistingWorkerUpdate(existing, want types.CoagentSourcePacket) error {
	if existing.AgentID != want.AgentID ||
		existing.TargetAgentID != want.TargetAgentID ||
		existing.ChannelID != want.ChannelID ||
		existing.Role != want.Role ||
		existing.Content != want.Content ||
		!reflect.DeepEqual(normalizeCoagentSourcePacketPayload(existing.Packet), normalizeCoagentSourcePacketPayload(want.Packet)) {
		return fmt.Errorf("update_id %s already exists with different payload", want.UpdateID)
	}
	return nil
}

func normalizeCoagentSourcePacketPayload(packet types.CoagentSourcePacketPayload) types.CoagentSourcePacketPayload {
	packet.SchemaVersion = strings.TrimSpace(packet.SchemaVersion)
	packet.Kind = strings.TrimSpace(packet.Kind)
	packet.Summary = strings.TrimSpace(packet.Summary)
	packet.Questions = trimNonEmpty(packet.Questions)
	packet.Notes = trimNonEmpty(packet.Notes)

	claims := make([]types.CoagentPacketClaim, 0, len(packet.Claims))
	for _, claim := range packet.Claims {
		normalized := types.CoagentPacketClaim{
			ClaimID:            strings.TrimSpace(claim.ClaimID),
			Text:               strings.TrimSpace(claim.Text),
			SourceIDs:          trimNonEmpty(claim.SourceIDs),
			Stance:             strings.TrimSpace(claim.Stance),
			RecommendedSurface: strings.TrimSpace(claim.RecommendedSurface),
		}
		if normalized.Text != "" {
			claims = append(claims, normalized)
		}
	}
	packet.Claims = claims

	sources := make([]types.CoagentPacketSource, 0, len(packet.Sources))
	for _, source := range packet.Sources {
		normalized := types.CoagentPacketSource{
			SourceID: strings.TrimSpace(source.SourceID),
			Kind:     sourcecontract.NormalizeSourceKind(source.Kind),
			Target: types.CoagentPacketSourceTarget{
				URI:       strings.TrimSpace(source.Target.URI),
				Title:     strings.TrimSpace(source.Target.Title),
				MediaType: strings.TrimSpace(source.Target.MediaType),
			},
			Evidence: types.CoagentPacketSourceEvidence{
				State:       strings.TrimSpace(source.Evidence.State),
				Confidence:  strings.TrimSpace(source.Evidence.Confidence),
				RightsScope: strings.TrimSpace(source.Evidence.RightsScope),
			},
			Excerpt: strings.TrimSpace(source.Excerpt),
		}
		if source.ReaderSnapshot != nil {
			snapshot := *source.ReaderSnapshot
			snapshot.TextContent = strings.TrimSpace(snapshot.TextContent)
			snapshot.SnapshotKind = strings.TrimSpace(snapshot.SnapshotKind)
			snapshot.MediaType = strings.TrimSpace(snapshot.MediaType)
			snapshot.OriginalMediaType = strings.TrimSpace(snapshot.OriginalMediaType)
			snapshot.SourceURL = strings.TrimSpace(snapshot.SourceURL)
			snapshot.AccessScope = strings.TrimSpace(snapshot.AccessScope)
			if snapshot.TextContent != "" || snapshot.SnapshotKind != "" || snapshot.MediaType != "" || snapshot.OriginalMediaType != "" || snapshot.SourceURL != "" || snapshot.AccessScope != "" || snapshot.Truncated {
				normalized.ReaderSnapshot = &snapshot
			}
		}
		for _, selector := range source.Selectors {
			selectorKind := strings.TrimSpace(selector.Kind)
			sel := types.CoagentPacketSourceSelector{
				Kind:   sourcecontract.NormalizeSelectorKind(selectorKind),
				Quote:  strings.TrimSpace(selector.Quote),
				Start:  selector.Start,
				End:    selector.End,
				X:      selector.X,
				Y:      selector.Y,
				Width:  selector.Width,
				Height: selector.Height,
			}
			if selectorKind != "" {
				normalized.Selectors = append(normalized.Selectors, sel)
			}
		}
		if normalized.Kind != "" || normalized.Target.URI != "" || normalized.Target.Title != "" {
			sources = append(sources, normalized)
		}
	}
	packet.Sources = sources

	actions := make([]types.CoagentPacketAction, 0, len(packet.Actions))
	for _, action := range packet.Actions {
		normalized := types.CoagentPacketAction{
			ActionID:  strings.TrimSpace(action.ActionID),
			Type:      strings.TrimSpace(action.Type),
			Objective: strings.TrimSpace(action.Objective),
			Inputs:    action.Inputs,
			Safety: types.CoagentPacketActionSafety{
				MutationClass: strings.TrimSpace(action.Safety.MutationClass),
				Network:       strings.TrimSpace(action.Safety.Network),
				FileMutation:  strings.TrimSpace(action.Safety.FileMutation),
			},
		}
		for _, expected := range action.ExpectedSources {
			kind := sourcecontract.NormalizeSourceKind(expected.Kind)
			if kind == "" {
				continue
			}
			normalized.ExpectedSources = append(normalized.ExpectedSources, types.CoagentPacketExpectedSource{Kind: kind, Required: expected.Required})
		}
		if normalized.Type != "" || normalized.Objective != "" {
			actions = append(actions, normalized)
		}
	}
	packet.Actions = actions
	return packet
}

func validateCoagentSourcePacketPayload(packet types.CoagentSourcePacketPayload) error {
	if packet.SchemaVersion != types.CoagentSourcePacketSchemaV1 {
		return fmt.Errorf("update_coagent schema_version must be %q", types.CoagentSourcePacketSchemaV1)
	}
	if !validCoagentPacketKind(packet.Kind) {
		return fmt.Errorf("update_coagent kind %q is not supported", packet.Kind)
	}
	if packet.Summary == "" {
		return fmt.Errorf("update_coagent summary is required")
	}
	if coagentPacketPayloadEmpty(packet) {
		return fmt.Errorf("update_coagent requires at least one of claims, sources, actions, questions, or notes")
	}
	if packet.Kind == "execution_request" && len(packet.Actions) == 0 {
		return fmt.Errorf("update_coagent kind=execution_request requires actions")
	}
	sourceIDs := make(map[string]bool, len(packet.Sources))
	for i, source := range packet.Sources {
		if err := validateCoagentPacketSource(source); err != nil {
			return fmt.Errorf("update_coagent sources[%d]: %w", i, err)
		}
		sourceID := strings.TrimSpace(source.SourceID)
		if sourceID == "" {
			return fmt.Errorf("update_coagent sources[%d].source_id is required", i)
		}
		if sourceIDs[sourceID] {
			return fmt.Errorf("update_coagent sources[%d].source_id %q is duplicated", i, sourceID)
		}
		sourceIDs[sourceID] = true
	}
	for i, claim := range packet.Claims {
		if err := validateCoagentPacketClaim(claim, sourceIDs); err != nil {
			return fmt.Errorf("update_coagent claims[%d]: %w", i, err)
		}
	}
	for i, action := range packet.Actions {
		if err := validateCoagentPacketAction(action, packet.Kind == "execution_request"); err != nil {
			return fmt.Errorf("update_coagent actions[%d]: %w", i, err)
		}
	}
	return nil
}

func validCoagentPacketKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "evidence_update", "execution_request", "execution_result", "blocker", "question", "proposal", "decision_request":
		return true
	default:
		return false
	}
}

func validateCoagentPacketClaim(claim types.CoagentPacketClaim, sourceIDs map[string]bool) error {
	if strings.TrimSpace(claim.Text) == "" {
		return fmt.Errorf("text is required")
	}
	if stance := strings.TrimSpace(claim.Stance); stance != "" && !validCoagentClaimStance(stance) {
		return fmt.Errorf("stance %q is not supported", stance)
	}
	if surface := strings.TrimSpace(claim.RecommendedSurface); surface != "" && !validCoagentClaimRecommendedSurface(surface) {
		return fmt.Errorf("recommended_surface %q is not supported", surface)
	}
	seen := map[string]bool{}
	for _, sourceID := range claim.SourceIDs {
		sourceID = strings.TrimSpace(sourceID)
		if sourceID == "" {
			return fmt.Errorf("source_ids must not contain empty values")
		}
		if seen[sourceID] {
			return fmt.Errorf("source_id %q is duplicated", sourceID)
		}
		seen[sourceID] = true
		if !sourceIDs[sourceID] {
			return fmt.Errorf("source_id %q does not match packet.sources", sourceID)
		}
	}
	return nil
}

func validateCoagentPacketSource(source types.CoagentPacketSource) error {
	if strings.TrimSpace(source.Kind) == "" {
		return fmt.Errorf("kind is required")
	}
	if !sourcecontract.IsSourceKind(source.Kind) {
		return fmt.Errorf("kind %q is not supported", source.Kind)
	}
	if strings.TrimSpace(source.Target.URI) == "" {
		return fmt.Errorf("target.uri is required")
	}
	if len([]rune(strings.TrimSpace(source.Excerpt))) > 2000 {
		return fmt.Errorf("excerpt must be at most 2000 characters")
	}
	if source.ReaderSnapshot != nil && len([]rune(strings.TrimSpace(source.ReaderSnapshot.TextContent))) > 100000 {
		return fmt.Errorf("reader_snapshot.text_content must be at most 100000 characters")
	}
	for i, selector := range source.Selectors {
		if strings.TrimSpace(selector.Kind) == "" {
			return fmt.Errorf("selectors[%d].kind is required", i)
		}
		if !sourcecontract.IsSelectorKind(selector.Kind) {
			return fmt.Errorf("selectors[%d].kind %q is not supported", i, selector.Kind)
		}
	}
	if state := strings.TrimSpace(source.Evidence.State); state != "" && !validCoagentSourceEvidenceState(state) {
		return fmt.Errorf("evidence.state %q is not supported", state)
	}
	if confidence := strings.TrimSpace(source.Evidence.Confidence); confidence != "" && !validCoagentSourceEvidenceConfidence(confidence) {
		return fmt.Errorf("evidence.confidence %q is not supported", confidence)
	}
	return nil
}

func validateCoagentPacketAction(action types.CoagentPacketAction, requireSafety bool) error {
	if strings.TrimSpace(action.Type) == "" {
		return fmt.Errorf("type is required")
	}
	if !validCoagentActionType(action.Type) {
		return fmt.Errorf("type %q is not supported", action.Type)
	}
	if strings.TrimSpace(action.Objective) == "" {
		return fmt.Errorf("objective is required")
	}
	for i, expected := range action.ExpectedSources {
		if strings.TrimSpace(expected.Kind) == "" {
			return fmt.Errorf("expected_sources[%d].kind is required", i)
		}
		if !sourcecontract.IsSourceKind(expected.Kind) {
			return fmt.Errorf("expected_sources[%d].kind %q is not supported", i, expected.Kind)
		}
	}
	safety := action.Safety
	if requireSafety {
		if strings.TrimSpace(safety.MutationClass) == "" || strings.TrimSpace(safety.Network) == "" || strings.TrimSpace(safety.FileMutation) == "" {
			return fmt.Errorf("safety.mutation_class, safety.network, and safety.file_mutation are required for execution_request actions")
		}
	}
	if mutationClass := strings.TrimSpace(safety.MutationClass); mutationClass != "" && !validMutationClass(mutationClass) {
		return fmt.Errorf("safety.mutation_class %q is not supported", mutationClass)
	}
	if network := strings.TrimSpace(safety.Network); network != "" && !validCoagentActionSafetyMode(network) {
		return fmt.Errorf("safety.network %q is not supported", network)
	}
	if fileMutation := strings.TrimSpace(safety.FileMutation); fileMutation != "" && !validCoagentActionSafetyMode(fileMutation) {
		return fmt.Errorf("safety.file_mutation %q is not supported", fileMutation)
	}
	return nil
}

func validCoagentClaimStance(stance string) bool {
	switch strings.TrimSpace(stance) {
	case "supports", "qualifies", "contradicts", "background":
		return true
	default:
		return false
	}
}

func validCoagentClaimRecommendedSurface(surface string) bool {
	switch strings.TrimSpace(surface) {
	case "inline_ref", "block_embed", "source_panel", "decision_log":
		return true
	default:
		return false
	}
}

func validCoagentSourceEvidenceState(state string) bool {
	switch strings.TrimSpace(state) {
	case "available", "pending", "blocked", "unavailable":
		return true
	default:
		return false
	}
}

func validCoagentSourceEvidenceConfidence(confidence string) bool {
	switch strings.TrimSpace(confidence) {
	case "low", "medium", "high":
		return true
	default:
		return false
	}
}

func validCoagentActionType(actionType string) bool {
	switch strings.TrimSpace(actionType) {
	case "run_command", "inspect_file", "produce_diff", "run_tests", "open_browser", "import_source", "revise_texture":
		return true
	default:
		return false
	}
}

func validMutationClass(mutationClass string) bool {
	switch strings.TrimSpace(mutationClass) {
	case "green", "yellow", "orange", "red", "black":
		return true
	default:
		return false
	}
}

func validCoagentActionSafetyMode(mode string) bool {
	switch strings.TrimSpace(mode) {
	case "forbidden", "allowed", "required":
		return true
	default:
		return false
	}
}

func coagentPacketPayloadEmpty(packet types.CoagentSourcePacketPayload) bool {
	return len(packet.Claims) == 0 &&
		len(packet.Sources) == 0 &&
		len(packet.Actions) == 0 &&
		len(packet.Questions) == 0 &&
		len(packet.Notes) == 0
}

func rejectLegacyUpdateCoagentFields(raw json.RawMessage) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return fmt.Errorf("decode update_coagent args: %w", err)
	}
	allowed := map[string]bool{
		"agent_id":           true,
		"channel_id":         true,
		"producer_update_id": true,
		"work_item_id":       true,
		"work_disposition":   true,
		"schema_version":     true,
		"kind":               true,
		"summary":            true,
		"claims":             true,
		"sources":            true,
		"actions":            true,
		"questions":          true,
		"notes":              true,
	}
	legacy := map[string]bool{
		"update_id":           true,
		"findings":            true,
		"evidence_ids":        true,
		"evidence":            true,
		"artifacts":           true,
		"refs":                true,
		"tests":               true,
		"proposals":           true,
		"capability_requests": true,
	}
	if rawDisposition, present := fields["work_disposition"]; present {
		var value any
		if err := json.Unmarshal(rawDisposition, &value); err != nil {
			return fmt.Errorf("decode update_coagent work_disposition: %w", err)
		}
		disposition, ok := value.(string)
		if !ok || (strings.TrimSpace(disposition) != "open" && strings.TrimSpace(disposition) != "completed") {
			return fmt.Errorf("update_coagent work_disposition must be open or completed when present")
		}
	}
	for key := range fields {
		key = strings.TrimSpace(key)
		if legacy[key] {
			return fmt.Errorf("update_coagent legacy field %q is not accepted; use claims, sources, actions, questions, or notes", key)
		}
		if !allowed[key] {
			return fmt.Errorf("update_coagent unknown field %q", key)
		}
	}
	return nil
}

func newCoagentPacket(kind, summary string, claims []types.CoagentPacketClaim, sources []types.CoagentPacketSource, actions []types.CoagentPacketAction, questions, notes []string) types.CoagentSourcePacketPayload {
	return normalizeCoagentSourcePacketPayload(types.CoagentSourcePacketPayload{
		SchemaVersion: types.CoagentSourcePacketSchemaV1,
		Kind:          strings.TrimSpace(kind),
		Summary:       strings.TrimSpace(summary),
		Claims:        claims,
		Sources:       sources,
		Actions:       actions,
		Questions:     questions,
		Notes:         notes,
	})
}

func coagentSourceFromURI(sourceID, kind, uri, title string) types.CoagentPacketSource {
	return types.CoagentPacketSource{
		SourceID: strings.TrimSpace(sourceID),
		Kind:     strings.TrimSpace(kind),
		Target: types.CoagentPacketSourceTarget{
			URI:   strings.TrimSpace(uri),
			Title: strings.TrimSpace(title),
		},
		Selectors: []types.CoagentPacketSourceSelector{{Kind: "whole_resource"}},
		Evidence: types.CoagentPacketSourceEvidence{
			State:       "available",
			Confidence:  "medium",
			RightsScope: "private_user_source",
		},
	}
}

func coagentSourcesFromTypedEvidenceRefs(refs []string) []types.CoagentPacketSource {
	out := make([]types.CoagentPacketSource, 0, len(refs))
	seen := map[string]bool{}
	for _, ref := range refs {
		source, ok := coagentSourceFromTypedEvidenceRef(ref)
		if !ok {
			continue
		}
		sourceID := strings.TrimSpace(source.SourceID)
		if sourceID == "" || seen[sourceID] {
			continue
		}
		seen[sourceID] = true
		out = append(out, source)
	}
	return out
}

func coagentSourceFromTypedEvidenceRef(ref string) (types.CoagentPacketSource, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return types.CoagentPacketSource{}, false
	}
	key, value := splitTypedWorkerUpdateRef(ref)
	uri := ref
	if key == "" && isHTTPURL(ref) {
		sourceID := "src-" + sanitizeExportPart(ref)
		if sourceID == "src-" {
			return types.CoagentPacketSource{}, false
		}
		return coagentSourceFromURI(sourceID, "web_url", ref, ""), true
	}
	if key == "" && looksLikeArtifactPath(ref) {
		key = "file_artifact"
		value = ref
		uri = "file_artifact:" + ref
	}
	if key == "" || value == "" {
		return types.CoagentPacketSource{}, false
	}
	kind := key
	switch key {
	case "content_id", "evidence":
		kind = "content_item"
	case "source_service_item":
		kind = "source_service_item"
	default:
		if !executionTargetKind(kind) {
			return types.CoagentPacketSource{}, false
		}
	}
	sourceID := "src-" + sanitizeExportPart(uri)
	if sourceID == "src-" {
		return types.CoagentPacketSource{}, false
	}
	return coagentSourceFromURI(sourceID, kind, uri, ""), true
}

func coagentClaimsFromTexts(texts []string, sources []types.CoagentPacketSource) []types.CoagentPacketClaim {
	sourceIDs := make([]string, 0, len(sources))
	for _, source := range sources {
		if id := strings.TrimSpace(source.SourceID); id != "" {
			sourceIDs = append(sourceIDs, id)
		}
	}
	claims := make([]types.CoagentPacketClaim, 0, len(texts))
	for _, text := range trimNonEmpty(texts) {
		claims = append(claims, coagentClaim(text, sourceIDs...))
	}
	return claims
}

func coagentClaim(text string, sourceIDs ...string) types.CoagentPacketClaim {
	return types.CoagentPacketClaim{
		Text:               strings.TrimSpace(text),
		SourceIDs:          trimNonEmpty(sourceIDs),
		Stance:             "supports",
		RecommendedSurface: "decision_log",
	}
}

func coagentAction(actionType, objective string, inputs map[string]any, expected []types.CoagentPacketExpectedSource, safety types.CoagentPacketActionSafety) types.CoagentPacketAction {
	return types.CoagentPacketAction{
		Type:            strings.TrimSpace(actionType),
		Objective:       strings.TrimSpace(objective),
		Inputs:          inputs,
		ExpectedSources: expected,
		Safety:          safety,
	}
}

func coagentActionsFromTexts(texts []string) []types.CoagentPacketAction {
	actions := make([]types.CoagentPacketAction, 0, len(texts))
	for _, text := range trimNonEmpty(texts) {
		actions = append(actions, coagentAction("revise_texture", text, nil, nil, types.CoagentPacketActionSafety{}))
	}
	return actions
}

func coagentSourcesFromResearchEvidence(items []researchFindingEvidenceInput) []types.CoagentPacketSource {
	sources := make([]types.CoagentPacketSource, 0, len(items))
	for i, item := range items {
		kind := strings.TrimSpace(item.Kind)
		uri := strings.TrimSpace(item.SourceURI)
		if kind == "" && uri == "" {
			continue
		}
		sourceID := fmt.Sprintf("src-evidence-%d", i+1)
		sources = append(sources, coagentSourceFromURI(sourceID, firstNonEmpty(kind, "content_item"), uri, item.Title))
	}
	return sources
}

func coagentPacketSummary(packet types.CoagentSourcePacketPayload) string {
	return strings.TrimSpace(packet.Summary)
}

func coagentPacketKind(packet types.CoagentSourcePacketPayload) string {
	return strings.TrimSpace(packet.Kind)
}

func coagentPacketQuestions(packet types.CoagentSourcePacketPayload) []string {
	return trimNonEmpty(packet.Questions)
}

func coagentPacketNotes(packet types.CoagentSourcePacketPayload) []string {
	return trimNonEmpty(packet.Notes)
}

func coagentPacketSourceURIs(packet types.CoagentSourcePacketPayload, kinds ...string) []string {
	want := map[string]bool{}
	for _, kind := range kinds {
		if kind = strings.TrimSpace(kind); kind != "" {
			want[kind] = true
		}
	}
	out := []string{}
	for _, source := range packet.Sources {
		if len(want) > 0 && !want[strings.TrimSpace(source.Kind)] {
			continue
		}
		if uri := strings.TrimSpace(source.Target.URI); uri != "" {
			out = append(out, uri)
		}
	}
	return out
}
