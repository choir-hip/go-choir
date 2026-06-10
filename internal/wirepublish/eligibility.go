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
)

// PlatformOwnerID returns the durable owner id for the Universal Wire platform computer.
func PlatformOwnerID() string {
	return "universal-wire-platform"
}

// EligibleForAutonomousPublish reports whether a re-loaded revision may enter the
// wire publication-policy choke point.
func EligibleForAutonomousPublish(doc types.Document, rev types.Revision, rec *types.RunRecord, platformOwnerID string) bool {
	if rec == nil || strings.TrimSpace(doc.DocID) == "" {
		return false
	}
	if strings.TrimSpace(rec.OwnerID) != strings.TrimSpace(platformOwnerID) {
		return false
	}
	if strings.TrimSpace(doc.OwnerID) != strings.TrimSpace(platformOwnerID) {
		return false
	}
	if !IsWireArticleRevisionRun(rec) {
		return false
	}
	meta := decodeMetadata(rev.Metadata)
	if metadataString(meta, "source") != "edit_vtext" {
		return false
	}
	if sourceNetworkCycleID(meta) == "" || !RevisionIsPublishableWireArticle(meta) {
		return false
	}
	content := strings.TrimSpace(rev.Content)
	if content == "" || articleContentLooksLikeSeed(content) {
		return false
	}
	return true
}

// IsWireArticleRevisionRun reports whether a VText run is part of the Universal
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
	if metadataString(rec.Metadata, "type") != "vtext_agent_revision" {
		return false
	}
	return sourceNetworkCycleID(rec.Metadata) != ""
}

// RevisionIsPublishableWireArticle reports whether revision metadata describes a
// reader-facing wire article (not a seed brief), including worker-integration
// edits that failed to promote revision_role to canonical.
func RevisionIsPublishableWireArticle(meta map[string]any) bool {
	if revisionIsCanonicalArticle(meta) && !revisionIsInput(meta) {
		return true
	}
	if metadataString(meta, "source") == "edit_vtext" &&
		sourceNetworkCycleID(meta) != "" &&
		metadataString(meta, "vtext_edit_kind") == "vtext_edit" {
		return true
	}
	return false
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
