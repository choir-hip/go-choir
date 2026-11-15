package agentcore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/calibration"
)

// The line reader and the validators of design 3.2. A line the model writes is
// one JSON object. Every check that fails drops the line (never the call); a
// run of three invalid lines ends the call (programme design 2.3).

const (
	programmeMaxLine    = 64 << 10
	programmeInvalidRun = 3
	programmeMaxCites   = 12
	programmeMaxLabel   = 200
	programmeMaxTitle   = 200
)

// Content kinds carry a forecast (design 3.1); the others carry none.
//
// Only what station L1 can legitimately produce is accepted: mail and
// podcast_note wait for their sources (programme design slices 3 and 6), and
// answer and result belong to the turn route and the hand-offs.
var programmeContentKinds = map[string]bool{"story": true, "update": true, "headlines": true}
var programmeOtherKinds = map[string]bool{"ident": true, "bridge": true, "podcast_clip": true}
var programmeLaterKinds = map[string]bool{"mail": true, "podcast_note": true, "answer": true, "result": true}

// programmeTextRange is the longest text a kind may have, in bytes. The
// programme design says "the length is within the kind's range" and gives no
// numbers; these are bounds on what a stranger-shaped model line can be, not
// steers, and are generous for twenty minutes of speech (about 150 words a
// minute across ten to fifteen segments).
var programmeTextRange = map[string]int{
	"story": 8000, "update": 4000, "headlines": 3000, "bridge": 500, "ident": 800, "podcast_clip": 800,
}

var (
	lessonIDRe   = regexp.MustCompile(`^L[0-9]{1,9}$`)
	editionRefRe = regexp.MustCompile(`^edition:P[1-9][0-9]{0,5}$`)
	noteIDRe     = regexp.MustCompile(`^[a-z0-9]{1,32}$`)
	fenceLineRe  = regexp.MustCompile("^```[A-Za-z]*$")
)

type programmeCite struct {
	Ref   string `json:"ref"`
	Role  string `json:"role"`
	Label string `json:"label"`
}

// programmeSegment is one valid line, ready to be written and streamed.
type programmeSegment struct {
	ID      string
	Kind    string
	Title   string
	Text    string
	Cites   []programmeCite
	Content bool
	Blocks  int
	SHA256  string

	// ClipID and ClipSeconds name the clip a podcast_clip segment leads in.
	ClipID      string
	ClipSeconds float64

	P        float64
	Cause    string // the cause class
	CausedBy string // the full ref
	LessonID string
}

type lineClass int

const (
	lineOK lineClass = iota
	lineInvalid
	// lineLesson is a lesson line: recognised and not processed (station L3).
	lineLesson
)

// lineReader assembles the model's deltas into lines. A line over the bound
// is dropped as it arrives (the bytes are discarded, never held), and is
// reported as one invalid line when its newline comes.
type lineReader struct {
	buf      []byte
	overflow bool
}

// push adds a delta and calls handle for each complete line; handle returns
// false to stop, and push then returns false.
func (r *lineReader) push(delta string, handle func(line string, tooLong bool) bool) bool {
	r.buf = append(r.buf, delta...)
	for {
		i := bytes.IndexByte(r.buf, '\n')
		if i < 0 {
			if len(r.buf) > programmeMaxLine {
				r.overflow = true
				r.buf = r.buf[:0]
			}
			return true
		}
		line, tooLong := string(r.buf[:i]), r.overflow || i > programmeMaxLine
		r.buf, r.overflow = r.buf[i+1:], false
		if !handle(line, tooLong) {
			return false
		}
	}
}

// flush handles the last line, which has no newline.
func (r *lineReader) flush(handle func(line string, tooLong bool) bool) bool {
	line, tooLong := string(r.buf), r.overflow || len(r.buf) > programmeMaxLine
	r.buf, r.overflow = nil, false
	if strings.TrimSpace(line) == "" && !tooLong {
		return true
	}
	return handle(line, tooLong)
}

// isSkippable is a blank line or a bare code fence: neither is a line.
func isSkippable(line string) bool {
	t := strings.TrimSpace(line)
	return t == "" || fenceLineRe.MatchString(t)
}

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func invalidf(format string, args ...any) (*programmeSegment, lineClass, string) {
	return nil, lineInvalid, fmt.Sprintf(format, args...)
}

