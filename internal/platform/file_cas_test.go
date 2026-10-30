package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/filecas"
	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/server"
)

func TestFileCASHTTPFlowAndWatermarkMonotonicity(t *testing.T) {
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), "")
	handler := NewHandler(service)
	chunk := []byte("encrypted chunk")
	sum := sha256.Sum256(chunk)
	digest := hex.EncodeToString(sum[:])
	putChunk := httptest.NewRequest(http.MethodPut, "/internal/computers/files/chunks/"+digest+"?computer_id=computer-filecas", bytes.NewReader(chunk))
	putChunk.Header.Set("X-Internal-Caller", "true")
	chunkResult := httptest.NewRecorder()
	handler.HandleFileCASChunk(chunkResult, putChunk)
	if chunkResult.Code != http.StatusCreated {
		t.Fatalf("put chunk = %d: %s", chunkResult.Code, chunkResult.Body.String())
	}
	manifest, err := filecas.BuildManifest("computer-filecas", []filecas.FileEntry{{Path: "a.txt", Mode: 0o600, Size: 1, Chunks: []string{digest}}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	rootJSON, _ := json.Marshal(fileCASRootRequest{ComputerID: "computer-filecas", Manifest: string(manifestJSON), HeadSequence: 7})
	putRoot := httptest.NewRequest(http.MethodPut, "/internal/computers/files/root", bytes.NewReader(rootJSON))
	putRoot.Header.Set("X-Internal-Caller", "true")
	rootResult := httptest.NewRecorder()
	handler.HandleFileCASRoot(rootResult, putRoot)
	if rootResult.Code != http.StatusCreated {
		t.Fatalf("put root = %d: %s", rootResult.Code, rootResult.Body.String())
	}
	getRoot := httptest.NewRequest(http.MethodGet, "/internal/computers/files/root?computer_id=computer-filecas&root="+manifest.Root, nil)
	getRoot.Header.Set("X-Internal-Caller", "true")
	getRootResult := httptest.NewRecorder()
	handler.HandleFileCASRoot(getRootResult, getRoot)
	if getRootResult.Code != http.StatusOK || !bytes.Equal(bytes.TrimSpace(getRootResult.Body.Bytes()), manifestJSON) {
		t.Fatalf("get root = %d: %s", getRootResult.Code, getRootResult.Body.String())
	}
	// The watermark is the newest snapshot root, read-only: vmctl reads it
	// as admission evidence. A replay watermark row an earlier build wrote
	// names a projection base the GC collects; it is not evidence.
	watermarkGET := func() *httptest.ResponseRecorder {
		get := httptest.NewRequest(http.MethodGet, "/internal/computers/files/watermark?computer_id=computer-filecas", nil)
		get.Header.Set("X-Internal-Caller", "true")
		got := httptest.NewRecorder()
		handler.HandleFileCASWatermark(got, get)
		return got
	}
	seedReplayWatermark(t, store, "computer-filecas", 10, "base-10")
	if got := watermarkGET(); got.Code != http.StatusNotFound {
		t.Fatalf("watermark GET with only a replay watermark row = %d, want 404: %s", got.Code, got.Body.String())
	}
	older, newer := strings.Repeat("b", 64), strings.Repeat("c", 64)
	for _, r := range []struct {
		root string
		head int64
	}{{newer, 12}, {older, 11}} {
		if err := store.RecordSnapshotRoot(context.Background(), "computer-filecas", r.root, r.head, []string{strings.Repeat("d", 64)}); err != nil {
			t.Fatal(err)
		}
	}
	seq, baseRef, err := store.RecoveryBase(context.Background(), "computer-filecas")
	if err != nil || seq != 12 || baseRef != newer {
		t.Fatalf("recovery base = %d, %q, %v; want the newest snapshot root (12)", seq, baseRef, err)
	}
	if got := watermarkGET(); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"base_ref":"`+newer+`"`) || !strings.Contains(got.Body.String(), `"watermark_sequence":12`) {
		t.Fatalf("watermark GET = %d: %s", got.Code, got.Body.String())
	}
	post := httptest.NewRequest(http.MethodPost, "/internal/computers/files/watermark", strings.NewReader(`{"computer_id":"computer-filecas","watermark_sequence":11,"base_ref":"`+strings.Repeat("a", 64)+`"}`))
	post.Header.Set("X-Internal-Caller", "true")
	refused := httptest.NewRecorder()
	handler.HandleFileCASWatermark(refused, post)
	if refused.Code != http.StatusMethodNotAllowed {
		t.Fatalf("watermark POST = %d, want 405: %s", refused.Code, refused.Body.String())
	}
	if seq, _, _ := store.RecoveryBase(context.Background(), "computer-filecas"); seq != 12 {
		t.Fatalf("refused POST changed the watermark to %d", seq)
	}
}

func TestGCFileChunksPreservesReachableAndNamespaces(t *testing.T) {
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), "")
	computerID := "computer-gc"
	live := []byte("live encrypted")
	stale := []byte("stale encrypted")
	liveSum, staleSum := sha256.Sum256(live), sha256.Sum256(stale)
	liveDigest, staleDigest := hex.EncodeToString(liveSum[:]), hex.EncodeToString(staleSum[:])
	if err := service.PinFileChunk(context.Background(), computerID, liveDigest, live); err != nil {
		t.Fatal(err)
	}
	if err := service.PinFileChunk(context.Background(), computerID, staleDigest, stale); err != nil {
		t.Fatal(err)
	}
	manifest, err := filecas.BuildManifest(computerID, []filecas.FileEntry{{Path: "live", Mode: 0o600, Size: 1, Chunks: []string{liveDigest}}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	manifestJSON, _ := json.Marshal(manifest)
	ref, err := service.pinFileManifest(computerID, manifest.Root, manifestJSON)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordFileRoot(context.Background(), computerID, manifest.Root, ref, 1); err != nil {
		t.Fatal(err)
	}
	staleRef, _ := fileCASChunkRef(computerID, staleDigest)
	stalePath, _ := service.artifactPath(staleRef)
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stalePath, old, old); err != nil {
		t.Fatal(err)
	}
	otherPath, err := service.artifactPath(filepath.Join("sha256", "other-namespace", staleDigest))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(otherPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherPath, stale, 0o640); err != nil {
		t.Fatal(err)
	}
	ensureGCRouteTables(t, store)
	report, err := service.RunArtifactGC(context.Background(), ArtifactGCConfig{Mode: ArtifactGCModeActive, Grace: time.Hour})
	if err != nil || report.Namespaces["file-cas-chunks"].Deleted != 1 {
		t.Fatalf("GC = %+v, %v", report.Namespaces["file-cas-chunks"], err)
	}
	if _, err := service.GetFileChunk(context.Background(), computerID, liveDigest); err != nil {
		t.Fatalf("live chunk removed: %v", err)
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("stale chunk remains: %v", err)
	}
	if _, err := os.Stat(otherPath); err != nil {
		t.Fatalf("other namespace changed: %v", err)
	}
}

// The projection-base routes are gone: the host keeps no bases and the guest
// recovers from its own sealed snapshots. Ways this could fail: a route is
// still registered (a base would be served until the GC collects it), or the
// removal took a neighbouring route (the watermark read vmctl uses) with it.
func TestProjectionBaseRoutesAreNotRegistered(t *testing.T) {
	store, root := openTestPlatformStore(t)
	const token = "projection-base-routes-token-0123456789abcdef01234567"
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, token)
	defer restore()
	srv := server.NewServer("corpusd-test", "0")
	RegisterRoutes(srv, NewHandler(NewService(store, filepath.Join(root, "artifacts"), "")))
	call := func(method, path string) int {
		req := httptest.NewRequest(method, path, nil)
		req.RemoteAddr = "127.0.0.1:5555"
		internalMarker(req)
		req.Header.Set(hostauth.Header, token)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec.Code
	}
	digest := strings.Repeat("a", 64)
	for _, path := range []string{
		"/internal/computers/files/projection-base/descriptor?computer_id=c&base_ref=" + digest,
		"/internal/computers/files/projection-base/blob?computer_id=c&base_ref=" + digest,
		"/internal/computers/files/projection-base/part?computer_id=c&base_ref=" + digest + "&part=" + digest,
		"/internal/computers/projection-base/jobs?computer_id=c",
	} {
		if code := call(http.MethodGet, path); code != http.StatusNotFound {
			t.Fatalf("GET %s = %d, want 404 (route not registered)", path, code)
		}
	}
	if err := store.RecordSnapshotRoot(context.Background(), "c", digest, 3, []string{digest}); err != nil {
		t.Fatal(err)
	}
	if code := call(http.MethodGet, "/internal/computers/files/watermark?computer_id=c"); code != http.StatusOK {
		t.Fatalf("watermark GET = %d, want 200: the removal must leave the read vmctl uses", code)
	}
}

// seedReplayWatermark writes a replay watermark row directly: nothing
// advertises one any more, but vmctl still reads the rows an earlier build
// wrote.
func seedReplayWatermark(t *testing.T, s *Store, computerID string, seq int64, baseRef string) {
	t.Helper()
	if _, err := s.db.ExecContext(context.Background(), `INSERT INTO computer_replay_watermarks (computer_id,watermark_sequence,base_ref,updated_at) VALUES (?,?,?,?) ON CONFLICT (computer_id) DO UPDATE SET watermark_sequence=excluded.watermark_sequence,base_ref=excluded.base_ref,updated_at=excluded.updated_at`, computerID, seq, baseRef, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
}
