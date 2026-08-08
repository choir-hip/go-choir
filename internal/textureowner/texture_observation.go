package textureowner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	textureObservationSchemaV1 = "choir.texture_observation.v1"
	textureSourceOpenSchemaV1  = "choir.texture_source_open.v1"
)

type textureSourceIdentity struct {
	SourceRefCanonicalID    string          `json:"source_ref_canonical_id"`
	SourceRefVersionID      string          `json:"source_ref_version_id"`
	SourceRefHash           string          `json:"source_ref_hash"`
	DisplayMode             string          `json:"display_mode"`
	BodyNodeID              string          `json:"body_node_id,omitempty"`
	BodyNodePathHash        string          `json:"body_node_path_hash,omitempty"`
	SourceEntityCanonicalID string          `json:"source_entity_canonical_id"`
	SourceEntityVersionID   string          `json:"source_entity_version_id"`
	SourceEntityHash        string          `json:"source_entity_hash"`
	Selectors               json.RawMessage `json:"selectors,omitempty"`
	OpenSurface             string          `json:"open_surface,omitempty"`
	OpenPath                string          `json:"open_path"`
}

type textureDurableEvent struct {
	Schema           string                  `json:"schema"`
	Cursor           int64                   `json:"cursor"`
	EventID          string                  `json:"event_id"`
	Kind             string                  `json:"kind"`
	EventType        string                  `json:"event_type"`
	DocID            string                  `json:"doc_id"`
	TrajectoryID     string                  `json:"trajectory_id"`
	RevisionID       string                  `json:"revision_id,omitempty"`
	ParentRevisionID string                  `json:"parent_revision_id,omitempty"`
	VersionNumber    *int                    `json:"version_number,omitempty"`
	WorkState        string                  `json:"work_state"`
	TrajectoryStatus types.TrajectoryStatus  `json:"trajectory_status"`
	CommandID        string                  `json:"command_id"`
	RequestID        string                  `json:"request_id,omitempty"`
	UpdateID         string                  `json:"update_id,omitempty"`
	ControlID        string                  `json:"control_id,omitempty"`
	WorkItemID       string                  `json:"work_item_id,omitempty"`
	CommandDigest    string                  `json:"command_digest"`
	SourceIdentities []textureSourceIdentity `json:"source_identities,omitempty"`
	CreatedAt        time.Time               `json:"created_at"`
}

type textureDurableEventPage struct {
	Schema         string                `json:"schema"`
	DocID          string                `json:"doc_id"`
	OwnerID        string                `json:"owner_id"`
	ComputerID     string                `json:"computer_id"`
	TrajectoryID   string                `json:"trajectory_id"`
	Events         []textureDurableEvent `json:"events"`
	NextCursor     int64                 `json:"next_cursor"`
	Watermark      int64                 `json:"watermark"`
	CursorExpired  bool                  `json:"cursor_expired"`
	ReplayRequired bool                  `json:"replay_required"`
}

type textureSourceOpenResponse struct {
	Schema         string                            `json:"schema"`
	DocID          string                            `json:"doc_id"`
	RevisionID     string                            `json:"revision_id"`
	SourceIdentity textureSourceIdentity             `json:"source_identity"`
	SourceRef      textureSourceRefObjectResponse    `json:"source_ref"`
	SourceEntity   textureSourceEntityObjectResponse `json:"source_entity"`
	Target         json.RawMessage                   `json:"target,omitempty"`
}

func parseTextureObservationCursor(r *http.Request) (int64, int, error) {
	afterValue := strings.TrimSpace(r.URL.Query().Get("after"))
	if afterValue == "" {
		afterValue = firstNonEmpty(strings.TrimSpace(r.Header.Get("Last-Event-ID")), "0")
	}
	after, err := strconv.ParseInt(afterValue, 10, 64)
	if err != nil || after < 0 {
		return 0, 0, fmt.Errorf("after must be a non-negative integer")
	}
	limit, err := strconv.Atoi(firstNonEmpty(r.URL.Query().Get("limit"), "100"))
	if err != nil || limit <= 0 || limit > 1000 {
		return 0, 0, fmt.Errorf("limit must be between 1 and 1000")
	}
	return after, limit, nil
}

