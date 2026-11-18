// Package notebook is the grammar and the arithmetic of the listener's
// notebook (docs/design/autoradio-learned-document-2026-11-17.md, sections 2
// and 3): one Markdown document, parsed into lines, patched by ops, rendered
// back. It calls no model and reads no store. The Autoradio handler uses it to
// check a proposal against the page it was shown; Texture uses the same code
// to apply the patch it was handed, so the check and the write cannot differ.
package notebook

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The sections, in the order the document renders them.
const (
	Title       = "My notebook"
	SecAsked    = "What you asked for"
	SecLearned  = "What Autoradio has learned"
	SecTrying   = "What Autoradio is trying"
	SecSteering = "Recent steering"

	// Budgets (design 2.1 and 2.4).
	MaxLearned  = 12
	MaxTrying   = 3
	MaxSteering = 10
	// TargetWords is the page the budget aims at; HardWords is the cap on the
	// sections Autoradio writes (learned, trying, steering) together.
	TargetWords = 400
	HardWords   = 1000
	// MaxLineChars bounds a lesson or trying line's text. MaxAskedChars bounds
	// a line Texture writes for the listener (their words and its decoration).
	MaxLineChars  = 200
	MaxAskedChars = 400
	// ConsolidateAbove is the count of listener lines past which the DJ may
	// suggest one consolidation. The notebook never applies it itself.
	ConsolidateAbove = 15
)

// Grants of the Autoradio desk on the Texture custodian (design 3.2).
const (
	GrantPatch = "notebook.patch"
	GrantRead  = "notebook.read"
)

// Patch statuses Texture answers with.
const (
	StatusApplied = "applied"
	StatusStale   = "stale"
	StatusRefused = "refused"
	StatusNoop    = "noop"
)

// Section is a heading with its lines.
type Section struct {
	Heading string
	Lines   []Line
}

// Line is one line of a section. A tagged line is `- text [L7 · heard 9 of 11]`;
// any other line (a listener's bullet, prose, a blank) is kept as written.
type Line struct {
	Raw    string
	Bullet bool
	Text   string // the bullet's text without the tag
	Tag    string // L7, T2, or "" when untagged
	Heard  int
	Of     int
}

// Doc is a parsed notebook: lines before the first section heading, then the
// sections in the order found.
type Doc struct {
	Head     []string
	Sections []Section
}

var tagRe = regexp.MustCompile(`^- (.+?) \[([LT])([0-9]{1,9}) · heard ([0-9]{1,9}) of ([0-9]{1,9})\]\s*$`)
var tagInTextRe = regexp.MustCompile(`\[[LT][0-9]`)

// Ref is a line's name for `caused_by` and for cites (design 2.2): `lesson:L7`,
// `try:T2`, `mine:<8 hex>` for any other bullet.
func (l Line) Ref() string {
	switch {
	case !l.Bullet:
		return ""
	case strings.HasPrefix(l.Tag, "L"):
		return "lesson:" + l.Tag
	case strings.HasPrefix(l.Tag, "T"):
		return "try:" + l.Tag
	}
	return MineRef(l.Text)
}

// MineRef is the ref of a listener's line: the first 8 hex digits of the
// sha256 of its trimmed text.
func MineRef(text string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(text)))
	return "mine:" + hex.EncodeToString(sum[:])[:8]
}

// Parse reads a notebook. It is total: any text parses, and Render of the
// result is the same text up to trailing blank lines.
func Parse(md string) Doc {
	var d Doc
	cur := -1
	for _, raw := range strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n") {
		if h, ok := strings.CutPrefix(raw, "## "); ok {
			d.Sections = append(d.Sections, Section{Heading: strings.TrimSpace(h)})
			cur = len(d.Sections) - 1
			continue
		}
		if cur < 0 {
			d.Head = append(d.Head, raw)
			continue
		}
		d.Sections[cur].Lines = append(d.Sections[cur].Lines, parseLine(raw))
	}
	for len(d.Head) > 0 && strings.TrimSpace(d.Head[len(d.Head)-1]) == "" {
		d.Head = d.Head[:len(d.Head)-1]
	}
	for i := range d.Sections {
		ls := d.Sections[i].Lines
		for len(ls) > 0 && strings.TrimSpace(ls[len(ls)-1].Raw) == "" {
			ls = ls[:len(ls)-1]
		}
		d.Sections[i].Lines = ls
	}
	return d
}

