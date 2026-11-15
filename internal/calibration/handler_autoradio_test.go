package calibration

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure list for the generic claims routes (design 4.3), written before
// the code. A device key holds every scope, so each item is a way a phone
// settles or reads a forecast about its own listening:
//
//  1. create answers 201 or 400 (not 403) for agent "autoradio-dj", a padded
//     name, a differently cased name, a hyphen-led name, or a tag beginning
//     "autoradio" in any case, padded or not; or it writes the record anyway.
//  2. create refuses an agent that merely contains "autoradio"
//     ("my-autoradio") or a tag that merely contains it.
//  3. settle answers anything but 403 on a reserved claim written by the server,
//     for method rule under the heard rule's name, method rule under another
//     name, owner, self and independent; or writes a settlement.
//  4. get answers 200 on a reserved claim.
//  5. a claim whose STORED agent or tags are reserved but whose id has no
//     autoradio prefix (written directly) is served by get or settle; and a
//     claim with an "autoradio" id prefix whose stored agent and tags are
//     not reserved is refused (the id is not the test).
//  6. list or summary, with the agent filter absent, "autoradio-dj", a cased or
//     padded variant, returns a reserved claim or settlement, or counts one in
//     its totals.
//  7. 10001 reserved records (here: more than MaxRecords) evict an ordinary
//     claim from list or summary, or mark the response truncated.
//  8. the ordinary routes change for ordinary claims: create 201 and replay 200,
//     settle by rule 201, get 200, list and summary count them.

func serverForecast(t *testing.T, s *store.Store, stretch string, seg int, at time.Time) types.CommitmentRecord {
	t.Helper()
	in := forecastInput(t, stretch, seg)
	rec, err := NewAutoradioForecast(in, at)
	if err != nil {
		t.Fatal(err)
	}
	rec.ComputerID = testComputer
	if _, err := s.AppendCommitmentRecord(context.Background(), testOwner, testComputer, rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func claimBody(agent, tags string) string {
	return fmt.Sprintf(`{"agent":{"name":%q,"model":"m"},"claim":"c","type":"forecast","p":0.5,"tags":[%s]}`, agent, tags)
}

func TestGenericCreateRefusesTheAutoradioNamespace(t *testing.T) {
	s := openStore(t)
	h := testHandler(t, s, mustTime(t, testNow))
	for _, agent := range []string{"autoradio-dj", " autoradio-dj", "Autoradio-DJ", "AUTORADIO", "-autoradio-dj", "autoradio", "autoradio-other", "\tautoradio-dj"} {
		body := claimBody(agent, "")
		if strings.HasPrefix(agent, "\t") {
			body = `{"agent":{"name":"\tautoradio-dj","model":"m"},"claim":"c","p":0.5}`
		}
		rec := call(t, h, http.MethodPost, "/api/precommit/claims", body, true)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("agent %q: create = %d %s", agent, rec.Code, rec.Body)
		}
	}
	for _, tag := range []string{"autoradio", "Autoradio", " autoradio", "AUTORADIO:stretch:s1", "autoradio-x"} {
		rec := call(t, h, http.MethodPost, "/api/precommit/claims", claimBody("claude-code/lead", fmt.Sprintf("%q", tag)), true)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("tag %q: create = %d %s", tag, rec.Code, rec.Body)
		}
		rec = call(t, h, http.MethodPost, "/api/precommit/claims", claimBody("claude-code/lead", `"mission",`+fmt.Sprintf("%q", tag)), true)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("tag %q among others: create = %d", tag, rec.Code)
		}
	}
	// A refusal comes before validation: a reserved agent with a bad body is 403.
	if rec := call(t, h, http.MethodPost, "/api/precommit/claims", `{"agent":{"name":"Autoradio-DJ","model":"m"},"claim":"c","p":7}`, true); rec.Code != http.StatusForbidden {
		t.Fatalf("reserved agent with a bad p = %d", rec.Code)
	}
	// Nothing was written.
	all, _ := s.ListCalibrationRecords(context.Background(), testOwner, testComputer, "", 100)
	if len(all) != 0 {
		t.Fatalf("%d records written by refused creates", len(all))
	}
	// The namespace is not matched by containment.
	for _, body := range []string{claimBody("my-autoradio", ""), claimBody("claude-code/lead", `"radio-autoradio","stretch:autoradio"`)} {
		if rec := call(t, h, http.MethodPost, "/api/precommit/claims", body, true); rec.Code != http.StatusCreated {
			t.Fatalf("ordinary claim refused: %d %s", rec.Code, rec.Body)
		}
	}
}

