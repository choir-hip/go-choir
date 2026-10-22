package main

// `choir trace export <trajectory-id> [--out <file>]` writes one trace bundle
// (docs/reports/reeng-trace-bundle-format-2026-10-12.md) read only through
// product API routes under the caller's scoped key: no SSH, no /internal/*.
// The guest store stays the authority; the bundle is a projection. Bundles
// are the owner's data, written unredacted and outside the repo.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	traceBundleSchema    = "choir.trace_bundle.v1"
	traceStatusPresent   = "present"
	traceStatusEmpty     = "empty"
	traceStatusUnserved  = "unserved"
	traceRunListCap      = 500
	traceEventPageLimit  = 1000
	traceMaxPages        = 100000
	tracePublicationNote = "platform publication accepted"
)

type traceBuild struct {
	Commit         string `json:"commit"`
	DeployedCommit string `json:"deployed_commit"`
	BuiltAt        string `json:"built_at"`
}

type traceLayer struct {
	Status  string            `json:"status"`
	Sources []string          `json:"sources"`
	Note    string            `json:"note"`
	Items   []json.RawMessage `json:"items"`
}

type traceBundle struct {
	Schema       string                 `json:"schema"`
	ExportedAt   string                 `json:"exported_at"`
	BaseURL      string                 `json:"base_url"`
	ComputerID   string                 `json:"computer_id"`
	TrajectoryID string                 `json:"trajectory_id"`
	Build        traceBuild             `json:"build"`
	Drill        json.RawMessage        `json:"drill"`
	Layers       map[string]*traceLayer `json:"layers"`
}

// layerBuilder accumulates one layer. Any failure marks the layer unserved;
// otherwise it is present when it holds items and empty when it does not.
type layerBuilder struct {
	sources  []string
	failures []string
	notes    []string
	items    []json.RawMessage
}

func newLayer(sources ...string) *layerBuilder {
	return &layerBuilder{sources: sources}
}

func (b *layerBuilder) fail(format string, args ...any) {
	b.failures = append(b.failures, fmt.Sprintf(format, args...))
}

func (b *layerBuilder) note(format string, args ...any) {
	b.notes = append(b.notes, fmt.Sprintf(format, args...))
}

func (b *layerBuilder) add(items ...json.RawMessage) {
	b.items = append(b.items, items...)
}

func (b *layerBuilder) failed() bool { return len(b.failures) > 0 }

func (b *layerBuilder) finish() *traceLayer {
	status := traceStatusEmpty
	switch {
	case len(b.failures) > 0:
		status = traceStatusUnserved
	case len(b.items) > 0:
		status = traceStatusPresent
	}
	items := b.items
	if items == nil {
		items = []json.RawMessage{}
	}
	sources := b.sources
	if sources == nil {
		sources = []string{}
	}
	return &traceLayer{
		Status:  status,
		Sources: sources,
		Note:    strings.Join(append(append([]string{}, b.failures...), b.notes...), "; "),
		Items:   items,
	}
}

func unservedLayer(note string, sources ...string) *traceLayer {
	b := newLayer(sources...)
	b.fail("%s", note)
	return b.finish()
}

type traceExporter struct {
	c            *client
	trajectoryID string
}

func (e *traceExporter) get(path string, out any) error {
	return e.c.do(http.MethodGet, path, nil, out)
}

func routeFailure(path string, err error) string {
	var apiErr *apiErrorResp
	if errors.As(err, &apiErr) {
		return fmt.Sprintf("GET %s: HTTP %d: %s", path, apiErr.Status, truncate(strings.TrimSpace(apiErr.Body), 200))
	}
	return fmt.Sprintf("GET %s: %v", path, err)
}

func isNotFound(err error) bool {
	var apiErr *apiErrorResp
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound
}

// ---- id discovery ----

func isRunIDKey(key string) bool {
	switch key {
	case "ci_run_id", "deploy_run_id":
		return false
	case "run_id", "loop_id", "run_ids", "loop_ids":
		return true
	}
	for _, suffix := range []string{"_run_id", "_loop_id", "_run_ids", "_loop_ids"} {
		if strings.HasSuffix(key, suffix) {
			return true
		}
	}
	return false
}

func isDocIDKey(key string) bool {
	switch key {
	case "doc_id", "document_id", "doc_ids":
		return true
	}
	return strings.HasSuffix(key, "_doc_id") || strings.HasSuffix(key, "_doc_ids")
}

