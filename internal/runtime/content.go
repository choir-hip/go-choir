package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const maxImportedContentBytes = 2 * 1024 * 1024
const maxStoredExtractedText = 300 * 1024

type contentItemListResponse struct {
	Items []types.ContentItem `json:"items"`
}

type contentCreateRequest struct {
	SourceType   string          `json:"source_type"`
	MediaType    string          `json:"media_type,omitempty"`
	AppHint      string          `json:"app_hint,omitempty"`
	Title        string          `json:"title,omitempty"`
	SourceURL    string          `json:"source_url,omitempty"`
	CanonicalURL string          `json:"canonical_url,omitempty"`
	FilePath     string          `json:"file_path,omitempty"`
	TextContent  string          `json:"text_content,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	Provenance   json.RawMessage `json:"provenance,omitempty"`
}

type contentImportURLRequest struct {
	URL   string `json:"url"`
	Query string `json:"query,omitempty"`
}

type extractionRung struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Endpoint    string `json:"endpoint,omitempty"`
	StatusCode  int    `json:"status_code,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	LatencyMs   int64  `json:"latency_ms,omitempty"`
	Bytes       int    `json:"bytes,omitempty"`
	TextChars   int    `json:"text_chars,omitempty"`
	Candidates  int    `json:"candidates,omitempty"`
	Error       string `json:"error,omitempty"`
}

