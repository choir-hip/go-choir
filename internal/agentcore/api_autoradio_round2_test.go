package agentcore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Round-2 review repairs. Failure list, written before the code:
//
//  R1. a completed Report whose request record was never written replays its
//      old answer for new words under the same turn_id;
//  R2. the tape records a client-asserted page hash as fact;
//  R3. a stray closing brace after the JSON body is accepted;
//  R4. two posts of one turn_id at once both run (no 409), untested;
//  R5. a panic inside settle() is swallowed with no log and no done event;
//  R6. done does not say what the gateway applied (deploy skew is silent) or
//      what the computer's policy had in force.

func TestAutoradioTurnReplayIsBoundToTheWordsEvenWithoutARequestRecord(t *testing.T) {
	rt, fake := radioRuntime(t, okAnswer("Never spoken."))
	// A completed Report for t-1 and the first words, with no request record.
	rep := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "autoradio-turn-rep:t-1", Kind: types.CommitmentKindReport,
		Provenance:  types.CommitmentProvenance{AgentID: "app:autoradio:owner-1"},
		Report:      &types.CommitmentReport{StopReason: "completed", Claims: []types.ReportClaim{{Text: "The old answer."}}},
		EmitterKind: types.CommitmentEmitterAgent, ComputerID: rt.cfg.ComputerID,
		SubjectRefs: map[string]string{"words_sha256": sha256Hex("What is the top story?")},
	}
	if _, err := rt.store.AppendCommitmentRecord(context.Background(), "owner-1", rt.cfg.ComputerID, rep); err != nil {
		t.Fatal(err)
	}
	other := `{"turn_id":"t-1","words":"A completely different question?","page":{"text":"p"}}`
	if rec := radioPost(rt, other); rec.Code != http.StatusConflict {
		t.Fatalf("(R1) different words: status %d, body %s", rec.Code, rec.Body.String())
	}
	if fake.callCount() != 0 {
		t.Fatal("(R1) a refused reuse called a model")
	}
	// The same words replay it.
	same := `{"turn_id":"t-1","words":"What is the top story?","page":{"text":"p"}}`
	evs := parseSSE(t, radioPost(rt, same).Body.String())
	if got := speechOf(evs); len(got) != 1 || got[0] != "The old answer." || fake.callCount() != 0 {
		t.Fatalf("(R1) same words: speech %q calls %d", got, fake.callCount())
	}
}

