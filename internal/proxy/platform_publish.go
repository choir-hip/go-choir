package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
)

const maxPublishedSourceSnapshotRunes = 20000

type publishTextureRequest struct {
	DocID        string          `json:"doc_id"`
	RevisionID   string          `json:"revision_id,omitempty"`
	Slug         string          `json:"slug,omitempty"`
	AccessPolicy json.RawMessage `json:"access_policy,omitempty"`
	ExportPolicy json.RawMessage `json:"export_policy,omitempty"`
}

type sandboxTextureDocument struct {
	DocID             string `json:"doc_id"`
	OwnerID           string `json:"owner_id"`
	Title             string `json:"title"`
	CurrentRevisionID string `json:"current_revision_id,omitempty"`
}

type sandboxTextureRevision struct {
	RevisionID       string          `json:"revision_id"`
	DocID            string          `json:"doc_id"`
	OwnerID          string          `json:"owner_id"`
	AuthorKind       string          `json:"author_kind,omitempty"`
	AuthorLabel      string          `json:"author_label,omitempty"`
	VersionNumber    int             `json:"version_number,omitempty"`
	Content          string          `json:"content"`
	BodyDoc          json.RawMessage `json:"body_doc,omitempty"`
	SourceEntities   json.RawMessage `json:"source_entities,omitempty"`
	Citations        json.RawMessage `json:"citations,omitempty"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
	Provenance       json.RawMessage `json:"provenance,omitempty"`
	RevisionHash     string          `json:"revision_hash,omitempty"`
	ParentRevisionID string          `json:"parent_revision_id,omitempty"`
	CreatedAt        string          `json:"created_at,omitempty"`
}

type sandboxTextureRevisionList struct {
	Revisions []sandboxTextureRevision `json:"revisions"`
}

type sandboxContentItem struct {
	ContentID    string          `json:"content_id"`
	OwnerID      string          `json:"owner_id"`
	SourceType   string          `json:"source_type"`
	MediaType    string          `json:"media_type"`
	AppHint      string          `json:"app_hint"`
	Title        string          `json:"title,omitempty"`
	SourceURL    string          `json:"source_url,omitempty"`
	CanonicalURL string          `json:"canonical_url,omitempty"`
	TextContent  string          `json:"text_content,omitempty"`
	ContentHash  string          `json:"content_hash,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	Provenance   json.RawMessage `json:"provenance,omitempty"`
}

