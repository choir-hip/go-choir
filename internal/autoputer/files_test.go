package autoputer

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// setupFileTest creates a temporary root directory and a FilesHandler for testing.
func setupFileTest(t *testing.T) (*FilesHandler, string) {
	t.Helper()
	rootDir := t.TempDir()
	fh := NewFilesHandler(rootDir)
	return fh, rootDir
}

// authRequest creates an httptest.Request with the X-Authenticated-User header
// set, simulating a request that has passed through the proxy's JWT validation.
func authRequest(method, path string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("X-Authenticated-User", "test-user@example.com")
	return req
}

func authBodyRequest(method, path string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("X-Authenticated-User", "test-user@example.com")
	return req
}

// --- GET /api/files (root listing) ---

func TestListRootDirectory(t *testing.T) {
	fh, root := setupFileTest(t)

	// Create test entries in root.
	os.MkdirAll(filepath.Join(root, "documents"), 0o755)
	os.WriteFile(filepath.Join(root, "readme.txt"), []byte("hello"), 0o644)

	req := authRequest(http.MethodGet, "/api/files")
	w := httptest.NewRecorder()
	fh.HandleListRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var entries []FileEntry
	if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Sort for deterministic comparison.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	if entries[0].Name != "documents" {
		t.Errorf("expected first entry name 'documents', got %q", entries[0].Name)
	}
	if entries[0].Type != "directory" {
		t.Errorf("expected documents type 'directory', got %q", entries[0].Type)
	}
	if entries[1].Name != "readme.txt" {
		t.Errorf("expected second entry name 'readme.txt', got %q", entries[1].Name)
	}
	if entries[1].Type != "file" {
		t.Errorf("expected readme.txt type 'file', got %q", entries[1].Type)
	}
	if entries[1].Size != 5 {
		t.Errorf("expected readme.txt size 5, got %d", entries[1].Size)
	}
	if entries[1].Modified == "" {
		t.Error("expected non-empty modified field")
	}
}

func TestUpdateFileWritesTextContent(t *testing.T) {
	fh, root := setupFileTest(t)

	parent := filepath.Join(root, "documents")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}

	req := authBodyRequest(http.MethodPut, "/api/files/documents/note.txt", strings.NewReader("hello texture"))
	w := httptest.NewRecorder()
	fh.HandleFileByPath(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	got, err := os.ReadFile(filepath.Join(root, "documents", "note.txt"))
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if string(got) != "hello texture" {
		t.Fatalf("saved content = %q, want %q", string(got), "hello texture")
	}
}

// --- GET /api/files/{path} (subdirectory listing or file download) ---

