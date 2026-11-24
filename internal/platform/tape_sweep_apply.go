package platform

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// tapeSweepLogRecord is the GC deletion-log record (artifactGCLogRecord's
// shape) plus the manifest coordinates of the item.
type tapeSweepLogRecord struct {
	At        string `json:"at"`
	Run       string `json:"run"`
	Op        string `json:"op"` // intent | deleted | gone | kept_* | deferred | error | aborted | archived | tombstone
	Namespace string `json:"ns,omitempty"`
	Path      string `json:"path,omitempty"`
	Bytes     int64  `json:"bytes,omitempty"`
	ModTime   string `json:"mtime,omitempty"`
	Error     string `json:"error,omitempty"`
	Sweep     string `json:"sweep,omitempty"`
	Manifest  string `json:"manifest,omitempty"`
	Computer  string `json:"computer,omitempty"`
	Sequence  uint64 `json:"sequence,omitempty"`
	Digest    string `json:"digest,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Count     int    `json:"count,omitempty"`
}

type tapeSweepLog struct {
	file     *os.File
	w        *bufio.Writer
	run      string
	manifest string
}

func (s *Service) openTapeSweepLog(run, manifest string) (*tapeSweepLog, error) {
	if err := os.MkdirAll(s.tapeSweepLogDir(), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(s.tapeSweepLogDir(), time.Now().UTC().Format("2006-01-02")+".jsonl"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	return &tapeSweepLog{file: f, w: bufio.NewWriter(f), run: run, manifest: manifest}, nil
}

func (l *tapeSweepLog) record(rec tapeSweepLogRecord) {
	rec.At = time.Now().UTC().Format(time.RFC3339Nano)
	rec.Run = l.run
	rec.Manifest = l.manifest
	line, _ := json.Marshal(rec)
	_, _ = l.w.Write(append(line, '\n'))
}

func (l *tapeSweepLog) item(op string, it tapeSweepItem, size int64, errText string) {
	l.record(tapeSweepLogRecord{Op: op, Sweep: it.Sweep, Namespace: it.Namespace, Path: it.Path, Bytes: size, ModTime: it.MTime, Computer: it.Computer, Sequence: it.Sequence, Digest: it.Digest, Reason: it.Reason, Error: errText})
}

func (l *tapeSweepLog) flush() error {
	if err := l.w.Flush(); err != nil {
		return err
	}
	return l.file.Sync()
}

func (l *tapeSweepLog) close() {
	_ = l.flush()
	_ = l.file.Close()
}

// tapeSweepApplyRun is the state one apply carries.
type tapeSweepApplyRun struct {
	p          tapeSweepParams
	header     tapeSweepManifestHeader
	manifestSH string
	cutoff     time.Time
	quiet      time.Duration
	started    time.Time
	archiveDir string
	registry   ComputerRegistry
	listed     map[string]bool
	routed     map[string]bool
	destroyErr string
	// skipped is the ids of computers apply left alone (a failed re-proof or
	// a spent cap), for a named apply to fail on.
	skipped []string
	pending map[string]bool
	groups  []*applyGroup
	// strict is set for the destroyed deletes: each batch of unlinks runs
	// under the tombstone gate and any outcome but deleted or gone stops the
	// run before the next delete.
	strict   bool
	nowHeads map[string]tapeHead
	delta    map[string]bool
	deltaOK  bool
	log      *tapeSweepLog
	counts   *TapeSweepApplyCounts
	deletes  int
	bytes    int64
	// onBatch, when set, runs after each batch of unlinks with that batch's
	// deleted files and bytes (the destroyed apply persists its tombstone
	// counts there, so a killed apply leaves a truthful tombstone).
	onBatch func(deleted int, bytes int64)
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// tapeSweepApply deletes the items of a verified dry-run manifest.
func (s *Service) tapeSweepApply(ctx context.Context, jobID string, p tapeSweepParams) (TapeSweepReport, error) {
	started := time.Now()
	policy, registry := s.tapeSweepPolicy()
	counts := &TapeSweepApplyCounts{}
	report := TapeSweepReport{RunID: p.runID, Action: p.action, Mode: policy.Mode, AcceptUnrecordedForkLineage: p.forkAck, Apply: counts, StartedAt: started.UTC()}
	defer func() { report.DurationMs = time.Since(started).Milliseconds() }()
	if policy.Mode != ArtifactGCModeActive {
		return report, errTapeSweepActiveNotConfigured
	}

	// The manifest must be the one the operator reviewed.
	s.setTapeSweepPhase(jobID, "verify-manifest")
	manifestPath := s.tapeSweepManifestPath(p.runID)
	digest, err := sha256File(manifestPath)
	if err != nil {
		return report, fmt.Errorf("tape sweep: manifest %s: %w", p.runID, err)
	}
	if digest != p.manifestSHA {
		return report, fmt.Errorf("tape sweep: manifest %s sha256 %s does not match the requested %s; nothing deleted", p.runID, digest, p.manifestSHA)
	}
	sidecar, err := os.ReadFile(manifestPath + ".sha256")
	if err != nil || !strings.HasPrefix(string(sidecar), digest) {
		return report, fmt.Errorf("tape sweep: manifest %s sha256 sidecar missing or mismatched; nothing deleted", p.runID)
	}
	report.ManifestPath, report.ManifestSHA256 = manifestPath, digest

	headerFile, err := os.Open(manifestPath)
	if err != nil {
		return report, err
	}
	first, err := bufio.NewReaderSize(headerFile, 1<<20).ReadBytes('\n')
	headerFile.Close()
	if err != nil && !(errors.Is(err, io.EOF) && len(first) > 0) {
		return report, fmt.Errorf("tape sweep: manifest header: %w", err)
	}
	run := &tapeSweepApplyRun{p: p, manifestSH: digest, started: started, archiveDir: policy.ArchiveDir, registry: registry, counts: counts}
	if err := json.Unmarshal(first, &run.header); err != nil || run.header.Kind != "header" || run.header.RunID != p.runID {
		return report, fmt.Errorf("tape sweep: manifest %s header invalid", p.runID)
	}
	report.Sweeps = run.header.Sweeps
	report.ComputerIDs = p.named
	report.ForkLineageAcknowledgedFor = p.named
	if !sameStringList(run.header.ForkLineageAcknowledgedFor, p.named) {
		return report, fmt.Errorf("tape sweep: the manifest's fork-lineage acknowledgement named %v; apply names %v; nothing deleted", run.header.ForkLineageAcknowledgedFor, p.named)
	}
	if !sameStringList(run.header.Computers, p.named) {
		return report, fmt.Errorf("tape sweep: the manifest was made for computers %v; apply names %v and must name exactly those; nothing deleted", run.header.Computers, p.named)
	}
	report.NoArchive = run.header.NoArchive
	if run.header.NoArchive != p.noArchive {
		return report, fmt.Errorf("tape sweep: the manifest was made with no_archive=%v; apply must repeat it exactly; nothing deleted", run.header.NoArchive)
	}
	if run.header.AcceptUnrecordedForkLineage && !p.forkAck {
		return report, errors.New("tape sweep: the manifest lists destroyed computers under accept_unrecorded_fork_lineage; apply must repeat it; nothing deleted")
	}
	grace, err := time.ParseDuration(run.header.Grace)
	if err != nil || grace < TapeSweepMinGrace {
		grace = TapeSweepMinGrace
	}
	if p.grace > grace {
		grace = p.grace
	}
	report.Grace = grace.String()
	run.cutoff = started.Add(-grace)
	// The quiet window re-checked is the dry-run's, never below grace.
	run.quiet, err = time.ParseDuration(run.header.Quiet)
	if err != nil {
		run.quiet = DefaultTapeSweepQuiet
	}
	run.quiet = max(run.quiet, grace)
	report.Quiet = run.quiet.String()

	// Delta references: rows appended since the snapshot heads, and the
	// extra records re-read now. A failure here refuses the whole apply.
	// A destroyed apply holds the tombstone gate (exclusive) from here to the
	// end of the proof and the tombstones: every path that adds a reference to
	// a computer (head CAS, snapshot-root registration, a lifecycle prepare)
	// takes it shared, so what is read here cannot change before the
	// tombstones refuse them.
	gateHeld := false
	unlockGate := func() {
		if gateHeld {
			gateHeld = false
			s.store.tombstones.gate.Unlock()
		}
	}
	if p.destroyed {
		s.store.tombstones.gate.Lock()
		gateHeld = true
		defer unlockGate()
	}
	s.setTapeSweepPhase(jobID, "delta-references")
	if err := s.tapeSweepDelta(ctx, run); err != nil {
		return report, err
	}
	if !run.deltaOK {
		report.Warnings = append(report.Warnings, "an event appended since the dry-run is unreadable; orphan payloads are kept")
	}
	if p.destroyed {
		if registry == nil {
			run.destroyErr = "computer registry (vmctl) is not configured"
		} else if ids, err := registry.ListComputerIDs(ctx); err != nil {
			run.destroyErr = "registry list: " + err.Error()
		} else {
			run.listed = map[string]bool{}
			for _, id := range ids {
				run.listed[strings.TrimSpace(id)] = true
			}
			report.RegistryComputers = len(ids)
			if reason := registryRefusal(ids, run.nowHeads); reason != "" {
				run.destroyErr = reason
			}
			if run.routed, err = s.tapeRoutedComputers(ctx); err != nil {
				run.destroyErr = err.Error()
			}
			if run.pending, err = s.computersWithPendingLifecycleOps(ctx); err != nil {
				run.destroyErr = err.Error()
			}
		}
		if !run.deltaOK && run.destroyErr == "" {
			// A living computer's event appended since the dry run cannot be
			// read, so what it references is unknown: no payload is deleted.
			run.destroyErr = "an event appended since the dry run is unreadable, so the live references are incomplete"
		}
		if run.destroyErr != "" {
			report.DestroyedRefused = run.destroyErr
			report.Warnings = append(report.Warnings, "destroyed apply refused: "+run.destroyErr)
		}
	}

	tlog, err := s.openTapeSweepLog(p.runID, digest)
	if err != nil {
		return report, fmt.Errorf("tape sweep: deletion log unavailable, nothing deleted: %w", err)
	}
	defer tlog.close()
	run.log = tlog
	tlog.record(tapeSweepLogRecord{Op: "apply_started", Reason: "job " + jobID})
	if err := tlog.flush(); err != nil {
		return report, fmt.Errorf("tape sweep: deletion log unavailable, nothing deleted: %w", err)
	}

	finish := func(failure error) (TapeSweepReport, error) {
		unlockGate()
		report.ReportPath = filepath.Join(s.tapeSweepLogDir(), "tape-sweep-"+p.runID+".apply-"+started.UTC().Format("20060102T150405Z")+".report.json")
		report.DurationMs = time.Since(started).Milliseconds()
		if failure != nil {
			report.Failure = failure.Error()
			tlog.record(tapeSweepLogRecord{Op: "apply_failed", Error: failure.Error(), Count: counts.Deleted, Bytes: counts.BytesDeleted})
		} else {
			tlog.record(tapeSweepLogRecord{Op: "apply_finished", Count: counts.Deleted, Bytes: counts.BytesDeleted})
		}
		_ = tlog.flush()
		if err := writeJSONFileSync(report.ReportPath, report); err != nil && failure == nil {
			return report, err
		}
		log.Printf("tape sweep: apply run=%s deleted=%d bytes=%d gone=%d kept_refreshed=%d kept_referenced=%d kept_not_destroyed=%d kept_unverified=%d deferred=%d errors=%d computers_deleted=%d computers_skipped=%d archives=%d failure=%v",
			p.runID, counts.Deleted, counts.BytesDeleted, counts.Gone, counts.KeptRefreshed, counts.KeptReferenced, counts.KeptNotDestroyed, counts.KeptUnverified, counts.DeferredByCap, counts.Errors, counts.ComputersDeleted, counts.ComputersSkipped, len(counts.Archives), failure)
		if failure != nil {
			return report, fmt.Errorf("%w (see the receipt %s)", failure, report.ReportPath)
		}
		return report, nil
	}

	// Pass 1: read the manifest's destroyed groups. Nothing is deleted yet.
	var orphans []tapeSweepItem
	if p.destroyed {
		_, err := s.walkApplyManifest(run, manifestPath, func(proof TapeSweepComputerProof, items []tapeSweepItem, dirs []string, rows map[string]int64) error {
			g := &applyGroup{proof: proof}
			for _, it := range items {
				if it.Namespace == tapeNSFileCASChunks {
					g.chunks = append(g.chunks, it.Digest)
				}
				if !isPurgeNamespace(it.Namespace) && run.delta[it.Digest] {
					continue
				}
				g.items++
				g.bytes += it.Size
			}
			g.onDisk = s.anyTapeItemOnDisk(items)
			run.groups = append(run.groups, g)
			return nil
		})
		if err != nil {
			return finish(err)
		}
		// Phase one, under the gate: prove every computer, then tombstone
		// every computer. One failure refuses the whole run.
		s.setTapeSweepPhase(jobID, "destroyed-proof")
		refusal, perr := s.proveDestroyedRun(ctx, run)
		unlockGate()
		if perr != nil {
			return finish(perr)
		}
		if refusal != "" {
			ids := make([]string, 0, len(run.groups))
			for _, g := range run.groups {
				ids = append(ids, g.proof.ComputerID)
				counts.KeptNotDestroyed += g.items
			}
			counts.ComputersSkipped += len(run.groups)
			report.Warnings = append(report.Warnings, "destroyed apply refused, nothing deleted: "+refusal)
			tlog.record(tapeSweepLogRecord{Op: "kept_not_destroyed", Sweep: TapeSweepDestroyed, Reason: refusal, Count: len(run.groups)})
			if len(p.named) > 0 {
				return finish(fmt.Errorf("tape sweep: apply refused, nothing deleted: %s", refusal))
			}
			log.Printf("tape sweep: destroyed apply refused for %v: %s", ids, refusal)
			run.groups = nil
		}
	}
	if len(run.groups) > 0 {
		// Phase two: archive and delete one computer at a time, stopping at
		// the first thing that is not a delete. A cap reached leaves the later
		// computers tombstoned and untouched; the named apply fails on them.
		run.strict = true
		if _, err := s.walkApplyManifest(run, manifestPath, func(proof TapeSweepComputerProof, items []tapeSweepItem, dirs []string, rows map[string]int64) error {
			return s.applyDestroyedComputer(ctx, jobID, run, proof, items, dirs, rows)
		}); err != nil {
			run.strict = false
			if run.counts.Deleted == 0 {
				// Nothing was unlinked anywhere: a tombstone this run wrote
				// and that never reached deleting is dropped.
				s.dropCreatedTombstones(ctx, run)
			}
			return finish(err)
		}
		run.strict = false
		if len(p.named) > 0 && len(run.skipped) > 0 {
			return finish(fmt.Errorf("tape sweep: named computers not deleted: %v", run.skipped))
		}
	}
	if p.orphans {
		var err error
		orphans, err = s.walkApplyManifest(run, manifestPath, func(TapeSweepComputerProof, []tapeSweepItem, []string, map[string]int64) error { return nil })
		if err != nil {
			return finish(err)
		}
	}
	if len(orphans) > 0 {
		if err := s.applyOrphans(ctx, jobID, run, orphans); err != nil {
			return finish(err)
		}
	}
	return finish(nil)
}

// applyGroup is one destroyed computer of the manifest as the apply holds it
// between its passes: the proof, what it will delete, and its tombstone.
type applyGroup struct {
	proof   TapeSweepComputerProof
	items   int
	bytes   int64
	chunks  []string
	onDisk  bool
	tomb    TapeTombstone
	hadTomb bool
	created bool
}

func (run *tapeSweepApplyRun) group(id string) *applyGroup {
	for _, g := range run.groups {
		if g.proof.ComputerID == id {
			return g
		}
	}
	return nil
}

// walkApplyManifest streams the manifest after its header, calling onGroup for
// each destroyed computer's items, directories and row counts, and returns
// the orphan items when the run asked for them. It validates every line.
func (s *Service) walkApplyManifest(run *tapeSweepApplyRun, manifestPath string, onGroup func(TapeSweepComputerProof, []tapeSweepItem, []string, map[string]int64) error) ([]tapeSweepItem, error) {
	p := run.p
	mf, err := os.Open(manifestPath)
	if err != nil {
		return nil, err
	}
	defer mf.Close()
	reader := bufio.NewReaderSize(mf, 1<<20)
	readLine := func() ([]byte, error) {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF && len(line) > 0 {
			return line, nil
		}
		return line, err
	}
	if _, err := readLine(); err != nil {
		return nil, fmt.Errorf("tape sweep: manifest header: %w", err)
	}
	var proof *TapeSweepComputerProof
	var group, orphans []tapeSweepItem
	var groupDirs []string
	var groupRows map[string]int64
	flushGroup := func() error {
		if proof == nil {
			return nil
		}
		err := onGroup(*proof, group, groupDirs, groupRows)
		proof, group, groupDirs, groupRows = nil, nil, nil, nil
		return err
	}
	for {
		line, err := readLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tape sweep: read manifest: %w", err)
		}
		var kind struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(line, &kind); err != nil {
			return nil, fmt.Errorf("tape sweep: manifest line invalid: %w", err)
		}
		switch kind.Kind {
		case "computer":
			if err := flushGroup(); err != nil {
				return nil, err
			}
			var c tapeSweepManifestComputer
			if err := json.Unmarshal(line, &c); err != nil {
				return nil, fmt.Errorf("tape sweep: manifest computer invalid: %w", err)
			}
			if len(p.named) > 0 && !slices.Contains(p.named, c.Proof.ComputerID) {
				return nil, fmt.Errorf("tape sweep: manifest lists computer %s, which this named apply does not name; nothing further deleted", c.Proof.ComputerID)
			}
			proof = &c.Proof
			groupRows = map[string]int64{}
		case "dir":
			var d tapeSweepDirLine
			if err := json.Unmarshal(line, &d); err != nil || proof == nil || proof.ComputerID != d.Computer || !validPurgeDir(d.Computer, d.Path) {
				return nil, fmt.Errorf("tape sweep: manifest dir line invalid")
			}
			groupDirs = append(groupDirs, d.Path)
		case "rows":
			var r tapeSweepRowsLine
			if err := json.Unmarshal(line, &r); err != nil || proof == nil || proof.ComputerID != r.Computer || !tapePurgeTableKnown(r.Table) || r.Count <= 0 {
				return nil, fmt.Errorf("tape sweep: manifest rows line invalid")
			}
			groupRows[r.Table] = r.Count
		case "item":
			var it tapeSweepItem
			if err := json.Unmarshal(line, &it); err != nil {
				return nil, fmt.Errorf("tape sweep: manifest item invalid: %w", err)
			}
			if !validTapeSweepItem(it) {
				return nil, fmt.Errorf("tape sweep: manifest item %q invalid", it.Path)
			}
			switch it.Sweep {
			case TapeSweepDestroyed:
				if proof == nil || proof.ComputerID != it.Computer {
					return nil, fmt.Errorf("tape sweep: manifest item %s outside its computer group", it.Path)
				}
				group = append(group, it)
			case TapeSweepOrphans:
				if err := flushGroup(); err != nil {
					return nil, err
				}
				if p.orphans {
					orphans = append(orphans, it)
				}
			}
		default:
			return nil, fmt.Errorf("tape sweep: manifest line kind %q unknown", kind.Kind)
		}
	}
	if err := flushGroup(); err != nil {
		return nil, err
	}
	return orphans, nil
}

func validTapeSweepItem(it tapeSweepItem) bool {
	if isPurgeNamespace(it.Namespace) {
		return validPurgeItem(it)
	}
	ok := false
	for _, ns := range tapeNamespaces {
		ok = ok || it.Namespace == ns
	}
	return ok && validHexDigest(it.Digest) && it.Path == tapeSweepRel(it.Namespace, it.Digest)
}

// tapeSweepDelta reads every append landed since the manifest's snapshot
// heads and the extra records, into run.delta (full digests).
func (s *Service) tapeSweepDelta(ctx context.Context, run *tapeSweepApplyRun) error {
	heads, err := s.tapeHeads(ctx)
	if err != nil {
		return err
	}
	run.nowHeads = heads
	run.delta = map[string]bool{}
	run.deltaOK = true
	for _, id := range sortedHeadIDs(heads) {
		now := heads[id]
		snap := run.header.Heads[id]
		if now.Sequence < snap.Sequence {
			return fmt.Errorf("tape sweep: computer %s head %d is below the manifest's %d", id, now.Sequence, snap.Sequence)
		}
		if now.Sequence == snap.Sequence {
			continue
		}
		err := s.walkTapeChain(ctx, id, snap.Sequence, now.Sequence, true, func(row tapeChainRow) error {
			run.delta[row.EventDigest] = true
			run.delta[row.EventPin] = true
			for _, d := range row.PayloadPins {
				run.delta[d] = true
			}
			for _, d := range row.PayloadRefs {
				run.delta[d] = true
			}
			if !row.EventReadable {
				run.deltaOK = false
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("tape sweep: delta references: %w", err)
		}
	}
	return s.tapeExtraRefs(ctx, func(text string) {
		for _, d := range tapeHexRefs(text) {
			run.delta[d] = true
		}
	})
}

// reproveDestroyed re-checks the destroyed proof at delete time.
func (s *Service) reproveDestroyed(ctx context.Context, run *tapeSweepApplyRun, proof TapeSweepComputerProof) string {
	id := proof.ComputerID
	if err := tapeSweepProtected(id); err != nil {
		return "protected computer: hard refusal list"
	}
	if run.destroyErr != "" {
		return run.destroyErr
	}
	if !run.p.forkAck {
		return "fork lineage unrecorded; not acknowledged"
	}
	if run.listed[id] {
		return "now in registry list"
	}
	if run.routed[id] {
		return "now has a route slot"
	}
	if run.pending[id] {
		return "a lifecycle operation is pending"
	}
	found, err := run.registry.LookupComputer(ctx, id)
	if err != nil {
		return "registry lookup failed: " + err.Error()
	}
	if found {
		return "ownership record exists"
	}
	snap, ok := run.header.Heads[id]
	now, nowOK := run.nowHeads[id]
	if !ok || !nowOK || now.Sequence != snap.Sequence || now.Canonical != snap.Canonical || now.Sequence != proof.HeadSequence {
		return "head changed since the dry-run"
	}
	if run.started.Sub(now.UpdatedAt) < run.quiet {
		return "head moved within the quiet window"
	}
	if reason, err := s.tapePurgeReferencedElsewhere(ctx, id); err != nil {
		return "cross-reference check failed: " + err.Error()
	} else if reason != "" {
		return reason
	}
	return ""
}

// proveDestroyedRun is phase one of a destroyed apply, run under the exclusive
// tombstone gate. It deletes nothing. Every computer of the manifest is
// re-proved; the chunk digests of all of them are checked against every
// computer outside the manifest; the caps are checked against the manifest's
// totals; then every tombstone is written and every head re-read. A non-empty
// refusal (or an error) means no computer is deleted, and the tombstones this
// run wrote are dropped.
func (s *Service) proveDestroyedRun(ctx context.Context, run *tapeSweepApplyRun) (string, error) {
	if len(run.groups) == 0 {
		return "", nil
	}
	refuse := func(reason string) (string, error) {
		s.dropCreatedTombstones(ctx, run)
		return reason, nil
	}
	doomed := make([]string, 0, len(run.groups))
	for _, g := range run.groups {
		id := g.proof.ComputerID
		doomed = append(doomed, id)
		if reason := s.reproveDestroyed(ctx, run, g.proof); reason != "" {
			return refuse(id + ": " + reason)
		}
		tomb, has, err := s.store.TapeTombstone(ctx, id)
		if err != nil {
			return "", err
		}
		g.tomb, g.hadTomb = tomb, has
		if has && tomb.Status == tapeTombstoneDeleted && g.onDisk {
			return refuse(id + ": tombstone already deleted; files remain")
		}
	}
	// Chunks: a digest any computer outside this run names (a snapshot root,
	// a registration in flight, a file root) is not deleted; an unreadable
	// answer fails the run.
	var anyChunks bool
	for _, g := range run.groups {
		anyChunks = anyChunks || len(g.chunks) > 0
	}
	if anyChunks {
		living, err := s.livingChunkDigests(ctx, doomed)
		if err != nil {
			return "", fmt.Errorf("tape sweep: snapshot chunk check failed, nothing deleted: %w", err)
		}
		for _, g := range run.groups {
			n := 0
			for _, d := range g.chunks {
				if living[d] {
					n++
				}
			}
			if n > 0 {
				return refuse(fmt.Sprintf("%s: %d snapshot chunk digests are named by a snapshot, an in-flight registration or a file root of a computer this run does not delete", g.proof.ComputerID, n))
			}
		}
	}
	// Every tombstone, then every head again: no append can land after this.
	for _, g := range run.groups {
		id, proof := g.proof.ComputerID, g.proof
		tomb := TapeTombstone{ComputerID: id, Status: tapeTombstoneArchiving, HeadSequence: proof.HeadSequence, CanonicalEventHead: proof.CanonicalEventHead, ManifestRun: run.p.runID, ManifestSHA256: run.manifestSH}
		if g.hadTomb {
			tomb = g.tomb
			tomb.ManifestRun, tomb.ManifestSHA256 = run.p.runID, run.manifestSH
		}
		proofJSON, _ := json.Marshal(proof)
		tomb.ProofJSON = string(proofJSON)
		if err := s.store.writeTapeTombstone(ctx, tomb); err != nil {
			run.counts.Errors++
			s.dropCreatedTombstones(ctx, run)
			return "", err
		}
		g.tomb = tomb
		if !g.hadTomb {
			g.created = true
			run.counts.ComputersTombstoned++
		}
		head, herr := readComputerEventHead(ctx, s.store.db, id, false)
		if herr != nil || head == nil || head.Sequence != proof.HeadSequence || head.CanonicalEventHead != proof.CanonicalEventHead {
			return refuse(id + ": head changed at tombstone")
		}
		run.log.record(tapeSweepLogRecord{Op: "tombstone", Sweep: TapeSweepDestroyed, Computer: id, Reason: tomb.Status, Count: g.items})
	}
	_ = run.log.flush()
	return "", nil
}

// dropCreatedTombstones removes the tombstones this run created, when no file
// of theirs was touched. A tombstone that already existed, or that reached
// deleting, is never dropped.
func (s *Service) dropCreatedTombstones(ctx context.Context, run *tapeSweepApplyRun) {
	for _, g := range run.groups {
		if !g.created {
			continue
		}
		if t, ok, _ := s.store.TapeTombstone(ctx, g.proof.ComputerID); ok && t.FilesDeleted == 0 && t.Status != tapeTombstoneDeleted && t.Status != tapeTombstoneDeleting {
			if err := s.store.removeTapeTombstone(ctx, g.proof.ComputerID); err != nil {
				log.Printf("tape sweep: computer=%s remove tombstone: %v", g.proof.ComputerID, err)
				continue
			}
			g.created = false
		}
	}
}

// archiveDestroyedComputer archives one computer before any computer is
// deleted (unless the request said no_archive), and moves its tombstone to
// archived.
func (s *Service) archiveDestroyedComputer(ctx context.Context, run *tapeSweepApplyRun, proof TapeSweepComputerProof, items []tapeSweepItem) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id := proof.ComputerID
	g := run.group(id)
	if g == nil {
		return fmt.Errorf("tape sweep: computer %s is not in the proved run", id)
	}
	tomb := g.tomb
	if !run.p.noArchive {
		extras, err := s.tapePurgeRowsDump(ctx, id)
		var archive TapeArchiveSidecar
		if err == nil {
			archive, err = s.ensureTapeArchive(ctx, run, id, items, extras...)
		}
		if err != nil {
			run.counts.Errors++
			run.log.record(tapeSweepLogRecord{Op: "error", Sweep: TapeSweepDestroyed, Computer: id, Error: "archive: " + err.Error()})
			_ = run.log.flush()
			return fmt.Errorf("tape sweep: computer %s: archive failed, nothing deleted: %w", id, err)
		}
		run.counts.Archives = append(run.counts.Archives, archive.Archive)
		tomb.ArchivePath, tomb.ArchiveSHA256, tomb.ArchiveManifestSHA256 = archive.Archive, archive.ArchiveSHA256, archive.ManifestSHA256
	} else if tomb.ArchivePath == "" {
		tomb.ArchivePath = "none (no_archive)"
	}
	if tomb.Status == tapeTombstoneArchiving {
		tomb.Status = tapeTombstoneArchived
	}
	if err := s.store.putTapeTombstone(ctx, tomb); err != nil {
		return err
	}
	g.tomb = tomb
	return nil
}

// applyDestroyedComputer deletes one proved, tombstoned and archived
// computer. The first thing that is not a delete stops the run with an error.
func (s *Service) applyDestroyedComputer(ctx context.Context, jobID string, run *tapeSweepApplyRun, proof TapeSweepComputerProof, items []tapeSweepItem, dirs []string, rows map[string]int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	id := proof.ComputerID
	g := run.group(id)
	if g == nil {
		return fmt.Errorf("tape sweep: computer %s is not in the proved run", id)
	}
	// A cap already spent: nothing of this computer is archived or deleted
	// (its tombstone stays, so no reference can be added to it).
	if g.items > 0 && (run.deletes >= run.p.maxDeletes || run.bytes >= run.p.maxBytes) {
		run.counts.DeferredByCap += g.items
		run.counts.ComputersSkipped++
		run.skipped = append(run.skipped, id)
		return nil
	}
	if err := s.archiveDestroyedComputer(ctx, run, proof, items); err != nil {
		return err
	}
	s.setTapeSweepPhase(jobID, "destroyed "+id)
	tomb := g.tomb
	// deleting is durable before the first unlink: a tombstone in this state
	// is never removed, whatever its counts say.
	tomb.Status = tapeTombstoneDeleting
	if err := s.store.putTapeTombstone(ctx, tomb); err != nil {
		return err
	}
	baseFiles, baseBytes, batchFiles, batchBytes := tomb.FilesDeleted, tomb.BytesDeleted, int64(0), int64(0)
	run.onBatch = func(n int, b int64) {
		batchFiles += int64(n)
		batchBytes += b
		tomb.FilesDeleted, tomb.BytesDeleted = baseFiles+batchFiles, baseBytes+batchBytes
		if perr := s.store.putTapeTombstone(context.WithoutCancel(ctx), tomb); perr != nil {
			log.Printf("tape sweep: computer=%s tombstone counts: %v", id, perr)
		}
	}
	deleted, bytesDeleted, complete, err := s.deleteTapeItems(ctx, run, items)
	run.onBatch = nil
	tomb.FilesDeleted, tomb.BytesDeleted = baseFiles+int64(deleted), baseBytes+bytesDeleted
	if err == nil && !complete {
		// A cap bit mid computer: its rows and directories stay for the next run.
		run.skipped = append(run.skipped, id)
		run.counts.ComputersSkipped++
		tomb.Status = tapeTombstoneArchived
		g.tomb = tomb
		_ = s.store.putTapeTombstone(context.WithoutCancel(ctx), tomb)
		return nil
	}
	if err == nil && !s.removePurgeDirs(run, id, dirs) {
		err = fmt.Errorf("a directory of the computer could not be removed")
	}
	if err == nil {
		removed, rerr := s.purgeComputerRows(ctx, run, id, rows)
		if rerr != nil {
			run.counts.Errors++
			run.log.record(tapeSweepLogRecord{Op: "error", Sweep: TapeSweepDestroyed, Computer: id, Error: "rows: " + rerr.Error()})
			_ = run.log.flush()
			err = fmt.Errorf("rows not purged: %w", rerr)
		} else {
			for table, n := range removed {
				run.counts.Classes = addClass(run.counts.Classes, "rows:"+table, 0, 0, n)
				run.log.record(tapeSweepLogRecord{Op: "deleted_rows", Sweep: TapeSweepDestroyed, Computer: id, Namespace: table, Count: int(n)})
			}
			_ = run.log.flush()
		}
	}
	if err == nil {
		tomb.Status = tapeTombstoneDeleted
		run.counts.ComputersDeleted++
	}
	g.tomb = tomb
	if perr := s.store.putTapeTombstone(context.WithoutCancel(ctx), tomb); perr != nil && err == nil {
		err = perr
	}
	log.Printf("tape sweep: computer=%s deleted=%d bytes=%d complete=%v", id, deleted, bytesDeleted, err == nil)
	if err != nil {
		run.skipped = append(run.skipped, id)
		return fmt.Errorf("tape sweep: computer %s not fully deleted, run stopped: %w", id, err)
	}
	return nil
}

// applyOrphans archives and deletes the orphan items still unreferenced.
func (s *Service) applyOrphans(ctx context.Context, jobID string, run *tapeSweepApplyRun, items []tapeSweepItem) error {
	s.setTapeSweepPhase(jobID, "orphans")
	var keep []tapeSweepItem
	for _, it := range items {
		switch {
		case run.delta[it.Digest]:
			run.counts.KeptReferenced++
			run.log.item("kept_referenced", it, it.Size, "")
		case it.Namespace == tapeNSPayload && !run.deltaOK:
			run.counts.KeptUnverified++
			run.log.item("kept_unverified", it, it.Size, "")
		default:
			keep = append(keep, it)
		}
	}
	if err := run.log.flush(); err != nil {
		return err
	}
	if len(keep) == 0 {
		return nil
	}
	archive, err := s.ensureTapeArchive(ctx, run, "orphans", keep)
	if err != nil {
		run.counts.Errors++
		run.log.record(tapeSweepLogRecord{Op: "error", Sweep: TapeSweepOrphans, Error: "archive: " + err.Error()})
		_ = run.log.flush()
		return fmt.Errorf("tape sweep: orphan archive failed, nothing deleted: %w", err)
	}
	run.counts.Archives = append(run.counts.Archives, archive.Archive)
	deleted, bytesDeleted, _, err := s.deleteTapeItems(ctx, run, keep)
	log.Printf("tape sweep: orphans deleted=%d bytes=%d archive=%s", deleted, bytesDeleted, archive.Archive)
	return err
}

// deleteTapeItems unlinks items in bounded batches: intent records are made
// durable first, then each item is re-checked (delta reference, file still
// regular and past grace) and unlinked under the exclusive CAS reuse lock.
// complete reports that no item was deferred, kept as unverified, or failed.
func (s *Service) deleteTapeItems(ctx context.Context, run *tapeSweepApplyRun, items []tapeSweepItem) (int, int64, bool, error) {
	deleted, bytesDeleted := 0, int64(0)
	complete := true
	for start := 0; start < len(items); start += run.p.batch {
		if err := ctx.Err(); err != nil {
			run.log.record(tapeSweepLogRecord{Op: "aborted", Error: err.Error()})
			_ = run.log.flush()
			return deleted, bytesDeleted, false, err
		}
		end := min(len(items), start+run.p.batch)
		var batch []tapeSweepItem
		for _, it := range items[start:end] {
			if !isPurgeNamespace(it.Namespace) && run.delta[it.Digest] {
				run.counts.KeptReferenced++
				run.log.item("kept_referenced", it, it.Size, "")
				continue
			}
			if run.deletes+1 > run.p.maxDeletes || run.bytes+it.Size > run.p.maxBytes {
				if it.Size > run.p.maxBytes {
					log.Printf("tape sweep: computer=%s item %s size=%d exceeds max_bytes=%d; it can never fit one apply", it.Computer, it.Path, it.Size, run.p.maxBytes)
				}
				run.counts.DeferredByCap++
				complete = false
				continue
			}
			run.deletes++
			run.bytes += it.Size
			batch = append(batch, it)
		}
		if len(batch) == 0 {
			continue
		}
		for _, it := range batch {
			run.log.item("intent", it, it.Size, "")
		}
		if err := run.log.flush(); err != nil {
			return deleted, bytesDeleted, false, fmt.Errorf("tape sweep: deletion log write failed, batch not deleted: %w", err)
		}
		batchDeleted, batchBytesDeleted := deleted, bytesDeleted
		// A destroyed delete holds the tombstone gate for the batch: nothing
		// that adds a reference to a computer runs between its re-check and
		// its unlink.
		if run.strict {
			s.store.tombstones.gate.Lock()
		}
		var stop error
		for _, it := range batch {
			if hook := s.tapeSweep.beforeDelete; hook != nil {
				hook(it)
			}
			path := filepath.Join(s.artifactsRoot, filepath.FromSlash(it.Path))
			op, size, uerr := s.unlinkIfStillUnreachable(ctx, gcCandidate{ns: it.Namespace, rel: it.Path, path: path, size: it.Size, destroyedPurge: isPurgeNamespace(it.Namespace)}, run.cutoff)
			errText := ""
			switch op {
			case "deleted":
				deleted++
				bytesDeleted += size
				run.counts.Deleted++
				run.counts.BytesDeleted += size
				run.counts.Classes = addClass(run.counts.Classes, it.Namespace, 1, size, 0)
			case "gone":
				run.counts.Gone++
			case "kept_refreshed":
				run.counts.KeptRefreshed++
			default:
				run.counts.Errors++
				complete = false
				if uerr != nil {
					errText = uerr.Error()
				}
			}
			run.log.item(op, it, size, errText)
			if run.strict && op != "deleted" && op != "gone" {
				stop = fmt.Errorf("%s was not deleted (%s%s); the run stops before the next delete", it.Path, op, func() string {
					if errText != "" {
						return ": " + errText
					}
					return ""
				}())
				run.log.record(tapeSweepLogRecord{Op: "aborted", Computer: it.Computer, Error: stop.Error()})
				break
			}
		}
		if run.strict {
			s.store.tombstones.gate.Unlock()
		}
		if run.onBatch != nil && deleted > batchDeleted {
			run.onBatch(deleted-batchDeleted, bytesDeleted-batchBytesDeleted)
		}
		if stop != nil {
			_ = run.log.flush()
			return deleted, bytesDeleted, false, stop
		}
		if err := run.log.flush(); err != nil {
			return deleted, bytesDeleted, false, fmt.Errorf("tape sweep: deletion log write failed: %w", err)
		}
		if run.p.pause > 0 && end < len(items) {
			select {
			case <-ctx.Done():
			case <-time.After(run.p.pause):
			}
		}
	}
	return deleted, bytesDeleted, complete, nil
}

// --- archive -------------------------------------------------------------

// TapeArchiveSidecar is <archive>.json: what the archive holds and its hashes.
type TapeArchiveSidecar struct {
	Archive        string `json:"archive"`
	ArchiveSHA256  string `json:"archive_sha256"`
	ManifestSHA256 string `json:"manifest_sha256"` // of the embedded MANIFEST.jsonl
	Entries        int    `json:"entries"`
	Bytes          int64  `json:"bytes"`
	Scope          string `json:"scope"` // computer id, or "orphans"
	RunID          string `json:"run_id"`
	RunManifest    string `json:"run_manifest_sha256"`
	CreatedAt      string `json:"created_at"`
}

type tapeArchiveEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

const tapeArchiveManifestName = "MANIFEST.jsonl"

// ensureTapeArchive returns a verified archive of the items present on disk,
// reusing this run's archive when its sidecar verifies (a resumed apply).
func (s *Service) ensureTapeArchive(ctx context.Context, run *tapeSweepApplyRun, scope string, items []tapeSweepItem, extras ...tapeArchiveExtra) (TapeArchiveSidecar, error) {
	if err := os.MkdirAll(run.archiveDir, 0o700); err != nil {
		return TapeArchiveSidecar{}, err
	}
	path := filepath.Join(run.archiveDir, scope+"-"+run.p.runID+".tar.zst")
	if raw, err := os.ReadFile(path + ".json"); err == nil {
		var sc TapeArchiveSidecar
		if json.Unmarshal(raw, &sc) == nil && sc.Archive == path {
			if err := verifyTapeArchive(ctx, path, sc); err == nil {
				return sc, nil
			}
		}
	}
	sc, err := s.writeTapeArchive(ctx, path, scope, run, items, extras)
	if err != nil {
		_ = os.Remove(path + ".tmp")
		return TapeArchiveSidecar{}, err
	}
	if err := verifyTapeArchive(ctx, path, sc); err != nil {
		return TapeArchiveSidecar{}, err
	}
	if err := writeJSONFileSync(path+".json", sc); err != nil {
		return TapeArchiveSidecar{}, err
	}
	run.log.record(tapeSweepLogRecord{Op: "archived", Path: path, Count: sc.Entries, Bytes: sc.Bytes, Reason: scope, Digest: sc.ArchiveSHA256})
	return sc, run.log.flush()
}

func (s *Service) writeTapeArchive(ctx context.Context, path, scope string, run *tapeSweepApplyRun, items []tapeSweepItem, extras []tapeArchiveExtra) (TapeArchiveSidecar, error) {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return TapeArchiveSidecar{}, err
	}
	defer f.Close()
	archiveHash := sha256.New()
	zw, err := zstd.NewWriter(io.MultiWriter(f, archiveHash))
	if err != nil {
		return TapeArchiveSidecar{}, err
	}
	tw := tar.NewWriter(zw)
	var manifest bytes.Buffer
	sc := TapeArchiveSidecar{Archive: path, Scope: scope, RunID: run.p.runID, RunManifest: run.manifestSH, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	for _, it := range items {
		if err := ctx.Err(); err != nil {
			return sc, err
		}
		full := filepath.Join(s.artifactsRoot, filepath.FromSlash(it.Path))
		raw, err := os.ReadFile(full)
		if os.IsNotExist(err) {
			continue // already gone (resumed run); nothing to archive
		}
		if err != nil {
			return sc, fmt.Errorf("read %s: %w", it.Path, err)
		}
		info, err := os.Lstat(full)
		if err != nil {
			return sc, err
		}
		if err := tw.WriteHeader(&tar.Header{Name: it.Path, Mode: 0o600, Size: int64(len(raw)), ModTime: info.ModTime(), Typeflag: tar.TypeReg}); err != nil {
			return sc, err
		}
		if _, err := tw.Write(raw); err != nil {
			return sc, err
		}
		sum := sha256.Sum256(raw)
		line, _ := json.Marshal(tapeArchiveEntry{Path: it.Path, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(raw))})
		manifest.Write(append(line, '\n'))
		sc.Entries++
		sc.Bytes += int64(len(raw))
	}
	for _, ex := range extras {
		if err := tw.WriteHeader(&tar.Header{Name: ex.Name, Mode: 0o600, Size: int64(len(ex.Body)), ModTime: time.Now(), Typeflag: tar.TypeReg}); err != nil {
			return sc, err
		}
		if _, err := tw.Write(ex.Body); err != nil {
			return sc, err
		}
		sum := sha256.Sum256(ex.Body)
		line, _ := json.Marshal(tapeArchiveEntry{Path: ex.Name, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(ex.Body))})
		manifest.Write(append(line, '\n'))
		sc.Entries++
		sc.Bytes += int64(len(ex.Body))
	}
	manifestSum := sha256.Sum256(manifest.Bytes())
	sc.ManifestSHA256 = hex.EncodeToString(manifestSum[:])
	if err := tw.WriteHeader(&tar.Header{Name: tapeArchiveManifestName, Mode: 0o600, Size: int64(manifest.Len()), ModTime: time.Now(), Typeflag: tar.TypeReg}); err != nil {
		return sc, err
	}
	if _, err := tw.Write(manifest.Bytes()); err != nil {
		return sc, err
	}
	if err := tw.Close(); err != nil {
		return sc, err
	}
	if err := zw.Close(); err != nil {
		return sc, err
	}
	if err := f.Sync(); err != nil {
		return sc, err
	}
	if err := f.Close(); err != nil {
		return sc, err
	}
	sc.ArchiveSHA256 = hex.EncodeToString(archiveHash.Sum(nil))
	return sc, os.Rename(tmp, path)
}

// verifyTapeArchive re-reads the archive: its hash must match the sidecar,
// every entry must hash to its MANIFEST.jsonl line, and every tape entry's
// content must hash to its content-addressed name.
func verifyTapeArchive(ctx context.Context, path string, sc TapeArchiveSidecar) error {
	digest, err := sha256File(path)
	if err != nil {
		return err
	}
	if digest != sc.ArchiveSHA256 {
		return fmt.Errorf("archive %s sha256 %s, sidecar %s", path, digest, sc.ArchiveSHA256)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zr, err := zstd.NewReader(f)
	if err != nil {
		return err
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	seen := map[string]string{}
	var manifest []byte
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("archive %s: %w", path, err)
		}
		raw, err := io.ReadAll(tr)
		if err != nil {
			return fmt.Errorf("archive %s: %w", path, err)
		}
		if hdr.Name == tapeArchiveManifestName {
			manifest = raw
			continue
		}
		sum := sha256.Sum256(raw)
		got := hex.EncodeToString(sum[:])
		parts := strings.Split(hdr.Name, "/")
		switch {
		case len(parts) == 4 && parts[0] == "sha256" && parts[1] == tapeNSFileCASChunks:
			// A sealed chunk is addressed by the hash of its bytes.
			if parts[3] != got {
				return fmt.Errorf("archive %s: entry %s does not hash to its name", path, hdr.Name)
			}
		case len(parts) == 4 && parts[0] == "sha256" && parts[1] == tapeNSFileCASRoots:
			// verified by the embedded manifest hash
		case len(parts) >= 3 && parts[0] == tapestore.SegDirName:
			// verified by the embedded manifest hash
		case len(parts) == 2 && parts[0] == "rows":
			// verified by the embedded manifest hash
		default:
			if len(parts) != 3 || parts[0] != "sha256" {
				return fmt.Errorf("archive %s: unexpected entry %q", path, hdr.Name)
			}
			name, ok := tapeSweepFileName(parts[1], parts[2])
			if !ok || name != got {
				return fmt.Errorf("archive %s: entry %s does not hash to its name", path, hdr.Name)
			}
		}
		seen[hdr.Name] = got
	}
	if manifest == nil {
		return fmt.Errorf("archive %s: %s missing", path, tapeArchiveManifestName)
	}
	ms := sha256.Sum256(manifest)
	if hex.EncodeToString(ms[:]) != sc.ManifestSHA256 {
		return fmt.Errorf("archive %s: embedded manifest hash mismatch", path)
	}
	entries := 0
	for _, line := range bytes.Split(bytes.TrimSpace(manifest), []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var e tapeArchiveEntry
		if err := json.Unmarshal(line, &e); err != nil {
			return err
		}
		if seen[e.Path] != e.SHA256 {
			return fmt.Errorf("archive %s: entry %s missing or changed", path, e.Path)
		}
		entries++
	}
	if entries != len(seen) || entries != sc.Entries {
		return fmt.Errorf("archive %s: %d entries, manifest %d, sidecar %d", path, len(seen), entries, sc.Entries)
	}
	return nil
}

// pruneTapeSweepArchives removes archives (and sidecars) older than the
// requested age, never younger than TapeSweepMinArchiveAge. It is explicit
// only: nothing prunes archives automatically.
func (s *Service) pruneTapeSweepArchives(p tapeSweepParams) (TapeSweepReport, error) {
	policy, _ := s.tapeSweepPolicy()
	counts := &TapeSweepApplyCounts{}
	report := TapeSweepReport{Action: p.action, Mode: policy.Mode, Apply: counts, StartedAt: time.Now().UTC()}
	entries, err := os.ReadDir(policy.ArchiveDir)
	if os.IsNotExist(err) {
		return report, nil
	}
	if err != nil {
		return report, err
	}
	cutoff := time.Now().Add(-p.olderThan)
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), ".tar.zst") {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		path := filepath.Join(policy.ArchiveDir, e.Name())
		if err := os.Remove(path); err != nil {
			counts.Errors++
			continue
		}
		_ = os.Remove(path + ".json")
		counts.ArchivesPruned++
		counts.BytesDeleted += info.Size()
		log.Printf("tape sweep: pruned archive %s bytes=%d", path, info.Size())
	}
	return report, nil
}

// anyTapeItemOnDisk reports whether any listed item still exists.
func (s *Service) anyTapeItemOnDisk(items []tapeSweepItem) bool {
	for _, it := range items {
		if _, err := os.Lstat(filepath.Join(s.artifactsRoot, filepath.FromSlash(it.Path))); err == nil {
			return true
		}
	}
	return false
}
