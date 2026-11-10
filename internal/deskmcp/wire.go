package deskmcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// The wire between the in-capsule server and the host is one JSON line each
// way over a per-run Unix socket. A request has two fields. There is no
// field for an assignment, a desk, a target or a run: the host binds those to
// the socket (F1), and the decoder refuses a frame that carries any other.
type wireRequest struct {
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args,omitempty"`
}

type wireResponse struct {
	Text    string `json:"text"`
	IsError bool   `json:"is_error,omitempty"`
}

const dialTimeout = 5 * time.Second

// SocketForwarder forwards calls to the host over its socket.
type SocketForwarder struct{ Path string }

// Forward sends one call and waits for the answer or the context's end. A
// missing or silent host is an error, never a hang (F10).
func (f SocketForwarder) Forward(ctx context.Context, tool string, args json.RawMessage) (Result, error) {
	d := net.Dialer{Timeout: dialTimeout}
	conn, err := d.DialContext(ctx, "unix", f.Path)
	if err != nil {
		return Result{}, err
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	} else {
		_ = conn.SetDeadline(time.Now().Add(defaultForward))
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if err := json.NewEncoder(conn).Encode(wireRequest{Tool: tool, Args: args}); err != nil {
		return Result{}, err
	}
	var resp wireResponse
	if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&resp); err != nil {
		return Result{}, fmt.Errorf("no answer: %w", err)
	}
	return Result{Text: resp.Text, IsError: resp.IsError}, nil
}

// ServeSocket answers calls on l for one binding until ctx ends. Each
// connection carries one call.
func ServeSocket(ctx context.Context, l net.Listener, h *Host, b Binding) error {
	go func() { <-ctx.Done(); _ = l.Close() }()
	for {
		conn, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go serveConn(ctx, conn, h, b)
	}
}

func serveConn(ctx context.Context, conn net.Conn, h *Host, b Binding) {
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	line, tooLong, err := readLine(bufio.NewReaderSize(conn, 64<<10), MaxLineBytes)
	reply := func(r Result) {
		_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
		_ = json.NewEncoder(conn).Encode(wireResponse{Text: r.Text, IsError: r.IsError})
	}
	if tooLong {
		reply(refusal("the call is over %d bytes", MaxLineBytes))
		return
	}
	if err != nil && len(line) == 0 {
		return
	}
	var req wireRequest
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		reply(refusal("malformed call: %v", err))
		return
	}
	// The read deadline covered the request only; the answer may take as long
	// as the tool does, bounded by the caller closing the connection.
	_ = conn.SetReadDeadline(time.Time{})
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { // a closed connection ends the call
		var one [1]byte
		_, _ = conn.Read(one[:])
		cancel()
	}()
	reply(h.Handle(cctx, b, req.Tool, req.Args))
}
