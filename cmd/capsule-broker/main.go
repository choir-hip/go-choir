//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Broker is the exec-broker that runs inside each capsule's namespace.
// It accepts JSON-RPC over a Unix domain socket and verifies Ed25519
// capabilities on every request.
//
// The broker is bind-mounted from a content-addressed host store (v2 decision).
// Its binary hash is verified at spawn time.
type Broker struct {
	mu                 sync.RWMutex
	socketPath         string
	capsuleID          string            // this broker's capsule ID (binding check)
	publicKey          ed25519.PublicKey // injected by Executor at spawn
	mergedDir          string            // capsule's merged overlayfs mount
	sessions           map[string]*Session
	sessionWorkers     map[string]*sessionWorker
	sessionMu          sync.Mutex
	revokedCaps        map[string]bool
	authorizedPeerUID  uint32
	listener           net.Listener
	brokerBin          string // path to this broker binary (for go_eval worker spawn)
	actuator           actuatorRoute
	sessionWorkerReady bool // session worker manager ships with Def 2 item 3
}

// actuatorRoute selects the execution route for an activation. Tools is the
// legacy JSON-verb dispatcher; RLM routes model work through model-written Go
// in the persistent session interpreter.
type actuatorRoute string

const (
	actuatorTools actuatorRoute = actuatorRoute(capsule.ActuatorTools)
	actuatorRLM   actuatorRoute = actuatorRoute(capsule.ActuatorRLM)
)

// resolveActuatorRoute is the dispatch side of the route authority: the guest
// kernel command line (machine boot setting choir.actuator=) wins, the
// forwarded broker env CHOIR_ACTUATOR is the fallback, and anything else
// fails closed to tools. The model-facing schema side lives in the host
// overlay builder (agentcore derives it from the host env); get_actuator
// only advertises this broker's resolved route for diagnosis.
func resolveActuatorRoute() actuatorRoute {
	return actuatorRoute(capsule.EffectiveActuator())
}

// effectiveRoute applies session-worker readiness: RLM requested but not
// ready falls back to tools with an observable receipt (Def 2 fallback).
func (b *Broker) effectiveRoute() actuatorRoute {
	if b != nil && b.actuator == actuatorRLM && b.sessionWorkerReady {
		return actuatorRLM
	}
	return actuatorTools
}

