package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestLiveWSPublishesOwnerScopedProductEvents(t *testing.T) {
	rt, handler := testAPISetup(t)
	srv := server.NewServer("runtime-live-ws-test", "0")
	RegisterRoutes(srv, handler)
	httpSrv := httptest.NewServer(srv)
	defer httpSrv.Close()

	conn := dialLiveWS(t, httpSrv.URL, "/api/ws?desktop_id=branch-a", "user-live")
	defer conn.Close()

	connected := readLiveJSON(t, conn)
	if connected["type"] != "connected" || connected["desktop_id"] != "branch-a" {
		t.Fatalf("connected message = %#v", connected)
	}
	if _, ok := connected["user"]; ok {
		t.Fatalf("connected message exposed user: %#v", connected)
	}
	if _, ok := connected["sandbox_id"]; ok {
		t.Fatalf("connected message exposed sandbox_id: %#v", connected)
	}

	_, err := rt.EmitProductEvent(context.Background(), "user-live", "branch-a", types.EventThemeUpdated, map[string]any{
		"theme": map[string]any{"id": "system-noir"},
	})
	if err != nil {
		t.Fatalf("emit product event: %v", err)
	}

	msg := readLiveJSON(t, conn)
	if msg["type"] != "event" || msg["kind"] != string(types.EventThemeUpdated) {
		t.Fatalf("event message = %#v", msg)
	}
	if numberValue(msg["stream_seq"]) <= 0 {
		t.Fatalf("stream_seq missing in event: %#v", msg)
	}
}

func TestLiveWSCatchesUpAfterStreamSeq(t *testing.T) {
	rt, handler := testAPISetup(t)
	srv := server.NewServer("runtime-live-ws-test", "0")
	RegisterRoutes(srv, handler)
	httpSrv := httptest.NewServer(srv)
	defer httpSrv.Close()

	first, err := rt.EmitProductEvent(context.Background(), "user-live", "primary", types.EventThemeUpdated, map[string]any{
		"theme": map[string]any{"id": "first"},
	})
	if err != nil {
		t.Fatalf("emit first product event: %v", err)
	}
	_, err = rt.EmitProductEvent(context.Background(), "user-live", "primary", types.EventMediaRecentUpdated, map[string]any{
		"kind":     "pdf",
		"identity": "file:/report.pdf",
	})
	if err != nil {
		t.Fatalf("emit second product event: %v", err)
	}

	conn := dialLiveWS(t, httpSrv.URL, "/api/ws?after_seq="+formatInt64(first.StreamSeq), "user-live")
	defer conn.Close()
	_ = readLiveJSON(t, conn)

	msg := readLiveJSON(t, conn)
	if msg["type"] != "event" || msg["kind"] != string(types.EventMediaRecentUpdated) {
		t.Fatalf("catch-up event = %#v", msg)
	}
}

func dialLiveWS(t *testing.T, serverURL, path, user string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(serverURL, "http") + path
	header := http.Header{}
	header.Set("X-Authenticated-User", user)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial live ws: %v", err)
	}
	return conn
}

func readLiveJSON(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msg map[string]any
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read live json: %v", err)
	}
	return msg
}

func formatInt64(value int64) string {
	return strconv.FormatInt(value, 10)
}

func numberValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}
