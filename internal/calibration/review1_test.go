package calibration

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure list for review round 1 (both seats accepted; their should-fix
// items), written before the code:
//  1. a tag-only legacy reserved claim, or a hyphen-led agent that Slug folds
//     in and the SQL prefix test does not, uses up the generic route's record
//     limit (list or summary shows fewer ordinary claims, or truncated);
//     truncated is true at exactly the limit.
//  2. Reader.StretchForecasts returns an ordinary claim that carries a
//     stretch tag, or a record of another kind.
//  3. the read-path test does not include the autoradio.turn records or does not
//     say what reads them.
//  4. the one-caller test is a substring scan: a function value, any use of
//     NewAutoradioSettlementRecord, a NewSettlementRecord call with the heard
//     rule, or a literal of the autoradio.settlement or autoradio.air kinds
//     outside the allowed files passes it; it skips internal/calibration whole.
//  5. NewAutoradioForecast refuses a padded title that design 3.2 does not
//     drop (it trims instead); an empty title is pinned as invalid in the
//     fixtures.
//  6. SettleAutoradio reports Won for a lost race whose settlement has the same
//     commit stamp but another air ref.
//  7. the fixtures leave the hash case, a non-hex hash, a signal that names a
//     segment with a wrong hash, and timestamps with offsets unpinned, and the
//     reference evaluator compares timestamps as strings.

func TestLegacyReservedRecordsDoNotConsumeTheLimit(t *testing.T) {
	s := openStore(t)
	now := mustTime(t, testNow)
	h := testHandler(t, s, now)
	h.MaxRecords = 3
	ctx := context.Background()
	if rec := call(t, h, http.MethodPost, "/api/precommit/claims", `{"agent":`+lead+`,"claim":"old and ordinary","p":0.5}`, true); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body)
	}
	put := func(id, agent string, tags ...string) {
		rec := claimRecord(t, id, agent, tags...)
		rec.ComputerID = testComputer
		rec.Provenance.AgentID = "agent:" + agent
		if _, err := s.AppendCommitmentRecord(ctx, testOwner, testComputer, rec); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 4; i++ {
		put(fmt.Sprintf("calib:legacy-h:%d", i), "-autoradio-dj")
		put(fmt.Sprintf("calib:legacy-t:%d", i), "legacy-agent", "mission", "AUTORADIO")
	}
	for _, path := range []string{"/api/precommit/claims", "/api/precommit/claims?limit=1000"} {
		resp := decode[ListResponse](t, call(t, h, http.MethodGet, path, "", true))
		if len(resp.Claims) != 1 || resp.Truncated {
			t.Fatalf("%s: %d claims truncated=%v: eight legacy reserved records used up the limit of 3", path, len(resp.Claims), resp.Truncated)
		}
	}
	sum := decode[Summary](t, call(t, h, http.MethodGet, "/api/precommit/calibration", "", true))
	if sum.Overall.Claims != 1 || sum.Truncated {
		t.Fatalf("summary claims=%d truncated=%v", sum.Overall.Claims, sum.Truncated)
	}
}

func TestGenericListTruncatesOnlyWhenMoreRemain(t *testing.T) {
	s := openStore(t)
	h := testHandler(t, s, mustTime(t, testNow))
	h.MaxRecords = 3
	for i := 0; i < 3; i++ {
		body := fmt.Sprintf(`{"agent":%s,"claim":"c%d","p":0.5,"idempotency_key":"k%d"}`, lead, i, i)
		if rec := call(t, h, http.MethodPost, "/api/precommit/claims", body, true); rec.Code != http.StatusCreated {
			t.Fatal(rec.Body)
		}
	}
	resp := decode[ListResponse](t, call(t, h, http.MethodGet, "/api/precommit/claims", "", true))
	if len(resp.Claims) != 3 || resp.Truncated {
		t.Fatalf("exactly the limit: %d claims truncated=%v", len(resp.Claims), resp.Truncated)
	}
	body := fmt.Sprintf(`{"agent":%s,"claim":"c9","p":0.5,"idempotency_key":"k9"}`, lead)
	if rec := call(t, h, http.MethodPost, "/api/precommit/claims", body, true); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body)
	}
	resp = decode[ListResponse](t, call(t, h, http.MethodGet, "/api/precommit/claims", "", true))
	if len(resp.Claims) != 3 || !resp.Truncated {
		t.Fatalf("one past the limit: %d claims truncated=%v", len(resp.Claims), resp.Truncated)
	}
}