// Session represents a long-lived shell session.
type Session struct {
	ID        string
	Cmd       *exec.Cmd
	Stdin     io.WriteCloser
	Stdout    io.ReadCloser
	Stderr    io.ReadCloser
	Cwd       string
	Env       []string
	CreatedAt time.Time
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

func main() {
	var (
		socketPath         string
		capsuleID          string
		pubKeyHex          string
		mergedDir          string
		listenerFD         int
		isolationStage     string
		authorizedPeerUID  uint
		sessionComputerID  string
		sessionEpoch       uint64
		sessionActivation  string
		sessionAllowedRoot string
		sessionRole        string
		sessionSockFD      int
	)

	flag.StringVar(&socketPath, "socket", "/tmp/capsule-broker.sock", "Unix socket path")
	flag.IntVar(&listenerFD, "listener-fd", -1, "inherited parent-owned Unix listener file descriptor")
	flag.StringVar(&isolationStage, "isolation-stage", "broker", "internal namespace launch stage")
	flag.StringVar(&capsuleID, "capsule-id", "", "Capsule ID this broker serves (binding check)")
	flag.StringVar(&pubKeyHex, "pubkey", "", "Ed25519 public key (hex)")
	flag.StringVar(&mergedDir, "merged", "/mnt/merged", "Merged overlayfs mount point")
	flag.UintVar(&authorizedPeerUID, "authorized-peer-uid", 65534, "UID guest-core presents inside the broker user namespace")
	flag.StringVar(&sessionComputerID, "session-computer-id", "", "Session worker computer identity")
	flag.Uint64Var(&sessionEpoch, "session-epoch", 1, "Session worker epoch fence")
	flag.StringVar(&sessionActivation, "session-activation", "", "Session worker activation identity")
	flag.StringVar(&sessionAllowedRoot, "session-allowed-root", "/tmp", "Session worker filesystem root")
	flag.StringVar(&sessionRole, "session-role", "", "Session worker role bounding the prebound choir surface (trusted, from verified capability)")
	flag.IntVar(&sessionSockFD, "session-sock-fd", -1, "inherited multiplexed session socket fd (Step 2 transport); -1 selects legacy stdio")
	flag.Parse()
	if uint64(authorizedPeerUID) > uint64(^uint32(0)) {
		log.Fatal("--authorized-peer-uid exceeds uint32")
	}
	if isolationStage == "exec-go-stdin" {
		// Standalone activation worker: read a SidecarRequest on stdin and
		// write a SidecarResponse on stdout. This is the killable process PG
		// boundary for model-authored Go evaluation inside the capsule.
		yaegikernel.ExecuteWorkerStdin()
		return
	}
	if isolationStage == "exec-go-session" {
		cfg := yaegikernel.SessionWorkerConfig{
			AllowedPackages: yaegikernel.DefaultSafeStdlibPackagesList(),
			ComputerID:      sessionComputerID,
			ActivationID:    sessionActivation,
			Epoch:           sessionEpoch,
			AllowedRoot:     sessionAllowedRoot,
			Role:            sessionRole,
		}
		if sessionSockFD >= 0 {
			// Multiplexed session socket (Step 2): the broker passed its
			// socketpair end as an inherited fd. Raw stdio piping is gone.
			sockFile := os.NewFile(uintptr(sessionSockFD), "session-sock")
			conn, err := net.FileConn(sockFile)
			_ = sockFile.Close()
			if err != nil {
				log.Fatalf("session socket fd %d: %v", sessionSockFD, err)
			}
			yaegikernel.ExecuteWorkerSessionConn(conn, cfg)
			return
		}
		// Legacy stdio fallback for pre-cutover brokers; emits the same
		// ready handshake so spawn verification cannot mistake silence.
		yaegikernel.ExecuteWorkerSessionStdin(cfg)
		return
	}
	if isolationStage == "launcher" {
		if listenerFD != 3 {
			log.Fatal("capsule broker launcher requires listener fd 3")
		}
		if err := runNamespaceLauncher(socketPath, capsuleID, pubKeyHex, mergedDir, authorizedPeerUID, listenerFD); err != nil {
			log.Fatalf("capsule broker launcher: %v", err)
		}
		return
	}
	if isolationStage != "broker" {
		log.Fatal("invalid capsule broker isolation stage")
	}

	if pubKeyHex == "" {
		log.Fatal("--pubkey is required (Ed25519 public key in hex)")
	}
	if capsuleID == "" {
		log.Fatal("--capsule-id is required (capsule binding check)")
	}

	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		log.Fatalf("failed to decode public key: %v", err)
	}
	if len(pubKeyBytes) != ed25519.PublicKeySize {
		log.Fatalf("invalid public key size: %d (expected %d)", len(pubKeyBytes), ed25519.PublicKeySize)
	}
	if listenerFD != 3 {
		log.Fatal("--listener-fd must be the parent-owned descriptor 3")
	}
	listenerFile := os.NewFile(uintptr(listenerFD), "capsule-broker-listener")
	if listenerFile == nil {
		log.Fatal("inherited broker listener is unavailable")
	}
	listener, err := net.FileListener(listenerFile)
	_ = listenerFile.Close()
	if err != nil {
		log.Fatalf("failed to inherit parent broker listener: %v", err)
	}
	if _, ok := listener.(*net.UnixListener); !ok {
		_ = listener.Close()
		log.Fatal("inherited broker listener is not Unix")
	}

	if err := unix.Mount("proc", "/proc", "proc", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC, "hidepid=2"); err != nil {
		log.Fatalf("failed to mount capsule procfs: %v", err)
	}
	if err := os.WriteFile("/run/capsule/empty", nil, 0o400); err != nil {
		log.Fatalf("failed to create procfs mask: %v", err)
	}
	if err := unix.Mount("/run/capsule/empty", "/proc/cmdline", "", unix.MS_BIND, ""); err != nil {
		log.Fatalf("failed to mask guest kernel command line: %v", err)
	}

	// Apply the filesystem boundary before the syscall filter, then make every
	// hardening failure fatal. The broker is guest TCB and must fail closed.
	landlock := capsule.NewBrokerLandlock(mergedDir, "/run/capsule/broker")
	if err := landlock.Apply(); err != nil {
		log.Fatalf("failed to apply Landlock restrictions: %v", err)
	}
	if err := capsule.DropBrokerCapabilities(); err != nil {
		log.Fatalf("failed to drop capabilities: %v", err)
	}
	if err := capsule.LoadBrokerFilter(); err != nil {
		log.Fatalf("failed to load seccomp filter: %v", err)
	}

	broker := &Broker{
		socketPath:         socketPath,
		capsuleID:          capsuleID,
		publicKey:          ed25519.PublicKey(pubKeyBytes),
		mergedDir:          mergedDir,
		authorizedPeerUID:  uint32(authorizedPeerUID),
		listener:           listener,
		sessions:           make(map[string]*Session),
		sessionWorkers:     make(map[string]*sessionWorker),
		revokedCaps:        make(map[string]bool),
		brokerBin:          "/run/capsule/broker",
		actuator:           resolveActuatorRoute(),
		sessionWorkerReady: true,
	}
	log.Printf("capsule-broker: actuator route=%s (CHOIR_ACTUATOR; session worker ready=%v)", broker.actuator, broker.sessionWorkerReady)

	// Handle signals for clean shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		log.Printf("capsule-broker: received signal %v, shutting down", sig)
		broker.listener.Close()
		os.Exit(0)
	}()

	log.Printf("capsule-broker: listening on %s (merged=%s)", socketPath, mergedDir)

	// Accept connections.
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept connection: %v", err)
			continue
		}
		go broker.handleConnection(conn)
	}
}

