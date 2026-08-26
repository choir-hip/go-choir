package autoputer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
