package textureowner

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/mail"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	contentowner "github.com/yusefmosiah/go-choir/internal/content"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	runMetadataAgentProfile = "agent_profile"
	runMetadataChannelID    = "channel_id"
	runMetadataAgentRole    = "agent_role"
	runMetadataAgentID      = "agent_id"
	runMetadataDesktopID    = "desktop_id"
	runMetadataToolCWD      = "tool_cwd"
	runMetadataOwnerEmail   = "owner_email"
)

type textureWakeLock struct {
	mu   sync.Mutex
	refs int
}

// Handler owns Texture's HTTP and lifecycle behavior while using agentcore as
// the concrete execution substrate.
type Handler struct {
	Core        *agentcore.Runtime
	Store       *store.Store
	Bus         *events.EventBus
	Content     *contentowner.Service
	ModelPolicy *modelpolicy.Manager
	Provider    provideriface.Provider

	textureEditMu sync.Mutex
	// evidenceSource overrides the visual-evidence listing in tests (station
	// TE); nil reads it from Core.
	evidenceSource     candidateEvidenceSource
	textureWakeLocksMu sync.Mutex
	textureWakeLocks   map[string]*textureWakeLock
	// The Texture root's digest state (PD3 S1, texture_root.go): in memory
	// only, so a restart forgets it. textureRootNow overrides the clock in
	// tests.
	textureRootMu     sync.Mutex
	textureRootScopes map[string]*textureRootScope
	textureRootNow    func() time.Time
	// textureRootListDocuments overrides the document listing the root's
	// shared context reads (one read per run); nil is the store's.
	textureRootListDocuments func(ctx context.Context, ownerID, computerID string, limit int) ([]types.Document, error)
}

func (h *Handler) lockTextureWakeScope(ownerID, computerID, docID string) func() {
	key := strings.Join([]string{strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(docID)}, "\x00")
	h.textureWakeLocksMu.Lock()
	if h.textureWakeLocks == nil {
		h.textureWakeLocks = make(map[string]*textureWakeLock)
	}
	lock := h.textureWakeLocks[key]
	if lock == nil {
		lock = &textureWakeLock{}
		h.textureWakeLocks[key] = lock
	}
	lock.refs++
	h.textureWakeLocksMu.Unlock()

	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		h.textureWakeLocksMu.Lock()
		lock.refs--
		if lock.refs == 0 && h.textureWakeLocks[key] == lock {
			delete(h.textureWakeLocks, key)
		}
		h.textureWakeLocksMu.Unlock()
	}
}

// NewHandler composes Texture ownership over the concrete agent lifecycle.
func NewHandler(core *agentcore.Runtime) *Handler {
	if core == nil {
		return &Handler{}
	}
	return &Handler{
		Core:        core,
		Store:       core.Store(),
		Bus:         core.EventBus(),
		Content:     core.TextureContentService(),
		ModelPolicy: core.TextureModelPolicy(),
		Provider:    core.TextureProvider(),
	}
}

func (h *Handler) getTextureDocument(ctx context.Context, ownerID, docID string) (types.Document, error) {
	if h == nil || h.Store == nil {
		return types.Document{}, fmt.Errorf("texture store unavailable")
	}
	computerID := ""
	if h.Core != nil {
		computerID = strings.TrimSpace(h.Core.TextureComputerID())
	}
	if computerID != "" {
		doc, err := h.Store.GetLifecycleDocument(ctx, ownerID, computerID, docID)
		if err == nil {
			return doc, nil
		}
		if !errors.Is(err, store.ErrNotFound) {
			return types.Document{}, err
		}
	}
	doc, err := h.Store.GetDocument(ctx, docID, ownerID)
	if err != nil {
		return types.Document{}, err
	}
	if strings.TrimSpace(doc.TrajectoryID) != "" {
		return types.Document{}, store.ErrLifecycleAuthorityRequired
	}
	return doc, nil
}

