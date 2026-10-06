package textureowner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// managementOpenRequest is the owner-side deterministic persistent-Management
// opener. It replaces texture-desk agency (open_persistent_super inside a
// model-authored ApplyTexture turn) with a direct owner call: a real
// conductor→texture activation is minted, then the control issues through the
// same canonical IssueLifecycleControl reducer the desk uses. The management
// run still mints via the persistent-management reconcile — this endpoint only
// supplies the durable control + wake deterministically.
type managementOpenRequest struct {
	Objective string `json:"objective"`
	Actions   []struct {
		Type      string `json:"type"`
		Objective string `json:"objective"`
		Safety    struct {
			MutationClass string `json:"mutation_class"`
			Network       string `json:"network"`
			FileMutation  string `json:"file_mutation"`
		} `json:"safety"`
	} `json:"actions"`
	CommandID string `json:"command_id,omitempty"`
}

type managementOpenResponse struct {
	Schema       string `json:"schema"`
	DocID        string `json:"doc_id"`
	TrajectoryID string `json:"trajectory_id"`
	TextureRunID string `json:"texture_run_id"`
	WorkItemID   string `json:"work_item_id"`
	ControlID    string `json:"control_id"`
	UpdateID     string `json:"update_id"`
	CommandID    string `json:"command_id"`
}

// HandleManagementOpen is the deterministic owner-side mint surface for
// persistent Management (consensus panel item C, 2026-10-06). It exists
// because the desk's open_persistent_super authoring is a model-behavior
// dependency with no convergence date: SMG acceptance legs need a repeatable
// trigger, and any owner-side probe that waits on agency is non-deterministic.
// The endpoint mints a REAL texture activation (same prompt-bar substrate as
// browser submits — no impersonated caller), issues an execution_request
// control through the canonical IssueLifecycleControl reducer, and wakes the
// management agent. Red surface: owner-reachable control path minting
// persistent Management.
func (h *Handler) HandleManagementOpen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if h.Core == nil || h.Store == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "texture lifecycle unavailable"})
		return
	}
	var req managementOpenRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid management-open request"})
		return
	}
	objective := strings.TrimSpace(req.Objective)
	if objective == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "objective is required"})
		return
	}
	if len(req.Actions) == 0 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "at least one execution_request action is required"})
		return
	}
	commandID := strings.TrimSpace(req.CommandID)
	if commandID == "" {
		commandID = "mgmt-open-" + strings.ReplaceAll(strings.ToLower(objective[:minInt(len(objective), 24)]), " ", "-")
	}

	// Mint a real conductor→texture activation through the same substrate the
	// browser prompt-bar uses: conductor decision run, then texture lifecycle
	// (StartLifecycle + texture run). The control's caller must be the active
	// texture run for the document — no synthetic caller exists that passes
	// the IssueLifecycleControl validator.
	commitCtx, commitCancel := context.WithTimeout(context.WithoutCancel(r.Context()), promptBarCommitTimeout)
	defer commitCancel()
	text := "owner management-open: " + objective
	metadata := map[string]any{
		"agent_profile":        agentprofile.Conductor,
		"agent_role":           agentprofile.Conductor,
		"input_source":         "management_open",
		"seed_prompt":          text,
		"submission_surface":   "management_open",
		"lifecycle_command_id": commandID,
	}
	if ownerEmail := authenticatedUserEmail(r); ownerEmail != "" {
		metadata["owner_email"] = ownerEmail
	}
	conductor, err := h.Core.CompletePromptBarDecision(commitCtx, text, ownerID, metadata, agentcore.PromptBarDecisionSpec{
		Action: "open_app", App: agentprofile.Texture, Title: provider.InitialTextureTitle(objective, ""),
	})
	if err == nil {
		handoff, handoffErr := h.EnsureTextureHandoff(commitCtx, conductor, HandoffRequest{
			Kind: HandoffKindUserPrompt, CallerProfile: agentprofile.Conductor,
			Objective: objective, Title: provider.InitialTextureTitle(objective, ""),
		})
		if handoffErr != nil {
			err = handoffErr
		} else {
			err = h.issueManagementOpenControl(commitCtx, ownerID, handoff, objective, req.Actions, commandID, w)
			if err == nil {
				return // response written inside
			}
		}
	}
	if err != nil {
		if errors.Is(err, agentcore.ErrPromptCommandConflict) || errors.Is(err, store.ErrLifecycleCommandConflict) || errors.Is(err, store.ErrConcurrentStateChange) {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "command identity conflicts with the stored request"})
			return
		}
		if errors.Is(err, agentcore.ErrPreGenesis) {
			writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "computer initializing"})
			return
		}
		log.Printf("management-open: issue control: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to open management"})
		return
	}
}

