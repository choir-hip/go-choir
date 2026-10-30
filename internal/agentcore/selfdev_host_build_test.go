package agentcore

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/hostbuild"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// Guest legs of docs/archive/reports/p5-host-build-layered-apply-2026-10-15.md:
// G1-G8 through the real reconciler, the real updater engine and the
// derivable fixture's tape, with a fake corpusd host build route.

const (
	hostBuildTestBaseCommit = "0123456789abcdef0123456789abcdef01234567"
	hostBuildTestCodeCommit = "c8fb78349781faef5ab648623919d394171d37ca"
	hostBuildTestLayer      = "3l816rvxd4qv5xdf3r9lnm1mr1a0cgb1-autoputer-0.1.0"
	hostBuildTestFrontend   = "<html>host-built selfdev-probe</html>\n"
)

var hostBuildTestPatch = []byte("diff --git a/internal/selfdevprobe/probe.go b/internal/selfdevprobe/probe.go\nnew file mode 100644\n--- /dev/null\n+++ b/internal/selfdevprobe/probe.go\n@@ -0,0 +1 @@\n+package selfdevprobe\n")

// narExport encodes a nix-store --export stream of one store path, enough
// for the updater's reader.
type narExport struct{ bytes.Buffer }

func (e *narExport) u64(v uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	e.Write(b[:])
}

func (e *narExport) str(s string) {
	e.u64(uint64(len(s)))
	e.WriteString(s)
	if pad := (8 - len(s)%8) % 8; pad > 0 {
		e.Write(make([]byte, pad))
	}
}

type narEntry struct {
	contents   string
	executable bool
}

func (e *narExport) tree(files map[string]narEntry, prefix string) {
	e.str("(")
	e.str("type")
	e.str("directory")
	children := map[string]bool{}
	for path := range files {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimPrefix(path, prefix), "/")
		children[name] = true
	}
	names := make([]string, 0, len(children))
	for name := range children {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		e.str("entry")
		e.str("(")
		e.str("name")
		e.str(name)
		e.str("node")
		if file, ok := files[prefix+name]; ok {
			e.str("(")
			e.str("type")
			e.str("regular")
			if file.executable {
				e.str("executable")
				e.str("")
			}
			e.str("contents")
			e.str(file.contents)
			e.str(")")
		} else {
			e.tree(files, prefix+name+"/")
		}
		e.str(")")
	}
	e.str(")")
}

func hostBuildTestClosure() []byte {
	var e narExport
	e.u64(1)
	e.str("nix-archive-1")
	e.tree(map[string]narEntry{
		"bin/autoputer":             {contents: "#!/bin/sh\necho host-built\n", executable: true},
		"share/go-choir/build.json": {contents: `{"schema_version":1,"artifact":"autoputer","commit":"` + hostBuildTestCodeCommit + `","built_at":"t"}`},
		"frontend/index.html":       {contents: hostBuildTestFrontend},
		"frontend/assets/app.js":    {contents: "console.log('probe')\n"},
	}, "")
	e.u64(0x000000004558494e)
	e.str("/nix/store/" + hostBuildTestLayer)
	e.u64(0)
	e.str("")
	e.u64(0)
	e.u64(0)
	return e.Bytes()
}

type fakeHostBuilder struct {
	key       ed25519.PrivateKey
	mu        sync.Mutex
	mode      string // done | pending | failed | forged | unconfigured
	jobs      map[string]hostbuild.Job
	blobs     map[string][]byte
	submits   atomic.Int32
	fetches   atomic.Int32
	lastPatch []byte
}

func newFakeHostBuilder(key ed25519.PrivateKey, mode string) *fakeHostBuilder {
	return &fakeHostBuilder{key: key, mode: mode, jobs: map[string]hostbuild.Job{}, blobs: map[string][]byte{}}
}

func (f *fakeHostBuilder) setMode(mode string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mode = mode
	for id, job := range f.jobs {
		f.jobs[id] = f.shape(job)
	}
}

