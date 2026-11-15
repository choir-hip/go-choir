package store

// The choir.run write counter (T2 of the store-once goal, section 9.4 of
// docs/design/run-record-written-less-2026-11-15.md): per run write, the site
// that wrote it, the state it wrote, which top-level fields differ from the
// write before it, and the body bytes by field. It is a counter and nothing
// else. Ways it could fail, written before the tests and the code:
//
//  1. It changes a write: the body, the hash, the metadata or the error a
//     writer returns. It must only read the bytes the writer already built.
//  2. It counts a write that did not commit: before the conditional put, on a
//     lost condition that is retried (counted twice), on a failed update of a
//     run that does not exist, or on a replay the store skipped.
//  3. Its key space grows without bound because a run id, a free-text state, a
//     metadata key name or a model-chosen string became part of a key.
//  4. It delays a write: it takes the counter lock while it decodes, it does
//     I/O, or it grows work with something other than the body it was handed.
//     The decode is one pass over the body, outside the lock; the lock covers
//     map adds only.
//  5. Its field sizes are wrong: they must be the stored bytes of the field's
//     value in the body, so that prompt + result + error + labels + metadata +
//     other equals the body length.
//  6. It names a metadata key that is not in the fixed list (a key name can be
//     chosen by whoever writes the run). Only a listed key is reported by
//     name; any other is "other".
//  7. It races: two writers count while the observability read copies.
//  8. A body that does not decode panics the writer, fails the write, or is
//     silently dropped. It must count as undecodable, by site, and nothing
//     more.
//  9. It is read as a history. It is a count since boot; a restart starts it at
//     zero, and the served block says so.
// 10. A site is mislabeled or missed, so the served total disagrees with the
//     tape's own per-kind op count. The attributed total is served beside it so
//     a gap shows (the agentcore test compares the two on the standard flow).

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func runBodyOf(t *testing.T, fields map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func baseRunFields() map[string]any {
	return map[string]any{
		"loop_id": "run-1", "agent_id": "agent-1", "state": "running",
		"prompt": "do the thing", "updated_at": "2026-11-15T10:00:00Z",
		"metadata": map[string]any{"llm_fallbacks": "a/b,c/d", "request_source": "x"},
	}
}

func TestRunWriteTallyFieldBytesAddUpToTheBody(t *testing.T) {
	f := baseRunFields()
	f["result"] = strings.Repeat("r", 300)
	f["error"] = "boom"
	f["labels"] = []string{"a", "b"}
	f["metadata"] = map[string]any{"texture_available_source_entities": strings.Repeat("e", 5000), "llm_fallbacks": "x"}
	body := runBodyOf(t, f)
	got := tallyRunWrite(runSiteActivation, nil, body)
	if got.undecodable {
		t.Fatal("a valid body counted as undecodable")
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(body, &raw)
	if got.bytes.prompt != int64(len(raw["prompt"])) || got.bytes.result != int64(len(raw["result"])) ||
		got.bytes.err != int64(len(raw["error"])) || got.bytes.labels != int64(len(raw["labels"])) ||
		got.bytes.metadata != int64(len(raw["metadata"])) {
		t.Errorf("field bytes = %+v, want the stored value lengths", got.bytes)
	}
	sum := got.bytes.prompt + got.bytes.result + got.bytes.err + got.bytes.labels + got.bytes.metadata + got.bytes.other
	if got.bytes.body != int64(len(body)) || sum != got.bytes.body {
		t.Errorf("body %d, fields sum %d, want both %d", got.bytes.body, sum, len(body))
	}
}

func TestRunWriteTallyNamesTheLargestMetadataKeyOnlyWhenListed(t *testing.T) {
	f := baseRunFields()
	f["metadata"] = map[string]any{"texture_available_source_entities": strings.Repeat("e", 4000), "llm_fallbacks": "x"}
	got := tallyRunWrite(runSiteActivation, nil, runBodyOf(t, f))
	if got.topKey != "texture_available_source_entities" || got.topKeyBytes < 4000 {
		t.Errorf("top key = %q %d", got.topKey, got.topKeyBytes)
	}
	f["metadata"] = map[string]any{"some_key_a_writer_made_up": strings.Repeat("e", 4000), "llm_fallbacks": "x"}
	got = tallyRunWrite(runSiteActivation, nil, runBodyOf(t, f))
	if got.topKey != runWriteOtherKey || got.topKeyBytes < 4000 {
		t.Errorf("an unlisted key must be reported as %q with its size, got %q %d", runWriteOtherKey, got.topKey, got.topKeyBytes)
	}
	f["metadata"] = map[string]any{}
	got = tallyRunWrite(runSiteActivation, nil, runBodyOf(t, f))
	if got.topKey != "" || got.topKeyBytes != 0 {
		t.Errorf("an empty metadata has no top key, got %q %d", got.topKey, got.topKeyBytes)
	}
}

func TestRunWriteTallyClassifiesWhatChanged(t *testing.T) {
	prev := baseRunFields()
	same := func() map[string]any { m := baseRunFields(); return m }
	cases := []struct {
		name   string
		mutate func(m map[string]any)
		want   string
	}{
		{"nothing", func(m map[string]any) {}, runChangeUnchanged},
		{"updated_at only", func(m map[string]any) { m["updated_at"] = "2026-11-15T10:00:05Z" }, runChangeUpdatedAtOnly},
		{"state", func(m map[string]any) { m["state"] = "completed"; m["updated_at"] = "2026-11-15T10:00:05Z" }, "state"},
		{"state and result", func(m map[string]any) { m["state"] = "completed"; m["result"] = "ok" }, "result+state"},
		{"labels", func(m map[string]any) { m["labels"] = []string{"a"} }, "labels"},
		{"prompt", func(m map[string]any) { m["prompt"] = "other" }, "prompt"},
		{"metadata", func(m map[string]any) {
			m["metadata"] = map[string]any{"llm_fallbacks": "a/b,c/d", "request_source": "y"}
		}, "metadata"},
		{"a field this counter does not list", func(m map[string]any) { m["channel_id"] = "c" }, "other"},
	}
	for _, tc := range cases {
		next := same()
		tc.mutate(next)
		got := tallyRunWrite(runSiteUpdateOG, runBodyOf(t, prev), runBodyOf(t, next))
		if got.change != tc.want {
			t.Errorf("%s: change = %q, want %q", tc.name, got.change, tc.want)
		}
	}
	if got := tallyRunWrite(runSiteActivation, nil, runBodyOf(t, prev)); got.change != runChangeNew {
		t.Errorf("no previous write: change = %q, want %q", got.change, runChangeNew)
	}
	if got := tallyRunWrite(runSiteTerminalizeRun, runWriteNoPrev, runBodyOf(t, prev)); got.change != runChangeNoPrev {
		t.Errorf("a site that has no previous body: change = %q, want %q", got.change, runChangeNoPrev)
	}
	if got := tallyRunWrite(runSiteUpdateOG, []byte("not json"), runBodyOf(t, prev)); got.change != runChangeNoPrev || got.undecodable {
		t.Errorf("an undecodable previous body is no_prev, not an undecodable write: %+v", got)
	}
}

func TestRunWriteTallyReportsWhichListedMetadataKeysChanged(t *testing.T) {
	prev := baseRunFields()
	next := baseRunFields()
	next["metadata"] = map[string]any{"llm_fallbacks": "a/b,c/d", "request_source": "x",
		"input_tokens": 5, "a_key_nobody_listed": 1}
	got := tallyRunWrite(runSiteActivation, runBodyOf(t, prev), runBodyOf(t, next))
	if len(got.metadataChanged) != 2 || got.metadataChanged[0] != "input_tokens" || got.metadataChanged[1] != runWriteOtherKey {
		t.Errorf("changed metadata keys = %v, want [input_tokens other]", got.metadataChanged)
	}
}

func TestRunWriteTallyUndecodableBodyNeverPanics(t *testing.T) {
	for _, body := range [][]byte{nil, {}, []byte("not json"), []byte("[1,2,3]"), []byte(`"x"`), []byte(`{"state":`), []byte("null")} {
		got := tallyRunWrite(runSiteCreateOG, nil, body)
		if !got.undecodable {
			t.Errorf("%q must count as undecodable", body)
		}
	}
	// A decodable body with odd field types is a write, not a failure: the
	// state is one of the closed names, and a metadata that is not an object
	// has no top key.
	odd := tallyRunWrite(runSiteCreateOG, nil, []byte(`{"state":5,"metadata":"text","prompt":7}`))
	if odd.undecodable || odd.state != runStateOther || odd.topKey != "" {
		t.Errorf("odd types: %+v", odd)
	}
	var c telemetryCounters
	c.countRunWrite(tallyRunWrite(runSiteCreateOG, nil, []byte("not json")))
	st := c.runWrites.stats()
	if st.Undecodable[runSiteCreateOG] != 1 || len(st.Lines) != 0 {
		t.Errorf("undecodable counted as %+v", st)
	}
}

func TestRunWriteKeySpaceIsFixedAndBounded(t *testing.T) {
	var c telemetryCounters
	for i := 0; i < 5000; i++ {
		f := baseRunFields()
		f["loop_id"] = fmt.Sprintf("run-%d", i)
		f["state"] = fmt.Sprintf("a-state-%d", i)
		f["metadata"] = map[string]any{fmt.Sprintf("key_%d", i): strings.Repeat("x", 10+i%7)}
		prev := baseRunFields()
		prev[fmt.Sprintf("extra_%d", i)] = i
		c.countRunWrite(tallyRunWrite(runSiteActivation, runBodyOf(t, prev), runBodyOf(t, f)))
	}
	st := c.runWrites.stats()
	if len(st.Lines) == 0 || len(st.Lines) > runWriteMaxLines {
		t.Fatalf("lines = %d, want 1..%d", len(st.Lines), runWriteMaxLines)
	}
	for key := range st.Lines {
		parts := strings.Split(key, "/")
		if len(parts) != 3 || !runWriteSites[parts[0]] || !runWriteStates[parts[1]] {
			t.Errorf("key %q is not site/state/change from the fixed sets", key)
		}
		if strings.Contains(key, "run-") || strings.Contains(key, "a-state") || strings.Contains(key, "key_") || strings.Contains(key, "extra_") {
			t.Errorf("key %q carries free text", key)
		}
	}
	for name := range st.TopMetadataKey {
		if name != runWriteOtherKey && !runWriteMetadataKeys[name] {
			t.Errorf("top metadata key %q is not in the fixed list", name)
		}
	}
	for name := range st.MetadataKeyChanges {
		if name != runWriteOtherKey && !runWriteMetadataKeys[name] {
			t.Errorf("changed metadata key %q is not in the fixed list", name)
		}
	}
	if st.Ops != 5000 {
		t.Errorf("ops = %d, want every write counted even past the key bound", st.Ops)
	}
}

func TestRunWriteCounterIsRaceClean(t *testing.T) {
	var c telemetryCounters
	var wg sync.WaitGroup
	body := runBodyOf(t, baseRunFields())
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				c.countRunWrite(tallyRunWrite(runSiteActivation, body, body))
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = c.runWrites.statsLocked(&c)
		}
	}()
	wg.Wait()
	if got := c.runWrites.statsLocked(&c).Ops; got != 16*200 {
		t.Errorf("ops = %d, want %d", got, 16*200)
	}
}