func (h *Handler) textureLifecycleEventPage(ctx context.Context, doc types.Document, after int64, limit int) (textureDurableEventPage, error) {
	out := textureDurableEventPage{
		Schema: textureObservationSchemaV1, DocID: doc.DocID, OwnerID: doc.OwnerID,
		ComputerID: doc.ComputerID, TrajectoryID: doc.TrajectoryID,
		Events: make([]textureDurableEvent, 0), NextCursor: after,
	}
	snapshot, err := h.Store.GetLifecycleSnapshot(ctx, doc.OwnerID, doc.ComputerID, doc.TrajectoryID)
	if err != nil {
		return out, err
	}
	if snapshot.Document.DocID != doc.DocID || snapshot.Trajectory.TrajectoryID != doc.TrajectoryID {
		return out, store.ErrNotFound
	}
	page, err := h.Store.ListLifecycleEventPage(ctx, doc.OwnerID, doc.ComputerID, doc.TrajectoryID, after, limit)
	out.NextCursor, out.Watermark = page.NextCursor, page.Watermark
	out.CursorExpired, out.ReplayRequired = page.CursorExpired, page.ReplayRequired
	if err != nil {
		return out, err
	}
	for _, event := range page.Events {
		projected, projectErr := h.projectTextureLifecycleEvent(ctx, doc, event)
		if projectErr != nil {
			return textureDurableEventPage{}, projectErr
		}
		out.Events = append(out.Events, projected)
	}
	return out, nil
}

func (h *Handler) projectTextureLifecycleEvent(ctx context.Context, doc types.Document, event types.LifecycleEvent) (textureDurableEvent, error) {
	status, workState := textureStatusAtEvent(event.Kind)
	out := textureDurableEvent{
		Schema: textureObservationSchemaV1, Cursor: event.ReducerSeq, EventID: event.EventID,
		Kind: string(event.Kind), EventType: "lifecycle", DocID: doc.DocID, TrajectoryID: doc.TrajectoryID,
		WorkState: workState, TrajectoryStatus: status,
		CommandID: event.CommandID, UpdateID: event.UpdateID,
		WorkItemID: event.WorkItemID, CommandDigest: event.CommandDigest, CreatedAt: event.CreatedAt,
	}
	if event.Kind == types.LifecycleControlQueued {
		out.EventType = "control"
		out.ControlID = event.UpdateID
	}
	for i := len(event.ArtifactRefs) - 1; i >= 0; i-- {
		revision, err := h.Store.GetLifecycleRevision(ctx, doc.OwnerID, doc.ComputerID, event.ArtifactRefs[i])
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return textureDurableEvent{}, err
		}
		if revision.DocID != doc.DocID || revision.TrajectoryID != doc.TrajectoryID {
			continue
		}
		out.EventType = "version"
		out.RevisionID, out.ParentRevisionID = revision.RevisionID, revision.ParentRevisionID
		version := revision.VersionNumber
		out.VersionNumber = &version
		identities, err := h.textureRevisionSourceIdentities(ctx, revision)
		if err != nil {
			return textureDurableEvent{}, err
		}
		out.SourceIdentities = identities
		break
	}
	return out, nil
}

func textureStatusAtEvent(kind types.LifecycleEventKind) (types.TrajectoryStatus, string) {
	switch kind {
	case types.LifecycleTrajectorySettled:
		return types.TrajectorySettled, "terminal"
	case types.LifecycleTrajectoryCancelled:
		return types.TrajectoryCancelled, "terminal"
	default:
		return types.TrajectoryLive, "working"
	}
}