// collectIDs walks a JSON value and records every non-empty string (or
// string array element) found under a key the matcher accepts.
func collectIDs(raw json.RawMessage, match func(string) bool, into map[string]bool) {
	if len(raw) == 0 {
		return
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return
	}
	var walk func(any)
	walk = func(node any) {
		switch n := node.(type) {
		case map[string]any:
			for key, value := range n {
				if match(key) {
					switch typed := value.(type) {
					case string:
						if id := strings.TrimSpace(typed); id != "" {
							into[id] = true
						}
					case []any:
						for _, el := range typed {
							if s, ok := el.(string); ok && strings.TrimSpace(s) != "" {
								into[strings.TrimSpace(s)] = true
							}
						}
					}
				}
				walk(value)
			}
		case []any:
			for _, el := range n {
				walk(el)
			}
		}
	}
	walk(v)
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ---- export ----

func (e *traceExporter) export(now time.Time) *traceBundle {
	tid := url.PathEscape(e.trajectoryID)
	bundle := &traceBundle{
		Schema:       traceBundleSchema,
		ExportedAt:   now.UTC().Format(time.RFC3339),
		BaseURL:      e.c.host,
		ComputerID:   e.c.computerID,
		TrajectoryID: e.trajectoryID,
		Drill:        nil,
		Layers:       map[string]*traceLayer{},
	}

	// trajectory: one LifecycleSnapshot.
	trajectoryPath := "/api/trajectories/" + tid
	trajectory := newLayer("GET /api/trajectories/{id}")
	var snapshot json.RawMessage
	if err := e.get(trajectoryPath, &snapshot); err != nil {
		trajectory.fail("%s", routeFailure(trajectoryPath, err))
	} else {
		trajectory.add(snapshot)
		var probe struct {
			Schema string `json:"schema"`
		}
		if json.Unmarshal(snapshot, &probe) == nil && probe.Schema == "" {
			trajectory.note("no durable lifecycle snapshot; the route answered with its obligations view")
		}
	}
	bundle.Layers["trajectory"] = trajectory.finish()

	// lifecycle_events: every page, in reducer_seq order.
	lifecycle := newLayer("GET /api/trajectories/{id}/events?after=&limit=1000")
	after := int64(0)
	for pages := 0; ; pages++ {
		if pages >= traceMaxPages {
			lifecycle.fail("lifecycle event paging exceeded %d pages", traceMaxPages)
			break
		}
		path := fmt.Sprintf("/api/trajectories/%s/events?after=%d&limit=%d", tid, after, traceEventPageLimit)
		var page struct {
			Events     []json.RawMessage `json:"events"`
			NextCursor int64             `json:"next_cursor"`
			Watermark  int64             `json:"watermark"`
		}
		if err := e.get(path, &page); err != nil {
			lifecycle.fail("%s", routeFailure(path, err))
			break
		}
		lifecycle.add(page.Events...)
		if len(page.Events) == 0 || page.NextCursor >= page.Watermark {
			break
		}
		if page.NextCursor <= after {
			lifecycle.fail("GET %s: cursor did not advance past %d", path, after)
			break
		}
		after = page.NextCursor
	}
	bundle.Layers["lifecycle_events"] = lifecycle.finish()

	// runs: the trajectory filter plus every run named by the snapshot or an
	// event.
	runs := newLayer("GET /api/runs?trajectory_id=", "GET /api/runs/{id}")
	runIDs := map[string]bool{}
	listPath := fmt.Sprintf("/api/runs?trajectory_id=%s&limit=%d", url.QueryEscape(e.trajectoryID), traceRunListCap)
	var list struct {
		Runs []struct {
			RunID string `json:"run_id"`
		} `json:"runs"`
	}
	if err := e.get(listPath, &list); err != nil {
		runs.fail("%s", routeFailure(listPath, err))
	} else {
		for _, run := range list.Runs {
			if id := strings.TrimSpace(run.RunID); id != "" {
				runIDs[id] = true
			}
		}
		if len(list.Runs) >= traceRunListCap {
			runs.note("run list hit its %d-run cap; older runs appear only if an event or the snapshot names them", traceRunListCap)
		}
	}
	collectIDs(snapshot, isRunIDKey, runIDs)
	for _, ev := range bundle.Layers["lifecycle_events"].Items {
		collectIDs(ev, isRunIDKey, runIDs)
	}
	if bundle.Layers["trajectory"].Status == traceStatusUnserved || bundle.Layers["lifecycle_events"].Status == traceStatusUnserved {
		runs.note("run discovery is partial: a snapshot or lifecycle event source is unserved")
	}
	var resolvedRuns []string
	var unresolvedRuns []string
	for _, id := range sortedKeys(runIDs) {
		path := "/api/runs/" + url.PathEscape(id)
		var rec json.RawMessage
		if err := e.get(path, &rec); err != nil {
			if isNotFound(err) {
				unresolvedRuns = append(unresolvedRuns, id)
				continue
			}
			runs.fail("%s", routeFailure(path, err))
			continue
		}
		runs.add(rec)
		resolvedRuns = append(resolvedRuns, id)
	}
	if len(unresolvedRuns) > 0 {
		runs.note("ids named but not readable as runs (404): %s", strings.Join(unresolvedRuns, ", "))
	}
	bundle.Layers["runs"] = runs.finish()

	// run_events: every page of every run, ordered by (loop_id, seq).
	runEvents := newLayer("GET /api/runs/{id}/events?after=&limit=1000")
	if bundle.Layers["runs"].Status == traceStatusUnserved && len(resolvedRuns) == 0 {
		runEvents.fail("runs is unserved")
	} else if bundle.Layers["runs"].Status == traceStatusUnserved {
		runEvents.fail("runs is unserved; events cover only the runs that resolved")
	}
	for _, id := range resolvedRuns {
		after := int64(0)
		for pages := 0; ; pages++ {
			if pages >= traceMaxPages {
				runEvents.fail("run %s event paging exceeded %d pages", id, traceMaxPages)
				break
			}
			path := fmt.Sprintf("/api/runs/%s/events?after=%d&limit=%d", url.PathEscape(id), after, traceEventPageLimit)
			var page struct {
				Events    []json.RawMessage `json:"events"`
				NextAfter int64             `json:"next_after"`
			}
			if err := e.get(path, &page); err != nil {
				runEvents.fail("%s", routeFailure(path, err))
				break
			}
			runEvents.add(page.Events...)
			if page.NextAfter == 0 {
				break
			}
			if page.NextAfter <= after {
				runEvents.fail("GET %s: cursor did not advance past %d", path, after)
				break
			}
			after = page.NextAfter
		}
	}
	sortRunEvents(runEvents.items)
	bundle.Layers["run_events"] = runEvents.finish()

	bundle.Layers["cells"] = deriveRunEventKind(bundle.Layers["run_events"], "desk.cell")
	bundle.Layers["egress"] = deriveEgress(bundle.Layers["run_events"])
	bundle.Layers["deliveries"] = deriveDeliveries(bundle.Layers["trajectory"])
	bundle.Layers["turn_commits"] = deriveTurnCommits(bundle.Layers["lifecycle_events"])

	// revisions: every Texture document the trajectory touched.
	revisions := newLayer("GET /api/texture/documents/{id}/revisions")
	docIDs := map[string]bool{}
	collectIDs(snapshot, isDocIDKey, docIDs)
	for _, ev := range bundle.Layers["lifecycle_events"].Items {
		collectIDs(ev, isDocIDKey, docIDs)
	}
	for _, rec := range bundle.Layers["runs"].Items {
		collectIDs(rec, isDocIDKey, docIDs)
	}
	if bundle.Layers["trajectory"].Status == traceStatusUnserved {
		revisions.fail("trajectory is unserved, so the head document is unknown")
	}
	if bundle.Layers["runs"].Status == traceStatusUnserved || bundle.Layers["lifecycle_events"].Status == traceStatusUnserved {
		revisions.note("document discovery is partial: runs or lifecycle_events is unserved")
	}
	var unresolvedDocs []string
	for _, docID := range sortedKeys(docIDs) {
		path := "/api/texture/documents/" + url.PathEscape(docID) + "/revisions?limit=10000"
		var resp struct {
			Revisions []json.RawMessage `json:"revisions"`
		}
		if err := e.get(path, &resp); err != nil {
			if isNotFound(err) {
				unresolvedDocs = append(unresolvedDocs, docID)
				continue
			}
			revisions.fail("%s", routeFailure(path, err))
			continue
		}
		revisions.add(resp.Revisions...)
	}
	if len(docIDs) == 0 {
		revisions.note("no Texture document named by the snapshot, lifecycle events or runs")
	}
	if len(unresolvedDocs) > 0 {
		revisions.note("ids named but not readable as Texture documents (404): %s", strings.Join(unresolvedDocs, ", "))
	}
	bundle.Layers["revisions"] = revisions.finish()

	bundle.Layers["records"], bundle.Layers["inboxes"] = e.exportInboxRecords(bundle, docIDs)
	bundle.Layers["publications"] = e.exportPublications(bundle, docIDs)
	bundle.Layers["search_arrivals"] = unservedLayer("async search arrivals have no product route until Cut 2")
	bundle.Layers["research_children"] = unservedLayer("choir.Research children run without a run record and emit no events")
	bundle.Layers["captures"] = e.exportCaptures(bundle, now)

	// obligations and boot.
	obligations := newLayer("GET /api/runtime/obligations")
	var obligationsBody json.RawMessage
	if err := e.get("/api/runtime/obligations", &obligationsBody); err != nil {
		obligations.fail("%s", routeFailure("/api/runtime/obligations", err))
	} else {
		obligations.add(obligationsBody)
	}
	bundle.Layers["obligations"] = obligations.finish()

	// census: the computer's retired work-state objects (work items and
	// delivery packets) with their routes, the checker's authority for I7
	// once the trajectory snapshot no longer carries them (C1e B0).
	census := newLayer("GET /api/runtime/census")
	var censusBody json.RawMessage
	if err := e.get("/api/runtime/census", &censusBody); err != nil {
		census.fail("%s", routeFailure("/api/runtime/census", err))
	} else {
		census.add(censusBody)
	}
	bundle.Layers["census"] = census.finish()

	boot := newLayer("GET /api/runtime/observability", "GET /api/runtime/obligations")
	var observability struct {
		ComputerID string          `json:"computer_id"`
		Build      json.RawMessage `json:"build"`
		RecentLog  json.RawMessage `json:"recent_log"`
	}
	if err := e.get("/api/runtime/observability", &observability); err != nil {
		boot.fail("%s", routeFailure("/api/runtime/observability", err))
	} else {
		var restart json.RawMessage = json.RawMessage("null")
		if len(obligationsBody) > 0 {
			var o struct {
				Restart json.RawMessage `json:"restart"`
			}
			if json.Unmarshal(obligationsBody, &o) == nil && len(o.Restart) > 0 {
				restart = o.Restart
			}
		} else {
			boot.note("restart kind unknown: obligations is unserved")
		}
		item, _ := json.Marshal(map[string]any{
			"computer_id": observability.ComputerID,
			"build":       observability.Build,
			"recent_log":  observability.RecentLog,
			"restart":     restart,
		})
		boot.add(item)
		if observability.ComputerID != "" {
			bundle.ComputerID = observability.ComputerID
		}
		_ = json.Unmarshal(observability.Build, &bundle.Build)
	}
	bundle.Layers["boot"] = boot.finish()
	return bundle
}

// traceCaptureWindowMargin opens the captures window before the earliest
// run or event time: capture times are corpusd's clock, run times the
// guest's.
const (
	traceCaptureWindowMargin = 2 * time.Minute
	traceCaptureIDCap        = 500
)

// exportCaptures builds the captures layer (Cut 2 amendment A5): the
// computer's captures created from the trajectory's earliest run or event
// time to the export, plus every capture a record, run event, lifecycle
// event or the snapshot names by capture_id, each with its blocks' cids and
// normtext, so the quote census can rerun on the bundle.
func (e *traceExporter) exportCaptures(bundle *traceBundle, now time.Time) *traceLayer {
	b := newLayer("GET /api/captures?since=&until=&id=")
	query := url.Values{}
	if since := earliestTraceTime(bundle); !since.IsZero() {
		query.Set("since", since.Add(-traceCaptureWindowMargin).UTC().Format(time.RFC3339Nano))
	} else {
		b.note("no run or event time found; the window has no lower bound")
	}
	query.Set("until", now.UTC().Format(time.RFC3339Nano))
	ids := map[string]bool{}
	for _, name := range []string{"trajectory", "lifecycle_events", "run_events", "records"} {
		if layer := bundle.Layers[name]; layer != nil {
			for _, item := range layer.Items {
				collectCaptureIDs(item, ids)
			}
		}
	}
	named := sortedKeys(ids)
	if len(named) > traceCaptureIDCap {
		b.note("%d capture ids named; the first %d are read", len(named), traceCaptureIDCap)
		named = named[:traceCaptureIDCap]
	}
	for _, id := range named {
		query.Add("id", id)
	}
	path := "/api/captures?" + query.Encode()
	var resp struct {
		Captures       []json.RawMessage `json:"captures"`
		WindowComplete bool              `json:"window_complete"`
		Note           string            `json:"note"`
	}
	if err := e.get(path, &resp); err != nil {
		b.fail("%s", routeFailure("/api/captures", err))
		return b.finish()
	}
	b.add(resp.Captures...)
	if !resp.WindowComplete {
		b.note("the capture window is incomplete")
	}
	if resp.Note != "" {
		b.note("%s", resp.Note)
	}
	return b.finish()
}

// earliestTraceTime is the earliest created_at, started_at, ts or
// timestamp on a run or lifecycle event.
func earliestTraceTime(bundle *traceBundle) time.Time {
	var earliest time.Time
	for _, name := range []string{"runs", "lifecycle_events"} {
		layer := bundle.Layers[name]
		if layer == nil {
			continue
		}
		for _, item := range layer.Items {
			var fields map[string]any
			if json.Unmarshal(item, &fields) != nil {
				continue
			}
			for _, key := range []string{"created_at", "started_at", "ts", "timestamp"} {
				raw, _ := fields[key].(string)
				t, err := time.Parse(time.RFC3339Nano, raw)
				if err == nil && !t.IsZero() && (earliest.IsZero() || t.Before(earliest)) {
					earliest = t
				}
			}
		}
	}
	return earliest
}

// collectCaptureIDs records every capture_id (or capture_ids element) in a
// JSON value, including inside string values that hold JSON (a Cut 1 report
// record carries its packet as a JSON string).
func collectCaptureIDs(raw json.RawMessage, into map[string]bool) {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return
	}
	var walk func(any, int)
	walk = func(node any, depth int) {
		if depth > 8 {
			return
		}
		switch n := node.(type) {
		case map[string]any:
			for key, value := range n {
				if key == "capture_id" || key == "capture_ids" {
					switch typed := value.(type) {
					case string:
						if id := strings.TrimSpace(typed); id != "" {
							into[id] = true
						}
					case []any:
						for _, el := range typed {
							if s, ok := el.(string); ok && strings.TrimSpace(s) != "" {
								into[strings.TrimSpace(s)] = true
							}
						}
					}
				}
				walk(value, depth+1)
			}
		case []any:
			for _, el := range n {
				walk(el, depth+1)
			}
		case string:
			if s := strings.TrimSpace(n); strings.HasPrefix(s, "{") && strings.Contains(s, "capture_id") {
				var inner any
				if json.Unmarshal([]byte(s), &inner) == nil {
					walk(inner, depth+1)
				}
			}
		}
	}
	walk(v, 0)
}

