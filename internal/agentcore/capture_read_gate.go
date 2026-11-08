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
// A built-in desk reads an owner-private capture by rule, not by observation
// (enforce flip section 7.2, owner ruling 2026-11-06: "it's expected that
// private captures will be used by texture, no approval required ... just keep
// the provenance"): the desks with capture authority (captureAuthority) read
// it without a count. Any other built-in desk is still served in S1 and S2 but
// counted (choir.web_capture|reader|wrong_reader), and refused at S3. An
// installed package desk is enforced now (MA3.4): it is served an
// owner-private capture only against a lease that names it.

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
		// The shared label-to-kind mapping: owner_private:capture names
		// choir.web_capture, and owner_private:evidence names
		// choir.agent_evidence, never "choir.<atom>".
		served := false
		for _, k := range objectgraph.LabelKinds(strings.TrimPrefix(l, types.LabelOwnerPrivatePrefix)) {
			if custodian, found := objectgraph.CustodianOf(k); found && ok && custodian == reader {
				served = true
			}
		}
		if !served {
			return false
		}
	}
	return true
}

// captureAuthority reports whether a built-in desk holds the authority to read
// an owner-private capture (enforce flip section 7.2): the custodian of the
// owner's documents (Texture), the research parent and its child roles (the
// exchange the owner's ruling covers), and any system desk or role that holds
// captures.read or a lease ceiling for captures (a management task is one).
// Parentage is read from the manifest, never from a name prefix.
func captureAuthority(deskRole string) bool {
	d, ok := deskmanifest.Lookup(canonicalDeskName(strings.TrimSpace(deskRole)))
	if !ok || d.Kind == deskmanifest.KindApp {
		return false
	}
	if w, found := objectgraph.CustodianOf("choir.texture_document"); found && w == objectgraph.Writer("desk:"+d.Name) {
		return true
	}
	if d.Name == deskmanifest.ResearcherName() {
		return true
	}
	if p, found := deskmanifest.ParentOf(d.Name); found && p.Name == deskmanifest.ResearcherName() {
		return true
	}
	return d.HasGrant("captures.read") || d.LeaseCaptures
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
// (enforce, as its kinds are); a built-in desk without capture authority is
// refused too (F6, FM9).
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
	if captureAuthority(deskRole) {
		return nil
	}
	if objectgraph.NoteWrongReader(objectgraph.WebCaptureObjectKind, reader, ok) {
		return fmt.Errorf("read_capture: capture %q is owner-private and desk %q holds no capture authority or lease for it; ask its custodian for a lease", captureRecordID(result), deskRole)
	}
	return nil
}

// runMemoryEntryInScope reports whether a run may read entry: its own run's
// entries only (X18, MA3.7b). A call that names no run (a host caller) is
// not a cell and reads as before.
func runMemoryEntryInScope(callerRunID string, entry types.RunMemoryEntry) bool {
	return callerRunID == "" || entry.RunID == callerRunID
}
