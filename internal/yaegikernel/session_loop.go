package yaegikernel

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
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
}

// RunSessionLoop serves framed eval cells on r/w using a single persistent
// Session: variables, imports, and definitions survive across frames until a
// close frame, EOF, or a failed cell. A failed cell poisons the session and
// the loop exits non-zero so the broker respawns a clean worker (poisoned
func RunSessionLoop(r io.Reader, w io.Writer, newSession func() (*Session, error)) error {
	return RunSessionLoopWithDrain(r, w, newSession, nil)
}

// RunSessionLoopWithDrain serves the same framed loop but attaches
// worker-local assign/message receipts recorded during each cell (via drain,
// nil to skip) to that cell's SessionResult, so the host reconciles outcomes
// without trusting worker memory as durable state.
func RunSessionLoopWithDrain(r io.Reader, w io.Writer, newSession func() (*Session, error), drain func() []string) error {
	return RunSessionLoopWithDrainAndHooks(r, w, newSession, drain, nil)
}

// RunSessionLoopWithDrainAndHooks is RunSessionLoopWithDrain with per-cell
// tray/inbox binding for RLM reduction. Nil hooks preserve legacy behavior.
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

// serveCell evaluates one frame on the session and reports the result. A
// failed cell poisons the session: the caller ships the error result and
// respawns a clean worker, never reusing poisoned state. Hooks bind the
// cell's tray and inbox snapshot around evaluation (nil hooks = legacy
// synchronous behavior, no staged intents). A failed cell drops its tray:
// only successful cells reduce intents and advance the inbox cursor.
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
	var staged []StagedIntent
	if hooks != nil && hooks.End != nil {
		staged = hooks.End()
	}
	if evalErr != nil {
		out.Error = evalErr.Error()
		return out, evalErr
	}
	out.Intents = staged
	return out, nil
}

// RunSessionLoopFramed serves eval cells over the multiplexed session socket
// (Step 2 transport): StreamCell frames carry SessionFrame JSON in and
// SessionResult JSON out. StreamCancel abandons the in-flight cell and
// poisons the session. Reserved output streams are tolerated and ignored.
func RunSessionLoopFramed(fc *FramedConn, newSession func() (*Session, error), drain func() []string) error {
	return RunSessionLoopFramedWithHooks(fc, newSession, drain, nil)
}

// RunSessionLoopFramedWithHooks is RunSessionLoopFramed with per-cell
// tray/inbox binding for RLM reduction. Nil hooks preserve legacy behavior.
func RunSessionLoopFramedWithHooks(fc *FramedConn, newSession func() (*Session, error), drain func() []string, hooks *CellHooks) error {
	sess, err := newSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	for {
		stream, payload, err := fc.ReadFrame()
		if err != nil {
			return err
		}
		switch stream {
		case StreamCancel:
			return fmt.Errorf("session: cell cancelled by broker")
		case StreamCell:
			// Handled below.
		default:
			continue
		}
		var frame SessionFrame
		if err := json.Unmarshal(payload, &frame); err != nil {
			return fmt.Errorf("session: decode frame: %w", err)
		}
		if frame.Close {
			return nil
		}
		out, evalErr := serveCell(sess, frame, drain, hooks)
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
