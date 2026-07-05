//go:build linux


package capsule

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Executor manages capsule lifecycle. One instance per Runtime.
// Runs INSIDE the Firecracker guest VM (needs kernel syscalls for
// namespaces, cgroups, overlayfs). The Ed25519 private key is held by
// HostAuthority on the host — Executor requests minting via vsock.
type Executor struct {
	mu                sync.RWMutex
	capsules          map[string]*Capsule        // capsuleID → Capsule
	capabilities      map[capKey]*Capability     // (agentRunID, handle) → Capability
	revokedCaps       map[string]bool            // per-capsule revoked CapabilityIDs (synced from HostAuthority)
	globalRevokedCaps map[string]bool            // wildcard revoked CapabilityIDs (apply to all capsules)
	hostClient        *HostClient                // vsock client to HostAuthority
	stateDir          string                     // /var/lib/capsules
	erofsMount        string                     // shared EROFS mount point
	brokerStore       string                     // content-addressed broker binary store
	vmMemoryTotal     int64                      // total VM RAM for admission control
	vmMemoryUsed      int64                      // committed memory across all capsules
}

// NewExecutor creates a new Executor with the given configuration.
func NewExecutor(stateDir, erofsMount, brokerStore string, vmMemoryTotal int64, hostClient *HostClient) *Executor {
	return &Executor{
		capsules:          make(map[string]*Capsule),
		capabilities:      make(map[capKey]*Capability),
		revokedCaps:       make(map[string]bool),
		globalRevokedCaps: make(map[string]bool),
		hostClient:        hostClient,
		stateDir:          stateDir,
		erofsMount:        erofsMount,
		brokerStore:       brokerStore,
		vmMemoryTotal:     vmMemoryTotal,
	}
}

// Spawn creates a new capsule with the given specification.
// This involves: namespace creation, cgroup setup, overlayfs mount,
// broker process spawn, and registration with HostAuthority.
func (e *Executor) Spawn(ctx context.Context, spec SpawnSpec) (*Capsule, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.capsules[spec.CapsuleID]; exists {
		return nil, fmt.Errorf("capsule %s already exists", spec.CapsuleID)
	}

	// Memory admission control.
	if e.vmMemoryUsed+spec.MemoryMax > e.vmMemoryTotal {
		return nil, fmt.Errorf("memory budget exceeded: used=%d, requested=%d, total=%d",
			e.vmMemoryUsed, spec.MemoryMax, e.vmMemoryTotal)
	}

	// Register capsule with HostAuthority (for mint auth).
	if err := e.hostClient.RegisterCapsule(ctx, spec.CapsuleID); err != nil {
		return nil, fmt.Errorf("failed to register capsule with HostAuthority: %w", err)
	}

	capsule := &Capsule{
		ID:          spec.CapsuleID,
		State:       StateSpawning,
		UpperDir:    fmt.Sprintf("%s/%s/upper", e.stateDir, spec.CapsuleID),
		WorkDir:     fmt.Sprintf("%s/%s/work", e.stateDir, spec.CapsuleID),
		MergedDir:   fmt.Sprintf("%s/%s/merged", e.stateDir, spec.CapsuleID),
		MemoryMax:   spec.MemoryMax,
		revokedCaps: make(map[string]bool),
	}

	// TODO: Create namespaces (gonso), cgroups, overlayfs mount, spawn broker.
	// These require kernel syscalls and will be implemented with:
	// - gonso for namespace creation (CLONE_NEWNS | CLONE_NEWPID | CLONE_NEWNET |
	//   CLONE_NEWUTS | CLONE_NEWIPC | CLONE_NEWUSER)
	// - containerd/cgroups/v3 for cgroup management
	// - unix.Mount for overlayfs
	// - os/exec for broker process spawn with seccomp + landlock + cap drop

	capsule.State = StateActive
	e.capsules[spec.CapsuleID] = capsule
	e.vmMemoryUsed += spec.MemoryMax

	return capsule, nil
}

// Destroy gracefully destroys a capsule.
func (e *Executor) Destroy(ctx context.Context, id string) error {
	e.mu.Lock()
	capsule, exists := e.capsules[id]
	e.mu.Unlock()

	if !exists {
		return fmt.Errorf("capsule %s not found", id)
	}

	if err := capsule.Destroy(ctx); err != nil {
		return fmt.Errorf("failed to destroy capsule %s: %w", id, err)
	}

	e.mu.Lock()
	delete(e.capsules, id)
	e.vmMemoryUsed -= capsule.MemoryMax
	e.mu.Unlock()

	// Unregister capsule with HostAuthority.
	if err := e.hostClient.UnregisterCapsule(ctx, id); err != nil {
		// Log but don't fail — capsule is already destroyed.
		fmt.Printf("warning: failed to unregister capsule %s with HostAuthority: %v\n", id, err)
	}

	return nil
}

// ForceDestroy forcefully destroys a capsule (SIGKILL broker, unmount, cleanup).
func (e *Executor) ForceDestroy(ctx context.Context, id string) error {
	// Same as Destroy but with SIGKILL instead of graceful shutdown.
	return e.Destroy(ctx, id)
}