func (h *Handler) getTextureRevision(ctx context.Context, ownerID, revisionID string) (types.Revision, error) {
	if h == nil || h.Store == nil {
		return types.Revision{}, fmt.Errorf("texture store unavailable")
	}
	computerID := ""
	if h.Core != nil {
		computerID = strings.TrimSpace(h.Core.TextureComputerID())
	}
	if computerID != "" {
		revision, err := h.Store.GetLifecycleRevision(ctx, ownerID, computerID, revisionID)
		if err == nil {
			return revision, nil
		}
		if !errors.Is(err, store.ErrNotFound) {
			return types.Revision{}, err
		}
	}
	revision, err := h.Store.GetRevision(ctx, revisionID, ownerID)
	if err != nil {
		return types.Revision{}, err
	}
	if strings.TrimSpace(revision.TrajectoryID) != "" {
		return types.Revision{}, store.ErrLifecycleAuthorityRequired
	}
	return revision, nil
}

// textureDocumentCursor is the position after the last document of a page in
// the list's order: updated_at descending, then doc_id descending.
type textureDocumentCursor struct {
	UpdatedAt time.Time `json:"u"`
	DocID     string    `json:"d"`
}

func (c textureDocumentCursor) encode() string {
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeTextureDocumentCursor(raw string) (textureDocumentCursor, error) {
	var cursor textureDocumentCursor
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return cursor, err
	}
	if err := json.Unmarshal(decoded, &cursor); err != nil {
		return cursor, err
	}
	if cursor.UpdatedAt.IsZero() || strings.TrimSpace(cursor.DocID) == "" {
		return cursor, fmt.Errorf("incomplete cursor")
	}
	return cursor, nil
}

func textureDocumentNewer(a, b types.Document) bool {
	if !a.UpdatedAt.Equal(b.UpdatedAt) {
		return a.UpdatedAt.After(b.UpdatedAt)
	}
	return a.DocID > b.DocID
}

// listTextureDocumentsPage lists the owner's documents visible on this
// computer, newest first, limit at a time after the cursor. It returns the
// next page's cursor when more documents follow. The stores rank in memory
// over every document of the kind, so asking them for all is no extra load.
func (h *Handler) listTextureDocumentsPage(ctx context.Context, ownerID string, limit int, after *textureDocumentCursor) ([]types.Document, *textureDocumentCursor, error) {
	if h == nil || h.Store == nil {
		return nil, nil, fmt.Errorf("texture store unavailable")
	}
	if limit <= 0 {
		limit = textureDocumentListDefaultLimit
	}
	const all = math.MaxInt32
	legacy, err := h.Store.ListDocumentsByOwner(ctx, ownerID, all)
	if err != nil {
		return nil, nil, err
	}
	docs := legacy
	computerID := ""
	if h.Core != nil {
		computerID = strings.TrimSpace(h.Core.TextureComputerID())
	}
	if computerID != "" {
		scoped, err := h.Store.ListDocumentsByScope(ctx, ownerID, computerID, all)
		if err != nil {
			return nil, nil, err
		}
		seen := make(map[string]bool, len(scoped))
		docs = append([]types.Document(nil), scoped...)
		for _, doc := range scoped {
			seen[doc.DocID] = true
		}
		for _, doc := range legacy {
			if !seen[doc.DocID] {
				docs = append(docs, doc)
			}
		}
	}
	sort.Slice(docs, func(i, j int) bool { return textureDocumentNewer(docs[i], docs[j]) })
	if after != nil {
		mark := types.Document{UpdatedAt: after.UpdatedAt, DocID: after.DocID}
		start := sort.Search(len(docs), func(i int) bool { return textureDocumentNewer(mark, docs[i]) })
		docs = docs[start:]
	}
	if len(docs) <= limit {
		return docs, nil, nil
	}
	page := docs[:limit]
	last := page[len(page)-1]
	return page, &textureDocumentCursor{UpdatedAt: last.UpdatedAt, DocID: last.DocID}, nil
}

func (h *Handler) listTextureRevisions(ctx context.Context, ownerID, docID string, limit int) ([]types.Revision, error) {
	doc, err := h.getTextureDocument(ctx, ownerID, docID)
	if err != nil {
		return nil, err
	}
	if computerID := strings.TrimSpace(doc.ComputerID); computerID != "" {
		return h.Store.ListRevisionsByScope(ctx, docID, ownerID, computerID, limit)
	}
	return h.Store.ListRevisionsByDoc(ctx, docID, ownerID, limit)
}