func TestRunWriteCounterIsNilSafeAndStatesItsScope(t *testing.T) {
	var c *telemetryCounters
	c.countRunWrite(tallyRunWrite(runSiteActivation, nil, []byte(`{}`))) // must not panic
	var s Store
	s.noteRunWrite(runSiteActivation, nil, []byte(`{}`)) // a zero store has nowhere to count; must not panic
	st := (&runWriteMaps{}).stats()
	if !strings.Contains(st.Scope, "since boot") || !strings.Contains(st.Scope, "restart") {
		t.Errorf("scope = %q, it must say the count is since boot and resets on restart", st.Scope)
	}
}

// Store-level: the counter follows commits, not attempts.

func runWritesOf(s *Store) RunWriteStats { return s.TelemetryStats().RunWrites }

func TestRunWriteCountsACommittedCreateAndUpdateByTheirSites(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	rec := seedLabelRun(t, s, "run-count-1") // running, created through CreateRunOG
	st := runWritesOf(s)
	if got := st.Lines["create_og/running/new"].Ops; got != 1 {
		t.Fatalf("create_og/running/new ops = %d, lines %v", got, st.Lines)
	}
	again := rec
	again.UpdatedAt = rec.UpdatedAt.Add(time.Second)
	if err := s.UpdateRun(ctx, again); err != nil {
		t.Fatal(err)
	}
	st = runWritesOf(s)
	line := st.Lines["update_og/running/"+runChangeUpdatedAtOnly]
	if line.Ops != 1 {
		t.Fatalf("an updated_at-only update must show as update_og/running/updated_at_only, lines %v", st.Lines)
	}
	if line.BodyBytes <= 0 || line.BodyBytes != line.PromptBytes+line.ResultBytes+line.ErrorBytes+line.LabelsBytes+line.MetadataBytes+line.OtherBytes {
		t.Errorf("line bytes do not add up: %+v", line)
	}
}

