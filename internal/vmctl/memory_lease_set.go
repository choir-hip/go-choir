package vmctl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// GuestMemoryLeases is the guest's side of its computer's one memory lease
// (docs/reports/memory-hotplug-controller-design-2026-10-14.md section 9):
// every capsule job that declares a memory need adds it, and the lease's
// target is the computer's base plus the sum of the live needs. One job's
// end steps the target down without taking another job's memory; the last
// one releases the lease. While any need is live the lease is renewed at
// half its TTL; once none is, nothing renews it, so a lost release ends at
// the TTL.
type GuestMemoryLeases struct {
	client *Client
	ttl    time.Duration
	wait   time.Duration

	mu      sync.Mutex
	baseMiB int
	needs   map[uint64]int
	next    uint64
	sum     int
	stop    context.CancelFunc
}

// errNoMemoryLeaseClient is the unavailable answer of a guest with no vmctl
// URL.
var errNoMemoryLeaseClient = &MemoryLeaseError{Status: http.StatusNotFound, Reason: "no vmctl client in this guest"}

// NewGuestMemoryLeases builds the lease set. ttl 0 is vmctl's default (10
// minutes); wait bounds how long a grow waits for the guest to plug it.
func NewGuestMemoryLeases(c *Client, ttl, wait time.Duration) *GuestMemoryLeases {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &GuestMemoryLeases{client: c, ttl: ttl, wait: wait, needs: map[uint64]int{}}
}

// MemoryLeaseUnavailable reports whether err means this computer has no
// lease to give (hotplug off, the computer not selected, no device, no
// vmctl client), as opposed to a grow vmctl refused. A caller with a need
// then runs on the memory it already has.
func MemoryLeaseUnavailable(err error) bool {
	var leaseErr *MemoryLeaseError
	if !errors.As(err, &leaseErr) {
		return false
	}
	return leaseErr.Status == http.StatusNotFound || leaseErr.Status == http.StatusConflict
}

// LiveMiB is the sum of the live needs.
func (g *GuestMemoryLeases) LiveMiB() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.sum
}

// Acquire adds needMiB to the computer's lease, waiting for the plug, and
// returns the lease view and the need's release (idempotent; it uses its
// own context, so a cancelled job still releases). A refusal leaves the
// live needs unchanged.
func (g *GuestMemoryLeases) Acquire(ctx context.Context, needMiB int) (*MemoryHotplugView, func(), error) {
	if g == nil || g.client == nil {
		return nil, nil, errNoMemoryLeaseClient
	}
	if needMiB <= 0 {
		return nil, nil, fmt.Errorf("vmctl memory lease: need must be positive, not %d MiB", needMiB)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.baseMiB == 0 {
		view, err := g.client.GuestMemoryView(ctx)
		if err != nil {
			return nil, nil, err
		}
		if view.BaseMiB <= 0 {
			return nil, nil, &MemoryLeaseError{Status: http.StatusConflict, Reason: "vmctl reports no base memory for this computer"}
		}
		g.baseMiB = view.BaseMiB
	}
	view, err := g.client.GuestMemoryLease(ctx, g.baseMiB+g.sum+needMiB, g.ttl, g.wait)
	if err != nil {
		return nil, nil, err
	}
	g.next++
	id := g.next
	g.needs[id] = needMiB
	g.sum += needMiB
	if g.stop == nil {
		renewCtx, stop := context.WithCancel(context.Background())
		g.stop = stop
		go g.renew(renewCtx)
	}
	var once sync.Once
	return view, func() { once.Do(func() { g.release(id) }) }, nil
}

// release removes one need and sets the lease to what is left.
func (g *GuestMemoryLeases) release(id uint64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	need, ok := g.needs[id]
	if !ok {
		return
	}
	delete(g.needs, id)
	g.sum -= need
	ctx, cancel := context.WithTimeout(context.Background(), memoryLeaseReleaseTimeout)
	defer cancel()
	var err error
	if g.sum == 0 {
		if g.stop != nil {
			g.stop()
			g.stop = nil
		}
		_, err = g.client.ReleaseGuestMemoryLease(ctx)
	} else {
		_, err = g.client.GuestMemoryLease(ctx, g.baseMiB+g.sum, g.ttl, 0)
	}
	if err != nil {
		log.Printf("vmctl memory lease: step down by %d MiB failed (the lease TTL ends it): %v", need, err)
	}
}

// renew re-asks for the live target at half the TTL until no need is live.
func (g *GuestMemoryLeases) renew(ctx context.Context) {
	ticker := time.NewTicker(g.ttl / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		g.mu.Lock()
		if ctx.Err() != nil || g.sum == 0 {
			g.mu.Unlock()
			return
		}
		reqCtx, cancel := context.WithTimeout(ctx, memoryLeaseReleaseTimeout)
		_, err := g.client.GuestMemoryLease(reqCtx, g.baseMiB+g.sum, g.ttl, 0)
		cancel()
		g.mu.Unlock()
		if err != nil && ctx.Err() == nil {
			log.Printf("vmctl memory lease: renewal failed: %v", err)
		}
	}
}

// ReleaseStale releases whatever lease an earlier guest process held: at
// start no job is live (a restart closes them), so no need is either (J10).
// An unavailable lease is not an error here.
func (g *GuestMemoryLeases) ReleaseStale(ctx context.Context) error {
	if g == nil || g.client == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.sum > 0 {
		return nil
	}
	if _, err := g.client.ReleaseGuestMemoryLease(ctx); err != nil && !MemoryLeaseUnavailable(err) {
		return err
	}
	return nil
}

// GuestMemoryView reads the caller's own computer's memory (its base and
// any lease) from vmctl.
func (c *Client) GuestMemoryView(ctx context.Context) (*MemoryHotplugView, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GuestMemoryEndpoint(c.baseURL), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vmctl memory view: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, fmt.Errorf("vmctl memory view: read answer: %w", err)
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
		return nil, fmt.Errorf("vmctl memory view: decode answer: %w", err)
	}
	return &view, nil
}
