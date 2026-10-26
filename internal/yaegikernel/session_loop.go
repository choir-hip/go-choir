package yaegikernel

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// SessionFrame is one newline-delimited JSON request on a worker session
// channel. Either Source carries a cell or Close ends the session. Inbox
// carries the cell-start snapshot injected by autoputer; the cell reads it
// through choir.Inbox() without network roundtrips.
type SessionFrame struct {
	ID     string            `json:"id"`
	Source string            `json:"source,omitempty"`
	Close  bool              `json:"close,omitempty"`
	Inbox  []IncomingMessage `json:"inbox,omitempty"`

	// Doc carries the cell-start document snapshot for a texture desk cell
	// (R3d): the bound doc's current revision id + content, so the cell
	// reads it via choir.ReadDoc() without a network roundtrip.
	Doc *DocSnapshot `json:"doc,omitempty"`
	// Pack carries the acting desk's score-free commitment context (R4):
	// its own committed/addressed acts joined with resolution observations
	// and discrepancies — never score fields, by construction — read inside
	// the cell through choir.Pack().
	Pack *types.ActingPack `json:"pack,omitempty"`
	// Updates carries the pending update_coagent records bound for this
	// cell (RLM prompt-as-variable): the cell reads them via
	// choir.Updates(); the wake turn in chat carries only their ids.
	Updates []PendingUpdate `json:"updates,omitempty"`
	// Emits carries the emitted signals addressed to this desk drained at the
	// cell boundary: the cell reads them via choir.Emits(); the chat notice
	// turn carries only a fixed-format line (sender/kind/seq/snippet), never
	// the body.
	Emits []PendingEmit `json:"emits,omitempty"`
	// Records carries the records addressed to this desk's inbox in
	// (cursor, head] at cell start, read through choir.Records().
	Records []InboxRecord `json:"records,omitempty"`
	// Jobs carries this run's capsule jobs at cell start, read through
	// choir.Poll (jobs.go).
	Jobs []JobView `json:"jobs,omitempty"`
}

// DocSnapshot is the bound Texture document as the cell's frame showed it:
// the revision a Write applies to, its text, and its top-level blocks by
// alias (b1..bN) in the Write grammar. Sources are the frame's record
// sources by handle (r2.c1), the handles a {{cite}} may name.
type DocSnapshot struct {
	DocID      string                `json:"doc_id"`
	RevisionID string                `json:"revision_id"`
	AuthorKind string                `json:"author_kind,omitempty"`
	Content    string                `json:"content"`
	Blocks     []texturewrite.Block  `json:"blocks,omitempty"`
	Sources    []texturewrite.Source `json:"sources,omitempty"`
	// OwnerRevision is the revision that woke this run when the owner wrote
	// it, empty otherwise. A Cast names it as owner_revision and the
	// reducer attaches the owner's words, so the cell never retypes them
	// (docs/problems/texture-retypes-owner-directions-2026-10-25.md).
	OwnerRevision string `json:"owner_revision,omitempty"`
}

// SessionResult is one newline-delimited JSON response. Value-carrying cells
// surface through Stdout like the one-shot path; structured values stay
// in-process (the broker re-fetches them with follow-up cells). Intents
// carries the cell's staged tray for post-cell reduction; it is empty for
// unbound (legacy synchronous) cells.
type SessionResult struct {
	ID         string         `json:"id"`
	Stdout     string         `json:"stdout"`
	Stderr     string         `json:"stderr"`
	Error      string         `json:"error,omitempty"`
	DurationMs int64          `json:"duration_ms"`
	Receipts   []string       `json:"receipts,omitempty"`
	Intents    []StagedIntent `json:"intents,omitempty"`
	// Reuse carries the session reuse disposition for a failed cell
	// (preserve: worker stays alive; otherwise the worker is poisoned).
	// DiagKind carries the diagnostic kind with the message verbatim.
	Reuse    ReuseDisposition `json:"reuse,omitempty"`
	DiagKind DiagnosticKind   `json:"diag_kind,omitempty"`
	// Searches carries one count line per async search with arrivals since
	// the last cell result ("s3: 3/4 answered, +2 new"): the only push of
	// search arrivals to the model, counts and never results.
	Searches []string `json:"searches,omitempty"`
	// RecordsShown is the highest record seq a choir.Records() call in the
	// cell returned (0: the cell never called it). The cell's commit marks
	// read only through it (autopaper-arrivals-unanswered-2026-10-13).
	RecordsShown int64 `json:"records_shown,omitempty"`
	// JobsAwaited are the jobs a choir.Poll in the cell returned unfinished
	// (running, or starting: staged in the cell). The host parks the turn
	// on them (problems/engineering-busy-polls-its-own-job-2026-10-24.md).
	JobsAwaited []string `json:"jobs_awaited,omitempty"`
	// SlotToken is set on the worker's ready handshake only (ARCH-07): the
	// per-spawn token the host generated, proving the process on the socket
	// is the one it started.
	SlotToken string `json:"slot_token,omitempty"`
}

