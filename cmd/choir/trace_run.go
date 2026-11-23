package main

// `choir trace run <run_id>` shows one run step by step with the reasons its
// record holds. It is a client-side assembler over routes that already exist
// and are read-only: GET /api/runs/{id}, GET /api/runs/{id}/events (every
// page), GET /api/trajectories/{id}/events (the run's turn commits, controls
// and work items), GET /api/inboxes/{inbox}/records (the precommitment
// records the run sent or received) and GET /api/runs?channel_id= (its child
// runs). Nothing is written, no model is called, and nothing is inferred
// without a label. The record holds less than a reader wants (no stated
// reason on a call, no query, no waking record); the command says so in a
// closing section built from what is absent in the run it read.

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

var (
	traceRunNow   = time.Now
	traceRunSleep = time.Sleep
)

const (
	traceRunPage            = 1000
	traceRunDefaultMax      = 50000
	traceRunLifecycleCap    = 20000
	traceRunDefaultInboxWin = 500
	traceRunChildListCap    = 500
	traceRunRetries         = 2
)

const traceRunHelp = `Show one run step by step, with the reasons its record holds.

Read-only. Assembled on this machine from routes that already exist
(GET /api/runs/{id}, /api/runs/{id}/events, /api/trajectories/{id}/events,
/api/inboxes/{inbox}/records, /api/runs?channel_id=); no server change, no
write, no model call. The key is an owner API key (choir_sk_...) for the
computer's owner with scope read:runtime (or admin): pass --api-key-file,
set $CHOIR_API_KEY or $CHOIR_API_KEY_FILE, or run choir login. The key is
never printed.

For each model call, cell and module-surface call, the reason shown is the
one the record holds, in this order: the turn commit's Reason, the model's
own text before the cell, or nothing (a step says only what is recorded). The closing section
"What this run's record does not contain" lists what was absent in this
run's data. Events are kept 7 or 14 days by kind; a run older than a kind's
retention says so. A run whose events could not all be read is printed as
PARTIAL with what is missing.

--json prints the assembled run as one JSON document with the same content.
--full does not clip sources, outputs or texts (the default clips each to a
head and says its size).`

func runTraceRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir trace run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "Print the assembled run as one JSON document (same content, stable field names)")
	full := fs.Bool("full", false, "Do not clip sources, outputs and texts (default clips to a head and its size)")
	noRecords := fs.Bool("no-records", false, "Do not read inboxes (skips the records the run sent or received and the waking candidate)")
	maxEvents := fs.Int("max-events", traceRunDefaultMax, "Stop reading the run's events after this many and print the run as partial")
	inboxWin := fs.Int("inbox-window", traceRunDefaultInboxWin, "How many of the newest records of each inbox to read")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: choir trace run [flags] <run_id>\n\n%s\n\nFlags:\n", traceRunHelp)
		fs.PrintDefaults()
	}
	flagArgs, positional := splitTraceArgsWith(args, map[string]bool{
		"api-key-file": true, "host": true, "computer": true, "timeout": true, "max-events": true, "inbox-window": true,
	})
	c, err := newClient(fs, flagArgs, stdout, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "choir trace run: %v\n", err)
		return 2
	}
	positional = append(positional, fs.Args()...)
	if len(positional) != 1 || strings.TrimSpace(positional[0]) == "" {
		fmt.Fprintln(stderr, "choir trace run: exactly one run id required")
		return 2
	}
	if *maxEvents < 1 || *inboxWin < 1 {
		fmt.Fprintln(stderr, "choir trace run: --max-events and --inbox-window must be at least 1")
		return 2
	}
	runID := strings.TrimSpace(positional[0])
	f := &traceRunFetcher{c: c, errw: stderr, maxEvents: *maxEvents, inboxWindow: *inboxWin, noRecords: *noRecords}
	in, ferr := f.fetch(runID)
	if ferr != nil {
		fmt.Fprintf(stderr, "choir trace run: %s\n", ferr.Error())
		return 1
	}
	in.Now = traceRunNow().UTC()
	in.Full = *full
	doc := assembleTraceRun(in)
	if *asJSON {
		return writeJSON(stdout, doc)
	}
	renderTraceRun(stdout, doc, *full)
	return 0
}

