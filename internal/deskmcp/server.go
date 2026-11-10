package deskmcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

// Forwarder carries one call to the host and returns its answer. The server
// holds no other capability (F9).
type Forwarder interface {
	Forward(ctx context.Context, tool string, args json.RawMessage) (Result, error)
}

const (
	// MaxLineBytes bounds one JSON-RPC line (F6).
	MaxLineBytes = 1 << 20
	// ProgressInterval is how often a blocked call tells the agent it is
	// alive, far inside its 30-minute idle abort (F7).
	ProgressInterval = 60 * time.Second
	// forwardSlack is added to a blocking call's own timeout.
	forwardSlack = 30 * time.Second
	// defaultForward bounds a call that does not name a timeout (F10).
	defaultForward = 2 * time.Minute

	latestProtocol = "2025-06-18"
	serverVersion  = "1"
)

var supportedProtocols = map[string]bool{"2024-11-05": true, "2025-03-26": true, "2025-06-18": true, "2025-11-25": true}

// Server is a stdio MCP server over a constant tool set.
type Server struct {
	Tools []Tool
	Fwd   Forwarder
	// Log receives diagnostics. Stdout carries JSON-RPC only (F6).
	Log io.Writer
	// ProgressEvery overrides ProgressInterval (tests).
	ProgressEvery time.Duration
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type session struct {
	s       *Server
	out     io.Writer
	outMu   sync.Mutex
	mu      sync.Mutex
	inited  bool
	cancels map[string]context.CancelFunc
	wg      sync.WaitGroup
}

func (s *Server) logf(format string, a ...any) {
	if s.Log != nil {
		fmt.Fprintf(s.Log, format+"\n", a...)
	}
}

// Serve reads newline-delimited JSON-RPC from in and answers on out until in
// ends. A line it cannot use gets an error and the server keeps serving.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ss := &session{s: s, out: out, cancels: map[string]context.CancelFunc{}}
	r := bufio.NewReaderSize(in, 64<<10)
	var err error
	for {
		var line []byte
		var tooLong bool
		line, tooLong, err = readLine(r, MaxLineBytes)
		if len(bytes.TrimSpace(line)) > 0 || tooLong {
			if tooLong {
				ss.fail(nil, -32600, fmt.Sprintf("a line is over %d bytes", MaxLineBytes))
			} else {
				ss.handle(ctx, line)
			}
		}
		if err != nil {
			break
		}
	}
	cancel()
	ss.wg.Wait()
	if err == io.EOF {
		return nil
	}
	return err
}

// readLine reads one line of at most max bytes; a longer line is consumed to
// its end and reported tooLong.
func readLine(r *bufio.Reader, max int) (line []byte, tooLong bool, err error) {
	for {
		chunk, isPrefix, e := r.ReadLine()
		if len(line)+len(chunk) > max {
			tooLong = true
		} else if !tooLong {
			line = append(line, chunk...)
		}
		if e != nil {
			return line, tooLong, e
		}
		if !isPrefix {
			return line, tooLong, nil
		}
	}
}

func (ss *session) write(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		ss.s.logf("marshal: %v", err)
		return
	}
	ss.outMu.Lock()
	defer ss.outMu.Unlock()
	_, _ = ss.out.Write(append(b, '\n'))
}

