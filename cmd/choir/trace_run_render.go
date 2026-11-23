package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

func fmtOffset(ms int64) string {
	sign := "+"
	if ms < 0 {
		sign, ms = "-", -ms
	}
	return fmt.Sprintf("%s%d.%03ds", sign, ms/1000, ms%1000)
}

func fmtDur(ms int64) string {
	switch {
	case ms < 1000:
		return fmt.Sprintf("%dms", ms)
	case ms < 60000:
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	default:
		return fmt.Sprintf("%dm%02ds", ms/60000, (ms%60000)/1000)
	}
}

func cutMarks(chars int, cut, storedCut bool) string {
	var b strings.Builder
	if cut {
		fmt.Fprintf(&b, " [clipped: showing a head of the %d chars; --full shows all]", chars)
	}
	if storedCut {
		fmt.Fprintf(&b, " [CUT by the writer before it was stored: the record holds a head of the %d chars]", chars)
	}
	return b.String()
}

func writeBlock(w io.Writer, indent, label, text string) {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	fmt.Fprintf(w, "%s%s\n", indent, label)
	for _, ln := range lines {
		fmt.Fprintf(w, "%s  | %s\n", indent, ln)
	}
}

func renderTraceRun(w io.Writer, d traceRunDoc, full bool) {
	h := d.Run
	fmt.Fprintf(w, "RUN %s\n", h.RunID)
	if d.Partial {
		fmt.Fprintln(w, "*** PARTIAL: this run was not read in full. What is missing:")
		for _, why := range d.PartialWhy {
			fmt.Fprintf(w, "***   - %s\n", why)
		}
	}
	agent := h.AgentID
	if h.AgentProfile != "" || h.AgentRole != "" {
		agent += fmt.Sprintf("  (profile %s, role %s)", h.AgentProfile, h.AgentRole)
	}
	fmt.Fprintf(w, "agent        %s\n", agent)
	if h.Desk != "" {
		fmt.Fprintf(w, "desk         %s\n", h.Desk)
	}
	fmt.Fprintf(w, "computer     %s\n", h.ComputerID)
	if h.ChannelID != "" || h.TrajectoryID != "" {
		fmt.Fprintf(w, "story        channel %s, trajectory %s\n", h.ChannelID, h.TrajectoryID)
	}
	state := h.State
	if h.Live {
		state += fmt.Sprintf("  (live, as of seq %d; the newest events lag the telemetry window by up to 2 s; no end is shown)", h.LiveAsOfSeq)
	}
	fmt.Fprintf(w, "state        %s\n", state)
	fmt.Fprintf(w, "started      %s\n", h.CreatedAt)
	if h.FinishedAt != "" {
		fmt.Fprintf(w, "ended        %s\n", h.FinishedAt)
	}
	if h.DurationMs != nil {
		fmt.Fprintf(w, "duration     %s\n", fmtDur(*h.DurationMs))
	}
	if h.ParentRunID != "" {
		fmt.Fprintf(w, "parent run   %s\n", h.ParentRunID)
	}
	if len(h.ChildRuns) > 0 {
		fmt.Fprintf(w, "child runs   %s  (%s)\n", strings.Join(h.ChildRuns, ", "), h.ChildRunsNote)
	} else if h.ChildRunsNote != "" {
		fmt.Fprintf(w, "child runs   none found  (%s)\n", h.ChildRunsNote)
	}
	if len(h.Models) > 0 {
		var ms []string
		for _, m := range h.Models {
			ms = append(ms, fmt.Sprintf("%s/%s x%d", m.Provider, m.Model, m.Calls))
		}
		fmt.Fprintf(w, "models       %s\n", strings.Join(ms, ", "))
	} else if len(h.Served) > 0 {
		fmt.Fprintf(w, "models       %s\n", servedLine(h.Served))
	} else if d.Totals.ModelCallsStatus == "aged_out" {
		fmt.Fprintln(w, "models       aged out (the model-call rows are kept 14 days; this run is older)")
	} else {
		fmt.Fprintln(w, "models       not recorded (no model-call or model.served rows in this run's events)")
	}
	var kids []traceRunServed
	for _, u := range h.Served {
		if u.Role != "" {
			kids = append(kids, u)
		}
	}
	if len(kids) > 0 && len(h.Models) > 0 {
		fmt.Fprintf(w, "children     %s (from model.served; their calls are in no row of this run)\n", servedLine(kids))
	}
	fmt.Fprintf(w, "%s\n", h.WokenBy.Text)
	if h.WokenBy.Candidate != "" {
		fmt.Fprintf(w, "             candidate: %s\n", h.WokenBy.Candidate)
	}
	t := d.Totals
	switch t.ModelCallsStatus {
	case "served_only":
		fmt.Fprintf(w, "totals       model calls %d (from model.served; per-call rows not recorded) | tokens in %d out %d | cells %d | errors %d\n",
			t.ModelCalls, t.InputTokens, t.OutputTokens, t.Cells, t.Errors)
	case "aged_out", "not_recorded":
		why := "not recorded"
		if t.ModelCallsStatus == "aged_out" {
			why = "aged out"
		}
		fmt.Fprintf(w, "totals       model calls %s, tokens %s | cells %d | errors %d\n", why, why, t.Cells, t.Errors)
	default:
		child := ""
		if t.ChildModelCalls > 0 {
			child = fmt.Sprintf(" (+%d by children, from model.served)", t.ChildModelCalls)
		}
		fmt.Fprintf(w, "totals       model calls %d%s | tokens in %d out %d cached %d | cells %d | errors %d\n",
			t.ModelCalls, child, t.InputTokens, t.OutputTokens, t.CachedTokens, t.Cells, t.Errors)
	}
	var verbs []string
	for v := range t.ModuleCalls {
		verbs = append(verbs, v)
	}
	sort.Strings(verbs)
	var vs []string
	for _, v := range verbs {
		vs = append(vs, fmt.Sprintf("%s %d", v, t.ModuleCalls[v]))
	}
	if len(vs) == 0 {
		vs = []string{"none"}
	}
	fmt.Fprintf(w, "             module-surface calls: %s\n", strings.Join(vs, ", "))
	if len(t.ModuleCallsOmitted) > 0 {
		var om []string
		for v, n := range t.ModuleCallsOmitted {
			om = append(om, fmt.Sprintf("%s %d", v, n))
		}
		sort.Strings(om)
		fmt.Fprintf(w, "             rows the writer dropped past its bound: %s\n", strings.Join(om, ", "))
	}
	fmt.Fprintf(w, "             searches %d (ended %d) | pages returned %d | pages read %d (as of the last search with counts)\n",
		t.Searches, t.SearchesEnded, t.PagesReturned, t.PagesRead)
	if h.Prompt.Chars > 0 {
		writeBlock(w, "", fmt.Sprintf("prompt (%d chars)%s:", h.Prompt.Chars, cutMarks(len([]rune(h.Prompt.Text)), h.Prompt.Cut, false)), h.Prompt.Text)
	}
	if h.Result.Chars > 0 {
		writeBlock(w, "", fmt.Sprintf("result (%d chars)%s:", h.Result.Chars, cutMarks(len([]rune(h.Result.Text)), h.Result.Cut, false)), h.Result.Text)
	}
	if h.Error != "" {
		fmt.Fprintf(w, "error        %s\n", h.Error)
	}
	if len(d.Retention) > 0 {
		fmt.Fprintln(w, "\nRetention (events are kept 7 or 14 days by kind; these are gone or partly gone for this run):")
		for _, r := range d.Retention {
			fmt.Fprintf(w, "  %s (%d days): %s: %s\n", r.Kind, r.Days, r.Status, r.Text)
		}
	}
	fmt.Fprintf(w, "\nSteps (%d top-level, in time order; offsets from the run's start)\n\n", len(d.Steps))
	for _, s := range d.Steps {
		renderStep(w, s, 0)
	}
	fmt.Fprintln(w, "\nRoutes read")
	for _, r := range d.Reads {
		line := fmt.Sprintf("  %-8s %s", r.Status, r.Route)
		if r.Items > 0 {
			line += fmt.Sprintf(" (%d items)", r.Items)
		}
		if r.Detail != "" {
			line += ": " + r.Detail
		}
		fmt.Fprintln(w, line)
	}
	fmt.Fprintln(w, "\nWhat this run's record does not contain")
	if len(d.Gaps) == 0 {
		fmt.Fprintln(w, "  (nothing absent that this view looks for)")
	}
	for _, g := range d.Gaps {
		fmt.Fprintf(w, "  - %s\n", g.Text)
	}
}