func TestAutoradioTurnTheTapeRecordsTheHashOfTheTextReceived(t *testing.T) {
	rt, _ := radioRuntime(t, okAnswer("Fine."))
	text := "A strike ended in Nancy."
	good := sha256Hex(text)
	lie := strings.Repeat("a", 64)
	if rec := radioPost(rt, `{"turn_id":"t-lie","words":"hi","page":{"text":"`+text+`","text_sha256":"`+lie+`"}}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("(R2) a mismatched hash: status %d", rec.Code)
	}
	if rec := radioPost(rt, `{"turn_id":"t-ok","words":"hi","page":{"text":"`+text+`","text_sha256":"`+good+`"}}`); rec.Code != http.StatusOK {
		t.Fatalf("(R2) a matching hash: status %d", rec.Code)
	}
	req, _ := rt.store.GetCommitmentRecord(context.Background(), "owner-1", rt.cfg.ComputerID, "autoradio-turn-req:t-ok")
	if req == nil || req.SubjectRefs["page_text_sha256"] != good {
		t.Fatalf("(R2) recorded %v", req)
	}
	// No client hash: the server's.
	radioPost(rt, `{"turn_id":"t-none","words":"hi","page":{"text":"`+text+`"}}`)
	req, _ = rt.store.GetCommitmentRecord(context.Background(), "owner-1", rt.cfg.ComputerID, "autoradio-turn-req:t-none")
	if req == nil || req.SubjectRefs["page_text_sha256"] != good {
		t.Fatalf("(R2) recorded %v", req)
	}
}

func TestAutoradioTurnRefusesTrailingBracesAfterTheBody(t *testing.T) {
	rt, fake := radioRuntime(t, okAnswer("Fine."))
	for _, body := range []string{radioBody + "}", radioBody + "]", radioBody + " 7"} {
		if rec := radioPost(rt, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("(R3) %q: status %d", body[len(body)-3:], rec.Code)
		}
	}
	if fake.callCount() != 0 {
		t.Fatal("(R3) a refused body reached a model")
	}
}

func TestAutoradioTurnASecondPostOfTheSameTurnWhileItRunsIs409(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	var once sync.Once
	rt, fake := radioRuntime(t, func(_ int, _ context.Context, req provideriface.TextStreamRequest, onDelta func(string)) (*provideriface.TextStreamResponse, error) {
		once.Do(func() { close(started) })
		<-release
		onDelta("Done.")
		return &provideriface.TextStreamResponse{Text: "Done.", Model: req.Model}, nil
	})
	first := make(chan *httptest.ResponseRecorder, 1)
	go func() { first <- radioPost(rt, radioBody) }()
	<-started
	if rec := radioPost(rt, radioBody); rec.Code != http.StatusConflict {
		t.Fatalf("(R4) second post status %d", rec.Code)
	}
	close(release)
	if rec := <-first; rec.Code != http.StatusOK {
		t.Fatalf("first post status %d", rec.Code)
	}
	if fake.callCount() != 1 {
		t.Fatalf("(R4) %d model calls", fake.callCount())
	}
	// The key is released: the turn can be posted again (a replay).
	if rec := radioPost(rt, radioBody); rec.Code != http.StatusOK {
		t.Fatalf("(R4) after settle: status %d", rec.Code)
	}
}

// panicWriter panics on the first write that carries an error event, as a
// writer that dies while settle() speaks the failure line.
type panicWriter struct {
	*httptest.ResponseRecorder
	once sync.Once
}

func (p *panicWriter) Write(b []byte) (int, error) {
	if strings.Contains(string(b), "event: error") {
		fire := false
		p.once.Do(func() { fire = true })
		if fire {
			panic("writer died")
		}
	}
	return p.ResponseRecorder.Write(b)
}

func TestAutoradioTurnAPanicInsideSettleStillEndsWithDone(t *testing.T) {
	rt, _ := radioRuntime(t, func(int, context.Context, provideriface.TextStreamRequest, func(string)) (*provideriface.TextStreamResponse, error) {
		return nil, &provideriface.ProviderQuotaError{Provider: "x", Message: "spent"}
	})
	req := httptest.NewRequest(http.MethodPost, "/api/autoradio/turns", strings.NewReader(radioBody))
	req.Header.Set("X-Authenticated-User", "owner-1")
	w := &panicWriter{ResponseRecorder: httptest.NewRecorder()}
	NewAPIHandler(rt).HandleAutoradioTurns(w, req)
	evs := parseSSE(t, w.Body.String())
	d := eventsNamed(evs, "done")
	if len(d) != 1 || d[0].Data["status"] != "failed" || d[0].Data["report_error"] == nil {
		t.Fatalf("(R5) events %v", evs)
	}
}

func TestAutoradioTurnDoneSaysWhatWasAppliedAndWhatWasInForce(t *testing.T) {
	rt, _ := radioRuntime(t, func(_ int, _ context.Context, req provideriface.TextStreamRequest, onDelta func(string)) (*provideriface.TextStreamResponse, error) {
		onDelta("Fine.")
		return &provideriface.TextStreamResponse{Text: "Fine.", Model: req.Model, AppliedReasoning: "off", AppliedCacheBreak: "accepted"}, nil
	})
	d := eventsNamed(parseSSE(t, radioPost(rt, radioBody).Body.String()), "done")[0].Data
	applied := d["applied"].(map[string]any)
	inForce := d["in_force"].(map[string]any)
	if applied["reasoning"] != "off" || applied["cache_break"] != "accepted" || inForce["reasoning"] != "off" || inForce["max_tokens"] != float64(0) {
		t.Fatalf("(R6) done %v", d)
	}
	// A fake that reports nothing (an older gateway) is "unknown", never "off".
	rt2, _ := radioRuntime(t, okAnswer("Fine."))
	d = eventsNamed(parseSSE(t, radioPost(rt2, radioBody).Body.String()), "done")[0].Data
	if got := d["applied"].(map[string]any)["reasoning"]; got != "unknown" {
		t.Fatalf("(R6) an unreported application is %v", got)
	}
}