func (ss *session) reply(id json.RawMessage, result any) {
	ss.write(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func (ss *session) fail(id json.RawMessage, code int, msg string) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	ss.write(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": msg}})
}

func (ss *session) handle(ctx context.Context, line []byte) {
	trim := bytes.TrimSpace(line)
	if trim[0] == '[' {
		ss.fail(nil, -32600, "batches are not supported")
		return
	}
	var req request
	if err := json.Unmarshal(trim, &req); err != nil || trim[0] != '{' {
		ss.fail(nil, -32700, "parse error")
		return
	}
	isCall := len(req.ID) > 0 && !bytes.Equal(req.ID, []byte("null"))
	if req.Method == "" {
		// A response from the client, which this server never awaits.
		return
	}
	if !isCall {
		ss.notification(req)
		return
	}
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		v := latestProtocol
		if supportedProtocols[p.ProtocolVersion] {
			v = p.ProtocolVersion
		}
		ss.mu.Lock()
		ss.inited = true
		ss.mu.Unlock()
		ss.reply(req.ID, map[string]any{
			"protocolVersion": v,
			// The tool set is constant for a run: no listChanged (F13).
			"capabilities": map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":   map[string]any{"name": ServerName, "version": serverVersion},
		})
	case "ping":
		ss.reply(req.ID, map[string]any{})
	case "tools/list", "tools/call":
		ss.mu.Lock()
		ok := ss.inited
		ss.mu.Unlock()
		if !ok {
			ss.fail(req.ID, -32002, "server not initialized")
			return
		}
		if req.Method == "tools/list" {
			ss.reply(req.ID, map[string]any{"tools": ss.listing()})
			return
		}
		ss.wg.Add(1)
		go func() {
			defer ss.wg.Done()
			ss.call(ctx, req)
		}()
	default:
		ss.fail(req.ID, -32601, "method not found: "+req.Method)
	}
}

func (ss *session) notification(req request) {
	if req.Method != "notifications/cancelled" {
		return
	}
	var p struct {
		RequestID json.RawMessage `json:"requestId"`
	}
	if json.Unmarshal(req.Params, &p) == nil {
		ss.mu.Lock()
		if c := ss.cancels[string(p.RequestID)]; c != nil {
			c()
		}
		ss.mu.Unlock()
	}
}

func (ss *session) listing() []map[string]any {
	out := make([]map[string]any, 0, len(ss.s.Tools))
	for _, t := range ss.s.Tools {
		out = append(out, map[string]any{"name": t.Name, "description": t.Description, "inputSchema": t.InputSchema()})
	}
	return out
}

func (ss *session) call(ctx context.Context, req request) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
		Meta      struct {
			ProgressToken json.RawMessage `json:"progressToken"`
		} `json:"_meta"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		ss.fail(req.ID, -32602, "invalid params")
		return
	}
	var tool *Tool
	for i := range ss.s.Tools {
		if ss.s.Tools[i].Name == p.Name {
			tool = &ss.s.Tools[i]
		}
	}
	if tool == nil {
		ss.fail(req.ID, -32602, "unknown tool "+p.Name)
		return
	}
	cctx, cancel := context.WithTimeout(ctx, forwardTimeout(p.Arguments))
	key := string(req.ID)
	ss.mu.Lock()
	ss.cancels[key] = cancel
	ss.mu.Unlock()
	defer func() {
		cancel()
		ss.mu.Lock()
		delete(ss.cancels, key)
		ss.mu.Unlock()
	}()

	type answer struct {
		res Result
		err error
	}
	done := make(chan answer, 1)
	go func() {
		res, err := ss.s.Fwd.Forward(cctx, p.Name, p.Arguments)
		done <- answer{res, err}
	}()
	every := ss.s.ProgressEvery
	if every <= 0 {
		every = ProgressInterval
	}
	tick := time.NewTicker(every)
	defer tick.Stop()
	n := 0
	for {
		select {
		case a := <-done:
			if a.err != nil {
				a.res = Result{Text: "the desk host is unreachable: " + a.err.Error(), IsError: true}
			}
			ss.reply(req.ID, map[string]any{
				"content": []map[string]any{{"type": "text", "text": a.res.Text}},
				"isError": a.res.IsError,
			})
			return
		case <-tick.C:
			if len(p.Meta.ProgressToken) > 0 {
				n++
				ss.write(map[string]any{"jsonrpc": "2.0", "method": "notifications/progress",
					"params": map[string]any{"progressToken": p.Meta.ProgressToken, "progress": n}})
			}
		}
	}
}

// forwardTimeout is a blocking call's own timeout plus slack, or the default.
func forwardTimeout(args json.RawMessage) time.Duration {
	var a struct {
		TimeoutS json.Number `json:"timeout_s"`
	}
	if json.Unmarshal(args, &a) == nil {
		if n, err := a.TimeoutS.Int64(); err == nil && n > 0 && n <= MaxCheckSecs {
			return time.Duration(n)*time.Second + forwardSlack
		}
	}
	return defaultForward
}