// checkLine parses and validates one line against the call's input.
func (in *programmeInput) checkLine(line string) (*programmeSegment, lineClass, string) {
	dec := json.NewDecoder(strings.NewReader(strings.TrimSpace(line)))
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil || raw == nil {
		return invalidf("not a JSON object")
	}
	if dec.More() {
		return invalidf("data after the JSON object")
	}
	var kind string
	if err := json.Unmarshal(raw["kind"], &kind); err != nil {
		return invalidf("kind is missing or not a string")
	}
	if kind == "lesson" {
		return nil, lineLesson, ""
	}
	content := programmeContentKinds[kind]
	if programmeLaterKinds[kind] {
		return invalidf("kind %q is not produced until its station", kind)
	}
	if !content && !programmeOtherKinds[kind] {
		return invalidf("unknown kind %q", truncate(kind, 40))
	}
	seg := &programmeSegment{Kind: kind, Content: content}

	var text string
	if err := json.Unmarshal(raw["text"], &text); err != nil {
		return invalidf("text is missing or not a string")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return invalidf("text is empty")
	}
	if len(text) > programmeTextRange[kind] {
		return invalidf("text is %d bytes, over the %d of a %s", len(text), programmeTextRange[kind], kind)
	}
	if !utf8.ValidString(text) {
		return invalidf("text is not valid UTF-8")
	}
	seg.Text = text

	if t, ok := raw["title"]; ok {
		var title string
		if err := json.Unmarshal(t, &title); err != nil {
			return invalidf("title is not a string")
		}
		title = strings.TrimSpace(title)
		if hasControl(title) {
			return invalidf("title is not one line without control characters")
		}
		if len(title) > programmeMaxTitle {
			return invalidf("title is over %d bytes", programmeMaxTitle)
		}
		seg.Title = title
	}
	if content && seg.Title == "" {
		return invalidf("a %s needs a title", kind)
	}

	if kind == "podcast_clip" {
		return in.checkClipLine(seg, raw)
	}
	cites, reason := in.checkCites(raw["cites"], content)
	if reason != "" {
		return invalidf("%s", reason)
	}
	seg.Cites = cites

	_, hasP := raw["p_heard"]
	_, hasC := raw["caused_by"]
	if !content {
		if hasP || hasC {
			return invalidf("a forecast field (p_heard or caused_by) on a %s, which carries none", kind)
		}
		return seg, lineOK, ""
	}
	if !hasP {
		return invalidf("p_heard is missing")
	}
	p, reason := parseP(raw["p_heard"])
	if reason != "" {
		return invalidf("%s", reason)
	}
	seg.P = p
	if !hasC {
		return invalidf("caused_by is missing")
	}
	var causedBy string
	if err := json.Unmarshal(raw["caused_by"], &causedBy); err != nil {
		return invalidf("caused_by is not a string")
	}
	if reason := in.checkCause(seg, causedBy); reason != "" {
		return invalidf("%s", reason)
	}
	return seg, lineOK, ""
}

// checkClipLine finishes a podcast_clip line. The line names a clip of the
// request and carries the DJ's lead-in as its text; the cite is the clip's own
// story, filled here and never taken from the model, and the segment carries
// no forecast (residual R2 of docs/design/podcast-clips-2026-11-15.md).
func (in *programmeInput) checkClipLine(seg *programmeSegment, raw map[string]json.RawMessage) (*programmeSegment, lineClass, string) {
	if seg.Title == "" {
		return invalidf("a podcast_clip needs a title")
	}
	_, hasP := raw["p_heard"]
	_, hasC := raw["caused_by"]
	if hasP || hasC {
		return invalidf("a forecast field (p_heard or caused_by) on a podcast_clip, which carries none")
	}
	var id string
	if err := json.Unmarshal(raw["clip"], &id); err != nil || id == "" {
		return invalidf("a podcast_clip names its clip in the clip field")
	}
	ref, ok := in.clipByID[id]
	if !ok {
		return invalidf("clip %q is not one of this request's clips", truncate(id, 60))
	}
	seg.ClipID, seg.ClipSeconds = id, ref.clip.Seconds
	seg.Cites = []programmeCite{{Ref: ref.cand.ref, Role: "quoted", Label: truncate(oneLine(ref.cand.Title), programmeMaxLabel)}}
	return seg, lineOK, ""
}

func parseP(raw json.RawMessage) (float64, string) {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 || !(t[0] == '-' || (t[0] >= '0' && t[0] <= '9')) {
		return 0, "p_heard is not a number"
	}
	var p float64
	if err := json.Unmarshal(t, &p); err != nil {
		return 0, "p_heard is not a finite number"
	}
	if p < 0 || p > 1 {
		return 0, fmt.Sprintf("p_heard %v is outside [0, 1]", p)
	}
	if p == 0 {
		p = 0 // the assignment is the point: it replaces -0 with +0, clearing the sign bit
	}
	return p, ""
}

