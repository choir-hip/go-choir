package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/calibration"
)

// Calibration level 2 (docs/archive/reports/choir-mcp-precommitments-2026-10-14.md):
// `choir precommit add|settle|list|calibration|import-ledger` write and read
// an agent's calibrated claims on a Choir computer. Agent identity defaults
// come from $CHOIR_AGENT, $CHOIR_MODEL, $CHOIR_SESSION and $CHOIR_PROJECT.

const (
	agentEnvVar   = "CHOIR_AGENT"
	modelEnvVar   = "CHOIR_MODEL"
	sessionEnvVar = "CHOIR_SESSION"
	projectEnvVar = "CHOIR_PROJECT"
)

// precommitAPI is the HTTP surface the CLI and the MCP server share.
type precommitAPI struct{ c *client }

func (p precommitAPI) add(req calibration.ClaimRequest) (json.RawMessage, error) {
	var out json.RawMessage
	err := p.c.do(http.MethodPost, "/api/precommit/claims", req, &out)
	return out, err
}

func (p precommitAPI) settle(claimID string, req calibration.SettlementRequest) (json.RawMessage, error) {
	var out json.RawMessage
	err := p.c.do(http.MethodPost, "/api/precommit/claims/"+url.PathEscape(strings.TrimSpace(claimID))+"/settlements", req, &out)
	return out, err
}

func (p precommitAPI) list(q url.Values) (json.RawMessage, error) {
	var out json.RawMessage
	err := p.c.do(http.MethodGet, "/api/precommit/claims"+encodeQuery(q), nil, &out)
	return out, err
}

func (p precommitAPI) calibration(q url.Values) (json.RawMessage, error) {
	var out json.RawMessage
	err := p.c.do(http.MethodGet, "/api/precommit/calibration"+encodeQuery(q), nil, &out)
	return out, err
}

