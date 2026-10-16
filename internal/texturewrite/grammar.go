// Package texturewrite is Texture's one write verb (Cut 2 unit 2a, item iv):
// the edit grammar choir.Write takes, the frame Texture reads each turn, and
// the materializer that turns a Write into the next structured revision.
//
// A Write is a short script of directives on block aliases. The frame names
// the head document's top-level blocks b1..bN; a directive line starts with
// @ and the lines under it are the new text, in a small markdown subset
// (paragraphs, # headings, - and 1. lists, > quotes, ``` code, ---, and
// inline **strong**, *emphasis* and `code`, which become marks):
//
//	@replace b3          replace b3 (or a range b3..b5) with the text below
//	@insert after b2     insert the text below after b2 (also: before bN, start, end)
//	@delete b4 b6..b7    delete blocks
//	@move b5 after b1    move a block (also: before bN, start, end)
//	@section b2          rewrite the section headed by b2 with the text below
//	@all                 rewrite the whole document with the text below
//
// Citations are inline tokens: {{cite <handle> "quote"}} cites the quoted
// words of a captured source, and {{ref <source_entity_id>}} keeps a
// citation the frame shows (one made before Write existed, or a stored
// embed that quoted nothing). {{embed}} is retired and refused. A handle
// is a frame source (r2.c1), a capture id or a URL. Every refusal names
// the script line it is about.
package texturewrite

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Op kinds.
const (
	OpReplace = "replace"
	OpInsert  = "insert"
	OpDelete  = "delete"
	OpMove    = "move"
	OpSection = "section"
	OpAll     = "all"
)

// Placement of an @insert or @move.
const (
	PlaceAfter  = "after"
	PlaceBefore = "before"
	PlaceStart  = "start"
	PlaceEnd    = "end"
)

// LineError is a refusal located at one line of a Write script.
type LineError struct {
	Line int
	Msg  string
}

func (e *LineError) Error() string { return fmt.Sprintf("line %d: %s", e.Line, e.Msg) }

// Errors is every refusal one script earned, in line order.
type Errors []*LineError

func (e Errors) Error() string {
	parts := make([]string, len(e))
	for i, le := range e {
		parts[i] = le.Error()
	}
	return strings.Join(parts, "; ")
}

func (e Errors) sorted() Errors {
	sort.SliceStable(e, func(i, j int) bool { return e[i].Line < e[j].Line })
	return e
}

// errorf appends a refusal; scripts stop collecting after maxErrors.
const maxErrors = 12

func (e *Errors) add(line int, format string, args ...any) {
	if len(*e) < maxErrors {
		*e = append(*e, &LineError{Line: line, Msg: fmt.Sprintf(format, args...)})
	}
}

// Op is one directive with its body.
type Op struct {
	Kind string
	// Line is the directive's line.
	Line int
	// Targets are the aliases the op claims (replace, delete, move, section).
	Targets []string
	// Place and Anchor say where an insert or move lands.
	Place  string
	Anchor string
	// Body is the parsed text under the directive.
	Body []BodyBlock
}

// Line is one script line with its number.
type Line struct {
	N    int
	Text string
}

// BodyBlock is one block of a directive's text.
type BodyBlock struct {
	Kind  string // paragraph, heading, bullet_list, ordered_list, blockquote, code_block, horizontal_rule
	Line  int
	Level int // heading level
	Start int // ordered list start
	Lang  string
	// Lines holds the block's text lines: one per list item for lists, the
	// paragraph or quote lines otherwise, the raw lines of a code block.
	Lines []Line
}

var (
	directiveRE = regexp.MustCompile(`^@([A-Za-z]+)\b(.*)$`)
	aliasRE     = regexp.MustCompile(`^b([1-9][0-9]*)$`)
	rangeRE     = regexp.MustCompile(`^b([1-9][0-9]*)(?:\.\.|-)b([1-9][0-9]*)$`)
	headingRE   = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	bulletRE    = regexp.MustCompile(`^[-*]\s+(.+)$`)
	orderedRE   = regexp.MustCompile(`^(\d{1,3})[.)]\s+(.+)$`)
	ruleRE      = regexp.MustCompile(`^(?:-{3,}|\*{3,}|_{3,})$`)
	fenceRE     = regexp.MustCompile("^```\\s*([A-Za-z0-9_+-]*)\\s*$")
)

func parse(script string, errs *Errors) []Op {
	script = strings.ReplaceAll(script, "\r\n", "\n")
	script = strings.ReplaceAll(script, "\r", "\n")
	raw := strings.Split(script, "\n")
	var ops []Op
	var cur *Op
	var body []Line
	flush := func() {
		if cur == nil {
			return
		}
		cur.Body = parseBody(body, errs)
		checkBody(cur, errs)
		ops = append(ops, *cur)
		cur, body = nil, nil
	}
	sawText := false
	for i, text := range raw {
		n := i + 1
		if m := directiveRE.FindStringSubmatch(strings.TrimRight(text, " \t")); m != nil {
			flush()
			op := parseDirective(strings.ToLower(m[1]), strings.Fields(m[2]), n, errs)
			cur = &op
			continue
		}
		if cur == nil {
			if strings.TrimSpace(text) != "" && !sawText {
				sawText = true
				errs.add(n, "text before the first directive; start the script with @replace, @insert, @delete, @move, @section or @all")
			}
			continue
		}
		if strings.HasPrefix(text, `\@`) {
			text = text[1:]
		}
		body = append(body, Line{N: n, Text: text})
	}
	flush()
	if len(ops) == 0 && len(*errs) == 0 {
		errs.add(1, "the script has no directive; start it with @replace, @insert, @delete, @move, @section or @all")
	}
	return ops
}

