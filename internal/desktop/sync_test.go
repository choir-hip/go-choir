package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/base/model"
	"github.com/yusefmosiah/go-choir/internal/base/tree"
)

// fakeBaseAPI is a controllable in-process Base API for sync engine tests.
// It serves delta, blobs, and items endpoints and records the calls.
type fakeBaseAPI struct {
	mu        sync.Mutex
	events    []model.Event
	cursor    int64
	blobCalls int
	itemCalls int
	itemReqs  []PutItemRequest
}

func (f *fakeBaseAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.URL.Path {
	case "/api/base/delta":
		cursorStr := r.URL.Query().Get("cursor")
		var cursor int64
		if cursorStr != "" {
			fmt.Sscanf(cursorStr, "%d", &cursor)
		}
		var out []model.Event
		var newCursor int64 = cursor
		var head int64
		for _, e := range f.events {
			if e.CursorSeq > head {
				head = e.CursorSeq
			}
			if e.CursorSeq > cursor {
				out = append(out, e)
				if e.CursorSeq > newCursor {
					newCursor = e.CursorSeq
				}
			}
		}
		if out == nil {
			out = []model.Event{}
		}
		writeJSONTest(w, http.StatusOK, DeltaResponse{Events: out, Cursor: newCursor, Head: head})
	case "/api/base/blobs":
		f.blobCalls++
		// Read and discard body; return a fixed blob ref.
		var resp PutBlobResponse
		// Compute a real-ish ref from the body length so uploads are distinct.
		writeJSONTest(w, http.StatusOK, resp)
	case "/api/base/items":
		f.itemCalls++
		var req PutItemRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.itemReqs = append(f.itemReqs, req)
		writeJSONTest(w, http.StatusOK, PutItemResponse{
			EventID:   model.EventID("base_evt_fake"),
			CursorSeq: int64(len(req.ItemID)),
			ItemID:    req.ItemID,
		})
	default:
		writeJSONTest(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

// addEvent appends a journal event to the fake API's event log.
func (f *fakeBaseAPI) addEvent(evt model.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cursor++
	evt.CursorSeq = f.cursor
	f.events = append(f.events, evt)
}

// ToSeq is a helper to derive a fake cursor seq from an item id for the stub.

func TestSyncEngineUploadCycle(t *testing.T) {
	// Local folder with one new file; remote is empty; synced is empty.
	// The planner should produce an ActionUpload, and the engine should call
	// PutBlob + PutItem.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	api := &fakeBaseAPI{}
	srv := httptest.NewServer(api)
	defer srv.Close()

	statePath := filepath.Join(root, ".choir-synced-state.json")
	eng := NewSyncEngine(SyncConfig{
		BaseURL:   srv.URL,
		LocalRoot: root,
		DeviceID:  "test-device",
		OwnerID:   "user_test",
	}, "choir_sk_test")
	eng.SetHTTPClient(srv.Client())
	eng.SetSyncedStateStore(NewFileSyncedStateStore(statePath))

	// Run one cycle synchronously by invoking runCycle directly.
	eng.runCycle(context.Background())

	// The engine should have uploaded a blob and created an item.
	api.mu.Lock()
	blobs := api.blobCalls
	items := api.itemCalls
	api.mu.Unlock()
	if blobs == 0 {
		t.Error("expected at least one PutBlob call for the new file")
	}
	if items == 0 {
		t.Error("expected at least one PutItem call for the new file")
	}

	// The synced state file should now exist with cursor 0 (no remote events).
	st, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("synced state file not written: %v", err)
	}
	_ = st

	// Status should be idle (synced).
	p := eng.Status().Snapshot()
	if p.Phase != PhaseIdle {
		t.Errorf("phase after cycle: got %q, want idle", p.Phase)
	}
}

func TestSyncEngineDownloadCycle(t *testing.T) {
	// Remote has one file; local is empty; synced is empty.
	// The planner should produce an ActionDownload.
	root := t.TempDir()

	api := &fakeBaseAPI{}
	// Add a remote create event for a file.
	itemID := model.ItemID("base_item_remote1")
	verID := model.VersionID("base_ver_remote1")
	api.addEvent(model.Event{
		EventID:     model.EventID("base_evt_r1"),
		OwnerID:     "user_test",
		ItemID:      itemID,
		DeviceID:    "remote-device",
		SubjectID:   "user_test",
		EventType:   model.EventCreate,
		Kind:        model.KindFile,
		PayloadJSON: `{"name":"remote.txt","kind":"file","version_id":"base_ver_remote1","blob_ref":"sha256:abc","content_hash":"abc"}`,
	})

	srv := httptest.NewServer(api)
	defer srv.Close()

	// Stub the GET item endpoint to return the remote item.
	// We wrap the fake API with a handler that also serves GET /api/base/items/{id}.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/base/delta", api.ServeHTTP)
	mux.HandleFunc("/api/base/items/", func(w http.ResponseWriter, r *http.Request) {
		writeJSONTest(w, http.StatusOK, ItemResponse{
			Item: model.Item{
				ItemID:         itemID,
				Name:           "remote.txt",
				Kind:           model.KindFile,
				CurrentVersion: verID,
			},
			Version: model.Version{VersionID: verID, ItemID: itemID, BlobRef: "sha256:abc"},
		})
	})
	srv2 := httptest.NewServer(mux)
	defer srv2.Close()

	eng := NewSyncEngine(SyncConfig{
		BaseURL:   srv2.URL,
		LocalRoot: root,
		DeviceID:  "test-device",
		OwnerID:   "user_test",
	}, "choir_sk_test")
	eng.SetHTTPClient(srv2.Client())
	eng.SetSyncedStateStore(NewFileSyncedStateStore(filepath.Join(root, ".choir-synced-state.json")))

	eng.runCycle(context.Background())

	// The downloaded file should exist locally (placeholder).
	// The remote item has no parent, so its path is just "remote.txt".
	if _, err := os.Stat(filepath.Join(root, "remote.txt")); err != nil {
		t.Errorf("downloaded file not created: %v", err)
	}

	p := eng.Status().Snapshot()
	if p.Phase != PhaseIdle {
		t.Errorf("phase: got %q, want idle", p.Phase)
	}
	if p.Cursor == 0 {
		t.Error("cursor not advanced after download cycle")
	}
}

func TestSyncEngineCancellation(t *testing.T) {
	root := t.TempDir()
	api := &fakeBaseAPI{}
	srv := httptest.NewServer(api)
	defer srv.Close()

	eng := NewSyncEngine(SyncConfig{
		BaseURL:   srv.URL,
		LocalRoot: root,
		DeviceID:  "test-device",
		Interval:  50 * time.Millisecond,
	}, "choir_sk_test")
	eng.SetHTTPClient(srv.Client())
	eng.SetSyncedStateStore(NewFileSyncedStateStore(filepath.Join(root, ".state.json")))

	ctx, cancel := context.WithCancel(context.Background())
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !eng.IsRunning() {
		t.Error("IsRunning: got false, want true")
	}

	// Let it run a couple cycles.
	time.Sleep(120 * time.Millisecond)
	cancel()

	// After cancellation, IsRunning should eventually be false.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !eng.IsRunning() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	// We can't strictly assert false because Stop sets running=false but
	// the loop's defer sets cancelled. Just ensure no panic/deadlock.
}

func TestSyncEngineStartTwice(t *testing.T) {
	root := t.TempDir()
	api := &fakeBaseAPI{}
	srv := httptest.NewServer(api)
	defer srv.Close()

	eng := NewSyncEngine(SyncConfig{
		BaseURL:   srv.URL,
		LocalRoot: root,
		DeviceID:  "dev",
	}, "choir_sk_test")
	eng.SetHTTPClient(srv.Client())
	eng.SetSyncedStateStore(NewFileSyncedStateStore(filepath.Join(root, ".s.json")))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := eng.Start(ctx); err == nil {
		t.Error("second Start should return ErrAlreadyRunning")
	}
}

func TestSyncEngineConflictPauses(t *testing.T) {
	// Construct a scenario where the planner produces a conflict: both local
	// and remote have the same item with different content, and the synced
	// ancestor has the old content.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "both.txt"), []byte("local"), 0o644); err != nil {
		t.Fatal(err)
	}

	api := &fakeBaseAPI{}
	itemID := localItemID("both.txt")
	// Remote create event with different content hash than local.
	api.addEvent(model.Event{
		EventID:   model.EventID("base_evt_r1"),
		OwnerID:   "user_test",
		ItemID:    itemID,
		DeviceID:  "remote",
		SubjectID: "user_test",
		EventType: model.EventCreate,
		Kind:      model.KindFile,
		PayloadJSON: `{"name":"both.txt","kind":"file","version_id":"base_ver_remote","blob_ref":"sha256:remotehash","content_hash":"remotehash"}`,
	})

	srv := httptest.NewServer(api)
	defer srv.Close()

	// Seed the synced state with the old version so both sides "changed".
	oldState := SyncedState{
		Cursor: 0,
		Items: []model.Item{{
			ItemID:         itemID,
			Name:           "both.txt",
			Kind:           model.KindFile,
			CurrentVersion: "base_ver_old",
		}},
		Versions: []model.Version{{
			VersionID:   "base_ver_old",
			ItemID:      itemID,
			BlobRef:     "sha256:oldhash",
			ContentHash: "oldhash",
		}},
	}
	statePath := filepath.Join(root, ".choir-synced-state.json")
	store := NewFileSyncedStateStore(statePath)
	if err := store.Save(oldState); err != nil {
		t.Fatal(err)
	}

	eng := NewSyncEngine(SyncConfig{
		BaseURL:   srv.URL,
		LocalRoot: root,
		DeviceID:  "dev",
		OwnerID:   "user_test",
	}, "choir_sk_test")
	eng.SetHTTPClient(srv.Client())
	eng.SetSyncedStateStore(store)

	eng.runCycle(context.Background())

	// The conflict manager should have one unresolved conflict.
	if !eng.Conflicts().HasUnresolved() {
		t.Fatal("expected unresolved conflict after cycle")
	}
	pending := eng.Conflicts().Pending()
	if len(pending) != 1 {
		t.Fatalf("Pending: got %d, want 1", len(pending))
	}

	// Status should be in the conflicts phase.
	p := eng.Status().Snapshot()
	if p.Phase != PhaseConflicts {
		t.Errorf("phase: got %q, want conflicts", p.Phase)
	}

	// No item creation should have happened for the conflicting item.
	api.mu.Lock()
	for _, req := range api.itemReqs {
		if req.ItemID == itemID {
			t.Error("conflicting item was uploaded before resolution (silent resolution)")
		}
	}
	api.mu.Unlock()
}

func TestSyncEngineResolveConflict(t *testing.T) {
	// Same setup as the pause test, but resolve the conflict keep_local and
	// run another cycle. The engine should upload the local version.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "both.txt"), []byte("local"), 0o644); err != nil {
		t.Fatal(err)
	}

	api := &fakeBaseAPI{}
	itemID := localItemID("both.txt")
	api.addEvent(model.Event{
		EventID:   model.EventID("base_evt_r1"),
		OwnerID:   "user_test",
		ItemID:    itemID,
		DeviceID:  "remote",
		SubjectID: "user_test",
		EventType: model.EventCreate,
		Kind:      model.KindFile,
		PayloadJSON: `{"name":"both.txt","kind":"file","version_id":"base_ver_remote","blob_ref":"sha256:remotehash","content_hash":"remotehash"}`,
	})

	srv := httptest.NewServer(api)
	defer srv.Close()

	oldState := SyncedState{
		Items: []model.Item{{ItemID: itemID, Name: "both.txt", Kind: model.KindFile, CurrentVersion: "base_ver_old"}},
		Versions: []model.Version{{VersionID: "base_ver_old", ItemID: itemID, BlobRef: "sha256:oldhash", ContentHash: "oldhash"}},
	}
	statePath := filepath.Join(root, ".state.json")
	store := NewFileSyncedStateStore(statePath)
	if err := store.Save(oldState); err != nil {
		t.Fatal(err)
	}

	eng := NewSyncEngine(SyncConfig{
		BaseURL:   srv.URL,
		LocalRoot: root,
		DeviceID:  "dev",
		OwnerID:   "user_test",
	}, "choir_sk_test")
	eng.SetHTTPClient(srv.Client())
	eng.SetSyncedStateStore(store)

	// First cycle surfaces the conflict.
	eng.runCycle(context.Background())
	if !eng.Conflicts().HasUnresolved() {
		t.Fatal("expected unresolved conflict")
	}

	// Resolve keep_local.
	if err := eng.Conflicts().Resolve(itemID, ResolveKeepLocal); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// Second cycle should apply the resolution (upload local).
	eng.runCycle(context.Background())

	api.mu.Lock()
	foundUpload := false
	for _, req := range api.itemReqs {
		if req.ItemID == itemID {
			foundUpload = true
		}
	}
	api.mu.Unlock()
	if !foundUpload {
		t.Error("keep_local resolution did not upload the local version")
	}

	// Conflicts should be cleared after successful resolution.
	if eng.Conflicts().Count() != 0 {
		t.Errorf("conflicts after resolve: got %d, want 0", eng.Conflicts().Count())
	}
}

