package hostbuild

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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
// docs/archive/reports/p5-host-build-layered-apply-2026-10-15.md.

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
	manifest, err := os.ReadFile(req.BaseManifestPath)
	if err != nil {
		return nil, err
	}
	res := &builder.ClosureResult{
		Base:                  builder.BaseIdentity{GuestImageManifestDigest: sha256Hex(manifest), StoreDiskSHA256: "d"},
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
		// H15: git's reading of the patch, not a text parse: any first
		// segment, no git header, C-quoted and escaped names.
		"any first segment": {"diff --git x/flake.nix y/flake.nix\n--- x/flake.nix\n+++ y/flake.nix\n@@ -1 +1 @@\n-a\n+b\n", ReasonRecipeChangeRefused},
		"no git header":     {"--- foo/flake.nix\n+++ bar/flake.nix\n@@ -1 +1 @@\n-a\n+b\n", ReasonRecipeChangeRefused},
		"quoted names":      {"diff --git \"a/nix/a.nix\" \"b/nix/a.nix\"\n--- \"a/nix/a.nix\"\n+++ \"b/nix/a.nix\"\n@@ -1 +1 @@\n-a\n+b\n", ReasonRecipeChangeRefused},
		"octal escape":      {"diff --git \"a/fl\\141ke.nix\" \"b/fl\\141ke.nix\"\n--- \"a/fl\\141ke.nix\"\n+++ \"b/fl\\141ke.nix\"\n@@ -1 +1 @@\n-a\n+b\n", ReasonRecipeChangeRefused},
		"nix anywhere":      {"--- a/internal/x/default.nix\n+++ b/internal/x/default.nix\n@@ -1 +1 @@\n-a\n+b\n", ReasonRecipeChangeRefused},
		"sub flake lock":    {"--- a/tools/flake.lock\n+++ b/tools/flake.lock\n@@ -1 +1 @@\n-a\n+b\n", ReasonRecipeChangeRefused},
		// H16: git metadata that changes what the tree means.
		"gitattributes": {"diff --git a/sub/.gitattributes b/sub/.gitattributes\nnew file mode 100644\n--- /dev/null\n+++ b/sub/.gitattributes\n@@ -0,0 +1 @@\n+* filter=x\n", ReasonPatchTreeRefused},
		"gitmodules":    {"diff --git a/.gitmodules b/.gitmodules\nnew file mode 100644\n--- /dev/null\n+++ b/.gitmodules\n@@ -0,0 +1 @@\n+[submodule]\n", ReasonPatchTreeRefused},
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
		// H24: an interrupted job's stored patch is removed.
		if _, err := os.Stat(filepath.Join(h.cfg.StateDir, "jobs", id+".patch")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("interrupted job %s kept its patch: %v", id, err)
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
	// One active job per computer. H20: a transient refusal records no job,
	// so the retry of the same request can still build.
	second := h.request("computer-a", goPatch)
	second.OperationID = "selfdev-op-second"
	_, err := svc.Submit(second)
	var refusal *RefusalError
	if !errors.As(err, &refusal) || refusal.Reason != ReasonComputerBusy || !refusal.Retry {
		t.Fatalf("busy = %v", err)
	}
	if id, _ := second.JobID(); func() bool { _, ok := svc.Get("computer-a", id); return ok }() {
		t.Fatal("a computer_busy refusal recorded a job under the retry's id")
	}
	svc.Submit(h.request("computer-b", goPatch))
	full := h.request("computer-c", goPatch)
	_, err = svc.Submit(full)
	if !errors.As(err, &refusal) || refusal.Reason != ReasonQueueFull || !refusal.Retry {
		t.Fatalf("full = %v", err)
	}
	if id, _ := full.JobID(); func() bool { _, ok := svc.Get("computer-c", id); return ok }() {
		t.Fatal("a queue_full refusal recorded a job under the retry's id")
	}
}

// H20: once the queue drains, the retry of a refused request builds.
func TestTransientRefusalRetryBuilds(t *testing.T) {
	h := newHarness(t)
	release := make(chan struct{})
	h.build = func(ctx context.Context, req builder.Request) (*builder.ClosureResult, error) {
		<-release
		return fakeBuild(req)
	}
	h.cfg.QueueTimeout = 5 * time.Second
	svc := h.service()
	first, _ := svc.Submit(h.request("computer-a", goPatch))
	second := h.request("computer-a", goPatch)
	second.OperationID = "selfdev-op-second"
	if _, err := svc.Submit(second); err == nil {
		t.Fatal("second build for a busy computer was accepted")
	}
	close(release)
	waitTerminal(t, svc, "computer-a", first.JobID)
	job, err := svc.Submit(second)
	if err != nil {
		t.Fatal(err)
	}
	if job = waitTerminal(t, svc, "computer-a", job.JobID); job.State != StateDone {
		t.Fatalf("retry after the queue drained = %+v", job)
	}
}

// H16, H15 authority: the builder hands the committed tree's changes to the
// host's admission; links, git metadata, the recipe and the store schema are
// refused there with the job's reason codes.
func TestAdmitPatchedTree(t *testing.T) {
	for name, tc := range map[string]struct {
		change builder.PatchedChange
		reason string
	}{
		"go file":        {builder.PatchedChange{Path: "internal/x/x.go", OldMode: "100644", NewMode: "100644"}, ""},
		"new go file":    {builder.PatchedChange{Path: "internal/x/y.go", OldMode: "000000", NewMode: "100644"}, ""},
		"symlink":        {builder.PatchedChange{Path: "internal/x/link", OldMode: "000000", NewMode: "120000"}, ReasonPatchTreeRefused},
		"gitlink":        {builder.PatchedChange{Path: "vendor2", OldMode: "000000", NewMode: "160000"}, ReasonPatchTreeRefused},
		"was a symlink":  {builder.PatchedChange{Path: "internal/x/link", OldMode: "120000", NewMode: "100644"}, ReasonPatchTreeRefused},
		"flake":          {builder.PatchedChange{Path: "flake.nix", OldMode: "100644", NewMode: "100644"}, ReasonRecipeChangeRefused},
		"nix dir":        {builder.PatchedChange{Path: "nix/a.nix", OldMode: "100644", NewMode: "100644"}, ReasonRecipeChangeRefused},
		"store schema":   {builder.PatchedChange{Path: "internal/storeschema/v.go", OldMode: "100644", NewMode: "100644"}, ReasonStoreEpochRefused},
		"gitattributes":  {builder.PatchedChange{Path: "a/.gitattributes", OldMode: "000000", NewMode: "100644"}, ReasonPatchTreeRefused},
		"executable bit": {builder.PatchedChange{Path: "scripts/x.sh", OldMode: "100644", NewMode: "100755"}, ""},
	} {
		err := AdmitPatchedTree([]builder.PatchedChange{tc.change})
		var refusal *RefusalError
		switch {
		case tc.reason == "" && err != nil:
			t.Errorf("%s: refused %v", name, err)
		case tc.reason != "" && (!errors.As(err, &refusal) || refusal.Reason != tc.reason):
			t.Errorf("%s: err = %v, want %s", name, err, tc.reason)
		}
	}
}

// H15: a refusal the builder's in-clone admission raises ends the job with
// its reason code, not build_failed.
func TestTreeRefusalInTheBuildEndsTheJobWithItsReason(t *testing.T) {
	h := newHarness(t)
	h.build = func(ctx context.Context, req builder.Request) (*builder.ClosureResult, error) {
		if req.AdmitPatchedTree == nil {
			return nil, errors.New("no admission wired")
		}
		err := req.AdmitPatchedTree([]builder.PatchedChange{{Path: "flake.nix", OldMode: "100644", NewMode: "100644"}})
		return nil, fmt.Errorf("builder: apply source patch: %w", err)
	}
	svc := h.service()
	job, _ := svc.Submit(h.request("computer-a", goPatch))
	job = waitTerminal(t, svc, "computer-a", job.JobID)
	if job.State != StateFailed || job.Reason != ReasonRecipeChangeRefused {
		t.Fatalf("job = %+v", job)
	}
}

// H19: a deploy that changes the host's base image while a job waits or
// builds fails the job base_moved; it never builds against, or records, a
// base the computer did not boot.
func TestBaseMovedWhileQueuedOrBuilding(t *testing.T) {
	h := newHarness(t)
	h.pressure.Store(Pressure{MemAvailableBytes: 1 << 30})
	h.cfg.QueueTimeout = 5 * time.Second
	svc := h.service()
	job, err := svc.Submit(h.request("computer-a", goPatch))
	if err != nil || job.Terminal() {
		t.Fatalf("submit = %+v %v", job, err)
	}
	if err := os.WriteFile(h.cfg.BaseManifestPath, []byte("contract=choir-guest-image-v1\nbuild_commit="+testBaseCommit+"\nnew=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.pressure.Store(Pressure{MemAvailableBytes: 64 << 30})
	job = waitTerminal(t, svc, "computer-a", job.JobID)
	if job.State != StateFailed || job.Reason != ReasonBaseMoved || h.builds.Load() != 0 {
		t.Fatalf("queued across a deploy = %+v, builds %d", job, h.builds.Load())
	}

	h2 := newHarness(t)
	h2.build = func(ctx context.Context, req builder.Request) (*builder.ClosureResult, error) {
		if err := os.WriteFile(req.BaseManifestPath, []byte("contract=choir-guest-image-v1\nbuild_commit="+testBaseCommit+"\nnew=2\n"), 0o644); err != nil {
			return nil, err
		}
		return fakeBuild(req)
	}
	svc2 := h2.service()
	job, _ = svc2.Submit(h2.request("computer-a", goPatch))
	job = waitTerminal(t, svc2, "computer-a", job.JobID)
	if job.State != StateFailed || job.Reason != ReasonBaseMoved || job.Result != nil {
		t.Fatalf("deploy during the build = %+v", job)
	}
}

// H21: one computer's builds are bounded per day; a refused request records
// no job.
func TestBuildsPerComputerAreBounded(t *testing.T) {
	h := newHarness(t)
	h.cfg.MaxBuildsPerComputerPerDay = 2
	svc := h.service()
	for i := 0; i < 2; i++ {
		req := h.request("computer-a", goPatch)
		req.OperationID = fmt.Sprintf("selfdev-op-%d", i)
		job, err := svc.Submit(req)
		if err != nil {
			t.Fatal(err)
		}
		waitTerminal(t, svc, "computer-a", job.JobID)
	}
	over := h.request("computer-a", goPatch)
	over.OperationID = "selfdev-op-over"
	_, err := svc.Submit(over)
	var refusal *RefusalError
	if !errors.As(err, &refusal) || refusal.Reason != ReasonBuildQuotaExhausted || refusal.Retry {
		t.Fatalf("over quota = %v", err)
	}
	if id, _ := over.JobID(); func() bool { _, ok := svc.Get("computer-a", id); return ok }() {
		t.Fatal("an over-quota request recorded a job")
	}
	// Another computer is not bounded by this one's builds.
	if job, err := svc.Submit(h.request("computer-b", goPatch)); err != nil || job.Terminal() && job.State != StateDone {
		t.Fatalf("other computer = %+v %v", job, err)
	}
	// A retry of a request already recorded still answers its job.
	again := h.request("computer-a", goPatch)
	again.OperationID = "selfdev-op-0"
	if _, err := svc.Submit(again); err != nil {
		t.Fatalf("retry of a recorded job = %v", err)
	}
}

// H22: terminal job records leave after their retention, on disk and in
// memory, and the record count is bounded.
func TestJobRecordsArePruned(t *testing.T) {
	h := newHarness(t)
	now := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	var clock atomic.Value
	clock.Store(now)
	h.cfg.Now = func() time.Time { return clock.Load().(time.Time) }
	h.cfg.MaxBuildsPerComputerPerDay = 100
	svc := h.service()
	refused, _ := svc.Submit(h.request("computer-a", "--- a/flake.nix\n+++ b/flake.nix\n@@ -1 +1 @@\n-a\n+b\n"))
	if refused.Reason != ReasonRecipeChangeRefused {
		t.Fatalf("refused = %+v", refused)
	}
	doneReq := h.request("computer-b", goPatch)
	done, _ := svc.Submit(doneReq)
	waitTerminal(t, svc, "computer-b", done.JobID)

	clock.Store(now.Add(25 * time.Hour))
	svc.Submit(h.request("computer-c", goPatch))
	if _, ok := svc.Get("computer-a", refused.JobID); ok {
		t.Fatal("a refused record outlived its retention")
	}
	if _, err := os.Stat(filepath.Join(h.cfg.StateDir, "jobs", refused.JobID+".json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a pruned record stayed on disk: %v", err)
	}
	if _, ok := svc.Get("computer-b", done.JobID); !ok {
		t.Fatal("a done record was pruned within its retention")
	}
	clock.Store(now.Add(8 * 24 * time.Hour))
	svc.Submit(h.request("computer-d", goPatch))
	if _, ok := svc.Get("computer-b", done.JobID); ok {
		t.Fatal("a done record outlived its retention")
	}

	// The record count is bounded: the oldest terminal records leave first.
	h2 := newHarness(t)
	h2.cfg.MaxJobRecords = 3
	h2.cfg.MaxBuildsPerComputerPerDay = 100
	svc2 := h2.service()
	var ids []string
	for i := 0; i < 5; i++ {
		req := h2.request("computer-a", "--- a/flake.nix\n+++ b/flake.nix\n@@ -1 +1 @@\n-a\n+b\n")
		req.OperationID = fmt.Sprintf("selfdev-op-%d", i)
		job, _ := svc2.Submit(req)
		ids = append(ids, job.JobID)
		time.Sleep(2 * time.Millisecond)
	}
	if n := svc2.recordCount(); n > 3 {
		t.Fatalf("records = %d, want at most 3", n)
	}
	if _, ok := svc2.Get("computer-a", ids[4]); !ok {
		t.Fatal("the newest record was pruned")
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
