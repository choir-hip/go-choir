package yaegikernel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Bounds on a broker request (MA6.3). The frame limit (MaxFramePayload,
// 4 MiB) bounds bytes; these bound shape. A cell is hostile input: every
// string, count, number and depth a host handler sees has a limit the daemon
// checks before any handler decodes or acts (admitFrame). The limits are
// generous for real calls and small enough that no field makes a handler
// allocate or loop without bound.
const (
	maxRequestIDLen     = 128
	maxHandleRefLen     = 2048
	maxSlotTokenLen     = 128
	maxTimeoutMs        = int64(60 * 60 * 1000)
	maxPayloadDepth     = 32
	maxPayloadItems     = 4096    // elements of one array, keys of one object
	maxPayloadTokens    = 200_000 // tokens in one payload
	maxPayloadString    = 3 << 20 // one string value (save_evidence content is the largest field)
	MaxResearchChildren = 16
	maxLeaseListItems   = 256
	maxLeaseDuration    = 31 * 24 * 60 * 60 // seconds

	// One research child's fields (MA6.3): the objective becomes a child's
	// prompt, the model a catalog name, the role one of two words.
	MaxResearchObjectiveBytes = 32 << 10
	maxResearchModelLen       = 256
	maxResearchRoleLen        = 32
)

// payloadFieldBound limits one top-level field of an action's payload: a
// string's length in bytes, or a number's range.
type payloadFieldBound struct {
	maxLen   int     // strings; 0 means no tighter limit than maxPayloadString
	min, max float64 // numbers; max 0 means not a number field
}

func strMax(n int) payloadFieldBound { return payloadFieldBound{maxLen: n} }
func numRange(lo, hi float64) payloadFieldBound {
	return payloadFieldBound{min: lo, max: hi}
}

// payloadBounds are the per-action limits on top-level fields. Ids and
// references are short; text is bounded by maxPayloadString. An action not in
// the table still gets the structural bounds.
var payloadBounds = map[BrokerAction]map[string]payloadFieldBound{
	ActionRead:              {"url": strMax(8192)},
	ActionSearchStart:       {"query": strMax(8192)},
	ActionSearchWireCorpus:  {"query": strMax(8192), "limit": numRange(0, 1000)},
	ActionTextureSearch:     {"query": strMax(8192), "limit": numRange(0, 1000)},
	ActionReadCapture:       {"capture_id": strMax(256), "from_block": numRange(0, 1_000_000), "max_chars": numRange(0, 1_000_000)},
	ActionPollSource:        {"source_id": strMax(256), "max_items": numRange(0, 1000)},
	ActionSourceAPI:         {"provider": strMax(128), "endpoint": strMax(256)},
	ActionReadEvidence:      {"evidence_id": strMax(256)},
	ActionListEvidence:      {"agent_id": strMax(256), "limit": numRange(0, 1000)},
	ActionGetRunMemoryEntry: {"entry_id": strMax(256)},
	ActionSaveEvidence:      {"kind": strMax(128), "source_uri": strMax(8192), "title": strMax(4096)},
	ActionStories:           {"limit": numRange(0, 500), "before": strMax(64)},
	ActionShotStart:         {"operation_id": strMax(128)},
	ActionScreens:           {"operation_id": strMax(128)},
	ActionInstallPacket:     {"release_id": strMax(128)},
	ActionShotWait:          {"job_id": strMax(128), "max_seconds": numRange(0, 240)},
	ActionProductAPI:        {"method": strMax(16), "path": strMax(4096)},
	ActionRequestLease:      {"purpose": strMax(4096), "duration_seconds": numRange(0, maxLeaseDuration)},
	ActionReadMail:          {"ref": strMax(256)},
	ActionObjectGet:         {"kind": strMax(128), "id": strMax(128)},
	ActionObjectList:        {"kind": strMax(128), "limit": numRange(0, 200)},
	ActionReadFile:          {"path": strMax(4096)},
	ActionWriteFile:         {"path": strMax(4096), "mode": numRange(0, 0o7777)},
	ActionListDir:           {"path": strMax(4096)},
	ActionExec:              {"command": strMax(4096), "dir": strMax(4096)},
}

// payloadListBounds limit the number of elements of a top-level array field
// and the length of each string element.
var payloadListBounds = map[BrokerAction]map[string]struct{ n, elemLen int }{
	ActionRequestLease: {
		"grants":         {maxLeaseListItems, 128},
		"refs":           {maxLeaseListItems, 256},
		"kinds":          {maxLeaseListItems, 128},
		"capture_ids":    {maxLeaseListItems, 256},
		"capture_labels": {maxLeaseListItems, 256},
	},
	ActionResearch: {"children": {MaxResearchChildren, 0}},
	ActionExec:     {"args": {1024, 8192}},
}

// ValidateBounds checks the shape of a request's own fields. It does not
// check who the request is from (admitFrame) or that the protocol version and
// action are known (Validate): it is the length and range half, safe to run on
// anything a worker wrote.
func (r *BrokerRequest) ValidateBounds() error {
	switch {
	case len(r.RequestID) > maxRequestIDLen:
		return fmt.Errorf("broker protocol: request_id is %d bytes, over %d", len(r.RequestID), maxRequestIDLen)
	case len(r.HandleRef) > maxHandleRefLen:
		return fmt.Errorf("broker protocol: handle_ref is %d bytes, over %d", len(r.HandleRef), maxHandleRefLen)
	case len(r.SlotToken) > maxSlotTokenLen:
		return fmt.Errorf("broker protocol: slot_token is %d bytes, over %d", len(r.SlotToken), maxSlotTokenLen)
	case r.TimeoutMs < 0 || r.TimeoutMs > maxTimeoutMs:
		return fmt.Errorf("broker protocol: timeout_ms %d is outside 0..%d", r.TimeoutMs, maxTimeoutMs)
	case len(r.Action) > 64:
		return fmt.Errorf("broker protocol: action is %d bytes", len(r.Action))
	}
	return ValidatePayload(r.Action, r.Payload)
}

// ResearchChildrenBounds is the host's length check on a research call
// (MA6.3): the count and each child's objective, role and model lengths. The
// handler runs it before any child starts; each child's role, questions,
// p_success and weight are still judged per child (an unknown role refuses
// that child, never its siblings).
func ResearchChildrenBounds(children []ResearchChild) error {
	if len(children) > MaxResearchChildren {
		return fmt.Errorf("choir.Research: %d children, at most %d per call", len(children), MaxResearchChildren)
	}
	for i, spec := range children {
		if err := researchChildBounds(spec); err != nil {
			return fmt.Errorf("choir.Research child %d: %w", i+1, err)
		}
	}
	return nil
}

// ValidateResearchChildren is the cell SDK's whole-call check: the bounds,
// then each child's role, typed questions, p_success and weight.
func ValidateResearchChildren(children []ResearchChild) error {
	if err := ResearchChildrenBounds(children); err != nil {
		return err
	}
	for i, spec := range children {
		switch strings.TrimSpace(spec.Role) {
		case "", ResearchChildWeb, ResearchChildTexture:
		default:
			return fmt.Errorf("choir: Research child %d: role %q is not %q or %q", i+1, spec.Role, ResearchChildWeb, ResearchChildTexture)
		}
		if err := types.ValidateTypedQuestions(spec.Questions); err != nil {
			return fmt.Errorf("choir: Research child %d: questions: %w", i+1, err)
		}
		for _, f := range []struct {
			name string
			v    *float64
		}{{"p_success", spec.PSuccess}, {"weight", spec.Weight}} {
			if f.v != nil && !(*f.v >= 0 && *f.v <= 1) {
				return fmt.Errorf("choir: Research child %d: %s %v is outside [0,1]", i+1, f.name, *f.v)
			}
		}
	}
	return nil
}

// researchChildBounds is the length half of ValidateResearchChildren, which
// admission runs on every research frame before any handler decodes it.
func researchChildBounds(spec ResearchChild) error {
	switch {
	case len(spec.Objective) > MaxResearchObjectiveBytes:
		return fmt.Errorf("objective is %d bytes, over %d", len(spec.Objective), MaxResearchObjectiveBytes)
	case len(spec.Model) > maxResearchModelLen:
		return fmt.Errorf("model is %d bytes, over %d", len(spec.Model), maxResearchModelLen)
	case len(spec.Role) > maxResearchRoleLen:
		return fmt.Errorf("role is %d bytes, over %d", len(spec.Role), maxResearchRoleLen)
	}
	return nil
}

// ValidatePayload checks one action's payload before any handler decodes it:
// valid JSON of bounded depth, size and token count, no string over
// maxPayloadString, and the action's field limits. An empty payload is valid
// (a verb with no arguments).
func ValidatePayload(action BrokerAction, raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if len(raw) > MaxFramePayload {
		return fmt.Errorf("broker protocol: %s payload is %d bytes, over %d", action, len(raw), MaxFramePayload)
	}
	if err := boundJSON(raw); err != nil {
		return fmt.Errorf("broker protocol: %s payload: %w", action, err)
	}
	if action == ActionResearch {
		// The children are objects: their fields are bounded here, the
		// semantics (role, questions) by the handler (ValidateResearchChildren).
		var req ResearchPayload
		if json.Unmarshal(raw, &req) == nil {
			for i, spec := range req.Children {
				if i >= MaxResearchChildren {
					break // the list bound below names the count
				}
				if err := researchChildBounds(spec); err != nil {
					return fmt.Errorf("broker protocol: research child %d: %w", i+1, err)
				}
			}
		}
	}
	bounds, lists := payloadBounds[action], payloadListBounds[action]
	if len(bounds) == 0 && len(lists) == 0 {
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return fmt.Errorf("broker protocol: %s payload is not an object: %w", action, err)
	}
	for name, b := range bounds {
		v, ok := fields[name]
		if !ok {
			continue
		}
		if b.max == 0 && b.min == 0 {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				continue // a type mismatch is the handler's decode error
			}
			if len(s) > b.maxLen {
				return fmt.Errorf("broker protocol: %s.%s is %d bytes, over %d", action, name, len(s), b.maxLen)
			}
			continue
		}
		var n float64
		if err := json.Unmarshal(v, &n); err != nil {
			continue
		}
		if n < b.min || n > b.max {
			return fmt.Errorf("broker protocol: %s.%s is %v, outside %v..%v", action, name, n, b.min, b.max)
		}
	}
	for name, b := range lists {
		v, ok := fields[name]
		if !ok {
			continue
		}
		var items []json.RawMessage
		if err := json.Unmarshal(v, &items); err != nil {
			continue
		}
		if len(items) > b.n {
			return fmt.Errorf("broker protocol: %s.%s has %d entries, over %d", action, name, len(items), b.n)
		}
		if b.elemLen > 0 {
			for _, it := range items {
				var s string
				if json.Unmarshal(it, &s) == nil && len(s) > b.elemLen {
					return fmt.Errorf("broker protocol: %s.%s has an entry of %d bytes, over %d", action, name, len(s), b.elemLen)
				}
			}
		}
	}
	return nil
}

