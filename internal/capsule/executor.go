//go:build linux

package capsule

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	grantedReceipts   map[string]GrantedExecutionReceipt
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
		grantedReceipts:   make(map[string]GrantedExecutionReceipt),
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

// InitializationError reports fail-closed construction validation before the
// executor is wired into a production role registry.
func (e *Executor) InitializationError() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.initErr
}

const subjectArtifactPrefix = "capsule-subject:sha256:"

func (e *Executor) subjectArtifactPath(ref string) (string, string, error) {
	if !strings.HasPrefix(ref, subjectArtifactPrefix) {
		return "", "", fmt.Errorf("capsule subject artifact ref is invalid")
	}
	digest := strings.TrimPrefix(ref, subjectArtifactPrefix)
	if len(digest) != sha256.Size*2 {
		return "", "", fmt.Errorf("capsule subject artifact digest is invalid")
	}
	if _, err := hex.DecodeString(digest); err != nil || strings.ToLower(digest) != digest {
		return "", "", fmt.Errorf("capsule subject artifact digest is invalid")
	}
	root := filepath.Join(e.stateDir, "subjects", digest, "workspace", "platform")
	return root, digest, nil
}

// PreflightSourceSnapshot persists an immutable content-addressed complete-tree
// source before assignment Open. A candidate ref selects that exact prior
// artifact; empty selects the current clean committed source tree.
func (e *Executor) PreflightSourceSnapshot(ctx context.Context, candidateRef string) (SourcePreflight, error) {
	if strings.TrimSpace(candidateRef) != "" {
		root, digest, err := e.subjectArtifactPath(strings.TrimSpace(candidateRef))
		if err != nil {
			return SourcePreflight{}, err
		}
		actual, err := digestCanonicalSubjectTree(ctx, root)
		if err != nil || actual != digest {
			return SourcePreflight{}, fmt.Errorf("capsule candidate artifact is unavailable or corrupt")
		}
		return SourcePreflight{SubjectDigest: digest, ArtifactRef: strings.TrimSpace(candidateRef)}, nil
	}
	commit, err := immutableGitCommitIdentity(ctx, e.sourceDir)
	if err != nil {
		return SourcePreflight{}, err
	}
	digest, err := canonicalImmutableCommitDigest(ctx, e.sourceDir, commit)
	if err != nil {
		return SourcePreflight{}, err
	}
	return SourcePreflight{SubjectDigest: digest, ArtifactRef: "capsule-source-git:" + commit + ":sha256:" + digest}, nil
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
			if caps.wait != nil {
				_ = caps.wait()
			}
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
			cleanupErr = removePrivateTree(base)
		}
		if cleanupErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("capsule admission cleanup failed: %w", cleanupErr))
		}
	}()

	source := SourcePreflight{SubjectDigest: strings.TrimSpace(spec.ExpectedSubjectDigest), ArtifactRef: strings.TrimSpace(spec.SourceArtifactRef)}
	if source.SubjectDigest == "" && source.ArtifactRef == "" {
		source, err = e.PreflightSourceSnapshot(ctx, "")
		if err != nil {
			return nil, fmt.Errorf("capsule preflight source: %w", err)
		}
	}
	sourceLower := filepath.Join(base, "source-lower")
	sourceTarget := filepath.Join(sourceLower, "workspace", "platform")
	var sourceDigest string
	if strings.HasPrefix(source.ArtifactRef, subjectArtifactPrefix) {
		artifactRoot, artifactDigest, resolveErr := e.subjectArtifactPath(source.ArtifactRef)
		if resolveErr != nil || source.SubjectDigest == "" || artifactDigest != source.SubjectDigest {
			return nil, fmt.Errorf("capsule spawn requires exact candidate source artifact and subject digest")
		}
		if err := copyCanonicalSubjectTree(ctx, artifactRoot, sourceTarget); err != nil {
			return nil, fmt.Errorf("capsule pin candidate source: %w", err)
		}
		sourceDigest, err = digestCanonicalSubjectTree(ctx, sourceTarget)
	} else {
		const prefix = "capsule-source-git:"
		raw := strings.TrimPrefix(source.ArtifactRef, prefix)
		separator := strings.Index(raw, ":sha256:")
		if !strings.HasPrefix(source.ArtifactRef, prefix) || separator <= 0 || raw[separator+len(":sha256:"):] != source.SubjectDigest {
			return nil, fmt.Errorf("capsule spawn requires exact preflight git source and subject digest")
		}
		commit := raw[:separator]
		decodedCommit, decodeErr := hex.DecodeString(commit)
		if decodeErr != nil || (len(decodedCommit) != 20 && len(decodedCommit) != 32) {
			return nil, fmt.Errorf("capsule spawn preflight git commit is invalid")
		}
		sourceDigest, err = copyImmutableCommitTree(ctx, e.sourceDir, commit, sourceTarget)
	}
	if err != nil || sourceDigest != source.SubjectDigest {
		return nil, fmt.Errorf("capsule source changed after durable preflight")
	}
	if err := requireFrozenComputerSurfaceSource(sourceTarget); err != nil {
		return nil, err
	}
	if err := makeSubjectTreeReadOnly(filepath.Join(sourceLower, "workspace", "platform")); err != nil {
		return nil, err
	}
	caps.SourceSnapshotDigest = sourceDigest
	lowerLayers := sourceLower + ":" + e.lowerDir
	if err := MountOverlayFS(caps.MergedDir, caps.UpperDir, caps.WorkDir, lowerLayers); err != nil {
		return nil, err
	}
	mounted = true
	if err := prepareCapsuleRoot(caps.MergedDir, caps.UpperDir); err != nil {
		return nil, err
	}
	if err := installBrokerMount(ctx, e.brokerPath, caps.MergedDir); err != nil {
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
	cgroupFile, err := caps.Cgroup.Open()
	if err != nil {
		_ = inheritedListener.Close()
		return err
	}
	args := []string{"--socket", "/run/capsule/broker.sock", "--listener-fd", "3", "--isolation-stage", "launcher", "--capsule-id", caps.ID, "--pubkey", hex.EncodeToString(e.publicKey), "--merged", "/", "--authorized-peer-uid", fmt.Sprint(capsuleNamespaceHostID)}
	cmd := exec.Command("/run/capsule/broker", args...)
	cmd.ExtraFiles = []*os.File{inheritedListener}
	brokerPath := os.Getenv("PATH")
	if strings.TrimSpace(brokerPath) == "" {
		brokerPath = "/run/current-system/sw/bin:/bin:/usr/bin"
	} else if !strings.Contains(brokerPath, "/run/current-system/sw/bin") {
		brokerPath = "/run/current-system/sw/bin:" + brokerPath
	}
	cmd.Env = []string{"PATH=" + brokerPath + ":/bin:/usr/bin", "HOME=/root", "TMPDIR=/tmp"}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot:                     caps.MergedDir,
		Cloneflags:                 unix.CLONE_NEWUSER,
		Unshareflags:               unix.CLONE_NEWNS | unix.CLONE_NEWNET | unix.CLONE_NEWUTS | unix.CLONE_NEWIPC | unix.CLONE_NEWCGROUP,
		UidMappings:                []syscall.SysProcIDMap{{ContainerID: 0, HostID: capsuleNamespaceHostID, Size: 1}},
		GidMappings:                []syscall.SysProcIDMap{{ContainerID: 0, HostID: capsuleNamespaceHostID, Size: 1}},
		AmbientCaps:                []uintptr{unix.CAP_SYS_ADMIN, unix.CAP_SETPCAP, unix.CAP_DAC_OVERRIDE, unix.CAP_FOWNER, unix.CAP_CHOWN, unix.CAP_SETUID, unix.CAP_SETGID},
		GidMappingsEnableSetgroups: false,
		UseCgroupFD:                true,
		CgroupFD:                   int(cgroupFile.Fd()),
		Pdeathsig:                  syscall.SIGKILL,
	}
	if err := waitForCommandStart(ctx, capsuleBrokerStartTimeout, cmd.Start); err != nil {
		_ = inheritedListener.Close()
		_ = cgroupFile.Close()
		return fmt.Errorf("capsule start broker launcher: %w", err)
	}
	_ = inheritedListener.Close()
	_ = cgroupFile.Close()
	caps.Process = cmd.Process
	caps.PID = cmd.Process.Pid
	caps.processDone = make(chan struct{})
	caps.wait = func() error {
		<-caps.processDone
		return caps.processErr
	}
	go func() {
		caps.processErr = cmd.Wait()
		close(caps.processDone)
	}()
	readinessCapability := &Capability{
		CapabilityID: "broker-readiness-" + caps.ID, Handle: "broker-readiness", CapsuleID: caps.ID,
		AgentRunID: "guest-core-readiness", AgentRole: RoleResearcher, TargetCapsule: caps.ID,
		Verbs: RoleVerbSets[RoleResearcher], ExpiresAt: time.Now().UTC().Add(time.Minute),
	}
	if err := SignCapability(readinessCapability, e.privateKey, "guest-ephemeral"); err != nil {
		return fmt.Errorf("capsule sign broker readiness capability: %w", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for {
		client := NewBrokerClient(hostSocket, e.publicKey)
		if err := client.Connect(ctx); err != nil {
			lastErr = err
		} else if _, probeErr := client.Stat(ctx, readinessCapability, "."); probeErr != nil {
			lastErr = probeErr
			_ = client.Close()
		} else {
			caps.broker = client
			return nil
		}
		if time.Now().After(deadline) {
			return brokerReadinessTimeoutError(lastErr)
		}
		select {
		case <-caps.processDone:
			return fmt.Errorf("capsule broker exited before readiness: %v", caps.processErr)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func brokerReadinessTimeoutError(lastErr error) error {
	if lastErr == nil {
		return fmt.Errorf("capsule broker readiness timed out")
	}
	return fmt.Errorf("capsule broker readiness timed out: %w", lastErr)
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
	var cleanupErr error
	if caps.Process != nil {
		_ = caps.Process.Signal(signal)
		kill := func() error {
			if caps.Process == nil {
				return nil
			}
			return caps.Process.Kill()
		}
		if waitErr := waitCapsuleProcess(ctx, caps.wait, kill, capsuleProcessWaitTimeout); waitErr != nil {
			cleanupErr = errors.Join(cleanupErr, waitErr)
		}
	}
	if err := unmountCapsuleRoot(caps.MergedDir); err != nil {
		cleanupErr = errors.Join(cleanupErr, err)
	}
	if caps.Cgroup != nil {
		if err := caps.Cgroup.Delete(); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	if cleanupErr == nil {
		if err := removePrivateTree(filepath.Join(e.stateDir, id)); err != nil {
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
	handle, err := randomOpaque("h-")
	if err != nil {
		return nil, err
	}
	return e.MintCapabilityHandle(agentRunID, role, capsuleID, handle, ttl)
}

// MintCapabilityHandle installs a runtime-precommitted opaque handle after the
// assignment opener has durably bound its digest. The handle remains usable
// only by the exact run/capsule pair and is never returned to the parent Super.
func (e *Executor) MintCapabilityHandle(agentRunID string, role AgentRole, capsuleID, handle string, ttl time.Duration) (*Capability, error) {
	if strings.TrimSpace(agentRunID) == "" || strings.TrimSpace(handle) == "" || handle != strings.TrimSpace(handle) || ttl <= 0 || ttl > 24*time.Hour {
		return nil, fmt.Errorf("capsule capability requires run identity, canonical opaque handle, and ttl in (0,24h]")
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
	key := capKey{AgentRunID: agentRunID, Handle: handle}
	if _, exists := e.capabilities[key]; exists {
		return nil, fmt.Errorf("capsule capability handle already exists")
	}
	capabilityID, err := randomOpaque("cap-")
	if err != nil {
		return nil, err
	}
	capability := &Capability{
		CapabilityID: capabilityID, Handle: handle, CapsuleID: capsuleID, AgentRunID: agentRunID,
		AgentRole: role, TargetCapsule: capsuleID, Verbs: cloneVerbSet(RoleVerbSets[role]),
		ExpiresAt: time.Now().UTC().Add(ttl),
	}
	if err := SignCapability(capability, e.privateKey, "guest-ephemeral"); err != nil {
		return nil, err
	}
	e.capabilities[key] = capability
	copy := *capability
	return &copy, nil
}

// AssignmentHandle resolves the only active opaque handle for an exact
// assignment run/capsule pair. It is a trusted runtime bridge, never a model
// result or durable field.
func (e *Executor) AssignmentHandle(agentRunID, capsuleID string) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var handle string
	for key, capability := range e.capabilities {
		if key.AgentRunID != agentRunID || capability.TargetCapsule != capsuleID || e.revokedCaps[capability.CapabilityID] {
			continue
		}
		if handle != "" {
			return "", fmt.Errorf("capsule assignment capability is ambiguous")
		}
		handle = key.Handle
	}
	if handle == "" {
		return "", fmt.Errorf("capsule assignment capability unavailable")
	}
	return handle, nil
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
		AgentRunID: agentRunID, CapabilityHandleDigest: computerevent.DigestBytes([]byte(handle)),
		CapsuleID: caps.ID, Command: request.Command, Cwd: request.Cwd, ExitCode: result.ExitCode,
		StdoutDigest: computerevent.DigestBytes([]byte(result.Stdout)), StderrDigest: computerevent.DigestBytes([]byte(result.Stderr)),
		WorktreeDigest: worktreeDigest, SourceTreeDigest: caps.SourceSnapshotDigest, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	canonical, err := computerevent.CanonicalJSON(receipt)
	if err != nil {
		return ExecResult{}, err
	}
	receipt.ReceiptRef = "capsule-exec:sha256:" + computerevent.DigestBytes(canonical)
	storedCanonical, err := computerevent.CanonicalJSON(receipt)
	if err != nil {
		return ExecResult{}, err
	}
	if err := e.persistReceiptArtifact("execution", receipt.ReceiptRef, storedCanonical); err != nil {
		return ExecResult{}, err
	}
	e.mu.Lock()
	e.executionReceipts[receipt.ReceiptRef] = receipt
	e.mu.Unlock()
	result.ReceiptRef = receipt.ReceiptRef
	return result, nil
}

func digestCapsuleWorktree(ctx context.Context, caps *Capsule) (string, error) {
	return digestCanonicalSubjectTree(ctx, filepath.Join(caps.MergedDir, "workspace", "platform"))
}

func receiptArtifactName(ref string) string {
	digest := sha256.Sum256([]byte(ref))
	return hex.EncodeToString(digest[:]) + ".json"
}

func (e *Executor) persistReceiptArtifact(kind, ref string, canonical []byte) error {
	root := filepath.Join(e.stateDir, "receipts", kind)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	path := filepath.Join(root, receiptArtifactName(ref))
	if err := os.WriteFile(path, canonical, 0o400); err != nil {
		if existing, readErr := os.ReadFile(path); readErr != nil || !bytes.Equal(existing, canonical) {
			return fmt.Errorf("persist executor receipt artifact: %w", err)
		}
	}
	return nil
}

func (e *Executor) PersistGrantedFreezeReceipt(ctx context.Context, agentRunID, handle string) (CapsuleFateReceipt, error) {
	capability, caps, err := e.resolveOne(agentRunID, handle, "exec")
	if err != nil || capability.AgentRole != RoleCoSuper {
		return CapsuleFateReceipt{}, fmt.Errorf("capsule freeze receipt authority unavailable")
	}
	caps.mu.RLock()
	state := caps.State
	caps.mu.RUnlock()
	if state != StateFrozen {
		return CapsuleFateReceipt{}, fmt.Errorf("capsule freeze receipt requires frozen capsule")
	}
	finalDigest, err := digestCapsuleWorktree(ctx, caps)
	if err != nil {
		return CapsuleFateReceipt{}, err
	}
	receipt := CapsuleFateReceipt{AgentRunID: agentRunID, CapabilityHandleDigest: computerevent.DigestBytes([]byte(handle)), CapsuleID: caps.ID,
		Disposition: "frozen", SourceSubjectDigest: caps.SourceSnapshotDigest, FinalSubjectDigest: finalDigest, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	unsigned, err := json.Marshal(receipt)
	if err != nil {
		return CapsuleFateReceipt{}, err
	}
	receipt.ReceiptRef = "capsule-fate:sha256:" + computerevent.DigestBytes(unsigned)
	canonical, err := json.Marshal(receipt)
	if err != nil {
		return CapsuleFateReceipt{}, err
	}
	if err := e.persistReceiptArtifact("fate", receipt.ReceiptRef, canonical); err != nil {
		return CapsuleFateReceipt{}, err
	}
	return receipt, nil
}

func (e *Executor) OpenCapsuleFateReceipt(ref string) (CapsuleFateReceipt, error) {
	raw, err := os.ReadFile(filepath.Join(e.stateDir, "receipts", "fate", receiptArtifactName(ref)))
	if err != nil {
		return CapsuleFateReceipt{}, fmt.Errorf("capsule fate receipt unavailable")
	}
	var receipt CapsuleFateReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil || receipt.ReceiptRef != ref {
		return CapsuleFateReceipt{}, fmt.Errorf("capsule fate receipt is invalid")
	}
	unsigned := receipt
	unsigned.ReceiptRef = ""
	canonical, err := json.Marshal(unsigned)
	if err != nil || "capsule-fate:sha256:"+computerevent.DigestBytes(canonical) != ref {
		return CapsuleFateReceipt{}, fmt.Errorf("capsule fate receipt digest mismatch")
	}
	return receipt, nil
}

func (e *Executor) OpenExecutionReceipt(ref string) (ExecutionReceipt, error) {
	e.mu.RLock()
	stored, ok := e.executionReceipts[ref]
	e.mu.RUnlock()
	if ok {
		return stored, nil
	}
	raw, err := os.ReadFile(filepath.Join(e.stateDir, "receipts", "execution", receiptArtifactName(ref)))
	if err != nil {
		return ExecutionReceipt{}, fmt.Errorf("executor receipt unavailable")
	}
	var receipt ExecutionReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil || receipt.ReceiptRef != ref {
		return ExecutionReceipt{}, fmt.Errorf("executor receipt is invalid")
	}
	unsigned := receipt
	unsigned.ReceiptRef = ""
	canonical, err := computerevent.CanonicalJSON(unsigned)
	if err != nil || "capsule-exec:sha256:"+computerevent.DigestBytes(canonical) != ref {
		return ExecutionReceipt{}, fmt.Errorf("executor receipt digest mismatch")
	}
	return receipt, nil
}

func (e *Executor) OpenGrantedExecutionReceipt(ref string) (GrantedExecutionReceipt, error) {
	e.mu.RLock()
	stored, ok := e.grantedReceipts[ref]
	e.mu.RUnlock()
	if ok {
		return stored, nil
	}
	raw, err := os.ReadFile(filepath.Join(e.stateDir, "receipts", "granted", receiptArtifactName(ref)))
	if err != nil {
		return GrantedExecutionReceipt{}, fmt.Errorf("granted executor receipt unavailable")
	}
	var receipt GrantedExecutionReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil || receipt.ReceiptRef != ref {
		return GrantedExecutionReceipt{}, fmt.Errorf("granted executor receipt is invalid")
	}
	unsigned := receipt
	unsigned.ReceiptRef = ""
	canonical, err := json.Marshal(unsigned)
	if err != nil || "capsule-granted-exec:sha256:"+computerevent.DigestBytes(canonical) != ref {
		return GrantedExecutionReceipt{}, fmt.Errorf("granted executor receipt digest mismatch")
	}
	return receipt, nil
}

func (e *Executor) ResolveGrantedExecutionReceipts(ctx context.Context, agentRunID, handle string, refs []string) ([]ExecutionReceipt, error) {
	capability, caps, err := e.resolveOne(agentRunID, handle, "exec")
	if err != nil || capability.AgentRole != RoleCoSuper {
		return nil, fmt.Errorf("capsule execution evidence unavailable")
	}
	caps.mu.RLock()
	state := caps.State
	caps.mu.RUnlock()
	if state != StateFrozen {
		return nil, fmt.Errorf("capsule execution evidence requires frozen capsule")
	}
	worktreeDigest, err := digestCapsuleWorktree(ctx, caps)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	receipts := make([]ExecutionReceipt, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if _, duplicate := seen[ref]; duplicate {
			continue
		}
		receipt, found := e.executionReceipts[ref]
		handleDigest := computerevent.DigestBytes([]byte(handle))
		if !found || receipt.AgentRunID != agentRunID || receipt.CapabilityHandleDigest != handleDigest ||
			receipt.CapsuleID != caps.ID || receipt.ExitCode != 0 || receipt.WorktreeDigest != worktreeDigest ||
			receipt.SourceTreeDigest != caps.SourceSnapshotDigest {
			return nil, fmt.Errorf("capsule execution evidence does not bind the exact run, handle, capsule, frozen source, and final successful subject")
		}
		granted := GrantedExecutionReceipt{Execution: receipt, AgentRunID: agentRunID, CapabilityHandleDigest: handleDigest,
			CapsuleID: caps.ID, Frozen: true, SourceSubjectDigest: caps.SourceSnapshotDigest, FinalSubjectDigest: worktreeDigest}
		canonicalWithoutRef, canonicalErr := json.Marshal(granted)
		if canonicalErr != nil {
			return nil, canonicalErr
		}
		granted.ReceiptRef = "capsule-granted-exec:sha256:" + computerevent.DigestBytes(canonicalWithoutRef)
		canonical, canonicalErr := json.Marshal(granted)
		if canonicalErr != nil {
			return nil, canonicalErr
		}
		if persistErr := e.persistReceiptArtifact("granted", granted.ReceiptRef, canonical); persistErr != nil {
			return nil, persistErr
		}
		e.grantedReceipts[granted.ReceiptRef] = granted
		receipt.GrantedReceiptRef = granted.ReceiptRef
		seen[ref] = struct{}{}
		receipts = append(receipts, receipt)
	}
	if len(receipts) == 0 {
		return nil, fmt.Errorf("capsule execution evidence is required")
	}
	return receipts, nil
}

func (e *Executor) ResolveExecutionReceipts(refs []string) ([]ExecutionReceipt, error) {
	receipts := make([]ExecutionReceipt, 0, len(refs))
	for _, ref := range refs {
		receipt, err := e.OpenExecutionReceipt(ref)
		if err != nil {
			return nil, err
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
	if err := caps.acquireOp(); err != nil {
		return nil, err
	}
	defer caps.releaseOp()
	return caps.broker.ReadFile(ctx, capability, path)
}

func (e *Executor) WriteFile(ctx context.Context, agentRunID, handle, path string, content []byte, mode uint32) error {
	capability, caps, err := e.resolveOne(agentRunID, handle, "write_file")
	if err != nil {
		return err
	}
	if err := caps.acquireOp(); err != nil {
		return err
	}
	defer caps.releaseOp()
	return caps.broker.WriteFile(ctx, capability, path, content, mode)
}

func (e *Executor) ListDir(ctx context.Context, agentRunID, handle, path string) ([]string, error) {
	capability, caps, err := e.resolveOne(agentRunID, handle, "list_dir")
	if err != nil {
		return nil, err
	}
	if err := caps.acquireOp(); err != nil {
		return nil, err
	}
	defer caps.releaseOp()
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

// ExtractGranted closes the capsule's execution boundary before observing its
// diff. A frozen capsule may be retried after a later freeze step fails.
func (e *Executor) ExtractGranted(ctx context.Context, agentRunID, handle string) ([]FileChange, error) {
	capability, err := e.ResolveCapability(agentRunID, handle)
	if err != nil || capability.AgentRole != RoleCoSuper {
		return nil, fmt.Errorf("capsule granted diff unavailable")
	}
	e.mu.RLock()
	caps := e.capsules[capability.TargetCapsule]
	e.mu.RUnlock()
	if caps == nil {
		return nil, fmt.Errorf("capsule granted diff unavailable")
	}
	caps.mu.RLock()
	state := caps.State
	caps.mu.RUnlock()
	switch state {
	case StateActive:
		if err := caps.Quiesce(ctx); err != nil {
			return nil, fmt.Errorf("freeze capsule before extracting diff: %w", err)
		}
	case StateFrozen:
		// A failed freeze operation may retry against the same immutable tree.
	default:
		return nil, fmt.Errorf("capsule granted diff unavailable in state %s", state)
	}
	return caps.Diff(ctx)
}

func (e *Executor) ResolveGrantedCapsuleID(agentRunID, handle string) (string, error) {
	capability, err := e.ResolveCapability(agentRunID, handle)
	if err != nil || capability.AgentRole != RoleCoSuper {
		return "", fmt.Errorf("capsule granted identity unavailable")
	}
	return capability.TargetCapsule, nil
}
func (e *Executor) ResolveGrantedWorktreeDigest(ctx context.Context, agentRunID, handle string) (string, error) {
	capability, err := e.ResolveCapability(agentRunID, handle)
	if err != nil || capability.AgentRole != RoleCoSuper {
		return "", fmt.Errorf("capsule granted worktree unavailable")
	}
	e.mu.RLock()
	capsule := e.capsules[capability.TargetCapsule]
	e.mu.RUnlock()
	if capsule == nil {
		return "", fmt.Errorf("capsule granted worktree unavailable")
	}
	capsule.mu.RLock()
	state := capsule.State
	capsule.mu.RUnlock()
	if state != StateFrozen {
		return "", fmt.Errorf("capsule granted worktree requires frozen capsule")
	}
	digest, err := digestCapsuleWorktree(ctx, capsule)
	if err != nil {
		return "", err
	}
	return "sha256:" + strings.TrimPrefix(digest, "sha256:"), nil
}

// PersistGrantedCandidate exports the frozen complete subject tree into the
// executor's content-addressed subject store. The opaque artifact ref is
// durable and can later be supplied to PreflightSourceSnapshot for exact
// verification; it never exposes a host path.
func (e *Executor) PersistGrantedCandidate(ctx context.Context, agentRunID, handle string) (SourcePreflight, error) {
	capability, err := e.ResolveCapability(agentRunID, handle)
	if err != nil || capability.AgentRole != RoleCoSuper {
		return SourcePreflight{}, fmt.Errorf("capsule candidate authority unavailable")
	}
	e.mu.RLock()
	caps := e.capsules[capability.TargetCapsule]
	e.mu.RUnlock()
	if caps == nil {
		return SourcePreflight{}, fmt.Errorf("capsule candidate authority unavailable")
	}
	caps.mu.RLock()
	state := caps.State
	caps.mu.RUnlock()
	if state != StateFrozen {
		return SourcePreflight{}, fmt.Errorf("capsule candidate requires frozen capsule")
	}
	subjectRoot := filepath.Join(caps.MergedDir, "workspace", "platform")
	digest, err := digestCanonicalSubjectTree(ctx, subjectRoot)
	if err != nil {
		return SourcePreflight{}, err
	}
	if err := os.MkdirAll(filepath.Join(e.stateDir, "subjects"), 0o700); err != nil {
		return SourcePreflight{}, err
	}
	temporary, err := os.MkdirTemp(e.stateDir, ".candidate-")
	if err != nil {
		return SourcePreflight{}, err
	}
	defer removePrivateTree(temporary)
	target := filepath.Join(temporary, "workspace", "platform")
	if err := copyCanonicalSubjectTree(ctx, subjectRoot, target); err != nil {
		return SourcePreflight{}, err
	}
	copied, err := digestCanonicalSubjectTree(ctx, target)
	if err != nil || copied != digest {
		return SourcePreflight{}, fmt.Errorf("capsule candidate reconstruction digest mismatch")
	}
	if err := makeSubjectTreeReadOnly(target); err != nil {
		return SourcePreflight{}, err
	}
	final := filepath.Join(e.stateDir, "subjects", digest)
	if err := os.Rename(temporary, final); err != nil {
		existing, verifyErr := digestCanonicalSubjectTree(ctx, filepath.Join(final, "workspace", "platform"))
		if verifyErr != nil || existing != digest {
			return SourcePreflight{}, fmt.Errorf("persist content-addressed capsule candidate: %w", err)
		}
	}
	return SourcePreflight{SubjectDigest: digest, ArtifactRef: subjectArtifactPrefix + digest}, nil
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

func (e *Executor) StageGrantedRelease(ctx context.Context, agentRunID, handle, incomingRoot string) ([]FrozenReleaseFile, string, error) {
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
	caps.mu.RLock()
	state := caps.State
	caps.mu.RUnlock()
	if state != StateFrozen {
		return nil, "", fmt.Errorf("capsule release staging requires frozen capsule")
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
	changes, err := caps.Diff(ctx)
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
	rootFD, err := unix.Open(caps.MergedDir, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, "", fmt.Errorf("capsule release root is unavailable: %w", err)
	}
	defer unix.Close(rootFD)
	var files []FrozenReleaseFile
	var total int64
	for _, change := range changes {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		if !strings.HasPrefix(change.Path, releasePrefix) {
			continue
		}
		relative := strings.TrimPrefix(change.Path, releasePrefix)
		clean := filepath.Clean(filepath.FromSlash(relative))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || change.Kind == ChangeDeleted {
			return nil, "", fmt.Errorf("capsule release contains unsafe path %q", change.Path)
		}
		sourcePath := filepath.FromSlash(strings.TrimPrefix(change.Path, "/"))
		sourceFD, err := unix.Openat2(rootFD, sourcePath, &unix.OpenHow{
			Flags:   uint64(unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NONBLOCK),
			Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS,
		})
		if err != nil {
			return nil, "", fmt.Errorf("capsule release file %q is unavailable: %w", change.Path, err)
		}
		input := os.NewFile(uintptr(sourceFD), change.Path)
		info, err := input.Stat()
		if err != nil {
			_ = input.Close()
			return nil, "", fmt.Errorf("capsule release file %q is unavailable", change.Path)
		}
		if info.IsDir() {
			_ = input.Close()
			continue
		}
		if !info.Mode().IsRegular() {
			_ = input.Close()
			return nil, "", fmt.Errorf("capsule release file %q is not regular", change.Path)
		}
		total += info.Size()
		if total > caps.MemoryMax {
			_ = input.Close()
			return nil, "", fmt.Errorf("capsule release exceeds resource budget")
		}
		target := filepath.Join(temporary, clean)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			_ = input.Close()
			return nil, "", err
		}
		secretPath := false
		for _, component := range strings.Split(filepath.ToSlash(clean), "/") {
			base := strings.ToLower(component)
			extension := strings.ToLower(filepath.Ext(base))
			if base == ".env" || strings.HasPrefix(base, ".env.") || base == ".npmrc" || base == ".netrc" ||
				base == "credentials.json" || base == "auth.json" || base == "id_rsa" || base == "id_ed25519" ||
				extension == ".pem" || extension == ".key" || extension == ".p12" || extension == ".pfx" {
				secretPath = true
				break
			}
		}
		if secretPath {
			_ = input.Close()
			return nil, "", fmt.Errorf("capsule release refuses secret-bearing path %q", change.Path)
		}
		scanner := bufio.NewScanner(&contextReader{ctx: ctx, reader: input})
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
		_, copyErr := io.Copy(io.MultiWriter(output, hash), &contextReader{ctx: ctx, reader: input})
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
	hasAutoputer, hasFrontend := false, false
	for _, file := range files {
		if file.Path == "bin/autoputer" && file.Mode&0o111 != 0 {
			hasAutoputer = true
		}
		if file.Path == "frontend/index.html" || strings.HasPrefix(file.Path, "frontend/") {
			hasFrontend = true
		}
	}
	if !hasAutoputer {
		return nil, "", fmt.Errorf("capsule release must contain executable bin/autoputer")
	}
	if !hasFrontend {
		return nil, "", fmt.Errorf("capsule freeze: computer-surface frontend artifacts are underivable")
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

func validCapsuleResidueIdentity(capsuleID string) bool {
	return capsuleID != "" && strings.TrimSpace(capsuleID) == capsuleID && capsuleID != "." && capsuleID != ".." &&
		!strings.ContainsAny(capsuleID, `/\`) && filepath.Base(capsuleID) == capsuleID
}

func mountedAtPath(path string) (bool, error) {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return false, fmt.Errorf("read mountinfo: %w", err)
	}
	want := filepath.Clean(path)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		mountPoint := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`).Replace(fields[4])
		if filepath.Clean(mountPoint) == want {
			return true, nil
		}
	}
	return false, nil
}

func (e *Executor) capsuleResidueExists(capsuleID string) (bool, error) {
	if !validCapsuleResidueIdentity(capsuleID) {
		return false, fmt.Errorf("capsule residue identity is invalid")
	}
	if _, err := os.Lstat(filepath.Join(e.stateDir, capsuleID)); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return capsuleCgroupResidueExists(capsuleID)
}

// CleanupOrphanedCapsule closes executor residue after a guest-core restart.
// It never trusts the reconstructed empty in-memory map: the exact cgroup is
// killed/deleted, the exact overlay is detached, and the private state tree is
// removed before absence can be receipted.
func (e *Executor) CleanupOrphanedCapsule(_ context.Context, capsuleID string) error {
	if !validCapsuleResidueIdentity(capsuleID) {
		return fmt.Errorf("orphan capsule cleanup identity is invalid")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, live := e.capsules[capsuleID]; live {
		return fmt.Errorf("orphan capsule cleanup cannot replace live executor destruction")
	}
	if err := cleanupOrphanedCapsuleCgroup(capsuleID); err != nil {
		return err
	}
	base := filepath.Join(e.stateDir, capsuleID)
	if _, err := os.Lstat(base); err == nil {
		root := filepath.Join(base, "root")
		mounted, mountErr := mountedAtPath(root)
		if mountErr != nil {
			return mountErr
		}
		if mounted {
			if err := unmountCapsuleRoot(root); err != nil {
				return err
			}
		}
		if err := removePrivateTree(base); err != nil {
			return fmt.Errorf("remove orphan capsule state: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	residue, err := e.capsuleResidueExists(capsuleID)
	if err != nil {
		return err
	}
	if residue {
		return fmt.Errorf("orphan capsule residue remained after cleanup")
	}
	return nil
}

// PersistRevocationReceipt durably acknowledges an already-requested exact
// assignment fate only after this executor has no capsule/process/overlay for
// the identity. capabilityDigest is the durable sha256 handle binding; the raw
// handle is neither required after revocation nor reintroduced.
func (e *Executor) PersistRevocationReceipt(agentRunID, capabilityDigest, capsuleID, intentRef string) (CapsuleRevocationReceipt, error) {
	agentRunID, capabilityDigest = strings.TrimSpace(agentRunID), strings.TrimSpace(capabilityDigest)
	intentRef = strings.TrimSpace(intentRef)
	encodedCapability := strings.TrimPrefix(capabilityDigest, "sha256:")
	if agentRunID == "" || !validCapsuleResidueIdentity(capsuleID) || intentRef == "" || !strings.HasPrefix(capabilityDigest, "sha256:") || len(encodedCapability) != sha256.Size*2 {
		return CapsuleRevocationReceipt{}, fmt.Errorf("capsule revocation receipt binding is invalid")
	}
	if _, err := hex.DecodeString(encodedCapability); err != nil {
		return CapsuleRevocationReceipt{}, fmt.Errorf("capsule revocation receipt binding is invalid")
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	if _, exists := e.capsules[capsuleID]; exists {
		return CapsuleRevocationReceipt{}, fmt.Errorf("capsule revocation acknowledgement requires absent capsule")
	}
	residue, residueErr := e.capsuleResidueExists(capsuleID)
	if residueErr != nil {
		return CapsuleRevocationReceipt{}, fmt.Errorf("inspect capsule revocation residue: %w", residueErr)
	}
	if residue {
		return CapsuleRevocationReceipt{}, fmt.Errorf("capsule revocation acknowledgement requires absent process/cgroup/overlay/state residue")
	}
	receipt := CapsuleRevocationReceipt{AgentRunID: agentRunID, AssignmentCapabilityDigest: capabilityDigest,
		CapsuleID: capsuleID, IntentRef: intentRef, Disposition: "revoked", CapsuleAbsent: true, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	unsigned, err := json.Marshal(receipt)
	if err != nil {
		return CapsuleRevocationReceipt{}, err
	}
	receipt.ReceiptRef = "capsule-revoke:sha256:" + computerevent.DigestBytes(unsigned)
	canonical, err := json.Marshal(receipt)
	if err != nil {
		return CapsuleRevocationReceipt{}, err
	}
	if err := e.persistReceiptArtifact("revocation", receipt.ReceiptRef, canonical); err != nil {
		return CapsuleRevocationReceipt{}, err
	}
	return receipt, nil
}

// HasCapsule is the trusted executor acknowledgement used by durable fate
// reconciliation. False means this executor owns no live or quarantined
// process/cgroup/overlay for the identity.
func (e *Executor) HasCapsule(id string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.capsules[id]
	return ok
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
	if !validCapsuleResidueIdentity(spec.CapsuleID) || spec.OwnerRunID == "" {
		return fmt.Errorf("capsule spawn requires safe capsule and owner-run identities")
	}
	if spec.MemoryMax <= 0 || spec.CpuQuota <= 0 || spec.CpuPeriod <= 0 || spec.PidsMax <= 0 {
		return fmt.Errorf("capsule spawn requires positive memory, cpu, period, and pid limits")
	}
	return nil
}

func installBrokerMount(ctx context.Context, source, root string) error {
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
	if err := waitForCommandStart(ctx, capsuleBrokerStartTimeout, func() error {
		return unix.Mount(source, target, "", unix.MS_BIND, "")
	}); err != nil {
		return fmt.Errorf("capsule bind broker: %w", err)
	}
	if err := waitForCommandStart(ctx, capsuleBrokerStartTimeout, func() error {
		return unix.Mount("", target, "", unix.MS_BIND|unix.MS_REMOUNT|unix.MS_RDONLY|unix.MS_NOSUID|unix.MS_NODEV, "")
	}); err != nil {
		return fmt.Errorf("capsule harden broker mount: %w", err)
	}
	return nil
}

func writeCapsuleIdentityEtc(upperDir string) error {
	etc := filepath.Join(upperDir, "etc")
	if err := os.MkdirAll(etc, 0o755); err != nil {
		return fmt.Errorf("capsule prepare etc: %w", err)
	}
	for name, content := range capsuleIdentityEtcFiles() {
		path := filepath.Join(etc, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("capsule write %s: %w", name, err)
		}
	}
	return nil
}

func capsuleIdentityEtcFiles() map[string]string {
	return map[string]string{
		"passwd":        "root:x:0:0:Capsule:/root:/bin/sh\n",
		"group":         "root:x:0:\n",
		"hosts":         "127.0.0.1 localhost\n::1 localhost\n",
		"nsswitch.conf": "hosts: files\n",
	}
}

func prepareCapsuleRoot(root, upperDir string) error {
	for _, path := range []string{"run", "tmp", "home", "root", "etc", "mnt", "var", "dev", "proc", "sys", "nix/store"} {
		if err := os.MkdirAll(filepath.Join(root, path), 0o755); err != nil {
			return fmt.Errorf("capsule prepare root: %w", err)
		}
	}
	for _, path := range []string{"run", "tmp", "mnt", "proc", "sys"} {
		target := filepath.Join(root, path)
		mode := "mode=0755,size=64m"
		if path == "tmp" || path == "mnt" || path == "run" {
			mode = "mode=1777,size=256m"
		}
		if err := unix.Mount("tmpfs", target, "tmpfs", unix.MS_NOSUID|unix.MS_NODEV, mode); err != nil {
			return fmt.Errorf("capsule mask %s: %w", path, err)
		}
		if path == "tmp" || path == "mnt" || path == "run" {
			_ = os.Chmod(target, 0o1777)
		}
	}
	currentSystem := "/run/current-system"
	if _, err := os.Stat(currentSystem); err == nil {
		currentSystemTarget := filepath.Join(root, "run", "current-system")
		if err := os.MkdirAll(currentSystemTarget, 0o755); err == nil {
			if err := unix.Mount(currentSystem, currentSystemTarget, "", unix.MS_BIND|unix.MS_REC, ""); err == nil {
				_ = unix.Mount("", currentSystemTarget, "", unix.MS_BIND|unix.MS_REMOUNT|unix.MS_RDONLY|unix.MS_NOSUID|unix.MS_NODEV, "")
			}
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
	pts := filepath.Join(root, "dev", "pts")
	if err := os.MkdirAll(pts, 0o755); err != nil {
		return fmt.Errorf("capsule prepare dev/pts: %w", err)
	}
	if err := unix.Mount("/dev/pts", pts, "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("capsule bind dev/pts: %w", err)
	}
	if err := writeCapsuleIdentityEtc(upperDir); err != nil {
		return err
	}
	return nil
}

func unmountCapsuleRoot(root string) error {
	currentSystemMount := filepath.Join(root, "run", "current-system")
	_ = unix.Unmount(currentSystemMount, unix.MNT_DETACH)
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

func removePrivateTree(root string) error {
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			_ = os.Chmod(path, 0o700)
		} else if info.Mode().IsRegular() {
			_ = os.Chmod(path, 0o600)
		}
		return nil
	})
	return os.RemoveAll(root)
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
