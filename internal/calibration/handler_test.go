package calibration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure modes for the routes (design §6, items 1-4, 7, 9-11), written
// before the code:
//   - an unauthenticated caller writes a claim;
//   - a claim without a model, without p, with p outside [0,1], or with an
//     unknown type is stored;
//   - a client backdates a claim (committed_at must be the server's), or
//     sends stated_at without marking the claim reconstructed;
//   - a replayed claim duplicates, or a reused idempotency key overwrites a
//     different claim;
//   - a settled claim is edited: a scorer's second, different settlement
//     replaces the first, or an edit route exists;
//   - "independent" is claimed by the claimant itself, or "self" by
//     someone else;
//   - a settlement targets a desk's record, or a missing claim;
//   - calibration records crowd a desk's own records out of its pack.

const testOwner, testComputer = "owner-calib", "computer-calib"

func openStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "calibration.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func testHandler(t *testing.T, s *store.Store, now time.Time) *Handler {
	t.Helper()
	return &Handler{
		Ledger:     s,
		ComputerID: func() string { return testComputer },
		Authenticate: func(r *http.Request) (string, error) {
			if u := r.Header.Get("X-Authenticated-User"); u != "" {
				return u, nil
			}
			return "", errors.New("no user")
		},
		Now: func() time.Time { return now },
	}
}

func call(t *testing.T, h http.Handler, method, path, body string, auth bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if auth {
		req.Header.Set("X-Authenticated-User", testOwner)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %s: %v", rec.Body, err)
	}
	return out
}

const lead = `{"name":"claude-code/lead","model":"claude-opus-5-5","session_id":"s1"}`

func TestClaimValidationAndAuth(t *testing.T) {
	now := time.Date(2026, 10, 14, 20, 0, 0, 0, time.UTC)
	h := testHandler(t, openStore(t), now)
	good := `{"agent":` + lead + `,"claim":"the jailer deploy causes no launch failure","type":"forecast","p":0.85,"criterion":"vmctl journal","deadline":"2026-10-15T20:00:00Z"}`
	if rec := call(t, h, http.MethodPost, "/api/precommit/claims", good, false); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated = %d", rec.Code)
	}
	for name, body := range map[string]string{
		"no model":        `{"agent":{"name":"x"},"claim":"c","p":0.5}`,
		"no agent":        `{"agent":{"model":"m"},"claim":"c","p":0.5}`,
		"no p":            `{"agent":` + lead + `,"claim":"c"}`,
		"p above 1":       `{"agent":` + lead + `,"claim":"c","p":1.2}`,
		"p below 0":       `{"agent":` + lead + `,"claim":"c","p":-0.1}`,
		"no claim":        `{"agent":` + lead + `,"claim":"  ","p":0.5}`,
		"unknown type":    `{"agent":` + lead + `,"claim":"c","p":0.5,"type":"vibe"}`,
		"bad deadline":    `{"agent":` + lead + `,"claim":"c","p":0.5,"deadline":"tomorrow"}`,
		"stated_at alone": `{"agent":` + lead + `,"claim":"c","p":0.5,"stated_at":"2026-10-01T00:00:00Z"}`,
		"unknown field":   `{"agent":` + lead + `,"claim":"c","p":0.5,"committed_at":"2020-01-01T00:00:00Z"}`,
		"not json":        `{`,
	} {
		if rec := call(t, h, http.MethodPost, "/api/precommit/claims", body, true); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d %s", name, rec.Code, rec.Body)
		}
	}
	rec := call(t, h, http.MethodPost, "/api/precommit/claims", good, true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rec.Code, rec.Body)
	}
	created := decode[struct{ Claim ClaimView }](t, rec).Claim
	if created.RecordedAt != now.Format(time.RFC3339Nano) || created.Status != StatusOpen || created.Model != "claude-opus-5-5" || created.SessionID != "s1" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(created.ID, "calib:claude-code-lead:") {
		t.Fatalf("claim id = %q", created.ID)
	}
	for _, method := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete} {
		if rec := call(t, h, method, "/api/precommit/claims/"+created.ID, good, true); rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s on a claim = %d; claims are never edited", method, rec.Code)
		}
	}
}