// MintCapability requests a capability from HostAuthority via vsock.
func (e *Executor) MintCapability(agentRunID string, role AgentRole, capsuleID string, ttl time.Duration) (*Capability, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cap, err := e.hostClient.MintCapability(ctx, agentRunID, role, capsuleID, ttl)
	if err != nil {
		return nil, fmt.Errorf("failed to mint capability: %w", err)
	}

	e.mu.Lock()
	e.capabilities[capKey{AgentRunID: agentRunID, Handle: cap.Handle}] = cap
	e.mu.Unlock()

	return cap, nil
}

// ResolveCapability looks up a capability by agent run ID and handle.
func (e *Executor) ResolveCapability(agentRunID, handle string) (*Capability, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	cap, ok := e.capabilities[capKey{AgentRunID: agentRunID, Handle: handle}]
	if !ok {
		return nil, fmt.Errorf("no capability found for agent=%s handle=%s", agentRunID, handle)
	}
	return cap, nil
}

// RevokeCapability requests revocation from HostAuthority and removes
// the capability from the local map.
func (e *Executor) RevokeCapability(agentRunID, handle string) error {
	cap, err := e.ResolveCapability(agentRunID, handle)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := e.hostClient.RevokeCapability(ctx, agentRunID, cap.CapsuleID, cap.CapabilityID); err != nil {
		return fmt.Errorf("failed to revoke capability: %w", err)
	}

	e.mu.Lock()
	delete(e.capabilities, capKey{AgentRunID: agentRunID, Handle: handle})
	e.mu.Unlock()

	return nil
}

// ResolveTarget expands a capability's TargetCapsule to concrete capsule IDs.
// For researcher capabilities (TargetCapsule="*"), returns all active capsule IDs.
func (e *Executor) ResolveTarget(cap *Capability) ([]string, error) {
	if cap.TargetCapsule != "*" {
		return []string{cap.TargetCapsule}, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	var ids []string
	for id, c := range e.capsules {
		if c.State == StateActive {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// InspectCapsuleRaw performs a host-side diagnostic inspection of a capsule,
// bypassing the broker. Uses openat2-safe path resolution.
func (e *Executor) InspectCapsuleRaw(id string) (*CapsuleDiagnostics, error) {
	e.mu.RLock()
	capsule, exists := e.capsules[id]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("capsule %s not found", id)
	}

	capsule.mu.RLock()
	defer capsule.mu.RUnlock()

	return &CapsuleDiagnostics{
		ID:        capsule.ID,
		State:     capsule.State,
		PID:       capsule.PID,
		UpperDir:  capsule.UpperDir,
		MergedDir: capsule.MergedDir,
	}, nil
}

// ExtractDiff performs a host-side diff extraction, bypassing the broker.
// The host walks the upperdir and classifies changes.
func (e *Executor) ExtractDiff(id string) ([]FileChange, error) {
	e.mu.RLock()
	capsule, exists := e.capsules[id]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("capsule %s not found", id)
	}

	ctx := context.Background()
	return capsule.Diff(ctx)
}

// ListCapsules returns a summary of all capsules.
func (e *Executor) ListCapsules() []CapsuleSummary {
	e.mu.RLock()
	defer e.mu.RUnlock()

	summaries := make([]CapsuleSummary, 0, len(e.capsules))
	for id, c := range e.capsules {
		c.mu.RLock()
		summaries = append(summaries, CapsuleSummary{
			ID:         id,
			State:      c.State,
			PID:        c.PID,
			Pinned:     c.Pinned,
			OwnerRunID: "", // TODO: track owner run ID
		})
		c.mu.RUnlock()
	}
	return summaries
}

// RestartBroker restarts a capsule's broker process. Re-syncs the revoked
// capability set from HostAuthority before the broker accepts any RPC.
func (e *Executor) RestartBroker(id string) error {
	e.mu.RLock()
	capsule, exists := e.capsules[id]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("capsule %s not found", id)
	}

	// Re-sync revoked caps from HostAuthority.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	revokedIDs, err := e.hostClient.GetRevokedCaps(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get revoked caps from HostAuthority: %w", err)
	}

	capsule.UpdateRevokedCaps(revokedIDs)

	// TODO: Kill old broker process, spawn new one, inject public key,
	// connect to new broker's Unix socket.

	return nil
}

// SyncRevokedCaps updates the revoked capability set from HostAuthority
// and forwards to all capsule brokers.
func (e *Executor) SyncRevokedCaps(revokedIDs []string) {
	e.mu.Lock()
	for _, id := range revokedIDs {
		e.revokedCaps[id] = true
	}
	e.mu.Unlock()

	// Forward to all capsules.
	e.mu.RLock()
	defer e.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, capsule := range e.capsules {
		capsule.UpdateRevokedCaps(revokedIDs)
		if capsule.broker != nil {
			capsule.broker.SyncRevokedCaps(ctx, revokedIDs)
		}
	}
}
