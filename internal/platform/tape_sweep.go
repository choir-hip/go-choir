package platform

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// The tape sweep extends the artifact GC to the event tape namespaces the
// reachability sweep excludes (computer-event, computer-event-payload,
// pin-receipts). It has two sweeps, both owner-approved (owner notes §26):
//
//   - orphans: tape files no append receipt, event, closure, program or
//     checkpoint names, older than grace (pins whose head CAS never landed);
//   - destroyed: every tape file of a computer proven destroyed.
//
// A dry-run (the default) writes a hashed manifest. Apply names the dry-run's
// run id and manifest hash, re-checks every item at delete time, archives
// before it deletes, tombstones destroyed computers, and logs every unlink in
// the GC deletion log
// (docs/problems/tape-sweeps-destroyed-and-orphans-2026-10-20.md).

const (
	TapeSweepActionDryRun      = "dry-run"
	TapeSweepActionApply       = "apply"
	TapeSweepActionUntombstone = "untombstone"
	TapeSweepActionPrune       = "prune-archives"

	TapeSweepOrphans   = "orphans"
	TapeSweepDestroyed = "destroyed"

	// TapeSweepMinGrace is the floor on the orphan and destroyed grace: an
	// in-flight append pins minutes before its head CAS.
	TapeSweepMinGrace = 24 * time.Hour
	// DefaultTapeSweepQuiet is how long a destroyed computer's head must not
	// have moved.
	DefaultTapeSweepQuiet = 7 * 24 * time.Hour
	// TapeSweepMinArchiveAge is the floor on archive pruning.
	TapeSweepMinArchiveAge = 7 * 24 * time.Hour

	DefaultTapeSweepBatch            = 256
	DefaultTapeSweepPause            = 250 * time.Millisecond
	DefaultTapeSweepMaxDeletes       = 2_000_000
	DefaultTapeSweepMaxBytes   int64 = 8 << 30

	tapeSweepArchiveDirName = "tape-sweep-archive"
	tapeSweepSeqChunk       = 1000
)

const (
	tapeNSEvent   = "computer-event"
	tapeNSPayload = "computer-event-payload"
	tapeNSReceipt = "pin-receipts"
)

// tapeNamespaces is the classification order; deletion within a destroyed
// computer runs payloads, then receipts, then events, so a capped run never
// removes an event before the payloads it names.
var tapeNamespaces = []string{tapeNSPayload, tapeNSReceipt, tapeNSEvent}

var (
	errTapeSweepActiveNotConfigured = errors.New("tape sweep: apply is not enabled (PLATFORM_TAPE_SWEEP_MODE=active)")
	errTapeSweepBusy                = errors.New("tape sweep: a run is in progress")
	hex64RE                         = regexp.MustCompile(`[0-9a-f]{64}`)
	tapeSweepRunIDRE                = regexp.MustCompile(`^ts-[0-9]{8}T[0-9]{6}Z-[0-9a-f]{8}$`)
)

// ComputerRegistry is the ownership authority the destroyed proof queries.
// LookupComputer reports whether any ownership record exists for the
// computer; ListComputerIDs lists every registry entry in every state.
type ComputerRegistry interface {
	LookupComputer(ctx context.Context, computerID string) (bool, error)
	ListComputerIDs(ctx context.Context) ([]string, error)
}

type vmctlComputerRegistry struct{ client *vmctl.Client }

// NewVmctlComputerRegistry adapts the vmctl client to ComputerRegistry.
func NewVmctlComputerRegistry(client *vmctl.Client) ComputerRegistry {
	if client == nil {
		return nil
	}
	return vmctlComputerRegistry{client: client}
}

func (r vmctlComputerRegistry) LookupComputer(ctx context.Context, computerID string) (bool, error) {
	own, err := r.client.LookupComputerByIDContext(ctx, computerID)
	if err != nil {
		return false, err
	}
	return own != nil, nil
}

func (r vmctlComputerRegistry) ListComputerIDs(ctx context.Context) ([]string, error) {
	owns, err := r.client.ListOwnershipsContext(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(owns))
	for _, own := range owns {
		out = append(out, own.ComputerID)
	}
	return out, nil
}

// TapeSweepPolicy is the configured policy. Mode gates apply; ArchiveDir is
// where archives go (default <artifacts_root>/tape-sweep-archive, inside
// corpusd's systemd ReadWritePaths).
type TapeSweepPolicy struct {
	Mode       string `json:"mode"`
	ArchiveDir string `json:"archive_dir"`
}

// TapeSweepRequest is one POST to /internal/platform/artifact-gc/tape.
// Durations are Go duration strings ("24h", "168h").
type TapeSweepRequest struct {
	Action string   `json:"action"`
	Sweeps []string `json:"sweeps,omitempty"`
	Grace  string   `json:"grace,omitempty"`
	Quiet  string   `json:"quiet,omitempty"`
	// AcceptUnrecordedForkLineage acknowledges that no fork-lineage record
	// exists to check. Without it on both dry-run and apply, no destroyed
	// computer's file is listed or deleted.
	AcceptUnrecordedForkLineage bool   `json:"accept_unrecorded_fork_lineage,omitempty"`
	RunID                       string `json:"run_id,omitempty"`
	ManifestSHA256              string `json:"manifest_sha256,omitempty"`
	MaxDeletes                  int    `json:"max_deletes,omitempty"`
	MaxBytes                    int64  `json:"max_bytes,omitempty"`
	BatchSize                   int    `json:"batch_size,omitempty"`
	Pause                       string `json:"pause,omitempty"`
	ComputerID                  string `json:"computer_id,omitempty"`
	OlderThan                   string `json:"older_than,omitempty"`
}

type tapeSweepParams struct {
	action      string
	orphans     bool
	destroyed   bool
	grace       time.Duration
	quiet       time.Duration
	forkAck     bool
	runID       string
	manifestSHA string
	maxDeletes  int
	maxBytes    int64
	batch       int
	pause       time.Duration
	computerID  string
	olderThan   time.Duration
}

// TapeSweepComputerProof is the destroyed proof recorded for one computer.
type TapeSweepComputerProof struct {
	ComputerID         string `json:"computer_id"`
	HeadSequence       uint64 `json:"head_sequence"`
	CanonicalEventHead string `json:"canonical_event_head"`
	HeadUpdatedAt      string `json:"head_updated_at"`
	OwnershipLookup    string `json:"ownership_lookup"` // not_found | found
	InRegistryList     bool   `json:"in_registry_list"`
	HasRoute           bool   `json:"has_route"`
	Quiet              bool   `json:"quiet"`
	// ForkLineage is always "unrecorded": no fork-lineage store exists.
	ForkLineage                 string `json:"fork_lineage"`
	AcceptUnrecordedForkLineage bool   `json:"accept_unrecorded_fork_lineage"`
	Tombstone                   string `json:"tombstone,omitempty"`
	Destroyed                   bool   `json:"destroyed"`
	Deletable                   bool   `json:"deletable"`
	Reason                      string `json:"reason"`
	Files                       int    `json:"files"`
	Bytes                       int64  `json:"bytes"`
	SharedPayloadsKept          int    `json:"shared_payloads_kept,omitempty"`
}