// boundJSON walks raw token by token and refuses a document that is not valid
// JSON, nests deeper than maxPayloadDepth, has more than maxPayloadItems
// elements in one container or maxPayloadTokens in all, or holds a string
// over maxPayloadString.
func boundJSON(raw []byte) error {
	if !json.Valid(raw) {
		return fmt.Errorf("is not one valid JSON value")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	// items counts elements: an array's values, an object's key and value
	// together as one (the key token is not counted).
	type frame struct {
		items  int
		object bool
		key    bool // the next token is a key (true until it is read)
	}
	var stack []frame
	tokens := 0
	// value records one value (a scalar or an opened container) in the
	// enclosing container; in an object the key before it is not an element.
	value := func() error {
		if len(stack) == 0 {
			return nil
		}
		top := &stack[len(stack)-1]
		top.items++
		top.key = top.object
		if top.items > maxPayloadItems {
			return fmt.Errorf("a container with more than %d elements", maxPayloadItems)
		}
		return nil
	}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			if tokens == 0 {
				return fmt.Errorf("is empty or only whitespace")
			}
			break
		}
		if err != nil {
			return err
		}
		tokens++
		if tokens > maxPayloadTokens {
			return fmt.Errorf("more than %d tokens", maxPayloadTokens)
		}
		if s, ok := tok.(string); ok {
			if len(s) > maxPayloadString {
				return fmt.Errorf("a string of %d bytes, over %d", len(s), maxPayloadString)
			}
			if n := len(stack); n > 0 && stack[n-1].object && stack[n-1].key {
				stack[n-1].key = false // an object key: not an element
				continue
			}
		}
		if d, ok := tok.(json.Delim); ok && (d == '}' || d == ']') {
			stack = stack[:len(stack)-1]
			continue
		}
		if err := value(); err != nil {
			return err
		}
		if d, ok := tok.(json.Delim); ok {
			if len(stack) >= maxPayloadDepth {
				return fmt.Errorf("nests deeper than %d", maxPayloadDepth)
			}
			stack = append(stack, frame{object: d == '{', key: d == '{'})
		}
	}
	return nil
}
