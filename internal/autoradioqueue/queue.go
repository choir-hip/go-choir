// Package autoradioqueue is the Autoradio desk's view of the listener's play
// queue (docs/design/autoradio-queue-desk-2026-11-15.md). The phone holds the
// queue and plays it; the desk proposes an ordered list of ops against the
// version it was shown. This package is the one place the rules live: the
// snapshot's bounds, what an op may name, and what the model's text is allowed
// to ask for. It reaches nothing: no store, no network, no clock.
//
// The Desk type is the surface a desk's Go cell will be given as its `queue`
// module (Snapshot, PlayNext, Move, Remove, ...). Tonight the fast turn drives
// the same methods from the model's JSON (ApplyJSON), so the cell changes who
// calls them and nothing about what they accept or what the phone receives.
package autoradioqueue

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Bounds on what a stranger can cause. They refuse; none of them is a limit on
// a model call.
const (
	MaxUpcoming    = 50
	MaxCandidates  = 40
	MaxOps         = 8 // recorded ops per turn
	MaxReasonBytes = 160
	maxTitleBytes  = 300
	maxRouteBytes  = 256
	maxIDBytes     = 128 // a document id, an episode id
	maxShowBytes   = 200
	maxURLBytes    = 1024
	maxAddedBy     = 32
	maxPosition    = 1e7 // seconds
	maxAttempts    = MaxOps * 4
)

// TooLargeError is a snapshot or candidate list that is well formed but over a
// bound. The route answers the turn without the queue and says so, rather than
// refusing a listener's words because their queue is long.
type TooLargeError struct{ msg string }

func (e *TooLargeError) Error() string { return e.msg }

// IsTooLarge reports whether err is a TooLargeError.
func IsTooLarge(err error) bool {
	var t *TooLargeError
	return errors.As(err, &t)
}

func tooLarge(format string, a ...any) error { return &TooLargeError{fmt.Sprintf(format, a...)} }

var entryID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)

// Item is what plays. Kind is story, page, document or audio and decides which
// of the other fields count.
type Item struct {
	Kind      string   `json:"kind"`
	Route     string   `json:"route,omitempty"`
	URL       string   `json:"url,omitempty"`
	ID        string   `json:"id,omitempty"`
	Title     string   `json:"title,omitempty"`
	At        int      `json:"at,omitempty"`
	Show      string   `json:"show,omitempty"`
	Start     *float64 `json:"start,omitempty"`
	End       *float64 `json:"end,omitempty"`
	EpisodeID string   `json:"episode_id,omitempty"`
}

// Entry is one queued item. ID is minted by the phone, unique and never reused.
type Entry struct {
	ID      string `json:"id"`
	Item    Item   `json:"item"`
	AddedBy string `json:"added_by,omitempty"`
}

// Current is the entry that is playing.
type Current struct {
	ID              string  `json:"id"`
	Item            Item    `json:"item"`
	PositionSeconds float64 `json:"position_seconds"`
}

// Snapshot is the phone's queue at one version.
type Snapshot struct {
	Version  int64    `json:"version"`
	Current  *Current `json:"current,omitempty"`
	Upcoming []Entry  `json:"upcoming"`
}

// Candidate is a paper story the desk may add to the queue.
type Candidate struct {
	Route string `json:"route"`
	Title string `json:"title"`
}

// Op is one change the desk proposes, in the phone's vocabulary.
type Op struct {
	Op          string `json:"op"`
	ID          string `json:"id,omitempty"`
	Item        *Item  `json:"item,omitempty"`
	ToIndex     *int   `json:"to_index,omitempty"`
	BaseVersion int64  `json:"base_version"`
	Reason      string `json:"reason"`
}

