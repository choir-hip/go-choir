package agentcore

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Rolling elision of old tool results (docs/reports/autopaper-compute-
// halving-2026-10-15.md, lever 2). 75% of a research run's input is the
// replay of earlier tool results: a run makes ~90 calls and resends every
// earlier result (each up to 2k tokens) on all of them.
//
// The context sent to the provider keeps the last KeepRecent tool results
// verbatim and replaces older ones of at least MinChars with a short stub
// that names the entry to read the output back (choir.RunMemoryEntry) and
// the first line of the output. The durable run memory is never rewritten.
//
// A sliding window would rewrite the whole cached prefix on every call and
// cost more than it saves (simulated: 165% cache-weighted). So the boundary
// moves only in steps of Batch results: with m results so far, the first
// floor((m-KeepRecent)/Batch)*Batch are elided. Between steps every call's
// context extends the last call's byte for byte, so the provider's prefix
// cache holds; at a step the prefix is rewritten once.
//
// Defaults are conservative: 8 verbatim, steps of 24 (a run of 90 calls is
// rewritten 3 times), results under 1,200 characters stay whole (a stub is
// not shorter). Research only, the one desk that has the retrieval verb.
//
// OFF BY DEFAULT (owner ruling 2026-10-15): elision rewrites already-cached
// history bytes, which works against the >90% prompt-cache goal; the
// append-only prompt pattern is kept. Opt in with CHOIR_RUN_MEMORY_ELISION=1
// on the guest runtime, or turn it on for one computer with the owner's
// research.run_memory_elision setting (research_switches.go).
const (
	defaultElisionKeepRecent = 8
	defaultElisionBatch      = 24
	defaultElisionMinChars   = 1200
	elisionStubPrefix        = "[Earlier tool output elided"
)

type runMemoryElision struct {
	Enabled    bool
	KeepRecent int
	Batch      int
	MinChars   int
	// Retrieval is the cell verb that reads a run-memory entry back.
	Retrieval string
}

func elisionIntEnv(name string, def int, min int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name))); err == nil && n >= min {
		return n
	}
	return def
}

// newRunMemoryElision is the run's policy: off unless the computer's
// research.run_memory_elision setting is on (settingOn) or
// CHOIR_RUN_MEMORY_ELISION is 1, on, true or yes (the fleet switch), and then
// only for desks that can read an entry back. The KEEP, BATCH and MIN_CHARS
// env knobs tune either way.
func newRunMemoryElision(rec *types.RunRecord, settingOn bool) runMemoryElision {
	if !settingOn {
		switch strings.ToLower(strings.TrimSpace(os.Getenv("CHOIR_RUN_MEMORY_ELISION"))) {
		case "1", "on", "true", "yes":
		default:
			return runMemoryElision{}
		}
	}
	retrieval := runMemoryRetrievalVerb(rec)
	if retrieval == "" {
		return runMemoryElision{}
	}
	return runMemoryElision{
		Enabled:    true,
		KeepRecent: elisionIntEnv("CHOIR_RUN_MEMORY_ELISION_KEEP", defaultElisionKeepRecent, 1),
		Batch:      elisionIntEnv("CHOIR_RUN_MEMORY_ELISION_BATCH", defaultElisionBatch, 1),
		MinChars:   elisionIntEnv("CHOIR_RUN_MEMORY_ELISION_MIN_CHARS", defaultElisionMinChars, 1),
		Retrieval:  retrieval,
	}
}

// apply is the context to send: the plain run-memory context with old tool
// results elided. The entries are not modified.
func (p runMemoryElision) apply(entries []types.RunMemoryEntry) []json.RawMessage {
	items := buildRunMemoryContextItems(entries)
	p.elide(items)
	messages := make([]json.RawMessage, len(items))
	for i, item := range items {
		messages[i] = item.msg
	}
	return messages
}

// elide rewrites old tool-result messages in items and returns how many
// results are behind the boundary.
func (p runMemoryElision) elide(items []runMemoryContextItem) int {
	if !p.Enabled || p.Batch <= 0 || p.KeepRecent < 0 {
		return 0
	}
	var results []int
	for i, item := range items {
		if isToolResultOnlyMessage(item.msg) {
			results = append(results, i)
		}
	}
	if len(results) <= p.KeepRecent {
		return 0
	}
	boundary := ((len(results) - p.KeepRecent) / p.Batch) * p.Batch
	for _, idx := range results[:boundary] {
		if stub, ok := p.stubMessage(items[idx]); ok {
			items[idx].msg = stub
		}
	}
	return boundary
}

// stubMessage replaces each long string tool_result of a message with a
// stub, keeping role, tool_use_id and is_error. ok is false when nothing
// in the message is worth eliding.
func (p runMemoryElision) stubMessage(item runMemoryContextItem) (json.RawMessage, bool) {
	var msg map[string]any
	if json.Unmarshal(item.msg, &msg) != nil {
		return nil, false
	}
	blocks, _ := msg["content"].([]any)
	changed := false
	for _, b := range blocks {
		block, _ := b.(map[string]any)
		if block == nil || block["type"] != "tool_result" {
			continue
		}
		text, isString := block["content"].(string)
		if !isString || len(text) < p.MinChars || strings.HasPrefix(text, elisionStubPrefix) {
			continue
		}
		block["content"] = p.stubText(item.entryID, text)
		changed = true
	}
	if !changed {
		return nil, false
	}
	out, err := json.Marshal(msg)
	if err != nil {
		return nil, false
	}
	return out, true
}

func (p runMemoryElision) stubText(entryID, text string) string {
	first := text
	if i := strings.IndexByte(first, '\n'); i >= 0 {
		first = first[:i]
	}
	first = truncateRunesForStub(strings.TrimSpace(first), 100)
	var b strings.Builder
	fmt.Fprintf(&b, "%s to keep the context small (%d characters). It began: %q.", elisionStubPrefix, len(text), first)
	if p.Retrieval != "" && strings.TrimSpace(entryID) != "" {
		fmt.Fprintf(&b, " Read it again with %s(%q), or re-run the cell; searches and variables of earlier cells are still in this session.]", p.Retrieval, entryID)
	} else {
		b.WriteString(" Re-run the cell to see it again.]")
	}
	return b.String()
}

func truncateRunesForStub(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit]) + "…"
}

// sentContext is the run-memory context for a provider call. It announces a
// batch step once, as run progress, so the cost of a rewrite is visible.
func (m *runMemoryManager) sentContext(entries []types.RunMemoryEntry) []json.RawMessage {
	items := buildRunMemoryContextItems(entries)
	boundary := m.elision.elide(items)
	if boundary > m.elidedThrough && m.emit != nil {
		m.elidedThrough = boundary
		payload, _ := json.Marshal(map[string]any{
			"elided_results": boundary,
			"keep_recent":    m.elision.KeepRecent,
			"batch":          m.elision.Batch,
		})
		m.emit(types.EventRunProgress, "run_memory_elision", payload)
	}
	messages := make([]json.RawMessage, len(items))
	for i, item := range items {
		messages[i] = item.msg
	}
	return messages
}
