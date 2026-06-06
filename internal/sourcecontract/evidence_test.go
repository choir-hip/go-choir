package sourcecontract

import "testing"

func TestNormalizeEvidenceState(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want string
	}{
		{raw: "candidate", want: EvidenceStateCandidate},
		{raw: "pending", want: EvidenceStateCandidate},
		{raw: "needs-source", want: EvidenceStateCandidate},
		{raw: "confirmed", want: EvidenceStateConfirms},
		{raw: "represented", want: EvidenceStateConfirms},
		{raw: "refuting", want: EvidenceStateRefutes},
		{raw: "qualifying", want: EvidenceStateQualifies},
		{raw: "blocked", want: EvidenceStateBlockedByAccess},
		{raw: "access blocked", want: EvidenceStateBlockedByAccess},
		{raw: "no-source-needed", want: EvidenceStateNoSourceNeeded},
		{raw: "error", want: EvidenceStateUnavailable},
		{raw: "fetch_failed", want: EvidenceStateUnavailable},
		{raw: "unknown", want: ""},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			if got := NormalizeEvidenceState(tc.raw); got != tc.want {
				t.Fatalf("NormalizeEvidenceState(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestIsRelationalEvidenceState(t *testing.T) {
	for _, value := range []string{"confirms", "confirmed", "refutes", "refuting", "qualifies", "qualifying"} {
		if !IsRelationalEvidenceState(value) {
			t.Fatalf("%q should be relational", value)
		}
	}
	for _, value := range []string{"candidate", "available", "blocked_by_access", "no_source_needed", "unavailable"} {
		if IsRelationalEvidenceState(value) {
			t.Fatalf("%q should not be relational", value)
		}
	}
}
