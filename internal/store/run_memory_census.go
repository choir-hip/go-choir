package store

// The census a run-memory retirement pass takes of every run group it
// considers (run_memory_retire.go). On the busiest computer the table holds
// 4.6M+ rows and a pass retired 4 runs; no counter said which exclusion held
// the rest. The census names, for each group, the one rule that kept it (or
// that retired it), with runs and rows. It reads nothing the selection does
// not: the group query's covering index, plus COUNT(*) from that same index.
// Body bytes are not counted: the body is in the row, not the index, and a
// large body sits on overflow pages, so summing it reads the table.

import (
	"log"
	"sort"
	"sync"
	"time"
)

// RunMemoryDisposition is one rule's runs and rows.
type RunMemoryDisposition struct {
	Disposition string `json:"disposition"`
	Runs        int64  `json:"runs"`
	// Entries is every row of those runs, cursors included.
	Entries int64 `json:"entries"`
	// RetirableEntries is the rows a retirement would remove (not cursors).
	RetirableEntries int64 `json:"retirable_entries"`
}

// RunMemoryTally is runs and rows.
type RunMemoryTally struct {
	Runs    int64 `json:"runs"`
	Entries int64 `json:"entries"`
}

func (t *RunMemoryTally) add(g runMemoryGroup) {
	t.Runs++
	t.Entries += g.entries
}

// RunMemoryNewestTwo counts the runs that are among the newest two of their
// document custodian agent whatever other rule also keeps them, by the age of their newest entry.
// The dispositions are exclusive and name the first rule in order, so a run in
// the window shows there; this block shows the overlap.
type RunMemoryNewestTwo struct {
	Within7d   RunMemoryTally `json:"newest_entry_within_7d"`
	From7To30d RunMemoryTally `json:"newest_entry_7_to_30d"`
	Over30d    RunMemoryTally `json:"newest_entry_over_30d"`
}

// RunMemoryWake is the retired runs of every pass of one wake (the first pass
// after boot, or a backlog's catch-up), so the claim a catch-up makes can be
// read after the last pass has replaced the census.
type RunMemoryWake struct {
	Passes  int64                  `json:"passes"`
	Retired []RunMemoryDisposition `json:"retired"`
}

