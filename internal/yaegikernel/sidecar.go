package yaegikernel

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"os"
)

// SessionWorkerConfig carries the per-activation identity a session worker
// needs before serving frames: package allowlist, computer/epoch fencing for
// handle verification, the activation the choir scope reports, and the role
// bounding the prebound choir surface (trusted worker flag from the verified
// outer capability, never model input).
type SessionWorkerConfig struct {
	AllowedPackages []string
	ComputerID      string
	ActivationID    string
	Epoch           uint64
	AllowedRoot     string
	Role            string
	// Slot is the Engineering slot (implementation|verifier) carried from the
	// verified capability; the choir scope reports it through Context and
	// gates the verifier-only affordances on it.
	Slot string
	// MemoryLimitBytes is the worker's address-space cap (RLIMIT_AS), applied
	// by the worker entrypoint before it serves cells — the D2 memory cap so
	// model-authored Go cannot OOM the daemon. 0 = uncapped.
	MemoryLimitBytes uint64
	// KeepCellOutput cuts a successful cell's print over the bound and keeps
	// the whole text for choir.CellOut (LR lever 6). The host desk worker
	// sets it; the capsule path does not, so its output and receipt digest
	// stay uncut.
	KeepCellOutput bool
}

// ExecuteWorkerSessionStdin serves framed eval cells on stdin/stdout with one
// persistent Session (see RunSessionLoop). It exits zero on a close frame or
// EOF and non-zero when a cell poisons the session, so the broker respawns a
// clean worker. The process group boundary (set by the spawner) provides
// SIGKILL containment exactly like the one-shot worker. After broker
// construction and module prebinding it emits one {"id":"ready"} frame so
// the spawner distinguishes a live worker from a process that started and
// died (spawn success is not readiness).
func ExecuteWorkerSessionStdin(cfg SessionWorkerConfig) {
	sess, broker, scope := buildSessionWorker(cfg)
	ready, err := json.Marshal(SessionResult{ID: "ready"})
	if err != nil {
		os.Exit(2)
	}
	if _, err := os.Stdout.Write(append(ready, '\n')); err != nil {
		os.Exit(2)
	}
	hooks := cellHooks(scope, cfg)
	if err := RunSessionLoopWithDrainAndHooks(os.Stdin, os.Stdout, func() (*Session, error) {
		return sess, nil
	}, broker.DrainReceipts, &hooks); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

// buildSessionWorker assembles the persistent interpreter, broker, and choir
// scope shared by both worker transports (stdio legacy and framed socket).
// It exits 2 when the worker cannot prove it will serve cells correctly:
// spawn success is not readiness.
func buildSessionWorker(cfg SessionWorkerConfig) (*Session, *Broker, *ChoirScope) {
	fail := func(step string, err error) (*Session, *Broker, *ChoirScope) {
		fmt.Fprintf(os.Stderr, "session worker build: %s: %v\n", step, err)
		os.Exit(2)
		return nil, nil, nil
	}
	allowlist := NewAllowlist(cfg.AllowedPackages...)
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return fail("secret", err)
	}
	issuer, err := NewHandleIssuer(secret)
	if err != nil {
		return fail("issuer", err)
	}
	root := cfg.AllowedRoot
	if root == "" {
		root = "/tmp"
	}
	computerID := cfg.ComputerID
	if computerID == "" {
		computerID = "worker-local"
	}
	broker, err := NewBroker(BrokerConfig{ComputerID: computerID, CurrentEpoch: cfg.Epoch, AllowedRoot: root}, issuer)
	if err != nil {
		return fail("broker", err)
	}
	scope, err := NewChoirScope(broker, issuer, computerID, cfg.ActivationID, cfg.Epoch, cfg.Role, cfg.Slot)
	if err != nil {
		return fail("choirscope", err)
	}
	sess, err := NewSession(allowlist, scope.ChoirExports())
	if err != nil {
		return fail("session", err)
	}
	return sess, broker, scope
}

// cellHooks binds the scope's per-cell hooks for a worker; KeepOutput is
// set only when the spawner asked for cut output.
func cellHooks(scope *ChoirScope, cfg SessionWorkerConfig) CellHooks {
	hooks := scope.BindCell()
	if cfg.KeepCellOutput {
		hooks.KeepOutput = scope.keepCellOutput
	}
	return hooks
}

// ExecuteWorkerSessionConn serves framed eval cells over the multiplexed
// session socket (Step 2 transport). It emits one ready result frame after
// prebind, then serves until a close frame, EOF, or a poisoned cell. One
// reader loop owns the socket (workerMux): emit and egress calls wait for
// the result carrying their own request id, so goroutines in one cell may
// call them in parallel, and search arrivals land in the scope's store.
func ExecuteWorkerSessionConn(conn net.Conn, cfg SessionWorkerConfig) {
	fc := NewFramedConn(conn)
	sess, broker, scope := buildSessionWorker(cfg)
	ready, err := json.Marshal(SessionResult{ID: "ready"})
	if err != nil {
		os.Exit(2)
	}
	if err := fc.WriteFrame(StreamCell, ready); err != nil {
		os.Exit(2)
	}
	mux := newWorkerMux(fc, scope.searches)
	// Emit crosses the socket so the host performs the durable channel write
	// and recipient wake.
	broker.SetEmit(func(ctx context.Context, payload EmitPayload) (EmitResult, error) {
		raw, err := json.Marshal(payload)
		if err != nil {
			return EmitResult{}, fmt.Errorf("emit marshal: %w", err)
		}
		req := BrokerRequest{ProtocolVersion: ProtocolVersion, RequestID: newReceiptID(),
			HandleRef: scope.HandleRef(), Epoch: cfg.Epoch, Action: ActionEmit, Payload: raw}
		resp, err := mux.roundTrip(ctx, StreamBrokerEmit, StreamBrokerEmitResult, req)
		if err != nil {
			return EmitResult{}, fmt.Errorf("emit: %w", err)
		}
		if !resp.Success {
			return EmitResult{}, fmt.Errorf("emit: %s", resp.Error)
		}
		var out EmitResult
		_ = json.Unmarshal(resp.Result, &out)
		return out, nil
	})
	// Egress verbs ride one generic frame round trip; the host resolves the
	// action against its search/HTTP/content deps.
	broker.SetEgress(func(ctx context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
		req := BrokerRequest{ProtocolVersion: ProtocolVersion, RequestID: newReceiptID(),
			HandleRef: scope.HandleRef(), Epoch: cfg.Epoch, Action: action, Payload: payload}
		resp, err := mux.roundTrip(ctx, StreamBrokerEgress, StreamBrokerEgressResult, req)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", action, err)
		}
		if !resp.Success {
			return nil, fmt.Errorf("%s: %s", action, resp.Error)
		}
		return resp.Result, nil
	})
	hooks := cellHooks(scope, cfg)
	if err := runSessionLoopMux(mux, fc, func() (*Session, error) {
		return sess, nil
	}, broker.DrainReceipts, &hooks); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
