package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// R1, R2 of docs/archive/reports/p5-host-build-layered-apply-2026-10-15.md: a
// restore that takes a layered release away (or brings one back) changes
// the binary that must serve; a restore between two plain releases does not.
func TestRestoredExecDiffers(t *testing.T) {
	layered := updater.ReleaseManifest{LayeringEntrypoint: "x-autoputer/bin/autoputer", CodeCommit: "c1"}
	plain := updater.ReleaseManifest{}
	for name, tc := range map[string]struct {
		running  *effectiveReleaseIdentity
		restored updater.ReleaseManifest
		err      error
		want     bool
	}{
		"layered to plain":      {&effectiveReleaseIdentity{Layered: true, CodeCommit: "c1"}, plain, nil, true},
		"plain to layered":      {&effectiveReleaseIdentity{}, layered, nil, true},
		"no release to layered": {nil, layered, nil, true},
		"plain to plain":        {&effectiveReleaseIdentity{}, plain, nil, false},
		"no release to plain":   {nil, plain, nil, false},
		"same layer":            {&effectiveReleaseIdentity{Layered: true, CodeCommit: "c1"}, layered, nil, false},
		"other layer":           {&effectiveReleaseIdentity{Layered: true, CodeCommit: "c0"}, layered, nil, true},
		"unreadable restored":   {&effectiveReleaseIdentity{}, plain, errors.New("gone"), true},
	} {
		if got := restoredExecDiffers(tc.running, tc.restored, tc.err); got != tc.want {
			t.Errorf("%s: got %v, want %v", name, got, tc.want)
		}
	}
}

// R3: the restart into the restored release is planned (the marker is on
// disk when the updater restarts), and a refused restart leaves no marker a
// later crash could read as planned.
func TestRestartIntoCurrentAfterRestoreWritesMarker(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	var sawMarker bool
	rt := &Runtime{cfg: provideriface.Config{StorePath: storePath}}
	rt.selfdevRestartCurrent = func(context.Context) error {
		_, err := os.Stat(PlannedRestartMarkerPath(storePath))
		sawMarker = err == nil
		return nil
	}
	if err := rt.restartIntoCurrentAfterRestore(context.Background(), "checkpoint-1"); err != nil {
		t.Fatal(err)
	}
	if !sawMarker {
		t.Fatal("the updater restarted before the planned-restart marker was written")
	}
	marker, planned := ConsumePlannedRestartMarker(storePath, time.Now().UTC())
	if !planned || marker.Reason != PlannedRestartSelfDevelopmentRestore || marker.Target != "checkpoint-1" {
		t.Fatalf("marker = %+v planned=%v", marker, planned)
	}

	// R4: a restore restart is not an apply; it spends no apply attempt, so
	// restore number four to the same checkpoint still restarts.
	for i := 0; i < guestApplyAttemptLimit+2; i++ {
		sawMarker = false
		if err := rt.restartIntoCurrentAfterRestore(context.Background(), "checkpoint-1"); err != nil || !sawMarker {
			t.Fatalf("restore restart %d: err=%v marker=%v", i+1, err, sawMarker)
		}
		_, _ = ConsumePlannedRestartMarker(storePath, time.Now().UTC())
	}

	rt.selfdevRestartCurrent = func(context.Context) error { return errors.New("refused") }
	if err := rt.restartIntoCurrentAfterRestore(context.Background(), "checkpoint-2"); err == nil {
		t.Fatal("a refused restart reported success")
	}
	if _, err := os.Stat(PlannedRestartMarkerPath(storePath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused restart left its marker: %v", err)
	}
}

// R5, R6: a restore restart waits for an idle computer (it never cuts a
// running run), and restart_scheduled is reported only once the marker is
// durable and the gate has passed; a busy computer reports the deferral and
// restarts at its next idle moment.
func TestRequestRestoreRestartGatesOnIdle(t *testing.T) {
	savedDelay, savedPoll := restoreRestartDelay, restoreRestartIdlePoll
	restoreRestartDelay, restoreRestartIdlePoll = 0, 10*time.Millisecond
	defer func() { restoreRestartDelay, restoreRestartIdlePoll = savedDelay, savedPoll }()

	storePath := filepath.Join(t.TempDir(), "runtime.db")
	restarted := make(chan bool, 4)
	rt := &Runtime{cfg: provideriface.Config{StorePath: storePath}, running: map[string]context.CancelFunc{}}
	rt.selfdevRestartCurrent = func(context.Context) error {
		_, err := os.Stat(PlannedRestartMarkerPath(storePath))
		restarted <- err == nil
		return nil
	}

	// Idle: scheduled, marker durable before the answer.
	scheduled, deferred := rt.requestRestoreRestart(context.Background(), "checkpoint-1")
	if !scheduled || deferred != "" {
		t.Fatalf("idle: scheduled=%v deferred=%q", scheduled, deferred)
	}
	if marker, ok := readMarkerForTest(storePath); !ok || marker.Reason != PlannedRestartSelfDevelopmentRestore {
		t.Fatalf("idle: no durable marker when restart_scheduled was answered (%+v)", marker)
	}
	select {
	case withMarker := <-restarted:
		if !withMarker {
			t.Fatal("idle: restarted without the marker")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("idle: no restart")
	}
	_, _ = ConsumePlannedRestartMarker(storePath, time.Now().UTC())

	// Busy: not scheduled, no marker, no restart until the run ends.
	rt.runningMu.Lock()
	rt.running["loop-1"] = func() {}
	rt.runningMu.Unlock()
	scheduled, deferred = rt.requestRestoreRestart(context.Background(), "checkpoint-2")
	if scheduled || !strings.Contains(deferred, "busy") {
		t.Fatalf("busy: scheduled=%v deferred=%q", scheduled, deferred)
	}
	if _, ok := readMarkerForTest(storePath); ok {
		t.Fatal("busy: a marker was written while runs were running")
	}
	select {
	case <-restarted:
		t.Fatal("busy: restarted over a running run")
	case <-time.After(100 * time.Millisecond):
	}
	rt.runningMu.Lock()
	delete(rt.running, "loop-1")
	rt.runningMu.Unlock()
	select {
	case withMarker := <-restarted:
		if !withMarker {
			t.Fatal("deferred: restarted without the marker")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("deferred: no restart at the idle moment")
	}

	// No durable marker: not scheduled, never restarted.
	rt2 := &Runtime{cfg: provideriface.Config{StorePath: filepath.Join(t.TempDir(), "missing", "runtime.db")}, running: map[string]context.CancelFunc{}}
	rt2.selfdevRestartCurrent = func(context.Context) error {
		restarted <- false
		return nil
	}
	if scheduled, deferred := rt2.requestRestoreRestart(context.Background(), "checkpoint-3"); scheduled || deferred == "" {
		t.Fatalf("unwritable marker: scheduled=%v deferred=%q", scheduled, deferred)
	}
	select {
	case <-restarted:
		t.Fatal("unwritable marker: restarted without a marker")
	case <-time.After(100 * time.Millisecond):
	}
}

func readMarkerForTest(storePath string) (PlannedRestart, bool) {
	raw, err := os.ReadFile(PlannedRestartMarkerPath(storePath))
	if err != nil {
		return PlannedRestart{}, false
	}
	var marker PlannedRestart
	if json.Unmarshal(raw, &marker) != nil {
		return PlannedRestart{}, false
	}
	return marker, true
}
