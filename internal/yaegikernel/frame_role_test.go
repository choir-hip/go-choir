package yaegikernel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// SEC-02 / ARCH-07 (docs/problems/desk-boundary-residuals-2026-10-21.md):
// the daemon decides what a worker may ask for from the worker it spawned
// (role, epoch, slot token), never from a field in the frame, and refuses
// every host-served action outside that desk's table.
//
// pinnedFrameActions is the authority the frame table must give each desk. It
// is written out here on purpose, independent of internal/deskmanifest: a
// manifest edit that widens a desk changes this table too, in review.
var pinnedFrameActions = map[string][]BrokerAction{
	"management": {ActionProductAPI, ActionReadCapture, ActionStories},
	"texture":    {ActionReadCapture},
	"research": {ActionSearchStart, ActionRead, ActionSearchWireCorpus, ActionSaveEvidence,
		ActionReadEvidence, ActionListEvidence, ActionGetRunMemoryEntry, ActionResearch,
		ActionPollSource, ActionReadCapture, ActionSystem1, ActionRelateStories, ActionRelate, ActionSourceAPI, ActionTextureSearch},
	SessionRoleResearchWeb: {ActionSearchStart, ActionRead, ActionReadCapture, ActionSearchWireCorpus,
		ActionReadEvidence, ActionListEvidence},
	SessionRoleResearchTexture: {ActionTextureSearch},
	SessionRoleManagementTask:  {ActionReadCapture, ActionStories},
	"engineering":              {},
}

var allBrokerActions = []BrokerAction{
	ActionExec, ActionReadFile, ActionWriteFile, ActionListDir,
	ActionSearchStart, ActionRead, ActionSearchWireCorpus, ActionSaveEvidence, ActionReadEvidence,
	ActionListEvidence, ActionGetRunMemoryEntry, ActionProductAPI, ActionResearch, ActionPollSource,
	ActionReadCapture, ActionStories, ActionSystem1, ActionRelateStories, ActionRelate, ActionSourceAPI, ActionRequestLease, ActionObjectGet, ActionObjectList,
	ActionTextureSearch,
}

const testSlotToken = "slot-token-0123456789abcdef0123456789abcdef"

// frameHarness is a host DeskSessionWorker on one end of a real socketpair and
// a hand-driven "worker" on the other, with a cell in flight so the host's
// reader loop is servicing frames.
type frameHarness struct {
	t        *testing.T
	w        *DeskSessionWorker
	wfc      *FramedConn
	egress   atomic.Int32
	mu       sync.Mutex
	refusals []FrameRefusal
	evalDone chan error
}

func newFrameHarness(t *testing.T, role string) *frameHarness {
	t.Helper()
	host, worker, err := SocketPair()
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	t.Cleanup(func() { host.Close(); worker.Close() })
	h := &frameHarness{t: t, wfc: NewFramedConn(worker), evalDone: make(chan error, 1)}
	h.w = &DeskSessionWorker{
		framed:    NewFramedConn(host),
		slotToken: []byte(testSlotToken),
		cfg: DeskSessionWorkerConfig{
			Session: SessionWorkerConfig{Role: role, Epoch: 1},
			Egress: func(ctx context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
				h.egress.Add(1)
				return json.RawMessage(`{"ok":true}`), nil
			},
			Refused: func(ctx context.Context, r FrameRefusal) {
				h.mu.Lock()
				h.refusals = append(h.refusals, r)
				h.mu.Unlock()
			},
		},
	}
	go func() {
		_, err := h.w.EvalCell(context.Background(), "print(1)", nil, nil, nil, nil, nil)
		h.evalDone <- err
	}()
	if _, _, err := h.wfc.ReadFrame(); err != nil { // the cell request
		t.Fatalf("worker read cell frame: %v", err)
	}
	return h
}

func (h *frameHarness) request(action BrokerAction, mutate func(*BrokerRequest)) BrokerRequest {
	req := BrokerRequest{ProtocolVersion: ProtocolVersion, RequestID: "r-" + string(action), HandleRef: "hnd_x.y", Epoch: 1,
		Action: action, Payload: json.RawMessage(`{}`), SlotToken: testSlotToken}
	if mutate != nil {
		mutate(&req)
	}
	return req
}

// send writes one request on stream and returns the host's answer.
func (h *frameHarness) send(stream byte, req BrokerRequest) BrokerResponse {
	h.t.Helper()
	raw, _ := json.Marshal(req)
	if err := h.wfc.WriteFrame(stream, raw); err != nil {
		h.t.Fatalf("worker write: %v", err)
	}
	h.wfc.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	got, payload, err := h.wfc.ReadFrame()
	if err != nil {
		h.t.Fatalf("worker read answer for %s: %v", req.Action, err)
	}
	if got != stream+1 {
		h.t.Fatalf("answer stream=%d want %d", got, stream+1)
	}
	var resp BrokerResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		h.t.Fatalf("decode answer: %v", err)
	}
	return resp
}

func (h *frameHarness) refusalCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.refusals)
}