// ParseSnapshot reads and validates the phone's snapshot. Fields it does not
// know are ignored, so a newer phone does not break an older server.
func ParseSnapshot(raw []byte) (Snapshot, error) {
	var s Snapshot
	if err := json.Unmarshal(raw, &s); err != nil {
		return Snapshot{}, fmt.Errorf("queue: %v", err)
	}
	if s.Version < 0 {
		return Snapshot{}, errors.New("queue.version must not be negative")
	}
	if len(s.Upcoming) > MaxUpcoming {
		return Snapshot{}, tooLarge("queue.upcoming holds at most %d entries", MaxUpcoming)
	}
	seen := map[string]bool{}
	check := func(where, id string, it Item) error {
		if !entryID.MatchString(id) {
			return fmt.Errorf("%s.id must be 1 to 64 characters of letters, digits and _ . -", where)
		}
		if seen[id] {
			return fmt.Errorf("%s.id %q is not unique", where, id)
		}
		seen[id] = true
		return validateItem(where+".item", it)
	}
	if s.Current != nil {
		if err := check("queue.current", s.Current.ID, s.Current.Item); err != nil {
			return Snapshot{}, err
		}
		if p := s.Current.PositionSeconds; p < 0 || p > maxPosition || math.IsNaN(p) {
			return Snapshot{}, errors.New("queue.current.position_seconds is out of range")
		}
	}
	for i := range s.Upcoming {
		e := &s.Upcoming[i]
		if err := check(fmt.Sprintf("queue.upcoming[%d]", i), e.ID, e.Item); err != nil {
			return Snapshot{}, err
		}
		if len(e.AddedBy) > maxAddedBy {
			return Snapshot{}, tooLarge("queue.upcoming[%d].added_by is too long", i)
		}
	}
	if s.Upcoming == nil {
		s.Upcoming = []Entry{}
	}
	return s, nil
}

func validateItem(where string, it Item) error {
	for _, f := range []struct {
		name, v string
		max     int
	}{
		{"title", it.Title, maxTitleBytes}, {"route", it.Route, maxRouteBytes}, {"url", it.URL, maxURLBytes},
		{"id", it.ID, maxIDBytes}, {"show", it.Show, maxShowBytes}, {"episode_id", it.EpisodeID, maxIDBytes},
	} {
		if len(f.v) > f.max {
			return tooLarge("%s.%s exceeds %d bytes", where, f.name, f.max)
		}
	}
	need := func(name, v string) error {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("%s.%s is required for a %s", where, name, it.Kind)
		}
		return nil
	}
	switch it.Kind {
	case "story":
		return need("route", it.Route)
	case "page":
		return need("url", it.URL)
	case "document":
		return need("id", it.ID)
	case "audio":
		return need("url", it.URL)
	}
	return fmt.Errorf("%s.kind %q is not story, page, document or audio", where, oneLineBounded(it.Kind, 80))
}

// ValidateCandidates checks the stories the desk may add.
func ValidateCandidates(c []Candidate) error {
	if len(c) > MaxCandidates {
		return fmt.Errorf("candidates holds at most %d stories", MaxCandidates)
	}
	for i, x := range c {
		if x.Route == "" || len(x.Route) > maxRouteBytes || strings.ContainsAny(x.Route, " \t\r\n@#") {
			return fmt.Errorf("candidates[%d].route is empty, too long or has a space, @ or #", i)
		}
		if len(x.Title) > maxTitleBytes {
			return fmt.Errorf("candidates[%d].title exceeds %d bytes", i, maxTitleBytes)
		}
	}
	return nil
}

// slot is an entry in the desk's working copy. A pending slot was added by an op
// this turn: the phone has not minted its id, so no later op may name it.
type slot struct {
	Entry
	pending bool
}

// Desk is the queue the Autoradio desk edits: a working copy of one snapshot
// and the ops proposed against it. Not safe for concurrent use.
type Desk struct {
	base   int64
	cur    *Current
	curNew bool // the current entry was put there by a play_now this turn
	up     []slot
	cands  map[string]string // route -> title
	ops    []Op
}

// New starts a desk on a validated snapshot. A candidate named twice keeps its
// first title.
func New(s Snapshot, cands []Candidate) *Desk {
	d := &Desk{base: s.Version, cands: map[string]string{}}
	if s.Current != nil {
		c := *s.Current
		d.cur = &c
	}
	for _, e := range s.Upcoming {
		d.up = append(d.up, slot{Entry: e})
	}
	for _, c := range cands {
		if _, dup := d.cands[c.Route]; !dup {
			d.cands[c.Route] = c.Title
		}
	}
	return d
}

// Snapshot is the state after the ops so far. Entries added this turn have an
// empty id.
func (d *Desk) Snapshot() Snapshot {
	s := Snapshot{Version: d.base, Current: nil, Upcoming: make([]Entry, 0, len(d.up))}
	if d.cur != nil {
		c := *d.cur
		s.Current = &c
	}
	for _, x := range d.up {
		s.Upcoming = append(s.Upcoming, x.Entry)
	}
	return s
}

