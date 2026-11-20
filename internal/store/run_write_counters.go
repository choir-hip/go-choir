package store

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Run write counters (T2 of the store-once goal; the design and the reason
// are section 9 of docs/design/run-record-written-less-2026-11-15.md). The
// per-kind rewrite counter says choir.run is written 4.7 times a run at about
// 18.7 KB each; it cannot say which writer wrote, what it wrote, which fields
// differ from the write before it, or which field carries the bytes. This
// counter can, one choir.run write at a time:
//
//   - site: the writer, from the closed list runWriteSites.
//   - state: the run's new state, from the closed list runWriteStates.
//   - change: which top-level body fields differ from the previous write
//     (state, prompt, result, error, finished_at, labels, metadata, other), or
//     updated_at_only, unchanged, new (no previous write) or no_prev (the site
//     has no previous body at hand).
//   - body bytes by field, as stored in the body: prompt, result, error,
//     labels, metadata, other (the rest, keys and scalars included).
//   - the single largest metadata key, by name when the name is in
//     runWriteMetadataKeys and as "other" otherwise, with its bytes.
//
// It is read-only and always on. It never gates, delays or changes a write:
// each writer calls it after its conditional put returned without error, with
// the bytes it already built. The work is one decode of the new body (and of
// the previous one where the site has it) outside the lock; the lock covers
// map adds only, and it is telemetryCounters.mu, the one the other tape
// counters use. The count is since boot: a restart starts it at zero, and the
// served block says so. Memory bound: runWriteMaxLines lines, plus one entry
// per listed metadata key and per site; no run id, state text or free key name
// is ever part of a key.
//
// Where it does not look: a body the site could not hand over as bytes counts
// under undecodable; a site that has no previous body (the lifecycle batch
// commits, which build several objects and hold none of the old bytes) says
// no_prev rather than guess.

// The writers of a choir.run object. Fixed: a site is a constant at the call.
const (
	runSiteCreateOG       = "create_og"                   // CreateRunOG (graph_store.go)
	runSiteUpdateOG       = "update_og"                   // UpdateRunOG (graph_store.go)
	runSiteActivation     = "activation"                  // projectLifecycleRun (lifecycle.go): create, running, completed, passivated
	runSiteLabels         = "labels"                      // UnionRunLabels (store.go)
	runSiteTerminalizeRun = "terminalize_run"             // commitLifecycleTransition, receipt kind terminalize_run
	runSiteReactivateRun  = "reactivate_run"              // commitLifecycleTransition, receipt kind reactivate_run
	runSiteControlBind    = "bind_control_delivery"       // commitLifecycleTransition, receipt kind bind_control_delivery
	runSiteEngineerBind   = "bind_engineering_assignment" // commitLifecycleTransition, receipt kind bind_co_super_assignment
	runSiteLifecycleOther = "lifecycle_other"             // any other lifecycle command that writes a run (engineering terminal projection)
)

var runWriteSites = map[string]bool{
	runSiteCreateOG: true, runSiteUpdateOG: true, runSiteActivation: true, runSiteLabels: true,
	runSiteTerminalizeRun: true, runSiteReactivateRun: true, runSiteControlBind: true,
	runSiteEngineerBind: true, runSiteLifecycleOther: true, "other": true,
}

const runStateOther = "other"

var runWriteStates = map[string]bool{
	"pending": true, "running": true, "completed": true, "failed": true,
	"cancelled": true, "blocked": true, "passivated": true, runStateOther: true,
}

const (
	runChangeUnchanged     = "unchanged"
	runChangeUpdatedAtOnly = "updated_at_only"
	runChangeNew           = "new"
	runChangeNoPrev        = "no_prev"
	runWriteOtherKey       = "other"
	// runWriteMaxLines bounds the site/state/change lines; the count past it is
	// "other/other/other". The real key space is a few dozen.
	runWriteMaxLines = 128
)

// runWriteNoPrev is what a site passes as the previous body when it has none
// at hand (as opposed to nil: there was no previous write).
var runWriteNoPrev = []byte{}

// runWriteMetadataKeys is the metadata keys the counter may name. A metadata
// key is chosen by whoever writes the run; only these, the ones our own code
// writes, are reported by name, and any other key is "other".
var runWriteMetadataKeys = map[string]bool{
	"texture_available_source_entities": true,
	"lifecycle_control_bindings":        true,
	"lifecycle_activation_versions":     true,
	"lifecycle_logical_activation_key":  true,
	"lifecycle_failed_attempt_key":      true,
	"lifecycle_activation_build_commit": true,
	"lifecycle_work_item_id":            true,
	"work_item_ids":                     true,
	"worker_update_ids":                 true,
	"assignment_trajectory_id":          true,
	"request_source":                    true,
	"llm_fallbacks":                     true,
	"llm_policy_source":                 true,
	"llm_thread":                        true,
	"texture_root_documents":            true,
	"texture_root_excerpt_bytes":        true,
	"texture_root_digested_through":     true,
	"texture_root_digested_docs":        true,
	"texture_root_started_at":           true,
	"input_tokens":                      true,
	"output_tokens":                     true,
	"cached_input_tokens":               true,
	"passivated_reason":                 true,
	"provider_admission_refusal":        true,
	"fate":                              true,
	"actor_reactivate_existing_memory":  true,
	"actor_reactivated_from_passivated": true,
}

// runWriteFieldNames are the top-level body fields a change is reported by.
// updated_at is not one: it differs on nearly every write and is reported only
// when it is the only difference.
var runWriteFieldNames = map[string]string{
	"state": "state", "prompt": "prompt", "result": "result", "error": "error",
	"finished_at": "finished_at", "labels": "labels", "metadata": "metadata",
}

// runWriteBytes is one write's body bytes by field.
type runWriteBytes struct {
	body, prompt, result, err, labels, metadata, other int64
}

// runWriteTally is what one write adds to the counters, worked out before any
// lock is taken.
type runWriteTally struct {
	site, state, change string
	undecodable         bool
	bytes               runWriteBytes
	topKey              string // "" when the metadata is empty or not an object
	topKeyBytes         int64
	metadataChanged     []string // listed names, then "other", sorted
}

// tallyRunWrite works out one write from the body a writer stored and, when it
// has it, the body before. prev nil means there was no previous write; any
// other prev that does not decode counts as no_prev.
func tallyRunWrite(site string, prev, next []byte) runWriteTally {
	if !runWriteSites[site] {
		site = "other"
	}
	t := runWriteTally{site: site, state: runStateOther}
	var nf map[string]json.RawMessage
	if err := json.Unmarshal(next, &nf); err != nil || nf == nil {
		t.undecodable = true
		return t
	}
	var state string
	if raw, ok := nf["state"]; ok && json.Unmarshal(raw, &state) == nil && runWriteStates[state] {
		t.state = state
	}
	b := &t.bytes
	b.body = int64(len(next))
	b.prompt, b.result = int64(len(nf["prompt"])), int64(len(nf["result"]))
	b.err, b.labels, b.metadata = int64(len(nf["error"])), int64(len(nf["labels"])), int64(len(nf["metadata"]))
	b.other = b.body - b.prompt - b.result - b.err - b.labels - b.metadata

	var nm map[string]json.RawMessage
	nmBad := false
	if raw := nf["metadata"]; len(raw) > 0 && json.Unmarshal(raw, &nm) != nil {
		nm, nmBad = nil, true
	}
	for key, raw := range nm {
		if n := int64(len(raw)); n > t.topKeyBytes || (n == t.topKeyBytes && key < t.topKey) {
			t.topKey, t.topKeyBytes = key, n
		}
	}
	if t.topKey != "" && !runWriteMetadataKeys[t.topKey] {
		t.topKey = runWriteOtherKey
	}

	switch {
	case prev == nil:
		t.change = runChangeNew
	default:
		var pf map[string]json.RawMessage
		if err := json.Unmarshal(prev, &pf); err != nil || pf == nil {
			t.change = runChangeNoPrev
			break
		}
		t.change = classifyRunChange(pf, nf)
		if !bytes.Equal(pf["metadata"], nf["metadata"]) {
			var pm map[string]json.RawMessage
			pmBad := false
			if raw := pf["metadata"]; len(raw) > 0 && json.Unmarshal(raw, &pm) != nil {
				pmBad = true
			}
			if !pmBad && !nmBad {
				t.metadataChanged = changedMetadataKeys(pm, nm)
			}
		}
	}
	return t
}

// classifyRunChange names the top-level fields that differ between two bodies.
func classifyRunChange(prev, next map[string]json.RawMessage) string {
	names := map[string]bool{}
	updatedAtDiffers := false
	seen := map[string]bool{}
	for key, raw := range next {
		seen[key] = true
		if old, ok := prev[key]; ok && bytes.Equal(old, raw) {
			continue
		}
		if key == "updated_at" {
			updatedAtDiffers = true
		} else if name, ok := runWriteFieldNames[key]; ok {
			names[name] = true
		} else {
			names["other"] = true
		}
	}
	for key := range prev {
		if seen[key] {
			continue
		}
		if key == "updated_at" {
			updatedAtDiffers = true
		} else if name, ok := runWriteFieldNames[key]; ok {
			names[name] = true
		} else {
			names["other"] = true
		}
	}
	if len(names) == 0 {
		if updatedAtDiffers {
			return runChangeUpdatedAtOnly
		}
		return runChangeUnchanged
	}
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return strings.Join(out, "+")
}

// changedMetadataKeys is the listed metadata keys whose value differs, sorted,
// plus "other" once when an unlisted key differs.
func changedMetadataKeys(prev, next map[string]json.RawMessage) []string {
	listed := map[string]bool{}
	other := false
	note := func(key string) {
		if runWriteMetadataKeys[key] {
			listed[key] = true
		} else {
			other = true
		}
	}
	for key, raw := range next {
		if old, ok := prev[key]; !ok || !bytes.Equal(old, raw) {
			note(key)
		}
	}
	for key := range prev {
		if _, ok := next[key]; !ok {
			note(key)
		}
	}
	out := make([]string, 0, len(listed)+1)
	for key := range listed {
		out = append(out, key)
	}
	sort.Strings(out)
	if other {
		out = append(out, runWriteOtherKey)
	}
	return out
}

// RunWriteLine is one site/state/change line on the wire. Bytes are the sums of
// the field's stored bytes over the line's writes (the tape carries a body at
// four thirds of these: base64).
type RunWriteLine struct {
	Ops           int64 `json:"ops"`
	BodyBytes     int64 `json:"body_bytes"`
	PromptBytes   int64 `json:"prompt_bytes"`
	ResultBytes   int64 `json:"result_bytes"`
	ErrorBytes    int64 `json:"error_bytes"`
	LabelsBytes   int64 `json:"labels_bytes"`
	MetadataBytes int64 `json:"metadata_bytes"`
	OtherBytes    int64 `json:"other_bytes"`
}

// RunWriteTopKey is how often a metadata key was the largest in a write, and
// its bytes summed over those writes.
type RunWriteTopKey struct {
	Writes int64 `json:"writes"`
	Bytes  int64 `json:"bytes"`
}

// RunWriteStats is the served block (telemetry_batching.run_writes).
type RunWriteStats struct {
	// Scope says what the numbers are: a count since boot.
	Scope string `json:"scope"`
	// Ops is every choir.run write counted: the sum of the lines.
	Ops int64 `json:"ops"`
	// Lines is keyed "site/state/change" from the fixed lists.
	Lines map[string]RunWriteLine `json:"lines"`
	// TopMetadataKey is the largest metadata key of each write, by listed name
	// or "other".
	TopMetadataKey map[string]RunWriteTopKey `json:"top_metadata_key"`
	// MetadataKeyChanges counts, per listed key or "other", the writes whose
	// value of that key differs from the write before it (sites that have it).
	MetadataKeyChanges map[string]int64 `json:"metadata_key_changes"`
	// Undecodable counts, per site, writes whose body would not decode; they
	// are in no line.
	Undecodable map[string]int64 `json:"undecodable"`
}

const runWriteScope = "since boot: a restart starts every count at zero; one choir.run write per op, counted after its put committed; bytes are body bytes by field, not tape bytes (the tape carries the body in base64, four thirds)"

// runWriteMaps is the counters. It is guarded by telemetryCounters.mu.
type runWriteMaps struct {
	ops         int64
	lines       map[string]*RunWriteLine
	topKey      map[string]*RunWriteTopKey
	keyChanges  map[string]int64
	undecodable map[string]int64
}

// add counts one tallied write. The caller holds telemetryCounters.mu.
func (m *runWriteMaps) add(t runWriteTally) {
	if t.undecodable {
		if m.undecodable == nil {
			m.undecodable = map[string]int64{}
		}
		m.undecodable[t.site]++
		return
	}
	m.ops++
	key := t.site + "/" + t.state + "/" + t.change
	if m.lines == nil {
		m.lines = map[string]*RunWriteLine{}
	}
	line, ok := m.lines[key]
	if !ok {
		if len(m.lines) >= runWriteMaxLines {
			key = "other/other/other"
			line = m.lines[key]
		}
		if line == nil {
			line = &RunWriteLine{}
			m.lines[key] = line
		}
	}
	line.Ops++
	line.BodyBytes += t.bytes.body
	line.PromptBytes += t.bytes.prompt
	line.ResultBytes += t.bytes.result
	line.ErrorBytes += t.bytes.err
	line.LabelsBytes += t.bytes.labels
	line.MetadataBytes += t.bytes.metadata
	line.OtherBytes += t.bytes.other
	if t.topKey != "" {
		if m.topKey == nil {
			m.topKey = map[string]*RunWriteTopKey{}
		}
		top := m.topKey[t.topKey]
		if top == nil {
			top = &RunWriteTopKey{}
			m.topKey[t.topKey] = top
		}
		top.Writes++
		top.Bytes += t.topKeyBytes
	}
	for _, name := range t.metadataChanged {
		if m.keyChanges == nil {
			m.keyChanges = map[string]int64{}
		}
		m.keyChanges[name]++
	}
}

// stats copies the counters into the wire block. The caller holds the mutex or
// owns the maps.
func (m *runWriteMaps) stats() RunWriteStats {
	out := RunWriteStats{
		Scope: runWriteScope, Ops: m.ops,
		Lines:              make(map[string]RunWriteLine, len(m.lines)),
		TopMetadataKey:     make(map[string]RunWriteTopKey, len(m.topKey)),
		MetadataKeyChanges: copyCounts(m.keyChanges),
		Undecodable:        copyCounts(m.undecodable),
	}
	for key, line := range m.lines {
		out.Lines[key] = *line
	}
	for key, top := range m.topKey {
		out.TopMetadataKey[key] = *top
	}
	return out
}

// statsLocked is stats under the counters' lock.
func (m *runWriteMaps) statsLocked(c *telemetryCounters) RunWriteStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return m.stats()
}

