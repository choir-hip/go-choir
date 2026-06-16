package platform

import (
	"encoding/json"
	"fmt"
	"html"
	"strconv"
	"strings"
)

func renderPublicationHTML(doc PublicationDocument, profile publicationExportProfile) string {
	manifestJSON := publicationSourceManifestJSON(doc.Manifest)
	ordinals := publicationSourceOrdinals(doc)
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html><head><meta charset=\"utf-8\"><title>")
	b.WriteString(html.EscapeString(doc.Title))
	b.WriteString(`</title><meta name="generator" content="Choir">`)
	b.WriteString(`<meta name="choir-export-profile" content="`)
	b.WriteString(html.EscapeString(profile.ID))
	b.WriteString(`"><script type="application/json" id="choir-export-profile">`)
	b.WriteString(safeScriptJSON(publicationExportProfileJSON(profile)))
	b.WriteString(`</script><style>`)
	b.WriteString(publicationHTMLProfileCSS(profile))
	b.WriteString(`</style>`)
	b.WriteString(`<script type="application/ld+json">`)
	b.WriteString(safeScriptJSON(publicationJSONLD(doc)))
	b.WriteString(`</script><script type="application/json" id="choir-source-manifest">`)
	b.WriteString(safeScriptJSON(manifestJSON))
	b.WriteString(`</script></head><body><article class="texture-publication">`)
	for _, block := range doc.Blocks {
		switch block.Kind {
		case "heading":
			level := clampInt(block.Level, 1, 6)
			b.WriteString("<h")
			b.WriteString(strconv.Itoa(level))
			b.WriteString(">")
			b.WriteString(renderHTMLInlines(block.Inlines, ordinals))
			b.WriteString("</h")
			b.WriteString(strconv.Itoa(level))
			b.WriteString(">\n")
		case "paragraph":
			b.WriteString("<p>")
			b.WriteString(renderHTMLInlines(block.Inlines, ordinals))
			b.WriteString("</p>\n")
		case "list_item":
			b.WriteString("<ul><li>")
			b.WriteString(renderHTMLInlines(block.Inlines, ordinals))
			b.WriteString("</li></ul>\n")
		case "table":
			b.WriteString(`<table class="texture-table"><tbody>`)
			for _, row := range block.Rows {
				b.WriteString("<tr>")
				for _, cell := range row {
					tag := "td"
					if cell.Header {
						tag = "th"
					}
					b.WriteString("<")
					b.WriteString(tag)
					b.WriteString(">")
					b.WriteString(renderHTMLInlines(cell.Inlines, ordinals))
					b.WriteString("</")
					b.WriteString(tag)
					b.WriteString(">")
				}
				b.WriteString("</tr>")
			}
			b.WriteString("</tbody></table>\n")
		case "rule":
			b.WriteString("<hr>\n")
		}
	}
	if len(doc.Manifest.Sources) > 0 {
		b.WriteString(`<section class="texture-sources" aria-labelledby="texture-sources-heading"><h2 id="texture-sources-heading">Sources</h2><ol>`)
		for _, source := range doc.Manifest.Sources {
			b.WriteString(`<li id="source-`)
			b.WriteString(html.EscapeString(source.SourceEntityID))
			b.WriteString(`"><span class="source-title">`)
			b.WriteString(html.EscapeString(firstNonEmpty(source.Title, source.SourceEntityID)))
			b.WriteString(`</span>`)
			if source.URL != "" {
				b.WriteString(` <a href="`)
				b.WriteString(html.EscapeString(source.URL))
				b.WriteString(`">`)
				b.WriteString(html.EscapeString(source.URL))
				b.WriteString(`</a>`)
			}
			if source.SnapshotText != "" {
				b.WriteString(`<blockquote>`)
				b.WriteString(html.EscapeString(source.SnapshotText))
				b.WriteString(`</blockquote>`)
			}
			b.WriteString(`</li>`)
		}
		b.WriteString(`</ol></section>`)
	}
	b.WriteString("</article></body></html>\n")
	return b.String()
}

func renderHTMLInlines(inlines []publicationInline, ordinals map[string]int) string {
	var b strings.Builder
	for _, inline := range inlines {
		switch inline.Kind {
		case "strong":
			b.WriteString("<strong>")
			b.WriteString(html.EscapeString(inline.Text))
			b.WriteString("</strong>")
		case "em":
			b.WriteString("<em>")
			b.WriteString(html.EscapeString(inline.Text))
			b.WriteString("</em>")
		case "link":
			b.WriteString(`<a href="`)
			b.WriteString(html.EscapeString(inline.Href))
			b.WriteString(`">`)
			b.WriteString(html.EscapeString(inline.Text))
			b.WriteString(`</a>`)
		case "source_ref":
			b.WriteString(`<a class="texture-source-ref" href="#source-`)
			b.WriteString(html.EscapeString(inline.SourceID))
			b.WriteString(`" data-source-id="`)
			b.WriteString(html.EscapeString(inline.SourceID))
			b.WriteString(`">`)
			b.WriteString(html.EscapeString(inline.Text))
			b.WriteString(`<sup>[`)
			b.WriteString(html.EscapeString(publicationSourceMarker(ordinals, inline.SourceID)))
			b.WriteString(`]</sup></a>`)
		default:
			b.WriteString(html.EscapeString(inline.Text))
		}
	}
	return b.String()
}

