package yaegikernel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Every verb's payload has a field-bound test (MA6.3): the value at the bound
// is accepted, one past it is refused, and the refusal reaches the cell as
// the daemon's admission answer, not a handler error.

func TestEveryRequestActionIsInTheFuzzList(t *testing.T) {
	listed := map[BrokerAction]bool{}
	for _, a := range allActions {
		listed[a] = true
	}
	for _, a := range allActions {
		req := BrokerRequest{ProtocolVersion: ProtocolVersion, RequestID: "r", HandleRef: "h", Epoch: 1, Action: a}
		if err := req.Validate(); err != nil {
			t.Errorf("%s: %v", a, err)
		}
	}
	// Validate's switch and the list agree: an action Validate knows is fuzzed.
	for _, a := range []BrokerAction{ActionExec, ActionReadFile, ActionWriteFile, ActionListDir, ActionSearchStart, ActionRead,
		ActionSearchWireCorpus, ActionSaveEvidence, ActionReadEvidence, ActionListEvidence, ActionGetRunMemoryEntry,
		ActionProductAPI, ActionResearch, ActionPollSource, ActionReadCapture, ActionStories, ActionSystem1, ActionRelateStories,
		ActionRelate, ActionSourceAPI, ActionRequestLease} {
		if !listed[a] {
			t.Errorf("%s is a wire action but not in the fuzz list", a)
		}
	}
}

func TestPayloadFieldBoundsPerAction(t *testing.T) {
	for action, fields := range payloadBounds {
		for name, b := range fields {
			var atBound, pastBound any
			if b.max == 0 && b.min == 0 {
				atBound, pastBound = strings.Repeat("a", b.maxLen), strings.Repeat("a", b.maxLen+1)
			} else {
				atBound, pastBound = b.max, b.max+1
				if err := ValidatePayload(action, mustPayload(name, b.min-1)); err == nil && b.min > 0 {
					t.Errorf("%s.%s below its minimum was accepted", action, name)
				}
			}
			if err := ValidatePayload(action, mustPayload(name, atBound)); err != nil {
				t.Errorf("%s.%s at its bound is refused: %v", action, name, err)
			}
			if err := ValidatePayload(action, mustPayload(name, pastBound)); err == nil {
				t.Errorf("%s.%s past its bound was accepted", action, name)
			}
		}
	}
	for action, lists := range payloadListBounds {
		for name, b := range lists {
			mk := func(n int) json.RawMessage {
				items := make([]any, n)
				for i := range items {
					items[i] = "x"
					if action == ActionResearch {
						items[i] = map[string]any{"objective": "o"}
					}
				}
				return mustPayload(name, items)
			}
			if err := ValidatePayload(action, mk(b.n)); err != nil {
				t.Errorf("%s.%s with %d entries is refused: %v", action, name, b.n, err)
			}
			if err := ValidatePayload(action, mk(b.n+1)); err == nil {
				t.Errorf("%s.%s with %d entries was accepted", action, name, b.n+1)
			}
			if b.elemLen > 0 {
				long := mustPayload(name, []any{strings.Repeat("a", b.elemLen+1)})
				if err := ValidatePayload(action, long); err == nil {
					t.Errorf("%s.%s with an entry past %d bytes was accepted", action, name, b.elemLen)
				}
			}
		}
	}
}

func mustPayload(name string, v any) json.RawMessage {
	b, _ := json.Marshal(map[string]any{name: v})
	return b
}

func TestPayloadStructuralBounds(t *testing.T) {
	deep := json.RawMessage(strings.Repeat("[", maxPayloadDepth+1) + strings.Repeat("]", maxPayloadDepth+1))
	if err := ValidatePayload(ActionSystem1, deep); err == nil {
		t.Error("a payload nested past the bound was accepted")
	}
	shallow := json.RawMessage(strings.Repeat("[", maxPayloadDepth) + strings.Repeat("]", maxPayloadDepth))
	if err := ValidatePayload(ActionSystem1, shallow); err != nil {
		t.Errorf("a payload at the depth bound is refused: %v", err)
	}
	wide := json.RawMessage("[" + strings.Repeat("1,", 2*maxPayloadItems) + "1]")
	if err := ValidatePayload(ActionRelate, wide); err == nil {
		t.Error("an array past the item bound was accepted")
	}
	huge := mustPayload("note", strings.Repeat("a", maxPayloadString+1))
	if err := ValidatePayload(ActionRelate, huge); err == nil {
		t.Error("a string past the bound was accepted")
	}
	if err := ValidatePayload(ActionRead, json.RawMessage(`{"url":`)); err == nil {
		t.Error("truncated JSON was accepted")
	}
	if err := ValidatePayload(ActionStories, nil); err != nil {
		t.Errorf("an empty payload is a verb with no arguments: %v", err)
	}
}

func TestRequestEnvelopeBounds(t *testing.T) {
	ok := BrokerRequest{ProtocolVersion: ProtocolVersion, RequestID: "r", HandleRef: "h", Epoch: 1, Action: ActionRead}
	if err := ok.ValidateBounds(); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*BrokerRequest){
		"request_id": func(r *BrokerRequest) { r.RequestID = strings.Repeat("a", maxRequestIDLen+1) },
		"handle_ref": func(r *BrokerRequest) { r.HandleRef = strings.Repeat("a", maxHandleRefLen+1) },
		"slot_token": func(r *BrokerRequest) { r.SlotToken = strings.Repeat("a", maxSlotTokenLen+1) },
		"timeout":    func(r *BrokerRequest) { r.TimeoutMs = maxTimeoutMs + 1 },
		"negative":   func(r *BrokerRequest) { r.TimeoutMs = -1 },
		"action":     func(r *BrokerRequest) { r.Action = BrokerAction(strings.Repeat("a", 65)) },
	} {
		r := ok
		mutate(&r)
		if err := r.ValidateBounds(); err == nil {
			t.Errorf("%s past its bound was accepted", name)
		}
	}
}

// TestAdmitFrameRefusesAnOutOfBoundPayload: the bound is part of admission,
// recorded as a refusal on the trace like any other.
func TestAdmitFrameRefusesAnOutOfBoundPayload(t *testing.T) {
	w := &DeskSessionWorker{slotToken: []byte("tok")}
	w.cfg.Session.Role = "research"
	w.cfg.Session.Epoch = 2
	var refused []FrameRefusal
	w.cfg.Refused = func(_ context.Context, r FrameRefusal) { refused = append(refused, r) }
	req := BrokerRequest{ProtocolVersion: ProtocolVersion, RequestID: "r", HandleRef: "h", Epoch: 2, SlotToken: "tok",
		Action: ActionResearch, Payload: mustPayload("children", make([]any, maxResearchKids+1))}
	reason, forged := w.admitFrame(context.Background(), StreamBrokerEgress, req)
	if reason == "" || forged {
		t.Fatalf("reason=%q forged=%v, want a plain refusal", reason, forged)
	}
	if len(refused) != 1 || !strings.Contains(refused[0].Reason, fmt.Sprintf("over %d", maxResearchKids)) {
		t.Fatalf("refusal not recorded with the bound: %+v", refused)
	}
	req.Payload = mustPayload("children", []any{map[string]any{"objective": "o"}})
	if reason, _ := w.admitFrame(context.Background(), StreamBrokerEgress, req); reason != "" {
		t.Fatalf("an in-bound research call was refused: %s", reason)
	}
}
