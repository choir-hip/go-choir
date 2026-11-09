package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Tier-2 review round 1 of DS2. Failure modes, decided first:
//
//   - the image cap undercounts when two jobs share a before-shot: the cap
//     counts image BLOCKS (any source kind) in the run's context plus the
//     whole new message;
//   - a second queueing in one cell silently drops the first;
//   - a finished job is overwritten as restarted by a racing read;
//   - a superseded job id reads as a failure of the shots;
//   - a package bundle with screens is verified on screens management never
//     looked at, or without a finding per screen (FM15).

func shaN(prefix string, i int) string { return fmt.Sprintf("%s%063x", prefix, i)[:64] }

func screensOf(prefix string, n int, from int) []shotJobScreen {
	var out []shotJobScreen
	for i := 0; i < n; i++ {
		out = append(out, shotJobScreen{Name: fmt.Sprintf("%02d-x.png", i), SHA256: shaN(prefix, from+i), Width: 1440, Height: 900})
	}
	return out
}

func appendMemoryMessage(t *testing.T, rt *Runtime, exec toolregistry.ExecutionContext, msg json.RawMessage) {
	t.Helper()
	if _, err := rt.store.AppendRunMemoryEntry(context.Background(), types.RunMemoryEntry{RunID: exec.RunID, OwnerID: exec.OwnerID, Kind: types.RunMemoryEntryMessage, Role: "user", Message: msg}); err != nil {
		t.Fatal(err)
	}
}

func TestQueueImageSetCountsEveryImageBlockOfBothTurns(t *testing.T) {
	rt, _ := testRuntime(t)
	exec := toolregistry.ExecutionContext{RunID: "run-cap", OwnerID: "owner-1"}
	// Job A's turn: 13 shots and one image of another source kind = 14 blocks.
	a := shotImagesMessage(shotJobRecord{JobID: "A", OperationID: "op", shotJobResult: shotJobResult{Screens: screensOf("a", 13, 0)}})
	var m map[string]any
	_ = json.Unmarshal(a, &m)
	m["content"] = append(m["content"].([]any), map[string]any{"type": "image", "source": map[string]string{"kind": "url", "url": "https://example.com/x.png"}})
	a, _ = json.Marshal(m)
	appendMemoryMessage(t, rt, exec, a)
	// Job B: 12 screens, 6 of them A's shas. Sent as 12 blocks: 14 + 12 > 20.
	b := append(screensOf("a", 6, 0), screensOf("b", 6, 0)...)
	err := rt.queueImageSet(context.Background(), exec, "job B", b, shotImagesMessage(shotJobRecord{JobID: "B", OperationID: "op", shotJobResult: shotJobResult{Screens: b}}))
	if err == nil || !strings.Contains(err.Error(), "request cap") {
		t.Fatalf("queue = %v, want the request cap named", err)
	}
}

func TestQueueImageSetAppendsAndCountsWhatIsQueued(t *testing.T) {
	rt, _ := testRuntime(t)
	exec := toolregistry.ExecutionContext{RunID: "run-append", OwnerID: "owner-1"}
	set := func(id string, s []shotJobScreen) error {
		return rt.queueImageSet(context.Background(), exec, id, s, shotImagesMessage(shotJobRecord{JobID: id, OperationID: "op", shotJobResult: shotJobResult{Screens: s}}))
	}
	if err := set("A", screensOf("a", 3, 0)); err != nil {
		t.Fatal(err)
	}
	if err := set("B", screensOf("b", 2, 0)); err != nil {
		t.Fatal(err)
	}
	turns, _ := rt.shotImageInjector(&types.RunRecord{RunID: exec.RunID, AgentProfile: agentprofile.Management})(false)
	if len(turns) != 2 {
		t.Fatalf("injected %d turns, want both A and B", len(turns))
	}
	// 15 queued and not yet injected: 12 more is over the cap.
	if err := set("C", screensOf("c", 15, 0)); err != nil {
		t.Fatal(err)
	}
	if err := set("D", screensOf("d", 12, 0)); err == nil || !strings.Contains(err.Error(), "request cap") {
		t.Fatalf("D over the queued images = %v, want refused", err)
	}
}

