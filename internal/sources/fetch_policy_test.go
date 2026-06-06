package sources

import (
	"context"
	"testing"
)

func TestGDELTFetcherRejectsForbiddenConfiguredURL(t *testing.T) {
	source := Source{
		ID:   "gdelt:local",
		Type: SourceTypeGDELT,
		URL:  "http://127.0.0.1/latest.txt",
	}
	result, err := NewGDELTFetcher("ChoirTest/1.0").Poll(context.Background(), &source)
	if err == nil {
		t.Fatal("Poll allowed forbidden configured URL")
	}
	if result.Fetch.Status != "error" || result.Fetch.Error == "" {
		t.Fatalf("fetch record = %+v, want error evidence", result.Fetch)
	}
}

func TestGDELTFetcherRejectsForbiddenSecondStageURL(t *testing.T) {
	source := Source{
		ID:   "gdelt:second-stage",
		Type: SourceTypeGDELT,
		URL:  "https://data.gdeltproject.org/gdeltv2/lastupdate.txt",
	}
	_, _, err := NewGDELTFetcher("ChoirTest/1.0").fetchGKG(context.Background(), "http://127.0.0.1/gkg.csv.zip", &source, "fetch-test")
	if err == nil {
		t.Fatal("fetchGKG allowed forbidden second-stage URL")
	}
}
