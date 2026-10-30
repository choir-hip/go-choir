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
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
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
	nowHeads   map[string]tapeHead
	delta      map[string]bool
	deltaOK    bool
	log        *tapeSweepLog
	counts     *TapeSweepApplyCounts
	deletes    int
	bytes      int64
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

	mf, err := os.Open(manifestPath)
	if err != nil {
		return report, err
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
	first, err := readLine()
	if err != nil {
		return report, fmt.Errorf("tape sweep: manifest header: %w", err)
	}
	run := &tapeSweepApplyRun{p: p, manifestSH: digest, started: started, archiveDir: policy.ArchiveDir, registry: registry, counts: counts}
	if err := json.Unmarshal(first, &run.header); err != nil || run.header.Kind != "header" || run.header.RunID != p.runID {
		return report, fmt.Errorf("tape sweep: manifest %s header invalid", p.runID)
	}
	report.Sweeps = run.header.Sweeps
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
			if run.routed, err = s.tapeRoutedComputers(ctx); err != nil {
				run.destroyErr = err.Error()
			}
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

	// Stream the manifest: one destroyed computer at a time, then orphans.
	var proof *TapeSweepComputerProof
	var group, orphans []tapeSweepItem
	flushGroup := func() error {
		if proof == nil {
			return nil
		}
		err := s.applyDestroyedComputer(ctx, jobID, run, *proof, group)
		proof, group = nil, nil
		return err
	}
	for {
		line, err := readLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return report, fmt.Errorf("tape sweep: read manifest: %w", err)
		}
		var kind struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(line, &kind); err != nil {
			return report, fmt.Errorf("tape sweep: manifest line invalid: %w", err)
		}
		switch kind.Kind {
		case "computer":
			if err := flushGroup(); err != nil {
				return report, err
			}
			var c tapeSweepManifestComputer
			if err := json.Unmarshal(line, &c); err != nil {
				return report, fmt.Errorf("tape sweep: manifest computer invalid: %w", err)
			}
			proof = &c.Proof
		case "item":
			var it tapeSweepItem
			if err := json.Unmarshal(line, &it); err != nil {
				return report, fmt.Errorf("tape sweep: manifest item invalid: %w", err)
			}
			if !validTapeSweepItem(it) {
				return report, fmt.Errorf("tape sweep: manifest item %q invalid", it.Path)
			}
			switch it.Sweep {
			case TapeSweepDestroyed:
				if proof == nil || proof.ComputerID != it.Computer {
					return report, fmt.Errorf("tape sweep: manifest item %s outside its computer group", it.Path)
				}
				group = append(group, it)
			case TapeSweepOrphans:
				if err := flushGroup(); err != nil {
					return report, err
				}
				if p.orphans {
					orphans = append(orphans, it)
				}
			}
		default:
			return report, fmt.Errorf("tape sweep: manifest line kind %q unknown", kind.Kind)
		}
	}
	if err := flushGroup(); err != nil {
		return report, err
	}
	if len(orphans) > 0 {
		if err := s.applyOrphans(ctx, jobID, run, orphans); err != nil {
			return report, err
		}
	}
	report.ReportPath = filepath.Join(s.tapeSweepLogDir(), "tape-sweep-"+p.runID+".apply-"+started.UTC().Format("20060102T150405Z")+".report.json")
	report.DurationMs = time.Since(started).Milliseconds()
	tlog.record(tapeSweepLogRecord{Op: "apply_finished", Count: counts.Deleted, Bytes: counts.BytesDeleted})
	_ = tlog.flush()
	if err := writeJSONFileSync(report.ReportPath, report); err != nil {
		return report, err
	}
	log.Printf("tape sweep: apply run=%s deleted=%d bytes=%d gone=%d kept_refreshed=%d kept_referenced=%d kept_not_destroyed=%d kept_unverified=%d deferred=%d errors=%d computers_deleted=%d computers_skipped=%d archives=%d",
		p.runID, counts.Deleted, counts.BytesDeleted, counts.Gone, counts.KeptRefreshed, counts.KeptReferenced, counts.KeptNotDestroyed, counts.KeptUnverified, counts.DeferredByCap, counts.Errors, counts.ComputersDeleted, counts.ComputersSkipped, len(counts.Archives))
	return report, nil
}

