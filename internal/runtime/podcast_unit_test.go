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
