package agentcore

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Rolling elision of old tool results (docs/reports/autopaper-compute-
// halving-2026-10-15.md, lever 2). 75% of a research run's input is the
// replay of earlier tool results: a run is ~90 calls and each result (up to
// 2k tokens) is resent on every later call.
//
// Failure modes written first:
//   - F1. A stub drops tool_use_id, the role or is_error, so the provider
//     rejects the history (a tool_result must answer a tool_use).
//   - F2. The window slides one result per call, which rewrites the whole
//     cached prefix on every call and costs more than it saves (simulated:
//     165% cache-weighted at N=8, B=4). Guard: the view only changes at
//     batch steps, and between steps each call's view extends the last
//     call's view byte for byte.
//   - F3. A recent result is elided and the model cannot see what it just
//     asked for. Guard: the last N results are always verbatim.
//   - F4. A small result is replaced by a longer stub. Guard: results under
//     the minimum size stay whole.
//   - F5. The model cannot get an elided result back. Guard: the stub names
//     the entry id and the retrieval verb.
//   - F6. Elision runs for a desk with no retrieval verb or when the flag is
//     off. Guard: view equals the plain context.
//   - F7. A compaction summary is rewritten, or results before it are
//     counted. Guard: the summary message is untouched and counting starts
//     after it.
//   - F8. The compaction threshold keeps reading the raw size, so a run
//     whose sent context is small compacts anyway. Guard: pressure estimate
//     uses the sent view.
//   - F9. Anything in the stored entries is rewritten (the store is the
//     durable ledger). Guard: the entries are unchanged after viewing.

func elisionEntries(results int, size int) []types.RunMemoryEntry {
	var entries []types.RunMemoryEntry
	add := func(role string, msg any) {
		raw, _ := json.Marshal(msg)
		entries = append(entries, types.RunMemoryEntry{
			EntryID: fmt.Sprintf("e%03d", len(entries)), Kind: types.RunMemoryEntryMessage, Role: role, Message: raw,
		})
	}
	add("user", map[string]any{"role": "user", "content": []any{map[string]string{"type": "text", "text": "Run context:\nassignment"}}})
	for i := 0; i < results; i++ {
		id := fmt.Sprintf("call_%d", i)
		add("assistant", map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "text", "text": "step"},
			map[string]any{"type": "tool_use", "id": id, "name": "desk_go_eval", "input": map[string]string{"source": "fmt.Println(1)"}},
		}})
		add("user", map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": id, "content": fmt.Sprintf("result %03d ", i) + strings.Repeat("x", size)},
		}})
	}
	return entries
}

func testElisionPolicy() runMemoryElision {
	return runMemoryElision{Enabled: true, KeepRecent: 4, Batch: 6, MinChars: 500, Retrieval: "choir.RunMemoryEntry"}
}

func resultTexts(t *testing.T, msgs []json.RawMessage) []string {
	t.Helper()
	var out []string
	for _, raw := range msgs {
		var m struct {
			Content []struct {
				Type    string `json:"type"`
				Content string `json:"content"`
			} `json:"content"`
		}
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		for _, b := range m.Content {
			if b.Type == "tool_result" {
				out = append(out, b.Content)
			}
		}
	}
	return out
}

func TestElisionKeepsPairingRecentResultsAndSmallResults(t *testing.T) {
	entries := elisionEntries(20, 3000)
	view := testElisionPolicy().apply(entries)
	plain := buildRunMemoryContext(entries)
	if len(view) != len(plain) {
		t.Fatalf("message count changed: %d vs %d", len(view), len(plain))
	}
	texts := resultTexts(t, view)
	if len(texts) != 20 {
		t.Fatalf("results = %d", len(texts))
	}
	// 20 results, keep 4, batch 6: boundary = floor((20-4)/6)*6 = 12.
	for i, text := range texts {
		elided := strings.HasPrefix(text, "[Earlier tool output elided")
		if want := i < 12; elided != want {
			t.Fatalf("result %d elided = %v, want %v: %.80s", i, elided, want, text)
		}
	}
	for i := 16; i < 20; i++ {
		if !strings.HasPrefix(texts[i], fmt.Sprintf("result %03d ", i)) {
			t.Fatalf("recent result %d is not verbatim", i)
		}
	}
	// F1: pairing and fields survive.
	var m struct {
		Role    string `json:"role"`
		Content []struct {
			Type      string `json:"type"`
			ToolUseID string `json:"tool_use_id"`
		} `json:"content"`
	}
	if err := json.Unmarshal(view[2], &m); err != nil || m.Role != "user" || m.Content[0].Type != "tool_result" || m.Content[0].ToolUseID != "call_0" {
		t.Fatalf("elided message = %s (%v)", view[2], err)
	}
	// F5: the stub names the entry and the verb; F4: it is shorter.
	if !strings.Contains(texts[0], `choir.RunMemoryEntry("e002")`) || len(texts[0]) >= 3000 {
		t.Fatalf("stub = %q", texts[0])
	}
	// F4: small results stay whole even when old.
	small := testElisionPolicy().apply(elisionEntries(20, 100))
	for i, text := range resultTexts(t, small) {
		if strings.HasPrefix(text, "[Earlier") {
			t.Fatalf("small result %d elided", i)
		}
	}
}

