package autoputer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/filecas"
)

const (
	fileSyncChunkSize         = 4 << 20
	defaultFileSyncInterval   = 15 * time.Minute
	fileRootManifestMediaType = "application/vnd.choir.file-root-manifest+json"
)

type cachedSyncedFile struct {
	size    int64
	modTime time.Time
	entry   filecas.FileEntry
}

type fileSyncResult struct {
	root           string
	files          int
	chunksUploaded int
}

type pendingFileRootCitation struct {
	root     string
	manifest []byte
}

// fileSync uploads encrypted, content-addressed snapshots of the guest Files
// root. Its cache is deliberately in-memory: the platform's immutable chunk PUT
// makes a reboot safe, while a live guest avoids re-uploading unchanged files.
type fileSync struct {
	filesRoot   string
	platformURL string
	capability  func(context.Context) (string, error)
	computerID  string
	chunkSize   int
	cipher      *computerevent.PrivateArtifactCipher
	head        func(context.Context) (uint64, error)
	appender    *computerevent.ComputerEventAppender
	httpClient  *http.Client

	mu              sync.Mutex
	fileCache       map[string]cachedSyncedFile
	chunkCache      map[string]map[string]bool
	pendingCitation *pendingFileRootCitation

	lastHydratedRoot string
}

func newFileSync(filesRoot, platformURL string, capability func(context.Context) (string, error), computerID string, cipher *computerevent.PrivateArtifactCipher, head func(context.Context) (uint64, error), appender *computerevent.ComputerEventAppender) *fileSync {
	return &fileSync{
		filesRoot:   filesRoot,
		platformURL: strings.TrimRight(platformURL, "/"),
		capability:  capability,
		computerID:  computerID,
		chunkSize:   fileSyncChunkSize,
		cipher:      cipher,
		head:        head,
		appender:    appender,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		fileCache:   make(map[string]cachedSyncedFile),
		chunkCache:  make(map[string]map[string]bool),
	}
}

// SyncOnce commits the current files root and returns its content root.
func (s *fileSync) SyncOnce(ctx context.Context) (string, error) {
	result, err := s.syncOnce(ctx)
	return result.root, err
}