func runNamespaceLauncher(socketPath, capsuleID, pubKeyHex, mergedDir string, authorizedPeerUID uint, listenerFD int) error {
	listenerFile := os.NewFile(uintptr(listenerFD), "capsule-broker-listener")
	if listenerFile == nil {
		return fmt.Errorf("inherited listener is unavailable")
	}
	args := []string{
		"--socket", socketPath, "--listener-fd", "3", "--isolation-stage", "broker",
		"--capsule-id", capsuleID, "--pubkey", pubKeyHex, "--merged", mergedDir,
		"--authorized-peer-uid", fmt.Sprint(authorizedPeerUID),
	}
	command := exec.Command("/run/capsule/broker", args...)
	command.ExtraFiles = []*os.File{listenerFile}
	// Inherit the autoputer's environment (notably PATH) so exec.LookPath in
	// the broker resolves `sh` exactly as the service PATH intends — busybox
	// ash ahead of bash. A hardcoded PATH here would resolve /bin/sh to bash,
	// whose initialize_job_control fails inside the capsule PID namespace.
	command.Env = append(os.Environ(), "HOME=/root", "TMPDIR=/tmp")
	command.Stdout = os.Stdout
	command.SysProcAttr = &syscall.SysProcAttr{Cloneflags: unix.CLONE_NEWPID, Pdeathsig: syscall.SIGKILL}
	if err := command.Start(); err != nil {
		_ = listenerFile.Close()
		return fmt.Errorf("start isolated broker: %w", err)
	}
	_ = listenerFile.Close()
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(signals)
	select {
	case err := <-done:
		return err
	case received := <-signals:
		if err := command.Process.Signal(received); err != nil {
			_ = command.Process.Kill()
		}
		return <-done
	}
}

// resolveWithin resolves relPath relative to base, returning the cleaned
// absolute path. It rejects any path that escapes base after cleaning
// (e.g. "../../etc/passwd"). This is the path containment check that
// prevents traversal attacks on the broker file API.
func resolveWithin(base, rel string) (string, error) {
	cleaned := filepath.Clean(filepath.Join(base, rel))
	relBase, err := filepath.Rel(base, cleaned)
	if err != nil || strings.HasPrefix(relBase, "..") {
		return "", fmt.Errorf("path escapes capsule root: %s", rel)
	}
	return cleaned, nil
}

// handleConnection accepts only guest-core peers. The broker runs as child UID
// 0 mapped to host UID 65534; parent guest-core UID 0 is unmapped in the child
// namespace and therefore presents as overflow UID 65534. Requiring that
// overflow identity rejects capsule-internal child UID 0 before capability
// verification.
func (b *Broker) handleConnection(conn net.Conn) {
	defer conn.Close()
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return
	}
	var credential *unix.Ucred
	var controlErr error
	if err := raw.Control(func(fd uintptr) {
		credential, controlErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil || controlErr != nil || credential == nil || credential.Uid != b.authorizedPeerUID {
		return
	}
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)
	for {
		var req BrokerRPCRequest
		if err := decoder.Decode(&req); err != nil {
			return
		}
		if err := encoder.Encode(b.handleRPC(req)); err != nil {
			return
		}
	}
}

