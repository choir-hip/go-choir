package store

// Release 1 of docs/design/object-bodies-inline-on-the-tape-2026-11-15.md
// (the reader), through the store's own projection and event-row decode.
// Written before the code. Failure list, from sections 1, 6 and 8 of the
// design, restricted to what the reader can fail through the store:
//
//  S1. A version 3 batch whose object op carries body_json is refused by
//      FinalizeBatch (the validator) or projected into og_objects/og_bodies
//      with an EMPTY body (the decode ignores the field). It must project the
//      exact body bytes. (failures 1, 11)
//  S2. The projected row of an inline op differs from the projected row of the
//      same object in the base64 form of a version 2 batch: the og_objects
//      header, the og_bodies body, the content_hash. They must be equal.
//      (section 3, "Proof"; old-form tape replay equality)
//  S3. A mixed version 3 batch (one inline op, one base64 op, in one batch)
//      projects both bodies exactly. (failure 11)
//  S4. decodeChoirEventRow (a stamped acknowledged event) decodes an inline
//      op into the same row as the base64 op: the body, the metadata fields,
//      the run and sequence columns. A body that is not an event record is
//      still refused. (section 1 reader list; failure 23's reading half)
//  S5. A golden: a fixed version 2 batch written in the pre-change encoding
//      (base64 body) and the same objects inline in a version 3 batch decode
//      to equal Objects, equal bodies and equal ContentHash per op, and
//      projecting each into its own store gives equal og_objects dumps.
//      (failure 28, "old-form tape replay equality")
//  S6. Opening a store in release 1 writes no floor of 7 and no inline
//      anything: a fresh store's floor file is absent or below 7, and its
//      schema receipt reads 7. (section 4: "It writes no floor"; the epoch)
//  S7. The upcaster re-encodes an inline-decoded object as base64 in memory
//      (failure 16): json.Marshal of the decoded object has body, not
//      body_json.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
	"github.com/yusefmosiah/go-choir/internal/types"
)

var inlineReaderAt = time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)

