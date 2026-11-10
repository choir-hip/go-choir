package deskmcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskjudge"
)

// client drives a Server over pipes, as an MCP client drives a stdio server.
type client struct {
	t     *testing.T
	in    io.WriteCloser
	lines chan string
	exit  chan error
}

func startServer(t *testing.T, s *Server) *client {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	c := &client{t: t, in: inW, lines: make(chan string, 64), exit: make(chan error, 1)}
	go func() {
		c.exit <- s.Serve(context.Background(), inR, outW)
		_ = outW.Close()
	}()
	go func() {
		sc := bufio.NewScanner(outR)
		sc.Buffer(make([]byte, 1<<20), 4<<20)
		for sc.Scan() {
			c.lines <- sc.Text()
		}
		close(c.lines)
	}()
	t.Cleanup(func() { _ = inW.Close() })
	return c
}

func (c *client) send(line string) {
	c.t.Helper()
	if _, err := io.WriteString(c.in, line+"\n"); err != nil {
		c.t.Fatal(err)
	}
}

func (c *client) next() map[string]any {
	c.t.Helper()
	select {
	case l, ok := <-c.lines:
		if !ok {
			c.t.Fatal("server output closed")
		}
		var m map[string]any
		// F6: stdout is JSON-RPC only.
		if err := json.Unmarshal([]byte(l), &m); err != nil || m["jsonrpc"] != "2.0" {
			c.t.Fatalf("a non-JSON-RPC line on stdout: %q", l)
		}
		return m
	case <-time.After(5 * time.Second):
		c.t.Fatal("no output")
	}
	return nil
}

func (c *client) silent() {
	c.t.Helper()
	select {
	case l := <-c.lines:
		c.t.Fatalf("unexpected output %q", l)
	case <-time.After(60 * time.Millisecond):
	}
}

func errCode(m map[string]any) int {
	e, _ := m["error"].(map[string]any)
	n, _ := e["code"].(float64)
	return int(n)
}

type fwdFunc func(ctx context.Context, tool string, args json.RawMessage) (Result, error)

func (f fwdFunc) Forward(ctx context.Context, tool string, args json.RawMessage) (Result, error) {
	return f(ctx, tool, args)
}

func echoFwd() Forwarder {
	return fwdFunc(func(_ context.Context, tool string, args json.RawMessage) (Result, error) {
		return Result{Text: tool + " " + string(args)}, nil
	})
}

const initialize = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`

// F6, F13: a handshake that advertises a constant tool set.
func TestHandshakeAndConstantToolList(t *testing.T) {
	c := startServer(t, &Server{Tools: Exposed(), Fwd: echoFwd()})
	c.send(initialize)
	m := c.next()
	res := m["result"].(map[string]any)
	if res["protocolVersion"] != "2025-06-18" {
		t.Errorf("%v", res)
	}
	tools := res["capabilities"].(map[string]any)["tools"].(map[string]any)
	if tools["listChanged"] != false {
		t.Errorf("capabilities %v", tools)
	}
	c.send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	c.silent()
	c.send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	list := c.next()["result"].(map[string]any)["tools"].([]any)
	if len(list) != len(Tools()) {
		t.Fatalf("%d tools", len(list))
	}
	for _, it := range list {
		tool := it.(map[string]any)
		schema := tool["inputSchema"].(map[string]any)
		if !strings.HasPrefix(tool["name"].(string), "desk_") || schema["additionalProperties"] != false {
			t.Errorf("tool %v", tool)
		}
	}
	// A second listing is identical: the set never changes.
	c.send(`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`)
	again, _ := json.Marshal(c.next()["result"])
	first, _ := json.Marshal(map[string]any{"tools": list})
	if string(again) != string(first) {
		t.Error("the tool list changed between calls")
	}
	// An unsupported protocol version gets the latest this server speaks.
	c2 := startServer(t, &Server{Tools: Exposed(), Fwd: echoFwd()})
	c2.send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}`)
	if v := c2.next()["result"].(map[string]any)["protocolVersion"]; v != latestProtocol {
		t.Errorf("version %v", v)
	}
}

