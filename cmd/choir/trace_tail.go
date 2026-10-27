package main

// `choir trace tail <trajectory-id>` is the verbose watcher: it follows one
// trajectory by cursor over the product routes `choir trace export` reads
// (lifecycle events, the trajectory's runs and their run events, the
// trajectory snapshot, the self-development operations listing) and prints
// one line per event. It is read-only and needs no server change. The export
// is a snapshot; this is the same data as a stream, with the terminal states
// an orchestrator waits on turned into distinct exit codes.

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

// Exit codes. 0 is the one success: a candidate is waiting on the owner.
const (
	tailExitRestart = 10 // an assignment was cancelled interrupted_by_restart
	tailExitRevoked = 11 // a capsule was revoked and no new assignment opened within --grace
	tailExitOwner   = 12 // a Texture turn says it waits on the owner or requester
	tailExitSettled = 13 // the trajectory settled
	tailExitStopped = 20 // --for elapsed or interrupted (no terminal state reached)

	tailPageLimit   = 1000
	tailRunListCap  = 500
	tailMinInterval = time.Second
)

// tailTerminalNames are the --until names, in the order the help lists them.
var tailTerminalNames = []string{"candidate", "restart", "revoked", "owner", "settled"}

var tailTerminalCodes = map[string]int{
	"candidate": 0, "restart": tailExitRestart, "revoked": tailExitRevoked,
	"owner": tailExitOwner, "settled": tailExitSettled,
}

const tailHelp = `Follow one trajectory and print one line per event.

Read-only; uses the routes 'choir trace export' reads. Starts at the current
head of the trajectory (use --history to print what happened before), polls
every --interval, and never exits on a transient read error: it prints a
'read failed' line and keeps going.

Levels: -v (default) turns, assignments, capsule fates, run terminalizations;
-vv adds tool calls (first code line, choir.Start job argv, exit codes);
-vvv adds truncated tool outputs and loop progress. --json prints JSONL.

Exit codes:
  0   a self-development candidate of this trajectory reached awaiting_approval
  1   fatal: the first read was refused (bad key, wrong computer) or unreadable
  2   usage error
  10  an assignment was cancelled interrupted_by_restart
  11  a capsule was revoked and no new assignment opened within --grace
  12  a Texture turn says it waits on the owner or requester
  13  the trajectory settled
  20  --for elapsed, or interrupted, with no terminal state reached
--until restricts which terminal states end the tail (comma list of: ` + "candidate, restart, revoked, owner, settled" + `);
the others are printed as ignored and the tail continues.`

// tailHooks lets tests drive the clock and the wait between polls.
type tailHooks struct {
	now   func() time.Time
	sleep func(time.Duration) bool // false stops the tail
}

func runTraceTailCLI(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runTraceTail(args, stdout, stderr, &tailHooks{
		now: time.Now,
		sleep: func(d time.Duration) bool {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return false
			case <-timer.C:
				return true
			}
		},
	})
}

// levelFromArgs reads -v, -vv and -vvv (the default and the minimum is 1).
func levelFromArgs(args []string) int {
	level := 1
	for _, a := range args {
		if a == "--" {
			break
		}
		if len(a) >= 2 && a[0] == '-' && a[1] == 'v' && strings.Trim(a[1:], "v") == "" {
			if n := len(a) - 1; n > level {
				level = n
			}
		}
	}
	if level > 3 {
		level = 3
	}
	return level
}

func stripLevelArgs(args []string) []string {
	var out []string
	for i, a := range args {
		if a == "--" {
			out = append(out, args[i:]...)
			break
		}
		if len(a) >= 2 && a[0] == '-' && a[1] == 'v' && strings.Trim(a[1:], "v") == "" {
			continue
		}
		out = append(out, a)
	}
	return out
}