// handleRPC dispatches a broker RPC request.
func (b *Broker) handleRPC(req BrokerRPCRequest) BrokerRPCResponse {

	// Verify the capability.
	var cap capsule.Capability
	if err := json.Unmarshal(req.Capability, &cap); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to parse capability: %v", err)}
	}

	if err := cap.Verify(b.publicKey); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("capability verification failed: %v", err)}
	}

	b.mu.RLock()
	revoked := b.revokedCaps[cap.CapabilityID]
	b.mu.RUnlock()
	if revoked {
		return BrokerRPCResponse{Error: fmt.Sprintf("capability %s has been revoked", cap.CapabilityID)}
	}

	// Bind every request to this capsule and to the fixed role policy. The
	// signed payload's Verbs field is evidence only and never authority.
	if cap.CapsuleID != b.capsuleID || cap.TargetCapsule != b.capsuleID || cap.AgentRunID == "" {
		return BrokerRPCResponse{Error: "capability binding mismatch"}
	}
	if !capsule.RoleVerbSets[cap.AgentRole][req.Verb] {
		return BrokerRPCResponse{Error: fmt.Sprintf("role %s does not allow verb %s", cap.AgentRole, req.Verb)}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	switch req.Verb {
	case "exec":
		return b.handleExec(ctx, &cap, req.Params)
	case "read_file":
		return b.handleReadFile(ctx, &cap, req.Params)
	case "write_file":
		return b.handleWriteFile(ctx, &cap, req.Params)
	case "list_dir":
		return b.handleListDir(ctx, &cap, req.Params)
	case "stat":
		return b.handleStat(ctx, &cap, req.Params)
	case "mkdir":
		return b.handleMkdir(ctx, &cap, req.Params)
	case "remove":
		return b.handleRemove(ctx, &cap, req.Params)
	case "kill_session":
		return b.handleKillSession(ctx, &cap, req.Params)
	case "go_eval":
		return b.handleGoEval(ctx, &cap, req.Params)
	case "get_actuator":
		return b.handleGetActuator(ctx, &cap, req.Params)
	case "init_session":
		return b.handleInitSession(ctx, &cap, req.Params)
	case "close_session":
		return b.handleCloseSession(ctx, &cap, req.Params)
	default:
		return BrokerRPCResponse{Error: fmt.Sprintf("unknown verb: %s", req.Verb)}
	}
}

// handleGetActuator advertises the activation route: the requested actuator,
// the effective route after session-worker readiness (fallback to tools when
// RLM is requested but unready), and session readiness itself. The host reads
// this when building the model-facing tool schema so schema and dispatcher
// cannot disagree.
func (b *Broker) handleGetActuator(_ context.Context, _ *capsule.Capability, _ json.RawMessage) BrokerRPCResponse {
	requested := actuatorTools
	ready := false
	if b != nil {
		requested = b.actuator
		if requested == "" {
			requested = actuatorTools
		}
		ready = b.sessionWorkerReady
	}
	route := b.effectiveRoute()
	if requested == actuatorRLM && route == actuatorTools {
		log.Printf("capsule-broker: actuator fallback to tools (RLM requested, session worker not ready)")
	}
	raw, err := json.Marshal(map[string]string{
		"requested":     string(requested),
		"route":         string(route),
		"session_ready": map[bool]string{true: "true", false: "false"}[ready],
	})
	if err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to marshal actuator route: %v", err)}
	}
	return BrokerRPCResponse{Result: raw}
}

// handleExec executes a command in the capsule.
//
// Direct-argv is canonical: when params carry a non-empty args vector, the
// binary runs with no shell, a strict environment allowlist, and its own
// process group (SIGKILL reaped within execKillReapGrace). An empty args
// vector selects the frozen legacy shell path, kept only for rollback of
// legacy JSON tools and unreachable in RLM mode.
func (b *Broker) handleExec(ctx context.Context, cap *capsule.Capability, params json.RawMessage) BrokerRPCResponse {
	var p capsule.ExecRequest
	if err := json.Unmarshal(params, &p); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to parse exec params: %v", err)}
	}

	// Resolve cwd safely within the merged dir.
	cwd := p.Cwd
	if cwd == "" {
		cwd = "/"
	}
	cwdPath, err := resolveWithin(b.mergedDir, cwd)
	if err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("invalid cwd: %v", err)}
	}

	timeout := 60 * time.Second
	if p.TimeoutMS > 0 {
		timeout = time.Duration(p.TimeoutMS) * time.Millisecond
	}
	evalCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if len(p.Args) > 0 {
		return b.handleExecDirect(evalCtx, cwdPath, p)
	}
	return b.handleExecLegacyShell(evalCtx, cwdPath, p)
}

