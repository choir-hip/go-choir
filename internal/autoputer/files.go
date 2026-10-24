package autoputer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/server"
)

// requireAuth checks that the proxy-authenticated user identity arrived over
// the host peer transport. A guest can assert the header itself, so it is not
// an identity unless its transport is host-sourced.
func requireAuth(r *http.Request) error {
	_, err := server.AuthenticatedOwner(r)
	return err
}

// FileEntry represents a single file or directory in a listing response.
type FileEntry struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // "file" or "directory"
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

// FileErrorResponse represents an error returned by the file API.
type FileErrorResponse struct {
	Error string `json:"error"`
}

// FileChangeEvent is emitted after an authenticated Files mutation succeeds.
type FileChangeEvent struct {
	Operation  string `json:"operation"`
	Path       string `json:"path"`
	ParentPath string `json:"parent_path"`
	Name       string `json:"name"`
	EntryType  string `json:"entry_type"`
	Size       int64  `json:"size,omitempty"`
	// FromPath is the source of a move ("moved").
	FromPath string `json:"from_path,omitempty"`
}

// FileChangeObserver receives durable Files mutation notifications.
type FileChangeObserver func(r *http.Request, event FileChangeEvent)

// FilesHandler provides HTTP handlers for file browser operations.
type FilesHandler struct {
	rootDir  string
	observer FileChangeObserver
	// beforeMoveRename is a test seam between the destination check and the rename.
	beforeMoveRename func(destPath string)
	// moveGate admits the move verb (PATCH). Nil or false answers 405, as
	// before the verb existed (the file object's switch, files.object).
	moveGate func(*http.Request) bool
}

// SetMoveGate sets the function that admits PATCH moves.
func (fh *FilesHandler) SetMoveGate(gate func(*http.Request) bool) {
	fh.moveGate = gate
}

// NewFilesHandler creates a new file browser handler rooted at rootDir.
// If rootDir is empty, the AUTOPUTER_FILES_ROOT env var is used, falling back
// to /tmp/go-choir-files.
func NewFilesHandler(rootDir string) *FilesHandler {
	return NewFilesHandlerWithObserver(rootDir, nil)
}

// NewFilesHandlerWithObserver creates a file browser handler and emits
// mutation events to observer after successful writes.
func NewFilesHandlerWithObserver(rootDir string, observer FileChangeObserver) *FilesHandler {
	rootDir = provideriface.ResolveFilesRoot(rootDir)
	// Ensure root directory exists.
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		log.Printf("files: could not create root directory %s: %v", rootDir, err)
	}
	return &FilesHandler{rootDir: rootDir, observer: observer}
}

func (fh *FilesHandler) relativePath(absPath string) string {
	rel, err := filepath.Rel(fh.rootDir, absPath)
	if err != nil || rel == "." {
		return ""
	}
	return filepath.ToSlash(rel)
}

func (fh *FilesHandler) emitChange(r *http.Request, event FileChangeEvent) {
	if fh.observer == nil {
		return
	}
	event.Path = strings.Trim(event.Path, "/")
	event.ParentPath = strings.Trim(event.ParentPath, "/")
	if event.ParentPath == "." {
		event.ParentPath = ""
	}
	event.Name = strings.TrimSpace(event.Name)
	event.Operation = strings.TrimSpace(event.Operation)
	event.EntryType = strings.TrimSpace(event.EntryType)
	event.FromPath = strings.Trim(event.FromPath, "/")
	fh.observer(r, event)
}

