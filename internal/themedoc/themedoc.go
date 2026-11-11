// Package themedoc parses a theme Texture into a validated version 3 theme
// (docs/design/autoradio-app-ui-2026-11-10.md, section 5, "A theme is a
// Texture or a file"; server gap 3). A theme document holds exactly one
// fenced `choir-theme` JSON block. The parser keeps only the closed set of
// top-level sections, checks every string against a restricted CSS subset so
// a forked or taken document cannot inject CSS, and gates contrast on every
// scheme the theme defines. It imports no desk or prompt code, like
// internal/styleguide.
//
// Ways it can fail, written before the code:
//
//	T1 Two blocks, none, or bad JSON is served as a theme or panics.
//	T2 A value carries CSS or script (url(), ;, {}, expression, @import,
//	   javascript:, a quote that breaks out), and a client pastes it.
//	T3 A scheme the theme claims has no colour for a required role, or one
//	   that fails the contrast gate, is served: unreadable text on the phone.
//	T4 A web-only or unknown section (layout, effects, fonts, chart*) is
//	   served, so the closed set widens silently; it must be reported ignored.
//	T5 An oversize or deeply nested document is parsed without bound.
//	T6 A version other than 3 is served as if it were v3.
package themedoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// MaxBytes bounds a theme document the parser reads.
const MaxBytes = 32 * 1024

const (
	maxDepth   = 5
	maxEntries = 400
	// Fence is the info string of the theme block.
	Fence = "choir-theme"
)

var sections = map[string]bool{
	"schema_version": true, "id": true, "name": true, "schemes": true,
	"colors": true, "type": true, "radii": true, "elevation": true, "motion": true, "mark": true,
}

var (
	idRE   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	hexRE  = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
	rgbRE  = regexp.MustCompile(`^rgba?\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})\s*(?:,\s*(0|1|0?\.\d+|1\.0+)\s*)?\)$`)
	pxRE   = regexp.MustCompile(`^\d{1,4}(\.\d+)?px$`)
	safeRE = regexp.MustCompile(`^[A-Za-z0-9 .,()%#+\-/]{1,200}$`)
	nameRE = regexp.MustCompile(`^[\p{L}\p{N} .'-]{1,40}$`)
	// stack is a font stack: names, quotes, commas; no punctuation that ends a
	// declaration.
	stackRE = regexp.MustCompile(`^[A-Za-z0-9 ,'"-]{1,200}$`)
)

var genericFonts = map[string]bool{
	"serif": true, "sans-serif": true, "monospace": true, "ui-serif": true,
	"ui-sans-serif": true, "ui-monospace": true, "ui-rounded": true, "system-ui": true,
}

var forbidden = []string{"url(", "expression", "@import", "javascript:", "var(", "calc("}

// Theme is a validated version 3 theme: the closed sections only.
type Theme map[string]any

// Parse reads a theme document's text. It returns the theme and the
// top-level sections it dropped, or an error naming why the document is not a
// usable theme.
func Parse(text string) (Theme, []string, error) {
	if len(text) > MaxBytes {
		return nil, nil, fmt.Errorf("the theme document is larger than %d bytes", MaxBytes)
	}
	if strings.ContainsRune(text, 0) {
		return nil, nil, fmt.Errorf("the theme document is not text")
	}
	block, err := block(text)
	if err != nil {
		return nil, nil, err
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(block)))
	dec.UseNumber()
	var raw map[string]any
	if err := dec.Decode(&raw); err != nil {
		return nil, nil, fmt.Errorf("the choir-theme block is not a JSON object: %v", err)
	}
	if v, ok := raw["schema_version"].(json.Number); !ok || v.String() != "3" { // T6
		return nil, nil, fmt.Errorf("schema_version must be 3")
	}
	var ignored []string
	out := Theme{}
	budget := maxEntries
	for k, v := range raw {
		if !sections[k] {
			ignored = append(ignored, k)
			continue
		}
		clean, err := cleanValue(k, v, 0, &budget)
		if err != nil {
			return nil, nil, err
		}
		out[k] = clean
	}
	sort.Strings(ignored)
	if err := out.check(); err != nil {
		return nil, nil, err
	}
	return out, ignored, nil
}

// fenceRE finds a choir-theme fenced block. The fence may stand on its own
// lines or, where Texture projects a code block into one line of text, on one
// line; either way the body runs to the next closing fence.
var fenceRE = regexp.MustCompile("(?s)```" + Fence + "(?:[ \\t]|\\r?\\n)(.*?)```")

// block returns the text of the one choir-theme fenced block.
func block(text string) (string, error) {
	found := fenceRE.FindAllStringSubmatch(text, 3)
	if len(found) != 1 { // T1
		return "", fmt.Errorf("a theme document holds exactly one %s block (found %d)", Fence, len(found))
	}
	return found[0][1], nil
}

func cleanValue(path string, v any, depth int, budget *int) (any, error) {
	if depth > maxDepth { // T5
		return nil, fmt.Errorf("%s: nested too deep", path)
	}
	*budget--
	if *budget < 0 {
		return nil, fmt.Errorf("the theme has too many entries")
	}
	switch x := v.(type) {
	case string:
		if err := checkString(path, x); err != nil {
			return nil, err
		}
		return x, nil
	case json.Number:
		return x, nil
	case bool:
		return x, nil
	case []any:
		out := make([]any, 0, len(x))
		for i, e := range x {
			c, err := cleanValue(fmt.Sprintf("%s[%d]", path, i), e, depth+1, budget)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		return out, nil
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			if !safeRE.MatchString(k) {
				return nil, fmt.Errorf("%s: key %q is not allowed", path, clip(k))
			}
			c, err := cleanValue(path+"."+k, e, depth+1, budget)
			if err != nil {
				return nil, err
			}
			out[k] = c
		}
		return out, nil
	}
	return nil, fmt.Errorf("%s: value is not allowed", path)
}