func encodeQuery(q url.Values) string {
	for k, v := range q {
		if len(v) == 0 || strings.TrimSpace(v[0]) == "" {
			delete(q, k)
		}
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// settlementFromInputs builds a settlement request. With no scorer named,
// the scorer is the calling agent ($CHOIR_AGENT, with $CHOIR_MODEL), and
// the server decides self (same name as the claimant) or independent.
func settlementFromInputs(outcome string, evidence []string, method, scorer, scorerModel, note string) calibration.SettlementRequest {
	method, scorer, scorerModel = strings.TrimSpace(method), strings.TrimSpace(scorer), strings.TrimSpace(scorerModel)
	if scorer == "" && method != "rule" && method != "owner" {
		scorer = envOr(agentEnvVar, "")
		if scorerModel == "" {
			scorerModel = envOr(modelEnvVar, "")
		}
	}
	return calibration.SettlementRequest{
		Outcome:  strings.TrimSpace(outcome),
		Evidence: evidence,
		Scorer:   calibration.ScorerIdentity{Method: method, Name: scorer, Model: scorerModel},
		Note:     strings.TrimSpace(note),
	}
}

// ---- import-ledger: the level-1 Markdown ledger into a computer ----

type importRow struct {
	N          int                            `json:"n"`
	Claim      calibration.ClaimRequest       `json:"claim"`
	Settlement *calibration.SettlementRequest `json:"settlement,omitempty"`
	ClaimID    string                         `json:"claim_id,omitempty"`
	Error      string                         `json:"error,omitempty"`
}

type importSkip struct {
	Line   int    `json:"line"`
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

type importPlan struct {
	Apply   bool         `json:"apply"`
	File    string       `json:"file"`
	Rows    []importRow  `json:"rows"`
	Skipped []importSkip `json:"skipped"`
}

type ledgerOptions struct {
	file, model, agentPrefix, project string
}

var ledgerDate = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

func tableCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	cells := strings.Split(line, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

// parseLedgerOutcome reads a settled row's outcome cell ("false in
// production: ...", "true: ...", "partly: ...").
func parseLedgerOutcome(cell string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(cell))
	switch {
	case strings.HasPrefix(lower, "true"):
		return "true", true
	case strings.HasPrefix(lower, "false"):
		return "false", true
	case strings.HasPrefix(lower, "partly"), strings.HasPrefix(lower, "partial"):
		return "partial", true
	case strings.HasPrefix(lower, "unresolv"):
		return "unresolvable", true
	}
	return "", false
}

// parseLedger turns the level-1 ledger's two tables into planned writes.
// Settled rows were reconstructed after the fact: they are marked so, with
// the section's date as stated_at, and settled by the lead (self for the
// lead's own claims, independent for subagents'). Open rows were recorded
// before settlement and become live forecasts. Anything unparseable is
// skipped with its reason, never guessed.
func parseLedger(text string, opts ledgerOptions) ([]importRow, []importSkip) {
	var rows []importRow
	skipped := []importSkip{}
	section, date := "", ""
	lead := opts.agentPrefix + "/lead"
	for i, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			switch {
			case strings.HasPrefix(trimmed, "## Settled claims"):
				section, date = "settled", ledgerDate.FindString(trimmed)
			case strings.HasPrefix(trimmed, "## Open claims"):
				section = "open"
			default:
				section = ""
			}
			continue
		}
		if section == "" || !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cells := tableCells(trimmed)
		n, err := strconv.Atoi(cells[0])
		if err != nil {
			continue // header or separator
		}
		skip := func(reason string) { skipped = append(skipped, importSkip{Line: i + 1, Reason: reason, Text: trimmed}) }
		want := 5
		if section == "settled" {
			want = 6
		}
		if len(cells) != want {
			skip(fmt.Sprintf("expected %d cells, found %d", want, len(cells)))
			continue
		}
		who := strings.ToLower(cells[1])
		var agent, role string
		switch {
		case who == "lead":
			agent = lead
		case strings.HasPrefix(who, "subagent"):
			agent = opts.agentPrefix + "/subagent"
			if lp, rp := strings.Index(cells[1], "("), strings.LastIndex(cells[1], ")"); lp >= 0 && rp > lp {
				role = strings.TrimSpace(cells[1][lp+1 : rp])
			}
		default:
			skip("unknown agent " + cells[1])
			continue
		}
		p, err := strconv.ParseFloat(cells[3], 64)
		if err != nil || p < 0 || p > 1 {
			skip("probability is not a number in [0,1]: " + cells[3])
			continue
		}
		row := importRow{N: n, Claim: calibration.ClaimRequest{
			Agent:          calibration.AgentIdentity{Name: agent, Model: opts.model},
			Claim:          cells[2],
			Type:           "assertion",
			P:              &p,
			Project:        opts.project,
			IdempotencyKey: "claude-ledger:" + strconv.Itoa(n),
			SourceRef:      opts.file + "#" + strconv.Itoa(n),
		}}
		if role != "" {
			row.Claim.Tags = []string{role}
		}
		if section == "open" {
			row.Claim.Type = "forecast"
			row.Claim.Criterion = cells[4]
			rows = append(rows, row)
			continue
		}
		outcome, ok := parseLedgerOutcome(cells[4])
		if !ok {
			skip("outcome is not true/false/partly/unresolvable: " + cells[4])
			continue
		}
		if cells[5] == "" {
			skip("settled row cites no evidence")
			continue
		}
		row.Claim.Reconstructed = true
		if date != "" {
			row.Claim.StatedAt = date + "T00:00:00Z"
		}
		method := "independent"
		if agent == lead {
			method = "self"
		}
		row.Settlement = &calibration.SettlementRequest{
			Outcome:  outcome,
			Evidence: []string{cells[5]},
			Scorer:   calibration.ScorerIdentity{Method: method, Name: lead, Model: opts.model},
			Note:     cells[4],
		}
		rows = append(rows, row)
	}
	return rows, skipped
}

func hasFlagArg(args []string, name string) bool {
	for _, a := range args {
		a = strings.TrimLeft(a, "-")
		if a == name || strings.HasPrefix(a, name+"=") && !strings.HasSuffix(a, "=false") {
			return true
		}
	}
	return false
}

func runPrecommitImportLedger(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir precommit import-ledger", flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("file", "docs/calibration/claude-ledger.md", "The level-1 Markdown ledger")
	model := fs.String("model", "", "Model the ledger's claims were made on (required with --apply)")
	prefix := fs.String("agent-prefix", "claude-code", "Agent names become <prefix>/lead and <prefix>/subagent")
	project := fs.String("project", "go-choir", "Project for the imported claims")
	apply := fs.Bool("apply", false, "Write to the computer (default: dry run that prints the plan)")
	var c *client
	if hasFlagArg(args, "apply") {
		var err error
		if c, err = newClient(fs, args, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "choir precommit import-ledger: %v\n", err)
			return 2
		}
	} else {
		// A dry run needs no key and never dials; accept the client flags.
		fs.String("api-key-file", "", "unused in a dry run")
		fs.String("host", "", "unused in a dry run")
		fs.String("computer", "", "unused in a dry run")
		fs.String("timeout", "", "unused in a dry run")
		if err := fs.Parse(args); err != nil {
			return 2
		}
	}
	if *apply && strings.TrimSpace(*model) == "" {
		fmt.Fprintln(stderr, "choir precommit import-ledger: --apply requires --model (calibration is per model)")
		return 2
	}
	raw, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(stderr, "choir precommit import-ledger: %v\n", err)
		return 2
	}
	rows, skipped := parseLedger(string(raw), ledgerOptions{file: *file, model: strings.TrimSpace(*model), agentPrefix: *prefix, project: *project})
	plan := importPlan{Apply: *apply, File: *file, Rows: rows, Skipped: skipped}
	if !*apply || c == nil {
		plan.Apply = false
		return writeJSON(stdout, plan)
	}
	api := precommitAPI{c}
	failed := false
	for i := range plan.Rows {
		row := &plan.Rows[i]
		out, err := api.add(row.Claim)
		if err != nil {
			row.Error, failed = "claim: "+err.Error(), true
			continue
		}
		var created struct {
			Claim struct {
				ID string `json:"id"`
			} `json:"claim"`
		}
		if err := json.Unmarshal(out, &created); err != nil || created.Claim.ID == "" {
			row.Error, failed = "claim: no id in response", true
			continue
		}
		row.ClaimID = created.Claim.ID
		if row.Settlement != nil {
			if _, err := api.settle(row.ClaimID, *row.Settlement); err != nil {
				row.Error, failed = "settlement: "+err.Error(), true
			}
		}
	}
	if code := writeJSON(stdout, plan); code != 0 {
		return code
	}
	if failed {
		return 1
	}
	return 0
}
