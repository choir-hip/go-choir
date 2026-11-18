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
var programmeContentKinds = map[string]bool{"story": true, "update": true, "headlines": true, "take": true}
var programmeOtherKinds = map[string]bool{"opening": true, "transition": true, "podcast_clip": true}

// programmeKindAliases are the old names of the two station kinds. A line that
// still uses one is accepted and renamed, so a stored stretch and an old
// prompt keep working; the names are deleted one release after the phone
// speaks the new ones.
var programmeKindAliases = map[string]string{"ident": "opening", "bridge": "transition"}
var programmeLaterKinds = map[string]bool{"mail": true, "podcast_note": true, "answer": true, "result": true}

// programmeTextRange is the longest text a kind may have, in bytes. The
// programme design says "the length is within the kind's range" and gives no
// numbers; these are bounds on what a stranger-shaped model line can be, not
// steers, and are generous for twenty minutes of speech (about 150 words a
// minute across ten to fifteen segments).
var programmeTextRange = map[string]int{
	"story": 8000, "update": 4000, "headlines": 3000, "transition": 500, "opening": 1500, "podcast_clip": 800, "take": programmeTakeLeadMax,
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
	// TakeID and TakeSeconds name the take a take segment leads in.
	TakeID        string
	TakeSeconds   float64
	TakePublisher string // the publisher's handle: the take entry's source label

	// Telling is full or followup, as the DJ wrote it (anything else is full).
	Telling string

	lineNo int // the line this segment came from, for the index's notes

	// ClaimID is the forecast claim of a content segment (set when the claim
	// is written, or read back on a replay): the segment event and the run
	// sheet entry carry it.
	ClaimID string

	// BetweenAfter and BetweenBefore name the neighbours a transition was
	// written for: a segment_id of this stretch, or ahead:<n>, the n-th
	// (1-based) title of the request's ahead_titles. Both set or both empty.
	BetweenAfter, BetweenBefore string

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
	// lineNotebook is a notebook line: collected, validated after the stream.
	lineNotebook
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
	if kind == "notebook" {
		return nil, lineNotebook, ""
	}
	if renamed, ok := programmeKindAliases[kind]; ok {
		kind = renamed
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

	if reason := in.checkBetween(seg, raw["between"]); reason != "" {
		return invalidf("%s", reason)
	}
	if kind == "podcast_clip" {
		return in.checkClipLine(seg, raw)
	}
	if kind == "take" {
		return in.checkTakeLine(seg, raw)
	}
	cites, reason := in.checkCites(raw["cites"], content)
	if reason != "" {
		return invalidf("%s", reason)
	}
	seg.Cites = cites
	if content {
		var telling string
		if json.Unmarshal(raw["telling"], &telling) == nil && strings.TrimSpace(telling) == tellingFollowup {
			seg.Telling = tellingFollowup
		}
	}

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
	if in.usedClips[id] {
		return invalidf("clip %q is already used in this stretch", truncate(id, 60))
	}
	if in.usedClips == nil {
		in.usedClips = map[string]bool{}
	}
	in.usedClips[id] = true
	seg.ClipID, seg.ClipSeconds = id, ref.clip.Seconds
	seg.Cites = []programmeCite{{Ref: ref.cand.ref, Role: "quoted", Label: truncate(oneLine(ref.cand.Title), programmeMaxLabel)}}
	return seg, lineOK, ""
}

// checkTakeLine finishes a take line (design 11.5). The line names a take of
// the request and carries the DJ's lead-in as its text; it has the forecast of
// a story (p_heard and caused_by, validated as a story's), and its cites are
// filled here and never taken from the model: the take itself, and the story
// it responds to when the request lists that story. A take of the listener's
// own, or by a muted publisher, is not in the request's set (buildInput
// removes it), so no line can name it.
func (in *programmeInput) checkTakeLine(seg *programmeSegment, raw map[string]json.RawMessage) (*programmeSegment, lineClass, string) {
	var id string
	if err := json.Unmarshal(raw["take"], &id); err != nil || id == "" {
		return invalidf("a take line names its take in the take field")
	}
	cand, ok := in.takeByID[id]
	if !ok {
		return invalidf("take %q is not one of this request's takes", truncate(id, 60))
	}
	if in.usedTakes[id] {
		return invalidf("take %q is already used in this stretch", truncate(id, 60))
	}
	if _, has := raw["p_heard"]; !has {
		return invalidf("p_heard is missing")
	}
	p, reason := parseP(raw["p_heard"])
	if reason != "" {
		return invalidf("%s", reason)
	}
	if _, has := raw["caused_by"]; !has {
		return invalidf("caused_by is missing")
	}
	var causedBy string
	if err := json.Unmarshal(raw["caused_by"], &causedBy); err != nil {
		return invalidf("caused_by is not a string")
	}
	if reason := in.checkCause(seg, causedBy); reason != "" {
		return invalidf("%s", reason)
	}
	seg.P = p
	if in.usedTakes == nil {
		in.usedTakes = map[string]bool{}
	}
	in.usedTakes[id] = true
	seg.TakeID, seg.TakeSeconds, seg.TakePublisher = id, cand.DurationSeconds, cand.Publisher
	seg.Cites = []programmeCite{{Ref: cand.ref, Role: "subject", Label: truncate(oneLine(cand.Title), programmeMaxLabel)}}
	if r := cand.RespondsTo; r != nil {
		for _, story := range in.candidates {
			if !story.isTake() && story.Route == r.Route {
				seg.Cites = append(seg.Cites, programmeCite{Ref: story.ref, Role: "mentioned", Label: truncate(oneLine(story.Title), programmeMaxLabel)})
				break
			}
		}
	}
	return seg, lineOK, ""
}

// checkBetween reads the optional between of a transition:
// {"after_segment": ..., "before_segment": ...}, each the s<n> of an entry of
// this response (n counting from 0, as the segment ids do) or ahead:<n>. The
// after_segment names an entry that already exists: an earlier s<n> of this
// response or an ahead:<n> title. before_segment may also name one forward
// reference, exactly the entry written next (s<this line's number + 1>), below
// the cap; it may never arrive, and the phone drops a transition whose before
// entry is not the entry that follows it. This is a shape check, not a
// judgement of the words. A
// transition that carries none stays valid (the old bridge had none).
func (in *programmeInput) checkBetween(seg *programmeSegment, raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	if seg.Kind != "transition" {
		return fmt.Sprintf("between on a %s, which joins nothing", seg.Kind)
	}
	if string(bytes.TrimSpace(raw)) == "null" {
		return ""
	}
	var b struct {
		After  string `json:"after_segment"`
		Before string `json:"before_segment"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return "between is not {after_segment, before_segment}"
	}
	after, reason := in.resolveNeighbour(b.After, false)
	if reason != "" {
		return "between.after_segment: " + reason
	}
	before, reason := in.resolveNeighbour(b.Before, true)
	if reason != "" {
		return "between.before_segment: " + reason
	}
	if after == before {
		return "between names one entry on both sides"
	}
	seg.BetweenAfter, seg.BetweenBefore = after, before
	return ""
}

// resolveNeighbour checks one side of a between and returns it as the id the
// event carries: the stretch-relative s<n> stays s<n> until the handler gives
// it the stretch id (see qualifyBetween), and ahead:<n> stays as written.
func (in *programmeInput) resolveNeighbour(ref string, before bool) (string, string) {
	switch {
	case strings.HasPrefix(ref, "ahead:"):
		n, err := strconv.Atoi(strings.TrimPrefix(ref, "ahead:"))
		if err != nil || strconv.Itoa(n) != strings.TrimPrefix(ref, "ahead:") || n < 1 || n > in.aheadCount {
			return "", fmt.Sprintf("%q is not one of the %d ahead titles of this request", truncate(ref, 40), in.aheadCount)
		}
		return ref, ""
	case strings.HasPrefix(ref, "s"):
		n, err := strconv.Atoi(ref[1:])
		if err != nil || strconv.Itoa(n) != ref[1:] || n < 0 || n >= programmeMaxSegments || n >= in.nextSeg && !(before && n == in.nextSeg+1) {
			return "", fmt.Sprintf("%q is not an entry of this response", truncate(ref, 40))
		}
		return ref, ""
	}
	return "", fmt.Sprintf("%q is neither s<n> nor ahead:<n>", truncate(ref, 40))
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
	case strings.HasPrefix(causedBy, "try:"):
		id := strings.TrimPrefix(causedBy, "try:")
		if !in.tryIDs[id] {
			return fmt.Sprintf("caused_by names %s, which is not a trying line in this call's input", truncate(id, 20))
		}
		seg.Cause, seg.CausedBy, seg.LessonID = calibration.CauseTrying, causedBy, id
		return ""
	case strings.HasPrefix(causedBy, "mine:"), strings.HasPrefix(causedBy, "tap:"):
		if !in.nbRefs[causedBy] {
			return fmt.Sprintf("caused_by %q is not a line of this call's notebook", truncate(causedBy, 60))
		}
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
