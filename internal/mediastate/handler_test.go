//go:build comprehensive

package mediastate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func testMediaStateSetup(t *testing.T) (*store.Store, *events.EventBus, *Handler) {
	t.Helper()

	s, err := store.Open(filepath.Join(t.TempDir(), "media-state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	bus := events.NewEventBus()
	return s, bus, NewHandler(s, bus)
}

func mediaStateRequest(handler http.HandlerFunc, method, path, body, ownerID string, headers map[string]string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	if ownerID != "" {
		req.Header.Set("X-Authenticated-User", ownerID)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func TestMediaProgressAPIStoresAndEmitsEvent(t *testing.T) {
	t.Parallel()
	_, bus, handler := testMediaStateSetup(t)
	ch := bus.SubscribeWithBuffer(8)
	defer bus.Unsubscribe(ch)

	headers := map[string]string{
		"X-Choir-Desktop": "desktop-header",
		"X-Choir-Device":  "source-device-a",
	}
	w := mediaStateRequest(handler.HandleMediaProgress, http.MethodPut, "/api/media/progress?desktop_id=desktop-query", `{
		"kind":"audio",
		"identity":"file:/song.mp3",
		"current_time":42,
		"duration":120,
		"playback_rate":1.5,
		"updated_by_device":"device-a"
	}`, "user-media", headers)
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
	var payload map[string]any
	if err := json.Unmarshal(ev.Record.Payload, &payload); err != nil {
		t.Fatalf("decode event payload: %v", err)
	}
	if payload["desktop_id"] != "desktop-query" || payload["source_device_id"] != "source-device-a" {
		t.Fatalf("event device metadata = %#v", payload)
	}

	getW := mediaStateRequest(handler.HandleMediaProgress, http.MethodGet, "/api/media/progress?kind=audio&identity=file:%2Fsong.mp3", "", "user-media", nil)
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

func TestMediaRecentsAPIStoresListsAndIsolatesOwners(t *testing.T) {
	t.Parallel()
	_, bus, handler := testMediaStateSetup(t)
	ch := bus.SubscribeWithBuffer(8)
	defer bus.Unsubscribe(ch)

	w := mediaStateRequest(handler.HandleMediaRecents, http.MethodPut, "/api/media/recents", `{
		"kind":"pdf",
		"identity":"file:/report.pdf",
		"title":"Quarterly Report",
		"file_name":"report.pdf",
		"file_path":"/documents/report.pdf",
		"source_url":"https://example.com/report.pdf",
		"media_type":"application/pdf",
		"content_id":"content-1"
	}`, "user-recents", map[string]string{
		"X-Choir-Desktop": "desktop-recents",
		"X-Choir-Device":  "device-recents",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("put status: got %d body=%s", w.Code, w.Body.String())
	}

	ev := <-ch
	if ev.Record.Kind != types.EventMediaRecentUpdated || ev.Record.OwnerID != "user-recents" || ev.Record.StreamSeq == 0 {
		t.Fatalf("recent event = kind %q owner %q stream %d", ev.Record.Kind, ev.Record.OwnerID, ev.Record.StreamSeq)
	}
	var payload map[string]any
	if err := json.Unmarshal(ev.Record.Payload, &payload); err != nil {
		t.Fatalf("decode event payload: %v", err)
	}
	if payload["desktop_id"] != "desktop-recents" || payload["source_device_id"] != "device-recents" {
		t.Fatalf("event device metadata = %#v", payload)
	}

	getW := mediaStateRequest(handler.HandleMediaRecents, http.MethodGet, "/api/media/recents?kind=pdf&limit=1", "", "user-recents", nil)
	if getW.Code != http.StatusOK {
		t.Fatalf("get status: got %d body=%s", getW.Code, getW.Body.String())
	}
	var body mediaRecentListResponse
	if err := json.NewDecoder(getW.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("recent items = %#v", body.Items)
	}
	item := body.Items[0]
	if item.OwnerID != "user-recents" || item.Kind != "pdf" || item.Identity != "file:/report.pdf" || item.ContentID != "content-1" {
		t.Fatalf("recent item = %#v", item)
	}

	otherW := mediaStateRequest(handler.HandleMediaRecents, http.MethodGet, "/api/media/recents?kind=pdf", "", "other-user", nil)
	if otherW.Code != http.StatusOK {
		t.Fatalf("other owner get status: got %d body=%s", otherW.Code, otherW.Body.String())
	}
	var otherBody mediaRecentListResponse
	if err := json.NewDecoder(otherW.Body).Decode(&otherBody); err != nil {
		t.Fatalf("decode other owner body: %v", err)
	}
	if len(otherBody.Items) != 0 {
		t.Fatalf("other owner recent items = %#v", otherBody.Items)
	}
}

func TestThemePreferenceAPIStoresAndEmitsEvent(t *testing.T) {
	t.Parallel()
	_, bus, handler := testMediaStateSetup(t)
	ch := bus.SubscribeWithBuffer(8)
	defer bus.Unsubscribe(ch)

	w := mediaStateRequest(handler.HandleThemePreference, http.MethodPut, "/api/preferences/theme", `{
		"theme":{"id":"next-workstation","schema_version":1}
	}`, "user-theme", map[string]string{
		"X-Choir-Desktop": "desktop-theme",
		"X-Choir-Device":  "device-theme",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d body=%s", w.Code, w.Body.String())
	}

	ev := <-ch
	if ev.Record.Kind != types.EventThemeUpdated {
		t.Fatalf("event kind = %q, want %q", ev.Record.Kind, types.EventThemeUpdated)
	}
	if ev.Record.OwnerID != "user-theme" || ev.Record.StreamSeq == 0 {
		t.Fatalf("event scope/seq = owner %q stream %d", ev.Record.OwnerID, ev.Record.StreamSeq)
	}
	var payload map[string]any
	if err := json.Unmarshal(ev.Record.Payload, &payload); err != nil {
		t.Fatalf("decode event payload: %v", err)
	}
	if payload["desktop_id"] != "desktop-theme" || payload["source_device_id"] != "device-theme" {
		t.Fatalf("event device metadata = %#v", payload)
	}

	getW := mediaStateRequest(handler.HandleThemePreference, http.MethodGet, "/api/preferences/theme", "", "user-theme", nil)
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