func (h *Handler) HandleTexturePublication(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		h.lifecycle.record("platform_publish.method", "method_not_allowed", time.Since(started))
		return
	}

	authStarted := time.Now()
	authResult, err := h.validateAccessJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		h.lifecycle.record("platform_publish.auth", "unauthorized", time.Since(authStarted))
		h.lifecycle.record("platform_publish.total", "unauthorized", time.Since(started))
		return
	}
	h.lifecycle.record("platform_publish.auth", "ok", time.Since(authStarted))

	var req publishTextureRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		h.lifecycle.record("platform_publish.total", "bad_request", time.Since(started))
		return
	}
	req.DocID = strings.TrimSpace(req.DocID)
	req.RevisionID = strings.TrimSpace(req.RevisionID)
	req.Slug = strings.TrimSpace(req.Slug)
	if req.DocID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "doc_id is required"})
		h.lifecycle.record("platform_publish.total", "bad_request", time.Since(started))
		return
	}
	if err := validateOptionalJSONObject(req.AccessPolicy, "access_policy"); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		h.lifecycle.record("platform_publish.total", "bad_request", time.Since(started))
		return
	}
	if err := validateOptionalJSONObject(req.ExportPolicy, "export_policy"); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		h.lifecycle.record("platform_publish.total", "bad_request", time.Since(started))
		return
	}

	desktopID := requestDesktopID(r)
	resolveStarted := time.Now()
	sandboxURL, err := h.resolveSandboxURL(r.Context(), authResult.UserID, desktopID)
	if err != nil {
		log.Printf("proxy: platform publish failed to resolve sandbox for user %s desktop %s: %v", authResult.UserID, desktopID, err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to resolve user sandbox"})
		h.lifecycle.record("platform_publish.resolve", "error", time.Since(resolveStarted))
		h.lifecycle.record("platform_publish.total", "resolve_error", time.Since(started))
		return
	}
	h.lifecycle.record("platform_publish.resolve", "ok", time.Since(resolveStarted))

	var doc sandboxTextureDocument
	if err := h.fetchSandboxJSON(r, sandboxURL, "/api/texture/documents/"+url.PathEscape(req.DocID), authResult.UserID, &doc); err != nil {
		log.Printf("proxy: platform publish fetch document: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to load private texture document"})
		h.lifecycle.record("platform_publish.private_read", "document_error", time.Since(started))
		return
	}
	if doc.OwnerID != authResult.UserID || doc.DocID != req.DocID {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "document does not belong to authenticated user"})
		h.lifecycle.record("platform_publish.private_read", "owner_mismatch", time.Since(started))
		return
	}
	if req.RevisionID == "" {
		req.RevisionID = doc.CurrentRevisionID
	}
	if req.RevisionID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "document has no revision to publish"})
		h.lifecycle.record("platform_publish.total", "bad_request", time.Since(started))
		return
	}

	var rev sandboxTextureRevision
	if err := h.fetchSandboxJSON(r, sandboxURL, "/api/texture/revisions/"+url.PathEscape(req.RevisionID), authResult.UserID, &rev); err != nil {
		log.Printf("proxy: platform publish fetch revision: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to load private texture revision"})
		h.lifecycle.record("platform_publish.private_read", "revision_error", time.Since(started))
		return
	}
	if rev.OwnerID != authResult.UserID || rev.DocID != req.DocID || rev.RevisionID != req.RevisionID {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "revision does not belong to authenticated document"})
		h.lifecycle.record("platform_publish.private_read", "revision_mismatch", time.Since(started))
		return
	}
	if textureSourceEntitiesRequireBodyDoc(rev.SourceEntities, rev.BodyDoc) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "source_entities require body_doc source_ref nodes"})
		h.lifecycle.record("platform_publish.private_read", "detached_source_entities", time.Since(started))
		return
	}
	enrichedSourceEntities, err := h.enrichTexturePublicationSourceEntities(r, sandboxURL, authResult.UserID, rev.SourceEntities)
	if err != nil {
		log.Printf("proxy: platform publish enrich source metadata: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to prepare publication source metadata"})
		h.lifecycle.record("platform_publish.private_read", "source_metadata_error", time.Since(started))
		return
	}
	history, err := h.gatherTextureRevisionHistory(r, sandboxURL, authResult.UserID, req.DocID)
	if err != nil {
		log.Printf("proxy: platform publish gather revision history: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to load private texture revision history"})
		h.lifecycle.record("platform_publish.private_read", "history_error", time.Since(started))
		return
	}
	h.lifecycle.record("platform_publish.private_read", "ok", time.Since(started))

	platformReq := platform.PublishTextureRequest{
		OwnerID:          authResult.UserID,
		SourceDocID:      doc.DocID,
		SourceRevisionID: rev.RevisionID,
		Title:            doc.Title,
		Content:          rev.Content,
		BodyDoc:          rev.BodyDoc,
		SourceEntities:   enrichedSourceEntities,
		Citations:        rev.Citations,
		Metadata:         rev.Metadata,
		Slug:             req.Slug,
		AccessPolicy:     req.AccessPolicy,
		ExportPolicy:     req.ExportPolicy,
		RequestedBy:      authResult.UserID,
		History:          history,
	}
	platformResp, status, err := h.postPlatformPublication(r, platformReq)
	if err != nil {
		log.Printf("proxy: platform publish post platformd: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to publish texture"})
		h.lifecycle.record("platform_publish.platformd", "error", time.Since(started))
		h.lifecycle.record("platform_publish.total", "platform_error", time.Since(started))
		return
	}
	if status < 200 || status >= 300 {
		writeJSON(w, status, platformResp)
		h.lifecycle.record("platform_publish.platformd", lifecycleHTTPStatus(status), time.Since(started))
		h.lifecycle.record("platform_publish.total", lifecycleHTTPStatus(status), time.Since(started))
		return
	}
	if resp, ok := platformResp.(*platform.PublishTextureResponse); ok && resp.RoutePath != "" {
		resp.PublicURL = publicURLForRoute(r, resp.RoutePath)
	}
	writeJSON(w, status, platformResp)
	h.lifecycle.record("platform_publish.platformd", lifecycleHTTPStatus(status), time.Since(started))
	h.lifecycle.record("platform_publish.total", "published", time.Since(started))
}

