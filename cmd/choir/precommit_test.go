package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Calibration level 2 CLI and MCP
// (docs/archive/reports/choir-mcp-precommitments-2026-10-14.md). Ways the CLI could
// fail, written before the code:
//   - a claim is sent without p, or with a p the CLI silently defaulted;
//   - repeated --tag/--evidence flags keep only the last value;
//   - `settle <id> --flags` drops the flags because Go's flag parsing stops
//     at the first positional argument;
//   - a lead settling a subagent's claim is recorded as self (the scorer
//     must default to the calling agent, and the server decides self vs
//     independent by comparing names);
//   - `calibration --text` prints JSON instead of the pasteable context;
//   - import-ledger needs a key or touches the network in a dry run,
//     mis-parses a row, or double-writes on a rerun;
//   - the MCP server writes non-protocol bytes to stdout, answers a
//     notification, or turns an HTTP failure into a protocol error instead
//     of an isError result.

type captured struct {
	method, path, query string
	body                []byte
}

func captureServer(t *testing.T, reply string) (*[]captured, func()) {
	t.Helper()
	var got []captured
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		got = append(got, captured{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: raw})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reply))
	}))
	t.Setenv(hostEnvVar, srv.URL)
	t.Setenv(apiKeyEnvVar, "choir_sk_test")
	return &got, srv.Close
}

func setIdentity(t *testing.T, agent, model string) {
	t.Helper()
	t.Setenv("CHOIR_AGENT", agent)
	t.Setenv("CHOIR_MODEL", model)
	t.Setenv("CHOIR_SESSION", "session-1")
	t.Setenv("CHOIR_PROJECT", "")
}

func TestPrecommitAddSendsTheClaim(t *testing.T) {
	got, done := captureServer(t, `{"claim":{"id":"calib:x:1"}}`)
	defer done()
	setIdentity(t, "claude-code/lead", "claude-opus-5-5")
	var stdout bytes.Buffer
	code := run([]string{"precommit", "add", "--claim", "the jailer deploy causes no launch failure", "--p", "0.85",
		"--type", "forecast", "--tag", "vmctl", "--tag", "jailer,deploy", "--deadline", "2026-10-15T20:00:00Z",
		"--criterion", "vmctl journal", "--key", "k1"}, &stdout, io.Discard)
	if code != 0 || len(*got) != 1 {
		t.Fatalf("add exit %d, %d requests", code, len(*got))
	}
	req := (*got)[0]
	if req.method != http.MethodPost || req.path != "/api/precommit/claims" {
		t.Fatalf("request = %s %s", req.method, req.path)
	}
	var body calibration.ClaimRequest
	if err := json.Unmarshal(req.body, &body); err != nil {
		t.Fatal(err)
	}
	if body.Agent.Name != "claude-code/lead" || body.Agent.Model != "claude-opus-5-5" || body.Agent.SessionID != "session-1" ||
		body.P == nil || *body.P != 0.85 || body.Type != "forecast" || strings.Join(body.Tags, "|") != "vmctl|jailer|deploy" ||
		body.Deadline != "2026-10-15T20:00:00Z" || body.Criterion != "vmctl journal" || body.IdempotencyKey != "k1" {
		t.Fatalf("claim body = %s", req.body)
	}
	if !strings.Contains(stdout.String(), "calib:x:1") {
		t.Fatalf("stdout = %s", stdout.String())
	}
	for name, args := range map[string][]string{
		"no p":     {"precommit", "add", "--claim", "c"},
		"bad p":    {"precommit", "add", "--claim", "c", "--p", "likely"},
		"no claim": {"precommit", "add", "--p", "0.5"},
	} {
		if code := run(args, io.Discard, io.Discard); code != 2 {
			t.Fatalf("%s: exit %d, want 2", name, code)
		}
	}
	if len(*got) != 1 {
		t.Fatalf("a refused command reached the server: %d requests", len(*got))
	}
}