func parseLine(raw string) Line {
	if m := tagRe.FindStringSubmatch(raw); m != nil {
		h, _ := strconv.Atoi(m[4])
		o, _ := strconv.Atoi(m[5])
		return Line{Raw: raw, Bullet: true, Text: m[1], Tag: m[2] + m[3], Heard: h, Of: o}
	}
	if t, ok := strings.CutPrefix(raw, "- "); ok && strings.TrimSpace(t) != "" {
		return Line{Raw: raw, Bullet: true, Text: strings.TrimSpace(t)}
	}
	return Line{Raw: raw}
}

// Fresh is the empty notebook: the title and the four sections.
func Fresh() Doc {
	d := Doc{Head: []string{"# " + Title}}
	for _, h := range []string{SecAsked, SecLearned, SecTrying, SecSteering} {
		d.Sections = append(d.Sections, Section{Heading: h})
	}
	return d
}

// Render writes the document as Markdown.
func (d Doc) Render() string {
	var b strings.Builder
	for _, l := range d.Head {
		b.WriteString(l + "\n")
	}
	for _, s := range d.Sections {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("## " + s.Heading + "\n")
		for _, l := range s.Lines {
			b.WriteString(l.render() + "\n")
		}
	}
	return b.String()
}

func (l Line) render() string {
	if l.Tag != "" {
		return fmt.Sprintf("- %s [%s · heard %d of %d]", l.Text, l.Tag, l.Heard, l.Of)
	}
	return l.Raw
}

// Section returns the named section, adding it at the end when it is absent.
func (d *Doc) section(heading string) *Section {
	for i := range d.Sections {
		if strings.EqualFold(d.Sections[i].Heading, heading) {
			return &d.Sections[i]
		}
	}
	d.Sections = append(d.Sections, Section{Heading: heading})
	return &d.Sections[len(d.Sections)-1]
}

// Lines returns the bullets of a section, in order.
func (d Doc) Lines(heading string) []Line {
	for _, s := range d.Sections {
		if strings.EqualFold(s.Heading, heading) {
			var out []Line
			for _, l := range s.Lines {
				if l.Bullet {
					out = append(out, l)
				}
			}
			return out
		}
	}
	return nil
}

// Tagged is every learned and trying line, by tag.
func (d Doc) Tagged() map[string]Line {
	out := map[string]Line{}
	for _, s := range d.Sections {
		for _, l := range s.Lines {
			if l.Tag != "" {
				out[l.Tag] = l
			}
		}
	}
	return out
}

// AllLines is every bullet of the document with its section, in order.
func (d Doc) AllLines() []SectionLine {
	var out []SectionLine
	// Bullets above the first heading (an adopted edition's own lines) are the
	// listener's: they count as "What you asked for".
	for _, raw := range d.Head {
		if l := parseLine(raw); l.Bullet && l.Tag == "" {
			out = append(out, SectionLine{Section: SecAsked, Line: l})
		}
	}
	for _, s := range d.Sections {
		for _, l := range s.Lines {
			if l.Bullet {
				out = append(out, SectionLine{Section: s.Heading, Line: l})
			}
		}
	}
	return out
}

// SectionLine is a line with the heading it sits under.
type SectionLine struct {
	Section string
	Line    Line
}

// MaxTag is the highest number among the tags of one letter (L or T).
func (d Doc) MaxTag(letter string) int {
	max := 0
	for tag := range d.Tagged() {
		if strings.HasPrefix(tag, letter) {
			if n, err := strconv.Atoi(tag[1:]); err == nil && n > max {
				max = n
			}
		}
	}
	return max
}