// execKillReapGrace bounds process-group reaping after SIGKILL: a timed-out
// command's whole group must be gone within half a second, never orphaned.
const execKillReapGrace = 500 * time.Millisecond

// directExecAllowlist is the authoritative environment allowlist for
// direct-argv execution. Only these keys pass; everything else from the
// caller or the broker daemon environment is dropped, so ambient credentials
// (CHOIR_*, tokens, keys) can never leak into executed commands.
var directExecAllowlist = map[string]bool{"PATH": true, "HOME": true, "TMPDIR": true, "LANG": true}

// directExecEnv builds the sanitized environment: fixed safe baseline plus
// caller overrides restricted to the allowlist. Credential-shaped keys never
// pass, even when allowlisted by name collision.
func directExecEnv(overrides []string) []string {
	env := []string{
		"PATH=/run/current-system/sw/bin:/bin:/usr/bin",
		"HOME=/tmp",
		"TMPDIR=/tmp",
		"LANG=C.UTF-8",
	}
	for _, kv := range overrides {
		name, value, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(name))
		if !directExecAllowlist[key] {
			continue
		}
		if strings.HasPrefix(key, "CHOIR_") || strings.Contains(key, "KEY") || strings.Contains(key, "TOKEN") {
			continue
		}
		env = append(env, key+"="+value)
	}
	return env
}

// handleExecDirect runs binary+args with no shell in its own process group.
func (b *Broker) handleExecDirect(ctx context.Context, cwdPath string, p capsule.ExecRequest) BrokerRPCResponse {
	binary := p.Command
	if !strings.Contains(binary, "/") {
		resolved, err := exec.LookPath(binary)
		if err != nil {
			return BrokerRPCResponse{Error: fmt.Sprintf("exec binary not found: %s", binary)}
		}
		binary = resolved
	}
	cmd := exec.CommandContext(ctx, binary, p.Args...)
	cmd.Dir = cwdPath
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Env = directExecEnv(p.Env)

	var stdout, stderr cappedBuffer
	stdout.max = goEvalMaxOutputBytes
	stderr.max = goEvalMaxOutputBytes
	if p.Stdin != "" {
		stdin, pipeErr := cmd.StdinPipe()
		if pipeErr != nil {
			return BrokerRPCResponse{Error: fmt.Sprintf("failed to create stdin pipe: %v", pipeErr)}
		}
		go func() {
			stdin.Write([]byte(p.Stdin))
			stdin.Close()
		}()
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("exec start failed: %v", err)}
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	result := capsule.ExecResult{SessionID: p.SessionID, Stdout: "", Stderr: ""}
	select {
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			_ = cmd.Process.Kill()
		}
		select {
		case <-done:
		case <-time.After(execKillReapGrace):
		}
		result.ExitCode = -1
		result.Stdout = stdout.String()
		result.Stderr = stderr.String()
		result.Duration = time.Since(start)
		resultBytes, _ := json.Marshal(result)
		return BrokerRPCResponse{Result: resultBytes}
	case waitErr := <-done:
		result.ExitCode = 0
		if waitErr != nil {
			if exitErr, ok := waitErr.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			} else {
				return BrokerRPCResponse{Error: fmt.Sprintf("exec failed: %v", waitErr)}
			}
		}
		result.Stdout = stdout.String()
		result.Stderr = stderr.String()
		result.Duration = time.Since(start)
		resultBytes, _ := json.Marshal(result)
		return BrokerRPCResponse{Result: resultBytes}
	}
}

