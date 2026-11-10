// Package styleguide parses a style-guide Texture into a closed, typed
// design (docs/design/autopaper-next-mission-handoff-2026-11-09.md, 4.G).
//
// A style guide is a Texture document titled "Style Guide" whose
// `## Design values` section lists `- key: value` lines. Every value is a
// name from a closed table; no guide text is ever passed on verbatim, so a
// guide a reader forked or took cannot inject CSS. Pass 1 applies a design
// at read time only: this package is imported by the Texture read surface
// and by nothing that builds a prompt (a test holds that line).
package styleguide

import (
	"strings"
)

// MaxGuideBytes bounds a guide the parser reads; a larger one is the default.
const MaxGuideBytes = 16 * 1024

const (
	maxGuideLines = 400
	sectionTitle  = "design values"
)

// Title is the title of a style-guide document.
const Title = "Style Guide"

// Design is the closed set of values a guide can choose. Fields hold names
// from the tables below, never free text.
type Design struct {
	Serif          string `json:"serif"`
	HeadlineWeight string `json:"headline_weight"`
	DekStyle       string `json:"dek_style"`
	NameplateRule  string `json:"nameplate_rule"`
}

// HouseGuideText is the house guide's text, the document the paper's
// computer holds. It parses to Default.
const HouseGuideText = `# Style Guide

The paper's front page look. Read time only: these values change how the
front page is drawn, never what a story says.

## Design values

- serif: classic
- headline-weight: 700
- dek-style: normal
- nameplate-rule: double

## Choices

- serif: classic or modern
- headline-weight: 600, 700 or 800
- dek-style: normal or italic
- nameplate-rule: double, single or none
`

var choices = map[string]map[string]bool{
	"serif":           {"classic": true, "modern": true},
	"headline-weight": {"600": true, "700": true, "800": true},
	"dek-style":       {"normal": true, "italic": true},
	"nameplate-rule":  {"double": true, "single": true, "none": true},
}

// Default is today's page.
func Default() Design {
	return Design{Serif: "classic", HeadlineWeight: "700", DekStyle: "normal", NameplateRule: "double"}
}

// IsGuideTitle reports whether a document title names a style guide.
func IsGuideTitle(title string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(title), " "), Title)
}

func (d *Design) set(key, value string) {
	switch key {
	case "serif":
		d.Serif = value
	case "headline-weight":
		d.HeadlineWeight = value
	case "dek-style":
		d.DekStyle = value
	case "nameplate-rule":
		d.NameplateRule = value
	}
}

// Parse reads a guide's text. It never fails: a bad, empty, binary or
// oversize guide is the default, and entries it did not apply (unknown key
// or value, a repeat) are reported in ignored.
func Parse(text string) (design Design, ignored []string) {
	design = Default()
	if text == "" || len(text) > MaxGuideBytes || strings.ContainsRune(text, 0) {
		return design, nil
	}
	seen := map[string]bool{}
	inSection, inFence := false, false
	lines := strings.Split(text, "\n")
	if len(lines) > maxGuideLines {
		lines = lines[:maxGuideLines]
	}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(line, "#") {
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			inSection = strings.EqualFold(title, sectionTitle)
			continue
		}
		if !inSection || len(line) < 2 || (line[0] != '-' && line[0] != '*') || line[1] != ' ' {
			continue
		}
		k, v, ok := strings.Cut(line[2:], ":")
		if !ok {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(k))
		value := strings.ToLower(strings.TrimSpace(v))
		if !choices[key][value] || seen[key] {
			ignored = append(ignored, clip(line))
			continue
		}
		seen[key] = true
		design.set(key, value)
	}
	return design, ignored
}

func clip(s string) string {
	if len(s) > 80 {
		return s[:80]
	}
	return s
}