// IsTapLine reports whether a bullet's text is a steering line Autoradio wrote
// from a tap (a listener's own bullet under that heading is not).
func IsTapLine(text string) bool {
	return strings.HasPrefix(text, "More like this: ") || strings.HasPrefix(text, "Less like this: ")
}

// tapKey is a tap line without its trailing time: "More like this: "T"".
func tapKey(text string) string {
	if i := strings.LastIndex(text, " ("); i > 0 {
		return text[:i]
	}
	return text
}

// WrittenWords counts the words of the lines Autoradio wrote: the tagged lines
// of learned and trying, and its tap lines under steering (design 2.4). A
// bullet the listener wrote, wherever it stands, is not counted: their text
// can never make the page refuse a patch.
func (d Doc) WrittenWords() int {
	n := 0
	for _, s := range d.Sections {
		h := strings.ToLower(s.Heading)
		for _, l := range s.Lines {
			if !l.Bullet {
				continue
			}
			switch {
			case (h == strings.ToLower(SecLearned) || h == strings.ToLower(SecTrying)) && l.Tag != "":
				n += len(strings.Fields(l.render()))
			case h == strings.ToLower(SecSteering) && l.Tag == "" && IsTapLine(l.Text):
				n += len(strings.Fields(l.render()))
			}
		}
	}
	return n
}

// Counts is the heard h of m of each tagged line, by tag.
type Counts map[string][2]int

// CheckText is the shape of the text of a line Autoradio writes: one line, not
// empty, within MaxLineChars, with no tag of its own and no control character.
func CheckText(text string) error {
	t := strings.TrimSpace(text)
	switch {
	case t == "":
		return fmt.Errorf("the text is empty")
	case utf8.RuneCountInString(t) > MaxLineChars:
		return fmt.Errorf("the text is over %d characters", MaxLineChars)
	case strings.ContainsAny(t, "\n\r\x00") || hasControl(t):
		return fmt.Errorf("the text is not one line")
	case tagInTextRe.MatchString(t):
		return fmt.Errorf("the text carries a tag of its own")
	case strings.HasPrefix(t, "-") || strings.HasPrefix(t, "#"):
		return fmt.Errorf("the text begins like a list item or a heading")
	case linkRe.MatchString(t):
		return fmt.Errorf("the text holds a link or a URL")
	}
	return nil
}

var linkRe = regexp.MustCompile(`(?i)https?:|www\.|\]\(|<https?|\[[^\]]*\]\[`)

func hasControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// Op is one change to the notebook. Texture applies a patch's ops in the order
// strike, untag, retire, add, ask, tap, whatever order they are listed in.
type Op struct {
	Op string `json:"op"` // add, retire, strike, untag, ask, tap
	// Section is learned or trying (add).
	Section string `json:"section,omitempty"`
	// ID is the minted tag of an add (L8, T3), or the line a retire or strike
	// names.
	ID       string   `json:"id,omitempty"`
	Text     string   `json:"text,omitempty"`
	Replaces []string `json:"replaces,omitempty"`
	// Ref is a tap's ref (tap:<entry id>); its Text is the whole line.
	Ref string `json:"ref,omitempty"`
	// Replacement is the listener's own corrected words, for a strike.
	Replacement string `json:"replacement,omitempty"`
}

// Patch is what Autoradio asks Texture to apply.
type Patch struct {
	PatchID        string `json:"patch_id"`
	BaseRevisionID string `json:"base_revision_id"` // empty: no notebook exists yet
	Ops            []Op   `json:"ops"`
	Counts         Counts `json:"counts,omitempty"`
}

func idLetter(id string) (string, bool) {
	if len(id) < 2 || (id[0] != 'L' && id[0] != 'T') {
		return "", false
	}
	if _, err := strconv.Atoi(id[1:]); err != nil {
		return "", false
	}
	return id[:1], true
}