// TapeSweepNamespaceCounts is one namespace's orphan classification.
type TapeSweepNamespaceCounts struct {
	Scanned       int    `json:"scanned"`
	Referenced    int    `json:"referenced"`
	Unrecognized  int    `json:"unrecognized"`
	InGrace       int    `json:"in_grace"`
	Eligible      int    `json:"eligible"`
	EligibleBytes int64  `json:"eligible_bytes"`
	Refused       string `json:"refused,omitempty"`
}

// TapeSweepReport is the receipt for one dry-run or apply.
type TapeSweepReport struct {
	RunID                       string                              `json:"run_id"`
	Action                      string                              `json:"action"`
	Mode                        string                              `json:"mode"`
	Grace                       string                              `json:"grace"`
	Quiet                       string                              `json:"quiet,omitempty"`
	Sweeps                      []string                            `json:"sweeps"`
	AcceptUnrecordedForkLineage bool                                `json:"accept_unrecorded_fork_lineage"`
	ManifestPath                string                              `json:"manifest_path,omitempty"`
	ManifestSHA256              string                              `json:"manifest_sha256,omitempty"`
	ReportPath                  string                              `json:"report_path,omitempty"`
	Computers                   int                                 `json:"computers"`
	ChainEvents                 uint64                              `json:"chain_events"`
	Orphans                     map[string]TapeSweepNamespaceCounts `json:"orphans,omitempty"`
	OrphanFiles                 int                                 `json:"orphan_files"`
	OrphanBytes                 int64                               `json:"orphan_bytes"`
	OrphansRefused              string                              `json:"orphans_refused,omitempty"`
	DestroyedRefused            string                              `json:"destroyed_refused,omitempty"`
	DestroyedComputers          int                                 `json:"destroyed_computers"`
	DestroyedBlockedByLineage   int                                 `json:"destroyed_blocked_by_fork_lineage"`
	DestroyedListed             int                                 `json:"destroyed_listed"`
	DestroyedFiles              int                                 `json:"destroyed_files"`
	DestroyedBytes              int64                               `json:"destroyed_bytes"`
	DestroyedProofs             []TapeSweepComputerProof            `json:"destroyed_proofs,omitempty"`
	Apply                       *TapeSweepApplyCounts               `json:"apply,omitempty"`
	Warnings                    []string                            `json:"warnings,omitempty"`
	StartedAt                   time.Time                           `json:"started_at"`
	DurationMs                  int64                               `json:"duration_ms"`
}

// TapeSweepApplyCounts are the apply outcomes, by the GC log's op names.
type TapeSweepApplyCounts struct {
	Deleted             int      `json:"deleted"`
	BytesDeleted        int64    `json:"bytes_deleted"`
	Gone                int      `json:"gone"`
	KeptRefreshed       int      `json:"kept_refreshed"`
	KeptReferenced      int      `json:"kept_referenced"`
	KeptNotDestroyed    int      `json:"kept_not_destroyed"`
	KeptUnverified      int      `json:"kept_unverified"`
	DeferredByCap       int      `json:"deferred_by_cap"`
	Errors              int      `json:"errors"`
	ComputersTombstoned int      `json:"computers_tombstoned"`
	ComputersDeleted    int      `json:"computers_deleted"`
	ComputersSkipped    int      `json:"computers_skipped"`
	Archives            []string `json:"archives,omitempty"`
	ArchivesPruned      int      `json:"archives_pruned,omitempty"`
}

// TapeSweepJob is the asynchronous run state GET returns.
type TapeSweepJob struct {
	JobID      string           `json:"job_id"`
	Action     string           `json:"action"`
	RunID      string           `json:"run_id,omitempty"`
	State      string           `json:"state"` // running | done | failed
	Phase      string           `json:"phase,omitempty"`
	Error      string           `json:"error,omitempty"`
	StartedAt  time.Time        `json:"started_at"`
	FinishedAt *time.Time       `json:"finished_at,omitempty"`
	Report     *TapeSweepReport `json:"report,omitempty"`
}

// tapeSweepState is the sweep's run lock, policy, registry and jobs. The run
// lock is not artifactGCMu: that lock also serializes projection-base pins,
// and a tape classification reads every event file.
type tapeSweepState struct {
	mu       sync.Mutex
	running  bool
	policy   TapeSweepPolicy
	registry ComputerRegistry
	jobs     map[string]*TapeSweepJob
	order    []string
	// beforeDelete is a test hook run before each item's locked unlink.
	beforeDelete func(item tapeSweepItem)
	// afterReferences is a test hook run after the dry-run's chain walk and
	// before the append-receipt check, to simulate live appends during the walk.
	afterReferences func()
	// extraRefsMax caps the unique digests the extra-refs route holds and
	// answers; 0 means defaultTapeExtraRefsMax. Test-only: set before the
	// route is called and never written in production, so it is read without
	// the sweep lock.
	extraRefsMax int
}

// SetTapeSweepPolicy records the configured policy.
func (s *Service) SetTapeSweepPolicy(policy TapeSweepPolicy) {
	if s == nil {
		return
	}
	policy.Mode = strings.TrimSpace(policy.Mode)
	switch policy.Mode {
	case ArtifactGCModeActive, ArtifactGCModeOff:
	default:
		policy.Mode = ArtifactGCModeDryRun
	}
	s.tapeSweep.mu.Lock()
	s.tapeSweep.policy = policy
	s.tapeSweep.mu.Unlock()
}

// SetComputerRegistry installs the ownership authority the destroyed proof
// needs. Without it the destroyed sweep is refused.
func (s *Service) SetComputerRegistry(registry ComputerRegistry) {
	if s == nil {
		return
	}
	s.tapeSweep.mu.Lock()
	s.tapeSweep.registry = registry
	s.tapeSweep.mu.Unlock()
}

func (s *Service) tapeSweepPolicy() (TapeSweepPolicy, ComputerRegistry) {
	s.tapeSweep.mu.Lock()
	defer s.tapeSweep.mu.Unlock()
	policy := s.tapeSweep.policy
	if policy.Mode == "" {
		policy.Mode = ArtifactGCModeDryRun
	}
	if strings.TrimSpace(policy.ArchiveDir) == "" {
		policy.ArchiveDir = filepath.Join(s.artifactsRoot, tapeSweepArchiveDirName)
	}
	return policy, s.tapeSweep.registry
}