// handleExecLegacyShell is the frozen rollback path for legacy JSON tools
// (sh -c string execution). It is unreachable in RLM mode: RLM callers must
// supply an args vector. Preserved byte-identical for mechanical rollback.
func (b *Broker) handleExecLegacyShell(ctx context.Context, cwdPath string, p capsule.ExecRequest) BrokerRPCResponse {
	shell := "sh"
	shellArgs := []string{"-c", p.Command}
	if _, err := exec.LookPath("sh"); err != nil {
		if path, err := exec.LookPath("bash"); err == nil {
			shell = path
			shellArgs = []string{"--noprofile", "--norc", "-c", p.Command}
		} else {
			shell = "/bin/sh"
		}
	}
	cmd := exec.CommandContext(ctx, shell, shellArgs...)
	cmd.Dir = cwdPath
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: false}
	brokerEnv := os.Environ()
	hasPath := false
	for _, env := range brokerEnv {
		if strings.HasPrefix(env, "PATH=") {
			hasPath = true
			break
		}
	}
	if !hasPath {
		brokerEnv = append(brokerEnv, "PATH=/run/current-system/sw/bin:/bin:/usr/bin")
	}
	cmd.Env = append(brokerEnv, p.Env...)

	var stdout, stderr cappedBuffer
	stdout.max = goEvalMaxOutputBytes
	stderr.max = goEvalMaxOutputBytes
	var execErr error
	if p.Stdin != "" {
		stdin, pipeErr := cmd.StdinPipe()
		if pipeErr != nil {
			return BrokerRPCResponse{Error: fmt.Sprintf("failed to create stdin pipe: %v", pipeErr)}
		}
		go func() {
			stdin.Write([]byte(p.Stdin))
			stdin.Close()
		}()
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	execErr = cmd.Run()

	result := capsule.ExecResult{
		ExitCode:  0,
		SessionID: p.SessionID,
		Duration:  0,
		Stdout:    stdout.String(),
		Stderr:    stderr.String(),
	}
	if execErr != nil {
		if exitErr, ok := execErr.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return BrokerRPCResponse{Error: fmt.Sprintf("exec failed: %v", execErr)}
		}
	}

	resultBytes, _ := json.Marshal(result)
	return BrokerRPCResponse{Result: resultBytes}
}

// handleGoEval evaluates model-authored Go source inside the capsule's
// restricted Yaegi interpreter, dispatching on the actuator route.
func (b *Broker) handleGoEval(ctx context.Context, cap *capsule.Capability, params json.RawMessage) BrokerRPCResponse {
	// RLM route serves cells on the activation's persistent session worker;
	// tools route keeps the one-shot worker. The route is resolved once per
	// call so an unhold/flag change takes effect without reboot.
	if b.effectiveRoute() == actuatorRLM {
		return b.handleGoEvalSession(ctx, cap, params)
	}
	return b.handleGoEvalOneShot(ctx, cap, params)
}

// handleGoEvalOneShot spawns this same broker binary in --exec-go-stdin
// worker mode as a separate killable process group, so a runaway interpreter
// is SIGKILLed on timeout and never runs in guest core. The session fallback
// calls this directly to avoid re-entering route dispatch (which would
// recurse under RLM when no session worker can start).
func (b *Broker) handleGoEvalOneShot(ctx context.Context, cap *capsule.Capability, params json.RawMessage) BrokerRPCResponse {
	var p capsule.GoEvalRequest
	if err := json.Unmarshal(params, &p); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to parse go_eval params: %v", err)}
	}

	// Resolve cwd safely within the merged dir.
	cwd := p.Cwd
	if cwd == "" {
		cwd = "/"
	}
	cwdPath, err := resolveWithin(b.mergedDir, cwd)
	if err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("invalid cwd: %v", err)}
	}

	timeout := 60 * time.Second
	if p.TimeoutMS > 0 {
		timeout = time.Duration(p.TimeoutMS) * time.Millisecond
	}

	// Server-side package allowlist, NEVER model-controlled. The model's
	// allowed_packages is ignored: the effective allowlist is derived from the
	// verified capability's AgentRole, so an actor cannot authorize its own
	// package deputies or otherwise expand its authority. This is the
	// assignment-scoped authority boundary, not a request-trusted vocabulary.
	allowed := yaegikernel.DefaultSafeStdlibPackages
	if cap.AgentRole == capsule.RoleCoSuper {
		// CoSuper may additionally use the narrow set needed for authoring,
		// but it is still a fixed server-owned set, not caller input.
		allowed = yaegikernel.DefaultSafeStdlibPackages
	}
	allowedList := make([]string, 0, len(allowed))
	for pkg := range allowed {
		allowedList = append(allowedList, pkg)
	}

	// Spawn the worker as a sibling child of the broker in its own process
	// group so a timeout can SIGKILL the whole group.
	req := yaegikernel.SidecarRequest{
		Source:          p.Source,
		AllowedPackages: allowedList,
	}
	reqData, err := json.Marshal(req)
	if err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to marshal go_eval request: %v", err)}
	}

	bin := "/run/capsule/broker"
	if b.brokerBin != "" {
		bin = b.brokerBin
	}
	evalCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(evalCtx, bin, "--isolation-stage", "exec-go-stdin")
	cmd.Dir = cwdPath
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL} // new group; worker dies if broker dies
	// Sanitized worker environment: the worker must not inherit broker-injected
	// credentials, CANONICAL paths, or control-socket variables. Only a minimal
	// PATH/TMPDIR is provided inside the capsule.
	cmd.Env = []string{"PATH=/run/current-system/sw/bin:/bin:/usr/bin", "TMPDIR=/tmp"}
	cmd.Stdin = bytes.NewReader(reqData)
	stdoutCap := &cappedBuffer{max: goEvalMaxOutputBytes}
	stderrCap := &cappedBuffer{max: goEvalMaxOutputBytes}
	cmd.Stdout = stdoutCap
	cmd.Stderr = stderrCap

	start := time.Now()
	err = cmd.Start()
	if err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to start go_eval worker: %v", err)}
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	result := capsule.GoEvalResult{ExitCode: 0}
	select {
	case <-evalCtx.Done():
		// Kill the whole worker process group, then REAP it with a bounded
		// grace period so no zombie or lingering cmd.Wait goroutine survives.
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			_ = cmd.Process.Kill()
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			// Worker not reaped in grace; it is already SIGKILLed as a group.
		}
		result.Stdout = stdoutCap.String()
		result.Stderr = stderrCap.String()
		result.Error = "go_eval evaluation timed out"
		result.ExitCode = 1 // a timed-out attempt must never look successful
		result.Duration = time.Since(start)
		resultBytes, _ := json.Marshal(result)
		return BrokerRPCResponse{Result: resultBytes}
	case waitErr := <-done:
		result.Stdout = stdoutCap.String()
		result.Stderr = stderrCap.String()
		result.Duration = time.Since(start)
		if stdoutCap.overflow || stderrCap.overflow {
			result.Error = "go_eval output exceeded limit (truncated)"
			if waitErr == nil {
				result.ExitCode = 1
			}
		}
		if waitErr != nil {
			if exitErr, ok := waitErr.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			}
			result.Error = waitErr.Error()
		}
		resultBytes, _ := json.Marshal(result)
		return BrokerRPCResponse{Result: resultBytes}
	}
}