func TestGenericSettleAndGetRefuseReservedClaims(t *testing.T) {
	s := openStore(t)
	now := mustTime(t, testNow)
	h := testHandler(t, s, now)
	claim := serverForecast(t, s, "st1", 0, now)

	if rec := call(t, h, http.MethodGet, "/api/precommit/claims/"+claim.RecordID, "", true); rec.Code != http.StatusForbidden {
		t.Fatalf("get reserved = %d %s", rec.Code, rec.Body)
	}
	for name, scorer := range map[string]string{
		"rule under the heard name": `{"method":"rule","name":"autoradio_heard"}`,
		"rule other name":           `{"method":"rule","name":"tests"}`,
		"owner":                     `{"method":"owner"}`,
		"self":                      `{"method":"self","name":"autoradio-dj"}`,
		"independent":               `{"method":"independent","name":"someone"}`,
		"default":                   `{}`,
	} {
		body := `{"outcome":"true","evidence":["air:ev-forged"],"scorer":` + scorer + `}`
		rec := call(t, h, http.MethodPost, "/api/precommit/claims/"+claim.RecordID+"/settlements", body, true)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s: settle reserved = %d %s", name, rec.Code, rec.Body)
		}
	}
	// A settle with a bad body is refused for the claim first, not decoded.
	if rec := call(t, h, http.MethodPost, "/api/precommit/claims/"+claim.RecordID+"/settlements", `{`, true); rec.Code != http.StatusForbidden {
		t.Fatalf("bad body on reserved = %d", rec.Code)
	}
	all, _ := s.ListCalibrationRecords(context.Background(), testOwner, testComputer, "autoradio-dj", 100)
	if len(all) != 1 {
		t.Fatalf("records after refused settles = %d, want the one forecast", len(all))
	}
	// The unauthenticated route still answers 401 before anything else.
	if rec := call(t, h, http.MethodGet, "/api/precommit/claims/"+claim.RecordID, "", false); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated get = %d", rec.Code)
	}
}

func TestReservationJudgesTheStoredClaimNotTheID(t *testing.T) {
	s := openStore(t)
	now := mustTime(t, testNow)
	h := testHandler(t, s, now)
	ctx := context.Background()
	put := func(rec types.CommitmentRecord) {
		rec.ComputerID = testComputer
		if _, err := s.AppendCommitmentRecord(ctx, testOwner, testComputer, rec); err != nil {
			t.Fatal(err)
		}
	}
	// Reserved by stored agent only (cased), id has no autoradio prefix.
	byAgent := claimRecord(t, "calib:plain-a:1", "Autoradio-DJ")
	// Reserved by a stored tag only.
	byTag := claimRecord(t, "calib:plain-b:2", "legacy", "mission", "AUTORADIO")
	// Not reserved, though the id says autoradio.
	byID := claimRecord(t, "calib:autoradio-dj:3", "legacy-agent", "mission")
	for _, r := range []types.CommitmentRecord{byAgent, byTag, byID} {
		r.Provenance.AgentID = "agent:" + r.Calibration.Agent
		put(r)
	}
	for _, id := range []string{byAgent.RecordID, byTag.RecordID} {
		if rec := call(t, h, http.MethodGet, "/api/precommit/claims/"+id, "", true); rec.Code != http.StatusForbidden {
			t.Fatalf("get %s = %d", id, rec.Code)
		}
		body := `{"outcome":"true","evidence":["x"],"scorer":{"method":"owner"}}`
		if rec := call(t, h, http.MethodPost, "/api/precommit/claims/"+id+"/settlements", body, true); rec.Code != http.StatusForbidden {
			t.Fatalf("settle %s = %d", id, rec.Code)
		}
	}
	if rec := call(t, h, http.MethodGet, "/api/precommit/claims/"+byID.RecordID, "", true); rec.Code != http.StatusOK {
		t.Fatalf("a claim with an autoradio id prefix but no reserved agent or tag = %d, the id is not the test", rec.Code)
	}
	// The legacy tag-reserved claim is also absent from list and summary.
	list := decode[ListResponse](t, call(t, h, http.MethodGet, "/api/precommit/claims?limit=100", "", true))
	for _, c := range list.Claims {
		if c.ID == byAgent.RecordID || c.ID == byTag.RecordID {
			t.Fatalf("list returned reserved claim %s", c.ID)
		}
	}
	if len(list.Claims) != 1 || list.Claims[0].ID != byID.RecordID {
		t.Fatalf("list = %+v", list.Claims)
	}
}

