//go:build linux

package capsule

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsulejob"
)

// BrokerClient communicates with a capsule's exec-broker via JSON-RPC
// over a Unix domain socket. The broker runs inside the capsule's
// namespace; the Executor connects to the socket from outside.
type BrokerClient struct {
	// socketPath is the socket's real filesystem path, however deep. Connect
	// derives a dialable address from it on every dial (unixSocketAddress),
	// because a path past sun_path is reached through a /proc/self/fd/<N>
	// directory descriptor that must be open only for the dial. A client that
	// stored that address would re-dial through a closed, reused descriptor
	// (docs/archive/problems/selfdev-jobs-0654-raced-activation-verified-engineering-budget-2026-10-15.md).
	socketPath string
	publicKey  ed25519.PublicKey // injected at spawn, used for capability verification

	// connMu serializes the full encode/decode round trip on the shared Unix
	// socket. A single net.Conn supports only one in-flight JSON-RPC exchange;
	// without serialization, two concurrent calls can interleave frames and one
	// caller reads the other's response. Operation admission (acquireOp) counts
	// inflight work for quiesce but does not serialize this stream.
	connMu sync.Mutex

	// stateMu guards the connection fields, so Close can interrupt an
	// exchange in flight without waiting for connMu.
	stateMu sync.Mutex
	conn    net.Conn
	// dec is the connection's one decoder: a per-call decoder drops bytes it
	// buffered past the frame it decoded.
	dec *json.Decoder
	// dialed is set by the first Connect; a connection dropped after a
	// transport error is re-dialed by the next call.
	dialed bool
	closed bool
}

// BrokerRPCRequest is the wire format for broker RPCs.
type BrokerRPCRequest struct {
	Verb       string          `json:"verb"`
	Capability json.RawMessage `json:"capability"`
	Params     json.RawMessage `json:"params"`
}

// BrokerRPCResponse is the wire format for broker responses.
type BrokerRPCResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// NewBrokerClient creates a new broker client for the broker socket at
// socketPath, the real path (not a /proc/self/fd address).
func NewBrokerClient(socketPath string, publicKey ed25519.PublicKey) *BrokerClient {
	return &BrokerClient{
		socketPath: socketPath,
		publicKey:  publicKey,
	}
}

// Connect establishes a connection to the broker's Unix socket.
func (b *BrokerClient) Connect(ctx context.Context) error {
	addr, release, err := unixSocketAddress(b.socketPath)
	if err != nil {
		return fmt.Errorf("failed to address broker socket %s: %w", b.socketPath, err)
	}
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "unix", addr)
	release() // the descriptor is needed only while connect resolves the path
	if err != nil {
		// Name the real path: the net error carries the transient
		// /proc/self/fd address, which means nothing once the descriptor is
		// closed.
		var opErr *net.OpError
		if errors.As(err, &opErr) && opErr.Err != nil {
			err = opErr.Err
		}
		return fmt.Errorf("failed to connect to broker socket %s: %w", b.socketPath, err)
	}
	b.stateMu.Lock()
	if b.closed {
		b.stateMu.Unlock()
		_ = conn.Close()
		return fmt.Errorf("broker client is closed")
	}
	old := b.conn
	b.conn, b.dec, b.dialed = conn, json.NewDecoder(conn), true
	b.stateMu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}

// Close closes the broker connection. An exchange in flight fails, and no
// later call re-dials.
func (b *BrokerClient) Close() error {
	b.stateMu.Lock()
	conn := b.conn
	b.conn, b.dec, b.closed = nil, nil, true
	b.stateMu.Unlock()
	if conn == nil {
		return nil
	}
	return conn.Close()
}

// connection returns the live connection and its decoder, re-dialing one a
// transport error dropped. The caller holds connMu.
func (b *BrokerClient) connection(ctx context.Context) (net.Conn, *json.Decoder, error) {
	b.stateMu.Lock()
	if b.conn != nil && b.dec == nil {
		b.dec = json.NewDecoder(b.conn) // a client built around an open conn
	}
	conn, dec, dialed, closed := b.conn, b.dec, b.dialed, b.closed
	b.stateMu.Unlock()
	switch {
	case closed:
		return nil, nil, fmt.Errorf("broker client is closed")
	case conn != nil:
		return conn, dec, nil
	case !dialed:
		return nil, nil, fmt.Errorf("not connected to broker")
	}
	if err := b.Connect(ctx); err != nil {
		return nil, nil, err
	}
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	if b.conn == nil {
		return nil, nil, fmt.Errorf("broker client is closed")
	}
	return b.conn, b.dec, nil
}

// drop discards a connection whose stream can no longer be trusted: an
// answer that missed its deadline still arrives on it, and a frame read in
// part leaves the rest behind, so the next exchange would read another
// call's answer (docs/problems/selfdev-go-drill-jobs-0101-2026-10-15.md).
func (b *BrokerClient) drop(conn net.Conn) {
	b.stateMu.Lock()
	if b.conn == conn {
		b.conn, b.dec = nil, nil
	}
	b.stateMu.Unlock()
	_ = conn.Close()
}

