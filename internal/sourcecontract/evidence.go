package sourcecontract

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
	return canonicalFromSchema(embeddedSourceContractSchema.EvidenceStates, value)
}

func IsRelationalEvidenceState(value string) bool {
	state := NormalizeEvidenceState(value)
	return state != "" && embeddedSourceContractSchema.EvidenceStates[state].Relational
}
