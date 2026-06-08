package sources

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
)

type TelegramScraper struct {
	Client    *http.Client
	UserAgent string
}

func NewTelegramScraper(userAgent string) *TelegramScraper {
	return &TelegramScraper{
		Client:    sourcefetch.Client(30 * time.Second),
		UserAgent: userAgent,
	}
}

func (s *TelegramScraper) Poll(ctx context.Context, source *Source) (PollResult, error) {
	// Telegram web preview URL: https://t.me/s/channelname
	url := source.URL
	if !strings.Contains(url, "/s/") {
		url = strings.Replace(url, "t.me/", "t.me/s/", 1)
	}
	started := time.Now().UTC()
	fetch := NewFetchRecord(*source, url, started)
	if err := sourcefetch.ValidateURL(url); err != nil {
		fetch = FinishFetch(fetch, 0, nil, err)
		return PollResult{Fetch: fetch}, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		fetch = FinishFetch(fetch, 0, nil, err)
		return PollResult{Fetch: fetch}, err
	}
	req.Header.Set("User-Agent", s.UserAgent)

	resp, err := s.Client.Do(req)
	if err != nil {
		fetch = FinishFetch(fetch, 0, nil, err)
		return PollResult{Fetch: fetch}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		fetch = FinishFetch(fetch, resp.StatusCode, nil, err)
		return PollResult{Fetch: fetch}, err
	}
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("telegram returned status: %d", resp.StatusCode)
		fetch = FinishFetch(fetch, resp.StatusCode, body, err)
		return PollResult{Fetch: fetch}, err
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		fetch = FinishFetch(fetch, resp.StatusCode, body, err)
		return PollResult{Fetch: fetch}, err
	}

	var items []Item
	maxItems := source.EffectiveMaxItemsPerPoll(100)
	var f func(*html.Node)
	f = func(n *html.Node) {
		if len(items) >= maxItems {
			return
		}
		if n.Type == html.ElementNode && n.Data == "div" {
			for _, a := range n.Attr {
				if a.Key == "class" && strings.Contains(a.Val, "tgme_widget_message_wrap") {
					item := s.parseMessage(n, source, fetch.FetchID)
					if item != nil {
						items = append(items, *item)
					}
					if len(items) >= maxItems {
						return
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	source.LastPolled = time.Now()
	fetch = FinishFetch(fetch, resp.StatusCode, body, nil)
	fetch.ItemCount = len(items)
	return PollResult{Fetch: fetch, Items: items}, nil
}

func (s *TelegramScraper) parseMessage(n *html.Node, source *Source, fetchID string) *Item {
	// Very basic parser for the toy version - extracts text from tgme_widget_message_text
	var text string
	var msgID string
	var published time.Time

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" {
			for _, a := range n.Attr {
				if a.Key == "class" && strings.Contains(a.Val, "tgme_widget_message_text") {
					text = s.getText(n)
				}
				if a.Key == "data-post" {
					msgID = a.Val
				}
			}
		}
		if n.Type == html.ElementNode && n.Data == "time" {
			for _, a := range n.Attr {
				if a.Key == "datetime" {
					t, err := time.Parse(time.RFC3339, a.Val)
					if err == nil {
						published = t
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)

	if text == "" {
		return nil
	}
	if published.IsZero() {
		published = time.Now().UTC()
	}
	sourceURL := fmt.Sprintf("https://t.me/%s", msgID)

	item := &Item{
		ID:            StableItemID(*source, msgID, sourceURL, source.Name, text),
		SourceID:      source.ID,
		SourceType:    source.Type,
		FetchID:       fetchID,
		OriginalID:    msgID,
		Title:         fmt.Sprintf("Telegram Post from %s", source.Name),
		Body:          text,
		URL:           sourceURL,
		CanonicalURL:  NormalizeURL(sourceURL),
		Published:     published.UTC(),
		FetchedAt:     time.Now().UTC(),
		Verticals:     source.Verticals,
		Language:      firstString(source.Languages),
		Region:        firstString(source.Regions),
		BodyKind:      BodyKindSocialPost,
		BodyLength:    len([]rune(strings.TrimSpace(text))),
		EvidenceLevel: "source_feed",
	}
	item.ContentHash = ContentHash(item.Title, item.Body, item.CanonicalURL)
	return item
}

func (s *TelegramScraper) getText(n *html.Node) string {
	var b strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return strings.TrimSpace(b.String())
}