func TestClaimIdempotencyAndSettlementImmutability(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 14, 20, 0, 0, 0, time.UTC)
	s := openStore(t)
	h := testHandler(t, s, now)
	body := `{"agent":` + lead + `,"claim":"tests pass","p":0.95,"idempotency_key":"k1","tags":["ci"," ",""]}`
	first := call(t, h, http.MethodPost, "/api/precommit/claims", body, true)
	if first.Code != http.StatusCreated {
		t.Fatalf("first = %d %s", first.Code, first.Body)
	}
	claim := decode[struct{ Claim ClaimView }](t, first).Claim
	if len(claim.Tags) != 1 || claim.Tags[0] != "ci" {
		t.Fatalf("tags = %q", claim.Tags)
	}
	// The replay lands later; it is a no-op, not a second claim.
	h.Now = func() time.Time { return now.Add(time.Hour) }
	if rec := call(t, h, http.MethodPost, "/api/precommit/claims", body, true); rec.Code != http.StatusOK {
		t.Fatalf("replay = %d %s", rec.Code, rec.Body)
	}
	reused := `{"agent":` + lead + `,"claim":"tests pass","p":0.6,"idempotency_key":"k1"}`
	if rec := call(t, h, http.MethodPost, "/api/precommit/claims", reused, true); rec.Code != http.StatusConflict {
		t.Fatalf("reused key with a different p = %d %s", rec.Code, rec.Body)
	}
	stored, err := s.GetCommitmentRecord(ctx, testOwner, testComputer, claim.ID)
	if err != nil || stored == nil || stored.Calibration.Claim.Probability != 0.95 || stored.Provenance.CommittedAt != now.Format(time.RFC3339Nano) {
		t.Fatalf("stored claim changed: %+v %v", stored, err)
	}
	if stored.Kind != types.CommitmentKindPrecommit || stored.Precommit == nil || stored.Precommit.Distribution["true"] != 0.95 || stored.Provenance.ModelID != "claude-opus-5-5" {
		t.Fatalf("claim is not a typed precommit record: %+v", stored)
	}

	settle := "/api/precommit/claims/" + claim.ID + "/settlements"
	for name, b := range map[string]string{
		"no evidence":           `{"outcome":"false"}`,
		"bad outcome":           `{"outcome":"no","evidence":["ci"]}`,
		"self by someone else":  `{"outcome":"false","evidence":["ci"],"scorer":{"method":"self","name":"other"}}`,
		"independent by itself": `{"outcome":"false","evidence":["ci"],"scorer":{"method":"independent","name":"claude-code/lead"}}`,
		"independent anonymous": `{"outcome":"false","evidence":["ci"],"scorer":{"method":"independent"}}`,
		"rule without a name":   `{"outcome":"false","evidence":["ci"],"scorer":{"method":"rule"}}`,
		"unknown method":        `{"outcome":"false","evidence":["ci"],"scorer":{"method":"vibes","name":"x"}}`,
	} {
		if rec := call(t, h, http.MethodPost, settle, b, true); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
	if rec := call(t, h, http.MethodPost, "/api/precommit/claims/calib:nobody:0000/settlements", `{"outcome":"false","evidence":["ci"]}`, true); rec.Code != http.StatusNotFound {
		t.Fatalf("missing claim = %d", rec.Code)
	}
	// A desk's record is not a calibration claim.
	desk := types.CommitmentRecord{SchemaID: types.CommitmentRecordSchemaV1, RecordID: "texture:doc:precommit:1", Kind: types.CommitmentKindPrecommit,
		Provenance: types.CommitmentProvenance{AgentID: "texture:doc", CommittedAt: now.Format(time.RFC3339Nano)}, Discrepancy: types.DiscrepancyUnresolved}
	if _, err := s.AppendCommitmentRecord(ctx, testOwner, testComputer, desk); err != nil {
		t.Fatal(err)
	}
	if rec := call(t, h, http.MethodPost, "/api/precommit/claims/texture:doc:precommit:1/settlements", `{"outcome":"false","evidence":["ci"]}`, true); rec.Code != http.StatusNotFound {
		t.Fatalf("settling a desk record = %d %s", rec.Code, rec.Body)
	}

	indep := `{"outcome":"false","evidence":["CI 38579173174"],"scorer":{"method":"independent","name":"owner-review","model":"human"}}`
	rec := call(t, h, http.MethodPost, settle, indep, true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("settle = %d %s", rec.Code, rec.Body)
	}
	got := decode[struct{ Claim ClaimView }](t, rec).Claim
	if got.Status != StatusSettled || got.Outcome != types.CalibrationOutcomeFalse || got.Brier == nil || !near(*got.Brier, 0.9025) {
		t.Fatalf("settled view = %+v", got)
	}
	if rec := call(t, h, http.MethodPost, settle, indep, true); rec.Code != http.StatusOK {
		t.Fatalf("identical settlement replay = %d %s", rec.Code, rec.Body)
	}
	flipped := strings.Replace(indep, `"false"`, `"true"`, 1)
	if rec := call(t, h, http.MethodPost, settle, flipped, true); rec.Code != http.StatusConflict {
		t.Fatalf("a different settlement by the same scorer = %d %s; settled records are never edited", rec.Code, rec.Body)
	}
	// Another scorer appends; disagreement is kept, and independent governs over self.
	if rec := call(t, h, http.MethodPost, settle, `{"outcome":"true","evidence":["my run"]}`, true); rec.Code != http.StatusCreated {
		t.Fatalf("self settlement = %d %s", rec.Code, rec.Body)
	}
	rec = call(t, h, http.MethodGet, "/api/precommit/claims/"+claim.ID, "", true)
	view := decode[struct{ Claim ClaimView }](t, rec).Claim
	if len(view.Settlements) != 2 || view.Outcome != types.CalibrationOutcomeFalse || view.GoverningMethod != types.CalibrationScorerIndependent || !view.Disagreement {
		t.Fatalf("multi-scorer view = %+v", view)
	}
	resolve, err := s.GetCommitmentRecord(ctx, testOwner, testComputer, claim.ID+":settle:independent:owner-review")
	if err != nil || resolve == nil || resolve.Kind != types.CommitmentKindResolve || resolve.ParentID != claim.ID || resolve.Discrepancy != types.DiscrepancyContradicted || resolve.Provenance.ModelID != "human" {
		t.Fatalf("settlement is not a typed resolve record: %+v %v", resolve, err)
	}
	// The generic supervision view reads the same records without change.
	all, _ := s.ListCalibrationRecords(ctx, testOwner, testComputer, "", 100)
	joined := types.ResolveCommitments(all, now)
	if len(joined) != 1 || len(joined[0].Resolutions) != 2 || !joined[0].Disagreement {
		t.Fatalf("ResolveCommitments over calibration records = %+v", joined)
	}
}

