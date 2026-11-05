package types

import "github.com/yusefmosiah/go-choir/internal/deskmanifest"

// The materiality catalogue (plan section 10.1, MA1.3): for each record kind,
// the desk that is its custodian and the realized-materiality procedure the
// custodian froze for it. The committing desk never picks the procedure: the
// reducer reads it from here and hashes its ref with the frozen weights.
//
// This is the minimal catalogue for the request kinds in use. A kind whose
// ProcedureRef is empty has no procedure: its records get no importance score
// (they are not scored zero). The custodian of each kind is named here until
// per-kind custody moves to the packages that own the kinds (MA8.2); each
// request kind is today admitted by management's intake and admission grants.

// MaterialityEntry is one kind's catalogue row.
type MaterialityEntry struct {
	Kind string
	// Custodian is the desk that owns the kind and its procedure.
	Custodian string
	// ProcedureRef names the frozen procedure ("" for none).
	ProcedureRef string
	// Window says when the procedure reads realized materiality.
	Window string
	// ImpactFloor is the lowest impact class a record of the kind can have.
	ImpactFloor ImpactClass
}

// The custodian of deskmanifest.AdmissionKind admits every request kind
// today; its owner in the deskmanifest registry is the custodian of the
// catalogue's kinds (no desk name is written here, R3).
func admissionCustodian() string {
	for _, name := range deskmanifest.Names() {
		d, ok := deskmanifest.Lookup(name)
		if !ok {
			continue
		}
		for _, k := range d.OwnedKinds {
			if k == deskmanifest.AdmissionKind {
				return d.Name
			}
		}
	}
	return ""
}

// MaterialityCatalogue lists the entries, in a stable order.
var MaterialityCatalogue = withCustodian([]MaterialityEntry{
	// A Cast opens work. For an engineering Cast the procedure reads the
	// floor measures of section 10.3: rollback or hotfix within 14 days (F1)
	// and escaped defects over 30 days (F2); for other casts, whether the
	// round it opened ended with its stated outcome.
	{Kind: RequestCast, ProcedureRef: "request.cast.v1", Window: "14d (F1), 30d (F2)", ImpactFloor: ImpactI1},
	// An Escalate surfaces an issue to management or the owner: the
	// procedure reads whether the owner or management acted on it.
	{Kind: RequestEscalate, ProcedureRef: "request.escalate.v1", Window: "7d", ImpactFloor: ImpactI1},
	// No realized-materiality procedure is defined for these: no importance
	// score, which is not a score of zero.
	{Kind: RequestAsk, ImpactFloor: ImpactI0},
	{Kind: RequestFeed, ImpactFloor: ImpactI0},
	{Kind: RequestDecline, ImpactFloor: ImpactI0},
	{Kind: RequestPrecommit, ImpactFloor: ImpactI0},
})

func withCustodian(entries []MaterialityEntry) []MaterialityEntry {
	c := admissionCustodian()
	for i := range entries {
		entries[i].Custodian = c
	}
	return entries
}

// MaterialityEntryFor returns the catalogue row of a kind.
func MaterialityEntryFor(kind string) (MaterialityEntry, bool) {
	for _, e := range MaterialityCatalogue {
		if e.Kind == kind {
			return e, true
		}
	}
	return MaterialityEntry{}, false
}
