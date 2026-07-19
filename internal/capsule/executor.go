//go:build linux

package capsule

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

const capsuleNamespaceHostID = 65534

// Executor is the guest-core authority for ephemeral capsule lifecycle and
// opaque, run-bound capabilities. All authority is process-local and is lost
// when the guest runtime restarts; no host daemon or vsock authority exists.
type Executor struct {
	mu                sync.RWMutex
	capsules          map[string]*Capsule
	capabilities      map[capKey]*Capability
	controlHandles    map[capKey]string
	revokedCaps       map[string]bool
	executionReceipts map[string]ExecutionReceipt
	stateDir          string
	lowerDir          string
	sourceDir         string
	brokerPath        string
	brokerDigest      [sha256.Size]byte
	publicKey         ed25519.PublicKey
	privateKey        ed25519.PrivateKey
	initErr           error
	vmMemoryTotal     int64
	vmMemoryUsed      int64
}

// NewExecutor constructs guest-local capsule authority. brokerPath must name a
// regular, immutable capsule-broker executable; its digest is pinned now and
// rechecked before every spawn.
func NewExecutor(stateDir, lowerDir, brokerPath string, vmMemoryTotal int64) *Executor {
	return NewExecutorWithSource(stateDir, lowerDir, "", brokerPath, vmMemoryTotal)
}

func NewExecutorWithSource(stateDir, lowerDir, sourceDir, brokerPath string, vmMemoryTotal int64) *Executor {
	e := &Executor{
		capsules:          make(map[string]*Capsule),
		capabilities:      make(map[capKey]*Capability),
		controlHandles:    make(map[capKey]string),
		revokedCaps:       make(map[string]bool),
		executionReceipts: make(map[string]ExecutionReceipt),
		stateDir:          filepath.Clean(stateDir),
		lowerDir:          filepath.Clean(lowerDir),
		sourceDir:         filepath.Clean(sourceDir),
		brokerPath:        filepath.Clean(brokerPath),
		vmMemoryTotal:     vmMemoryTotal,
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		e.initErr = fmt.Errorf("capsule: generate guest capability key: %w", err)
		return e
	}
	e.publicKey, e.privateKey = publicKey, privateKey
	e.brokerDigest, e.initErr = digestRegularFile(e.brokerPath)
	return e
}

// Spawn creates an isolated capsule with a private user/PID/mount/network/UTS/
// IPC/cgroup namespace, overlay root, cgroup-v2 budget, and broker process.
func (e *Executor) Spawn(ctx context.Context, spec SpawnSpec) (_ *Capsule, retErr error) {
	if err := validateSpawnSpec(spec); err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.initErr != nil {
		return nil, e.initErr
	}
	if _, exists := e.capsules[spec.CapsuleID]; exists {
		return nil, fmt.Errorf("capsule %s already exists", spec.CapsuleID)
	}
	if e.vmMemoryTotal <= 0 || e.vmMemoryUsed+spec.MemoryMax > e.vmMemoryTotal {
		return nil, fmt.Errorf("capsule memory budget exceeded: used=%d requested=%d total=%d", e.vmMemoryUsed, spec.MemoryMax, e.vmMemoryTotal)
	}
	if digest, err := digestRegularFile(e.brokerPath); err != nil || digest != e.brokerDigest {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("capsule broker digest changed after executor initialization")
	}
	if info, err := os.Stat(e.lowerDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("capsule lower root is unavailable: %s", e.lowerDir)
	}
	if info, err := os.Stat(e.sourceDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("capsule source root is unavailable: %s", e.sourceDir)
	}

	controlHandle, err := randomOpaque("c-")
	if err != nil {
		return nil, err
	}
	base := filepath.Join(e.stateDir, spec.CapsuleID)
	caps := &Capsule{
		ID:          spec.CapsuleID,
		State:       StateSpawning,
		UpperDir:    filepath.Join(base, "upper"),
		WorkDir:     filepath.Join(base, "work"),
		MergedDir:   filepath.Join(base, "root"),
		MemoryMax:   spec.MemoryMax,
		OwnerRunID:  spec.OwnerRunID,
		StartedAt:   time.Now().UTC(),
		Spec:        spec,
		revokedCaps: make(map[string]bool),
	}
	if _, err := os.Lstat(base); err == nil {
		return nil, fmt.Errorf("capsule state %s is quarantined or already exists", spec.CapsuleID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(base, 0o700); err != nil {
		return nil, fmt.Errorf("capsule create state: %w", err)
	}
	mounted := false
	defer func() {
		if retErr == nil {
			return
		}
		var cleanupErr error
		if caps.Process != nil {
			_ = caps.Process.Kill()
			_, _ = caps.Process.Wait()
		}
		if caps.listener != nil {
			cleanupErr = errors.Join(cleanupErr, caps.listener.Close())
			caps.listener = nil
		}
		if caps.Cgroup != nil {
			cleanupErr = errors.Join(cleanupErr, caps.Cgroup.Delete())
		}
		if mounted {
			cleanupErr = errors.Join(cleanupErr, unmountCapsuleRoot(caps.MergedDir))
		}
		if cleanupErr == nil {
			cleanupErr = os.RemoveAll(base)
		}
		if cleanupErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("capsule admission cleanup failed: %w", cleanupErr))
		}
	}()

	sourceLower := filepath.Join(base, "source-lower")
	sourceDigest, err := copyImmutableSourceTree(e.sourceDir, filepath.Join(sourceLower, "workspace", "platform"))
	if err != nil {
		return nil, fmt.Errorf("capsule pin source: %w", err)
	}
	caps.SourceSnapshotDigest = sourceDigest
	lowerLayers := sourceLower + ":" + e.lowerDir
	if err := MountOverlayFS(caps.MergedDir, caps.UpperDir, caps.WorkDir, lowerLayers); err != nil {
		return nil, err
	}
	mounted = true
	if err := prepareCapsuleRoot(caps.MergedDir); err != nil {
		return nil, err
	}
	if err := installBrokerMount(e.brokerPath, caps.MergedDir); err != nil {
		return nil, err
	}
	cgroup, err := CreateCgroup(spec.CapsuleID, spec)
	if err != nil {
		return nil, err
	}
	caps.Cgroup = cgroup
	if err := e.startBrokerLocked(ctx, caps); err != nil {
		return nil, err
	}

	caps.State = StateActive
	e.capsules[caps.ID] = caps
	e.controlHandles[capKey{AgentRunID: spec.OwnerRunID, Handle: controlHandle}] = caps.ID
	e.vmMemoryUsed += caps.MemoryMax
	return caps, nil
}