func parseTapeSweepDuration(raw string, def time.Duration) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("tape sweep: invalid duration %q", raw)
	}
	return d, nil
}

// clampTapeSweepRequest validates a request against the policy. Grace and
// quiet are clamped up to their floors, never down; apply and prune need the
// active mode; apply needs a run id and manifest hash.
func (s *Service) clampTapeSweepRequest(req TapeSweepRequest) (tapeSweepParams, error) {
	policy, _ := s.tapeSweepPolicy()
	p := tapeSweepParams{action: strings.TrimSpace(req.Action), forkAck: req.AcceptUnrecordedForkLineage}
	if p.action == "" {
		p.action = TapeSweepActionDryRun
	}
	if policy.Mode == ArtifactGCModeOff {
		return p, errors.New("tape sweep: disabled (PLATFORM_TAPE_SWEEP_MODE=off)")
	}
	var err error
	if p.grace, err = parseTapeSweepDuration(req.Grace, TapeSweepMinGrace); err != nil {
		return p, err
	}
	if p.grace < TapeSweepMinGrace {
		p.grace = TapeSweepMinGrace
	}
	if p.quiet, err = parseTapeSweepDuration(req.Quiet, DefaultTapeSweepQuiet); err != nil {
		return p, err
	}
	if p.quiet < p.grace {
		p.quiet = p.grace
	}
	if p.pause, err = parseTapeSweepDuration(req.Pause, DefaultTapeSweepPause); err != nil {
		return p, err
	}
	if p.pause < 0 {
		p.pause = 0
	}
	p.batch = req.BatchSize
	if p.batch <= 0 || p.batch > 4096 {
		p.batch = DefaultTapeSweepBatch
	}
	p.maxDeletes = req.MaxDeletes
	if p.maxDeletes <= 0 || p.maxDeletes > DefaultTapeSweepMaxDeletes {
		p.maxDeletes = DefaultTapeSweepMaxDeletes
	}
	p.maxBytes = req.MaxBytes
	if p.maxBytes <= 0 || p.maxBytes > DefaultTapeSweepMaxBytes {
		p.maxBytes = DefaultTapeSweepMaxBytes
	}
	if len(req.Sweeps) == 0 {
		p.orphans, p.destroyed = true, true
	}
	for _, sw := range req.Sweeps {
		switch strings.TrimSpace(sw) {
		case TapeSweepOrphans:
			p.orphans = true
		case TapeSweepDestroyed:
			p.destroyed = true
		default:
			return p, fmt.Errorf("tape sweep: unknown sweep %q", sw)
		}
	}
	switch p.action {
	case TapeSweepActionDryRun:
	case TapeSweepActionApply:
		if policy.Mode != ArtifactGCModeActive {
			return p, errTapeSweepActiveNotConfigured
		}
		p.runID = strings.TrimSpace(req.RunID)
		p.manifestSHA = strings.ToLower(strings.TrimSpace(req.ManifestSHA256))
		if !tapeSweepRunIDRE.MatchString(p.runID) || !validHexDigest(p.manifestSHA) {
			return p, errors.New("tape sweep: apply needs the dry-run's run_id and manifest_sha256")
		}
	case TapeSweepActionUntombstone:
		p.computerID = strings.TrimSpace(req.ComputerID)
		if p.computerID == "" {
			return p, errors.New("tape sweep: untombstone needs computer_id")
		}
	case TapeSweepActionPrune:
		if policy.Mode != ArtifactGCModeActive {
			return p, errTapeSweepActiveNotConfigured
		}
		if p.olderThan, err = parseTapeSweepDuration(req.OlderThan, TapeSweepMinArchiveAge); err != nil {
			return p, err
		}
		if p.olderThan < TapeSweepMinArchiveAge {
			p.olderThan = TapeSweepMinArchiveAge
		}
	default:
		return p, fmt.Errorf("tape sweep: unknown action %q", p.action)
	}
	return p, nil
}

// StartTapeSweep validates the request and starts it in the background. One
// run at a time; the job is read back with TapeSweepJobStatus.
func (s *Service) StartTapeSweep(req TapeSweepRequest) (TapeSweepJob, error) {
	if s == nil || s.store == nil || s.artifactsRoot == "" {
		return TapeSweepJob{}, errors.New("tape sweep: platform store and artifacts root are required")
	}
	params, err := s.clampTapeSweepRequest(req)
	if err != nil {
		return TapeSweepJob{}, err
	}
	s.tapeSweep.mu.Lock()
	if s.tapeSweep.running {
		s.tapeSweep.mu.Unlock()
		return TapeSweepJob{}, errTapeSweepBusy
	}
	s.tapeSweep.running = true
	job := &TapeSweepJob{JobID: newTapeSweepRunID(time.Now()), Action: params.action, RunID: params.runID, State: "running", StartedAt: time.Now().UTC()}
	if params.action == TapeSweepActionDryRun {
		job.RunID = job.JobID
	}
	if s.tapeSweep.jobs == nil {
		s.tapeSweep.jobs = map[string]*TapeSweepJob{}
	}
	s.tapeSweep.jobs[job.JobID] = job
	s.tapeSweep.order = append(s.tapeSweep.order, job.JobID)
	if len(s.tapeSweep.order) > 32 {
		delete(s.tapeSweep.jobs, s.tapeSweep.order[0])
		s.tapeSweep.order = s.tapeSweep.order[1:]
	}
	snapshot := *job
	s.tapeSweep.mu.Unlock()
	go func() {
		report, err := s.runTapeSweep(context.Background(), job.JobID, params)
		s.finishTapeSweepJob(job.JobID, report, err)
	}()
	return snapshot, nil
}

// RunTapeSweep runs one request synchronously (tests and the job runner).
func (s *Service) RunTapeSweep(ctx context.Context, req TapeSweepRequest) (TapeSweepReport, error) {
	params, err := s.clampTapeSweepRequest(req)
	if err != nil {
		return TapeSweepReport{}, err
	}
	s.tapeSweep.mu.Lock()
	if s.tapeSweep.running {
		s.tapeSweep.mu.Unlock()
		return TapeSweepReport{}, errTapeSweepBusy
	}
	s.tapeSweep.running = true
	s.tapeSweep.mu.Unlock()
	defer func() {
		s.tapeSweep.mu.Lock()
		s.tapeSweep.running = false
		s.tapeSweep.mu.Unlock()
	}()
	return s.runTapeSweep(ctx, newTapeSweepRunID(time.Now()), params)
}