func runTraceTail(args []string, stdout, stderr io.Writer, hooks *tailHooks) int {
	fs := flag.NewFlagSet("choir trace tail", flag.ContinueOnError)
	fs.SetOutput(stderr)
	interval := fs.Duration("interval", 5*time.Second, "Poll interval (floor 1s)")
	grace := fs.Duration("grace", 120*time.Second, "How long a revoked capsule may go without a new assignment before the tail ends (exit 11)")
	until := fs.String("until", "", "Only these terminal states end the tail (comma list of: candidate, restart, revoked, owner, settled); default all")
	history := fs.Bool("history", false, "Also print events from before the tail started")
	asJSON := fs.Bool("json", false, "Print JSONL instead of human lines")
	maxFor := fs.Duration("for", 0, "Stop after this long (exit 20); 0 follows until a terminal state")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: choir trace tail [flags] <trajectory-id>   (-v, -vv, -vvv set the level)\n\n%s\n\nFlags:\n", tailHelp)
		fs.PrintDefaults()
	}
	level := levelFromArgs(args)
	flagArgs, positional := splitTraceArgsWith(stripLevelArgs(args), map[string]bool{
		"api-key-file": true, "host": true, "computer": true, "timeout": true,
		"interval": true, "grace": true, "until": true, "for": true,
	})
	c, err := newClient(fs, flagArgs, stdout, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "choir trace tail: %v\n", err)
		return 2
	}
	positional = append(positional, fs.Args()...)
	if len(positional) != 1 || strings.TrimSpace(positional[0]) == "" {
		fmt.Fprintln(stderr, "choir trace tail: exactly one trajectory id required")
		return 2
	}
	if *interval < tailMinInterval {
		fmt.Fprintf(stderr, "choir trace tail: --interval must be at least %s\n", tailMinInterval)
		return 2
	}
	if *grace < 0 || *maxFor < 0 {
		fmt.Fprintln(stderr, "choir trace tail: --grace and --for must not be negative")
		return 2
	}
	allowed := map[string]bool{}
	if strings.TrimSpace(*until) == "" {
		for _, n := range tailTerminalNames {
			allowed[n] = true
		}
	} else {
		for _, n := range strings.Split(*until, ",") {
			n = strings.TrimSpace(n)
			if _, ok := tailTerminalCodes[n]; !ok {
				fmt.Fprintf(stderr, "choir trace tail: unknown --until state %q (want %s)\n", n, strings.Join(tailTerminalNames, ", "))
				return 2
			}
			allowed[n] = true
		}
	}
	if hooks == nil {
		hooks = &tailHooks{now: time.Now, sleep: func(d time.Duration) bool { time.Sleep(d); return true }}
	}
	t := &tailer{
		c: c, traj: strings.TrimSpace(positional[0]), level: level, asJSON: *asJSON, history: *history,
		interval: *interval, grace: *grace, allowed: allowed, maxFor: *maxFor,
		out: stdout, errw: stderr, hooks: hooks,
		runs: map[string]*tailRunState{}, pendingRevoke: map[string]time.Time{}, seen: map[string]bool{},
	}
	return t.run()
}

type tailRunState struct {
	cursor  int64
	hist    bool // the run existed when the tail started: its first drain is history
	drained bool
	done    bool // terminal state seen and drained afterwards
}

type tailer struct {
	c        *client
	traj     string
	level    int
	asJSON   bool
	history  bool
	interval time.Duration
	grace    time.Duration
	maxFor   time.Duration
	allowed  map[string]bool
	out      io.Writer
	errw     io.Writer
	hooks    *tailHooks

	tick          int
	lifeCursor    int64
	lifeDrained   bool
	runs          map[string]*tailRunState
	seen          map[string]bool
	computerID    string
	hadRead       bool
	failStreak    int
	opsDisabled   bool
	pendingRevoke map[string]time.Time
	settledShown  bool
}

// tailEvent is one printable, ordered event.
type tailEvent struct {
	when    time.Time
	source  string // lifecycle | run | tail
	seq     int64
	agent   string
	kind    string
	summary string
	level   int
	raw     json.RawMessage
}

type tailTerminal struct {
	name    string
	summary string
}