func sortRunEvents(items []json.RawMessage) {
	type key struct {
		LoopID string `json:"loop_id"`
		Seq    int64  `json:"seq"`
	}
	keys := make([]key, len(items))
	for i, item := range items {
		_ = json.Unmarshal(item, &keys[i])
	}
	idx := make([]int, len(items))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		ka, kb := keys[idx[a]], keys[idx[b]]
		if ka.LoopID != kb.LoopID {
			return ka.LoopID < kb.LoopID
		}
		return ka.Seq < kb.Seq
	})
	sorted := make([]json.RawMessage, len(items))
	for i, j := range idx {
		sorted[i] = items[j]
	}
	copy(items, sorted)
}

// deriveRunEventKind projects run events of one kind into items: the event
// payload, with the event's identity (event_id, loop_id, seq, ts) added
// where the payload does not already carry it.
func deriveRunEventKind(parent *traceLayer, kind string) *traceLayer {
	source := "derived from run_events (kind " + kind + ")"
	if parent.Status == traceStatusUnserved {
		return unservedLayer("run_events is unserved", source)
	}
	b := newLayer(source)
	for _, raw := range parent.Items {
		var ev struct {
			EventID string          `json:"event_id"`
			LoopID  string          `json:"loop_id"`
			Seq     int64           `json:"seq"`
			TS      string          `json:"ts"`
			Kind    string          `json:"kind"`
			Payload json.RawMessage `json:"payload"`
		}
		if json.Unmarshal(raw, &ev) != nil || ev.Kind != kind {
			continue
		}
		item := map[string]any{}
		if json.Unmarshal(ev.Payload, &item) != nil || item == nil {
			item = map[string]any{"payload": ev.Payload}
		}
		for k, v := range map[string]any{"event_id": ev.EventID, "loop_id": ev.LoopID, "seq": ev.Seq, "ts": ev.TS} {
			if _, ok := item[k]; !ok {
				item[k] = v
			}
		}
		encoded, _ := json.Marshal(item)
		b.add(encoded)
	}
	return b.finish()
}

