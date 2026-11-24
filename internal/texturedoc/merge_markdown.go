package texturedoc

// MergeMarkdown is the server's side of an owner's text edit: the Markdown the owner typed plus the parent revision's body_doc become the new body_doc, keeping every node of the parent the text did not
// touch exactly as it was (its ids, its marks, an embed, a source_ref and the entity it cites), so a client that only has text (the phone's Markdown editor) never builds or infers a document. The web editor
// builds its body_doc from its DOM, where embeds and source refs are atoms; a text editor has none, and the rule below stands in for them. Written once, here, next to Project, so Validate and Project (the
// store's one check of content against body_doc) stay the single truth. It is the port of mobile/AutoradioLab/texture-body.ts `merge`, which the phone drops when it uses this.
//
// The rule:
//   - a block of the text equal to the text the parent's node projects to (an embed's "[file: x]" line, a cited paragraph with its markers, an unchanged paragraph) IS that node, byte for byte;
//   - otherwise a block claims the next node of a compatible kind whose words are the same (letters and digits), so a moved or re-marked block keeps its node and id;
//   - a changed block is written fresh (paragraph, heading, lists, quote, code block, rule: what the web editor writes) and keeps the id of the node it replaces; a changed cited paragraph keeps its
//     source_refs (at its end, where the web editor puts them) and loses the typed "[1]" markers, which are the refs' own;
//   - an embed whose line the owner changed or replaced is deleted (the owner may keep, delete or move an embed, never alter one: revise_edit.go newEmbeds), and what was typed in its place is a new
//     paragraph, never dropped;
//   - one newline inside a block is a hard_break (what the server reads of plain text, plainTextInlineNodes, and what the web's Shift+Enter writes); a blank line is a new paragraph.
//
// The entities returned are those the new body cites; an entity nothing cites would be refused by Validate.

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"unicode"
)

func mergeNewID(kind string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return kind + "-" + hex.EncodeToString(b)
}

// MergeMarkdown returns the body_doc and the cited source entities for `markdown`, given the parent revision (`parent` has Doc.Type "doc"; a zero value means a document with no revision yet), its entities and
// its content (the text the parent projects to).
func MergeMarkdown(parent StructuredTextureDoc, entities []SourceEntity, parentMarkdown, markdown string) (StructuredTextureDoc, []SourceEntity) {
	hasParent := parent.Doc.Type == "doc"
	if hasParent && markdown == parentMarkdown {
		return parent, citedEntities(parent, entities)
	}
	var s []Node
	if hasParent {
		s = parent.Doc.Content
	}
	b := mdBlocks(parentMarkdown)
	n := mdBlocks(markdown)
	blank := true
	for _, x := range s {
		if x.Type != "paragraph" || nodePlainText(x) != "" {
			blank = false
		}
	}
	if !hasParent || (blank && len(b) == 0) {
		nodes := make([]Node, 0, len(n))
		for _, blk := range n {
			nodes = append(nodes, mdNodeFor(blk))
		}
		if len(nodes) == 0 {
			nodes = []Node{mdParagraph("")}
		}
		out := mdRoot(nodes, parent, hasParent)
		return out, citedEntities(out, entities)
	}

	texts := ProjectBlockTexts(parent, entities)
	claim := make([]int, len(b)) // the node a block is, or -1
	for i := range claim {
		claim[i] = -1
	}
	cursor := 0
	for i := range b {
		kind := mdKind(b[i])
		key := mdKey(b[i])
		for j := cursor; j < len(s); j++ {
			if texts[j] == b[i] || (mdCompatible(kind, s[j].Type) && mdNodeKey(s[j]) == key) {
				claim[i] = j
				cursor = j + 1
				break
			}
		}
	}
	// Nodes no block claims (an empty paragraph) stay after the block that came before them.
	var lead []Node
	attach := map[int][]Node{}
	blockOf := map[int]int{}
	for i, j := range claim {
		if j >= 0 {
			blockOf[j] = i
		}
	}
	lastClaimed := -1
	for j := range s {
		if i, ok := blockOf[j]; ok {
			lastClaimed = i
		} else if lastClaimed >= 0 {
			attach[lastClaimed] = append(attach[lastClaimed], s[j])
		} else {
			lead = append(lead, s[j])
		}
	}
	// Block diff: the longest common subsequence on the exact source.
	m, p := len(b), len(n)
	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, p+1)
	}
	for i := m - 1; i >= 0; i-- {
		for k := p - 1; k >= 0; k-- {
			if b[i] == n[k] {
				lcs[i][k] = lcs[i+1][k+1] + 1
			} else if lcs[i+1][k] >= lcs[i][k+1] {
				lcs[i][k] = lcs[i+1][k]
			} else {
				lcs[i][k] = lcs[i][k+1]
			}
		}
	}
	type pair struct{ i, k int }
	var pairs []pair
	for i, k := 0, 0; i < m && k < p; {
		if b[i] == n[k] {
			pairs = append(pairs, pair{i, k})
			i++
			k++
		} else if lcs[i+1][k] >= lcs[i][k+1] {
			i++
		} else {
			k++
		}
	}
	pairs = append(pairs, pair{m, p})
	out := append([]Node{}, lead...)
	pi, pk := 0, 0
	for _, pr := range pairs {
		var deleted, inserted []int
		for x := pi; x < pr.i; x++ {
			deleted = append(deleted, x)
		}
		for x := pk; x < pr.k; x++ {
			inserted = append(inserted, x)
		}
		for t, kk := range inserted {
			var old *Node
			if t < len(deleted) && claim[deleted[t]] >= 0 {
				old = &s[claim[deleted[t]]]
			}
			text := n[kk]
			cited := old != nil && old.Type == "paragraph" && hasSourceRef(*old)
			if cited {
				text = citationMarker.ReplaceAllString(text, "")
			}
			fresh := mdNodeFor(text)
			if old != nil && old.Type != "embed" {
				mdCarry(*old, &fresh)
			}
			out = append(out, fresh)
		}
		for _, d := range deleted {
			out = append(out, attach[d]...)
		}
		if pr.i < m {
			if j := claim[pr.i]; j >= 0 {
				out = append(out, s[j])
			} else {
				out = append(out, mdNodeFor(n[pr.k]))
			}
			out = append(out, attach[pr.i]...)
		}
		pi, pk = pr.i+1, pr.k+1
	}
	if len(out) == 0 {
		out = []Node{mdParagraph("")}
	}
	doc := mdRoot(out, parent, true)
	return doc, citedEntities(doc, entities)
}