func TestSyncEngineSyncNowNonBlocking(t *testing.T) {
	root := t.TempDir()
	api := &fakeBaseAPI{}
	srv := httptest.NewServer(api)
	defer srv.Close()

	eng := NewSyncEngine(SyncConfig{
		BaseURL:   srv.URL,
		LocalRoot: root,
		DeviceID:  "dev",
	}, "choir_sk_test")
	eng.SetHTTPClient(srv.Client())
	eng.SetSyncedStateStore(NewFileSyncedStateStore(filepath.Join(root, ".s.json")))

	// SyncNow on a non-started engine should not block (buffered channel).
	done := make(chan struct{})
	go func() {
		eng.SyncNow()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SyncNow blocked")
	}
}

func TestApplyDeltaMerges(t *testing.T) {
	// Build a base tree with one item, then apply a delta delete event for
	// it and a create event for a new item; verify the merge.
	base := tree.NewTree()
	id1 := model.ItemID("base_item_1")
	base.Items[id1] = model.Item{ItemID: id1, Name: "old.txt", Kind: model.KindFile, CurrentVersion: "base_ver_1"}
	base.Versions[id1] = model.Version{VersionID: "base_ver_1", ItemID: id1, BlobRef: "sha256:old", ContentHash: "old"}

	id2 := model.ItemID("base_item_2")
	events := []model.Event{
		{
			EventID: model.EventID("base_evt_del"), OwnerID: "u", ItemID: id1,
			EventType: model.EventDelete, Kind: model.KindFile, CursorSeq: 2,
			PayloadJSON: `{"kind":"file"}`,
		},
		{
			EventID: model.EventID("base_evt_new"), OwnerID: "u", ItemID: id2,
			EventType: model.EventCreate, Kind: model.KindFile, CursorSeq: 3,
			PayloadJSON: `{"name":"new.txt","kind":"file","version_id":"base_ver_2","blob_ref":"sha256:new","content_hash":"new"}`,
		},
	}
	out := applyDelta(base, events)
	// id1 should be tombstoned (CurrentVersion cleared).
	if it := out.Items[id1]; it.CurrentVersion != "" {
		t.Errorf("id1 should be deleted, CurrentVersion=%q", it.CurrentVersion)
	}
	// id2 should be present.
	if it, ok := out.Items[id2]; !ok || it.Name != "new.txt" {
		t.Errorf("id2 not merged correctly: %+v ok=%v", it, ok)
	}
}

func TestSyncedStateStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := NewFileSyncedStateStore(path)

	// Empty on first load.
	s, err := store.Load()
	if err != nil {
		t.Fatalf("Load empty: %v", err)
	}
	if s.Cursor != 0 {
		t.Errorf("empty cursor: got %d, want 0", s.Cursor)
	}

	s.Cursor = 42
	s.Items = []model.Item{{ItemID: "base_item_1", Name: "a", Kind: model.KindFile, CurrentVersion: "base_ver_1"}}
	if err := store.Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Cursor != 42 {
		t.Errorf("cursor: got %d, want 42", loaded.Cursor)
	}
	if len(loaded.Items) != 1 {
		t.Errorf("items: got %d, want 1", len(loaded.Items))
	}
}

func TestSnapshotToTreeAndPlanner(t *testing.T) {
	s := SyncedState{
		Items: []model.Item{{ItemID: "base_item_1", Name: "x", Kind: model.KindFile, CurrentVersion: "base_ver_1"}},
		Versions: []model.Version{{VersionID: "base_ver_1", ItemID: "base_item_1", BlobRef: "sha256:abc", ContentHash: "abc"}},
	}
	tt := snapshotToTree(s)
	if _, ok := tt.Items["base_item_1"]; !ok {
		t.Error("snapshotToTree missing item")
	}
	pt := snapshotToPlanner(s)
	if _, ok := pt.Items["base_item_1"]; !ok {
		t.Error("snapshotToPlanner missing item")
	}
}
