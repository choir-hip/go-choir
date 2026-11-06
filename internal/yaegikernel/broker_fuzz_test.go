package yaegikernel

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// MA6.3: the broker protocol as hostile input. The cell is model-authored Go,
// so every byte it writes to fd 3 is attacker-controlled. These native fuzz
// targets pin that no input panics, hangs or allocates past a bound at the
// decode (BrokerRequest), the frame reader, the worker's frame multiplexer,
// each verb's payload, and the daemon's admission check.
//
// Ways this could fail, written first:
//  1. a decode panics or loops on malformed JSON or deep nesting;
//  2. a frame header advertises a length the reader allocates;
//  3. the multiplexer's reader loop blocks forever or panics on a frame of an
//     unknown stream, a result with no waiter, or a truncated frame;
//  4. a payload field is accepted past its bound (a handler would then
//     allocate or loop on it);
//  5. admitFrame serves an action the desk's manifest row lacks, or a frame
//     whose token or epoch is wrong, for any bytes at all.
// `go test` runs the checked-in seeds (testdata/fuzz) as ordinary tests; CI also
// fuzzes each target for a fixed time (scripts/fuzz-broker-protocol.sh).

// bytesConn is a net.Conn that reads from a fixed buffer and discards writes.
type bytesConn struct{ r *bytes.Reader }

func (c *bytesConn) Read(p []byte) (int, error)       { return c.r.Read(p) }
func (c *bytesConn) Write(p []byte) (int, error)      { return len(p), nil }
func (c *bytesConn) Close() error                     { return nil }
func (c *bytesConn) LocalAddr() net.Addr              { return nil }
func (c *bytesConn) RemoteAddr() net.Addr             { return nil }
func (c *bytesConn) SetDeadline(time.Time) error      { return nil }
func (c *bytesConn) SetReadDeadline(time.Time) error  { return nil }
func (c *bytesConn) SetWriteDeadline(time.Time) error { return nil }

func frameBytes(stream byte, payload []byte) []byte {
	hdr := make([]byte, frameHeaderLen, frameHeaderLen+len(payload))
	binary.BigEndian.PutUint32(hdr[0:4], uint32(len(payload)))
	hdr[4] = stream
	hdr[5] = FrameVersion
	return append(hdr, payload...)
}

var allActions = []BrokerAction{
	ActionExec, ActionReadFile, ActionWriteFile, ActionListDir, ActionSearchStart, ActionRead, ActionPollSource,
	ActionSourceAPI, ActionReadCapture, ActionSearchWireCorpus, ActionResearch, ActionSaveEvidence, ActionReadEvidence,
	ActionListEvidence, ActionGetRunMemoryEntry, ActionProductAPI, ActionStories, ActionSystem1, ActionRelateStories,
	ActionRelate, ActionRequestLease, ActionTextureSearch, ActionObjectGet, ActionObjectList,
}

func seedRequests(f *testing.F) {
	for _, a := range allActions {
		f.Add([]byte(`{"protocol_version":2,"request_id":"r-1","handle_ref":"h","epoch":1,"action":"` + string(a) + `","payload":{"url":"https://example.com","query":"q","limit":5,"capture_id":"c","children":[{"objective":"o"}]},"slot_token":"t"}`))
	}
	f.Add([]byte(`{"payload":` + strings.Repeat("[", 5000) + strings.Repeat("]", 5000) + `}`))
	f.Add([]byte(`{"timeout_ms":-1,"epoch":18446744073709551615,"action":null,"payload":"x"}`))
	f.Add([]byte(``))
	f.Add([]byte(`[]`))
	f.Add([]byte(`{"protocol_version":2,"request_id":"` + strings.Repeat("a", 1000) + `"}`))
}

// FuzzBrokerRequestDecode: decoding a request and validating it never
// panics, and whatever passes ValidateBounds is inside every bound.
func FuzzBrokerRequestDecode(f *testing.F) {
	seedRequests(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		var req BrokerRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return
		}
		_ = req.Validate()
		if err := req.ValidateBounds(); err != nil {
			return
		}
		if len(req.RequestID) > maxRequestIDLen || len(req.HandleRef) > maxHandleRefLen || len(req.SlotToken) > maxSlotTokenLen ||
			req.TimeoutMs < 0 || req.TimeoutMs > maxTimeoutMs {
			t.Fatalf("ValidateBounds accepted an out-of-bound field: %+v", req)
		}
		// Every typed payload decodes or fails cleanly.
		decodePayloads(req.Payload)
		// A request that passes re-encodes and decodes to the same bounds.
		enc, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("re-encode: %v", err)
		}
		var again BrokerRequest
		if err := json.Unmarshal(enc, &again); err != nil {
			t.Fatalf("re-decode: %v", err)
		}
		if err := again.ValidateBounds(); err != nil {
			t.Fatalf("a request that passed no longer passes after a round trip: %v", err)
		}
	})
}

