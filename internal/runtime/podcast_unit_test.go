package runtime

import "testing"

func TestPodcastSubscriptionTitleNeedsRefresh(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		imported string
		source   string
		want     bool
	}{
		{
			name:     "blank title",
			current:  "",
			imported: "Tasteland",
			source:   "https://example.com/feed.xml",
			want:     true,
		},
		{
			name:     "raw URL title",
			current:  "https://example.com/feed.xml",
			imported: "Tasteland",
			source:   "https://example.com/feed.xml",
			want:     true,
		},
		{
			name:     "raw file title",
			current:  "019d2b90-8e5b-7649-94e1-df24921da466.xml",
			imported: "Tasteland",
			source:   "https://example.com/rss/019d2b90-8e5b-7649-94e1-df24921da466.xml",
			want:     true,
		},
		{
			name:     "user readable title",
			current:  "My favorite tech show",
			imported: "Tasteland",
			source:   "https://example.com/feed.xml",
			want:     false,
		},
		{
			name:     "no imported title",
			current:  "podcast.rss",
			imported: "",
			source:   "https://example.com/podcast.rss",
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := podcastSubscriptionTitleNeedsRefresh(tt.current, tt.imported, tt.source)
			if got != tt.want {
				t.Fatalf("podcastSubscriptionTitleNeedsRefresh(%q, %q, %q) = %t, want %t", tt.current, tt.imported, tt.source, got, tt.want)
			}
		})
	}
}

func TestExtractRSSFeedTitle(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?>
<rss><channel>
  <title><![CDATA[Tasteland &amp; Friends]]></title>
  <item><title>Episode title should not win</title></item>
</channel></rss>`)
	if got := extractRSSFeedTitle(raw); got != "Tasteland & Friends" {
		t.Fatalf("extractRSSFeedTitle() = %q, want %q", got, "Tasteland & Friends")
	}
}

func TestContentTextLooksRSSFeed(t *testing.T) {
	if !contentTextLooksRSSFeed(`<?xml version="1.0"?><rss><channel><title>Tasteland</title></channel></rss>`) {
		t.Fatal("expected RSS channel XML to be recognized")
	}
	if contentTextLooksRSSFeed(`<html><body><channel>not a feed</channel></body></html>`) {
		t.Fatal("expected non-RSS text to be ignored")
	}
}
