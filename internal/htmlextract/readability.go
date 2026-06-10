package htmlextract

import (
	"bytes"
	"html"
	"regexp"
	"strings"
	"unicode"

	xhtml "golang.org/x/net/html"
)

// ExtractReadableHTML returns document title and best-effort article text.
func ExtractReadableHTML(data []byte) (string, string) {
	doc, err := xhtml.Parse(bytes.NewReader(data))
	if err != nil {
		return extractReadableHTMLFallback(data)
	}
	title := htmlTextTitle(doc)
	candidates := readableHTMLCandidates(doc)
	best := ""
	bestScore := 0
	for _, candidate := range candidates {
		text := cleanReadableText(extractHTMLNodeText(candidate))
		score := readableTextScore(text)
		if score > bestScore {
			best = text
			bestScore = score
		}
	}
	return strings.TrimSpace(title), strings.TrimSpace(best)
}

func extractReadableHTMLFallback(data []byte) (string, string) {
	source := string(data)
	title := ""
	if matches := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`).FindStringSubmatch(source); len(matches) > 1 {
		title = htmlEntityDecode(stripHTMLTags(matches[1]))
	}
	cleaned := source
	for _, tag := range []string{"script", "style", "noscript", "svg"} {
		cleaned = regexp.MustCompile(`(?is)<`+tag+`[^>]*>.*?</`+tag+`>`).ReplaceAllString(cleaned, " ")
	}
	cleaned = regexp.MustCompile(`(?is)<br\s*/?>|</p>|</div>|</section>|</article>|</h[1-6]>|</li>`).ReplaceAllString(cleaned, "\n")
	text := htmlEntityDecode(stripHTMLTags(cleaned))
	return strings.TrimSpace(title), cleanReadableText(text)
}

func cleanReadableText(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if normalized := collapseWhitespace(line); normalized != "" {
			if readerNoiseLine(normalized) {
				continue
			}
			out = append(out, normalized)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func htmlTextTitle(node *xhtml.Node) string {
	if node == nil {
		return ""
	}
	if node.Type == xhtml.ElementNode && strings.EqualFold(node.Data, "title") {
		return collapseWhitespace(extractHTMLNodeText(node))
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if title := htmlTextTitle(child); title != "" {
			return title
		}
	}
	return ""
}

func readableHTMLCandidates(doc *xhtml.Node) []*xhtml.Node {
	var preferred []*xhtml.Node
	var body *xhtml.Node
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node == nil {
			return
		}
		if node.Type == xhtml.ElementNode {
			tag := strings.ToLower(node.Data)
			if tag == "article" || tag == "main" {
				preferred = append(preferred, node)
			}
			if tag == "body" && body == nil {
				body = node
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if len(preferred) > 0 {
		return preferred
	}
	if body != nil {
		return []*xhtml.Node{body}
	}
	return []*xhtml.Node{doc}
}

func extractHTMLNodeText(node *xhtml.Node) string {
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n == nil || skipReaderHTMLNode(n) {
			return
		}
		if n.Type == xhtml.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
			return
		}
		block := n.Type == xhtml.ElementNode && readerHTMLBlockTag(n.Data)
		if block {
			b.WriteByte('\n')
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if block {
			b.WriteByte('\n')
		}
	}
	walk(node)
	return htmlEntityDecode(b.String())
}

func skipReaderHTMLNode(node *xhtml.Node) bool {
	if node.Type != xhtml.ElementNode {
		return false
	}
	tag := strings.ToLower(node.Data)
	switch tag {
	case "script", "style", "noscript", "svg", "canvas", "nav", "header", "footer", "aside", "form", "select", "option", "button", "input", "textarea", "dialog":
		return true
	}
	for _, attr := range node.Attr {
		key := strings.ToLower(strings.TrimSpace(attr.Key))
		value := strings.ToLower(strings.TrimSpace(attr.Val))
		if key == "role" {
			switch value {
			case "banner", "navigation", "search", "dialog", "menu", "menubar", "toolbar", "complementary", "contentinfo":
				return true
			}
		}
		if key == "class" || key == "id" || key == "aria-label" {
			if readerHTMLNoiseAttribute(value) {
				return true
			}
		}
	}
	return false
}

func readerHTMLNoiseAttribute(value string) bool {
	for _, token := range []string{
		"cookie", "consent", "cmp", "gdpr-banner", "privacy-banner",
		"breadcrumb", "sidebar", "newsletter",
	} {
		if strings.Contains(value, token) {
			return true
		}
	}
	for _, token := range readerHTMLAttributeTokens(value) {
		switch token {
		case "modal", "popup", "overlay", "nav", "navbar", "navigation", "menu", "footer", "header", "search", "login", "toolbar", "widget":
			return true
		}
	}
	return false
}

func readerHTMLAttributeTokens(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
}

func readerHTMLBlockTag(tag string) bool {
	switch strings.ToLower(tag) {
	case "article", "main", "section", "div", "p", "br", "li", "ul", "ol", "blockquote", "pre", "table", "tr", "h1", "h2", "h3", "h4", "h5", "h6":
		return true
	default:
		return false
	}
}

func readerNoiseLine(line string) bool {
	normalized := strings.ToLower(strings.TrimSpace(line))
	if normalized == "" {
		return true
	}
	for _, exact := range []string{
		"allow all cookies",
		"allow necessary cookies",
		"choose your location settings",
		"save settings",
		"language",
		"currency",
		"country (vat)",
		"menu and widgets",
		"skip to content",
		"login",
	} {
		if normalized == exact {
			return true
		}
	}
	for _, needle := range []string{
		"this website uses cookies",
		"cookie settings",
		"we use cookies",
		"we noticed you're browsing",
		"switch to settings tailored",
		"search for:",
	} {
		if strings.Contains(normalized, needle) {
			return true
		}
	}
	return false
}

func readableTextScore(text string) int {
	score := 0
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) >= 40 {
			score += len(trimmed)
		}
	}
	if score == 0 {
		score = len(strings.TrimSpace(text))
	}
	return score
}

func collapseWhitespace(s string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, s))
}

func stripHTMLTags(source string) string {
	return regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(source, " ")
}

func htmlEntityDecode(source string) string {
	return html.UnescapeString(source)
}