func TestElisionChangesTheSentPrefixOnlyAtBatchSteps(t *testing.T) {
	policy := testElisionPolicy()
	joined := func(msgs []json.RawMessage) string {
		var b strings.Builder
		for _, m := range msgs {
			b.Write(m)
			b.WriteByte('\n')
		}
		return b.String()
	}
	steps := 0
	var prev string
	for n := 1; n <= 60; n++ {
		cur := joined(policy.apply(elisionEntries(n, 3000)))
		if prev != "" && !strings.HasPrefix(cur, prev) {
			steps++
		}
		prev = cur
	}
	// 60 results with keep 4 and batch 6: the boundary moves at n = 10, 16,
	// ..., 58, that is 9 times; a sliding window would rewrite on ~56 calls.
	if steps > 9 {
		t.Fatalf("the sent prefix was rewritten on %d of 60 calls, want at most 9", steps)
	}
	if steps == 0 {
		t.Fatalf("nothing was ever elided")
	}
}

func TestElisionOffForOtherDesksAndWhenDisabled(t *testing.T) {
	t.Setenv("CHOIR_RUN_MEMORY_ELISION", "1")
	entries := elisionEntries(30, 3000)
	plain := buildRunMemoryContext(entries)
	for name, policy := range map[string]runMemoryElision{
		"disabled":     {Enabled: false, KeepRecent: 4, Batch: 6, MinChars: 500, Retrieval: "choir.RunMemoryEntry"},
		"zero batch":   {Enabled: true, KeepRecent: 4, Batch: 0, MinChars: 500},
		"no verb desk": newRunMemoryElision(&types.RunRecord{AgentID: "texture:x", AgentProfile: agentprofile.Texture}, false),
	} {
		view := policy.apply(entries)
		if len(view) != len(plain) {
			t.Fatalf("%s: message count changed", name)
		}
		for i := range view {
			if string(view[i]) != string(plain[i]) {
				t.Fatalf("%s: message %d changed", name, i)
			}
		}
	}
}

func TestElisionLeavesCompactionSummaryAloneAndCountsAfterIt(t *testing.T) {
	entries := elisionEntries(10, 3000)
	summary := types.RunMemoryEntry{EntryID: "c1", Kind: types.RunMemoryEntryCompaction, Summary: "S", FirstKeptEntryID: "", TokensBefore: 99}
	entries = append(entries, summary)
	tail := elisionEntries(6, 3000)[1:] // no second run-context message
	for i := range tail {
		tail[i].EntryID = fmt.Sprintf("t%03d", i)
	}
	entries = append(entries, tail...)
	view := testElisionPolicy().apply(entries)
	if !strings.Contains(string(view[0]), "Context checkpoint summary") || strings.Contains(string(view[0]), "elided") {
		t.Fatalf("summary message = %s", view[0])
	}
	// 6 results after the summary, keep 4: boundary is 0, nothing elided.
	for i, text := range resultTexts(t, view) {
		if strings.HasPrefix(text, "[Earlier") {
			t.Fatalf("result %d after a compaction was elided", i)
		}
	}
}

func TestElisionDoesNotRewriteStoredEntriesAndPressureUsesTheSentView(t *testing.T) {
	entries := elisionEntries(40, 3000)
	before, _ := json.Marshal(entries)
	policy := testElisionPolicy()
	_ = policy.apply(entries)
	after, _ := json.Marshal(entries)
	if string(before) != string(after) {
		t.Fatalf("viewing rewrote the stored entries")
	}
	m := &runMemoryManager{cfg: provideriface.Config{}, elision: policy}
	withElision := m.estimatePromptPressureTokens(entries)
	m.elision = runMemoryElision{}
	without := m.estimatePromptPressureTokens(entries)
	if without-withElision < 15000 {
		t.Fatalf("pressure with elision %d, without %d: the estimate reads the raw size", withElision, without)
	}
}

func TestElisionPolicyFromEnvironmentDefaults(t *testing.T) {
	research := &types.RunRecord{AgentID: "research:doc", AgentProfile: agentprofile.Research}
	t.Setenv("CHOIR_RUN_MEMORY_ELISION", "")
	if newRunMemoryElision(research, false).Enabled {
		t.Fatalf("elision must be off when the flag is unset (owner ruling 2026-10-15)")
	}
	t.Setenv("CHOIR_RUN_MEMORY_ELISION", "1")
	p := newRunMemoryElision(research, false)
	if !p.Enabled || p.KeepRecent != defaultElisionKeepRecent || p.Batch != defaultElisionBatch || p.MinChars != defaultElisionMinChars || p.Retrieval != "choir.RunMemoryEntry" {
		t.Fatalf("defaults = %+v", p)
	}
	t.Setenv("CHOIR_RUN_MEMORY_ELISION", "0")
	if newRunMemoryElision(research, false).Enabled {
		t.Fatalf("flag 0 did not disable elision")
	}
	t.Setenv("CHOIR_RUN_MEMORY_ELISION", "1")
	t.Setenv("CHOIR_RUN_MEMORY_ELISION_KEEP", "junk")
	t.Setenv("CHOIR_RUN_MEMORY_ELISION_BATCH", "-2")
	p = newRunMemoryElision(research, false)
	if p.KeepRecent != defaultElisionKeepRecent || p.Batch != defaultElisionBatch {
		t.Fatalf("garbage did not fall back: %+v", p)
	}
}
