package fileprovider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/base/model"
	"github.com/yusefmosiah/go-choir/internal/desktop"
)

// Bridge is the IPC HTTP server that exposes the Base sync engine to the
// macOS File Provider extension. It listens on a Unix domain socket (so the
// traffic stays local and is not visible on the network) and serves a small
// JSON REST API for enumeration, read, write, delete, move, conflicts, and
// sync status.
//
// The bridge is safe for concurrent use: HTTP handlers run on separate
// goroutines, and all sync-engine access goes through the engine's own
// concurrency-safe methods. Filesystem mutations are serialized by the OS
// per-path; the bridge does not add its own locking because the File
// Provider extension serializes mutations per item via the coordinator.
type Bridge struct {
	engine   *desktop.SyncEngine
	scan     *desktop.LocalTreeBuilder
	root     string
	socket   string
	server   *http.Server
	listener net.Listener
}

// BridgeConfig configures the IPC bridge.
type BridgeConfig struct {
	// Engine is the running sync engine. May be nil for a read-only bridge
	// (enumerate + read only); write/delete/move/conflict/status handlers
	// will return an error if the engine is nil.
	Engine *desktop.SyncEngine

	// LocalRoot is the absolute path to the sync root (same as the engine's).
	LocalRoot string

	// SocketPath is the Unix domain socket path to listen on.
	SocketPath string

	// DeviceID is used by the LocalTreeBuilder for version stamping.
	DeviceID string
}

// NewBridge constructs the bridge from the config. It does not start
// listening; call Start to begin serving.
func NewBridge(cfg BridgeConfig) (*Bridge, error) {
	if cfg.LocalRoot == "" {
		return nil, fmt.Errorf("fileprovider: LocalRoot is required")
	}
	if cfg.SocketPath == "" {
		return nil, fmt.Errorf("fileprovider: SocketPath is required")
	}
	info, err := os.Stat(cfg.LocalRoot)
	if err != nil {
		return nil, fmt.Errorf("fileprovider: stat local root %s: %w", cfg.LocalRoot, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("fileprovider: local root %s is not a directory", cfg.LocalRoot)
	}
	scan := desktop.NewLocalTreeBuilder(cfg.LocalRoot, cfg.DeviceID)
	return &Bridge{
		engine: cfg.Engine,
		scan:   scan,
		root:   cfg.LocalRoot,
		socket: cfg.SocketPath,
	}, nil
}

// Start opens the Unix socket and begins serving HTTP. It returns
// immediately; the server runs in a background goroutine until Stop is
// called or the context is cancelled.
func (b *Bridge) Start(ctx context.Context) error {
	// Remove any stale socket file.
	_ = os.Remove(b.socket)
	// Ensure the socket directory exists.
	if dir := filepath.Dir(b.socket); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("fileprovider: mkdir socket dir: %w", err)
		}
	}
	ln, err := net.Listen("unix", b.socket)
	if err != nil {
		return fmt.Errorf("fileprovider: listen %s: %w", b.socket, err)
	}
	b.listener = ln
	mux := b.routes()
	b.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		_ = b.server.Close()
	}()
	go func() {
		if err := b.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			// The bridge is best-effort; log to stderr.
			fmt.Fprintf(os.Stderr, "[fileprovider] serve error: %v\n", err)
		}
	}()
	return nil
}

// Stop closes the server and removes the socket file.
func (b *Bridge) Stop() {
	if b.server != nil {
		_ = b.server.Close()
	}
	if b.listener != nil {
		_ = b.listener.Close()
	}
	_ = os.Remove(b.socket)
}

// SocketPath returns the Unix socket path the bridge is listening on.
func (b *Bridge) SocketPath() string { return b.socket }

// routes builds the HTTP mux for the bridge API.
func (b *Bridge) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/enumerate", b.handleEnumerate)
	mux.HandleFunc("/read", b.handleRead)
	mux.HandleFunc("/write", b.handleWrite)
	mux.HandleFunc("/mkdir", b.handleCreateDir)
	mux.HandleFunc("/move", b.handleMove)
	mux.HandleFunc("/delete", b.handleDelete)
	mux.HandleFunc("/conflicts", b.handleConflicts)
	mux.HandleFunc("/status", b.handleStatus)
	mux.HandleFunc("/sync", b.handleSyncNow)
	mux.HandleFunc("/health", b.handleHealth)
	return mux
}

