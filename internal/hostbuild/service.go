package hostbuild

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/builder"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// BuildFunc builds one request; builder.Build in production.
type BuildFunc func(ctx context.Context, req builder.Request) (*builder.ClosureResult, error)

// Pressure is one host memory observation for admission.
type Pressure struct {
	MemAvailableBytes uint64
	MemorySomeAvg60   float64
}

// Sampler reads host memory pressure.
type Sampler func() (Pressure, error)

// Signer signs a done job's result under the platform-control key.
type Signer func(computerID, jobID, resultDigest string, at time.Time) (selfdevprotocol.AuthorityReceipt, error)

// BlobSink stores a file in the platform-update blob namespace under its
// sha256, verifying the digest as it copies.
type BlobSink func(digest, path string) error

// Config configures the service. Zero values take the defaults below.
type Config struct {
	StateDir          string
	SourceDir         string
	Installable       string
	BaseManifestPath  string
	BaseStoreDiskPath string
	// BaseStoreDiskBasePath is a layered image's base layer; empty for a
	// single image (BaseStoreDiskPath is then the whole storedisk.erofs).
	BaseStoreDiskBasePath string

	MaxQueue           int
	BuildTimeout       time.Duration
	QueueTimeout       time.Duration
	AdmitPoll          time.Duration
	MemoryFloorBytes   uint64
	MaxMemorySomeAvg60 float64

	// MaxBuildsPerComputerPerDay bounds the jobs one computer records in a
	// rolling 24 hours (H21); each done build keeps a closure in the blob
	// store.
	MaxBuildsPerComputerPerDay int
	// FailedRetention and DoneRetention are how long terminal job records
	// stay; MaxJobRecords bounds the records kept at all (H22).
	FailedRetention time.Duration
	DoneRetention   time.Duration
	MaxJobRecords   int

	Build  BuildFunc
	Sample Sampler
	Sign   Signer
	Store  BlobSink
	Now    func() time.Time
}

const (
	defaultMaxQueue           = 4
	defaultBuildTimeout       = 40 * time.Minute
	defaultQueueTimeout       = 20 * time.Minute
	defaultAdmitPoll          = 15 * time.Second
	defaultMemoryFloorBytes   = 6 << 30
	defaultMaxMemorySomeAvg60 = 10
	defaultMaxBuildsPerDay    = 8
	defaultFailedRetention    = 24 * time.Hour
	defaultDoneRetention      = 7 * 24 * time.Hour
	defaultMaxJobRecords      = 512
	quotaWindow               = 24 * time.Hour
	maxDetailBytes            = 2048
)

// Service is the host build service. One worker runs one build at a time.
type Service struct {
	cfg  Config
	mu   sync.Mutex
	jobs map[string]*Job
	wake chan struct{}
	done chan struct{}
}

