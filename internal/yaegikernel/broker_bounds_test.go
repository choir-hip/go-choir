package yaegikernel

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Every verb's payload has a field-bound test (MA6.3): the value at the bound
// is accepted, one past it is refused, and the refusal reaches the cell as
// the daemon's admission answer, not a handler error.

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
		Action: ActionResearch, Payload: mustPayload("children", make([]any, MaxResearchChildren+1))}
	reason, forged := w.admitFrame(context.Background(), StreamBrokerEgress, req)
	if reason == "" || forged {
		t.Fatalf("reason=%q forged=%v, want a plain refusal", reason, forged)
	}
	if len(refused) != 1 || !strings.Contains(refused[0].Reason, fmt.Sprintf("over %d", MaxResearchChildren)) {
		t.Fatalf("refusal not recorded with the bound: %+v", refused)
	}
	req.Payload = mustPayload("children", []any{map[string]any{"objective": "o"}})
	if reason, _ := w.admitFrame(context.Background(), StreamBrokerEgress, req); reason != "" {
		t.Fatalf("an in-bound research call was refused: %s", reason)
	}
}

// wireActions reads every BrokerAction constant from broker_protocol.go, so
// an action added there without a place in the fuzz list, the payload table
// and the field accounting fails here.
func wireActions(t *testing.T) []BrokerAction {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "broker_protocol.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []BrokerAction
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs := spec.(*ast.ValueSpec)
			if id, ok := vs.Type.(*ast.Ident); !ok || id.Name != "BrokerAction" {
				continue
			}
			for _, v := range vs.Values {
				lit, ok := v.(*ast.BasicLit)
				if !ok {
					t.Fatalf("a BrokerAction constant is not a literal")
				}
				s, _ := strconv.Unquote(lit.Value)
				out = append(out, BrokerAction(s))
			}
		}
	}
	if len(out) < 20 {
		t.Fatalf("found only %d BrokerAction constants", len(out))
	}
	return out
}

func TestEveryWireActionIsFuzzedAndValidated(t *testing.T) {
	listed := map[BrokerAction]bool{}
	for _, a := range allActions {
		listed[a] = true
	}
	for _, a := range wireActions(t) {
		if !listed[a] {
			t.Errorf("%s is a wire action but not in the fuzz list (allActions)", a)
		}
		if _, ok := actionPayloads[a]; !ok {
			t.Errorf("%s has no payload struct in actionPayloads", a)
		}
		req := BrokerRequest{ProtocolVersion: ProtocolVersion, RequestID: "r", HandleRef: "h", Epoch: 1, Action: a}
		if err := req.Validate(); err != nil {
			t.Errorf("%s: %v", a, err)
		}
	}
}

// actionPayloads is each action's payload struct: the decode its handler
// runs.
var actionPayloads = map[BrokerAction]any{
	ActionExec: ExecPayload{}, ActionReadFile: ReadFilePayload{}, ActionWriteFile: WriteFilePayload{}, ActionListDir: ListDirPayload{},
	ActionSearchStart: SearchStartPayload{}, ActionRead: ReadPayload{}, ActionPollSource: PollSourcePayload{},
	ActionSourceAPI: SourceAPIPayload{}, ActionReadCapture: ReadCapturePayload{}, ActionSearchWireCorpus: SearchWireCorpusPayload{},
	ActionTextureSearch: TextureSearchPayload{}, ActionResearch: ResearchPayload{}, ActionSaveEvidence: SaveEvidencePayload{},
	ActionReadEvidence: ReadEvidencePayload{}, ActionListEvidence: ListEvidencePayload{},
	ActionGetRunMemoryEntry: GetRunMemoryEntryPayload{}, ActionProductAPI: ProductAPIPayload{}, ActionStories: StoriesPayload{}, ActionShotStart: ShotStartPayload{}, ActionShotWait: ShotWaitPayload{}, ActionScreens: ScreensPayload{}, ActionInstallPacket: InstallPacketPayload{},
	ActionSystem1: System1Payload{}, ActionRelateStories: RelateStoriesPayload{}, ActionRelate: RelatePayload{},
	ActionRequestLease: RequestLeasePayload{}, ActionObjectGet: ObjectGetPayload{}, ActionObjectList: ObjectListPayload{},
}

