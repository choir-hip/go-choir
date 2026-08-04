package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

type openSupervisionAssignmentsArgs struct {
	CommandID   string `json:"command_id"`
	Assignments []struct {
		AssignmentID          string   `json:"assignment_id"`
		AssignedAgentID       string   `json:"assigned_agent_id,omitempty"`
		ParentDecisionID      string   `json:"parent_decision_id"`
		ScopeDigest           string   `json:"scope_digest"`
		CapabilityDigest      string   `json:"capability_digest"`
		PolicyDigest          string   `json:"policy_digest"`
		ObligationIDs         []string `json:"obligation_ids"`
		IdempotencyCommitment string   `json:"idempotency_commitment"`
	} `json:"assignments"`
}

func newOpenSupervisionAssignmentsTool(rt *Runtime) toolregistry.Tool {
	return toolregistry.Tool{Name: "open_supervision_assignments", Description: "Atomically open pre-authorized assignments for derived CoSuper actors. Reuse command_id exactly on a retry.", Parameters: toolregistry.JSONSchemaObject(map[string]any{
		"command_id": map[string]any{"type": "string", "format": "uuid"},
		"assignments": map[string]any{"type": "array", "minItems": 1, "maxItems": 64, "items": map[string]any{"type": "object", "properties": map[string]any{
			"assignment_id": map[string]any{"type": "string", "format": "uuid"}, "assigned_agent_id": map[string]any{"type": "string"}, "parent_decision_id": map[string]any{"type": "string"}, "scope_digest": map[string]any{"type": "string"}, "capability_digest": map[string]any{"type": "string"}, "policy_digest": map[string]any{"type": "string"}, "obligation_ids": map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string"}}, "idempotency_commitment": map[string]any{"type": "string"},
		}, "required": []string{"assignment_id", "parent_decision_id", "scope_digest", "capability_digest", "policy_digest", "obligation_ids", "idempotency_commitment"}, "additionalProperties": false}},
	}, []string{"command_id", "assignments"}, false), Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
		var in openSupervisionAssignmentsArgs
		if err := json.Unmarshal(raw, &in); err != nil {
			return "", fmt.Errorf("decode open_supervision_assignments args: %w", err)
		}
		exec := toolregistry.ExecutionContextFrom(ctx)
		if agentprofile.Canonical(exec.Profile) != agentprofile.Super || exec.RunRecord == nil {
			return "", fmt.Errorf("open_supervision_assignments requires trusted Super run context")
		}
		if _, err := uuid.Parse(in.CommandID); err != nil {
			return "", fmt.Errorf("open_supervision_assignments command_id must be a UUID")
		}
		snapshot, supervised, err := rt.supervisionSnapshotForRun(ctx, exec.RunRecord)
		if err != nil {
			return "", err
		}
		if !supervised {
			return "", fmt.Errorf("open_supervision_assignments requires a supervision trajectory")
		}
		base, err := supervisionObservedBase(snapshot)
		if err != nil {
			return "", err
		}
		seen := make(map[string]struct{}, len(in.Assignments))
		mutations := make([]computerevent.SupervisionMutation, 0, len(in.Assignments))
		ids := make([]string, 0, len(in.Assignments))
		for _, a := range in.Assignments {
			if _, err := uuid.Parse(a.AssignmentID); err != nil {
				return "", fmt.Errorf("assignment_id must be a UUID")
			}
			if _, ok := seen[a.AssignmentID]; ok {
				return "", fmt.Errorf("duplicate assignment_id %q", a.AssignmentID)
			}
			seen[a.AssignmentID] = struct{}{}
			actorID := supervisedCoSuperAgentID(a.AssignmentID)
			if supplied := strings.TrimSpace(a.AssignedAgentID); supplied != "" && supplied != actorID {
				return "", fmt.Errorf("assigned_agent_id must be the trusted assignment actor %q", actorID)
			}
			body, err := json.Marshal(map[string]any{"assignment_id": a.AssignmentID, "assigned_actor_id": actorID, "assigned_role": agentprofile.CoSuper, "parent_decision_id": a.ParentDecisionID, "intent_revision_id": snapshot.IntentRevisionID, "observed_base": base, "scope_digest": a.ScopeDigest, "capability_digest": a.CapabilityDigest, "policy_digest": a.PolicyDigest, "obligation_ids": a.ObligationIDs, "idempotency_commitment": a.IdempotencyCommitment})
			if err != nil {
				return "", err
			}
			mutations = append(mutations, computerevent.SupervisionMutation{Kind: "assignment_opened", Body: body})
			ids = append(ids, a.AssignmentID)
		}
		transaction := computerevent.SupervisionTransaction{Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1, DigestRecipe: computerevent.SupervisionDigestRecipeV1, TransactionID: in.CommandID, TransactionClass: "open_assignment", OwnerID: exec.OwnerID, ComputerID: exec.SandboxID, TrajectoryID: trajectoryIDForRun(exec.RunRecord), CommandID: in.CommandID, Actor: computerevent.SupervisionActor{ActorID: agentIDForRun(exec.RunRecord), Role: "super", AuthorityRef: "run:" + exec.RunID}, Expected: supervisionExpected(snapshot), ObservedBase: &base, Mutations: mutations}
		_, head, err := rt.AppendSupervisionTransaction(ctx, transaction)
		if err != nil {
			return "", fmt.Errorf("open supervision assignments: %w", err)
		}
		return toolregistry.ResultJSON(map[string]any{"assignment_ids": ids, "canonical_event_head": head})
	}}
}