func TestShotLoadNeverOverwritesAFinishedJobAsRestarted(t *testing.T) {
	rt, _ := testRuntime(t)
	rt.cfg.StorePath = filepath.Join(t.TempDir(), "runtime.db")
	for i := 0; i < 300; i++ {
		op := fmt.Sprintf("op-%d", i)
		rec := shotJobRecord{JobID: shotJobID(shotJobKey(op, "h"), 1), Gen: 1, OperationID: op, HostBuildJobID: "h", shotJobResult: shotJobResult{State: shotJobRunning}}
		key := shotJobKey(op, "h")
		if err := rt.writeShotJobFile(rec); err != nil {
			t.Fatal(err)
		}
		rt.shotJobsMu.Lock()
		if rt.shotJobsLive == nil {
			rt.shotJobsLive = map[string]chan struct{}{}
		}
		done := make(chan struct{})
		rt.shotJobsLive[key] = done
		rt.shotJobsMu.Unlock()
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			rt.finishShotJob(key, rec, shotJobResult{State: shotJobDone}, done)
		}()
		for j := 0; j < 20; j++ {
			if got, _, err := rt.loadShotJob(key); err != nil {
				t.Fatal(err)
			} else if got.State == shotJobRestarted {
				t.Fatalf("iteration %d: a job that finished was read as restarted", i)
			}
		}
		wg.Wait()
		if got, _, _ := rt.loadShotJob(key); got.State != shotJobDone {
			t.Fatalf("iteration %d: final state = %s", i, got.State)
		}
	}
}

func TestShotWaitOnASupersededJobIdIsStaleNotFailed(t *testing.T) {
	rt, _ := testRuntime(t)
	rt.cfg.StorePath = filepath.Join(t.TempDir(), "runtime.db")
	id := shotJobID(shotJobKey("op-x", "h"), 1)
	payload, _ := json.Marshal(yaegikernel.ShotWaitPayload{JobID: id, MaxSeconds: 0})
	raw, err := rt.shotWaitEgress(context.Background(), "management", toolregistry.ExecutionContext{RunID: "r", OwnerID: "o"}, payload)
	if err != nil {
		t.Fatal(err)
	}
	var out yaegikernel.ShotWaitResult
	_ = json.Unmarshal(raw, &out)
	if out.State != shotJobStale {
		t.Fatalf("state = %q, want stale", out.State)
	}
	// A job of an older generation is stale too, never a verdict's failure.
	rec := shotJobRecord{JobID: shotJobID(shotJobKey("op-y", "h"), 2), Gen: 2, OperationID: "op-y", HostBuildJobID: "h", shotJobResult: shotJobResult{State: shotJobDone}}
	if err := rt.writeShotJobFile(rec); err != nil {
		t.Fatal(err)
	}
	payload, _ = json.Marshal(yaegikernel.ShotWaitPayload{JobID: shotJobID(shotJobKey("op-y", "h"), 1)})
	raw, _ = rt.shotWaitEgress(context.Background(), "management", toolregistry.ExecutionContext{RunID: "r", OwnerID: "o"}, payload)
	_ = json.Unmarshal(raw, &out)
	if out.State != shotJobStale {
		t.Fatalf("older generation state = %q, want stale", out.State)
	}
}