func TestListAndCalibrationRoutes(t *testing.T) {
	now := time.Date(2026, 10, 14, 20, 0, 0, 0, time.UTC)
	h := testHandler(t, openStore(t), now)
	add := func(agent, claim string, p float64, deadline string) string {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"agent": map[string]string{"name": agent, "model": "m1"}, "claim": claim, "p": p, "deadline": deadline})
		rec := call(t, h, http.MethodPost, "/api/precommit/claims", string(body), true)
		if rec.Code != http.StatusCreated {
			t.Fatalf("add = %d %s", rec.Code, rec.Body)
		}
		return decode[struct{ Claim ClaimView }](t, rec).Claim.ID
	}
	a1 := add("alpha", "a one", 0.9, "")
	add("alpha", "a two", 0.7, "2026-10-14T01:00:00Z")
	add("alpha", "a three", 0.6, "2026-10-20T00:00:00Z")
	add("beta", "b one", 0.5, "")
	if rec := call(t, h, http.MethodPost, "/api/precommit/claims/"+a1+"/settlements", `{"outcome":"false","evidence":["e"],"scorer":{"method":"rule","name":"ci"}}`, true); rec.Code != http.StatusCreated {
		t.Fatalf("settle = %d %s", rec.Code, rec.Body)
	}
	list := func(q string) ListResponse {
		t.Helper()
		rec := call(t, h, http.MethodGet, "/api/precommit/claims"+q, "", true)
		if rec.Code != http.StatusOK {
			t.Fatalf("list %s = %d %s", q, rec.Code, rec.Body)
		}
		return decode[ListResponse](t, rec)
	}
	if got := list("?agent=alpha"); len(got.Claims) != 3 {
		t.Fatalf("alpha claims = %d", len(got.Claims))
	}
	if got := list("?agent=alpha&status=open"); len(got.Claims) != 2 {
		t.Fatalf("alpha open (incl. overdue) = %d", len(got.Claims))
	}
	if got := list("?status=overdue"); len(got.Claims) != 1 || got.Claims[0].Claim != "a two" {
		t.Fatalf("overdue = %+v", got.Claims)
	}
	if got := list("?status=settled"); len(got.Claims) != 1 || got.Claims[0].ID != a1 {
		t.Fatalf("settled = %+v", got.Claims)
	}
	if got := list("?agent=alpha&limit=1"); len(got.Claims) != 1 {
		t.Fatalf("limit ignored: %d", len(got.Claims))
	}
	if rec := call(t, h, http.MethodGet, "/api/precommit/claims?status=bogus", "", true); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad status = %d", rec.Code)
	}
	rec := call(t, h, http.MethodGet, "/api/precommit/calibration?agent=alpha", "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("calibration = %d %s", rec.Code, rec.Body)
	}
	sum := decode[Summary](t, rec)
	if sum.Overall.Claims != 3 || sum.Overall.Settled != 1 || sum.Overall.Overdue != 1 || sum.Overall.Brier == nil || !near(*sum.Overall.Brier, 0.81) {
		t.Fatalf("summary = %+v", sum.Overall)
	}
	if !strings.Contains(sum.Context, "alpha") {
		t.Fatalf("context = %q", sum.Context)
	}
	if rec := call(t, h, http.MethodGet, "/api/precommit/calibration?scorer=vibes", "", true); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad scorer = %d", rec.Code)
	}
	if rec := call(t, h, http.MethodGet, "/api/precommit/calibration?group_by=colour", "", true); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad group_by = %d", rec.Code)
	}
	if rec := call(t, h, http.MethodGet, "/api/precommit/nothing", "", true); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown route = %d", rec.Code)
	}
}

