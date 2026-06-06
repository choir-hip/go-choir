package sourcecontract

import "strings"

const (
	EvidenceStateCandidate       = "candidate"
	EvidenceStateAvailable       = "available"
	EvidenceStateConfirms        = "confirms"
	EvidenceStateRefutes         = "refutes"
	EvidenceStateQualifies       = "qualifies"
	EvidenceStateNoSourceNeeded  = "no_source_needed"
	EvidenceStateStale           = "stale"
	EvidenceStateBlockedByAccess = "blocked_by_access"
	EvidenceStateUnavailable     = "unavailable"
)

func NormalizeEvidenceState(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	switch normalized {
	case EvidenceStateCandidate,
		EvidenceStateAvailable,
		EvidenceStateConfirms,
		EvidenceStateRefutes,
		EvidenceStateQualifies,
		EvidenceStateNoSourceNeeded,
		EvidenceStateStale,
		EvidenceStateBlockedByAccess,
		EvidenceStateUnavailable:
		return normalized
	case "pending", "needs_source", "source_needed":
		return EvidenceStateCandidate
	case "confirming", "confirmed", "represented", "owner_supplied":
		return EvidenceStateConfirms
	case "refuting", "refuted":
		return EvidenceStateRefutes
	case "qualifying", "qualified":
		return EvidenceStateQualifies
	case "blocked", "blocked_access", "access_blocked":
		return EvidenceStateBlockedByAccess
	case "not_needed", "no_source":
		return EvidenceStateNoSourceNeeded
	case "error", "failed", "fetch_failed":
		return EvidenceStateUnavailable
	default:
		return ""
	}
}

func IsRelationalEvidenceState(value string) bool {
	switch NormalizeEvidenceState(value) {
	case EvidenceStateConfirms, EvidenceStateRefutes, EvidenceStateQualifies:
		return true
	default:
		return false
	}
}