func TestStretchForecastsAreTheServersOnly(t *testing.T) {
	s := openStore(t)
	at := mustTime(t, testStart)
	seedStretch(t, s, "sf", 2, 0, at)
	foreign := claimRecord(t, "calib:plain:foreign", "someone-else", "stretch:sf")
	foreign.ComputerID = testComputer
	foreign.Provenance.AgentID = "agent:someone-else"
	if _, err := s.AppendCommitmentRecord(context.Background(), testOwner, testComputer, foreign); err != nil {
		t.Fatal(err)
	}
	claims, truncated, err := Reader{Ledger: s}.StretchForecasts(context.Background(), testOwner, testComputer, "sf")
	if err != nil || truncated || len(claims) != 2 {
		t.Fatalf("%d claims truncated=%v err=%v: an ordinary claim carrying the stretch tag is not a forecast", len(claims), truncated, err)
	}
	for _, c := range claims {
		if c.Calibration.Agent != AutoradioAgent || c.SubjectRefs["record_kind"] != "autoradio.forecast" {
			t.Fatalf("returned %s", c.RecordID)
		}
	}
}

func turnRecords(owner string) (req, rep types.CommitmentRecord) {
	labels := types.NewLabelSet(types.OwnerPrivateLabel("autoradio_turn")).Strings()
	refs := func() map[string]string {
		return map[string]string{"desk": AutoradioDesk, "record_kind": "autoradio.turn", "turn_id": "t-1"}
	}
	req = types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "autoradio-turn-req:t-1", Kind: types.CommitmentKindDirective,
		Discrepancy: types.DiscrepancyUnresolved, ComputerID: testComputer, EmitterKind: types.CommitmentEmitterOwner,
		Directive:   &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNotice, Objective: "more like this", Body: "more like this"},
		Provenance:  types.CommitmentProvenance{AgentID: owner, ModelID: "owner", Origin: types.OriginOwner, CommittedAt: "2026-11-15T10:00:00Z", Labels: labels},
		SubjectRefs: refs(), Addressee: AutoradioDesk + ":" + owner,
	}
	rep = types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "autoradio-turn-rep:t-1", Kind: types.CommitmentKindReport,
		Discrepancy: types.DiscrepancyUnresolved, ComputerID: testComputer, EmitterKind: types.CommitmentEmitterAgent,
		Report:      &types.CommitmentReport{Claims: []types.ReportClaim{{Text: "Here is another."}}, StopReason: "completed"},
		Provenance:  types.CommitmentProvenance{AgentID: AutoradioDesk + ":" + owner, ModelID: "m", CommittedAt: "2026-11-15T10:00:01Z", Labels: labels},
		SubjectRefs: refs(), RelatedIDs: []string{req.RecordID},
	}
	return req, rep
}

// The calibration kinds are kept out of every other path by their calibration
// body. The turn records already exist and are not: the request is a
// directive (kept out of the desk pack by that), the reply is a plain report
// and the desk pack returns it. That is recorded as a problem, not fixed here
// (docs/problems/autoradio-turn-reply-in-desk-pack-2026-11-15.md).
func TestAutoradioTurnRecordsReadPaths(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()
	req, rep := turnRecords(testOwner)
	for _, r := range []types.CommitmentRecord{req, rep} {
		if _, err := s.AppendCommitmentRecord(ctx, testOwner, testComputer, r); err != nil {
			t.Fatal(err)
		}
	}
	has := func(recs []types.CommitmentRecord, id string) bool {
		for _, r := range recs {
			if r.RecordID == id {
				return true
			}
		}
		return false
	}
	bySubject, _ := s.ListCommitmentRecordsBySubjectRef(ctx, testOwner, testComputer, "record_kind", "autoradio.turn", 10)
	if !has(bySubject, req.RecordID) || !has(bySubject, rep.RecordID) {
		t.Fatalf("the read by subject ref lost a turn record: %d", len(bySubject))
	}
	byAddressee, _ := s.ListCommitmentRecords(ctx, testOwner, testComputer, AutoradioDesk+":"+testOwner, 10)
	if !has(byAddressee, req.RecordID) {
		t.Fatal("the autoradio addressee does not list the request")
	}
	other, _ := s.ListCommitmentRecords(ctx, testOwner, testComputer, "desk:texture", 10)
	if has(other, req.RecordID) || has(other, rep.RecordID) {
		t.Fatal("another desk's addressee listing returned a turn record")
	}
	h := testHandler(t, s, mustTime(t, testNow))
	for _, path := range []string{"/api/precommit/claims", "/api/precommit/calibration"} {
		if body := call(t, h, http.MethodGet, path, "", true).Body.String(); strings.Contains(body, "autoradio-turn") {
			t.Fatalf("%s returned a turn record", path)
		}
	}
	pack, err := s.ListCommitmentClaimRecords(ctx, testOwner, testComputer, 100)
	if err != nil {
		t.Fatal(err)
	}
	if has(pack, req.RecordID) {
		t.Fatal("the desk pack returned the turn request")
	}
	// The known problem, pinned so a fix is noticed: the reply is in the pack.
	if !has(pack, rep.RecordID) {
		t.Fatal("the desk pack no longer returns the turn reply: the problem note docs/problems/autoradio-turn-reply-in-desk-pack-2026-11-15.md is resolved; update it and this test")
	}
}

