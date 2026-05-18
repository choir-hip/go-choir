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
