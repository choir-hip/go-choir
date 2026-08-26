package autoputer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/filecas"
)

func TestFileSyncUploadsVerifiedManifestAndSkipsUnchangedFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "note.txt"), []byte("sealed file contents"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "empty.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	chunks := make(map[string][]byte)
	var manifests [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer file-sync-token" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch {
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/internal/computers/files/chunks/"):
			digest := strings.TrimPrefix(r.URL.Path, "/internal/computers/files/chunks/")
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read chunk: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			sum := sha256.Sum256(data)
			if digest != hex.EncodeToString(sum[:]) {
				t.Errorf("chunk route digest %q does not match body", digest)
			}
			mu.Lock()
			chunks[digest] = data
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPut && r.URL.Path == "/internal/computers/files/root":
			var request struct {
				ComputerID   string `json:"computer_id"`
				Manifest     string `json:"manifest"`
				HeadSequence int64  `json:"head_sequence"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if request.ComputerID != "computer-test" || request.HeadSequence != 7 {
				t.Errorf("root request = %#v", request)
			}
			mu.Lock()
			manifests = append(manifests, []byte(request.Manifest))
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	cipher, err := computerevent.NewPrivateArtifactCipher("computer-test", key)
	if err != nil {
		t.Fatal(err)
	}
	service := newFileSync(root, server.URL, func(context.Context) (string, error) { return "file-sync-token", nil }, "computer-test", cipher, func(context.Context) (uint64, error) { return 7, nil }, nil)
	first, err := service.syncOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.files != 2 || first.chunksUploaded != 1 || first.root == "" {
		t.Fatalf("first sync = %#v", first)
	}

	mu.Lock()
	if len(manifests) != 1 || len(chunks) != 1 {
		mu.Unlock()
		t.Fatalf("uploads manifests=%d chunks=%d", len(manifests), len(chunks))
	}
	manifest, parseErr := filecas.ParseManifest(manifests[0])
	chunkCopy := make(map[string][]byte, len(chunks))
	for digest, data := range chunks {
		chunkCopy[digest] = append([]byte(nil), data...)
	}
	mu.Unlock()
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if err := manifest.VerifyRoot(); err != nil {
		t.Fatal(err)
	}
	if manifest.Root != first.root || len(manifest.Files) != 2 {
		t.Fatalf("manifest = %#v", manifest)
	}
	for _, entry := range manifest.Files {
		for _, digest := range entry.Chunks {
			sealed := chunkCopy[digest]
			if sealed == nil {
				t.Fatalf("manifest chunk %s was not uploaded", digest)
			}
			if _, err := filecas.OpenChunk(key, "computer-test", digest, sealed); err != nil {
				t.Fatalf("chunk %s does not open: %v", digest, err)
			}
		}
	}

	second, err := service.syncOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if second.chunksUploaded != 0 {
		t.Fatalf("second sync uploaded %d chunks", second.chunksUploaded)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(chunks) != 1 {
		t.Fatalf("second sync changed chunk uploads: %d", len(chunks))
	}
}

func TestFileSyncHandlerRequiresAuthAndReturnsBarrierResult(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("note"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	cipher, err := computerevent.NewPrivateArtifactCipher("computer-test", make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	service := newFileSync(root, server.URL, nil, "computer-test", cipher, nil, nil)

	unauthorized := httptest.NewRecorder()
	service.HandleSync(unauthorized, httptest.NewRequest(http.MethodPost, "/api/files/sync", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", unauthorized.Code)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/files/sync", nil)
	request.Header.Set("X-Authenticated-User", "user@example.com")
	response := httptest.NewRecorder()
	service.HandleSync(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var result struct {
		Root           string `json:"root"`
		Files          int    `json:"files"`
		ChunksUploaded int    `json:"chunks_uploaded"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Root == "" || result.Files != 1 || result.ChunksUploaded != 1 {
		t.Fatalf("response = %#v", result)
	}
}