func decodePayloads(raw json.RawMessage) {
	for _, dst := range []any{&ExecPayload{}, &ReadFilePayload{}, &WriteFilePayload{}, &ListDirPayload{}, &ResearchPayload{},
		&ProductAPIPayload{}, &StoriesPayload{}, &ReadPayload{}, &PollSourcePayload{}, &SourceAPIPayload{}, &RequestLeasePayload{},
		&ReadCapturePayload{}, &SearchWireCorpusPayload{}, &SaveEvidencePayload{}, &ReadEvidencePayload{}, &ListEvidencePayload{},
		&GetRunMemoryEntryPayload{}, &SearchStartPayload{}, &System1Payload{}, &RelatePayload{}, &RelateStoriesPayload{},
		&ObjectGetPayload{}, &ObjectListPayload{}, &TextureSearchPayload{}} {
		_ = json.Unmarshal(raw, dst)
	}
}

// FuzzValidatePayload: every action's payload check never panics, and an
// accepted payload is inside the structural and field bounds, checked by an
// independent decode.
func FuzzValidatePayload(f *testing.F) {
	for i, a := range allActions {
		f.Add(uint8(i), []byte(`{"url":"u","query":"q","limit":3,"capture_id":"c","kinds":["a"],"children":[{"objective":"o"}],"evidence_id":"e","agent_id":"a","path":"p","purpose":"x"}`))
		f.Add(uint8(i), []byte(`{"limit":-1,"max_items":1e99,"children":[`+strings.Repeat(`{},`, 100)+`{}]}`))
		_ = a
	}
	f.Add(uint8(0), []byte(strings.Repeat("[", 100)+strings.Repeat("]", 100)))
	f.Fuzz(func(t *testing.T, which uint8, data []byte) {
		action := allActions[int(which)%len(allActions)]
		if err := ValidatePayload(action, data); err != nil {
			return
		}
		if len(data) == 0 || string(data) == "null" {
			return
		}
		var generic any
		if err := json.Unmarshal(data, &generic); err != nil {
			t.Fatalf("accepted payload is not JSON: %q", data)
		}
		if d := jsonDepth(generic); d > maxPayloadDepth {
			t.Fatalf("accepted a payload nested %d deep", d)
		}
		fields, ok := generic.(map[string]any)
		if !ok {
			return
		}
		for name, b := range payloadBounds[action] {
			switch v := fields[name].(type) {
			case string:
				if b.max == 0 && b.min == 0 && len(v) > b.maxLen {
					t.Fatalf("%s.%s accepted at %d bytes, bound %d", action, name, len(v), b.maxLen)
				}
			case float64:
				if b.max != 0 && (v < b.min || v > b.max) {
					t.Fatalf("%s.%s accepted at %v, bound %v..%v", action, name, v, b.min, b.max)
				}
			}
		}
		for name, b := range payloadListBounds[action] {
			if l, ok := fields[name].([]any); ok && len(l) > b.n {
				t.Fatalf("%s.%s accepted with %d entries, bound %d", action, name, len(l), b.n)
			}
		}
	})
}

func jsonDepth(v any) int {
	switch x := v.(type) {
	case []any:
		d := 0
		for _, e := range x {
			if n := jsonDepth(e); n > d {
				d = n
			}
		}
		return d + 1
	case map[string]any:
		d := 0
		for _, e := range x {
			if n := jsonDepth(e); n > d {
				d = n
			}
		}
		return d + 1
	}
	return 0
}

// FuzzReadFrame: the frame reader never panics, never returns a payload over
// the limit, and refuses a header with another version or reserved bytes.
func FuzzReadFrame(f *testing.F) {
	f.Add(frameBytes(StreamCell, []byte(`{"id":"cell-1"}`)))
	f.Add(frameBytes(StreamBrokerEgress, []byte(`{}`)))
	f.Add([]byte{0xff, 0xff, 0xff, 0xff, 6, 1, 0, 0})
	f.Add([]byte{0, 0, 0, 1, 6, 2, 0, 0, 'x'})
	f.Add([]byte{0, 0, 0, 1, 6, 1, 1, 0, 'x'})
	f.Add([]byte{0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		fc := NewFramedConn(&bytesConn{r: bytes.NewReader(data)})
		for i := 0; i < 64; i++ {
			_, payload, err := fc.ReadFrame()
			if err != nil {
				return
			}
			if len(payload) > MaxFramePayload {
				t.Fatalf("payload of %d bytes returned", len(payload))
			}
		}
	})
}

