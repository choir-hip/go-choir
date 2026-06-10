package sources

import "testing"

func TestParseGDELTLastUpdateExtractsGKGAndMentions(t *testing.T) {
	body := `1234567890 123 http://data.gdeltproject.org/gdeltv2/20260101120000.export.CSV.zip
1234567890 123 http://data.gdeltproject.org/gdeltv2/20260101120000.mentions.CSV.zip
1234567890 123 http://data.gdeltproject.org/gdeltv2/20260101120000.gkg.CSV.zip
`
	gkg, mentions := parseGDELTLastUpdate(body)
	if gkg != "http://data.gdeltproject.org/gdeltv2/20260101120000.gkg.CSV.zip" {
		t.Fatalf("gkg URL = %q", gkg)
	}
	if mentions != "http://data.gdeltproject.org/gdeltv2/20260101120000.mentions.CSV.zip" {
		t.Fatalf("mentions URL = %q", mentions)
	}
}
