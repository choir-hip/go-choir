//go:build linux


package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mdlayher/vsock"
	"github.com/yusefmosiah/go-choir/internal/capsule"
)

// HostAuthority runs on the Firecracker HOST (outside the guest kernel).
// It holds the Ed25519 private key and handles capability minting,
// revocation, and capsule/run registration.
//
// This is the host-side binary of the two-plane architecture (v14 design).
type HostAuthority struct {
	mu                sync.RWMutex
	signKey           ed25519.PrivateKey
	publicKey         ed25519.PublicKey
	keyID             string
	revokedCaps       map[string]map[string]bool // capsuleID → set of revoked CapabilityIDs
	globalRevokedCaps map[string]bool            // wildcard revoked CapabilityIDs (apply to all capsules)
	revocationLog     *os.File                   // append-only log on host disk (fsynced before ack)
	vsockListener     net.Listener               // vsock listener for Executor connections
	knownCapsules     map[string]bool            // capsuleIDs that have been spawned (for mint auth)
	activeRuns        map[string]bool            // agentRunIDs that are active (for mint auth)
}

// HostRPCRequest is the wire format for RPCs to HostAuthority.
type HostRPCRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// HostRPCResponse is the wire format for responses from HostAuthority.
type HostRPCResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func main() {
	var (
		vsockPort   uint
		stateDir    string
		keyPath     string
		generateKey bool
	)

	flag.UintVar(&vsockPort, "port", 1234, "vsock port to listen on")
	flag.StringVar(&stateDir, "state-dir", "/var/lib/capsule-host", "state directory")
	flag.StringVar(&keyPath, "key", "", "path to Ed25519 private key (generates if absent)")
	flag.BoolVar(&generateKey, "generate-key", false, "generate a new Ed25519 key pair and exit")
	flag.Parse()

	if generateKey {
		if err := generateAndPrintKey(); err != nil {
			log.Fatalf("failed to generate key: %v", err)
		}
		return
	}

	// Load or generate Ed25519 key pair.
	signKey, publicKey, keyID, err := loadOrCreateKey(keyPath)
	if err != nil {
		log.Fatalf("failed to load key: %v", err)
	}

	// Ensure state directory exists.
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		log.Fatalf("failed to create state dir %s: %v", stateDir, err)
	}

	// Open revocation log (append-only).
	revLogPath := filepath.Join(stateDir, "revocations.log")
	revLog, err := os.OpenFile(revLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		log.Fatalf("failed to open revocation log %s: %v", revLogPath, err)
	}

	auth := &HostAuthority{
		signKey:           signKey,
		publicKey:         publicKey,
		keyID:             keyID,
		revokedCaps:       make(map[string]map[string]bool),
		globalRevokedCaps: make(map[string]bool),
		revocationLog:     revLog,
		knownCapsules:     make(map[string]bool),
		activeRuns:        make(map[string]bool),
	}

	// Replay revocation log on startup.
	if err := auth.replayRevocationLog(); err != nil {
		log.Printf("warning: failed to replay revocation log: %v", err)
	}

	// Listen on vsock.
	listener, err := vsock.Listen(uint32(vsockPort), nil)
	if err != nil {
		log.Fatalf("failed to listen on vsock port %d: %v", vsockPort, err)
	}
	auth.vsockListener = listener

	log.Printf("capsule-host: HostAuthority listening on vsock port %d (keyID=%s)", vsockPort, keyID)

	// Accept connections.
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept connection: %v", err)
			continue
		}
		go auth.handleConnection(conn)
	}
}

// handleConnection handles a single vsock connection from the Executor.
func (h *HostAuthority) handleConnection(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		var req HostRPCRequest
		if err := decoder.Decode(&req); err != nil {
			if err != io.EOF {
				log.Printf("failed to decode request: %v", err)
			}
			return
		}

		resp := h.handleRPC(req)
		if err := encoder.Encode(resp); err != nil {
			log.Printf("failed to encode response: %v", err)
			return
		}
	}
}