// FuzzWorkerMuxReadLoop: the worker's reader loop consumes any byte stream:
// it neither panics nor stalls, whatever the streams, ids and payloads.
func FuzzWorkerMuxReadLoop(f *testing.F) {
	f.Add(append(frameBytes(StreamBrokerEgressResult, []byte(`{"request_id":"w","success":true}`)), frameBytes(StreamCell, []byte(`{}`))...))
	f.Add(frameBytes(StreamSearchArrival, []byte(`{"id":"s","kind":"start","query":"q"}`)))
	f.Add(frameBytes(StreamSearchArrival, []byte(`not json`)))
	f.Add(frameBytes(StreamBrokerEgressResult, []byte(`{"request_id":`)))
	f.Add(frameBytes(99, []byte(`{}`)))
	f.Add(frameBytes(StreamStdout, nil))
	f.Fuzz(func(t *testing.T, data []byte) {
		mux := newWorkerMux(NewFramedConn(&bytesConn{r: bytes.NewReader(data)}), newSearchStore())
		// A waiter the stream may answer.
		mux.mu.Lock()
		mux.waiters["w"] = muxWaiter{stream: StreamBrokerEgressResult, ch: make(chan []byte, 1)}
		mux.mu.Unlock()
		deadline := time.After(10 * time.Second)
		for {
			done := make(chan struct{})
			var err error
			go func() { _, _, err = mux.next(); close(done) }()
			select {
			case <-done:
				if err != nil {
					return
				}
			case <-deadline:
				t.Fatal("the reader loop stalled")
			}
		}
	})
}

// FuzzSearchFrame: a search arrival payload never panics the store.
func FuzzSearchFrame(f *testing.F) {
	f.Add([]byte(`{"id":"s","kind":"start","query":"q","providers":["a"]}`))
	f.Add([]byte(`{"id":"s","kind":"provider","arrival":{"provider":"p"}}`))
	f.Add([]byte(`{"id":"s","kind":"done","pending":["a"]}`))
	f.Add([]byte(`{"id":" ","kind":"x"}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		st := newSearchStore()
		st.applyFrame(data)
		st.applyFrame(data)
	})
}

// FuzzSessionFrameDecode: the worker's decode of a cell frame never panics.
func FuzzSessionFrameDecode(f *testing.F) {
	f.Add([]byte(`{"id":"cell-1","source":"1+1"}`))
	f.Add([]byte(`{"close":true}`))
	f.Add([]byte(`{"id":1}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		var frame SessionFrame
		_ = json.Unmarshal(data, &frame)
	})
}

// FuzzAdmitFrame: for any request, admitFrame serves only a frame that
// carries this worker's slot token and epoch and an action the desk's
// manifest row names (the confused-deputy guard: the role is the worker's,
// never a field of the frame), and that is inside every bound.
func FuzzAdmitFrame(f *testing.F) {
	const token = "0123456789abcdef"
	for _, a := range allActions {
		f.Add(string(a), token, uint64(3), int64(0), []byte(`{"query":"q"}`))
	}
	f.Add("read", "wrong", uint64(3), int64(0), []byte(`{}`))
	f.Add("read", token, uint64(4), int64(-5), []byte(`{}`))
	f.Add("product_api", token, uint64(3), int64(0), []byte(`{"method":"`+strings.Repeat("G", 50)+`"}`))
	f.Fuzz(func(t *testing.T, action, tok string, epoch uint64, timeoutMs int64, payload []byte) {
		for _, role := range []string{"management", "texture", "research", "nonexistent", ""} {
			w := &DeskSessionWorker{slotToken: []byte(token)}
			w.cfg.Session.Role = role
			w.cfg.Session.Epoch = 3
			req := BrokerRequest{ProtocolVersion: ProtocolVersion, RequestID: "r", HandleRef: "h", Epoch: epoch,
				Action: BrokerAction(action), Payload: payload, TimeoutMs: timeoutMs, SlotToken: tok}
			reason, forged := w.admitFrame(t.Context(), StreamBrokerEgress, req)
			if reason != "" {
				if forged && tok == token {
					t.Fatalf("a frame with the right token was called forged: %s", reason)
				}
				continue
			}
			if tok != token {
				t.Fatalf("served a frame with token %q", tok)
			}
			if epoch != 3 {
				t.Fatalf("served a frame of epoch %d to a worker of epoch 3", epoch)
			}
			if !deskmanifest.FrameAllowed(role, action) {
				t.Fatalf("served %q to desk %q, whose row does not name it", action, role)
			}
			if err := req.ValidateBounds(); err != nil {
				t.Fatalf("served a frame outside the bounds: %v", err)
			}
		}
	})
}

