package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
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
// counters) and served. MA3.7 flips it, in code. An installed package desk is
// enforced now (MA3.4): it is served an owner-private capture only against a
// lease that names it.

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

// captureRecordID is the id the capture record carries.
func captureRecordID(result json.RawMessage) string {
	var rec struct {
		CaptureID string `json:"capture_id"`
	}
	if len(result) == 0 || json.Unmarshal(result, &rec) != nil {
		return ""
	}
	return rec.CaptureID
}

// gateCaptureRead decides one ReadCapture result against the desk that asked.
// The desk is the session's role, bound by the frame layer.
//
// A capture the reader is not the custodian of is still served against a live
// lease that names the capture by id or by one of its labels (MA3.4); the
// lease and the run it is for come from the run's context, never the cell. A
// reader that is an installed package desk and holds no such lease is refused
// (enforce, as its kinds are); a built-in desk only observes until MA3.7.
func gateCaptureRead(ctx context.Context, deskRole string, result json.RawMessage) error {
	reader, ok := objectgraph.DeskWriter(canonicalDeskName(strings.TrimSpace(deskRole)))
	if !ok {
		reader, ok = objectgraph.ReaderFrom(ctx)
	}
	labels := captureRecordLabels(result)
	if captureServedTo(reader, ok, labels) {
		return nil
	}
	if ok && objectgraph.LeaseAdmitsCapture(ctx, reader, captureRecordID(result), labels.Strings()) {
		objectgraph.NoteLeaseRead(reader, "capture "+captureRecordID(result))
		return nil
	}
	if d, found := deskmanifest.Lookup(canonicalDeskName(strings.TrimSpace(deskRole))); found && d.Kind == deskmanifest.KindApp {
		objectgraph.NoteWrongReader(objectgraph.WebCaptureObjectKind, reader, ok)
		return fmt.Errorf("read_capture: capture %q is owner-private and desk %q holds no lease for it; ask its custodian for a lease", captureRecordID(result), deskRole)
	}
	objectgraph.NoteWrongReader(objectgraph.WebCaptureObjectKind, reader, ok)
	return nil
}

// runMemoryEntryInScope reports whether a run may read entry: its own run's
// entries only (X18, MA3.7b). A call that names no run (a host caller) is
// not a cell and reads as before.
func runMemoryEntryInScope(callerRunID string, entry types.RunMemoryEntry) bool {
	return callerRunID == "" || entry.RunID == callerRunID
}

// runMemoryEntryKind is the kind of a run-memory entry (a commons kind).
const runMemoryEntryKind = objectgraph.ObjectKind("choir.run_memory_entry")