// --- security helpers ---

// resolvePath converts a relative POSIX path from the extension into an
// absolute filesystem path within the sync root. It rejects paths that
// escape the root (directory traversal) and normalizes separators.
func (b *Bridge) resolvePath(rel string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(rel))
	if cleaned == "." || cleaned == "" {
		return b.root, nil
	}
	if strings.HasPrefix(cleaned, "..") {
		return "", fmt.Errorf("path escapes sync root: %s", rel)
	}
	abs := filepath.Join(b.root, cleaned)
	// Double-check via Rel that the result is inside root.
	r, err := filepath.Rel(b.root, abs)
	if err != nil || strings.HasPrefix(r, "..") {
		return "", fmt.Errorf("path escapes sync root: %s", rel)
	}
	return abs, nil
}

// relPath returns the slash-relative path for an absolute path within root.
func (b *Bridge) relPath(abs string) string {
	rel, err := filepath.Rel(b.root, abs)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(rel)
}

// --- handlers ---

func (b *Bridge) handleEnumerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	rel := r.URL.Query().Get("path")
	abs, err := b.resolvePath(rel)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	entries, err := b.enumerateDir(abs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Append projected conflict files.
	conflicts := b.projectConflicts(rel)
	entries = append(entries, conflicts...)
	writeJSON(w, http.StatusOK, EnumerateResponse{
		Path:    rel,
		Entries: entries,
	})
}

// enumerateDir lists the children of abs, skipping hidden files and the
// Choir metadata directory (matching LocalTreeBuilder's scan rules).
func (b *Bridge) enumerateDir(abs string) ([]Entry, error) {
	var entries []Entry
	err := filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == abs {
			return nil // skip the directory itself
		}
		base := filepath.Base(path)
		// Skip hidden entries and Choir metadata.
		if strings.HasPrefix(base, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel := b.relPath(path)
		info, serr := d.Info()
		if serr != nil {
			return serr
		}
		kind := KindFile
		if d.IsDir() {
			kind = KindFolder
		}
		entries = append(entries, Entry{
			Path:       rel,
			Name:       base,
			Kind:       kind,
			Size:       info.Size(),
			ModifiedAt: info.ModTime().UTC(),
			SyncState:  model.StateSynced, // best-effort; enriched below if engine present
		})
		if d.IsDir() {
			return filepath.SkipDir // only immediate children
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("enumerate %s: %w", abs, err)
	}
	return entries, nil
}

// projectConflicts returns virtual .conflict entries for the given
// directory. Each unresolved Base conflict is projected as a sibling file
// with ".conflict" appended to the original filename.
func (b *Bridge) projectConflicts(dirRel string) []Entry {
	if b.engine == nil {
		return nil
	}
	pending := b.engine.Conflicts().Pending()
	var out []Entry
	for _, c := range pending {
		// Only project conflicts whose parent directory matches the
		// enumerated directory.
		parent := filepath.Dir(c.Path)
		parent = strings.ReplaceAll(parent, "\\", "/")
		if parent == "." {
			parent = ""
		}
		if parent != dirRel {
			continue
		}
		conflictPath := c.Path + ".conflict"
		out = append(out, Entry{
			Path:         conflictPath,
			Name:         filepath.Base(conflictPath),
			Kind:         KindConflict,
			SyncState:    model.StateConflict,
			ConflictPath: c.Path,
			ItemID:       c.ItemID,
		})
	}
	return out
}

func (b *Bridge) handleRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	rel := r.URL.Query().Get("path")
	abs, err := b.resolvePath(rel)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if info.IsDir() {
		writeError(w, http.StatusBadRequest, "path is a directory")
		return
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ReadResponse{
		Path:       rel,
		Size:       info.Size(),
		ContentB64: base64.StdEncoding.EncodeToString(data),
		MediaType:  mediaTypeForPath(filepath.Base(abs)),
		ModifiedAt: info.ModTime().UTC(),
	})
}

