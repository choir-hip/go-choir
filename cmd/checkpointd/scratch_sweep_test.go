package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// Failure modes, written before the code
// (docs/problems/platform-artifact-retention-2026-10-22.md section 8, F1):
// a directory a queued, running or retrying job holds is removed; a
// genesis-repair scratch is removed; a name not shaped <computer>-<int> or a
// plain file is removed; a crash leaves a half-deleted tree under a live
// name; an orphan is never removed; a failed job read removes anything; a
// fresh orphan is removed inside the age gate.

type sweepFixture struct {
	t       *testing.T
	root    string
	logDir  string
	jobs    map[string]scratchJobState
	jobsErr error
	now     time.Time
}

func newSweepFixture(t *testing.T) *sweepFixture {
	t.Helper()
	base := t.TempDir()
	return &sweepFixture{
		t:      t,
		root:   filepath.Join(base, "checkpoint-scratch"),
		logDir: filepath.Join(base, "artifact-gc-log"),
		jobs:   map[string]scratchJobState{},
		now:    time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC),
	}
}

// dir makes a scratch directory holding size bytes with an mtime age ago.
func (f *sweepFixture) dir(name string, size int, age time.Duration) string {
	f.t.Helper()
	p := filepath.Join(f.root, name)
	if err := os.MkdirAll(filepath.Join(p, "sub"), 0o700); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "sub", "runtime.db"), []byte(strings.Repeat("x", size)), 0o600); err != nil {
		f.t.Fatal(err)
	}
	when := f.now.Add(-age)
	if err := os.Chtimes(p, when, when); err != nil {
		f.t.Fatal(err)
	}
	return p
}

// marker writes the worker's genesis-repair marker beside a scratch directory.
func (f *sweepFixture) marker(name string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.root, name+genesisRepairSuffix), nil, 0o600); err != nil {
		f.t.Fatal(err)
	}
}

func (f *sweepFixture) cfg() scratchSweepConfig {
	return scratchSweepConfig{
		Root:   f.root,
		LogDir: f.logDir,
		MinAge: 48 * time.Hour,
		Now:    f.now,
		Jobs: func() (map[string]scratchJobState, error) {
			if f.jobsErr != nil {
				return nil, f.jobsErr
			}
			return f.jobs, nil
		},
	}
}

func (f *sweepFixture) exists(name string) bool {
	_, err := os.Stat(filepath.Join(f.root, name))
	return err == nil
}

func (f *sweepFixture) logRecords() []map[string]any {
	f.t.Helper()
	matches, _ := filepath.Glob(filepath.Join(f.logDir, "checkpoint-scratch-*.jsonl"))
	sort.Strings(matches)
	var out []map[string]any
	for _, m := range matches {
		file, err := os.Open(m)
		if err != nil {
			f.t.Fatal(err)
		}
		sc := bufio.NewScanner(file)
		for sc.Scan() {
			var rec map[string]any
			if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
				f.t.Fatalf("log line %q: %v", sc.Text(), err)
			}
			out = append(out, rec)
		}
		file.Close()
	}
	return out
}

const oldAge = 5 * 24 * time.Hour

func TestSweepKeepsDirectoriesALiveJobHolds(t *testing.T) {
	f := newSweepFixture(t)
	for _, status := range []string{"queued", "running", "retry"} {
		c := "computer-" + status
		f.dir(c+"-7", 100, oldAge)
		f.jobs[c] = scratchJobState{Generation: 7, Status: status}
	}
	// A generation newer than the row's is out of order: held, fail closed.
	f.dir("computer-ahead-9", 100, oldAge)
	f.jobs["computer-ahead"] = scratchJobState{Generation: 8, Status: "succeeded"}

	res, err := sweepOrphanScratch(f.cfg())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Removed) != 0 {
		t.Fatalf("removed held directories: %+v", res.Removed)
	}
	for _, name := range []string{"computer-queued-7", "computer-running-7", "computer-retry-7", "computer-ahead-9"} {
		if !f.exists(name) {
			t.Fatalf("held directory %s was removed", name)
		}
	}
}