// resolvePath safely resolves a user-supplied relative path against the
// autoputer root. It returns an error if the resolved path escapes the root.
func (fh *FilesHandler) resolvePath(relativePath string) (string, error) {
	// Remove leading slashes to treat the path as relative, then clean.
	rel := strings.TrimLeft(relativePath, "/")
	cleaned := filepath.Clean(rel)

	// filepath.Join with a cleaned relative path will always produce a path
	// under root. But if cleaned starts with "..", Join will walk up. Check
	// for that explicitly.
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes autoputer root")
	}

	absPath := filepath.Join(fh.rootDir, cleaned)

	// Final safety check: the result must be under the root.
	if !strings.HasPrefix(absPath, fh.rootDir+string(filepath.Separator)) && absPath != fh.rootDir {
		return "", fmt.Errorf("path escapes autoputer root")
	}
	// The check above is lexical. A symlinked directory in the path would
	// carry any verb out of the root (a read, a write, a move), and the
	// walk that records files never follows one, so no parent component of
	// the path may be a symlink. The last component is the verb's own to
	// judge (a move refuses a symlink source; a delete removes the link).
	parts := strings.Split(cleaned, string(filepath.Separator))
	cur := fh.rootDir
	for _, part := range parts[:len(parts)-1] {
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if err != nil {
			break // absent, or not a directory: the verb reports it
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("path crosses a symlink")
		}
	}
	return absPath, nil
}

// HandleListRoot handles GET /api/files — lists the root directory contents.
// It verifies the X-Authenticated-User header as defense-in-depth auth gating
// (the proxy validates the JWT and injects this header).
func (fh *FilesHandler) HandleListRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeFileError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := requireAuth(r); err != nil {
		writeFileError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	fh.listDirectory(w, fh.rootDir)
}

// HandleFileByPath handles GET/POST/DELETE /api/files/{path} — operates on
// a specific file or directory identified by the URL path suffix.
// It verifies the X-Authenticated-User header as defense-in-depth auth gating.
func (fh *FilesHandler) HandleFileByPath(w http.ResponseWriter, r *http.Request) {
	// Verify authentication before any file operations.
	if err := requireAuth(r); err != nil {
		writeFileError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Extract the path after "/api/files/".
	suffix := strings.TrimPrefix(r.URL.Path, "/api/files/")
	if suffix == "" {
		// Redirect to root listing.
		fh.HandleListRoot(w, r)
		return
	}

	absPath, err := fh.resolvePath(suffix)
	if err != nil {
		writeFileError(w, http.StatusForbidden, "access denied")
		return
	}

	switch r.Method {
	case http.MethodGet:
		fh.handleGet(w, r, absPath)
	case http.MethodPost:
		fh.handleCreateDirectory(w, r, absPath)
	case http.MethodPut:
		fh.handleUpdateFile(w, r, absPath)
	case http.MethodDelete:
		fh.handleDelete(w, r, absPath)
	case http.MethodPatch:
		if fh.moveGate == nil || !fh.moveGate(r) {
			writeFileError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		fh.handleMove(w, r, absPath)
	default:
		writeFileError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleGet either lists a directory's contents or serves a file for download.
func (fh *FilesHandler) handleGet(w http.ResponseWriter, r *http.Request, absPath string) {
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeFileError(w, http.StatusNotFound, "not found")
			return
		}
		if os.IsPermission(err) {
			writeFileError(w, http.StatusForbidden, "access denied")
			return
		}
		writeFileError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if info.IsDir() {
		fh.listDirectory(w, absPath)
		return
	}

	// Serve file for download.
	fh.serveFile(w, r, absPath, info)
}

// listDirectory returns a JSON array of FileEntry objects for the given directory.
func (fh *FilesHandler) listDirectory(w http.ResponseWriter, dirPath string) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeFileError(w, http.StatusNotFound, "directory not found")
			return
		}
		if os.IsPermission(err) {
			writeFileError(w, http.StatusForbidden, "access denied")
			return
		}
		writeFileError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var fileEntries []FileEntry
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue // skip entries we can't stat
		}

		entryType := "file"
		if entry.IsDir() {
			entryType = "directory"
		}

		fileEntries = append(fileEntries, FileEntry{
			Name:     entry.Name(),
			Type:     entryType,
			Size:     info.Size(),
			Modified: info.ModTime().UTC().Format(time.RFC3339),
		})
	}

	// Return empty array instead of null when no entries.
	if fileEntries == nil {
		fileEntries = []FileEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(fileEntries)
}