// F6: hostile lines get an error and the server keeps serving.
func TestHostileLinesDoNotKillTheServer(t *testing.T) {
	c := startServer(t, &Server{Tools: Exposed(), Fwd: echoFwd()})
	c.send(`{"jsonrpc":"2.0","id":9,"method":"tools/list"}`)
	if code := errCode(c.next()); code != -32002 {
		t.Errorf("a request before initialize: code %d", code)
	}
	c.send(`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"desk_context"}}`)
	if code := errCode(c.next()); code != -32002 {
		t.Errorf("a call before initialize: code %d", code)
	}
	c.send(`this is not json`)
	if code := errCode(c.next()); code != -32700 {
		t.Errorf("garbage: code %d", code)
	}
	c.send(`"a string"`)
	if code := errCode(c.next()); code != -32700 {
		t.Errorf("a bare string: code %d", code)
	}
	c.send(`[{"jsonrpc":"2.0","id":1,"method":"ping"}]`)
	if code := errCode(c.next()); code != -32600 {
		t.Errorf("a batch: code %d", code)
	}
	c.send(`{"jsonrpc":"2.0","id":5,"method":"nonsense"}`)
	if code := errCode(c.next()); code != -32601 {
		t.Errorf("an unknown method: code %d", code)
	}
	c.send(`{"jsonrpc":"2.0","method":"nonsense","id":null}`)
	c.silent() // a notification: never answered
	c.send(`{"jsonrpc":"2.0","method":"notifications/whatever"}`)
	c.silent()
	c.send(`{"jsonrpc":"2.0","id":7,"result":{}}`) // a stray client response
	c.silent()
	c.send(`{"jsonrpc":"2.0","id":"s","method":"ping"}`)
	if m := c.next(); m["id"] != "s" || m["error"] != nil {
		t.Errorf("ping: %v", m)
	}
	// An oversize line is consumed, answered, and the server serves on.
	c.send(`{"jsonrpc":"2.0","id":1,"method":"x","params":"` + strings.Repeat("a", MaxLineBytes) + `"}`)
	if code := errCode(c.next()); code != -32600 {
		t.Errorf("an oversize line: code %d", code)
	}
	c.send(initialize)
	if c.next()["result"] == nil {
		t.Error("the server did not serve after an oversize line")
	}
}

// F2, F10, F14: unknown tools are protocol errors; a host that cannot be
// reached is a tool result that says so; a host refusal is an isError result.
func TestToolCallsAndTheirFailures(t *testing.T) {
	var gotTool string
	var gotArgs json.RawMessage
	fwd := fwdFunc(func(_ context.Context, tool string, args json.RawMessage) (Result, error) {
		gotTool, gotArgs = tool, args
		switch tool {
		case "desk_ask":
			return Result{}, errors.New("dial unix /x: no such file")
		case "desk_reply":
			return Result{Text: "refused: not yours", IsError: true}, nil
		}
		return Result{Text: "fine"}, nil
	})
	c := startServer(t, &Server{Tools: Exposed(), Fwd: fwd})
	c.send(initialize)
	c.next()
	call := func(id int, name, args string) map[string]any {
		c.send(`{"jsonrpc":"2.0","id":` + string(rune('0'+id)) + `,"method":"tools/call","params":{"name":"` + name + `","arguments":` + args + `}}`)
		return c.next()
	}
	if m := call(1, "desk_context", `{}`); m["result"].(map[string]any)["isError"] != false || gotTool != "desk_context" {
		t.Errorf("%v", m)
	}
	if string(gotArgs) != `{}` {
		t.Errorf("args %s", gotArgs)
	}
	if code := errCode(call(2, "desk_cast", `{}`)); code != -32602 {
		t.Errorf("an unknown tool: code %d", code)
	}
	m := call(3, "desk_ask", `{"question":"q"}`)["result"].(map[string]any)
	text := m["content"].([]any)[0].(map[string]any)["text"].(string)
	if m["isError"] != true || !strings.Contains(text, "unreachable") {
		t.Errorf("unreachable host: %v", m)
	}
	m = call(4, "desk_reply", `{}`)["result"].(map[string]any)
	if m["isError"] != true || m["content"].([]any)[0].(map[string]any)["text"] != "refused: not yours" {
		t.Errorf("refusal: %v", m)
	}
}

