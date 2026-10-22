package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Orphan sweep for checkpoint scratch
// (docs/problems/platform-artifact-retention-2026-10-22.md section 8, F1).
//
// pruneSupersededScratch removes older generations of the computer a job is
// claimed for. Nothing removed the scratch of a computer that is never
// claimed again (a failed or blocked last generation, a crash, a destroyed
// computer): 6.4 GB on Node B on 2026-10-22. This sweep removes every
// scratch directory that no queued, running or retrying job holds.

const (
	// genesisRepairSuffix names the marker file the worker writes beside the
	// scratch directory of a genesis repair job (<dir>.genesis-repair). It is
	// a sibling, never inside the directory, which is packed into the
	// published base blob. The sweep never removes a directory that has one.
	genesisRepairSuffix = ".genesis-repair"
	// sweepingPrefix names a directory the sweep has renamed out of the live
	// namespace and is removing. A crash leaves it for the next sweep.
	sweepingPrefix = ".sweeping-"
	// scratchSweepMinAge is the record's gate (c): a directory modified in
	// the last 48 hours is never swept.
	scratchSweepMinAge = 48 * time.Hour
	// scratchSweepInterval is how often a long-running worker re-sweeps.
	scratchSweepInterval = 15 * time.Minute

	scratchLogNamespace = "checkpoint-scratch"
)

var scratchDirNameRE = regexp.MustCompile(`^(.+)-([0-9]+)$`)

// scratchJobState is the part of a computer's projection job row the sweep
// needs. There is one row per computer, so the row names the only generation
// whose scratch a job can still hold.
type scratchJobState struct {
	Generation    uint64
	Status        string
	GenesisRepair bool
}

type scratchSweepConfig struct {
	Root   string
	LogDir string
	MinAge time.Duration
	Now    time.Time
	// Jobs reads the job rows after the directories are listed, so a job that
	// exists when the rows are read is held.
	Jobs func() (map[string]scratchJobState, error)
	// afterRename is a test hook, called between the atomic rename and the
	// removal.
	afterRename func(name string)
}

type scratchSweepEntry struct {
	Name   string
	Bytes  int64
	Reason string
}

type scratchSweepResult struct {
	Removed      []scratchSweepEntry
	Kept         []scratchSweepEntry
	BytesRemoved int64
}

