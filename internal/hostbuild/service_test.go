package hostbuild

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/builder"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// The failure modes these tests hold are H1-H14 of
// docs/reports/p5-host-build-layered-apply-2026-10-15.md.

const testBaseCommit = "0123456789abcdef0123456789abcdef01234567"

type harness struct {
	t        *testing.T
	dir      string
	cfg      Config
	pub      ed25519.PublicKey
	blobs    sync.Map // digest -> bytes
	builds   atomic.Int32
	pressure atomic.Value // Pressure
	build    func(ctx context.Context, req builder.Request) (*builder.ClosureResult, error)
	baseHash string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{t: t, dir: t.TempDir()}
	manifest := "contract=choir-guest-image-v1\nbuild_commit=" + testBaseCommit + "\nautoputer=/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-autoputer-0.1.0\n"
	manifestPath := filepath.Join(h.dir, "guest-image-manifest")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	h.baseHash = sha256Hex([]byte(manifest))
	diskPath := filepath.Join(h.dir, "storedisk.erofs")
	if err := os.WriteFile(diskPath, []byte("erofs"), 0o644); err != nil {
		t.Fatal(err)
	}
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	h.pub = pub
	signer := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test-key"}, PrivateKey: priv}
	h.pressure.Store(Pressure{MemAvailableBytes: 64 << 30})
	h.build = func(ctx context.Context, req builder.Request) (*builder.ClosureResult, error) {
		return fakeBuild(req)
	}
	h.cfg = Config{
		StateDir: filepath.Join(h.dir, "state"), SourceDir: filepath.Join(h.dir, "src"),
		BaseManifestPath: manifestPath, BaseStoreDiskPath: diskPath,
		MaxQueue: 4, BuildTimeout: 2 * time.Second, QueueTimeout: 300 * time.Millisecond,
		AdmitPoll: 10 * time.Millisecond, MemoryFloorBytes: 6 << 30, MaxMemorySomeAvg60: 10,
		Build: func(ctx context.Context, req builder.Request) (*builder.ClosureResult, error) {
			h.builds.Add(1)
			return h.build(ctx, req)
		},
		Sample: func() (Pressure, error) { return h.pressure.Load().(Pressure), nil },
		Sign: func(computerID, jobID, resultDigest string, at time.Time) (selfdevprotocol.AuthorityReceipt, error) {
			return selfdevprotocol.NewAuthorityReceipt(selfdevprotocol.ReceiptKindHostBuild, computerID, jobID, resultDigest, "corpusd-test", signer, at)
		},
		Store: func(digest, path string) error {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if sha256Hex(raw) != digest {
				return errors.New("digest mismatch")
			}
			h.blobs.Store(digest, raw)
			return nil
		},
	}
	return h
}

// fakeBuild writes a tiny closure and receipt the way builder.Build does.
func fakeBuild(req builder.Request) (*builder.ClosureResult, error) {
	if err := os.MkdirAll(req.OutDir, 0o700); err != nil {
		return nil, err
	}
	patch, err := os.ReadFile(req.SourcePatch)
	if err != nil {
		return nil, err
	}
	nar := append([]byte("nar:"), patch...)
	narPath := filepath.Join(req.OutDir, "app-layer-closure.nar")
	if err := os.WriteFile(narPath, nar, 0o444); err != nil {
		return nil, err
	}
	res := &builder.ClosureResult{
		Base:                  builder.BaseIdentity{GuestImageManifestDigest: "m", StoreDiskSHA256: "d"},
		RuntimePath:           "/nix/store/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-autoputer-0.1.0",
		ClosurePaths:          []string{"/nix/store/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-autoputer-0.1.0"},
		ExportedDigest:        sha256Hex(nar),
		ExportedPath:          narPath,
		DerivationPath:        "/nix/store/cccccccccccccccccccccccccccccccc-autoputer-0.1.0.drv",
		CodeCommit:            "fedcba9876543210fedcba9876543210fedcba98",
		CodeCommitSource:      "derived-patch",
		SourcePatchSHA256:     sha256Hex(patch),
		SourcePatchBaseCommit: req.SourcePatchBaseCommit,
		StoreSchemaVersion:    1,
		BuiltAt:               "2026-10-15T00:00:00Z",
	}
	if err := res.WriteReceipt(filepath.Join(req.OutDir, "builder-receipt.json")); err != nil {
		return nil, err
	}
	return res, nil
}

