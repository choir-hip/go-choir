package runtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestVerifyPromotionEvidenceRejectsEmptyResults is the verifier-evidence
// gate: a promotion with no verifier results must be rejected. No evidence =
// no promotion.
func TestVerifyPromotionEvidenceRejectsEmptyResults(t *testing.T) {
	rec := types.AppAdoptionRecord{VerifierResultsJSON: json.RawMessage(`[]`)}
	if err := verifyPromotionEvidence(rec); err == nil {
		t.Fatal("empty verifier results must be rejected")
	}
}

// TestVerifyPromotionEvidenceRejectsNilResults ensures a nil/invalid JSON
// payload is rejected, not silently accepted.
func TestVerifyPromotionEvidenceRejectsNilResults(t *testing.T) {
	rec := types.AppAdoptionRecord{VerifierResultsJSON: nil}
	if err := verifyPromotionEvidence(rec); err == nil {
		t.Fatal("nil verifier results must be rejected")
	}
	rec.VerifierResultsJSON = json.RawMessage(`{not-json}`)
	if err := verifyPromotionEvidence(rec); err == nil {
		t.Fatal("malformed verifier results must be rejected")
	}
}

// TestVerifyPromotionEvidenceRejectsAllFailed ensures that verifier results
// with no passed contract are rejected. A build failure leaves evidence, but
// it is failure evidence — not authorization.
func TestVerifyPromotionEvidenceRejectsAllFailed(t *testing.T) {
	rec := types.AppAdoptionRecord{VerifierResultsJSON: json.RawMessage(`[
		{"contract_id":"build","status":"failed","summary":"build failed"}
	]`)}
	if err := verifyPromotionEvidence(rec); err == nil {
		t.Fatal("all-failed verifier results must be rejected")
	}
}

// TestVerifyPromotionEvidenceAcceptsPassedResults ensures that verifier
// results with at least one passed contract are accepted.
func TestVerifyPromotionEvidenceAcceptsPassedResults(t *testing.T) {
	rec := types.AppAdoptionRecord{VerifierResultsJSON: json.RawMessage(`[
		{"contract_id":"source-refs","status":"passed","summary":"refs resolve"},
		{"contract_id":"build","status":"passed","summary":"build ok"}
	]`)}
	if err := verifyPromotionEvidence(rec); err != nil {
		t.Fatalf("passed verifier results must be accepted: %v", err)
	}
}

// TestVerifyPromotionEvidenceAcceptsMixedResults ensures that a mix of
// passed and failed contracts is accepted as long as at least one passed.
func TestVerifyPromotionEvidenceAcceptsMixedResults(t *testing.T) {
	rec := types.AppAdoptionRecord{VerifierResultsJSON: json.RawMessage(`[
		{"contract_id":"source-refs","status":"passed","summary":"refs resolve"},
		{"contract_id":"optional-lint","status":"failed","summary":"lint warnings"}
	]`)}
	if err := verifyPromotionEvidence(rec); err != nil {
		t.Fatalf("mixed verifier results with a pass must be accepted: %v", err)
	}
}

// TestPromoteFreshnessCASAlreadyFresh is a regression guard for the existing
// freshness check — it should pass when the lineage hasn't moved.
func TestPromoteFreshnessCASAlreadyFresh(t *testing.T) {
	base := "refs/computers/c1/active@v1"
	rec := adoptionWithRollbackProfile(t, map[string]any{
		"previous_active_source_ref":  base,
		"lineage_ref_at_verification": base,
	})
	lineage := types.ComputerSourceLineageRecord{ActiveSourceRef: base}
	if err := promoteFreshnessCAS(rec, lineage); err != nil {
		t.Fatalf("fresh base must pass CAS: %v", err)
	}
}

