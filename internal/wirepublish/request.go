package wirepublish

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// BuildAutonomousPublishRequest shapes a platformd publish request from re-loaded
// Dolt state. Access and export policies are forced server-side.
func BuildAutonomousPublishRequest(doc types.Document, rev types.Revision, rec *types.RunRecord, enrichedMetadata json.RawMessage) PublishVTextRequest {
	meta := decodeMetadata(enrichedMetadata)
	meta["publication_kind"] = PublicationKind
	meta["revision_role"] = RevisionRoleCanonical
	if cycleID := sourceNetworkCycleID(meta); cycleID != "" {
		meta["source_network_cycle_id"] = cycleID
	}
	if rec != nil {
		if intent := metadataString(rec.Metadata, "request_intent"); intent != "" {
			meta["wire_request_intent"] = intent
		}
		if strings.TrimSpace(rec.RunID) != "" {
			meta["wire_run_id"] = strings.TrimSpace(rec.RunID)
		}
	}
	delete(meta, "access_policy")
	delete(meta, "route_policy")
	merged, _ := json.Marshal(meta)

	traceID := strings.TrimSpace(rev.RevisionID)
	if rec != nil && strings.TrimSpace(rec.RunID) != "" {
		traceID = strings.TrimSpace(rec.RunID) + ":" + strings.TrimSpace(rev.RevisionID)
	}

	return PublishVTextRequest{
		OwnerID:          PlatformOwnerID(),
		SourceDocID:      doc.DocID,
		SourceRevisionID: rev.RevisionID,
		Title:            doc.Title,
		Content:          rev.Content,
		Citations:        rev.Citations,
		Metadata:         merged,
		AccessPolicy:     defaultWireAccessPolicy(),
		ExportPolicy:     defaultWireExportPolicy(),
		SourceTraceID:    traceID,
		RequestedBy:      RequestedByWirePolicy,
	}
}

func defaultWireAccessPolicy() json.RawMessage {
	raw, err := json.Marshal(map[string]any{
		"visibility": "public",
		"route":      "public",
	})
	if err != nil {
		panic(fmt.Sprintf("wirepublish: marshal access policy: %v", err))
	}
	return raw
}

func defaultWireExportPolicy() json.RawMessage {
	raw, err := json.Marshal(map[string]any{
		"copy_allowed":     true,
		"download_allowed": true,
		"formats":          []string{"txt", "md", "html", "docx", "pdf"},
	})
	if err != nil {
		panic(fmt.Sprintf("wirepublish: marshal export policy: %v", err))
	}
	return raw
}