func (s *Service) finishTapeSweepJob(jobID string, report TapeSweepReport, err error) {
	s.tapeSweep.mu.Lock()
	defer s.tapeSweep.mu.Unlock()
	s.tapeSweep.running = false
	job := s.tapeSweep.jobs[jobID]
	if job == nil {
		return
	}
	now := time.Now().UTC()
	job.FinishedAt = &now
	job.Report = &report
	job.Phase = ""
	if err != nil {
		job.State = "failed"
		job.Error = err.Error()
		return
	}
	job.State = "done"
}

func (s *Service) setTapeSweepPhase(jobID, phase string) {
	s.tapeSweep.mu.Lock()
	if job := s.tapeSweep.jobs[jobID]; job != nil {
		job.Phase = phase
	}
	s.tapeSweep.mu.Unlock()
	log.Printf("tape sweep: job=%s phase=%s", jobID, phase)
}

// TapeSweepJobStatus returns a job by id, or the latest job when id is empty.
func (s *Service) TapeSweepJobStatus(jobID string) (TapeSweepJob, bool) {
	s.tapeSweep.mu.Lock()
	defer s.tapeSweep.mu.Unlock()
	if jobID == "" && len(s.tapeSweep.order) > 0 {
		jobID = s.tapeSweep.order[len(s.tapeSweep.order)-1]
	}
	job := s.tapeSweep.jobs[jobID]
	if job == nil {
		return TapeSweepJob{}, false
	}
	return *job, true
}

func newTapeSweepRunID(now time.Time) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return "ts-" + now.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(b[:])
}

func (s *Service) runTapeSweep(ctx context.Context, jobID string, p tapeSweepParams) (TapeSweepReport, error) {
	switch p.action {
	case TapeSweepActionDryRun:
		return s.tapeSweepDryRun(ctx, jobID, p)
	case TapeSweepActionApply:
		return s.tapeSweepApply(ctx, jobID, p)
	case TapeSweepActionUntombstone:
		report := TapeSweepReport{RunID: jobID, Action: p.action, StartedAt: time.Now().UTC()}
		err := s.store.removeTapeTombstone(ctx, p.computerID)
		if err == nil {
			log.Printf("tape sweep: untombstoned computer=%s", p.computerID)
		}
		return report, err
	case TapeSweepActionPrune:
		return s.pruneTapeSweepArchives(p)
	}
	return TapeSweepReport{}, fmt.Errorf("tape sweep: unknown action %q", p.action)
}

// --- digest prefix sets --------------------------------------------------

// digestPrefixSet holds 8-byte digest prefixes, sorted. A prefix collision
// can only keep a file (a false "referenced"), never delete one, and keeps
// the full-store reference set near 8 bytes per digest.
type digestPrefixSet struct{ v []uint64 }

func digestPrefix(digest string) (uint64, bool) {
	if len(digest) != 64 || !hexDigestRE.MatchString(digest) {
		return 0, false
	}
	v, err := strconv.ParseUint(digest[:16], 16, 64)
	return v, err == nil
}

func (p *digestPrefixSet) add(digest string) {
	if v, ok := digestPrefix(digest); ok {
		p.v = append(p.v, v)
	}
}

func (p *digestPrefixSet) addAllHex(text string) {
	for _, m := range tapeHexRefs(text) {
		p.add(m)
	}
}

func (p *digestPrefixSet) seal() {
	sort.Slice(p.v, func(i, j int) bool { return p.v[i] < p.v[j] })
	out := p.v[:0]
	for i, v := range p.v {
		if i == 0 || v != p.v[i-1] {
			out = append(out, v)
		}
	}
	p.v = out
}

func (p *digestPrefixSet) has(digest string) bool {
	v, ok := digestPrefix(digest)
	if !ok {
		return false
	}
	i := sort.Search(len(p.v), func(i int) bool { return p.v[i] >= v })
	return i < len(p.v) && p.v[i] == v
}

// --- chain reading -------------------------------------------------------

type tapeHead struct {
	ComputerID string    `json:"computer_id"`
	Sequence   uint64    `json:"sequence"`
	Canonical  string    `json:"canonical_event_head"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (s *Service) tapeHeads(ctx context.Context) (map[string]tapeHead, error) {
	rows, err := s.store.db.QueryContext(ctx, `SELECT computer_id, sequence, canonical_event_head, updated_at FROM computer_event_heads`)
	if err != nil {
		return nil, fmt.Errorf("tape sweep: read heads: %w", err)
	}
	defer rows.Close()
	out := map[string]tapeHead{}
	for rows.Next() {
		var h tapeHead
		if err := rows.Scan(&h.ComputerID, &h.Sequence, &h.Canonical, &h.UpdatedAt); err != nil {
			return nil, fmt.Errorf("tape sweep: scan heads: %w", err)
		}
		out[h.ComputerID] = h
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tape sweep: iterate heads: %w", err)
	}
	return out, nil
}

// tapeReceiptsMatchSnapshot is the orphan sweep's completeness check on the
// append-receipt table, bounded by the same head snapshot the walk used. The
// walk takes minutes on a live host, so a global receipt count would never
// match: live computers append meanwhile. Instead, per computer, the receipts
// at or below the snapshot head must equal the rows the walk saw. A receipt
// above the snapshot head is allowed only when it is at or below the
// computer's current head (a live append: its files are inside the grace
// window and apply re-checks references in its delta phase). A receipt above
// the current head, or for a computer with no head row, is a leftover and
// refuses. It returns a non-empty refusal reason, or an error when the
// check itself could not run.
func (s *Service) tapeReceiptsMatchSnapshot(ctx context.Context, heads map[string]tapeHead, ids []string, walked map[string]uint64) (string, error) {
	// Highest receipt sequence per computer, read before the current heads so
	// every receipt seen here is covered by a head at least as new.
	type maxSeq struct {
		computerID string
		sequence   uint64
	}
	var maxes []maxSeq
	rows, err := s.store.db.QueryContext(ctx, `SELECT computer_id, MAX(sequence) FROM computer_event_append_receipts GROUP BY computer_id ORDER BY computer_id`)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var m maxSeq
		if err := rows.Scan(&m.computerID, &m.sequence); err != nil {
			rows.Close()
			return "", err
		}
		maxes = append(maxes, m)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", err
	}
	rows.Close()

	for _, id := range ids {
		var bounded uint64
		if err := s.store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM computer_event_append_receipts WHERE computer_id=? AND sequence<=?`, id, heads[id].Sequence).Scan(&bounded); err != nil {
			return "", err
		}
		if bounded != walked[id] {
			return fmt.Sprintf("computer %s: append receipts (%d) up to snapshot head sequence %d do not match the head-bounded chain (%d)", id, bounded, heads[id].Sequence, walked[id]), nil
		}
	}
	for _, m := range maxes {
		snap, known := heads[m.computerID]
		if !known {
			return fmt.Sprintf("computer %s has append receipts (up to sequence %d) but no head row", m.computerID, m.sequence), nil
		}
		if m.sequence <= snap.Sequence {
			continue
		}
		var current uint64
		err := s.store.db.QueryRowContext(ctx, `SELECT sequence FROM computer_event_heads WHERE computer_id=?`, m.computerID).Scan(&current)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
		if m.sequence > current {
			return fmt.Sprintf("computer %s has an append receipt at sequence %d above its current head sequence %d", m.computerID, m.sequence, current), nil
		}
	}
	return "", nil
}

