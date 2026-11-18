package agentcore

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/notebook"
)

// The phone's wire (lane learn-phone, mobile/fixtures/learning/): each file is
// decoded by the server's own types, so the two sides cannot drift.

func phoneFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../mobile/fixtures/learning/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestThePhonesLearningFixturesDecodeAsTheRoutesShape(t *testing.T) {
	for _, name := range []string{"learning.json", "learning-empty.json"} {
		var out struct {
			Scorecard learningCard  `json:"scorecard"`
			Rows      []learningRow `json:"rows"`
			Truncated bool          `json:"truncated"`
		}
		dec := json.NewDecoder(bytes.NewReader(phoneFixture(t, name)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&out); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if name == "learning.json" && (len(out.Rows) != 4 || out.Rows[1].Op == nil || out.Rows[1].Op.Outcome != "finished" || out.Rows[3].Reason != "left") {
			t.Errorf("%s: %+v", name, out.Rows)
		}
	}
}

func TestThePhonesSegmentStreamsCarryOnlyKeysTheServerSends(t *testing.T) {
	sent := map[string]bool{}
	for _, k := range []string{"stretch_id", "segment_id", "kind", "title", "text", "cites", "block_count", "est_seconds", "cut_at", "entry_id", "claim_id", "between", "clip"} {
		sent[k] = true
	}
	for _, name := range []string{"segment-settle.sse", "segment-old.sse"} {
		event := ""
		for _, line := range strings.Split(string(phoneFixture(t, name)), "\n") {
			if e, ok := strings.CutPrefix(line, "event: "); ok {
				event = e
			} else if d, ok := strings.CutPrefix(line, "data: "); ok && event == "segment" {
				var m map[string]any
				if err := json.Unmarshal([]byte(d), &m); err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				for k := range m {
					if !sent[k] {
						t.Errorf("%s: the phone expects a key %q the server does not send", name, k)
					}
				}
			}
		}
	}
}

func TestThePhonesSteerRequestIsAccepted(t *testing.T) {
	fake := &fakeNotebook{}
	rt, _ := radioRuntime(t, notebookScript(nil))
	h := NewAPIHandler(rt)
	h.SetNotebook(fake)
	// The entry is not on the server's sheet: the phone's own title stands.
	if rec := steerPost(h, string(phoneFixture(t, "steer-request.json"))); rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if got := notebook.Parse(fake.text()).Lines(notebook.SecSteering); len(got) != 1 || !strings.HasPrefix(got[0].Text, `Less like this: "Harbor fees rise"`) {
		t.Errorf("steering: %v", got)
	}
	if rec := steerPost(h, `{"entry_id":"d-s7-1","direction":"mute","handle":"voice-3fa2c1","title":"x","stories":["a"]}`); rec.Code != http.StatusOK {
		t.Errorf("mute by handle: %d %s", rec.Code, rec.Body.String())
	}
	if got := notebook.Parse(fake.text()).Lines(notebook.SecAsked); len(got) != 1 || got[0].Text != "Never play takes by voice-3fa2c1." {
		t.Errorf("asked: %v", got)
	}
	// Rows behind a line are looked up by the line id.
	if code, out := learningGet(t, h, "?line=L1"); code != 200 || len(out["rows"].([]any)) != 0 {
		t.Errorf("line=L1: %d %v", code, out["rows"])
	}
}

func TestThePhonesTurnQueueWithOnAirIsNotRefused(t *testing.T) {
	q := `{"version":3,"on_air":{"id":"d-s7-1","title":"Harbor fees rise","claim":"calib:autoradio-dj:s7:1","stories":["harbor-fees"]},"upcoming":[]}`
	rt, _ := radioRuntime(t, notebookScript(nil))
	body := `{"turn_id":"t-9","words":"less sport","queue":` + q + `}`
	if rec := radioPost(rt, body); rec.Code == http.StatusBadRequest {
		t.Errorf("the turn route refused the phone's queue: %s", rec.Body.String())
	}
}