func (h *harness) service() *Service {
	h.t.Helper()
	svc, err := New(h.cfg)
	if err != nil {
		h.t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.t.Cleanup(func() {
		cancel()
		svc.Wait()
	})
	svc.Start(ctx)
	return svc
}

const goPatch = "diff --git a/internal/selfdevprobe/probe.go b/internal/selfdevprobe/probe.go\nnew file mode 100644\n--- /dev/null\n+++ b/internal/selfdevprobe/probe.go\n@@ -0,0 +1 @@\n+package selfdevprobe\n"

func (h *harness) request(computer, patch string) Request {
	return Request{
		ComputerID: computer, OperationID: "selfdev-op-" + computer, BundleDigest: strings.Repeat("b", 64),
		BaseCommit: testBaseCommit, SourcePatchSHA256: sha256Hex([]byte(patch)),
		BaseImageManifestDigest: h.baseHash, PatchBase64: base64.StdEncoding.EncodeToString([]byte(patch)),
	}
}

func waitTerminal(t *testing.T, svc *Service, computer, jobID string) Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := svc.Get(computer, jobID)
		if ok && job.Terminal() {
			return job
		}
		time.Sleep(5 * time.Millisecond)
	}
	job, _ := svc.Get(computer, jobID)
	t.Fatalf("job %s did not end: %+v", jobID, job)
	return Job{}
}

func TestBuildDoneIsSignedStoredAndBound(t *testing.T) {
	h := newHarness(t)
	svc := h.service()
	job, err := svc.Submit(h.request("computer-a", goPatch))
	if err != nil {
		t.Fatal(err)
	}
	job = waitTerminal(t, svc, "computer-a", job.JobID)
	if job.State != StateDone || job.Result == nil || job.Receipt == nil {
		t.Fatalf("job = %+v", job)
	}
	// H10: the closure and the builder receipt are in the blob store under
	// the digests the result names.
	if _, ok := h.blobs.Load(job.Result.ClosureDigest); !ok {
		t.Fatalf("closure %s not stored", job.Result.ClosureDigest)
	}
	if _, ok := h.blobs.Load(job.Result.BuilderReceiptDigest); !ok {
		t.Fatalf("builder receipt %s not stored", job.Result.BuilderReceiptDigest)
	}
	if job.Result.LayeringEntrypoint != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-autoputer-0.1.0/bin/autoputer" {
		t.Fatalf("entrypoint = %q", job.Result.LayeringEntrypoint)
	}
	// H11: the receipt verifies under the platform-control key and binds the
	// job id and the result digest.
	if err := job.Receipt.Verify(h.pub); err != nil {
		t.Fatal(err)
	}
	digest, _ := job.Result.Digest()
	if job.Receipt.Kind != selfdevprotocol.ReceiptKindHostBuild || job.Receipt.RequestCommitment != job.JobID || job.Receipt.ArtifactDigest != digest {
		t.Fatalf("receipt binding = %+v", job.Receipt)
	}
	if err := VerifyJob(job, h.request("computer-a", goPatch), h.pub); err != nil {
		t.Fatalf("VerifyJob: %v", err)
	}
	// G4 mirror: a tampered result fails verification.
	tampered := job
	r := *job.Result
	r.CodeCommit = strings.Repeat("0", 40)
	tampered.Result = &r
	if err := VerifyJob(tampered, h.request("computer-a", goPatch), h.pub); err == nil {
		t.Fatal("tampered result verified")
	}
	// The work dir (and its GC-root link) is gone after ingest.
	if entries, _ := os.ReadDir(filepath.Join(h.cfg.StateDir, "work")); len(entries) != 0 {
		t.Fatalf("work dir left behind: %v", entries)
	}
}

func TestPatchDigestMismatchIsRefusedWithoutAJob(t *testing.T) { // H1
	h := newHarness(t)
	svc := h.service()
	req := h.request("computer-a", goPatch)
	req.PatchBase64 = base64.StdEncoding.EncodeToString([]byte(goPatch + "+tampered\n"))
	if _, err := svc.Submit(req); err == nil || !strings.Contains(err.Error(), ReasonPatchDigestMismatch) {
		t.Fatalf("err = %v", err)
	}
	id, _ := req.JobID()
	if _, ok := svc.Get("computer-a", id); ok {
		t.Fatal("a mismatched patch recorded a job (a corrupt transfer would poison the retry)")
	}
	if h.builds.Load() != 0 {
		t.Fatal("built")
	}
}

