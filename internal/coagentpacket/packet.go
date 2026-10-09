// Package coagentpacket is the single contract for coagent source packets
// (coagent_source_packet.v1): the reducer validates a committed report with
// it, and the cell kernel validates choir.ReportPacket at the call, so a
// desk learns of a malformed packet in the same cell
// (docs/problems/research-report-packets-rejected-by-schema-2026-10-09.md).
package coagentpacket

import (
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func Normalize(packet types.CoagentSourcePacketPayload) types.CoagentSourcePacketPayload {
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

func Validate(packet types.CoagentSourcePacketPayload) error {
	if packet.SchemaVersion != types.CoagentSourcePacketSchemaV1 {
		return fmt.Errorf("update_coagent schema_version must be %q", types.CoagentSourcePacketSchemaV1)
	}
	if !validCoagentPacketKind(packet.Kind) {
		return fmt.Errorf("update_coagent kind %q is not supported", packet.Kind)
	}
	if packet.Summary == "" {
		return fmt.Errorf("update_coagent summary is required")
	}
	if d := packet.WorkDisposition; d != "" && d != types.WorkItemOpen && d != types.WorkItemCompleted {
		return fmt.Errorf("update_coagent work_disposition %q must be open or completed", d)
	}
	if Empty(packet) {
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
	case "evidence_update", "execution_request", "execution_result", "blocker", "question", "proposal", "decision_request", "directive":
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

func Empty(packet types.CoagentSourcePacketPayload) bool {
	return len(packet.Claims) == 0 &&
		len(packet.Sources) == 0 &&
		len(packet.Actions) == 0 &&
		len(packet.Questions) == 0 &&
		len(packet.Notes) == 0
}

func trimNonEmpty(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