// handleReadFile reads a file from the capsule.
func (b *Broker) handleReadFile(ctx context.Context, cap *capsule.Capability, params json.RawMessage) BrokerRPCResponse {
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to parse params: %v", err)}
	}

	fullPath, err := resolveWithin(b.mergedDir, p.Path)
	if err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("invalid path: %v", err)}
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to read file: %v", err)}
	}

	result, _ := json.Marshal(map[string][]byte{"content": data})
	return BrokerRPCResponse{Result: result}
}

// handleWriteFile writes a file to the capsule.
func (b *Broker) handleWriteFile(ctx context.Context, cap *capsule.Capability, params json.RawMessage) BrokerRPCResponse {
	var p struct {
		Path    string `json:"path"`
		Content []byte `json:"content"`
		Mode    uint32 `json:"mode"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to parse params: %v", err)}
	}

	fullPath, err := resolveWithin(b.mergedDir, p.Path)
	if err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("invalid path: %v", err)}
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to create parent dir: %v", err)}
	}

	mode := os.FileMode(0o644)
	if p.Mode != 0 {
		mode = os.FileMode(p.Mode)
	}

	if err := os.WriteFile(fullPath, p.Content, mode); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to write file: %v", err)}
	}

	return BrokerRPCResponse{Result: json.RawMessage(`"ok"`)}
}

// handleListDir lists directory contents.
func (b *Broker) handleListDir(ctx context.Context, cap *capsule.Capability, params json.RawMessage) BrokerRPCResponse {
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to parse params: %v", err)}
	}

	fullPath, err := resolveWithin(b.mergedDir, p.Path)
	if err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("invalid path: %v", err)}
	}
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to list dir: %v", err)}
	}

	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	result, _ := json.Marshal(names)
	return BrokerRPCResponse{Result: result}
}

// handleStat returns file stat info.
func (b *Broker) handleStat(ctx context.Context, cap *capsule.Capability, params json.RawMessage) BrokerRPCResponse {
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to parse params: %v", err)}
	}

	fullPath, err := resolveWithin(b.mergedDir, p.Path)
	if err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("invalid path: %v", err)}
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to stat: %v", err)}
	}

	fi := capsule.FileInfo{
		FiName:    info.Name(),
		FiSize:    info.Size(),
		FiMode:    uint32(info.Mode()),
		FiIsDir:   info.IsDir(),
		FiModTime: info.ModTime().Unix(),
	}

	result, _ := json.Marshal(fi)
	return BrokerRPCResponse{Result: result}
}

// handleMkdir creates a directory.
func (b *Broker) handleMkdir(ctx context.Context, cap *capsule.Capability, params json.RawMessage) BrokerRPCResponse {
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to parse params: %v", err)}
	}

	fullPath, err := resolveWithin(b.mergedDir, p.Path)
	if err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("invalid path: %v", err)}
	}
	if err := os.Mkdir(fullPath, 0o755); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to mkdir: %v", err)}
	}

	return BrokerRPCResponse{Result: json.RawMessage(`"ok"`)}
}

// handleRemove removes a file or directory.
func (b *Broker) handleRemove(ctx context.Context, cap *capsule.Capability, params json.RawMessage) BrokerRPCResponse {
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to parse params: %v", err)}
	}

	fullPath, err := resolveWithin(b.mergedDir, p.Path)
	if err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("invalid path: %v", err)}
	}
	if err := os.Remove(fullPath); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to remove: %v", err)}
	}

	return BrokerRPCResponse{Result: json.RawMessage(`"ok"`)}
}

// handleKillSession kills a shell session.
func (b *Broker) handleKillSession(ctx context.Context, cap *capsule.Capability, params json.RawMessage) BrokerRPCResponse {
	var p struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to parse params: %v", err)}
	}

	b.mu.Lock()
	session, exists := b.sessions[p.SessionID]
	if exists {
		delete(b.sessions, p.SessionID)
	}
	b.mu.Unlock()

	if !exists {
		return BrokerRPCResponse{Error: fmt.Sprintf("session %s not found", p.SessionID)}
	}

	if session.Cmd != nil && session.Cmd.Process != nil {
		session.Cmd.Process.Kill()
	}

	return BrokerRPCResponse{Result: json.RawMessage(`"ok"`)}
}

// handleSyncRevokedCaps updates the revoked capability set.
func (b *Broker) handleSyncRevokedCaps(params json.RawMessage) BrokerRPCResponse {
	var p struct {
		RevokedCapabilityIDs []string `json:"revoked_capability_ids"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to parse params: %v", err)}
	}

	b.mu.Lock()
	b.revokedCaps = make(map[string]bool, len(p.RevokedCapabilityIDs))
	for _, id := range p.RevokedCapabilityIDs {
		b.revokedCaps[id] = true
	}
	b.mu.Unlock()

	return BrokerRPCResponse{Result: json.RawMessage(`"ok"`)}
}