func allowed(role string, action BrokerAction) bool {
	for _, a := range pinnedFrameActions[role] {
		if a == action {
			return true
		}
	}
	return false
}

// A frame naming an action outside the sending desk's table is refused with a
// visible error, the handler never runs, and the refusal is reported. Every
// action in the table is served. Failure mode 1.
func TestFrameRoleTableEveryDeskEveryAction(t *testing.T) {
	for role := range pinnedFrameActions {
		for _, action := range allBrokerActions {
			role, action := role, action
			t.Run(role+"/"+string(action), func(t *testing.T) {
				h := newFrameHarness(t, role)
				resp := h.send(StreamBrokerEgress, h.request(action, nil))
				if allowed(role, action) {
					if !resp.Success || h.egress.Load() != 1 {
						t.Fatalf("%s/%s must be served: success=%v err=%q calls=%d", role, action, resp.Success, resp.Error, h.egress.Load())
					}
					return
				}
				if resp.Success || h.egress.Load() != 0 {
					t.Fatalf("%s/%s outside the table was served (success=%v calls=%d)", role, action, resp.Success, h.egress.Load())
				}
				if !strings.Contains(resp.Error, "refused") || !strings.Contains(resp.Error, role) || !strings.Contains(resp.Error, string(action)) {
					t.Fatalf("refusal must name the desk and the action, got %q", resp.Error)
				}
				if h.refusalCount() != 1 {
					t.Fatalf("refusal not reported to the host: %d", h.refusalCount())
				}
			})
		}
	}
}

// An empty role, an unknown role and an app-looking name the manifest does not
// know serve nothing. Failure modes 7 and M2.
func TestFrameRoleUnknownDeskServesNothing(t *testing.T) {
	for _, role := range []string{"", "root", "app:never-registered", "Research", "research ", "conductor"} {
		role := role
		t.Run("role="+role, func(t *testing.T) {
			h := newFrameHarness(t, role)
			for _, action := range []BrokerAction{ActionRead, ActionReadCapture, ActionProductAPI, ActionSearchStart} {
				if resp := h.send(StreamBrokerEgress, h.request(action, nil)); resp.Success {
					t.Fatalf("unknown desk %q was served %s", role, action)
				}
			}
			if h.egress.Load() != 0 {
				t.Fatalf("handler ran %d times for an unknown desk", h.egress.Load())
			}
		})
	}
}

// A texture worker whose frame claims management in every field it has gets
// texture's table. The daemon trusts the worker it spawned, not the frame.
// Failure mode 2.
func TestForgedFrameClaimsAnotherDesk(t *testing.T) {
	h := newFrameHarness(t, "texture")
	forged := h.request(ActionProductAPI, func(r *BrokerRequest) {
		r.Payload = json.RawMessage(`{"role":"management","desk":"management","actor_profile":"management","method":"POST","path":"/api/engineering/assignments"}`)
		r.HandleRef = "hnd_" + "eyJhY3Rvcl9wcm9maWxlIjoibWFuYWdlbWVudCJ9" + ".c2ln"
	})
	resp := h.send(StreamBrokerEgress, forged)
	if resp.Success || h.egress.Load() != 0 {
		t.Fatalf("texture reached product_api by claiming management: success=%v calls=%d", resp.Success, h.egress.Load())
	}
	// The same frame is fine for the desk that really is management.
	hm := newFrameHarness(t, "management")
	if resp := hm.send(StreamBrokerEgress, hm.request(ActionProductAPI, nil)); !resp.Success {
		t.Fatalf("management refused its own product_api: %q", resp.Error)
	}
}

// A research child is served its own role's narrower table even though its
// parent is research. Failure mode 6.
//
// MA4.6 (plan 8.6): the two child roles split the research desk's grants. The
// web child tries the Texture child's texture_search and the Texture child
// tries every world.read action and the rest of research's table; each is
// refused at the frame check, named, and reported, and no handler runs.
func TestFrameRoleResearchChildrenAreNarrowerAndDisjoint(t *testing.T) {
	refused := map[string][]BrokerAction{
		SessionRoleResearchWeb: {ActionTextureSearch, ActionSaveEvidence, ActionPollSource, ActionSourceAPI, ActionSystem1,
			ActionRelateStories, ActionRelate, ActionResearch, ActionGetRunMemoryEntry},
		SessionRoleResearchTexture: {ActionSearchStart, ActionRead, ActionSearchWireCorpus, ActionReadCapture, ActionPollSource,
			ActionSourceAPI, ActionReadEvidence, ActionListEvidence, ActionSaveEvidence, ActionResearch, ActionSystem1,
			ActionRelateStories, ActionRelate, ActionGetRunMemoryEntry, ActionProductAPI},
	}
	for role, actions := range refused {
		h := newFrameHarness(t, role)
		for _, action := range actions {
			resp := h.send(StreamBrokerEgress, h.request(action, nil))
			if resp.Success {
				t.Fatalf("%s was served %s", role, action)
			}
			if !strings.Contains(resp.Error, "refused") || !strings.Contains(resp.Error, role) || !strings.Contains(resp.Error, string(action)) {
				t.Fatalf("%s/%s: the refusal must name the desk and the action, got %q", role, action, resp.Error)
			}
		}
		if h.egress.Load() != 0 {
			t.Fatalf("handler ran for a %s action outside its table", role)
		}
		if h.refusalCount() != len(actions) {
			t.Fatalf("%s: %d refusals reported, want %d", role, h.refusalCount(), len(actions))
		}
	}
	// Each is served its own grant.
	for role, action := range map[string]BrokerAction{SessionRoleResearchWeb: ActionSearchWireCorpus, SessionRoleResearchTexture: ActionTextureSearch} {
		h := newFrameHarness(t, role)
		if resp := h.send(StreamBrokerEgress, h.request(action, nil)); !resp.Success {
			t.Fatalf("%s refused its own %s: %q", role, action, resp.Error)
		}
	}
}

