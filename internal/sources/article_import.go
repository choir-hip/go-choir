package sources

import (
	"context"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/htmlextract"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
)

const (
	minReaderSnapshotRunes = 200
	maxReaderSnapshotRunes = 32000
)

// SourceAllowsReaderImport reports whether store_body_policy permits article fetch.
func SourceAllowsReaderImport(policy string) bool {
	switch strings.TrimSpace(policy) {
	case "bounded_text", "bounded_release_text", "bounded_abstract":
		return true
	default:
		return false
	}
}

func enrichItemWithReaderSnapshot(ctx context.Context, client *http.Client, userAgent string, source *Source, item *Item) {
	if client == nil || source == nil || item == nil || !SourceAllowsReaderImport(source.StoreBodyPolicy) {
		return
	}
	if item.BodyKind != BodyKindFeedSummary && item.BodyKind != BodyKindEmpty {
		return
	}
	link := strings.TrimSpace(item.URL)
	if link == "" || !strings.HasPrefix(strings.ToLower(link), "http") {
		return
	}
	if err := sourcefetch.ValidateURL(link); err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return
	}
	_, text := htmlextract.ExtractReadableHTML(body)
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) < minReaderSnapshotRunes {
		return
	}
	if runes := utf8.RuneCountInString(text); runes > maxReaderSnapshotRunes {
		text = truncateRunes(text, maxReaderSnapshotRunes)
	}
	item.Body = text
	item.BodyKind = BodyKindReaderSnapshot
	item.ReaderSnapshot = true
	item.BodyLength = utf8.RuneCountInString(text)
	item.StoreBodyPolicy = source.StoreBodyPolicy
	item.ContentHash = ContentHash(item.Title, item.Body, item.CanonicalURL)
}

func truncateRunes(text string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(text) <= max {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	count := 0
	for _, r := range text {
		if count >= max {
			break
		}
		b.WriteRune(r)
		count++
	}
	return b.String()
}