// closeNonStdioFDs closes all file descriptors except stdin (0), stdout (1),
// and stderr (2). This is the FD hygiene step from the v7 design.
// We collect all fd numbers first, then close them, to avoid closing
// the /proc/self/fd directory stream while iterating it.
func closeNonStdioFDs() error {
	// Collect fd numbers first (avoids closing the directory fd mid-iteration).
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return fmt.Errorf("failed to read /proc/self/fd: %w", err)
	}

	var fds []int
	for _, entry := range entries {
		var fd int
		if _, err := fmt.Sscanf(entry.Name(), "%d", &fd); err != nil {
			continue
		}
		if fd <= 2 {
			continue
		}
		fds = append(fds, fd)
	}

	// Now close them all.
	for _, fd := range fds {
		syscall.Close(fd)
	}

	return nil
}

// stdoutWriter and stderrWriter implement io.Writer for capturing output.
type stdoutWriter struct{ buf *[]byte }
type stderrWriter struct{ buf *[]byte }

func (w *stdoutWriter) Write(p []byte) (int, error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}

func (w *stderrWriter) Write(p []byte) (int, error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}

// cappedBuffer bounds total buffered output to maxBytes and records overflow.
// It lets the broker enforce a hard output limit so a misbehaving worker cannot
// exhaust the capsule cgroup or the trusted broker with unbounded output.
const goEvalMaxOutputBytes = 2 * 1024 * 1024 // 2 MiB

type cappedBuffer struct {
	buf      bytes.Buffer
	overflow bool
	max      int
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if c.buf.Len()+len(p) > c.max {
		c.overflow = true
		// Accept only the bytes that fit, then stop accepting.
		remaining := c.max - c.buf.Len()
		if remaining > 0 {
			_, _ = c.buf.Write(p[:remaining])
		}
		return len(p), nil
	}
	_, _ = c.buf.Write(p)
	return len(p), nil
}
func (c *cappedBuffer) String() string { return c.buf.String() }
