package textureowner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"sort"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	contentowner "github.com/yusefmosiah/go-choir/internal/content"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
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
		computerID = strings.TrimSpace(h.Core.TextureSandboxID())
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
		computerID = strings.TrimSpace(h.Core.TextureSandboxID())
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
func (h *Handler) listTextureDocuments(ctx context.Context, ownerID string, limit int) ([]types.Document, error) {
	if h == nil || h.Store == nil {
		return nil, fmt.Errorf("texture store unavailable")
	}
	if limit <= 0 {
		limit = 50
	}
	legacy, err := h.Store.ListDocumentsByOwner(ctx, ownerID, limit)
	if err != nil {
		return nil, err
	}
	computerID := ""
	if h.Core != nil {
		computerID = strings.TrimSpace(h.Core.TextureSandboxID())
	}
	if computerID == "" {
		return legacy, nil
	}
	scoped, err := h.Store.ListDocumentsByScope(ctx, ownerID, computerID, limit)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(scoped))
	docs := append([]types.Document(nil), scoped...)
	for _, doc := range scoped {
		seen[doc.DocID] = true
	}
	for _, doc := range legacy {
		if !seen[doc.DocID] {
			docs = append(docs, doc)
		}
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].UpdatedAt.After(docs[j].UpdatedAt) })
	if len(docs) > limit {
		docs = docs[:limit]
	}
	return docs, nil
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

func authenticateUser(r *http.Request) (string, error) {
	user := r.Header.Get("X-Authenticated-User")
	if user == "" {
		return "", fmt.Errorf("missing authenticated user identity")
	}
	return user, nil
}

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

func toolExecutionContextForRun(rec *types.RunRecord) toolregistry.ExecutionContext {
	if rec == nil {
		return toolregistry.ExecutionContext{}
	}
	execution := toolregistry.ExecutionContext{
		RunID:     rec.RunID,
		AgentID:   agentIDForRun(rec),
		OwnerID:   rec.OwnerID,
		Profile:   configuredAgentProfileForRun(rec),
		Role:      agentRoleForRun(rec),
		ChannelID: channelIDForRun(rec),
		SandboxID: rec.SandboxID,
		DesktopID: desktopIDForRun(rec),
		RunRecord: rec,
	}
	if rec.Metadata != nil {
		execution.WorkingDir, _ = rec.Metadata[runMetadataToolCWD].(string)
		execution.OwnerEmail, _ = rec.Metadata[runMetadataOwnerEmail].(string)
	}
	return execution
}

func configuredAgentProfileForRun(rec *types.RunRecord) string {
	if rec == nil {
		return ""
	}
	if strings.TrimSpace(rec.AgentProfile) != "" {
		return agentprofile.Canonical(rec.AgentProfile)
	}
	if rec.Metadata != nil {
		if profile, _ := rec.Metadata[runMetadataAgentProfile].(string); strings.TrimSpace(profile) != "" {
			return agentprofile.Canonical(profile)
		}
	}
	if taskType, _ := rec.Metadata["type"].(string); isTextureAgentRevisionTaskType(taskType) {
		return agentprofile.Texture
	}
	return ""
}

func agentRoleForRun(rec *types.RunRecord) string {
	if rec == nil {
		return agentprofile.Super
	}
	if strings.TrimSpace(rec.AgentRole) != "" {
		return agentprofile.Canonical(rec.AgentRole)
	}
	if rec.Metadata != nil {
		if role, _ := rec.Metadata[runMetadataAgentRole].(string); strings.TrimSpace(role) != "" {
			return agentprofile.Canonical(role)
		}
	}
	return agentProfileForRun(rec)
}

func agentIDForRun(rec *types.RunRecord) string {
	if rec == nil {
		return ""
	}
	if strings.TrimSpace(rec.AgentID) != "" {
		return strings.TrimSpace(rec.AgentID)
	}
	if rec.Metadata != nil {
		if agentID, _ := rec.Metadata[runMetadataAgentID].(string); strings.TrimSpace(agentID) != "" {
			return strings.TrimSpace(agentID)
		}
	}
	return strings.TrimSpace(rec.RunID)
}

func channelIDForRun(rec *types.RunRecord) string {
	if rec == nil {
		return ""
	}
	if strings.TrimSpace(rec.ChannelID) != "" {
		return strings.TrimSpace(rec.ChannelID)
	}
	if rec.Metadata != nil {
		if channelID, _ := rec.Metadata[runMetadataChannelID].(string); strings.TrimSpace(channelID) != "" {
			return strings.TrimSpace(channelID)
		}
	}
	if strings.TrimSpace(rec.AgentID) != "" {
		return strings.TrimSpace(rec.AgentID)
	}
	return strings.TrimSpace(rec.RunID)
}

func desktopIDForRun(rec *types.RunRecord) string {
	if rec == nil {
		return types.PrimaryDesktopID
	}
	if rec.Metadata != nil {
		if desktopID, _ := rec.Metadata[runMetadataDesktopID].(string); strings.TrimSpace(desktopID) != "" {
			return strings.TrimSpace(desktopID)
		}
	}
	return types.PrimaryDesktopID
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
		return agentprofile.Super
	}
	if strings.TrimSpace(rec.AgentProfile) != "" {
		return agentprofile.Canonical(rec.AgentProfile)
	}
	if rec.Metadata != nil {
		if profile, _ := rec.Metadata[runMetadataAgentProfile].(string); strings.TrimSpace(profile) != "" {
			return agentprofile.Canonical(profile)
		}
	}
	if taskType, _ := rec.Metadata["type"].(string); taskType == textureAgentRevisionTaskType {
		return agentprofile.Texture
	}
	return agentprofile.Super
}