// F7: a blocked call sends progress notifications until it returns.
func TestBlockedCallSendsProgress(t *testing.T) {
	release := make(chan struct{})
	fwd := fwdFunc(func(ctx context.Context, _ string, _ json.RawMessage) (Result, error) {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return Result{Text: "records"}, nil
	})
	c := startServer(t, &Server{Tools: Exposed(), Fwd: fwd, ProgressEvery: 10 * time.Millisecond})
	c.send(initialize)
	c.next()
	c.send(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"desk_wait","arguments":{"timeout_s":60},"_meta":{"progressToken":"tok"}}}`)
	for i := 1; i <= 2; i++ {
		m := c.next()
		p, _ := m["params"].(map[string]any)
		if m["method"] != "notifications/progress" || p["progressToken"] != "tok" || int(p["progress"].(float64)) != i {
			t.Fatalf("progress %d: %v", i, m)
		}
	}
	close(release)
	for {
		m := c.next()
		if m["method"] == "notifications/progress" {
			continue
		}
		if m["id"] != float64(4) || m["result"] == nil {
			t.Fatalf("%v", m)
		}
		break
	}
	if ProgressInterval >= 30*time.Minute/4 {
		t.Errorf("progress interval %v is not far inside the idle abort", ProgressInterval)
	}
	if MaxWaitSeconds != 1500 {
		t.Errorf("wait cap %d", MaxWaitSeconds)
	}
}

func TestCancelEndsAForwardedCall(t *testing.T) {
	ended := make(chan struct{})
	fwd := fwdFunc(func(ctx context.Context, _ string, _ json.RawMessage) (Result, error) {
		<-ctx.Done()
		close(ended)
		return Result{Text: "cancelled"}, nil
	})
	c := startServer(t, &Server{Tools: Exposed(), Fwd: fwd})
	c.send(initialize)
	c.next()
	c.send(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"desk_wait","arguments":{"timeout_s":60}}}`)
	time.Sleep(30 * time.Millisecond)
	c.send(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":4}}`)
	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		t.Fatal("the cancel did not reach the call")
	}
}

func TestServeEndsOnEOF(t *testing.T) {
	c := startServer(t, &Server{Tools: Exposed(), Fwd: echoFwd()})
	c.send(initialize)
	c.next()
	_ = c.in.Close()
	select {
	case err := <-c.exit:
		if err != nil {
			t.Errorf("serve: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not end at EOF")
	}
}

// F9: the config that starts the server carries no environment, and the tool
// allowlist names exactly the catalogue.
func TestMcpConfigCarriesNoEnvironment(t *testing.T) {
	var cfg struct {
		Servers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(McpConfigJSON("/usr/local/bin/choir-desk-mcp", "/run/x.sock")), &cfg); err != nil {
		t.Fatal(err)
	}
	s := cfg.Servers[ServerName]
	if _, has := s["env"]; has || s["command"] != "/usr/local/bin/choir-desk-mcp" || len(cfg.Servers) != 1 {
		t.Errorf("config %v", cfg.Servers)
	}
	names := AllowedToolNames()
	if len(names) != len(Tools()) || !strings.HasPrefix(names[0], "mcp__choirdesk__desk_") {
		t.Errorf("names %v", names)
	}
}

// socketPath is short: a unix socket path has a small limit.
func socketPath(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("", "dm")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	return filepath.Join(d, "s")
}

func serveHost(t *testing.T, h *Host, b Binding) string {
	t.Helper()
	path := socketPath(t)
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = ServeSocket(ctx, l, h, b); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	})
	return path
}

// F1, F12: the socket carries the identity. A call to another assignment's
// socket is that assignment's; a frame naming an assignment is refused; a
// missing socket is an error, not a hang.
func TestIdentityIsTheSocketNotTheCall(t *testing.T) {
	be := newBackend()
	h := NewHost(be, nil)
	pathA := serveHost(t, h, Binding{AssignmentID: "A", RunID: "runA"})
	pathB := serveHost(t, h, Binding{AssignmentID: "B", RunID: "runB"})
	for _, p := range []string{pathA, pathB} {
		res, err := SocketForwarder{Path: p}.Forward(context.Background(), "desk_report", args(`{"claim":"c"}`))
		if err != nil || res.IsError {
			t.Fatalf("%v %+v", err, res)
		}
	}
	if len(be.bindings) != 2 || be.bindings[0].AssignmentID != "A" || be.bindings[1].AssignmentID != "B" || be.calls[1].Acting != "runB" {
		t.Errorf("bindings %+v", be.bindings)
	}
	// A forged frame: an assignment field the wire does not have.
	conn, err := net.Dial("unix", pathA)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, _ = io.WriteString(conn, `{"tool":"desk_report","assignment":"B","args":{"claim":"c"}}`+"\n")
	var resp wireResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil || !resp.IsError || !strings.Contains(resp.Text, "malformed call") {
		t.Errorf("forged frame: %v %+v", err, resp)
	}
	if len(be.calls) != 2 {
		t.Errorf("the forged frame reached the backend: %v", be.toolCalls())
	}
	// A forged verb: a tool outside the catalogue.
	res, err := SocketForwarder{Path: pathA}.Forward(context.Background(), "Cast", args(`{}`))
	if err != nil || !res.IsError {
		t.Errorf("forged verb: %v %+v", err, res)
	}
	// No host at the socket.
	start := time.Now()
	if _, err := (SocketForwarder{Path: filepath.Join(filepath.Dir(pathA), "absent")}).Forward(context.Background(), "desk_context", nil); err == nil || time.Since(start) > 6*time.Second {
		t.Errorf("a missing host: %v after %v", err, time.Since(start))
	}
}

