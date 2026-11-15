package store

// Release 1 of docs/design/object-bodies-inline-on-the-tape-2026-11-15.md
// (the reader), failures 16 and 23: the two consumers that re-parse an object
// op's body, the deposit upcaster and the telemetry sealer's second look, are
// driven here with a version 3 batch whose ops carry the body inline as
// "body_json". The reader is Object.UnmarshalJSON; these tests pin what each
// consumer then holds and re-encodes.

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// inlineOpBody returns the JSON of obj with its body carried as "body_json"
// holding inline exactly, as release 2's writer will emit it. The bytes are
// spliced, not marshalled: json.Marshal compacts and HTML-escapes a
// RawMessage, which is the trap the design's section 3 names.
func inlineOpBody(t *testing.T, obj objectgraph.Object, inline []byte) json.RawMessage {
	t.Helper()
	obj.Body = nil // omitempty: no "body" key
	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 2 || raw[len(raw)-1] != '}' {
		t.Fatalf("unexpected object JSON %s", raw)
	}
	out := append([]byte(nil), raw[:len(raw)-1]...)
	out = append(out, `,"body_json":`...)
	out = append(out, inline...)
	out = append(out, '}')
	return out
}

func TestDepositUpcastReadsAnInlineBodyAndReencodesItAsBody(t *testing.T) {
	s, _ := vocabUpcastOpenStore(t, "inlinebody")
	base := time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)

	// A legacy-spelled agent: the upcast rewrites the body, so it is dirty
	// and the op is re-encoded through json.Marshal(Object): the base64 form.
	legacy := vocabUpcastAgentObject(t, "super:root", "super", "a", false, base, base)
	legacyInline := []byte("{ \"agent_id\": \"super:root\",\n  \"profile\": \"super\", \"state\": \"a\", \"note\": \"<&>\" }")
	// A current-spelled agent: nothing to rewrite, so the op must come out
	// untouched, inline form and exact bytes included.
	current := vocabUpcastAgentObject(t, "management:root", "management", "c", false, base, base)
	currentInline := []byte("{ \"agent_id\": \"management:root\",\n  \"profile\": \"management\", \"state\": \"c\", \"note\": \"<&>\" }")

	dirtyBody := inlineOpBody(t, legacy, legacyInline)
	cleanBody := inlineOpBody(t, current, currentInline)
	batch := computerevent.ProjectionBatch{
		Version:          computerevent.ProjectionBatchV3,
		ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID:       vocabUpcastComputerID,
		EventID:          "inline-body-upcast",
		EventDigest:      storeTestDigest('d'),
		Ops: []computerevent.ProjectionOp{
			{Kind: computerevent.ProjectionOpObject, CanonicalID: legacy.CanonicalID, Body: append(json.RawMessage(nil), dirtyBody...)},
			{Kind: computerevent.ProjectionOpObject, CanonicalID: current.CanonicalID, Body: append(json.RawMessage(nil), cleanBody...)},
		},
	}
	if err := batch.Validate(); err != nil {
		t.Fatalf("a version 3 batch must validate: %v", err)
	}
	if err := s.upcastProjectionDeposit(&batch, true, true, false); err != nil {
		t.Fatalf("upcast of a version 3 batch with inline bodies: %v", err)
	}

	if !bytes.Contains(cleanBody, currentInline) {
		t.Fatal("test premise: the inline bytes must be in the op verbatim")
	}
	if err := batch.Validate(); err != nil {
		t.Fatalf("the upcast batch no longer validates: %v", err)
	}
	// Clean op: left exactly as recorded.
	if !bytes.Equal(batch.Ops[1].Body, cleanBody) {
		t.Fatalf("an op the upcast did not change was rewritten:\n got %s\nwant %s", batch.Ops[1].Body, cleanBody)
	}

	// Dirty op: decoded from body_json, rewritten, re-encoded as "body".
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(batch.Ops[0].Body, &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["body_json"]; ok {
		t.Fatalf("a re-encoded op still carries body_json: %s", batch.Ops[0].Body)
	}
	if _, ok := fields["body"]; !ok {
		t.Fatalf("a re-encoded op lost its body: %s", batch.Ops[0].Body)
	}
	var got objectgraph.Object
	if err := json.Unmarshal(batch.Ops[0].Body, &got); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(got.Body, &decoded); err != nil {
		t.Fatalf("re-encoded body is not JSON: %v: %s", err, got.Body)
	}
	if decoded["agent_id"] != "management:root" || decoded["note"] != "<&>" || decoded["state"] != "a" {
		t.Fatalf("the inline body was not read through the upcast: %v", decoded)
	}
	if want := objectgraph.ContentHash(got.ObjectKind, got.Body, got.Metadata); got.ContentHash != want {
		t.Fatalf("re-encoded content hash %s does not match its body %s", got.ContentHash, want)
	}
}

func TestTelemetryRecheckReadsAnInlineBodyAndLeavesTheOpAlone(t *testing.T) {
	rec := eventTableRecord("inline", "run-i", "owner-1", "traj-i", types.EventToolResult, 1, 1, eventTableBase)
	obj, err := buildEventObject(rec, rec.Timestamp)
	if err != nil {
		t.Fatal(err)
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, obj.Body, "", "  "); err != nil {
		t.Fatal(err)
	}
	op := computerevent.ProjectionOp{
		Kind:        computerevent.ProjectionOpObject,
		CanonicalID: obj.CanonicalID,
		Body:        inlineOpBody(t, obj, indented.Bytes()),
	}
	if err := recheckTelemetryOp(op); err != nil {
		t.Fatalf("recheck of an event op carrying its body inline: %v", err)
	}
	// The body the recheck read is the inline bytes, whitespace included.
	var read objectgraph.Object
	if err := json.Unmarshal(op.Body, &read); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(read.Body, indented.Bytes()) {
		t.Fatalf("inline body bytes changed on read:\n got %q\nwant %q", read.Body, indented.Bytes())
	}
}
