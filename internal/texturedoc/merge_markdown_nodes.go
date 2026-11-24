package texturedoc

// The Markdown to nodes half of MergeMarkdown: what the web editor writes for a block of text (paragraph, heading, bullet and ordered lists, quote, code block, rule) with the inline marks strong, emphasis and
// code, and hard breaks. A link keeps its text (the web has no link mark).

import (
	"regexp"
	"strings"
)

var (
	citationMarker = regexp.MustCompile(`\[source \d+: [^\]]*\]|\[\d+\]`)
	listPrefix     = regexp.MustCompile(`^\s*(>\s*)*([-*+]|\d+[.)])\s+`)
	linkDest       = regexp.MustCompile(`\]\([^)]*\)`)
)

func mdParagraph(text string) Node {
	return Node{Type: "paragraph", Attrs: map[string]any{"id": mergeNewID("paragraph")}, Content: mdInline(text)}
}

func mdNodeFor(block string) Node {
	switch mdKind(block) {
	case "code":
		lines := strings.Split(block, "\n")
		language := mdTrim(strings.TrimPrefix(strings.TrimPrefix(mdTrim(lines[0]), "```"), "~~~"))
		lines = lines[1:]
		if len(lines) > 0 {
			last := mdTrim(lines[len(lines)-1])
			if strings.HasPrefix(last, "```") || strings.HasPrefix(last, "~~~") {
				lines = lines[:len(lines)-1]
			}
		}
		attrs := map[string]any{"id": mergeNewID("code-block")}
		if language != "" {
			attrs["language"] = language
		}
		n := Node{Type: "code_block", Attrs: attrs}
		if len(lines) > 0 {
			n.Content = []Node{{Type: "text", Text: strings.Join(lines, "\n")}}
		}
		return n
	case "heading":
		h := mdHeading(mdTrim(block))
		if h == nil {
			h = &mdHeadingInfo{1, block}
		}
		return Node{Type: "heading", Attrs: map[string]any{"id": mergeNewID("heading"), "level": h.level}, Content: mdInline(h.text)}
	case "rule":
		return Node{Type: "horizontal_rule", Attrs: map[string]any{"id": mergeNewID("hr")}}
	case "quote":
		var inner []string
		for _, l := range strings.Split(block, "\n") {
			t := mdTrim(l)
			if strings.HasPrefix(t, ">") {
				t = strings.TrimPrefix(strings.TrimPrefix(t, ">"), " ")
			}
			inner = append(inner, t)
		}
		var content []Node
		for _, b := range mdBlocks(strings.Join(inner, "\n")) {
			content = append(content, mdNodeFor(b))
		}
		if len(content) == 0 {
			content = []Node{mdParagraph("")}
		}
		return Node{Type: "blockquote", Attrs: map[string]any{"id": mergeNewID("blockquote")}, Content: content}
	case "list":
		return mdList(strings.Split(block, "\n"))
	}
	return mdParagraph(block)
}

func mdList(lines []string) Node {
	base := int(^uint(0) >> 1)
	for _, l := range lines {
		if m := mdListMarker(l); m != nil && m.indent < base {
			base = m.indent
		}
	}
	if base == int(^uint(0)>>1) {
		base = 0
	}
	first := mdListMarker(lines[0])
	var items [][]string
	for _, l := range lines {
		m := mdListMarker(l)
		if m != nil && m.indent <= base {
			items = append(items, []string{l})
		} else if len(items) > 0 {
			items[len(items)-1] = append(items[len(items)-1], l)
		} else {
			items = append(items, []string{l})
		}
	}
	ordered := first != nil && first.ordered
	var nodes []Node
	for _, item := range items {
		head := item[0]
		if m := mdListMarker(item[0]); m != nil {
			head = m.rest
		}
		rest := item[1:]
		k := -1
		for i, x := range rest {
			if mdListMarker(x) != nil {
				k = i
				break
			}
		}
		para := []string{head}
		var content []Node
		if k >= 0 {
			for _, x := range rest[:k] {
				para = append(para, mdTrim(x))
			}
			content = append(content, mdParagraphLines(para), mdList(rest[k:]))
		} else {
			for _, x := range rest {
				para = append(para, mdTrim(x))
			}
			content = append(content, mdParagraphLines(para))
		}
		nodes = append(nodes, Node{Type: "list_item", Attrs: map[string]any{"id": mergeNewID("list-item")}, Content: content})
	}
	attrs := map[string]any{"id": mergeNewID("list")}
	typ := "bullet_list"
	if ordered {
		typ = "ordered_list"
		start := 1
		if first.number > 1 {
			start = first.number
		}
		attrs["start"] = start
	}
	return Node{Type: typ, Attrs: attrs, Content: nodes}
}

