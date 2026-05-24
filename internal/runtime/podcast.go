package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

type podcastSearchResult struct {
	Provider     string `json:"provider"`
	Title        string `json:"title"`
	Author       string `json:"author,omitempty"`
	FeedURL      string `json:"feed_url"`
	ArtworkURL   string `json:"artwork_url,omitempty"`
	EpisodeCount int    `json:"episode_count,omitempty"`
	Genre        string `json:"genre,omitempty"`
	Source       string `json:"source"`
}

type podcastSearchResponse struct {
	Query          string                `json:"query"`
	Provider       string                `json:"provider"`
	ProviderStatus string                `json:"provider_status"`
	Results        []podcastSearchResult `json:"results"`
	Warnings       []string              `json:"warnings,omitempty"`
}

type podcastSubscriptionRequest struct {
	FeedURL    string `json:"feed_url"`
	Title      string `json:"title,omitempty"`
	Author     string `json:"author,omitempty"`
	ArtworkURL string `json:"artwork_url,omitempty"`
	Force      bool   `json:"force,omitempty"`
}

type podcastSubscriptionResponse struct {
	Subscription types.PodcastSubscription `json:"subscription"`
}

type podcastSubscriptionsResponse struct {
	Subscriptions []types.PodcastSubscription `json:"subscriptions"`
	Refreshed     int                         `json:"refreshed,omitempty"`
	Errors        []string                    `json:"errors,omitempty"`
}

// Podcast APIs run from the sandbox runtime package so subscription fixes can
// deploy without rebuilding the base guest image.
func (h *APIHandler) HandlePodcastSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "q is required"})
		return
	}
	limit := apiLimit(r, 10)
	if limit > 25 {
		limit = 25
	}
	resp := podcastSearchResponse{Query: query, Provider: podcastSearchProviderName(), ProviderStatus: "pending"}
	client := &http.Client{Timeout: 12 * time.Second}
	results, err := searchPodcastProvider(r.Context(), client, query, limit)
	if err != nil {
		resp.ProviderStatus = "error"
		resp.Warnings = append(resp.Warnings, err.Error())
	} else {
		resp.ProviderStatus = "success"
		resp.Results = results
	}
	if len(resp.Results) == 0 {
		fallback, fallbackErr := h.podcastLibrarySearchFallback(r.Context(), ownerID, query, limit)
		if fallbackErr != nil {
			resp.Warnings = append(resp.Warnings, fallbackErr.Error())
		}
		if len(fallback) > 0 {
			resp.ProviderStatus = firstNonEmptyPromotion(resp.ProviderStatus, "fallback")
			resp.Results = fallback
		}
	}
	writeAPIJSON(w, http.StatusOK, resp)
}

func (h *APIHandler) HandlePodcastSubscriptions(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		limit := apiLimit(r, 100)
		if limit > 200 {
			limit = 200
		}
		subs, err := h.listPodcastSubscriptionsWithContent(r.Context(), ownerID, limit)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusOK, podcastSubscriptionsResponse{Subscriptions: subs})
	case http.MethodPost:
		var req podcastSubscriptionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid JSON body"})
			return
		}
		sub, err := h.subscribePodcastFeed(r.Context(), ownerID, req)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusCreated, podcastSubscriptionResponse{Subscription: sub})
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

func (h *APIHandler) HandlePodcastSubscriptionsRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	var req podcastSubscriptionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	limit := apiLimit(r, 50)
	if limit > 100 {
		limit = 100
	}
	subs, err := h.listPodcastSubscriptionsWithContent(r.Context(), ownerID, limit)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	refreshed := 0
	refreshErrors := []string{}
	for _, sub := range subs {
		if !req.Force && !podcastSubscriptionRefreshDue(sub, time.Now().UTC()) {
			continue
		}
		next, err := h.refreshPodcastSubscription(r.Context(), sub)
		if err != nil {
			refreshErrors = append(refreshErrors, fmt.Sprintf("%s: %v", sub.FeedURL, err))
			continue
		}
		refreshed++
		_ = next
	}
	out, err := h.listPodcastSubscriptionsWithContent(r.Context(), ownerID, limit)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, podcastSubscriptionsResponse{Subscriptions: out, Refreshed: refreshed, Errors: refreshErrors})
}