func TestPrecommitSettleDefaultsTheScorerToTheCallingAgent(t *testing.T) {
	got, done := captureServer(t, `{"claim":{}}`)
	defer done()
	setIdentity(t, "claude-code/lead", "claude-opus-5-5")
	if code := run([]string{"precommit", "settle", "calib:sub:1", "--outcome", "false", "--evidence", "CI 38579173174", "--evidence", "bdc996432"}, io.Discard, io.Discard); code != 0 {
		t.Fatalf("settle exit %d", code)
	}
	if code := run([]string{"precommit", "settle", "--outcome", "true", "--evidence", "e", "--method", "rule", "--scorer", "ci", "calib:sub:2"}, io.Discard, io.Discard); code != 0 {
		t.Fatalf("settle with trailing id exit %d", code)
	}
	if len(*got) != 2 || (*got)[0].path != "/api/precommit/claims/calib:sub:1/settlements" || (*got)[1].path != "/api/precommit/claims/calib:sub:2/settlements" {
		t.Fatalf("requests = %+v", *got)
	}
	var first, second calibration.SettlementRequest
	_ = json.Unmarshal((*got)[0].body, &first)
	_ = json.Unmarshal((*got)[1].body, &second)
	if first.Outcome != "false" || len(first.Evidence) != 2 || first.Scorer.Name != "claude-code/lead" || first.Scorer.Model != "claude-opus-5-5" || first.Scorer.Method != "" {
		t.Fatalf("default scorer = %s", (*got)[0].body)
	}
	if second.Scorer.Method != "rule" || second.Scorer.Name != "ci" || second.Scorer.Model != "" {
		t.Fatalf("explicit rule scorer = %s", (*got)[1].body)
	}
	if code := run([]string{"precommit", "settle", "--outcome", "true", "--evidence", "e"}, io.Discard, io.Discard); code != 2 {
		t.Fatalf("settle without an id exit %d", code)
	}
}

func TestPrecommitListAndCalibrationQueries(t *testing.T) {
	got, done := captureServer(t, `{"context":"Calibration track record (agent=a).\n","overall":{}}`)
	defer done()
	setIdentity(t, "a", "m")
	var stdout bytes.Buffer
	if code := run([]string{"precommit", "calibration", "--text", "--scorer", "independent"}, &stdout, io.Discard); code != 0 {
		t.Fatalf("calibration exit %d", code)
	}
	if stdout.String() != "Calibration track record (agent=a).\n" {
		t.Fatalf("--text printed %q", stdout.String())
	}
	if code := run([]string{"precommit", "list", "--all", "--status", "open", "--limit", "5"}, io.Discard, io.Discard); code != 0 {
		t.Fatalf("list exit %d", code)
	}
	if (*got)[0].path != "/api/precommit/calibration" || !strings.Contains((*got)[0].query, "agent=a") || !strings.Contains((*got)[0].query, "scorer=independent") {
		t.Fatalf("calibration request = %+v", (*got)[0])
	}
	if (*got)[1].path != "/api/precommit/claims" || strings.Contains((*got)[1].query, "agent=") || !strings.Contains((*got)[1].query, "status=open") {
		t.Fatalf("list request = %+v", (*got)[1])
	}
}

func ledgerPath(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "docs", "calibration", "claude-ledger.md"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestImportLedgerDryRunParsesTheLevelOneLedger(t *testing.T) {
	t.Setenv(apiKeyEnvVar, "")
	t.Setenv(hostEnvVar, "http://127.0.0.1:1") // a dry run must not dial
	var stdout bytes.Buffer
	if code := run([]string{"precommit", "import-ledger", "--file", ledgerPath(t)}, &stdout, io.Discard); code != 0 {
		t.Fatalf("dry run exit %d", code)
	}
	var plan importPlan
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("decode plan: %v\n%s", err, stdout.String())
	}
	// The ledger only grows; rows 1-18 are pinned below.
	if plan.Apply || len(plan.Rows) < 18 || len(plan.Skipped) != 0 {
		t.Fatalf("plan: apply=%v rows=%d skipped=%+v", plan.Apply, len(plan.Rows), plan.Skipped)
	}
	settledRows := 0
	for _, row := range plan.Rows {
		if row.Settlement != nil {
			settledRows++
		}
	}
	if settledRows < 12 {
		t.Fatalf("settled rows = %d, want at least 12", settledRows)
	}
	row := func(n int) importRow { return plan.Rows[n-1] }
	r1 := row(1)
	if r1.Claim.Agent.Name != "claude-code/subagent" || r1.Claim.Tags[0] != "S2a build" || *r1.Claim.P != 0.95 || !r1.Claim.Reconstructed ||
		r1.Claim.StatedAt != "2026-10-14T00:00:00Z" || r1.Claim.IdempotencyKey != "claude-ledger:1" ||
		r1.Settlement.Outcome != "false" || r1.Settlement.Scorer.Method != "independent" || r1.Settlement.Scorer.Name != "claude-code/lead" {
		t.Fatalf("row 1 = %+v / %+v", r1.Claim, r1.Settlement)
	}
	if r3 := row(3); r3.Claim.Agent.Name != "claude-code/lead" || r3.Settlement.Scorer.Method != "self" || r3.Settlement.Outcome != "false" {
		t.Fatalf("row 3 = %+v / %+v", r3.Claim, r3.Settlement)
	}
	if r5 := row(5); r5.Settlement.Outcome != "true" || r5.Settlement.Evidence[0] != "CI 38565529909" {
		t.Fatalf("row 5 = %+v", r5.Settlement)
	}
	if r11 := row(11); r11.Settlement.Outcome != "partial" || *r11.Claim.P != 0.8 {
		t.Fatalf("row 11 = %+v / %+v", r11.Claim, r11.Settlement)
	}
	if r13 := row(13); r13.Settlement != nil || r13.Claim.Reconstructed || r13.Claim.StatedAt != "" || r13.Claim.Criterion != "vmctl journal + verifier `--expect direct`" || *r13.Claim.P != 0.85 {
		t.Fatalf("row 13 = %+v", r13.Claim)
	}
	if code := run([]string{"precommit", "import-ledger", "--file", ledgerPath(t), "--apply"}, io.Discard, io.Discard); code != 2 {
		t.Fatalf("--apply without --model exit %d, want 2", code)
	}
}