// serveFile streams a file with appropriate headers. Downloads remain the
// default, while first-class media apps can request inline rendering.
func (fh *FilesHandler) serveFile(w http.ResponseWriter, r *http.Request, absPath string, info fs.FileInfo) {
	file, err := os.Open(absPath)
	if err != nil {
		if os.IsPermission(err) {
			writeFileError(w, http.StatusForbidden, "access denied")
			return
		}
		writeFileError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer file.Close()

	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(info.Name())))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	disposition := "attachment"
	if r.URL.Query().Get("disposition") == "inline" || r.URL.Query().Get("inline") == "1" {
		disposition = "inline"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disposition, info.Name()))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

// handleCreateDirectory creates a new directory at the given path.
func (fh *FilesHandler) handleCreateDirectory(w http.ResponseWriter, r *http.Request, absPath string) {
	// Check if it already exists.
	info, err := os.Stat(absPath)
	if err == nil {
		// Path already exists.
		if info.IsDir() {
			writeFileError(w, http.StatusConflict, "directory already exists")
		} else {
			writeFileError(w, http.StatusConflict, "a file with that name already exists")
		}
		return
	}
	if !os.IsNotExist(err) {
		if os.IsPermission(err) {
			writeFileError(w, http.StatusForbidden, "access denied")
			return
		}
		writeFileError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Ensure parent directory exists.
	parentDir := filepath.Dir(absPath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		writeFileError(w, http.StatusNotFound, "parent directory not found")
		return
	}

	if err := os.Mkdir(absPath, 0o755); err != nil {
		writeFileError(w, http.StatusInternalServerError, "failed to create directory")
		return
	}

	relPath := fh.relativePath(absPath)
	fh.emitChange(r, FileChangeEvent{
		Operation:  "created",
		Path:       relPath,
		ParentPath: filepath.ToSlash(filepath.Dir(relPath)),
		Name:       filepath.Base(absPath),
		EntryType:  "directory",
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "created",
		"message": "directory created",
	})
}

// handleUpdateFile writes the request body to a regular file. If the file
// does not exist, it is created. Parent directories must already exist.
func (fh *FilesHandler) handleUpdateFile(w http.ResponseWriter, r *http.Request, absPath string) {
	info, err := os.Stat(absPath)
	operation := "updated"
	if os.IsNotExist(err) {
		operation = "created"
	}
	if err == nil && info.IsDir() {
		writeFileError(w, http.StatusConflict, "path is a directory")
		return
	}
	if err != nil && !os.IsNotExist(err) {
		if os.IsPermission(err) {
			writeFileError(w, http.StatusForbidden, "access denied")
			return
		}
		writeFileError(w, http.StatusInternalServerError, "internal error")
		return
	}

	parentDir := filepath.Dir(absPath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		writeFileError(w, http.StatusNotFound, "parent directory not found")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeFileError(w, http.StatusInternalServerError, "failed to read body")
		return
	}

	mode := os.FileMode(0o644)
	if info != nil {
		mode = info.Mode()
		if mode == 0 {
			mode = 0o644
		}
	}

	if err := os.WriteFile(absPath, body, mode); err != nil {
		if os.IsPermission(err) {
			writeFileError(w, http.StatusForbidden, "access denied")
			return
		}
		writeFileError(w, http.StatusInternalServerError, "failed to write file")
		return
	}

	relPath := fh.relativePath(absPath)
	fh.emitChange(r, FileChangeEvent{
		Operation:  operation,
		Path:       relPath,
		ParentPath: filepath.ToSlash(filepath.Dir(relPath)),
		Name:       filepath.Base(absPath),
		EntryType:  "file",
		Size:       int64(len(body)),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "updated",
		"message": "file saved",
	})
}

// handleDelete removes a file or directory at the given path.
func (fh *FilesHandler) handleDelete(w http.ResponseWriter, r *http.Request, absPath string) {
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeFileError(w, http.StatusNotFound, "not found")
			return
		}
		if os.IsPermission(err) {
			writeFileError(w, http.StatusForbidden, "access denied")
			return
		}
		writeFileError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if info.IsDir() {
		// Only remove empty directories to prevent accidental data loss.
		entries, err := os.ReadDir(absPath)
		if err != nil {
			writeFileError(w, http.StatusInternalServerError, "failed to read directory")
			return
		}
		if len(entries) > 0 {
			writeFileError(w, http.StatusConflict, "directory not empty")
			return
		}
		if err := os.Remove(absPath); err != nil {
			writeFileError(w, http.StatusInternalServerError, "failed to delete directory")
			return
		}
	} else {
		if err := os.Remove(absPath); err != nil {
			writeFileError(w, http.StatusInternalServerError, "failed to delete file")
			return
		}
	}

	entryType := "file"
	if info.IsDir() {
		entryType = "directory"
	}
	relPath := fh.relativePath(absPath)
	fh.emitChange(r, FileChangeEvent{
		Operation:  "deleted",
		Path:       relPath,
		ParentPath: filepath.ToSlash(filepath.Dir(relPath)),
		Name:       filepath.Base(absPath),
		EntryType:  entryType,
		Size:       info.Size(),
	})

	w.WriteHeader(http.StatusNoContent)
}