// deriveEgress projects the egress calls of the run events into items, one
// per call. Since journal cut R4 a cell's calls ride its desk.cell event, in
// its egress array; the standalone desk.egress events are older tapes, refused
// frames and the end of an async search. A folded row becomes an item with the
// fields it carried, the identity of its cell event (cell_event_id, and an
// event_id of the cell's event_id plus "#egress.<n>"), and the cell event's
// loop_id, seq and ts where the row does not carry them.
func deriveEgress(parent *traceLayer) *traceLayer {
	source := "derived from run_events (kind desk.egress, and the egress array of kind desk.cell)"
	if parent.Status == traceStatusUnserved {
		return unservedLayer("run_events is unserved", source)
	}
	b := newLayer(source)
	for _, raw := range parent.Items {
		var ev struct {
			EventID string          `json:"event_id"`
			LoopID  string          `json:"loop_id"`
			Seq     int64           `json:"seq"`
			TS      string          `json:"ts"`
			Kind    string          `json:"kind"`
			Payload json.RawMessage `json:"payload"`
		}
		if json.Unmarshal(raw, &ev) != nil {
			continue
		}
		switch ev.Kind {
		case "desk.egress":
			item := map[string]any{}
			if json.Unmarshal(ev.Payload, &item) != nil || item == nil {
				item = map[string]any{"payload": ev.Payload}
			}
			for k, v := range map[string]any{"event_id": ev.EventID, "loop_id": ev.LoopID, "seq": ev.Seq, "ts": ev.TS} {
				if _, ok := item[k]; !ok {
					item[k] = v
				}
			}
			encoded, _ := json.Marshal(item)
			b.add(encoded)
		case "desk.cell":
			var cell struct {
				Egress []map[string]any `json:"egress"`
			}
			if json.Unmarshal(ev.Payload, &cell) != nil {
				continue
			}
			for i, item := range cell.Egress {
				if item == nil {
					continue
				}
				for k, v := range map[string]any{"event_id": fmt.Sprintf("%s#egress.%d", ev.EventID, i), "cell_event_id": ev.EventID, "loop_id": ev.LoopID, "seq": ev.Seq, "ts": ev.TS} {
					if _, ok := item[k]; !ok {
						item[k] = v
					}
				}
				encoded, _ := json.Marshal(item)
				b.add(encoded)
			}
		}
	}
	return b.finish()
}