func (h *Handler) textureRevisionSourceIdentities(ctx context.Context, revision types.Revision) ([]textureSourceIdentity, error) {
	refs, err := h.Store.ListTextureSourceRefsForRevisionByScope(ctx, revision.OwnerID, revision.ComputerID, revision.DocID, revision.RevisionID)
	if err != nil {
		return nil, err
	}
	entities, err := h.Store.ListTextureSourceEntitiesForRevisionByScope(ctx, revision.OwnerID, revision.ComputerID, revision.DocID, revision.RevisionID)
	if err != nil {
		return nil, err
	}
	byVersion := make(map[string]store.TextureSourceEntityGraphRecord, len(entities))
	for _, entity := range entities {
		byVersion[entity.CanonicalID+"\x00"+entity.VersionID] = entity
	}
	out := make([]textureSourceIdentity, 0, len(refs))
	for _, ref := range refs {
		entity, ok := byVersion[ref.SourceEntityCanonicalID+"\x00"+ref.SourceEntityVersionID]
		if !ok {
			return nil, fmt.Errorf("source_ref %s/%s points at unavailable source entity version", ref.CanonicalID, ref.VersionID)
		}
		out = append(out, textureSourceIdentityFromRecords(revision, ref, entity))
	}
	return out, nil
}

func textureSourceIdentityFromRecords(revision types.Revision, ref store.TextureSourceRefGraphRecord, entity store.TextureSourceEntityGraphRecord) textureSourceIdentity {
	selectors, openSurface, _ := textureSourceOpenMetadata(entity.Metadata)
	return textureSourceIdentity{
		SourceRefCanonicalID: ref.CanonicalID, SourceRefVersionID: ref.VersionID, SourceRefHash: ref.ContentHash,
		DisplayMode: ref.DisplayMode, BodyNodeID: ref.BodyNodeID, BodyNodePathHash: ref.BodyNodePathHash,
		SourceEntityCanonicalID: entity.CanonicalID, SourceEntityVersionID: entity.VersionID, SourceEntityHash: entity.ContentHash,
		Selectors: selectors, OpenSurface: openSurface,
		OpenPath: textureExactSourceOpenPath(revision.DocID, revision.RevisionID, ref.CanonicalID, ref.VersionID),
	}
}

func textureSourceOpenMetadata(raw json.RawMessage) (json.RawMessage, string, json.RawMessage) {
	var metadata map[string]json.RawMessage
	if json.Unmarshal(raw, &metadata) != nil {
		return nil, "", nil
	}
	selector := metadata["selectors"]
	var evidence struct {
		OpenSurface string `json:"open_surface"`
	}
	_ = json.Unmarshal(metadata["evidence"], &evidence)
	return selector, strings.TrimSpace(evidence.OpenSurface), metadata["target"]
}

func textureExactSourceOpenPath(docID, revisionID, sourceRefID, sourceRefVersionID string) string {
	query := url.Values{}
	query.Set("revision_id", revisionID)
	query.Set("source_ref_id", sourceRefID)
	query.Set("source_ref_version_id", sourceRefVersionID)
	return "/api/texture/documents/" + url.PathEscape(docID) + "/source-open?" + query.Encode()
}

func (h *Handler) HandleTextureDocumentEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	doc, err := h.getTextureDocument(r.Context(), ownerID, extractDocID(r.URL.Path))
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}
	if strings.TrimSpace(doc.TrajectoryID) == "" {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "document has no durable lifecycle event authority"})
		return
	}
	after, limit, err := parseTextureObservationCursor(r)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid Texture observation cursor: " + err.Error()})
		return
	}
	page, pageErr := h.textureLifecycleEventPage(r.Context(), doc, after, limit)
	if errors.Is(pageErr, store.ErrLifecycleCursorExpired) {
		writeAPIJSON(w, http.StatusConflict, page)
		return
	}
	if pageErr != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load durable Texture events"})
		return
	}
	writeAPIJSON(w, http.StatusOK, page)
}

