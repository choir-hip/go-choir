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

// Calibration level 2 (docs/reports/choir-mcp-precommitments-2026-10-14.md):
// `choir precommit add|settle|list|calibration|import-ledger` write and read
// an agent's calibrated claims on a Choir computer. Agent identity defaults
// come from $CHOIR_AGENT, $CHOIR_MODEL, $CHOIR_SESSION and $CHOIR_PROJECT.

const (
	agentEnvVar   = "CHOIR_AGENT"
	modelEnvVar   = "CHOIR_MODEL"
	sessionEnvVar = "CHOIR_SESSION"
	projectEnvVar = "CHOIR_PROJECT"
)

// commaList is a repeatable flag whose values may also be comma-separated.
type commaList []string

func (l *commaList) String() string { return strings.Join(*l, ",") }
func (l *commaList) Set(v string) error {
	for _, part := range strings.Split(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			*l = append(*l, part)
		}
	}
	return nil
}

// repeatList is a repeatable flag kept verbatim (evidence refs may contain
// commas).
type repeatList []string

func (l *repeatList) String() string { return strings.Join(*l, "; ") }
func (l *repeatList) Set(v string) error {
	if v = strings.TrimSpace(v); v != "" {
		*l = append(*l, v)
	}
	return nil
}

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

// splitLeadingID lets `settle <id> --flags` and `settle --flags <id>` both
// work: Go's flag parsing stops at the first positional argument.
func splitLeadingID(args []string) (string, []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:]
	}
	return "", args
}

func runPrecommit(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "choir precommit: subcommand required (add|settle|list|calibration|import-ledger)")
		return 2
	}
	switch args[0] {
	case "add":
		return runPrecommitAdd(args[1:], stdout, stderr)
	case "settle":
		return runPrecommitSettle(args[1:], stdout, stderr)
	case "list":
		return runPrecommitList(args[1:], stdout, stderr)
	case "calibration":
		return runPrecommitCalibration(args[1:], stdout, stderr)
	case "import-ledger":
		return runPrecommitImportLedger(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "choir precommit: unknown subcommand %q\n", args[0])
		return 2
	}
}

func runPrecommitAdd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir precommit add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	claim := fs.String("claim", "", "The claim, stated so it can be settled true or false (or pass it as arguments)")
	pRaw := fs.String("p", "", "Probability the claim is true, 0..1 (required)")
	claimType := fs.String("type", "assertion", "assertion | forecast | commitment")
	criterion := fs.String("criterion", "", "How the claim settles (the evidence that will decide it)")
	deadline := fs.String("deadline", "", "RFC 3339 time by which it should be settled")
	project := fs.String("project", envOr(projectEnvVar, ""), "Project the claim belongs to")
	var tags commaList
	fs.Var(&tags, "tag", "Tag (repeatable, or comma-separated)")
	agent := fs.String("agent", envOr(agentEnvVar, ""), "Agent name (default $"+agentEnvVar+")")
	model := fs.String("model", envOr(modelEnvVar, ""), "Agent model id (default $"+modelEnvVar+")")
	session := fs.String("session", envOr(sessionEnvVar, ""), "Session id (default $"+sessionEnvVar+")")
	key := fs.String("key", "", "Idempotency key: a rerun with the same key and claim writes nothing new")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir precommit add: %v\n", err)
		return 2
	}
	text := strings.TrimSpace(*claim)
	if text == "" {
		text = strings.TrimSpace(strings.Join(fs.Args(), " "))
	}
	if text == "" {
		fmt.Fprintln(stderr, "choir precommit add: --claim is required")
		return 2
	}
	if strings.TrimSpace(*pRaw) == "" {
		fmt.Fprintln(stderr, "choir precommit add: --p is required (the probability that the claim is true)")
		return 2
	}
	p, err := strconv.ParseFloat(strings.TrimSpace(*pRaw), 64)
	if err != nil || p < 0 || p > 1 {
		fmt.Fprintln(stderr, "choir precommit add: --p must be a number between 0 and 1")
		return 2
	}
	if strings.TrimSpace(*agent) == "" || strings.TrimSpace(*model) == "" {
		fmt.Fprintf(stderr, "choir precommit add: --agent and --model (or $%s and $%s) are required\n", agentEnvVar, modelEnvVar)
		return 2
	}
	req := calibration.ClaimRequest{
		Agent:     calibration.AgentIdentity{Name: *agent, Model: *model, SessionID: *session},
		Claim:     text,
		Type:      *claimType,
		P:         &p,
		Criterion: *criterion, Deadline: *deadline, Project: *project, Tags: tags,
		IdempotencyKey: *key,
	}
	out, err := precommitAPI{c}.add(req)
	if err != nil {
		fmt.Fprintf(stderr, "choir precommit add: %v\n", err)
		return 1
	}
	return writeJSON(stdout, out)
}

