package texturedoc

// The phone's Texture editor writes a revision's `body_doc` itself (mobile/AutoradioLab/texture-body.ts) and sends `content` as the text the server derives from it. The server refuses any other
// content ("content must match derived body_doc projection": internal/store/texture_structured_revision.go prepareTextureRevision), so the phone's projection must be this package's, byte for byte.
//
// The fixture mobile/fixtures/texture-body/projection.json is the shared truth. Its `content` fields are WRITTEN by this test (UPDATE_PHONE_PROJECTION=1) from Project, never by hand and never by the
// phone; the phone's bun test (texture-body-projection.test.ts) reads the same file and must produce the same text, and must produce the same body_doc shapes for the `merge` cases (the `body_doc` of
// those is written by the phone's test). A change on either side that the other has not followed fails one of the two tests.
//   - every projection case's `content` is Project(body_doc).Text;
//   - every merge case's `body_doc` (what the phone writes for `text`) is valid for the server (Validate) and projects to its `content`, which is `text` with a single newline kept as a line break.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const phoneProjectionFixture = "../../mobile/fixtures/texture-body/projection.json"

type phoneProjectionCase struct {
	Name     string               `json:"name"`
	Text     string               `json:"text,omitempty"`
	HeadBody *json.RawMessage     `json:"head_body,omitempty"`
	HeadText string               `json:"head_content,omitempty"`
	BodyDoc  StructuredTextureDoc `json:"body_doc"`
	Content  string               `json:"content"`
}

type phoneProjectionFile struct {
	Comment    string                `json:"comment"`
	Projection []phoneProjectionCase `json:"projection"`
	Merge      []phoneProjectionCase `json:"merge"`
}

func TestPhoneProjectionFixture(t *testing.T) {
	raw, err := os.ReadFile(phoneProjectionFixture)
	if err != nil {
		t.Fatal(err)
	}
	var f phoneProjectionFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Projection) < 16 || len(f.Merge) < 11 {
		t.Fatalf("the fixture has %d projection and %d merge cases", len(f.Projection), len(f.Merge))
	}
	if os.Getenv("UPDATE_PHONE_PROJECTION") == "1" {
		// the file is read generically and only each case's `content` is set, from Project: nothing else in it is touched
		var g map[string]any
		if err := json.Unmarshal(raw, &g); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"projection", "merge"} {
			items, ok := g[key].([]any)
			if !ok {
				t.Fatalf("the fixture has no %q list", key)
			}
			for _, item := range items {
				c, ok := item.(map[string]any)
				if !ok {
					t.Fatalf("%q holds a case that is not an object", key)
				}
				body, _ := json.Marshal(c["body_doc"])
				var doc StructuredTextureDoc
				if err := json.Unmarshal(body, &doc); err != nil {
					t.Fatalf("%v: %v", c["name"], err)
				}
				p, err := Project(doc, nil)
				if err != nil {
					t.Fatalf("%v: the server refuses this body_doc: %v", c["name"], err)
				}
				c["content"] = p.Text
			}
		}
		out, err := json.MarshalIndent(g, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(phoneProjectionFixture, append(out, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	for _, group := range [][]phoneProjectionCase{f.Projection, f.Merge} {
		for _, c := range group {
			p, err := Project(c.BodyDoc, nil)
			if err != nil {
				t.Errorf("%s: the server refuses this body_doc: %v", c.Name, err)
				continue
			}
			if p.Text != c.Content {
				t.Errorf("%s: the server projects %q, the fixture says %q", c.Name, p.Text, c.Content)
			}
		}
	}
	for _, c := range f.Merge {
		if want := strings.TrimRight(c.Text, "\n"); !strings.Contains(c.Name, "markup") && c.Content != want {
			t.Errorf("%s: the text %q was sent and the server derives %q", c.Name, c.Text, c.Content)
		}
	}
}