func TestRunWriteDoesNotCountALostConditionOrAMissingRun(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	rec := seedLabelRun(t, s, "run-count-2")
	labels := types.NewLabelSet(types.LabelWorldUntrusted)
	fired := false
	runUpdateAfterRead = func() {
		if fired {
			return
		}
		fired = true
		if _, err := s.UnionRunLabels(ctx, rec.OwnerID, rec.ComputerID, rec.RunID, labels); err != nil {
			t.Errorf("label persist: %v", err)
		}
	}
	t.Cleanup(func() { runUpdateAfterRead = func() {} })
	done := rec
	now := time.Now().UTC()
	done.State, done.FinishedAt, done.UpdatedAt = types.RunCompleted, &now, now
	if err := s.UpdateRun(ctx, done); err != nil { // attempt 1 loses its condition, attempt 2 commits
		t.Fatal(err)
	}
	if !fired {
		t.Fatal("the interleaving never happened")
	}
	st := runWritesOf(s)
	if got := st.Lines["update_og/completed/finished_at+state"].Ops; got != 1 {
		t.Errorf("the update that lost its condition once must count once, lines %v", st.Lines)
	}
	if got := st.Lines["labels/running/labels"].Ops; got != 1 {
		t.Errorf("labels write must count once, lines %v", st.Lines)
	}
	if st.Ops != 3 { // create, labels, one committed update
		t.Errorf("total = %d, want 3 (create, labels, update); lines %v", st.Ops, st.Lines)
	}
	missing := types.RunRecord{RunID: "no-such-run", OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, State: types.RunRunning}
	if err := s.UpdateRunOG(ctx, missing); err == nil {
		t.Fatal("updating a run that does not exist must fail")
	}
	if got := runWritesOf(s).Ops; got != 3 {
		t.Errorf("a failed update was counted: total %d, want 3", got)
	}
}

