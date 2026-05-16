package runtime

import (
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestDelegateWorkerCheckpointUpdatePreservesTypedExportPatchsets(t *testing.T) {
	rec := &types.RunRecord{
		RunID:   "super-run-1",
		AgentID: "super:user-alice",
		OwnerID: "user-alice",
		Metadata: map[string]any{
			runMetadataTrajectoryID: "trajectory-1",
		},
	}
	output := map[string]any{
		"status":                       "worker_run_completed",
		"state":                        types.RunCompleted,
		"loop_id":                      "worker-run-1",
		"worker_vm_id":                 "vm-1",
		"worker_id":                    "worker-1",
		"event_count":                  12,
		"worker_root_event_count":      5,
		"worker_child_run_ids":         []string{"implementation-run-1", "verifier-run-1"},
		"worker_child_event_counts":    map[string]int{"implementation-run-1": 7, "verifier-run-1": 3},
		"worker_channel_message_count": 3,
		"worker_spawned_profiles":      []string{AgentProfileCoSuper},
		"worker_event_summary": []map[string]any{
			{
				"kind":           types.EventToolResult,
				"tool":           "spawn_agent",
				"output_excerpt": `{"agent_id":"agent-implementation-1","loop_id":"implementation-run-1","channel_id":"channel-implementation-1","profile":"co-super","state":"completed"}`,
			},
			{
				"kind":            types.EventChannelMessage,
				"role":            "result",
				"from_agent_id":   "agent-verifier-1",
				"to_agent_id":     "agent-vsuper-1",
				"content_excerpt": "Verifier observed exported patchset evidence and no fake island placeholders.",
			},
		},
		"export_patchsets": []map[string]any{{
			"manifest_path":   "/mnt/persistent/files/patchsets/manifest.json",
			"patchset_path":   "/mnt/persistent/files/patchsets/changes.patch",
			"base_sha":        "base-1",
			"worker_head":     "head-1",
			"patchset_sha256": "patch-sha-1",
		}},
		"promotion_queue": []map[string]any{{
			"candidate_id":       "candidate-1",
			"status":             "queued",
			"integration_branch": "agent/implementation-run-1/candidate",
			"destination_branch": "main",
			"manifest_path":      "/mnt/persistent/promotion-artifacts/candidate-1/manifest.json",
			"patchset_path":      "/mnt/persistent/promotion-artifacts/candidate-1/changes.patch",
			"base_sha":           "base-1",
			"worker_head":        "head-1",
			"patchset_sha256":    "patch-sha-1",
		}},
	}

	update := delegateWorkerCheckpointUpdate(rec, output, "vtext:doc-1", "doc-1", "terminal_result", time.Unix(0, 0).UTC())
	joinedFindings := strings.Join(update.Findings, "\n")
	if !strings.Contains(joinedFindings, `worker state "completed"`) {
		t.Fatalf("checkpoint findings did not preserve typed worker state: %#v", update.Findings)
	}
	if !strings.Contains(joinedFindings, "returned 1 export patchset") {
		t.Fatalf("checkpoint findings did not preserve export count: %#v", update.Findings)
	}
	if strings.Contains(joinedFindings, "no export patchsets") {
		t.Fatalf("checkpoint still reported missing exports: %#v", update.Findings)
	}
	if !containsString(update.Artifacts, "/mnt/persistent/files/patchsets/manifest.json") ||
		!containsString(update.Artifacts, "/mnt/persistent/files/patchsets/changes.patch") {
		t.Fatalf("checkpoint artifacts missing export refs: %#v", update.Artifacts)
	}
	for _, want := range []string{
		"worker_child_loop:implementation-run-1",
		"worker_child_loop:verifier-run-1",
		"worker_child_agent:agent-implementation-1",
		"worker_channel:channel-implementation-1",
		"promotion_candidate:candidate-1",
	} {
		if !containsString(update.EvidenceIDs, want) {
			t.Fatalf("checkpoint evidence ids missing %q: %#v", want, update.EvidenceIDs)
		}
	}
	joinedRefs := strings.Join(update.Refs, "\n")
	for _, want := range []string{
		"worker_state:completed",
		"worker_head:head-1",
		"patchset_sha256:patch-sha-1",
		"worker_channel_message:agent-verifier-1->agent-vsuper-1",
	} {
		if !strings.Contains(joinedRefs, want) {
			t.Fatalf("checkpoint refs missing %q: %#v", want, update.Refs)
		}
	}
	joinedNotes := strings.Join(update.Notes, "\n")
	for _, want := range []string{
		"worker_child_event_count:implementation-run-1=7",
		"worker_root_event_count=5",
		"Verifier observed exported patchset evidence",
	} {
		if !strings.Contains(joinedNotes, want) {
			t.Fatalf("checkpoint notes missing %q: %#v", want, update.Notes)
		}
	}
}
