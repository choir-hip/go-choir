package sources

import "testing"

func TestParseGDELTLastUpdateExtractsGKGMentionsAndExport(t *testing.T) {
	body := `1234567890 123 http://data.gdeltproject.org/gdeltv2/20260101120000.export.CSV.zip
1234567890 123 http://data.gdeltproject.org/gdeltv2/20260101120000.mentions.CSV.zip
1234567890 123 http://data.gdeltproject.org/gdeltv2/20260101120000.gkg.CSV.zip
`
	urls := parseGDELTLastUpdate(body)
	if urls.GKG != "http://data.gdeltproject.org/gdeltv2/20260101120000.gkg.CSV.zip" {
		t.Fatalf("gkg URL = %q", urls.GKG)
	}
	if urls.Mentions != "http://data.gdeltproject.org/gdeltv2/20260101120000.mentions.CSV.zip" {
		t.Fatalf("mentions URL = %q", urls.Mentions)
	}
	if urls.Export != "http://data.gdeltproject.org/gdeltv2/20260101120000.export.CSV.zip" {
		t.Fatalf("export URL = %q", urls.Export)
	}
}

func TestGDELTStreamsUpToDateRequiresAllPresentCursors(t *testing.T) {
	urls := gdeltLastUpdateURLs{
		GKG:      "http://example.test/gkg.csv.zip",
		Mentions: "http://example.test/mentions.csv.zip",
		Export:   "http://example.test/export.csv.zip",
	}
	source := &Source{
		LastETag:      urls.GKG,
		LastModified:  urls.Mentions,
		LastAuxCursor: urls.Export,
	}
	if !gdeltStreamsUpToDate(urls, source) {
		t.Fatal("expected all GDELT streams to be up to date")
	}
	source.LastAuxCursor = ""
	if gdeltStreamsUpToDate(urls, source) {
		t.Fatal("expected stale export cursor to prevent up-to-date state")
	}
}
