package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// ---- what the routes return (the shapes of the handlers' own types) ----

// traceRunRow is a run row: types.RunRecord (GET /api/runs/{id}, JSON key
// loop_id) or the list's runStatusResponse (run_id).
type traceRunRow struct {
	LoopID           string         `json:"loop_id"`
	RunID            string         `json:"run_id"`
	AgentID          string         `json:"agent_id"`
	ChannelID        string         `json:"channel_id"`
	RequestedByRunID string         `json:"requested_by_run_id"`
	TrajectoryID     string         `json:"trajectory_id"`
	AgentProfile     string         `json:"agent_profile"`
	AgentRole        string         `json:"agent_role"`
	OwnerID          string         `json:"owner_id"`
	ComputerID       string         `json:"computer_id"`
	State            string         `json:"state"`
	Prompt           string         `json:"prompt"`
	Result           string         `json:"result"`
	Error            string         `json:"error"`
	CreatedAt        string         `json:"created_at"`
	UpdatedAt        string         `json:"updated_at"`
	FinishedAt       *string        `json:"finished_at"`
	Metadata         map[string]any `json:"metadata"`
}

// traceRunRawEvent is types.EventRecord.
type traceRunRawEvent struct {
	EventID string          `json:"event_id"`
	Seq     int64           `json:"seq"`
	TS      string          `json:"ts"`
	Kind    string          `json:"kind"`
	Phase   string          `json:"phase"`
	Payload json.RawMessage `json:"payload"`
}

// traceRunLifecycleRow is the part of types.LifecycleEvent the view reads.
type traceRunLifecycleRow struct {
	EventID    string `json:"event_id"`
	RunID      string `json:"run_id"`
	WorkItemID string `json:"work_item_id"`
	UpdateID   string `json:"update_id"`
	Kind       string `json:"kind"`
	Reason     string `json:"reason"`
	CreatedAt  string `json:"created_at"`
	ReducerSeq int64  `json:"reducer_seq"`
}

// traceRunRecordRow is the part of types.CommitmentRecord the view reads.
type traceRunRecordRow struct {
	RecordID    string   `json:"record_id"`
	Kind        string   `json:"kind"`
	Inbox       string   `json:"inbox"`
	SenderInbox string   `json:"sender_inbox"`
	InboxSeq    int64    `json:"inbox_seq"`
	Round       int      `json:"round"`
	Final       bool     `json:"final"`
	PSuccess    *float64 `json:"p_success"`
	Provenance  struct {
		AgentID     string `json:"agent_id"`
		CommittedAt string `json:"committed_at"`
		RunID       string `json:"run_id"`
		ActingRunID string `json:"acting_run_id"`
	} `json:"provenance"`
	Prediction struct {
		CommittedAt string `json:"committed_at"`
	} `json:"prediction"`
	Directive *struct {
		Subtype   string `json:"subtype"`
		Objective string `json:"objective"`
	} `json:"directive"`
	Cast *struct {
		Hypotheses []struct {
			ID          string  `json:"id"`
			Statement   string  `json:"statement"`
			Probability float64 `json:"probability"`
		} `json:"hypotheses"`
	} `json:"cast"`
	Report *struct {
		Summary    string `json:"summary"`
		StopReason string `json:"stop_reason"`
	} `json:"report"`
}

func (r traceRunRecordRow) committedAt() time.Time {
	if t := parseTraceTime(r.Provenance.CommittedAt); !t.IsZero() {
		return t
	}
	return parseTraceTime(r.Prediction.CommittedAt)
}

// traceRunInput is everything read for one run.
type traceRunInput struct {
	RunID        string
	Run          traceRunRow
	Events       []traceRunRawEvent
	Lifecycle    []traceRunLifecycleRow
	Records      []traceRunRecordRow
	OwnInbox     string
	Children     []string
	ChildrenNote string
	Reads        []traceRunRead
	PartialWhy   []string
	Now          time.Time
	Full         bool
}

// ---- helpers ----

func parseTraceTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

func payloadMap(raw json.RawMessage) map[string]any {
	m := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m
}