// boundElsewhere names the payload fields whose bound is not in the
// admission tables, and where it is. Every other field must be in
// payloadBounds or payloadListBounds. A field in neither fails
// TestEveryPayloadFieldIsBounded: "bound every field" (MA6.3).
var boundElsewhere = map[BrokerAction]map[string]string{
	ActionExec:          {"env": "structural only: no desk row serves exec on the host (FrameAllowed refuses it)"},
	ActionWriteFile:     {"content": "structural only: no desk row serves write_file on the host"},
	ActionResearch:      {"children": "admission: researchChildBounds per child; handler: ResearchChildrenBounds, then per child researchChildRoleOf and openChildCast"},
	ActionSaveEvidence:  {"content": "maxPayloadString (3 MiB): evidence text is the largest field", "metadata": "structural; labels in it are discarded and restamped (MA3.5)"},
	ActionProductAPI:    {"body": "handler: productAPIMaxBodyBytes"},
	ActionSourceAPI:     {"params": "structural; the gateway checks each provider's endpoint params"},
	ActionRequestLease:  {"req": "handler: the lease service validates the request fields (types.RequestFields)"},
	ActionSystem1:       {"questions": "handler: ValidateSystem1Questions (count, id, text, choices, refs, passages)"},
	ActionRelate:        {"story": "handler: ValidateRelate", "entities": "handler: ValidateRelate (count, name, role, type)"},
	ActionRelateStories: {"pairs": "handler: ValidateRelateStoriesPairs (count, refs, relation, reason)"},
}

func TestEveryPayloadFieldIsBounded(t *testing.T) {
	for action, payload := range actionPayloads {
		fields := map[string]bool{}
		rt := reflect.TypeOf(payload)
		for i := 0; i < rt.NumField(); i++ {
			name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
			if name == "" || name == "-" {
				t.Errorf("%s: field %s has no json name", action, rt.Field(i).Name)
				continue
			}
			fields[name] = true
			_, inBounds := payloadBounds[action][name]
			_, inLists := payloadListBounds[action][name]
			_, named := boundElsewhere[action][name]
			if !inBounds && !inLists && !named {
				t.Errorf("%s.%s has no bound: add it to payloadBounds or payloadListBounds, or name where it is bound in boundElsewhere", action, name)
			}
		}
		// A bound on a field the handler does not decode bounds nothing.
		for name := range payloadBounds[action] {
			if !fields[name] {
				t.Errorf("payloadBounds names %s.%s, which the payload struct does not have", action, name)
			}
		}
		for name := range payloadListBounds[action] {
			if !fields[name] {
				t.Errorf("payloadListBounds names %s.%s, which the payload struct does not have", action, name)
			}
		}
		for name := range boundElsewhere[action] {
			if !fields[name] {
				t.Errorf("boundElsewhere names %s.%s, which the payload struct does not have", action, name)
			}
		}
	}
}

// TestResearchChildFieldBounds: each child field at its bound passes, one
// past is refused at admission and by the handler's check; role, questions,
// p_success and weight are checked by the SDK's whole-call check and, per
// child, by the host (researchChildRoleOf, openChildCast).
func TestResearchChildFieldBounds(t *testing.T) {
	one := func(c ResearchChild) json.RawMessage {
		b, _ := json.Marshal(ResearchPayload{Children: []ResearchChild{c}})
		return b
	}
	for name, pair := range map[string][2]ResearchChild{
		"objective": {{Objective: strings.Repeat("o", MaxResearchObjectiveBytes)}, {Objective: strings.Repeat("o", MaxResearchObjectiveBytes+1)}},
		"model":     {{Objective: "o", Model: strings.Repeat("m", maxResearchModelLen)}, {Objective: "o", Model: strings.Repeat("m", maxResearchModelLen+1)}},
		"role":      {{Objective: "o", Role: "web"}, {Objective: "o", Role: strings.Repeat("r", maxResearchRoleLen+1)}},
	} {
		if err := ValidatePayload(ActionResearch, one(pair[0])); err != nil {
			t.Errorf("%s at its bound refused at admission: %v", name, err)
		}
		if err := ResearchChildrenBounds([]ResearchChild{pair[0]}); err != nil {
			t.Errorf("%s at its bound refused by the handler check: %v", name, err)
		}
		if err := ValidatePayload(ActionResearch, one(pair[1])); err == nil {
			t.Errorf("%s past its bound accepted at admission", name)
		}
		if err := ResearchChildrenBounds([]ResearchChild{pair[1]}); err == nil {
			t.Errorf("%s past its bound accepted by the handler check", name)
		}
	}
	two, half := 2.0, 0.5
	for name, c := range map[string]ResearchChild{
		"role":      {Objective: "o", Role: "admin"},
		"p_success": {Objective: "o", PSuccess: &two, Weight: &half},
		"weight":    {Objective: "o", PSuccess: &half, Weight: &two},
		"questions": {Objective: "o", Questions: []types.TypedQuestion{{Question: "q", Answers: []string{"only"}}}},
	} {
		if err := ValidateResearchChildren([]ResearchChild{c}); err == nil {
			t.Errorf("a child with a bad %s was accepted by the SDK check", name)
		}
	}
	if err := ResearchChildrenBounds(make([]ResearchChild, MaxResearchChildren+1)); err == nil {
		t.Error("too many children accepted")
	}
}