type tailRoute struct {
	err    error
	status int
	hard   bool // transport failure or 5xx: stop the rest of this poll
}

func (t *tailer) run() int {
	var deadline time.Time
	if t.maxFor > 0 {
		deadline = t.hooks.now().Add(t.maxFor)
	}
	for {
		code, done := t.poll()
		if done {
			return code
		}
		t.tick++
		if !deadline.IsZero() && !t.hooks.now().Before(deadline) {
			return tailExitStopped
		}
		wait := t.interval
		if !deadline.IsZero() {
			if left := deadline.Sub(t.hooks.now()); left < wait {
				wait = left
			}
		}
		if !t.hooks.sleep(wait) {
			return tailExitStopped
		}
		if !deadline.IsZero() && !t.hooks.now().Before(deadline) {
			// One last poll so the final seconds are not lost.
			if code, done := t.poll(); done {
				return code
			}
			return tailExitStopped
		}
	}
}

func (t *tailer) get(path string, out any) *tailRoute {
	err := t.c.do(http.MethodGet, path, nil, out)
	if err == nil {
		return nil
	}
	r := &tailRoute{err: fmt.Errorf("GET %s: %w", redactTailPath(path), err), hard: true}
	var apiErr *apiErrorResp
	if errors.As(err, &apiErr) {
		r.status = apiErr.Status
		r.hard = apiErr.Status >= 500
	}
	return r
}

func redactTailPath(p string) string {
	if i := strings.Index(p, "?"); i >= 0 {
		return p[:i]
	}
	return p
}

// poll reads every source once, prints the new events and evaluates the
// terminal states. It returns the exit code when the tail is done.
func (t *tailer) poll() (int, bool) {
	var events []tailEvent
	var terminals []tailTerminal
	var failures []*tailRoute
	fail := func(r *tailRoute) bool {
		failures = append(failures, r)
		return r.hard
	}
	abort := false

	// Lifecycle events.
	if !abort {
		evs, terms, r := t.pollLifecycle()
		events = append(events, evs...)
		terminals = append(terminals, terms...)
		if r != nil {
			if !t.hadRead && (r.status == http.StatusUnauthorized || r.status == http.StatusForbidden || r.status == http.StatusNotFound) {
				fmt.Fprintf(t.errw, "choir trace tail: %v\n", r.err)
				return 1, true
			}
			abort = fail(r)
		} else {
			t.hadRead = true
		}
	}
	// Runs and run events.
	if !abort {
		evs, r := t.pollRuns()
		events = append(events, evs...)
		if r != nil {
			abort = fail(r)
		}
	}
	// Trajectory snapshot: settled?
	settled := false
	if !abort {
		var snap struct {
			Trajectory struct {
				Status string `json:"status"`
			} `json:"trajectory"`
		}
		if r := t.get("/api/trajectories/"+url.PathEscape(t.traj), &snap); r != nil {
			abort = fail(r)
		} else if snap.Trajectory.Status == "settled" {
			settled = true
		}
	}
	// Candidates awaiting approval.
	var candidate string
	if !abort && !t.opsDisabled && strings.TrimSpace(t.computerID) != "" {
		var list struct {
			Operations []struct {
				OperationID  string `json:"operation_id"`
				TrajectoryID string `json:"trajectory_id"`
				State        string `json:"state"`
			} `json:"operations"`
		}
		path := selfDevPath(t.computerID, "operations") + "?state=awaiting_approval"
		if r := t.get(path, &list); r != nil {
			if r.status == http.StatusForbidden || r.status == http.StatusNotFound || r.status == http.StatusUnauthorized {
				t.opsDisabled = true
				events = append(events, t.noteEvent(fmt.Sprintf("candidate detection unavailable (%v)", r.err), "candidate_unavailable"))
			} else {
				abort = fail(r)
			}
		} else {
			for _, op := range list.Operations {
				if op.TrajectoryID == t.traj && op.State == "awaiting_approval" {
					candidate = op.OperationID
					break
				}
			}
		}
	}

	sort.SliceStable(events, func(i, j int) bool { return events[i].when.Before(events[j].when) })
	for _, ev := range events {
		t.print(ev)
	}
	t.reportReads(failures)

	now := t.hooks.now()
	if t.grace >= 0 {
		for agent, since := range t.pendingRevoke {
			if now.Sub(since) >= t.grace {
				terminals = append(terminals, tailTerminal{"revoked", fmt.Sprintf("capsule of %s revoked %s ago with no new assignment (grace %s)", agent, now.Sub(since).Round(time.Second), t.grace)})
				delete(t.pendingRevoke, agent)
				break
			}
		}
	}
	if settled {
		terminals = append(terminals, tailTerminal{"settled", "trajectory settled"})
	}
	if candidate != "" {
		terminals = append(terminals, tailTerminal{"candidate", "operation " + candidate + " is awaiting_approval"})
	}
	return t.resolve(terminals)
}