// FuzzHandlerPayloads: each verb's own host-side check (the handler's strict
// decode and its validator) never panics, and what it accepts is inside the
// verb's bounds. These are the checks the agentcore handlers run after
// admission (system1_verb.go, story_entities_verb.go, story_relation_verb.go,
// research_children.go).
func FuzzHandlerPayloads(f *testing.F) {
	f.Add(uint8(0), []byte(`{"questions":[{"id":"q1","text":"Is it?","choices":["yes","no"],"context_refs":["choir.web_capture:abc"]}]}`))
	f.Add(uint8(0), []byte(`{"questions":[{"id":"q1","text":"t","choices":["a","b"],"passages":["`+strings.Repeat("p", 100)+`"]}]}`))
	f.Add(uint8(1), []byte(`{"story":"story:head","entities":[{"name":"Ada","role":"subject","type":"person"}]}`))
	f.Add(uint8(2), []byte(`{"pairs":[{"story_a":"a","story_b":"b","relation":"same_event","judgment_ref":"system1:x","question_id":"q1"}]}`))
	f.Add(uint8(3), []byte(`{"children":[{"objective":"o","role":"texture","p_success":0.5,"weight":2}]}`))
	f.Add(uint8(3), []byte(`{"children":[{"objective":"o","questions":[{"question":"q","answers":["a","b"],"probabilities":{"a":0.5,"b":0.5}}]}]}`))
	f.Fuzz(func(t *testing.T, which uint8, data []byte) {
		strict := func(dst any) bool {
			dec := json.NewDecoder(bytes.NewReader(data))
			dec.DisallowUnknownFields()
			return dec.Decode(dst) == nil
		}
		switch which % 4 {
		case 0:
			var req System1Payload
			if !strict(&req) {
				return
			}
			qs, err := ValidateSystem1Questions(req.Questions)
			if err != nil {
				return
			}
			if len(qs) > System1MaxQuestions {
				t.Fatalf("accepted %d questions", len(qs))
			}
			for _, q := range qs {
				if len(q.Choices) > System1MaxChoices || len(q.ContextRefs) > System1MaxRefsPerQuestion ||
					len(q.Passages) > System1MaxPassagesPerQuestion {
					t.Fatalf("accepted a question past its bounds: %+v", q)
				}
				for _, ref := range q.ContextRefs {
					if len(ref) > system1MaxRefLen {
						t.Fatalf("accepted a ref of %d bytes", len(ref))
					}
				}
			}
		case 1:
			var req RelatePayload
			if !strict(&req) {
				return
			}
			got, err := ValidateRelate(req)
			if err != nil {
				return
			}
			if len(got.Entities) > RelateMaxEntities {
				t.Fatalf("accepted %d entities", len(got.Entities))
			}
		case 2:
			var req RelateStoriesPayload
			if !strict(&req) {
				return
			}
			pairs, err := ValidateRelateStoriesPairs(req.Pairs)
			if err != nil {
				return
			}
			if len(pairs) > RelateStoriesMaxPairs {
				t.Fatalf("accepted %d pairs", len(pairs))
			}
			for _, p := range pairs {
				if len(p.StoryA) > relateStoriesMaxRefLen || len(p.StoryB) > relateStoriesMaxRefLen {
					t.Fatalf("accepted a story ref past its bound: %+v", p)
				}
			}
		case 3:
			var req ResearchPayload
			if json.Unmarshal(data, &req) != nil {
				return
			}
			if err := ResearchChildrenBounds(req.Children); err != nil {
				return
			}
			if ValidateResearchChildren(req.Children) != nil {
				return
			}
			if len(req.Children) > MaxResearchChildren {
				t.Fatalf("accepted %d children", len(req.Children))
			}
			for _, c := range req.Children {
				if len(c.Objective) > MaxResearchObjectiveBytes || len(c.Model) > maxResearchModelLen {
					t.Fatalf("accepted a child past its bounds")
				}
				for _, v := range []*float64{c.PSuccess, c.Weight} {
					if v != nil && (*v < 0 || *v > 1) {
						t.Fatalf("accepted p_success or weight %v", *v)
					}
				}
			}
		}
	})
}

var _ = io.EOF