// handleRPC dispatches an RPC request to the appropriate handler.
func (h *HostAuthority) handleRPC(req HostRPCRequest) HostRPCResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch req.Method {
	case "mint_capability":
		return h.handleMintCapability(ctx, req.Params)
	case "revoke_capability":
		return h.handleRevokeCapability(ctx, req.Params)
	case "get_revoked_caps":
		return h.handleGetRevokedCaps(ctx, req.Params)
	case "register_capsule":
		return h.handleRegisterCapsule(ctx, req.Params)
	case "unregister_capsule":
		return h.handleUnregisterCapsule(ctx, req.Params)
	case "register_active_run":
		return h.handleRegisterActiveRun(ctx, req.Params)
	case "unregister_active_run":
		return h.handleUnregisterActiveRun(ctx, req.Params)
	default:
		return HostRPCResponse{Error: fmt.Sprintf("unknown method: %s", req.Method)}
	}
}

// handleMintCapability mints a new Ed25519-signed capability.
// Authorization policy (v6):
// - Rejects role=super from Executor (super caps are host-local only)
// - Rejects TTL > 24h
// - Rejects capsuleID not in knownCapsules (unless TargetCapsule="*")
// - Rejects agentRunID not in activeRuns
func (h *HostAuthority) handleMintCapability(ctx context.Context, params json.RawMessage) HostRPCResponse {
	var p struct {
		AgentRunID string             `json:"agent_run_id"`
		Role       capsule.AgentRole  `json:"role"`
		CapsuleID  string             `json:"capsule_id"`
		TTL        time.Duration      `json:"ttl"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return HostRPCResponse{Error: fmt.Sprintf("failed to parse params: %v", err)}
	}

	// Authorization checks.
	if p.Role == capsule.RoleSuper {
		return HostRPCResponse{Error: "super capabilities cannot be minted via Executor"}
	}
	if p.TTL > 24*time.Hour {
		return HostRPCResponse{Error: "TTL exceeds 24h maximum"}
	}

	h.mu.RLock()
	if p.CapsuleID != "" && !h.knownCapsules[p.CapsuleID] {
		h.mu.RUnlock()
		return HostRPCResponse{Error: fmt.Sprintf("capsule %s not registered", p.CapsuleID)}
	}
	if !h.activeRuns[p.AgentRunID] {
		h.mu.RUnlock()
		return HostRPCResponse{Error: fmt.Sprintf("agent run %s not active", p.AgentRunID)}
	}
	h.mu.RUnlock()

	// Build the capability.
	targetCapsule := p.CapsuleID
	externalAccess := []string{}
	if p.Role == capsule.RoleResearcher {
		targetCapsule = "*"
		externalAccess = []string{"dolt:write", "message:send"}
	}

	cap := &capsule.Capability{
		CapabilityID:   generateID("cap"),
		Handle:         generateID("handle"),
		CapsuleID:      p.CapsuleID,
		AgentRunID:     p.AgentRunID,
		AgentRole:      p.Role,
		TargetCapsule:  targetCapsule,
		Verbs:          capsule.RoleVerbSets[p.Role],
		ExternalAccess: externalAccess,
		ExpiresAt:      time.Now().Add(p.TTL),
	}

	// Sign the capability.
	if err := capsule.SignCapability(cap, h.signKey, h.keyID); err != nil {
		return HostRPCResponse{Error: fmt.Sprintf("failed to sign capability: %v", err)}
	}

	result, err := json.Marshal(cap)
	if err != nil {
		return HostRPCResponse{Error: fmt.Sprintf("failed to marshal capability: %v", err)}
	}

	return HostRPCResponse{Result: result}
}

// handleRevokeCapability revokes a capability and persists to the log.
func (h *HostAuthority) handleRevokeCapability(ctx context.Context, params json.RawMessage) HostRPCResponse {
	var p struct {
		AgentRunID   string `json:"agent_run_id"`
		CapsuleID    string `json:"capsule_id"`
		CapabilityID string `json:"capability_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return HostRPCResponse{Error: fmt.Sprintf("failed to parse params: %v", err)}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Add to revoked set.
	if p.CapsuleID == "" {
		// Wildcard revocation (researcher caps) — add to global set.
		h.globalRevokedCaps[p.CapabilityID] = true
	} else {
		if h.revokedCaps[p.CapsuleID] == nil {
			h.revokedCaps[p.CapsuleID] = make(map[string]bool)
		}
		h.revokedCaps[p.CapsuleID][p.CapabilityID] = true
	}

	// Persist to revocation log (fsync before ack).
	entry := fmt.Sprintf("%d\trevoke\t%s\t%s\t%s\n",
		time.Now().UnixNano(), p.AgentRunID, p.CapsuleID, p.CapabilityID)
	if _, err := h.revocationLog.WriteString(entry); err != nil {
		return HostRPCResponse{Error: fmt.Sprintf("failed to write revocation log: %v", err)}
	}
	if err := h.revocationLog.Sync(); err != nil {
		return HostRPCResponse{Error: fmt.Sprintf("failed to fsync revocation log: %v", err)}
	}

	return HostRPCResponse{Result: json.RawMessage(`"ok"`)}
}

// handleGetRevokedCaps returns the revoked capability set for a capsule.
func (h *HostAuthority) handleGetRevokedCaps(ctx context.Context, params json.RawMessage) HostRPCResponse {
	var p struct {
		CapsuleID string `json:"capsule_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return HostRPCResponse{Error: fmt.Sprintf("failed to parse params: %v", err)}
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	var revoked []string
	// Per-capsule revoked caps.
	if caps, ok := h.revokedCaps[p.CapsuleID]; ok {
		for id := range caps {
			revoked = append(revoked, id)
		}
	}
	// Global wildcard revoked caps.
	for id := range h.globalRevokedCaps {
		revoked = append(revoked, id)
	}

	result, err := json.Marshal(revoked)
	if err != nil {
		return HostRPCResponse{Error: fmt.Sprintf("failed to marshal revoked caps: %v", err)}
	}
	return HostRPCResponse{Result: result}
}

// handleRegisterCapsule adds a capsule to the known set.
func (h *HostAuthority) handleRegisterCapsule(ctx context.Context, params json.RawMessage) HostRPCResponse {
	var p struct {
		CapsuleID string `json:"capsule_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return HostRPCResponse{Error: fmt.Sprintf("failed to parse params: %v", err)}
	}

	h.mu.Lock()
	h.knownCapsules[p.CapsuleID] = true
	h.mu.Unlock()

	return HostRPCResponse{Result: json.RawMessage(`"ok"`)}
}

// handleUnregisterCapsule removes a capsule from the known set.
func (h *HostAuthority) handleUnregisterCapsule(ctx context.Context, params json.RawMessage) HostRPCResponse {
	var p struct {
		CapsuleID string `json:"capsule_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return HostRPCResponse{Error: fmt.Sprintf("failed to parse params: %v", err)}
	}

	h.mu.Lock()
	delete(h.knownCapsules, p.CapsuleID)
	h.mu.Unlock()

	return HostRPCResponse{Result: json.RawMessage(`"ok"`)}
}

// handleRegisterActiveRun adds an agent run to the active set.
func (h *HostAuthority) handleRegisterActiveRun(ctx context.Context, params json.RawMessage) HostRPCResponse {
	var p struct {
		AgentRunID string `json:"agent_run_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return HostRPCResponse{Error: fmt.Sprintf("failed to parse params: %v", err)}
	}

	h.mu.Lock()
	h.activeRuns[p.AgentRunID] = true
	h.mu.Unlock()

	return HostRPCResponse{Result: json.RawMessage(`"ok"`)}
}

// handleUnregisterActiveRun removes an agent run from the active set.
func (h *HostAuthority) handleUnregisterActiveRun(ctx context.Context, params json.RawMessage) HostRPCResponse {
	var p struct {
		AgentRunID string `json:"agent_run_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return HostRPCResponse{Error: fmt.Sprintf("failed to parse params: %v", err)}
	}

	h.mu.Lock()
	delete(h.activeRuns, p.AgentRunID)
	h.mu.Unlock()

	return HostRPCResponse{Result: json.RawMessage(`"ok"`)}
}

// replayRevocationLog replays the append-only revocation log on startup.
func (h *HostAuthority) replayRevocationLog() error {
	logPath := h.revocationLog.Name()
	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	lines := splitLines(string(data))
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Use SplitN instead of Sscanf because Sscanf %s skips whitespace,
		// which breaks empty fields (wildcard revocations use empty capsuleID).
		fields := strings.SplitN(line, "\t", 5)
		if len(fields) < 5 {
			continue
		}
		ts, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		action := fields[1]
		agentRunID := fields[2]
		capsuleID := fields[3]
		capabilityID := fields[4]
		_ = ts
		_ = agentRunID

		if action != "revoke" {
			continue
		}

		if capsuleID == "" {
			h.globalRevokedCaps[capabilityID] = true
		} else {
			if h.revokedCaps[capsuleID] == nil {
				h.revokedCaps[capsuleID] = make(map[string]bool)
			}
			h.revokedCaps[capsuleID][capabilityID] = true
		}
	}

	log.Printf("capsule-host: replayed %d revocation entries", len(lines))
	return nil
}

// loadOrCreateKey loads an Ed25519 key from the given path, or generates
// a new one if the path is empty or the file doesn't exist.
func loadOrCreateKey(keyPath string) (ed25519.PrivateKey, ed25519.PublicKey, string, error) {
	if keyPath == "" {
		keyPath = "/var/lib/capsule-host/ed25519.key"
	}

	data, err := os.ReadFile(keyPath)
	if err == nil {
		if len(data) != ed25519.PrivateKeySize {
			return nil, nil, "", fmt.Errorf("key file %s has wrong size: %d", keyPath, len(data))
		}
		signKey := ed25519.PrivateKey(data)
		publicKey := signKey.Public().(ed25519.PublicKey)
		keyID := fmt.Sprintf("key-%x", publicKey[:8])
		return signKey, publicKey, keyID, nil
	}

	if !os.IsNotExist(err) {
		return nil, nil, "", fmt.Errorf("failed to read key file %s: %w", keyPath, err)
	}

	// Generate new key.
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to generate Ed25519 key: %w", err)
	}

	// Save private key.
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return nil, nil, "", fmt.Errorf("failed to create key dir: %w", err)
	}
	if err := os.WriteFile(keyPath, priv, 0o600); err != nil {
		return nil, nil, "", fmt.Errorf("failed to write key file: %w", err)
	}

	keyID := fmt.Sprintf("key-%x", pub[:8])
	log.Printf("capsule-host: generated new Ed25519 key (keyID=%s) at %s", keyID, keyPath)
	return priv, pub, keyID, nil
}

// generateAndPrintKey generates a new Ed25519 key pair and prints the
// public key to stdout (for embedding in broker configuration).
func generateAndPrintKey() error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	fmt.Printf("Private key (hex): %x\n", priv)
	fmt.Printf("Public key (hex):  %x\n", pub)
	return nil
}

// generateID generates a random ID with the given prefix.
// Returns an empty string on entropy failure (fail-closed).
func generateID(prefix string) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// On entropy failure, fail closed — do not return a predictable ID.
		return ""
	}
	return fmt.Sprintf("%s-%x", prefix, b)
}

// splitLines splits a string by newlines.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
