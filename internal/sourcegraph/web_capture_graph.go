package sourcegraph

import (
	"net/url"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

// webCaptureGraphRequestsFromItem shapes a polled or read item into the
// capture request WriteCaptures completes (identity, blobs and blocks are
// set there). An item with no id, source, body or http(s) URL is skipped.
func webCaptureGraphRequestsFromItem(item sources.Item, ownerID, computerID string, now time.Time) (objectgraph.CreateWebCaptureRequest, bool) {
	item = sources.NormalizeItemBodyClassification(item)
	itemID := strings.TrimSpace(item.ID)
	body := strings.TrimSpace(item.Body)
	if itemID == "" || strings.TrimSpace(item.SourceID) == "" || body == "" {
		return objectgraph.CreateWebCaptureRequest{}, false
	}
	itemURL := firstNonEmptyString(item.URL, item.CanonicalURL)
	canonicalURL := firstNonEmptyString(item.CanonicalURL, item.URL)
	if !isHTTPURL(itemURL) || !isHTTPURL(canonicalURL) {
		return objectgraph.CreateWebCaptureRequest{}, false
	}
	fetchedAt := item.FetchedAt
	if fetchedAt.IsZero() {
		fetchedAt = item.Published
	}
	if fetchedAt.IsZero() {
		fetchedAt = now
	}
	return objectgraph.CreateWebCaptureRequest{
		OwnerID:       ownerID,
		ComputerID:    computerID,
		URL:           itemURL,
		CanonicalURL:  canonicalURL,
		Title:         strings.TrimSpace(item.Title),
		FetchedAt:     fetchedAt,
		ExtractedText: []byte(body),
		Now:           now,
	}, true
}

func isHTTPURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
