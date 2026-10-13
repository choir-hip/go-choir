package texturewrite

import (
	"strings"
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

// Segment is a run of plain text or one token.
type Segment struct {
	Text  string
	Token *Token
}

// parseInline splits one line into text and tokens. A token never spans
// lines, so every token error names its own line.
func parseInline(s string, line int) ([]Segment, *LineError) {
	var out []Segment
	for {
		i := strings.Index(s, "{{")
		if i < 0 {
			if s != "" {
				out = append(out, Segment{Text: s})
			}
			return out, nil
		}
		if i > 0 {
			out = append(out, Segment{Text: s[:i]})
		}
		tok, rest, err := parseToken(s[i+2:], line)
		if err != nil {
			return nil, err
		}
		out = append(out, Segment{Token: tok})
		s = rest
	}
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
