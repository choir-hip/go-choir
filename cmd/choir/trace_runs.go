package main

// `choir trace runs` lists recent runs with enough to choose one for
// `choir trace run`. It is built on GET /api/runs, which returns the run
// rows only: id, agent, profile, role, state, channel, parent, created and
// finished times. It does not return counts of model calls, tokens or
// searches, so those columns are not here and --sort cannot use them
// (a candidate field for a later, separately reviewed server change).

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
)

const traceRunsHelp = `List recent runs, newest first, with enough to choose one for 'choir trace run'.

Columns: run id, agent, state, started, duration.
Not available (GET /api/runs does not return them; --sort cannot use them):
model calls, tokens, searches, cells, errors. Read them for one run with
'choir trace run <id>'. --sort accepts started or duration; --desk matches the
agent id prefix, profile or role (applied to the newest 500 runs the route
returns); --limit caps the list. Read-only; needs read:runtime.`

type traceRunsEntry struct {
	RunID      string `json:"run_id"`
	AgentID    string `json:"agent_id"`
	Profile    string `json:"agent_profile,omitempty"`
	Role       string `json:"agent_role,omitempty"`
	State      string `json:"state"`
	StartedAt  string `json:"started_at"`
	EndedAt    string `json:"ended_at,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	// Live says the duration runs to now.
	Live      bool   `json:"live,omitempty"`
	ChannelID string `json:"channel_id,omitempty"`
	ParentRun string `json:"parent_run_id,omitempty"`
}

type traceRunsDoc struct {
	Schema   string           `json:"schema"`
	Searched int              `json:"searched"`
	Runs     []traceRunsEntry `json:"runs"`
}

func runTraceRuns(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir trace runs", flag.ContinueOnError)
	fs.SetOutput(stderr)
	desk := fs.String("desk", "", "Only runs of this desk (agent id prefix, profile or role)")
	limit := fs.Int("limit", 20, "How many runs to list (at most 500)")
	sortBy := fs.String("sort", "started", "started or duration (searches and calls are not available from /api/runs)")
	asJSON := fs.Bool("json", false, "Print JSON")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: choir trace runs [flags]\n\n%s\n\nFlags:\n", traceRunsHelp)
		fs.PrintDefaults()
	}
	flagArgs, positional := splitTraceArgsWith(args, map[string]bool{
		"api-key-file": true, "host": true, "computer": true, "timeout": true, "desk": true, "limit": true, "sort": true,
	})
	c, err := newClient(fs, flagArgs, stdout, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "choir trace runs: %v\n", err)
		return 2
	}
	if len(positional)+len(fs.Args()) > 0 {
		fmt.Fprintln(stderr, "choir trace runs: no positional arguments")
		return 2
	}
	switch *sortBy {
	case "started", "duration":
	case "searches", "calls":
		fmt.Fprintf(stderr, "choir trace runs: --sort %s is not available: GET /api/runs does not return counts of model calls or searches. Use --sort started or duration, then 'choir trace run <id>' to read one run's counts.\n", *sortBy)
		return 2
	default:
		fmt.Fprintf(stderr, "choir trace runs: unknown --sort %q (want started or duration)\n", *sortBy)
		return 2
	}
	if *limit < 1 || *limit > traceRunChildListCap {
		fmt.Fprintf(stderr, "choir trace runs: --limit must be between 1 and %d\n", traceRunChildListCap)
		return 2
	}
	fetch := *limit
	if strings.TrimSpace(*desk) != "" {
		fetch = traceRunChildListCap
	}
	f := &traceRunFetcher{c: c, errw: stderr}
	path := fmt.Sprintf("/api/runs?limit=%d", fetch)
	var list struct {
		Runs []traceRunRow `json:"runs"`
	}
	if err := f.get(path, &list); err != nil {
		if st := authStatus(err); st != 0 {
			fmt.Fprintf(stderr, "choir trace runs: %s\n", authMessage(st, "/api/runs"))
			return 1
		}
		fmt.Fprintf(stderr, "choir trace runs: %s\n", f.failText(path, err))
		return 1
	}
	now := traceRunNow().UTC()
	var entries []traceRunsEntry
	for _, r := range list.Runs {
		if d := strings.TrimSpace(*desk); d != "" && !deskMatches(r, d) {
			continue
		}
		e := traceRunsEntry{RunID: firstNonEmptyStr(r.RunID, r.LoopID), AgentID: r.AgentID, Profile: r.AgentProfile, Role: r.AgentRole,
			State: r.State, StartedAt: r.CreatedAt, ChannelID: r.ChannelID, ParentRun: r.RequestedByRunID}
		start := parseTraceTime(r.CreatedAt)
		end := now
		if r.FinishedAt != nil && *r.FinishedAt != "" {
			e.EndedAt = *r.FinishedAt
			end = parseTraceTime(*r.FinishedAt)
		} else if !terminalStates[r.State] {
			e.Live = true
		} else if u := parseTraceTime(r.UpdatedAt); !u.IsZero() {
			end = u
		}
		if !start.IsZero() && !end.IsZero() {
			e.DurationMs = end.Sub(start).Milliseconds()
		}
		entries = append(entries, e)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if *sortBy == "duration" && a.DurationMs != b.DurationMs {
			return a.DurationMs > b.DurationMs
		}
		if ta, tb := parseTraceTime(a.StartedAt), parseTraceTime(b.StartedAt); !ta.Equal(tb) {
			return ta.After(tb)
		}
		return a.RunID < b.RunID
	})
	if len(entries) > *limit {
		entries = entries[:*limit]
	}
	if entries == nil {
		entries = []traceRunsEntry{}
	}
	if *asJSON {
		return writeJSON(stdout, traceRunsDoc{Schema: "choir.trace_runs.v1", Searched: len(list.Runs), Runs: entries})
	}
	fmt.Fprintf(stdout, "%-38s %-30s %-10s %-20s %s\n", "RUN", "AGENT", "STATE", "STARTED", "DURATION")
	for _, e := range entries {
		live := ""
		if e.Live {
			live = " (live)"
		}
		started := e.StartedAt
		if t := parseTraceTime(e.StartedAt); !t.IsZero() {
			started = t.Format("2006-01-02 15:04:05Z")
		}
		fmt.Fprintf(stdout, "%-38s %-30s %-10s %-20s %s%s\n", e.RunID, e.AgentID, e.State, started, fmtDur(e.DurationMs), live)
	}
	fmt.Fprintf(stdout, "\n%d of %d runs the route returned (newest first by the route). For a run's counts, use: choir trace run <id>.\n", len(entries), len(list.Runs))
	return 0
}

func deskMatches(r traceRunRow, desk string) bool {
	desk = strings.ToLower(desk)
	return strings.HasPrefix(strings.ToLower(r.AgentID), desk+":") || strings.EqualFold(r.AgentProfile, desk) || strings.EqualFold(r.AgentRole, desk)
}
