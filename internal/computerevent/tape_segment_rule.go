package computerevent

// Pin namespaces: the legacy directories under sha256/ that a pin receipt
// names in its pin_namespace field.
const (
	PinNamespaceEvent   = "computer-event"
	PinNamespacePayload = "computer-event-payload"
)

// TapeSegmentEligible is the one rule for which pins belong in the pinning
// computer's tape segment rather than a loose file under sha256/. The pin
// path (internal/platform event_artifacts.go) writes by it and the repack
// tool (internal/taperepack) copies and deletes by it, so the two cannot
// drift.
//
//   - An event: always.
//   - A payload: when its class is not public, or it is a projection batch
//     (ProjectionBatchMediaType). A public non-batch payload stays loose:
//     artifact+sha256 URIs name it by digest alone (the capsule effect bundle
//     a self-development route applies), and their reader, vmctl's
//     LocalArtifactContentVerifier, opens the loose file.
//   - A private payload must be a private envelope; the caller also checks
//     that the envelope names this computer (the pin refuses one that does
//     not; the repack tool leaves it loose).
//   - A pin receipt follows its artifact.
//
// The rule is about the kind only. A computer id that cannot name a segment
// directory (tapestore.ValidComputerID) and a record over
// tapestore.MaxRecordBytes stay loose too; both callers check those.
//
// Scope: projection batches alone until 2026-11-01, every kind but public
// non-batch payloads since (docs/problems/offhost-backup-shard-thrashes-at-4g-2026-11-01.md).
func TapeSegmentEligible(pinNamespace, mediaType, privacyClass string) bool {
	switch pinNamespace {
	case PinNamespaceEvent:
		return true
	case PinNamespacePayload:
		switch privacyClass {
		case "":
			return false
		case "private":
			return mediaType == PrivateArtifactMediaType
		case "public":
			return mediaType == ProjectionBatchMediaType
		default:
			return mediaType != ""
		}
	}
	return false
}