func parseDirective(kind string, args []string, line int, errs *Errors) Op {
	op := Op{Kind: kind, Line: line}
	switch kind {
	case OpReplace, OpDelete:
		if len(args) == 0 {
			errs.add(line, "@%s needs a block alias such as b3 (or a range b3..b5)", kind)
		}
		if kind == OpReplace && len(args) > 1 {
			errs.add(line, "@replace takes one alias or one range, not %q", strings.Join(args, " "))
		}
		for _, a := range args {
			targets, ok := expandAliases(a)
			if !ok {
				errs.add(line, "%q is not a block alias (b1, b2, ...) or range (b3..b5)", a)
				continue
			}
			op.Targets = append(op.Targets, targets...)
		}
	case OpSection:
		if len(args) != 1 || !aliasRE.MatchString(args[0]) {
			errs.add(line, "@section takes the alias of one heading, such as @section b4")
		} else {
			op.Targets = []string{args[0]}
		}
	case OpAll:
		if len(args) != 0 {
			errs.add(line, "@all takes no arguments; the whole document follows on the next lines")
		}
	case OpInsert:
		op.Place, op.Anchor = parsePlace(kind, args, line, errs)
	case OpMove:
		if len(args) == 0 || !aliasRE.MatchString(args[0]) {
			errs.add(line, "@move needs the block to move and where: @move b5 after b2")
			return op
		}
		op.Targets = []string{args[0]}
		op.Place, op.Anchor = parsePlace(kind, args[1:], line, errs)
	default:
		errs.add(line, "@%s is not a directive; use @replace, @insert, @delete, @move, @section or @all (write \\@ to start a line with a literal @)", kind)
	}
	return op
}

func parsePlace(kind string, args []string, line int, errs *Errors) (string, string) {
	usage := fmt.Sprintf("@%s needs a place: after bN, before bN, start or end", kind)
	if len(args) == 0 {
		errs.add(line, "%s", usage)
		return "", ""
	}
	switch strings.ToLower(args[0]) {
	case PlaceStart, PlaceEnd:
		if len(args) != 1 {
			errs.add(line, "%s", usage)
		}
		return strings.ToLower(args[0]), ""
	case PlaceAfter, PlaceBefore:
		if len(args) != 2 || !aliasRE.MatchString(args[1]) {
			errs.add(line, "%s", usage)
			return "", ""
		}
		return strings.ToLower(args[0]), args[1]
	}
	errs.add(line, "%s", usage)
	return "", ""
}

func expandAliases(a string) ([]string, bool) {
	if aliasRE.MatchString(a) {
		return []string{a}, true
	}
	m := rangeRE.FindStringSubmatch(a)
	if m == nil {
		return nil, false
	}
	from, _ := strconv.Atoi(m[1])
	to, _ := strconv.Atoi(m[2])
	if to < from {
		return nil, false
	}
	out := make([]string, 0, to-from+1)
	for i := from; i <= to; i++ {
		out = append(out, "b"+strconv.Itoa(i))
	}
	return out, true
}

func checkBody(op *Op, errs *Errors) {
	switch op.Kind {
	case OpReplace, OpInsert, OpSection, OpAll:
		if len(op.Body) == 0 {
			errs.add(op.Line, "@%s needs text: the new block(s) go on the lines after it", op.Kind)
		}
	case OpDelete, OpMove:
		if len(op.Body) > 0 {
			errs.add(op.Body[0].Line, "@%s takes no text; put new text under @replace or @insert", op.Kind)
		}
	}
}

