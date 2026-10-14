package vmctl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// Guest side of memory leases (docs/reports/memory-hotplug-controller-design-2026-10-14.md
// section 4.6). The guest runtime calls vmctl over its tap
// (RUNTIME_VMCTL_URL); vmctl derives the computer from the source address,
// so these calls name no computer and can only change the caller's own.

// MemoryLeaseError is a refused or failed lease request.
type MemoryLeaseError struct {
	Status int
	Reason string
}

func (e *MemoryLeaseError) Error() string {
	return fmt.Sprintf("vmctl memory lease: status %d: %s", e.Status, e.Reason)
}

// GuestMemoryLease asks for the caller's computer to have targetMiB of
// total memory for ttl (0: vmctl's default), waiting up to wait for the
// guest to plug it. targetMiB at or below the base releases.
func (c *Client) GuestMemoryLease(ctx context.Context, targetMiB int, ttl, wait time.Duration) (*MemoryHotplugView, error) {
	return c.postMemory(ctx, GuestMemoryEndpoint(c.baseURL), memoryRequestBody{
		TargetMiB:   targetMiB,
		TTLSeconds:  int(ttl / time.Second),
		WaitSeconds: int(wait / time.Second),
	}, false)
}

// ReleaseGuestMemoryLease returns the caller's computer to its base.
func (c *Client) ReleaseGuestMemoryLease(ctx context.Context) (*MemoryHotplugView, error) {
	return c.GuestMemoryLease(ctx, 0, 0, 0)
}

// SetComputerMemory is the host-side request (hostauth credential):
// targetMiB of total memory for one computer.
func (c *Client) SetComputerMemory(ctx context.Context, computerID string, targetMiB int, ttl, wait time.Duration) (*MemoryHotplugView, error) {
	return c.postMemory(ctx, MemoryEndpoint(c.baseURL), memoryRequestBody{
		ComputerID:  computerID,
		TargetMiB:   targetMiB,
		TTLSeconds:  int(ttl / time.Second),
		WaitSeconds: int(wait / time.Second),
	}, true)
}

func (c *Client) postMemory(ctx context.Context, endpoint string, body memoryRequestBody, internal bool) (*MemoryHotplugView, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if internal {
		hostauth.MarkInternal(req)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vmctl memory lease: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, fmt.Errorf("vmctl memory lease: read answer: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var errResp vmctlErrorResponse
		reason := resp.Status
		if json.Unmarshal(raw, &errResp) == nil && errResp.Error != "" {
			reason = errResp.Error
		}
		return nil, &MemoryLeaseError{Status: resp.StatusCode, Reason: reason}
	}
	var view MemoryHotplugView
	if err := json.Unmarshal(raw, &view); err != nil {
		return nil, fmt.Errorf("vmctl memory lease: decode answer: %w", err)
	}
	return &view, nil
}

// memoryLeaseReleaseTimeout bounds the release after fn returns; a lost
// release is ended by the lease TTL.
const memoryLeaseReleaseTimeout = 10 * time.Second

// WithGuestMemoryLease holds a lease of targetMiB total memory around fn:
// request (waiting up to wait for the plug), renew at half the TTL while
// fn runs, release when fn returns, even if ctx was cancelled. A refused
// grant returns the refusal without running fn; the caller decides whether
// to run smaller. A failed renewal is logged and left to the TTL.
func WithGuestMemoryLease(ctx context.Context, c *Client, targetMiB int, ttl, wait time.Duration, fn func(context.Context, *MemoryHotplugView) error) error {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	view, err := c.GuestMemoryLease(ctx, targetMiB, ttl, wait)
	if err != nil {
		return err
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), memoryLeaseReleaseTimeout)
		defer cancel()
		if _, err := c.ReleaseGuestMemoryLease(releaseCtx); err != nil {
			log.Printf("vmctl memory lease: release failed (the lease TTL ends it): %v", err)
		}
	}()
	renewCtx, stopRenew := context.WithCancel(ctx)
	defer stopRenew()
	go func() {
		ticker := time.NewTicker(ttl / 2)
		defer ticker.Stop()
		for {
			select {
			case <-renewCtx.Done():
				return
			case <-ticker.C:
				if _, err := c.GuestMemoryLease(renewCtx, targetMiB, ttl, 0); err != nil && renewCtx.Err() == nil {
					log.Printf("vmctl memory lease: renewal failed: %v", err)
				}
			}
		}
	}()
	return fn(ctx, view)
}
