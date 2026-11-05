package agentcore

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The capture read gate (MA3.3). choir.ReadCapture fetches any capture by id
// and sits in every app desk's verb set, so it is the widest by-id read a desk
// inherits. Its frame handler decides from the capture record, never from the
// cell:
//
//   - the labels come from the record corpusd returns (platform.CaptureReadResponse
//     .Labels, stamped by the host when the capture was written), and join the
//     run's labels with the floor every capture read carries (run_labels.go);
//   - a capture whose record carries no owner-private label is shared world
//     content (E4) and is served under captures.read to every desk that holds
//     the verb;
//   - a capture whose record carries an owner-private label is served only to
//     the custodian of that label's kind, to the owner and to the kernel
//     (against a lease once MA3.4 lands).
//
// Built-in desks only observe in this slice: a capture the reader may not read
// is counted (choir.web_capture|reader|wrong_reader on the reader gate's
// counters) and served. MA3.7 flips it, in code.

// captureRecordLabels is the label set of the capture a ReadCapture result
// describes, as the capture record carries it.
func captureRecordLabels(result json.RawMessage) types.LabelSet {
	var rec struct {
		Labels []string `json:"labels"`
	}
	if len(result) == 0 || json.Unmarshal(result, &rec) != nil {
		return nil
	}
	return types.NewLabelSet(rec.Labels...)
}

// captureServedTo reports whether reader may read a capture whose record
// carries labels: a world capture, or every owner-private label naming a kind
// reader is the custodian of.
func captureServedTo(reader objectgraph.Writer, ok bool, labels types.LabelSet) bool {
	if !labels.OwnerPrivate() {
		return true
	}
	if ok && (reader == objectgraph.WriterOwner || reader == objectgraph.WriterKernel) {
		return true
	}
	for _, l := range labels {
		if !strings.HasPrefix(l, types.LabelOwnerPrivatePrefix) {
			continue
		}
		custodian, found := objectgraph.CustodianOf(objectgraph.ObjectKind("choir." + strings.TrimPrefix(l, types.LabelOwnerPrivatePrefix)))
		if !found || !ok || custodian != reader {
			return false
		}
	}
	return true
}

// gateCaptureRead observes one ReadCapture result against the desk that
// asked. The desk is the session's role, bound by the frame layer.
func gateCaptureRead(ctx context.Context, deskRole string, result json.RawMessage) {
	reader, ok := objectgraph.DeskWriter(canonicalDeskName(strings.TrimSpace(deskRole)))
	if !ok {
		reader, ok = objectgraph.ReaderFrom(ctx)
	}
	if captureServedTo(reader, ok, captureRecordLabels(result)) {
		return
	}
	objectgraph.NoteWrongReader(objectgraph.WebCaptureObjectKind, reader, ok)
}

// noteRunMemoryScope observes the one scope check the kind gate cannot see
// (X18, section 8.4). Run-memory entries are a commons kind kept in their own
// table, so a read by id passes the reader gate, but a desk's tool reads any
// run's entry by its id. The ruling self-scopes it to the caller's own run
// (MA3.7); until then the read is served and an entry of another run is
// counted as the desk reading it.
func noteRunMemoryScope(ctx context.Context, callerRunID string, entry types.RunMemoryEntry) {
	if callerRunID == "" || entry.RunID == callerRunID {
		return
	}
	reader, ok := objectgraph.ReaderFrom(ctx)
	objectgraph.NoteWrongReader(runMemoryEntryKind, reader, ok)
}

// runMemoryEntryKind is the kind of a run-memory entry (a commons kind).
const runMemoryEntryKind = objectgraph.ObjectKind("choir.run_memory_entry")
