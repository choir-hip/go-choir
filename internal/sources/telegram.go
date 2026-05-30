package sources

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type TelegramScraper struct {
	Client    *http.Client
	UserAgent string
}

func NewTelegramScraper(userAgent string) *TelegramScraper {
	return &TelegramScraper{
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		UserAgent: userAgent,
	}
}

func (s *TelegramScraper) Poll(ctx context.Context, source *Source) ([]Item, error) {
	// Telegram web preview URL: https://t.me/s/channelname
	url := source.URL
	if !strings.Contains(url, "/s/") {
		url = strings.Replace(url, "t.me/", "t.me/s/", 1)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.UserAgent)

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram returned status: %d", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var items []Item
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" {
			for _, a := range n.Attr {
				if a.Key == "class" && strings.Contains(a.Val, "tgme_widget_message_wrap") {
					item := s.parseMessage(n, source)
					if item != nil {
						items = append(items, *item)
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
	return items, nil
}

func (s *TelegramScraper) parseMessage(n *html.Node, source *Source) *Item {
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

	return &Item{
		ID:         fmt.Sprintf("telegram:%s", msgID),
		SourceID:   source.ID,
		OriginalID: msgID,
		Title:      fmt.Sprintf("Telegram Post from %s", source.Name),
		Body:       text,
		URL:        fmt.Sprintf("https://t.me/%s", msgID),
		Published:  published,
		FetchedAt:  time.Now(),
		Verticals:  source.Verticals,
	}
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