func sortedHeadIDs(heads map[string]tapeHead) []string {
	ids := make([]string, 0, len(heads))
	for id := range heads {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// tapeChainRow is one append receipt's tape references.
type tapeChainRow struct {
	Sequence      uint64
	EventDigest   string
	EventPin      string
	PayloadPins   []string
	PayloadRefs   []string // every 64-hex digest the event file names
	EventReadable bool
}

// walkTapeChain visits rows from+1..to of a computer's gapless chain by
// indexed sequence point lookups (event_replay.go), reading each event file
// when readEvents is set. A gap or an unreadable row is an error: a partial
// chain must never pass for a complete one.
func (s *Service) walkTapeChain(ctx context.Context, computerID string, from, to uint64, readEvents bool, visit func(tapeChainRow) error) error {
	for start := from + 1; start <= to; start += tapeSweepSeqChunk {
		end := min(to, start+tapeSweepSeqChunk-1)
		args := []any{computerID}
		for seq := start; seq <= end; seq++ {
			args = append(args, seq)
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(args)-1), ",")
		rows, err := s.store.db.QueryContext(ctx, `SELECT sequence, event_digest, event_pin_receipt_digest, event_head_receipt_json FROM computer_event_append_receipts WHERE computer_id=? AND sequence IN (`+placeholders+`) ORDER BY sequence`, args...)
		if err != nil {
			return fmt.Errorf("tape sweep: chain %s: %w", computerID, err)
		}
		expected := start
		var batch []tapeChainRow
		for rows.Next() {
			var row tapeChainRow
			var rawReceipt string
			if err := rows.Scan(&row.Sequence, &row.EventDigest, &row.EventPin, &rawReceipt); err != nil {
				rows.Close()
				return fmt.Errorf("tape sweep: chain %s: %w", computerID, err)
			}
			if row.Sequence != expected {
				rows.Close()
				return fmt.Errorf("tape sweep: chain %s: gap at sequence %d", computerID, expected)
			}
			expected++
			var receipt computerevent.Receipt
			if err := json.Unmarshal([]byte(rawReceipt), &receipt); err != nil {
				rows.Close()
				return fmt.Errorf("tape sweep: chain %s sequence %d: decode receipt: %w", computerID, row.Sequence, err)
			}
			if pins, ok := receiptPinDigests(receipt.KindFields); ok && len(pins) > 1 {
				row.PayloadPins = hex64RE.FindAllString(strings.Join(pins[1:], " "), -1)
			}
			batch = append(batch, row)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("tape sweep: chain %s: %w", computerID, err)
		}
		rows.Close()
		if expected != end+1 {
			return fmt.Errorf("tape sweep: chain %s: gap at sequence %d", computerID, expected)
		}
		for _, row := range batch {
			if readEvents {
				// Segment first, legacy second. A corrupt segment is not
				// "unreadable, no references": it stops the walk, so no
				// caller can read it as an empty reference set.
				raw, _, err := s.tapeReader().Read(computerID, tapestore.NSEvent, row.EventDigest)
				if errors.Is(err, tapestore.ErrCorrupt) {
					return fmt.Errorf("tape sweep: chain %s sequence %d: %w", computerID, row.Sequence, err)
				}
				if err == nil && computerevent.DigestBytes(raw) == row.EventDigest {
					row.EventReadable = true
					row.PayloadRefs = hex64RE.FindAllString(string(raw), -1)
				}
			}
			if err := visit(row); err != nil {
				return err
			}
		}
	}
	return nil
}

// tapeExtraRefQueries are the non-chain records that may name a tape digest.
// Every 64-hex substring counts as a reference (over-retention is safe).
var tapeExtraRefQueries = []string{
	`SELECT closure_json FROM computer_version_code_closures`,
	`SELECT program_json FROM computer_version_artifact_programs`,
	`SELECT checkpoint_json FROM computer_checkpoints`,
	`SELECT receipt_json FROM computer_checkpoints`,
	`SELECT certificate_json FROM computer_route_projection_certificates`,
}

// tapeHexRefs is the one extraction of digests from a database value: every
// 64-hex substring counts as a reference. The sweep's live set, the apply
// pass's delta and the extra-refs route all call it, so the route lists
// exactly what the sweep keeps.
func tapeHexRefs(text string) []string {
	return hex64RE.FindAllString(text, -1)
}

func (s *Service) tapeExtraRefs(ctx context.Context, add func(string)) error {
	for _, q := range tapeExtraRefQueries {
		if err := scanStrings(ctx, s.store.db, q, add); err != nil {
			return fmt.Errorf("tape sweep: extra refs (%s): %w", q, err)
		}
	}
	return nil
}

// tapeSweepFileName maps a namespace entry name to its digest; ok is false
// for a name the sweep does not recognize (never deleted).
func tapeSweepFileName(ns, name string) (string, bool) {
	if ns == tapeNSReceipt {
		if !strings.HasSuffix(name, ".json") {
			return "", false
		}
		name = strings.TrimSuffix(name, ".json")
	}
	if len(name) != 64 || !hexDigestRE.MatchString(name) {
		return "", false
	}
	return name, true
}

func tapeSweepRel(ns, digest string) string {
	if ns == tapeNSReceipt {
		return "sha256/" + ns + "/" + digest + ".json"
	}
	return "sha256/" + ns + "/" + digest
}

// --- manifest ------------------------------------------------------------

type tapeSweepManifestHeader struct {
	Kind                        string              `json:"kind"` // header
	RunID                       string              `json:"run_id"`
	CreatedAt                   string              `json:"created_at"`
	Grace                       string              `json:"grace"`
	Quiet                       string              `json:"quiet"`
	Sweeps                      []string            `json:"sweeps"`
	AcceptUnrecordedForkLineage bool                `json:"accept_unrecorded_fork_lineage"`
	Heads                       map[string]tapeHead `json:"heads"`
}

type tapeSweepItem struct {
	Kind      string `json:"kind"` // item
	Sweep     string `json:"sweep"`
	Computer  string `json:"computer,omitempty"`
	Sequence  uint64 `json:"sequence,omitempty"`
	Namespace string `json:"ns"`
	Digest    string `json:"digest"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	MTime     string `json:"mtime"`
	Reason    string `json:"reason"`
}

type tapeSweepManifestComputer struct {
	Kind  string                 `json:"kind"` // computer
	Proof TapeSweepComputerProof `json:"proof"`
}

func (s *Service) tapeSweepLogDir() string {
	return filepath.Join(s.artifactsRoot, artifactGCLogDir)
}

func (s *Service) tapeSweepManifestPath(runID string) string {
	return filepath.Join(s.tapeSweepLogDir(), "tape-sweep-"+runID+".manifest.jsonl")
}

type manifestWriter struct {
	file   *os.File
	buf    *bufio.Writer
	hash   io.Writer
	digest func() string
}

func newManifestWriter(path string) (*manifestWriter, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	return &manifestWriter{file: f, buf: bufio.NewWriterSize(io.MultiWriter(f, h), 1<<16), hash: h, digest: func() string { return hex.EncodeToString(h.Sum(nil)) }}, nil
}

func (w *manifestWriter) line(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.buf.Write(append(b, '\n'))
	return err
}

func (w *manifestWriter) close() (string, error) {
	if err := w.buf.Flush(); err != nil {
		w.file.Close()
		return "", err
	}
	if err := w.file.Sync(); err != nil {
		w.file.Close()
		return "", err
	}
	if err := w.file.Close(); err != nil {
		return "", err
	}
	return w.digest(), nil
}

func writeJSONFileSync(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// --- dry-run -------------------------------------------------------------

// tapeSweepDryRun classifies the tape and writes the manifest. Nothing is
// deleted.
func (s *Service) tapeSweepDryRun(ctx context.Context, jobID string, p tapeSweepParams) (TapeSweepReport, error) {
	started := time.Now()
	policy, registry := s.tapeSweepPolicy()
	report := TapeSweepReport{RunID: jobID, Action: p.action, Mode: policy.Mode, Grace: p.grace.String(), Quiet: p.quiet.String(), AcceptUnrecordedForkLineage: p.forkAck, Orphans: map[string]TapeSweepNamespaceCounts{}, StartedAt: started.UTC()}
	if p.orphans {
		report.Sweeps = append(report.Sweeps, TapeSweepOrphans)
	}
	if p.destroyed {
		report.Sweeps = append(report.Sweeps, TapeSweepDestroyed)
	}
	defer func() { report.DurationMs = time.Since(started).Milliseconds() }()
	cutoff := started.Add(-p.grace)

	s.setTapeSweepPhase(jobID, "heads")
	heads, err := s.tapeHeads(ctx)
	if err != nil {
		return report, err
	}
	report.Computers = len(heads)
	ids := sortedHeadIDs(heads)
	if err := s.store.loadTapeTombstones(ctx); err != nil {
		return report, err
	}

	// Destroyed proof, per computer. Any registry or route failure refuses
	// the whole destroyed sweep (fail closed); the orphan sweep still runs,
	// with every computer's references in its set.
	proofs := map[string]*TapeSweepComputerProof{}
	if p.destroyed {
		s.setTapeSweepPhase(jobID, "destroyed-proof")
		proofs, err = s.tapeDestroyedProofs(ctx, registry, heads, ids, started, p)
		if err != nil {
			report.DestroyedRefused = err.Error()
			report.Warnings = append(report.Warnings, "destroyed sweep refused: "+err.Error())
			proofs = map[string]*TapeSweepComputerProof{}
		}
	}
	deletable := func(id string) bool { pr := proofs[id]; return pr != nil && pr.Deletable }

	// Reference sets. live holds every reference of every computer that is
	// not deletable plus every extra record; doomed holds the deletable
	// computers' references. Orphans are named by neither; a deletable
	// computer's payload is kept when live also names it.
	s.setTapeSweepPhase(jobID, "references")
	var live, doomed digestPrefixSet
	orphansIncomplete := ""
	walkedRows := map[string]uint64{}
	tombstoned := 0
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		h := heads[id]
		report.ChainEvents += h.Sequence
		set := &live
		if deletable(id) {
			set = &doomed
		}
		tomb, hasTomb, err := s.store.TapeTombstone(ctx, id)
		if err != nil {
			return report, err
		}
		// A tombstoned computer's event files may be gone: its payload refs
		// are unknown, so an unfinished one refuses the payload sweep.
		readEvents := !hasTomb
		if hasTomb {
			tombstoned++
			if tomb.Status != tapeTombstoneDeleted && !deletable(id) {
				orphansIncomplete = fmt.Sprintf("computer %s has an unfinished tape deletion (tombstone %s)", id, tomb.Status)
			}
		}
		err = s.walkTapeChain(ctx, id, 0, h.Sequence, readEvents || deletable(id), func(row tapeChainRow) error {
			walkedRows[id]++
			set.add(row.EventDigest)
			set.add(row.EventPin)
			for _, d := range row.PayloadPins {
				set.add(d)
			}
			for _, d := range row.PayloadRefs {
				set.add(d)
			}
			if readEvents && !row.EventReadable && orphansIncomplete == "" {
				orphansIncomplete = fmt.Sprintf("event %s (computer %s sequence %d) is unreadable; its payload references are unknown", row.EventDigest, id, row.Sequence)
			}
			return nil
		})
		if err != nil {
			report.OrphansRefused = err.Error()
			report.Warnings = append(report.Warnings, err.Error())
			if pr := proofs[id]; pr != nil && pr.Deletable {
				pr.Deletable, pr.Reason = false, "chain unreadable: "+err.Error()
			}
		}
	}
	if hook := s.tapeSweep.afterReferences; hook != nil {
		hook()
	}
	// Keep the first refusal: a chain that failed to read already says why
	// the counts differ.
	if refusal, err := s.tapeReceiptsMatchSnapshot(ctx, heads, ids, walkedRows); err != nil {
		report.OrphansRefused = "count append receipts: " + err.Error()
	} else if refusal != "" && report.OrphansRefused == "" {
		report.OrphansRefused = refusal
	}
	extraErr := s.tapeExtraRefs(ctx, func(text string) { live.addAllHex(text) })
	if extraErr != nil {
		report.OrphansRefused = extraErr.Error()
		if p.destroyed && report.DestroyedRefused == "" {
			report.DestroyedRefused = extraErr.Error()
			for _, pr := range proofs {
				pr.Deletable = false
			}
		}
	}
	live.seal()
	doomed.seal()

	// Manifest.
	if err := os.MkdirAll(s.tapeSweepLogDir(), 0o700); err != nil {
		return report, fmt.Errorf("tape sweep: log dir: %w", err)
	}
	manifestPath := s.tapeSweepManifestPath(jobID)
	mw, err := newManifestWriter(manifestPath)
	if err != nil {
		return report, fmt.Errorf("tape sweep: manifest: %w", err)
	}
	header := tapeSweepManifestHeader{Kind: "header", RunID: jobID, CreatedAt: started.UTC().Format(time.RFC3339Nano), Grace: p.grace.String(), Quiet: p.quiet.String(), Sweeps: report.Sweeps, AcceptUnrecordedForkLineage: p.forkAck, Heads: heads}
	if err := mw.line(header); err != nil {
		mw.close()
		return report, err
	}

	// Destroyed items, per deletable computer: payloads, receipts, events.
	if p.destroyed && report.DestroyedRefused == "" {
		s.setTapeSweepPhase(jobID, "destroyed-items")
		listedPayloads := map[string]bool{}
		for _, id := range ids {
			pr := proofs[id]
			if pr == nil {
				continue
			}
			if pr.Destroyed {
				report.DestroyedComputers++
			}
			if pr.Destroyed && !pr.Deletable && pr.Reason == "fork lineage unrecorded; not acknowledged" {
				report.DestroyedBlockedByLineage++
			}
			if !pr.Deletable {
				continue
			}
			items, shared, err := s.tapeDestroyedItems(ctx, id, heads[id], &live, listedPayloads, cutoff)
			if err != nil {
				pr.Deletable, pr.Reason = false, err.Error()
				report.Warnings = append(report.Warnings, err.Error())
				continue
			}
			pr.SharedPayloadsKept = shared
			for _, it := range items {
				pr.Files++
				pr.Bytes += it.Size
			}
			if err := mw.line(tapeSweepManifestComputer{Kind: "computer", Proof: *pr}); err != nil {
				mw.close()
				return report, err
			}
			for _, it := range items {
				if err := mw.line(it); err != nil {
					mw.close()
					return report, err
				}
			}
			report.DestroyedListed++
			report.DestroyedFiles += pr.Files
			report.DestroyedBytes += pr.Bytes
		}
		for _, id := range ids {
			if pr := proofs[id]; pr != nil && pr.Destroyed {
				report.DestroyedProofs = append(report.DestroyedProofs, *pr)
			}
		}
	}

	// Orphan items.
	if p.orphans {
		if report.OrphansRefused != "" {
			report.Warnings = append(report.Warnings, "orphan sweep refused: "+report.OrphansRefused)
		} else {
			s.setTapeSweepPhase(jobID, "orphan-items")
			for _, ns := range tapeNamespaces {
				counts := TapeSweepNamespaceCounts{}
				if ns == tapeNSPayload && orphansIncomplete != "" {
					counts.Refused = orphansIncomplete
					report.Orphans[ns] = counts
					report.Warnings = append(report.Warnings, "orphan payload sweep refused: "+orphansIncomplete)
					continue
				}
				err := s.tapeOrphanItems(ctx, ns, &live, &doomed, cutoff, &counts, func(it tapeSweepItem) error {
					report.OrphanFiles++
					report.OrphanBytes += it.Size
					return mw.line(it)
				})
				report.Orphans[ns] = counts
				if err != nil {
					mw.close()
					return report, err
				}
			}
		}
	}
	digest, err := mw.close()
	if err != nil {
		return report, fmt.Errorf("tape sweep: manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath+".sha256", []byte(digest+"  "+filepath.Base(manifestPath)+"\n"), 0o600); err != nil {
		return report, err
	}
	report.ManifestPath, report.ManifestSHA256 = manifestPath, digest
	report.ReportPath = filepath.Join(s.tapeSweepLogDir(), "tape-sweep-"+jobID+".report.json")
	report.DurationMs = time.Since(started).Milliseconds()
	if err := writeJSONFileSync(report.ReportPath, report); err != nil {
		return report, err
	}
	log.Printf("tape sweep: dry-run run=%s manifest_sha256=%s computers=%d destroyed=%d listed=%d blocked_by_fork_lineage=%d destroyed_files=%d destroyed_bytes=%d orphan_files=%d orphan_bytes=%d tombstoned=%d warnings=%d",
		jobID, digest, report.Computers, report.DestroyedComputers, report.DestroyedListed, report.DestroyedBlockedByLineage, report.DestroyedFiles, report.DestroyedBytes, report.OrphanFiles, report.OrphanBytes, tombstoned, len(report.Warnings))
	return report, nil
}

// tapeDestroyedProofs evaluates the destroyed proof for every head. It fails
// (refusing the destroyed sweep) when the registry is missing or any registry
// or route query fails.
func (s *Service) tapeDestroyedProofs(ctx context.Context, registry ComputerRegistry, heads map[string]tapeHead, ids []string, now time.Time, p tapeSweepParams) (map[string]*TapeSweepComputerProof, error) {
	if registry == nil {
		return nil, errors.New("computer registry (vmctl) is not configured")
	}
	listed, err := registry.ListComputerIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("registry list: %w", err)
	}
	inList := map[string]bool{}
	for _, id := range listed {
		inList[strings.TrimSpace(id)] = true
	}
	routed, err := s.tapeRoutedComputers(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]*TapeSweepComputerProof{}
	for _, id := range ids {
		h := heads[id]
		pr := &TapeSweepComputerProof{ComputerID: id, HeadSequence: h.Sequence, CanonicalEventHead: h.Canonical, HeadUpdatedAt: h.UpdatedAt.UTC().Format(time.RFC3339Nano), InRegistryList: inList[id], HasRoute: routed[id], ForkLineage: "unrecorded", AcceptUnrecordedForkLineage: p.forkAck}
		pr.Quiet = !h.UpdatedAt.IsZero() && now.Sub(h.UpdatedAt) >= p.quiet
		if t, ok, err := s.store.TapeTombstone(ctx, id); err != nil {
			return nil, err
		} else if ok {
			pr.Tombstone = t.Status
		}
		out[id] = pr
		switch {
		case pr.InRegistryList:
			pr.Reason = "in registry list"
			continue
		case pr.HasRoute:
			pr.Reason = "has route slot"
			continue
		}
		found, err := registry.LookupComputer(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("registry lookup %s: %w", id, err)
		}
		if found {
			pr.OwnershipLookup, pr.Reason = "found", "ownership record exists"
			continue
		}
		pr.OwnershipLookup = "not_found"
		if !pr.Quiet {
			pr.Reason = "head moved within the quiet window"
			continue
		}
		if pr.Tombstone == tapeTombstoneDeleted {
			pr.Destroyed, pr.Reason = true, "already deleted"
			continue
		}
		pr.Destroyed = true
		if !p.forkAck {
			pr.Reason = "fork lineage unrecorded; not acknowledged"
			continue
		}
		// The destroyed sweep archives and unlinks legacy files only. It has
		// no way to archive or delete a segment, so a computer with any is
		// left whole (A2 teaches the sweep segments).
		hasSegments, err := s.tapeReader().HasSegments(id)
		if err != nil {
			return nil, fmt.Errorf("segments of %s: %w", id, err)
		}
		if hasSegments {
			pr.Reason = "computer has tape segments; the destroyed sweep cannot archive or delete segments yet"
			continue
		}
		pr.Deletable, pr.Reason = true, "destroyed: no ownership, not listed, no route, quiet; fork lineage unrecorded (acknowledged)"
	}
	return out, nil
}

// tapeRoutedComputers returns every computer with a route slot. Slot ids are
// computer:<owner>:<computer>; an unparseable id fails closed.
func (s *Service) tapeRoutedComputers(ctx context.Context) (map[string]bool, error) {
	routed := map[string]bool{}
	var bad string
	err := scanStrings(ctx, s.store.db, `SELECT route_slot_id FROM computer_version_route_slots`, func(slot string) {
		parts := strings.Split(slot, ":")
		if len(parts) != 3 || parts[0] != "computer" || parts[2] == "" {
			bad = slot
			return
		}
		routed[parts[2]] = true
	})
	if err != nil {
		return nil, fmt.Errorf("route slots: %w", err)
	}
	if bad != "" {
		return nil, fmt.Errorf("route slots: unparseable slot id %q", bad)
	}
	return routed, nil
}

// tapeDestroyedItems lists one deletable computer's tape files, payloads
// first. A payload any live record names is kept (shared).
func (s *Service) tapeDestroyedItems(ctx context.Context, id string, h tapeHead, live *digestPrefixSet, listedPayloads map[string]bool, cutoff time.Time) ([]tapeSweepItem, int, error) {
	var payloads, receipts, events []tapeSweepItem
	shared := 0
	seen := map[string]bool{}
	add := func(dst *[]tapeSweepItem, ns, digest string, seq uint64, reason string) {
		key := ns + "/" + digest
		if seen[key] {
			return
		}
		seen[key] = true
		rel := tapeSweepRel(ns, digest)
		info, err := os.Lstat(filepath.Join(s.artifactsRoot, filepath.FromSlash(rel)))
		if err != nil || !info.Mode().IsRegular() || !info.ModTime().Before(cutoff) {
			return
		}
		*dst = append(*dst, tapeSweepItem{Kind: "item", Sweep: TapeSweepDestroyed, Computer: id, Sequence: seq, Namespace: ns, Digest: digest, Path: rel, Size: info.Size(), MTime: info.ModTime().UTC().Format(time.RFC3339Nano), Reason: reason})
	}
	err := s.walkTapeChain(ctx, id, 0, h.Sequence, true, func(row tapeChainRow) error {
		add(&events, tapeNSEvent, row.EventDigest, row.Sequence, "destroyed computer event")
		add(&receipts, tapeNSReceipt, row.EventPin, row.Sequence, "destroyed computer event pin receipt")
		for _, d := range row.PayloadPins {
			add(&receipts, tapeNSReceipt, d, row.Sequence, "destroyed computer payload pin receipt")
		}
		for _, d := range row.PayloadRefs {
			if d == row.EventDigest || seen[tapeNSPayload+"/"+d] {
				continue
			}
			if _, err := os.Lstat(filepath.Join(s.artifactsRoot, "sha256", tapeNSPayload, d)); err != nil {
				continue // not a payload file (another digest the event names)
			}
			if live.has(d) {
				seen[tapeNSPayload+"/"+d] = true
				shared++
				continue
			}
			if listedPayloads[d] {
				continue
			}
			listedPayloads[d] = true
			add(&payloads, tapeNSPayload, d, row.Sequence, "destroyed computer payload")
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	out := append(append(payloads, receipts...), events...)
	return out, shared, nil
}

// tapeOrphanItems streams one namespace directory and emits every
// recognized, unreferenced file older than cutoff.
func (s *Service) tapeOrphanItems(ctx context.Context, ns string, live, doomed *digestPrefixSet, cutoff time.Time, counts *TapeSweepNamespaceCounts, emit func(tapeSweepItem) error) error {
	dir := filepath.Join(s.artifactsRoot, "sha256", ns)
	f, err := os.Open(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("tape sweep: open %s: %w", ns, err)
	}
	defer f.Close()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, err := f.ReadDir(4096)
		for _, entry := range entries {
			if !entry.Type().IsRegular() {
				continue
			}
			counts.Scanned++
			digest, ok := tapeSweepFileName(ns, entry.Name())
			if !ok {
				counts.Unrecognized++
				continue
			}
			if live.has(digest) || doomed.has(digest) {
				counts.Referenced++
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if !info.ModTime().Before(cutoff) {
				counts.InGrace++
				continue
			}
			counts.Eligible++
			counts.EligibleBytes += info.Size()
			item := tapeSweepItem{Kind: "item", Sweep: TapeSweepOrphans, Computer: s.tapeFileComputer(ns, digest), Namespace: ns, Digest: digest, Path: tapeSweepRel(ns, digest), Size: info.Size(), MTime: info.ModTime().UTC().Format(time.RFC3339Nano), Reason: "unreferenced pin past grace"}
			if err := emit(item); err != nil {
				return err
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("tape sweep: read %s: %w", ns, err)
		}
	}
}

// tapeFileComputer attributes an orphan to its computer, best effort, for
// the manifest (receipts and events carry computer_id; private payloads
// carry it in their envelope).
func (s *Service) tapeFileComputer(ns, digest string) string {
	raw, err := os.ReadFile(filepath.Join(s.artifactsRoot, filepath.FromSlash(tapeSweepRel(ns, digest))))
	if err != nil {
		return ""
	}
	switch ns {
	case tapeNSReceipt:
		var r computerevent.Receipt
		if json.Unmarshal(raw, &r) != nil {
			return ""
		}
		id, _ := r.KindFields["computer_id"].(string)
		return id
	case tapeNSEvent:
		var e struct {
			ComputerID string `json:"computer_id"`
		}
		_ = json.Unmarshal(raw, &e)
		return e.ComputerID
	default:
		if meta, err := computerevent.InspectPrivateArtifactEnvelope(raw); err == nil {
			return meta.ComputerID
		}
	}
	return ""
}
