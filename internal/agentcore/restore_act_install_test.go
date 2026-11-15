package agentcore

// The flip as a durable protocol (docs/problems/restore-cannot-finish-inside-
// its-ten-minute-request-2026-11-14.md, "Review round 1 repair", failure list
// 20 to 28). The act's record carries install_phase, a durable fact; boot
// decides from the fact, never from which files exist. These tests kill a real
// process at each point of the real engine's flip and read what the boot makes
// of what is left on disk.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// crashChild is what a killed child left, as the child described it before it
// died.
type crashChild struct {
	base                        string
	dataDir, storePath, updater string
	Head, ActID, CheckpointHead string
	Sequence                    uint64
	output                      string
}

func (c crashChild) record(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(c.dataDir, "restore-acts", c.ActID+".json"))
	if err != nil {
		t.Fatalf("the act's record: %v", err)
	}
	var rec map[string]any
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatalf("the act's record is not JSON: %v", err)
	}
	return rec
}

// TestRestoreActChildProcess is the child of the crash tests: it builds a
// computer, accepts a restore and kills itself at the named point (or holds the
// replay and waits to be killed).
func TestRestoreActChildProcess(t *testing.T) {
	base := os.Getenv("GOCHOIR_RESTORE_ACT_CHILD")
	if base == "" {
		t.Skip("child of the restore crash tests")
	}
	point := os.Getenv("GOCHOIR_RESTORE_ACT_CRASH")
	f := newRestoreActFixtureIn(t, base, "computer-act-kill", 6)
	readyPath := filepath.Join(base, "child-ready.json")
	var reached <-chan struct{}
	killAt := point
	switch point {
	case "replay":
		reached, _ = holdReplayAfter(t, 2)
	case "after_undo":
		// The flip completes but its fact cannot be written: the engine undoes it
		// itself (the one routine), and dies right after the undo.
		restoreInstallWriteFault = func(phase string) error {
			if phase == "flip_completed" {
				return errors.New("write restore-acts: no space left on device")
			}
			return nil
		}
	case "live_wal_at_flip_started":
		// A live store's -wal that its close left behind, there when the flip
		// starts; the staged store, closed cleanly, never had one.
		killAt = "after_flip_started"
		restoreInstallCrashHook = func(p string) {
			if p == "before_flip_started" {
				_ = os.WriteFile(filepath.Join(base, "data", "state.db-wal"), []byte("live wal frames"), 0o600)
			}
			if p == killAt {
				for {
					if _, err := os.Stat(readyPath); err == nil {
						break
					}
					time.Sleep(2 * time.Millisecond)
				}
				_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
				select {}
			}
		}
	}
	if point != "replay" && point != "live_wal_at_flip_started" {
		restoreInstallCrashHook = func(p string) {
			if p != killAt {
				return
			}
			// The parent must be told what the computer was before the restore:
			// the hook waits for that file, then dies.
			for {
				if _, err := os.Stat(readyPath); err == nil {
					break
				}
				time.Sleep(2 * time.Millisecond)
			}
			_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
			select {}
		}
	}
	code, accepted := f.post("")
	if code != http.StatusAccepted {
		fmt.Fprintf(os.Stderr, "child: restore answered %d %v\n", code, accepted)
		os.Exit(3)
	}
	head, err := f.rt.store.Head(context.Background(), f.computerID)
	if err != nil || head == nil {
		os.Exit(4)
	}
	raw, _ := json.Marshal(map[string]any{"head": head.CanonicalEventHead, "sequence": head.Sequence, "act_id": accepted["act_id"], "checkpoint_head": f.checkpoint.Request.AcceptedEventHead})
	if err := os.WriteFile(readyPath+".tmp", raw, 0o600); err != nil || os.Rename(readyPath+".tmp", readyPath) != nil {
		os.Exit(5)
	}
	if reached != nil {
		<-reached
		select {} // killed by the parent
	}
	// Killed by the hook at its point. An act that ends without the hook having
	// fired is a failure of the test, not a hang: say so and exit.
	for i := 0; i < 3000; i++ {
		if _, view := f.status(f.actID(accepted)); restoreActTerminal(fmt.Sprint(view["state"])) {
			fmt.Fprintf(os.Stderr, "child: the act ended %v without reaching %s\n", view["state"], point)
			os.Exit(6)
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {}
}

// runCrashChild starts the child, waits for it to die and returns what it left.
func runCrashChild(t *testing.T, point string) crashChild {
	t.Helper()
	base := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestRestoreActChildProcess$", "-test.timeout=3m")
	cmd.Env = append(os.Environ(), "GOCHOIR_RESTORE_ACT_CHILD="+base, "GOCHOIR_RESTORE_ACT_CRASH="+point)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	deadline := time.After(120 * time.Second)
	if point == "replay" {
		// Held mid-replay: wait for the file, then kill our own child by pid.
		for {
			if _, err := os.Stat(filepath.Join(base, "child-ready.json")); err == nil {
				break
			}
			select {
			case <-exited:
				t.Fatalf("the child ended before it held the replay:\n%s", output.String())
			case <-deadline:
				t.Fatalf("the child never held the replay:\n%s", output.String())
			case <-time.After(20 * time.Millisecond):
			}
		}
		time.Sleep(300 * time.Millisecond) // into the replay, past the file
		if err := cmd.Process.Kill(); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-exited:
	case <-deadline:
		t.Fatalf("the child did not reach %s:\n%s", point, output.String())
	}
	raw, err := os.ReadFile(filepath.Join(base, "child-ready.json"))
	if err != nil {
		t.Fatalf("the child died before it described itself (%v):\n%s", err, output.String())
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatal(err)
	}
	c := crashChild{base: base, dataDir: filepath.Join(base, "data"), storePath: filepath.Join(base, "data", "runtime.db"), updater: filepath.Join(base, "updater"), output: output.String()}
	c.Head, _ = generic["head"].(string)
	c.ActID, _ = generic["act_id"].(string)
	c.CheckpointHead, _ = generic["checkpoint_head"].(string)
	c.Sequence = uint64(generic["sequence"].(float64))
	return c
}

func servedOf(t *testing.T, updater string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(updater, "current", "frontend", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func headOf(t *testing.T, storePath, computerID string) *computerevent.Head {
	t.Helper()
	live, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatalf("the store does not open: %v", err)
	}
	defer live.Close()
	head, err := live.Head(context.Background(), computerID)
	if err != nil || head == nil {
		t.Fatalf("head: %v", err)
	}
	return head
}

func leftovers(dataDir, pattern string) []string {
	found, _ := filepath.Glob(filepath.Join(dataDir, pattern))
	return found
}

// wantPreRestore asserts the computer is on exactly its pre-restore state.
func (c crashChild) wantPreRestore(t *testing.T) {
	t.Helper()
	head := headOf(t, c.storePath, "computer-act-kill")
	if head.CanonicalEventHead != c.Head || head.Sequence != c.Sequence || head.CanonicalEventHead == c.CheckpointHead {
		t.Fatalf("head = %+v, want the one the act was accepted at (%d, %s): the restore was applied or the store changed", head, c.Sequence, c.Head)
	}
	if got := servedOf(t, c.updater); got != "<html>live</html>" {
		t.Fatalf("served SPA = %q, want the live release back", got)
	}
	if staged := leftovers(c.dataDir, "rematerialize-staging-*"); len(staged) != 0 {
		t.Fatalf("staged store left: %v", staged)
	}
	if quarantined := leftovers(c.dataDir, "rematerialize-quarantine-*"); len(quarantined) != 0 {
		t.Fatalf("quarantine left on a restore that did not happen: %v", quarantined)
	}
	rec := c.record(t)
	if rec["state"] != "interrupted" || rec["live_state"] != "pre_restore" || !strings.Contains(fmt.Sprint(rec["reason"]), "interrupted by a restart") {
		t.Fatalf("the act = %v, want interrupted, live_state pre_restore", rec)
	}
}

// wantRestored asserts the computer is on exactly the restored state: the
// restored store and the restored release, together.
func (c crashChild) wantRestored(t *testing.T) {
	t.Helper()
	head := headOf(t, c.storePath, "computer-act-kill")
	if head.CanonicalEventHead != c.CheckpointHead {
		t.Fatalf("head = %+v, want the checkpoint's %s: the flip was undone", head, c.CheckpointHead)
	}
	if got := servedOf(t, c.updater); got != "<html>checkpoint</html>" {
		t.Fatalf("served SPA = %q, want the restored release with the restored store", got)
	}
	if quarantined := leftovers(c.dataDir, "rematerialize-quarantine-*"); len(quarantined) != 1 {
		t.Fatalf("quarantine = %v, want the old store kept in one", quarantined)
	}
	rec := c.record(t)
	if rec["state"] != "restored" || rec["live_state"] != "restored" || !strings.Contains(fmt.Sprint(rec["reason"]), "restored") {
		t.Fatalf("the act = %v, want restored with live_state restored", rec)
	}
}

// F9, F21: a real SIGKILL at each point of the real engine's install, and the
// boot that follows.
func TestRestoreCrashPoints(t *testing.T) {
	pre := []string{"after_prepared", "after_release_restaged", "after_flip_started", "mid_flip", "before_flip_completed", "after_undo"}
	post := []string{"after_flip_completed", "after_reopen"}
	for _, point := range append(append([]string{}, pre...), post...) {
		t.Run(point, func(t *testing.T) {
			c := runCrashChild(t, point)
			if point == "mid_flip" || point == "before_flip_completed" {
				if len(leftovers(c.dataDir, "rematerialize-quarantine-*/*")) == 0 {
					t.Fatalf("the crash left no renames made: the test did not stop the flip half way\n%s", c.output)
				}
			}
			report := RepairRestoreAtBoot(c.storePath, c.updater)
			if len(report.Closed) != 1 || report.Closed[0] != c.ActID || len(report.Unresolved) != 0 {
				t.Fatalf("boot report = %+v, want the one act closed and nothing unresolved", report)
			}
			isPre := false
			for _, p := range pre {
				isPre = isPre || p == point
			}
			if isPre {
				c.wantPreRestore(t)
			} else {
				c.wantRestored(t)
			}
			if again := RepairRestoreAtBoot(c.storePath, c.updater); len(again.Closed) != 0 || len(again.Unresolved) != 0 {
				t.Fatalf("a second boot closed %+v: a closed act must stay closed", again)
			}
		})
	}
}

// F9: killed mid-replay, nothing of the install had begun.
func TestRestoreKilledMidReplayLeavesTheLiveStateUntouched(t *testing.T) {
	c := runCrashChild(t, "replay")
	if staged := leftovers(c.dataDir, "rematerialize-staging-*"); len(staged) == 0 {
		t.Fatal("the killed restore left no staged store: the test did not stop it mid-replay")
	}
	report := RepairRestoreAtBoot(c.storePath, c.updater)
	if len(report.Closed) != 1 || report.Closed[0] != c.ActID {
		t.Fatalf("boot report = %+v", report)
	}
	c.wantPreRestore(t)
}

// F22: the repair, killed between its steps, then run again, then again.
func TestRepairIsIdempotentAndSafeWhenInterrupted(t *testing.T) {
	t.Cleanup(func() { restoreRepairStepHook = nil })
	t.Run("an undo killed after its first rename", func(t *testing.T) {
		c := runCrashChild(t, "mid_flip")
		undos := 0
		restoreRepairStepHook = func(step string) bool {
			if step == "undo" {
				undos++
				return undos == 2
			}
			return false
		}
		first := RepairRestoreAtBoot(c.storePath, c.updater)
		restoreRepairStepHook = nil
		if undos < 2 || len(first.Closed) != 0 {
			t.Fatalf("the interrupted repair undid %d renames and closed %v: it did not stop between steps", undos, first.Closed)
		}
		if rec := c.record(t); rec["state"] == "interrupted" {
			t.Fatal("a repair killed half way closed its act: the next boot would not finish it")
		}
		second := RepairRestoreAtBoot(c.storePath, c.updater)
		if len(second.Closed) != 1 {
			t.Fatalf("the second run closed %v", second.Closed)
		}
		c.wantPreRestore(t)
		if third := RepairRestoreAtBoot(c.storePath, c.updater); len(third.Closed) != 0 {
			t.Fatalf("the third run closed %v", third.Closed)
		}
	})
	t.Run("a roll forward killed before it closes its act", func(t *testing.T) {
		c := runCrashChild(t, "after_flip_completed")
		restoreRepairStepHook = func(step string) bool { return step == "close" }
		first := RepairRestoreAtBoot(c.storePath, c.updater)
		restoreRepairStepHook = nil
		if len(first.Closed) != 0 {
			t.Fatalf("the interrupted repair closed %v", first.Closed)
		}
		if second := RepairRestoreAtBoot(c.storePath, c.updater); len(second.Closed) != 1 {
			t.Fatalf("the second run closed %v", second.Closed)
		}
		c.wantRestored(t)
	})
}

// F23: what the repair cannot read it does not touch, and says so.
func TestRepairRefusesWhatItCannotRead(t *testing.T) {
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("a corrupt record keeps the staged store", func(t *testing.T) {
		dir := t.TempDir()
		storePath := filepath.Join(dir, "runtime.db")
		write(filepath.Join(dir, "restore-acts", "ra-bad.json"), "{not json")
		write(filepath.Join(dir, "rematerialize-staging-x", "state.db"), "staged")
		report := RepairRestoreAtBoot(storePath, "")
		if len(report.Unresolved) != 1 || report.Unresolved[0] != "ra-bad" {
			t.Fatalf("report = %+v, want ra-bad unresolved", report)
		}
		if _, err := os.Stat(filepath.Join(dir, "rematerialize-staging-x", "state.db")); err != nil {
			t.Fatalf("the staged store was deleted under a record the boot could not read: %v", err)
		}
	})
	t.Run("a corrupt record does not stop another record being closed, but keeps the staging", func(t *testing.T) {
		dir := t.TempDir()
		storePath := filepath.Join(dir, "runtime.db")
		write(filepath.Join(dir, "restore-acts", "ra-bad.json"), "")
		good, _ := json.Marshal(map[string]any{"schema_version": 1, "act_id": "ra-good", "computer_id": "c", "state": "replaying", "install_phase": ""})
		write(filepath.Join(dir, "restore-acts", "ra-good.json"), string(good))
		write(filepath.Join(dir, "rematerialize-staging-x", "state.db"), "staged")
		report := RepairRestoreAtBoot(storePath, "")
		if len(report.Closed) != 1 || len(report.Unresolved) != 1 {
			t.Fatalf("report = %+v", report)
		}
		if _, err := os.Stat(filepath.Join(dir, "rematerialize-staging-x", "state.db")); err != nil {
			t.Fatalf("staging deleted although a record is unresolved: %v", err)
		}
	})
	t.Run("a path outside the data volume is not renamed", func(t *testing.T) {
		dir, outside := t.TempDir(), t.TempDir()
		storePath := filepath.Join(dir, "runtime.db")
		victim := filepath.Join(outside, "victim.db")
		write(victim, "mine")
		moved := filepath.Join(dir, "rematerialize-quarantine-x", "victim.db")
		write(moved, "x")
		if err := os.Remove(victim); err != nil { // the "to" exists, the "from" does not: an undo would rename
			t.Fatal(err)
		}
		rec, _ := json.Marshal(map[string]any{
			"schema_version": 1, "act_id": "ra-evil", "computer_id": "c", "state": "installing", "install_phase": "flip_started",
			"flip": []map[string]string{{"from": victim, "to": moved}},
		})
		write(filepath.Join(dir, "restore-acts", "ra-evil.json"), string(rec))
		report := RepairRestoreAtBoot(storePath, "")
		if len(report.Unresolved) != 1 || len(report.Closed) != 0 {
			t.Fatalf("report = %+v, want the record unresolved", report)
		}
		if _, err := os.Stat(victim); err == nil {
			t.Fatal("the repair renamed a path outside the data volume")
		}
	})
	t.Run("an empty store path is refused", func(t *testing.T) {
		if report := RepairRestoreAtBoot("", ""); len(report.Closed) != 0 {
			t.Fatalf("report = %+v", report)
		}
	})
}

// F23: an unresolved act refuses new restore acts and is named in /health.
func TestUnresolvedRestoreActRefusesNewActsAndShowsInHealth(t *testing.T) {
	f := newRestoreActFixture(t, "computer-act-unresolved", 2)
	WithRestoreUnresolved([]string{"ra-bad"})(f.rt)
	code, view := f.post("")
	refusal, _ := view["refusal"].(map[string]any)
	if code != http.StatusConflict || refusal == nil || refusal["name"] != "restore_unresolved" {
		t.Fatalf("a restore with an unresolved act answered %d %v, want 409 naming restore_unresolved", code, view)
	}
	if n := f.intents(); n != 0 {
		t.Fatalf("the refusal appended %d intents", n)
	}
	rec, _ := serveHealth(t, &APIHandler{rt: f.rt})
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	list, _ := body["restore_unresolved"].([]any)
	if len(list) != 1 || list[0] != "ra-bad" {
		t.Fatalf("/health restore_unresolved = %v", body["restore_unresolved"])
	}
}

// F20: the install record is written durably before anything live is touched,
// and a write that fails stops the install where it is.
func TestRestoreInstallRecordWriteFailureTouchesNothing(t *testing.T) {
	t.Cleanup(func() { restoreInstallWriteFault = nil })
	for _, phase := range []string{"prepared", "release_restaged", "flip_started", "flip_completed"} {
		t.Run(phase, func(t *testing.T) {
			f := newRestoreActFixture(t, "computer-act-enospc-"+strings.ReplaceAll(phase, "_", "-"), 3)
			restoreInstallWriteFault = func(p string) error {
				if p == phase {
					return errors.New("write restore-acts: no space left on device")
				}
				return nil
			}
			code, accepted := f.post("")
			if code != http.StatusAccepted {
				t.Fatalf("answered %d %v", code, accepted)
			}
			end := f.waitTerminal(f.actID(accepted))
			if end["state"] != "failed" || end["live_state"] != "pre_restore" {
				t.Fatalf("act = %v, want failed on the pre-restore state", end)
			}
			if !strings.Contains(fmt.Sprint(end["refusal"]), "no space left") {
				t.Fatalf("the refusal does not say why: %v", end["refusal"])
			}
			if got := f.served(); got != "<html>live</html>" {
				t.Fatalf("served SPA = %q after a failed %s write", got, phase)
			}
			if quarantined := leftovers(filepath.Dir(f.storePath), "rematerialize-quarantine-*"); len(quarantined) != 0 {
				t.Fatalf("quarantine left: %v", quarantined)
			}
			if staged := leftovers(filepath.Dir(f.storePath), "rematerialize-staging-*"); len(staged) != 0 {
				t.Fatalf("staging left: %v", staged)
			}
			// The live store answers, on the pre-restore realization.
			head, err := f.rt.store.Head(context.Background(), f.computerID)
			if err != nil || head == nil || head.CanonicalEventHead == f.checkpoint.Request.AcceptedEventHead {
				t.Fatalf("live head = %+v %v, want the pre-restore head (the store open and unrestored)", head, err)
			}
			restoreInstallWriteFault = nil
			if again := RepairRestoreAtBoot(f.storePath, f.updaterRoot); len(again.Closed) != 0 {
				t.Fatalf("boot closed %v after a failed act", again.Closed)
			}
		})
	}
}

// F24: a reopen that fails after the flip is restored, not failed.
func TestRestoreReopenFailureIsRestoredNotFailed(t *testing.T) {
	t.Cleanup(func() { restoreReopenFault = nil })
	f := newRestoreActFixture(t, "computer-act-reopen", 3)
	restoreReopenFault = func() error { return errors.New("injected: the flipped store would not open") }
	_, accepted := f.post("")
	end := f.waitTerminal(f.actID(accepted))
	if end["state"] != "restored" || end["live_state"] != "restored" || !strings.Contains(fmt.Sprint(end["reason"]), "restart required") {
		t.Fatalf("act = %v, want restored, live_state restored, restart required", end)
	}
	if got := f.served(); got != "<html>checkpoint</html>" {
		t.Fatalf("served SPA = %q: the prior release was put back over a flipped store", got)
	}
	if !f.rt.restoreRestaged.Load() {
		t.Fatal("restoreRestaged is not set: a migration could move `current` under the restored store")
	}
	result, _ := end["result"].(map[string]any)
	if result == nil || (result["restart_scheduled"] != true && fmt.Sprint(result["restart_deferred"]) == "<nil>") {
		t.Fatalf("result = %v: the restart a restored act needs was not requested", result)
	}
	if len(leftovers(filepath.Dir(f.storePath), "rematerialize-quarantine-*")) != 1 {
		t.Fatal("the old store is not in quarantine")
	}
	// The boot after it leaves it alone.
	if again := RepairRestoreAtBoot(f.storePath, f.updaterRoot); len(again.Closed) != 0 {
		t.Fatalf("boot closed %v", again.Closed)
	}
	if got := f.served(); got != "<html>checkpoint</html>" {
		t.Fatalf("boot moved the release to %q", got)
	}
}

// F25: acceptance refuses live apply or capsule work, and the work that starts
// during the replay is found under the lock before anything live is touched.
func TestRestoreActRefusedWhileApplyOrCapsuleIsLive(t *testing.T) {
	t.Run("an apply mid-transaction", func(t *testing.T) {
		f := newRestoreActFixture(t, "computer-act-apply", 2)
		operations, err := selfdev.NewStore(f.rt.store, f.rt.store)
		if err != nil {
			t.Fatal(err)
		}
		f.rt.selfdevOperations = operations
		now := time.Now().UTC()
		seedDecisionHoldOperation(t, f.rt.store, f.computerID, "selfdev-apply", selfdev.StateExecuting, now, now)
		code, view := f.post("")
		refusal, _ := view["refusal"].(map[string]any)
		if code != http.StatusConflict || refusal == nil || refusal["name"] != "blocked_by_live_work" {
			t.Fatalf("answered %d %v, want 409 blocked_by_live_work", code, view)
		}
		if n := f.intents(); n != 0 {
			t.Fatalf("%d intents appended", n)
		}
	})
	t.Run("a live engineering capsule", func(t *testing.T) {
		f := newRestoreActFixture(t, "computer-act-capsule", 2)
		seedLiveCapsule(t, f.handler, f.rt.store)
		code, view := f.post("")
		refusal, _ := view["refusal"].(map[string]any)
		if code != http.StatusConflict || refusal == nil || refusal["name"] != "blocked_by_live_work" {
			t.Fatalf("answered %d %v, want 409 blocked_by_live_work", code, view)
		}
	})
	t.Run("work that starts during the replay", func(t *testing.T) {
		f := newRestoreActFixture(t, "computer-act-late", 4)
		reached, release := holdReplayAfter(t, 1)
		_, accepted := f.post("")
		waitChanClosed(t, reached, "the replay to reach the hold")
		operations, err := selfdev.NewStore(f.rt.store, f.rt.store)
		if err != nil {
			t.Fatal(err)
		}
		f.rt.selfdevOperations = operations
		now := time.Now().UTC()
		seedDecisionHoldOperation(t, f.rt.store, f.computerID, "selfdev-late", selfdev.StateExecuting, now, now)
		release()
		end := f.waitTerminal(f.actID(accepted))
		refusal, _ := end["refusal"].(map[string]any)
		if end["state"] != "failed" || refusal == nil || refusal["name"] != "blocked_by_live_work" || end["live_state"] != "pre_restore" {
			t.Fatalf("act = %v, want failed blocked_by_live_work on the pre-restore state", end)
		}
		if got := f.served(); got != "<html>live</html>" {
			t.Fatalf("served SPA = %q: the release moved under live work", got)
		}
		if quarantined := leftovers(filepath.Dir(f.storePath), "rematerialize-quarantine-*"); len(quarantined) != 0 {
			t.Fatalf("quarantine left: %v", quarantined)
		}
	})
}

// F25: a snapshot capture does not start during a restore (the capture holds a
// read transaction the flip's store close would meet).
func TestSnapshotCaptureIsRefusedDuringARestore(t *testing.T) {
	f := newRestoreActFixture(t, "computer-act-snapshot", 4)
	if reason, err := f.rt.snapshotBusyReason(context.Background()); err != nil || reason != "" {
		t.Fatalf("an idle computer is busy for a snapshot: %q %v", reason, err)
	}
	reached, release := holdReplayAfter(t, 1)
	_, accepted := f.post("")
	waitChanClosed(t, reached, "the replay to reach the hold")
	if reason, err := f.rt.snapshotBusyReason(context.Background()); err != nil || !strings.Contains(reason, "restore") {
		t.Fatalf("snapshot busy reason mid-restore = %q %v, want a restore named", reason, err)
	}
	release()
	f.waitTerminal(f.actID(accepted))
}

// F27: a desk's product_api cannot read the status route (paths, digests).
func TestProductAPIRefusesTheRestoreStatus(t *testing.T) {
	err := validateProductAPIRoute(http.MethodGet, "/api/computers/computer-own/lifecycle/restore-status?act=ra-1")
	if err == nil || !strings.Contains(err.Error(), "restore-status") {
		t.Fatalf("a desk's read of the restore status = %v, want a refusal naming it", err)
	}
}

// A live sidecar present when the flip starts is not moved by an undo that
// infers from which files exist: the record lists only the renames whose source
// existed, and the staged store's never-existing sidecars are not in it.
func TestRestoreCrashPointLiveWalAtFlipStarted(t *testing.T) {
	c := runCrashChild(t, "live_wal_at_flip_started")
	report := RepairRestoreAtBoot(c.storePath, c.updater)
	if len(report.Closed) != 1 || len(report.Unresolved) != 0 {
		t.Fatalf("boot report = %+v", report)
	}
	raw, err := os.ReadFile(filepath.Join(c.dataDir, "state.db-wal"))
	if err != nil || string(raw) != "live wal frames" {
		t.Fatalf("the live -wal = %q %v: the undo moved a sidecar it never renamed (and boot then deleted it with the staging)", raw, err)
	}
	if staged := leftovers(c.dataDir, "rematerialize-staging-*"); len(staged) != 0 {
		t.Fatalf("staging left: %v", staged)
	}
	rec := c.record(t)
	if rec["state"] != "interrupted" || rec["live_state"] != "pre_restore" {
		t.Fatalf("the act = %v", rec)
	}
}

// An engine failure is settled by the one routine: nothing is deleted by the
// engine, the renames are undone from the facts, the store is reopened, and
// the act ends failed on the pre-restore state.
func TestRestoreEnginePanicsAreSettledByTheRoutine(t *testing.T) {
	t.Cleanup(func() { restoreInstallCrashHook = nil })
	for _, point := range []string{"after_flip_started", "before_flip_completed"} {
		name := map[string]string{"after_flip_started": "before_renames", "before_flip_completed": "after_renames"}[point]
		t.Run(name, func(t *testing.T) {
			f := newRestoreActFixture(t, "computer-act-panic-"+strings.ReplaceAll(name, "_", "-"), 3)
			restoreInstallCrashHook = func(p string) {
				if p == point {
					panic("injected panic at " + point)
				}
			}
			_, accepted := f.post("")
			end := f.waitTerminal(f.actID(accepted))
			if end["state"] != "failed" || end["live_state"] != "pre_restore" {
				t.Fatalf("act = %v, want failed on the pre-restore state", end)
			}
			head, err := f.rt.store.Head(context.Background(), f.computerID)
			if err != nil || head == nil || head.CanonicalEventHead == f.checkpoint.Request.AcceptedEventHead {
				t.Fatalf("live head = %+v %v: the store is not the open, unrestored one", head, err)
			}
			if got := f.served(); got != "<html>live</html>" {
				t.Fatalf("served SPA = %q", got)
			}
			dataDir := filepath.Dir(f.storePath)
			if len(leftovers(dataDir, "rematerialize-staging-*"))+len(leftovers(dataDir, "rematerialize-quarantine-*")) != 0 {
				t.Fatal("directories left after the act ended")
			}
			restoreInstallCrashHook = nil
			if again := RepairRestoreAtBoot(f.storePath, f.updaterRoot); len(again.Closed) != 0 || len(again.Unresolved) != 0 {
				t.Fatalf("boot found work after a settled act: %+v", again)
			}
		})
	}
}

// Two write failures in a row: the flip completed, its fact cannot be written
// and neither can the corrective one. The routine does not undo (the files and
// whichever fact the disk holds must agree), the act stays open and
// unresolved, acceptance refuses from the record with no boot between, and the
// boot then finishes it from the fact.
func TestRestoreDoubleWriteFailureIsUnresolvedNotTerminal(t *testing.T) {
	t.Cleanup(func() { restoreInstallWriteFault = nil })
	f := newRestoreActFixture(t, "computer-act-double", 3)
	flipStarts := 0
	restoreInstallWriteFault = func(phase string) error {
		switch phase {
		case "flip_completed":
			return errors.New("write restore-acts: no space left on device")
		case "flip_started":
			flipStarts++
			if flipStarts >= 2 {
				return errors.New("write restore-acts: no space left on device")
			}
		}
		return nil
	}
	_, accepted := f.post("")
	act := f.actID(accepted)
	drainRestoreActs(f.rt)
	_, view := f.status(act)
	if restoreActTerminal(fmt.Sprint(view["state"])) || view["live_state"] != "unresolved" {
		t.Fatalf("act = %v: an act with an unresolved live state was written terminal", view)
	}
	code, refused := f.post("another-key")
	refusal, _ := refused["refusal"].(map[string]any)
	if code != http.StatusConflict || refusal == nil || refusal["name"] != "restore_unresolved" {
		t.Fatalf("a restore over an unresolved act answered %d %v, want 409 restore_unresolved with no boot between", code, refused)
	}
	rec, _ := serveHealth(t, &APIHandler{rt: f.rt})
	var health map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
		t.Fatal(err)
	}
	if list, _ := health["restore_unresolved"].([]any); len(list) != 1 || list[0] != act {
		t.Fatalf("/health restore_unresolved = %v, want [%s]", health["restore_unresolved"], act)
	}
	restoreInstallWriteFault = nil
	report := RepairRestoreAtBoot(f.storePath, f.updaterRoot)
	if len(report.Closed) != 1 || len(report.Unresolved) != 0 {
		t.Fatalf("boot report = %+v, want the act finished from the fact the disk held", report)
	}
	if got := f.served(); got != "<html>live</html>" {
		t.Fatalf("served SPA = %q after the boot's undo", got)
	}
}

// Unresolved is the answer when the prior release cannot be put back: the
// store is undone but `current` is not, and that is never reported clean.
func TestRepairIsUnresolvedWhenThePriorReleaseCannotBePutBack(t *testing.T) {
	c := runCrashChild(t, "after_release_restaged")
	path := filepath.Join(c.dataDir, "restore-acts", c.ActID+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	rec["prior_release_digest"] = "sha256:" + strings.Repeat("f", 64) // not a release this updater root has
	raw, _ = json.Marshal(rec)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	report := RepairRestoreAtBoot(c.storePath, c.updater)
	if len(report.Unresolved) != 1 || len(report.Closed) != 0 {
		t.Fatalf("boot report = %+v, want the act unresolved", report)
	}
	if after := c.record(t); after["state"] == "interrupted" || after["state"] == "restored" {
		t.Fatalf("the record was closed over a release that was not put back: %v", after["state"])
	}
	if staged := leftovers(c.dataDir, "rematerialize-staging-*"); len(staged) == 0 {
		t.Fatal("staging was deleted under an unresolved act")
	}
}

// Acceptance reads the records: an open record refuses a new restore with no
// boot in between.
func TestRestoreActRefusedFromAnOpenRecord(t *testing.T) {
	f := newRestoreActFixture(t, "computer-act-open-record", 2)
	dir := filepath.Join(filepath.Dir(f.storePath), "restore-acts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	rec, _ := json.Marshal(map[string]any{"schema_version": 1, "act_id": "ra-open", "computer_id": f.computerID, "state": "installing", "install_phase": "flip_started", "live_state": "unresolved"})
	if err := os.WriteFile(filepath.Join(dir, "ra-open.json"), rec, 0o600); err != nil {
		t.Fatal(err)
	}
	code, view := f.post("")
	refusal, _ := view["refusal"].(map[string]any)
	if code != http.StatusConflict || refusal == nil || refusal["name"] != "restore_unresolved" {
		t.Fatalf("answered %d %v, want 409 restore_unresolved from the record", code, view)
	}
}

// The first record is durable and its failure refuses the request.
func TestRestoreAcceptRefusedWhenItsFirstRecordCannotBeWritten(t *testing.T) {
	t.Cleanup(func() { restoreInstallWriteFault = nil })
	f := newRestoreActFixture(t, "computer-act-first-record", 2)
	restoreInstallWriteFault = func(phase string) error {
		if phase == "accepted" {
			return errors.New("write restore-acts: no space left on device")
		}
		return nil
	}
	code, view := f.post("")
	if code != http.StatusInternalServerError || !strings.Contains(fmt.Sprint(view["error"]), "no space left") {
		t.Fatalf("answered %d %v, want the failed write named", code, view)
	}
	if n := f.intents(); n != 0 {
		t.Fatalf("the refused request left %d intents", n)
	}
	restoreInstallWriteFault = nil
	if code, view := f.post(""); code != http.StatusAccepted {
		t.Fatalf("the slot was not released: %d %v", code, view)
	}
}

// The renames' directories are synced before the fact that says they were made.
func TestRestoreSyncsTheFlipsDirectoriesBeforeItsFact(t *testing.T) {
	t.Cleanup(func() { restoreInstallWriteFault = nil; restoreDirSyncHook = nil })
	f := newRestoreActFixture(t, "computer-act-syncs", 2)
	var mu sync.Mutex
	var log []string
	restoreDirSyncHook = func(dir string) {
		mu.Lock()
		log = append(log, "sync:"+filepath.Base(dir))
		mu.Unlock()
	}
	restoreInstallWriteFault = func(phase string) error {
		mu.Lock()
		log = append(log, "write:"+phase)
		mu.Unlock()
		return nil
	}
	_, accepted := f.post("")
	f.waitTerminal(f.actID(accepted))
	mu.Lock()
	defer mu.Unlock()
	completed := -1
	for i, entry := range log {
		if entry == "write:flip_completed" {
			completed = i
		}
	}
	if completed < 0 {
		t.Fatalf("no flip_completed write in %v", log)
	}
	var sawData, sawQuarantine bool
	for _, entry := range log[:completed] {
		sawData = sawData || entry == "sync:data"
		sawQuarantine = sawQuarantine || strings.HasPrefix(entry, "sync:rematerialize-quarantine-")
	}
	if !sawData || !sawQuarantine {
		t.Fatalf("directories synced before flip_completed: data=%v quarantine=%v in %v", sawData, sawQuarantine, log)
	}
}
