package jevrecord

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Ways the record can fail (docs/evidence/jev-measurement-2026-11-10.md,
// "Build: failure list", Recording), each with its test:
//   - a failed call records nothing;
//   - the write is slow or fails and holds the caller;
//   - the writer is left running, or loses what was queued at Close;
//   - a large question or answer makes a record without bound;
//   - the log grows without bound;
//   - the state text (which can carry an owner's request) is stored;
//   - the answer's distribution is not kept, so a new threshold cannot replay.

const distribution = `{"id":"gen-1","provider":"typesafe","model":"typesafe/jev-1.14-20261014",
 "answers":{"event":{"choice":"c1","probabilities":{"c1":0.91,"c2":0.05,"none":0.04}}},
 "usage":{"input_tokens":120,"output_tokens":9,"cost":0.0000168}}`

func questions() map[string]json.RawMessage {
	return map[string]json.RawMessage{
		"event": json.RawMessage(`{"type":"choice","instructions":"Which story reports the same event?","choices":["c1","c2","none"]}`),
	}
}

func readLines(t *testing.T, path string) []Record {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []Record
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			t.Fatalf("bad line %q: %v", line, err)
		}
		out = append(out, r)
	}
	return out
}

func TestObserveRecordsSuccessWithTheWholeDistribution(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calls.jsonl")
	rec := NewRecorder(path)
	state := json.RawMessage(`{"capture":{"title":"the owner asked for a secret thing"}}`)
	raw, err := Observe(rec, Meta{Site: "intake_labels", DecisionID: "cap-1", Threshold: "join=0.92"}, state, questions(),
		func() (json.RawMessage, error) { return json.RawMessage(distribution), nil })
	if err != nil || string(raw) != distribution {
		t.Fatalf("Observe changed the result: %v", err)
	}
	rec.Close()
	got := readLines(t, path)
	if len(got) != 1 {
		t.Fatalf("records = %d", len(got))
	}
	r := got[0]
	if !r.OK || r.Site != "intake_labels" || r.DecisionID != "cap-1" || r.Threshold != "join=0.92" || r.Model != "typesafe/jev-1.14-20261014" ||
		r.InputTokens != 120 || r.CostUSD == 0 || r.ResponseID != "gen-1" {
		t.Fatalf("record = %+v", r)
	}
	if a := r.Answers["event"]; a.Choice != "c1" || a.P["c2"] != 0.05 {
		t.Fatalf("distribution not kept: %+v", a)
	}
	if len(r.Questions) != 1 || len(r.Questions[0].Options) != 3 || r.Questions[0].Digest == "" {
		t.Fatalf("questions = %+v", r.Questions)
	}
	if line, _ := os.ReadFile(path); strings.Contains(string(line), "secret thing") {
		t.Fatal("the state text was stored")
	}
}

func TestObserveRecordsFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calls.jsonl")
	rec := NewRecorder(path)
	boom := errors.New("OpenRouter returned 502 Bad Gateway")
	_, err := Observe(rec, Meta{Site: "front_importance", DecisionID: "pub-1@2"}, json.RawMessage(`{}`), questions(),
		func() (json.RawMessage, error) { return nil, boom })
	if !errors.Is(err, boom) {
		t.Fatalf("error changed: %v", err)
	}
	rec.Close()
	got := readLines(t, path)
	if len(got) != 1 || got[0].OK || !strings.Contains(got[0].Error, "502") || got[0].Site != "front_importance" {
		t.Fatalf("failure not recorded: %+v", got)
	}
}

func TestRecorderNeverBlocksTheCaller(t *testing.T) {
	// The path's parent is a file, so every write fails.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := NewRecorder(filepath.Join(blocker, "calls.jsonl"))
	start := time.Now()
	for i := 0; i < 5000; i++ {
		rec.Record(Record{Schema: Schema, Site: "x"})
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("5000 records took %s on a failing disk", took)
	}
	rec.Close()
	var nilRec *Recorder
	nilRec.Record(Record{}) // a nil recorder is a no-op
	nilRec.Close()
}

func TestRecorderCloseDrainsAndRefusesAfter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calls.jsonl")
	rec := NewRecorder(path)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rec.Record(Record{Schema: Schema, Site: fmt.Sprint(i), OK: true})
		}(i)
	}
	wg.Wait()
	rec.Close()
	if got := readLines(t, path); len(got) != 50 {
		t.Fatalf("records after Close = %d, want 50", len(got))
	}
	rec.Record(Record{Schema: Schema})
	if rec.Dropped() != 1 {
		t.Fatalf("dropped = %d, want 1", rec.Dropped())
	}
}

func TestRecordIsBounded(t *testing.T) {
	qs := map[string]json.RawMessage{}
	answers := map[string]map[string]any{}
	for i := 0; i < 40; i++ {
		name := fmt.Sprintf("k%02d", i)
		choices := make([]string, 30)
		p := map[string]float64{}
		for j := range choices {
			choices[j] = fmt.Sprintf("choice-%d", j)
			p[choices[j]] = 1.0 / 30
		}
		q, _ := json.Marshal(map[string]any{"type": "choice", "instructions": strings.Repeat("long ", 400), "choices": choices})
		qs[name] = q
		answers[name] = map[string]any{"choice": "choice-1", "probabilities": p}
	}
	body, _ := json.Marshal(map[string]any{"model": "m", "answers": answers})
	r := Build(Meta{Site: "event_layer"}, json.RawMessage(`{}`), qs, body, nil, time.Now(), time.Millisecond)
	line := Encode(r)
	if len(line) > MaxRecordBytes {
		t.Fatalf("record = %d bytes, cap %d", len(line), MaxRecordBytes)
	}
	var back Record
	if err := json.Unmarshal(line, &back); err != nil {
		t.Fatal(err)
	}
	for name, a := range back.Answers {
		if len(a.P) > topProbs+1 {
			t.Fatalf("answer %s keeps %d probabilities", name, len(a.P))
		}
	}
}

func TestRecorderRotates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calls.jsonl")
	rec := NewRecorder(path)
	rec.maxFile, rec.keep = 2000, 3
	for i := 0; i < 400; i++ {
		rec.Record(Record{Schema: Schema, Site: "x", OK: true})
		if i%20 == 0 {
			time.Sleep(5 * time.Millisecond) // let the drainer write between rotations
		}
	}
	rec.Close()
	if _, err := os.Stat(path + ".3"); err == nil {
		t.Fatal("kept more files than the bound")
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("no rotation happened: %v", err)
	}
}

// A site whose questions an agent wrote keeps the digest, not the text.
func TestNoTextKeepsNeitherExcerptNorOptions(t *testing.T) {
	r := Build(Meta{Site: "system1_verb", NoText: true}, json.RawMessage(`{}`), questions(), json.RawMessage(distribution), nil, time.Now(), time.Millisecond)
	q := r.Questions[0]
	if q.Excerpt != "" || len(q.Options) != 0 || q.Digest == "" || q.Name != "event" {
		t.Fatalf("question = %+v", q)
	}
}