func runPrecommitSettle(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir precommit settle", flag.ContinueOnError)
	fs.SetOutput(stderr)
	outcome := fs.String("outcome", "", "true | false | partial | unresolvable (required)")
	var evidence repeatList
	fs.Var(&evidence, "evidence", "Evidence ref: commit, CI run, staging check, trace, problem doc (repeatable; at least one)")
	method := fs.String("method", "", "self | independent | rule | owner (default: self when you are the claimant, else independent)")
	scorer := fs.String("scorer", "", "Scorer name (default $"+agentEnvVar+"; for rule, the rule's name)")
	scorerModel := fs.String("scorer-model", "", "Scorer model (default $"+modelEnvVar+" when the scorer defaults)")
	note := fs.String("note", "", "Short note on the outcome")
	id, rest := splitLeadingID(args)
	c, err := newClient(fs, rest, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir precommit settle: %v\n", err)
		return 2
	}
	if id == "" && fs.NArg() > 0 {
		id = fs.Arg(0)
	}
	if strings.TrimSpace(id) == "" {
		fmt.Fprintln(stderr, "choir precommit settle: claim id required")
		return 2
	}
	if strings.TrimSpace(*outcome) == "" || len(evidence) == 0 {
		fmt.Fprintln(stderr, "choir precommit settle: --outcome and at least one --evidence are required")
		return 2
	}
	out, err := precommitAPI{c}.settle(id, settlementFromInputs(*outcome, evidence, *method, *scorer, *scorerModel, *note))
	if err != nil {
		fmt.Fprintf(stderr, "choir precommit settle: %v\n", err)
		return 1
	}
	return writeJSON(stdout, out)
}

// readFilterFlags registers the shared claim filters. The agent defaults to
// $CHOIR_AGENT; --all drops it.
func readFilterFlags(fs *flag.FlagSet) func() url.Values {
	agent := fs.String("agent", envOr(agentEnvVar, ""), "Claimant agent (default $"+agentEnvVar+")")
	all := fs.Bool("all", false, "All agents (ignore $"+agentEnvVar+")")
	model := fs.String("model", "", "Claimant model")
	claimType := fs.String("type", "", "assertion | forecast | commitment")
	project := fs.String("project", "", "Project")
	return func() url.Values {
		q := url.Values{}
		if !*all {
			q.Set("agent", strings.TrimSpace(*agent))
		}
		q.Set("model", strings.TrimSpace(*model))
		q.Set("type", strings.TrimSpace(*claimType))
		q.Set("project", strings.TrimSpace(*project))
		return q
	}
}

func runPrecommitList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir precommit list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	filters := readFilterFlags(fs)
	status := fs.String("status", "all", "open (includes overdue) | overdue | settled | all")
	limit := fs.Int("limit", defaultListLimit, "Maximum claims to return")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir precommit list: %v\n", err)
		return 2
	}
	q := filters()
	q.Set("status", strings.TrimSpace(*status))
	q.Set("limit", strconv.Itoa(*limit))
	out, err := precommitAPI{c}.list(q)
	if err != nil {
		fmt.Fprintf(stderr, "choir precommit list: %v\n", err)
		return 1
	}
	return writeJSON(stdout, out)
}

func runPrecommitCalibration(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir precommit calibration", flag.ContinueOnError)
	fs.SetOutput(stderr)
	filters := readFilterFlags(fs)
	scorer := fs.String("scorer", "", "governing (default) | self | independent | rule | owner")
	groupBy := fs.String("group-by", "", "Comma-separated: agent, model, type, project (default agent,model,type)")
	include := fs.Bool("include-reconstructed", false, "Count claims recorded after the fact (level-1 imports)")
	text := fs.Bool("text", false, "Print only the pasteable track record")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir precommit calibration: %v\n", err)
		return 2
	}
	q := filters()
	q.Set("scorer", strings.TrimSpace(*scorer))
	q.Set("group_by", strings.TrimSpace(*groupBy))
	if *include {
		q.Set("include_reconstructed", "true")
	}
	out, err := precommitAPI{c}.calibration(q)
	if err != nil {
		fmt.Fprintf(stderr, "choir precommit calibration: %v\n", err)
		return 1
	}
	if *text {
		var sum struct {
			Context string `json:"context"`
		}
		if err := json.Unmarshal(out, &sum); err != nil {
			fmt.Fprintf(stderr, "choir precommit calibration: decode summary: %v\n", err)
			return 1
		}
		fmt.Fprint(stdout, sum.Context)
		return 0
	}
	return writeJSON(stdout, out)
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
