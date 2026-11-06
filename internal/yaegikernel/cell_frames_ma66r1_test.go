package yaegikernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// MA6.6 review r1, the frame loop. A host-side DeskSessionWorker on one end
// of a pipe; the test plays the worker on the other, so it can send any frame
// in any order. Failure modes, written first:
//
//	S2 a handler admitted before a forged frame acts after the forged frame
//	   is traced (and the breaker trips), or acts for an aborted cell;
//	S3 refused frames do not count toward MaxFrameCallsPerCell, or every
//	   refusal is traced, so a cell can write unbounded refusal events;
//	D1 a disclosure sink is served while an earlier read is still tainting;
//	D2 two goroutines in one cell stage two Completes.

type cellFrameHarness struct {
	t       *testing.T
	w       *DeskSessionWorker
	peer    *FramedConn
	results chan BrokerResponse
	done    chan error
	cellID  string
}

func newCellFrameHarness(t *testing.T, role string, egress func(context.Context, BrokerAction, json.RawMessage) (json.RawMessage, error), refused func(FrameRefusal)) *cellFrameHarness {
	t.Helper()
	host, wk := net.Pipe()
	w := &DeskSessionWorker{framed: NewFramedConn(host), slotToken: []byte("tok")}
	w.cfg.Session.Role = role
	w.cfg.Session.Epoch = 2
	w.cfg.Egress = egress
	w.cfg.Refused = func(_ context.Context, r FrameRefusal) {
		if refused != nil {
			refused(r)
		}
	}
	h := &cellFrameHarness{t: t, w: w, peer: NewFramedConn(wk), results: make(chan BrokerResponse, 4096), done: make(chan error, 1)}
	t.Cleanup(func() { _ = h.peer.Close(); _ = w.framed.Close() })
	go func() {
		_, err := w.EvalFrame(context.Background(), SessionFrame{Source: "x"})
		h.done <- err
	}()
	stream, raw, err := h.peer.ReadFrame()
	if err != nil || stream != StreamCell {
		t.Fatalf("cell frame: %d %v", stream, err)
	}
	var frame SessionFrame
	_ = json.Unmarshal(raw, &frame)
	go func() {
		for {
			stream, raw, err := h.peer.ReadFrame()
			if err != nil {
				return
			}
			if stream == StreamBrokerEgressResult {
				var r BrokerResponse
				_ = json.Unmarshal(raw, &r)
				h.results <- r
			}
		}
	}()
	h.cellID = frame.ID
	return h
}

func (h *cellFrameHarness) send(id string, action BrokerAction, token string) {
	h.t.Helper()
	req := BrokerRequest{ProtocolVersion: ProtocolVersion, RequestID: id, HandleRef: "h", Epoch: 2, SlotToken: token,
		Action: action, Payload: json.RawMessage(`{}`)}
	raw, _ := json.Marshal(req)
	if err := h.peer.WriteFrame(StreamBrokerEgress, raw); err != nil {
		h.t.Fatalf("send %s: %v", id, err)
	}
}

func (h *cellFrameHarness) finish() error {
	h.t.Helper()
	raw, _ := json.Marshal(SessionResult{ID: h.cellID})
	if err := h.peer.WriteFrame(StreamCell, raw); err != nil {
		h.t.Fatalf("cell result: %v", err)
	}
	return h.wait()
}

func (h *cellFrameHarness) wait() error {
	h.t.Helper()
	select {
	case err := <-h.done: // hang-guard: bounded by the time.After case below
		return err
	case <-time.After(30 * time.Second):
		h.t.Fatal("EvalFrame did not return")
		return nil
	}
}

