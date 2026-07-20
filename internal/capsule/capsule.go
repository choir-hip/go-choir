//go:build linux

package capsule

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

// Capsule represents a single capsule instance — an isolated execution
// environment with its own namespaces, cgroups, overlayfs, and broker.
type Capsule struct {
	mu                   sync.RWMutex
	ID                   string
	PID                  int
	UpperDir             string
	WorkDir              string
	SourceSnapshotDigest string
	MergedDir            string
	MemoryMax            int64 // memory budget for admission control accounting
	State                CapsuleState
	CommitEpoch          uint64         // audit metadata (not enforced for exec/read/write)
	LastManifest         []FileManifest // last committed snapshot
	OwnerRunID           string
	StartedAt            time.Time
	Spec                 SpawnSpec
	Process              *os.Process
	Cgroup               *CgroupManager
	wait                 func() error
	listener             net.Listener
	processDone          chan struct{}
	processErr           error
	Pinned               bool
	PinExpiry            *time.Time
	inflightOps          int
	inflightMu           sync.Mutex

	// broker is the client for communicating with this capsule's broker.
	broker *BrokerClient

	// revokedCaps is the guest-core revocation set mirrored for local checks.
	revokedCaps map[string]bool
}

// Exec executes a command in the capsule via the broker.
func (c *Capsule) Exec(ctx context.Context, cap *Capability, req ExecRequest) (ExecResult, error) {

	if !cap.AgentRole.HasVerb("exec") {
		return ExecResult{}, fmt.Errorf("role %s does not allow exec", cap.AgentRole)
	}

	if err := VerifyCapabilityWithKey(cap, c.broker.publicKey, c.revokedCaps); err != nil {
		return ExecResult{}, fmt.Errorf("capability verification failed: %w", err)
	}

	if err := c.acquireOp(); err != nil {
		return ExecResult{}, err
	}
	defer c.releaseOp()

	return c.broker.Exec(ctx, cap, req)
}

// Quiesce freezes the capsule — no new operations accepted, existing
// operations complete. Used before snapshot/commit.
func (c *Capsule) Quiesce(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.State != StateActive {
		return fmt.Errorf("capsule %s is not active (state=%s)", c.ID, c.State)
	}

	c.State = StateQuiescing
	// Wait for inflight operations to complete.
	c.inflightMu.Lock()
	for c.inflightOps > 0 {
		c.inflightMu.Unlock()
		select {
		case <-ctx.Done():
			c.inflightMu.Lock()
			return ctx.Err()
		default:
			c.inflightMu.Lock()
		}
	}
	c.inflightMu.Unlock()

	c.State = StateFrozen
	return nil
}

// Thaw unfreezes a quiesced capsule — operations resume.
func (c *Capsule) Thaw(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.State != StateFrozen {
		return fmt.Errorf("capsule %s is not frozen (state=%s)", c.ID, c.State)
	}

	c.State = StateActive
	return nil
}

// Diff computes the snapshot diff between the current upperdir state
// and the last committed manifest. Does not require remount (crash-safe).
func (c *Capsule) Diff(ctx context.Context) ([]FileChange, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.State == StateDestroyed {
		return nil, fmt.Errorf("capsule %s is destroyed", c.ID)
	}

	current, err := walkUpperdir(c.UpperDir)
	if err != nil {
		return nil, fmt.Errorf("failed to walk upperdir: %w", err)
	}

	return diffManifests(c.LastManifest, current), nil
}

// CommitManifest records the current upperdir state as the new committed
// manifest. Called after the host has extracted the diff and appended it
// to the tape.
func (c *Capsule) CommitManifest(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.State == StateDestroyed {
		return fmt.Errorf("capsule %s is destroyed", c.ID)
	}

	current, err := walkUpperdir(c.UpperDir)
	if err != nil {
		return fmt.Errorf("failed to walk upperdir for commit: %w", err)
	}

	c.LastManifest = current
	c.CommitEpoch++
	return nil
}

// Destroy is intentionally owned by Executor, which tears down the process,
// overlay mount, cgroup, and ephemeral grants as one lifecycle transition.
func (c *Capsule) Destroy(ctx context.Context) error {
	return fmt.Errorf("capsule destruction must be performed by Executor")
}

// acquireOp increments the inflight operation counter. Must be paired
// with releaseOp. Rejects if capsule is not active or is quiescing.
func (c *Capsule) acquireOp() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.State != StateActive {
		return fmt.Errorf("capsule %s is not active (state=%s)", c.ID, c.State)
	}

	c.inflightMu.Lock()
	c.inflightOps++
	c.inflightMu.Unlock()
	return nil
}

// releaseOp decrements the inflight operation counter.
func (c *Capsule) releaseOp() {
	c.inflightMu.Lock()
	c.inflightOps--
	c.inflightMu.Unlock()
}

// UpdateRevokedCaps replaces the guest-core revocation view.
func (c *Capsule) UpdateRevokedCaps(revokedIDs []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.revokedCaps = make(map[string]bool, len(revokedIDs))
	for _, id := range revokedIDs {
		c.revokedCaps[id] = true
	}
}

// IsPinned returns whether the capsule is pinned (long-lived).
func (c *Capsule) IsPinned() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Pinned
}