// New loads the durable job records and closes every job a previous process
// left queued or running: a restart ends work, it never resumes it (H9).
func New(cfg Config) (*Service, error) {
	if strings.TrimSpace(cfg.StateDir) == "" || strings.TrimSpace(cfg.BaseManifestPath) == "" || strings.TrimSpace(cfg.BaseStoreDiskPath) == "" || strings.TrimSpace(cfg.SourceDir) == "" {
		return nil, fmt.Errorf("host build: state dir, source dir, and base image paths are required")
	}
	if cfg.Sign == nil || cfg.Store == nil {
		return nil, fmt.Errorf("host build: signer and blob store are required")
	}
	if cfg.Installable == "" {
		cfg.Installable = ".#autoputer"
	}
	if cfg.MaxQueue <= 0 {
		cfg.MaxQueue = defaultMaxQueue
	}
	if cfg.BuildTimeout <= 0 {
		cfg.BuildTimeout = defaultBuildTimeout
	}
	if cfg.QueueTimeout <= 0 {
		cfg.QueueTimeout = defaultQueueTimeout
	}
	if cfg.AdmitPoll <= 0 {
		cfg.AdmitPoll = defaultAdmitPoll
	}
	if cfg.MemoryFloorBytes == 0 {
		cfg.MemoryFloorBytes = defaultMemoryFloorBytes
	}
	if cfg.MaxMemorySomeAvg60 <= 0 {
		cfg.MaxMemorySomeAvg60 = defaultMaxMemorySomeAvg60
	}
	if cfg.MaxBuildsPerComputerPerDay <= 0 {
		cfg.MaxBuildsPerComputerPerDay = defaultMaxBuildsPerDay
	}
	if cfg.FailedRetention <= 0 {
		cfg.FailedRetention = defaultFailedRetention
	}
	if cfg.DoneRetention <= 0 {
		cfg.DoneRetention = defaultDoneRetention
	}
	if cfg.MaxJobRecords <= 0 {
		cfg.MaxJobRecords = defaultMaxJobRecords
	}
	if cfg.Build == nil {
		cfg.Build = builder.Build
	}
	if cfg.Sample == nil {
		cfg.Sample = SampleHostPressure
	}
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	for _, dir := range []string{"jobs", "work"} {
		if err := os.MkdirAll(filepath.Join(cfg.StateDir, dir), 0o700); err != nil {
			return nil, fmt.Errorf("host build: state dir: %w", err)
		}
	}
	s := &Service{cfg: cfg, jobs: map[string]*Job{}, wake: make(chan struct{}, 1)}
	entries, err := os.ReadDir(filepath.Join(cfg.StateDir, "jobs"))
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(cfg.StateDir, "jobs", entry.Name()))
		if err != nil {
			return nil, err
		}
		var job Job
		if err := json.Unmarshal(raw, &job); err != nil || !computerevent.IsSHA256(job.JobID) {
			log.Printf("host build: skip unreadable job record %s", entry.Name())
			continue
		}
		if !job.Terminal() {
			s.finish(&job, StateFailed, ReasonInterruptedByRestart, "the host build service restarted while this job was "+job.State+"; a restart ends a build, it never resumes it")
			_ = os.RemoveAll(filepath.Join(cfg.StateDir, "work", job.JobID))
			_ = os.Remove(s.patchPath(job.JobID)) // H24
			if err := s.persist(&job); err != nil {
				return nil, err
			}
		}
		s.jobs[job.JobID] = &job
	}
	s.prune(s.cfg.Now())
	return s, nil
}

// recordCount is the number of job records held (tests).
func (s *Service) recordCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.jobs)
}

// prune drops terminal job records past their retention, then the oldest
// terminal records beyond MaxJobRecords, from memory and disk (H22). The
// caller holds s.mu (or owns s exclusively).
func (s *Service) prune(now time.Time) {
	var terminal []*Job
	for id, job := range s.jobs {
		if !job.Terminal() {
			continue
		}
		retention := s.cfg.FailedRetention
		if job.State == StateDone {
			retention = s.cfg.DoneRetention
		}
		finished := job.CreatedAt
		if job.FinishedAt != nil {
			finished = *job.FinishedAt
		}
		if now.Sub(finished) > retention {
			s.drop(id)
			continue
		}
		terminal = append(terminal, job)
	}
	if excess := len(s.jobs) - s.cfg.MaxJobRecords; excess > 0 {
		sort.Slice(terminal, func(i, j int) bool {
			a, b := terminal[i].CreatedAt, terminal[j].CreatedAt
			if terminal[i].FinishedAt != nil {
				a = *terminal[i].FinishedAt
			}
			if terminal[j].FinishedAt != nil {
				b = *terminal[j].FinishedAt
			}
			return a.Before(b)
		})
		for i := 0; i < excess && i < len(terminal); i++ {
			s.drop(terminal[i].JobID)
		}
	}
}

func (s *Service) drop(jobID string) {
	delete(s.jobs, jobID)
	_ = os.Remove(filepath.Join(s.cfg.StateDir, "jobs", jobID+".json"))
	_ = os.Remove(s.patchPath(jobID))
}

// Start runs the single build worker until ctx ends.
func (s *Service) Start(ctx context.Context) {
	s.mu.Lock()
	if s.done != nil {
		s.mu.Unlock()
		return
	}
	s.done = make(chan struct{})
	s.mu.Unlock()
	go func() {
		defer close(s.done)
		s.worker(ctx)
	}()
}

// Wait blocks until a started worker has returned (after its ctx ended and
// any build in flight finished).
func (s *Service) Wait() {
	s.mu.Lock()
	done := s.done
	s.mu.Unlock()
	if done != nil {
		<-done
	}
}

// Get returns a job only to the computer it belongs to (H13).
func (s *Service) Get(computerID, jobID string) (Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[jobID]
	if !ok || job.ComputerID != computerID {
		return Job{}, false
	}
	return *job, true
}