func (in *programmeInput) checkCause(seg *programmeSegment, causedBy string) string {
	switch {
	case causedBy == calibration.CauseImportance || causedBy == calibration.CauseExplore:
		seg.Cause, seg.CausedBy = causedBy, causedBy
		return ""
	case strings.HasPrefix(causedBy, "lesson:"):
		id := strings.TrimPrefix(causedBy, "lesson:")
		if !lessonIDRe.MatchString(id) {
			return "caused_by is not a lesson ref of the grammar"
		}
		if !in.lessonIDs[id] {
			return fmt.Sprintf("caused_by names lesson %s, which is not an active lesson in this call's input", id)
		}
		seg.Cause, seg.CausedBy, seg.LessonID = calibration.CauseLesson, causedBy, id
		return ""
	case strings.HasPrefix(causedBy, "instruction:turn/"):
		id := strings.TrimPrefix(causedBy, "instruction:turn/")
		if !autoradioTurnID.MatchString(id) {
			return "caused_by is not an instruction ref of the grammar"
		}
		if !in.turnIDs[id] {
			return fmt.Sprintf("caused_by names turn %s, which is not listed in this call's input", id)
		}
	case strings.HasPrefix(causedBy, "instruction:note/"):
		id := strings.TrimPrefix(causedBy, "instruction:note/")
		if !noteIDRe.MatchString(id) {
			return "caused_by is not an instruction ref of the grammar"
		}
		if !in.noteIDs[id] {
			return fmt.Sprintf("caused_by names note %s, which is not listed in this call's input", id)
		}
	case strings.HasPrefix(causedBy, "edition:"):
		if !editionRefRe.MatchString(causedBy) {
			return "caused_by is not an edition ref of the grammar"
		}
		n, _ := strconv.Atoi(strings.TrimPrefix(causedBy, "edition:P"))
		if n < 1 || n > len(in.edition) {
			return fmt.Sprintf("caused_by names edition paragraph %d, but the input numbers %d", n, len(in.edition))
		}
		seg.Cause, seg.CausedBy = calibration.CauseEdition, causedBy
		return ""
	default:
		return fmt.Sprintf("caused_by %q is not a cause class or a ref", truncate(causedBy, 60))
	}
	seg.Cause, seg.CausedBy = calibration.CauseInstruction, causedBy
	return ""
}

// checkCites validates the cites: each a candidate ref, as a string or as
// {ref, role, label}; a content segment needs at least one.
func (in *programmeInput) checkCites(raw json.RawMessage, content bool) ([]programmeCite, string) {
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		if content {
			return nil, "a content segment needs at least one cite"
		}
		return nil, ""
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, "cites is not an array"
	}
	if content && len(items) == 0 {
		return nil, "a content segment needs at least one cite"
	}
	if len(items) > programmeMaxCites {
		return nil, fmt.Sprintf("more than %d cites", programmeMaxCites)
	}
	var out []programmeCite
	seen := map[string]bool{}
	for _, item := range items {
		var c programmeCite
		var ref string
		if err := json.Unmarshal(item, &ref); err == nil {
			c.Ref = ref
		} else {
			var obj struct {
				Ref   string `json:"ref"`
				Role  string `json:"role"`
				Label string `json:"label"`
			}
			if err := json.Unmarshal(item, &obj); err != nil {
				return nil, "a cite is neither a ref string nor an object"
			}
			c = programmeCite{Ref: obj.Ref, Role: obj.Role, Label: obj.Label}
		}
		cand := in.byRef[c.Ref]
		if cand == nil {
			return nil, fmt.Sprintf("cite %q is not in the candidate set", truncate(c.Ref, 80))
		}
		switch c.Role {
		case "":
			c.Role = "subject"
		case "subject", "quoted", "mentioned":
		default:
			return nil, fmt.Sprintf("cite role %q is not subject, quoted or mentioned", truncate(c.Role, 20))
		}
		c.Label = strings.TrimSpace(c.Label)
		if c.Label == "" {
			c.Label = truncate(oneLine(cand.Title), programmeMaxLabel)
		}
		if hasControl(c.Label) || len(c.Label) > programmeMaxLabel {
			return nil, "a cite label is not one short line"
		}
		if seen[c.Ref] {
			continue
		}
		seen[c.Ref] = true
		out = append(out, c)
	}
	return out, ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "..."
}
