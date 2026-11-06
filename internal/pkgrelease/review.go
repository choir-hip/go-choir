package pkgrelease

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
)

// Tiered review (slice L3; docs/design/modular-architecture-2026-11-04.md
// section 1, trust tiers, and Phase E): the mechanical review every release
// gets by its tier, at publish (the host, and the publisher's own draft) and
// again at install (the installing computer), before any model review of an
// extended ceiling (MA7.5) and before the owner's confirmation. The tier is
// the host's (apppackage.Manifest.Tier), never the publisher's.
//
//	T0 data (every release): the schema (ParseManifest), commands, menus and
//	   keybindings that name only the package's own declared ids
//	   (validateContributes), and each contributed theme file: the theme
//	   schema, only safe CSS values (no fetching function, no declaration
//	   break) and the contrast gate the shell applies (frontend/src/lib/theme.ts
//	   contributedThemeErrors), so a pack the shell would refuse is refused
//	   where it is published, not on each computer that installs it.
//	T1 code (a window): the release carries its contract fixture
//	   (choir-contract.json, the package contract runner's smoke contract)
//	   at publish, and at install the SDK names the computer's own build
//	   extracted imply no grant the manifest does not hold (CheckUses).
//	T2 shell (ui.shell: a provider of a shell surface): satisfiable only with
//	   a platform review record bound to the release's digest. The platform
//	   issues one only to the base image's releases, which are built in its
//	   own tree and never come through the Library, so every Library release
//	   of tier T2 is refused, with its own reason, whatever its origin claims.
//
// Failure modes:
//
//	R1 a theme value that fetches (url(), image-set(), @import, an escaped
//	   spelling of one) or breaks out of its declaration: refused
//	   (package_unsafe_value);
//	R2 a theme with unreadable contrast or text below the gate: refused
//	   (package_theme_contrast);
//	R3 a window app with no contract fixture is listed: refused at publish
//	   (package_contract_required); an already listed release is not
//	   re-judged at install for it (the contract is a listing requirement);
//	R4 a module that calls a shell-wide SDK function without ui.shell: the
//	   install is refused (package_uses_exceed_grants);
//	R5 a T2 release reaches the Library by claiming an origin: the check
//	   reads the provides and the grant, never the name or the origin
//	   (package_platform_review_required).

// ContractFile is the contract fixture a window package carries beside its
// manifest (frontend/scripts/package-contracts.mjs).
const ContractFile = "choir-contract.json"

// ContractSchema is the contract fixture's schema.
const ContractSchema = "choir.package-contract/v1"

