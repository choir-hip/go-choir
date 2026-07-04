package computerversion

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPromotionCertificateObservationSetEmitsCandidateScopedCanonicalPayload(t *testing.T) {
	certificate := promotionCertificateFixture()

	observations, err := certificate.ObservationSet("promotion evidence")
	if err != nil {
		t.Fatalf("promotion certificate observation set: %v", err)
	}
	if observations.Version != certificate.Candidate {
		t.Fatalf("version = %#v, want candidate %#v", observations.Version, certificate.Candidate)
	}
	if len(observations.Required) != 1 || observations.Required[0] != ObservationPromotionCertificate {
		t.Fatalf("required observations = %#v", observations.Required)
	}
	if len(observations.Observations) != 1 {
		t.Fatalf("expected one promotion observation, got %#v", observations.Observations)
	}
	observation := observations.Observations[0]
	if observation.Kind != ObservationPromotionCertificate {
		t.Fatalf("observation kind = %q, want %q", observation.Kind, ObservationPromotionCertificate)
	}
	if observation.Key != "promotion:promote-2026-07-04" {
		t.Fatalf("observation key = %q", observation.Key)
	}

	expectedPayload := `{"id":"promote-2026-07-04","route_slot":"prod",` +
		`"active":{"code_ref":"git:active","artifact_program_ref":"tape:org/prod-active"},` +
		`"candidate":{"code_ref":"git:candidate","artifact_program_ref":"tape:org/prod-candidate"},` +
		`"base":{"code_ref":"git:base","artifact_program_ref":"tape:org/prod-base"},` +
		`"owner_approved":true,"health_window":"confirmed",` +
		`"ledgers":[{"name":"audit-ledger","state":"verified"},{"name":"route-ledger","state":"prepared"}],` +
		`"rollback_ref":"rollback:promote-2026-07-04","evidence_ref":"evidence:promote-2026-07-04"}`
	if observation.Value != expectedPayload {
		t.Fatalf("canonical payload = %s, want %s", observation.Value, expectedPayload)
	}

	var payload PromotionCertificate
	if err := json.Unmarshal([]byte(observation.Value), &payload); err != nil {
		t.Fatalf("decode canonical payload: %v", err)
	}
	if !payload.Active.Valid() || !payload.Candidate.Valid() || !payload.Base.Valid() {
		t.Fatalf("payload does not carry concrete computer version refs: %#v", payload)
	}
	if payload.Active != certificate.Active || payload.Candidate != certificate.Candidate || payload.Base != certificate.Base {
		t.Fatalf("payload refs = active %#v candidate %#v base %#v", payload.Active, payload.Candidate, payload.Base)
	}
	if len(payload.Ledgers) != 2 || payload.Ledgers[0].Name != "audit-ledger" || payload.Ledgers[1].Name != "route-ledger" {
		t.Fatalf("payload ledgers are not sorted by name: %#v", payload.Ledgers)
	}
}

func TestPromotionCertificateEquivalentWhenLedgersSuppliedInDifferentOrder(t *testing.T) {
	leftCertificate := promotionCertificateFixture()
	rightCertificate := promotionCertificateFixture()
	rightCertificate.Ledgers = []PromotionLedgerCertificate{
		{Name: "audit-ledger", State: PromotionLedgerVerified},
		{Name: "route-ledger", State: PromotionLedgerPrepared},
	}

	left, err := leftCertificate.ObservationSet("left")
	if err != nil {
		t.Fatalf("left promotion certificate observations: %v", err)
	}
	right, err := rightCertificate.ObservationSet("right")
	if err != nil {
		t.Fatalf("right promotion certificate observations: %v", err)
	}

	result := EquivalenceChecker{}.CheckObservationSets(left, right)
	if !result.Equivalent() {
		t.Fatalf("expected ledger order normalization to compare equivalent, got %#v", result)
	}
}