// HydrateIfNeeded restores the latest CAS root only when the local Files tree
// has no regular files. A local file always wins over a remote snapshot.
func (s *fileSync) HydrateIfNeeded(ctx context.Context) (int, error) {
	if s == nil || s.cipher == nil || strings.TrimSpace(s.filesRoot) == "" || strings.TrimSpace(s.platformURL) == "" || strings.TrimSpace(s.computerID) == "" {
		return 0, fmt.Errorf("file sync: incomplete configuration")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastHydratedRoot = ""

	hasFiles, err := hasRegularFiles(s.filesRoot)
	if err != nil {
		return 0, fmt.Errorf("file sync: inspect files root: %w", err)
	}
	if hasFiles {
		return 0, nil
	}

	root, err := s.latestRoot(ctx)
	if err != nil {
		return 0, err
	}
	if root == "" {
		return 0, nil
	}
	manifestJSON, err := s.get(ctx, "/internal/computers/files/root?computer_id="+url.QueryEscape(s.computerID)+"&root="+url.QueryEscape(root))
	if err != nil {
		return 0, fmt.Errorf("file sync: fetch root %s: %w", root, err)
	}
	manifest, err := filecas.ParseManifest(manifestJSON)
	if err != nil {
		return 0, fmt.Errorf("file sync: parse root %s: %w", root, err)
	}
	if err := manifest.VerifyRoot(); err != nil {
		return 0, fmt.Errorf("file sync: verify root %s: %w", root, err)
	}
	if manifest.Root != root || manifest.ComputerID != s.computerID {
		return 0, fmt.Errorf("file sync: root %s does not belong to computer", root)
	}

	dek, err := s.cipher.ExportKeyForEscrow(ctx, s.computerID)
	if err != nil {
		return 0, fmt.Errorf("file sync: export DEK: %w", err)
	}
	defer clearBytes(dek)

	stagingDir := filepath.Clean(s.filesRoot) + ".hydrate-staging"
	_ = os.RemoveAll(stagingDir)
	if err := os.MkdirAll(stagingDir, 0o700); err != nil {
		return 0, fmt.Errorf("file sync: create staging directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(stagingDir)
	}()

	restored := 0
	for _, entry := range manifest.Files {
		rel, err := safeHydrationPath(entry.Path)
		if err != nil {
			return 0, fmt.Errorf("file sync: invalid manifest path %q: %w", entry.Path, err)
		}
		if err := s.hydrateFile(ctx, dek, stagingDir, rel, entry); err != nil {
			return 0, err
		}
		restored++
	}

	if err := swapHydrationTree(stagingDir, s.filesRoot); err != nil {
		return 0, fmt.Errorf("file sync: install hydrated tree: %w", err)
	}

	// Seed in-memory cache so subsequent syncs do not re-upload restored chunks
	for _, entry := range manifest.Files {
		rel, err := safeHydrationPath(entry.Path)
		if err != nil {
			continue
		}
		info, err := os.Stat(filepath.Join(s.filesRoot, filepath.FromSlash(rel)))
		if err == nil {
			pathChunks := make(map[string]bool)
			for _, ch := range entry.Chunks {
				pathChunks[ch] = true
			}
			s.fileCache[rel] = cachedSyncedFile{size: info.Size(), modTime: info.ModTime(), entry: entry}
			s.chunkCache[rel] = pathChunks
		}
	}

	s.lastHydratedRoot = root
	return restored, nil
}

func (s *fileSync) hydratedRoot() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastHydratedRoot
}

func hasRegularFiles(root string) (bool, error) {
	info, err := os.Stat(root)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("files root is not a directory")
	}
	found := false
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != root && entry.Type().IsRegular() {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found, err
}

func (s *fileSync) latestRoot(ctx context.Context) (string, error) {
	body, err := s.get(ctx, "/internal/computers/files/roots?computer_id="+url.QueryEscape(s.computerID)+"&limit=1")
	if err != nil {
		return "", fmt.Errorf("file sync: list roots: %w", err)
	}
	var response struct {
		Roots []struct {
			Root string `json:"root"`
		} `json:"roots"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("file sync: decode roots: %w", err)
	}
	if len(response.Roots) == 0 {
		return "", nil
	}
	if strings.TrimSpace(response.Roots[0].Root) == "" {
		return "", fmt.Errorf("file sync: latest root is empty")
	}
	return response.Roots[0].Root, nil
}

func (s *fileSync) get(ctx context.Context, path string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.platformURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if err := s.authorize(request); err != nil {
		return nil, err
	}
	response, err := s.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("status %d", response.StatusCode)
	}
	return io.ReadAll(response.Body)
}

func safeHydrationPath(path string) (string, error) {
	if path == "" || filepath.IsAbs(path) {
		return "", fmt.Errorf("path must be relative")
	}
	for _, component := range strings.Split(filepath.ToSlash(path), "/") {
		if component == ".." {
			return "", fmt.Errorf("path traversal")
		}
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
		return "", fmt.Errorf("path traversal")
	}
	return clean, nil
}

func (s *fileSync) hydrateFile(ctx context.Context, dek []byte, targetRoot, rel string, entry filecas.FileEntry) (err error) {
	parent, err := ensureHydrationParent(targetRoot, filepath.Dir(rel))
	if err != nil {
		return fmt.Errorf("file sync: create parent for %s: %w", entry.Path, err)
	}
	file, err := os.CreateTemp(parent, ".choir-hydrate-*")
	if err != nil {
		return fmt.Errorf("file sync: create %s: %w", entry.Path, err)
	}
	temp := file.Name()
	defer func() {
		if err != nil {
			_ = file.Close()
			_ = os.Remove(temp)
		}
	}()

	var written int64
	for _, digest := range entry.Chunks {
		sealed, err := s.get(ctx, "/internal/computers/files/chunks/"+digest+"?computer_id="+url.QueryEscape(s.computerID))
		if err != nil {
			return fmt.Errorf("file sync: fetch chunk %s for %s: %w", digest, entry.Path, err)
		}
		if len(sealed) == 0 {
			return fmt.Errorf("file sync: empty chunk %s for %s", digest, entry.Path)
		}
		plain, err := filecas.OpenChunk(dek, s.computerID, digest, sealed)
		if err != nil {
			return fmt.Errorf("file sync: open chunk %s for %s: %w", digest, entry.Path, err)
		}
		n, err := file.Write(plain)
		written += int64(n)
		if err != nil {
			return fmt.Errorf("file sync: write %s: %w", entry.Path, err)
		}
		if n != len(plain) {
			return fmt.Errorf("file sync: short write %s", entry.Path)
		}
	}
	if written != entry.Size {
		return fmt.Errorf("file sync: size mismatch for %s: got %d, want %d", entry.Path, written, entry.Size)
	}
	if err := file.Chmod(fs.FileMode(entry.Mode).Perm()); err != nil {
		return fmt.Errorf("file sync: chmod %s: %w", entry.Path, err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("file sync: sync %s: %w", entry.Path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("file sync: close %s: %w", entry.Path, err)
	}
	if err := os.Rename(temp, filepath.Join(parent, filepath.Base(rel))); err != nil {
		return fmt.Errorf("file sync: install %s: %w", entry.Path, err)
	}
	return nil
}

func ensureHydrationParent(root, relativeParent string) (string, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("files root is not a directory")
	}
	parent := root
	if relativeParent == "." {
		return parent, nil
	}
	for _, component := range strings.Split(relativeParent, string(filepath.Separator)) {
		parent = filepath.Join(parent, component)
		info, err := os.Lstat(parent)
		if os.IsNotExist(err) {
			if err := os.Mkdir(parent, 0o700); err != nil {
				return "", err
			}
			continue
		}
		if err != nil {
			return "", err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("parent %q is not a directory", component)
		}
	}
	return parent, nil
}
func swapHydrationTree(stagingDir, targetRoot string) error {
	parent := filepath.Dir(targetRoot)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}

	backupDir := filepath.Clean(targetRoot) + ".old"
	_ = os.RemoveAll(backupDir)

	if _, err := os.Stat(targetRoot); err == nil {
		if err := os.Rename(targetRoot, backupDir); err != nil {
			return fmt.Errorf("backup existing empty root: %w", err)
		}
	}

	if err := os.Rename(stagingDir, targetRoot); err != nil {
		// Roll back backup if rename fails
		if _, statErr := os.Stat(backupDir); statErr == nil {
			_ = os.Rename(backupDir, targetRoot)
		}
		return fmt.Errorf("install staged root: %w", err)
	}
	_ = os.RemoveAll(backupDir)

	// Fsync targetRoot directory
	if dir, err := os.Open(targetRoot); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	// Fsync parent directory
	if dir, err := os.Open(parent); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

func (s *fileSync) syncOnce(ctx context.Context) (fileSyncResult, error) {
	if s == nil || s.cipher == nil || strings.TrimSpace(s.filesRoot) == "" || strings.TrimSpace(s.platformURL) == "" || strings.TrimSpace(s.computerID) == "" {
		return fileSyncResult{}, fmt.Errorf("file sync: incomplete configuration")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// ExportKeyForEscrow is the sole guest accessor for the DEK. File CAS uses
	// it only transiently to seal chunks; it is never logged or persisted.
	dek, err := s.cipher.ExportKeyForEscrow(ctx, s.computerID)
	if err != nil {
		return fileSyncResult{}, fmt.Errorf("file sync: export DEK: %w", err)
	}
	defer clearBytes(dek)

	s.retryPendingCitation(ctx)
	entries := make([]filecas.FileEntry, 0)
	seen := make(map[string]struct{})
	result := fileSyncResult{}
	err = filepath.WalkDir(s.filesRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == s.filesRoot {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.IsDir() || !info.Mode().IsRegular() || info.Mode()&os.ModeSocket != 0 {
			return nil
		}
		rel, err := filepath.Rel(s.filesRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		seen[rel] = struct{}{}
		entry, uploaded, err := s.syncFile(ctx, dek, rel, path, info)
		if err != nil {
			return err
		}
		entries = append(entries, entry)
		result.files++
		result.chunksUploaded += uploaded
		return nil
	})
	if err != nil {
		return fileSyncResult{}, fmt.Errorf("file sync: walk files root: %w", err)
	}
	for path := range s.fileCache {
		if _, ok := seen[path]; !ok {
			delete(s.fileCache, path)
			delete(s.chunkCache, path)
		}
	}
	manifest, err := filecas.BuildManifest(s.computerID, entries, time.Now())
	if err != nil {
		return fileSyncResult{}, fmt.Errorf("file sync: build manifest: %w", err)
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return fileSyncResult{}, fmt.Errorf("file sync: marshal manifest: %w", err)
	}
	headSequence := uint64(0)
	if s.head != nil {
		headSequence, err = s.head(ctx)
		if err != nil {
			return fileSyncResult{}, fmt.Errorf("file sync: event head: %w", err)
		}
	}
	if err := s.putRoot(ctx, manifestJSON, headSequence); err != nil {
		return fileSyncResult{}, err
	}
	result.root = manifest.Root
	if err := s.appendRootCommitted(ctx, manifest.Root, manifestJSON); err != nil {
		s.pendingCitation = &pendingFileRootCitation{root: manifest.Root, manifest: append([]byte(nil), manifestJSON...)}
		log.Printf("autoputer: file root citation deferred for %s: %v", manifest.Root, err)
	}
	return result, nil
}

func (s *fileSync) syncFile(ctx context.Context, dek []byte, rel, path string, info fs.FileInfo) (filecas.FileEntry, int, error) {
	if cached, ok := s.fileCache[rel]; ok && s.chunkCache[rel] != nil && cached.size == info.Size() && cached.modTime.Equal(info.ModTime()) {
		return cached.entry, 0, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return filecas.FileEntry{}, 0, fmt.Errorf("read %s: %w", rel, err)
	}
	entry := filecas.FileEntry{Path: rel, Mode: uint32(info.Mode().Perm()), Size: info.Size()}
	uploaded := 0
	pathChunks := make(map[string]bool)
	for _, chunk := range filecas.ChunkBytes(data, s.chunkSize) {
		sealed, digest, err := filecas.SealChunk(dek, s.computerID, chunk)
		if err != nil {
			return filecas.FileEntry{}, 0, fmt.Errorf("seal %s: %w", rel, err)
		}
		entry.Chunks = append(entry.Chunks, digest)
		pathChunks[digest] = true
		if err := s.putChunk(ctx, digest, sealed); err != nil {
			return filecas.FileEntry{}, 0, err
		}
		uploaded++
	}
	s.fileCache[rel] = cachedSyncedFile{size: info.Size(), modTime: info.ModTime(), entry: entry}
	s.chunkCache[rel] = pathChunks
	return entry, uploaded, nil
}

func (s *fileSync) putChunk(ctx context.Context, digest string, sealed []byte) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, s.platformURL+"/internal/computers/files/chunks/"+digest+"?computer_id="+url.QueryEscape(s.computerID), bytes.NewReader(sealed))
	if err != nil {
		return fmt.Errorf("file sync: build chunk request: %w", err)
	}
	request.Header.Set("Content-Type", "application/octet-stream")
	if err := s.authorize(request); err != nil {
		return err
	}
	if err := s.do(request); err != nil {
		return fmt.Errorf("file sync: upload chunk %s: %w", digest, err)
	}
	return nil
}

func (s *fileSync) putRoot(ctx context.Context, manifest []byte, headSequence uint64) error {
	if headSequence > uint64(^uint64(0)>>1) {
		return fmt.Errorf("file sync: event head sequence exceeds platform range")
	}
	body, err := json.Marshal(struct {
		ComputerID   string `json:"computer_id"`
		Manifest     string `json:"manifest"`
		HeadSequence int64  `json:"head_sequence"`
	}{ComputerID: s.computerID, Manifest: string(manifest), HeadSequence: int64(headSequence)})
	if err != nil {
		return fmt.Errorf("file sync: marshal root request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, s.platformURL+"/internal/computers/files/root", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("file sync: build root request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if err := s.authorize(request); err != nil {
		return err
	}
	if err := s.do(request); err != nil {
		return fmt.Errorf("file sync: commit root: %w", err)
	}
	return nil
}

func (s *fileSync) authorize(request *http.Request) error {
	if s.capability == nil {
		return nil
	}
	token, err := s.capability(request.Context())
	if err != nil {
		return fmt.Errorf("file sync: capability: %w", err)
	}
	if strings.TrimSpace(token) != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return nil
}

func (s *fileSync) do(request *http.Request) error {
	response, err := s.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("status %d", response.StatusCode)
	}
	return nil
}

func (s *fileSync) retryPendingCitation(ctx context.Context) {
	if s.pendingCitation == nil {
		return
	}
	if err := s.appendRootCommitted(ctx, s.pendingCitation.root, s.pendingCitation.manifest); err != nil {
		log.Printf("autoputer: file root citation retry deferred for %s: %v", s.pendingCitation.root, err)
		return
	}
	s.pendingCitation = nil
}

func (s *fileSync) appendRootCommitted(ctx context.Context, root string, manifest []byte) error {
	if s.appender == nil {
		return nil
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		return err
	}
	event := computerevent.Event{
		SchemaVersion:      computerevent.SchemaVersionV1,
		EventID:            eventID,
		ComputerID:         s.computerID,
		EventKind:          computerevent.EventFileRootCommitted,
		OccurredAt:         time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey:     "file-root:" + root,
		ActorProfile:       agentprofile.Super,
		AuthorityRef:       "authority:guest-file-sync",
		PrivacyClass:       "owner",
		ReducerVersion:     computerevent.ReducerVersionV1,
		OutputArtifactRefs: []string{},
	}
	_, _, err = s.appender.AppendNewPayload(ctx, event, computerevent.TransitionInput{}, manifest, fileRootManifestMediaType, "owner")
	return err
}

func clearBytes(data []byte) {
	for i := range data {
		data[i] = 0
	}
}

// StartPeriodicFileSync schedules best-effort checkpoints. A zero interval
// disables periodic syncing; explicit API sync remains available.
func StartPeriodicFileSync(ctx context.Context, service *fileSync, interval time.Duration) {
	if service == nil || interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := service.SyncOnce(ctx); err != nil {
					log.Printf("autoputer: periodic file sync failed: %v", err)
				}
			}
		}
	}()
}

func fileSyncIntervalFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("AUTOPUTER_FILE_SYNC_INTERVAL"))
	if raw == "" {
		return defaultFileSyncInterval
	}
	if raw == "0" {
		return 0
	}
	interval, err := time.ParseDuration(raw)
	if err != nil || interval <= 0 {
		log.Printf("autoputer: invalid AUTOPUTER_FILE_SYNC_INTERVAL=%q; using %s", raw, defaultFileSyncInterval)
		return defaultFileSyncInterval
	}
	return interval
}

// HandleSync handles POST /api/files/sync, the synchronous sync_computer_files
// barrier used by the guest runtime surface.
func (s *fileSync) HandleSync(w http.ResponseWriter, r *http.Request) {
	if err := requireAuth(r); err != nil {
		writeFileError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if r.Method != http.MethodPost {
		writeFileError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	result, err := s.syncOnce(r.Context())
	if err != nil {
		writeFileError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Root           string `json:"root"`
		Files          int    `json:"files"`
		ChunksUploaded int    `json:"chunks_uploaded"`
	}{Root: result.root, Files: result.files, ChunksUploaded: result.chunksUploaded})
}

func RegisterFileSyncRoute(s interface {
	HandleFunc(string, http.HandlerFunc)
}, service *fileSync) {
	if service != nil {
		s.HandleFunc("/api/files/sync", service.HandleSync)
	}
}