// countRunWrite adds one tallied write. A nil counters counts nothing.
func (c *telemetryCounters) countRunWrite(t runWriteTally) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.runWrites.add(t)
	c.mu.Unlock()
}

// noteRunWrite counts one committed choir.run write. Every caller calls it
// after its conditional put returned without error and passes the bytes it
// already built: prev is the body before (nil when there was none,
// runWriteNoPrev when the site does not hold it) and next the body written.
func (s *Store) noteRunWrite(site string, prev, next []byte) {
	if s == nil {
		return
	}
	s.telemetryStats.countRunWrite(tallyRunWrite(site, prev, next))
}

// noteLifecycleRunWrites counts the choir.run objects of one committed
// lifecycle batch, by the command that wrote them. commitLifecycleTransition
// calls it after its conditional put returned without error. The batch holds
// the new objects and not the old bytes, so the change is no_prev.
func (s *Store) noteLifecycleRunWrites(kind types.LifecycleCommandKind, objects []objectgraph.Object) {
	site := runSiteLifecycleOther
	switch kind {
	case types.LifecycleTerminalizeRun:
		site = runSiteTerminalizeRun
	case types.LifecycleReactivateRun:
		site = runSiteReactivateRun
	case types.LifecycleBindControlDelivery:
		site = runSiteControlBind
	case types.LifecycleBindEngineeringAssignment:
		site = runSiteEngineerBind
	}
	for i := range objects {
		if objects[i].ObjectKind == ogKindRun {
			s.noteRunWrite(site, runWriteNoPrev, objects[i].Body)
		}
	}
}