// Submit records a job for the request, or returns the job the same request
// already names. An error means the request itself is invalid (no job is
// recorded); every refusal of a valid request is a failed job with a reason.
func (s *Service) Submit(req Request) (Job, error) {
	if err := req.Validate(); err != nil {
		return Job{}, err
	}
	jobID, err := req.JobID()
	if err != nil {
		return Job{}, err
	}
	s.mu.Lock()
	if existing, ok := s.jobs[jobID]; ok {
		job := *existing
		s.mu.Unlock()
		return job, nil
	}
	// H21: one computer records a bounded number of jobs a day, refused ones
	// included; checked before the patch is read (and again under the lock
	// that records the job). The refusal records nothing.
	if err := s.quotaRefusal(req.ComputerID, s.cfg.Now()); err != nil {
		s.mu.Unlock()
		return Job{}, err
	}
	s.mu.Unlock()

	patch, err := base64.StdEncoding.DecodeString(req.PatchBase64)
	if err != nil {
		return Job{}, fmt.Errorf("host build: %s: patch does not decode", ReasonPatchDigestMismatch)
	}
	// H1: a patch that does not hash to its declared sha records no job, so
	// a corrupt transfer cannot poison the retry that names the same job.
	if sha256Hex(patch) != req.SourcePatchSHA256 {
		return Job{}, fmt.Errorf("host build: %s: patch bytes do not hash to source_patch_sha256", ReasonPatchDigestMismatch)
	}
	now := s.cfg.Now()
	job := &Job{
		JobID: jobID, ComputerID: req.ComputerID, OperationID: req.OperationID, BundleDigest: req.BundleDigest,
		BaseCommit: req.BaseCommit, SourcePatchSHA256: req.SourcePatchSHA256,
		BaseImageManifestDigest: req.BaseImageManifestDigest,
		State:                   StateQueued, CreatedAt: now, QueueDeadline: now.Add(s.cfg.QueueTimeout),
	}
	if reason, detail := s.refuse(req, patch); reason != "" {
		s.finish(job, StateFailed, reason, detail)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.jobs[jobID]; ok {
		return *existing, nil
	}
	s.prune(now)
	if err := s.quotaRefusal(req.ComputerID, now); err != nil {
		return Job{}, err
	}
	// H20: a full queue or a busy computer is transient. Nothing is recorded
	// under the deterministic job id, so the retry of this request builds.
	if !job.Terminal() {
		active := 0
		for _, other := range s.jobs {
			if other.Terminal() {
				continue
			}
			active++
			if other.ComputerID == req.ComputerID {
				return Job{}, &RefusalError{Reason: ReasonComputerBusy, Retry: true, Detail: "this computer already has host build " + other.JobID + " " + other.State + "; one build per computer at a time"}
			}
		}
		if active >= s.cfg.MaxQueue {
			return Job{}, &RefusalError{Reason: ReasonQueueFull, Retry: true, Detail: fmt.Sprintf("%d host builds are already queued or running", active)}
		}
	}
	if !job.Terminal() {
		if err := writeFileAtomic(s.patchPath(jobID), patch, 0o600); err != nil {
			return Job{}, fmt.Errorf("host build: store patch: %w", err)
		}
	}
	if err := s.persist(job); err != nil {
		return Job{}, err
	}
	s.jobs[jobID] = job
	s.prune(now)
	if !job.Terminal() {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
	return *job, nil
}

// refuse returns a reason when a valid request must not be built (H2-H4).
func (s *Service) refuse(req Request, patch []byte) (string, string) {
	if len(patch) > MaxPatchBytes {
		return ReasonPatchTooLarge, fmt.Sprintf("the patch is %d bytes; the host builds patches up to %d", len(patch), MaxPatchBytes)
	}
	// The early check, on git's own reading of the patch (H15). The
	// authority is AdmitPatchedTree on the tree the builder commits.
	paths, err := PatchPaths(patch)
	if err != nil {
		return ReasonPatchUnreadable, err.Error()
	}
	if reason, detail := RefusePatchPaths(paths); reason != "" {
		return reason, detail
	}
	return s.baseRefusal(req.BaseImageManifestDigest, req.BaseCommit)
}

// baseRefusal refuses a build whose booted base is not the host's deployed
// guest image, or whose patch base is not that image's commit (H4, H19).
func (s *Service) baseRefusal(bootedBase, baseCommit string) (string, string) {
	raw, err := os.ReadFile(s.cfg.BaseManifestPath)
	if err != nil {
		return ReasonBaseMoved, "the host cannot read its deployed guest image manifest"
	}
	if hostBase := sha256Hex(raw); hostBase != bootedBase {
		return ReasonBaseMoved, "the computer booted base " + bootedBase + "; the host's deployed guest image is " + hostBase + " (a deploy landed since the computer booted)"
	}
	if commit := manifestBuildCommit(raw); commit != baseCommit {
		return ReasonBaseCommitMismatch, "the patch was authored against " + baseCommit + "; the booted image was built from " + commit
	}
	return "", ""
}

// quotaRefusal is H21's bound; the caller holds s.mu.
func (s *Service) quotaRefusal(computerID string, now time.Time) error {
	recent := 0
	for _, other := range s.jobs {
		if other.ComputerID == computerID && now.Sub(other.CreatedAt) < quotaWindow {
			recent++
		}
	}
	if recent >= s.cfg.MaxBuildsPerComputerPerDay {
		return &RefusalError{Reason: ReasonBuildQuotaExhausted, Detail: fmt.Sprintf("this computer recorded %d host builds in the last 24 hours; the bound is %d", recent, s.cfg.MaxBuildsPerComputerPerDay)}
	}
	return nil
}

func (s *Service) worker(ctx context.Context) {
	for {
		job, ok := s.nextQueued()
		if !ok {
			select {
			case <-ctx.Done():
				return
			case <-s.wake:
			case <-time.After(s.cfg.AdmitPoll):
			}
			continue
		}
		if !s.admit(ctx, job) {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		s.run(ctx, job)
	}
}

func (s *Service) nextQueued() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var queued []*Job
	for _, job := range s.jobs {
		if job.State == StateQueued {
			queued = append(queued, job)
		}
	}
	if len(queued) == 0 {
		return "", false
	}
	sort.Slice(queued, func(i, j int) bool {
		if queued[i].CreatedAt.Equal(queued[j].CreatedAt) {
			return queued[i].JobID < queued[j].JobID
		}
		return queued[i].CreatedAt.Before(queued[j].CreatedAt)
	})
	return queued[0].JobID, true
}

// admit waits until host memory admits a build or the job's queue deadline
// passes (H5, H6). It moves the job to running or failed.
func (s *Service) admit(ctx context.Context, jobID string) bool {
	for {
		pressure, err := s.cfg.Sample()
		var why string
		switch {
		case err != nil:
			why = "host memory pressure is unreadable: " + err.Error()
		case pressure.MemAvailableBytes < s.cfg.MemoryFloorBytes:
			why = fmt.Sprintf("host memory available %d bytes is under the floor %d", pressure.MemAvailableBytes, s.cfg.MemoryFloorBytes)
		case pressure.MemorySomeAvg60 > s.cfg.MaxMemorySomeAvg60:
			why = fmt.Sprintf("host memory some avg60 %.2f is over %.2f", pressure.MemorySomeAvg60, s.cfg.MaxMemorySomeAvg60)
		}
		s.mu.Lock()
		job := s.jobs[jobID]
		now := s.cfg.Now()
		if why == "" {
			job.State = StateRunning
			job.StartedAt = &now
			job.Admission = fmt.Sprintf("admitted: memory available %d bytes, some avg60 %.2f", pressure.MemAvailableBytes, pressure.MemorySomeAvg60)
			err := s.persist(job)
			s.mu.Unlock()
			if err != nil {
				log.Printf("host build: persist %s: %v", jobID, err)
			}
			return true
		}
		if !now.Before(job.QueueDeadline) {
			s.finish(job, StateFailed, ReasonNotAdmitted, "not admitted before the queue deadline: "+why)
			_ = s.persist(job)
			_ = os.Remove(s.patchPath(jobID))
			s.mu.Unlock()
			return false
		}
		job.Admission = "waiting: " + why
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return false
		case <-time.After(s.cfg.AdmitPoll):
		}
	}
}

func (s *Service) run(ctx context.Context, jobID string) {
	s.mu.Lock()
	job := *s.jobs[jobID]
	s.mu.Unlock()
	workDir := filepath.Join(s.cfg.StateDir, "work", jobID)
	result, reason, detail := s.build(ctx, job, workDir)
	// The outputs are in the blob store; drop the work dir (and its nix
	// result link, a GC root) and the patch before the job reads terminal.
	_ = os.RemoveAll(workDir)
	_ = os.Remove(s.patchPath(jobID))
	s.mu.Lock()
	defer s.mu.Unlock()
	live := s.jobs[jobID]
	if reason != "" {
		s.finish(live, StateFailed, reason, detail)
	} else {
		live.Result = result
		s.finish(live, StateDone, "", "")
		digest, err := result.Digest()
		if err == nil {
			var receipt selfdevprotocol.AuthorityReceipt
			receipt, err = s.cfg.Sign(live.ComputerID, live.JobID, digest, s.cfg.Now())
			live.Receipt = &receipt
		}
		if err != nil {
			live.Result, live.Receipt = nil, nil
			s.finish(live, StateFailed, ReasonBuildFailed, "sign the host build result: "+err.Error())
		}
	}
	if err := s.persist(live); err != nil {
		log.Printf("host build: persist %s: %v", jobID, err)
	}
}

func (s *Service) build(ctx context.Context, job Job, workDir string) (*Result, string, string) {
	// H19: a deploy may have landed while the job waited; build only against
	// the base the computer booted.
	if reason, detail := s.baseRefusal(job.BaseImageManifestDigest, job.BaseCommit); reason != "" {
		return nil, reason, detail
	}
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return nil, ReasonBuildFailed, "work dir: " + err.Error()
	}
	buildCtx, cancel := context.WithTimeout(ctx, s.cfg.BuildTimeout)
	defer cancel()
	res, err := s.cfg.Build(buildCtx, builder.Request{
		Installable:           s.cfg.Installable,
		ResultLink:            filepath.Join(workDir, "result"),
		BaseManifestPath:      s.cfg.BaseManifestPath,
		BaseStoreDiskPath:     s.cfg.BaseStoreDiskPath,
		BaseStoreDiskBasePath: s.cfg.BaseStoreDiskBasePath,
		OutDir:                filepath.Join(workDir, "out"),
		SourceDir:             s.cfg.SourceDir,
		SourcePatch:           s.patchPath(job.JobID),
		SourcePatchBaseCommit: job.BaseCommit,
		BuildTimeout:          s.cfg.BuildTimeout,
		AdmitPatchedTree:      AdmitPatchedTree,
	})
	if err != nil {
		var refusal *RefusalError
		var treeRefusal *builder.PatchRefusal
		switch {
		case errors.As(err, &refusal):
			return nil, refusal.Reason, refusal.Detail
		case errors.As(err, &treeRefusal):
			return nil, ReasonPatchTreeRefused, treeRefusal.Error()
		case errors.Is(buildCtx.Err(), context.DeadlineExceeded):
			return nil, ReasonBuildTimeout, fmt.Sprintf("the build ran past %s", s.cfg.BuildTimeout)
		}
		return nil, ReasonBuildFailed, truncate(err.Error())
	}
	if res.SourcePatchSHA256 != job.SourcePatchSHA256 || res.SourcePatchBaseCommit != job.BaseCommit {
		return nil, ReasonBuildFailed, "the builder built a different patch or base than the job names"
	}
	if res.Base.GuestImageManifestDigest != job.BaseImageManifestDigest {
		return nil, ReasonBaseMoved, "the builder resolved base " + res.Base.GuestImageManifestDigest + "; the computer booted " + job.BaseImageManifestDigest + " (a deploy landed during the build)"
	}
	if !computerevent.IsSHA256(res.ExportedDigest) || strings.TrimSpace(res.RuntimePath) == "" {
		return nil, ReasonBuildFailed, "the builder returned no closure"
	}
	if err := s.cfg.Store(res.ExportedDigest, res.ExportedPath); err != nil {
		return nil, ReasonBuildFailed, "store the closure: " + err.Error()
	}
	receiptPath := filepath.Join(workDir, "out", "builder-receipt.json")
	receiptRaw, err := os.ReadFile(receiptPath)
	if err != nil {
		return nil, ReasonBuildFailed, "read the builder receipt: " + err.Error()
	}
	receiptDigest := sha256Hex(receiptRaw)
	if err := s.cfg.Store(receiptDigest, receiptPath); err != nil {
		return nil, ReasonBuildFailed, "store the builder receipt: " + err.Error()
	}
	info, err := os.Stat(res.ExportedPath)
	if err != nil {
		return nil, ReasonBuildFailed, "stat the closure: " + err.Error()
	}
	return &Result{
		JobID: job.JobID, ComputerID: job.ComputerID, OperationID: job.OperationID, BundleDigest: job.BundleDigest,
		BaseCommit: job.BaseCommit, SourcePatchSHA256: job.SourcePatchSHA256,
		BaseImageManifestDigest: job.BaseImageManifestDigest, BaseStoreDiskSHA256: res.Base.StoreDiskSHA256,
		CodeCommit: res.CodeCommit, CodeCommitSource: res.CodeCommitSource, RuntimePath: res.RuntimePath,
		LayeringEntrypoint: filepath.Base(res.RuntimePath) + "/bin/autoputer",
		ClosureDigest:      res.ExportedDigest, ClosureBytes: info.Size(), ClosurePaths: res.ClosurePaths,
		BuilderReceiptDigest: receiptDigest, DerivationPath: res.DerivationPath, FlakeLockSHA256: res.FlakeLockSHA256,
		StoreSchemaVersion: res.StoreSchemaVersion, BuiltAt: res.BuiltAt,
	}, "", ""
}

