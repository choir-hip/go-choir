package types

import (
	"fmt"
	"strings"
)

// AutoradioRecordKind is one row of the record table of the learning-engine
// design, section 12.2: a record_kind subject ref and the word its
// owner-private label is made from. Every Autoradio record is a commitment
// record with desk app:autoradio, that record_kind and the label
// OwnerPrivateLabel(Label).
type AutoradioRecordKind struct {
	Kind  string
	Label string
}

// AutoradioRecordKinds is the table of design 12.2, in its order. A kind
// outside it is refused at append (CheckAutoradioLabel), so no station can
// write an unlabelled or unlisted Autoradio record. The label is
// "autoradio_" plus the word after the dot of the kind.
var AutoradioRecordKinds = []AutoradioRecordKind{
	{"autoradio.turn", "autoradio_turn"},
	{"autoradio.request", "autoradio_request"},
	{"autoradio.forecast", "autoradio_forecast"},
	{"autoradio.settlement", "autoradio_settlement"},
	{"autoradio.stretch_start", "autoradio_stretch_start"},
	{"autoradio.segment", "autoradio_segment"},
	{"autoradio.stretch", "autoradio_stretch"},
	{"autoradio.air", "autoradio_air"},
	{"autoradio.conflict", "autoradio_conflict"},
	{"autoradio.lesson", "autoradio_lesson"},
	{"autoradio.strike", "autoradio_strike"},
	{"autoradio.clear", "autoradio_clear"},
	{"autoradio.note", "autoradio_note"},
	{"autoradio.sheet_op", "autoradio_sheet_op"},
	{"autoradio.sheet_snapshot", "autoradio_sheet_snapshot"},
	// no_verdict: an applied op reported the end of a desk entry whose forecast
	// it does not settle (the listener left, the entry failed to play, or the
	// op is later than the forecast's deadline). Written by the run sheet's
	// reducer path only; it carries the op and its position so the rule can be
	// revisited with data. Not in the design's 12.2 table (docs/reference/AUTORADIO-SHEET.md).
	{"autoradio.no_verdict", "autoradio_no_verdict"},
	// notebook_patch: the desk's ask to Texture to apply a patch to the
	// listener's notebook, and Texture's answer (docs/reference/AUTORADIO-NOTEBOOK.md).
	{"autoradio.notebook_patch", "autoradio_notebook_patch"},
}

// AutoradioKindPrefix begins every Autoradio record_kind.
const AutoradioKindPrefix = "autoradio."

// AutoradioKindLabel returns the owner-private label atom of an Autoradio
// record kind, and false for a kind outside the table.
func AutoradioKindLabel(kind string) (string, bool) {
	for _, k := range AutoradioRecordKinds {
		if k.Kind == kind {
			return OwnerPrivateLabel(k.Label), true
		}
	}
	return "", false
}

// CheckAutoradioLabel refuses a record of an Autoradio record_kind that is
// outside the table or does not carry its kind's owner-private label (other
// atoms may ride along: labels only ever join). A record of any other kind
// passes.
func CheckAutoradioLabel(rec CommitmentRecord) error {
	kind := strings.TrimSpace(rec.SubjectRefs["record_kind"])
	if !strings.HasPrefix(kind, AutoradioKindPrefix) {
		return nil
	}
	label, ok := AutoradioKindLabel(kind)
	if !ok {
		return fmt.Errorf("commitment record %s: %q is not an Autoradio record kind (design 12.2)", rec.RecordID, kind)
	}
	if !NewLabelSet(rec.Provenance.Labels...).Has(label) {
		return fmt.Errorf("commitment record %s of kind %s requires the label %s", rec.RecordID, kind, label)
	}
	return nil
}
