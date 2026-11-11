package themedoc

import (
	"strings"
	"testing"
)

const newsprint = "# Newsprint\n\nprose\n\n```choir-theme\n" + `{
  "schema_version": 3, "id": "newsprint", "name": "Newsprint", "schemes": ["light", "dark"],
  "colors": {
    "paper":  { "light": "#F7F5F0", "dark": "#0E0E0E" },
    "fg":     { "light": "#111111", "dark": "#EDEDED" },
    "muted":  { "light": "#555555", "dark": "#A3A3A3" },
    "accent": { "light": "#B3261E", "dark": "#FF6E5E" },
    "onAccent": { "light": "#FFFFFF", "dark": "#0E0E0E" },
    "rule":   { "light": "rgba(32,20,58,.22)", "dark": "rgba(243,236,246,.18)" }
  },
  "type": { "read": "ui-serif, Georgia, serif", "ui": "system-ui, Inter, sans-serif", "mono": "ui-monospace, monospace", "scale": "editorial" },
  "radii": { "card": "0px", "control": "12px" },
  "elevation": { "1": { "shadow": "0 1px 2px rgba(32,20,58,.08), 0 6px 16px rgba(32,20,58,.08)", "press": 0.98 } },
  "motion": { "enter": "320ms cubic-bezier(0.16,1,0.3,1)", "spring": { "stiffness": 240 } },
  "layout": { "promptSurfacePlacement": "top" }
}` + "\n```\n"

func TestParseNewsprint(t *testing.T) {
	th, ignored, err := Parse(newsprint)
	if err != nil {
		t.Fatal(err)
	}
	if th["id"] != "newsprint" || len(ignored) != 1 || ignored[0] != "layout" { // T4
		t.Fatalf("theme=%v ignored=%v", th, ignored)
	}
	if _, ok := th["layout"]; ok {
		t.Fatal("T4: layout served")
	}
}

func TestParseRefusals(t *testing.T) {
	fence := func(body string) string { return "```choir-theme\n" + body + "\n```" }
	cases := map[string]string{
		"T1 none":          "just prose",
		"T1 two":           newsprint + newsprint,
		"T1 bad json":      fence("{not json"),
		"T6 version 2":     strings.Replace(newsprint, `"schema_version": 3`, `"schema_version": 2`, 1),
		"T2 url":           strings.Replace(newsprint, "0 1px 2px", "url(//evil) 0 1px 2px", 1),
		"T2 brace":         strings.Replace(newsprint, "320ms cubic", "320ms}body{x:y cubic", 1),
		"T2 colour":        strings.Replace(newsprint, `"#111111"`, `"red; } body{display:none}"`, 1),
		"T3 low contrast":  strings.Replace(newsprint, `"#555555"`, `"#DDDDDD"`, 1),
		"T3 missing role":  strings.Replace(newsprint, `"onAccent": { "light": "#FFFFFF", "dark": "#0E0E0E" },`, "", 1),
		"T3 missing dark":  strings.Replace(newsprint, `"fg":     { "light": "#111111", "dark": "#EDEDED" }`, `"fg": { "light": "#111111" }`, 1),
		"font not generic": strings.Replace(newsprint, "ui-serif, Georgia", "Georgia, ui-serif", 1),
		"T5 oversize":      newsprint + strings.Repeat("x", MaxBytes),
		"bad radius":       strings.Replace(newsprint, `"0px"`, `"calc(1px)"`, 1),
	}
	for name, text := range cases {
		if _, _, err := Parse(text); err == nil {
			t.Errorf("%s: parsed, want a refusal", name)
		}
	}
}
