package ledgerwitness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// L3 witness (a) (docs/definitions/choir-ledger-l3-hypotheses-settled-
// 2026-10-13.md): the paper-chain position of an inbox record, read from
// the host's event artifacts without SQL and without the guest. Ways it
// could fail, written before the code:
//   - W1 a record projected on the chain is not found, or is reported with
//     the wrong sequence or event digest;
//   - W2 a record outside the walk is guessed or omitted silently: it must
//     be listed missing, so the caller fails closed;
//   - W3 a chain whose file does not hash to its name, or an event of
//     another computer, is walked through instead of stopping;
//   - W4 a projection event whose payload is missing is skipped silently;
//   - W5 the walk ignores its cap and reads the whole 700k-event chain;
//   - W6 a record id matched inside another object (not a commitment
//     record), or as a substring, counts as the record.

const testComputer = "computer-walk"

// fill is a well-formed digest for event fields the walk never reads.
var fill = strings.Repeat("ab", 32)

type chain struct {
	root string
	head string
	seq  uint64
}

func newChain(t *testing.T) *chain {
	t.Helper()
	root := t.TempDir()
	for _, ns := range []string{"computer-event", "computer-event-payload"} {
		if err := os.MkdirAll(filepath.Join(root, "sha256", ns), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return &chain{root: root, head: computerevent.ZeroHead}
}

func (c *chain) writeFile(t *testing.T, ns string, raw []byte) string {
	t.Helper()
	digest := computerevent.DigestBytes(raw)
	if err := os.WriteFile(filepath.Join(c.root, "sha256", ns, digest), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return digest
}

// append adds one event; objects make it a projection batch.
func (c *chain) append(t *testing.T, computerID string, objects []objectgraph.Object, writePayload bool) string {
	t.Helper()
	c.seq++
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID,
		ComputerID: computerID, Sequence: c.seq, PreviousHead: c.head, EventKind: computerevent.EventLifecycleObserved,
		OccurredAt: "2026-10-15T22:00:00Z", IdempotencyKey: "walk-" + eventID, RequestCommitment: fill,
		ActorProfile: "trusted-core", AuthorityRef: "authority:test", PrivacyClass: "owner", ReducerVersion: computerevent.ReducerVersionV1,
		PayloadCommitment: fill, ExpectedDesiredEventHead: fill, ExpectedEffectiveEventHead: fill,
		ExpectedDesiredStateCommitment: fill, ExpectedEffectiveStateCommitment: fill}
	if objects != nil {
		var ops []computerevent.ProjectionOp
		for _, obj := range objects {
			body, _ := json.Marshal(obj)
			ops = append(ops, computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObject, CanonicalID: obj.CanonicalID, Body: body})
		}
		payload, _ := json.Marshal(computerevent.ProjectionBatch{Version: computerevent.ProjectionBatchV2, ComputerID: computerID, EventID: event.EventID, Ops: ops})
		event.EventKind = computerevent.EventProjectionBatchRecorded
		event.PayloadCommitment = computerevent.DigestBytes(payload)
		if writePayload {
			c.writeFile(t, "computer-event-payload", payload)
		}
	}
	raw, err := event.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	c.head = c.writeFile(t, "computer-event", raw)
	return c.head
}

func record(id string) objectgraph.Object {
	meta, _ := json.Marshal(map[string]any{"record_id": id})
	return objectgraph.Object{CanonicalID: "obj:choir.commitment_record:" + id, ObjectKind: objectgraph.ObjectKind(CommitmentRecordKind), Metadata: meta}
}

func TestWalkFindsRecordsAndOrdersThem(t *testing.T) {
	c := newChain(t)
	c.append(t, testComputer, nil, false)
	castEvent := c.append(t, testComputer, []objectgraph.Object{record("cast-1")}, true)
	c.append(t, testComputer, nil, false)
	arrivalEvent := c.append(t, testComputer, []objectgraph.Object{record("capture-arrival:rss:x:abc")}, true)
	c.append(t, testComputer, []objectgraph.Object{{CanonicalID: "obj:other", ObjectKind: "choir.web_capture", Metadata: json.RawMessage(`{"record_id":"cast-1"}`)}}, true)
	res, err := Walk(context.Background(), Config{ArtifactsRoot: c.root, ComputerID: testComputer, Head: c.head, MaxEvents: 100},
		[]string{"cast-1", "capture-arrival:rss:x:abc", "never-written"})
	if err != nil {
		t.Fatal(err)
	}
	cast, arrival := res.Found["cast-1"], res.Found["capture-arrival:rss:x:abc"]
	// W1
	if cast.Sequence != 2 || cast.EventDigest != castEvent || arrival.Sequence != 4 || arrival.EventDigest != arrivalEvent {
		t.Fatalf("found = %+v", res.Found)
	}
	if !(cast.Sequence < arrival.Sequence) {
		t.Fatal("the Cast must order before the arrival")
	}
	// W2
	if len(res.Missing) != 1 || res.Missing[0] != "never-written" {
		t.Fatalf("missing = %v", res.Missing)
	}
	if res.Walked != 5 || res.HeadSequence != 5 {
		t.Fatalf("walked %d head %d", res.Walked, res.HeadSequence)
	}
}

func TestWalkStopsOnBrokenOrForeignChain(t *testing.T) {
	c := newChain(t)
	c.append(t, testComputer, nil, false)
	c.append(t, testComputer, nil, false)
	// W3: corrupt the head file so it no longer hashes to its name.
	if err := os.WriteFile(filepath.Join(c.root, "sha256", "computer-event", c.head), []byte(`{"tampered":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Walk(context.Background(), Config{ArtifactsRoot: c.root, ComputerID: testComputer, Head: c.head, MaxEvents: 10}, []string{"x"}); err == nil {
		t.Fatal("a tampered event must stop the walk")
	}
	f := newChain(t)
	f.append(t, "computer-other", nil, false)
	if _, err := Walk(context.Background(), Config{ArtifactsRoot: f.root, ComputerID: testComputer, Head: f.head, MaxEvents: 10}, []string{"x"}); err == nil || !strings.Contains(err.Error(), "computer") {
		t.Fatalf("a foreign event must stop the walk: %v", err)
	}
}

func TestWalkRefusesMissingPayload(t *testing.T) {
	c := newChain(t)
	c.append(t, testComputer, []objectgraph.Object{record("cast-1")}, false) // W4
	if _, err := Walk(context.Background(), Config{ArtifactsRoot: c.root, ComputerID: testComputer, Head: c.head, MaxEvents: 10}, []string{"cast-1"}); err == nil || !strings.Contains(err.Error(), "payload") {
		t.Fatalf("a projection event without its payload must stop the walk: %v", err)
	}
}

func TestWalkHonoursItsCap(t *testing.T) {
	c := newChain(t)
	c.append(t, testComputer, []objectgraph.Object{record("old")}, true)
	for i := 0; i < 5; i++ {
		c.append(t, testComputer, nil, false)
	}
	res, err := Walk(context.Background(), Config{ArtifactsRoot: c.root, ComputerID: testComputer, Head: c.head, MaxEvents: 3}, []string{"old"})
	if err != nil {
		t.Fatal(err)
	}
	// W5: three events read, the record beyond the cap reported missing.
	if res.Walked != 3 || len(res.Missing) != 1 || !res.Capped {
		t.Fatalf("capped walk = %+v", res)
	}
}

func TestWalkMatchesWholeRecordIDsOnly(t *testing.T) {
	c := newChain(t)
	c.append(t, testComputer, []objectgraph.Object{record("cast-10")}, true)
	res, err := Walk(context.Background(), Config{ArtifactsRoot: c.root, ComputerID: testComputer, Head: c.head, MaxEvents: 10}, []string{"cast-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Found) != 0 { // W6
		t.Fatalf("cast-1 matched cast-10: %+v", res.Found)
	}
}