// RunMemoryCensus is the last pass's census, as served on
// /api/runtime/observability.
type RunMemoryCensus struct {
	// Passes counts the passes since boot; zero before the first.
	Passes     int64      `json:"passes"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	// Groups is the runs holding entries, Entries all their rows.
	Groups  int64 `json:"groups"`
	Entries int64 `json:"entries"`
	// Agents is the distinct owner/agent pairs; AgentsWithOneOrTwoRuns those
	// with at most two runs (all agents; only a document custodian's are
	// protected by the newest-two rule).
	Agents                 int64 `json:"agents"`
	AgentsWithOneOrTwoRuns int64 `json:"agents_with_one_or_two_runs"`
	// Dispositions: every group in exactly one, in the order the rules apply.
	Dispositions []RunMemoryDisposition `json:"dispositions"`
	CarrySources RunMemoryNewestTwo     `json:"carry_sources_by_entry_age"`
	// FirstPass is the first pass of the wake, whole (the census a backlog's
	// catch-up begins with); Wake totals the retired runs of every pass of it.
	FirstPass *RunMemoryCensus `json:"first_pass_of_wake,omitempty"`
	Wake      *RunMemoryWake   `json:"wake,omitempty"`
	// BodyBytes says why no byte count is given.
	BodyBytes string `json:"body_bytes"`
}

// runMemoryCensusOrder is the fixed dispositions, listed even at zero.
var runMemoryCensusOrder = []string{
	"retired",
	"retired_no_run_record",
	"kept_only_cursor_entries",
	"kept_newest_entry_in_window",
	"kept_carry_source_7_30d",
	"kept_carry_source_over_30d",
	"kept_no_run_record_in_hold",
	"kept_run_record_unreadable",
	"kept_state_passivated",
	"kept_state_blocked",
	"kept_finished_in_window",
	"kept_unreadable_time",
	"not_reached_over_cap",
}

const runMemoryCensusBodyBytes = "not counted: the body is in the row, not the index; summing it reads the table and its overflow pages"

func newRunMemoryCensus() RunMemoryCensus {
	return RunMemoryCensus{Dispositions: []RunMemoryDisposition{}, BodyBytes: runMemoryCensusBodyBytes}
}

// add counts a group under a disposition.
func (c *RunMemoryCensus) add(name string, g runMemoryGroup) {
	for i := range c.Dispositions {
		if c.Dispositions[i].Disposition == name {
			d := &c.Dispositions[i]
			d.Runs++
			d.Entries += g.entries
			d.RetirableEntries += g.retirable
			return
		}
	}
	c.Dispositions = append(c.Dispositions, RunMemoryDisposition{Disposition: name, Runs: 1, Entries: g.entries, RetirableEntries: g.retirable})
}

// finish totals the groups and puts the dispositions in order; the fixed ones
// are always listed.
func (c *RunMemoryCensus) finish() {
	have := map[string]bool{}
	for _, d := range c.Dispositions {
		have[d.Disposition] = true
		c.Groups += d.Runs
		c.Entries += d.Entries
	}
	for _, n := range runMemoryCensusOrder {
		if !have[n] {
			c.Dispositions = append(c.Dispositions, RunMemoryDisposition{Disposition: n})
		}
	}
	rank := map[string]int{}
	for i, n := range runMemoryCensusOrder {
		rank[n] = i
	}
	sort.SliceStable(c.Dispositions, func(i, j int) bool {
		ri, iok := rank[c.Dispositions[i].Disposition]
		rj, jok := rank[c.Dispositions[j].Disposition]
		switch {
		case iok && jok:
			return ri < rj
		case iok != jok:
			return iok
		}
		return c.Dispositions[i].Disposition < c.Dispositions[j].Disposition
	})
}

// logLines writes one line per disposition.
func (c RunMemoryCensus) logLines() {
	log.Printf("store: run memory census: %d runs, %d entries, %d agents (%d with 1-2 runs)", c.Groups, c.Entries, c.Agents, c.AgentsWithOneOrTwoRuns)
	for _, d := range c.Dispositions {
		log.Printf("store: run memory census: %s: %d runs, %d entries (%d retirable)", d.Disposition, d.Runs, d.Entries, d.RetirableEntries)
	}
}

type runMemoryCensusHolder struct {
	mu   sync.Mutex
	last RunMemoryCensus
	set  bool
	// passes counts the passes since boot.
	passes int64
	// wakeOpen is set while a backlog wake runs; a pass outside one is a wake
	// of its own. first is the wake's first pass, whole; wake its retired totals.
	wakeOpen bool
	first    *RunMemoryCensus
	wake     RunMemoryWake
}

// beginWake starts a wake: the next pass is its first.
func (h *runMemoryCensusHolder) beginWake() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.wakeOpen, h.first, h.wake = true, nil, RunMemoryWake{}
}

func (h *runMemoryCensusHolder) endWake() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.wakeOpen = false
}

// retiredDispositions are the dispositions a wake totals: the others hold the
// same runs again each pass.
var retiredDispositions = []string{"retired", "retired_no_run_record"}

func copyCensus(c RunMemoryCensus) RunMemoryCensus {
	c.Dispositions = append([]RunMemoryDisposition(nil), c.Dispositions...)
	return c
}

// recordRunMemoryCensus keeps the pass's census as the last, with its times,
// the wake's first pass and the wake's totals.
func (s *Store) recordRunMemoryCensus(c RunMemoryCensus, started time.Time) {
	finished := time.Now().UTC()
	h := &s.runMemoryCensus
	h.mu.Lock()
	defer h.mu.Unlock()
	h.passes++
	c.Passes = h.passes
	c.StartedAt, c.FinishedAt = &started, &finished
	if !h.wakeOpen {
		h.first, h.wake = nil, RunMemoryWake{}
	}
	if h.first == nil {
		first := copyCensus(c)
		h.first = &first
	}
	h.wake.Passes++
	if len(h.wake.Retired) == 0 {
		for _, n := range retiredDispositions {
			h.wake.Retired = append(h.wake.Retired, RunMemoryDisposition{Disposition: n})
		}
	}
	for _, d := range c.Dispositions {
		for i := range h.wake.Retired {
			if h.wake.Retired[i].Disposition == d.Disposition {
				h.wake.Retired[i].Runs += d.Runs
				h.wake.Retired[i].Entries += d.Entries
				h.wake.Retired[i].RetirableEntries += d.RetirableEntries
			}
		}
	}
	first := copyCensus(*h.first)
	wake := h.wake
	wake.Retired = append([]RunMemoryDisposition(nil), wake.Retired...)
	c.FirstPass, c.Wake = &first, &wake
	h.last, h.set = c, true
}

// RunMemoryRetirementCensus is the last pass's census: empty (no passes, no
// times) before the first pass since boot.
func (s *Store) RunMemoryRetirementCensus() RunMemoryCensus {
	h := &s.runMemoryCensus
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.set {
		return newRunMemoryCensus()
	}
	return copyCensus(h.last)
}
