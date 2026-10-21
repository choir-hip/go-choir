package yaegikernel

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// Emit mail is retired (choir.Emit refuses; protocol P2c): no desk's manifest
// row holds the emit action, so the host's emit handler is unreachable from a
// worker. This replaces the round trip that served an emit from any desk: it
// proves that, over a real socketpair, an emit frame is answered with a
// refusal (the cell does not hang) and the handler is never called, even when
// one is configured.
func TestDeskWorkerEmitIsRefusedNotServed(t *testing.T) {
	for _, role := range []string{"research", "management", "texture", "engineering", "research_child", "management_task", ""} {
		role := role
		t.Run("role="+role, func(t *testing.T) {
			host, worker, err := SocketPair()
			if err != nil {
				t.Fatalf("socketpair: %v", err)
			}
			defer host.Close()
			defer worker.Close()
			called := false
			w := &DeskSessionWorker{
				framed:    NewFramedConn(host),
				slotToken: []byte(testSlotToken),
				cfg: DeskSessionWorkerConfig{
					Session: SessionWorkerConfig{Role: role, Epoch: 1},
					Emit: func(ctx context.Context, p EmitPayload) (EmitResult, error) {
						called = true
						return EmitResult{Seq: 41}, nil
					},
				},
			}
			go func() { _, _ = w.EvalCell(context.Background(), "x", nil, nil, nil, nil, nil, nil) }()
			wfc := NewFramedConn(worker)
			if _, _, err := wfc.ReadFrame(); err != nil {
				t.Fatalf("worker read cell frame: %v", err)
			}
			payload, _ := json.Marshal(EmitPayload{ToDesk: "research", Kind: "k", Body: "b"})
			req, _ := json.Marshal(BrokerRequest{ProtocolVersion: ProtocolVersion, RequestID: "r2", Epoch: 1, SlotToken: testSlotToken, Action: ActionEmit, Payload: payload})
			_ = wfc.WriteFrame(StreamBrokerEmit, req)
			wfc.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			stream, rraw, err := wfc.ReadFrame()
			if err != nil {
				t.Fatalf("worker read emit result: %v", err)
			}
			if stream != StreamBrokerEmitResult {
				t.Fatalf("emit result stream=%d want %d", stream, StreamBrokerEmitResult)
			}
			var resp BrokerResponse
			_ = json.Unmarshal(rraw, &resp)
			if resp.Success || called {
				t.Fatalf("emit was served (success=%v handler called=%v)", resp.Success, called)
			}
		})
	}
}