func (f *fakeHostBuilder) shape(job hostbuild.Job) hostbuild.Job {
	job.Result, job.Receipt, job.Reason, job.Detail = nil, nil, "", ""
	switch f.mode {
	case "pending":
		job.State, job.Admission = hostbuild.StateQueued, "waiting: host memory available is under the floor"
	case "failed":
		job.State, job.Reason, job.Detail = hostbuild.StateFailed, hostbuild.ReasonBuildFailed, "builder: apply source patch: patch does not apply"
	case "done", "forged":
		nar := hostBuildTestClosure()
		// The real builder receipt (builder.ClosureResult) names the patch
		// and base it built; the fake keeps those fields so the release
		// witness reads the same shape.
		receipt := []byte(`{"code_commit":"` + hostBuildTestCodeCommit + `","source_patch_sha256":"` + job.SourcePatchSHA256 + `","source_patch_base_commit":"` + job.BaseCommit + `"}`)
		f.blobs[computerevent.DigestBytes(nar)] = nar
		f.blobs[computerevent.DigestBytes(receipt)] = receipt
		result := hostbuild.Result{
			JobID: job.JobID, ComputerID: job.ComputerID, OperationID: job.OperationID, BundleDigest: job.BundleDigest,
			BaseCommit: job.BaseCommit, SourcePatchSHA256: job.SourcePatchSHA256, BaseImageManifestDigest: job.BaseImageManifestDigest,
			CodeCommit: hostBuildTestCodeCommit, CodeCommitSource: "derived-patch", RuntimePath: "/nix/store/" + hostBuildTestLayer,
			LayeringEntrypoint: hostBuildTestLayer + "/bin/autoputer", ClosureDigest: computerevent.DigestBytes(nar),
			ClosureBytes: int64(len(nar)), BuilderReceiptDigest: computerevent.DigestBytes(receipt),
			DerivationPath: "/nix/store/cccccccccccccccccccccccccccccccc-autoputer.drv", BuiltAt: "2026-10-15T00:00:00Z",
		}
		key := f.key
		if f.mode == "forged" {
			_, key, _ = ed25519.GenerateKey(rand.Reader)
		}
		digest, _ := result.Digest()
		signed, err := selfdevprotocol.NewAuthorityReceipt(selfdevprotocol.ReceiptKindHostBuild, job.ComputerID, job.JobID, digest, "corpusd",
			computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "authority-test"}, PrivateKey: key}, time.Now().UTC())
		if err != nil {
			panic(err)
		}
		job.State, job.Result, job.Receipt = hostbuild.StateDone, &result, &signed
	}
	return job
}

func (f *fakeHostBuilder) Submit(_ context.Context, req hostbuild.Request) (hostbuild.Job, int, error) {
	f.submits.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	patch, err := base64.StdEncoding.DecodeString(req.PatchBase64)
	if err != nil || computerevent.DigestBytes(patch) != req.SourcePatchSHA256 {
		return hostbuild.Job{}, http.StatusBadRequest, fmt.Errorf("patch digest mismatch")
	}
	switch f.mode {
	case "busy":
		return hostbuild.Job{}, http.StatusTooManyRequests, errors.New("computer event client: corpusd returned 429: computer_busy: this computer already has host build x queued")
	case "quota":
		return hostbuild.Job{}, http.StatusTooManyRequests, errors.New("computer event client: corpusd returned 429: build_quota_exhausted: this computer recorded 8 host builds in the last 24 hours")
	}
	f.lastPatch = patch
	id, _ := req.JobID()
	job := f.shape(hostbuild.Job{
		JobID: id, ComputerID: req.ComputerID, OperationID: req.OperationID, BundleDigest: req.BundleDigest,
		BaseCommit: req.BaseCommit, SourcePatchSHA256: req.SourcePatchSHA256, BaseImageManifestDigest: req.BaseImageManifestDigest,
	})
	f.jobs[id] = job
	return job, http.StatusOK, nil
}

func (f *fakeHostBuilder) Status(_ context.Context, computerID, jobID string) (hostbuild.Job, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.mode == "unconfigured" {
		return hostbuild.Job{}, http.StatusServiceUnavailable, errors.New("computer event client: corpusd returned 503: host_builder_unconfigured: this host has no build service configured")
	}
	job, ok := f.jobs[jobID]
	if !ok || job.ComputerID != computerID {
		return hostbuild.Job{}, http.StatusNotFound, errors.New("computer event client: corpusd returned 404: host build job not found")
	}
	return job, http.StatusOK, nil
}

func (f *fakeHostBuilder) FetchBlob(_ context.Context, _ string, digest string) (io.ReadCloser, error) {
	f.fetches.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	raw, ok := f.blobs[digest]
	if !ok {
		return nil, fmt.Errorf("blob %s unavailable", digest)
	}
	return io.NopCloser(bytes.NewReader(raw)), nil
}