// issueManagementOpenControl issues the execution_request control through the
// canonical IssueLifecycleControl reducer and returns the durable update. The
// caller run is the texture activation minted by the handoff — the exact
// texture agent/run the validator requires.
func (h *Handler) issueManagementOpenControl(ctx context.Context, ownerID string, handoff HandoffDecision, objective string, actions []struct {
	Type      string `json:"type"`
	Objective string `json:"objective"`
	Safety    struct {
		MutationClass string `json:"mutation_class"`
		Network       string `json:"network"`
		FileMutation  string `json:"file_mutation"`
	} `json:"safety"`
}, commandID string, w http.ResponseWriter) error {
	computerID := strings.TrimSpace(h.Core.TextureComputerID())
	callerRun, err := h.Store.GetLifecycleRun(ctx, ownerID, computerID, handoff.InitialLoopID)
	if err != nil {
		return fmt.Errorf("load texture caller run: %w", err)
	}
	targetAgentID := agentprofile.Management + ":" + ownerID
	// Ensure the persistent-management agent record exists (the desk schema
	// registers it on owner computers; a fresh disposable or first-open path
	// has no prior registration). Idempotent: UpsertAgent over existing is a
	// no-op update.
	if _, err := h.Core.EnsurePersistentManagementAgent(ctx, ownerID); err != nil {
		return fmt.Errorf("ensure persistent management agent: %w", err)
	}
	if _, err := h.Store.GetAgentByScope(ctx, ownerID, computerID, targetAgentID); err != nil {
		return fmt.Errorf("persistent management agent not registered: %w", err)
	}
	workItemID := "mgmt-work-" + commandID
	acts := make([]types.CoagentPacketAction, 0, len(actions))
	for _, a := range actions {
		acts = append(acts, types.CoagentPacketAction{
			Type:      strings.TrimSpace(a.Type),
			Objective: strings.TrimSpace(a.Objective),
			Safety: types.CoagentPacketActionSafety{
				MutationClass: strings.TrimSpace(a.Safety.MutationClass),
				Network:       strings.TrimSpace(a.Safety.Network),
				FileMutation:  strings.TrimSpace(a.Safety.FileMutation),
			},
		})
	}
	packet, err := agentcore.PrepareTextureControlPacket(types.CoagentSourcePacketPayload{
		Kind:    "execution_request",
		Summary: objective,
		Actions: acts,
	})
	if err != nil {
		return fmt.Errorf("control packet: %w", err)
	}
	work := types.WorkItemRecord{
		WorkItemID: workItemID, Objective: objective,
		AuthorityProfile: agentprofile.Management, Status: types.WorkItemOpen,
		AssignedAgentID: targetAgentID,
	}
	content := agentcore.BuildTextureLifecycleControlContent(packet, targetAgentID, workItemID)
	payloadDigest, err := store.ComputeLifecycleUpdatePayloadDigest(packet, content)
	if err != nil {
		return fmt.Errorf("payload digest: %w", err)
	}
	controlID := "mgmt-control-" + commandID
	controls, err := h.Core.IssueLifecycleControl(ctx, &callerRun, []types.TextureTurnControl{{
		ControlID: controlID, TargetAgentID: targetAgentID, TargetWorkItemID: workItemID,
		OpenWork: &work, Packet: packet, Content: content, PayloadDigest: payloadDigest,
	}}, "owner_management_open")
	if err != nil {
		return err
	}
	updateID := ""
	if len(controls) > 0 {
		updateID = controls[0].UpdateID
	}
	writeAPIJSON(w, http.StatusAccepted, managementOpenResponse{
		Schema: types.DurableWorkSchemaV1, DocID: handoff.DocID, TrajectoryID: handoff.Conductor.TrajectoryID,
		TextureRunID: handoff.InitialLoopID, WorkItemID: workItemID, ControlID: controlID,
		UpdateID: updateID, CommandID: commandID,
	})
	return nil
}