func (s *Service) finish(job *Job, state, reason, detail string) {
	now := s.cfg.Now()
	job.State, job.Reason, job.Detail = state, reason, truncate(detail)
	job.FinishedAt = &now
}

func (s *Service) persist(job *Job) error {
	raw, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(s.cfg.StateDir, "jobs", job.JobID+".json"), append(raw, '\n'), 0o600)
}

func (s *Service) patchPath(jobID string) string {
	return filepath.Join(s.cfg.StateDir, "jobs", jobID+".patch")
}

// VerifyJob is the guest's check of a done job (G4): the job answers this
// request, the result binds the same request, and the platform-control
// receipt binds the job id and the result digest.
func VerifyJob(job Job, req Request, platformControl ed25519.PublicKey) error {
	jobID, err := req.JobID()
	if err != nil {
		return err
	}
	if job.JobID != jobID || job.State != StateDone || job.Result == nil || job.Receipt == nil {
		return fmt.Errorf("host build: job %s is not a done build of this request", job.JobID)
	}
	r := job.Result
	if r.JobID != jobID || r.ComputerID != req.ComputerID || r.OperationID != req.OperationID || r.BundleDigest != req.BundleDigest ||
		r.BaseCommit != req.BaseCommit || r.SourcePatchSHA256 != req.SourcePatchSHA256 || r.BaseImageManifestDigest != req.BaseImageManifestDigest {
		return fmt.Errorf("host build: result does not bind this request")
	}
	if !computerevent.IsSHA256(r.ClosureDigest) || !computerevent.IsSHA256(r.BuilderReceiptDigest) || !isCommit(r.CodeCommit) ||
		r.LayeringEntrypoint != filepath.Base(r.RuntimePath)+"/bin/autoputer" || strings.Contains(filepath.Base(r.RuntimePath), "..") {
		return fmt.Errorf("host build: result is incomplete")
	}
	digest, err := r.Digest()
	if err != nil {
		return err
	}
	receipt := job.Receipt
	if receipt.Kind != selfdevprotocol.ReceiptKindHostBuild || receipt.ComputerID != req.ComputerID ||
		receipt.RequestCommitment != jobID || receipt.ArtifactDigest != digest {
		return fmt.Errorf("host build: receipt does not bind this job and result")
	}
	return receipt.Verify(platformControl)
}