// find returns the section and index of the line carrying tag.
func (d *Doc) find(tag string) (*Section, int) {
	for i := range d.Sections {
		for j, l := range d.Sections[i].Lines {
			if l.Tag == tag {
				return &d.Sections[i], j
			}
		}
	}
	return nil, -1
}

func (s *Section) remove(i int) { s.Lines = append(s.Lines[:i], s.Lines[i+1:]...) }

func headingFor(section string) (heading, letter string, max int, err error) {
	switch section {
	case "learned":
		return SecLearned, "L", MaxLearned, nil
	case "trying":
		return SecTrying, "T", MaxTrying, nil
	}
	return "", "", 0, fmt.Errorf("section %q is not learned or trying", section)
}

func tagged(section *Section) int {
	n := 0
	for _, l := range section.Lines {
		if l.Tag != "" {
			n++
		}
	}
	return n
}

// ApplyOp applies one op to a copy of the document and returns it, or the
// reason it is refused. Budgets are checked at the end of Apply, not here,
// except the page-full rule of an add.
func (d Doc) ApplyOp(op Op) (Doc, error) {
	out := d.clone()
	switch op.Op {
	case "add":
		heading, letter, max, err := headingFor(op.Section)
		if err != nil {
			return d, err
		}
		if l, ok := idLetter(op.ID); !ok || l != letter {
			return d, fmt.Errorf("id %q is not a %s tag", op.ID, letter)
		}
		if s, _ := out.find(op.ID); s != nil {
			return d, fmt.Errorf("%s is already a line", op.ID)
		}
		if err := CheckText(op.Text); err != nil {
			return d, err
		}
		sec := out.section(heading)
		seen := map[string]bool{}
		for _, id := range op.Replaces {
			if seen[id] {
				return d, fmt.Errorf("replaces names %s twice", id)
			}
			seen[id] = true
			s, i := out.find(id)
			if s == nil || !strings.EqualFold(s.Heading, heading) {
				return d, fmt.Errorf("replaces names %s, which is not a live line of %s", id, heading)
			}
			s.remove(i)
			sec = out.section(heading)
		}
		if tagged(sec) >= max && len(op.Replaces) == 0 {
			return d, fmt.Errorf("%s holds %d lines and has %d: a new line must replace or merge lines, named in replaces", heading, max, tagged(sec))
		}
		sec.Lines = append(sec.Lines, Line{Bullet: true, Text: strings.TrimSpace(op.Text), Tag: op.ID})
	case "retire":
		if _, ok := idLetter(op.ID); !ok {
			return d, fmt.Errorf("id %q is not a line tag", op.ID)
		}
		s, i := out.find(op.ID)
		if s == nil {
			return d, fmt.Errorf("%s is not a live line", op.ID)
		}
		s.remove(i)
	case "strike":
		if _, ok := idLetter(op.ID); !ok {
			return d, fmt.Errorf("id %q is not a line tag", op.ID)
		}
		if s, i := out.find(op.ID); s != nil {
			s.remove(i)
		}
		if r := strings.TrimSpace(op.Replacement); r != "" {
			line := r + " (you corrected " + op.ID + ")"
			if err := checkAsked(line); err != nil {
				return d, err
			}
			sec := out.section(SecAsked)
			sec.Lines = append(sec.Lines, Line{Raw: "- " + line, Bullet: true, Text: line})
		}
	case "untag":
		// The listener reworded a line Autoradio wrote and the rewording is not
		// one a strike can move: their words stay where they put them, as their
		// own bullet, never tagged as a lesson.
		if _, ok := idLetter(op.ID); !ok {
			return d, fmt.Errorf("id %q is not a line tag", op.ID)
		}
		s, i := out.find(op.ID)
		if s == nil {
			return d, fmt.Errorf("%s is not a live line", op.ID)
		}
		t := strings.TrimSpace(strings.Map(func(r rune) rune {
			if r < 0x20 || r == 0x7f {
				return ' '
			}
			return r
		}, op.Text))
		if t == "" {
			return d, fmt.Errorf("the text is empty")
		}
		s.Lines[i] = Line{Raw: "- " + t, Bullet: true, Text: t}
	case "ask":
		if err := checkAsked(op.Text); err != nil {
			return d, err
		}
		sec := out.section(SecAsked)
		line := strings.TrimSpace(op.Text)
		for _, l := range sec.Lines {
			if l.Bullet && l.Text == line {
				return out, nil // the listener's line is already there
			}
		}
		sec.Lines = append(sec.Lines, Line{Raw: "- " + line, Bullet: true, Text: line})
	case "tap":
		if !strings.HasPrefix(op.Ref, "tap:") {
			return d, fmt.Errorf("a tap's ref is tap:<entry id>")
		}
		if err := checkAsked(op.Text); err != nil {
			return d, err
		}
		sec := out.section(SecSteering)
		line := strings.TrimSpace(op.Text)
		// The notebook already holds this tap (same verb and title, whatever
		// the time): a re-sent or deferred tap is a no-op.
		for _, l := range sec.Lines {
			if l.Bullet && l.Tag == "" && IsTapLine(l.Text) && tapKey(l.Text) == tapKey(line) {
				return out, nil
			}
		}
		sec.Lines = append(sec.Lines, Line{Raw: "- " + line, Bullet: true, Text: line})
		// Only the lines Autoradio wrote are cut, oldest first; a bullet the
		// listener put under this heading is never removed.
		for {
			taps, first := 0, -1
			for i, l := range sec.Lines {
				if l.Bullet && l.Tag == "" && IsTapLine(l.Text) {
					taps++
					if first < 0 {
						first = i
					}
				}
			}
			if taps <= MaxSteering {
				break
			}
			sec.remove(first)
		}
	default:
		return d, fmt.Errorf("op %q is not add, retire, strike, ask or tap", op.Op)
	}
	return out, nil
}

