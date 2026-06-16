package wirepublish

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestEligibleForAutonomousPublishRequiresCanonicalArticleRevision(t *testing.T) {
	owner := PlatformOwnerID()
	meta, _ := json.Marshal(map[string]any{
		"source":                     "edit_texture",
		"revision_role":              RevisionRoleCanonical,
		"ingestion_handoff_cycle_id": "cycle-1",
	})
	rev := types.Revision{
		RevisionID: "rev-1",
		DocID:      "doc-1",
		OwnerID:    owner,
		Content:    "# Story\n\nMADRID -- Officials confirmed the route change.",
		Metadata:   meta,
	}
	doc := types.Document{DocID: "doc-1", OwnerID: owner, Title: "Story.vtext"}
	rec := &types.RunRecord{
		OwnerID: owner,
		RunID:   "run-1",
		Metadata: map[string]any{
			"request_intent": "universal_wire_processor_article_revision",
		},
	}
	if !EligibleForAutonomousPublish(doc, rev, rec, owner) {
		t.Fatal("expected eligible canonical wire article revision")
	}

	inputMeta, _ := json.Marshal(map[string]any{
		"source":                     "edit_texture",
		"revision_role":              RevisionRoleInput,
		"ingestion_handoff_cycle_id": "cycle-1",
	})
	inputRev := rev
	inputRev.Metadata = inputMeta
	if EligibleForAutonomousPublish(doc, inputRev, rec, owner) {
		t.Fatal("seed-brief input revisions must not be eligible")
	}
}

func TestEligibleForAutonomousPublishAcceptsLegacyEditVTextSource(t *testing.T) {
	owner := PlatformOwnerID()
	meta, _ := json.Marshal(map[string]any{
		"source":                     "edit_vtext", // texture-cutover-allow: deletion receipt remove after legacy revision metadata migration
		"revision_role":              RevisionRoleCanonical,
		"ingestion_handoff_cycle_id": "cycle-legacy",
	})
	rev := types.Revision{
		RevisionID: "rev-legacy",
		DocID:      "doc-legacy",
		OwnerID:    owner,
		Content:    "# Story\n\nMADRID -- Officials confirmed the route change.",
		Metadata:   meta,
	}
	doc := types.Document{DocID: "doc-legacy", OwnerID: owner, Title: "Story.vtext"}
	rec := &types.RunRecord{
		OwnerID: owner,
		RunID:   "run-legacy",
		Metadata: map[string]any{
			"request_intent": "universal_wire_processor_article_revision",
		},
	}
	if !EligibleForAutonomousPublish(doc, rev, rec, owner) {
		t.Fatal("legacy edit-source revisions should remain eligible during Texture metadata migration")
	}
}

func TestEligibleForAutonomousPublishAcceptsWorkerIntegrationArticleEdit(t *testing.T) {
	owner := PlatformOwnerID()
	meta, _ := json.Marshal(map[string]any{
		"source":                     "edit_texture",
		"revision_role":              RevisionRoleInput,
		"artifact_kind":              "source_brief",
		"vtext_edit_kind":            "vtext_edit",
		"ingestion_handoff_cycle_id": "cycle-worker-1",
	})
	rev := types.Revision{
		RevisionID: "rev-worker",
		DocID:      "doc-worker",
		OwnerID:    owner,
		Content:    "# Story\n\nMADRID -- Officials confirmed the route change.",
		Metadata:   meta,
	}
	doc := types.Document{DocID: "doc-worker", OwnerID: owner, Title: "Story.vtext"}
	for _, taskType := range []string{textureAgentRevisionTaskType, legacyVTextAgentRevisionTaskType} {
		t.Run(taskType, func(t *testing.T) {
			rec := &types.RunRecord{
				OwnerID: owner,
				RunID:   "run-worker",
				Metadata: map[string]any{
					"type":                       taskType,
					"request_intent":             "integrate_worker_findings",
					"ingestion_handoff_cycle_id": "cycle-worker-1",
				},
			}
			if !EligibleForAutonomousPublish(doc, rev, rec, owner) {
				t.Fatal("worker-integration article edits with ingestion lineage should be eligible")
			}
		})
	}
}

func TestEligibleForAutonomousPublishStagingRevisionFixture(t *testing.T) {
	owner := PlatformOwnerID()
	meta, _ := json.Marshal(map[string]any{
		"source":                     "edit_texture",
		"revision_role":              RevisionRoleInput,
		"artifact_kind":              "source_brief",
		"vtext_edit_kind":            "vtext_edit",
		"ingestion_handoff_cycle_id": "cycle_b692f2803101f30af0a1bcbb",
	})
	rev := types.Revision{
		RevisionID: "7cdd5c3e-43e4-4ed7-bff7-e32b80188349",
		DocID:      "15f7405a-108b-4b44-acc5-3f3be11ff4e6",
		OwnerID:    owner,
		Content:    "# Nuclear and Natural Gas Are Teaming Up to Power the AI Data Center Boom\n\nThe electricity demands",
		Metadata:   meta,
	}
	doc := types.Document{DocID: rev.DocID, OwnerID: owner, Title: "Nuclear.vtext"}
	rec := &types.RunRecord{
		OwnerID: owner,
		Metadata: map[string]any{
			"request_intent": "universal_wire_processor_article_revision",
		},
	}
	if !EligibleForAutonomousPublish(doc, rev, rec, owner) {
		t.Fatal("staging fixture revision should be eligible after worker-integration fix")
	}
}

func TestEligibleForAutonomousPublishRejectsSeedBrief(t *testing.T) {
	owner := PlatformOwnerID()
	meta, _ := json.Marshal(map[string]any{
		"source":                     "edit_texture",
		"revision_role":              RevisionRoleCanonical,
		"ingestion_handoff_cycle_id": "cycle-1",
	})
	rev := types.Revision{
		RevisionID: "rev-seed",
		DocID:      "doc-seed",
		OwnerID:    owner,
		Content:    "## Source Brief\n\nProcessor handoff only.",
		Metadata:   meta,
		CreatedAt:  time.Now().UTC(),
	}
	doc := types.Document{DocID: "doc-seed", OwnerID: owner}
	rec := &types.RunRecord{
		OwnerID: owner,
		Metadata: map[string]any{
			"request_intent": "universal_wire_processor_article_revision",
		},
	}
	if EligibleForAutonomousPublish(doc, rev, rec, owner) {
		t.Fatal("seed brief content should not be eligible")
	}
	if !articleContentLooksLikeSeed(rev.Content) {
		t.Fatal("seed heuristic should match fixture content")
	}
}

func TestEligibleForAutonomousPublishAcceptsRevisionLineageWithoutRunMetadata(t *testing.T) {
	owner := PlatformOwnerID()
	meta, _ := json.Marshal(map[string]any{
		"source":                      "edit_texture",
		"revision_role":               RevisionRoleCanonical,
		"vtext_edit_kind":             "vtext_edit",
		"source_network_cycle_id":     "cycle-live-1",
		"source_network_request_kind": "processor",
	})
	rev := types.Revision{
		RevisionID: "rev-live",
		DocID:      "doc-live",
		OwnerID:    owner,
		Content:    "# Story\n\nReal publishable article body.",
		Metadata:   meta,
	}
	doc := types.Document{DocID: rev.DocID, OwnerID: owner, Title: "Live.vtext"}
	rec := &types.RunRecord{OwnerID: owner, Metadata: map[string]any{"request_intent": "integrate_worker_findings"}}
	if !EligibleForAutonomousPublish(doc, rev, rec, owner) {
		t.Fatal("revision lineage should make live worker-integration article publishable")
	}
}
