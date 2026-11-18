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
// Reads, per programme call: the scorecard (every Autoradio claim joined with
// its settlements; the notebook's cases and counts use the same read) and one
// list of the segment records (the cites, title and words of a segment are
// there; the notebook's cases take their titles from the same list).

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
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
	heardSegmentsRead = 5000

	tellingFull     = "full"
	tellingFollowup = "followup"
)

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
	Day     string
	Version string // the version heard
	Text    string // empty when older than the window
}

// heardCounts are what the call's log line says.
type heardCounts struct{ Candidates, Never, Same, Earlier, Recent int }

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

// scorecard reads the Autoradio claims with their settlements once per call.
func (c *programmeCall) scorecard(ctx context.Context) (*calibration.Scorecard, error) {
	if c.card != nil {
		return c.card, nil
	}
	card, err := calibration.Reader{Ledger: c.rt.store}.Scorecard(ctx, c.ownerID, c.rt.cfg.ComputerID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("read the settled forecasts: %w", err)
	}
	c.card = &card
	return c.card, nil
}

// segmentRecords lists the listener's segment records once per call, by id.
func (c *programmeCall) segmentRecords(ctx context.Context) (map[string]types.CommitmentRecord, error) {
	if c.segRecs != nil {
		return c.segRecs, nil
	}
	recs, err := c.ledger.ListCommitmentRecordsBySubjectRef(ctx, c.ownerID, c.rt.cfg.ComputerID, "record_kind", kindSegment, heardSegmentsRead)
	if err != nil {
		return nil, fmt.Errorf("read the segments: %w", err)
	}
	m := make(map[string]types.CommitmentRecord, len(recs))
	for _, r := range recs {
		if strings.HasPrefix(r.RecordID, segmentRecordPrefix) && r.Directive != nil {
			m[r.RecordID] = r
		}
	}
	c.segRecs = m
	return m, nil
}

// loadHeard derives the heard index from the listener's records and marks the
// call's candidates and recent list in the input.
func (c *programmeCall) loadHeard(ctx context.Context, in *programmeInput) error {
	card, err := c.scorecard(ctx)
	if err != nil {
		return err
	}
	segRecs, err := c.segmentRecords(ctx)
	if err != nil {
		return err
	}
	var ix heardIndex
	for _, v := range card.Claims {
		if v.Agent != calibration.AutoradioAgent || (v.Project != "story" && v.Project != "update" && v.Project != "headlines") {
			continue
		}
		g, ok := calibration.GoverningAutoradioHeard(v.Settlements)
		if !ok {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, g.SettledAt)
		if err != nil {
			continue
		}
		rec, ok := segRecs[segmentRecordID(tagOf(v.Tags, "stretch:"), tagOf(v.Tags, "seg:"))]
		if !ok {
			continue
		}
		s := heardSeg{At: at.UTC(), Title: strings.TrimSpace(rec.Directive.Objective), Text: rec.Directive.Body}
		var cites []programmeCite
		_ = json.Unmarshal([]byte(rec.SubjectRefs["cites"]), &cites)
		for _, ct := range cites {
			if r, ok := parsePubRef(ct.Ref); ok {
				s.Refs = append(s.Refs, r)
			}
		}
		ix.segs = append(ix.segs, s)
	}
	sort.SliceStable(ix.segs, func(i, j int) bool { return ix.segs[i].At.After(ix.segs[j].At) })

	now := time.Now()
	in.heardMarks = map[string]heardMark{}
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
		in.heardMarks[cand.ref] = m
		switch m.Kind {
		case heardSame:
			counts.Same++
		case heardEarlier:
			counts.Earlier++
		default:
			counts.Never++
		}
	}
	seen := map[string]bool{}
	for _, s := range ix.segs {
		if len(in.recent) >= heardRecentMax || now.Sub(s.At) > heardWindow {
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
		in.recent = append(in.recent, fmt.Sprintf("%s | %s | %s", s.At.Format("2006-01-02"),
			cutAtSentence(safeLine(s.Title), heardRecentTitleMax), cutAtSentence(safeLine(s.Text), heardRecentLineMax)))
	}
	counts.Recent = len(in.recent)
	in.heardCounts = counts
	return nil
}

// mark is what the listener heard of a route, as it stands at version cv: the
// newest telling at this version (or later) wins, else the newest earlier one.
func (ix *heardIndex) mark(route, cv string, now time.Time) heardMark {
	var earlier *heardMark
	for _, s := range ix.segs {
		for _, r := range s.Refs {
			if r.Route != route {
				continue
			}
			m := heardMark{Kind: heardSame, Day: s.At.Format("2006-01-02"), Version: r.Version}
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

// line renders a candidate's mark for the DJ's input.
func (m heardMark) line(cv string) string {
	if m.Kind == heardNever {
		return "heard: never"
	}
	var b strings.Builder
	if m.Kind == heardSame {
		fmt.Fprintf(&b, "heard: at this version (v%s) on %s.", cv, m.Day)
	} else {
		fmt.Fprintf(&b, "heard: at an earlier version (v%s, now v%s) on %s.", m.Version, cv, m.Day)
	}
	if m.Text != "" {
		b.WriteString(" Said then: " + m.Text)
	} else {
		b.WriteString(" (The words are no longer kept here.)")
	}
	return b.String()
}

// logHeard writes the call's one line about the heard index: counts only.
func (c *programmeCall) logHeard() {
	if c.input == nil {
		return
	}
	n := c.input.heardCounts
	log.Printf("autoradio heard: stretch=%s request=%s candidates=%d never=%d same=%d earlier=%d recent=%d followups=%d",
		c.stretchID, c.req.RequestID, n.Candidates, n.Never, n.Same, n.Earlier, n.Recent, c.followups)
}