// resolve picks the terminal that ends the tail: a candidate wins, otherwise
// the first allowed one. Terminals --until excludes are printed as ignored.
func (t *tailer) resolve(terminals []tailTerminal) (int, bool) {
	var pick *tailTerminal
	for i := range terminals {
		term := terminals[i]
		if !t.allowed[term.name] {
			if term.name == "settled" {
				if t.settledShown {
					continue
				}
				t.settledShown = true
			}
			t.print(t.noteEventAt(fmt.Sprintf("terminal %s ignored by --until: %s", term.name, term.summary), "terminal_ignored", t.hooks.now()))
			continue
		}
		if pick == nil || term.name == "candidate" {
			pick = &terminals[i]
		}
	}
	if pick == nil {
		return 0, false
	}
	code := tailTerminalCodes[pick.name]
	ev := t.noteEventAt(fmt.Sprintf("terminal %s (exit %d): %s", pick.name, code, pick.summary), "terminal", t.hooks.now())
	ev.raw, _ = json.Marshal(map[string]any{"terminal": pick.name, "exit_code": code})
	t.print(ev)
	return code, true
}

func (t *tailer) noteEvent(summary, kind string) tailEvent {
	return t.noteEventAt(summary, kind, t.hooks.now())
}

func (t *tailer) noteEventAt(summary, kind string, at time.Time) tailEvent {
	return tailEvent{when: at.UTC(), source: "tail", agent: "-", kind: kind, summary: summary, level: 1}
}

// reportReads prints one line for a poll with failures, and one when reads
// recover.
func (t *tailer) reportReads(failures []*tailRoute) {
	if len(failures) > 0 {
		t.failStreak++
		msg := tailClip(failures[0].err.Error(), 200)
		if len(failures) > 1 {
			msg += fmt.Sprintf(" (+%d more)", len(failures)-1)
		}
		t.print(t.noteEvent("read failed: "+msg, "read_failed"))
		return
	}
	if t.failStreak > 0 {
		t.print(t.noteEvent(fmt.Sprintf("read recovered after %d failed polls", t.failStreak), "read_recovered"))
		t.failStreak = 0
	}
}

