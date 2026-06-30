package fileprovider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/base/model"
	"github.com/yusefmosiah/go-choir/internal/base/planner"
	"github.com/yusefmosiah/go-choir/internal/desktop"
)

// --- test helpers ---

// fakeBaseAPI is a minimal stub for the sync engine's Base API. It returns
// an empty delta so the engine's runCycle completes without error.
type fakeBaseAPI struct{}

func (f *fakeBaseAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/base/delta":
		writeJSONTest(w, http.StatusOK, desktop.DeltaResponse{
			Events: []model.Event{},
			Cursor: 0,
			Head:   0,
		})
	case "/api/base/blobs":
		writeJSONTest(w, http.StatusOK, desktop.PutBlobResponse{
			BlobRef: model.BlobRef("sha256:0000000000000000000000000000000000000000000000000000000000000000"),
		})
	case "/api/base/items":
		var req desktop.PutItemRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		writeJSONTest(w, http.StatusOK, desktop.PutItemResponse{
			EventID: model.EventID("base_evt_test"),
			ItemID:  req.ItemID,
		})
	default:
		writeJSONTest(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func writeJSONTest(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// testBridge holds the bridge, its root, an HTTP client, and cleanup.
type testBridge struct {
	bridge  *Bridge
	root    string
	client  *http.Client
	cleanup func()
}

// newTestBridge creates a bridge over a temp directory with optional running
// sync engine backed by a fake Base API. The temp root is seeded with a few
// files for enumeration/read tests.
func newTestBridge(t *testing.T, withEngine bool) *testBridge {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "note.md"), []byte("# Note"), 0o644); err != nil {
		t.Fatal(err)
	}

	socketPath := filepath.Join(t.TempDir(), "fp.sock")
	cfg := BridgeConfig{
		LocalRoot:  root,
		SocketPath: socketPath,
		DeviceID:   "test-device",
	}

	if withEngine {
		api := &fakeBaseAPI{}
		srv := httptest.NewServer(api)
		eng := desktop.NewSyncEngine(desktop.SyncConfig{
			BaseURL:   srv.URL,
			LocalRoot: root,
			DeviceID:  "test-device",
			OwnerID:   "user_test",
		}, "choir_sk_test")
		eng.SetHTTPClient(srv.Client())
		statePath := filepath.Join(root, ".choir", "synced-state.json")
		eng.SetSyncedStateStore(desktop.NewFileSyncedStateStore(statePath))
		cfg.Engine = eng
	}

	b, err := NewBridge(cfg)
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}
	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	return &testBridge{
		bridge: b,
		root:   root,
		client: client,
		cleanup: func() {
			b.Stop()
		},
	}
}

// do issues a request to the bridge via the Unix-socket client.
func (tb *testBridge) do(t *testing.T, method, path string, body string) *http.Response {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, "http://unix"+path, r)
	if err != nil {
		t.Fatalf("NewRequest %s %s: %v", method, path, err)
	}
	resp, err := tb.client.Do(req)
	if err != nil {
		t.Fatalf("Do %s %s: %v", method, path, err)
	}
	return resp
}

// --- validation tests ---

func TestNewBridgeValidation(t *testing.T) {
	// Missing LocalRoot.
	_, err := NewBridge(BridgeConfig{SocketPath: "/tmp/x.sock"})
	if err == nil {
		t.Error("expected error for missing LocalRoot")
	}

	// Missing SocketPath.
	_, err = NewBridge(BridgeConfig{LocalRoot: t.TempDir()})
	if err == nil {
		t.Error("expected error for missing SocketPath")
	}

	// Non-existent root.
	_, err = NewBridge(BridgeConfig{LocalRoot: "/nonexistent/path/xyz", SocketPath: "/tmp/x.sock"})
	if err == nil {
		t.Error("expected error for non-existent root")
	}

	// Root is a file, not a directory.
	f := filepath.Join(t.TempDir(), "file.txt")
	_ = os.WriteFile(f, []byte("x"), 0o644)
	_, err = NewBridge(BridgeConfig{LocalRoot: f, SocketPath: "/tmp/x.sock"})
	if err == nil {
		t.Error("expected error for file-as-root")
	}
}

