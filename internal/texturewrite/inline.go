package texturewrite

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Token kinds.
const (
	TokenCite  = "cite"
	TokenEmbed = "embed"
	TokenRef   = "ref"
)

// Token is one {{...}} citation token in a block's text.
type Token struct {
	Kind   string
	Handle string
	Quote  string
	Line   int
}

// Mark types a Write's inline markdown makes, in the order they nest.
const (
	MarkStrong   = "strong"
	MarkEmphasis = "emphasis"
	MarkCode     = "code"
)

// Segment is a run of plain text or one token.
type Segment struct {
	Text  string
	Token *Token
	// Marks are the text's marks, outermost first: strong, emphasis, code.
	// A token carries none; the words around it keep theirs.
	Marks []string
}

// piece is one scanned unit of a line: text, a token, a code span, or a run
// of 1-3 asterisks that may open or close emphasis (1 emphasis, 2 strong,
// 3 both). pair is the index of the run it pairs with, -1 if none.
type piece struct {
	text        string
	token       *Token
	code        bool
	delim       int
	open, close bool
	pair        int
}

// parseInline splits one line into text and tokens, with **strong**,
// *emphasis* and `code` as marks on the text. A token never spans lines, so
// every token error names its own line.
func parseInline(s string, line int) ([]Segment, *LineError) {
	ps, err := scanInline(s, line)
	if err != nil {
		return nil, err
	}
	return resolveInline(ps), nil
}

// scanInline reads a line left to right: a {{token}}, a `code span` (its
// contents literal, tokens included) or an asterisk run; everything else is
// text. An asterisk run can open when a non-space follows it and no letter
// or digit precedes it, and close when a non-space precedes it and no letter
// or digit follows, so "5 * 3", "a ** b" and snake*case*words stay literal.
func scanInline(s string, line int) ([]piece, *LineError) {
	var ps []piece
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			ps = append(ps, piece{text: text.String(), pair: -1})
			text.Reset()
		}
	}
	for i := 0; i < len(s); {
		switch {
		case strings.HasPrefix(s[i:], "{{"):
			flush()
			tok, rest, err := parseToken(s[i+2:], line)
			if err != nil {
				return nil, err
			}
			ps = append(ps, piece{token: tok, pair: -1})
			i = len(s) - len(rest)
		case s[i] == '`':
			if j := strings.IndexByte(s[i+1:], '`'); j > 0 {
				flush()
				ps = append(ps, piece{text: s[i+1 : i+1+j], code: true, pair: -1})
				i += j + 2
				continue
			}
			text.WriteByte('`')
			i++
		case s[i] == '*':
			j := i
			for j < len(s) && s[j] == '*' {
				j++
			}
			if j-i > 3 {
				text.WriteString(s[i:j])
				i = j
				continue
			}
			prev, next := ' ', ' '
			if i > 0 {
				prev, _ = utf8.DecodeLastRuneInString(s[:i])
			}
			if j < len(s) {
				next, _ = utf8.DecodeRuneInString(s[j:])
			}
			flush()
			ps = append(ps, piece{
				text:  s[i:j],
				delim: j - i,
				open:  !unicode.IsSpace(next) && !isWordRune(prev),
				close: !unicode.IsSpace(prev) && !isWordRune(next),
				pair:  -1,
			})
			i = j
		default:
			text.WriteByte(s[i])
			i++
		}
	}
	flush()
	return ps, nil
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// resolveInline pairs each closing asterisk run with the nearest open run of
// the same length (open runs between them stay literal) and emits text with
// the marks in force. Unpaired runs are text.
func resolveInline(ps []piece) []Segment {
	var stack []int
	for i := range ps {
		p := &ps[i]
		if p.delim == 0 {
			continue
		}
		if p.close {
			for k := len(stack) - 1; k >= 0; k-- {
				if ps[stack[k]].delim == p.delim {
					p.pair, ps[stack[k]].pair = stack[k], i
					stack = stack[:k]
					break
				}
			}
			if p.pair >= 0 {
				continue
			}
		}
		if p.open {
			stack = append(stack, i)
		}
	}
	var out []Segment
	strong, em := 0, 0
	for i, p := range ps {
		switch {
		case p.token != nil:
			out = append(out, Segment{Token: p.token})
		case p.delim > 0 && p.pair >= 0:
			d := 1
			if p.pair < i {
				d = -1
			}
			if p.delim&2 != 0 {
				strong += d
			}
			if p.delim&1 != 0 {
				em += d
			}
		default:
			var marks []string
			if strong > 0 {
				marks = append(marks, MarkStrong)
			}
			if em > 0 {
				marks = append(marks, MarkEmphasis)
			}
			if p.code {
				marks = append(marks, MarkCode)
			}
			if k := len(out) - 1; k >= 0 && out[k].Token == nil && sameMarks(out[k].Marks, marks) {
				out[k].Text += p.text
				continue
			}
			out = append(out, Segment{Text: p.text, Marks: marks})
		}
	}
	return out
}