func (t *tailer) pollLifecycle() ([]tailEvent, []tailTerminal, *tailRoute) {
	var events []tailEvent
	var terms []tailTerminal
	for page := 0; page < traceMaxPages; page++ {
		path := fmt.Sprintf("/api/trajectories/%s/events?after=%d&limit=%d", url.PathEscape(t.traj), t.lifeCursor, tailPageLimit)
		var resp struct {
			Events     []json.RawMessage `json:"events"`
			NextCursor int64             `json:"next_cursor"`
			Watermark  int64             `json:"watermark"`
		}
		if r := t.get(path, &resp); r != nil {
			return events, terms, r
		}
		live := t.lifeDrained
		for _, raw := range resp.Events {
			var ev struct {
				EventID    string   `json:"event_id"`
				Kind       string   `json:"kind"`
				ReducerSeq int64    `json:"reducer_seq"`
				ComputerID string   `json:"computer_id"`
				AgentID    string   `json:"agent_id"`
				RunID      string   `json:"run_id"`
				Reason     string   `json:"reason"`
				Created    string   `json:"created_at"`
				Refs       []string `json:"artifact_refs"`
			}
			if json.Unmarshal(raw, &ev) != nil {
				continue
			}
			if ev.ReducerSeq > t.lifeCursor {
				t.lifeCursor = ev.ReducerSeq
			}
			if t.computerID == "" {
				t.computerID = firstNonEmpty(t.c.computerID, ev.ComputerID)
			}
			if ev.EventID != "" {
				if t.seen["l:"+ev.EventID] {
					continue
				}
				t.seen["l:"+ev.EventID] = true
			}
			if live {
				terms = append(terms, t.lifecycleTerminals(ev.Kind, ev.AgentID, ev.Reason)...)
			}
			if !live && !t.history {
				continue
			}
			events = append(events, lifecycleEvent(ev.Kind, ev.AgentID, ev.RunID, ev.Reason, ev.Created, ev.ReducerSeq, ev.Refs, raw, t.hooks.now()))
		}
		if resp.NextCursor > t.lifeCursor {
			t.lifeCursor = resp.NextCursor
		}
		if len(resp.Events) == 0 || resp.NextCursor >= resp.Watermark {
			t.lifeDrained = true
			break
		}
	}
	return events, terms, nil
}

var tailWaitsOnOwner = regexp.MustCompile(`(?i)\b(wait(?:s|ing)?|await(?:s|ing)?)\b[^.\n]{0,40}\b(?:owner|requester)\b|\b(?:needs?|requires?|requesting)\b[^.\n]{0,20}\b(?:owner|requester)\b[^.\n]{0,20}\b(?:decision|input|answer|direction|approval|reply|response)\b`)

// lifecycleTerminals updates the revocation bookkeeping and returns the
// terminal states one live lifecycle event ends the tail with.
func (t *tailer) lifecycleTerminals(kind, agent, reason string) []tailTerminal {
	switch kind {
	case "co_super_assignment_cancelled":
		if strings.Contains(reason, "interrupted_by_restart") {
			return []tailTerminal{{"restart", fmt.Sprintf("assignment of %s cancelled: %s", agentLabel(agent, ""), reason)}}
		}
	case "co_super_capsule_disposition_set":
		if reason == "revoked" {
			if _, ok := t.pendingRevoke[agent]; !ok {
				t.pendingRevoke[agent] = t.hooks.now()
			}
		}
	case "co_super_assignment_opened":
		for k := range t.pendingRevoke {
			delete(t.pendingRevoke, k)
		}
	case "texture_turn_committed":
		if tailWaitsOnOwner.MatchString(reason) {
			return []tailTerminal{{"owner", "texture turn waits on the owner: " + tailClip(reason, 200)}}
		}
	}
	return nil
}

func lifecycleEvent(kind, agent, runID, reason, created string, seq int64, refs []string, raw json.RawMessage, fallback time.Time) tailEvent {
	when := parseTailTime(created, fallback)
	label := agentLabel(agent, "")
	if agent == "" {
		switch {
		case kind == "texture_turn_committed":
			label = "texture"
		case kind == "run_terminalized" && len(refs) > 0:
			label = agentLabel("", refs[0])
		case runID != "":
			label = agentLabel("", runID)
		default:
			label = "trajectory"
		}
	}
	summary := tailClip(oneLine(reason), 300)
	switch kind {
	case "co_super_assignment_bound":
		summary = strings.TrimSpace("run " + shortID(runID) + " " + summary)
	case "run_terminalized":
		if len(refs) > 0 {
			summary = strings.TrimSpace("run " + shortID(refs[0]) + " " + summary)
		}
	case "artifact_head_advanced":
		if len(refs) > 1 {
			summary = strings.TrimSpace("revision " + shortID(refs[len(refs)-1]) + " " + summary)
		}
	}
	return tailEvent{when: when, source: "lifecycle", seq: seq, agent: label, kind: kind, summary: summary, level: 1, raw: raw}
}

