package agentcore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// R1, R2 of docs/reports/p5-host-build-layered-apply-2026-10-15.md: a
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

	rt.selfdevRestartCurrent = func(context.Context) error { return errors.New("refused") }
	if err := rt.restartIntoCurrentAfterRestore(context.Background(), "checkpoint-2"); err == nil {
		t.Fatal("a refused restart reported success")
	}
	if _, err := os.Stat(PlannedRestartMarkerPath(storePath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused restart left its marker: %v", err)
	}
}