func (e *Executor) startBrokerLocked(ctx context.Context, caps *Capsule) error {
	hostSocket := filepath.Join(caps.MergedDir, "run", "capsule", "broker.sock")
	_ = os.Remove(hostSocket)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: hostSocket, Net: "unix"})
	if err != nil {
		return fmt.Errorf("capsule create parent broker listener: %w", err)
	}
	caps.listener = listener
	if err := os.Chmod(hostSocket, 0o600); err != nil {
		return fmt.Errorf("capsule secure parent broker listener: %w", err)
	}
	inheritedListener, err := listener.File()
	if err != nil {
		return fmt.Errorf("capsule duplicate broker listener: %w", err)
	}
	args := []string{"--socket", "/run/capsule/broker.sock", "--listener-fd", "3", "--capsule-id", caps.ID, "--pubkey", hex.EncodeToString(e.publicKey), "--merged", "/", "--authorized-peer-uid", fmt.Sprint(capsuleNamespaceHostID)}
	cmd := exec.Command("/run/capsule/broker", args...)
	cmd.ExtraFiles = []*os.File{inheritedListener}
	cmd.Env = []string{"PATH=/bin:/usr/bin:/run/current-system/sw/bin", "HOME=/root", "TMPDIR=/tmp"}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot:                     caps.MergedDir,
		Cloneflags:                 unix.CLONE_NEWUSER | unix.CLONE_NEWPID | unix.CLONE_NEWNS | unix.CLONE_NEWNET | unix.CLONE_NEWUTS | unix.CLONE_NEWIPC | unix.CLONE_NEWCGROUP,
		UidMappings:                []syscall.SysProcIDMap{{ContainerID: 0, HostID: capsuleNamespaceHostID, Size: 1}},
		GidMappings:                []syscall.SysProcIDMap{{ContainerID: 0, HostID: capsuleNamespaceHostID, Size: 1}},
		GidMappingsEnableSetgroups: false,
		Pdeathsig:                  syscall.SIGKILL,
	}
	if err := cmd.Start(); err != nil {
		_ = inheritedListener.Close()
		return fmt.Errorf("capsule start broker: %w", err)
	}
	_ = inheritedListener.Close()
	caps.Process = cmd.Process
	caps.wait = cmd.Wait
	caps.PID = cmd.Process.Pid
	if err := caps.Cgroup.AddPID(caps.PID); err != nil {
		return fmt.Errorf("capsule admit broker to cgroup: %w", err)
	}
	readinessCapability := &Capability{
		CapabilityID: "broker-readiness-" + caps.ID, Handle: "broker-readiness", CapsuleID: caps.ID,
		AgentRunID: "guest-core-readiness", AgentRole: RoleResearcher, TargetCapsule: caps.ID,
		Verbs: RoleVerbSets[RoleResearcher], ExpiresAt: time.Now().UTC().Add(time.Minute),
	}
	if err := SignCapability(readinessCapability, e.privateKey, "guest-ephemeral"); err != nil {
		return fmt.Errorf("capsule sign broker readiness capability: %w", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		client := NewBrokerClient(hostSocket, e.publicKey)
		if err := client.Connect(ctx); err == nil {
			if _, probeErr := client.Stat(ctx, readinessCapability, "."); probeErr == nil {
				caps.broker = client
				return nil
			}
			_ = client.Close()
		}
		if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
			return fmt.Errorf("capsule broker exited before readiness")
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("capsule broker readiness timed out")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}

// Destroy terminates the namespace leader, unmounts the overlay, deletes the
// cgroup, and invalidates every capability targeting the capsule.
func (e *Executor) Destroy(ctx context.Context, id string) error {
	return e.destroy(ctx, id, syscall.SIGTERM)
}

func (e *Executor) ForceDestroy(ctx context.Context, id string) error {
	return e.destroy(ctx, id, syscall.SIGKILL)
}

func (e *Executor) destroy(ctx context.Context, id string, signal syscall.Signal) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	caps, ok := e.capsules[id]
	if !ok {
		return fmt.Errorf("capsule %s not found", id)
	}
	caps.State = StateDestroying
	if caps.broker != nil {
		_ = caps.broker.Close()
	}
	if caps.listener != nil {
		_ = caps.listener.Close()
		caps.listener = nil
	}
	if caps.Process != nil {
		_ = caps.Process.Signal(signal)
		done := make(chan error, 1)
		go func() { done <- caps.wait() }()
		select {
		case <-done:
		case <-ctx.Done():
			_ = caps.Process.Kill()
			<-done
		}
	}
	var cleanupErr error
	if err := unmountCapsuleRoot(caps.MergedDir); err != nil {
		cleanupErr = errors.Join(cleanupErr, err)
	}
	if caps.Cgroup != nil {
		if err := caps.Cgroup.Delete(); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	if cleanupErr == nil {
		if err := os.RemoveAll(filepath.Join(e.stateDir, id)); err != nil {
			cleanupErr = err
		}
	}
	for key, capability := range e.capabilities {
		if capability.TargetCapsule == id {
			e.revokedCaps[capability.CapabilityID] = true
			delete(e.capabilities, key)
		}
	}
	if cleanupErr != nil {
		// Keep the destroying capsule and its owner control handle quarantined so
		// ForceDestroy can retry cleanup. Never release its admission budget.
		return cleanupErr
	}
	for key, capsuleID := range e.controlHandles {
		if capsuleID == id {
			delete(e.controlHandles, key)
		}
	}
	caps.State = StateDestroyed
	delete(e.capsules, id)
	e.vmMemoryUsed -= caps.MemoryMax
	return nil
}

// MintCapability creates a random opaque handle bound to one agent run. Raw
// signed capability material remains inside guest core and is never returned by
// the agent tool surface.
func (e *Executor) MintCapability(agentRunID string, role AgentRole, capsuleID string, ttl time.Duration) (*Capability, error) {
	if strings.TrimSpace(agentRunID) == "" || ttl <= 0 || ttl > 24*time.Hour {
		return nil, fmt.Errorf("capsule capability requires run identity and ttl in (0,24h]")
	}
	if role != RoleCoSuper && role != RoleResearcher {
		return nil, fmt.Errorf("capsule capability role %q is not grantable", role)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if role == RoleCoSuper {
		if caps, ok := e.capsules[capsuleID]; !ok || caps.State != StateActive {
			return nil, fmt.Errorf("capsule %s is not active", capsuleID)
		}
	} else if capsuleID != "*" {
		return nil, fmt.Errorf("researcher capability must target wildcard")
	}
	capabilityID, err := randomOpaque("cap-")
	if err != nil {
		return nil, err
	}
	handle, err := randomOpaque("h-")
	if err != nil {
		return nil, err
	}
	capability := &Capability{
		CapabilityID:  capabilityID,
		Handle:        handle,
		CapsuleID:     capsuleID,
		AgentRunID:    agentRunID,
		AgentRole:     role,
		TargetCapsule: capsuleID,
		Verbs:         cloneVerbSet(RoleVerbSets[role]),
		ExpiresAt:     time.Now().UTC().Add(ttl),
	}
	if err := SignCapability(capability, e.privateKey, "guest-ephemeral"); err != nil {
		return nil, err
	}
	e.capabilities[capKey{AgentRunID: agentRunID, Handle: handle}] = capability
	copy := *capability
	return &copy, nil
}

func (e *Executor) ResolveCapability(agentRunID, handle string) (*Capability, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	capability, ok := e.capabilities[capKey{AgentRunID: agentRunID, Handle: handle}]
	if !ok || e.revokedCaps[capability.CapabilityID] || time.Now().After(capability.ExpiresAt) {
		return nil, fmt.Errorf("capsule capability unavailable")
	}
	copy := *capability
	return &copy, nil
}

func (e *Executor) RevokeCapability(agentRunID, handle string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := capKey{AgentRunID: agentRunID, Handle: handle}
	capability, ok := e.capabilities[key]
	if !ok {
		return fmt.Errorf("capsule capability unavailable")
	}
	e.revokedCaps[capability.CapabilityID] = true
	delete(e.capabilities, key)
	if caps := e.capsules[capability.TargetCapsule]; caps != nil {
		caps.revokedCaps[capability.CapabilityID] = true
	}
	return nil
}

func (e *Executor) ResolveTarget(capability *Capability) ([]string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if capability.TargetCapsule != "*" {
		if _, ok := e.capsules[capability.TargetCapsule]; !ok {
			return nil, fmt.Errorf("capsule target unavailable")
		}
		return []string{capability.TargetCapsule}, nil
	}
	ids := make([]string, 0, len(e.capsules))
	for id, caps := range e.capsules {
		if caps.State == StateActive {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func (e *Executor) Exec(ctx context.Context, agentRunID, handle string, request ExecRequest) (ExecResult, error) {
	capability, caps, err := e.resolveOne(agentRunID, handle, "exec")
	if err != nil {
		return ExecResult{}, err
	}
	result, err := caps.Exec(ctx, capability, request)
	if err != nil {
		return ExecResult{}, err
	}
	if len(computerevent.DetectPrivateSecrets([]byte(request.Command))) != 0 {
		return ExecResult{}, fmt.Errorf("capsule: secret-bearing command cannot produce auditable execution evidence")
	}
	worktreeDigest, err := digestCapsuleWorktree(ctx, caps)
	if err != nil {
		return ExecResult{}, err
	}
	receipt := ExecutionReceipt{
		CapsuleID: caps.ID, Command: request.Command, Cwd: request.Cwd, ExitCode: result.ExitCode,
		StdoutDigest: computerevent.DigestBytes([]byte(result.Stdout)), StderrDigest: computerevent.DigestBytes([]byte(result.Stderr)),
		WorktreeDigest: worktreeDigest, SourceTreeDigest: caps.SourceSnapshotDigest, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	canonical, err := computerevent.CanonicalJSON(receipt)
	if err != nil {
		return ExecResult{}, err
	}
	receipt.ReceiptRef = "capsule-exec:sha256:" + computerevent.DigestBytes(canonical)
	e.mu.Lock()
	e.executionReceipts[receipt.ReceiptRef] = receipt
	e.mu.Unlock()
	result.ReceiptRef = receipt.ReceiptRef
	return result, nil
}

func digestCapsuleWorktree(ctx context.Context, caps *Capsule) (string, error) {
	changes, err := caps.Diff(ctx)
	if err != nil {
		return "", err
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	type entry struct {
		Path          string `json:"path"`
		Kind          string `json:"kind"`
		Mode          uint32 `json:"mode"`
		ContentDigest string `json:"content_digest,omitempty"`
	}
	entries := make([]entry, 0, len(changes))
	for _, change := range changes {
		item := entry{Path: change.Path, Kind: change.Kind.String(), Mode: uint32(change.Mode.Perm())}
		if change.Kind != ChangeDeleted {
			path := filepath.Join(caps.MergedDir, filepath.FromSlash(strings.TrimPrefix(change.Path, "/")))
			info, statErr := os.Lstat(path)
			if statErr != nil {
				return "", statErr
			}
			switch {
			case info.Mode().IsRegular():
				input, openErr := os.Open(path)
				if openErr != nil {
					return "", openErr
				}
				hash := sha256.New()
				_, copyErr := io.Copy(hash, input)
				closeErr := input.Close()
				if copyErr != nil || closeErr != nil {
					return "", errors.Join(copyErr, closeErr)
				}
				item.ContentDigest = hex.EncodeToString(hash.Sum(nil))
			case info.Mode()&os.ModeSymlink != 0:
				target, linkErr := os.Readlink(path)
				if linkErr != nil {
					return "", linkErr
				}
				item.ContentDigest = computerevent.DigestBytes([]byte(target))
			}
		}
		entries = append(entries, item)
	}
	canonical, err := computerevent.CanonicalJSON(entries)
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(canonical), nil
}

func (e *Executor) ResolveGrantedExecutionReceipts(ctx context.Context, agentRunID, handle string, refs []string) ([]ExecutionReceipt, error) {
	capability, caps, err := e.resolveOne(agentRunID, handle, "exec")
	if err != nil || capability.AgentRole != RoleCoSuper {
		return nil, fmt.Errorf("capsule execution evidence unavailable")
	}
	worktreeDigest, err := digestCapsuleWorktree(ctx, caps)
	if err != nil {
		return nil, err
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	receipts := make([]ExecutionReceipt, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if _, duplicate := seen[ref]; duplicate {
			continue
		}
		receipt, found := e.executionReceipts[ref]
		if !found || receipt.CapsuleID != caps.ID || receipt.ExitCode != 0 || receipt.WorktreeDigest != worktreeDigest ||
			receipt.SourceTreeDigest != caps.SourceSnapshotDigest {
			return nil, fmt.Errorf("capsule execution evidence does not bind the final successful worktree")
		}
		seen[ref] = struct{}{}
		receipts = append(receipts, receipt)
	}
	if len(receipts) == 0 {
		return nil, fmt.Errorf("capsule execution evidence is required")
	}
	return receipts, nil
}

func (e *Executor) ResolveExecutionReceipts(refs []string) ([]ExecutionReceipt, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	receipts := make([]ExecutionReceipt, 0, len(refs))
	for _, ref := range refs {
		receipt, found := e.executionReceipts[ref]
		if !found {
			return nil, fmt.Errorf("capsule execution receipt unavailable")
		}
		receipts = append(receipts, receipt)
	}
	return receipts, nil
}

func (e *Executor) ReadFile(ctx context.Context, agentRunID, handle, path string) ([]byte, error) {
	capability, caps, err := e.resolveOne(agentRunID, handle, "read_file")
	if err != nil {
		return nil, err
	}
	return caps.broker.ReadFile(ctx, capability, path)
}

func (e *Executor) WriteFile(ctx context.Context, agentRunID, handle, path string, content []byte, mode uint32) error {
	capability, caps, err := e.resolveOne(agentRunID, handle, "write_file")
	if err != nil {
		return err
	}
	return caps.broker.WriteFile(ctx, capability, path, content, mode)
}

func (e *Executor) ListDir(ctx context.Context, agentRunID, handle, path string) ([]string, error) {
	capability, caps, err := e.resolveOne(agentRunID, handle, "list_dir")
	if err != nil {
		return nil, err
	}
	return caps.broker.ListDir(ctx, capability, path)
}

func (e *Executor) resolveOne(agentRunID, handle, verb string) (*Capability, *Capsule, error) {
	capability, err := e.ResolveCapability(agentRunID, handle)
	if err != nil || !capability.AgentRole.HasVerb(verb) || capability.TargetCapsule == "*" {
		return nil, nil, fmt.Errorf("capsule operation refused")
	}
	e.mu.RLock()
	caps := e.capsules[capability.TargetCapsule]
	e.mu.RUnlock()
	if caps == nil {
		return nil, nil, fmt.Errorf("capsule operation refused")
	}
	return capability, caps, nil
}

func (e *Executor) ControlHandle(agentRunID, capsuleID string) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for key, id := range e.controlHandles {
		if key.AgentRunID == agentRunID && id == capsuleID {
			return key.Handle, nil
		}
	}
	return "", fmt.Errorf("capsule control handle unavailable")
}

func (e *Executor) GrantCoSuper(superRunID, controlHandle, coSuperRunID string, ttl time.Duration) (string, error) {
	capsuleID, err := e.resolveControl(superRunID, controlHandle)
	if err != nil {
		return "", err
	}
	capability, err := e.MintCapability(coSuperRunID, RoleCoSuper, capsuleID, ttl)
	if err != nil {
		return "", err
	}
	return capability.Handle, nil
}

func (e *Executor) DestroyOwned(ctx context.Context, agentRunID, handle string, force bool) error {
	capsuleID, err := e.resolveControl(agentRunID, handle)
	if err != nil {
		return err
	}
	if force {
		return e.ForceDestroy(ctx, capsuleID)
	}
	return e.Destroy(ctx, capsuleID)
}

func (e *Executor) InspectOwned(agentRunID, handle string) (CapsuleControlSummary, error) {
	capsuleID, err := e.resolveControl(agentRunID, handle)
	if err != nil {
		return CapsuleControlSummary{}, err
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	caps := e.capsules[capsuleID]
	if caps == nil {
		return CapsuleControlSummary{}, fmt.Errorf("capsule control handle unavailable")
	}
	return CapsuleControlSummary{Handle: handle, State: caps.State, MemoryMax: caps.MemoryMax, Uptime: time.Since(caps.StartedAt), SourceSnapshotDigest: caps.SourceSnapshotDigest}, nil
}

func (e *Executor) ExtractOwned(agentRunID, handle string) ([]FileChange, error) {
	capsuleID, err := e.resolveControl(agentRunID, handle)
	if err != nil {
		return nil, err
	}
	return e.ExtractDiff(capsuleID)
}

func (e *Executor) ExtractGranted(agentRunID, handle string) ([]FileChange, error) {
	capability, err := e.ResolveCapability(agentRunID, handle)
	if err != nil || capability.AgentRole != RoleCoSuper {
		return nil, fmt.Errorf("capsule granted diff unavailable")
	}
	return e.ExtractDiff(capability.TargetCapsule)
}

func (e *Executor) ResolveGrantedCapsuleID(agentRunID, handle string) (string, error) {
	capability, err := e.ResolveCapability(agentRunID, handle)
	if err != nil || capability.AgentRole != RoleCoSuper {
		return "", fmt.Errorf("capsule granted identity unavailable")
	}
	return capability.TargetCapsule, nil
}
func (e *Executor) ResolveGrantedSourceSnapshotDigest(agentRunID, handle string) (string, error) {
	capability, err := e.ResolveCapability(agentRunID, handle)
	if err != nil || capability.AgentRole != RoleCoSuper {
		return "", fmt.Errorf("capsule granted source snapshot unavailable")
	}
	e.mu.RLock()
	capsule := e.capsules[capability.TargetCapsule]
	e.mu.RUnlock()
	if capsule == nil || !computerevent.IsSHA256(capsule.SourceSnapshotDigest) {
		return "", fmt.Errorf("capsule granted source snapshot unavailable")
	}
	return capsule.SourceSnapshotDigest, nil
}

func (e *Executor) ResolveGrantedFreezeBindings(agentRunID, handle string) (string, string, error) {
	capability, err := e.ResolveCapability(agentRunID, handle)
	if err != nil || capability.AgentRole != RoleCoSuper {
		return "", "", fmt.Errorf("capsule freeze bindings unavailable")
	}
	e.mu.RLock()
	capsule := e.capsules[capability.TargetCapsule]
	e.mu.RUnlock()
	if capsule == nil {
		return "", "", fmt.Errorf("capsule freeze bindings unavailable")
	}
	capabilityBytes, err := computerevent.CanonicalJSON(capability)
	if err != nil {
		return "", "", err
	}
	resourceBytes, err := computerevent.CanonicalJSON(capsule.Spec)
	if err != nil {
		return "", "", err
	}
	return computerevent.DigestBytes(capabilityBytes), "resource:sha256:" + computerevent.DigestBytes(resourceBytes), nil
}

func (e *Executor) StageGrantedRelease(agentRunID, handle, incomingRoot string) ([]FrozenReleaseFile, string, error) {
	capability, err := e.ResolveCapability(agentRunID, handle)
	if err != nil || capability.AgentRole != RoleCoSuper {
		return nil, "", fmt.Errorf("capsule release staging unavailable")
	}
	e.mu.RLock()
	caps := e.capsules[capability.TargetCapsule]
	e.mu.RUnlock()
	if caps == nil {
		return nil, "", fmt.Errorf("capsule release staging unavailable")
	}
	incomingRoot = filepath.Clean(incomingRoot)
	if !filepath.IsAbs(incomingRoot) {
		return nil, "", fmt.Errorf("capsule release incoming root must be absolute")
	}
	if err := os.MkdirAll(incomingRoot, 0o700); err != nil {
		return nil, "", err
	}
	if info, err := os.Stat(incomingRoot); err != nil || info.Mode().Perm()&0o077 != 0 {
		return nil, "", fmt.Errorf("capsule release incoming root must be private")
	}
	changes, err := caps.Diff(context.Background())
	if err != nil {
		return nil, "", err
	}
	const releasePrefix = "var/lib/artifact/release/"
	temporary, err := os.MkdirTemp(incomingRoot, ".freeze-")
	if err != nil {
		return nil, "", err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(temporary)
		}
	}()
	var files []FrozenReleaseFile
	var total int64
	for _, change := range changes {
		if !strings.HasPrefix(change.Path, releasePrefix) {
			continue
		}
		relative := strings.TrimPrefix(change.Path, releasePrefix)
		clean := filepath.Clean(filepath.FromSlash(relative))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || change.Kind == ChangeDeleted {
			return nil, "", fmt.Errorf("capsule release contains unsafe path %q", change.Path)
		}
		source := filepath.Join(caps.MergedDir, filepath.FromSlash(strings.TrimPrefix(change.Path, "/")))
		info, err := os.Lstat(source)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, "", fmt.Errorf("capsule release file %q is not regular", change.Path)
		}
		total += info.Size()
		if total > caps.MemoryMax {
			return nil, "", fmt.Errorf("capsule release exceeds resource budget")
		}
		target := filepath.Join(temporary, clean)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return nil, "", err
		}
		base := strings.ToLower(filepath.Base(clean))
		extension := strings.ToLower(filepath.Ext(base))
		if base == ".env" || strings.HasPrefix(base, ".env.") || base == ".npmrc" || base == ".netrc" ||
			base == "credentials.json" || base == "auth.json" || base == "id_rsa" || base == "id_ed25519" ||
			extension == ".pem" || extension == ".key" || extension == ".p12" || extension == ".pfx" {
			return nil, "", fmt.Errorf("capsule release refuses secret-bearing path %q", change.Path)
		}
		input, err := os.Open(source)
		if err != nil {
			return nil, "", err
		}
		scanner := bufio.NewScanner(input)
		scanner.Buffer(make([]byte, 64<<10), 1<<20)
		for scanner.Scan() {
			if findings := computerevent.DetectPrivateSecrets(scanner.Bytes()); len(findings) != 0 {
				_ = input.Close()
				return nil, "", fmt.Errorf("capsule release refuses secret content in %q", change.Path)
			}
		}
		if scanErr := scanner.Err(); scanErr != nil {
			_ = input.Close()
			return nil, "", fmt.Errorf("capsule release secret scan failed for %q: %w", change.Path, scanErr)
		}
		if _, err := input.Seek(0, io.SeekStart); err != nil {
			_ = input.Close()
			return nil, "", err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			_ = input.Close()
			return nil, "", err
		}
		hash := sha256.New()
		_, copyErr := io.Copy(io.MultiWriter(output, hash), input)
		closeErr := errors.Join(input.Close(), output.Sync(), output.Close())
		if copyErr != nil || closeErr != nil {
			return nil, "", errors.Join(copyErr, closeErr)
		}
		mode := uint32(info.Mode().Perm() & 0o555)
		if mode == 0 {
			mode = 0o444
		}
		if err := os.Chmod(target, os.FileMode(mode)); err != nil {
			return nil, "", err
		}
		files = append(files, FrozenReleaseFile{Path: filepath.ToSlash(clean), SHA256: hex.EncodeToString(hash.Sum(nil)), Mode: mode})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	if len(files) == 0 || files[0].Path == "" {
		return nil, "", fmt.Errorf("capsule release contains no frozen runtime artifacts")
	}
	hasSandbox := false
	for _, file := range files {
		if file.Path == "bin/sandbox" && file.Mode&0o111 != 0 {
			hasSandbox = true
		}
	}
	if !hasSandbox {
		return nil, "", fmt.Errorf("capsule release must contain executable bin/sandbox")
	}
	cleanup = false
	return files, temporary, nil
}

// ResolveOwnedCapsuleID is a trusted-core bridge for semantic event binding.
// Callers must never expose the returned raw identity to agent arguments or
// results.
func (e *Executor) ResolveOwnedCapsuleID(agentRunID, handle string) (string, error) {
	return e.resolveControl(agentRunID, handle)
}

func (e *Executor) ListOwned(agentRunID string) []CapsuleControlSummary {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]CapsuleControlSummary, 0)
	for key, capsuleID := range e.controlHandles {
		if key.AgentRunID != agentRunID {
			continue
		}
		if caps := e.capsules[capsuleID]; caps != nil {
			out = append(out, CapsuleControlSummary{Handle: key.Handle, State: caps.State, MemoryMax: caps.MemoryMax, Uptime: time.Since(caps.StartedAt)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Handle < out[j].Handle })
	return out
}

func (e *Executor) resolveControl(agentRunID, handle string) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	capsuleID, ok := e.controlHandles[capKey{AgentRunID: agentRunID, Handle: handle}]
	if !ok || e.capsules[capsuleID] == nil {
		return "", fmt.Errorf("capsule control handle unavailable")
	}
	return capsuleID, nil
}

func (e *Executor) InspectCapsuleRaw(id string) (*CapsuleDiagnostics, error) {
	e.mu.RLock()
	caps := e.capsules[id]
	e.mu.RUnlock()
	if caps == nil {
		return nil, fmt.Errorf("capsule %s not found", id)
	}
	return &CapsuleDiagnostics{ID: caps.ID, State: caps.State, PID: caps.PID, UpperDir: caps.UpperDir, MergedDir: caps.MergedDir, MemoryMax: caps.MemoryMax, Uptime: time.Since(caps.StartedAt)}, nil
}

func (e *Executor) ExtractDiff(id string) ([]FileChange, error) {
	e.mu.RLock()
	caps := e.capsules[id]
	e.mu.RUnlock()
	if caps == nil {
		return nil, fmt.Errorf("capsule %s not found", id)
	}
	return caps.Diff(context.Background())
}

func (e *Executor) ListCapsules() []CapsuleSummary {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]CapsuleSummary, 0, len(e.capsules))
	for _, caps := range e.capsules {
		out = append(out, CapsuleSummary{ID: caps.ID, State: caps.State, PID: caps.PID, MemoryMax: caps.MemoryMax, Pinned: caps.Pinned, OwnerRunID: caps.OwnerRunID})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func validateSpawnSpec(spec SpawnSpec) error {
	if strings.TrimSpace(spec.CapsuleID) == "" || filepath.Base(spec.CapsuleID) != spec.CapsuleID || spec.OwnerRunID == "" {
		return fmt.Errorf("capsule spawn requires safe capsule and owner-run identities")
	}
	if spec.MemoryMax <= 0 || spec.CpuQuota <= 0 || spec.CpuPeriod <= 0 || spec.PidsMax <= 0 {
		return fmt.Errorf("capsule spawn requires positive memory, cpu, period, and pid limits")
	}
	return nil
}

func installBrokerMount(source, root string) error {
	dir := filepath.Join(root, "run", "capsule")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("capsule create broker directory: %w", err)
	}
	target := filepath.Join(dir, "broker")
	file, err := os.OpenFile(target, os.O_CREATE|os.O_RDONLY, 0o500)
	if err != nil {
		return fmt.Errorf("capsule create broker mountpoint: %w", err)
	}
	_ = file.Close()
	if err := unix.Mount(source, target, "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("capsule bind broker: %w", err)
	}
	if err := unix.Mount("", target, "", unix.MS_BIND|unix.MS_REMOUNT|unix.MS_RDONLY|unix.MS_NOSUID|unix.MS_NODEV, ""); err != nil {
		return fmt.Errorf("capsule harden broker mount: %w", err)
	}
	return nil
}

func prepareCapsuleRoot(root string) error {
	for _, path := range []string{"run", "tmp", "home", "root", "etc", "mnt", "var", "dev", "proc", "sys", "nix/store"} {
		if err := os.MkdirAll(filepath.Join(root, path), 0o755); err != nil {
			return fmt.Errorf("capsule prepare root: %w", err)
		}
	}
	for _, path := range []string{"run", "tmp", "mnt"} {
		target := filepath.Join(root, path)
		if err := unix.Mount("tmpfs", target, "tmpfs", unix.MS_NOSUID|unix.MS_NODEV, "mode=0755,size=64m"); err != nil {
			return fmt.Errorf("capsule mask %s: %w", path, err)
		}
	}
	storeTarget := filepath.Join(root, "nix", "store")
	if err := unix.Mount("/nix/store", storeTarget, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
		return fmt.Errorf("capsule bind immutable store: %w", err)
	}
	if err := unix.Mount("", storeTarget, "", unix.MS_BIND|unix.MS_REMOUNT|unix.MS_RDONLY|unix.MS_NOSUID|unix.MS_NODEV, ""); err != nil {
		return fmt.Errorf("capsule harden immutable store: %w", err)
	}
	for _, device := range []string{"null", "zero", "random", "urandom"} {
		target := filepath.Join(root, "dev", device)
		file, err := os.OpenFile(target, os.O_CREATE|os.O_RDONLY, 0o666)
		if err != nil {
			return fmt.Errorf("capsule create device target: %w", err)
		}
		_ = file.Close()
		if err := unix.Mount(filepath.Join("/dev", device), target, "", unix.MS_BIND, ""); err != nil {
			return fmt.Errorf("capsule bind device %s: %w", device, err)
		}
	}
	etc := filepath.Join(root, "etc")
	for name, content := range map[string]string{
		"passwd":        "root:x:0:0:Capsule:/root:/bin/sh\n",
		"group":         "root:x:0:\n",
		"hosts":         "127.0.0.1 localhost\n::1 localhost\n",
		"nsswitch.conf": "hosts: files\n",
	} {
		if err := os.WriteFile(filepath.Join(etc, name), []byte(content), 0o644); err != nil {
			return fmt.Errorf("capsule write %s: %w", name, err)
		}
	}
	return nil
}

func unmountCapsuleRoot(root string) error {
	brokerMount := filepath.Join(root, "run", "capsule", "broker")
	_ = unix.Unmount(brokerMount, unix.MNT_DETACH)
	if err := unix.Unmount(root, unix.MNT_DETACH); err != nil && err != unix.EINVAL {
		return fmt.Errorf("capsule unmount root: %w", err)
	}
	return nil
}

func digestRegularFile(path string) ([sha256.Size]byte, error) {
	var zero [sha256.Size]byte
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o022 != 0 {
		return zero, fmt.Errorf("capsule broker must be an existing non-group/world-writable regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return zero, err
	}
	return sha256.Sum256(data), nil
}

func randomOpaque(prefix string) (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("capsule random handle: %w", err)
	}
	return prefix + hex.EncodeToString(value[:]), nil
}

func cloneVerbSet(input VerbSet) VerbSet {
	out := make(VerbSet, len(input))
	for verb, allowed := range input {
		out[verb] = allowed
	}
	return out
}