func checkAsked(text string) error {
	t := strings.TrimSpace(text)
	switch {
	case t == "":
		return fmt.Errorf("the text is empty")
	case utf8.RuneCountInString(t) > MaxAskedChars:
		return fmt.Errorf("the text is over %d characters", MaxAskedChars)
	case strings.ContainsAny(t, "\n\r") || hasControl(t):
		return fmt.Errorf("the text is not one line")
	case tagInTextRe.MatchString(t):
		return fmt.Errorf("the text carries a tag of its own")
	}
	return nil
}

func (d Doc) clone() Doc {
	out := Doc{Head: append([]string(nil), d.Head...)}
	for _, s := range d.Sections {
		out.Sections = append(out.Sections, Section{Heading: s.Heading, Lines: append([]Line(nil), s.Lines...)})
	}
	return out
}

// Apply applies a patch: the ops in the order strike, retire, add, ask, tap,
// then the counts, then the page budget. Any refused op refuses the patch.
func (d Doc) Apply(p Patch) (Doc, error) {
	cur := d
	for _, phase := range []string{"strike", "untag", "retire", "add", "ask", "tap"} {
		for i, op := range p.Ops {
			if op.Op != phase {
				continue
			}
			next, err := cur.ApplyOp(op)
			if err != nil {
				return d, fmt.Errorf("op %d (%s): %w", i, op.Op, err)
			}
			cur = next
		}
	}
	for i := range cur.Sections {
		for j := range cur.Sections[i].Lines {
			l := &cur.Sections[i].Lines[j]
			if c, ok := p.Counts[l.Tag]; ok && l.Tag != "" {
				l.Heard, l.Of = c[0], c[1]
			}
		}
	}
	hasAdd := false
	for _, op := range p.Ops {
		if op.Op == "add" {
			hasAdd = true
		}
	}
	// A strike, a retire or a tap is never refused for the page's words: they
	// cannot make it longer than the lines that were already there.
	if w := cur.WrittenWords(); hasAdd && w > HardWords {
		return d, fmt.Errorf("the sections Autoradio writes would hold %d words, over the %d of the page", w, HardWords)
	}
	return cur, nil
}
