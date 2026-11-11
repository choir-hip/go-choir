package themedoc

import (
	"strings"
	"testing"
)

// Regression tests for review round 1 (T7-T10): no input may panic.

func TestWrongTypedValuesRefuseAndNeverPanic(t *testing.T) {
	repl := func(old, new string) string { return strings.Replace(newsprint, old, new, 1) }
	cases := map[string]string{
		"schemes number":    repl(`["light", "dark"]`, `[1]`),
		"schemes object":    repl(`["light", "dark"]`, `[{}]`),
		"schemes bool":      repl(`["light", "dark"]`, `[true]`),
		"schemes scalar":    repl(`["light", "dark"]`, `1`),
		"schemes string":    repl(`["light", "dark"]`, `"light"`),
		"schemes null":      repl(`["light", "dark"]`, `[null]`),
		"id number":         repl(`"id": "newsprint"`, `"id": 7`),
		"id object":         repl(`"id": "newsprint"`, `"id": {}`),
		"id list":           repl(`"id": "newsprint"`, `"id": []`),
		"name number":       repl(`"name": "Newsprint"`, `"name": 1`),
		"name bool":         repl(`"name": "Newsprint"`, `"name": false`),
		"colors list":       repl(`"colors": {`, `"colors": [{`),
		"colors number":     repl(`"fg":     { "light": "#111111", "dark": "#EDEDED" }`, `"fg": 5`),
		"colors bool leaf":  repl(`"light": "#111111"`, `"light": true`),
		"colors null leaf":  repl(`"light": "#111111"`, `"light": null`),
		"colors list leaf":  repl(`"light": "#111111"`, `"light": ["#111111"]`),
		"colors whole num":  repl(`"colors": {`, `"colors": 1, "x": {`),
		"radii number":      repl(`"card": "0px"`, `"card": 0`),
		"radii list":        repl(`"card": "0px"`, `"card": ["0px"]`),
		"type number":       repl(`"read": "ui-serif, Georgia, serif"`, `"read": 1`),
		"type list":         repl(`"read": "ui-serif, Georgia, serif"`, `"read": ["serif"]`),
		"type object":       repl(`"read": "ui-serif, Georgia, serif"`, `"read": {}`),
		"version string":    repl(`"schema_version": 3`, `"schema_version": "3"`),
		"version object":    repl(`"schema_version": 3`, `"schema_version": {}`),
		"top level array":   "```choir-theme\n[1]\n```",
		"top level number":  "```choir-theme\n1\n```",
		"top level null":    "```choir-theme\nnull\n```",
		"elevation list ok": repl(`"press": 0.98`, `"press": [[[[[[1]]]]]]`),
	}
	for name, text := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: panicked: %v", name, r)
				}
			}()
			if _, _, err := Parse(text); err == nil {
				t.Errorf("%s: parsed, want a refusal", name)
			}
		}()
	}
}

func TestTransparentOnAccentFailsTheGate(t *testing.T) { // T9
	text := strings.Replace(newsprint, `"onAccent": { "light": "#FFFFFF", "dark": "#0E0E0E" }`, `"onAccent": { "light": "rgba(255,255,255,0)", "dark": "#0E0E0E" }`, 1)
	if _, _, err := Parse(text); err == nil {
		t.Fatal("transparent onAccent passed the contrast gate")
	}
	text = strings.Replace(newsprint, `"paper":  { "light": "#F7F5F0"`, `"paper":  { "light": "rgba(247,245,240,0.5)"`, 1)
	if _, _, err := Parse(text); err == nil {
		t.Fatal("translucent paper accepted")
	}
	text = strings.Replace(newsprint, `"fg":     { "light": "#111111"`, `"fg":     { "light": "rgba(17,17,17,0.1)"`, 1)
	if _, _, err := Parse(text); err == nil {
		t.Fatal("translucent fg passed the contrast gate")
	}
}

func TestUnbalancedQuotesAndOddKeys(t *testing.T) {
	text := strings.Replace(newsprint, "ui-serif, Georgia, serif", "ui-serif, 'Georgia, serif", 1) // T10
	if _, _, err := Parse(text); err == nil {
		t.Fatal("unbalanced quote accepted")
	}
	text = strings.Replace(newsprint, `"radii": {`, `"radii": { "a b": "1px",`, 1) // T8
	if _, _, err := Parse(text); err == nil {
		t.Fatal("odd key under radii accepted")
	}
	text = strings.Replace(newsprint, `"layout":`, `"<script>alert(1)</script>": 1, "web_only-1": 2, "layout":`, 1)
	_, ignored, err := Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(ignored, "|")
	if strings.Contains(joined, "script") || !strings.Contains(joined, "web_only-1") || !strings.Contains(joined, "(1 other)") {
		t.Fatalf("ignored = %v", ignored)
	}
}