func (h *Handler) handleLifecycleTextureDocumentStream(w http.ResponseWriter, r *http.Request, doc types.Document) {
	after, limit, err := parseTextureObservationCursor(r)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid Texture observation cursor: " + err.Error()})
		return
	}
	if r.URL.Query().Get("format") == "json" {
		page, pageErr := h.textureLifecycleEventPage(r.Context(), doc, after, limit)
		if errors.Is(pageErr, store.ErrLifecycleCursorExpired) {
			writeAPIJSON(w, http.StatusConflict, page)
			return
		}
		if pageErr != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load durable Texture events"})
			return
		}
		writeAPIJSON(w, http.StatusOK, page)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "Texture streaming unavailable"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	cursor := after
	once := r.URL.Query().Get("once") == "true" || r.URL.Query().Get("once") == "1"
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		page, pageErr := h.textureLifecycleEventPage(r.Context(), doc, cursor, limit)
		if errors.Is(pageErr, store.ErrLifecycleCursorExpired) {
			payload, _ := json.Marshal(page)
			fmt.Fprintf(w, "event: replay_required\ndata: %s\n\n", payload)
			flusher.Flush()
			return
		}
		if pageErr != nil {
			return
		}
		for _, event := range page.Events {
			payload, _ := json.Marshal(event)
			fmt.Fprintf(w, "id: %d\nevent: texture\ndata: %s\n\n", event.Cursor, payload)
			cursor = event.Cursor
		}
		if len(page.Events) > 0 {
			flusher.Flush()
		}
		if once {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case <-ticker.C:
		}
	}
}

func (h *Handler) HandleTextureSourceOpen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	docID := extractDocID(r.URL.Path)
	doc, err := h.getTextureDocument(r.Context(), ownerID, docID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}
	revisionID := strings.TrimSpace(r.URL.Query().Get("revision_id"))
	refID := strings.TrimSpace(r.URL.Query().Get("source_ref_id"))
	refVersionID := strings.TrimSpace(r.URL.Query().Get("source_ref_version_id"))
	if revisionID == "" || refID == "" || refVersionID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "revision_id, source_ref_id, and source_ref_version_id are required"})
		return
	}
	revision, err := h.getTextureRevision(r.Context(), ownerID, revisionID)
	if err != nil || revision.DocID != doc.DocID || revision.ComputerID != doc.ComputerID {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "revision not found"})
		return
	}
	refs, err := h.Store.ListTextureSourceRefsForRevisionByScope(r.Context(), ownerID, doc.ComputerID, doc.DocID, revisionID)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load source references"})
		return
	}
	var matchedRef *store.TextureSourceRefGraphRecord
	for i := range refs {
		if refs[i].CanonicalID == refID && refs[i].VersionID == refVersionID {
			matchedRef = &refs[i]
			break
		}
	}
	if matchedRef == nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "source reference not found"})
		return
	}
	entities, err := h.Store.ListTextureSourceEntitiesForRevisionByScope(r.Context(), ownerID, doc.ComputerID, doc.DocID, revisionID)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load source entity"})
		return
	}
	var matchedEntity *store.TextureSourceEntityGraphRecord
	for i := range entities {
		if entities[i].CanonicalID == matchedRef.SourceEntityCanonicalID && entities[i].VersionID == matchedRef.SourceEntityVersionID {
			matchedEntity = &entities[i]
			break
		}
	}
	if matchedEntity == nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "source reference target version is unavailable"})
		return
	}
	identity := textureSourceIdentityFromRecords(revision, *matchedRef, *matchedEntity)
	_, _, target := textureSourceOpenMetadata(matchedEntity.Metadata)
	writeAPIJSON(w, http.StatusOK, textureSourceOpenResponse{
		Schema: textureSourceOpenSchemaV1, DocID: doc.DocID, RevisionID: revision.RevisionID,
		SourceIdentity: identity, SourceRef: textureSourceRefObjectResponseFromRecord(*matchedRef),
		SourceEntity: textureSourceEntityObjectResponseFromRecord(*matchedEntity), Target: target,
	})
}