// RunSessionLoopWithDrainAndHooks serves newline-delimited SessionFrames from
// r and writes one SessionResult per frame to w, with an optional receipt
// drain and per-cell tray/inbox binding for RLM reduction (nil hooks bind
// nothing).
func RunSessionLoopWithDrainAndHooks(r io.Reader, w io.Writer, newSession func() (*Session, error), drain func() []string, hooks *CellHooks) error {
	sess, err := newSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4<<20), 4<<20)
	enc := json.NewEncoder(w)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var frame SessionFrame
		if err := json.Unmarshal(line, &frame); err != nil {
			return fmt.Errorf("session: decode frame: %w", err)
		}
		if frame.Close {
			return nil
		}
		out, evalErr := serveCell(sess, frame, drain, hooks)
		if err := enc.Encode(out); err != nil {
			return fmt.Errorf("session: encode result: %w", err)
		}
		if evalErr != nil {
			return fmt.Errorf("session: cell poisoned worker: %w", evalErr)
		}
	}
	return scanner.Err()
}

// serveCell evaluates one frame on the session and reports the result. An
// unsafe-to-reuse failure poisons the session: the caller ships the error
// result and respawns a clean worker, never reusing poisoned state. A proven
// non-executing rejection ships the error result with the worker alive.
// Hooks bind the cell's tray and inbox snapshot around evaluation (nil hooks
// = legacy synchronous behavior, no staged intents). A failed cell drops its
// tray: only successful cells reduce intents and advance the inbox cursor.
func serveCell(sess *Session, frame SessionFrame, drain func() []string, hooks *CellHooks) (SessionResult, error) {
	if hooks != nil && hooks.Begin != nil {
		hooks.Begin(frame)
	}
	start := time.Now()
	res, evalErr := sess.Eval(context.Background(), frame.Source)
	out := SessionResult{ID: frame.ID, Stdout: res.Stdout, Stderr: res.Stderr, DurationMs: time.Since(start).Milliseconds()}
	if drain != nil {
		out.Receipts = drain()
	}
	if hooks != nil && hooks.RecordsShown != nil {
		out.RecordsShown = hooks.RecordsShown()
	}
	if hooks != nil && hooks.JobsAwaited != nil {
		out.JobsAwaited = hooks.JobsAwaited()
	}
	var staged []StagedIntent
	if hooks != nil && hooks.End != nil {
		staged = hooks.End()
	}
	if hooks != nil && hooks.SearchCounts != nil {
		out.Searches = hooks.SearchCounts()
	}
	if evalErr != nil {
		out.Error = evalErr.Error()
		if hint := sess.ErrorHint(frame.Source, out.Error); hint != "" {
			out.Error += "\nhint: " + hint
		}
		if ee, ok := AsEvalError(evalErr); ok {
			out.Reuse = ee.Reuse
			out.DiagKind = ee.Kind
			if ee.Reuse == ReusePreserve {
				// Proven non-executing rejection: the heap is intact, so
				// the worker stays alive. No intents ship (nothing ran).
				return out, nil
			}
		}
		return out, evalErr
	}
	out.Intents = staged
	// Only the cell's own prints are cut, and only on this success path: a
	// poisoned session could never serve the handle. The value views below
	// keep their own bound.
	if hooks != nil && hooks.KeepOutput != nil {
		out.Stdout = hooks.KeepOutput(out.Stdout)
	}
	out.Stdout += sess.showListedNames(frame.Source)
	if shown := describeCellValue(frame.Source, res.Value); shown != "" {
		out.Stdout += shown
	}
	if hooks != nil && hooks.Refusals != nil {
		for _, line := range hooks.Refusals() {
			if !strings.Contains(out.Stdout, line[strings.Index(line, " refused: ")+len(" refused: "):]) {
				out.Stdout += line + "\n"
			}
		}
	}
	return out, nil
}

// maxShownCellValue bounds one value a cell result shows. It is sized under
// the 100 KB a tool result carries to the model, not for probing: a
// choir.Research result is several children's multi-thousand-character
// findings, and a 4,000-character bound hid most of them
// (problems/research-parent-cannot-read-its-children-2026-10-11.md).
// The bound is in bytes, and the cut falls on a rune boundary. Value views
// are not prints: the LR lever 6 cut (cell_output.go) leaves them alone.
const maxShownCellValue = 48000

// describeCellValue renders a cell's final value, REPL style, when the cell
// ends in a bare expression that is not itself a print call. Models probe
// state with `updates` or `choir.Help("X")`; without this the result was
// empty and they spent cells on panic(string(b)) to see it.
func describeCellValue(source string, value reflect.Value) (shown string) {
	if !value.IsValid() || !endsInBareExpression(source) {
		return ""
	}
	defer func() {
		if recover() != nil {
			shown = ""
		}
	}()
	text, ok := cellValueText(value, maxShownCellValue)
	if !ok {
		return ""
	}
	return "=> " + text + "\n"
}