type scratchLogRecord struct {
	At        string `json:"at"`
	Op        string `json:"op"` // scratch_removed | scratch_kept
	Namespace string `json:"ns"`
	Path      string `json:"path"`
	Bytes     int64  `json:"bytes,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// prepareScratchDir creates a job's scratch directory. A genesis repair gets a
// sibling marker so the sweep keeps it whatever its job row later says.
func prepareScratchDir(scratch string, genesisRepair bool) error {
	if err := os.MkdirAll(scratch, 0o700); err != nil {
		return err
	}
	if !genesisRepair {
		return nil
	}
	marker := scratch + genesisRepairSuffix
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	return os.WriteFile(marker, nil, 0o600)
}

// removeScratchDir removes a finished job's scratch directory and its marker.
func removeScratchDir(scratch string) error {
	err := os.RemoveAll(scratch)
	if err == nil {
		err = os.RemoveAll(scratch + genesisRepairSuffix)
	}
	return err
}

// sweepOrphanScratch removes scratch directories no live job holds. It never
// removes: a directory a queued, running or retrying job holds; a directory
// of a newer generation than the computer's row; a genesis-repair scratch
// (marker, or the row is a genesis repair of that generation); a name not
// shaped <computer>-<generation>; anything modified within MinAge. If the job
// rows cannot be read, it removes nothing.
func sweepOrphanScratch(cfg scratchSweepConfig) (scratchSweepResult, error) {
	var res scratchSweepResult
	if cfg.Now.IsZero() {
		cfg.Now = time.Now()
	}
	entries, err := os.ReadDir(cfg.Root)
	if err != nil {
		if os.IsNotExist(err) {
			return res, nil
		}
		return res, err
	}
	type candidate struct {
		name     string
		computer string
		gen      uint64
	}
	var cands []candidate
	var leftovers []string
	dirs := map[string]bool{}
	var markers []string
	for _, e := range entries {
		if !e.IsDir() {
			if e.Type().IsRegular() && strings.HasSuffix(e.Name(), genesisRepairSuffix) {
				markers = append(markers, e.Name())
			}
			continue
		}
		name := e.Name()
		dirs[name] = true
		if strings.HasPrefix(name, sweepingPrefix) {
			leftovers = append(leftovers, name)
			continue
		}
		m := scratchDirNameRE.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		gen, err := strconv.ParseUint(m[2], 10, 64)
		if err != nil {
			continue
		}
		cands = append(cands, candidate{name: name, computer: m[1], gen: gen})
	}
	// A marker whose directory is gone (published, or pruned as superseded)
	// guards nothing.
	for _, m := range markers {
		if !dirs[strings.TrimSuffix(m, genesisRepairSuffix)] {
			_ = os.Remove(filepath.Join(cfg.Root, m))
		}
	}
	if len(cands) == 0 && len(leftovers) == 0 {
		return res, nil
	}
	var jobs map[string]scratchJobState
	if len(cands) > 0 {
		if jobs, err = cfg.Jobs(); err != nil {
			return res, fmt.Errorf("checkpoint scratch sweep: reading jobs: %w", err)
		}
	}

	logger := newScratchLog(cfg.LogDir, cfg.Now)
	defer logger.close()

	remove := func(name, reason string) error {
		// The removal log must be writable before anything is removed.
		if !logger.open() {
			return fmt.Errorf("deletion log unavailable, nothing removed: %v", logger.failed)
		}
		src := filepath.Join(cfg.Root, name)
		size := dirBytes(src)
		dst := src
		if !strings.HasPrefix(name, sweepingPrefix) {
			dst = filepath.Join(cfg.Root, sweepingPrefix+name)
			// Rename is atomic: the live name never names a half-deleted tree.
			if err := os.Rename(src, dst); err != nil {
				return err
			}
			if cfg.afterRename != nil {
				cfg.afterRename(name)
			}
		}
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
		res.Removed = append(res.Removed, scratchSweepEntry{Name: name, Bytes: size, Reason: reason})
		res.BytesRemoved += size
		return logger.write(scratchLogRecord{Op: "scratch_removed", Namespace: scratchLogNamespace, Path: scratchLogNamespace + "/" + strings.TrimPrefix(name, sweepingPrefix), Bytes: size, Reason: reason})
	}

	for _, name := range leftovers {
		if err := remove(name, "finished a removal interrupted by a crash"); err != nil {
			log.Printf("checkpoint scratch sweep: leftover %s: %v", name, err)
		}
	}
	for _, c := range cands {
		job, hasJob := jobs[c.computer]
		if hasJob && c.gen > job.Generation {
			continue // out of order: fail closed
		}
		if hasJob && c.gen == job.Generation {
			switch job.Status {
			case "queued", "running", "retry":
				continue // held
			}
		}
		dir := filepath.Join(cfg.Root, c.name)
		var keepReason string
		if _, err := os.Stat(dir + genesisRepairSuffix); err == nil {
			keepReason = "genesis-repair marker: removal needs the owner's yes"
		} else if hasJob && c.gen == job.Generation && job.GenesisRepair {
			keepReason = "genesis-repair job (" + job.Status + "): removal needs the owner's yes"
		}
		if keepReason != "" {
			res.Kept = append(res.Kept, scratchSweepEntry{Name: c.name, Reason: keepReason})
			logger.keep(scratchLogRecord{Op: "scratch_kept", Namespace: scratchLogNamespace, Path: scratchLogNamespace + "/" + c.name, Reason: keepReason})
			continue
		}
		info, err := os.Lstat(dir)
		if err != nil || cfg.Now.Sub(info.ModTime()) < cfg.MinAge {
			continue
		}
		reason := "no live job holds it"
		if err := remove(c.name, reason); err != nil {
			log.Printf("checkpoint scratch sweep: %s: %v", c.name, err)
		}
	}
	if err := logger.err(); err != nil {
		return res, err
	}
	return res, nil
}

// dirBytes sums regular file sizes under dir; unreadable entries count zero.
func dirBytes(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// scratchLog appends the sweep's records to
// artifact-gc-log/checkpoint-scratch-<UTC day>.jsonl. It is a file of its own
// because corpusd's GC writer buffers and may split a line across writes. The
// file is opened lazily; a kept directory is logged once per day.
type scratchLog struct {
	dir    string
	day    string
	file   *os.File
	w      *bufio.Writer
	seen   map[string]bool
	failed error
	now    time.Time
}

func newScratchLog(dir string, now time.Time) *scratchLog {
	return &scratchLog{dir: dir, day: now.UTC().Format("2006-01-02"), now: now}
}

func (l *scratchLog) open() bool {
	if l.file != nil {
		return true
	}
	if l.failed != nil || l.dir == "" {
		return false
	}
	if err := os.MkdirAll(l.dir, 0o700); err != nil {
		l.failed = err
		return false
	}
	path := filepath.Join(l.dir, "checkpoint-scratch-"+l.day+".jsonl")
	l.seen = map[string]bool{}
	if f, err := os.Open(path); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var rec scratchLogRecord
			if json.Unmarshal(sc.Bytes(), &rec) == nil && rec.Op == "scratch_kept" {
				l.seen[rec.Path] = true
			}
		}
		f.Close()
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		l.failed = err
		return false
	}
	l.file, l.w = f, bufio.NewWriter(f)
	return true
}

func (l *scratchLog) write(rec scratchLogRecord) error {
	if !l.open() {
		return l.failed
	}
	rec.At = time.Now().UTC().Format(time.RFC3339Nano)
	line, _ := json.Marshal(rec)
	if _, err := l.w.Write(append(line, '\n')); err != nil {
		l.failed = err
		return err
	}
	// A removal is durable in the log as soon as it happens.
	if err := l.w.Flush(); err != nil {
		l.failed = err
		return err
	}
	return nil
}

func (l *scratchLog) keep(rec scratchLogRecord) {
	if !l.open() || l.seen[rec.Path] {
		return
	}
	l.seen[rec.Path] = true
	_ = l.write(rec)
}

func (l *scratchLog) err() error { return l.failed }

func (l *scratchLog) close() {
	if l.file != nil {
		_ = l.w.Flush()
		_ = l.file.Sync()
		_ = l.file.Close()
	}
}