func TestRecipeAndEpochPatchesAreRefused(t *testing.T) { // H2, H3
	for name, tc := range map[string]struct{ patch, reason string }{
		"flake":     {"--- a/flake.nix\n+++ b/flake.nix\n@@ -1 +1 @@\n-a\n+b\n", ReasonRecipeChangeRefused},
		"go.sum":    {"diff --git a/go.sum b/go.sum\n--- a/go.sum\n+++ b/go.sum\n@@ -1 +1 @@\n-a\n+b\n", ReasonRecipeChangeRefused},
		"nix dir":   {"--- a/nix/node-b.nix\n+++ b/nix/node-b.nix\n@@ -1 +1 @@\n-a\n+b\n", ReasonRecipeChangeRefused},
		"npm lock":  {"--- a/frontend/package-lock.json\n+++ b/frontend/package-lock.json\n@@ -1 +1 @@\n-a\n+b\n", ReasonRecipeChangeRefused},
		"rename in": {"diff --git a/x.go b/vendor/x.go\nrename from x.go\nrename to vendor/x.go\n", ReasonRecipeChangeRefused},
		"epoch":     {"--- a/internal/storeschema/schema.go\n+++ b/internal/storeschema/schema.go\n@@ -1 +1 @@\n-a\n+b\n", ReasonStoreEpochRefused},
		"escape":    {"--- a/../etc/passwd\n+++ b/../etc/passwd\n@@ -1 +1 @@\n-a\n+b\n", ReasonPatchUnreadable},
		"no paths":  {"hello\n", ReasonPatchUnreadable},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			svc := h.service()
			job, err := svc.Submit(h.request("computer-a", tc.patch))
			if err != nil {
				t.Fatal(err)
			}
			if job.State != StateFailed || job.Reason != tc.reason {
				t.Fatalf("job = %s %s %s, want failed %s", job.State, job.Reason, job.Detail, tc.reason)
			}
			if h.builds.Load() != 0 {
				t.Fatal("built a refused patch")
			}
		})
	}
}

func TestBaseMovedAndBaseCommitMismatchAreRefused(t *testing.T) { // H4
	h := newHarness(t)
	svc := h.service()
	moved := h.request("computer-a", goPatch)
	moved.BaseImageManifestDigest = strings.Repeat("e", 64)
	job, err := svc.Submit(moved)
	if err != nil || job.State != StateFailed || job.Reason != ReasonBaseMoved {
		t.Fatalf("moved: %+v %v", job, err)
	}
	lagging := h.request("computer-b", goPatch)
	lagging.BaseCommit = strings.Repeat("1", 40)
	job, err = svc.Submit(lagging)
	if err != nil || job.State != StateFailed || job.Reason != ReasonBaseCommitMismatch {
		t.Fatalf("lagging: %+v %v", job, err)
	}
	if h.builds.Load() != 0 {
		t.Fatal("built")
	}
}

func TestOneBuildAtATimeAndPressureGatesStart(t *testing.T) { // H5, H6
	h := newHarness(t)
	release := make(chan struct{})
	var running atomic.Int32
	var peak atomic.Int32
	h.build = func(ctx context.Context, req builder.Request) (*builder.ClosureResult, error) {
		n := running.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		defer running.Add(-1)
		<-release
		return fakeBuild(req)
	}
	h.cfg.QueueTimeout = 5 * time.Second
	svc := h.service()
	a, _ := svc.Submit(h.request("computer-a", goPatch))
	b, _ := svc.Submit(h.request("computer-b", goPatch))
	time.Sleep(50 * time.Millisecond)
	if job, _ := svc.Get("computer-b", b.JobID); job.State != StateQueued {
		t.Fatalf("second job state = %s while the first builds", job.State)
	}
	close(release)
	waitTerminal(t, svc, "computer-a", a.JobID)
	waitTerminal(t, svc, "computer-b", b.JobID)
	if peak.Load() != 1 {
		t.Fatalf("peak concurrent builds = %d", peak.Load())
	}

	// Under pressure a queued job never starts, and fails at its deadline.
	h2 := newHarness(t)
	h2.pressure.Store(Pressure{MemAvailableBytes: 1 << 30, MemorySomeAvg60: 30})
	svc2 := h2.service()
	job, _ := svc2.Submit(h2.request("computer-a", goPatch))
	job = waitTerminal(t, svc2, "computer-a", job.JobID)
	if job.State != StateFailed || job.Reason != ReasonNotAdmitted || h2.builds.Load() != 0 {
		t.Fatalf("pressured job = %+v, builds %d", job, h2.builds.Load())
	}
	if !strings.Contains(job.Detail, "memory") {
		t.Fatalf("not_admitted detail = %q", job.Detail)
	}
}