// e2eServer serves the real calibration handler over a local Dolt store,
// with the proxy's role (authenticated owner) played by a key check.
func e2eServer(t *testing.T) (*store.Store, string) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "precommit-e2e.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	h := &calibration.Handler{
		Ledger:     s,
		ComputerID: func() string { return "computer-e2e" },
		Authenticate: func(r *http.Request) (string, error) {
			if r.Header.Get("Authorization") != "Bearer choir_sk_e2e" {
				return "", io.EOF
			}
			return "owner-e2e", nil
		},
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	t.Setenv(hostEnvVar, srv.URL)
	t.Setenv(apiKeyEnvVar, "choir_sk_e2e")
	return s, srv.URL
}

func runOK(t *testing.T, args ...string) []byte {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("choir %s: exit %d: %s", strings.Join(args, " "), code, stderr.String())
	}
	return stdout.Bytes()
}

func TestPrecommitEndToEndAgainstALocalStore(t *testing.T) {
	ctx := context.Background()
	s, _ := e2eServer(t)
	setIdentity(t, "claude-code/subagent", "claude-sonnet-5")

	// A subagent claims; the lead settles it independently.
	var added struct{ Claim calibration.ClaimView }
	if err := json.Unmarshal(runOK(t, "precommit", "add", "--claim", "go test -race passes on amd64", "--p", "0.95", "--key", "e2e-1"), &added); err != nil {
		t.Fatal(err)
	}
	if added.Claim.Status != calibration.StatusOpen || added.Claim.Model != "claude-sonnet-5" {
		t.Fatalf("added = %+v", added.Claim)
	}
	setIdentity(t, "claude-code/lead", "claude-opus-5-5")
	var settledOut struct{ Claim calibration.ClaimView }
	_ = json.Unmarshal(runOK(t, "precommit", "settle", added.Claim.ID, "--outcome", "false", "--evidence", "CI run 1"), &settledOut)
	if settledOut.Claim.Outcome != "false" || settledOut.Claim.GoverningMethod != "independent" || settledOut.Claim.Brier == nil {
		t.Fatalf("settled = %+v", settledOut.Claim)
	}
	// The same scorer cannot change its settlement.
	var stderr bytes.Buffer
	if code := run([]string{"precommit", "settle", added.Claim.ID, "--outcome", "true", "--evidence", "CI run 2"}, io.Discard, &stderr); code != 1 || !strings.Contains(stderr.String(), "409") {
		t.Fatalf("re-settle exit %d: %s", code, stderr.String())
	}

	// Import the level-1 ledger, twice: the second run writes nothing new.
	// The ledger is a living document, so the expected counts come from the
	// plan of its current rows, plus the one claim and settlement above.
	ledgerClaims, ledgerSettled, ledgerLeadOpen := 0, 0, 0
	for i := 0; i < 2; i++ {
		var applied importPlan
		if err := json.Unmarshal(runOK(t, "precommit", "import-ledger", "--file", ledgerPath(t), "--model", "claude-opus-5-5", "--apply"), &applied); err != nil {
			t.Fatal(err)
		}
		for _, row := range applied.Rows {
			if row.Error != "" {
				t.Fatalf("run %d row %d: %s", i, row.N, row.Error)
			}
		}
		if i == 0 {
			ledgerClaims = len(applied.Rows)
			for _, row := range applied.Rows {
				if row.Settlement != nil {
					ledgerSettled++
				} else if row.Claim.Agent.Name == "claude-code/lead" {
					ledgerLeadOpen++
				}
			}
		}
	}
	if ledgerClaims < 18 || ledgerSettled < 12 {
		t.Fatalf("ledger import planned %d claims and %d settlements; the ledger had 18 and 12 when this test was written", ledgerClaims, ledgerSettled)
	}
	records, err := s.ListCalibrationRecords(ctx, "owner-e2e", "computer-e2e", "", 1000)
	if err != nil {
		t.Fatal(err)
	}
	claims, settlements := 0, 0
	for _, rec := range records {
		if rec.Calibration.Claim != nil {
			claims++
		} else {
			settlements++
		}
	}
	if claims != ledgerClaims+1 || settlements != ledgerSettled+1 {
		t.Fatalf("store holds %d claims and %d settlements; want %d and %d (ledger plus one)", claims, settlements, ledgerClaims+1, ledgerSettled+1)
	}

	// The lead's pasteable track record: the imported rows are reconstructed,
	// so they count only when asked for.
	text := string(runOK(t, "precommit", "calibration", "--text", "--include-reconstructed"))
	if !strings.Contains(text, "agent=claude-code/lead") || !strings.Contains(text, "Brier") {
		t.Fatalf("context text:\n%s", text)
	}
	t.Logf("lead's pasteable track record:\n%s", text)
	var sum calibration.Summary
	_ = json.Unmarshal(runOK(t, "precommit", "calibration", "--all", "--include-reconstructed", "--group-by", "agent"), &sum)
	ledgerOpen := ledgerClaims - ledgerSettled
	if sum.Overall.Settled != ledgerSettled+1 || len(sum.Groups) != 2 || sum.Overall.Open != ledgerOpen {
		t.Fatalf("summary overall = %+v groups = %d", sum.Overall, len(sum.Groups))
	}
	var open calibration.ListResponse
	_ = json.Unmarshal(runOK(t, "precommit", "list", "--status", "open"), &open)
	if len(open.Claims) != ledgerLeadOpen {
		t.Fatalf("lead open claims = %d, want %d", len(open.Claims), ledgerLeadOpen)
	}

	// The same store through the MCP server.
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"precommit_add","arguments":{"claim":"the MCP path works","p":0.7,"type":"forecast"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"calibration_summary","arguments":{"include_reconstructed":true}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"precommit_add","arguments":{"claim":"no p"}}}`,
		`not json`,
		`{"jsonrpc":"2.0","id":6,"method":"resources/list"}`,
		`{"jsonrpc":"2.0","id":7,"method":"ping"}`,
	}, "\n") + "\n"
	cliStdin = strings.NewReader(in)
	defer func() { cliStdin = os.Stdin }()
	var out bytes.Buffer
	if code := run([]string{"mcp"}, &out, io.Discard); code != 0 {
		t.Fatalf("mcp exit %d", code)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 8 { // every request answered; the notification is not
		t.Fatalf("mcp wrote %d lines:\n%s", len(lines), out.String())
	}
	type rpcOut struct {
		ID     json.RawMessage `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	var msgs []rpcOut
	for _, line := range lines {
		var m rpcOut
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("non-JSON on stdout: %q", line)
		}
		msgs = append(msgs, m)
	}
	if !strings.Contains(string(msgs[0].Result), `"protocolVersion":"2025-06-18"`) || !strings.Contains(string(msgs[0].Result), `"tools"`) {
		t.Fatalf("initialize = %s", msgs[0].Result)
	}
	for _, tool := range []string{"precommit_add", "precommit_settle", "precommit_list", "calibration_summary"} {
		if !strings.Contains(string(msgs[1].Result), `"name":"`+tool+`"`) {
			t.Fatalf("tools/list lacks %s: %s", tool, msgs[1].Result)
		}
	}
	if strings.Contains(string(msgs[2].Result), `"isError":true`) || !strings.Contains(string(msgs[2].Result), "calib:claude-code-lead:") {
		t.Fatalf("precommit_add = %s", msgs[2].Result)
	}
	var called struct {
		Content []struct{ Text string } `json:"content"`
	}
	_ = json.Unmarshal(msgs[3].Result, &called)
	if len(called.Content) < 2 || !strings.HasPrefix(called.Content[0].Text, "Calibration track record") {
		t.Fatalf("calibration_summary = %s", msgs[3].Result)
	}
	if !strings.Contains(string(msgs[4].Result), `"isError":true`) || !strings.Contains(string(msgs[4].Result), "p is required") {
		t.Fatalf("tool failure must be an isError result: %s", msgs[4].Result)
	}
	if msgs[5].Error == nil || msgs[5].Error.Code != -32700 {
		t.Fatalf("parse error = %+v", msgs[5])
	}
	if msgs[6].Error == nil || msgs[6].Error.Code != -32601 {
		t.Fatalf("unknown method = %+v", msgs[6])
	}
	if string(msgs[7].ID) != "7" || string(msgs[7].Result) != "{}" {
		t.Fatalf("ping = %+v", msgs[7])
	}
}
