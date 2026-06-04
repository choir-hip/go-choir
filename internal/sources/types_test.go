package sources

import "testing"

func TestStableItemIDUsesFallbackWhenOriginalIDMissing(t *testing.T) {
	source := Source{ID: "rss:test", Type: SourceTypeRSS}
	first := StableItemID(source, "", "https://example.com/story#section", "Title", "Body")
	second := StableItemID(source, "", "https://example.com/story", "Different title", "Different body")
	if first == "" {
		t.Fatal("stable item id is empty")
	}
	if first != second {
		t.Fatalf("stable item id should normalize URL fragments: %q != %q", first, second)
	}
	third := StableItemID(source, "", "", "Title", "Body")
	fourth := StableItemID(source, "", "", "Title", "Body")
	if third == "" || third != fourth {
		t.Fatalf("content fallback should be stable: %q != %q", third, fourth)
	}
}

func TestContentHashChangesWithContent(t *testing.T) {
	first := ContentHash("title", "body")
	second := ContentHash("title", "other")
	if first == "" || second == "" {
		t.Fatal("content hash is empty")
	}
	if first == second {
		t.Fatalf("content hash did not change")
	}
}