// gatherTextureRevisionHistory loads the full revision chain for a document so
// publish can persist the whole versioned history (a Texture is its history, not
// only the head revision). The sandbox list endpoint returns newest-first; the
// chain is returned oldest-first so the persisted manifest's hash chain reads in
// causal order. Returns an empty chain (not an error) when no revisions are
// available so legacy/head-only behavior degrades gracefully.
func (h *Handler) gatherTextureRevisionHistory(r *http.Request, sandboxURL, userID, docID string) ([]platform.PublishTextureRevision, error) {
	var list sandboxTextureRevisionList
	if err := h.fetchSandboxJSON(r, sandboxURL, "/api/texture/documents/"+url.PathEscape(docID)+"/revisions", userID, &list); err != nil {
		return nil, err
	}
	revs := list.Revisions
	sort.SliceStable(revs, func(i, j int) bool {
		if revs[i].VersionNumber != revs[j].VersionNumber {
			return revs[i].VersionNumber < revs[j].VersionNumber
		}
		return revs[i].CreatedAt < revs[j].CreatedAt
	})
	history := make([]platform.PublishTextureRevision, 0, len(revs))
	for _, rev := range revs {
		if rev.OwnerID != "" && rev.OwnerID != userID {
			return nil, fmt.Errorf("revision %s does not belong to authenticated user", rev.RevisionID)
		}
		if textureSourceEntitiesRequireBodyDoc(rev.SourceEntities, rev.BodyDoc) {
			return nil, fmt.Errorf("revision %s source_entities require body_doc source_ref nodes", rev.RevisionID)
		}
		history = append(history, platform.PublishTextureRevision{
			RevisionID:       rev.RevisionID,
			ParentRevisionID: rev.ParentRevisionID,
			VersionNumber:    rev.VersionNumber,
			AuthorKind:       rev.AuthorKind,
			AuthorLabel:      rev.AuthorLabel,
			Content:          rev.Content,
			BodyDoc:          rev.BodyDoc,
			SourceEntities:   rev.SourceEntities,
			Citations:        rev.Citations,
			Metadata:         rev.Metadata,
			Provenance:       rev.Provenance,
			RevisionHash:     rev.RevisionHash,
			CreatedAt:        rev.CreatedAt,
		})
	}
	return history, nil
}

func validateOptionalJSONObject(raw json.RawMessage, label string) error {
	if len(raw) == 0 {
		return nil
	}
	if !json.Valid(raw) {
		return fmt.Errorf("%s must be valid JSON", label)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return fmt.Errorf("%s must be a JSON object", label)
	}
	return nil
}

