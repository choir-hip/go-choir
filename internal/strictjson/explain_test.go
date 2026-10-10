package strictjson

import (
	"strings"
	"testing"
)

type packet struct {
	Kind  string   `json:"kind"`
	Notes []string `json:"notes,omitempty"`
}

type control struct {
	Objective string  `json:"objective"`
	Packet    *packet `json:"packet,omitempty"`
}

type edit struct {
	Op   string `json:"op"`
	Text string `json:"text"`
}

type body struct {
	Op        string         `json:"op"`
	Rationale string         `json:"rationale,omitempty"`
	Edits     []edit         `json:"edits,omitempty"`
	Controls  []control      `json:"controls,omitempty"`
	Extra     map[string]any `json:"extra,omitempty"`
	Raw       any            `json:"raw,omitempty"`
	ignored   string
}

// docs/problems/desk-cell-ergonomics-from-demo-traces-2026-10-10.md (T2):
// Go's strict decoder says `unknown field "rationale"` without a location,
// and the hint listed the top-level fields, where rationale is valid. Texture
// lost five cells to it. Failure modes pinned: the path of a nested unknown
// field is missing or wrong; the valid list is the top level's instead of the
// object the field sits in; free-form maps and interface values are treated
// as closed; a well-formed body is reported.
func TestUnknownFieldNamesItsPathAndTheFieldsValidThere(t *testing.T) {
	got := Explain([]byte(`{"op":"apply","edits":[{"op":"x","text":"t"},{"op":"x","rationale":"why"}]}`), &body{})
	if !strings.Contains(got, `"rationale" at edits[1]`) || !strings.Contains(got, "valid there: op, text") {
		t.Fatalf("nested edit: %q", got)
	}
	got = Explain([]byte(`{"op":"apply","controls":[{"objective":"o","packet":{"kind":"k","decision":"d"}}]}`), &body{})
	if !strings.Contains(got, `"decision" at controls[0].packet`) || !strings.Contains(got, "valid there: kind, notes") {
		t.Fatalf("nested packet: %q", got)
	}
	got = Explain([]byte(`{"op":"apply","questions_note":"q"}`), &body{})
	if !strings.Contains(got, `"questions_note" at the top level`) || !strings.Contains(got, "controls, edits, extra, op, rationale, raw") {
		t.Fatalf("top level: %q", got)
	}
	if got := Explain([]byte(`{"op":"apply","extra":{"anything":1},"raw":{"free":true}}`), &body{}); got != "" {
		t.Fatalf("free-form values reported: %q", got)
	}
	if got := Explain([]byte(`{"op":"apply","rationale":"ok"}`), &body{}); got != "" {
		t.Fatalf("well-formed body reported: %q", got)
	}
}