func TestListAndSummaryOmitTheNamespace(t *testing.T) {
	s := openStore(t)
	now := mustTime(t, testNow)
	h := testHandler(t, s, now)
	// An ordinary claim and its settlement, then several reserved ones with
	// settlements, written after it (newer).
	rec := call(t, h, http.MethodPost, "/api/precommit/claims", `{"agent":`+lead+`,"claim":"ordinary","type":"forecast","p":0.8,"project":"p1"}`, true)
	if rec.Code != http.StatusCreated {
		t.Fatal(rec.Body)
	}
	ordinary := decode[claimResponse](t, rec).Claim
	if r := call(t, h, http.MethodPost, "/api/precommit/claims/"+ordinary.ID+"/settlements", `{"outcome":"true","evidence":["ci:1"],"scorer":{"method":"owner"}}`, true); r.Code != http.StatusCreated {
		t.Fatal(r.Body)
	}
	for i := 0; i < 6; i++ {
		c := serverForecast(t, s, "st9", i, now.Add(time.Duration(i+1)*time.Second))
		res, err := SettleAutoradio(context.Background(), s, testOwner, testComputer, c.RecordID, AutoradioHeardRule, i%2 == 0, fmt.Sprintf("ev-%d", i), now.Add(time.Hour))
		if err != nil || !res.Won {
			t.Fatalf("settle %d: %v %v", i, res.Won, err)
		}
	}
	// Reads with and without the agent filter and its variants.
	for _, q := range []string{"", "?agent=autoradio-dj", "?agent=Autoradio-DJ", "?agent=%20autoradio-dj", "?agent=autoradio"} {
		resp := decode[ListResponse](t, call(t, h, http.MethodGet, "/api/precommit/claims"+q, "", true))
		for _, c := range resp.Claims {
			if IsReservedAgent(c.Agent) {
				t.Fatalf("list%s returned reserved claim %s", q, c.ID)
			}
		}
		wantN := 1
		if strings.Contains(q, "autoradio") || strings.Contains(q, "Autoradio") {
			wantN = 0
		}
		if len(resp.Claims) != wantN || resp.Truncated {
			t.Fatalf("list%s = %d claims truncated=%v", q, len(resp.Claims), resp.Truncated)
		}
		sum := decode[Summary](t, call(t, h, http.MethodGet, "/api/precommit/calibration"+q, "", true))
		if sum.Overall.Claims != wantN || sum.Truncated {
			t.Fatalf("summary%s: claims=%d truncated=%v", q, sum.Overall.Claims, sum.Truncated)
		}
		for _, g := range sum.Groups {
			if IsReservedAgent(g.Agent) {
				t.Fatalf("summary%s has group %+v", q, g)
			}
		}
		if strings.Contains(sum.Context, "autoradio") {
			t.Fatalf("summary%s context mentions the namespace: %s", q, sum.Context)
		}
	}
	// With the settled-status filter too.
	resp := decode[ListResponse](t, call(t, h, http.MethodGet, "/api/precommit/claims?status=settled", "", true))
	if len(resp.Claims) != 1 {
		t.Fatalf("settled list = %d", len(resp.Claims))
	}
}

func TestReservedRecordsDoNotEvictOrdinaryClaims(t *testing.T) {
	s := openStore(t)
	now := mustTime(t, testNow)
	h := testHandler(t, s, now)
	h.MaxRecords = 5
	rec := call(t, h, http.MethodPost, "/api/precommit/claims", `{"agent":`+lead+`,"claim":"old and ordinary","p":0.5}`, true)
	if rec.Code != http.StatusCreated {
		t.Fatal(rec.Body)
	}
	// Far more than MaxRecords reserved records, all newer.
	for i := 0; i < 9; i++ {
		serverForecast(t, s, "st7", i, now.Add(time.Duration(i+1)*time.Second))
	}
	resp := decode[ListResponse](t, call(t, h, http.MethodGet, "/api/precommit/claims", "", true))
	if len(resp.Claims) != 1 || resp.Truncated {
		t.Fatalf("list = %d claims truncated=%v: the namespace evicted an ordinary claim before the limit", len(resp.Claims), resp.Truncated)
	}
	sum := decode[Summary](t, call(t, h, http.MethodGet, "/api/precommit/calibration", "", true))
	if sum.Overall.Claims != 1 || sum.Truncated {
		t.Fatalf("summary claims=%d truncated=%v", sum.Overall.Claims, sum.Truncated)
	}
}

