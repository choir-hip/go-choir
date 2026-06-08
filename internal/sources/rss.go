package sources

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

type RSSPoller struct {
	Client    *http.Client
	UserAgent string
}

func NewRSSPoller(userAgent string) *RSSPoller {
	return &RSSPoller{
		Client:    sourcefetch.Client(30 * time.Second),
		UserAgent: userAgent,
	}
}

func (p *RSSPoller) Poll(ctx context.Context, source *Source) (PollResult, error) {
	started := time.Now().UTC()
	fetch := NewFetchRecord(*source, source.URL, started)
	if err := sourcefetch.ValidateURL(source.URL); err != nil {
		fetch = FinishFetch(fetch, 0, nil, err)
		return PollResult{Fetch: fetch}, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", source.URL, nil)
	if err != nil {
		fetch = FinishFetch(fetch, 0, nil, err)
		return PollResult{Fetch: fetch}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", p.UserAgent)
	if source.LastETag != "" {
		req.Header.Set("If-None-Match", source.LastETag)
	}
	if source.LastModified != "" {
		req.Header.Set("If-Modified-Since", source.LastModified)
	}

	resp, err := p.Client.Do(req)
	if err != nil {
		fetch = FinishFetch(fetch, 0, nil, err)
		return PollResult{Fetch: fetch}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		fetch = FinishFetch(fetch, resp.StatusCode, nil, nil)
		return PollResult{Fetch: fetch}, nil
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if readErr != nil {
		fetch = FinishFetch(fetch, resp.StatusCode, nil, readErr)
		return PollResult{Fetch: fetch}, fmt.Errorf("read response: %w", readErr)
	}
	fetch.ResponseETag = resp.Header.Get("ETag")
	fetch.ResponseModified = resp.Header.Get("Last-Modified")

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		fetch = FinishFetch(fetch, resp.StatusCode, body, err)
		return PollResult{Fetch: fetch}, err
	}

	// Update Source metadata for next poll
	source.LastETag = resp.Header.Get("ETag")
	source.LastModified = resp.Header.Get("Last-Modified")
	source.LastPolled = time.Now()

	feed, err := parseRSSLikeFeed(body)
	if err != nil {
		fetch = FinishFetch(fetch, resp.StatusCode, body, err)
		return PollResult{Fetch: fetch}, fmt.Errorf("failed to parse feed: %w", err)
	}

	maxItems := source.EffectiveMaxItemsPerPoll(100)
	items := make([]Item, 0, minInt(len(feed.Items), maxItems))
	for _, feedItem := range feed.Items {
		if len(items) >= maxItems {
			break
		}
		published := time.Now()
		if parsed, ok := parseFeedTime(feedItem.Published); ok {
			published = parsed
		}

		bodyText := cleanFeedDescriptionText(feedItem.Description)
		item := Item{
			ID:            StableItemID(*source, feedItem.GUID, feedItem.Link, feedItem.Title, feedItem.Description),
			SourceID:      source.ID,
			SourceType:    source.Type,
			FetchID:       fetch.FetchID,
			OriginalID:    feedItem.GUID,
			Title:         feedItem.Title,
			Body:          bodyText,
			URL:           feedItem.Link,
			CanonicalURL:  NormalizeURL(feedItem.Link),
			Published:     published.UTC(),
			FetchedAt:     time.Now().UTC(),
			Verticals:     source.Verticals,
			Language:      firstString(source.Languages),
			Region:        firstString(source.Regions),
			EvidenceLevel: "source_feed",
		}
		item.ContentHash = ContentHash(item.Title, item.Body, item.CanonicalURL)
		items = append(items, item)
	}
	fetch = FinishFetch(fetch, resp.StatusCode, body, nil)
	fetch.ItemCount = len(items)

	return PollResult{Fetch: fetch, Items: items}, nil
}

func cleanFeedDescriptionText(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "<") && !strings.Contains(raw, "&") {
		return cleanFeedDescriptionWhitespace(raw)
	}
	if !strings.Contains(raw, "<") {
		return cleanFeedDescriptionWhitespace(stdhtml.UnescapeString(raw))
	}
	var parts []string
	tokenizer := xhtml.NewTokenizer(strings.NewReader(raw))
	for {
		switch tokenizer.Next() {
		case xhtml.ErrorToken:
			if len(parts) == 0 {
				return cleanFeedDescriptionWhitespace(stdhtml.UnescapeString(raw))
			}
			return cleanFeedDescriptionWhitespace(strings.Join(parts, " "))
		case xhtml.TextToken:
			if text := strings.TrimSpace(stdhtml.UnescapeString(tokenizer.Token().Data)); text != "" {
				parts = append(parts, text)
			}
		}
	}
}

func cleanFeedDescriptionWhitespace(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	replacer := strings.NewReplacer(
		" .", ".",
		" ,", ",",
		" ;", ";",
		" :", ":",
		" !", "!",
		" ?", "?",
		"( ", "(",
		" )", ")",
	)
	return replacer.Replace(text)
}

type feedEnvelope struct {
	XMLName xml.Name
	Channel *struct {
		Items []feedItem `xml:"item"`
	} `xml:"channel"`
	Entries []atomEntry `xml:"entry"`
}

type feedItem struct {
	GUID        string `xml:"guid"`
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Published   string `xml:"pubDate"`
}

type atomEntry struct {
	ID        string `xml:"id"`
	Title     string `xml:"title"`
	Summary   string `xml:"summary"`
	Content   string `xml:"content"`
	Published string `xml:"published"`
	Updated   string `xml:"updated"`
	Links     []struct {
		Href string `xml:"href,attr"`
		Rel  string `xml:"rel,attr"`
	} `xml:"link"`
}

type parsedFeed struct {
	Items []feedItem
}

func parseRSSLikeFeed(data []byte) (parsedFeed, error) {
	var env feedEnvelope
	decoder := xml.NewDecoder(bytes.NewReader(stripInvalidXMLControlBytes(data)))
	decoder.CharsetReader = charset.NewReaderLabel
	decoder.Strict = false
	if err := decoder.Decode(&env); err != nil {
		return parsedFeed{}, err
	}
	if env.Channel != nil {
		return parsedFeed{Items: env.Channel.Items}, nil
	}
	items := make([]feedItem, 0, len(env.Entries))
	for _, entry := range env.Entries {
		link := ""
		for _, candidate := range entry.Links {
			if candidate.Rel == "" || candidate.Rel == "alternate" {
				link = candidate.Href
				break
			}
		}
		items = append(items, feedItem{
			GUID:        entry.ID,
			Title:       entry.Title,
			Link:        link,
			Description: firstString([]string{entry.Summary, entry.Content}),
			Published:   firstString([]string{entry.Published, entry.Updated}),
		})
	}
	return parsedFeed{Items: items}, nil
}

func stripInvalidXMLControlBytes(data []byte) []byte {
	for _, b := range data {
		if b < 0x20 && b != '\t' && b != '\n' && b != '\r' {
			out := make([]byte, 0, len(data))
			for _, candidate := range data {
				if candidate < 0x20 && candidate != '\t' && candidate != '\n' && candidate != '\r' {
					continue
				}
				out = append(out, candidate)
			}
			return out
		}
	}
	return data
}

func parseFeedTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC3339, time.RFC822Z, time.RFC822} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func firstString(values []string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