func (b *Bridge) handleWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if b.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "sync engine not available")
		return
	}
	var req WriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	abs, err := b.resolvePath(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	data, err := base64.StdEncoding.DecodeString(req.ContentB64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid base64 content: "+err.Error())
		return
	}
	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "mkdir parent: "+err.Error())
		return
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, "write file: "+err.Error())
		return
	}
	// Trigger a sync cycle so the change is uploaded.
	b.engine.SyncNow()
	writeJSON(w, http.StatusOK, WriteResponse{
		Path:          req.Path,
		SyncTriggered: true,
	})
}

func (b *Bridge) handleCreateDir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if b.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "sync engine not available")
		return
	}
	var req CreateDirRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	abs, err := b.resolvePath(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "mkdir: "+err.Error())
		return
	}
	b.engine.SyncNow()
	writeJSON(w, http.StatusOK, map[string]string{"path": req.Path, "status": "created"})
}

func (b *Bridge) handleMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if b.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "sync engine not available")
		return
	}
	var req MoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	fromAbs, err := b.resolvePath(req.FromPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "from: "+err.Error())
		return
	}
	toAbs, err := b.resolvePath(req.ToPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "to: "+err.Error())
		return
	}
	if err := os.MkdirAll(filepath.Dir(toAbs), 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "mkdir dest parent: "+err.Error())
		return
	}
	if err := os.Rename(fromAbs, toAbs); err != nil {
		writeError(w, http.StatusInternalServerError, "rename: "+err.Error())
		return
	}
	b.engine.SyncNow()
	writeJSON(w, http.StatusOK, map[string]string{"from": req.FromPath, "to": req.ToPath, "status": "moved"})
}

func (b *Bridge) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if b.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "sync engine not available")
		return
	}
	var req DeleteRequest
	// Support both JSON body and query param.
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
			return
		}
	} else {
		req.Path = r.URL.Query().Get("path")
	}
	abs, err := b.resolvePath(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := os.RemoveAll(abs); err != nil {
		writeError(w, http.StatusInternalServerError, "delete: "+err.Error())
		return
	}
	b.engine.SyncNow()
	writeJSON(w, http.StatusOK, map[string]string{"path": req.Path, "status": "deleted"})
}

func (b *Bridge) handleConflicts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if b.engine == nil {
		writeJSON(w, http.StatusOK, ConflictsResponse{})
		return
	}
	all := b.engine.Conflicts().All()
	out := make([]ConflictEntry, 0, len(all))
	for _, c := range all {
		out = append(out, ConflictEntry{
			ItemID:       c.ItemID,
			Path:         c.Path,
			ConflictPath: c.Path + ".conflict",
			Reason:       c.Reason,
			LocalVer:     c.LocalVer,
			RemoteVer:    c.RemoteVer,
			Resolution:   c.Resolved,
		})
	}
	writeJSON(w, http.StatusOK, ConflictsResponse{Conflicts: out})
}

func (b *Bridge) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if b.engine == nil {
		writeJSON(w, http.StatusOK, StatusResponse{Phase: string(desktop.PhaseIdle)})
		return
	}
	p := b.engine.Status().Snapshot()
	writeJSON(w, http.StatusOK, StatusResponse{
		Phase:          string(p.Phase),
		LastSyncAt:     p.LastSyncAt,
		Cursor:         p.Cursor,
		RemoteHead:     p.RemoteHead,
		ConflictsCount: p.ConflictsCount,
		LastError:      p.LastError,
	})
}

func (b *Bridge) handleSyncNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if b.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "sync engine not available")
		return
	}
	b.engine.SyncNow()
	writeJSON(w, http.StatusOK, map[string]string{"status": "sync_triggered"})
}

func (b *Bridge) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- HTTP helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}

// mediaTypeForPath mirrors LocalTreeBuilder's media-type heuristic. We
// duplicate it here to avoid exporting the internal helper; the bridge is
// the only consumer outside the scan path.
func mediaTypeForPath(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".txt":
		return "text/plain"
	case ".md":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".go":
		return "text/x-go"
	default:
		return "application/octet-stream"
	}
}