func mdRoot(content []Node, like StructuredTextureDoc, has bool) StructuredTextureDoc {
	doc := StructuredTextureDoc{Schema: SchemaV1, Doc: Node{Type: "doc", Attrs: map[string]any{"id": mergeNewID("doc")}, Content: content}}
	if has {
		if s := strings.TrimSpace(like.Schema); s != "" {
			doc.Schema = s
		}
		if len(like.Doc.Attrs) > 0 {
			doc.Doc.Attrs = like.Doc.Attrs
		}
	}
	return doc
}

// citedEntities are the entities (in their order) that the body cites.
func citedEntities(doc StructuredTextureDoc, entities []SourceEntity) []SourceEntity {
	cited := map[string]bool{}
	var walk func(n Node)
	walk = func(n Node) {
		if n.Type == "source_ref" {
			cited[stringAttr(n, "source_entity_id")] = true
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	for _, n := range doc.Doc.Content {
		walk(n)
	}
	out := []SourceEntity{}
	for _, e := range entities {
		if cited[strings.TrimSpace(e.SourceEntityID)] {
			out = append(out, e)
		}
	}
	return out
}

func hasSourceRef(n Node) bool {
	for _, c := range n.Content {
		if c.Type == "source_ref" {
			return true
		}
	}
	return false
}

func nodePlainText(n Node) string {
	if n.Type == "text" {
		return n.Text
	}
	var sb strings.Builder
	for _, c := range n.Content {
		sb.WriteString(nodePlainText(c))
	}
	return sb.String()
}

func mdCarry(old Node, fresh *Node) {
	if id, ok := old.Attrs["id"]; ok {
		if fresh.Attrs == nil {
			fresh.Attrs = map[string]any{}
		}
		fresh.Attrs["id"] = id
	}
	if old.Type != "paragraph" || fresh.Type != "paragraph" {
		return
	}
	for _, c := range old.Content {
		if c.Type == "source_ref" {
			fresh.Content = append(fresh.Content, c)
		}
	}
}

// ---------------------------------------------------------------------------------------------------------------- blocks and kinds

func mdTrim(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return unicode.Is(unicode.Zs, r) || r == '\t' })
}

