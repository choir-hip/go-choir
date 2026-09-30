package yaegikernel

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestDeskWorkerEgressRoundTrip drives the mid-cell egress path end to end
// over a real socketpair: the host EvalCell services a StreamBrokerEgress
// frame via the configured Egress handler, writes StreamBrokerEgressResult,
// and the cell's StreamCell result still completes the Eval. This is the
// choir.WebSearch/FetchURL boundary — the cell never holds a socket.
func TestDeskWorkerEgressRoundTrip(t *testing.T) {
	host, worker, err := SocketPair()
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	defer host.Close()
	defer worker.Close()

	var gotAction BrokerAction
	var gotPayload json.RawMessage
	w := &DeskSessionWorker{
		framed: NewFramedConn(host),
		cfg: DeskSessionWorkerConfig{Egress: func(ctx context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
			gotAction = action
			gotPayload = payload
			return json.Marshal(map[string]any{"results": []string{"a", "b"}})
		}},
	}

	evalDone := make(chan SessionResult, 1)
	evalErr := make(chan error, 1)
	go func() {
		res, err := w.EvalCell(context.Background(), "print(1)", nil, nil, nil, nil, nil)
		if err != nil {
			evalErr <- err
			return
		}
		evalDone <- res
	}()

	wfc := NewFramedConn(worker)
	if _, _, err := wfc.ReadFrame(); err != nil { // cell request
		t.Fatalf("worker read cell frame: %v", err)
	}
	payload, _ := json.Marshal(WebSearchPayload{Query: "choir", MaxResults: 2})
	req, _ := json.Marshal(BrokerRequest{ProtocolVersion: ProtocolVersion, RequestID: "e1", Action: ActionWebSearch, Payload: payload})
	if err := wfc.WriteFrame(StreamBrokerEgress, req); err != nil {
		t.Fatalf("worker write egress: %v", err)
	}
	stream, rraw, err := wfc.ReadFrame()
	if err != nil {
		t.Fatalf("worker read egress result: %v", err)
	}
	if stream != StreamBrokerEgressResult {
		t.Fatalf("egress result stream=%d want %d", stream, StreamBrokerEgressResult)
	}
	var resp BrokerResponse
	if err := json.Unmarshal(rraw, &resp); err != nil {
		t.Fatalf("decode egress result: %v", err)
	}
	if !resp.Success {
		t.Fatalf("egress refused: %s", resp.Error)
	}
	if gotAction != ActionWebSearch {
		t.Fatalf("host egress action=%q want web_search", gotAction)
	}
	var gotQuery WebSearchPayload
	_ = json.Unmarshal(gotPayload, &gotQuery)
	if gotQuery.Query != "choir" {
		t.Fatalf("host egress payload query=%q want choir", gotQuery.Query)
	}
	// Finish the cell.
	_ = wfc.WriteFrame(StreamCell, []byte(`{"id":"x"}`))
	select {
	case <-evalErr:
	case <-evalDone:
	case <-time.After(2 * time.Second):
		t.Fatal("EvalCell hung — egress frame was not serviced")
	}
}

// TestDeskWorkerEgressRefused proves a nil Egress handler answers the frame
// (not hangs), so the cell's egress verb returns a refusal rather than
// deadlocking the cell.
func TestDeskWorkerEgressRefused(t *testing.T) {
	host, worker, err := SocketPair()
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	defer host.Close()
	defer worker.Close()

	w := &DeskSessionWorker{framed: NewFramedConn(host), cfg: DeskSessionWorkerConfig{}}

	go func() {
		_, _ = w.EvalCell(context.Background(), "print(1)", nil, nil, nil, nil, nil)
	}()
	wfc := NewFramedConn(worker)
	if _, _, err := wfc.ReadFrame(); err != nil {
		t.Fatalf("worker read cell frame: %v", err)
	}
	payload, _ := json.Marshal(FetchURLPayload{URL: "https://x"})
	req, _ := json.Marshal(BrokerRequest{ProtocolVersion: ProtocolVersion, RequestID: "e2", Action: ActionFetchURL, Payload: payload})
	_ = wfc.WriteFrame(StreamBrokerEgress, req)
	stream, rraw, err := wfc.ReadFrame()
	if err != nil {
		t.Fatalf("worker read egress result: %v", err)
	}
	if stream != StreamBrokerEgressResult {
		t.Fatalf("egress result stream=%d want %d", stream, StreamBrokerEgressResult)
	}
	var resp BrokerResponse
	_ = json.Unmarshal(rraw, &resp)
	if resp.Success {
		t.Fatal("nil egress handler must refuse, got success")
	}
}