func TestRunWriteCounterLeavesTheStoredRunExactlyAsWritten(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	rec := seedLabelRun(t, s, "run-count-3")
	rec.Prompt = "the prompt"
	rec.Metadata = map[string]any{"llm_fallbacks": "a/b", "texture_available_source_entities": []string{"x", "y"}}
	rec.UpdatedAt = rec.UpdatedAt.Add(time.Second)
	if err := s.UpdateRun(ctx, rec); err != nil {
		t.Fatal(err)
	}
	obj, err := s.getRunObjectByOwnerOG(ctx, rec.OwnerID, rec.RunID)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := json.Marshal(rec)
	if string(obj.Body) != string(want) {
		t.Errorf("stored body differs from the record written:\n got %s\nwant %s", obj.Body, want)
	}
	if got := runWritesOf(s).Lines["update_og/running/metadata+prompt"].PromptBytes; got != int64(len(`"the prompt"`)) {
		t.Errorf("prompt bytes = %d, lines %v", got, runWritesOf(s).Lines)
	}
}

// The lifecycle writers: activation (projectLifecycleRun) and the commits that
// carry the run in a batch (terminalize, reactivate). A replayed command and a
// projection the store skips as identical write nothing and count nothing.
func TestRunWriteCountsTheLifecycleWritersAndNotReplays(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatal(err)
	}
	project := func(run types.RunRecord, id string) {
		t.Helper()
		req := types.ReplaceLifecycleActivationRequest{OwnerID: start.OwnerID, ComputerID: start.ComputerID,
			CommandID: id, TrajectoryID: start.TrajectoryID, AgentID: start.Agent.AgentID, Run: run}
		req.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(req)
		if _, err := s.ReplaceLifecycleActivation(ctx, req); err != nil {
			t.Fatalf("project %s: %v", id, err)
		}
	}
	run := lifecycleRunFixture(start, "run-count-lc", types.RunPassivated)
	project(run, "count-project-1")
	if got := runWritesOf(s).Lines["activation/passivated/new"].Ops; got != 1 {
		t.Fatalf("first projection: lines %v", runWritesOf(s).Lines)
	}
	react := types.ReactivateRunRequest{OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "count-react-1", TrajectoryID: start.TrajectoryID, AgentID: start.Agent.AgentID,
		RunID: run.RunID, TargetState: types.RunPending}
	react.CommandDigest, _ = ComputeReactivateRunDigest(react)
	if _, err := s.ReactivateRun(ctx, react); err != nil {
		t.Fatal(err)
	}
	if got := runWritesOf(s).Lines["reactivate_run/pending/"+runChangeNoPrev].Ops; got != 1 {
		t.Fatalf("reactivate: lines %v", runWritesOf(s).Lines)
	}
	running := run
	running.State, running.UpdatedAt = types.RunRunning, run.UpdatedAt.Add(time.Second)
	project(running, "count-project-2")
	var activationRunning int64
	for key, line := range runWritesOf(s).Lines {
		if strings.HasPrefix(key, "activation/running/") {
			activationRunning += line.Ops
		}
	}
	if activationRunning != 1 {
		t.Fatalf("running projection: lines %v", runWritesOf(s).Lines)
	}
	before := runWritesOf(s).Ops
	project(running, "count-project-3") // the same body again: the store skips it
	if got := runWritesOf(s).Ops; got != before {
		t.Errorf("a projection the store skipped as identical was counted: %d -> %d", before, got)
	}
	term := types.TerminalizeRunRequest{OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "count-term-1", TrajectoryID: start.TrajectoryID, AgentID: start.Agent.AgentID,
		RunID: run.RunID, TerminalState: types.RunCancelled, Reason: "test"}
	term.CommandDigest, _ = ComputeTerminalizeRunDigest(term)
	if _, err := s.TerminalizeRun(ctx, term); err != nil {
		t.Fatal(err)
	}
	if got := runWritesOf(s).Lines["terminalize_run/cancelled/"+runChangeNoPrev].Ops; got != 1 {
		t.Fatalf("terminalize: lines %v", runWritesOf(s).Lines)
	}
	after := runWritesOf(s).Ops
	if replay, err := s.TerminalizeRun(ctx, term); err != nil || !replay.Replay {
		t.Fatalf("replay: %v %+v", err, replay)
	}
	if got := runWritesOf(s).Ops; got != after {
		t.Errorf("a replayed command was counted: %d -> %d", after, got)
	}
	if len(runWritesOf(s).Undecodable) != 0 {
		t.Errorf("undecodable = %v", runWritesOf(s).Undecodable)
	}
}