// Ops is the list proposed so far, each against the snapshot's version.
func (d *Desk) Ops() []Op { return append([]Op(nil), d.ops...) }

// BaseVersion is the version every op is against.
func (d *Desk) BaseVersion() int64 { return d.base }

func oneLineBounded(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= max {
		return s
	}
	s = s[:max]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

func (d *Desk) record(op Op, reason string) error {
	reason = oneLineBounded(reason, MaxReasonBytes)
	if reason == "" {
		return errors.New("an op needs a reason in one line")
	}
	if len(d.ops) >= MaxOps {
		return fmt.Errorf("at most %d ops in one turn", MaxOps)
	}
	op.BaseVersion, op.Reason = d.base, reason
	d.ops = append(d.ops, op)
	return nil
}

func (d *Desk) find(id string) int {
	if id == "" {
		return -1
	}
	for i, x := range d.up {
		if !x.pending && x.ID == id {
			return i
		}
	}
	return -1
}

func (d *Desk) knownStory(route string) bool {
	if d.cur != nil && d.cur.Item.Kind == "story" && d.cur.Item.Route == route {
		return true
	}
	for _, x := range d.up {
		if x.Item.Kind == "story" && x.Item.Route == route {
			return true
		}
	}
	return false
}

// story resolves an item to a named candidate, with the candidate's own title.
func (d *Desk) story(it Item) (Item, error) {
	if it.Kind != "story" {
		return Item{}, fmt.Errorf("the DJ can add only a story named in candidates, not a %q", oneLineBounded(it.Kind, 80))
	}
	title, ok := d.cands[it.Route]
	if !ok {
		return Item{}, fmt.Errorf("story %q is not one of the candidates", oneLineBounded(it.Route, 80))
	}
	if d.knownStory(it.Route) {
		return Item{}, fmt.Errorf("story %q is already in the queue; move it instead", oneLineBounded(it.Route, 80))
	}
	return Item{Kind: "story", Route: it.Route, Title: title}, nil
}

func (d *Desk) add(op string, it Item, reason string, at func(*Desk, slot)) error {
	resolved, err := d.story(it)
	if err != nil {
		return err
	}
	if err := d.record(Op{Op: op, Item: &resolved}, reason); err != nil {
		return err
	}
	at(d, slot{Entry: Entry{Item: resolved, AddedBy: "dj"}, pending: true})
	return nil
}

// PlayNext puts a named story at the front of upcoming.
func (d *Desk) PlayNext(it Item, reason string) error {
	return d.add("play_next", it, reason, func(d *Desk, s slot) { d.up = append([]slot{s}, d.up...) })
}

// Enqueue puts a named story at the end of upcoming.
func (d *Desk) Enqueue(it Item, reason string) error {
	return d.add("enqueue", it, reason, func(d *Desk, s slot) { d.up = append(d.up, s) })
}

// PlayNow makes a named story the current entry. What the phone does with the
// entry it replaces is its own; the desk's copy forgets it, so no later op this
// turn can name it.
func (d *Desk) PlayNow(it Item, reason string) error {
	return d.add("play_now", it, reason, func(d *Desk, s slot) {
		d.cur = &Current{Item: s.Item}
		d.curNew = true
	})
}

// Move puts an upcoming entry at index to; to is clamped to the list.
func (d *Desk) Move(id string, to int, reason string) error {
	from := d.find(id)
	if from < 0 {
		return fmt.Errorf("no upcoming entry %q (a playing entry is skipped, not moved)", oneLineBounded(id, 80))
	}
	to = max(0, min(to, len(d.up)-1))
	if to == from {
		return fmt.Errorf("entry %q is already at index %d", oneLineBounded(id, 80), to)
	}
	if err := d.record(Op{Op: "move", ID: id, ToIndex: &to}, reason); err != nil {
		return err
	}
	x := d.up[from]
	d.up = append(d.up[:from], d.up[from+1:]...)
	d.up = append(d.up[:to], append([]slot{x}, d.up[to:]...)...)
	return nil
}

// Remove drops an upcoming entry.
func (d *Desk) Remove(id string, reason string) error {
	i := d.find(id)
	if i < 0 {
		return fmt.Errorf("no upcoming entry %q (a playing entry is skipped, not removed)", oneLineBounded(id, 80))
	}
	if err := d.record(Op{Op: "remove", ID: id}, reason); err != nil {
		return err
	}
	d.up = append(d.up[:i], d.up[i+1:]...)
	return nil
}

// Skip ends the playing entry. The op names it, so a phone whose current entry
// has changed since drops the op.
func (d *Desk) Skip(reason string) error {
	if d.cur == nil {
		return errors.New("nothing is playing to skip")
	}
	if d.curNew || d.cur.ID == "" {
		return errors.New("the entry playing was added this turn and cannot be named")
	}
	if err := d.record(Op{Op: "skip", ID: d.cur.ID}, reason); err != nil {
		return err
	}
	if len(d.up) > 0 && !d.up[0].pending {
		next := d.up[0]
		d.cur = &Current{ID: next.ID, Item: next.Item}
		d.up = d.up[1:]
	} else {
		d.cur = nil
	}
	return nil
}

// Previous goes back one entry.
func (d *Desk) Previous(reason string) error {
	if d.cur == nil {
		return errors.New("nothing is playing to go back from")
	}
	return d.record(Op{Op: "previous"}, reason)
}

// Clear empties upcoming.
func (d *Desk) Clear(reason string) error {
	if len(d.up) == 0 {
		return errors.New("nothing is upcoming to clear")
	}
	if err := d.record(Op{Op: "clear"}, reason); err != nil {
		return err
	}
	d.up = nil
	return nil
}

type modelOp struct {
	Op      string `json:"op"`
	ID      string `json:"id"`
	Item    *Item  `json:"item"`
	ToIndex *int   `json:"to_index"`
	Reason  string `json:"reason"`
}

// ApplyJSON applies the ops the model wrote: a JSON array of
// {op, id, item:{kind, route}, to_index, reason}. It never fails the turn: each
// op that cannot be applied is dropped with a reason, in order, and the rest
// stand. A code fence around the array is tolerated.
func (d *Desk) ApplyJSON(text string) (drops []string) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimPrefix(text, "json")
		text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "```"))
	}
	if text == "" {
		return nil
	}
	if text[0] != '[' {
		return []string{"the ops were not a JSON array"}
	}
	dec := json.NewDecoder(strings.NewReader(text))
	var raws []json.RawMessage
	if err := dec.Decode(&raws); err != nil {
		return []string{"the ops were not a JSON array: " + oneLineBounded(err.Error(), 120)}
	}
	if dec.More() {
		// text after the array does not cost its ops; the tail is only logged
		drops = append(drops, "text after the JSON array was ignored: "+oneLineBounded(text[dec.InputOffset():], 80))
	}
	for i, raw := range raws {
		if i >= maxAttempts {
			drops = append(drops, fmt.Sprintf("ops past %d were not read", maxAttempts))
			break
		}
		var m modelOp
		if err := json.Unmarshal(raw, &m); err != nil || bytes.TrimSpace(raw)[0] != '{' {
			drops = append(drops, fmt.Sprintf("op %d is not an op object", i))
			continue
		}
		if err := d.applyOne(m); err != nil {
			drops = append(drops, fmt.Sprintf("op %d (%s): %v", i, oneLineBounded(m.Op, 24), err))
		}
	}
	return drops
}