// inlineObjectJSON is an object op's JSON with the body given as either form.
func inlineObjectJSON(t *testing.T, id string, body []byte, inline bool) json.RawMessage {
	t.Helper()
	obj := objectgraph.Object{
		CanonicalID: id, ObjectKind: "choir.texture_revision", OwnerID: "owner-1", ComputerID: "computer-project",
		VersionID: "v1", Metadata: json.RawMessage(`{"k":1}`), CreatedAt: inlineReaderAt, UpdatedAt: inlineReaderAt,
	}
	obj.ContentHash = objectgraph.ContentHash(obj.ObjectKind, body, obj.Metadata)
	if !inline {
		obj.Body = body
		raw, err := json.Marshal(obj) // the pre-change encoding: body is base64
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	return spliceInline(t, obj, body)
}

// spliceInline writes the object without a body and appends the one new
// field by hand, so the body bytes are exactly the given bytes.
func spliceInline(t *testing.T, obj objectgraph.Object, body []byte) json.RawMessage {
	t.Helper()
	obj.Body = nil
	raw, err := json.Marshal(obj)
	if err != nil || len(raw) == 0 || raw[len(raw)-1] != '}' {
		t.Fatalf("marshal: %v %s", err, raw)
	}
	out := append(append(append([]byte{}, raw[:len(raw)-1]...), []byte(`,"body_json":`)...), body...)
	return append(out, '}')
}

func finalizeInlineBatch(t *testing.T, s *Store, version int, ops []computerevent.ProjectionOp) {
	t.Helper()
	ctx := context.Background()
	computerID := "computer-project"
	prepareGenesis(t, s, computerID, "genesis-inline")
	head, err := s.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("genesis head: %v %#v", err, head)
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event, digest := prepareProjectionEvent(t, s, *head, eventID, "inline-1")
	batch := computerevent.ProjectionBatch{
		Version: version, ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID: computerID, EventID: event.EventID, EventDigest: digest, Ops: ops,
	}
	if err := s.FinalizeBatch(ctx, computerID, digest, signedReceipt(t, computerID, digest, event.Sequence), &batch); err != nil {
		t.Fatalf("FinalizeBatch version %d: %v", version, err)
	}
}

type projectedRow struct {
	kind, owner, hash, metadata, body string
}

func projectedRows(t *testing.T, s *Store) map[string]projectedRow {
	t.Helper()
	rows, err := s.db.Query(`SELECT o.canonical_id, o.object_kind, o.owner_id, o.content_hash, o.metadata, COALESCE(b.body, '')
		FROM og_objects o LEFT JOIN og_bodies b ON b.obj_id = o.obj_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]projectedRow{}
	for rows.Next() {
		var id string
		var r projectedRow
		if err := rows.Scan(&id, &r.kind, &r.owner, &r.hash, &r.metadata, &r.body); err != nil {
			t.Fatal(err)
		}
		out[id] = r
	}
	return out
}

var inlineFixtureBodies = map[string]string{
	"obj-a": `{"text":"ok","n":[1,2,3],"u":"\u2028 x"}`,
	"obj-b": `{"b":1,"a":2,"dup":1,"dup":2,"big":12345678901234567890}`,
	"obj-c": `[1,2,3]`,
}

func TestInlineVersion3BatchProjectsExactBody(t *testing.T) { // S1, S2, S3
	var inlineOps, base64Ops, mixedOps []computerevent.ProjectionOp
	for _, id := range []string{"obj-a", "obj-b", "obj-c"} {
		body := []byte(inlineFixtureBodies[id])
		inlineOps = append(inlineOps, computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObject, CanonicalID: id, Body: inlineObjectJSON(t, id, body, true)})
		base64Ops = append(base64Ops, computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObject, CanonicalID: id, Body: inlineObjectJSON(t, id, body, false)})
	}
	mixedOps = []computerevent.ProjectionOp{inlineOps[0], base64Ops[1], inlineOps[2]}

	old := openProjectStore(t)
	finalizeInlineBatch(t, old, computerevent.ProjectionBatchV2, base64Ops)
	want := projectedRows(t, old)
	if len(want) != 3 {
		t.Fatalf("version 2 projected %d rows", len(want))
	}
	for id, row := range want {
		if row.body != inlineFixtureBodies[id] {
			t.Fatalf("version 2 %s body = %q", id, row.body)
		}
	}
	for name, ops := range map[string][]computerevent.ProjectionOp{"inline": inlineOps, "mixed": mixedOps} {
		s := openProjectStore(t)
		finalizeInlineBatch(t, s, computerevent.ProjectionBatchV3, ops)
		got := projectedRows(t, s)
		if len(got) != len(want) {
			t.Fatalf("%s: %d rows, want %d", name, len(got), len(want))
		}
		for id, w := range want {
			if got[id] != w {
				t.Errorf("%s %s: row differs from the base64 form:\n got %+v\nwant %+v", name, id, got[id], w)
			}
		}
	}
}

func TestDecodeChoirEventRowReadsInlineOp(t *testing.T) { // S4
	rec := &types.EventRecord{EventID: "ev-in-1", RunID: "run-in", OwnerID: "owner-1", Seq: 4, StreamSeq: 9,
		Kind: types.EventToolResult, Timestamp: inlineReaderAt, Payload: json.RawMessage(`{"output":"a <b> & c"}`)}
	obj, err := buildEventObject(rec, inlineReaderAt)
	if err != nil {
		t.Fatal(err)
	}
	b64, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	inline := spliceInline(t, obj, obj.Body)
	want, err := decodeChoirEventRow(b64, 5, 0)
	if err != nil {
		t.Fatalf("base64 op: %v", err)
	}
	got, err := decodeChoirEventRow(inline, 5, 0)
	if err != nil {
		t.Fatalf("inline op: %v", err)
	}
	if string(got.body) != string(want.body) || got.contentHash != want.contentHash || got.runID != want.runID ||
		got.seq != want.seq || got.streamSeq != want.streamSeq || got.eventKind != want.eventKind || got.metadata != want.metadata {
		t.Fatalf("inline row differs:\n got %+v\nwant %+v", got, want)
	}
	// A body that is not an event record is still refused.
	bad := spliceInline(t, obj, []byte(`"not an event record"`))
	if _, err := decodeChoirEventRow(bad, 5, 0); err == nil {
		t.Fatal("an inline body that is not an event record was accepted")
	}
}

// goldenV2 is a batch in the pre-change encoding, written out literally (not
// produced by the current encoder), so a change to the encoder cannot change
// what the golden proves.
const goldenV2 = `{"version":2,"projector_version":2,"computer_id":"computer-project","event_id":"ev","ops":[` +
	`{"kind":"object_recorded","canonical_id":"obj-g1","body":{"canonical_id":"obj-g1","object_kind":"choir.texture_revision","owner_id":"owner-1","computer_id":"computer-project","version_id":"v1","content_hash":"sha256:g1","body":"eyJhIjoxLCJiIjpbMSwyXX0=","metadata":{"k":1},"created_at":"2026-11-15T09:00:00Z","updated_at":"2026-11-15T09:00:00Z"}},` +
	`{"kind":"object_recorded","canonical_id":"obj-g2","body":{"canonical_id":"obj-g2","object_kind":"choir.texture_revision","owner_id":"owner-1","computer_id":"computer-project","version_id":"v1","content_hash":"sha256:g2","body":"cGxhaW4gdGV4dA==","metadata":{},"created_at":"2026-11-15T09:00:00Z","updated_at":"2026-11-15T09:00:00Z"}}]}`

// goldenV3 is the same objects with the first body inline; the second is
// plain text, which keeps its base64 body in the same batch.
const goldenV3 = `{"version":3,"projector_version":2,"computer_id":"computer-project","event_id":"ev","ops":[` +
	`{"kind":"object_recorded","canonical_id":"obj-g1","body":{"canonical_id":"obj-g1","object_kind":"choir.texture_revision","owner_id":"owner-1","computer_id":"computer-project","version_id":"v1","content_hash":"sha256:g1","body_json":{"a":1,"b":[1,2]},"metadata":{"k":1},"created_at":"2026-11-15T09:00:00Z","updated_at":"2026-11-15T09:00:00Z"}},` +
	`{"kind":"object_recorded","canonical_id":"obj-g2","body":{"canonical_id":"obj-g2","object_kind":"choir.texture_revision","owner_id":"owner-1","computer_id":"computer-project","version_id":"v1","content_hash":"sha256:g2","body":"cGxhaW4gdGV4dA==","metadata":{},"created_at":"2026-11-15T09:00:00Z","updated_at":"2026-11-15T09:00:00Z"}}]}`

func TestGoldenOldFormAndInlineFormDecodeAndProjectEqual(t *testing.T) { // S5
	old, err := computerevent.DecodeProjectionBatch([]byte(goldenV2))
	if err != nil {
		t.Fatal(err)
	}
	inline, err := computerevent.DecodeProjectionBatch([]byte(goldenV3))
	if err != nil {
		t.Fatal(err)
	}
	if len(old.Ops) != len(inline.Ops) {
		t.Fatal("op counts differ")
	}
	for i := range old.Ops {
		var a, b objectgraph.Object
		if err := json.Unmarshal(old.Ops[i].Body, &a); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(inline.Ops[i].Body, &b); err != nil {
			t.Fatal(err)
		}
		if string(a.Body) != string(b.Body) || a.ContentHash != b.ContentHash ||
			objectgraph.ContentHash(a.ObjectKind, a.Body, a.Metadata) != objectgraph.ContentHash(b.ObjectKind, b.Body, b.Metadata) {
			t.Fatalf("op %d: decoded objects differ: %q vs %q", i, a.Body, b.Body)
		}
	}
	if want, _ := base64.StdEncoding.DecodeString("eyJhIjoxLCJiIjpbMSwyXX0="); string(want) != `{"a":1,"b":[1,2]}` {
		t.Fatalf("golden is inconsistent: %q", want)
	}
	oldStore, inlineStore := openProjectStore(t), openProjectStore(t)
	finalizeInlineBatch(t, oldStore, computerevent.ProjectionBatchV2, old.Ops)
	finalizeInlineBatch(t, inlineStore, computerevent.ProjectionBatchV3, inline.Ops)
	a, b := projectedRows(t, oldStore), projectedRows(t, inlineStore)
	if len(a) != 2 || len(b) != 2 {
		t.Fatalf("rows %d and %d", len(a), len(b))
	}
	for id, row := range a {
		if b[id] != row {
			t.Errorf("%s: inline projection differs from the old form:\n got %+v\nwant %+v", id, b[id], row)
		}
	}
}

func TestReaderReleaseWritesNoFloorAndReceiptReadsEpoch7(t *testing.T) { // S6
	s := openProjectStore(t)
	workspace := s.texturePath
	if got := storeschema.Version; got != 7 {
		t.Fatalf("storeschema.Version = %d, want 7 (the reader epoch)", got)
	}
	receipt, found, err := storeschema.Read(filepath.Join(workspace, storeschema.File))
	if err != nil || !found || receipt.Version != 7 {
		t.Fatalf("receipt = %+v found=%v err=%v, want version 7", receipt, found, err)
	}
	floor, found, err := storeschema.ReadFloor(filepath.Join(workspace, storeschema.FloorFile))
	if err != nil {
		t.Fatal(err)
	}
	if found && floor.Version >= 7 {
		t.Fatalf("floor = %+v: the reader release writes no floor of 7 (that is release 2's)", floor)
	}
	if _, err := os.Stat(filepath.Join(workspace, storeschema.FloorFile)); err == nil && !found {
		t.Fatal("floor file exists but was not read")
	}
}

func TestDecodedInlineObjectReEncodesAsBase64(t *testing.T) { // S7
	var obj objectgraph.Object
	if err := json.Unmarshal(inlineObjectJSON(t, "obj-u", []byte(`{"a":1}`), true), &obj); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(out, &generic); err != nil {
		t.Fatal(err)
	}
	if _, ok := generic["body_json"]; ok {
		t.Fatalf("re-encode emitted body_json: %s", out)
	}
	if string(generic["body"]) != `"`+base64.StdEncoding.EncodeToString([]byte(`{"a":1}`))+`"` {
		t.Fatalf("re-encode body = %s", generic["body"])
	}
}
