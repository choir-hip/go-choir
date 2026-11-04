// Package release holds what remains of the software release (P7) after
// its publish and adoption were deleted (docs/design/package-install-2026-11-04.md):
// the precommitment a package release carries, and the source-tree git
// helpers a package install uses.
package release

// Precommitment is one of engineering's rule-settled say-do claims for the
// assignment that froze the bundle (types/commitment_rules.go). PYesPermille
// is p(yes) in thousandths: canonical JSON forbids non-integral numbers.
type Precommitment struct {
	RecordID        string `json:"record_id"`
	Kind            string `json:"kind"`
	Question        string `json:"question"`
	PYesPermille    int    `json:"p_yes_permille"`
	Settled         string `json:"settled,omitempty"`
	ResolveRecordID string `json:"resolve_record_id,omitempty"`
}
