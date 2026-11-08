package agentcore

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestATaskCastsCapturesResolveThroughItsDelegatedRecords: a task's story
// Cast relates no record, so every source was found by scanning the newest
// admitter records; on the paper (2026-11-08) that scan spanned only hours
// and an older story's capture "named no source", and its publish asked.
// The acting task's delegated arrival records are read by id, so the bound
// no longer decides. A capture outside them still refuses.
func TestATaskCastsCapturesResolveThroughItsDelegatedRecords(t *testing.T) {
	f := newDelegationFixture(t, "owner-pa-task-sources")
	ctx := context.Background()
	for i, source := range []string{"rss:ts1", "rss:ts2", "rss:ts3"} {
		seedArrival(t, f.s, f.owner, f.computer, source, []types.CaptureArrival{
			{CaptureID: "cap-ts" + string(rune('1'+i)), Title: source, URL: "https://ts.example/" + source}})
	}
	f.parentTurn()
	queued := f.tasks(types.ManagementTaskQueued)
	if len(queued) == 0 {
		t.Fatal("no task delegated")
	}
	task := queued[0]
	saved := publishAutoCaptureScan
	publishAutoCaptureScan = 1 // the scan sees only the newest admitter record
	defer func() { publishAutoCaptureScan = saved }()

	cast := types.CommitmentRecord{
		Cast:       &types.CommitmentCast{Publish: types.CastPublishAuto, Captures: task.LeasedCaptures},
		Provenance: types.CommitmentProvenance{ActingTask: task.TaskID},
	}
	sources, err := f.rt.castCaptureSources(ctx, f.owner, f.computer, cast)
	if err != nil || len(sources) == 0 {
		t.Fatalf("a task's captures did not resolve through its delegated records: %v, %v", sources, err)
	}
	cast.Cast.Captures = append(append([]string(nil), task.LeasedCaptures...), "cap-not-delegated")
	if _, err := f.rt.castCaptureSources(ctx, f.owner, f.computer, cast); err == nil {
		t.Fatal("a capture outside the task's delegated records resolved")
	}
}
