package runtime

import (
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestDelegateWorkerCheckpointUpdatePreservesTypedAppChangePackages(t *testing.T) {
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
				"content_excerpt": "Verifier observed AppChangePackage evidence and no fake island placeholders.",
			},
		},
		"app_change_packages": []map[string]any{{
			"package_id":                  "package-1",
			"base_sha":                    "base-1",
			"worker_head":                 "head-1",
			"package_manifest_sha256":     "manifest-sha-1",
			"runtime_source_delta_sha256": "runtime-sha-1",
			"ui_source_delta_sha256":      "ui-sha-1",
		}},
		"app_adoptions": []map[string]any{{
			"adoption_id":             "adoption-1",
			"package_id":              "package-1",
			"status":                  "proposed",
			"target_computer_id":      "computer-1",
			"target_candidate_id":     "candidate-1",
			"candidate_source_ref":    "refs/computers/computer-1/candidates/candidate-1",
			"runtime_artifact_digest": "sha256:runtime-digest-1",
			"ui_artifact_digest":      "sha256:ui-digest-1",
		}},
	}

	update := delegateWorkerCheckpointUpdate(rec, output, "texture:doc-1", "doc-1", "terminal_result", time.Unix(0, 0).UTC())
	joinedFindings := strings.Join(update.Findings, "\n")
	if !strings.Contains(joinedFindings, `worker state "completed"`) {
		t.Fatalf("checkpoint findings did not preserve typed worker state: %#v", update.Findings)
	}
	if !strings.Contains(joinedFindings, "returned 1 AppChangePackage") {
		t.Fatalf("checkpoint findings did not preserve export count: %#v", update.Findings)
	}
	if strings.Contains(joinedFindings, "no AppChangePackages") {
		t.Fatalf("checkpoint still reported missing exports: %#v", update.Findings)
	}
	for _, want := range []string{
		"worker_child_loop:implementation-run-1",
		"worker_child_loop:verifier-run-1",
		"worker_child_agent:agent-implementation-1",
		"worker_channel:channel-implementation-1",
		"app_change_package:package-1",
		"app_adoption:adoption-1",
	} {
		if !containsString(update.EvidenceIDs, want) {
			t.Fatalf("checkpoint evidence ids missing %q: %#v", want, update.EvidenceIDs)
		}
	}
	joinedRefs := strings.Join(update.Refs, "\n")
	for _, want := range []string{
		"worker_state:completed",
		"app_change_package:package-1",
		"worker_head:head-1",
		"package_manifest_sha256:manifest-sha-1",
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
		"Verifier observed AppChangePackage evidence",
	} {
		if !strings.Contains(joinedNotes, want) {
			t.Fatalf("checkpoint notes missing %q: %#v", want, update.Notes)
		}
	}
}