func validTapeSweepItem(it tapeSweepItem) bool {
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

func (s *Service) tapeSweepSkipComputer(run *tapeSweepApplyRun, proof TapeSweepComputerProof, items []tapeSweepItem, reason string) {
	run.counts.ComputersSkipped++
	run.counts.KeptNotDestroyed += len(items)
	run.log.record(tapeSweepLogRecord{Op: "kept_not_destroyed", Sweep: TapeSweepDestroyed, Computer: proof.ComputerID, Reason: reason, Count: len(items)})
	_ = run.log.flush()
	log.Printf("tape sweep: computer=%s skipped (%s) items=%d", proof.ComputerID, reason, len(items))
}

// reproveDestroyed re-checks the destroyed proof at delete time.
func (s *Service) reproveDestroyed(ctx context.Context, run *tapeSweepApplyRun, proof TapeSweepComputerProof) string {
	id := proof.ComputerID
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
	if has, err := s.tapeReader().HasSegments(id); err != nil {
		return "segments unreadable: " + err.Error()
	} else if has {
		return "computer has tape segments"
	}
	return ""
}

// applyDestroyedComputer re-proves, tombstones, archives and deletes one
// destroyed computer's tape.
func (s *Service) applyDestroyedComputer(ctx context.Context, jobID string, run *tapeSweepApplyRun, proof TapeSweepComputerProof, items []tapeSweepItem) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !run.p.destroyed {
		return nil
	}
	id := proof.ComputerID
	existing, hasTomb, err := s.store.TapeTombstone(ctx, id)
	if err != nil {
		return err
	}
	dropTomb := func() {
		if t, ok, _ := s.store.TapeTombstone(ctx, id); ok && t.FilesDeleted == 0 && t.Status != tapeTombstoneDeleted {
			if err := s.store.removeTapeTombstone(ctx, id); err != nil {
				log.Printf("tape sweep: computer=%s remove tombstone: %v", id, err)
			}
		}
	}
	if reason := s.reproveDestroyed(ctx, run, proof); reason != "" {
		dropTomb()
		s.tapeSweepSkipComputer(run, proof, items, reason)
		return nil
	}
	s.setTapeSweepPhase(jobID, "destroyed "+id)

	// Tombstone under the CAS gate, then re-read the head: no append can
	// land after this check.
	tomb := TapeTombstone{ComputerID: id, Status: tapeTombstoneArchiving, HeadSequence: proof.HeadSequence, CanonicalEventHead: proof.CanonicalEventHead, ManifestRun: run.p.runID, ManifestSHA256: run.manifestSH}
	if hasTomb {
		tomb = existing
		tomb.ManifestRun, tomb.ManifestSHA256 = run.p.runID, run.manifestSH
	}
	proofJSON, _ := json.Marshal(proof)
	tomb.ProofJSON = string(proofJSON)
	s.store.tombstones.gate.Lock()
	werr := s.store.writeTapeTombstone(ctx, tomb)
	head, herr := readComputerEventHead(ctx, s.store.db, id, false)
	s.store.tombstones.gate.Unlock()
	if werr != nil {
		run.counts.Errors++
		return werr
	}
	if herr != nil || head == nil || head.Sequence != proof.HeadSequence || head.CanonicalEventHead != proof.CanonicalEventHead {
		dropTomb()
		s.tapeSweepSkipComputer(run, proof, items, "head changed at tombstone")
		return nil
	}
	if !hasTomb {
		run.counts.ComputersTombstoned++
	}
	run.log.record(tapeSweepLogRecord{Op: "tombstone", Sweep: TapeSweepDestroyed, Computer: id, Reason: tomb.Status, Count: len(items)})

	// Archive before any unlink.
	archive, err := s.ensureTapeArchive(run, id, items)
	if err != nil {
		run.counts.Errors++
		dropTomb()
		run.log.record(tapeSweepLogRecord{Op: "error", Sweep: TapeSweepDestroyed, Computer: id, Error: "archive: " + err.Error()})
		_ = run.log.flush()
		log.Printf("tape sweep: computer=%s archive failed, nothing deleted: %v", id, err)
		return nil
	}
	run.counts.Archives = append(run.counts.Archives, archive.Archive)
	if tomb.Status == tapeTombstoneArchiving {
		tomb.Status = tapeTombstoneArchived
	}
	tomb.ArchivePath, tomb.ArchiveSHA256, tomb.ArchiveManifestSHA256 = archive.Archive, archive.ArchiveSHA256, archive.ManifestSHA256
	if err := s.store.putTapeTombstone(ctx, tomb); err != nil {
		return err
	}

	deleted, bytesDeleted, complete, err := s.deleteTapeItems(ctx, run, items)
	tomb.FilesDeleted += int64(deleted)
	tomb.BytesDeleted += bytesDeleted
	if complete {
		tomb.Status = tapeTombstoneDeleted
		run.counts.ComputersDeleted++
	}
	if perr := s.store.putTapeTombstone(context.WithoutCancel(ctx), tomb); perr != nil && err == nil {
		err = perr
	}
	log.Printf("tape sweep: computer=%s deleted=%d bytes=%d complete=%v archive=%s", id, deleted, bytesDeleted, complete, archive.Archive)
	return err
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
	archive, err := s.ensureTapeArchive(run, "orphans", keep)
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
			if run.delta[it.Digest] {
				run.counts.KeptReferenced++
				run.log.item("kept_referenced", it, it.Size, "")
				continue
			}
			if run.deletes+1 > run.p.maxDeletes || run.bytes+it.Size > run.p.maxBytes {
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
		for _, it := range batch {
			if hook := s.tapeSweep.beforeDelete; hook != nil {
				hook(it)
			}
			path := filepath.Join(s.artifactsRoot, filepath.FromSlash(it.Path))
			op, size, uerr := s.unlinkIfStillUnreachable(gcCandidate{ns: it.Namespace, rel: it.Path, path: path, size: it.Size}, run.cutoff)
			errText := ""
			switch op {
			case "deleted":
				deleted++
				bytesDeleted += size
				run.counts.Deleted++
				run.counts.BytesDeleted += size
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
func (s *Service) ensureTapeArchive(run *tapeSweepApplyRun, scope string, items []tapeSweepItem) (TapeArchiveSidecar, error) {
	if err := os.MkdirAll(run.archiveDir, 0o700); err != nil {
		return TapeArchiveSidecar{}, err
	}
	path := filepath.Join(run.archiveDir, scope+"-"+run.p.runID+".tar.zst")
	if raw, err := os.ReadFile(path + ".json"); err == nil {
		var sc TapeArchiveSidecar
		if json.Unmarshal(raw, &sc) == nil && sc.Archive == path {
			if err := verifyTapeArchive(path, sc); err == nil {
				return sc, nil
			}
		}
	}
	sc, err := s.writeTapeArchive(path, scope, run, items)
	if err != nil {
		_ = os.Remove(path + ".tmp")
		return TapeArchiveSidecar{}, err
	}
	if err := verifyTapeArchive(path, sc); err != nil {
		return TapeArchiveSidecar{}, err
	}
	if err := writeJSONFileSync(path+".json", sc); err != nil {
		return TapeArchiveSidecar{}, err
	}
	run.log.record(tapeSweepLogRecord{Op: "archived", Path: path, Count: sc.Entries, Bytes: sc.Bytes, Reason: scope, Digest: sc.ArchiveSHA256})
	return sc, run.log.flush()
}

func (s *Service) writeTapeArchive(path, scope string, run *tapeSweepApplyRun, items []tapeSweepItem) (TapeArchiveSidecar, error) {
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
func verifyTapeArchive(path string, sc TapeArchiveSidecar) error {
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
		if len(parts) != 3 || parts[0] != "sha256" {
			return fmt.Errorf("archive %s: unexpected entry %q", path, hdr.Name)
		}
		name, ok := tapeSweepFileName(parts[1], parts[2])
		if !ok || name != got {
			return fmt.Errorf("archive %s: entry %s does not hash to its name", path, hdr.Name)
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