// deriveDeliveries projects the snapshot's updates (delivery packets) and
// work items, each tagged with its kind.
func deriveDeliveries(parent *traceLayer) *traceLayer {
	source := "derived from trajectory (updates, work_items)"
	if parent.Status == traceStatusUnserved {
		return unservedLayer("trajectory is unserved", source)
	}
	b := newLayer(source)
	for _, raw := range parent.Items {
		var snap struct {
			Updates   []json.RawMessage `json:"updates"`
			WorkItems []json.RawMessage `json:"work_items"`
		}
		if json.Unmarshal(raw, &snap) != nil {
			continue
		}
		for _, u := range snap.Updates {
			item, _ := json.Marshal(map[string]any{"kind": "update", "item": u})
			b.add(item)
		}
		for _, w := range snap.WorkItems {
			item, _ := json.Marshal(map[string]any{"kind": "work_item", "item": w})
			b.add(item)
		}
	}
	return b.finish()
}

func deriveTurnCommits(parent *traceLayer) *traceLayer {
	source := "derived from lifecycle_events (kind texture_turn_committed)"
	if parent.Status == traceStatusUnserved {
		return unservedLayer("lifecycle_events is unserved", source)
	}
	b := newLayer(source)
	for _, raw := range parent.Items {
		var ev struct {
			Kind string `json:"kind"`
		}
		if json.Unmarshal(raw, &ev) == nil && ev.Kind == "texture_turn_committed" {
			b.add(raw)
		}
	}
	return b.finish()
}