// Design §6 item 7: calibration records never enter a desk's pack window.
func TestDeskPackQueryExcludesCalibrationRecords(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 14, 20, 0, 0, 0, time.UTC)
	s := openStore(t)
	desk := types.CommitmentRecord{SchemaID: types.CommitmentRecordSchemaV1, RecordID: "management:o:report:1", Kind: types.CommitmentKindReport,
		Provenance: types.CommitmentProvenance{AgentID: "management:o", CommittedAt: now.Format(time.RFC3339Nano)}, Discrepancy: types.DiscrepancyUnresolved}
	if _, err := s.AppendCommitmentRecord(ctx, testOwner, testComputer, desk); err != nil {
		t.Fatal(err)
	}
	h := testHandler(t, s, now)
	for i := 0; i < 3; i++ {
		body := `{"agent":` + lead + `,"claim":"c","p":0.5,"idempotency_key":"` + string(rune('a'+i)) + `"}`
		if rec := call(t, h, http.MethodPost, "/api/precommit/claims", body, true); rec.Code != http.StatusCreated {
			t.Fatalf("add = %d %s", rec.Code, rec.Body)
		}
	}
	// The filter runs in the query, before the limit: the window holds the
	// desk's record and nothing else.
	got, err := s.ListCommitmentClaimRecords(ctx, testOwner, testComputer, 10)
	if err != nil || len(got) != 1 || got[0].RecordID != desk.RecordID {
		ids := []string{}
		for _, rec := range got {
			ids = append(ids, rec.RecordID)
		}
		t.Fatalf("desk pack window = %v, %v; calibration claims entered the desk's window", ids, err)
	}
}
