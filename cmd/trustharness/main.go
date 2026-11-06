// Command trustharness is the evidence harness for the paper (modular
// architecture plan MA7.9): it reads one computer's instrumented records
// through the owner's product API (never SSH), stores the reduced inputs, and
// writes the pre-registered tests of MA0.6 and the mission's own ledger
// scores as a dated JSON and markdown artifact.
//
// Fetch and compute (read-only; every request names its computer):
//
//	go run ./cmd/trustharness -computer computer-... [-base https://choir.news]
//	    [-key-file ~/.config/choir-qa/keys/d-e.key] [-out DIR]
//
// Reproduce from stored inputs, byte for byte:
//
//	go run ./cmd/trustharness -from DIR -check
//
// Run it against d@e only, never the paper or the owner's computers: drills
// and replays are test-computer work (plan MA7.9; pre-registration section 1).
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/trust"
	"github.com/yusefmosiah/go-choir/internal/trustharness"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	defaultLedger        = "docs/definitions/choir-modular-architecture-2026-11-04.precommit.md"
	defaultTranscription = "docs/evidence/ma7-9-harness/mission-ledger-settlements.json"
	defaultEvidenceRoot  = "docs/evidence/ma7-9-harness"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "trustharness:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("trustharness", flag.ContinueOnError)
	base := fs.String("base", "https://choir.news", "origin of the product API")
	home, _ := os.UserHomeDir()
	keyFile := fs.String("key-file", filepath.Join(home, ".config/choir-qa/keys/d-e.key"), "file holding the API key (never printed)")
	computer := fs.String("computer", "", "the computer to read (required to fetch); the response must name the same computer")
	ledger := fs.String("ledger", defaultLedger, "the mission's precommitment ledger")
	trans := fs.String("transcription", defaultTranscription, "the ledger settlement transcription")
	out := fs.String("out", "", "output directory (default docs/evidence/ma7-9-harness/<date>-<computer>)")
	from := fs.String("from", "", "recompute from DIR/inputs.json instead of fetching")
	check := fs.Bool("check", false, "with -from: compare the recomputed bytes with DIR/report.json and DIR/report.md and write nothing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *from != "" {
		return reproduce(*from, *check, stdout)
	}
	if *check {
		return errors.New("-check needs -from")
	}
	if strings.TrimSpace(*computer) == "" {
		return errors.New("-computer is required: name the test computer (d@e); never the paper or an owner's computer")
	}
	keyBytes, err := os.ReadFile(*keyFile)
	if err != nil {
		return fmt.Errorf("read key file: %w", err)
	}
	c := client{base: strings.TrimRight(*base, "/"), key: strings.TrimSpace(string(keyBytes)), computer: strings.TrimSpace(*computer), http: &http.Client{Timeout: 2 * time.Minute}}
	in, err := fetch(c)
	if err != nil {
		return err
	}
	in.Ledger, err = loadLedger(*ledger, *trans)
	if err != nil {
		return err
	}
	dir := *out
	if dir == "" {
		dir = filepath.Join(defaultEvidenceRoot, in.FetchedAt.Format("2006-01-02")+"-"+strings.TrimPrefix(in.ComputerID, "computer-")[:8])
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ib, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "inputs.json"), append(ib, '\n'), 0o644); err != nil {
		return err
	}
	return write(dir, in, stdout)
}

func loadLedger(path, transPath string) (trustharness.LedgerInput, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return trustharness.LedgerInput{}, fmt.Errorf("read ledger: %w", err)
	}
	var trans []trustharness.Transcription
	if transPath != "" {
		raw, err := os.ReadFile(transPath)
		if err != nil {
			return trustharness.LedgerInput{}, fmt.Errorf("read transcription: %w", err)
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&trans); err != nil {
			return trustharness.LedgerInput{}, fmt.Errorf("decode transcription: %w", err)
		}
	}
	return trustharness.ParseLedger(filepath.ToSlash(path), string(text), trans), nil
}

func write(dir string, in trustharness.Inputs, stdout io.Writer) error {
	jb, md, err := trustharness.Render(trustharness.Compute(in))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "report.json"), jb, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "report.md"), md, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %s/{inputs.json,report.json,report.md}\n", dir)
	return nil
}