func (t *tailer) pollRuns() ([]tailEvent, *tailRoute) {
	var events []tailEvent
	var list struct {
		Runs []struct {
			LoopID  string `json:"loop_id"`
			RunID   string `json:"run_id"`
			AgentID string `json:"agent_id"`
			State   string `json:"state"`
		} `json:"runs"`
	}
	listPath := fmt.Sprintf("/api/runs?trajectory_id=%s&limit=%d", url.QueryEscape(t.traj), tailRunListCap)
	var firstErr *tailRoute
	if r := t.get(listPath, &list); r != nil {
		if r.hard {
			return nil, r
		}
		firstErr = r
	}
	states := map[string]string{}
	agents := map[string]string{}
	for _, run := range list.Runs {
		id := firstNonEmpty(run.LoopID, run.RunID)
		if id == "" {
			continue
		}
		states[id] = run.State
		agents[id] = run.AgentID
		if _, ok := t.runs[id]; !ok {
			t.runs[id] = &tailRunState{hist: t.tick == 0}
		}
	}
	ids := make([]string, 0, len(t.runs))
	for id := range t.runs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		st := t.runs[id]
		if st.done {
			continue
		}
		terminal := tailRunTerminal(states[id])
		for page := 0; page < traceMaxPages; page++ {
			path := fmt.Sprintf("/api/runs/%s/events?after=%d&limit=%d", url.PathEscape(id), st.cursor, tailPageLimit)
			var resp struct {
				Events    []json.RawMessage `json:"events"`
				NextAfter int64             `json:"next_after"`
			}
			if r := t.get(path, &resp); r != nil {
				if r.hard {
					return events, r
				}
				if firstErr == nil {
					firstErr = r
				}
				break
			}
			show := !st.hist || t.history
			for _, raw := range resp.Events {
				ev, seq, eventID := runEvent(raw, agents[id], id, t.hooks.now())
				if seq > st.cursor {
					st.cursor = seq
				}
				if eventID != "" {
					if t.seen["r:"+eventID] {
						continue
					}
					t.seen["r:"+eventID] = true
				}
				if show && ev.level <= t.level {
					events = append(events, ev)
				}
			}
			if resp.NextAfter > st.cursor {
				st.cursor = resp.NextAfter
			}
			if resp.NextAfter == 0 || len(resp.Events) == 0 {
				st.drained = true
				st.hist = false
				if terminal {
					st.done = true
				}
				break
			}
		}
	}
	return events, firstErr
}

func tailRunTerminal(state string) bool {
	switch state {
	case "completed", "cancelled", "canceled", "failed", "errored", "error", "done", "terminalized", "expired":
		return true
	}
	return false
}