// redactKey removes the key from any text before it is printed.
func redactKey(s, key string) string {
	if key != "" {
		s = strings.ReplaceAll(s, key, "[key]")
	}
	// A key-shaped token is never printed, whoever echoed it.
	for {
		i := strings.Index(s, apiKeyPrefix)
		if i < 0 {
			return s
		}
		j := i + len(apiKeyPrefix)
		for j < len(s) && (s[j] == '_' || s[j] == '-' || (s[j] >= '0' && s[j] <= '9') || (s[j] >= 'a' && s[j] <= 'z') || (s[j] >= 'A' && s[j] <= 'Z')) {
			j++
		}
		s = s[:i] + "[key]" + s[j:]
	}
}

// ---- fetching ----

type traceRunFetcher struct {
	c           *client
	errw        io.Writer
	maxEvents   int
	inboxWindow int
	noRecords   bool
}

// get is one GET with up to traceRunRetries retries on a transport error or
// a 5xx answer; a 4xx answer is final.
func (f *traceRunFetcher) get(path string, out any) error {
	var err error
	for attempt := 0; attempt <= traceRunRetries; attempt++ {
		if attempt > 0 {
			traceRunSleep(time.Duration(attempt) * time.Second)
		}
		err = f.c.do(http.MethodGet, path, nil, out)
		if err == nil {
			return nil
		}
		var apiErr *apiErrorResp
		if errors.As(err, &apiErr) && apiErr.Status < 500 {
			return err
		}
	}
	return err
}

func (f *traceRunFetcher) failText(path string, err error) string {
	text := routeFailure(path, err)
	return redactKey(text, f.c.apiKey)
}

// authMessage says which credential a refused route needs.
func authMessage(status int, route string) string {
	switch status {
	case http.StatusUnauthorized:
		return fmt.Sprintf("HTTP 401 on %s: the key was not accepted (missing, malformed, expired or revoked). "+
			"This route needs an owner API key (choir_sk_...) of the computer's owner with scope read:runtime (or admin); "+
			"pass --api-key-file, set $%s or $%s, or run choir login", route, apiKeyEnvVar, apiKeyFileEnvVar)
	default:
		return fmt.Sprintf("HTTP 403 on %s: the key was accepted but is refused. This route needs scope read:runtime (or admin), "+
			"and an owner-wide key needs --computer for the computer that holds the run; a key bound to another computer is refused", route)
	}
}

func authStatus(err error) int {
	var apiErr *apiErrorResp
	if errors.As(err, &apiErr) && (apiErr.Status == http.StatusUnauthorized || apiErr.Status == http.StatusForbidden) {
		return apiErr.Status
	}
	return 0
}

func (f *traceRunFetcher) fetch(runID string) (*traceRunInput, error) {
	in := &traceRunInput{RunID: runID}
	// The run row. A missing run, or another owner's, is the same 404.
	runPath := "/api/runs/" + url.PathEscape(runID)
	if err := f.get(runPath, &in.Run); err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("run %s not found (or not owned by this key's owner)", runID)
		}
		if st := authStatus(err); st != 0 {
			return nil, errors.New(authMessage(st, runPath))
		}
		return nil, errors.New(f.failText(runPath, err))
	}
	in.Reads = append(in.Reads, traceRunRead{Route: "GET /api/runs/{id}", Status: "ok"})
	if id := strings.TrimSpace(in.Run.LoopID); id != "" {
		in.RunID = id
	}

	// Every page of the run's own events.
	if err := f.fetchEvents(in); err != nil {
		return nil, err
	}

	// Lifecycle events of the trajectory: turn commits, controls, work items.
	f.fetchLifecycle(in)

	// Children: runs of the same channel that name this run as their requester.
	f.fetchChildren(in)

	// Inboxes: the records the run sent or received.
	if f.noRecords {
		in.Reads = append(in.Reads, traceRunRead{Route: "GET /api/inboxes/{inbox}/records", Status: "skipped", Detail: "--no-records"})
	} else {
		f.fetchRecords(in)
	}
	return in, nil
}

