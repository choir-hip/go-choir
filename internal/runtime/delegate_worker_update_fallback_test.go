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
		"state":                        "completed",
		"loop_id":                      "worker-run-1",
		"worker_vm_id":                 "vm-1",
		"worker_id":                    "worker-1",
		"event_count":                  12,
		"worker_channel_message_count": 3,
		"export_patchsets": []map[string]any{{
			"manifest_path": "/mnt/persistent/files/patchsets/manifest.json",
			"patchset_path": "/mnt/persistent/files/patchsets/changes.patch",
			"base_sha":      "base-1",
			"worker_head":   "head-1",
		}},
	}

	update := delegateWorkerCheckpointUpdate(rec, output, "vtext:doc-1", "doc-1", "terminal_result", time.Unix(0, 0).UTC())
	joinedFindings := strings.Join(update.Findings, "\n")
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
}
