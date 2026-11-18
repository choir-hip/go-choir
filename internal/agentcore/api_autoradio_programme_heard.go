package agentcore

// The heard index (problem record
// docs/problems/autoradio-repeats-what-the-listener-already-heard-2026-11-18.md):
// the DJ is told, in the one call it already makes, what this listener heard.
//
// What counts as heard is one thing: a forecast of the Autoradio desk whose
// governing rule settlement (the heard rule, written from the phone's ended
// outcome by the run sheet's op path and nothing else) says true, with an air
// event as its evidence. A segment record alone, a skipped or cut entry, a
// left or failed entry (a no-verdict record, no settlement) and anything the
// DJ wrote about itself are not heard. The index is derived per call from the
// listener's own records; there is no store of its own.
//
// Reads, per programme call, for the heard index and nothing else: one list of
// the computer's newest segment records (the cites, title and words) and one
// list of its newest heard settlements (calibration.ListAutoradioHeard). No
// scorecard, no claim reads: the segment record names its claim by a
// deterministic id. Both lists are bounded by heardSegmentsRead and the call
// logs when either is full. A failed read never fails the call: the DJ is told
// the history is unavailable and shown no mark (unknown is never "never").

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	// heardWindow is how far back the DJ is shown what was said: the words of
	// a matched candidate and the recent list. The mark itself (heard, at
	// which version, on which day) outlives it for as long as the records do.
	// These bounds keep the input small; they are not a quota on calls.
	heardWindow = 14 * 24 * time.Hour
	// heardMatchedMax is the characters of what was said, per matched
	// candidate, cut at a sentence end.
	heardMatchedMax = 600
	// heardRecentMax is the most stories listed as recently heard and not
	// among the candidates; heardRecentLineMax the characters of each one's
	// words and heardRecentTitleMax of its title.
	heardRecentMax      = 12
	heardRecentLineMax  = 160
	heardRecentTitleMax = 100
	// heardSegmentsRead bounds the segment list, as listNotices bounds its own.

	tellingFull     = "full"
	tellingFollowup = "followup"
)

// heardSegmentsRead bounds the segment list, as listNotices bounds its own; a
// variable so a test can lower it.
var heardSegmentsRead = 2000

// heardSettlementsRead bounds the heard-settlement list likewise.
var heardSettlementsRead = 2000

type heardRef struct{ Route, Version string }

type heardSeg struct {
	At    time.Time // when the phone settled it (the op's own time)
	Title string
	Text  string
	Refs  []heardRef
}

type heardIndex struct {
	segs []heardSeg // newest first
}

const (
	heardNever = iota
	heardSame
	heardEarlier
)

type heardMark struct {
	Kind    int
	At      time.Time
	Version string // the version heard
	Text    string // empty when older than the window
}

// heardCounts are what the call's log line says. AllHeard is 1 when every
// candidate was heard at its current version.
type heardCounts struct{ Candidates, Never, Same, Earlier, Recent, AllHeard int }