func newHostBuildFixture(t *testing.T, computerID, mode string) (*derivableSelfDevFixture, *fakeHostBuilder) {
	t.Helper()
	fx := newDerivableSelfDevFixture(t, computerID)
	fx.goPatch = hostBuildTestPatch
	manifest := filepath.Join(t.TempDir(), "guest-image-manifest")
	if err := os.WriteFile(manifest, []byte("contract=choir-guest-image-v1\nbuild_commit="+hostBuildTestBaseCommit+"\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	fx.rt.selfdevGuestImageManifest = manifest
	host := newFakeHostBuilder(fx.platformKey, mode)
	fx.rt.selfdevHostBuilder = host
	return fx, host
}

// G1, G6: an accepted Go change applies as the host-built layered release:
// the layer's closure is materialized, current carries its exec pointer and
// code commit, the served frontend is the layer's, the checkpoint publishes
// (the frontend identity is derivable), and a second pass asks the host for
// nothing.
func TestSelfDevGoChangeAppliesHostBuiltLayeredRelease(t *testing.T) {
	fx, host := newHostBuildFixture(t, "computer-p5-layered", "done")
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-p5-layered")
	fx.appendDecision(t, operation, bundleDigest, "decision-p5-layered")
	applied := fx.waitForState(t, operation.OperationID, selfdev.StateApplied)

	current, err := updater.ReadCurrentManifest(fx.updaterRoot)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContentDigest != applied.ReleaseDigest || current.LayeringEntrypoint != hostBuildTestLayer+"/bin/autoputer" ||
		current.CodeCommit != hostBuildTestCodeCommit || current.BaseCommit != hostBuildTestBaseCommit || current.ClosureDigest == "" || current.BuilderReceiptDigest == "" {
		t.Fatalf("current release is not the host-built layer: %+v", current)
	}
	for _, file := range current.Files {
		if file.Path == "bin/autoputer" {
			t.Fatal("the capsule-built binary is a release input")
		}
	}
	entry, err := os.ReadFile(filepath.Join(fx.updaterRoot, "current", "layering-entrypoint"))
	if err != nil || strings.TrimSpace(string(entry)) != filepath.Join(fx.updaterRoot, "store", hostBuildTestLayer, "bin", "autoputer") {
		t.Fatalf("layering-entrypoint = %q %v", entry, err)
	}
	served, err := os.ReadFile(filepath.Join(fx.updaterRoot, "current", "frontend", "index.html"))
	if err != nil || string(served) != hostBuildTestFrontend {
		t.Fatalf("served frontend = %q %v, want the layer's", served, err)
	}
	if !bytes.Equal(host.lastPatch, hostBuildTestPatch) {
		t.Fatalf("host received patch %q", host.lastPatch)
	}
	kinds := fx.countEventKinds()
	for _, want := range []computerevent.EventKind{computerevent.EventMaterializationApplied, computerevent.EventCheckpointPublished, computerevent.EventRouteProjectionUpdated} {
		if kinds[want] != 1 {
			t.Fatalf("event %s count=%d (%v)", want, kinds[want], kinds)
		}
	}
	if fx.service.restarts != 1 {
		t.Fatalf("restarts = %d, want 1 planned restart", fx.service.restarts)
	}
	submits, fetches := host.submits.Load(), host.fetches.Load()
	fx.rt.reconcileSelfDevelopmentMaterialization(context.Background())
	if host.submits.Load() != submits || host.fetches.Load() != fetches {
		t.Fatal("a settled operation asked the host again")
	}
	// G6: a resumed pass (as after the planned restart) reads the staged host
	// build, asks the host nothing, and mints the identical layered manifest.
	raw, err := os.ReadFile(filepath.Join(fx.updaterRoot, "incoming", applied.BundleDigest, "bundle.json"))
	if err != nil {
		t.Fatal(err)
	}
	var bundle transaction.CapsuleEffectBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatal(err)
	}
	again, err := fx.rt.selfDevelopmentHostBuild(context.Background(), applied, bundle)
	if err != nil {
		t.Fatal(err)
	}
	if host.submits.Load() != submits || host.fetches.Load() != fetches {
		t.Fatal("a resumed pass re-fetched the host build instead of reusing the staged one")
	}
	remint, err := updater.FinalizeManifest(layeredSelfDevelopmentManifest(updater.ReleaseManifest{
		Version: current.Version, ComputerID: current.ComputerID, AcceptedEventHead: current.AcceptedEventHead,
		CodeRef: current.CodeRef, ArtifactProgramRef: current.ArtifactProgramRef, EventSchemaVersion: current.EventSchemaVersion,
		ReducerVersion: current.ReducerVersion, Marker: current.Marker,
	}, again))
	if err != nil || remint.ContentDigest != current.ContentDigest {
		t.Fatalf("resumed mint %s != applied %s (%v)", remint.ContentDigest, current.ContentDigest, err)
	}
}

// G8 then G1: while the host build is queued the operation stays accepted
// (no materialization started, desks not held); once done, it applies.
func TestSelfDevGoChangeWaitsAcceptedWhileHostBuilds(t *testing.T) {
	fx, host := newHostBuildFixture(t, "computer-p5-pending", "pending")
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-p5-pending")
	fx.appendDecision(t, operation, bundleDigest, "decision-p5-pending")
	accepted := fx.waitForState(t, operation.OperationID, selfdev.StateAccepted)
	err := fx.rt.materializeSelfDevelopmentOperation(context.Background(), accepted, false)
	if !errors.Is(err, errHostBuildPending) {
		t.Fatalf("pending pass = %v", err)
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventMaterializationStarted] != 0 {
		t.Fatalf("materialization started while the host builds: %v", kinds)
	}
	if fx.rt.SelfDevelopmentApplyMaterializing(context.Background()) {
		t.Fatal("desk work held while the host builds")
	}
	if fx.applyCalls.Load() != 0 {
		t.Fatal("updater called before the host build finished")
	}
	host.setMode("done")
	fx.rt.reconcileSelfDevelopmentMaterialization(context.Background())
	fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
	if host.submits.Load() != 1 {
		t.Fatalf("submits = %d, want 1 (a retry names the same job)", host.submits.Load())
	}
}

// H20 (guest side): a busy host (429 computer_busy or queue_full) keeps the
// operation accepted; the next pass submits again and builds.
func TestSelfDevGoChangeBusyHostKeepsAccepted(t *testing.T) {
	fx, host := newHostBuildFixture(t, "computer-p5-busy", "busy")
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-p5-busy")
	fx.appendDecision(t, operation, bundleDigest, "decision-p5-busy")
	accepted := fx.waitForState(t, operation.OperationID, selfdev.StateAccepted)
	err := fx.rt.materializeSelfDevelopmentOperation(context.Background(), accepted, false)
	if !errors.Is(err, errHostBuildPending) {
		t.Fatalf("busy host pass = %v, want pending", err)
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventMaterializationFailed] != 0 {
		t.Fatalf("a busy host failed the operation: %v", kinds)
	}
	host.setMode("done")
	fx.rt.reconcileSelfDevelopmentMaterialization(context.Background())
	fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
}

// G3, G4: a failed, forged or unconfigured host build fails the operation
// with a visible reason, records materialization_failed, and never calls the
// updater; the computer keeps its current release.
func TestSelfDevGoChangeHostBuildFailureFailsCleanly(t *testing.T) {
	for mode, reason := range map[string]string{
		"failed":       "host build failed (build_failed)",
		"forged":       "host build failed (receipt_refused)",
		"unconfigured": "host build failed (host_builder_unconfigured)",
		"quota":        "host build failed (build_quota_exhausted)",
	} {
		t.Run(mode, func(t *testing.T) {
			fx, _ := newHostBuildFixture(t, "computer-p5-"+mode, mode)
			before, beforeErr := updater.ReadCurrentManifest(fx.updaterRoot)
			operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-p5-"+mode)
			fx.appendDecision(t, operation, bundleDigest, "decision-p5-"+mode)
			failed := fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
			if !strings.Contains(failed.TerminalError, reason) {
				t.Fatalf("terminal error = %q, want %q", failed.TerminalError, reason)
			}
			if fx.applyCalls.Load() != 0 || fx.service.restarts != 0 {
				t.Fatalf("updater applied (%d) or restarted (%d) after a failed host build", fx.applyCalls.Load(), fx.service.restarts)
			}
			if kinds := fx.countEventKinds(); kinds[computerevent.EventMaterializationFailed] != 1 || kinds[computerevent.EventMaterializationStarted] != 0 {
				t.Fatalf("events = %v", kinds)
			}
			after, afterErr := updater.ReadCurrentManifest(fx.updaterRoot)
			if (beforeErr == nil) != (afterErr == nil) || before.ContentDigest != after.ContentDigest {
				t.Fatalf("current release changed: %+v -> %+v", before, after)
			}
		})
	}
}
