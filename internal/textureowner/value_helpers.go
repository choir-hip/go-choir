package textureowner

import (
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func cloneMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(metadata))
	for key, value := range metadata {
		out[key] = value
	}
	return out
}

func truncateRunes(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}

func isTextureAgentID(agentID string) bool {
	return strings.HasPrefix(strings.TrimSpace(agentID), "texture:")
}

type submitCoagentUpdateArgs struct {
	AgentID   string `json:"agent_id"`
	ChannelID string `json:"channel_id,omitempty"`
	types.CoagentSourcePacketPayload
}

func copyStringAnyMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func persistentSuperAgentID(ownerID string) string {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return "super"
	}
	return "super:" + ownerID
}

func coagentPacketSourceURIs(packet types.CoagentSourcePacketPayload, kinds ...string) []string {
	wanted := make(map[string]bool, len(kinds))
	for _, kind := range kinds {
		if kind = strings.TrimSpace(kind); kind != "" {
			wanted[kind] = true
		}
	}
	var out []string
	for _, source := range packet.Sources {
		if len(wanted) > 0 && !wanted[strings.TrimSpace(source.Kind)] {
			continue
		}
		if uri := strings.TrimSpace(source.Target.URI); uri != "" {
			out = append(out, uri)
		}
	}
	return out
}

func newCoagentPacket(kind, summary string, claims []types.CoagentPacketClaim, sources []types.CoagentPacketSource, actions []types.CoagentPacketAction, questions, notes []string) types.CoagentSourcePacketPayload {
	return types.CoagentSourcePacketPayload{
		SchemaVersion: types.CoagentSourcePacketSchemaV1,
		Kind:          strings.TrimSpace(kind),
		Summary:       strings.TrimSpace(summary),
		Claims:        claims,
		Sources:       sources,
		Actions:       actions,
		Questions:     trimNonEmpty(questions),
		Notes:         trimNonEmpty(notes),
	}
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
		if !ok || seen[source.SourceID] {
			continue
		}
		seen[source.SourceID] = true
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
		return coagentSourceFromURI("src-"+sanitizeExportPart(ref), "web_url", ref, ""), true
	}
	if key == "" && looksLikeArtifactPath(ref) {
		key, value, uri = "file_artifact", ref, "file_artifact:"+ref
	}
	if key == "" || value == "" {
		return types.CoagentPacketSource{}, false
	}
	kind := key
	switch key {
	case "content_id", "evidence":
		kind = "content_item"
	case "source_service_item":
	default:
		if !executionTargetKind(kind) {
			return types.CoagentPacketSource{}, false
		}
	}
	return coagentSourceFromURI("src-"+sanitizeExportPart(uri), kind, uri, ""), true
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
		claims = append(claims, types.CoagentPacketClaim{
			Text: text, SourceIDs: sourceIDs, Stance: "supports", RecommendedSurface: "decision_log",
		})
	}
	return claims
}

func sanitizeExportPart(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "run"
	}
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		} else if b.Len() > 0 && b.String()[b.Len()-1] != '-' {
			b.WriteByte('-')
		}
	}
	if out := strings.Trim(b.String(), "-"); out != "" {
		return out
	}
	return "run"
}
