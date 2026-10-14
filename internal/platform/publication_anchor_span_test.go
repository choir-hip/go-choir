package platform

import (
	"encoding/json"
	"strings"
	"testing"
)

// A citation's quote span (H-OG3) rides the Ref from the body_doc into the
// version's anchors, and so into the manifest the reader resolves; a Ref
// written before spans has none and keeps its bytes.
func TestPublicationAnchorsCarryTheQuoteSpan(t *testing.T) {
	bodyDoc := `{"schema":"choir.texture.structured.v1","doc":{"type":"doc","attrs":{"id":"root"},"content":[
	  {"type":"paragraph","attrs":{"id":"p1"},"content":[
	    {"type":"text","text":"Ruled again."},
	    {"type":"source_ref","attrs":{"id":"ref-new","source_entity_id":"se-1","display_mode":"numbered_ref",
	      "ref":{"doc":"cap-1","version":"v1","cid":"k1","hash":"sha256:b","quote":"ruled again",
	             "span":{"start":4,"end":15,"text_hash":"sha256:t","segments":[{"start":4,"end":9},{"start":10,"end":15}]}}}},
	    {"type":"source_ref","attrs":{"id":"ref-old","source_entity_id":"se-1","display_mode":"numbered_ref",
	      "ref":{"doc":"cap-1","version":"v1","cid":"k1","hash":"sha256:b","quote":"ruled"}}}
	  ]}]}}`
	anchors, err := publicationAnchors(json.RawMessage(bodyDoc))
	if err != nil {
		t.Fatal(err)
	}
	if len(anchors) != 2 {
		t.Fatalf("anchors = %d, want 2", len(anchors))
	}
	span := anchors[0].Ref.Span
	if span == nil || span.Start != 4 || span.End != 15 || span.TextHash != "sha256:t" || len(span.Segments) != 2 {
		t.Fatalf("new anchor span = %+v", span)
	}
	if anchors[1].Ref.Span != nil {
		t.Fatalf("old anchor grew a span: %+v", anchors[1].Ref.Span)
	}
	raw, err := json.Marshal(anchors[1].Ref)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "span") {
		t.Fatalf("old anchor ref bytes changed: %s", raw)
	}
}