// fileMoveRequest is the body of PATCH /api/files/{path}.
type fileMoveRequest struct {
	MoveTo string `json:"move_to"`
}

// handleMove renames or moves one regular file to move_to (a path under the
// root whose parent exists and which does not exist). Directories are not
// moved in this slice.
func (fh *FilesHandler) handleMove(w http.ResponseWriter, r *http.Request, absPath string) {
	var req fileMoveRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&req); err != nil || strings.TrimSpace(req.MoveTo) == "" {
		writeFileError(w, http.StatusBadRequest, "move_to is required")
		return
	}
	destPath, err := fh.resolvePath(req.MoveTo)
	if err != nil || destPath == fh.rootDir || absPath == fh.rootDir {
		writeFileError(w, http.StatusForbidden, "access denied")
		return
	}
	info, err := os.Lstat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeFileError(w, http.StatusNotFound, "not found")
			return
		}
		writeFileError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !info.Mode().IsRegular() {
		writeFileError(w, http.StatusConflict, "only a regular file can be moved")
		return
	}
	if _, err := os.Lstat(destPath); err == nil {
		writeFileError(w, http.StatusConflict, "destination exists")
		return
	} else if !os.IsNotExist(err) {
		writeFileError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if parent, err := os.Stat(filepath.Dir(destPath)); err != nil || !parent.IsDir() {
		writeFileError(w, http.StatusNotFound, "parent directory not found")
		return
	}
	if fh.beforeMoveRename != nil {
		fh.beforeMoveRename(destPath)
	}
	// The existence check above is only a fast refusal. The move itself never
	// replaces a destination: one created in the gap (a concurrent PUT or move)
	// is refused, not destroyed.
	if err := renameNoReplace(absPath, destPath); err != nil {
		if errors.Is(err, fs.ErrExist) {
			writeFileError(w, http.StatusConflict, "destination exists")
			return
		}
		writeFileError(w, http.StatusInternalServerError, "failed to move file")
		return
	}
	relPath := fh.relativePath(destPath)
	fh.emitChange(r, FileChangeEvent{
		Operation:  "moved",
		Path:       relPath,
		ParentPath: filepath.ToSlash(filepath.Dir(relPath)),
		Name:       filepath.Base(destPath),
		EntryType:  "file",
		Size:       info.Size(),
		FromPath:   fh.relativePath(absPath),
	})
	writeFilesJSON(w, http.StatusOK, map[string]string{"status": "moved", "path": relPath})
}

// RegisterFileRoutes registers all file browser routes on the given server.
func RegisterFileRoutes(s interface {
	HandleFunc(string, http.HandlerFunc)
}, fh *FilesHandler) {
	s.HandleFunc("/api/files", fh.HandleListRoot)
	s.HandleFunc("/api/files/", fh.HandleFileByPath)
}

func writeFilesJSON(w http.ResponseWriter, statusCode int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}

// writeFileError writes a JSON error response with the given status code.
func writeFileError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(FileErrorResponse{Error: message})
}