func sameMarks(a, b []string) bool {
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

// renderMarks moves the open marks to want, writing the markdown that closes
// and opens them, and returns the marks now open. Code stays innermost.
func renderMarks(b *strings.Builder, open, want []string) []string {
	keep := 0
	for keep < len(open) && containsMark(want, open[keep]) {
		keep++
	}
	if keep > 0 && open[keep-1] == MarkCode && len(want) > keep {
		keep--
	}
	for k := len(open) - 1; k >= keep; k-- {
		b.WriteString(markDelim(open[k]))
	}
	open = open[:keep]
	for _, m := range want {
		if !containsMark(open, m) {
			b.WriteString(markDelim(m))
			open = append(open, m)
		}
	}
	return open
}

func containsMark(marks []string, m string) bool {
	for _, x := range marks {
		if x == m {
			return true
		}
	}
	return false
}

func markDelim(m string) string {
	switch m {
	case MarkStrong:
		return "**"
	case MarkEmphasis:
		return "*"
	}
	return "`"
}

// parseToken reads `kind handle "quote"}}` after the opening braces and
// returns the rest of the line.
func parseToken(s string, line int) (*Token, string, *LineError) {
	usage := `a citation token is {{cite <handle> "quote"}}, {{embed <handle>}} or {{ref <source id>}}`
	s = strings.TrimLeft(s, " \t")
	kindEnd := strings.IndexAny(s, " \t}")
	if kindEnd <= 0 {
		return nil, "", &LineError{Line: line, Msg: "unclosed or empty {{ token; " + usage}
	}
	tok := &Token{Kind: strings.ToLower(s[:kindEnd]), Line: line}
	switch tok.Kind {
	case TokenCite, TokenEmbed, TokenRef:
	default:
		return nil, "", &LineError{Line: line, Msg: "{{" + s[:kindEnd] + " is not a citation token; " + usage}
	}
	s = strings.TrimLeft(s[kindEnd:], " \t")
	handleEnd := strings.IndexAny(s, " \t}\"“")
	if handleEnd < 0 {
		return nil, "", &LineError{Line: line, Msg: "unclosed {{" + tok.Kind + " token: it must end with }} on the same line"}
	}
	tok.Handle = s[:handleEnd]
	if tok.Handle == "" {
		return nil, "", &LineError{Line: line, Msg: "{{" + tok.Kind + "}} needs a handle (a frame source such as r2.c1, a capture id or a URL)"}
	}
	s = strings.TrimLeft(s[handleEnd:], " \t")
	if strings.HasPrefix(s, `"`) || strings.HasPrefix(s, "“") {
		quote, rest, ok := readQuote(s)
		if !ok {
			return nil, "", &LineError{Line: line, Msg: "unterminated quote in {{" + tok.Kind + " " + tok.Handle + "; close the quote with \" before }}"}
		}
		tok.Quote = quote
		s = strings.TrimLeft(rest, " \t")
	}
	if !strings.HasPrefix(s, "}}") {
		return nil, "", &LineError{Line: line, Msg: "unclosed {{" + tok.Kind + " " + tok.Handle + " token: it must end with }} on the same line; " + usage}
	}
	switch tok.Kind {
	case TokenCite:
		if strings.TrimSpace(tok.Quote) == "" {
			return nil, "", &LineError{Line: line, Msg: "{{cite " + tok.Handle + "}} needs the quoted source words: {{cite " + tok.Handle + ` "exact words"}}`}
		}
	case TokenRef:
		if tok.Quote != "" {
			return nil, "", &LineError{Line: line, Msg: "{{ref}} keeps an existing citation and takes no quote; use {{cite}} to cite new words"}
		}
	}
	return tok, s[2:], nil
}

// readQuote reads a double-quoted string ("..." with \" and \\ escapes, or
// curly “...”) and returns it with the rest of the line.
func readQuote(s string) (string, string, bool) {
	closer := `"`
	if strings.HasPrefix(s, "“") {
		s = strings.TrimPrefix(s, "“")
		closer = "”"
	} else {
		s = s[1:]
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && (s[i+1] == '"' || s[i+1] == '\\') {
			b.WriteByte(s[i+1])
			i++
			continue
		}
		if strings.HasPrefix(s[i:], closer) {
			return b.String(), s[i+len(closer):], true
		}
		b.WriteByte(s[i])
	}
	return "", "", false
}

// quoteToken renders a quote for a token, escaping what readQuote unescapes.
func quoteToken(q string) string {
	q = strings.ReplaceAll(q, `\`, `\\`)
	q = strings.ReplaceAll(q, `"`, `\"`)
	return `"` + q + `"`
}

// TokenText renders a token the way the frame shows it and Write reads it.
func TokenText(t Token) string {
	switch {
	case t.Kind == TokenRef || (t.Kind == TokenEmbed && t.Quote == ""):
		return "{{" + t.Kind + " " + t.Handle + "}}"
	default:
		return "{{" + t.Kind + " " + t.Handle + " " + quoteToken(t.Quote) + "}}"
	}
}