func mdBlocks(src string) []string {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	var out, cur []string
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for i := 0; i < len(lines); {
		l := lines[i]
		t := mdTrim(l)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			flush()
			fence := t[:3]
			block := []string{l}
			i++
			for i < len(lines) {
				block = append(block, lines[i])
				if strings.HasPrefix(mdTrim(lines[i]), fence) {
					i++
					break
				}
				i++
			}
			out = append(out, strings.Join(block, "\n"))
			continue
		}
		if t == "" {
			flush()
		} else {
			cur = append(cur, l)
		}
		i++
	}
	flush()
	return out
}

func mdKind(block string) string {
	lines := strings.Split(block, "\n")
	t := mdTrim(lines[0])
	switch {
	case strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~"):
		return "code"
	case mdHeading(t) != nil:
		return "heading"
	case mdIsRule(t) && !strings.Contains(block, "\n"):
		return "rule"
	case strings.HasPrefix(t, ">"):
		return "quote"
	case mdListMarker(lines[0]) != nil:
		return "list"
	}
	return "paragraph"
}

type mdHeadingInfo struct {
	level int
	text  string
}

func mdHeading(t string) *mdHeadingInfo {
	n := 0
	for _, c := range t {
		if c == '#' {
			n++
		} else {
			break
		}
	}
	if n < 1 || n > 6 || len(t) <= n || t[n] != ' ' {
		return nil
	}
	return &mdHeadingInfo{n, mdTrim(t[n+1:])}
}

func mdIsRule(t string) bool {
	s := []rune(strings.ReplaceAll(t, " ", ""))
	if len(s) < 3 || !strings.ContainsRune("-*_", s[0]) {
		return false
	}
	for _, c := range s {
		if c != s[0] {
			return false
		}
	}
	return true
}

type mdMarker struct {
	indent  int
	ordered bool
	number  int
	rest    string
}

func mdListMarker(line string) *mdMarker {
	chars := []rune(line)
	indent, p := 0, 0
	for p < len(chars) && (chars[p] == ' ' || chars[p] == '\t') {
		if chars[p] == '\t' {
			indent += 4
		} else {
			indent++
		}
		p++
	}
	if p+1 < len(chars) && strings.ContainsRune("-*+", chars[p]) && chars[p+1] == ' ' {
		return &mdMarker{indent: indent, rest: string(chars[p+2:])}
	}
	q := p
	digits := ""
	for q < len(chars) && unicode.IsNumber(chars[q]) {
		digits += string(chars[q])
		q++
	}
	if digits != "" && q+1 < len(chars) && (chars[q] == '.' || chars[q] == ')') && chars[q+1] == ' ' {
		num := 0
		for _, c := range digits {
			if c >= '0' && c <= '9' {
				num = num*10 + int(c-'0')
				if num > 1<<30 {
					num = 1
					break
				}
			}
		}
		if num == 0 {
			num = 1
		}
		return &mdMarker{indent: indent, ordered: true, number: num, rest: string(chars[q+2:])}
	}
	return nil
}

func mdLettersDigits(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsMark(r) || unicode.Is(unicode.Nd, r) {
			sb.WriteRune(unicode.ToLower(r))
		}
	}
	return sb.String()
}

func mdKey(block string) string {
	s := block
	t := mdTrim(s)
	if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
		if i := strings.Index(s, "\n"); i >= 0 {
			s = s[i+1:]
		} else {
			s = ""
		}
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = listPrefix.ReplaceAllString(l, "")
	}
	s = strings.Join(lines, "\n")
	s = linkDest.ReplaceAllString(s, "]")
	return mdLettersDigits(s)
}

func mdNodeKey(n Node) string { return mdLettersDigits(nodePlainText(n)) }

func mdCompatible(kind, typ string) bool {
	switch kind {
	case "code":
		return typ == "code_block"
	case "heading":
		return typ == "heading"
	case "rule":
		return typ == "horizontal_rule"
	case "quote":
		return typ == "blockquote"
	case "list":
		return typ == "bullet_list" || typ == "ordered_list"
	case "paragraph":
		return typ == "paragraph"
	}
	return false
}
