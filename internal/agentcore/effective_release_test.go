package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/updater"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// P6 failure modes pinned here (docs/archive/reports/p6-p8-release-adopt-2026-10-15.md):
//   I1 /health reports the release the process started with after `current`
//      moved;
//   I2 a self-developed computer reports the base build's tree;
//   I3 the tree is computed by writing to the source repository (pinned in
//      internal/release; here: the tree equals a real commit's);
//   I4 /health runs git on every call;
//   I5 the event head is read from a cache.

func waitEffectiveTree(t *testing.T, rt *Runtime) EffectiveRelease {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		identity := rt.EffectiveRelease(context.Background())
		if !identity.TreePending || time.Now().After(deadline) {
			return identity
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestEffectiveReleaseNamesTheAppliedOperationTreeHeadAndCodeRef(t *testing.T) {
	const computerID = "computer-identity"
	rt, productStore, updaterRoot := releaseTestRuntime(t, computerID)
	repo, base := releaseSourceRepo(t)
	rt.releaseSourceDir = repo
	patch, wantTree := releasePatch(t, repo, map[string]string{"frontend/public/drill.txt": "token\n"})

	// Startup saw a base release; then a self-development apply swapped
	// current without a restart yet (I1).
	baseDigest := writeCurrentRelease(t, updaterRoot, computerID, "base", nil)
	rt.selfdevStartupMu.Lock()
	rt.selfdevStartupReleaseDigest = baseDigest
	rt.selfdevStartupMu.Unlock()
	bundle := releaseFrozenBundle(t, updaterRoot, computerID, "op-applied", base, patch)
	applied := writeCurrentRelease(t, updaterRoot, computerID, "selfdev-op-applied", func(m *updater.ReleaseManifest) {
		m.CodeRef = "code:sha256:" + strings.Repeat("5", 64)
	})
	insertReleaseOperation(t, productStore, releaseOpRow{operationID: "op-applied", computerID: computerID, state: selfdev.StateApplied, bundleDigest: bundle, releaseDigest: applied})
	insertEventHead(t, productStore, computerID, 7, strings.Repeat("1", 64), strings.Repeat("2", 64))

	var calls int
	rt.effectiveTreeCompute = func(ctx context.Context, repo, commit string, patch []byte) (string, error) {
		calls++
		return defaultEffectiveTreeCompute(ctx, repo, commit, patch)
	}
	identity := waitEffectiveTree(t, rt)
	if identity.ReleaseDigest != applied || identity.ServingReleaseDigest != baseDigest {
		t.Fatalf("I1: release %s serving %s, want live %s and startup %s", identity.ReleaseDigest, identity.ServingReleaseDigest, applied, baseDigest)
	}
	if identity.OperationID != "op-applied" || identity.SourceBaseCommit != base || identity.SourcePatchSHA256 != computerevent.DigestBytes(patch) {
		t.Fatalf("effective operation = %+v", identity)
	}
	if identity.TreeHash != wantTree {
		t.Fatalf("I2: tree %q (error %q), want the patched tree %s", identity.TreeHash, identity.TreeError, wantTree)
	}
	if identity.CodeRef != "code:sha256:"+strings.Repeat("5", 64) {
		t.Fatalf("code ref = %q", identity.CodeRef)
	}
	if identity.EventHead == nil || identity.EventHead.Sequence != 7 || identity.EventHead.Effective != strings.Repeat("2", 64) {
		t.Fatalf("event head = %+v", identity.EventHead)
	}
	// I4: later reads reuse the tree.
	for i := 0; i < 5; i++ {
		_ = rt.EffectiveRelease(context.Background())
	}
	if calls != 1 {
		t.Fatalf("I4: git ran %d times for one release", calls)
	}
	// I5: the head is read live.
	insertEventHead(t, productStore, computerID, 8, strings.Repeat("3", 64), strings.Repeat("4", 64))
	if identity := rt.EffectiveRelease(context.Background()); identity.EventHead == nil || identity.EventHead.Sequence != 8 {
		t.Fatalf("I5: head after a new event = %+v", identity.EventHead)
	}

	// Both product surfaces carry the block.
	handler := NewAPIHandler(rt)
	rec := httptest.NewRecorder()
	handler.HandleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var health struct {
		ReleaseDigest    string           `json:"release_digest"`
		EffectiveRelease EffectiveRelease `json:"effective_release"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil || health.EffectiveRelease.TreeHash != wantTree || health.EffectiveRelease.ReleaseDigest != applied {
		t.Fatalf("/health effective_release = %s (%v)", rec.Body.String(), err)
	}
	rec = httptest.NewRecorder()
	obsReq := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
	obsReq.Header.Set("X-Authenticated-User", "owner")
	handler.HandleGuestObservability(rec, obsReq)
	var obs struct {
		EffectiveRelease *EffectiveRelease `json:"effective_release"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil || obs.EffectiveRelease == nil || obs.EffectiveRelease.OperationID != "op-applied" {
		t.Fatalf("observability effective_release = %d %s", rec.Code, rec.Body.String())
	}
}

func TestEffectiveReleaseOfABaseComputerIsTheBuildCommitTree(t *testing.T) {
	const computerID = "computer-base"
	rt, _, updaterRoot := releaseTestRuntime(t, computerID)
	repo, base := releaseSourceRepo(t)
	rt.releaseSourceDir = repo
	rt.effectiveBuildCommit = func() string { return base }
	digest := writeCurrentRelease(t, updaterRoot, computerID, "base", nil)
	identity := waitEffectiveTree(t, rt)
	if identity.ReleaseDigest != digest || identity.OperationID != "" || identity.TreeHash != releaseGit(t, repo, "rev-parse", base+"^{tree}") {
		t.Fatalf("base identity = %+v", identity)
	}
	// A build commit the source repository lacks (the seed lags the image,
	// P13) is reported as such, never as another tree.
	rt2, _, updaterRoot2 := releaseTestRuntime(t, computerID+"-lag")
	rt2.releaseSourceDir = repo
	rt2.effectiveBuildCommit = func() string { return strings.Repeat("0", 40) }
	writeCurrentRelease(t, updaterRoot2, computerID+"-lag", "base", nil)
	lagging := waitEffectiveTree(t, rt2)
	if lagging.TreeHash != "" || lagging.TreeError == "" {
		t.Fatalf("lagging seed = %+v, want an empty tree with a reason", lagging)
	}
}

// I6: vmctl learns the identity from the guest's own report, never a probe.
func TestEffectiveReleaseIsReportedToVmctl(t *testing.T) {
	const computerID = "computer-report"
	rt, productStore, updaterRoot := releaseTestRuntime(t, computerID)
	repo, base := releaseSourceRepo(t)
	rt.releaseSourceDir = repo
	rt.effectiveBuildCommit = func() string { return base }
	digest := writeCurrentRelease(t, updaterRoot, computerID, "base", nil)
	insertEventHead(t, productStore, computerID, 3, strings.Repeat("1", 64), strings.Repeat("2", 64))
	got := make(chan vmctl.EffectiveReleaseReport, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var report vmctl.EffectiveReleaseReport
		if r.URL.Path == "/internal/vmctl/computers/effective-release" && json.NewDecoder(r.Body).Decode(&report) == nil {
			got <- report
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"recorded"}`))
	}))
	defer server.Close()
	rt.selfdevRoute = vmctl.NewClient(server.URL)
	rt.reportEffectiveReleaseAsync("test")
	select {
	case report := <-got:
		if report.ComputerID != computerID || report.Release.ReleaseDigest != digest || report.Release.TreeHash != releaseGit(t, repo, "rev-parse", base+"^{tree}") ||
			report.Release.EventHeadSequence != 3 || report.Release.EffectiveEventHead != strings.Repeat("2", 64) {
			t.Fatalf("report = %+v", report)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("no report reached vmctl")
	}
}