func (f *traceRunFetcher) fetchEvents(in *traceRunInput) error {
	after := int64(0)
	read := traceRunRead{Route: "GET /api/runs/{id}/events?after=&limit=1000", Status: "ok"}
	for {
		path := fmt.Sprintf("/api/runs/%s/events?after=%d&limit=%d", url.PathEscape(in.RunID), after, traceRunPage)
		var page struct {
			Events    []traceRunRawEvent `json:"events"`
			NextAfter int64              `json:"next_after"`
		}
		if err := f.get(path, &page); err != nil {
			if st := authStatus(err); st != 0 {
				return errors.New(authMessage(st, "/api/runs/{id}/events"))
			}
			read.Status = "partial"
			read.Detail = fmt.Sprintf("stopped after seq %d: %s", after, f.failText(path, err))
			in.PartialWhy = append(in.PartialWhy, "events: "+read.Detail+"; the run's later events are missing")
			break
		}
		in.Events = append(in.Events, page.Events...)
		read.Items = len(in.Events)
		if page.NextAfter == 0 {
			break
		}
		if page.NextAfter <= after {
			read.Status = "partial"
			read.Detail = fmt.Sprintf("the cursor did not advance past seq %d", after)
			in.PartialWhy = append(in.PartialWhy, "events: "+read.Detail+"; later events are missing")
			break
		}
		after = page.NextAfter
		fmt.Fprintf(f.errw, "choir trace run: fetched %d events (through seq %d)...\n", len(in.Events), after)
		if len(in.Events) >= f.maxEvents {
			read.Status = "partial"
			read.Detail = fmt.Sprintf("stopped after seq %d at --max-events %d", after, f.maxEvents)
			in.PartialWhy = append(in.PartialWhy, "events: "+read.Detail+"; later events are missing")
			break
		}
	}
	in.Reads = append(in.Reads, read)
	return nil
}

// fetchLifecycle reads the trajectory's lifecycle events from the newest
// window backwards until it passes the run's start (the route filters by
// nothing but trajectory), so a long trajectory is read only as far as the
// run needs.
func (f *traceRunFetcher) fetchLifecycle(in *traceRunInput) {
	read := traceRunRead{Route: "GET /api/trajectories/{id}/events?after=&limit=1000", Status: "ok"}
	tid := strings.TrimSpace(in.Run.TrajectoryID)
	if tid == "" {
		read.Status, read.Detail = "skipped", "the run row names no trajectory"
		in.Reads = append(in.Reads, read)
		return
	}
	base := "/api/trajectories/" + url.PathEscape(tid) + "/events"
	type pageT struct {
		Events         []traceRunLifecycleRow `json:"events"`
		NextCursor     int64                  `json:"next_cursor"`
		Watermark      int64                  `json:"watermark"`
		CursorExpired  bool                   `json:"cursor_expired"`
		ReplayRequired bool                   `json:"replay_required"`
	}
	var probe pageT
	if err := f.get(base+"?after=0&limit=1", &probe); err != nil {
		f.lifecycleFailed(in, &read, base, err)
		return
	}
	start := parseTraceTime(in.Run.CreatedAt)
	hi := probe.Watermark
	seen := 0
	for hi > 0 {
		lo := hi - traceRunPage
		if lo < 0 {
			lo = 0
		}
		oldest := time.Time{}
		cursor := lo
		for {
			var page pageT
			path := fmt.Sprintf("%s?after=%d&limit=%d", base, cursor, traceRunPage)
			if err := f.get(path, &page); err != nil {
				f.lifecycleFailed(in, &read, base, err)
				return
			}
			if page.CursorExpired || page.ReplayRequired {
				read.Status = "partial"
				read.Detail = "the route says the cursor expired or a replay is required"
				in.PartialWhy = append(in.PartialWhy, "lifecycle events: "+read.Detail)
			}
			for _, ev := range page.Events {
				seen++
				if t := parseTraceTime(ev.CreatedAt); !t.IsZero() && (oldest.IsZero() || t.Before(oldest)) {
					oldest = t
				}
				if strings.TrimSpace(ev.RunID) == in.RunID {
					in.Lifecycle = append(in.Lifecycle, ev)
				}
			}
			if len(page.Events) == 0 || page.NextCursor >= hi || page.NextCursor <= cursor {
				break
			}
			cursor = page.NextCursor
		}
		if lo == 0 {
			break
		}
		if !start.IsZero() && !oldest.IsZero() && oldest.Before(start.Add(-time.Minute)) {
			break
		}
		if seen >= traceRunLifecycleCap {
			read.Status = "partial"
			read.Detail = fmt.Sprintf("read the newest %d of %d lifecycle events; the run began before the oldest read", seen, probe.Watermark)
			in.PartialWhy = append(in.PartialWhy, "lifecycle events: "+read.Detail+" (turn commits, controls and work items before it are missing)")
			break
		}
		hi = lo
	}
	read.Items = len(in.Lifecycle)
	in.Reads = append(in.Reads, read)
}

func (f *traceRunFetcher) lifecycleFailed(in *traceRunInput, read *traceRunRead, base string, err error) {
	read.Status = "failed"
	if isNotFound(err) {
		read.Status = "skipped"
		read.Detail = "the trajectory is not readable (404)"
		in.Reads = append(in.Reads, *read)
		return
	}
	read.Detail = f.failText(base, err)
	in.Reads = append(in.Reads, *read)
	in.PartialWhy = append(in.PartialWhy, "lifecycle events not read: "+read.Detail+" (turn-commit reasons, controls and work items are missing)")
}