// parsePubRef splits pub:<route>@v<version>.
func parsePubRef(ref string) (heardRef, bool) {
	rest, ok := strings.CutPrefix(ref, "pub:")
	if !ok {
		return heardRef{}, false
	}
	i := strings.LastIndex(rest, "@v")
	if i <= 0 {
		return heardRef{}, false
	}
	return heardRef{Route: rest[:i], Version: rest[i+2:]}, true
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// atThisVersion: the heard version is this one, or (whole numbers) a later one.
func atThisVersion(heard, current string) bool {
	if heard == current {
		return true
	}
	if isDigits(heard) && isDigits(current) {
		h, c := strings.TrimLeft(heard, "0"), strings.TrimLeft(current, "0")
		return len(h) > len(c) || (len(h) == len(c) && h >= c)
	}
	return false
}

// cutAtSentence cuts s to at most max characters, at the last sentence end
// that leaves a third of the room used, else at a word.
func cutAtSentence(s string, max int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	end := -1
	for i := max - 1; i >= 0; i-- {
		if (r[i] == '.' || r[i] == '!' || r[i] == '?') && (i+1 >= len(r) || r[i+1] == ' ') {
			end = i + 1
			break
		}
	}
	if end >= max/3 {
		return strings.TrimSpace(string(r[:end]))
	}
	cut := max
	for cut > 0 && r[cut-1] != ' ' {
		cut--
	}
	if cut < max/3 {
		cut = max
	}
	return strings.TrimSpace(string(r[:cut])) + "…"
}

// errClass names the kind of an error without its words.
func errClass(err error) string {
	for {
		u := errors.Unwrap(err)
		if u == nil {
			break
		}
		err = u
	}
	return fmt.Sprintf("%T", err)
}

// listenerLoc is the listener's zone: the request's tz, else the offset of its
// local_time. nil when the request names neither: the DJ is then given no
// date, only how many days ago (counted in UTC).
func (c *programmeCall) listenerLoc() *time.Location {
	if c.req.TZ != "" {
		if loc, err := time.LoadLocation(c.req.TZ); err == nil {
			return loc
		}
	}
	if c.req.LocalTime != "" {
		if at, err := time.Parse(time.RFC3339, c.req.LocalTime); err == nil {
			_, off := at.Zone()
			return time.FixedZone("", off)
		}
	}
	return nil
}

// dayPhrase says when, in the listener's day: "on 2026-11-17" with a zone,
// else "earlier" or "<n> days ago" by UTC dates (no weekday, no date).
func dayPhrase(loc *time.Location, at, now time.Time) string {
	if loc != nil {
		return "on " + at.In(loc).Format("2006-01-02")
	}
	y1, m1, d1 := at.UTC().Date()
	y2, m2, d2 := now.UTC().Date()
	days := int(time.Date(y2, m2, d2, 0, 0, 0, 0, time.UTC).Sub(time.Date(y1, m1, d1, 0, 0, 0, 0, time.UTC)).Hours() / 24)
	switch {
	case days <= 0:
		return "earlier"
	case days == 1:
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}

// buildHeardSegs joins the segment records with the heard settlements (by the
// claim id each segment names) into the heard index, newest first. A cite
// whose role is mentioned is not a telling and is not kept; a quoted cite is
// (the story relayed what that publication said, and the words are shown, so
// the DJ can judge). Pure: no reads.
func buildHeardSegs(segs map[string]types.CommitmentRecord, heard map[string]time.Time) []heardSeg {
	var out []heardSeg
	for _, rec := range segs {
		if rec.Directive == nil || rec.SubjectRefs["forecast"] != "true" {
			continue
		}
		switch rec.SubjectRefs["kind"] {
		case "story", "update", "headlines":
		default:
			continue
		}
		claim := calibration.ClaimRecordID(calibration.AutoradioAgent, rec.SubjectRefs["stretch_id"]+"/"+rec.SubjectRefs["segment_id"], "")
		at, ok := heard[claim]
		if !ok {
			continue
		}
		s := heardSeg{At: at, Title: strings.TrimSpace(rec.Directive.Objective), Text: rec.Directive.Body}
		var cites []programmeCite
		_ = json.Unmarshal([]byte(rec.SubjectRefs["cites"]), &cites)
		for _, ct := range cites {
			if ct.Role == "mentioned" {
				continue
			}
			if r, ok := parsePubRef(ct.Ref); ok {
				s.Refs = append(s.Refs, r)
			}
		}
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].At.Equal(out[j].At) {
			return out[i].At.After(out[j].At)
		}
		return out[i].Title < out[j].Title
	})
	return out
}

// readHeard reads the two lists at the same time (neither needs the other) and
// notes the full ones for the log. Either failing fails the read.
func (c *programmeCall) readHeard(ctx context.Context) (segs map[string]types.CommitmentRecord, heard map[string]time.Time, err error) {
	var (
		wg                sync.WaitGroup
		recs              []types.CommitmentRecord
		segUndec, segRows int
		segErr, heardErr  error
		rd                calibration.HeardRead
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		// Content segments only (the forecast flag): openings and transitions
		// are never heard-settled and would use up the page.
		recs, segUndec, segRows, segErr = c.ledger.ListCommitmentRecordsByBodyOfComputer(ctx, c.ownerID, c.rt.cfg.ComputerID,
			map[string]string{"$.subject_refs.record_kind": kindSegment, "$.subject_refs.forecast": "true"}, heardSegmentsRead)
	}()
	go func() {
		defer wg.Done()
		heard, rd, heardErr = calibration.ListAutoradioHeard(ctx, c.ledger, c.ownerID, c.rt.cfg.ComputerID, heardSettlementsRead)
	}()
	wg.Wait()
	if segErr != nil {
		return nil, nil, fmt.Errorf("read the segments: %w", segErr)
	}
	if heardErr != nil {
		return nil, nil, fmt.Errorf("read the heard settlements: %w", heardErr)
	}
	if segRows >= heardSegmentsRead {
		c.heardFull = append(c.heardFull, fmt.Sprintf("kind=segment rows=%d undecodable=%d limit=%d", segRows, segUndec, heardSegmentsRead))
	}
	if rd.Full {
		c.heardFull = append(c.heardFull, fmt.Sprintf("kind=settlement rows=%d undecodable=%d unparsable=%d limit=%d", rd.Rows, rd.Undecodable, rd.Unparsable, rd.Limit))
	}
	c.heardUnparsable = rd.Unparsable
	segs = make(map[string]types.CommitmentRecord, len(recs))
	for _, r := range recs {
		if strings.HasPrefix(r.RecordID, segmentRecordPrefix) {
			segs[r.RecordID] = r
		}
	}
	return segs, heard, nil
}