// cellValueText renders one value for a cell result. Bytes that are valid
// UTF-8 (a json.RawMessage from choir.Read) render as text, not as a
// list of byte values.
func cellValueText(value reflect.Value, max int) (string, bool) {
	if !value.IsValid() || value.Kind() == reflect.Func {
		return "", false
	}
	var text string
	switch {
	case value.Kind() == reflect.String:
		text = value.String()
	case value.Kind() == reflect.Slice && value.Type().Elem().Kind() == reflect.Uint8 && utf8.Valid(value.Bytes()):
		text = string(value.Bytes())
	case value.CanInterface():
		text = fmt.Sprintf("%+v", value.Interface())
	default:
		return "", false
	}
	if len(text) > max {
		// The bound is in bytes; the cut backs off to a rune boundary.
		cut := runeBoundary(text, max)
		text = text[:cut] + fmt.Sprintf("… [%d bytes truncated]", len(text)-cut)
	}
	return text, true
}

// listedNames returns the bare identifiers a cell lists as statements before
// its last statement (`r; err` lists r). The REPL shows only the final
// value, so research read `r; err` as a nil result (problem doc
// cell-shows-only-last-value-research-reads-results-as-nil-2026-10-10.md).
func listedNames(source string) []string {
	file, err := parser.ParseFile(token.NewFileSet(), "cell.go", "package p\nfunc _() {\n"+source+"\n}", 0)
	if err != nil || len(file.Decls) == 0 {
		return nil
	}
	fn, ok := file.Decls[0].(*ast.FuncDecl)
	if !ok || fn.Body == nil || len(fn.Body.List) < 2 {
		return nil
	}
	seen := map[string]bool{}
	var names []string
	for _, stmt := range fn.Body.List[:len(fn.Body.List)-1] {
		expr, ok := stmt.(*ast.ExprStmt)
		if !ok {
			continue
		}
		ident, ok := expr.X.(*ast.Ident)
		if !ok || ident.Name == "_" || seen[ident.Name] {
			continue
		}
		seen[ident.Name] = true
		names = append(names, ident.Name)
	}
	return names
}

// endsInBareExpression reports whether the cell's last statement is an
// expression statement other than a print call.
func endsInBareExpression(source string) bool {
	file, err := parser.ParseFile(token.NewFileSet(), "cell.go", "package p\nfunc _() {\n"+source+"\n}", 0)
	if err != nil || len(file.Decls) == 0 {
		return false
	}
	fn, ok := file.Decls[0].(*ast.FuncDecl)
	if !ok || fn.Body == nil || len(fn.Body.List) == 0 {
		return false
	}
	stmt, ok := fn.Body.List[len(fn.Body.List)-1].(*ast.ExprStmt)
	if !ok {
		return false
	}
	if call, ok := stmt.X.(*ast.CallExpr); ok {
		switch callee := call.Fun.(type) {
		case *ast.FuncLit:
			// `func() { ... }()` returns nothing to show.
			if callee.Type.Results == nil || len(callee.Type.Results.List) == 0 {
				return false
			}
		case *ast.Ident:
			if callee.Name == "println" || callee.Name == "print" || callee.Name == "panic" {
				return false
			}
		case *ast.SelectorExpr:
			if pkg, ok := callee.X.(*ast.Ident); ok && pkg.Name == "fmt" && strings.HasPrefix(callee.Sel.Name, "Print") {
				return false
			}
		}
	}
	return true
}

// runSessionLoopMux serves SessionFrames read through the worker's mux and
// writes each result on fc, with an optional receipt drain and per-cell
// tray/inbox binding for RLM reduction (nil hooks bind nothing). The cell
// boundary is marked on the mux so a host call from a goroutine that
// outlives its cell is refused instead of riding the next cell.
func runSessionLoopMux(mux *workerMux, fc *FramedConn, newSession func() (*Session, error), drain func() []string, hooks *CellHooks) error {
	sess, err := newSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	for {
		stream, payload, err := mux.next()
		if err != nil {
			return err
		}
		if stream == StreamCancel {
			return fmt.Errorf("session: cell cancelled by broker")
		}
		var frame SessionFrame
		if err := json.Unmarshal(payload, &frame); err != nil {
			return fmt.Errorf("session: decode frame: %w", err)
		}
		if frame.Close {
			return nil
		}
		mux.beginCell()
		out, evalErr := serveCell(sess, frame, drain, hooks)
		mux.endCell()
		raw, err := json.Marshal(out)
		if err != nil {
			return fmt.Errorf("session: encode result: %w", err)
		}
		if err := fc.WriteFrame(StreamCell, raw); err != nil {
			return fmt.Errorf("session: send result: %w", err)
		}
		if evalErr != nil {
			return fmt.Errorf("session: cell poisoned worker: %w", evalErr)
		}
	}
}
