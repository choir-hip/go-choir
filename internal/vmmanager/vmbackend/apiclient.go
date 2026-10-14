package vmbackend

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

// unixAPI speaks a VMM's HTTP control API over its unix socket. Firecracker
// and Cloud Hypervisor both serve REST over a unix socket; only paths and
// bodies differ.
type unixAPI struct {
	label string
	sock  string
	http  *http.Client
}

func newUnixAPI(label, sock string) *unixAPI {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", sock)
		},
		DisableKeepAlives: true,
	}
	return &unixAPI{label: label, sock: sock, http: &http.Client{Transport: transport}}
}

// APIError is a non-2xx answer from a VMM API.
type APIError struct {
	Backend Name
	Method  string
	Path    string
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s API %s %s: status %d: %s", e.Backend, e.Method, e.Path, e.Status, e.Message)
}

// do sends body (JSON, or nothing) and decodes a 2xx JSON answer into out
// when out is non-nil.
func (c *unixAPI) do(ctx context.Context, backend Name, method, path string, body, out any) error {
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
		return fmt.Errorf("%s API %s %s: %w", backend, method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var fault struct {
			FaultMessage string `json:"fault_message"`
		}
		_ = json.Unmarshal(raw, &fault)
		msg := fault.FaultMessage
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
		}
		return &APIError{Backend: backend, Method: method, Path: path, Status: resp.StatusCode, Message: msg}
	}
	if out != nil && len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("%s API %s %s: decode: %w", backend, method, path, err)
		}
	}
	return nil
}

// waitReady polls ping until it answers or ctx ends.
func waitReady(ctx context.Context, sock string, ping func(context.Context) error) error {
	for {
		attempt, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		err := ping(attempt)
		cancel()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("API socket %s not ready: %w", sock, err)
		case <-time.After(20 * time.Millisecond):
		}
	}
}