func TestMA66r1ForgedFrameStopsHandlersBeforeItIsTraced(t *testing.T) {
	var mu sync.Mutex
	var order []string
	note := func(s string) { mu.Lock(); order = append(order, s); mu.Unlock() }
	started := make(chan struct{})
	h := newCellFrameHarness(t, "research", func(ctx context.Context, a BrokerAction, _ json.RawMessage) (json.RawMessage, error) {
		close(started)
		<-ctx.Done() // a slow grant: it can only end by the cell's abort
		note("handler saw the abort")
		return nil, ctx.Err()
	}, func(r FrameRefusal) {
		if r.Forged {
			note("forged frame traced")
		}
	})
	h.send("r1", ActionListEvidence, "tok")
	<-started
	h.send("r2", ActionListEvidence, "forged")
	if err := h.wait(); !errors.Is(err, errForgedFrame) {
		t.Fatalf("EvalFrame: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(order) != 2 || order[0] != "handler saw the abort" || order[1] != "forged frame traced" {
		t.Fatalf("order = %v; the in-flight handler must stop before the forged frame is traced", order)
	}
}

func TestMA66r1HostKillCancelsTheCellsHandlers(t *testing.T) {
	cancelled := make(chan struct{})
	started := make(chan struct{})
	h := newCellFrameHarness(t, "research", func(ctx context.Context, a BrokerAction, _ json.RawMessage) (json.RawMessage, error) {
		close(started)
		<-ctx.Done()
		close(cancelled)
		return nil, ctx.Err()
	}, nil)
	h.send("r1", ActionListEvidence, "tok")
	<-started
	// The cell's end: a read error when the worker's side goes away.
	_ = h.peer.Close()
	select {
	case <-cancelled:
	case <-time.After(10 * time.Second):
		t.Fatal("the handler of an aborted cell was not cancelled")
	}
	_ = h.wait()
}

func TestMA66r1RefusedFramesCountAndAreTracedBoundedly(t *testing.T) {
	var traced atomic.Int32
	var served atomic.Int32
	h := newCellFrameHarness(t, "research", func(context.Context, BrokerAction, json.RawMessage) (json.RawMessage, error) {
		served.Add(1)
		return json.RawMessage(`{}`), nil
	}, func(FrameRefusal) { traced.Add(1) })
	// research may not call product_api: every one is refused.
	for i := 0; i < MaxFrameCallsPerCell; i++ {
		h.send(fmt.Sprintf("x%d", i), ActionProductAPI, "tok")
	}
	h.send("last", ActionListEvidence, "tok")
	var last BrokerResponse
	for i := 0; i <= MaxFrameCallsPerCell; i++ {
		r := <-h.results
		if r.RequestID == "last" {
			last = r
		}
	}
	if err := h.finish(); err != nil {
		t.Fatal(err)
	}
	if served.Load() != 0 || last.Success || !strings.Contains(last.Error, "host calls") {
		t.Fatalf("a call past the bound of refused frames was served: served %d, %+v", served.Load(), last)
	}
	if n := traced.Load(); n != maxTracedRefusalsPerCell+1 {
		t.Fatalf("traced %d refusals, want %d", n, maxTracedRefusalsPerCell+1)
	}
}

// D1: the gate step of a sink reads the run's labels; every read admitted
// before it has tainted them first. The handlers model it: a read adds its
// label after a delay, the sink records the labels it was gated on.
func TestMA66r1ASinkIsGatedAfterEarlierReadsTaint(t *testing.T) {
	var mu sync.Mutex
	labels := map[string]bool{}
	var gatedOn []string
	h := newCellFrameHarness(t, "research", func(_ context.Context, a BrokerAction, _ json.RawMessage) (json.RawMessage, error) {
		switch a {
		case ActionReadCapture:
			time.Sleep(100 * time.Millisecond)
			mu.Lock()
			labels["owner_private:capture"] = true
			mu.Unlock()
		case ActionRead:
			mu.Lock()
			for l := range labels {
				gatedOn = append(gatedOn, l)
			}
			mu.Unlock()
		}
		return json.RawMessage(`{}`), nil
	}, nil)
	h.send("read", ActionReadCapture, "tok")
	h.send("sink", ActionRead, "tok")
	<-h.results
	<-h.results
	if err := h.finish(); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(gatedOn) != 1 || gatedOn[0] != "owner_private:capture" {
		t.Fatalf("the sink was gated on %v, before the earlier read tainted the run", gatedOn)
	}
}

// D2: concurrent staging keeps the per-kind rules.
func TestMA66r1ConcurrentCompletesStageOne(t *testing.T) {
	var tray Tray
	var ok atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tray.Complete(CompleteCompleted, "none", "s", nil, nil) == nil {
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 1 || len(tray.Close()) != 1 {
		t.Fatalf("%d Completes staged", ok.Load())
	}
	if _, err := tray.Cast("management", "late", "", nil); err == nil {
		t.Fatal("a closed tray staged")
	}
}

// D2 in a real worker: two goroutines of one cell call choir.Complete; the
// cell ends with one Complete staged.
func TestMA66r1TwoGoroutinesStageOneCompleteInACell(t *testing.T) {
	w := ma66Worker(t, "engineering", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := w.Eval(ctx, `import "sync"
var wg sync.WaitGroup
errs := make([]error, 2)
for i := 0; i < 2; i++ {
	i := i
	wg.Add(1)
	go func() { defer wg.Done(); errs[i] = choir.Complete("completed", "none", "done", nil, nil) }()
}
wg.Wait()
n := 0
for _, e := range errs { if e == nil { n++ } }
fmt.Println("staged", n)`)
	if err != nil {
		t.Fatal(err)
	}
	complete := 0
	for _, in := range res.Intents {
		if in.Kind == IntentComplete {
			complete++
		}
	}
	if !strings.Contains(res.Stdout, "staged 1") || complete != 1 {
		t.Fatalf("stdout %q error %q, %d Completes staged", res.Stdout, res.Error, complete)
	}
}