func TestGetSubdirectoryListing(t *testing.T) {
	fh, root := setupFileTest(t)

	// Create subdirectory with entries.
	os.MkdirAll(filepath.Join(root, "documents"), 0o755)
	os.WriteFile(filepath.Join(root, "documents", "notes.txt"), []byte("my notes"), 0o644)

	req := authRequest(http.MethodGet, "/api/files/documents")
	w := httptest.NewRecorder()
	fh.HandleFileByPath(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var entries []FileEntry
	if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "notes.txt" {
		t.Errorf("expected name 'notes.txt', got %q", entries[0].Name)
	}
}

func TestGetNonexistentPathReturns404(t *testing.T) {
	fh, _ := setupFileTest(t)

	req := authRequest(http.MethodGet, "/api/files/nonexistent")
	w := httptest.NewRecorder()
	fh.HandleFileByPath(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	var errResp FileErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error != "not found" {
		t.Errorf("expected error 'not found', got %q", errResp.Error)
	}
}

func TestGetFileDownload(t *testing.T) {
	fh, root := setupFileTest(t)

	content := []byte("file content for download")
	os.WriteFile(filepath.Join(root, "download.txt"), content, 0o644)

	req := authRequest(http.MethodGet, "/api/files/download.txt")
	w := httptest.NewRecorder()
	fh.HandleFileByPath(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Check Content-Disposition header.
	cd := w.Header().Get("Content-Disposition")
	if cd != `attachment; filename="download.txt"` {
		t.Errorf("expected Content-Disposition for download, got %q", cd)
	}

	if w.Body.String() != string(content) {
		t.Errorf("expected body %q, got %q", string(content), w.Body.String())
	}
}

// --- POST /api/files/{path} (create directory) ---

func TestCreateDirectory(t *testing.T) {
	fh, root := setupFileTest(t)

	req := authRequest(http.MethodPost, "/api/files/new-folder")
	w := httptest.NewRecorder()
	fh.HandleFileByPath(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify directory was actually created.
	info, err := os.Stat(filepath.Join(root, "new-folder"))
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected a directory")
	}
}

func TestCreateDirectoryReturnsConflictIfExists(t *testing.T) {
	fh, root := setupFileTest(t)

	// Pre-create the directory.
	os.MkdirAll(filepath.Join(root, "existing"), 0o755)

	req := authRequest(http.MethodPost, "/api/files/existing")
	w := httptest.NewRecorder()
	fh.HandleFileByPath(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}

	var errResp FileErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error != "directory already exists" {
		t.Errorf("expected error 'directory already exists', got %q", errResp.Error)
	}
}

func TestCreateDirectoryConflictWithFile(t *testing.T) {
	fh, root := setupFileTest(t)

	// Pre-create a file with the same name.
	os.WriteFile(filepath.Join(root, "conflict"), []byte("data"), 0o644)

	req := authRequest(http.MethodPost, "/api/files/conflict")
	w := httptest.NewRecorder()
	fh.HandleFileByPath(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestCreateDirectoryParentNotFound(t *testing.T) {
	fh, _ := setupFileTest(t)

	// Try to create a directory in a nonexistent parent.
	req := authRequest(http.MethodPost, "/api/files/nonexistent/new-folder")
	w := httptest.NewRecorder()
	fh.HandleFileByPath(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// --- DELETE /api/files/{path} (delete file/folder) ---

func TestDeleteFile(t *testing.T) {
	fh, root := setupFileTest(t)

	// Create a file to delete.
	os.WriteFile(filepath.Join(root, "temp.txt"), []byte("temp"), 0o644)

	req := authRequest(http.MethodDelete, "/api/files/temp.txt")
	w := httptest.NewRecorder()
	fh.HandleFileByPath(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	// Verify file is gone.
	if _, err := os.Stat(filepath.Join(root, "temp.txt")); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}

func TestDeleteEmptyDirectory(t *testing.T) {
	fh, root := setupFileTest(t)

	// Create an empty directory to delete.
	os.MkdirAll(filepath.Join(root, "empty-dir"), 0o755)

	req := authRequest(http.MethodDelete, "/api/files/empty-dir")
	w := httptest.NewRecorder()
	fh.HandleFileByPath(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	// Verify directory is gone.
	if _, err := os.Stat(filepath.Join(root, "empty-dir")); !os.IsNotExist(err) {
		t.Error("expected directory to be deleted")
	}
}

func TestDeleteNonexistentReturns404(t *testing.T) {
	fh, _ := setupFileTest(t)

	req := authRequest(http.MethodDelete, "/api/files/nonexistent")
	w := httptest.NewRecorder()
	fh.HandleFileByPath(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeleteNonEmptyDirectoryReturns409(t *testing.T) {
	fh, root := setupFileTest(t)

	// Create a non-empty directory.
	os.MkdirAll(filepath.Join(root, "notempty"), 0o755)
	os.WriteFile(filepath.Join(root, "notempty", "file.txt"), []byte("data"), 0o644)

	req := authRequest(http.MethodDelete, "/api/files/notempty")
	w := httptest.NewRecorder()
	fh.HandleFileByPath(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

// --- Path traversal protection ---

func TestPathTraversalBlocked(t *testing.T) {
	fh, _ := setupFileTest(t)

	// Test various path traversal patterns - resolvePath should catch them
	// before any filesystem access.
	paths := []string{
		"../../etc/passwd",
		"../../../tmp/something",
		"../..",
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			_, err := fh.resolvePath(p)
			if err == nil {
				t.Errorf("expected error for path traversal %q, but got nil", p)
			}
		})
	}
}

func TestPathTraversalHTTPBlocked(t *testing.T) {
	fh, _ := setupFileTest(t)

	// Simulate a URL with path traversal (as the HTTP mux would see it after
	// stripping the prefix). Note: in real routing, Go's http.ServeMux
	// will clean path components, so this is defense-in-depth.
	req := authRequest(http.MethodGet, "/api/files/..%2F..%2Fetc%2Fpasswd")
	w := httptest.NewRecorder()
	fh.HandleFileByPath(w, req)

	// The path gets cleaned by resolvePath - should be blocked or 404.
	// Either way, no file contents from outside the root should be returned.
	if w.Code != http.StatusForbidden && w.Code != http.StatusNotFound {
		t.Errorf("expected 403 or 404, got %d", w.Code)
	}
}

// --- Special characters in filenames ---

func TestSpecialCharactersInFilenames(t *testing.T) {
	fh, root := setupFileTest(t)

	// Create files with special characters.
	specialNames := []string{
		"file with spaces.txt",
		"file(1).txt",
		"file_underscore.txt",
		"file-dash.txt",
	}
	for _, name := range specialNames {
		err := os.WriteFile(filepath.Join(root, name), []byte("content"), 0o644)
		if err != nil {
			t.Fatalf("failed to create file %q: %v", name, err)
		}
	}

	req := authRequest(http.MethodGet, "/api/files")
	w := httptest.NewRecorder()
	fh.HandleListRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var entries []FileEntry
	if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(entries) != len(specialNames) {
		t.Fatalf("expected %d entries, got %d", len(specialNames), len(entries))
	}

	// Check all names are present.
	found := map[string]bool{}
	for _, e := range entries {
		found[e.Name] = true
	}
	for _, name := range specialNames {
		if !found[name] {
			t.Errorf("expected entry %q not found", name)
		}
	}
}

// --- Unsupported methods ---

func TestFileByPathRejectsUnsupportedMethods(t *testing.T) {
	fh, _ := setupFileTest(t)

	for _, method := range []string{http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			req := authRequest(method, "/api/files/something")
			w := httptest.NewRecorder()
			fh.HandleFileByPath(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected 405 for %s, got %d", method, w.Code)
			}
		})
	}
}

// --- End-to-end workflow test ---