// reviewPlatform refuses a T2 release (R5): it provides a shell surface or
// asks for ui.shell. It reads the raw manifest, before the manifest checks,
// so the refusal names the tier rather than the privileged grant.
func reviewPlatform(raw []byte) error {
	var probe struct {
		Provides     []json.RawMessage `json:"provides"`
		Capabilities []struct {
			Grant string `json:"grant"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil // the manifest checks name the problem
	}
	shell := len(probe.Provides) > 0
	for _, c := range probe.Capabilities {
		if c.Grant == apppackage.GrantUIShell {
			shell = true
		}
	}
	if shell {
		return refuse(ReasonPlatformReviewRequired, "the release is tier T2 (it draws part of Choir's own screen: ui.shell); T2 needs a platform review record bound to its digest, which the platform issues only to the base image's releases")
	}
	return nil
}

// reviewData is the T0 review every release gets beyond the manifest
// checks: each contributed theme file (R1, R2).
func reviewData(m apppackage.Manifest, files apppackage.Source) error {
	for _, t := range m.Contributes.Themes {
		raw, err := files.Read(t.File)
		if err != nil {
			return refuse(ReasonManifestInvalid, "contributes.themes: %s: %v", t.File, err)
		}
		if len(raw) > apppackage.MaxThemeFileBytes {
			return refuse(ReasonManifestInvalid, "contributes.themes: %s is over %d bytes", t.File, apppackage.MaxThemeFileBytes)
		}
		if err := ReviewTheme(t.ID, raw); err != nil {
			return err
		}
	}
	return nil
}

// reviewListing is the T1 listing requirement (R3): a window app carries its
// contract fixture.
func reviewListing(m apppackage.Manifest, files apppackage.Source) error {
	if m.App == nil {
		return nil
	}
	raw, err := files.Read(ContractFile)
	if err != nil {
		return refuse(ReasonContractRequired, "a window app is tier T1 and is listed with its contract fixture, %s, beside its manifest", ContractFile)
	}
	var c struct {
		Schema string `json:"schema"`
		Ready  string `json:"ready"`
	}
	if err := json.Unmarshal(raw, &c); err != nil || c.Schema != ContractSchema || strings.TrimSpace(c.Ready) == "" || utf8.RuneCountInString(c.Ready) > 512 {
		return refuse(ReasonContractRequired, "%s must be a %s contract with a ready selector", ContractFile, ContractSchema)
	}
	return nil
}

// CheckUses is the install-time T1 review (R4): every SDK name the
// computer's build extracted from the module that implies a grant
// (apppackage.SDKNameGrants) needs that grant among the capabilities of
// manifest, the manifest bytes the build validated.
func CheckUses(manifest json.RawMessage, uses []string) error {
	var needs []string
	for _, name := range uses {
		if _, ok := apppackage.SDKNameGrants[name]; ok {
			needs = append(needs, name)
		}
	}
	if len(needs) == 0 {
		return nil
	}
	var probe struct {
		Capabilities []struct {
			Grant string `json:"grant"`
		} `json:"capabilities"`
	}
	_ = json.Unmarshal(manifest, &probe) // unreadable: no grant is held
	held := map[string]bool{}
	for _, c := range probe.Capabilities {
		held[c.Grant] = true
	}
	var missing []string
	for _, name := range needs {
		if grant := apppackage.SDKNameGrants[name]; !held[grant] {
			missing = append(missing, name+" ("+grant+")")
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return refuse(ReasonUsesExceedGrants, "the module uses %s without the grant", strings.Join(missing, ", "))
	}
	return nil
}

// ---- themes ----

const themeSchemaVersion = 2

var (
	themeGroups       = []string{"colors", "radii", "motion", "fonts", "effects"}
	themeLayoutValues = []string{"promptSurfaceMinHeight", "deskSheetHeight"}
	// The contrast gate: text on its surface (theme.ts CONTRAST_PAIRS).
	contrastPairs = []struct {
		fg, bg string
		min    float64
	}{{"fg", "bg", 4.5}, {"fg", "panel", 4.5}, {"muted", "bg", 4.5}, {"onAccent", "accent", 3}, {"onDanger", "danger", 3}}
	cssHexEscape = regexp.MustCompile(`\\([0-9a-fA-F]{1,6})\s?`)
	cssEscape    = regexp.MustCompile(`\\(.)`)
	cssComment   = regexp.MustCompile(`/\*[\s\S]*?\*/`)
	cssBreak     = regexp.MustCompile("[;{}\x00-\x1f]")
	cssFetching  = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_-])(?:` + strings.Join(append(quoted(CSSFetchFunctions), "expression"), "|") + `)\s*\(|@import`)
	hexColor     = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	rgbColor     = regexp.MustCompile(`^(?i)rgba?\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)`)
)

// CSSFetchFunctions are the CSS functions that take a URL and so fetch: a
// copy of frontend/scripts/css-fetch-functions.json, which the shell's theme
// gate and the package CSS gate read (a test keeps the two equal).
var CSSFetchFunctions = []string{"url", "image-set", "-webkit-image-set", "image", "cross-fade", "-webkit-cross-fade", "src", "element", "paint"}

func quoted(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = regexp.QuoteMeta(n)
	}
	return out
}

func decodeCSSEscapes(v string) string {
	v = cssHexEscape.ReplaceAllStringFunc(v, func(m string) string {
		hex := cssHexEscape.FindStringSubmatch(m)[1]
		cp, err := strconv.ParseUint(hex, 16, 32)
		if err != nil || cp == 0 || cp > 0x10ffff {
			return ""
		}
		return string(rune(cp))
	})
	v = cssEscape.ReplaceAllString(v, "$1")
	return cssComment.ReplaceAllString(v, "")
}

// SafeCSSValue is the shell's isSafeCSSValue: a non-empty string of at most
// 512 characters that cannot end its declaration and calls nothing that
// fetches, escapes decoded.
func SafeCSSValue(v any) bool {
	s, ok := v.(string)
	if !ok || s == "" || utf8.RuneCountInString(s) > 512 {
		return false
	}
	return !cssBreak.MatchString(s) && !cssFetching.MatchString(decodeCSSEscapes(s))
}