func TestSweepRemovesOrphansAndLogsBytes(t *testing.T) {
	f := newSweepFixture(t)
	f.dir("computer-failed-4", 1000, oldAge) // terminal job, same generation
	f.dir("computer-blocked-2", 500, oldAge) // blocked, not a genesis repair
	f.dir("computer-gone-3", 250, oldAge)    // no job row at all
	f.dir("computer-moved-3", 125, oldAge)   // superseded generation
	f.dir("computer-moved-4", 60, oldAge)    // the live generation of the same computer
	f.jobs["computer-failed"] = scratchJobState{Generation: 4, Status: "failed"}
	f.jobs["computer-blocked"] = scratchJobState{Generation: 2, Status: "blocked"}
	f.jobs["computer-moved"] = scratchJobState{Generation: 4, Status: "running"}

	res, err := sweepOrphanScratch(f.cfg())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"computer-failed-4", "computer-blocked-2", "computer-gone-3", "computer-moved-3"} {
		if f.exists(name) {
			t.Fatalf("orphan %s was kept", name)
		}
	}
	if !f.exists("computer-moved-4") {
		t.Fatal("the live generation was removed")
	}
	if len(res.Removed) != 4 || res.BytesRemoved != 1000+500+250+125 {
		t.Fatalf("result = %+v", res)
	}
	removed := map[string]float64{}
	for _, rec := range f.logRecords() {
		if rec["op"] == "scratch_removed" {
			removed[rec["path"].(string)] = rec["bytes"].(float64)
			if rec["ns"] != "checkpoint-scratch" {
				t.Fatalf("namespace = %v", rec["ns"])
			}
		}
	}
	if len(removed) != 4 || removed["checkpoint-scratch/computer-failed-4"] != 1000 || removed["checkpoint-scratch/computer-gone-3"] != 250 {
		t.Fatalf("log removals = %v", removed)
	}
}

func TestSweepKeepsGenesisRepairScratchAndLogsItKept(t *testing.T) {
	f := newSweepFixture(t)
	// Row says genesis repair at this generation, job blocked.
	f.dir("computer-gen-5", 900, oldAge)
	f.jobs["computer-gen"] = scratchJobState{Generation: 5, Status: "blocked", GenesisRepair: true}
	// The row moved on to a later non-genesis generation, but the worker left
	// its marker beside the directory.
	f.dir("computer-marked-3", 800, oldAge)
	f.marker("computer-marked-3")
	f.jobs["computer-marked"] = scratchJobState{Generation: 4, Status: "queued"}
	// A marker with no job row at all is still genesis scratch.
	f.dir("computer-nojob-1", 700, oldAge)
	f.marker("computer-nojob-1")

	res, err := sweepOrphanScratch(f.cfg())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"computer-gen-5", "computer-marked-3", "computer-nojob-1"} {
		if !f.exists(name) {
			t.Fatalf("genesis-repair scratch %s was removed", name)
		}
	}
	if len(res.Removed) != 0 || len(res.Kept) != 3 {
		t.Fatalf("result = %+v", res)
	}
	kept := 0
	for _, rec := range f.logRecords() {
		if rec["op"] == "scratch_kept" {
			kept++
			if rec["reason"] == "" || rec["reason"] == nil {
				t.Fatalf("kept record without a reason: %v", rec)
			}
		}
	}
	if kept != 3 {
		t.Fatalf("logged %d kept records, want 3", kept)
	}
}