func renderReason(w io.Writer, ind string, r *traceRunReason) {
	if r == nil {
		return
	}
	switch r.Source {
	case "turn_commit":
		fmt.Fprintf(w, "%swhy: turn commit: %q%s\n", ind, r.Text, cutMarks(len([]rune(r.Text)), r.Cut, false))
		if r.Note != "" {
			fmt.Fprintf(w, "%s        (%s)\n", ind, r.Note)
		}
	case "model_words":
		fmt.Fprintf(w, "%swhy: the model said before this cell: %q%s\n", ind, r.Text, cutMarks(r.Chars, r.Cut, r.StoredCut))
	default:
		// "cell" (as the cell's) and "not_recorded" print nothing: what the
		// record lacks is listed once, at the end.
	}
}

func renderStep(w io.Writer, s traceRunStep, depth int) {
	ind := strings.Repeat("  ", depth)
	head := fmt.Sprintf("%s%-9s ", ind, fmtOffset(s.OffsetMs))
	dur := ""
	if s.DurationMs != nil {
		dur = "  " + fmtDur(*s.DurationMs)
	}
	body := ind + "          "
	switch s.Kind {
	case "model_call":
		m := s.Model
		fmt.Fprintf(w, "%sMODEL CALL  %s%s  in %d out %d cached %d  stop=%s  calls=%d %s\n", head, s.Title, dur,
			m.InputTokens, m.OutputTokens, m.CachedTokens, m.StopReason, m.ToolCalls, strings.Join(m.ToolNames, ","))
		if strings.TrimSpace(m.Words.Text) != "" {
			writeBlock(w, body, "said:"+cutMarks(m.Words.Chars, m.Words.Cut, m.Words.StoredCut), m.Words.Text)
		}
	case "cell", "tool_call":
		c := s.Cell
		label := "CELL"
		if s.Kind == "tool_call" {
			label = "TOOL " + c.Tool
		}
		errText := ""
		if c.IsError || c.Error != "" {
			errText = "  ERROR " + firstNonEmptyStr(c.Error, "(is_error on the tool result)")
		}
		fmt.Fprintf(w, "%s%s%s%s%s\n", head, label, dur, errText, killNote(c))
		if s.Note != "" {
			fmt.Fprintf(w, "%snote: %s\n", body, s.Note)
		}
		if c.SourceMissing != "" {
			fmt.Fprintf(w, "%ssource: not available (%s)\n", body, c.SourceMissing)
		} else {
			writeBlock(w, body, "source:"+cutMarks(c.Source.Chars, c.Source.Cut, false)+fmt.Sprintf(" (%d chars)", c.Source.Chars), c.Source.Text)
		}
		if c.OutputMissing != "" {
			fmt.Fprintf(w, "%soutput: not available (%s)\n", body, c.OutputMissing)
		} else {
			writeBlock(w, body, fmt.Sprintf("output (%d bytes)%s:", c.OutputBytes, cutMarks(c.Output.Chars, c.Output.Cut, false)), c.Output.Text)
		}
		renderReason(w, body, s.Reason)
		for _, ch := range s.Steps {
			renderStep(w, ch, depth+1)
		}
	case "module_call":
		c := s.Call
		errText := ""
		if c.IsError {
			errText = "  ERROR " + c.Error
		}
		fmt.Fprintf(w, "%s%s%s%s\n", head, c.Verb, dur, errText)
		if c.Query != "" && c.Query != "not recorded" {
			fmt.Fprintf(w, "%squery: %s\n", body, c.Query)
		}
		if d := c.Decision; d != nil {
			fmt.Fprintf(w, "%scounts: earlier searches %d, earlier pages new %d, pages tried %d, pages read %d, outlets read %d\n", body,
				d.EarlierSearches, d.EarlierPagesNew, d.PagesTried, d.PagesRead, d.OutletsRead)
		}
		if e := c.End; e != nil {
			fmt.Fprintf(w, "%send: %s (%s) answered %d/%d providers, arrived %d, pages returned %d%s [paired by %s]\n", body,
				fmtOffset(e.OffsetMs), fmtDur(e.DurationMs), e.Answered, e.Total, e.Arrived, e.PagesReturned, errSuffix(e.Error), e.PairedBy)
		}
		if c.EndNote != "" {
			fmt.Fprintf(w, "%s%s\n", body, c.EndNote)
		}
		renderReason(w, body, s.Reason)
	case "record":
		fmt.Fprintf(w, "%s%s\n", head, s.Title)
		r := s.Record
		fmt.Fprintf(w, "%srecord %s, round %d%s, from %s\n", body, r.RecordID, r.Round, finalNote(r.Final), r.From)
		for _, f := range r.Forecast {
			fmt.Fprintf(w, "%sforecast: %s\n", body, f)
		}
		if r.PSuccess != nil {
			fmt.Fprintf(w, "%sp_success: %g\n", body, *r.PSuccess)
		}
		if r.Summary != "" {
			fmt.Fprintf(w, "%s%s\n", body, r.Summary)
		}
		if r.Report != "" {
			fmt.Fprintf(w, "%sReport: %s\n", body, r.Report)
		}
	default:
		fmt.Fprintf(w, "%s%s%s\n", head, s.Title, dur)
		if s.Note != "" {
			fmt.Fprintf(w, "%s%s\n", body, s.Note)
		}
		if len(s.Fields) > 0 {
			var keys []string
			for k := range s.Fields {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			var kv []string
			for _, k := range keys {
				kv = append(kv, k+"="+s.Fields[k])
			}
			fmt.Fprintf(w, "%s%s\n", body, strings.Join(kv, " "))
		}
	}
	if s.Kind == "model_call" {
		for _, ch := range s.Steps {
			renderStep(w, ch, depth+1)
		}
	}
}

func errSuffix(e string) string {
	if e == "" {
		return ""
	}
	return " ERROR " + e
}

func finalNote(f bool) string {
	if f {
		return " (final)"
	}
	return ""
}

func killNote(c *traceRunCell) string {
	var parts []string
	if c.KilledBy != "" {
		parts = append(parts, "killed by "+c.KilledBy)
	}
	if c.WorkerExit != "" {
		parts = append(parts, "worker exit: "+c.WorkerExit)
	}
	if len(parts) == 0 {
		return ""
	}
	return "  [" + strings.Join(parts, "; ") + "]"
}

func servedLine(us []traceRunServed) string {
	var ps []string
	for _, u := range us {
		who := u.Purpose
		if u.Role != "" {
			who = u.Role
		}
		if who == "" {
			who = "run"
		}
		ps = append(ps, fmt.Sprintf("%s: %s/%s x%d", who, u.Provider, u.Model, u.Calls))
	}
	return strings.Join(ps, ", ")
}