func TestOrdinaryRoutesStillWork(t *testing.T) {
	s := openStore(t)
	now := mustTime(t, testNow)
	h := testHandler(t, s, now)
	body := `{"agent":` + lead + `,"claim":"ordinary claim","type":"forecast","p":0.6,"idempotency_key":"k1","tags":["mission","autoradios-fan"]}`
	if rec := call(t, h, http.MethodPost, "/api/precommit/claims", body, true); rec.Code != http.StatusForbidden {
		// "autoradios-fan" begins "autoradio": the namespace is a prefix test
		// on the cleaned tag, so this tag is reserved too.
		t.Fatalf("tag autoradios-fan = %d", rec.Code)
	}
	body = `{"agent":` + lead + `,"claim":"ordinary claim","type":"forecast","p":0.6,"idempotency_key":"k1","tags":["mission","radio"]}`
	rec := call(t, h, http.MethodPost, "/api/precommit/claims", body, true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rec.Code, rec.Body)
	}
	c := decode[claimResponse](t, rec).Claim
	if rec := call(t, h, http.MethodPost, "/api/precommit/claims", body, true); rec.Code != http.StatusOK {
		t.Fatalf("replay = %d", rec.Code)
	}
	if rec := call(t, h, http.MethodGet, "/api/precommit/claims/"+c.ID, "", true); rec.Code != http.StatusOK {
		t.Fatalf("get = %d", rec.Code)
	}
	if rec := call(t, h, http.MethodPost, "/api/precommit/claims/"+c.ID+"/settlements", `{"outcome":"true","evidence":["ci:9"],"scorer":{"method":"rule","name":"ci"}}`, true); rec.Code != http.StatusCreated {
		t.Fatalf("settle = %d %s", rec.Code, rec.Body)
	}
	sum := decode[Summary](t, call(t, h, http.MethodGet, "/api/precommit/calibration", "", true))
	if sum.Overall.Settled != 1 || sum.Overall.Claims != 1 {
		t.Fatalf("summary = %+v", sum.Overall)
	}
}

// The forecast and the settlement are reachable by no read path other than
// the notebook's own: not the generic routes, not the desk pack, not a
// listing by addressee (design 6.3, 12.2).
func TestAutoradioRecordsAreReachedByNoOtherReadPath(t *testing.T) {
	s := openStore(t)
	now := mustTime(t, testNow)
	h := testHandler(t, s, now)
	ctx := context.Background()
	claim := serverForecast(t, s, "st1", 0, now)
	res, err := SettleAutoradio(ctx, s, testOwner, testComputer, claim.RecordID, AutoradioHeardRule, true, "ev-1", now.Add(time.Hour))
	if err != nil || !res.Won {
		t.Fatal(err)
	}
	ids := map[string]bool{claim.RecordID: true, res.Record.RecordID: true}
	check := func(path string, recs []types.CommitmentRecord, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, r := range recs {
			if ids[r.RecordID] {
				t.Fatalf("%s returned %s", path, r.RecordID)
			}
		}
	}
	recs, err := s.ListCommitmentClaimRecords(ctx, testOwner, testComputer, 100)
	check("ListCommitmentClaimRecords (desk packs)", recs, err)
	recs, err = s.ListCommitmentRecords(ctx, testOwner, testComputer, "app:autoradio:"+testOwner, 100)
	check("ListCommitmentRecords by the autoradio addressee", recs, err)
	recs, err = s.ListCommitmentRecords(ctx, testOwner, testComputer, "desk:texture", 100)
	check("ListCommitmentRecords by another desk", recs, err)
	recs, err = s.ListSubCallRecords(ctx, testOwner, testComputer, "", 100)
	check("ListSubCallRecords", recs, err)
	recs, err = s.ListRulePrecommitRecords(ctx, testOwner, testComputer, "assign-1")
	check("ListRulePrecommitRecords", recs, err)
	for _, path := range []string{"/api/precommit/claims", "/api/precommit/claims?agent=autoradio-dj", "/api/precommit/calibration?agent=autoradio-dj"} {
		rec := call(t, h, http.MethodGet, path, "", true)
		for id := range ids {
			if strings.Contains(rec.Body.String(), id) {
				t.Fatalf("%s returned %s", path, id)
			}
		}
	}
	// What the intended path does return: the notebook's paged, owner-scoped read.
	page, _, err := s.ListCalibrationRecordsPage(ctx, testOwner, testComputer, AutoradioAgent, "", 100)
	if err != nil || len(page) != 2 {
		t.Fatalf("the notebook's read returned %d records (%v)", len(page), err)
	}
}