func (h *Handler) enrichTexturePublicationSourceEntities(r *http.Request, sandboxURL, userID string, raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	var entities []any
	if err := json.Unmarshal(raw, &entities); err != nil {
		return raw, nil
	}
	if len(entities) == 0 {
		return raw, nil
	}
	changed := false
	for _, value := range entities {
		entity, ok := value.(map[string]any)
		if !ok || !sourceEntityAllowsPublishedSnapshot(entity) || mapValue(entity["reader_snapshot"]) != nil {
			continue
		}
		item, status, ok, err := h.publicationSourceSnapshotItem(r, sandboxURL, userID, entity)
		if err != nil {
			return nil, err
		}
		if !ok {
			if status != nil {
				entity["reader_snapshot_status"] = status
				changed = true
			}
			continue
		}
		if !contentItemAllowsPublishedSnapshot(item) {
			entity["reader_snapshot_status"] = map[string]any{"state": sourcecontract.ReaderArtifactStateNotPublicationSafe}
			changed = true
			continue
		}
		text := strings.TrimSpace(item.TextContent)
		if text == "" {
			entity["reader_snapshot_status"] = map[string]any{"state": sourcecontract.ReaderArtifactStateBoundedExcerptOnly, "reason": "source_import_empty"}
			changed = true
			continue
		}
		snapshotText, truncated := truncateRunes(text, maxPublishedSourceSnapshotRunes)
		entity["reader_snapshot"] = map[string]any{
			"snapshot_kind":       "cleaned_reader_markdown",
			"source":              "content_item",
			"source_content_id":   item.ContentID,
			"title":               firstNonEmptyString(item.Title, item.SourceURL, item.CanonicalURL),
			"source_url":          item.SourceURL,
			"canonical_url":       item.CanonicalURL,
			"media_type":          "text/markdown",
			"original_media_type": item.MediaType,
			"content_hash":        item.ContentHash,
			"text_content":        snapshotText,
			"text_char_count":     len([]rune(text)),
			"truncated":           truncated,
			"access_scope":        "publication_reader",
		}
		entity["reader_snapshot_status"] = map[string]any{
			"state":           sourcecontract.ReaderArtifactStateReady,
			"text_char_count": len([]rune(text)),
			"truncated":       truncated,
		}
		status = mapValue(entity["reader_snapshot_status"])
		for key, value := range publicationReaderSnapshotQuality(item) {
			status[key] = value
		}
		changed = true
	}
	if !changed {
		return raw, nil
	}
	out, err := json.Marshal(entities)
	if err != nil {
		return nil, fmt.Errorf("marshal enriched source_entities: %w", err)
	}
	return out, nil
}

func (h *Handler) publicationSourceSnapshotItem(r *http.Request, sandboxURL, userID string, entity map[string]any) (sandboxContentItem, map[string]any, bool, error) {
	if contentID := sourceEntityContentID(entity); contentID != "" {
		var item sandboxContentItem
		if err := h.fetchSandboxJSON(r, sandboxURL, "/api/content/items/"+url.PathEscape(contentID), userID, &item); err != nil {
			return sandboxContentItem{}, nil, false, fmt.Errorf("load content item %s: %w", contentID, err)
		}
		if item.OwnerID != userID || item.ContentID != contentID {
			return sandboxContentItem{}, nil, false, fmt.Errorf("content item %s does not belong to authenticated user", contentID)
		}
		return item, nil, true, nil
	}
	sourceURL := sourceEntityTargetURL(entity)
	if sourceURL == "" {
		return sandboxContentItem{}, sourceSnapshotStatus("source_target_missing", "source_target_missing", nil), false, nil
	}
	item, err := h.importSandboxURLContent(r, sandboxURL, userID, sourceURL, sourceEntityImportQuery(entity))
	if err != nil {
		log.Printf("proxy: platform publish source URL snapshot import failed for %s: %v", sourceURL, err)
		return sandboxContentItem{}, sourceSnapshotStatus("import_failed", "source_import_failed", classifySourceImportError(err)), false, nil
	}
	if item.OwnerID != userID {
		return sandboxContentItem{}, nil, false, fmt.Errorf("imported source URL item does not belong to authenticated user")
	}
	return item, nil, true, nil
}

func sourceSnapshotStatus(state, reason string, attrs map[string]any) map[string]any {
	if normalized := sourcecontract.NormalizeReaderArtifactState(state); normalized != "" {
		state = normalized
	}
	status := map[string]any{"state": state}
	if strings.TrimSpace(reason) != "" {
		status["reason"] = strings.TrimSpace(reason)
	}
	for key, value := range attrs {
		if strings.TrimSpace(key) == "" || value == nil {
			continue
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			continue
		}
		status[key] = value
	}
	return status
}

