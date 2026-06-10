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
		"source":                   "edit_vtext",
		"revision_role":            RevisionRoleCanonical,
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
		"source":                   "edit_vtext",
		"revision_role":            RevisionRoleInput,
		"ingestion_handoff_cycle_id": "cycle-1",
	})
	inputRev := rev
	inputRev.Metadata = inputMeta
	if EligibleForAutonomousPublish(doc, inputRev, rec, owner) {
		t.Fatal("seed-brief input revisions must not be eligible")
	}
}

func TestEligibleForAutonomousPublishAcceptsWorkerIntegrationArticleEdit(t *testing.T) {
	owner := PlatformOwnerID()
	meta, _ := json.Marshal(map[string]any{
		"source":                     "edit_vtext",
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
	rec := &types.RunRecord{
		OwnerID: owner,
		RunID:   "run-worker",
		Metadata: map[string]any{
			"type":                       "vtext_agent_revision",
			"request_intent":             "integrate_worker_findings",
			"ingestion_handoff_cycle_id": "cycle-worker-1",
		},
	}
	if !EligibleForAutonomousPublish(doc, rev, rec, owner) {
		t.Fatal("worker-integration article edits with ingestion lineage should be eligible")
	}
}

func TestEligibleForAutonomousPublishRejectsSeedBrief(t *testing.T) {
	owner := PlatformOwnerID()
	meta, _ := json.Marshal(map[string]any{
		"source":                   "edit_vtext",
		"revision_role":            RevisionRoleCanonical,
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
