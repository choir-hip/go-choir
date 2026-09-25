package promptspec

import "testing"

func TestParseRequiresVersionAndBody(t *testing.T) {
	if _, err := Parse([]byte("version: 1\nbody: | \n  hello")); err != nil {
		t.Fatalf("parse valid prompt: %v", err)
	}
	if _, err := Parse([]byte("body: hello")); err == nil {
		t.Fatal("expected version requirement")
	}
	if _, err := Parse([]byte("version: 1\nbody: \"\"")); err == nil {
		t.Fatal("expected body requirement")
	}
}

func TestRenderConditionalTemplate(t *testing.T) {
	raw := []byte(`version: 1
body: |
  {{if .Enabled}}on{{else}}off{{end}}
`)
	doc, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	on, err := doc.Render(struct{ Enabled bool }{Enabled: true})
	if err != nil || on != "on" {
		t.Fatalf("render on = %q err=%v", on, err)
	}
	off, err := doc.Render(struct{ Enabled bool }{Enabled: false})
	if err != nil || off != "off" {
		t.Fatalf("render off = %q err=%v", off, err)
	}
}