func (f *traceRunFetcher) fetchChildren(in *traceRunInput) {
	read := traceRunRead{Route: "GET /api/runs?channel_id=&limit=500", Status: "ok"}
	ch := strings.TrimSpace(in.Run.ChannelID)
	if ch == "" {
		read.Status, read.Detail = "skipped", "the run row names no channel"
		in.Reads = append(in.Reads, read)
		return
	}
	path := fmt.Sprintf("/api/runs?channel_id=%s&limit=%d", url.QueryEscape(ch), traceRunChildListCap)
	var list struct {
		Runs []struct {
			RunID            string `json:"run_id"`
			RequestedByRunID string `json:"requested_by_run_id"`
		} `json:"runs"`
	}
	if err := f.get(path, &list); err != nil {
		read.Status = "failed"
		read.Detail = f.failText(path, err)
		in.Reads = append(in.Reads, read)
		in.ChildrenNote = "child runs not read: " + read.Detail
		return
	}
	for _, r := range list.Runs {
		if r.RequestedByRunID == in.RunID && r.RunID != "" {
			in.Children = append(in.Children, r.RunID)
		}
	}
	read.Items = len(list.Runs)
	in.ChildrenNote = fmt.Sprintf("searched the newest %d runs of channel %s", len(list.Runs), ch)
	in.Reads = append(in.Reads, read)
}

func (f *traceRunFetcher) fetchRecords(in *traceRunInput) {
	doc := strings.TrimSpace(in.Run.ChannelID)
	agent := strings.TrimSpace(in.Run.AgentID)
	if doc == "" && strings.Contains(agent, ":") {
		doc = agent[strings.Index(agent, ":")+1:]
	}
	var inboxes []string
	add := func(s string) {
		for _, x := range inboxes {
			if x == s {
				return
			}
		}
		inboxes = append(inboxes, s)
	}
	if strings.Contains(agent, ":") {
		add(agent)
	}
	if doc != "" {
		add(deskmanifest.DocumentInbox(doc))
		add(deskmanifest.ResearchInbox(doc))
	}
	if owner := strings.TrimSpace(in.Run.OwnerID); owner != "" {
		add(deskmanifest.AdmitterInbox(owner))
	}
	in.OwnInbox = agent
	for _, inbox := range inboxes {
		read := traceRunRead{Route: "GET /api/inboxes/" + inbox + "/records", Status: "ok"}
		base := "/api/inboxes/" + url.PathEscape(inbox) + "/records"
		var probe struct {
			Head int64 `json:"head"`
		}
		if err := f.get(base+"?after=0&limit=1", &probe); err != nil {
			read.Status = "failed"
			read.Detail = f.failText(base, err)
			in.Reads = append(in.Reads, read)
			in.PartialWhy = append(in.PartialWhy, "inbox "+inbox+" not read: "+read.Detail)
			continue
		}
		after := probe.Head - int64(f.inboxWindow)
		if after < 0 {
			after = 0
		}
		var page struct {
			Records []traceRunRecordRow `json:"records"`
		}
		path := fmt.Sprintf("%s?after=%d&limit=%d", base, after, f.inboxWindow)
		if err := f.get(path, &page); err != nil {
			read.Status = "failed"
			read.Detail = f.failText(base, err)
			in.Reads = append(in.Reads, read)
			in.PartialWhy = append(in.PartialWhy, "inbox "+inbox+" not read: "+read.Detail)
			continue
		}
		read.Items = len(page.Records)
		if after > 0 {
			// The window starts mid-inbox: complete for the run only when its
			// oldest record already predates the run.
			start := parseTraceTime(in.Run.CreatedAt)
			oldest := time.Time{}
			for _, r := range page.Records {
				if t := r.committedAt(); !t.IsZero() && (oldest.IsZero() || t.Before(oldest)) {
					oldest = t
				}
			}
			if oldest.IsZero() || oldest.After(start) {
				read.Status = "partial"
				read.Detail = fmt.Sprintf("the newest %d of %d records were read; older records (seq <= %d) were not", f.inboxWindow, probe.Head, after)
				in.PartialWhy = append(in.PartialWhy, "inbox "+inbox+": "+read.Detail)
			}
		}
		for _, r := range page.Records {
			if r.Inbox == "" {
				r.Inbox = inbox
			}
			in.Records = append(in.Records, r)
		}
		in.Reads = append(in.Reads, read)
	}
}