// checkString holds one string to the restricted subset for its section.
func checkString(path, s string) error {
	low := strings.ToLower(s)
	for _, bad := range forbidden { // T2
		if strings.Contains(low, bad) {
			return fmt.Errorf("%s: %q is not allowed", path, bad)
		}
	}
	switch {
	case path == "id":
		if !idRE.MatchString(s) {
			return fmt.Errorf("id must be lower-case letters, digits and hyphens")
		}
	case path == "name":
		if !nameRE.MatchString(s) {
			return fmt.Errorf("name has characters that are not allowed")
		}
	case strings.HasPrefix(path, "schemes["):
		if s != "light" && s != "dark" {
			return fmt.Errorf("%s: a scheme is light or dark", path)
		}
	case strings.HasPrefix(path, "colors."):
		if _, ok := parseColor(s); !ok {
			return fmt.Errorf("%s: %q is not a hex or rgb(a) colour", path, clip(s))
		}
	case strings.HasPrefix(path, "radii."):
		if !pxRE.MatchString(s) {
			return fmt.Errorf("%s: %q is not a px length", path, clip(s))
		}
	case path == "type.read" || path == "type.ui" || path == "type.mono":
		if !stackRE.MatchString(s) {
			return fmt.Errorf("%s: font stack has characters that are not allowed", path)
		}
		first := strings.ToLower(strings.TrimSpace(strings.Split(s, ",")[0]))
		if !genericFonts[first] {
			return fmt.Errorf("%s: the first entry must be a generic family", path)
		}
	default:
		if !safeRE.MatchString(s) {
			return fmt.Errorf("%s: %q has characters that are not allowed", path, clip(s))
		}
	}
	return nil
}

func clip(s string) string {
	if len(s) > 40 {
		return s[:40]
	}
	return s
}

type rgba struct{ r, g, b, a float64 }

func parseColor(s string) (rgba, bool) {
	s = strings.TrimSpace(s)
	if hexRE.MatchString(s) {
		h := s[1:]
		if len(h) == 3 {
			h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
		}
		v := func(i int) float64 { n, _ := strconv.ParseUint(h[i:i+2], 16, 8); return float64(n) }
		c := rgba{v(0), v(2), v(4), 1}
		if len(h) == 8 {
			c.a = v(6) / 255
		}
		return c, true
	}
	if m := rgbRE.FindStringSubmatch(s); m != nil {
		var c rgba
		for i, p := range []*float64{&c.r, &c.g, &c.b} {
			n, _ := strconv.Atoi(m[i+1])
			if n > 255 {
				return rgba{}, false
			}
			*p = float64(n)
		}
		c.a = 1
		if m[4] != "" {
			c.a, _ = strconv.ParseFloat(m[4], 64)
		}
		return c, true
	}
	return rgba{}, false
}

// roleColor returns the colour of a role in a scheme: a bare string serves
// every scheme, an object serves the scheme it names.
func (t Theme) roleColor(role, scheme string) (rgba, bool) {
	colors, _ := t["colors"].(map[string]any)
	switch v := colors[role].(type) {
	case string:
		return parseColor(v)
	case map[string]any:
		s, _ := v[scheme].(string)
		return parseColor(s)
	}
	return rgba{}, false
}

func over(fg, bg rgba) rgba {
	return rgba{fg.r*fg.a + bg.r*(1-fg.a), fg.g*fg.a + bg.g*(1-fg.a), fg.b*fg.a + bg.b*(1-fg.a), 1}
}

func luminance(c rgba) float64 {
	f := func(v float64) float64 {
		v /= 255
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*f(c.r) + 0.7152*f(c.g) + 0.0722*f(c.b)
}

func contrast(a, b rgba) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// check holds the theme's required fields and the contrast gate (the web's
// pairs: fg and muted on paper 4.5, onAccent on accent 3) on every scheme.
func (t Theme) check() error {
	for _, k := range []string{"id", "name", "schemes", "colors"} {
		if _, ok := t[k]; !ok {
			return fmt.Errorf("the theme has no %s", k)
		}
	}
	schemes, _ := t["schemes"].([]any)
	if len(schemes) == 0 || len(schemes) > 2 {
		return fmt.Errorf("schemes lists light, dark or both")
	}
	pairs := []struct {
		fg, bg string
		min    float64
	}{{"fg", "paper", 4.5}, {"muted", "paper", 4.5}, {"onAccent", "accent", 3}}
	for _, sc := range schemes {
		scheme := sc.(string)
		for _, role := range []string{"paper", "fg", "muted", "accent", "onAccent"} { // T3
			if _, ok := t.roleColor(role, scheme); !ok {
				return fmt.Errorf("colors.%s has no %s value", role, scheme)
			}
		}
		paper, _ := t.roleColor("paper", scheme)
		for _, p := range pairs {
			fg, _ := t.roleColor(p.fg, scheme)
			bg, _ := t.roleColor(p.bg, scheme)
			if p.bg == "paper" {
				fg = over(fg, paper)
			}
			if r := contrast(fg, over(bg, paper)); r < p.min {
				return fmt.Errorf("%s scheme: %s on %s has contrast %.2f, below %.1f", scheme, p.fg, p.bg, r, p.min)
			}
		}
	}
	return nil
}
