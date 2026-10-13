package sourcefetch

import (
	"bytes"
	"html"
	"regexp"
	"strings"

	xhtml "golang.org/x/net/html"
)

// ExtractReadableHTML returns the document title and its readable text: the
// projection of ExtractBlocks, block texts separated by blank lines.
func ExtractReadableHTML(data []byte) (string, string) {
	doc := ExtractBlocks(data)
	return doc.Title, doc.Projection()
}

// ExtractBlocks walks an HTML page into typed blocks (headings with their
// level, paragraphs, list items, table captions, header rows and rows,
// figure captions, notes and code), dropping navigation, chrome, ads and
// consent banners. Inline markup (links, emphasis, <br>, footnote markers
// such as "[12]") stays inside its block, and links are kept as marks. Of
// the page's <article>/<main> candidates (or its <body>), the one with the
// most readable text wins. Block ids are assigned with AssignIDs.
func ExtractBlocks(data []byte) Document {
	doc, err := xhtml.Parse(bytes.NewReader(data))
	if err != nil {
		title, text := extractReadableHTMLFallback(data)
		return Document{Extractor: ExtractorVersion, Title: title, Blocks: TextBlocks(text)}
	}
	out := Document{Extractor: ExtractorVersion, Title: htmlTextTitle(doc)}
	bestScore := 0
	for _, candidate := range readableHTMLCandidates(doc) {
		w := &blockWalker{kind: KindParagraph}
		w.walk(candidate, walkContext{inArticle: hasAncestor(candidate, "article")})
		w.flush()
		score := readableTextScore(w.blocks)
		if score > bestScore {
			out.Blocks, bestScore = w.blocks, score
		}
	}
	if out.Blocks == nil {
		out.Blocks = []Block{}
	}
	AssignIDs(out.Blocks)
	return out
}