// exportInboxRecords reads every record of the Texture and research inboxes
// of the trajectory's documents and of its owners' management inboxes (Cut 1
// L3a, L3b: their conversations are on records), and each inbox's head,
// cursor and open rounds.
func (e *traceExporter) exportInboxRecords(bundle *traceBundle, docIDs map[string]bool) (*traceLayer, *traceLayer) {
	records := newLayer("GET /api/inboxes/{inbox}/records?after=&limit=1000")
	inboxes := newLayer("GET /api/inboxes/{inbox}/records (head, cursor, open_rounds)")
	if bundle.Layers["revisions"].Status == traceStatusUnserved {
		records.fail("revisions is unserved, so the documents' inboxes are unknown")
		inboxes.fail("revisions is unserved, so the documents' inboxes are unknown")
	}
	// Management holds one inbox per owner (Cut 1 L3b); its owners are the
	// owners of the trajectory's runs.
	owners := map[string]bool{}
	for _, item := range bundle.Layers["runs"].Items {
		var run struct {
			OwnerID string `json:"owner_id"`
		}
		if json.Unmarshal(item, &run) == nil && strings.TrimSpace(run.OwnerID) != "" {
			owners[strings.TrimSpace(run.OwnerID)] = true
		}
	}
	var inboxIDs []string
	for _, docID := range sortedKeys(docIDs) {
		inboxIDs = append(inboxIDs, "texture:"+docID, "research:"+docID)
	}
	for _, ownerID := range sortedKeys(owners) {
		inboxIDs = append(inboxIDs, "management:"+ownerID)
	}
	for _, inbox := range inboxIDs {
		after := int64(0)
		for pages := 0; ; pages++ {
			if pages >= traceMaxPages {
				records.fail("inbox %s paging exceeded %d pages", inbox, traceMaxPages)
				break
			}
			path := fmt.Sprintf("/api/inboxes/%s/records?after=%d&limit=%d", url.PathEscape(inbox), after, traceEventPageLimit)
			var page struct {
				Inbox      string            `json:"inbox"`
				Head       int64             `json:"head"`
				Cursor     int64             `json:"cursor"`
				Records    []json.RawMessage `json:"records"`
				OpenRounds []json.RawMessage `json:"open_rounds"`
				NextAfter  int64             `json:"next_after"`
			}
			if err := e.get(path, &page); err != nil {
				records.fail("%s", routeFailure(path, err))
				inboxes.fail("%s", routeFailure(path, err))
				break
			}
			records.add(page.Records...)
			if after == 0 {
				state, _ := json.Marshal(map[string]any{"inbox": inbox, "head": page.Head, "cursor": page.Cursor, "open_rounds": page.OpenRounds})
				inboxes.add(state)
			}
			if page.NextAfter == 0 {
				break
			}
			if page.NextAfter <= after {
				records.fail("GET %s: cursor did not advance past %d", path, after)
				break
			}
			after = page.NextAfter
		}
	}
	return records.finish(), inboxes.finish()
}