// ---- the one-caller test by reference, go/ast ----

// The reserved settle method and the records only the heard route may write are
// found by what the code refers to, not by call syntax: any identifier use of
// SettleAutoradio or NewAutoradioSettlementRecord, any NewSettlementRecord
// call that passes the heard rule, any use of AutoradioHeardRule, and any
// string literal of the autoradio.settlement or autoradio.air record kinds, in
// a non-test Go file outside the files that own them.
func TestReservedWritersAreReferencedOnlyByTheirOwners(t *testing.T) {
	root := filepath.Join("..", "..")
	owners := map[string]map[string]bool{
		"SettleAutoradio":              {"internal/calibration/autoradio.go": true, "internal/agentcore/api_autoradio_heard.go": true},
		"NewAutoradioSettlementRecord": {"internal/calibration/autoradio.go": true},
		"AutoradioHeardRule":           {"internal/calibration/autoradio.go": true, "internal/agentcore/api_autoradio_heard.go": true},
		`"autoradio_heard"`:            {"internal/calibration/autoradio.go": true, "internal/agentcore/api_autoradio_heard.go": true},
		`"autoradio.settlement"`:       {"internal/calibration/autoradio.go": true, "internal/types/autoradio_records.go": true, "internal/agentcore/api_autoradio_heard.go": true},
		`"autoradio.air"`:              {"internal/types/autoradio_records.go": true, "internal/agentcore/api_autoradio_heard.go": true},
	}
	var offenders []string
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil // a file that does not parse is not a caller
		}
		declared := map[*ast.Ident]bool{}
		ast.Inspect(file, func(n ast.Node) bool {
			if fd, ok := n.(*ast.FuncDecl); ok {
				declared[fd.Name] = true
			}
			return true
		})
		ast.Inspect(file, func(n ast.Node) bool {
			var key string
			switch x := n.(type) {
			case *ast.Ident:
				if !declared[x] {
					key = x.Name
				}
			case *ast.BasicLit:
				if x.Kind == token.STRING {
					key = x.Value
				}
			}
			if allowed, watched := owners[key]; watched && !allowed[rel] {
				offenders = append(offenders, rel+": "+key)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("reserved writers referenced outside their owners: %v", offenders)
	}
}

// ---- constructor and settle, review items ----

func TestNewAutoradioForecastTrimsAPaddedTitle(t *testing.T) {
	in := forecastInput(t, "st1", 0)
	in.Title = "  A quiet harbour \t"
	rec, err := NewAutoradioForecast(in, mustTime(t, testNow))
	if err != nil {
		t.Fatalf("a padded title is one line and design 3.2 does not drop it: %v", err)
	}
	if got := rec.Calibration.Claim.Text; got != "The listener hears segment st1.s0 (story: A quiet harbour) through." {
		t.Fatalf("claim = %q", got)
	}
	for _, title := range []string{"", "   ", "\t"} {
		in.Title = title
		if _, err := NewAutoradioForecast(in, mustTime(t, testNow)); err == nil {
			t.Fatalf("empty title %q accepted", title)
		}
	}
	in.Title = strings.Repeat("x", 200)
	if _, err := NewAutoradioForecast(in, mustTime(t, testNow)); err != nil {
		t.Fatalf("200 bytes refused: %v", err)
	}
	in.Title = "  " + strings.Repeat("x", 201)
	if _, err := NewAutoradioForecast(in, mustTime(t, testNow)); err == nil {
		t.Fatal("201 bytes accepted")
	}
}

func TestSettleAutoradioWonComparesTheAirRef(t *testing.T) {
	ctx := context.Background()
	m := newMemLedger()
	claim := seedForecast(t, m, "st1", 0)
	at := mustTime(t, "2026-11-15T10:30:00Z")
	winner, err := NewAutoradioSettlementRecord(claim, AutoradioHeardRule, true, "ev-OTHER", at)
	if err != nil {
		t.Fatal(err)
	}
	m.raceOnAppend = &winner
	// Same commit stamp, same outcome, another air ref: the stored one is not ours.
	res, err := SettleAutoradio(ctx, m, "o", "c", claim.RecordID, AutoradioHeardRule, true, "ev-MINE", at)
	if err != nil || res.Won {
		t.Fatalf("won=%v err=%v", res.Won, err)
	}
	if res.Record.EvidenceRefs[0] != "air:ev-OTHER" {
		t.Fatalf("returned %v", res.Record.EvidenceRefs)
	}
}