func channels(v string) ([3]float64, bool) {
	v = strings.TrimSpace(v)
	if m := hexColor.FindStringSubmatch(v); m != nil {
		h := m[1]
		if len(h) == 3 {
			h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
		}
		var out [3]float64
		for i := 0; i < 3; i++ {
			n, _ := strconv.ParseUint(h[2*i:2*i+2], 16, 8)
			out[i] = float64(n)
		}
		return out, true
	}
	if m := rgbColor.FindStringSubmatch(v); m != nil {
		var out [3]float64
		for i := 0; i < 3; i++ {
			n, err := strconv.ParseFloat(m[i+1], 64)
			if err != nil {
				return out, false
			}
			out[i] = n
		}
		return out, true
	}
	return [3]float64{}, false
}

func luminance(c [3]float64) float64 {
	lin := func(v float64) float64 {
		x := v / 255
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c[0]) + 0.7152*lin(c[1]) + 0.0722*lin(c[2])
}

// ContrastRatio is the WCAG contrast ratio of two hex or rgb colors; ok is
// false when either cannot be read.
func ContrastRatio(a, b string) (float64, bool) {
	ca, okA := channels(a)
	cb, okB := channels(b)
	if !okA || !okB {
		return 0, false
	}
	la, lb := luminance(ca), luminance(cb)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05), true
}

// ReviewTheme is the T0 review of one contributed theme file declared as id:
// the theme schema, safe values only (R1) and the contrast gate (R2). The
// shell's own gate (contributedThemeErrors) still runs when the theme
// applies, and also requires every token key; this review refuses what is
// unsafe or unreadable before the release is listed or installed.
func ReviewTheme(id string, raw []byte) error {
	var theme map[string]any
	if err := json.Unmarshal(raw, &theme); err != nil || theme == nil {
		return refuse(ReasonManifestInvalid, "theme %s is not a JSON object", id)
	}
	if v, ok := theme["schema_version"].(float64); !ok || v != themeSchemaVersion {
		return refuse(ReasonManifestInvalid, "theme %s: schema_version must be %d", id, themeSchemaVersion)
	}
	if theme["id"] != id {
		return refuse(ReasonManifestInvalid, "theme file for %s has id %v", id, theme["id"])
	}
	if name, ok := theme["name"].(string); !ok || strings.TrimSpace(name) == "" || utf8.RuneCountInString(name) > 80 {
		return refuse(ReasonManifestInvalid, "theme %s: name must be a non-empty string of at most 80 characters", id)
	}
	for _, g := range themeGroups {
		group, ok := theme[g].(map[string]any)
		if !ok {
			return refuse(ReasonManifestInvalid, "theme %s: %s must be an object", id, g)
		}
		keys := make([]string, 0, len(group))
		for k := range group {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if !SafeCSSValue(group[k]) {
				return refuse(ReasonUnsafeValue, "theme %s: %s.%s is not a safe CSS value", id, g, k)
			}
		}
	}
	layout, ok := theme["layout"].(map[string]any)
	if !ok {
		return refuse(ReasonManifestInvalid, "theme %s: layout must be an object", id)
	}
	if p := layout["promptSurfacePlacement"]; p != "top" && p != "bottom" {
		return refuse(ReasonManifestInvalid, "theme %s: layout.promptSurfacePlacement must be top or bottom", id)
	}
	for _, k := range themeLayoutValues {
		if !SafeCSSValue(layout[k]) {
			return refuse(ReasonUnsafeValue, "theme %s: layout.%s is not a safe CSS value", id, k)
		}
	}
	colors := theme["colors"].(map[string]any)
	for _, p := range contrastPairs {
		fg, _ := colors[p.fg].(string)
		bg, _ := colors[p.bg].(string)
		ratio, ok := ContrastRatio(fg, bg)
		if !ok {
			return refuse(ReasonThemeContrast, "theme %s: colors.%s and colors.%s must be hex or rgb colors so contrast can be checked", id, p.fg, p.bg)
		}
		if ratio < p.min {
			return refuse(ReasonThemeContrast, "theme %s: colors.%s on colors.%s has contrast %s, below %s", id, p.fg, p.bg, fmt.Sprintf("%.2f", ratio), strconv.FormatFloat(p.min, 'f', -1, 64))
		}
	}
	return nil
}
