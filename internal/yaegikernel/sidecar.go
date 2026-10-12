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
	hooks := scope.BindCell()
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

// ExecuteWorkerSessionConn serves framed eval cells over the multiplexed
// session socket (Step 2 transport). It emits one ready result frame after
// prebind, then serves until a close frame, EOF, or a poisoned cell.
func ExecuteWorkerSessionConn(conn net.Conn, cfg SessionWorkerConfig) {
	fc := NewFramedConn(conn)
	sess, broker, scope := buildSessionWorker(cfg)
	// Emit crosses the socket so the host performs the durable channel write
	// + recipient wake. The cell is blocked inside serveCell while this runs,
	// so a synchronous frame round-trip on fc is the only reader — the host's
	// Eval loop answers the emit then returns to the cell's pending StreamCell.
	broker.SetEmit(func(ctx context.Context, payload EmitPayload) (EmitResult, error) {
		raw, err := json.Marshal(payload)
		if err != nil {
			return EmitResult{}, fmt.Errorf("emit marshal: %w", err)
		}
		req := BrokerRequest{ProtocolVersion: ProtocolVersion, RequestID: newReceiptID(),
			HandleRef: scope.HandleRef(), Epoch: cfg.Epoch, Action: ActionEmit, Payload: raw}
		enc, err := json.Marshal(req)
		if err != nil {
			return EmitResult{}, fmt.Errorf("emit request marshal: %w", err)
		}
		if err := fc.WriteFrame(StreamBrokerEmit, enc); err != nil {
			return EmitResult{}, fmt.Errorf("emit write frame: %w", err)
		}
		stream, rraw, err := fc.ReadFrame()
		if err != nil {
			return EmitResult{}, fmt.Errorf("emit read result: %w", err)
		}
		if stream != StreamBrokerEmitResult {
			return EmitResult{}, fmt.Errorf("emit: unexpected stream %d", stream)
		}
		var resp BrokerResponse
		if err := json.Unmarshal(rraw, &resp); err != nil {
			return EmitResult{}, fmt.Errorf("emit decode result: %w", err)
		}
		if !resp.Success {
			return EmitResult{}, fmt.Errorf("emit: %s", resp.Error)
		}
		var out EmitResult
		_ = json.Unmarshal(resp.Result, &out)
		return out, nil
	})
	// Egress verbs ride one generic frame round-trip: the cell is blocked
	// inside serveCell while the host resolves the action against its
	// search/HTTP/content deps under the egress budget.
	broker.SetEgress(func(ctx context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
		return egressRoundTrip(fc, scope, cfg.Epoch, action, payload)
	})
	ready, err := json.Marshal(SessionResult{ID: "ready"})
	if err != nil {
		os.Exit(2)
	}
	if err := fc.WriteFrame(StreamCell, ready); err != nil {
		os.Exit(2)
	}
	hooks := scope.BindCell()
	if err := RunSessionLoopFramedWithHooks(fc, func() (*Session, error) {
		return sess, nil
	}, broker.DrainReceipts, &hooks); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

// egressRoundTrip runs one host-mediated research verb through the session
// socket: wrap the action + raw payload in a BrokerRequest, write it on
// StreamBrokerEgress, wait for the host's StreamBrokerEgressResult, and hand
// back the raw JSON result. The cell is blocked inside serveCell for the
// whole round-trip — no concurrent frame traffic on this conn.
func egressRoundTrip(fc *FramedConn, scope *ChoirScope, epoch uint64, action BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
	req := BrokerRequest{ProtocolVersion: ProtocolVersion, RequestID: newReceiptID(),
		HandleRef: scope.HandleRef(), Epoch: epoch, Action: action, Payload: payload}
	enc, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("%s request marshal: %w", action, err)
	}
	if err := fc.WriteFrame(StreamBrokerEgress, enc); err != nil {
		return nil, fmt.Errorf("%s write frame: %w", action, err)
	}
	stream, rraw, err := fc.ReadFrame()
	if err != nil {
		return nil, fmt.Errorf("%s read result: %w", action, err)
	}
	if stream != StreamBrokerEgressResult {
		return nil, fmt.Errorf("%s: unexpected stream %d", action, stream)
	}
	var resp BrokerResponse
	if err := json.Unmarshal(rraw, &resp); err != nil {
		return nil, fmt.Errorf("%s decode result: %w", action, err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s: %s", action, resp.Error)
	}
	return resp.Result, nil
}