type searxngCandidate struct {
	Title   string `json:"title,omitempty"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
	Engine  string `json:"engine,omitempty"`
}

type fetchedURLContent struct {
	URL         string
	StatusCode  int
	ContentType string
	MediaType   string
	Title       string
	Text        string
	RawHash     string
	RawBytes    int
	Rungs       []extractionRung
	Warnings    []string
}

type youtubeTranscriptSegment struct {
	Start    float64 `json:"start"`
	Duration float64 `json:"duration,omitempty"`
	Text     string  `json:"text"`
}

type youtubeTranscriptFetchResult struct {
	Availability string
	Language     string
	Kind         string
	Provider     string
	Text         string
	Segments     []youtubeTranscriptSegment
	Error        string
}

type youtubeTranscriptProviderConfig struct {
	Name       string
	BaseURL    string
	APIKey     string
	AuthScheme string
}

type youtubeCaptionTrack struct {
	BaseURL      string `json:"baseUrl"`
	LanguageCode string `json:"languageCode"`
	Kind         string `json:"kind"`
}

func (h *APIHandler) HandleContentItemsRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.HandleContentList(w, r)
	case http.MethodPost:
		h.HandleContentCreate(w, r)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

func (h *APIHandler) HandleContentRouter(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/content/import-url" {
		h.HandleContentImportURL(w, r)
		return
	}
	const prefix = "/api/content/items/"
	if strings.HasPrefix(r.URL.Path, prefix) {
		h.HandleContentItem(w, r)
		return
	}
	writeAPIJSON(w, http.StatusNotFound, apiError{Error: "content endpoint not found"})
}

func (h *APIHandler) HandleContentList(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	items, err := h.rt.Store().ListContentItems(r.Context(), ownerID, limit)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list content items"})
		return
	}
	writeAPIJSON(w, http.StatusOK, contentItemListResponse{Items: items})
}

func (h *APIHandler) HandleContentItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	contentID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/content/items/"))
	if contentID == "" || strings.Contains(contentID, "/") {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "content item not found"})
		return
	}
	item, err := h.rt.Store().GetContentItem(r.Context(), ownerID, contentID)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "content item not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load content item"})
		return
	}
	writeAPIJSON(w, http.StatusOK, item)
}

func (h *APIHandler) HandleContentCreate(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	var req contentCreateRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid content item request"})
		return
	}
	item, err := buildContentItem(ownerID, req)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	if err := h.rt.Store().CreateContentItem(r.Context(), item); err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create content item"})
		return
	}
	_, _ = h.rt.emitProductEvent(r.Context(), ownerID, requestDesktopID(r), types.EventContentItemCreated, contentItemEventPayload(item))
	writeAPIJSON(w, http.StatusCreated, item)
}

func (h *APIHandler) HandleContentImportURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	var req contentImportURLRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid URL import request"})
		return
	}
	item, err := h.rt.ImportURLContent(r.Context(), ownerID, strings.TrimSpace(req.URL), strings.TrimSpace(req.Query))
	if err != nil {
		writeAPIJSON(w, http.StatusBadGateway, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusCreated, item)
}

func buildContentItem(ownerID string, req contentCreateRequest) (types.ContentItem, error) {
	sourceType := normalizeContentSourceType(req.SourceType)
	if sourceType == "" {
		return types.ContentItem{}, fmt.Errorf("source_type is required")
	}
	sourceURL := strings.TrimSpace(req.SourceURL)
	filePath := strings.TrimSpace(req.FilePath)
	text := strings.TrimSpace(req.TextContent)
	if sourceType == "url" || sourceType == "extracted_url" {
		if _, err := normalizeHTTPURL(sourceURL); err != nil {
			return types.ContentItem{}, err
		}
	}
	if sourceType == "file" || sourceType == "upload" {
		if filePath == "" {
			return types.ContentItem{}, fmt.Errorf("file_path is required for file content")
		}
	}
	if sourceURL == "" && filePath == "" && text == "" {
		return types.ContentItem{}, fmt.Errorf("content item needs source_url, file_path, or text_content")
	}
	now := time.Now().UTC()
	mediaType := normalizeMediaType(req.MediaType)
	if mediaType == "" {
		mediaType = detectMediaType(sourceURL, filePath, "")
	}
	hash := contentHash(text)
	item := types.ContentItem{
		ContentID:    uuid.NewString(),
		OwnerID:      ownerID,
		SourceType:   sourceType,
		MediaType:    mediaType,
		AppHint:      normalizeAppHint(nonEmpty(req.AppHint, appHintForMedia(mediaType, sourceURL, filePath))),
		Title:        strings.TrimSpace(req.Title),
		SourceURL:    sourceURL,
		CanonicalURL: strings.TrimSpace(req.CanonicalURL),
		FilePath:     filePath,
		TextContent:  text,
		ContentHash:  hash,
		Metadata:     ensureJSONObject(req.Metadata),
		Provenance:   ensureJSONObject(req.Provenance),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if item.Title == "" {
		item.Title = fallbackContentTitle(item)
	}
	return item, nil
}

func (rt *Runtime) ImportURLContent(ctx context.Context, ownerID, rawURL, query string) (types.ContentItem, error) {
	normalizedURL, err := normalizeHTTPURL(rawURL)
	if err != nil {
		return types.ContentItem{}, err
	}
	if isYouTubeURL(normalizedURL) {
		return rt.importYouTubeURLContent(ctx, ownerID, normalizedURL)
	}
	if existing, ok := rt.findExistingURLContentItem(ctx, ownerID, normalizedURL, ""); ok {
		return existing, nil
	}
	started := time.Now().UTC()
	rungs := []extractionRung{}
	warnings := []string{}
	candidates := []searxngCandidate{}

	client := &http.Client{Timeout: 30 * time.Second}

	selected, primaryErr := fetchAndExtractURL(ctx, client, normalizedURL, "direct_http", "readability_lite", "plain_text")
	rungs = append(rungs, selected.Rungs...)
	warnings = append(warnings, selected.Warnings...)
	if primaryErr != nil {
		warnings = append(warnings, "direct fetch failed: "+primaryErr.Error())
	}
	if shouldRunSearXNGDiscovery(query, selected, primaryErr) {
		discovered, discoveryRung := discoverSearXNGCandidates(ctx, client, searxngBaseURL(), query, normalizedURL)
		rungs = append(rungs, discoveryRung)
		candidates = discovered
		for _, candidate := range discovered {
			alternate, alternateErr := fetchAndExtractURL(ctx, client, candidate.URL, "searxng_alt_http", "searxng_alt_readability_lite", "searxng_alt_plain_text")
			rungs = append(rungs, alternate.Rungs...)
			warnings = append(warnings, alternate.Warnings...)
			if alternateErr != nil {
				warnings = append(warnings, "alternate fetch failed for "+candidate.URL+": "+alternateErr.Error())
				continue
			}
			if betterFetchedContent(alternate, selected) {
				selected = alternate
				break
			}
		}
	}
	if primaryErr != nil && strings.TrimSpace(selected.Text) == "" {
		return types.ContentItem{}, fmt.Errorf("URL import failed: %w", primaryErr)
	}
	if len(strings.TrimSpace(selected.Text)) < 400 {
		warnings = append(warnings, "extracted text is low-content")
	}

	provenance, _ := json.Marshal(map[string]any{
		"source_url":     normalizedURL,
		"fetched_at":     started.Format(time.RFC3339Nano),
		"rungs":          rungs,
		"warnings":       warnings,
		"candidates":     candidates,
		"hash_algorithm": "sha256",
	})
	metadata, _ := json.Marshal(map[string]any{
		"query":              query,
		"http_status":        selected.StatusCode,
		"http_content_type":  selected.ContentType,
		"retrieval_strategy": retrievalStrategy(rungs),
	})
	sourceType := "extracted_url"
	if !isHTMLMedia(selected.MediaType) && !isTextMedia(selected.MediaType) {
		sourceType = "url"
	}
	itemReq := contentCreateRequest{
		SourceType:   sourceType,
		MediaType:    selected.MediaType,
		AppHint:      appHintForMedia(selected.MediaType, selected.URL, ""),
		Title:        selected.Title,
		SourceURL:    normalizedURL,
		CanonicalURL: selected.URL,
		TextContent:  selected.Text,
		Metadata:     metadata,
		Provenance:   provenance,
	}
	item, err := buildContentItem(ownerID, itemReq)
	if err != nil {
		return types.ContentItem{}, err
	}
	if item.ContentHash == "" {
		item.ContentHash = selected.RawHash
	}
	if err := rt.Store().CreateContentItem(ctx, item); err != nil {
		return types.ContentItem{}, err
	}
	_, _ = rt.emitProductEvent(ctx, ownerID, types.PrimaryDesktopID, types.EventContentItemCreated, contentItemEventPayload(item))
	return item, nil
}

func contentItemEventPayload(item types.ContentItem) map[string]any {
	return map[string]any{
		"content_id":   item.ContentID,
		"source_type":  item.SourceType,
		"media_type":   item.MediaType,
		"app_hint":     item.AppHint,
		"title":        item.Title,
		"source_url":   item.SourceURL,
		"file_path":    item.FilePath,
		"content_hash": item.ContentHash,
		"created_at":   item.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":   item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func fetchAndExtractURL(ctx context.Context, client *http.Client, targetURL, fetchRungName, htmlRungName, textRungName string) (fetchedURLContent, error) {
	result := fetchedURLContent{URL: targetURL}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return result, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ChoirBot/0.1; +https://choir.news)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,text/plain;q=0.8,*/*;q=0.5")

	fetchStarted := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		result.Rungs = append(result.Rungs, extractionRung{Name: fetchRungName, Status: "error", LatencyMs: time.Since(fetchStarted).Milliseconds(), Error: err.Error()})
		return result, err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxImportedContentBytes+1))
	if readErr != nil {
		return result, fmt.Errorf("read URL response: %w", readErr)
	}
	if len(raw) > maxImportedContentBytes {
		raw = raw[:maxImportedContentBytes]
		result.Warnings = append(result.Warnings, "response truncated at 2MiB")
	}
	contentType := normalizeMediaType(resp.Header.Get("Content-Type"))
	result.StatusCode = resp.StatusCode
	result.ContentType = contentType
	result.RawHash = contentHashBytes(raw)
	result.RawBytes = len(raw)
	result.Rungs = append(result.Rungs, extractionRung{
		Name:        fetchRungName,
		Status:      statusForHTTP(resp.StatusCode),
		StatusCode:  resp.StatusCode,
		ContentType: contentType,
		LatencyMs:   time.Since(fetchStarted).Milliseconds(),
		Bytes:       len(raw),
	})
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return result, fmt.Errorf("%s returned status %s", fetchRungName, resp.Status)
	}

	result.MediaType = detectMediaType(targetURL, "", contentType)
	if isHTMLMedia(result.MediaType) {
		result.Title, result.Text = extractReadableHTML(raw)
		result.Rungs = append(result.Rungs, extractionRung{Name: htmlRungName, Status: statusForText(result.Text), TextChars: len(result.Text)})
	} else if isTextMedia(result.MediaType) {
		result.Text = strings.TrimSpace(string(raw))
		if result.MediaType == "application/rss+xml" {
			result.Title = extractRSSFeedTitle(raw)
		}
		result.Rungs = append(result.Rungs, extractionRung{Name: textRungName, Status: statusForText(result.Text), TextChars: len(result.Text)})
	}
	if len(result.Text) > maxStoredExtractedText {
		result.Text = result.Text[:maxStoredExtractedText]
		result.Warnings = append(result.Warnings, "extracted text truncated at 300KiB")
	}
	return result, nil
}

func (rt *Runtime) importYouTubeURLContent(ctx context.Context, ownerID, normalizedURL string) (types.ContentItem, error) {
	videoID := youtubeVideoID(normalizedURL)
	if videoID == "" {
		return types.ContentItem{}, fmt.Errorf("youtube url is missing a video id")
	}
	canonicalURL := "https://www.youtube.com/watch?v=" + videoID
	if existing, ok := rt.findExistingURLContentItem(ctx, ownerID, canonicalURL, "video/youtube"); ok {
		return existing, nil
	}

	now := time.Now().UTC()
	videoContentID := uuid.NewString()
	transcript := fetchYouTubeTranscript(ctx, videoID)
	transcriptContentID := ""
	if strings.TrimSpace(transcript.Text) != "" || transcript.Availability != "" {
		transcriptContentID = uuid.NewString()
	}
	metadata := map[string]any{
		"platform":                "youtube",
		"video_id":                videoID,
		"transcript_availability": firstNonEmpty(transcript.Availability, "unavailable"),
	}
	if transcriptContentID != "" {
		metadata["transcript_content_id"] = transcriptContentID
	}
	videoMetadata, _ := json.Marshal(metadata)
	videoProvenance, _ := json.Marshal(map[string]any{
		"source_url":              normalizedURL,
		"canonical_url":           canonicalURL,
		"registered_at":           now.Format(time.RFC3339Nano),
		"rights_scope":            "private_user_source",
		"untrusted_source_media":  true,
		"transcript_provider":     firstNonEmpty(transcript.Provider, "youtube_caption_tracks"),
		"transcript_fetch_status": firstNonEmpty(transcript.Availability, "unavailable"),
	})
	videoItem := types.ContentItem{
		ContentID:    videoContentID,
		OwnerID:      ownerID,
		SourceType:   "url",
		MediaType:    "video/youtube",
		AppHint:      "video",
		Title:        "YouTube " + videoID,
		SourceURL:    normalizedURL,
		CanonicalURL: canonicalURL,
		ContentHash:  contentHash(canonicalURL),
		Metadata:     videoMetadata,
		Provenance:   videoProvenance,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := rt.Store().CreateContentItem(ctx, videoItem); err != nil {
		return types.ContentItem{}, err
	}
	_, _ = rt.emitProductEvent(ctx, ownerID, types.PrimaryDesktopID, types.EventContentItemCreated, contentItemEventPayload(videoItem))

	if transcriptContentID != "" {
		transcriptMetadata, _ := json.Marshal(map[string]any{
			"platform":         "youtube",
			"video_content_id": videoContentID,
			"video_id":         videoID,
			"language":         transcript.Language,
			"kind":             transcript.Kind,
			"provider":         firstNonEmpty(transcript.Provider, "youtube_caption_tracks"),
			"segments":         transcript.Segments,
			"fetched_at":       now.Format(time.RFC3339Nano),
			"availability":     firstNonEmpty(transcript.Availability, "unavailable"),
			"error":            transcript.Error,
		})
		transcriptProvenance, _ := json.Marshal(map[string]any{
			"source_url":            canonicalURL,
			"rights_scope":          "private_user_source",
			"untrusted_source_text": true,
		})
		transcriptItem := types.ContentItem{
			ContentID:    transcriptContentID,
			OwnerID:      ownerID,
			SourceType:   "derived_transcript",
			MediaType:    "text/x-youtube-transcript",
			AppHint:      "vtext",
			Title:        "Transcript for YouTube " + videoID,
			SourceURL:    canonicalURL,
			CanonicalURL: "youtube://" + videoID + "/transcript/" + firstNonEmpty(transcript.Language, "unknown"),
			TextContent:  strings.TrimSpace(transcript.Text),
			ContentHash:  contentHash(strings.TrimSpace(transcript.Text)),
			Metadata:     transcriptMetadata,
			Provenance:   transcriptProvenance,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if transcriptItem.ContentHash == "" {
			transcriptItem.ContentHash = contentHash(transcriptItem.CanonicalURL + ":" + firstNonEmpty(transcript.Availability, "unavailable") + ":" + transcript.Error)
		}
		if err := rt.Store().CreateContentItem(ctx, transcriptItem); err != nil {
			return types.ContentItem{}, err
		}
		_, _ = rt.emitProductEvent(ctx, ownerID, types.PrimaryDesktopID, types.EventContentItemCreated, contentItemEventPayload(transcriptItem))
	}
	return videoItem, nil
}

func (rt *Runtime) findExistingURLContentItem(ctx context.Context, ownerID, canonicalURL, mediaType string) (types.ContentItem, bool) {
	if rt == nil || rt.Store() == nil || strings.TrimSpace(ownerID) == "" || strings.TrimSpace(canonicalURL) == "" {
		return types.ContentItem{}, false
	}
	items, err := rt.Store().ListContentItems(ctx, ownerID, 1000)
	if err != nil {
		return types.ContentItem{}, false
	}
	normalizedMedia := normalizeMediaType(mediaType)
	for _, item := range items {
		if normalizedMedia != "" && normalizeMediaType(item.MediaType) != normalizedMedia {
			continue
		}
		if sameNormalizedURL(item.CanonicalURL, canonicalURL) || sameNormalizedURL(item.SourceURL, canonicalURL) || strings.TrimSpace(item.CanonicalURL) == strings.TrimSpace(canonicalURL) {
			return item, true
		}
	}
	return types.ContentItem{}, false
}

func fetchYouTubeTranscript(ctx context.Context, videoID string) youtubeTranscriptFetchResult {
	result := youtubeTranscriptFetchResult{
		Availability: "unavailable",
		Provider:     "youtube_caption_tracks",
	}
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		result.Error = "missing video id"
		return result
	}
	if os.Getenv("CHOIR_DISABLE_YOUTUBE_TRANSCRIPT_FETCH") == "1" {
		result.Error = "transcript fetch disabled"
		return result
	}
	if cfg, ok := configuredYouTubeTranscriptProvider(); ok {
		providerResult := fetchConfiguredYouTubeTranscript(ctx, videoID, cfg)
		if providerResult.Availability == "available" {
			return providerResult
		}
		result.Error = providerResult.Error
		result.Provider = providerResult.Provider
	}
	innerTubeResult := fetchYouTubeTranscriptFromInnerTube(ctx, videoID)
	if innerTubeResult.Availability == "available" {
		return innerTubeResult
	}
	captionResult := fetchYouTubeTranscriptFromCaptionTracks(ctx, videoID)
	if captionResult.Availability == "available" {
		return captionResult
	}
	if innerTubeResult.Error != "" {
		if result.Error == "" {
			result.Error = innerTubeResult.Error
			result.Provider = innerTubeResult.Provider
		} else {
			result.Error += "; " + innerTubeResult.Error
			result.Provider += "+" + innerTubeResult.Provider
		}
	}
	if result.Error != "" {
		captionErr := captionResult.Error
		if captionErr == "" {
			captionErr = "caption-track fallback unavailable"
		}
		captionResult.Error = result.Error + "; " + captionErr
		captionResult.Provider = result.Provider + "+youtube_caption_tracks"
	}
	return captionResult
}

func fetchYouTubeTranscriptFromInnerTube(ctx context.Context, videoID string) youtubeTranscriptFetchResult {
	result := youtubeTranscriptFetchResult{
		Availability: "unavailable",
		Provider:     "youtube_innertube_android",
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 8 * time.Second}
	playerURL := youtubeInnerTubePlayerEndpoint()
	body, _ := json.Marshal(map[string]any{
		"context": map[string]any{
			"client": map[string]any{
				"clientName":        "ANDROID",
				"clientVersion":     "20.10.38",
				"androidSdkVersion": 35,
				"hl":                "en",
				"gl":                "US",
				"userAgent":         "com.google.android.youtube/20.10.38 (Linux; U; Android 15) gzip",
			},
		},
		"videoId": videoID,
	})
	req, err := http.NewRequestWithContext(fetchCtx, http.MethodPost, playerURL, bytes.NewReader(body))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	applyYouTubeInnerTubeHeaders(req)
	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxImportedContentBytes))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		result.Error = fmt.Sprintf("innertube player returned status %s: %s", resp.Status, strings.TrimSpace(string(raw)))
		return result
	}
	var player struct {
		PlayabilityStatus struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		} `json:"playabilityStatus"`
		Captions struct {
			PlayerCaptionsTracklistRenderer struct {
				CaptionTracks []youtubeCaptionTrack `json:"captionTracks"`
			} `json:"playerCaptionsTracklistRenderer"`
		} `json:"captions"`
	}
	if err := json.Unmarshal(raw, &player); err != nil {
		result.Error = err.Error()
		return result
	}
	if status := strings.TrimSpace(player.PlayabilityStatus.Status); status != "" && status != "OK" {
		result.Error = strings.TrimSpace(status + " " + player.PlayabilityStatus.Reason)
		return result
	}
	track, ok := chooseYouTubeCaptionTrack(player.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks)
	if !ok {
		result.Error = "caption tracks unavailable"
		return result
	}
	captionURL := youtubeJSON3CaptionURL(track.BaseURL)
	captionReq, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, captionURL, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	applyYouTubeInnerTubeHeaders(captionReq)
	captionResp, err := client.Do(captionReq)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer func() { _ = captionResp.Body.Close() }()
	if captionResp.StatusCode < 200 || captionResp.StatusCode >= 400 {
		result.Error = captionResp.Status
		return result
	}
	captionRaw, err := io.ReadAll(io.LimitReader(captionResp.Body, maxImportedContentBytes))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	segments, text := parseYouTubeJSON3Transcript(captionRaw)
	if strings.TrimSpace(text) == "" {
		result.Error = "caption track had no text"
		return result
	}
	result.Availability = "available"
	result.Language = track.LanguageCode
	result.Kind = firstNonEmpty(track.Kind, "caption")
	result.Segments = segments
	result.Text = text
	result.Error = ""
	return result
}

func youtubeInnerTubePlayerEndpoint() string {
	if value := strings.TrimSpace(os.Getenv("CHOIR_YOUTUBE_INNERTUBE_PLAYER_URL")); value != "" {
		return value
	}
	return "https://www.youtube.com/youtubei/v1/player?prettyPrint=false"
}

func applyYouTubeInnerTubeHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "com.google.android.youtube/20.10.38 (Linux; U; Android 15) gzip")
	req.Header.Set("Origin", "https://www.youtube.com")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-YouTube-Client-Name", "3")
	req.Header.Set("X-YouTube-Client-Version", "20.10.38")
}

func fetchYouTubeTranscriptFromCaptionTracks(ctx context.Context, videoID string) youtubeTranscriptFetchResult {
	result := youtubeTranscriptFetchResult{
		Availability: "unavailable",
		Provider:     "youtube_caption_tracks",
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 6 * time.Second}
	watchURL := "https://www.youtube.com/watch?v=" + url.QueryEscape(videoID) + "&hl=en"
	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, watchURL, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ChoirBot/0.1; +https://choir.news)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		result.Error = resp.Status
		return result
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxImportedContentBytes))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	playerJSON := extractYouTubePlayerResponse(raw)
	if len(playerJSON) == 0 {
		result.Error = "caption tracks not found"
		return result
	}
	var player struct {
		Captions struct {
			PlayerCaptionsTracklistRenderer struct {
				CaptionTracks []youtubeCaptionTrack `json:"captionTracks"`
			} `json:"playerCaptionsTracklistRenderer"`
		} `json:"captions"`
	}
	if err := json.Unmarshal(playerJSON, &player); err != nil {
		result.Error = err.Error()
		return result
	}
	track, ok := chooseYouTubeCaptionTrack(player.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks)
	if !ok {
		result.Error = "caption tracks unavailable"
		return result
	}
	if strings.TrimSpace(track.BaseURL) == "" {
		result.Error = "caption track missing base url"
		return result
	}
	captionURL := youtubeJSON3CaptionURL(track.BaseURL)
	captionReq, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, captionURL, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	captionReq.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ChoirBot/0.1; +https://choir.news)")
	captionResp, err := client.Do(captionReq)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer func() { _ = captionResp.Body.Close() }()
	if captionResp.StatusCode < 200 || captionResp.StatusCode >= 400 {
		result.Error = captionResp.Status
		return result
	}
	captionRaw, err := io.ReadAll(io.LimitReader(captionResp.Body, maxImportedContentBytes))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	segments, text := parseYouTubeJSON3Transcript(captionRaw)
	if strings.TrimSpace(text) == "" {
		result.Error = "caption track had no text"
		return result
	}
	result.Availability = "available"
	result.Language = track.LanguageCode
	result.Kind = firstNonEmpty(track.Kind, "caption")
	result.Segments = segments
	result.Text = text
	result.Error = ""
	return result
}

func chooseYouTubeCaptionTrack(tracks []youtubeCaptionTrack) (youtubeCaptionTrack, bool) {
	if len(tracks) == 0 {
		return youtubeCaptionTrack{}, false
	}
	for _, candidate := range tracks {
		if strings.EqualFold(candidate.LanguageCode, "en") && strings.TrimSpace(candidate.Kind) == "" {
			return candidate, true
		}
	}
	for _, candidate := range tracks {
		if strings.EqualFold(candidate.LanguageCode, "en") {
			return candidate, true
		}
	}
	for _, candidate := range tracks {
		if strings.HasPrefix(strings.ToLower(candidate.LanguageCode), "en") {
			return candidate, true
		}
	}
	return tracks[0], true
}

func configuredYouTubeTranscriptProvider() (youtubeTranscriptProviderConfig, bool) {
	cfg := youtubeTranscriptProviderConfig{
		Name:       strings.TrimSpace(os.Getenv("CHOIR_YOUTUBE_TRANSCRIPT_PROVIDER")),
		BaseURL:    strings.TrimSpace(os.Getenv("CHOIR_YOUTUBE_TRANSCRIPT_API_URL")),
		APIKey:     strings.TrimSpace(os.Getenv("CHOIR_YOUTUBE_TRANSCRIPT_API_KEY")),
		AuthScheme: strings.TrimSpace(os.Getenv("CHOIR_YOUTUBE_TRANSCRIPT_AUTH_SCHEME")),
	}
	if cfg.Name == "" && cfg.BaseURL == "" && cfg.APIKey == "" {
		return youtubeTranscriptProviderConfig{}, false
	}
	cfg.Name = strings.ToLower(cfg.Name)
	if cfg.Name == "" {
		cfg.Name = "generic"
	}
	if cfg.AuthScheme == "" {
		cfg.AuthScheme = "bearer"
	}
	if cfg.BaseURL == "" {
		switch cfg.Name {
		case "gettranscript":
			cfg.BaseURL = "https://gettranscript.io/api/get-transcript"
		case "transcriptapi":
			cfg.BaseURL = "https://transcriptapi.com/api/v2/youtube/transcript"
		case "youtube-transcript-io":
			cfg.BaseURL = "https://www.youtube-transcript.io/api/transcripts"
		}
	}
	return cfg, cfg.BaseURL != ""
}

func fetchConfiguredYouTubeTranscript(ctx context.Context, videoID string, cfg youtubeTranscriptProviderConfig) youtubeTranscriptFetchResult {
	result := youtubeTranscriptFetchResult{
		Availability: "unavailable",
		Provider:     firstNonEmpty(cfg.Name, "configured"),
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	req, err := buildYouTubeTranscriptProviderRequest(fetchCtx, videoID, cfg)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	applyYouTubeTranscriptProviderAuth(req, cfg)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxImportedContentBytes))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		result.Error = fmt.Sprintf("%s returned status %s: %s", result.Provider, resp.Status, strings.TrimSpace(string(raw)))
		return result
	}
	segments, text, language, kind, err := parseYouTubeTranscriptProviderPayload(raw, videoID)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if strings.TrimSpace(text) == "" {
		result.Error = "configured transcript provider returned no text"
		return result
	}
	result.Availability = "available"
	result.Language = firstNonEmpty(language, "unknown")
	result.Kind = firstNonEmpty(kind, "provider")
	result.Text = text
	result.Segments = segments
	result.Error = ""
	return result
}

func buildYouTubeTranscriptProviderRequest(ctx context.Context, videoID string, cfg youtubeTranscriptProviderConfig) (*http.Request, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Name))
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("configured transcript provider missing base URL")
	}
	canonicalURL := "https://www.youtube.com/watch?v=" + url.QueryEscape(videoID)
	switch provider {
	case "youtube-transcript-io":
		body, _ := json.Marshal(map[string]any{"ids": []string{videoID}})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	case "generic-post":
		body, _ := json.Marshal(map[string]any{"video_id": videoID, "url": canonicalURL})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	default:
		parsed, err := url.Parse(baseURL)
		if err != nil {
			return nil, err
		}
		q := parsed.Query()
		switch provider {
		case "transcriptapi":
			q.Set("video_url", canonicalURL)
			q.Set("format", "json")
		default:
			q.Set("videoId", videoID)
		}
		parsed.RawQuery = q.Encode()
		return http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	}
}

func applyYouTubeTranscriptProviderAuth(req *http.Request, cfg youtubeTranscriptProviderConfig) {
	if req == nil || cfg.APIKey == "" {
		return
	}
	switch strings.ToLower(strings.TrimSpace(cfg.AuthScheme)) {
	case "none":
		return
	case "basic":
		req.SetBasicAuth(cfg.APIKey, "")
	case "x-api-key":
		req.Header.Set("X-API-Key", cfg.APIKey)
	default:
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
}

func parseYouTubeTranscriptProviderPayload(raw []byte, videoID string) ([]youtubeTranscriptSegment, string, string, string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, "", "", "", err
	}
	segments, text, language, kind, ok := extractYouTubeTranscriptProviderCandidate(value, videoID)
	if !ok {
		return nil, "", "", "", fmt.Errorf("configured transcript provider payload has no transcript text")
	}
	if strings.TrimSpace(text) == "" && len(segments) > 0 {
		lines := make([]string, 0, len(segments))
		for _, segment := range segments {
			if segment.Text != "" {
				lines = append(lines, segment.Text)
			}
		}
		text = strings.Join(lines, "\n")
	}
	if len(segments) == 0 && strings.TrimSpace(text) != "" {
		segments = []youtubeTranscriptSegment{{Start: 0, Text: strings.TrimSpace(text)}}
	}
	return segments, strings.TrimSpace(text), language, kind, nil
}

func extractYouTubeTranscriptProviderCandidate(value any, videoID string) ([]youtubeTranscriptSegment, string, string, string, bool) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			segments, text, language, kind, ok := extractYouTubeTranscriptProviderCandidate(item, videoID)
			if ok && (matchesTranscriptVideoID(item, videoID) || strings.TrimSpace(text) != "" || len(segments) > 0) {
				return segments, text, language, kind, true
			}
		}
	case map[string]any:
		if errText := providerStringField(typed, "error", "message"); errText != "" && providerStringField(typed, "text", "transcript") == "" {
			return nil, "", "", "", false
		}
		language := providerStringField(typed, "language", "language_code", "lang")
		kind := providerStringField(typed, "kind", "type")
		if rawSegments, ok := firstProviderField(typed, "segments", "transcript", "transcripts", "captions", "items"); ok {
			if segments, text := parseProviderTranscriptSegments(rawSegments); strings.TrimSpace(text) != "" || len(segments) > 0 {
				return segments, text, language, kind, true
			}
			if segments, text, childLanguage, childKind, ok := extractYouTubeTranscriptProviderCandidate(rawSegments, videoID); ok {
				return segments, text, firstNonEmpty(language, childLanguage), firstNonEmpty(kind, childKind), true
			}
		}
		if text := providerStringField(typed, "text", "transcript", "content", "body"); strings.TrimSpace(text) != "" {
			return nil, strings.TrimSpace(text), language, kind, true
		}
		for _, key := range []string{"data", "result", "results"} {
			if child, ok := typed[key]; ok {
				segments, text, childLanguage, childKind, childOK := extractYouTubeTranscriptProviderCandidate(child, videoID)
				if childOK {
					return segments, text, firstNonEmpty(language, childLanguage), firstNonEmpty(kind, childKind), true
				}
			}
		}
	}
	return nil, "", "", "", false
}

func parseProviderTranscriptSegments(value any) ([]youtubeTranscriptSegment, string) {
	items, ok := value.([]any)
	if !ok {
		return nil, ""
	}
	segments := make([]youtubeTranscriptSegment, 0, len(items))
	lines := []string{}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		text := providerStringField(m, "text", "utf8", "content", "caption")
		if text == "" {
			continue
		}
		segment := youtubeTranscriptSegment{
			Start:    providerFloatField(m, "start", "start_seconds", "offset", "offset_seconds"),
			Duration: providerFloatField(m, "duration", "duration_seconds", "dur"),
			Text:     collapseWhitespace(text),
		}
		segments = append(segments, segment)
		lines = append(lines, segment.Text)
	}
	return segments, strings.TrimSpace(strings.Join(lines, "\n"))
}

func firstProviderField(m map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			return value, true
		}
	}
	return nil, false
}

func providerStringField(m map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := m[key].(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		case json.Number:
			return value.String()
		}
	}
	return ""
}

func providerFloatField(m map[string]any, keys ...string) float64 {
	for _, key := range keys {
		switch value := m[key].(type) {
		case float64:
			return value
		case int:
			return float64(value)
		case json.Number:
			if parsed, err := strconv.ParseFloat(value.String(), 64); err == nil {
				return parsed
			}
		case string:
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func matchesTranscriptVideoID(value any, videoID string) bool {
	m, ok := value.(map[string]any)
	if !ok || videoID == "" {
		return false
	}
	for _, key := range []string{"video_id", "videoId", "id"} {
		if providerStringField(m, key) == videoID {
			return true
		}
	}
	return false
}

func youtubeJSON3CaptionURL(raw string) string {
	captionURL := strings.TrimSpace(raw)
	if parsedCaptionURL, err := url.Parse(captionURL); err == nil {
		q := parsedCaptionURL.Query()
		q.Set("fmt", "json3")
		parsedCaptionURL.RawQuery = q.Encode()
		return parsedCaptionURL.String()
	}
	if !strings.Contains(captionURL, "fmt=") {
		sep := "&"
		if !strings.Contains(captionURL, "?") {
			sep = "?"
		}
		captionURL += sep + "fmt=json3"
	}
	return captionURL
}

func extractYouTubePlayerResponse(raw []byte) []byte {
	source := string(raw)
	for _, marker := range []string{"ytInitialPlayerResponse =", "ytInitialPlayerResponse="} {
		idx := strings.Index(source, marker)
		if idx < 0 {
			continue
		}
		start := strings.Index(source[idx+len(marker):], "{")
		if start < 0 {
			continue
		}
		start += idx + len(marker)
		if data := extractJSONObjectAt(source, start); len(data) > 0 {
			return []byte(data)
		}
	}
	return nil
}

func extractJSONObjectAt(source string, start int) string {
	if start < 0 || start >= len(source) || source[start] != '{' {
		return ""
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(source); i++ {
		ch := source[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return source[start : i+1]
			}
		}
	}
	return ""
}

func parseYouTubeJSON3Transcript(raw []byte) ([]youtubeTranscriptSegment, string) {
	var parsed struct {
		Events []struct {
			TStartMs    int `json:"tStartMs"`
			DDurationMs int `json:"dDurationMs"`
			Segs        []struct {
				UTF8 string `json:"utf8"`
			} `json:"segs"`
		} `json:"events"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, ""
	}
	segments := make([]youtubeTranscriptSegment, 0, len(parsed.Events))
	lines := []string{}
	for _, event := range parsed.Events {
		parts := []string{}
		for _, seg := range event.Segs {
			if text := collapseWhitespace(seg.UTF8); text != "" {
				parts = append(parts, text)
			}
		}
		text := strings.TrimSpace(strings.Join(parts, " "))
		if text == "" {
			continue
		}
		segments = append(segments, youtubeTranscriptSegment{
			Start:    float64(event.TStartMs) / 1000,
			Duration: float64(event.DDurationMs) / 1000,
			Text:     text,
		})
		lines = append(lines, text)
	}
	return segments, strings.TrimSpace(strings.Join(lines, "\n"))
}

