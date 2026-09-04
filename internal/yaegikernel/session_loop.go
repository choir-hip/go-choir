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
// channel. Either Source carries a cell or Close ends the session.
type SessionFrame struct {
	ID     string `json:"id"`
	Source string `json:"source,omitempty"`
	Close  bool   `json:"close,omitempty"`
}

// SessionResult is one newline-delimited JSON response. Value-carrying cells
// surface through Stdout like the one-shot path; structured values stay
// in-process (the broker re-fetches them with follow-up cells).
type SessionResult struct {
	ID         string `json:"id"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// RunSessionLoop serves framed eval cells on r/w using a single persistent
// Session: variables, imports, and definitions survive across frames until a
// close frame, EOF, or a failed cell. A failed cell poisons the session and
// the loop exits non-zero so the broker respawns a clean worker (poisoned
// session replacement). Extracted from stdio so tests drive it over pipes.
func RunSessionLoop(r io.Reader, w io.Writer, newSession func() (*Session, error)) error {
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
		start := time.Now()
		res, evalErr := sess.Eval(context.Background(), frame.Source)
		out := SessionResult{ID: frame.ID, Stdout: res.Stdout, Stderr: res.Stderr, DurationMs: time.Since(start).Milliseconds()}
		if evalErr != nil {
			out.Error = evalErr.Error()
		}
		if err := enc.Encode(out); err != nil {
			return fmt.Errorf("session: encode result: %w", err)
		}
		if evalErr != nil {
			return fmt.Errorf("session: cell poisoned worker: %w", evalErr)
		}
	}
	return scanner.Err()
}
