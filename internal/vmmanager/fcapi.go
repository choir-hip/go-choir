package vmmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// fcAPIClient speaks the Firecracker HTTP API over a VM's unix socket. It is
// used only in snapshot hibernate mode (S3): pause, snapshot create, and
// snapshot load. Firecracker 1.15.1 serves the API alongside --config-file
// when --api-sock is given (docs/getting-started.md; main.rs run_with_api).
type fcAPIClient struct {
	sock string
	http *http.Client
}

func newFCAPIClient(sock string) *fcAPIClient {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", sock)
		},
		DisableKeepAlives: true,
	}
	return &fcAPIClient{sock: sock, http: &http.Client{Transport: transport}}
}

// fcAPIError is a non-2xx answer from Firecracker. FaultMessage carries the
// API's own explanation.
type fcAPIError struct {
	Method       string
	Path         string
	Status       int
	FaultMessage string
}

func (e *fcAPIError) Error() string {
	return fmt.Sprintf("firecracker API %s %s: status %d: %s", e.Method, e.Path, e.Status, e.FaultMessage)
}

func (c *fcAPIClient) do(ctx context.Context, method, path string, body any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://localhost"+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("firecracker API %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	var fault struct {
		FaultMessage string `json:"fault_message"`
	}
	_ = json.Unmarshal(raw, &fault)
	if fault.FaultMessage == "" {
		fault.FaultMessage = strings.TrimSpace(string(raw))
	}
	return &fcAPIError{Method: method, Path: path, Status: resp.StatusCode, FaultMessage: fault.FaultMessage}
}

// Ping answers whether the API socket is served (GET / is instance info).
func (c *fcAPIClient) Ping(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/", nil)
}

// WaitReady polls the socket until Firecracker answers or ctx ends.
func (c *fcAPIClient) WaitReady(ctx context.Context) error {
	for {
		attempt, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		err := c.Ping(attempt)
		cancel()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("firecracker API socket %s not ready: %w", c.sock, err)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// Pause stops the vCPUs (PATCH /vm {"state":"Paused"}).
func (c *fcAPIClient) Pause(ctx context.Context) error {
	return c.do(ctx, http.MethodPatch, "/vm", map[string]string{"state": "Paused"})
}

// CreateFullSnapshot writes the VM state and the full guest memory. The VM
// must be paused.
func (c *fcAPIClient) CreateFullSnapshot(ctx context.Context, statePath, memPath string) error {
	return c.do(ctx, http.MethodPut, "/snapshot/create", map[string]string{
		"snapshot_type": "Full",
		"snapshot_path": statePath,
		"mem_file_path": memPath,
	})
}

// LoadSnapshot restores a full snapshot into a fresh Firecracker process
// (only valid before any other configuration) and resumes the VM. A failed
// load ends the Firecracker process (snapshot-support.md, "Loading
// snapshots").
func (c *fcAPIClient) LoadSnapshot(ctx context.Context, statePath, memPath string) error {
	return c.do(ctx, http.MethodPut, "/snapshot/load", map[string]any{
		"snapshot_path": statePath,
		"mem_backend": map[string]string{
			"backend_type": "File",
			"backend_path": memPath,
		},
		"resume_vm": true,
	})
}