// parseBody splits a directive's lines into blocks.
func parseBody(lines []Line, errs *Errors) []BodyBlock {
	var out []BodyBlock
	var cur *BodyBlock
	flush := func() {
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}
	for i := 0; i < len(lines); i++ {
		ln := lines[i]
		trimmed := strings.TrimSpace(ln.Text)
		if trimmed == "" {
			flush()
			continue
		}
		if m := fenceRE.FindStringSubmatch(trimmed); m != nil {
			flush()
			block := BodyBlock{Kind: "code_block", Line: ln.N, Lang: m[1]}
			closed := false
			for i++; i < len(lines); i++ {
				if strings.TrimSpace(lines[i].Text) == "```" {
					closed = true
					break
				}
				block.Lines = append(block.Lines, lines[i])
			}
			if !closed {
				errs.add(ln.N, "code block opened here is not closed with ```")
			}
			out = append(out, block)
			continue
		}
		if m := headingRE.FindStringSubmatch(trimmed); m != nil {
			flush()
			out = append(out, BodyBlock{Kind: "heading", Line: ln.N, Level: len(m[1]), Lines: []Line{{N: ln.N, Text: m[2]}}})
			continue
		}
		if ruleRE.MatchString(trimmed) {
			flush()
			out = append(out, BodyBlock{Kind: "horizontal_rule", Line: ln.N})
			continue
		}
		if m := bulletRE.FindStringSubmatch(trimmed); m != nil {
			if cur == nil || cur.Kind != "bullet_list" {
				flush()
				cur = &BodyBlock{Kind: "bullet_list", Line: ln.N}
			}
			cur.Lines = append(cur.Lines, Line{N: ln.N, Text: m[1]})
			continue
		}
		if m := orderedRE.FindStringSubmatch(trimmed); m != nil {
			if cur == nil || cur.Kind != "ordered_list" {
				flush()
				start, _ := strconv.Atoi(m[1])
				if start < 1 {
					start = 1
				}
				cur = &BodyBlock{Kind: "ordered_list", Line: ln.N, Start: start}
			}
			cur.Lines = append(cur.Lines, Line{N: ln.N, Text: m[2]})
			continue
		}
		if strings.HasPrefix(trimmed, ">") {
			text := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
			if cur == nil || cur.Kind != "blockquote" {
				flush()
				cur = &BodyBlock{Kind: "blockquote", Line: ln.N}
			}
			if text != "" {
				cur.Lines = append(cur.Lines, Line{N: ln.N, Text: text})
			}
			continue
		}
		if cur == nil || cur.Kind != "paragraph" {
			flush()
			cur = &BodyBlock{Kind: "paragraph", Line: ln.N}
		}
		cur.Lines = append(cur.Lines, Line{N: ln.N, Text: trimmed})
	}
	flush()
	return out
}

// frameHandleRE is a handle the frame minted: record r, source c.
var frameHandleRE = regexp.MustCompile(`^r[1-9][0-9]*\.c[1-9][0-9]*$`)

// Check is the synchronous check a cell runs before staging a Write: the
// grammar, the aliases against a document of blockCount top-level blocks,
// and frame handles against the handles the frame showed. Capture handles
// and quotes are checked when the turn commits (Apply).
func Check(script string, blockCount int, frameHandles map[string]bool) error {
	var errs Errors
	ops := parse(script, &errs)
	checkOps(ops, blockCount, nil, &errs)
	for _, op := range ops {
		for _, b := range op.Body {
			if b.Kind == "code_block" {
				continue
			}
			for _, ln := range b.Lines {
				segs, err := parseInline(ln.Text, ln.N)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				for _, s := range segs {
					if s.Token != nil && frameHandleRE.MatchString(s.Token.Handle) && !frameHandles[s.Token.Handle] {
						errs.add(ln.N, "%s is not a source in your frame; frame sources are listed under the records as rN.cM", s.Token.Handle)
					}
				}
			}
		}
	}
	if len(errs) > 0 {
		return errs.sorted()
	}
	return nil
}

// checkOps checks aliases, claims and @all exclusivity. headings, when set,
// says which aliases are headings (for @section).
func checkOps(ops []Op, blockCount int, headings map[string]bool, errs *Errors) {
	valid := func(a string) bool {
		m := aliasRE.FindStringSubmatch(a)
		if m == nil {
			return false
		}
		n, _ := strconv.Atoi(m[1])
		return n >= 1 && n <= blockCount
	}
	docRange := "the document is empty"
	if blockCount > 0 {
		docRange = fmt.Sprintf("the document has b1..b%d", blockCount)
	}
	claimed := map[string]int{}
	moved := map[string]bool{}
	for _, op := range ops {
		if op.Kind == OpMove {
			for _, t := range op.Targets {
				moved[t] = true
			}
		}
	}
	for _, op := range ops {
		if op.Kind == OpAll && len(ops) > 1 {
			errs.add(op.Line, "@all rewrites the whole document and must be the only directive in the Write")
		}
		for _, t := range op.Targets {
			if !valid(t) {
				errs.add(op.Line, "%s is not a block in this document (%s)", t, docRange)
				continue
			}
			if first, ok := claimed[t]; ok {
				errs.add(op.Line, "%s is already changed by the directive on line %d; one directive per block", t, first)
				continue
			}
			claimed[t] = op.Line
		}
		if op.Anchor != "" {
			switch {
			case !valid(op.Anchor):
				errs.add(op.Line, "%s is not a block in this document (%s)", op.Anchor, docRange)
			case moved[op.Anchor]:
				errs.add(op.Line, "%s is itself moved; place relative to a block that stays", op.Anchor)
			case op.Kind == OpMove && len(op.Targets) == 1 && op.Targets[0] == op.Anchor:
				errs.add(op.Line, "a block cannot move relative to itself")
			}
		}
		if op.Kind == OpSection && headings != nil && len(op.Targets) == 1 && valid(op.Targets[0]) && !headings[op.Targets[0]] {
			errs.add(op.Line, "@section %s: %s is not a heading; use @replace for a single block", op.Targets[0], op.Targets[0])
		}
	}
}
