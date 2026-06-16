package wirepublish

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	RevisionRoleInput     = "input"
	RevisionRoleCanonical = "canonical"

	RequestedByWirePolicy = "wire_publication_policy"
	PublicationKind       = "universal_wire_autonomous"

	patchTextureSource   = "patch_texture"
	rewriteTextureSource = "rewrite_texture"
	editTextureSource    = "edit_texture"

	textureAgentRevisionTaskType = "texture_agent_revision"
)

// PlatformOwnerID returns the durable owner id for the Universal Wire platform computer.
func PlatformOwnerID() string {
	return "universal-wire-platform"
}

// EligibleForAutonomousPublish reports whether a re-loaded revision may enter the
// wire publication-policy choke point (proxy re-reads revision metadata here).
func EligibleForAutonomousPublish(doc types.Document, rev types.Revision, rec *types.RunRecord, platformOwnerID string) bool {
	if strings.TrimSpace(doc.DocID) == "" {
		return false
	}
	if strings.TrimSpace(doc.OwnerID) != strings.TrimSpace(platformOwnerID) {
		return false
	}
	meta := decodeMetadata(rev.Metadata)
	if !isTextureEditSource(metadataString(meta, "source")) {
		return false
	}
	if sourceNetworkCycleID(meta) == "" || !RevisionIsPublishableWireArticle(meta) {
		return false
	}
	if rec != nil {
		if strings.TrimSpace(rec.OwnerID) != strings.TrimSpace(platformOwnerID) {
			return false
		}
		if !IsWireArticleRevisionRun(rec) && !revisionCarriesWireLineage(meta) {
			return false
		}
	} else if !revisionCarriesWireLineage(meta) {
		return false
	}
	content := strings.TrimSpace(rev.Content)
	if content == "" || articleContentLooksLikeSeed(content) {
		return false
	}
	return true
}

// IsWireArticleRevisionRun reports whether a Texture run is part of the Universal
// Wire article pipeline. Processor/reconciler handoffs use universal_wire_* intents;
// worker-integration child runs inherit ingestion lineage without that intent.
func IsWireArticleRevisionRun(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	intent := metadataString(rec.Metadata, "request_intent")
	if strings.HasPrefix(intent, "universal_wire_") && strings.HasSuffix(intent, "_article_revision") {
		return true
	}
	if !isTextureAgentRevisionTaskType(metadataString(rec.Metadata, "type")) {
		return false
	}
	return sourceNetworkCycleID(rec.Metadata) != ""
}

func isTextureAgentRevisionTaskType(value string) bool {
	switch strings.TrimSpace(value) {
	case textureAgentRevisionTaskType:
		return true
	default:
		return false
	}
}

// RevisionIsPublishableWireArticle reports whether revision metadata describes a
// reader-facing wire article (not a seed brief), including worker-integration
// edits that failed to promote revision_role to canonical.
func RevisionIsPublishableWireArticle(meta map[string]any) bool {
	if revisionIsCanonicalArticle(meta) && !revisionIsInput(meta) {
		return true
	}
	if isTextureEditSource(metadataString(meta, "source")) &&
		sourceNetworkCycleID(meta) != "" &&
		metadataString(meta, "texture_edit_kind") == "texture_edit" {
		return true
	}
	return false
}

func isTextureEditSource(source string) bool {
	switch source {
	case patchTextureSource, rewriteTextureSource, editTextureSource:
		return true
	default:
		return false
	}
}

func revisionIsCanonicalArticle(meta map[string]any) bool {
	if metadataString(meta, "revision_role") == RevisionRoleCanonical {
		return true
	}
	if v, ok := meta["article_version"].(bool); ok && v {
		return true
	}
	return false
}

func revisionIsInput(meta map[string]any) bool {
	return metadataString(meta, "revision_role") == RevisionRoleInput
}

func sourceNetworkCycleID(meta map[string]any) string {
	return firstNonEmpty(
		metadataString(meta, "source_network_cycle_id"),
		metadataString(meta, "ingestion_handoff_cycle_id"),
	)
}

func revisionCarriesWireLineage(meta map[string]any) bool {
	if sourceNetworkCycleID(meta) == "" {
		return false
	}
	kind := metadataString(meta, "source_network_request_kind")
	if kind == "" {
		kind = metadataString(meta, "ingestion_handoff_request_kind")
	}
	if kind == "processor" || kind == "reconciler" {
		return true
	}
	if metadataString(meta, "input_origin") == "processor_handoff" {
		return true
	}
	if metadataString(meta, "processor_key") != "" {
		return true
	}
	return false
}

func articleContentLooksLikeSeed(content string) bool {
	return strings.Contains(content, "## Source Brief") ||
		strings.Contains(content, "## Evidence Gathering") ||
		strings.Contains(content, "## Working Revision")
}

func decodeMetadata(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func metadataString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	if v, ok := meta[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return strings.TrimSpace(strings.Trim(fmt.Sprint(meta[key]), `"`))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
