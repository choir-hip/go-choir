package texturedoc

import (
	"os"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
)

// World documents (captures) hold the page's own words (Cut 2 unit 2a).
// Ways it can fail, written before the code:
//
//   - A captured page with footnote markers "[12]", "Source:" lines or links
//     is refused by the rules written for Texture's own prose, so a real page
//     cannot be a block document.
//   - Relaxing the rules for world documents relaxes them for Texture too:
//     a Texture-authored block with "[12]" or a link mark must still refuse.
//   - A link mark without a target, or a block without its cid, validates.
//   - A world document carries a source_ref (an article's citation chip),
//     which belongs to Texture documents only.
func TestWorldDocumentAcceptsFootnotesAndLinksTextureStillRefuses(t *testing.T) {
	page, err := os.ReadFile("../sourcefetch/testdata/wiki-table.html")
	if err != nil {
		t.Fatal(err)
	}
	blocks := sourcefetch.ExtractBlocks(page).Blocks
	world := WorldDocument("choir.web_capture:owner:abc", blocks)
	if err := ValidateWorld(world); err != nil {
		t.Fatalf("ValidateWorld() = %v", err)
	}
	var sawMarker, sawLink bool
	var para Node
	for _, node := range world.Doc.Content {
		for _, inline := range node.Content {
			if strings.Contains(inline.Text, "[12]") {
				sawMarker, para = true, node
			}
			for _, mark := range inline.Marks {
				if mark.Type == "link" && mark.Attrs["href"] == "/wiki/City_proper" && inline.Text == "city proper" {
					sawLink = true
				}
			}
		}
	}
	if !sawMarker || !sawLink {
		t.Fatalf("world document lost the footnote marker (%v) or the link (%v)", sawMarker, sawLink)
	}
	if id := nodeID(para); id != blocks[1].ID {
		t.Fatalf("block node id = %q, want the block's cid %q", id, blocks[1].ID)
	}

	// The same paragraph authored by Texture refuses both.
	doc := validDoc()
	doc.Doc.Content[0].Content = []Node{{Type: "text", Text: "Cities may be defined three ways.[12]"}}
	if err := Validate(doc, validEntities()); err == nil || !strings.Contains(err.Error(), "numbered citation") {
		t.Fatalf("Texture block with [12]: Validate() = %v", err)
	}
	doc = validDoc()
	doc.Doc.Content[0].Content = []Node{{Type: "text", Text: "city proper", Marks: []Mark{{Type: "link", Attrs: map[string]any{"href": "/wiki/City_proper"}}}}}
	if err := Validate(doc, validEntities()); err == nil || !strings.Contains(err.Error(), "link") {
		t.Fatalf("Texture link mark: Validate() = %v", err)
	}
}

func TestWorldDocumentRefusesMalformedNodes(t *testing.T) {
	blocks := sourcefetch.TextBlocks("Source: Reuters. Prices rose.[3]")
	cases := map[string]func(*StructuredTextureDoc){
		"link without href": func(d *StructuredTextureDoc) {
			d.Doc.Content[0].Content[0].Marks = []Mark{{Type: "link"}}
		},
		"block without id": func(d *StructuredTextureDoc) {
			delete(d.Doc.Content[0].Attrs, "id")
		},
		"source_ref": func(d *StructuredTextureDoc) {
			d.Doc.Content[0].Content = append(d.Doc.Content[0].Content, Node{Type: "source_ref", Attrs: map[string]any{"id": "r1", "source_entity_id": "s1"}})
		},
		"texture schema": func(d *StructuredTextureDoc) {
			d.Schema = SchemaV1
		},
	}
	if err := ValidateWorld(WorldDocument("capture-1", blocks)); err != nil {
		t.Fatalf("plain world document: %v", err)
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			doc := WorldDocument("capture-1", blocks)
			mutate(&doc)
			if err := ValidateWorld(doc); err == nil {
				t.Fatalf("ValidateWorld accepted %s", name)
			}
		})
	}
}