func publicationHTMLProfileCSS(profile publicationExportProfile) string {
	fontStack := firstNonEmpty(profile.Typography.FontStack, `-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif`)
	bodySize := clampInt(profile.Typography.BodySizePX, 12, 24)
	lineHeight := float64(clampInt(profile.Typography.LineHeightPct, 120, 200)) / 100
	maxWidth := clampInt(profile.Typography.MaxWidthPX, 560, 1100)
	titleSize := clampInt(firstNonZero(profile.Headings.TitleSizePX, profile.Headings.H1SizePX, 30), 22, 54)
	h2Size := clampInt(firstNonZero(profile.Headings.H2SizePX, 22), 16, 42)
	h3Size := clampInt(firstNonZero(profile.Headings.H3SizePX, 18), 14, 34)
	borderColor := firstNonEmpty(profile.Table.BorderColor, "#d6dbe3")
	headerFill := firstNonEmpty(profile.Table.HeaderFill, "#f1f4f8")
	return fmt.Sprintf(`:root{color-scheme:light;--choir-text:#171717;--choir-muted:#5f6673;--choir-rule:%s;--choir-accent:#1d4f91;--choir-source-bg:#f7f9fc;--choir-table-head:%s}`+
		`html{background:#f4f5f7}`+
		`body{margin:0;color:var(--choir-text);font:%dpx/%.2f %s}`+
		`.texture-publication{box-sizing:border-box;max-width:%dpx;margin:42px auto;padding:56px 64px;background:#fff;box-shadow:0 1px 8px rgba(15,23,42,.08)}`+
		`h1{font-size:%dpx;line-height:1.18;margin:0 0 24px;font-weight:750;letter-spacing:0}`+
		`h2{font-size:%dpx;line-height:1.28;margin:34px 0 14px;font-weight:720;letter-spacing:0}`+
		`h3{font-size:%dpx;line-height:1.35;margin:28px 0 10px;font-weight:700;letter-spacing:0}`+
		`h4,h5,h6{font-size:16px;line-height:1.4;margin:24px 0 8px;font-weight:700;letter-spacing:0}`+
		`p{margin:0 0 16px}`+
		`ul{margin:0 0 16px 1.25rem;padding:0}`+
		`li{margin:0 0 8px}`+
		`.texture-table{border-collapse:collapse;width:100%%;margin:22px 0 28px;font-size:14px;line-height:1.45}`+
		`.texture-table th,.texture-table td{border:1px solid var(--choir-rule);padding:9px 11px;vertical-align:top;text-align:left}`+
		`.texture-table th{background:var(--choir-table-head);font-weight:700}`+
		`.texture-source-ref{color:var(--choir-accent);text-decoration:none;border-bottom:1px solid rgba(29,79,145,.28)}`+
		`.texture-source-ref sup{font-size:.72em;line-height:0;margin-left:2px}`+
		`.texture-sources{margin-top:44px;padding-top:22px;border-top:1px solid var(--choir-rule);font-size:14px;color:#2f3744}`+
		`.texture-sources h2{font-size:18px;margin:0 0 14px}`+
		`.texture-sources ol{padding-left:1.35rem}`+
		`.texture-sources li{margin-bottom:14px}`+
		`.texture-sources blockquote{margin:8px 0 0;padding:10px 12px;background:var(--choir-source-bg);border-left:3px solid var(--choir-rule);color:var(--choir-muted)}`+
		`@media(max-width:760px){.texture-publication{margin:0;padding:32px 22px;box-shadow:none}h1{font-size:26px}}`,
		borderColor, headerFill, bodySize, lineHeight, fontStack, maxWidth, titleSize, h2Size, h3Size)
}

func publicationJSONLD(doc PublicationDocument) string {
	citations := make([]map[string]string, 0, len(doc.Manifest.Sources))
	for _, source := range doc.Manifest.Sources {
		citation := map[string]string{
			"@type":      "CreativeWork",
			"identifier": source.SourceEntityID,
			"name":       firstNonEmpty(source.Title, source.SourceEntityID),
		}
		if source.URL != "" {
			citation["url"] = source.URL
		}
		citations = append(citations, citation)
	}
	raw, err := json.Marshal(map[string]any{
		"@context": "https://schema.org",
		"@type":    "CreativeWork",
		"name":     doc.Title,
		"identifier": map[string]string{
			"publication_id":         doc.Manifest.PublicationID,
			"publication_version_id": doc.Manifest.PublicationVersionID,
		},
		"citation": citations,
	})
	if err != nil {
		return "{}"
	}
	return string(raw)
}