func (d *Desk) applyOne(m modelOp) error {
	item := Item{}
	if m.Item != nil {
		item = *m.Item
	}
	switch m.Op {
	case "play_next":
		return d.PlayNext(item, m.Reason)
	case "play_now":
		return d.PlayNow(item, m.Reason)
	case "enqueue":
		return d.Enqueue(item, m.Reason)
	case "move":
		if m.ToIndex == nil {
			return errors.New("a move needs to_index")
		}
		return d.Move(m.ID, *m.ToIndex, m.Reason)
	case "remove":
		return d.Remove(m.ID, m.Reason)
	case "skip":
		return d.Skip(m.Reason)
	case "previous":
		return d.Previous(m.Reason)
	case "clear":
		return d.Clear(m.Reason)
	}
	return fmt.Errorf("unknown op")
}

// Cut separates the model's spoken reply from the queue ops it appends after
// the request's fence line. The fence is exactly "<<<" + token + " queue>>>"
// (the route puts that line in the request and tells the model to end with it),
// and it counts only when it starts a line (after a newline or the start of the
// stream, spaces and tabs aside) and the next thing after it, past blank space,
// is "[" (the ops array), "{" (a malformed one, never read aloud) or a code
// fence "```" around the array. Anything else is speech, released whole:
// text the model quotes from the page, any other "<<<", and the exact line
// itself when the model merely echoes it in a sentence. It reads the stream as
// it arrives and holds back only what could still become the fence, plus the
// blank space after a fence, until the next byte decides.
type Cut struct {
	fence     string
	pending   string
	lineStart bool // what came before pending is the stream's start, or a newline and then only spaces and tabs
	fenced    bool
	ops       strings.Builder
}

