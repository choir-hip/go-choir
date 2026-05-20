package runtime

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestMediaProgressAPIStoresAndEmitsEvent(t *testing.T) {
	rt, handler := testAPISetup(t)
	ch := rt.EventBus().SubscribeWithBuffer(8)
	defer rt.EventBus().Unsubscribe(ch)

	w := registeredRuntimeRequest(t, handler, http.MethodPut, "/api/media/progress", `{
		"kind":"audio",
		"identity":"file:/song.mp3",
		"current_time":42,
		"duration":120,
		"playback_rate":1.5,
		"updated_by_device":"device-a"
	}`, "user-media")
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d body=%s", w.Code, w.Body.String())
	}

	ev := <-ch
	if ev.Record.Kind != types.EventMediaProgressUpdated {
		t.Fatalf("event kind = %q, want %q", ev.Record.Kind, types.EventMediaProgressUpdated)
	}
	if ev.Record.OwnerID != "user-media" || ev.Record.StreamSeq == 0 {
		t.Fatalf("event scope/seq = owner %q stream %d", ev.Record.OwnerID, ev.Record.StreamSeq)
	}

	getW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/media/progress?kind=audio&identity=file:%2Fsong.mp3", "", "user-media")
	if getW.Code != http.StatusOK {
		t.Fatalf("get status: got %d body=%s", getW.Code, getW.Body.String())
	}
	var body struct {
		CurrentTime float64 `json:"current_time"`
		Duration    float64 `json:"duration"`
	}
	if err := json.NewDecoder(getW.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.CurrentTime != 42 || body.Duration != 120 {
		t.Fatalf("progress body = %#v", body)
	}
}

func TestThemePreferenceAPIStoresAndEmitsEvent(t *testing.T) {
	rt, handler := testAPISetup(t)
	ch := rt.EventBus().SubscribeWithBuffer(8)
	defer rt.EventBus().Unsubscribe(ch)

	w := registeredRuntimeRequest(t, handler, http.MethodPut, "/api/preferences/theme", `{
		"theme":{"id":"next-workstation","schema_version":1}
	}`, "user-theme")
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d body=%s", w.Code, w.Body.String())
	}

	ev := <-ch
	if ev.Record.Kind != types.EventThemeUpdated {
		t.Fatalf("event kind = %q, want %q", ev.Record.Kind, types.EventThemeUpdated)
	}

	getW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/preferences/theme", "", "user-theme")
	if getW.Code != http.StatusOK {
		t.Fatalf("get status: got %d body=%s", getW.Code, getW.Body.String())
	}
	var body struct {
		Theme map[string]any `json:"theme"`
	}
	if err := json.NewDecoder(getW.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Theme["id"] != "next-workstation" {
		t.Fatalf("theme body = %#v", body.Theme)
	}
}
