//go:build linux

package main

import (
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
)

// Broker is the exec-broker that runs inside each capsule's namespace.
// It accepts JSON-RPC over a Unix domain socket and verifies Ed25519
// capabilities on every request.
//
// The broker is bind-mounted from a content-addressed host store (v2 decision).
// Its binary hash is verified at spawn time.
type Broker struct {
	mu                sync.RWMutex
	socketPath        string
	capsuleID         string            // this broker's capsule ID (binding check)
	publicKey         ed25519.PublicKey // injected by Executor at spawn
	mergedDir         string            // capsule's merged overlayfs mount
	sessions          map[string]*Session
	revokedCaps       map[string]bool
	authorizedPeerUID uint32
	listener          net.Listener
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
		socketPath        string
		capsuleID         string
		pubKeyHex         string
		mergedDir         string
		listenerFD        int
		authorizedPeerUID uint
	)

	flag.StringVar(&socketPath, "socket", "/tmp/capsule-broker.sock", "Unix socket path")
	flag.IntVar(&listenerFD, "listener-fd", -1, "inherited parent-owned Unix listener file descriptor")
	flag.StringVar(&capsuleID, "capsule-id", "", "Capsule ID this broker serves (binding check)")
	flag.StringVar(&pubKeyHex, "pubkey", "", "Ed25519 public key (hex)")
	flag.StringVar(&mergedDir, "merged", "/mnt/merged", "Merged overlayfs mount point")
	flag.UintVar(&authorizedPeerUID, "authorized-peer-uid", 65534, "UID guest-core presents inside the broker user namespace")
	flag.Parse()
	if uint64(authorizedPeerUID) > uint64(^uint32(0)) {
		log.Fatal("--authorized-peer-uid exceeds uint32")
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
		socketPath:        socketPath,
		capsuleID:         capsuleID,
		publicKey:         ed25519.PublicKey(pubKeyBytes),
		mergedDir:         mergedDir,
		authorizedPeerUID: uint32(authorizedPeerUID),
		listener:          listener,
		sessions:          make(map[string]*Session),
		revokedCaps:       make(map[string]bool),
	}

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
	default:
		return BrokerRPCResponse{Error: fmt.Sprintf("unknown verb: %s", req.Verb)}
	}
}

// handleExec executes a command in the capsule.
func (b *Broker) handleExec(ctx context.Context, cap *capsule.Capability, params json.RawMessage) BrokerRPCResponse {
	var p capsule.ExecRequest
	if err := json.Unmarshal(params, &p); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to parse exec params: %v", err)}
	}

	// Create or reuse session.
	cwd := p.Cwd
	if cwd == "" {
		cwd = "/"
	}

	// Resolve cwd safely within the merged dir.
	cwdPath, err := resolveWithin(b.mergedDir, cwd)
	if err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("invalid cwd: %v", err)}
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", p.Command)
	cmd.Dir = cwdPath
	cmd.Env = append(os.Environ(), p.Env...)

	var stdout, stderr []byte
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
		stdout, execErr = cmd.CombinedOutput()
	} else {
		cmd.Stdout = &stdoutWriter{&stdout}
		cmd.Stderr = &stderrWriter{&stderr}
		execErr = cmd.Run()
	}

	result := capsule.ExecResult{
		ExitCode:  0,
		SessionID: p.SessionID,
		Duration:  0, // TODO: track duration
		Stdout:    string(stdout),
		Stderr:    string(stderr),
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