func manifestBuildCommit(raw []byte) string {
	for _, line := range strings.Split(string(raw), "\n") {
		if key, value, ok := strings.Cut(strings.TrimSpace(line), "="); ok && key == "build_commit" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func truncate(s string) string {
	if len(s) <= maxDetailBytes {
		return s
	}
	return s[:maxDetailBytes] + "…"
}

func writeFileAtomic(path string, content []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// SampleHostPressure reads MemAvailable and the PSI memory some avg60.
func SampleHostPressure() (Pressure, error) {
	var p Pressure
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return p, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "MemAvailable:" {
			kb, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return p, fmt.Errorf("meminfo: %w", err)
			}
			p.MemAvailableBytes = kb * 1024
		}
	}
	if p.MemAvailableBytes == 0 {
		return p, fmt.Errorf("meminfo has no MemAvailable")
	}
	raw, err := os.ReadFile("/proc/pressure/memory")
	if err != nil {
		return p, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "some" {
			continue
		}
		for _, field := range fields[1:] {
			if v, ok := strings.CutPrefix(field, "avg60="); ok {
				p.MemorySomeAvg60, err = strconv.ParseFloat(v, 64)
				if err != nil {
					return p, fmt.Errorf("pressure/memory: %w", err)
				}
				return p, nil
			}
		}
	}
	return p, fmt.Errorf("pressure/memory has no some avg60")
}
