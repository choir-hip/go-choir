package agentcore

// The heard receipt: the stretch's start record carries the heard counts, and
// GET /api/autoradio/learning shows them in notebook.inputs[].heard, from the
// same variables as the `autoradio heard:` log line. Ids and counts only. The
// failure list, written before the code:
//
//	T1 a stretch after a finished story shows same=1 (and the others in
//	   not_in_window), and a higher version shows earlier=1;
//	T2 an unavailable read shows unavailable and its error class, and no counts;
//	T3 the receipt holds no title and no text.

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// heardReceipt is the heard object of the stretch's input receipt.
func heardReceipt(t *testing.T, h *APIHandler, stretch string) map[string]any {
	t.Helper()
	code, out := learningGet(t, h, "")
	if code != 200 {
		t.Fatalf("learning: %d", code)
	}
	nb, _ := out["notebook"].(map[string]any)
	inputs, _ := nb["inputs"].([]any)
	for _, in := range inputs {
		m, _ := in.(map[string]any)
		if m["stretch_id"] == stretch {
			hd, _ := m["heard"].(map[string]any)
			return hd
		}
	}
	return nil
}

func num(m map[string]any, k string) int {
	f, _ := m[k].(float64)
	return int(f)
}

func TestT1_TheReceiptCarriesTheHeardCounts(t *testing.T) {
	rt, _ := radioRuntime(t, oneStory())
	h := NewAPIHandler(rt)
	seedHeard(t, rt, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Harbour reopens", text: "The harbour reopened after repairs.", at: time.Now().Add(-26 * time.Hour)})
	rec := httptest.NewRecorder()
	progServe(h, rec, progBody(withRequestID("t1-a")))
	sid := stretchIDOf(t, parseSSE(t, rec.Body.String()))
	r := heardReceipt(t, h, sid)
	if r == nil {
		t.Fatalf("(T1) no heard receipt for %s", sid)
	}
	if num(r, "candidates") != 3 || num(r, "same") != 1 || num(r, "not_in_window") != 2 || num(r, "earlier") != 0 || num(r, "recent") != 0 {
		t.Errorf("(T1) counts: %v", r)
	}
	if r["unavailable"] != false || r["all_heard"] != false {
		t.Errorf("(T1) flags: %v", r)
	}
	if num(r, "segments_rows") < 1 || num(r, "settlements_rows") != 1 || r["segments_full"] != false || r["settlements_full"] != false {
		t.Errorf("(T1) reads: %v", r)
	}

	// The finished story again at a higher version: earlier=1.
	rec2 := httptest.NewRecorder()
	progServe(h, rec2, progBody(withRequestID("t1-b"), func(m map[string]any) {
		c := progCandidates()
		c[0]["version"] = 2
		m["candidates"] = c
	}))
	r2 := heardReceipt(t, h, stretchIDOf(t, parseSSE(t, rec2.Body.String())))
	if r2 == nil || num(r2, "earlier") != 1 || num(r2, "same") != 0 || num(r2, "not_in_window") != 2 {
		t.Errorf("(T1) higher version: %v", r2)
	}

	// T3: ids and counts only.
	b, _ := json.Marshal(r)
	for _, w := range []string{"Harbour", "harbour", "reopened", "repairs"} {
		if strings.Contains(string(b), w) {
			t.Errorf("(T3) the receipt holds %q: %s", w, b)
		}
	}
}

func TestT2_AnUnavailableReadIsInTheReceiptWithNoCounts(t *testing.T) {
	rt, _ := radioRuntime(t, oneStory())
	seedHeard(t, rt, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Harbour reopens", text: "The harbour reopened.", at: time.Now().Add(-26 * time.Hour)})
	h := NewAPIHandler(rt)
	h.autoradioLedgerOverride = &wrapLedger{autoradioLedger: rt.store, failListKind: kindSegment}
	rec := httptest.NewRecorder()
	progServe(h, rec, progBody(withRequestID("t2")))
	r := heardReceipt(t, h, stretchIDOf(t, parseSSE(t, rec.Body.String())))
	if r == nil || r["unavailable"] != true || r["error_class"] == "" || r["error_class"] == nil {
		t.Fatalf("(T2) receipt: %v", r)
	}
	for _, k := range []string{"candidates", "same", "earlier", "not_in_window", "recent", "segments_rows"} {
		if _, has := r[k]; has {
			t.Errorf("(T2) an unavailable receipt carries %s", k)
		}
	}
}