// runEvent turns one run event record into a printable event.
func runEvent(raw json.RawMessage, runAgent, runID string, fallback time.Time) (tailEvent, int64, string) {
	var ev struct {
		EventID string                     `json:"event_id"`
		Seq     int64                      `json:"seq"`
		Stream  int64                      `json:"stream_seq"`
		TS      string                     `json:"ts"`
		LoopID  string                     `json:"loop_id"`
		AgentID string                     `json:"agent_id"`
		Kind    string                     `json:"kind"`
		Payload map[string]json.RawMessage `json:"payload"`
	}
	_ = json.Unmarshal(raw, &ev)
	loop := firstNonEmpty(ev.LoopID, runID)
	label := agentLabel(firstNonEmpty(ev.AgentID, runAgent), loop)
	out := tailEvent{when: parseTailTime(ev.TS, fallback), source: "run", seq: firstNonZero(ev.Stream, ev.Seq), agent: label, kind: ev.Kind, raw: raw}
	str := func(key string) string {
		var s string
		if v, ok := ev.Payload[key]; ok && json.Unmarshal(v, &s) == nil {
			return s
		}
		if v, ok := ev.Payload[key]; ok {
			return strings.Trim(string(v), `"`)
		}
		return ""
	}
	switch ev.Kind {
	case "loop.started":
		out.level = 1
	case "loop.completed":
		out.level = 1
		out.summary = joinFields(field("result_length", str("result_length")), field("out_tokens", str("output_tokens")))
	case "loop.cancelled":
		out.level = 1
		out.summary = firstNonEmpty(str("fate"), str("reason"))
	case "loop.failed", "loop.error", "loop.errored":
		out.level = 1
		out.summary = tailClip(oneLine(firstNonEmpty(str("error"), str("reason"), str("fate"))), 300)
	case "tool.result":
		out.level = 2
		out.summary = toolSummary(ev.Payload)
	case "loop.progress":
		out.level = 3
		out.summary = joinFields(field("iter", str("iteration")), str("llm_model"), "stop="+str("stop_reason"), field("tool_calls", str("tool_calls")), tailClip(oneLine(str("response_text")), 160))
	case "channel.message":
		out.level = 2
		out.summary = tailClip(oneLine(str("content")), 200)
	case "activation.passivated":
		out.level = 2
		out.summary = str("reason")
	case "loop.submitted":
		out.level = 2
		out.summary = field("prompt_length", str("prompt_length"))
	default:
		out.level = 3
		out.summary = tailClip(oneLine(string(ev.Payload["doc_id"])+" "+str("revision_id")), 120)
		if strings.HasPrefix(ev.Kind, "texture.") {
			out.level = 2
		}
	}
	return out, ev.Seq, ev.EventID
}

// toolSummary renders a tool.result payload. The output head is appended
// after a marker so print can drop it below level 3.
func toolSummary(payload map[string]json.RawMessage) string {
	var tool, source, output string
	var dur int64
	var isErr bool
	_ = json.Unmarshal(payload["tool"], &tool)
	_ = json.Unmarshal(payload["duration_ms"], &dur)
	_ = json.Unmarshal(payload["is_error"], &isErr)
	var args struct {
		Source string `json:"source"`
	}
	_ = json.Unmarshal(payload["arguments"], &args)
	source = args.Source
	_ = json.Unmarshal(payload["output"], &output)

	var res struct {
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
		Error    string `json:"error"`
		ExitCode *int   `json:"exit_code"`
	}
	_ = json.Unmarshal([]byte(output), &res)

	parts := []string{firstNonEmpty(tool, "tool"), fmt.Sprintf("%dms", dur)}
	if res.ExitCode != nil {
		parts = append(parts, fmt.Sprintf("exit=%d", *res.ExitCode))
	}
	if isErr {
		parts = append(parts, "ERROR")
	}
	if res.Error != "" {
		parts = append(parts, "err="+strconv.Quote(tailClip(oneLine(res.Error), 80)))
	}
	if line := firstMeaningfulLine(source); line != "" {
		parts = append(parts, "| "+tailClip(line, 100))
	}
	if argv := startArgv(source); argv != "" {
		parts = append(parts, "| job: "+tailClip(argv, 160))
	}
	head := strings.Join(parts, " ")
	var heads []string
	if res.Stdout != "" {
		heads = append(heads, "stdout: "+tailClip(oneLine(res.Stdout), 200))
	}
	if res.Stderr != "" {
		heads = append(heads, "stderr: "+tailClip(oneLine(res.Stderr), 200))
	}
	if len(heads) > 0 {
		head += tailOutputMarker + strings.Join(heads, " | ")
	}
	return head
}

// tailOutputMarker separates a tool summary from its output heads (-vvv).
const tailOutputMarker = "\x00"

var startCall = regexp.MustCompile(`choir\.Start\(\s*\[\]string\{((?:[^{}"` + "`" + `]|"(?:\\.|[^"\\])*"|` + "`[^`]*`" + `)*)\}`)
var quotedString = regexp.MustCompile(`"(?:\\.|[^"\\])*"|` + "`[^`]*`")