func (h *Handler) getTextureHistory(ctx context.Context, ownerID, docID string, limit int) ([]types.HistoryEntry, error) {
	doc, err := h.getTextureDocument(ctx, ownerID, docID)
	if err != nil {
		return nil, err
	}
	if computerID := strings.TrimSpace(doc.ComputerID); computerID != "" {
		revisions, err := h.Store.ListRevisionsByScope(ctx, docID, ownerID, computerID, limit)
		if err != nil {
			return nil, err
		}
		entries := make([]types.HistoryEntry, 0, len(revisions))
		for _, revision := range revisions {
			entries = append(entries, types.HistoryEntry{
				RevisionID: revision.RevisionID, DocID: revision.DocID,
				AuthorKind: revision.AuthorKind, AuthorLabel: revision.AuthorLabel,
				CreatedAt: revision.CreatedAt, ParentRevisionID: revision.ParentRevisionID,
			})
		}
		return entries, nil
	}
	return h.Store.GetHistory(ctx, docID, ownerID, limit)
}

type apiError struct {
	Error string `json:"error"`
}

// authenticateUser is the guest's one owner-identity rule: the proxy's
// X-Authenticated-User, believed only from the host side of the guest
// boundary, never from an in-guest caller
// (docs/problems/auth-boundary-gaps-from-surface-audit-2026-10-14.md F2).
var authenticateUser = server.AuthenticatedOwner

func authenticatedUserEmail(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("X-Authenticated-Email"))
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return ""
	}
	addr, err := mail.ParseAddress(value)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(addr.Address))
}

func requestDesktopID(r *http.Request) string {
	if r == nil {
		return types.PrimaryDesktopID
	}
	if desktopID := strings.TrimSpace(r.URL.Query().Get("desktop_id")); desktopID != "" {
		return desktopID
	}
	if desktopID := strings.TrimSpace(r.Header.Get("X-Choir-Desktop")); desktopID != "" {
		return desktopID
	}
	return types.PrimaryDesktopID
}

func writeAPIJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("texture api: json encode error: %v", err)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func configuredAgentProfileForRun(rec *types.RunRecord) string {
	if rec == nil {
		return ""
	}
	if strings.TrimSpace(rec.AgentProfile) != "" {
		configuredProfile, _ := agentprofile.Canonical(rec.AgentProfile)
		return configuredProfile
	}
	if rec.Metadata != nil {
		if profile, _ := rec.Metadata[runMetadataAgentProfile].(string); strings.TrimSpace(profile) != "" {
			configuredMetaProfile, _ := agentprofile.Canonical(profile)
			return configuredMetaProfile
		}
	}
	if taskType, _ := rec.Metadata["type"].(string); isTextureAgentRevisionTaskType(taskType) {
		return agentprofile.Texture
	}
	return ""
}

func agentRoleForRun(rec *types.RunRecord) string {
	if rec == nil {
		return agentprofile.Management
	}
	if strings.TrimSpace(rec.AgentRole) != "" {
		configuredRole, _ := agentprofile.Canonical(rec.AgentRole)
		return configuredRole
	}
	if rec.Metadata != nil {
		if role, _ := rec.Metadata[runMetadataAgentRole].(string); strings.TrimSpace(role) != "" {
			configuredMetaRole, _ := agentprofile.Canonical(role)
			return configuredMetaRole
		}
	}
	return agentProfileForRun(rec)
}

func currentTextureAgentID(docID string) string {
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return ""
	}
	return agentprofile.Texture + ":" + docID
}

func agentProfileForRun(rec *types.RunRecord) string {
	if rec == nil {
		return agentprofile.Management
	}
	if strings.TrimSpace(rec.AgentProfile) != "" {
		configuredProfile, _ := agentprofile.Canonical(rec.AgentProfile)
		return configuredProfile
	}
	if rec.Metadata != nil {
		if profile, _ := rec.Metadata[runMetadataAgentProfile].(string); strings.TrimSpace(profile) != "" {
			configuredMetaProfile, _ := agentprofile.Canonical(profile)
			return configuredMetaProfile
		}
	}
	if taskType, _ := rec.Metadata["type"].(string); taskType == textureAgentRevisionTaskType {
		return agentprofile.Texture
	}
	return agentprofile.Management
}
