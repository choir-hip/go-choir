package texturedoc

// The golden documents (internal/texturedoc/golden/README.md) are the page
// format's shared test: the web renderer and the native renderer draw the same
// files and scripts/golden-docs compares what they draw. This test holds the
// other side of the bargain: every golden body is a valid document of the
// schema, so a renderer is never graded on a page the server would refuse.
//
// Ways it can fail, written before the code:
//   - a golden body the validator refuses (a feature the format does not have);
//   - a source entity nobody cites, or a source_ref with no entity;
//   - a file without its golden header, or a name that is not its file name;
//   - a golden set that stops covering a block, mark or embed kind the schema
//     defines (the set is chosen from the format, not from what renders).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type goldenFile struct {
	Golden struct {
		Name   string   `json:"name"`
		Covers []string `json:"covers"`
	} `json:"golden"`
	Artifact struct {
		BodyDoc *StructuredTextureDoc `json:"body_doc"`
		Content string                `json:"content"`
	} `json:"artifact"`
	SourceEntities []struct {
		ID     string       `json:"source_entity_id"`
		Entity SourceEntity `json:"entity"`
	} `json:"source_entities"`
}

func TestGoldenDocumentsAreValidAndCoverTheFormat(t *testing.T) {
	files, err := filepath.Glob("golden/*.json")
	if err != nil || len(files) < 8 {
		t.Fatalf("golden documents: %v, %d files", err, len(files))
	}
	covered := map[string]bool{}
	for _, file := range files {
		if filepath.Base(file) == "known-differences.json" {
			continue
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var g goldenFile
		if err := json.Unmarshal(raw, &g); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if want := strings.TrimSuffix(filepath.Base(file), ".json"); g.Golden.Name != want {
			t.Fatalf("%s: golden.name %q must be the file name", file, g.Golden.Name)
		}
		for _, c := range g.Golden.Covers {
			covered[c] = true
		}
		if g.Artifact.BodyDoc == nil {
			if strings.TrimSpace(g.Artifact.Content) == "" {
				t.Fatalf("%s: neither body_doc nor content", file)
			}
			continue
		}
		var ents []SourceEntity
		for _, e := range g.SourceEntities {
			ents = append(ents, e.Entity)
		}
		if err := Validate(*g.Artifact.BodyDoc, ents); err != nil {
			t.Errorf("%s: %v", file, err)
		}
	}
	// Every block, mark, inline and embed kind of the schema must be covered.
	for _, want := range []string{
		"block:paragraph", "block:heading-1", "block:heading-6", "block:bullet_list", "block:ordered_list", "block:list_item-nested-list",
		"block:blockquote", "block:code_block", "block:horizontal_rule",
		"block:embed-image", "block:embed-audio", "block:embed-video", "block:embed-file", "block:embed-capsule_evidence",
		"inline:strong", "inline:emphasis", "inline:code", "inline:hard_break",
		"inline:source_ref-numbered_ref", "inline:source_ref-expanded_ref", "entity:web_url", "entity:texture_span",
	} {
		if !covered[want] {
			t.Errorf("no golden document covers %s", want)
		}
	}
}
