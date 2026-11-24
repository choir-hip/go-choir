package textureowner

// The phone sends `content` as the text the server derives from its body_doc (mobile/fixtures/texture-body/projection.json, checked against texturedoc.Project by internal/texturedoc). The owner revision route
// rewrites a draft before that check (`stabilizeTextureUserMarkdownStructures`, revise_edit.go): a plain document's content must come through it unchanged, or the projection check would refuse it. The phone
// edits only documents without a Markdown table in their content (texture-body.ts `plainOnly`), the one case in which that function acts.

import (
	"encoding/json"
	"os"
	"testing"
)

func TestPhoneContentSurvivesTheStabilizer(t *testing.T) {
	raw, err := os.ReadFile("../../mobile/fixtures/texture-body/projection.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Projection []struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		} `json:"projection"`
		Merge []struct {
			Name        string `json:"name"`
			HeadContent string `json:"head_content"`
			Content     string `json:"content"`
		} `json:"merge"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Merge) < 11 {
		t.Fatalf("the fixture has %d merge cases", len(f.Merge))
	}
	for _, c := range f.Merge {
		if got, changed := stabilizeTextureUserMarkdownStructures(c.HeadContent, c.Content); changed || got != c.Content {
			t.Errorf("%s: the stabilizer rewrote %q to %q", c.Name, c.Content, got)
		}
	}
	for _, c := range f.Projection {
		if got, changed := stabilizeTextureUserMarkdownStructures(c.Content, c.Content); changed || got != c.Content {
			t.Errorf("%s: the stabilizer rewrote %q to %q", c.Name, c.Content, got)
		}
	}
}