// exportPublications resolves every published route of the trajectory's
// documents. Routes are named by revision metadata (corpusd_route_path and
// corpusd_publication_ref.route_path, written when a publication is
// accepted) and by lifecycle refs recorded for an accepted publication.
func (e *traceExporter) exportPublications(bundle *traceBundle, docIDs map[string]bool) *traceLayer {
	b := newLayer("GET /api/platform/publications/resolve?route=")
	routes := map[string]bool{}
	revisionsLayer := bundle.Layers["revisions"]
	if revisionsLayer.Status == traceStatusUnserved {
		b.fail("revisions is unserved, so publication routes are unknown")
	}
	for _, raw := range revisionsLayer.Items {
		var rev struct {
			Metadata struct {
				RoutePath string `json:"corpusd_route_path"`
				Ref       struct {
					RoutePath string `json:"route_path"`
				} `json:"corpusd_publication_ref"`
			} `json:"metadata"`
		}
		if json.Unmarshal(raw, &rev) != nil {
			continue
		}
		for _, route := range []string{rev.Metadata.RoutePath, rev.Metadata.Ref.RoutePath} {
			if route = strings.TrimSpace(route); route != "" {
				routes[route] = true
			}
		}
	}
	if bundle.Layers["lifecycle_events"].Status == traceStatusUnserved {
		b.note("lifecycle publication refs unread: lifecycle_events is unserved")
	}
	for _, raw := range bundle.Layers["lifecycle_events"].Items {
		var ev struct {
			Reason       string   `json:"reason"`
			ArtifactRefs []string `json:"artifact_refs"`
		}
		if json.Unmarshal(raw, &ev) != nil || ev.Reason != tracePublicationNote {
			continue
		}
		for _, route := range ev.ArtifactRefs {
			if route = strings.TrimSpace(route); route != "" {
				routes[route] = true
			}
		}
	}
	var unresolved []string
	for _, route := range sortedKeys(routes) {
		path := "/api/platform/publications/resolve?route=" + url.QueryEscape(route)
		var pub json.RawMessage
		if err := e.get(path, &pub); err != nil {
			if isNotFound(err) {
				unresolved = append(unresolved, route)
				continue
			}
			b.fail("%s", routeFailure(path, err))
			continue
		}
		b.add(pub)
	}
	if len(routes) == 0 {
		b.note("no publication route named by revision metadata or lifecycle refs")
	}
	if len(unresolved) > 0 {
		b.note("routes named but not resolvable (404): %s", strings.Join(unresolved, ", "))
	}
	return b.finish()
}