func extractReadableHTMLFallback(data []byte) (string, string) {
	source := string(data)
	title := ""
	if matches := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`).FindStringSubmatch(source); len(matches) > 1 {
		title = collapseWhitespace(html.UnescapeString(stripHTMLTags(matches[1])))
	}
	cleaned := source
	for _, tag := range []string{"script", "style", "noscript", "svg"} {
		cleaned = regexp.MustCompile(`(?is)<`+tag+`[^>]*>.*?</`+tag+`>`).ReplaceAllString(cleaned, " ")
	}
	cleaned = regexp.MustCompile(`(?is)<br\s*/?>|</p>|</div>|</section>|</article>|</h[1-6]>|</li>`).ReplaceAllString(cleaned, "\n")
	text := html.UnescapeString(stripHTMLTags(cleaned))
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if normalized := collapseWhitespace(line); normalized != "" && !readerNoiseLine(normalized) {
			out = append(out, normalized)
		}
	}
	return title, strings.Join(out, "\n")
}

// walkContext is what a node inherits from its ancestors.
type walkContext struct {
	inArticle bool // the <header> of an article holds its headline, not site chrome
	notes     bool // list items here are footnotes or references
}

// blockWalker emits blocks in document order. Inline content accumulates in
// cur under the innermost block element's kind; entering or leaving any
// block-level element flushes it as one block.
type blockWalker struct {
	blocks []Block
	cur    inlineText
	kind   string
	level  int
}

func (w *blockWalker) flush() {
	text := w.cur.String()
	links := w.cur.links
	w.cur.reset()
	if text == "" || readerNoiseLine(text) {
		return
	}
	b := Block{Kind: w.kind, Text: text, Links: links}
	if w.kind == KindHeading {
		b.Level = w.level
	}
	w.blocks = append(w.blocks, b)
}

func (w *blockWalker) walk(n *xhtml.Node, ctx walkContext) {
	switch n.Type {
	case xhtml.TextNode:
		w.cur.text(n.Data)
		return
	case xhtml.ElementNode:
	default:
		w.children(n, ctx)
		return
	}
	if skipBlockNode(n, ctx) {
		return
	}
	tag := strings.ToLower(n.Data)
	switch tag {
	case "article":
		ctx.inArticle = true
		w.container(n, ctx)
	case "h1", "h2", "h3", "h4", "h5", "h6":
		w.block(n, ctx, KindHeading, int(tag[1]-'0'))
	case "p", "dt", "dd":
		w.block(n, ctx, w.inheritedKind(), 0)
	case "li":
		kind := KindListItem
		if ctx.notes || noteListItem(n) {
			kind = KindNote
		}
		w.block(n, ctx, kind, 0)
	case "figcaption":
		w.block(n, ctx, KindFigureCaption, 0)
	case "caption":
		w.block(n, ctx, KindTableCaption, 0)
	case "pre":
		w.block(n, ctx, KindCode, 0)
	case "ol", "ul":
		if noteList(n) {
			ctx.notes = true
		}
		w.container(n, ctx)
	case "table":
		w.flush()
		if layoutTable(n) {
			w.container(n, ctx)
		} else {
			w.table(n, ctx)
		}
	case "br":
		w.cur.breakSpace()
	case "a":
		w.cur.openLink(linkHref(n))
		w.children(n, ctx)
		w.cur.closeLink()
	default:
		if blockLevelTag(tag) {
			w.container(n, ctx)
		} else {
			w.children(n, ctx)
		}
	}
}

func (w *blockWalker) children(n *xhtml.Node, ctx walkContext) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		w.walk(c, ctx)
	}
}

// container is a block-level element with no kind of its own (div,
// section, blockquote, figure, a layout table's cells): loose text inside it
// keeps the enclosing kind, and its edges end the current block.
func (w *blockWalker) container(n *xhtml.Node, ctx walkContext) {
	w.flush()
	w.children(n, ctx)
	w.flush()
}

func (w *blockWalker) block(n *xhtml.Node, ctx walkContext, kind string, level int) {
	w.flush()
	prevKind, prevLevel := w.kind, w.level
	w.kind, w.level = kind, level
	w.children(n, ctx)
	w.flush()
	w.kind, w.level = prevKind, prevLevel
}

// inheritedKind keeps a paragraph inside a list item, note, caption or code
// block in that kind; elsewhere it is a paragraph.
func (w *blockWalker) inheritedKind() string {
	switch w.kind {
	case KindListItem, KindNote, KindFigureCaption, KindTableCaption, KindCode:
		return w.kind
	}
	return KindParagraph
}

// table emits a data table as rows: the caption is its own block, the
// header row (in <thead>, or all <th>) is a table_header block, and each
// other row is a table_row block whose text joins its trimmed cells with
// " | " (Cut 2 amendment A3). Nested tables are read as part of their cell.
func (w *blockWalker) table(n *xhtml.Node, ctx walkContext) {
	var visit func(*xhtml.Node, bool)
	visit = func(node *xhtml.Node, inHead bool) {
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != xhtml.ElementNode || skipBlockNode(c, ctx) {
				continue
			}
			switch strings.ToLower(c.Data) {
			case "caption":
				w.block(c, ctx, KindTableCaption, 0)
			case "thead":
				visit(c, true)
			case "tbody", "tfoot":
				visit(c, false)
			case "tr":
				w.row(c, ctx, inHead)
			}
		}
	}
	visit(n, false)
}

func (w *blockWalker) row(tr *xhtml.Node, ctx walkContext, inHead bool) {
	var row inlineText
	cells, headerCells := 0, 0
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != xhtml.ElementNode {
			continue
		}
		tag := strings.ToLower(c.Data)
		if tag != "td" && tag != "th" {
			continue
		}
		if tag == "th" {
			headerCells++
		}
		var cell inlineText
		if !skipBlockNode(c, ctx) {
			inlineWalk(c, &cell, ctx)
		}
		if cells > 0 {
			row.b.WriteString(" | ")
		}
		offset := row.b.Len()
		row.b.WriteString(cell.String())
		for _, l := range cell.links {
			row.links = append(row.links, Link{Start: l.Start + offset, End: l.End + offset, Href: l.Href})
		}
		cells++
	}
	text := strings.TrimSpace(strings.ReplaceAll(row.String(), "|", ""))
	if cells == 0 || text == "" {
		return
	}
	kind := KindTableRow
	if inHead || headerCells == cells {
		kind = KindTableHeader
	}
	w.blocks = append(w.blocks, Block{Kind: kind, Text: row.String(), Links: row.links})
}

// inlineWalk reads a table cell's content as inline text: block-level
// boundaries inside it read as spaces.
func inlineWalk(n *xhtml.Node, t *inlineText, ctx walkContext) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case xhtml.TextNode:
			t.text(c.Data)
			continue
		case xhtml.ElementNode:
		default:
			continue
		}
		if skipBlockNode(c, ctx) {
			continue
		}
		tag := strings.ToLower(c.Data)
		switch {
		case tag == "br":
			t.breakSpace()
		case tag == "a":
			t.openLink(linkHref(c))
			inlineWalk(c, t, ctx)
			t.closeLink()
		case blockLevelTag(tag) || tag == "li" || tag == "p" || tag == "table" || tag == "tr" || tag == "td" || tag == "th":
			t.breakSpace()
			inlineWalk(c, t, ctx)
			t.breakSpace()
		default:
			inlineWalk(c, t, ctx)
		}
	}
}

// layoutTable reports a table used for page layout rather than data: it is
// marked presentational, or a cell holds paragraphs, headings or a table.
func layoutTable(n *xhtml.Node) bool {
	for _, attr := range n.Attr {
		if strings.EqualFold(attr.Key, "role") && strings.EqualFold(strings.TrimSpace(attr.Val), "presentation") {
			return true
		}
	}
	var cellHolds func(*xhtml.Node, bool) bool
	cellHolds = func(node *xhtml.Node, inCell bool) bool {
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != xhtml.ElementNode {
				continue
			}
			tag := strings.ToLower(c.Data)
			if inCell {
				switch tag {
				case "p", "table", "h1", "h2", "h3", "h4", "h5", "h6", "article", "section":
					return true
				}
			}
			if tag == "table" {
				continue // a nested table's own cells are its business
			}
			if cellHolds(c, inCell || tag == "td" || tag == "th") {
				return true
			}
		}
		return false
	}
	return cellHolds(n, false)
}

func linkHref(n *xhtml.Node) string {
	for _, attr := range n.Attr {
		if strings.EqualFold(attr.Key, "href") {
			href := strings.TrimSpace(attr.Val)
			if strings.HasPrefix(strings.ToLower(href), "javascript:") {
				return ""
			}
			return href
		}
	}
	return ""
}

// noteList reports a list of footnotes or references.
func noteList(n *xhtml.Node) bool {
	for _, attr := range n.Attr {
		key := strings.ToLower(attr.Key)
		if key != "class" && key != "id" && key != "role" {
			continue
		}
		value := strings.ToLower(attr.Val)
		for _, token := range []string{"references", "footnotes", "endnotes", "doc-endnotes", "doc-footnotes"} {
			if strings.Contains(value, token) {
				return true
			}
		}
	}
	return false
}

// noteListItem reports a footnote list item outside a recognised notes list
// (Wikipedia's cite_note ids, role="doc-footnote").
func noteListItem(n *xhtml.Node) bool {
	for _, attr := range n.Attr {
		key, value := strings.ToLower(attr.Key), strings.ToLower(attr.Val)
		if key == "id" && (strings.HasPrefix(value, "cite_note") || strings.HasPrefix(value, "fn")) {
			return true
		}
		if key == "role" && (value == "doc-footnote" || value == "doc-endnote") {
			return true
		}
	}
	return false
}

func hasAncestor(n *xhtml.Node, tag string) bool {
	for p := n; p != nil; p = p.Parent {
		if p.Type == xhtml.ElementNode && strings.EqualFold(p.Data, tag) {
			return true
		}
	}
	return false
}

// blockLevelTag reports elements whose edges end a block. Elements not
// listed (span, em, strong, sup, code, ...) are inline.
func blockLevelTag(tag string) bool {
	switch tag {
	case "html", "body", "main", "article", "section", "div", "blockquote", "figure", "header", "footer",
		"address", "details", "summary", "dl", "center", "hgroup", "fieldset", "tbody", "thead", "tfoot",
		"tr", "td", "th", "ol", "ul", "picture", "video", "audio":
		return true
	}
	return false
}

func htmlTextTitle(node *xhtml.Node) string {
	if node == nil {
		return ""
	}
	if node.Type == xhtml.ElementNode && strings.EqualFold(node.Data, "title") {
		var t inlineText
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == xhtml.TextNode {
				t.text(c.Data)
			}
		}
		return t.String()
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

// skipBlockNode drops chrome: scripts and styles, navigation, site headers
// and footers, asides, forms, hidden elements, and elements whose role,
// class, id or label names navigation, consent banners, ads or similar.
// An article's own <header> is kept: it holds the headline and byline.
func skipBlockNode(node *xhtml.Node, ctx walkContext) bool {
	if node.Type != xhtml.ElementNode {
		return false
	}
	tag := strings.ToLower(node.Data)
	switch tag {
	case "script", "style", "noscript", "svg", "canvas", "nav", "footer", "aside", "form", "select", "option", "button", "input", "textarea", "dialog", "template", "iframe":
		return true
	case "header":
		if !ctx.inArticle {
			return true
		}
	}
	for _, attr := range node.Attr {
		key := strings.ToLower(strings.TrimSpace(attr.Key))
		value := strings.ToLower(strings.TrimSpace(attr.Val))
		switch key {
		case "hidden":
			return true
		case "aria-hidden":
			if value == "true" {
				return true
			}
		case "role":
			switch value {
			case "banner", "navigation", "search", "dialog", "menu", "menubar", "toolbar", "complementary", "contentinfo":
				return true
			}
		case "class", "id", "aria-label":
			if readerHTMLNoiseAttribute(value, ctx.inArticle) {
				return true
			}
		}
	}
	return false
}

// readerHTMLNoiseAttribute reports a class, id or label naming chrome.
// Inside an article, "header" names the article's own header (headline and
// byline), not the site's.
func readerHTMLNoiseAttribute(value string, inArticle bool) bool {
	for _, token := range []string{
		"cookie", "consent", "cmp", "gdpr-banner", "privacy-banner",
		"breadcrumb", "sidebar", "newsletter", "advert", "editsection",
	} {
		if strings.Contains(value, token) {
			return true
		}
	}
	for _, token := range readerHTMLAttributeTokens(value) {
		switch token {
		case "modal", "popup", "overlay", "nav", "navbar", "navigation", "menu", "footer", "search", "login", "toolbar", "widget",
			"ad", "ads", "sponsored", "promo":
			return true
		case "header":
			if !inArticle {
				return true
			}
		}
	}
	return false
}

func readerHTMLAttributeTokens(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
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

// readableTextScore scores a candidate by its blocks of 40 or more bytes,
// or by its total text when it has none.
func readableTextScore(blocks []Block) int {
	score, total := 0, 0
	for _, b := range blocks {
		total += len(b.Text)
		if len(b.Text) >= 40 {
			score += len(b.Text)
		}
	}
	if score == 0 {
		score = total
	}
	return score
}

func collapseWhitespace(s string) string {
	var t inlineText
	t.text(s)
	return t.String()
}

func stripHTMLTags(source string) string {
	return regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(source, " ")
}