func TestBuildTimeoutAndFailureEndTheJob(t *testing.T) { // H7
	h := newHarness(t)
	h.cfg.BuildTimeout = 50 * time.Millisecond
	h.build = func(ctx context.Context, req builder.Request) (*builder.ClosureResult, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	svc := h.service()
	job, _ := svc.Submit(h.request("computer-a", goPatch))
	job = waitTerminal(t, svc, "computer-a", job.JobID)
	if job.State != StateFailed || job.Reason != ReasonBuildTimeout {
		t.Fatalf("timeout job = %+v", job)
	}

	h2 := newHarness(t)
	h2.build = func(ctx context.Context, req builder.Request) (*builder.ClosureResult, error) {
		return nil, errors.New("builder: apply source patch: git apply: patch does not apply")
	}
	svc2 := h2.service()
	job, _ = svc2.Submit(h2.request("computer-a", goPatch))
	job = waitTerminal(t, svc2, "computer-a", job.JobID)
	if job.State != StateFailed || job.Reason != ReasonBuildFailed || !strings.Contains(job.Detail, "does not apply") {
		t.Fatalf("failed job = %+v", job)
	}
}

func TestRetryNamesTheSameJobAndNeverRebuilds(t *testing.T) { // H8
	h := newHarness(t)
	svc := h.service()
	first, _ := svc.Submit(h.request("computer-a", goPatch))
	waitTerminal(t, svc, "computer-a", first.JobID)
	again, err := svc.Submit(h.request("computer-a", goPatch))
	if err != nil || again.JobID != first.JobID || again.State != StateDone {
		t.Fatalf("retry = %+v %v", again, err)
	}
	if h.builds.Load() != 1 {
		t.Fatalf("builds = %d, want 1", h.builds.Load())
	}
	other := goPatch + "+// more\n"
	otherReq := h.request("computer-a", other)
	otherReq.OperationID = "selfdev-op-other"
	otherJob, _ := svc.Submit(otherReq)
	if otherJob.JobID == first.JobID {
		t.Fatal("a different patch reused a job")
	}
}

func TestRestartFailsJobsInFlight(t *testing.T) { // H9
	h := newHarness(t)
	block := make(chan struct{})
	h.build = func(ctx context.Context, req builder.Request) (*builder.ClosureResult, error) {
		<-block
		return nil, errors.New("killed")
	}
	svc, err := New(h.cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	svc.Start(ctx)
	a, _ := svc.Submit(h.request("computer-a", goPatch))
	b, _ := svc.Submit(h.request("computer-b", goPatch))
	time.Sleep(50 * time.Millisecond)
	// Simulate a crash: the process is gone; a new service reads the records.
	restarted, err := New(h.cfg)
	if err != nil {
		t.Fatal(err)
	}
	for computer, id := range map[string]string{"computer-a": a.JobID, "computer-b": b.JobID} {
		job, ok := restarted.Get(computer, id)
		if !ok || job.State != StateFailed || job.Reason != ReasonInterruptedByRestart {
			t.Fatalf("after restart %s = %+v", computer, job)
		}
	}
	cancel()
	close(block)
	svc.Wait()
}

func TestJobsAreComputerBound(t *testing.T) { // H13
	h := newHarness(t)
	svc := h.service()
	job, _ := svc.Submit(h.request("computer-a", goPatch))
	if _, ok := svc.Get("computer-b", job.JobID); ok {
		t.Fatal("another computer read the job")
	}
}

func TestQueueIsBounded(t *testing.T) { // H14
	h := newHarness(t)
	release := make(chan struct{})
	defer close(release)
	h.build = func(ctx context.Context, req builder.Request) (*builder.ClosureResult, error) {
		<-release
		return fakeBuild(req)
	}
	h.cfg.MaxQueue = 2
	h.cfg.QueueTimeout = 5 * time.Second
	svc := h.service()
	a, _ := svc.Submit(h.request("computer-a", goPatch))
	if a.State != StateQueued && a.State != StateRunning {
		t.Fatalf("a = %+v", a)
	}
	// One active job per computer.
	second := h.request("computer-a", goPatch)
	second.OperationID = "selfdev-op-second"
	busy, _ := svc.Submit(second)
	if busy.State != StateFailed || busy.Reason != ReasonComputerBusy {
		t.Fatalf("busy = %+v", busy)
	}
	svc.Submit(h.request("computer-b", goPatch))
	full, _ := svc.Submit(h.request("computer-c", goPatch))
	if full.State != StateFailed || full.Reason != ReasonQueueFull {
		t.Fatalf("full = %+v", full)
	}
}

func TestJobRecordIsTheWireShape(t *testing.T) {
	h := newHarness(t)
	svc := h.service()
	job, _ := svc.Submit(h.request("computer-a", goPatch))
	job = waitTerminal(t, svc, "computer-a", job.JobID)
	raw, err := json.Marshal(job)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "patch_base64") {
		t.Fatal("the job record carries the patch bytes")
	}
}