// F10: a host that never answers ends at the context's deadline.
func TestSilentHostEndsAtTheDeadline(t *testing.T) {
	path := socketPath(t)
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() { _, _ = io.Copy(io.Discard, conn) }()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := (SocketForwarder{Path: path}).Forward(ctx, "desk_wait", args(`{"timeout_s":5}`)); err == nil || time.Since(start) > 3*time.Second {
		t.Errorf("silent host: %v after %v", err, time.Since(start))
	}
}

// The whole path, in memory: an MCP client calls desk_complete through the
// server, the socket and the host gate.
func TestEndToEndCompleteThroughTheServerAndSocket(t *testing.T) {
	be := completedBackend()
	h := NewHost(be, scriptedJudge{err: errors.New("down")})
	path := serveHost(t, h, bind)
	c := startServer(t, &Server{Tools: Exposed(), Fwd: SocketForwarder{Path: path}})
	c.send(initialize)
	c.next()
	c.send(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"desk_complete","arguments":` + completeArgs + `}}`)
	m := c.next()["result"].(map[string]any)
	text := m["content"].([]any)[0].(map[string]any)["text"].(string)
	if m["isError"] != false || !strings.Contains(text, "Management has been asked") || len(be.calls) != 0 {
		t.Errorf("%v %s", m, text)
	}
	var _ deskjudge.Judge = scriptedJudge{}
}

// B1: a desk_complete runs the assignment's checks and Jev's read inside one
// call, so its forward bound is the tool's own, not the default.
func TestSlowCompleteReturnsItsRealOutcome(t *testing.T) {
	fwd := fwdFunc(func(ctx context.Context, _ string, _ json.RawMessage) (Result, error) {
		select {
		case <-time.After(250 * time.Millisecond):
			return Result{Text: "the real outcome"}, nil
		case <-ctx.Done():
			return Result{}, ctx.Err()
		}
	})
	c := startServer(t, &Server{Tools: Exposed(), Fwd: fwd, DefaultForward: 50 * time.Millisecond})
	c.send(initialize)
	c.next()
	for i, name := range []string{"desk_complete", "desk_freeze"} {
		args := completeArgs
		if name == "desk_freeze" {
			args = `{"build_recipe_ref":"b","test_receipts":["t"],"dependency_toolchain_refs":["d"]}`
		}
		c.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, 10+i, name, args))
		m := c.next()["result"].(map[string]any)
		if got := m["content"].([]any)[0].(map[string]any)["text"]; got != "the real outcome" || m["isError"] != false {
			t.Errorf("%s: %v", name, m)
		}
	}
	// A tool with no bound of its own still ends at the default.
	c.send(`{"jsonrpc":"2.0","id":20,"method":"tools/call","params":{"name":"desk_context","arguments":{}}}`)
	m := c.next()["result"].(map[string]any)
	if m["isError"] != true {
		t.Errorf("the default bound did not apply: %v", m)
	}
}

// A protocol version this server does not implement is answered with the
// latest it does.
func TestUnimplementedProtocolVersionIsNotEchoed(t *testing.T) {
	c := startServer(t, &Server{Tools: Exposed(), Fwd: echoFwd()})
	c.send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`)
	if v := c.next()["result"].(map[string]any)["protocolVersion"]; v != latestProtocol {
		t.Errorf("version %v", v)
	}
}

func TestCompleteBoundCoversChecksAndTheJevRead(t *testing.T) {
	c, _ := Lookup("desk_complete")
	if c.Forward < 30*time.Minute {
		t.Errorf("desk_complete bound %v", c.Forward)
	}
	f, _ := Lookup("desk_freeze")
	if f.Forward < 10*time.Minute {
		t.Errorf("desk_freeze bound %v", f.Forward)
	}
}