func reproduce(dir string, check bool, stdout io.Writer) error {
	raw, err := os.ReadFile(filepath.Join(dir, "inputs.json"))
	if err != nil {
		return err
	}
	var in trustharness.Inputs
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return fmt.Errorf("decode inputs: %w", err)
	}
	if in.Schema != trustharness.InputsSchema {
		return fmt.Errorf("inputs schema %q, want %q", in.Schema, trustharness.InputsSchema)
	}
	if !check {
		return write(dir, in, stdout)
	}
	jb, md, err := trustharness.Render(trustharness.Compute(in))
	if err != nil {
		return err
	}
	for name, want := range map[string][]byte{"report.json": jb, "report.md": md} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		if !bytes.Equal(got, want) {
			return fmt.Errorf("%s differs from the recomputed bytes", name)
		}
	}
	fmt.Fprintf(stdout, "reproduced %s/report.json and report.md byte for byte\n", dir)
	return nil
}

type client struct {
	base, key, computer string
	http                *http.Client
}

func (c client) get(path string, v any) error {
	req, err := http.NewRequest(http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("X-Choir-Computer", c.computer)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s: %s", path, resp.Status, strings.TrimSpace(string(body[:min(len(body), 300)])))
	}
	return json.Unmarshal(body, v)
}

type trustResp struct {
	ComputerID string `json:"computer_id"`
	View       struct {
		Rows []json.RawMessage `json:"rows"`
	} `json:"view"`
	Replay      *trust.ReplayReport     `json:"replay"`
	GateReplay  *trust.GateReport       `json:"gate_replay"`
	PreregUnits *trust.PreregUnitSet    `json:"prereg_units"`
	GatedActs   []trustharness.GatedAct `json:"gated_acts"`
}

type subcallsResp struct {
	ComputerID string `json:"computer_id"`
	Summary    struct {
		ByPlane map[string]map[string]int `json:"settlement_by_plane"`
	} `json:"summary"`
	Inert struct {
		Rates     []types.InertRateRow      `json:"rates"`
		Agreement types.InertJudgeAgreement `json:"judge_agreement"`
	} `json:"inert"`
}

type observabilityResp struct {
	ComputerID string `json:"computer_id"`
	Build      struct {
		Commit         string `json:"commit"`
		DeployedCommit string `json:"deployed_commit"`
	} `json:"build"`
	ActGate *struct {
		Counts map[string]int64 `json:"counts"`
	} `json:"act_gate"`
}

func fetch(c client) (trustharness.Inputs, error) {
	in := trustharness.Inputs{Schema: trustharness.InputsSchema, FetchedAt: time.Now().UTC().Truncate(time.Second), Served: map[string]bool{},
		Leases: []trustharness.LeaseObs{}, Drills: []trustharness.Drill{}, InertRates: []types.InertRateRow{}}
	var obs observabilityResp
	if err := c.get("/api/runtime/observability", &obs); err != nil {
		return in, err
	}
	var tr trustResp
	if err := c.get("/api/trust?replay=1", &tr); err != nil {
		return in, err
	}
	var sc subcallsResp
	if err := c.get("/api/subcalls?limit=1", &sc); err != nil {
		return in, err
	}
	var ls struct {
		Leases []trustharness.LeaseObs `json:"leases"`
	}
	if err := c.get("/api/leases", &ls); err != nil {
		return in, err
	}
	for name, id := range map[string]string{"observability": obs.ComputerID, "trust": tr.ComputerID, "subcalls": sc.ComputerID} {
		if id != c.computer {
			return in, fmt.Errorf("%s answered for computer %q, not the named %q", name, id, c.computer)
		}
	}
	in.ComputerID = c.computer
	in.BuildCommit = obs.Build.DeployedCommit
	if in.BuildCommit == "" {
		in.BuildCommit = obs.Build.Commit
	}
	in.ViewRows = len(tr.View.Rows)
	in.Replay, in.GateReplay, in.PreregUnits = tr.Replay, tr.GateReplay, tr.PreregUnits
	served := tr.PreregUnits != nil
	in.Served["prereg_units"], in.Served["gated_acts"], in.Served["gate_replay_by_day"] = served, served, served
	if served {
		in.GatedActs = tr.GatedActs
		if in.GatedActs == nil {
			in.GatedActs = []trustharness.GatedAct{}
		}
	}
	if sc.Inert.Rates != nil {
		in.InertRates = sc.Inert.Rates
	}
	in.InertAgreement, in.SettlementsByPlane = sc.Inert.Agreement, sc.Summary.ByPlane
	if ls.Leases != nil {
		in.Leases = ls.Leases
	}
	if obs.ActGate != nil {
		in.ActGateCounts = obs.ActGate.Counts
	}
	return in, nil
}