// FenceLine is the line the model ends its reply with for a token.
func FenceLine(token string) string { return "<<<" + token + " queue>>>" }

// NewCut reads a stream for the fence of token.
func NewCut(token string) *Cut { return &Cut{fence: FenceLine(token), lineStart: true} }

// atLineStart reports whether position p in pending starts a line.
func (c *Cut) atLineStart(p int) bool {
	for p > 0 && (c.pending[p-1] == ' ' || c.pending[p-1] == '\t') {
		p--
	}
	if p == 0 {
		return c.lineStart
	}
	return c.pending[p-1] == '\n'
}

// findFence is the index in pending of a fence that starts a line, or -1.
func (c *Cut) findFence() int {
	for from := 0; from < len(c.pending); {
		k := strings.Index(c.pending[from:], c.fence)
		if k < 0 {
			return -1
		}
		p := from + k
		if c.atLineStart(p) {
			return p
		}
		from = p + 1
	}
	return -1
}

// emit returns out as speech and remembers whether it ended a line.
func (c *Cut) emit(out string) string {
	if out == "" {
		return out
	}
	if k := strings.LastIndexByte(out, '\n'); k >= 0 {
		c.lineStart = strings.Trim(out[k+1:], " \t") == ""
	} else {
		c.lineStart = c.lineStart && strings.Trim(out, " \t") == ""
	}
	return out
}

// Push takes the next delta and returns the speech in it that is certain.
func (c *Cut) Push(delta string) string {
	if c.fenced {
		c.ops.WriteString(delta)
		return ""
	}
	c.pending += delta
	var out strings.Builder
	for {
		i := c.findFence()
		if i < 0 {
			break
		}
		rest := c.pending[i+len(c.fence):]
		next := strings.TrimLeft(rest, " \t\r\n")
		switch {
		case next == "" || (len(next) < 3 && strings.HasPrefix("```", next)):
			// the fence, and not yet enough to say what follows it: hold it
			out.WriteString(c.emit(c.pending[:i]))
			c.pending = c.pending[i:]
			return out.String()
		case next[0] == '[' || next[0] == '{' || strings.HasPrefix(next, "```"):
			// an object is a malformed array: dropped and said, never read aloud
			out.WriteString(c.emit(c.pending[:i]))
			c.ops.WriteString(next)
			c.pending, c.fenced = "", true
			return out.String()
		}
		// an echo, not an ops fence: speech through the fence, then scan on
		out.WriteString(c.emit(c.pending[:i+len(c.fence)]))
		c.pending = c.pending[i+len(c.fence):]
	}
	keep := 0
	for k := min(len(c.fence)-1, len(c.pending)); k > 0; k-- {
		at := len(c.pending) - k
		if strings.HasSuffix(c.pending, c.fence[:k]) && c.atLineStart(at) {
			keep = k
			break
		}
	}
	out.WriteString(c.emit(c.pending[:len(c.pending)-keep]))
	c.pending = c.pending[len(c.pending)-keep:]
	return out.String()
}

// Flush ends the stream and returns any speech still held (a start of the
// fence that never completed, or a fence with no array after it, is speech).
func (c *Cut) Flush() string {
	out := c.pending
	c.pending = ""
	return out
}

// Fenced reports whether the model wrote the fence line.
func (c *Cut) Fenced() bool { return c.fenced }

// Ops is the text after the fence line.
func (c *Cut) Ops() string { return c.ops.String() }
