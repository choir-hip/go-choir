//go:build linux


package capsule

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

// BrokerClient communicates with a capsule's exec-broker via JSON-RPC
// over a Unix domain socket. The broker runs inside the capsule's
// namespace; the Executor connects to the socket from outside.
type BrokerClient struct {
	conn       net.Conn
	socketPath string
	publicKey  ed25519.PublicKey // injected at spawn, used for capability verification
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

// NewBrokerClient creates a new broker client for the given Unix socket path.
func NewBrokerClient(socketPath string, publicKey ed25519.PublicKey) *BrokerClient {
	return &BrokerClient{
		socketPath: socketPath,
		publicKey:  publicKey,
	}
}

// Connect establishes a connection to the broker's Unix socket.
func (b *BrokerClient) Connect(ctx context.Context) error {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "unix", b.socketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to broker socket %s: %w", b.socketPath, err)
	}
	b.conn = conn
	return nil
}

// Close closes the broker connection.
func (b *BrokerClient) Close() error {
	if b.conn == nil {
		return nil
	}
	return b.conn.Close()
}

// call sends a broker RPC and returns the response.
func (b *BrokerClient) call(ctx context.Context, verb string, cap *Capability, params interface{}) (*BrokerRPCResponse, error) {
	if b.conn == nil {
		return nil, fmt.Errorf("not connected to broker")
	}

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

	if deadline, ok := ctx.Deadline(); ok {
		b.conn.SetWriteDeadline(deadline)
	} else {
		b.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	}

	if err := json.NewEncoder(b.conn).Encode(&req); err != nil {
		return nil, fmt.Errorf("failed to send broker RPC: %w", err)
	}

	if deadline, ok := ctx.Deadline(); ok {
		b.conn.SetReadDeadline(deadline)
	} else {
		b.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	}

	var resp BrokerRPCResponse
	if err := json.NewDecoder(b.conn).Decode(&resp); err != nil {
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
