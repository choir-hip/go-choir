package platform

import (
	"encoding/json"
	"html"
	"strings"
)

func publicationSourceManifestJSON(manifest publicationSourceManifest) string {
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func safeScriptJSON(value string) string {
	value = strings.ReplaceAll(value, "</", `<\/`)
	value = strings.ReplaceAll(value, "<!--", `<\!--`)
	return value
}

func xmlEscape(s string) string {
	return html.EscapeString(s)
}

func pdfEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "(", `\(`)
	s = strings.ReplaceAll(s, ")", `\)`)
	return s
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