// --- enumeration tests ---

func TestEnumerateRoot(t *testing.T) {
	tb := newTestBridge(t, false)
	defer tb.cleanup()

	resp := tb.do(t, "GET", "/enumerate?path=", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var er EnumerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		t.Fatalf("decode: %v", err)
	}
	names := make(map[string]bool)
	for _, e := range er.Entries {
		names[e.Name] = true
	}
	if !names["hello.txt"] {
		t.Error("expected hello.txt in root enumeration")
	}
	if !names["sub"] {
		t.Error("expected sub/ folder in root enumeration")
	}
}

func TestEnumerateSubdir(t *testing.T) {
	tb := newTestBridge(t, false)
	defer tb.cleanup()

	resp := tb.do(t, "GET", "/enumerate?path=sub", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var er EnumerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(er.Entries) != 1 {
		t.Fatalf("expected 1 entry in sub/, got %d", len(er.Entries))
	}
	if er.Entries[0].Name != "note.md" {
		t.Errorf("expected note.md, got %s", er.Entries[0].Name)
	}
	if er.Entries[0].Kind != KindFile {
		t.Errorf("expected kind file, got %s", er.Entries[0].Kind)
	}
}

func TestEnumerateSkipsHidden(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, ".hidden"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "visible.txt"), []byte("y"), 0o644)
	socketPath := filepath.Join(t.TempDir(), "fp.sock")
	b, err := NewBridge(BridgeConfig{LocalRoot: root, SocketPath: socketPath, DeviceID: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer b.Stop()
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}
	req, _ := http.NewRequest("GET", "http://unix/enumerate?path=", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var er EnumerateResponse
	_ = json.NewDecoder(resp.Body).Decode(&er)
	for _, e := range er.Entries {
		if e.Name == ".hidden" {
			t.Error("hidden file should not appear in enumeration")
		}
	}
}

// --- read tests ---

func TestReadFile(t *testing.T) {
	tb := newTestBridge(t, false)
	defer tb.cleanup()

	resp := tb.do(t, "GET", "/read?path=hello.txt", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var rr ReadResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, err := base64.StdEncoding.DecodeString(rr.ContentB64)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("content: got %q, want %q", string(data), "hello world")
	}
	if rr.Size != int64(len("hello world")) {
		t.Errorf("size: got %d, want %d", rr.Size, len("hello world"))
	}
}

func TestReadNotFound(t *testing.T) {
	tb := newTestBridge(t, false)
	defer tb.cleanup()

	resp := tb.do(t, "GET", "/read?path=nonexistent.txt", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestReadDirectory(t *testing.T) {
	tb := newTestBridge(t, false)
	defer tb.cleanup()

	resp := tb.do(t, "GET", "/read?path=sub", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// --- write tests ---

func TestWriteFile(t *testing.T) {
	tb := newTestBridge(t, true)
	defer tb.cleanup()

	content := []byte("edited by Finder")
	body := fmt.Sprintf(`{"path":"hello.txt","content_b64":"%s"}`, base64.StdEncoding.EncodeToString(content))
	resp := tb.do(t, "PUT", "/write", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var wr WriteResponse
	if err := json.NewDecoder(resp.Body).Decode(&wr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !wr.SyncTriggered {
		t.Error("expected sync_triggered=true")
	}
	data, err := os.ReadFile(filepath.Join(tb.root, "hello.txt"))
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("file content: got %q, want %q", string(data), string(content))
	}
}

func TestWriteNewFile(t *testing.T) {
	tb := newTestBridge(t, true)
	defer tb.cleanup()

	content := []byte("brand new file")
	body := fmt.Sprintf(`{"path":"newdir/new.txt","content_b64":"%s"}`, base64.StdEncoding.EncodeToString(content))
	resp := tb.do(t, "PUT", "/write", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	data, err := os.ReadFile(filepath.Join(tb.root, "newdir", "new.txt"))
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("content: got %q, want %q", string(data), string(content))
	}
}

func TestWriteNoEngine(t *testing.T) {
	tb := newTestBridge(t, false)
	defer tb.cleanup()

	body := `{"path":"x.txt","content_b64":""}`
	resp := tb.do(t, "PUT", "/write", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

// --- mkdir / move / delete tests ---

func TestCreateDir(t *testing.T) {
	tb := newTestBridge(t, true)
	defer tb.cleanup()

	resp := tb.do(t, "POST", "/mkdir", `{"path":"newfolder"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	info, err := os.Stat(filepath.Join(tb.root, "newfolder"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected a directory")
	}
}

func TestMove(t *testing.T) {
	tb := newTestBridge(t, true)
	defer tb.cleanup()

	resp := tb.do(t, "POST", "/move", `{"from_path":"hello.txt","to_path":"renamed.txt"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if _, err := os.Stat(filepath.Join(tb.root, "hello.txt")); !os.IsNotExist(err) {
		t.Error("old path should not exist after move")
	}
	if _, err := os.Stat(filepath.Join(tb.root, "renamed.txt")); err != nil {
		t.Errorf("new path should exist: %v", err)
	}
}

func TestDelete(t *testing.T) {
	tb := newTestBridge(t, true)
	defer tb.cleanup()

	resp := tb.do(t, "POST", "/delete", `{"path":"hello.txt"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if _, err := os.Stat(filepath.Join(tb.root, "hello.txt")); !os.IsNotExist(err) {
		t.Error("file should be deleted")
	}
}

func TestDeleteViaQuery(t *testing.T) {
	tb := newTestBridge(t, true)
	defer tb.cleanup()

	resp := tb.do(t, "DELETE", "/delete?path=hello.txt", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// --- security tests ---

func TestPathTraversalBlocked(t *testing.T) {
	tb := newTestBridge(t, false)
	defer tb.cleanup()

	resp := tb.do(t, "GET", "/read?path=../../etc/passwd", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for traversal, got %d", resp.StatusCode)
	}

	resp2 := tb.do(t, "GET", "/enumerate?path=../../../", "")
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for traversal enumerate, got %d", resp2.StatusCode)
	}
}

// --- status / sync / health tests ---

func TestHealth(t *testing.T) {
	tb := newTestBridge(t, false)
	defer tb.cleanup()

	resp := tb.do(t, "GET", "/health", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestStatusNoEngine(t *testing.T) {
	tb := newTestBridge(t, false)
	defer tb.cleanup()

	resp := tb.do(t, "GET", "/status", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var sr StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if sr.Phase != string(desktop.PhaseIdle) {
		t.Errorf("phase: got %s, want %s", sr.Phase, string(desktop.PhaseIdle))
	}
}

func TestStatusWithEngine(t *testing.T) {
	tb := newTestBridge(t, true)
	defer tb.cleanup()

	resp := tb.do(t, "GET", "/status", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var sr StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Phase should be one of the valid phases.
	switch desktop.SyncPhase(sr.Phase) {
	case desktop.PhaseIdle, desktop.PhaseScanning, desktop.PhaseFetching,
		desktop.PhasePlanning, desktop.PhaseExecuting, desktop.PhaseConflicts,
		desktop.PhaseError, desktop.PhaseCancelled:
		// ok
	default:
		t.Errorf("invalid phase: %s", sr.Phase)
	}
}

func TestSyncNow(t *testing.T) {
	tb := newTestBridge(t, true)
	defer tb.cleanup()

	resp := tb.do(t, "POST", "/sync", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestSyncNowNoEngine(t *testing.T) {
	tb := newTestBridge(t, false)
	defer tb.cleanup()

	resp := tb.do(t, "POST", "/sync", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

// --- conflict tests ---

func TestConflictsEmpty(t *testing.T) {
	tb := newTestBridge(t, true)
	defer tb.cleanup()

	resp := tb.do(t, "GET", "/conflicts", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var cr ConflictsResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(cr.Conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(cr.Conflicts))
	}
}

func TestConflictsExposeCollisionParticipants(t *testing.T) {
	tb := newTestBridge(t, true)
	defer tb.cleanup()

	tb.bridge.engine.Conflicts().SetConflicts([]planner.Conflict{{
		ItemID:       "base_item_remote",
		LocalItemID:  "base_item_local",
		RemoteItemID: "base_item_remote",
		Reason:       "add/add path collision",
	}}, planner.NewTree(), planner.NewTree())

	resp := tb.do(t, "GET", "/conflicts", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var cr ConflictsResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(cr.Conflicts) != 1 {
		t.Fatalf("conflicts: got %d want 1", len(cr.Conflicts))
	}
	got := cr.Conflicts[0]
	if got.LocalItemID != "base_item_local" || got.RemoteItemID != "base_item_remote" {
		t.Fatalf("participant ids: got local=%q remote=%q", got.LocalItemID, got.RemoteItemID)
	}
}

func TestConflictsNoEngine(t *testing.T) {
	tb := newTestBridge(t, false)
	defer tb.cleanup()

	resp := tb.do(t, "GET", "/conflicts", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var cr ConflictsResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(cr.Conflicts) != 0 {
		t.Errorf("expected 0 conflicts without engine, got %d", len(cr.Conflicts))
	}
}

func TestConflictProjectionNoEngine(t *testing.T) {
	root := t.TempDir()
	b, err := NewBridge(BridgeConfig{LocalRoot: root, SocketPath: filepath.Join(t.TempDir(), "s"), DeviceID: "t"})
	if err != nil {
		t.Fatal(err)
	}
	// With no engine, projectConflicts returns nil.
	got := b.projectConflicts("")
	if got != nil {
		t.Errorf("expected nil conflicts without engine, got %v", got)
	}
}

// --- lifecycle tests ---

func TestStopRemovesSocket(t *testing.T) {
	tb := newTestBridge(t, false)
	socket := tb.bridge.SocketPath()
	tb.cleanup()
	if _, err := os.Stat(socket); !os.IsNotExist(err) {
		t.Errorf("socket file should be removed after Stop")
	}
}

func TestStartReplacesStaleSocket(t *testing.T) {
	root := t.TempDir()
	socketPath := filepath.Join(t.TempDir(), "fp.sock")
	// Create a stale socket file.
	f, err := os.Create(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	b, err := NewBridge(BridgeConfig{LocalRoot: root, SocketPath: socketPath, DeviceID: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start with stale socket: %v", err)
	}
	defer b.Stop()
	if _, err := os.Stat(socketPath); err != nil {
		t.Errorf("socket should exist after Start: %v", err)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	tb := newTestBridge(t, false)
	defer tb.cleanup()

	resp := tb.do(t, "POST", "/enumerate", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

// --- internal helper tests ---

func TestResolvePath(t *testing.T) {
	root := t.TempDir()
	b, err := NewBridge(BridgeConfig{LocalRoot: root, SocketPath: filepath.Join(t.TempDir(), "s"), DeviceID: "t"})
	if err != nil {
		t.Fatal(err)
	}

	// Empty path resolves to root.
	abs, err := b.resolvePath("")
	if err != nil {
		t.Fatalf("empty path: %v", err)
	}
	if abs != root {
		t.Errorf("empty path: got %s, want %s", abs, root)
	}

	// Normal relative path.
	abs, err = b.resolvePath("foo/bar.txt")
	if err != nil {
		t.Fatalf("normal path: %v", err)
	}
	want := filepath.Join(root, "foo", "bar.txt")
	if abs != want {
		t.Errorf("normal path: got %s, want %s", abs, want)
	}

	// Traversal blocked.
	_, err = b.resolvePath("../../etc/passwd")
	if err == nil {
		t.Error("expected error for traversal path")
	}

	// Dot path resolves to root.
	abs, err = b.resolvePath(".")
	if err != nil {
		t.Fatalf("dot path: %v", err)
	}
	if abs != root {
		t.Errorf("dot path: got %s, want %s", abs, root)
	}
}

func TestRelPath(t *testing.T) {
	root := t.TempDir()
	b, err := NewBridge(BridgeConfig{LocalRoot: root, SocketPath: filepath.Join(t.TempDir(), "s"), DeviceID: "t"})
	if err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(root, "sub", "file.txt")
	rel := b.relPath(abs)
	if rel != "sub/file.txt" {
		t.Errorf("relPath: got %s, want sub/file.txt", rel)
	}
}