func classifySourceImportError(err error) map[string]any {
	message := ""
	if err != nil {
		message = strings.ToLower(err.Error())
	}
	out := map[string]any{"error_class": "import_error"}
	for _, status := range []int{400, 401, 403, 404, 408, 429, 500, 502, 503, 504} {
		if strings.Contains(message, fmt.Sprintf("%d", status)) {
			out["http_status"] = status
			out["error_class"] = fmt.Sprintf("http_%d", status)
			return out
		}
	}
	switch {
	case strings.Contains(message, "timeout"), strings.Contains(message, "deadline exceeded"):
		out["error_class"] = "timeout"
	case strings.Contains(message, "no such host"), strings.Contains(message, "dns"):
		out["error_class"] = "dns_error"
	case strings.Contains(message, "low-content"):
		out["error_class"] = "low_content"
	}
	return out
}

func sourceEntityAllowsPublishedSnapshot(entity map[string]any) bool {
	provenance := mapValue(entity["provenance"])
	rights := strings.ToLower(strings.TrimSpace(stringValue(provenance["rights_scope"])))
	switch rights {
	case "public_source", "official_public_source", "public_domain", "open_access", "public_url_snapshot":
		return true
	}
	policy := mapValue(entity["publication_policy"])
	if policy == nil {
		policy = mapValue(entity["access_policy"])
	}
	return boolValue(policy["publish_source_snapshot"]) || boolValue(policy["reader_snapshot"])
}

func contentItemAllowsPublishedSnapshot(item sandboxContentItem) bool {
	provenance := jsonObjectValue(item.Provenance)
	rights := strings.ToLower(strings.TrimSpace(stringValue(provenance["rights_scope"])))
	switch rights {
	case "", "public_source", "official_public_source", "public_domain", "open_access":
		return true
	case "private_user_source":
		return false
	}
	return boolValue(provenance["publish_source_snapshot"]) || boolValue(provenance["reader_snapshot"])
}

func sourceEntityContentID(entity map[string]any) string {
	target := mapValue(entity["target"])
	if strings.TrimSpace(stringValue(target["kind"])) == "content_item" {
		if id := strings.TrimSpace(stringValue(target["id"])); id != "" {
			return id
		}
	}
	return firstNonEmptyString(
		stringValue(target["content_id"]),
		stringValue(target["content_item_id"]),
		stringValue(entity["content_id"]),
		stringValue(entity["content_item_id"]),
	)
}

func sourceEntityTargetURL(entity map[string]any) string {
	target := mapValue(entity["target"])
	return firstNonEmptyString(
		stringValue(target["uri"]),
		stringValue(target["canonical_url"]),
		stringValue(target["url"]),
		stringValue(entity["canonical_url"]),
		stringValue(entity["url"]),
	)
}

func sourceEntityImportQuery(entity map[string]any) string {
	display := mapValue(entity["display"])
	target := mapValue(entity["target"])
	return firstNonEmptyString(
		stringValue(entity["label"]),
		stringValue(entity["title"]),
		stringValue(display["title"]),
		stringValue(display["label"]),
		stringValue(target["title"]),
		stringValue(entity["source_entity_id"]),
		stringValue(entity["entity_id"]),
	)
}

func jsonObjectValue(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func publicationReaderSnapshotQuality(item sandboxContentItem) map[string]any {
	out := map[string]any{}
	if item.MediaType != "" {
		out["original_media_type"] = item.MediaType
	}
	if metadata := jsonObjectValue(item.Metadata); metadata != nil {
		if retrievalStrategy := stringValue(metadata["retrieval_strategy"]); retrievalStrategy != "" {
			out["retrieval_strategy"] = retrievalStrategy
		}
	}
	warnings := contentItemExtractionWarnings(item)
	if len(warnings) > 0 {
		out["warnings"] = warnings
		out["warning_count"] = len(warnings)
		out["quality"] = "warning"
	} else {
		out["warning_count"] = 0
		out["quality"] = "cleaned_reader"
	}
	return out
}

func contentItemExtractionWarnings(item sandboxContentItem) []string {
	provenance := jsonObjectValue(item.Provenance)
	if provenance == nil {
		return nil
	}
	return compactStringList(provenance["warnings"], 8)
}

func compactStringList(value any, limit int) []string {
	values := []string{}
	seen := map[string]bool{}
	appendValue := func(raw any) {
		text := strings.TrimSpace(fmt.Sprint(raw))
		if text == "" || seen[text] {
			return
		}
		seen[text] = true
		values = append(values, text)
	}
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			appendValue(item)
			if limit > 0 && len(values) >= limit {
				break
			}
		}
	case []string:
		for _, item := range typed {
			appendValue(item)
			if limit > 0 && len(values) >= limit {
				break
			}
		}
	case string:
		appendValue(typed)
	}
	return values
}