type recordSupervisionTransitionArgs struct {
	CommandID        string `json:"command_id"`
	TransactionClass string `json:"transaction_class"`
	Mutations        []struct {
		Kind string          `json:"kind"`
		Body json.RawMessage `json:"body"`
	} `json:"mutations"`
	PrivateArtifacts []struct {
		BindingID string `json:"binding_id"`
		MediaType string `json:"media_type"`
		Plaintext string `json:"plaintext"`
	} `json:"private_artifacts,omitempty"`
}

var superTransitionKinds = map[string]map[string]struct{}{
	"record_belief":         {"super_belief_recorded": {}},
	"record_finding":        {"super_finding_recorded": {}},
	"record_dissent":        {"dissent_recorded": {}},
	"record_reconciliation": {"super_reconciliation_recorded": {}},
	"propose_decision":      {"super_decision_proposed": {}},
	"cancel_assignment":     {"assignment_cancelled": {}},
	"record_disposition":    {"disposition_recorded": {}},
	"propose_settlement":    {"settlement_proposed": {}},
}

func newRecordSupervisionTransitionTool(rt *Runtime) toolregistry.Tool {
	classes := make([]string, 0, len(superTransitionKinds))
	kinds := make(map[string]struct{})
	for class, allowedKinds := range superTransitionKinds {
		classes = append(classes, class)
		for kind := range allowedKinds {
			kinds[kind] = struct{}{}
		}
	}
	sort.Strings(classes)
	kindNames := make([]string, 0, len(kinds))
	for kind := range kinds {
		kindNames = append(kindNames, kind)
	}
	sort.Strings(kindNames)
	return toolregistry.Tool{
		Name:        "record_supervision_transition",
		Description: "Append one exact typed Super transition. Use only the enumerated transaction class and matching closed mutation body; reuse command_id exactly on retry.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"command_id":        map[string]any{"type": "string", "format": "uuid"},
			"transaction_class": map[string]any{"type": "string", "enum": classes},
			"mutations": map[string]any{"type": "array", "minItems": 1, "maxItems": 64, "items": map[string]any{
				"type": "object", "properties": map[string]any{
					"kind": map[string]any{"type": "string", "enum": kindNames},
					"body": map[string]any{"type": "object"},
				}, "required": []string{"kind", "body"}, "additionalProperties": false,
			}},
			"private_artifacts": map[string]any{"type": "array", "items": map[string]any{
				"type": "object", "properties": map[string]any{
					"binding_id": map[string]any{"type": "string"},
					"media_type": map[string]any{"type": "string"},
					"plaintext":  map[string]any{"type": "string"},
				}, "required": []string{"binding_id", "media_type", "plaintext"}, "additionalProperties": false,
			}, "description": "Private payloads owned by this command. Reference a payload from mutation body fields as $private:<binding_id>; runtime replaces it with the authenticated artifact ref after command reservation."},
		}, []string{"command_id", "transaction_class", "mutations"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in recordSupervisionTransitionArgs
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode record_supervision_transition args: %w", err)
			}
			exec := toolregistry.ExecutionContextFrom(ctx)
			if agentprofile.Canonical(exec.Profile) != agentprofile.Super || exec.RunRecord == nil {
				return "", fmt.Errorf("record_supervision_transition requires trusted Super run context")
			}
			if _, err := uuid.Parse(in.CommandID); err != nil {
				return "", fmt.Errorf("record_supervision_transition command_id must be a UUID")
			}
			allowedKinds, ok := superTransitionKinds[strings.TrimSpace(in.TransactionClass)]
			if !ok {
				return "", fmt.Errorf("unsupported Super transaction_class %q", in.TransactionClass)
			}
			mutations := make([]computerevent.SupervisionMutation, 0, len(in.Mutations))
			for _, mutation := range in.Mutations {
				if _, ok := allowedKinds[mutation.Kind]; !ok {
					return "", fmt.Errorf("mutation %q is not valid for %s", mutation.Kind, in.TransactionClass)
				}
				mutations = append(mutations, computerevent.SupervisionMutation{Kind: mutation.Kind, Body: mutation.Body})
			}
			payloads := make([]computerevent.PrivateSupervisionArtifactPayload, 0, len(in.PrivateArtifacts))
			seenBindings := make(map[string]struct{}, len(in.PrivateArtifacts))
			for _, artifact := range in.PrivateArtifacts {
				bindingID := strings.TrimSpace(artifact.BindingID)
				mediaType := strings.TrimSpace(artifact.MediaType)
				if bindingID == "" || mediaType == "" || artifact.Plaintext == "" {
					return "", fmt.Errorf("private_artifacts require binding_id, media_type, and plaintext")
				}
				if _, exists := seenBindings[bindingID]; exists {
					return "", fmt.Errorf("duplicate private artifact binding %q", bindingID)
				}
				seenBindings[bindingID] = struct{}{}
				referenced := replaceTransitionPrivateArtifactPlaceholder(mutations, bindingID)
				if !referenced {
					return "", fmt.Errorf("private artifact binding %q is not referenced by a mutation body", bindingID)
				}
				payloads = append(payloads, computerevent.PrivateSupervisionArtifactPayload{
					BindingID: bindingID, MediaType: mediaType, Plaintext: []byte(artifact.Plaintext),
				})
			}
			snapshot, supervised, err := rt.supervisionSnapshotForRun(ctx, exec.RunRecord)
			if err != nil {
				return "", err
			}
			if !supervised {
				return "", fmt.Errorf("record_supervision_transition requires a supervision trajectory")
			}
			base, err := supervisionObservedBase(snapshot)
			if err != nil {
				return "", err
			}
			transaction := computerevent.SupervisionTransaction{
				Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1,
				DigestRecipe: computerevent.SupervisionDigestRecipeV1, TransactionID: in.CommandID,
				TransactionClass: in.TransactionClass, OwnerID: exec.OwnerID, ComputerID: exec.SandboxID,
				TrajectoryID: trajectoryIDForRun(exec.RunRecord), CommandID: in.CommandID,
				Actor:    computerevent.SupervisionActor{ActorID: agentIDForRun(exec.RunRecord), Role: "super", AuthorityRef: "run:" + exec.RunID},
				Expected: supervisionExpected(snapshot), ObservedBase: &base, Mutations: mutations,
			}
			_, artifactDigest, _, err := rt.AppendSupervisionTransactionWithPrivateArtifacts(ctx, transaction, payloads)
			if err != nil {
				return "", fmt.Errorf("record Super supervision transition: %w", err)
			}

			return toolregistry.ResultJSON(map[string]any{
				"command_id": in.CommandID, "transaction_class": in.TransactionClass,
				"transaction_artifact_digest": artifactDigest, "status": "recorded",
			})
		},
	}
}

func replaceTransitionPrivateArtifactPlaceholder(mutations []computerevent.SupervisionMutation, bindingID string) bool {
	placeholder := []byte(computerevent.SupervisionArtifactPlaceholder(bindingID))
	logicalRef := []byte("$private:" + bindingID)
	referenced := false
	for index := range mutations {
		if bytes.Contains(mutations[index].Body, logicalRef) {
			referenced = true
			mutations[index].Body = bytes.ReplaceAll(mutations[index].Body, logicalRef, placeholder)
		}
	}
	return referenced
}