func TestPromotionCertificateSeededMismatchFailsEquivalence(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mutate     func(*PromotionCertificate)
		assertDiff func(*testing.T, Difference)
	}{
		{
			name: "candidate ref",
			mutate: func(c *PromotionCertificate) {
				c.Candidate.CodeRef = "git:different-candidate"
			},
			assertDiff: func(t *testing.T, diff Difference) {
				t.Helper()
				if diff.Reason != "observation sets name different computer versions" || diff.Left == "" || diff.Right == "" {
					t.Fatalf("unexpected candidate ref difference: %#v", diff)
				}
			},
		},
		{
			name: "ledger state",
			mutate: func(c *PromotionCertificate) {
				c.Ledgers[0].State = PromotionLedgerApplied
			},
			assertDiff: func(t *testing.T, diff Difference) {
				t.Helper()
				if diff.Kind != ObservationPromotionCertificate || diff.Key != "promotion:promote-2026-07-04" {
					t.Fatalf("unexpected ledger state difference target: %#v", diff)
				}
				if diff.Reason != "observation values differ" || diff.Left == "" || diff.Right == "" || diff.Left == diff.Right {
					t.Fatalf("unexpected ledger state difference values: %#v", diff)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			leftCertificate := promotionCertificateFixture()
			rightCertificate := promotionCertificateFixture()
			tc.mutate(&rightCertificate)

			left, err := leftCertificate.ObservationSet("left")
			if err != nil {
				t.Fatalf("left promotion certificate observations: %v", err)
			}
			right, err := rightCertificate.ObservationSet("right")
			if err != nil {
				t.Fatalf("right promotion certificate observations: %v", err)
			}

			result := EquivalenceChecker{}.CheckObservationSets(left, right)
			if result.Status != EquivalenceNotEquivalent {
				t.Fatalf("expected not_equivalent status, got %#v", result)
			}
			if len(result.Differences) != 1 {
				t.Fatalf("expected one difference, got %#v", result.Differences)
			}
			tc.assertDiff(t, result.Differences[0])
		})
	}
}

func TestPromotionCertificateObservationSetRejectsInvalidCertificates(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*PromotionCertificate)
		wantErr string
	}{
		{
			name: "missing approval",
			mutate: func(c *PromotionCertificate) {
				c.OwnerApproved = false
			},
			wantErr: "owner approval is required",
		},
		{
			name: "duplicate ledgers",
			mutate: func(c *PromotionCertificate) {
				c.Ledgers = append(c.Ledgers, PromotionLedgerCertificate{Name: " audit-ledger ", State: PromotionLedgerApplied})
			},
			wantErr: "duplicate ledger \"audit-ledger\"",
		},
		{
			name: "active equals candidate",
			mutate: func(c *PromotionCertificate) {
				c.Candidate = c.Active
			},
			wantErr: "active and candidate ComputerVersion must differ",
		},
		{
			name: "unsupported health window",
			mutate: func(c *PromotionCertificate) {
				c.HealthWindow = "warming"
			},
			wantErr: "unsupported health window \"warming\"",
		},
		{
			name: "unsupported ledger state",
			mutate: func(c *PromotionCertificate) {
				c.Ledgers[0].State = "waiting"
			},
			wantErr: "ledger \"route-ledger\" has unsupported state \"waiting\"",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			certificate := promotionCertificateFixture()
			tc.mutate(&certificate)

			observations, err := certificate.ObservationSet("invalid")
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
			if observations.Version.Valid() || len(observations.Required) != 0 || len(observations.Observations) != 0 {
				t.Fatalf("invalid certificate emitted observations before validation failed: %#v", observations)
			}
		})
	}
}

func promotionCertificateFixture() PromotionCertificate {
	return PromotionCertificate{
		ID:        "promote-2026-07-04",
		RouteSlot: "prod",
		Active: ComputerVersion{
			CodeRef:            "git:active",
			ArtifactProgramRef: "tape:org/prod-active",
		},
		Candidate: ComputerVersion{
			CodeRef:            "git:candidate",
			ArtifactProgramRef: "tape:org/prod-candidate",
		},
		Base: ComputerVersion{
			CodeRef:            "git:base",
			ArtifactProgramRef: "tape:org/prod-base",
		},
		OwnerApproved: true,
		HealthWindow:  PromotionHealthConfirmed,
		Ledgers: []PromotionLedgerCertificate{
			{Name: "route-ledger", State: PromotionLedgerPrepared},
			{Name: "audit-ledger", State: PromotionLedgerVerified},
		},
		RollbackRef: "rollback:promote-2026-07-04",
		EvidenceRef: "evidence:promote-2026-07-04",
	}
}