func TestSweepIgnoresUnshapedNamesFilesAndFreshOrphans(t *testing.T) {
	f := newSweepFixture(t)
	for _, name := range []string{"computer-aa-notes", "computer-aa-", "computer-aa-4x", "stray", "-5"} {
		f.dir(name, 10, oldAge)
	}
	if err := os.WriteFile(filepath.Join(f.root, "computer-aa-1"), []byte("a file"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.dir("computer-fresh-2", 10, time.Hour) // an orphan inside the age gate
	res, err := sweepOrphanScratch(f.cfg())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Removed) != 0 {
		t.Fatalf("removed %+v", res.Removed)
	}
	for _, name := range []string{"computer-aa-notes", "computer-aa-", "computer-aa-4x", "stray", "-5", "computer-aa-1", "computer-fresh-2"} {
		if !f.exists(name) {
			t.Fatalf("%s was removed", name)
		}
	}
}

func TestSweepRemovesNothingWhenJobsCannotBeRead(t *testing.T) {
	f := newSweepFixture(t)
	f.dir("computer-x-1", 10, oldAge)
	f.jobsErr = errors.New("store down")
	if _, err := sweepOrphanScratch(f.cfg()); err == nil {
		t.Fatal("expected the job read error")
	}
	if !f.exists("computer-x-1") {
		t.Fatal("a directory was removed without a job snapshot")
	}
}

// A crash between the atomic rename and the removal leaves no half-deleted
// tree under a live name, and the next sweep finishes the leftover.
func TestSweepCrashMidRemovalIsSafe(t *testing.T) {
	f := newSweepFixture(t)
	f.dir("computer-orph-3", 400, oldAge)
	f.dir("computer-held-5", 400, oldAge)
	f.jobs["computer-held"] = scratchJobState{Generation: 5, Status: "running"}

	cfg := f.cfg()
	cfg.afterRename = func(name string) { panic("simulated crash after rename of " + name) }
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected the simulated crash")
			}
		}()
		_, _ = sweepOrphanScratch(cfg)
	}()
	if f.exists("computer-orph-3") {
		t.Fatal("the orphan's live name survived the rename")
	}
	if !f.exists("computer-held-5") {
		t.Fatal("the held directory was touched")
	}
	leftovers, _ := filepath.Glob(filepath.Join(f.root, sweepingPrefix+"*"))
	if len(leftovers) != 1 {
		t.Fatalf("leftovers = %v", leftovers)
	}
	// A partial removal inside a leftover (some files already gone).
	_ = os.RemoveAll(filepath.Join(leftovers[0], "sub"))

	res, err := sweepOrphanScratch(f.cfg())
	if err != nil {
		t.Fatal(err)
	}
	if left, _ := filepath.Glob(filepath.Join(f.root, sweepingPrefix+"*")); len(left) != 0 {
		t.Fatalf("leftover not finished: %v", left)
	}
	if !f.exists("computer-held-5") {
		t.Fatal("the held directory was removed by the second sweep")
	}
	_ = res
}

// The marker is a sibling file, never inside the scratch directory: the
// directory is packed into the published base blob.
func TestPrepareScratchDirMarksGenesisRepairOnly(t *testing.T) {
	root := t.TempDir()
	plain := filepath.Join(root, "computer-a-1")
	if err := prepareScratchDir(plain, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(plain + genesisRepairSuffix); !os.IsNotExist(err) {
		t.Fatalf("a normal job's scratch was marked: %v", err)
	}
	genesis := filepath.Join(root, "computer-b-1")
	if err := prepareScratchDir(genesis, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(genesis + genesisRepairSuffix); err != nil {
		t.Fatalf("a genesis repair's scratch has no marker: %v", err)
	}
	if entries, _ := os.ReadDir(genesis); len(entries) != 0 {
		t.Fatalf("the marker went inside the scratch directory: %v", entries)
	}
}

func TestSweepRemovesOnlyDanglingMarkers(t *testing.T) {
	f := newSweepFixture(t)
	f.dir("computer-live-2", 10, oldAge)
	f.marker("computer-live-2")
	f.marker("computer-gone-9") // its directory was removed by the worker
	if _, err := sweepOrphanScratch(f.cfg()); err != nil {
		t.Fatal(err)
	}
	if !f.exists("computer-live-2" + genesisRepairSuffix) {
		t.Fatal("the marker of an existing genesis-repair directory was removed")
	}
	if f.exists("computer-gone-9" + genesisRepairSuffix) {
		t.Fatal("a dangling marker was kept")
	}
}