func mdParagraphLines(lines []string) Node {
	return Node{Type: "paragraph", Attrs: map[string]any{"id": mergeNewID("paragraph")}, Content: mdInline(strings.Join(lines, "\n"))}
}

// mdInline is inline Markdown as text nodes with the marks strong, emphasis and code, and hard breaks (a newline, a backslash newline, two spaces and a newline).
func mdInline(src string) []Node {
	var out []Node
	var lastMarks []string
	haveLast := false
	same := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	add := func(text string, marks []string) {
		if text == "" {
			return
		}
		if n := len(out); n > 0 && out[n-1].Type == "text" && haveLast && same(lastMarks, marks) {
			out[n-1].Text += text
			return
		}
		node := Node{Type: "text", Text: text}
		for _, m := range marks {
			node.Marks = append(node.Marks, Mark{Type: m})
		}
		out = append(out, node)
		lastMarks = append([]string{}, marks...)
		haveLast = true
	}
	has := func(marks []string, m string) bool {
		for _, x := range marks {
			if x == m {
				return true
			}
		}
		return false
	}
	with := func(marks []string, m string) []string {
		if has(marks, m) {
			return marks
		}
		return append(append([]string{}, marks...), m)
	}
	closer := func(s []rune, from int, pat []rune) int {
		for j := from; j+len(pat) <= len(s); j++ {
			if j > from {
				ok := true
				for k := range pat {
					if s[j+k] != pat[k] {
						ok = false
						break
					}
				}
				if ok {
					return j
				}
			}
		}
		return -1
	}
	indexFrom := func(s []rune, c rune, from int) int {
		for j := from; j < len(s); j++ {
			if s[j] == c {
				return j
			}
		}
		return -1
	}
	var scan func(s []rune, marks []string)
	scan = func(s []rune, marks []string) {
		i := 0
		var buf strings.Builder
		flush := func() { add(buf.String(), marks); buf.Reset() }
		for i < len(s) {
			c := s[i]
			if c == '\\' && i+1 < len(s) {
				if s[i+1] == '\n' {
					flush()
					out = append(out, Node{Type: "hard_break"})
					haveLast = false
					i += 2
					continue
				}
				buf.WriteRune(s[i+1])
				i += 2
				continue
			}
			if c == ' ' && i+2 < len(s) && s[i+1] == ' ' && s[i+2] == '\n' {
				flush()
				out = append(out, Node{Type: "hard_break"})
				haveLast = false
				i += 3
				continue
			}
			if c == '\n' {
				flush()
				out = append(out, Node{Type: "hard_break"})
				haveLast = false
				i++
				continue
			}
			if c == '`' {
				if j := indexFrom(s, '`', i+1); j >= 0 {
					flush()
					add(string(s[i+1:j]), with(marks, "code"))
					i = j + 1
					continue
				}
			}
			if (c == '*' || c == '_') && i+1 < len(s) && s[i+1] == c {
				if j := closer(s, i+2, []rune{c, c}); j >= 0 {
					flush()
					scan(s[i+2:j], with(marks, "strong"))
					i = j + 2
					continue
				}
			}
			if (c == '*' || c == '_') && i+1 < len(s) && s[i+1] != ' ' {
				if j := closer(s, i+1, []rune{c}); j >= 0 {
					flush()
					scan(s[i+1:j], with(marks, "emphasis"))
					i = j + 1
					continue
				}
			}
			if c == '[' {
				cl := indexFrom(s, ']', i+1)
				if cl >= 0 && cl+1 < len(s) && s[cl+1] == '(' {
					if end := indexFrom(s, ')', cl+2); end >= 0 {
						flush()
						scan(s[i+1:cl], marks)
						i = end + 1
						continue
					}
				}
			}
			buf.WriteRune(c)
			i++
		}
		flush()
	}
	scan([]rune(src), nil)
	return out
}