// FM15: the package bundle's screens are judged by Verify with a finding per
// screen, every screen in run memory; a platform bundle with screens and no
// plan is refused.
func TestVerifyOfAPackageBundleWithScreensNeedsFindingsAndSeenScreens(t *testing.T) {
	ctx := context.Background()
	rt, s, assignment := assignmentEndFixture(t, "owner-fm15")
	rt.selfdevUpdaterRoot = t.TempDir()
	operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
	if _, err := rt.commitAssignedEngineeringReport(ctx, c1eGet(t, s, assignment), terminalAssignmentReport(assignment, types.EngineeringResultCompleted)); err != nil {
		t.Fatal(err)
	}
	screens := []capsule.ScreenFile{
		{Name: "01-home.png", SHA256: shaN("a", 1), Bytes: 1, Width: 1440, Height: 900},
		{Name: "02-list.png", SHA256: shaN("b", 2), Bytes: 1, Width: 1440, Height: 900},
	}
	bundle := transaction.CapsuleEffectBundle{BundleVersion: 1, ComputerID: assignment.Binding.ComputerID, TrajectoryRef: assignment.Binding.TrajectoryID, VerifierReceipts: []string{}, Screens: screens}
	var err error
	if bundle.ContentDigest, err = bundle.ComputeContentDigest(); err != nil {
		t.Fatal(err)
	}
	raw, err := computerevent.CanonicalJSON(bundle)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(rt.selfdevUpdaterRoot, "incoming", bundle.ContentDigest)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bundle.draft.json"), raw, 0o400); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.store.DB().Exec(`UPDATE self_development_operations SET bundle_digest = ?, kind = ? WHERE operation_id = ?`, bundle.ContentDigest, selfdev.KindPackage, operationID); err != nil {
		t.Fatal(err)
	}
	yes := map[string]string{"q1": "yes", "q2": "yes", "q3": "yes", "q4": "n/a", "q6": "yes"}
	both := []yaegikernel.ShotFinding{
		{Pair: "01", BeforeSHA256: screens[0].SHA256, AfterSHA256: screens[0].SHA256, Answers: yes},
		{Pair: "02", BeforeSHA256: screens[1].SHA256, AfterSHA256: screens[1].SHA256, Answers: yes},
	}
	reduce := verifyReduction(rt, assignment)
	intent := func(f []yaegikernel.ShotFinding) yaegikernel.StagedIntent {
		return yaegikernel.StagedIntent{LocalID: "v1", TargetRef: assignment.AssignmentID, Decision: "pass", BundleDigest: bundle.ContentDigest, VerifierRefs: []string{"ref"}, ShotFindings: f}
	}
	refused := func(f []yaegikernel.ShotFinding, want string) {
		t.Helper()
		_, err := reduce.commitVerifyIntent(ctx, intent(f))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("verify = %v, want %q", err, want)
		}
	}
	// (b) a missing finding, and (c) screens not in memory.
	refused(both[:1], "do not cover every screen")
	refused(both, "run's memory does not hold the screens")
	// Screens(operationID) queues them; the injected turn is persisted.
	exec := toolregistry.ExecutionContext{RunID: reduce.scope.RunID, OwnerID: reduce.scope.OwnerID, ComputerID: assignment.Binding.ComputerID}
	rawOut, err := rt.screensEgress(ctx, "management", exec, json.RawMessage(`{"operation_id":"`+operationID+`"}`))
	if err != nil {
		t.Fatalf("Screens: %v", err)
	}
	var queued yaegikernel.ScreensResult
	_ = json.Unmarshal(rawOut, &queued)
	if !queued.ImagesQueued || len(queued.Screens) != 2 {
		t.Fatalf("Screens = %+v", queued)
	}
	turns, _ := rt.shotImageInjector(&types.RunRecord{RunID: exec.RunID, AgentProfile: agentprofile.Management})(false)
	if len(turns) != 1 {
		t.Fatalf("injected %d turns", len(turns))
	}
	if _, err := rt.screensEgress(ctx, "texture", exec, json.RawMessage(`{"operation_id":"`+operationID+`"}`)); err == nil {
		t.Fatal("a desk other than management queued screens")
	}
	appendMemoryMessage(t, rt, exec, turns[0])
	// (a) with the screens in memory and a finding per screen, the screens
	// refusal is gone (the rest of Verify may still stop on the fixture).
	if _, err := reduce.commitVerifyIntent(ctx, intent(both)); err != nil && (strings.Contains(err.Error(), "carries screens") || strings.Contains(err.Error(), "run's memory") || strings.Contains(err.Error(), "legacy screens")) {
		t.Fatalf("verify with findings and seen screens = %v", err)
	}
	// (d) a platform bundle with screens and no plan can never pass: Verify
	// records a fail verdict (a terminal fate), it does not leave the
	// candidate frozen behind a refusal.
	if _, err := rt.store.DB().Exec(`UPDATE self_development_operations SET kind = '' WHERE operation_id = ?`, operationID); err != nil {
		t.Fatal(err)
	}
	if _, err := reduce.commitVerifyIntent(ctx, intent(both)); err != nil && strings.Contains(err.Error(), "legacy screens bundle") {
		t.Fatalf("a legacy screens bundle stays a refusal that holds the candidate frozen: %v", err)
	}
	// The fixture has no verdict authority, so the verdict stops there; what
	// matters is that it reached the verdict instead of the stamped refusal.
	after, getErr := rt.selfdevOperations.Get(ctx, assignment.Binding.ComputerID, operationID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if strings.Contains(after.LastIntentError, "legacy screens bundle") || !strings.Contains(after.LastIntentError, "verification authority unavailable") {
		t.Fatalf("a legacy screens bundle did not reach the fail verdict: %q", after.LastIntentError)
	}
}

var _ = time.Second