// startArgv extracts the argv of each choir.Start job in a Go source cell.
func startArgv(source string) string {
	var jobs []string
	for _, m := range startCall.FindAllStringSubmatch(source, -1) {
		var argv []string
		for _, q := range quotedString.FindAllString(m[1], -1) {
			if s, err := strconv.Unquote(q); err == nil {
				argv = append(argv, s)
			}
		}
		if len(argv) > 0 {
			jobs = append(jobs, strings.Join(argv, " "))
		}
	}
	return strings.Join(jobs, " ; ")
}

// firstMeaningfulLine is the first line of a Go cell that is not blank, a
// comment, a package clause or part of an import.
func firstMeaningfulLine(source string) string {
	inImport := false
	for _, line := range strings.Split(source, "\n") {
		s := strings.TrimSpace(line)
		switch {
		case s == "" || strings.HasPrefix(s, "//") || strings.HasPrefix(s, "/*") || strings.HasPrefix(s, "*"):
			continue
		case inImport:
			if s == ")" {
				inImport = false
			}
			continue
		case strings.HasPrefix(s, "package "):
			continue
		case s == "import (":
			inImport = true
			continue
		case strings.HasPrefix(s, "import "):
			continue
		}
		return s
	}
	return ""
}

// agentLabel is "profile:short-id" for an agent id, "loop:short-id" for a loop.
func agentLabel(agentID, loopID string) string {
	if a := strings.TrimSpace(agentID); a != "" {
		prefix, rest, ok := strings.Cut(a, ":")
		if !ok {
			return shortID(a)
		}
		return prefix + ":" + shortID(rest)
	}
	if l := strings.TrimSpace(loopID); l != "" {
		return "loop:" + shortID(l)
	}
	return "-"
}

func shortID(id string) string {
	id = strings.TrimPrefix(id, "run:")
	id = strings.TrimPrefix(id, "assignment-")
	if i := strings.LastIndex(id, ":"); i >= 0 && !strings.HasPrefix(id, "obj:") {
		id = id[i+1:]
	}
	if utf8.RuneCountInString(id) > 8 {
		return string([]rune(id)[:8])
	}
	return id
}

func parseTailTime(s string, fallback time.Time) time.Time {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC()
	}
	return fallback.UTC()
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", `\n`)
	return strings.TrimSpace(s)
}

func tailClip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "..."
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func firstNonZero(vals ...int64) int64 {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

func field(name, val string) string {
	if val == "" {
		return ""
	}
	return name + "=" + val
}

func joinFields(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" && !strings.HasSuffix(p, "=") {
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}

// print writes one event if its level is shown.
func (t *tailer) print(ev tailEvent) {
	if ev.level > t.level {
		return
	}
	summary := ev.summary
	if i := strings.Index(summary, tailOutputMarker); i >= 0 {
		if t.level >= 3 {
			summary = summary[:i] + " || " + summary[i+1:]
		} else {
			summary = summary[:i]
		}
	}
	stamp := ev.when.UTC().Format("2006-01-02T15:04:05Z")
	if t.asJSON {
		rec := map[string]any{"time": stamp, "source": ev.source, "agent": ev.agent, "kind": ev.kind, "summary": summary}
		if ev.seq != 0 {
			rec["seq"] = ev.seq
		}
		if t.level >= 3 && len(ev.raw) > 0 {
			rec["event"] = ev.raw
		}
		if ev.kind == "terminal" && len(ev.raw) > 0 {
			var extra map[string]any
			if json.Unmarshal(ev.raw, &extra) == nil {
				for k, v := range extra {
					rec[k] = v
				}
			}
		}
		line, err := json.Marshal(rec)
		if err == nil {
			fmt.Fprintln(t.out, string(line))
		}
		return
	}
	fmt.Fprintf(t.out, "%s  %-24s  %-34s  %s\n", stamp, ev.agent, ev.kind, strings.TrimSpace(summary))
}