// call sends a broker RPC and returns the response. A transport or framing
// error drops the connection; the next call dials a fresh one.
func (b *BrokerClient) call(ctx context.Context, verb string, cap *Capability, params interface{}) (*BrokerRPCResponse, error) {
	capBytes, err := cap.MarshalForTransport()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal capability: %w", err)
	}

	paramBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	req := BrokerRPCRequest{
		Verb:       verb,
		Capability: capBytes,
		Params:     paramBytes,
	}

	// Serialize the whole exchange so concurrent calls do not interleave frames
	// on the shared connection.
	b.connMu.Lock()
	defer b.connMu.Unlock()

	conn, dec, err := b.connection(ctx)
	if err != nil {
		return nil, err
	}

	if deadline, ok := ctx.Deadline(); ok {
		conn.SetWriteDeadline(deadline)
	} else {
		conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	}

	if err := json.NewEncoder(conn).Encode(&req); err != nil {
		b.drop(conn)
		return nil, fmt.Errorf("failed to send broker RPC: %w", err)
	}

	if deadline, ok := ctx.Deadline(); ok {
		conn.SetReadDeadline(deadline)
	} else {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	}

	var resp BrokerRPCResponse
	if err := dec.Decode(&resp); err != nil {
		b.drop(conn)
		return nil, fmt.Errorf("failed to read broker response: %w", err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("broker error: %s", resp.Error)
	}

	return &resp, nil
}

// Exec sends an exec command to the broker.
func (b *BrokerClient) Exec(ctx context.Context, cap *Capability, req ExecRequest) (ExecResult, error) {
	resp, err := b.call(ctx, "exec", cap, req)
	if err != nil {
		return ExecResult{}, err
	}

	var result ExecResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return ExecResult{}, fmt.Errorf("failed to unmarshal exec result: %w", err)
	}
	return result, nil
}

// GoEval evaluates model-authored Go source in the capsule via the go_eval verb.
func (b *BrokerClient) GoEval(ctx context.Context, cap *Capability, req GoEvalRequest) (GoEvalResult, error) {
	resp, err := b.call(ctx, "go_eval", cap, req)
	if err != nil {
		return GoEvalResult{}, err
	}

	var result GoEvalResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return GoEvalResult{}, fmt.Errorf("failed to unmarshal go_eval result: %w", err)
	}
	return result, nil
}

// Job sends one job verb (job_start, job_status, job_stop) to the broker.
func (b *BrokerClient) Job(ctx context.Context, verb string, cap *Capability, params JobParams) (capsulejob.Status, error) {
	resp, err := b.call(ctx, verb, cap, params)
	if err != nil {
		return capsulejob.Status{}, err
	}
	var status capsulejob.Status
	if err := json.Unmarshal(resp.Result, &status); err != nil {
		return capsulejob.Status{}, fmt.Errorf("failed to unmarshal %s result: %w", verb, err)
	}
	return status, nil
}

// ReadFile reads a file from the capsule via the broker.
func (b *BrokerClient) ReadFile(ctx context.Context, cap *Capability, path string) ([]byte, error) {
	resp, err := b.call(ctx, "read_file", cap, map[string]string{"path": path})
	if err != nil {
		return nil, err
	}

	var result struct {
		Content []byte `json:"content"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal read_file result: %w", err)
	}
	return result.Content, nil
}

// WriteFile writes a file to the capsule via the broker.
func (b *BrokerClient) WriteFile(ctx context.Context, cap *Capability, path string, content []byte, mode uint32) error {
	_, err := b.call(ctx, "write_file", cap, map[string]interface{}{
		"path":    path,
		"content": content,
		"mode":    mode,
	})
	return err
}

// ListDir lists directory contents in the capsule via the broker.
func (b *BrokerClient) ListDir(ctx context.Context, cap *Capability, path string) ([]string, error) {
	resp, err := b.call(ctx, "list_dir", cap, map[string]string{"path": path})
	if err != nil {
		return nil, err
	}

	var entries []string
	if err := json.Unmarshal(resp.Result, &entries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal list_dir result: %w", err)
	}
	return entries, nil
}

// Stat returns file stat info from the capsule via the broker.
func (b *BrokerClient) Stat(ctx context.Context, cap *Capability, path string) (os.FileInfo, error) {
	resp, err := b.call(ctx, "stat", cap, map[string]string{"path": path})
	if err != nil {
		return nil, err
	}

	var info FileInfo
	if err := json.Unmarshal(resp.Result, &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stat result: %w", err)
	}
	return &info, nil
}

// FileInfo is a JSON-serializable representation of os.FileInfo.
type FileInfo struct {
	FiName    string `json:"name"`
	FiSize    int64  `json:"size"`
	FiMode    uint32 `json:"mode"`
	FiIsDir   bool   `json:"is_dir"`
	FiModTime int64  `json:"mod_time_unix"`
}

func (fi *FileInfo) Name() string       { return fi.FiName }
func (fi *FileInfo) Size() int64        { return fi.FiSize }
func (fi *FileInfo) Mode() os.FileMode  { return os.FileMode(fi.FiMode) }
func (fi *FileInfo) ModTime() time.Time { return time.Unix(fi.FiModTime, 0) }
func (fi *FileInfo) IsDir() bool        { return fi.FiIsDir }
func (fi *FileInfo) Sys() interface{}   { return nil }

// SyncRevokedCaps sends the updated revoked capability set to the broker.
func (b *BrokerClient) SyncRevokedCaps(ctx context.Context, revokedIDs []string) error {
	_, err := b.call(ctx, "sync_revoked_caps", nil, map[string][]string{
		"revoked_capability_ids": revokedIDs,
	})
	return err
}