// A wrong or missing slot token is a forged frame: refused, reported, and the
// worker is ended. A token from another worker is the same thing. Failure
// mode 3.
func TestSlotTokenWrongMissingOrReplayed(t *testing.T) {
	cases := map[string]string{
		"wrong":         "not-the-token",
		"missing":       "",
		"other-workers": "slot-token-ffffffffffffffffffffffffffffffff",
		"prefix":        testSlotToken[:10],
		"suffix-extra":  testSlotToken + "x",
	}
	for name, token := range cases {
		name, token := name, token
		t.Run(name, func(t *testing.T) {
			h := newFrameHarness(t, "research")
			resp := h.send(StreamBrokerEgress, h.request(ActionRead, func(r *BrokerRequest) { r.SlotToken = token }))
			if resp.Success || h.egress.Load() != 0 {
				t.Fatalf("frame with %s token was served", name)
			}
			if !strings.Contains(resp.Error, "slot token") {
				t.Fatalf("refusal must say the token is wrong, got %q", resp.Error)
			}
			select {
			case err := <-h.evalDone:
				if err == nil {
					t.Fatal("a forged frame must end the cell with an error")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("a forged frame did not end the worker")
			}
			if !h.w.Dead() {
				t.Fatal("worker survived a forged frame")
			}
			if h.refusalCount() != 1 {
				t.Fatalf("forged frame not reported: %d", h.refusalCount())
			}
		})
	}
}

// A frame whose epoch is not the worker's is refused. A frame whose protocol
// version is not 1 is refused.
func TestFrameEpochAndProtocolVersion(t *testing.T) {
	h := newFrameHarness(t, "research")
	if resp := h.send(StreamBrokerEgress, h.request(ActionRead, func(r *BrokerRequest) { r.Epoch = 9 })); resp.Success {
		t.Fatal("stale epoch served")
	}
	if resp := h.send(StreamBrokerEgress, h.request(ActionRead, func(r *BrokerRequest) { r.ProtocolVersion = ProtocolVersion + 1 })); resp.Success {
		t.Fatal("unknown protocol version served")
	}
	if h.egress.Load() != 0 {
		t.Fatalf("handler ran for a bad frame")
	}
	// The worker is still usable after a refused (not forged) frame.
	if resp := h.send(StreamBrokerEgress, h.request(ActionRead, nil)); !resp.Success {
		t.Fatalf("a refusal must not poison the worker: %q", resp.Error)
	}
}

// TestBadWireHeaderRecordsATransportRefusal: a worker that sends an
// unsupported version or a nonzero reserved byte is refused from eight
// bytes (transport), and the refusal reaches the trace hook before the
// worker ends; it needs no decoded action or request id.
func TestBadWireHeaderRecordsATransportRefusal(t *testing.T) {
	for name, hdr := range map[string][]byte{
		"version-2":  rawHeader(0, StreamBrokerEgress, 2, 0, 0),
		"reserved-1": rawHeader(0, StreamBrokerEgress, FrameVersion, 1, 0),
		"reserved-2": rawHeader(0, StreamBrokerEgress, FrameVersion, 0, 7),
	} {
		hdr := hdr
		t.Run(name, func(t *testing.T) {
			h := newFrameHarness(t, "research")
			if _, err := h.wfc.conn.Write(hdr); err != nil {
				t.Fatalf("worker write: %v", err)
			}
			select {
			case err := <-h.evalDone:
				if !errors.Is(err, ErrFrameVersion) {
					t.Fatalf("EvalCell err = %v, want ErrFrameVersion", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("EvalCell did not end on a bad wire header")
			}
			h.mu.Lock()
			defer h.mu.Unlock()
			if len(h.refusals) != 1 {
				t.Fatalf("want one transport refusal before the worker ends, got %d", len(h.refusals))
			}
			r := h.refusals[0]
			if r.Role != "research" || r.Action != ActionTransport || r.Reason == "" || r.Forged {
				t.Errorf("refusal = %+v, want research / transport / a reason / not forged", r)
			}
		})
	}
}
