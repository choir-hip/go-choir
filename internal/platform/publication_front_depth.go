package platform

// The depth of a front story's latest version: how many distinct sources it
// cites and how long its body is. The front page weighs importance by depth
// so the paper does not lead with a one-source stub
// (docs/archive/problems/front-page-leads-with-a-one-source-stub-2026-10-14.md).
// Both are read from the version manifest the front row already loads; no
// capture or computer is read.

import (
	"encoding/json"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/texturedoc"
)

// frontDepth returns a version's distinct cited sources and body words, or
// nil for both when the version has no structured body to count.
//
// Sources are counted the way the reader's "Sources N" counts them
// (buildPublishedCitationContext in
// frontend/src/packages/texture/texture-published-citations.ts): one per distinct source
// entity over the body's citations and embeds, keyed by the anchor's source
// entity id, else by the capture it cites. A capture cited many times is one
// source.
func frontDepth(bodyDoc json.RawMessage, anchors []PublicationAnchor) (*int, *int) {
	raw := strings.TrimSpace(string(bodyDoc))
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var doc texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(bodyDoc, &doc); err != nil {
		return nil, nil
	}
	sources := map[string]bool{}
	for _, a := range anchors {
		if strings.TrimSpace(a.AnchorID) == "" || strings.TrimSpace(a.Ref.Doc) == "" {
			continue
		}
		key := strings.TrimSpace(a.SourceEntityID)
		if key == "" {
			key = strings.TrimSpace(a.Ref.Doc)
		}
		sources[key] = true
	}
	words := 0
	var walk func(texturedoc.Node)
	walk = func(n texturedoc.Node) {
		if n.Type == "text" {
			words += countWords(n.Text)
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(doc.Doc)
	sourceCount := len(sources)
	return &sourceCount, &words
}

// countWords counts words the way the publish word floor does.
func countWords(text string) int { return texturedoc.CountWords(text) }
