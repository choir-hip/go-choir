// Package podcastworker is the operator's podcast worker: it polls feeds,
// transcribes episodes with parakeet.cpp and posts timed transcripts to the
// platform (docs/design/podcast-clips-2026-11-15.md section 3). It holds the
// logic; cmd/podcast-worker is its command line.
package podcastworker

import (
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Episode is one feed item as the platform's episode route takes it.
type Episode struct {
	FeedURL         string  `json:"feed_url"`
	GUID            string  `json:"guid"`
	Show            string  `json:"show"`
	Title           string  `json:"title"`
	Published       string  `json:"published"`
	DurationSeconds float64 `json:"duration_seconds"`
	EnclosureURL    string  `json:"enclosure_url"`
	Notes           string  `json:"notes,omitempty"`
}

type rssFeed struct {
	Channel struct {
		Title string    `xml:"title"`
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title     string `xml:"title"`
	GUID      string `xml:"guid"`
	PubDate   string `xml:"pubDate"`
	Desc      string `xml:"description"`
	Duration  string `xml:"duration"`
	Enclosure struct {
		URL  string `xml:"url,attr"`
		Type string `xml:"type,attr"`
	} `xml:"enclosure"`
}

var tagRe = regexp.MustCompile(`<[^>]*>`)

// ParseFeed reads an RSS 2.0 podcast feed and returns its items that carry an
// audio enclosure, newest first as the feed lists them. showHint names the
// show when the feed has no title.
func ParseFeed(feedURL string, r io.Reader, showHint string) ([]Episode, error) {
	var feed rssFeed
	dec := xml.NewDecoder(io.LimitReader(r, 32<<20))
	dec.Strict = false
	if err := dec.Decode(&feed); err != nil {
		return nil, fmt.Errorf("parse feed %s: %w", feedURL, err)
	}
	show := strings.TrimSpace(feed.Channel.Title)
	if showHint != "" {
		show = showHint
	}
	var out []Episode
	for _, it := range feed.Channel.Items {
		enc := strings.TrimSpace(it.Enclosure.URL)
		if enc == "" || (it.Enclosure.Type != "" && !strings.HasPrefix(strings.ToLower(it.Enclosure.Type), "audio")) {
			continue
		}
		pub, ok := parsePubDate(it.PubDate)
		if !ok {
			continue
		}
		guid := strings.TrimSpace(it.GUID)
		if guid == "" {
			guid = enc
		}
		out = append(out, Episode{
			FeedURL: feedURL, GUID: guid, Show: show, Title: oneLine(it.Title), Published: pub.UTC().Format(time.RFC3339),
			DurationSeconds: parseDuration(it.Duration), EnclosureURL: enc, Notes: plainNotes(it.Desc),
		})
	}
	return out, nil
}

var pubFormats = []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822, "Mon, 2 Jan 2006 15:04:05 -0700", "Mon, 2 Jan 2006 15:04:05 MST", time.RFC3339}

func parsePubDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, f := range pubFormats {
		if t, err := time.Parse(f, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseDuration reads itunes:duration: seconds, MM:SS or HH:MM:SS.
func parseDuration(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	total := 0.0
	for _, part := range strings.Split(s, ":") {
		v, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil || v < 0 {
			return 0
		}
		total = total*60 + v
	}
	return total
}

func oneLine(s string) string { return strings.Join(strings.Fields(html.UnescapeString(s)), " ") }

func plainNotes(s string) string {
	s = oneLine(tagRe.ReplaceAllString(s, " "))
	const max = 1800
	if len(s) > max {
		cut := max
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = s[:cut]
	}
	return s
}

// FeedSpec is a feed to poll: a show name when the operator's list gives one.
type FeedSpec struct {
	Show string
	URL  string
}

// NormalizeFeedKey makes two spellings of one feed one key (the platform's
// rule: lower-case scheme and host, default port, fragment and trailing slash
// dropped).
func NormalizeFeedKey(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return strings.TrimSpace(raw)
	}
	u.Scheme, u.Host, u.Fragment = strings.ToLower(u.Scheme), strings.ToLower(u.Host), ""
	if (u.Scheme == "https" && strings.HasSuffix(u.Host, ":443")) || (u.Scheme == "http" && strings.HasSuffix(u.Host, ":80")) {
		u.Host = u.Host[:strings.LastIndex(u.Host, ":")]
	}
	if len(u.Path) > 1 {
		u.Path = strings.TrimRight(u.Path, "/")
	}
	return u.String()
}

// MergeFeeds is the union of the operator's list and the feeds readers report,
// the operator's names and order first, each feed once.
func MergeFeeds(seed []FeedSpec, reported []WorkerFeed) []FeedSpec {
	seen := map[string]bool{}
	var out []FeedSpec
	for _, f := range seed {
		if k := NormalizeFeedKey(f.URL); !seen[k] {
			seen[k] = true
			out = append(out, f)
		}
	}
	for _, f := range reported {
		if k := NormalizeFeedKey(f.FeedURL); !seen[k] {
			seen[k] = true
			out = append(out, FeedSpec{Show: f.Title, URL: f.FeedURL})
		}
	}
	return out
}