// loadHeard derives the heard index from the listener's records and marks the
// call's candidates and recent list in the input. On an error nothing is set.
func (c *programmeCall) loadHeard(ctx context.Context, in *programmeInput) error {
	segRecs, heard, err := c.readHeard(ctx)
	if err != nil {
		return err
	}
	c.segRecs = segRecs
	ix := heardIndex{segs: buildHeardSegs(segRecs, heard)}

	now := time.Now()
	loc := c.listenerLoc()
	marks := map[string]heardMark{}
	routes := map[string]bool{}
	var counts heardCounts
	for i := range in.candidates {
		cand := &in.candidates[i]
		if cand.isTake() {
			continue
		}
		routes[cand.Route] = true
		counts.Candidates++
		_, cv, _ := strings.Cut(strings.TrimPrefix(cand.ref, "pub:"+cand.Route), "@v")
		m := ix.mark(cand.Route, cv, now)
		marks[cand.ref] = m
		switch m.Kind {
		case heardSame:
			counts.Same++
		case heardEarlier:
			counts.Earlier++
		default:
			counts.Never++
		}
	}
	if counts.Candidates > 0 && counts.Same == counts.Candidates {
		counts.AllHeard = 1
	}
	var recent []string
	seen := map[string]bool{}
	for _, s := range ix.segs {
		if len(recent) >= heardRecentMax || now.Sub(s.At) > heardWindow {
			break
		}
		key, among := s.Title, false
		for _, r := range s.Refs {
			if routes[r.Route] {
				among = true
			}
		}
		if len(s.Refs) > 0 {
			key = s.Refs[0].Route
		}
		if among || seen[key] {
			continue
		}
		seen[key] = true
		recent = append(recent, fmt.Sprintf("%s | %s | %s", strings.TrimPrefix(dayPhrase(loc, s.At, now), "on "),
			cutAtSentence(safeLine(s.Title), heardRecentTitleMax), cutAtSentence(safeLine(s.Text), heardRecentLineMax)))
	}
	counts.Recent = len(recent)
	in.heardMarks, in.recent, in.heardCounts, in.heardLoc = marks, recent, counts, loc
	return nil
}

// mark is what the listener heard of a route, as it stands at version cv: the
// newest telling at this version (or a later one) wins, else the newest
// earlier one.
func (ix *heardIndex) mark(route, cv string, now time.Time) heardMark {
	var earlier *heardMark
	for _, s := range ix.segs {
		for _, r := range s.Refs {
			if r.Route != route {
				continue
			}
			m := heardMark{Kind: heardSame, At: s.At, Version: r.Version}
			if !atThisVersion(r.Version, cv) {
				m.Kind = heardEarlier
			}
			if now.Sub(s.At) <= heardWindow {
				m.Text = cutAtSentence(safeLine(s.Text), heardMatchedMax)
			}
			if m.Kind == heardSame {
				return m
			}
			if earlier == nil {
				earlier = &m
			}
		}
	}
	if earlier != nil {
		return *earlier
	}
	return heardMark{Kind: heardNever}
}

// line renders a candidate's mark for the DJ's input. A same-version mark
// prints the version actually heard (it can be later than the candidate's).
func (m heardMark) line(cv string, loc *time.Location, now time.Time) string {
	if m.Kind == heardNever {
		return "heard: never"
	}
	var b strings.Builder
	when := dayPhrase(loc, m.At, now)
	if m.Kind == heardSame {
		fmt.Fprintf(&b, "heard: at this version (v%s) %s.", m.Version, when)
	} else {
		fmt.Fprintf(&b, "heard: at an earlier version (v%s, now v%s) %s.", m.Version, cv, when)
	}
	if m.Text != "" {
		b.WriteString(" Said then: " + m.Text)
	} else {
		b.WriteString(" (The words are no longer kept here.)")
	}
	return b.String()
}

// logHeard writes the call's one line about the heard index: counts only, no
// words; then one line for each read that was full.
func (c *programmeCall) logHeard() {
	if c.input == nil {
		return
	}
	for _, f := range c.heardFull {
		log.Printf("autoradio heard: read full stretch=%s request=%s %s", c.stretchID, c.req.RequestID, f)
	}
	if c.input.heardUnavailable {
		log.Printf("autoradio heard: unavailable stretch=%s request=%s class=%s followups=%d", c.stretchID, c.req.RequestID, c.input.heardClass, c.followups)
		return
	}
	n := c.input.heardCounts
	log.Printf("autoradio heard: stretch=%s request=%s candidates=%d never=%d same=%d earlier=%d recent=%d all_heard=%d unparsable=%d followups=%d",
		c.stretchID, c.req.RequestID, n.Candidates, n.Never, n.Same, n.Earlier, n.Recent, n.AllHeard, c.heardUnparsable, c.followups)
}