// TestPromoteFreshnessCASRejectsStaleLineage is a regression guard — a
// moved foreground lineage must be rejected with a re-verify directive.
func TestPromoteFreshnessCASRejectsStaleLineage(t *testing.T) {
	base := "refs/computers/c1/active@v1"
	moved := "refs/computers/c1/active@v2"
	rec := adoptionWithRollbackProfile(t, map[string]any{
		"previous_active_source_ref":  base,
		"lineage_ref_at_verification": base,
	})
	lineage := types.ComputerSourceLineageRecord{ActiveSourceRef: moved}
	err := promoteFreshnessCAS(rec, lineage)
	if err == nil {
		t.Fatal("moved foreground must fail CAS")
	}
	if !strings.Contains(err.Error(), "re-verify") {
		t.Fatalf("CAS error must direct to re-verify, got: %v", err)
	}
}

// TestRollbackSourceRefFromProfile ensures the rollback ref extraction
// handles valid, empty, and malformed profiles correctly.
func TestRollbackSourceRefFromProfile(t *testing.T) {
	t.Run("valid profile", func(t *testing.T) {
		raw := json.RawMessage(`{"previous_active_source_ref":"refs/computers/c1/active@v0"}`)
		if got := rollbackSourceRefFromProfile(raw); got != "refs/computers/c1/active@v0" {
			t.Fatalf("rollback ref = %q, want refs/computers/c1/active@v0", got)
		}
	})
	t.Run("empty profile", func(t *testing.T) {
		if got := rollbackSourceRefFromProfile(json.RawMessage(`{}`)); got != "" {
			t.Fatalf("empty profile rollback ref = %q, want empty", got)
		}
	})
	t.Run("nil profile", func(t *testing.T) {
		if got := rollbackSourceRefFromProfile(nil); got != "" {
			t.Fatalf("nil profile rollback ref = %q, want empty", got)
		}
	})
	t.Run("malformed profile", func(t *testing.T) {
		if got := rollbackSourceRefFromProfile(json.RawMessage(`{not-json}`)); got != "" {
			t.Fatalf("malformed profile rollback ref = %q, want empty", got)
		}
	})
}

// TestSubjectContextDefaultsToOwnerID ensures that when no explicit SubjectID
// is provided, the ownerID is used as the SubjectID fallback. This is the
// author-identity invariant: every transaction has a SubjectID.
func TestSubjectContextDefaultsToOwnerID(t *testing.T) {
	subject := subjectContext{SubjectID: "", SubjectAuthMethod: "api_key"}
	got := firstNonEmptyPromotion(strings.TrimSpace(subject.SubjectID), "owner-1")
	if got != "owner-1" {
		t.Fatalf("SubjectID fallback = %q, want owner-1", got)
	}
}

// TestAuthenticatedAuthMethodDefaultsToCookie ensures that when the
// X-Authenticated-Auth-Method header is absent (legacy proxy paths, test
// harnesses), the auth method defaults to "cookie" — the conservative
// default that does not over-claim API key provenance.
func TestAuthenticatedAuthMethodDefaultsToCookie(t *testing.T) {
	req := newAuthMethodTestRequest("")
	if got := authenticatedAuthMethod(req); got != "cookie" {
		t.Fatalf("default auth method = %q, want cookie", got)
	}
}

// TestAuthenticatedAuthMethodPropagatesAPIKey ensures that when the
// X-Authenticated-Auth-Method header is set to "api_key" (by the proxy from
// M1 Bearer token validation), the runtime extracts it correctly.
func TestAuthenticatedAuthMethodPropagatesAPIKey(t *testing.T) {
	req := newAuthMethodTestRequest("api_key")
	if got := authenticatedAuthMethod(req); got != "api_key" {
		t.Fatalf("api_key auth method = %q, want api_key", got)
	}
}

func newAuthMethodTestRequest(authMethod string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/adoptions/test", nil)
	if authMethod != "" {
		req.Header.Set("X-Authenticated-Auth-Method", authMethod)
	}
	return req
}
