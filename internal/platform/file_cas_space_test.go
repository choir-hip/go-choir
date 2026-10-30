package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/filecas"
	"github.com/yusefmosiah/go-choir/internal/persistentdisk"
)

// Ways the host free-space floor on chunk writes could fail, written before
// the code:
//
//  1. a chunk is stored with the volume under the floor, so one computer's
//     snapshots fill the disk the tape is on;
//  2. a volume it cannot read is taken as roomy (fails open);
//  3. the refusal is not a status the guest reads as "not stored" (it must
//     not be 2xx), or does not say why;
//  4. the floor also blocks reads or the existence check, which need no
//     space, so a restore on a full host fails.
func TestFileCASChunkWriteKeepsTheHostFreeSpaceFloor(t *testing.T) {
	store, root := openTestPlatformStore(t)
	handler := NewHandler(NewService(store, filepath.Join(root, "artifacts"), ""))
	saved := fileCASVolumeUsage
	t.Cleanup(func() { fileCASVolumeUsage = saved })

	const total = 1000 << 30 // a 1000 GiB volume: the floor is 50 GiB
	var avail uint64
	var statErr error
	fileCASVolumeUsage = func(string) (persistentdisk.Usage, error) {
		return persistentdisk.Usage{TotalBytes: total, UsedBytes: total - avail, AvailBytes: avail}, statErr
	}
	put := func(body string) (int, string) {
		sum := sha256.Sum256([]byte(body))
		req := httptest.NewRequest(http.MethodPut, "/internal/computers/files/chunks/"+hex.EncodeToString(sum[:])+"?computer_id=computer-a", bytes.NewReader([]byte(body)))
		req.Header.Set("X-Internal-Caller", "true")
		result := httptest.NewRecorder()
		handler.HandleFileCASChunk(result, req)
		return result.Code, result.Body.String()
	}
	get := func(method, body string) int {
		sum := sha256.Sum256([]byte(body))
		req := httptest.NewRequest(method, "/internal/computers/files/chunks/"+hex.EncodeToString(sum[:])+"?computer_id=computer-a", nil)
		req.Header.Set("X-Internal-Caller", "true")
		result := httptest.NewRecorder()
		handler.HandleFileCASChunk(result, req)
		return result.Code
	}

	avail = 50 << 30
	if code, body := put("at the floor"); code != http.StatusCreated {
		t.Fatalf("PUT at the floor = %d %s, want 201", code, body)
	}
	avail = 50<<30 - 1
	code, body := put("under the floor")
	if code != http.StatusInsufficientStorage || !strings.Contains(body, "floor") {
		t.Fatalf("PUT under the floor = %d %s, want 507 naming the floor", code, body)
	}
	if code := get(http.MethodHead, "under the floor"); code != http.StatusNotFound {
		t.Fatalf("a refused chunk is held: HEAD = %d", code)
	}
	// Reads and the existence check need no space.
	if code := get(http.MethodGet, "at the floor"); code != http.StatusOK {
		t.Fatalf("GET under the floor = %d, want 200", code)
	}
	if code := get(http.MethodHead, "at the floor"); code != http.StatusOK {
		t.Fatalf("HEAD under the floor = %d, want 200", code)
	}

	avail, statErr = 900<<30, errors.New("statfs failed")
	if code, body := put("unreadable volume"); code != http.StatusInsufficientStorage || !strings.Contains(body, "fails closed") {
		t.Fatalf("PUT with an unreadable volume = %d %s, want 507 failing closed", code, body)
	}

	if got := fileCASFreeFloorBytes(100 << 30); got != 8<<30 {
		t.Fatalf("floor of a 100 GiB volume = %d, want the 8 GiB minimum", got)
	}
}

// A root that names a chunk the host does not hold is refused, so a guest
// whose cache vouched for a collected chunk sends it again instead of
// recording a file nobody can restore.
func TestFileCASRootRefusesAManifestNamingAMissingChunk(t *testing.T) {
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), "")
	handler := NewHandler(service)
	held := []byte("held chunk")
	heldSum := sha256.Sum256(held)
	heldDigest := hex.EncodeToString(heldSum[:])
	if err := service.PinFileChunk(context.Background(), "computer-a", heldDigest, held); err != nil {
		t.Fatal(err)
	}
	missingSum := sha256.Sum256([]byte("never sent"))
	missingDigest := hex.EncodeToString(missingSum[:])
	putRoot := func(chunks ...string) (int, string) {
		manifest, err := filecas.BuildManifest("computer-a", []filecas.FileEntry{{Path: "f", Mode: 0o600, Size: 1, Chunks: chunks}}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(manifest)
		body, _ := json.Marshal(fileCASRootRequest{ComputerID: "computer-a", Manifest: string(data), HeadSequence: 1})
		req := httptest.NewRequest(http.MethodPut, "/internal/computers/files/root", bytes.NewReader(body))
		req.Header.Set("X-Internal-Caller", "true")
		result := httptest.NewRecorder()
		handler.HandleFileCASRoot(result, req)
		return result.Code, result.Body.String()
	}
	if code, body := putRoot(heldDigest, missingDigest); code != http.StatusConflict || !strings.Contains(body, missingDigest) {
		t.Fatalf("root naming a missing chunk = %d %s, want 409 naming it", code, body)
	}
	roots, err := store.LatestFileRoots(context.Background(), "computer-a", 1)
	if err != nil || len(roots) != 0 {
		t.Fatalf("a refused root was recorded: %v %v", roots, err)
	}
	if code, body := putRoot(heldDigest); code != http.StatusCreated {
		t.Fatalf("root naming held chunks = %d %s, want 201", code, body)
	}
}