// ---- command ----

var traceFileUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func defaultTraceOut(trajectoryID string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	name := strings.Trim(traceFileUnsafe.ReplaceAllString(trajectoryID, "_"), "._")
	if name == "" {
		name = "trajectory"
	}
	return filepath.Join(home, ".config", "choir-qa", "traces", name+".json"), nil
}

// splitTraceArgs lets the trajectory id come before or after the flags.
func splitTraceArgs(args []string) (flagArgs, positional []string) {
	valueFlags := map[string]bool{"out": true, "api-key-file": true, "host": true, "computer": true, "timeout": true}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			flagArgs = append(flagArgs, arg)
			name := strings.TrimLeft(arg, "-")
			if !strings.Contains(name, "=") && valueFlags[name] && i+1 < len(args) {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
			continue
		}
		positional = append(positional, arg)
	}
	return flagArgs, positional
}

func runTrace(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "export" {
		fmt.Fprintln(stderr, "choir trace: subcommand required (export)")
		return 2
	}
	return runTraceExport(args[1:], stdout, stderr)
}

func runTraceExport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir trace export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "", "Bundle path (default ~/.config/choir-qa/traces/<trajectory>.json)")
	flagArgs, positional := splitTraceArgs(args)
	c, err := newClient(fs, flagArgs, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir trace export: %v\n", err)
		return 2
	}
	positional = append(positional, fs.Args()...)
	if len(positional) != 1 || strings.TrimSpace(positional[0]) == "" {
		fmt.Fprintln(stderr, "choir trace export: exactly one trajectory id required")
		return 2
	}
	trajectoryID := strings.TrimSpace(positional[0])
	path := strings.TrimSpace(*out)
	if path == "" {
		if path, err = defaultTraceOut(trajectoryID); err != nil {
			fmt.Fprintf(stderr, "choir trace export: %v\n", err)
			return 1
		}
	}

	bundle := (&traceExporter{c: c, trajectoryID: trajectoryID}).export(time.Now())
	raw, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "choir trace export: encode bundle: %v\n", err)
		return 1
	}
	raw = append(raw, '\n')
	if err := writePrivateFile(path, raw); err != nil {
		fmt.Fprintf(stderr, "choir trace export: %v\n", err)
		return 1
	}
	sum := sha256.Sum256(raw)

	type layerSummary struct {
		Status string `json:"status"`
		Items  int    `json:"items"`
		Note   string `json:"note,omitempty"`
	}
	summary := struct {
		Path         string                  `json:"path"`
		SHA256       string                  `json:"sha256"`
		Bytes        int                     `json:"bytes"`
		TrajectoryID string                  `json:"trajectory_id"`
		ComputerID   string                  `json:"computer_id"`
		Layers       map[string]layerSummary `json:"layers"`
	}{
		Path:         path,
		SHA256:       hex.EncodeToString(sum[:]),
		Bytes:        len(raw),
		TrajectoryID: trajectoryID,
		ComputerID:   bundle.ComputerID,
		Layers:       map[string]layerSummary{},
	}
	for name, layer := range bundle.Layers {
		summary.Layers[name] = layerSummary{Status: layer.Status, Items: len(layer.Items), Note: layer.Note}
	}
	return writeJSON(stdout, summary)
}

// writePrivateFile writes data with mode 0600, creating parent directories
// with mode 0700. An existing file is truncated and narrowed to 0600.
func writePrivateFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create bundle directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open bundle: %w", err)
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return fmt.Errorf("restrict bundle mode: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("write bundle: %w", err)
	}
	return f.Close()
}