func (h *APIHandler) subscribePodcastFeed(ctx context.Context, ownerID string, req podcastSubscriptionRequest) (types.PodcastSubscription, error) {
	feedURL := strings.TrimSpace(req.FeedURL)
	if feedURL == "" {
		return types.PodcastSubscription{}, fmt.Errorf("feed_url is required")
	}
	normalizedURL, err := normalizeHTTPURL(feedURL)
	if err != nil {
		return types.PodcastSubscription{}, err
	}
	now := time.Now().UTC()
	sub := types.PodcastSubscription{
		OwnerID:    ownerID,
		FeedURL:    normalizedURL,
		Title:      strings.TrimSpace(req.Title),
		Author:     strings.TrimSpace(req.Author),
		ArtworkURL: strings.TrimSpace(req.ArtworkURL),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	return h.refreshPodcastSubscription(ctx, sub)
}

func (h *APIHandler) refreshPodcastSubscription(ctx context.Context, sub types.PodcastSubscription) (types.PodcastSubscription, error) {
	query := firstNonEmptyPromotion(sub.Title, sub.FeedURL)
	item, err := h.rt.ImportURLContent(ctx, sub.OwnerID, sub.FeedURL, query)
	if err != nil {
		return sub, err
	}
	now := time.Now().UTC()
	sub.ContentID = item.ContentID
	if strings.TrimSpace(sub.Title) == "" {
		sub.Title = firstNonEmptyPromotion(item.Title, item.SourceURL)
	}
	sub.LastFetchedAt = now
	sub.UpdatedAt = now
	saved, err := h.rt.Store().UpsertPodcastSubscription(ctx, sub)
	if err != nil {
		return sub, err
	}
	saved.ContentItem = &item
	_, _ = h.rt.emitProductEvent(ctx, sub.OwnerID, types.PrimaryDesktopID, types.EventContentItemCreated, map[string]any{
		"content_id":       item.ContentID,
		"subscription_id":  saved.SubscriptionID,
		"feed_url":         saved.FeedURL,
		"podcast_library":  true,
		"last_fetched_at":  saved.LastFetchedAt.UTC().Format(time.RFC3339Nano),
		"subscription_ref": saved.SubscriptionID,
	})
	return saved, nil
}

func podcastSubscriptionRefreshDue(sub types.PodcastSubscription, now time.Time) bool {
	if sub.LastFetchedAt.IsZero() {
		return true
	}
	// Opening Podcast should refresh stale feeds without importing a new RSS
	// content item on every app launch.
	return now.Sub(sub.LastFetchedAt) >= 30*time.Minute
}

func (h *APIHandler) listPodcastSubscriptionsWithContent(ctx context.Context, ownerID string, limit int) ([]types.PodcastSubscription, error) {
	subs, err := h.rt.Store().ListPodcastSubscriptions(ctx, ownerID, limit)
	if err != nil {
		return nil, err
	}
	if len(subs) == 0 {
		if err := h.seedPodcastSubscriptionsFromContentItems(ctx, ownerID); err != nil {
			return nil, err
		}
		subs, err = h.rt.Store().ListPodcastSubscriptions(ctx, ownerID, limit)
		if err != nil {
			return nil, err
		}
	}
	for i := range subs {
		if strings.TrimSpace(subs[i].ContentID) == "" {
			continue
		}
		item, err := h.rt.Store().GetContentItem(ctx, ownerID, subs[i].ContentID)
		if err != nil {
			continue
		}
		subs[i].ContentItem = &item
	}
	return subs, nil
}

func (h *APIHandler) seedPodcastSubscriptionsFromContentItems(ctx context.Context, ownerID string) error {
	items, err := h.rt.Store().ListContentItems(ctx, ownerID, 200)
	if err != nil {
		return err
	}
	for _, item := range items {
		if !contentItemLooksPodcast(item) {
			continue
		}
		feedURL := firstNonEmptyPromotion(item.SourceURL, item.CanonicalURL)
		if feedURL == "" {
			continue
		}
		_, err := h.rt.Store().UpsertPodcastSubscription(ctx, types.PodcastSubscription{
			OwnerID:   ownerID,
			FeedURL:   feedURL,
			ContentID: item.ContentID,
			Title:     firstNonEmptyPromotion(item.Title, item.SourceURL, item.CanonicalURL),
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func podcastSearchProviderName() string {
	if strings.TrimSpace(os.Getenv("CHOIR_PODCAST_SEARCH_URL")) != "" {
		return "configured"
	}
	return "apple-itunes"
}

func podcastSearchEndpoint(query string, limit int) (string, error) {
	raw := strings.TrimSpace(os.Getenv("CHOIR_PODCAST_SEARCH_URL"))
	if raw == "" {
		raw = "https://itunes.apple.com/search"
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("podcast search provider URL is invalid")
	}
	values := parsed.Query()
	values.Set("term", query)
	values.Set("media", "podcast")
	values.Set("entity", "podcast")
	values.Set("limit", strconv.Itoa(limit))
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func searchPodcastProvider(ctx context.Context, client *http.Client, query string, limit int) ([]podcastSearchResult, error) {
	endpoint, err := podcastSearchEndpoint(query, limit)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ChoirPodcastSearch/0.1 (+https://choir-ip.com)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("podcast provider request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read podcast provider response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("podcast provider status %s", resp.Status)
	}
	var parsed struct {
		Results []struct {
			CollectionName string `json:"collectionName"`
			ArtistName     string `json:"artistName"`
			FeedURL        string `json:"feedUrl"`
			ArtworkURL100  string `json:"artworkUrl100"`
			TrackCount     int    `json:"trackCount"`
			PrimaryGenre   string `json:"primaryGenreName"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("decode podcast provider response: %w", err)
	}
	out := make([]podcastSearchResult, 0, len(parsed.Results))
	for _, result := range parsed.Results {
		feedURL := strings.TrimSpace(result.FeedURL)
		if feedURL == "" {
			continue
		}
		title := strings.TrimSpace(result.CollectionName)
		if title == "" {
			title = feedURL
		}
		out = append(out, podcastSearchResult{
			Provider:     podcastSearchProviderName(),
			Title:        title,
			Author:       strings.TrimSpace(result.ArtistName),
			FeedURL:      feedURL,
			ArtworkURL:   strings.TrimSpace(result.ArtworkURL100),
			EpisodeCount: result.TrackCount,
			Genre:        strings.TrimSpace(result.PrimaryGenre),
			Source:       "provider",
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (h *APIHandler) podcastLibrarySearchFallback(ctx context.Context, ownerID, query string, limit int) ([]podcastSearchResult, error) {
	items, err := h.rt.Store().ListContentItems(ctx, ownerID, 200)
	if err != nil {
		return nil, fmt.Errorf("podcast library fallback failed: %w", err)
	}
	needle := strings.ToLower(query)
	out := []podcastSearchResult{}
	for _, item := range items {
		if !contentItemLooksPodcast(item) {
			continue
		}
		haystack := strings.ToLower(item.Title + " " + item.SourceURL + " " + item.CanonicalURL)
		if !strings.Contains(haystack, needle) {
			continue
		}
		out = append(out, podcastSearchResult{
			Provider: "library",
			Title:    firstNonEmptyPromotion(item.Title, item.SourceURL, item.ContentID),
			FeedURL:  firstNonEmptyPromotion(item.SourceURL, item.CanonicalURL),
			Source:   "library",
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func contentItemLooksPodcast(item types.ContentItem) bool {
	return item.AppHint == "podcast" ||
		item.MediaType == "application/rss+xml" ||
		strings.Contains(strings.ToLower(item.SourceURL+" "+item.FilePath), "podcast") ||
		strings.Contains(strings.ToLower(item.SourceURL+" "+item.FilePath), "rss")
}