func shouldRunSearXNGDiscovery(query string, selected fetchedURLContent, primaryErr error) bool {
	if strings.TrimSpace(query) == "" {
		return false
	}
	if primaryErr != nil {
		return true
	}
	return len(strings.TrimSpace(selected.Text)) < 400
}

func searxngBaseURL() string {
	for _, key := range []string{"SEARXNG_URL", "CHOIR_SEARXNG_URL"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func discoverSearXNGCandidates(ctx context.Context, client *http.Client, baseURL, query, originalURL string) ([]searxngCandidate, extractionRung) {
	rung := extractionRung{Name: "searxng_discovery", Status: "not_configured"}
	if strings.TrimSpace(baseURL) == "" {
		return nil, rung
	}
	searchURL, err := buildSearXNGSearchURL(baseURL, query, true)
	if err != nil {
		rung.Status = "error"
		rung.Error = err.Error()
		return nil, rung
	}
	rung.Endpoint = redactedURL(searchURL)
	started := time.Now()
	candidates, statusCode, err := fetchSearXNGJSON(ctx, client, searchURL, originalURL)
	rung.LatencyMs = time.Since(started).Milliseconds()
	rung.StatusCode = statusCode
	if err == nil && len(candidates) > 0 {
		rung.Status = "success"
		rung.Candidates = len(candidates)
		return candidates, rung
	}

	htmlURL, htmlErr := buildSearXNGSearchURL(baseURL, query, false)
	if htmlErr != nil {
		rung.Status = "error"
		rung.Error = nonEmpty(errString(err), htmlErr.Error())
		return nil, rung
	}
	started = time.Now()
	htmlCandidates, htmlStatusCode, htmlFetchErr := fetchSearXNGHTML(ctx, client, htmlURL, originalURL)
	rung.Endpoint = redactedURL(htmlURL)
	rung.LatencyMs += time.Since(started).Milliseconds()
	if htmlStatusCode != 0 {
		rung.StatusCode = htmlStatusCode
	}
	if htmlFetchErr != nil {
		rung.Status = "error"
		rung.Error = nonEmpty(errString(err), htmlFetchErr.Error())
		return nil, rung
	}
	if len(htmlCandidates) == 0 {
		rung.Status = "low_content"
		rung.Error = errString(err)
		return nil, rung
	}
	rung.Status = "success"
	rung.Candidates = len(htmlCandidates)
	return htmlCandidates, rung
}

func buildSearXNGSearchURL(baseURL, query string, jsonFormat bool) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("SEARXNG_URL must be an absolute URL")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/search"
	values := parsed.Query()
	values.Set("q", strings.TrimSpace(query))
	if jsonFormat {
		values.Set("format", "json")
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func fetchSearXNGJSON(ctx context.Context, client *http.Client, searchURL, originalURL string) ([]searxngCandidate, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, resp.StatusCode, fmt.Errorf("searxng json status %s", resp.Status)
	}
	var parsed struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
			Engine  string `json:"engine"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, resp.StatusCode, err
	}
	candidates := make([]searxngCandidate, 0, len(parsed.Results))
	for _, result := range parsed.Results {
		if candidateURL := usableCandidateURL(result.URL, originalURL); candidateURL != "" {
			candidates = append(candidates, searxngCandidate{Title: strings.TrimSpace(result.Title), URL: candidateURL, Snippet: strings.TrimSpace(result.Content), Engine: strings.TrimSpace(result.Engine)})
		}
		if len(candidates) >= 5 {
			break
		}
	}
	return candidates, resp.StatusCode, nil
}

func fetchSearXNGHTML(ctx context.Context, client *http.Client, searchURL, originalURL string) ([]searxngCandidate, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, resp.StatusCode, fmt.Errorf("searxng html status %s", resp.Status)
	}
	return extractSearXNGHTMLCandidates(string(data), originalURL), resp.StatusCode, nil
}

func extractSearXNGHTMLCandidates(source, originalURL string) []searxngCandidate {
	matches := regexp.MustCompile(`(?is)\bhref=["']([^"']+)["']`).FindAllStringSubmatch(source, -1)
	candidates := []searxngCandidate{}
	seen := map[string]struct{}{}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		candidateURL := htmlEntityDecode(strings.TrimSpace(match[1]))
		if strings.HasPrefix(candidateURL, "/url?") {
			if parsed, err := url.Parse(candidateURL); err == nil {
				candidateURL = parsed.Query().Get("url")
			}
		}
		candidateURL = usableCandidateURL(candidateURL, originalURL)
		if candidateURL == "" {
			continue
		}
		if _, ok := seen[candidateURL]; ok {
			continue
		}
		seen[candidateURL] = struct{}{}
		candidates = append(candidates, searxngCandidate{URL: candidateURL})
		if len(candidates) >= 5 {
			break
		}
	}
	return candidates
}

func usableCandidateURL(candidateURL, originalURL string) string {
	normalized, err := normalizeHTTPURL(candidateURL)
	if err != nil {
		return ""
	}
	if sameNormalizedURL(normalized, originalURL) {
		return ""
	}
	return normalized
}

func sameNormalizedURL(a, b string) bool {
	na, errA := normalizeHTTPURL(a)
	nb, errB := normalizeHTTPURL(b)
	if errA != nil || errB != nil {
		return strings.TrimSpace(a) == strings.TrimSpace(b)
	}
	return na == nb
}

func betterFetchedContent(candidate, current fetchedURLContent) bool {
	if strings.TrimSpace(candidate.Text) == "" {
		return false
	}
	if strings.TrimSpace(current.Text) == "" {
		return true
	}
	return len(candidate.Text) > len(current.Text)*2 && len(candidate.Text) >= 400
}

func retrievalStrategy(rungs []extractionRung) string {
	for _, rung := range rungs {
		if rung.Name == "searxng_discovery" && rung.Status == "success" {
			return "direct_http_readability_with_searxng_discovery"
		}
	}
	return "direct_http_then_readability_lite"
}

func redactedURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	values := parsed.Query()
	if values.Has("q") {
		values.Set("q", "<query>")
	}
	parsed.RawQuery = values.Encode()
	return parsed.String()
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func normalizeHTTPURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("url is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("url must be an absolute http or https URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("url scheme must be http or https")
	}
	parsed.Fragment = ""
	return parsed.String(), nil
}

func classifyPromptBarContentIntent(text string) (appHint, sourceURL, mediaType string, ok bool) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) != 1 {
		return "", "", "", false
	}
	normalizedURL, err := normalizeHTTPURL(fields[0])
	if err != nil {
		return "", "", "", false
	}
	mediaType = detectMediaType(normalizedURL, "", "")
	appHint = appHintForMedia(mediaType, normalizedURL, "")
	if appHint == "files" {
		appHint = "browser"
	}
	return appHint, normalizedURL, mediaType, true
}

func normalizeContentSourceType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "upload", "file", "url", "extracted_url", "derived_transcript", "text":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeMediaType(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	if parsed, _, err := mime.ParseMediaType(value); err == nil {
		return strings.ToLower(parsed)
	}
	return value
}

func detectMediaType(sourceURL, filePath, contentType string) string {
	if isYouTubeURL(sourceURL) {
		return "video/youtube"
	}
	if normalized := normalizeMediaType(contentType); normalized != "" && normalized != "application/octet-stream" {
		if normalized == "application/xml" && strings.Contains(strings.ToLower(sourceURL), "rss") {
			return "application/rss+xml"
		}
		return normalized
	}
	ext := strings.ToLower(path.Ext(nonEmpty(filePath, sourceURL)))
	switch ext {
	case ".md", ".markdown":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".pdf":
		return "application/pdf"
	case ".epub":
		return "application/epub+zip"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".m4a":
		return "audio/mp4"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".rss", ".xml":
		return "application/rss+xml"
	}
	return "application/octet-stream"
}

func appHintForMedia(mediaType, sourceURL, filePath string) string {
	mediaType = normalizeMediaType(mediaType)
	switch {
	case isYouTubeURL(sourceURL), strings.HasPrefix(mediaType, "video/"):
		return "video"
	case strings.HasPrefix(mediaType, "image/"):
		return "image"
	case strings.HasPrefix(mediaType, "audio/"):
		return "audio"
	case mediaType == "application/pdf":
		return "pdf"
	case mediaType == "application/epub+zip":
		return "epub"
	case mediaType == "application/rss+xml" || strings.Contains(strings.ToLower(sourceURL+filePath), "podcast"):
		return "podcast"
	case mediaType == "text/markdown" || mediaType == "text/plain":
		return "vtext"
	case mediaType == "text/html" || mediaType == "application/xhtml+xml":
		return "browser"
	default:
		return "files"
	}
}

func normalizeAppHint(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "vtext", "browser", "files", "pdf", "epub", "image", "video", "audio", "podcast":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "files"
	}
}

func isAllowedProductApp(value string) bool {
	return normalizeAppHint(value) == strings.ToLower(strings.TrimSpace(value))
}

func isHTMLMedia(mediaType string) bool {
	mediaType = normalizeMediaType(mediaType)
	return mediaType == "text/html" || mediaType == "application/xhtml+xml"
}

func isTextMedia(mediaType string) bool {
	mediaType = normalizeMediaType(mediaType)
	return strings.HasPrefix(mediaType, "text/") || mediaType == "application/json" || mediaType == "application/xml" || mediaType == "application/rss+xml"
}

func isYouTubeURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "youtube.com" || host == "www.youtube.com" || host == "youtu.be" || host == "m.youtube.com"
}

func youtubeVideoID(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "youtu.be":
		return sanitizeYouTubeVideoID(strings.Trim(strings.TrimSpace(parsed.Path), "/"))
	case "youtube.com", "www.youtube.com", "m.youtube.com":
		if id := sanitizeYouTubeVideoID(parsed.Query().Get("v")); id != "" {
			return id
		}
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) >= 2 {
			switch parts[0] {
			case "embed", "shorts", "live":
				return sanitizeYouTubeVideoID(parts[1])
			}
		}
	}
	return ""
}

func sanitizeYouTubeVideoID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.IndexAny(value, "?&#/"); idx >= 0 {
		value = value[:idx]
	}
	if regexp.MustCompile(`^[A-Za-z0-9_-]{6,}$`).MatchString(value) {
		return value
	}
	return ""
}

func statusForHTTP(code int) string {
	if code >= 200 && code < 400 {
		return "success"
	}
	return "error"
}

func statusForText(text string) string {
	if strings.TrimSpace(text) == "" {
		return "low_content"
	}
	return "success"
}

func extractReadableHTML(data []byte) (string, string) {
	source := string(data)
	title := ""
	if matches := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`).FindStringSubmatch(source); len(matches) > 1 {
		title = htmlEntityDecode(stripHTMLTags(matches[1]))
	}
	cleaned := source
	for _, tag := range []string{"script", "style", "noscript", "svg"} {
		cleaned = regexp.MustCompile(`(?is)<`+tag+`[^>]*>.*?</`+tag+`>`).ReplaceAllString(cleaned, " ")
	}
	cleaned = regexp.MustCompile(`(?is)<br\s*/?>|</p>|</div>|</section>|</article>|</h[1-6]>|</li>`).ReplaceAllString(cleaned, "\n")
	text := htmlEntityDecode(stripHTMLTags(cleaned))
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if normalized := collapseWhitespace(line); normalized != "" {
			out = append(out, normalized)
		}
	}
	return strings.TrimSpace(title), strings.TrimSpace(strings.Join(out, "\n"))
}