func mapValue(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return nil
}

func stringValue(value any) string {
	if typed, ok := value.(string); ok {
		return strings.TrimSpace(typed)
	}
	return ""
}

func boolValue(value any) bool {
	if typed, ok := value.(bool); ok {
		return typed
	}
	if typed, ok := value.(string); ok {
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	}
	return false
}

func truncateRunes(value string, limit int) (string, bool) {
	if limit <= 0 {
		return "", value != ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value, false
	}
	return string(runes[:limit]), true
}

func (h *Handler) fetchSandboxJSON(r *http.Request, sandboxBase, path, userID string, out any) error {
	target, err := joinBasePath(sandboxBase, path)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil {
		return fmt.Errorf("build sandbox request: %w", err)
	}
	req.Header.Set("X-Authenticated-User", userID)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("call sandbox: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("sandbox status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode sandbox response: %w", err)
	}
	return nil
}

func (h *Handler) importSandboxURLContent(r *http.Request, sandboxBase, userID, sourceURL, query string) (sandboxContentItem, error) {
	target, err := joinBasePath(sandboxBase, "/api/content/import-url")
	if err != nil {
		return sandboxContentItem{}, err
	}
	payload := map[string]string{"url": sourceURL}
	if strings.TrimSpace(query) != "" {
		payload["query"] = strings.TrimSpace(query)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return sandboxContentItem{}, fmt.Errorf("marshal content import request: %w", err)
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return sandboxContentItem{}, fmt.Errorf("build sandbox import request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Authenticated-User", userID)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return sandboxContentItem{}, fmt.Errorf("call sandbox import: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return sandboxContentItem{}, fmt.Errorf("sandbox import status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var item sandboxContentItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return sandboxContentItem{}, fmt.Errorf("decode sandbox import response: %w", err)
	}
	return item, nil
}

func (h *Handler) postPlatformPublication(r *http.Request, req platform.PublishTextureRequest) (any, int, error) {
	target, err := joinBasePath(h.cfg.PlatformdURL, "/internal/platform/publications/texture")
	if err != nil {
		return nil, 0, err
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal platform request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return nil, 0, fmt.Errorf("build platform request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Caller", "true")
	resp, err := h.platformd.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("call platformd: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read platformd response: %w", err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var out platform.PublishTextureResponse
		if err := json.Unmarshal(body, &out); err != nil {
			return nil, resp.StatusCode, fmt.Errorf("decode platformd response: %w", err)
		}
		return &out, resp.StatusCode, nil
	}
	var out errorResponse
	if err := json.Unmarshal(body, &out); err != nil || out.Error == "" {
		out.Error = strings.TrimSpace(string(body))
		if out.Error == "" {
			out.Error = fmt.Sprintf("platformd status %d", resp.StatusCode)
		}
	}
	return out, resp.StatusCode, nil
}

func joinBasePath(rawBase, path string) (string, error) {
	u, err := url.Parse(strings.TrimRight(rawBase, "/"))
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}
	u.Path = "/" + strings.TrimLeft(path, "/")
	u.RawQuery = ""
	return u.String(), nil
}

func publicURLForRoute(r *http.Request, routePath string) string {
	proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		proto = "https"
		if r.TLS == nil && strings.HasPrefix(r.Host, "127.0.0.1") {
			proto = "http"
		}
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return routePath
	}
	return proto + "://" + host + "/" + strings.TrimLeft(routePath, "/")
}
