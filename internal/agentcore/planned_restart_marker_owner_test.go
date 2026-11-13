package agentcore

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Review round 3: the planned-restart marker has ONE owner
// (planned_restart_marker.go) with one mutex. Failure list, written first:
//
//  O1. a guest marker (platform_update, a self-development apply or restore)
//      lands between the withdrawal's reason check and its delete, and the
//      withdrawal removes the guest's marker;
//  O2. some non-test file touches the marker path, or the raw write/clear
//      helpers, outside the owner, so the next unserialised interleaving is
//      one edit away;
//  O3. the caller-gone refusal lifts the refresh hold and the producer's
//      restart block although an earlier request's durable marker means the
//      host's kill is coming.

// O2: only the owner file names the marker path or the raw helpers. A source
// scan, in the style of TestAppendReceiptRowWritersAreKnown.
func TestPlannedRestartMarkerIsTouchedOnlyThroughItsOwner(t *testing.T) {
	const owner = "planned_restart_marker.go"
	forbidden := []string{
		"PlannedRestartMarkerPath(", // the path, outside the owner
		`.planned-restart"`,         // a second spelling of the path
		"writePlannedRestartMarkerLocked(",
		"clearPlannedRestartMarkerLocked(",
	}
	var offenders []string
	for _, root := range []string{"../../internal", "../../cmd"} {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if filepath.Base(path) == owner && strings.Contains(filepath.ToSlash(path), "internal/agentcore/") {
				return nil
			}
			raw, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil
			}
			for _, word := range forbidden {
				if bytes.Contains(raw, []byte(word)) {
					offenders = append(offenders, path+" uses "+word)
				}
			}
			return nil
		})
	}
	if len(offenders) > 0 {
		t.Fatalf("the planned-restart marker is touched outside %s (one owner, one mutex):\n%s", owner, strings.Join(offenders, "\n"))
	}
}

// O3: the caller-gone refusal keeps the holds while an earlier request's
// marker is durable.
func TestCallerGoneKeepsTheHoldsWhileAnEarlierRequestsMarkerIsDurable(t *testing.T) {
	f := newPauseFixture(t)
	if code, body := f.mark(t, "?allow_busy=1"); code != http.StatusConflict {
		t.Fatalf("phase 1: %d %s", code, body)
	}
	f.rt.waitCapsulePauses()
	// An earlier request's marker: the host's kill is coming.
	if err := WritePlannedRestartMarker(f.rt.cfg.StorePath, PlannedRestart{Reason: PlannedRestartPlatformRefresh, Target: "earlier", RequestedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.rt.beforeMarkerWriteHook = cancel
	req := httptest.NewRequest(http.MethodPost, "/internal/runtime/planned-restart?allow_busy=1", bytes.NewBufferString(`{"target":"761d94c5"}`)).WithContext(ctx)
	req.RemoteAddr = "192.0.2.1:4000"
	f.handler.HandleInternalPlannedRestart(httptest.NewRecorder(), req)

	if !f.rt.refreshHoldsNewWork(time.Now()) {
		t.Fatal("new engineering was admitted into a kill that an earlier marker announces")
	}
	f.rt.snapshots.mu.Lock()
	blocked := time.Now().Before(f.rt.snapshots.blockUntil)
	f.rt.snapshots.mu.Unlock()
	if !blocked {
		t.Fatal("the producer's block was lifted although a marker is durable")
	}
}
