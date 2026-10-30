package platform

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Ways the chunk existence check (HEAD) could fail, written before the code:
//
//  1. it reports a chunk the host does not hold (the snapshot would name a
//     chunk nobody can fetch);
//  2. it finds another computer's chunk (a cross-namespace oracle: the check
//     must stay in the caller's own namespace);
//  3. a found chunk is not claimed, so the GC grace window lapses before the
//     snapshot root that names it commits;
//  4. an unauthenticated caller can probe.
func TestFileCASChunkHeadFindsOnlyOwnChunksAndClaimsThem(t *testing.T) {
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), "")
	handler := NewHandler(service)
	chunk := []byte("sealed snapshot chunk")
	sum := sha256.Sum256(chunk)
	digest := hex.EncodeToString(sum[:])
	do := func(method, computerID string, trusted bool) int {
		req := httptest.NewRequest(method, "/internal/computers/files/chunks/"+digest+"?computer_id="+computerID, bytes.NewReader(chunk))
		if trusted {
			req.Header.Set("X-Internal-Caller", "true")
		}
		result := httptest.NewRecorder()
		handler.HandleFileCASChunk(result, req)
		return result.Code
	}
	if code := do(http.MethodHead, "computer-a", true); code != http.StatusNotFound {
		t.Fatalf("HEAD before the PUT = %d, want 404", code)
	}
	if code := do(http.MethodPut, "computer-a", true); code != http.StatusCreated {
		t.Fatalf("PUT = %d", code)
	}
	if code := do(http.MethodHead, "computer-a", true); code != http.StatusOK {
		t.Fatalf("HEAD after the PUT = %d, want 200", code)
	}
	if code := do(http.MethodHead, "computer-b", true); code != http.StatusNotFound {
		t.Fatalf("HEAD from another computer's namespace = %d, want 404", code)
	}
	if code := do(http.MethodHead, "computer-a", false); code != http.StatusForbidden {
		t.Fatalf("unauthenticated HEAD = %d, want 403", code)
	}

	// A claim refreshes the mtime.
	path, err := service.artifactPath(filepath.Join("sha256", "file-cas-chunks", "computer-a", digest))
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if code := do(http.MethodHead, "computer-a", true); code != http.StatusOK {
		t.Fatalf("HEAD = %d", code)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(info.ModTime()) > time.Hour {
		t.Fatalf("a claimed chunk kept its old mtime %s: the GC grace window would lapse", info.ModTime())
	}
}
