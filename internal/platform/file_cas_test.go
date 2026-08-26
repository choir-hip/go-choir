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
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/filecas"
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
	for _, input := range []fileCASWatermarkRequest{{ComputerID: "computer-filecas", WatermarkSequence: 10, BaseRef: "base-10"}, {ComputerID: "computer-filecas", WatermarkSequence: 9, BaseRef: "base-9"}} {
		body, _ := json.Marshal(input)
		req := httptest.NewRequest(http.MethodPost, "/internal/computers/files/watermark", bytes.NewReader(body))
		req.Header.Set("X-Internal-Caller", "true")
		result := httptest.NewRecorder()
		handler.HandleFileCASWatermark(result, req)
		if result.Code != http.StatusOK {
			t.Fatalf("put watermark = %d: %s", result.Code, result.Body.String())
		}
	}
	seq, baseRef, err := store.ReplayWatermark(context.Background(), "computer-filecas")
	if err != nil || seq != 10 || baseRef != "base-10" {
		t.Fatalf("watermark = %d, %q, %v", seq, baseRef, err)
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
	removed, err := service.GCFileChunks(context.Background(), computerID, time.Hour)
	if err != nil || removed != 1 {
		t.Fatalf("GC = %d, %v", removed, err)
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