func extractRSSFeedTitle(data []byte) string {
	source := string(data)
	if channel := regexp.MustCompile(`(?is)<channel\b[^>]*>(.*?)</channel>`).FindStringSubmatch(source); len(channel) > 1 {
		source = channel[1]
	}
	if matches := regexp.MustCompile(`(?is)<title\b[^>]*>(.*?)</title>`).FindStringSubmatch(source); len(matches) > 1 {
		return collapseWhitespace(htmlEntityDecode(stripHTMLTags(stripCDATA(matches[1]))))
	}
	return ""
}

func collapseWhitespace(s string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, s))
}

func stripHTMLTags(source string) string {
	return regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(source, " ")
}

func htmlEntityDecode(source string) string {
	replacer := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&nbsp;", " ",
	)
	return replacer.Replace(source)
}

func stripCDATA(source string) string {
	return regexp.MustCompile(`(?is)<!\[CDATA\[(.*?)\]\]>`).ReplaceAllString(source, "$1")
}

func ensureJSONObject(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "" {
		return json.RawMessage(`{}`)
	}
	if json.Valid(raw) {
		return raw
	}
	return json.RawMessage(`{}`)
}

func contentHash(text string) string {
	if text == "" {
		return ""
	}
	return contentHashBytes([]byte(text))
}

func contentHashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func fallbackContentTitle(item types.ContentItem) string {
	switch {
	case item.FilePath != "":
		return path.Base(item.FilePath)
	case item.CanonicalURL != "":
		return hostPathTitle(item.CanonicalURL)
	case item.SourceURL != "":
		return hostPathTitle(item.SourceURL)
	default:
		return "Untitled content"
	}
}

func hostPathTitle(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	base := path.Base(parsed.Path)
	if base == "." || base == "/" || base == "" {
		return parsed.Hostname()
	}
	return base
}