func mStr(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func mInt(m map[string]any, k string) int64 {
	switch v := m[k].(type) {
	case float64:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	}
	return 0
}

func mBool(m map[string]any, k string) bool {
	b, _ := m[k].(bool)
	return b
}

func mTime(m map[string]any, k string) time.Time { return parseTraceTime(mStr(m, k)) }

func clipText(s string, max int, full bool) traceRunText {
	runes := []rune(s)
	t := traceRunText{Text: s, Chars: len(runes)}
	if !full && len(runes) > max {
		t.Text = string(runes[:max])
		t.Cut = true
	}
	return t
}

const (
	clipSource = 1500
	clipOutput = 800
	clipWords  = 1200
	clipPrompt = 500
	// storedWordsLimit is where the writer cuts the model's text before it
	// stores it (toolloop.go: truncatePromptSnippet(resp.Text, 2000)).
	storedWordsLimit = 2000
)

var terminalStates = map[string]bool{"completed": true, "failed": true, "cancelled": true}

func isSearchVerb(v string) bool { return v == "search_start" || v == "web_search" }

type traceRunRetentionRule struct {
	kind  string
	phase string
	days  int
	note  string
}

var traceRunRetentionRules = []traceRunRetentionRule{
	{"tool.result", "", 7, "the cell source and output rows, except rows a run acceptance cites"},
	{"desk.cell", "", 14, "the cell record and its module-surface rows"},
	{"desk.egress", "", 14, "standalone module-surface rows and search ends"},
	{"loop.progress", "tool_loop", 14, "the model call rows (model, tokens, latency, the model's text)"},
}

// ---- assembly ----

type tev struct {
	raw traceRunRawEvent
	ts  time.Time
	p   map[string]any
}

type cellUnit struct {
	step  traceRunStep
	start time.Time
	end   time.Time
	seq   int64
}

type modelUnit struct {
	step traceRunStep
	ts   time.Time
	text traceRunReason
}

type orderedStep struct {
	t     time.Time
	group int
	seq   int64
	step  traceRunStep
}

func assembleTraceRun(in *traceRunInput) traceRunDoc {
	doc := traceRunDoc{Schema: traceRunSchema, Steps: []traceRunStep{}, Retention: []traceRunRetention{}, Reads: in.Reads, Gaps: []traceRunGap{}}
	if doc.Reads == nil {
		doc.Reads = []traceRunRead{}
	}
	run := in.Run
	start := parseTraceTime(run.CreatedAt)
	off := func(t time.Time) int64 {
		if start.IsZero() || t.IsZero() {
			return 0
		}
		return t.Sub(start).Milliseconds()
	}
	stamp := func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.Format(time.RFC3339Nano)
	}
	dur := func(ms int64) *int64 { return &ms }

	// Events: one per event_id, in (time, seq) order. A replayed commit
	// writes its line again under the same id.
	var evs []tev
	seenID := map[string]bool{}
	sorted := append([]traceRunRawEvent(nil), in.Events...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Seq < sorted[j].Seq })
	for _, e := range sorted {
		if e.EventID != "" {
			if seenID[e.EventID] {
				continue
			}
			seenID[e.EventID] = true
		}
		pm := payloadMap(e.Payload)
		// The runtime stores a run event's phase inside its payload
		// (payloadWithPhase); the record's own phase field is empty for run
		// events. Read either.
		if e.Phase == "" {
			e.Phase = mStr(pm, "phase")
		}
		evs = append(evs, tev{raw: e, ts: parseTraceTime(e.TS), p: pm})
	}
	sort.SliceStable(evs, func(i, j int) bool {
		if !evs[i].ts.Equal(evs[j].ts) {
			return evs[i].ts.Before(evs[j].ts)
		}
		return evs[i].raw.Seq < evs[j].raw.Seq
	})
	var maxSeq int64
	rowsByKind := map[string]int{}
	var sawDigest, sawReasoning, sawProviderCall bool
	for _, e := range evs {
		if e.raw.Seq > maxSeq {
			maxSeq = e.raw.Seq
		}
		key := e.raw.Kind
		if e.raw.Kind == "loop.progress" {
			key += "/" + e.raw.Phase
		}
		rowsByKind[key]++
		if e.raw.Kind == "loop.progress" && e.raw.Phase == "provider_call" {
			sawProviderCall = true
		}
		if bytes.Contains(e.raw.Payload, []byte(`"digest`)) {
			sawDigest = true
		}
		if bytes.Contains(e.raw.Payload, []byte(`"reasoning_content"`)) {
			sawReasoning = true
		}
	}

	var top []orderedStep
	var models []*modelUnit
	var servedRows []tev
	var trs, dcs []tev
	var dones []tev
	var standalone []tev
	startedCount := 0
	modelUse := map[string]*traceRunModelUse{}
	var modelOrder []string
	deskName := ""
	var totals traceRunTotals
	totals.ModuleCalls = map[string]int{}

	addTop := func(t time.Time, group int, seq int64, s traceRunStep) {
		top = append(top, orderedStep{t: t, group: group, seq: seq, step: s})
	}
	base := func(kind string, t time.Time, seq int64, id, title string) traceRunStep {
		return traceRunStep{Kind: kind, At: stamp(t), OffsetMs: off(t), Seq: seq, EventID: id, Title: title}
	}
	scalarFields := func(p map[string]any, skip ...string) map[string]string {
		out := map[string]string{}
		keys := make([]string, 0, len(p))
		for k := range p {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	next:
		for _, k := range keys {
			for _, s := range skip {
				if k == s {
					continue next
				}
			}
			var v string
			switch x := p[k].(type) {
			case string:
				v = x
			case float64:
				v = fmt.Sprintf("%g", x)
			case bool:
				v = fmt.Sprint(x)
			default:
				continue
			}
			if v == "" {
				continue
			}
			if r := []rune(v); len(r) > 160 {
				v = string(r[:160]) + "..."
			}
			out[k] = v
			if len(out) >= 8 {
				break
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}

	for _, e := range evs {
		p := e.p
		seq := e.raw.Seq
		switch e.raw.Kind {
		case "loop.submitted":
			s := base("lifecycle", e.ts, seq, e.raw.EventID, "run submitted")
			if n := mInt(p, "prompt_length"); n > 0 {
				s.Note = fmt.Sprintf("prompt %d chars", n)
			}
			addTop(e.ts, 0, seq, s)
		case "loop.started":
			startedCount++
			title := "run started"
			if startedCount > 1 {
				title = "run resumed (a later loop.started after the first)"
			}
			s := base("lifecycle", e.ts, seq, e.raw.EventID, title)
			s.Fields = scalarFields(p)
			addTop(e.ts, 0, seq, s)
		case "loop.completed":
			addTop(e.ts, 0, seq, base("lifecycle", e.ts, seq, e.raw.EventID, "run completed"))
		case "loop.failed", "loop.blocked", "loop.cancelled":
			word := strings.TrimPrefix(e.raw.Kind, "loop.")
			errText := mStr(p, "error")
			title := "run " + word
			if errText != "" {
				title += ": " + errText
			}
			if strings.Contains(errText, "interrupted_by_restart") || strings.Contains(errText, "interrupted by a restart") {
				title = "interrupted by a restart (run " + word + "): " + errText
			}
			addTop(e.ts, 0, seq, base("lifecycle", e.ts, seq, e.raw.EventID, title))
		case "activation.passivated":
			title := "run passivated"
			if mStr(p, "recovery") == "passivated_on_restart" {
				title = "interrupted by a restart (run passivated at boot)"
			} else if r := mStr(p, "reason"); r != "" {
				title += ": " + r
			}
			s := base("lifecycle", e.ts, seq, e.raw.EventID, title)
			s.Fields = scalarFields(p)
			addTop(e.ts, 0, seq, s)
		case "loop.compaction.started", "loop.compaction.completed":
			s := base("compaction", e.ts, seq, e.raw.EventID, strings.Replace(e.raw.Kind, "loop.", "", 1))
			s.Fields = scalarFields(p)
			addTop(e.ts, 0, seq, s)
		case "loop.retry":
			title := "retry"
			if r := mStr(p, "reason"); r != "" {
				title += ": " + r
			}
			s := base("retry", e.ts, seq, e.raw.EventID, title)
			s.Fields = scalarFields(p, "reason")
			addTop(e.ts, 0, seq, s)
		case "loop.progress":
			switch e.raw.Phase {
			case "tool_loop":
				callMs := mInt(p, "provider_call_ms")
				callStart := e.ts.Add(-time.Duration(callMs) * time.Millisecond)
				prov, mod := mStr(p, "llm_provider"), mStr(p, "llm_model")
				if mod == "" {
					mod = mStr(p, "model")
				}
				chars := int(mInt(p, "response_text_chars"))
				words := clipText(mStr(p, "response_text"), clipWords, in.Full)
				if chars > storedWordsLimit {
					words.StoredCut = true
					words.Chars = chars
				}
				if d := mStr(p, "desk"); d != "" && deskName == "" {
					deskName = d
				}
				m := &traceRunModel{Provider: prov, Model: mod, Iteration: int(mInt(p, "iteration")),
					InputTokens: mInt(p, "input_tokens"), OutputTokens: mInt(p, "output_tokens"), CachedTokens: mInt(p, "cached_input_tokens"),
					LatencyMs: callMs, StopReason: mStr(p, "stop_reason"), ToolCalls: int(mInt(p, "tool_calls")), Words: words}
				if names, ok := p["tool_call_names"].([]any); ok {
					for _, n := range names {
						if s, ok := n.(string); ok {
							m.ToolNames = append(m.ToolNames, s)
						}
					}
				}
				title := fmt.Sprintf("%s/%s", prov, mod)
				s := base("model_call", callStart, seq, e.raw.EventID, strings.Trim(title, "/"))
				s.DurationMs = dur(callMs)
				s.Model = m
				mu := &modelUnit{step: s, ts: e.ts}
				if strings.TrimSpace(words.Text) == "" {
					mu.text = traceRunReason{Source: "not_recorded", Note: "the model's text for this turn is empty or absent"}
				} else {
					mu.text = traceRunReason{Source: "model_words", Text: words.Text, Chars: words.Chars, Cut: words.Cut, StoredCut: words.StoredCut}
				}
				r := mu.text
				mu.step.Reason = &r
				if _, ok := p["tool_call_ids"]; ok {
					mu.step.Note = "tool call ids recorded"
				}
				models = append(models, mu)
				totals.ModelCalls++
				totals.InputTokens += m.InputTokens
				totals.OutputTokens += m.OutputTokens
				totals.CachedTokens += m.CachedTokens
				key := prov + "/" + mod
				if modelUse[key] == nil {
					modelUse[key] = &traceRunModelUse{Provider: prov, Model: mod}
					modelOrder = append(modelOrder, key)
				}
				modelUse[key].Calls++
			case "jev_judgment":
				s := base("judgment", e.ts, seq, e.raw.EventID, fmt.Sprintf("judgment call %s/%s", mStr(p, "llm_provider"), mStr(p, "llm_model")))
				s.Fields = scalarFields(p, "phase", "llm_provider", "llm_model")
				addTop(e.ts, 0, seq, s)
			default:
				title := "loop.progress"
				if e.raw.Phase != "" {
					title = "progress: " + e.raw.Phase
				}
				s := base("event", e.ts, seq, e.raw.EventID, title)
				s.Fields = scalarFields(p)
				addTop(e.ts, 0, seq, s)
			}
		case "tool.result":
			trs = append(trs, e)
		case "desk.cell":
			dcs = append(dcs, e)
		case "desk.egress":
			if mStr(p, "verb") == "search_done" {
				dones = append(dones, e)
			} else {
				standalone = append(standalone, e)
			}
		case "system1.judgment":
			s := base("judgment", e.ts, seq, e.raw.EventID, "system1.judgment")
			s.Fields = scalarFields(p)
			addTop(e.ts, 0, seq, s)
		case "model.served", "search.served":
			if e.raw.Kind == "model.served" {
				servedRows = append(servedRows, e)
			}
			s := base("served", e.ts, seq, e.raw.EventID, e.raw.Kind)
			s.Fields = scalarFields(p)
			addTop(e.ts, 0, seq, s)
		default:
			s := base("event", e.ts, seq, e.raw.EventID, e.raw.Kind)
			s.Fields = scalarFields(p)
			addTop(e.ts, 0, seq, s)
		}
	}

	// ---- cells: a desk_go_eval tool.result (source, output) paired with its
	// desk.cell (timing, module-surface rows) by time containment ----
	type trU struct {
		e          tev
		start, end time.Time
		used       bool
	}
	var trList []*trU
	var toolCalls []*cellUnit
	for _, e := range trs {
		tool := mStr(e.p, "tool")
		st := mTime(e.p, "started_at")
		d := time.Duration(mInt(e.p, "duration_ms")) * time.Millisecond
		if st.IsZero() {
			st = e.ts.Add(-d)
		}
		if tool == "desk_go_eval" {
			trList = append(trList, &trU{e: e, start: st, end: st.Add(d)})
			continue
		}
		// Any other tool: one step with its arguments and output.
		args := ""
		if a, ok := e.p["arguments"]; ok {
			b, _ := json.Marshal(a)
			args = string(b)
		} else {
			args = mStr(e.p, "arguments_raw")
		}
		out := mStr(e.p, "output")
		c := &traceRunCell{Tool: tool, Source: clipText(args, clipSource, in.Full), Output: clipText(out, clipOutput, in.Full),
			OutputBytes: mInt(e.p, "output_len"), IsError: mBool(e.p, "is_error"), CallID: mStr(e.p, "call_id")}
		s := base("tool_call", st, e.raw.Seq, e.raw.EventID, "tool "+tool)
		s.DurationMs = dur(d.Milliseconds())
		s.Cell = c
		toolCalls = append(toolCalls, &cellUnit{step: s, start: st, end: st.Add(d), seq: e.raw.Seq})
	}
	var cells []*cellUnit
	slack := time.Second
	for _, dc := range dcs {
		cs := mTime(dc.p, "started_at")
		cf := mTime(dc.p, "finished_at")
		if cs.IsZero() {
			cs = dc.ts.Add(-time.Duration(mInt(dc.p, "duration_ms")) * time.Millisecond)
		}
		if cf.IsZero() {
			cf = dc.ts
		}
		var best *trU
		var bestGap time.Duration
		for _, tr := range trList {
			if tr.used || cs.Before(tr.start.Add(-slack)) || cf.After(tr.end.Add(slack)) {
				continue
			}
			gap := cs.Sub(tr.start)
			if gap < 0 {
				gap = -gap
			}
			if best == nil || gap < bestGap {
				best, bestGap = tr, gap
			}
		}
		cell := &traceRunCell{Tool: "desk_go_eval", CellRecorded: true, Intents: int(mInt(dc.p, "intents")), KilledBy: mStr(dc.p, "killed_by"),
			WorkerExit: mStr(dc.p, "worker_exit"), Error: mStr(dc.p, "error")}
		seq := dc.raw.Seq
		id := dc.raw.EventID
		if best != nil {
			best.used = true
			fillCellFromResult(cell, best.e.p, in.Full)
			if best.e.raw.Seq < seq {
				seq = best.e.raw.Seq
			}
		} else {
			cell.SourceMissing = "no tool.result paired with this cell: retired after 7 days, or not recorded"
			cell.OutputMissing = cell.SourceMissing
		}
		s := base("cell", cs, seq, id, "cell")
		s.DurationMs = dur(cf.Sub(cs).Milliseconds())
		s.Cell = cell
		if d := mInt(dc.p, "duration_ms"); d > 0 {
			s.DurationMs = dur(d)
		}
		// The cell's module-surface rows.
		if rows, ok := dc.p["egress"].([]any); ok {
			for i, r := range rows {
				row, _ := r.(map[string]any)
				if row == nil {
					continue
				}
				s.Steps = append(s.Steps, callStep(row, fmt.Sprintf("%s#egress.%d", dc.raw.EventID, i), seq, start))
			}
		}
		if om, ok := dc.p["egress_omitted"].(map[string]any); ok {
			if totals.ModuleCallsOmitted == nil {
				totals.ModuleCallsOmitted = map[string]int{}
			}
			for verb, n := range om {
				if f, ok := n.(float64); ok {
					totals.ModuleCallsOmitted[verb] += int(f)
				}
			}
		}
		cells = append(cells, &cellUnit{step: s, start: cs, end: cf, seq: seq})
	}
	for _, tr := range trList {
		if tr.used {
			continue
		}
		cell := &traceRunCell{Tool: "desk_go_eval"}
		fillCellFromResult(cell, tr.e.p, in.Full)
		s := base("cell", tr.start, tr.e.raw.Seq, tr.e.raw.EventID, "cell")
		s.DurationMs = dur(tr.end.Sub(tr.start).Milliseconds())
		s.Cell = cell
		s.Note = "no desk.cell record for this cell (retired after 14 days, or not written): its module-surface calls are not shown"
		cells = append(cells, &cellUnit{step: s, start: tr.start, end: tr.end, seq: tr.e.raw.Seq})
	}
	cells = append(cells, toolCalls...)
	sort.SliceStable(cells, func(i, j int) bool {
		if !cells[i].start.Equal(cells[j].start) {
			return cells[i].start.Before(cells[j].start)
		}
		return cells[i].seq < cells[j].seq
	})

	// Standalone module-surface rows (refusals, late rows): into the cell
	// that was running, else top level.
	for _, e := range standalone {
		st := callStep(e.p, e.raw.EventID, e.raw.Seq, start)
		t := mTime(e.p, "started_at")
		if t.IsZero() {
			t = e.ts
		}
		var host *cellUnit
		for _, c := range cells {
			if !t.Before(c.start) && !t.After(c.end.Add(slack)) {
				host = c
			}
		}
		if host != nil {
			host.step.Steps = append(host.step.Steps, st)
		} else {
			addTop(t, 0, e.raw.Seq, st)
		}
	}

	// ---- search starts paired with search ends by order and time ----
	var starts []*traceRunStep
	collect := func(steps []traceRunStep) {
		for i := range steps {
			if steps[i].Kind == "module_call" && steps[i].Call != nil && isSearchVerb(steps[i].Call.Verb) {
				starts = append(starts, &steps[i])
			}
		}
	}
	for _, c := range cells {
		collect(c.step.Steps)
	}
	for i := range top {
		if top[i].step.Kind == "module_call" && top[i].step.Call != nil && isSearchVerb(top[i].step.Call.Verb) {
			starts = append(starts, &top[i].step)
		}
	}
	startTime := func(s *traceRunStep) time.Time { return parseTraceTime(s.At) }
	sort.SliceStable(starts, func(i, j int) bool { return startTime(starts[i]).Before(startTime(starts[j])) })
	matched := map[*traceRunStep]bool{}
	sort.SliceStable(dones, func(i, j int) bool {
		return mTime(dones[i].p, "started_at").Before(mTime(dones[j].p, "started_at"))
	})
	const maxPairGap = 10 * time.Second
	for _, d := range dones {
		ds := mTime(d.p, "started_at")
		if ds.IsZero() {
			ds = d.ts
		}
		df := mTime(d.p, "finished_at")
		if df.IsZero() {
			df = d.ts
		}
		pages := int(mInt(d.p, "pages_returned"))
		totals.PagesReturned += pages
		var pick *traceRunStep
		for _, s := range starts {
			if matched[s] || startTime(s).After(ds) || ds.Sub(startTime(s)) > maxPairGap {
				continue
			}
			if pick == nil || startTime(s).After(startTime(pick)) {
				pick = s
			}
		}
		end := &traceRunSearchEnd{SearchID: mStr(d.p, "search_id"), OffsetMs: off(df), DurationMs: df.Sub(ds).Milliseconds(),
			Answered: int(mInt(d.p, "answered")), Arrived: int(mInt(d.p, "arrived")), Total: int(mInt(d.p, "total")),
			PagesReturned: pages, Error: mStr(d.p, "error"), PairedBy: "order and time (the start row carries no search id)"}
		if pick != nil {
			matched[pick] = true
			pick.Call.End = end
			pick.Call.EndNote = ""
			totals.SearchesEnded++
			continue
		}
		s := base("module_call", df, d.raw.Seq, d.raw.EventID, "search end with no start row")
		s.Call = &traceRunCall{Verb: "search_done", End: end, EndNote: "no start row paired with this end"}
		addTop(df, 0, d.raw.Seq, s)
	}
	for _, s := range starts {
		if s.Call.End == nil {
			s.Call.EndNote = "no end recorded"
		}
	}

	// ---- turn commits, controls and work items from the lifecycle events ----
	var commits []traceRunLifecycleRow
	for _, le := range in.Lifecycle {
		t := parseTraceTime(le.CreatedAt)
		switch {
		case le.Kind == "texture_turn_committed":
			commits = append(commits, le)
			s := base("turn_commit", t, 0, le.EventID, "turn commit")
			if le.Reason != "" {
				s.Title += ": " + le.Reason
			} else {
				s.Title += " (no Reason stored)"
			}
			addTop(t, 1, le.ReducerSeq, s)
		case strings.HasPrefix(le.Kind, "control_") || strings.HasPrefix(le.Kind, "update_"):
			title := "control " + le.Kind
			if le.WorkItemID != "" {
				title += " (work item " + le.WorkItemID + ")"
			}
			if le.Reason != "" {
				title += ": " + le.Reason
			}
			s := base("control", t, 0, le.EventID, title)
			if le.UpdateID != "" {
				s.Fields = map[string]string{"update_id": le.UpdateID}
			}
			addTop(t, 1, le.ReducerSeq, s)
		case strings.HasPrefix(le.Kind, "work_"):
			title := "work item " + le.Kind
			if le.WorkItemID != "" {
				title += " " + le.WorkItemID
			}
			if le.Reason != "" {
				title += ": " + le.Reason
			}
			addTop(t, 1, le.ReducerSeq, base("work_item", t, 0, le.EventID, title))
		default:
			title := "lifecycle " + le.Kind
			if le.Reason != "" {
				title += ": " + le.Reason
			}
			addTop(t, 1, le.ReducerSeq, base("event", t, 0, le.EventID, title))
		}
	}

	// ---- records the run sent or received ----
	var candidate *traceRunRecordRow
	var candidateT time.Time
	runEnd := in.Now
	if run.FinishedAt != nil {
		if t := parseTraceTime(*run.FinishedAt); !t.IsZero() {
			runEnd = t
		}
	}
	seenRec := map[string]bool{}
	for i := range in.Records {
		r := in.Records[i]
		key := r.Inbox + "|" + r.RecordID
		if seenRec[key] {
			continue
		}
		seenRec[key] = true
		t := r.committedAt()
		sent := r.Provenance.RunID == in.RunID || r.Provenance.ActingRunID == in.RunID
		recv := !sent && in.OwnInbox != "" && r.Inbox == in.OwnInbox
		if recv && !t.IsZero() && !start.IsZero() && t.Before(start) {
			if candidate == nil || t.After(candidateT) {
				cp := r
				candidate, candidateT = &cp, t
			}
			continue
		}
		if !sent && !(recv && !t.IsZero() && !t.Before(start) && !t.After(runEnd.Add(2*time.Second))) {
			continue
		}
		rec := &traceRunRecord{Direction: "received", RecordID: r.RecordID, Kind: r.Kind, Inbox: r.Inbox, From: r.SenderInbox,
			Round: r.Round, Final: r.Final, PSuccess: r.PSuccess}
		if sent {
			rec.Direction = "sent"
		}
		if r.Directive != nil {
			rec.Subtype = r.Directive.Subtype
			if r.Directive.Objective != "" {
				rec.Summary = r.Directive.Objective
			}
		}
		if r.Cast != nil {
			for _, h := range r.Cast.Hypotheses {
				rec.Forecast = append(rec.Forecast, fmt.Sprintf("%s %s (p=%g)", h.ID, h.Statement, h.Probability))
			}
		}
		if r.Report != nil {
			rec.Report = r.Report.Summary
			if r.Report.StopReason != "" && rec.Summary == "" {
				rec.Summary = "stop: " + r.Report.StopReason
			}
		}
		kind := rec.Kind
		if rec.Subtype != "" {
			kind += "/" + rec.Subtype
		}
		dir := "sent record "
		to := " to " + rec.Inbox
		if rec.Direction == "received" {
			dir = "received record "
			to = " in " + rec.Inbox
		}
		s := base("record", t, 0, r.RecordID, dir+kind+to)
		s.Record = rec
		addTop(t, 1, r.InboxSeq, s)
	}

	// ---- cells under their model call; reasons ----
	sort.SliceStable(models, func(i, j int) bool { return models[i].ts.Before(models[j].ts) })
	ownerOf := func(t time.Time) *modelUnit {
		var o *modelUnit
		for _, m := range models {
			if !m.ts.After(t) {
				o = m
			}
		}
		return o
	}
	// A turn commit's Reason attaches to the cell that finished last before it.
	claimed := map[*cellUnit]*traceRunLifecycleRow{}
	sort.SliceStable(commits, func(i, j int) bool {
		return parseTraceTime(commits[i].CreatedAt).Before(parseTraceTime(commits[j].CreatedAt))
	})
	for ci := range commits {
		c := commits[ci]
		if strings.TrimSpace(c.Reason) == "" {
			continue
		}
		ct := parseTraceTime(c.CreatedAt)
		var pick *cellUnit
		for _, cu := range cells {
			if cu.step.Kind != "cell" || claimed[cu] != nil || cu.start.After(ct) || cu.end.After(ct.Add(500*time.Millisecond)) {
				continue
			}
			if pick == nil || cu.end.After(pick.end) {
				pick = cu
			}
		}
		if pick != nil {
			claimed[pick] = &commits[ci]
		}
	}
	var withReason, noReason int
	for _, mu := range models {
		if mu.text.Source == "not_recorded" {
			noReason++
		}
		withReason++
	}
	for _, cu := range cells {
		var reason traceRunReason
		owner := ownerOf(cu.start)
		switch {
		case claimed[cu] != nil:
			c := claimed[cu]
			t := clipText(c.Reason, clipWords, in.Full)
			reason = traceRunReason{Source: "turn_commit", Text: t.Text, Chars: t.Chars, Cut: t.Cut,
				Note: "attached by time: the turn commit " + c.EventID + " followed this cell (the commit carries no cell id)"}
		case owner != nil && owner.text.Source == "model_words":
			reason = owner.text
			reason.Note = "the model's text for the turn that issued this cell"
		case owner != nil:
			reason = traceRunReason{Source: "not_recorded", Note: "the turn's model text is empty"}
		default:
			reason = traceRunReason{Source: "not_recorded", Note: "no model call row precedes this cell (tool_loop rows retired or not read)"}
		}
		r := reason
		cu.step.Reason = &r
		withReason++
		if reason.Source == "not_recorded" {
			noReason++
		}
		for i := range cu.step.Steps {
			cu.step.Steps[i].Reason = &traceRunReason{Source: "cell", Note: "same as the cell's"}
		}
		sort.SliceStable(cu.step.Steps, func(i, j int) bool {
			return parseTraceTime(cu.step.Steps[i].At).Before(parseTraceTime(cu.step.Steps[j].At))
		})
		if owner != nil {
			owner.step.Steps = append(owner.step.Steps, cu.step)
		} else {
			addTop(cu.start, 0, cu.seq, cu.step)
		}
	}
	for _, mu := range models {
		addTop(parseTraceTime(mu.step.At), 0, mu.step.Seq, mu.step)
	}
	sort.SliceStable(top, func(i, j int) bool {
		if !top[i].t.Equal(top[j].t) {
			return top[i].t.Before(top[j].t)
		}
		if top[i].group != top[j].group {
			return top[i].group < top[j].group
		}
		return top[i].seq < top[j].seq
	})
	for _, o := range top {
		doc.Steps = append(doc.Steps, o.step)
	}

	// ---- totals ----
	var decisionReadMax int
	var researchCalls int
	for _, s := range flattenRunSteps(doc.Steps) {
		switch s.Kind {
		case "cell":
			totals.Cells++
			if s.Cell != nil && (s.Cell.IsError || s.Cell.Error != "") {
				totals.Errors++
			}
		case "tool_call":
			if s.Cell != nil && s.Cell.IsError {
				totals.Errors++
			}
		case "module_call":
			if s.Call == nil || s.Call.Verb == "search_done" {
				continue
			}
			totals.ModuleCalls[s.Call.Verb]++
			if s.Call.IsError {
				totals.Errors++
			}
			// The verb of choir.Research is the researcher desk's name (yaegikernel.ActionResearch).
			if s.Call.Verb == deskmanifest.ResearcherName() {
				researchCalls++
			}
			if isSearchVerb(s.Call.Verb) {
				totals.Searches++
				if d := s.Call.Decision; d != nil && d.PagesRead > decisionReadMax {
					decisionReadMax = d.PagesRead
				}
			}
		}
	}
	totals.PagesRead = decisionReadMax
	if run.Error != "" {
		totals.Errors++
	}
	doc.Totals = totals

	// ---- header ----
	hdr := traceRunHeader{RunID: in.RunID, AgentID: run.AgentID, AgentProfile: run.AgentProfile, AgentRole: run.AgentRole,
		Desk: firstNonEmptyStr(deskName, run.AgentProfile), ChannelID: run.ChannelID, TrajectoryID: run.TrajectoryID,
		ComputerID: run.ComputerID, State: run.State, CreatedAt: run.CreatedAt, ParentRunID: run.RequestedByRunID,
		ChildRuns: in.Children, ChildRunsNote: in.ChildrenNote, Error: run.Error,
		Prompt: clipText(run.Prompt, clipPrompt, in.Full), Result: clipText(run.Result, clipPrompt, in.Full)}
	for _, k := range modelOrder {
		hdr.Models = append(hdr.Models, *modelUse[k])
	}
	if run.FinishedAt != nil && *run.FinishedAt != "" {
		hdr.FinishedAt = *run.FinishedAt
		if !start.IsZero() {
			if fin := parseTraceTime(*run.FinishedAt); !fin.IsZero() {
				d := fin.Sub(start).Milliseconds()
				hdr.DurationMs = &d
			}
		}
	}
	if !terminalStates[run.State] && run.State != "blocked" && run.State != "passivated" {
		hdr.Live = true
		hdr.LiveAsOfSeq = maxSeq
	}
	hdr.WokenBy = wokenBy(run.Metadata, candidate)
	doc.Run = hdr

	// ---- retention: what the working store has retired for a run this old ----
	for _, rule := range traceRunRetentionRules {
		cutoff := in.Now.Add(-time.Duration(rule.days) * 24 * time.Hour)
		key := rule.kind
		if rule.phase != "" {
			key += "/" + rule.phase
		}
		present := rowsByKind[key]
		switch {
		case !runEnd.IsZero() && runEnd.Before(cutoff):
			doc.Retention = append(doc.Retention, traceRunRetention{Kind: rule.kind, Days: rule.days, Status: "gone", Rows: present,
				Text: fmt.Sprintf("%s rows (%s) are kept %d days; this run ended %s, before the cutoff %s, so they are retired%s",
					rule.kind, rule.note, rule.days, runEnd.Format(time.RFC3339), cutoff.Format(time.RFC3339), presentNote(present))})
		case !start.IsZero() && start.Before(cutoff):
			doc.Retention = append(doc.Retention, traceRunRetention{Kind: rule.kind, Days: rule.days, Status: "partial", Rows: present,
				Text: fmt.Sprintf("%s rows (%s) are kept %d days; this run began %s, before the cutoff %s, so its earlier rows are retired%s",
					rule.kind, rule.note, rule.days, start.Format(time.RFC3339), cutoff.Format(time.RFC3339), presentNote(present))})
		}
	}

	// ---- model calls: the tool_loop rows, else the model.served summary,
	// else "aged out" or "not recorded"; never a zero that means unknown ----
	for _, e := range servedRows {
		ms, _ := e.p["models"].([]any)
		for _, m := range ms {
			mm, _ := m.(map[string]any)
			if mm == nil {
				continue
			}
			u := traceRunServed{Purpose: mStr(e.p, "purpose"), Role: mStr(e.p, "role"), Provider: mStr(mm, "provider"), Model: mStr(mm, "model"),
				Calls: int(mInt(mm, "calls")), InputTokens: mInt(mm, "input_tokens"), OutputTokens: mInt(mm, "output_tokens")}
			doc.Run.Served = append(doc.Run.Served, u)
			if u.Role != "" {
				doc.Totals.ChildModelCalls += u.Calls
			}
		}
	}
	switch {
	case doc.Totals.ModelCalls > 0:
		doc.Totals.ModelCallsStatus = "rows"
	case len(doc.Run.Served) > 0:
		doc.Totals.ModelCallsStatus = "served_only"
		for _, u := range doc.Run.Served {
			doc.Totals.ModelCalls += u.Calls
			doc.Totals.InputTokens += u.InputTokens
			doc.Totals.OutputTokens += u.OutputTokens
		}
		doc.Totals.ChildModelCalls = 0
	default:
		doc.Totals.ModelCallsStatus = "not_recorded"
		for _, r := range doc.Retention {
			if r.Kind == "loop.progress" {
				doc.Totals.ModelCallsStatus = "aged_out"
			}
		}
	}

	doc.Partial = len(in.PartialWhy) > 0
	doc.PartialWhy = in.PartialWhy

	// ---- gaps: only what is absent in this run's data ----
	gap := func(id, text string) { doc.Gaps = append(doc.Gaps, traceRunGap{ID: id, Text: text}) }
	if noReason > 0 {
		gap("no_stated_reason", fmt.Sprintf("%d of %d model calls and cells have no stated reason: no turn-commit Reason and no model text before them. The record has no field for why a call was made.", noReason, withReason))
	}
	if doc.Totals.ModelCalls > 0 && !sawProviderCall {
		gap("prompt_not_stored", "The prompt each model call saw is not stored: the request-side event is live-only and absent here; only the shas and sizes on the model-call rows survive.")
	}
	if doc.Totals.Searches > 0 {
		gap("query_not_recorded", fmt.Sprintf("The query of each of the %d searches is not recorded: the module-surface row holds counts and the verb, never the query text (read the cell source for a literal).", doc.Totals.Searches))
		gap("search_start_no_search_id", "A search's start row carries no search id: starts and ends are paired by order and time, not by id.")
	}
	if !doc.Run.WokenBy.Recorded {
		gap("wake_record_not_recorded", "The record that woke this run is not stored: the run row holds at most a label (request_source), not the inbox record or act.")
	}
	if !sawDigest {
		gap("digest_ids_absent", "No event of this run names the captures handed to it (digest ids); they live in run memory (7 days, no route) and the console (about 3 hours).")
	}
	if len(doc.Retention) > 0 {
		var kinds []string
		for _, r := range doc.Retention {
			kinds = append(kinds, r.Kind)
		}
		gap("events_past_retention", "Events retired for a run this old: "+strings.Join(kinds, ", ")+". A quiet-looking step list for these kinds is not evidence that nothing happened.")
	}
	if doc.Totals.ModelCalls > 0 && !anyModelHasCallIDs(doc.Steps) {
		gap("tool_loop_no_call_ids", "A model-call row carries no tool call ids: a model call is joined to its cells by order and time only.")
	}
	for _, s := range doc.Steps {
		if s.Kind == "model_call" && s.Model != nil && s.Model.Words.StoredCut {
			gap("model_text_cut", "At least one model text was cut to 2000 characters by the writer before it was stored; the rest is not in the record.")
			break
		}
	}
	if doc.Totals.ModelCalls > 0 && !sawReasoning {
		gap("reasoning_text_absent", "No event of this run carries reasoning text (reasoning_content); run memory may hold it for 7 days but no route returns it.")
	}
	if researchCalls > 0 {
		gap("research_children_events", fmt.Sprintf("%d choir.Research calls were made: the events of those children are not attached to this run's record.", researchCalls))
	}
	if len(doc.Totals.ModuleCallsOmitted) > 0 {
		gap("module_rows_omitted", "The writer dropped module-surface rows past its bound; the record keeps only their count by verb.")
	}
	return doc
}

func presentNote(n int) string {
	if n == 0 {
		return " (none present)"
	}
	return fmt.Sprintf(" (%d rows still present)", n)
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func flattenRunSteps(steps []traceRunStep) []traceRunStep {
	var out []traceRunStep
	for _, s := range steps {
		out = append(out, s)
		out = append(out, flattenRunSteps(s.Steps)...)
	}
	return out
}

func anyModelHasCallIDs(steps []traceRunStep) bool {
	for _, s := range steps {
		if s.Kind == "model_call" && s.Note == "tool call ids recorded" {
			return true
		}
	}
	return false
}

func fillCellFromResult(cell *traceRunCell, p map[string]any, full bool) {
	cell.CallID = mStr(p, "call_id")
	cell.IsError = mBool(p, "is_error")
	cell.OutputBytes = mInt(p, "output_len")
	source := ""
	if a, ok := p["arguments"].(map[string]any); ok {
		source = mStr(a, "source")
	}
	if source == "" {
		source = mStr(p, "arguments_raw")
	}
	cell.Source = clipText(source, clipSource, full)
	if source == "" {
		cell.SourceMissing = "the tool.result holds no source"
	}
	cell.Output = clipText(mStr(p, "output"), clipOutput, full)
}

// callStep builds one module-surface call step from a row.
func callStep(row map[string]any, id string, seq int64, start time.Time) traceRunStep {
	verb := mStr(row, "verb")
	st := mTime(row, "started_at")
	fin := mTime(row, "finished_at")
	c := &traceRunCall{Verb: verb, IsError: mBool(row, "is_error"), Error: mStr(row, "error")}
	if isSearchVerb(verb) {
		c.Query = "not recorded"
		if d, ok := row["decision"].(map[string]any); ok {
			c.Decision = &traceRunDecision{Verb: mStr(d, "search_verb"), EarlierSearches: int(mInt(d, "earlier_searches")),
				EarlierPagesNew: int(mInt(d, "earlier_pages_new")), PagesTried: int(mInt(d, "pages_tried")),
				PagesRead: int(mInt(d, "pages_read")), OutletsRead: int(mInt(d, "outlets_read"))}
		}
	}
	s := traceRunStep{Kind: "module_call", Seq: seq, EventID: id, Title: verb, Call: c}
	if !st.IsZero() {
		s.At = st.Format(time.RFC3339Nano)
		if !start.IsZero() {
			s.OffsetMs = st.Sub(start).Milliseconds()
		}
		if !fin.IsZero() {
			d := fin.Sub(st).Milliseconds()
			s.DurationMs = &d
		}
	}
	return s
}

var wakeRecordKeys = []string{"route_record_id", "woken_by_record", "woken_by_record_id", "wake_record_id"}
var wakeOtherKeys = []string{"lifecycle_work_item_id", "actor_rewarm_source_loop_id", "route_destination", "route_reason"}

func wokenBy(meta map[string]any, candidate *traceRunRecordRow) traceRunWoke {
	w := traceRunWoke{Fields: map[string]string{}}
	w.RequestSource = mStr(meta, "request_source")
	for _, k := range wakeRecordKeys {
		if v := mStr(meta, k); v != "" {
			w.Fields[k] = v
			w.Recorded = true
		}
	}
	for _, k := range wakeOtherKeys {
		if v := mStr(meta, k); v != "" {
			w.Fields[k] = v
		}
	}
	if len(w.Fields) == 0 {
		w.Fields = nil
	}
	switch {
	case w.Recorded:
		var parts []string
		for _, k := range wakeRecordKeys {
			if v := w.Fields[k]; v != "" {
				parts = append(parts, k+"="+v)
			}
		}
		w.Text = "woken by: " + strings.Join(parts, ", ")
		if w.RequestSource != "" {
			w.Text += " (request_source=" + w.RequestSource + ")"
		}
	case w.RequestSource != "":
		w.Text = "woken by: request_source=" + w.RequestSource + " (a label; the waking record is not recorded)"
	default:
		w.Text = "woken by: not recorded"
	}
	if candidate != nil {
		w.Candidate = fmt.Sprintf("%s (%s, inbox %s seq %d, committed %s): the newest record received before the run started (inferred; the run row does not record what woke it)",
			candidate.RecordID, candidate.Kind, candidate.Inbox, candidate.InboxSeq, candidate.committedAt().Format(time.RFC3339))
	}
	return w
}
