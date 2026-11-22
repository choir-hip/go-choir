// Package offlineslice is the replay harness of rung 0 of the offline slice
// (docs/design/first-design-offline-slice-plan-2026-11-22.md). It replays a
// week of captures in arrival order and lists, for each, the candidate
// subjects the rules alone propose, with no model: known labels and aliases,
// names not yet known, and the subjects of the nearest earlier captures by
// vector. The harness owns the rules that keep the replay honest (arrival
// order, one copy of a URL, no hindsight, earlier hours only). The candidate
// rules themselves are the deployed code of internal/platform and
// internal/entitytext, reached through the Rules interface (an in-package
// adapter in internal/platform, offline_slice_replay_test.go).
package offlineslice

import (
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/capturefts"
)

// Capture is one capture as exported.
type Capture struct {
	ID        string
	Fetched   time.Time
	Published string
	Host      string
	URLKey    string
	CanonKey  string
	Language  string
	SourceID  string
	Title     string
	TextHash  string
	Lead      string
	Vec       []float32 // unit length; nil when the capture has no stored vector
}

// Entity is one choir.entity row.
type Entity struct {
	Canonical string
	EntityID  string
	Name      string
	Type      string
	Aliases   []string
	Created   time.Time
	Updated   time.Time
	Tombstone bool
}

// Stop is one learned stopword row.
type Stop struct {
	Key       string
	Active    bool
	Created   time.Time
	Activated time.Time
	Revoked   time.Time
}

// AliasPolicy says which aliases an as-of lexicon may carry. Alias times are
// not stored, only the row's creation and last update.
type AliasPolicy int

const (
	// AliasesAll keeps every alias the row carries today: a row created before
	// the day may carry an alias learned after it (hindsight).
	AliasesAll AliasPolicy = iota
	// AliasesAsOf keeps a row's aliases only if the row was not modified on or
	// after the day: no hindsight, at the price of dropping real earlier ones.
	AliasesAsOf
	// AliasesNone uses names only.
	AliasesNone
)

// Dedupe sorts captures into arrival order (fetch time, then id) and keeps the
// first capture of every URL: a later capture that shares its url_key or its
// canonical_key with a kept one is dropped. It returns the kept captures and
// the number dropped.
func Dedupe(caps []Capture) ([]Capture, int) {
	cs := append([]Capture(nil), caps...)
	sort.SliceStable(cs, func(i, j int) bool {
		if !cs[i].Fetched.Equal(cs[j].Fetched) {
			return cs[i].Fetched.Before(cs[j].Fetched)
		}
		return cs[i].ID < cs[j].ID
	})
	seenURL, seenCanon := map[string]bool{}, map[string]bool{}
	out := cs[:0:0]
	dropped := 0
	for _, c := range cs {
		if (c.URLKey != "" && seenURL[c.URLKey]) || (c.CanonKey != "" && seenCanon[c.CanonKey]) {
			dropped++
			continue
		}
		if c.URLKey != "" {
			seenURL[c.URLKey] = true
		}
		if c.CanonKey != "" {
			seenCanon[c.CanonKey] = true
		}
		out = append(out, c)
	}
	return out, dropped
}

// EntitiesAsOf is the lexicon's entities as they could have been known on the
// morning of day (UTC): live rows created strictly before it.
func EntitiesAsOf(ents []Entity, day time.Time, p AliasPolicy) []Entity {
	var out []Entity
	for _, e := range ents {
		if e.Tombstone || !e.Created.Before(day) {
			continue
		}
		switch p {
		case AliasesNone:
			e.Aliases = nil
		case AliasesAsOf:
			if !e.Updated.Before(day) {
				e.Aliases = nil
			}
		}
		out = append(out, e)
	}
	return out
}

// StopsAsOf is the keys the learned stoplist stopped on the morning of day: a
// row is in force from its activation (its creation when none is written)
// until its revocation. An inactive row with no revocation time never was.
func StopsAsOf(stops []Stop, day time.Time) []string {
	var out []string
	for _, s := range stops {
		start := s.Activated
		if start.IsZero() {
			start = s.Created
		}
		if !start.Before(day) {
			continue
		}
		if !s.Active {
			if s.Revoked.IsZero() || !day.Before(s.Revoked) {
				continue
			}
		}
		out = append(out, s.Key)
	}
	return out
}

// Neighbour is one nearest earlier capture.
type Neighbour struct {
	Index int
	Cos   float64
}

func normalize(v []float32) []float32 {
	var n float64
	for _, x := range v {
		n += float64(x) * float64(x)
	}
	if n == 0 {
		return v
	}
	inv := float32(1 / math.Sqrt(n))
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = x * inv
	}
	return out
}

func hourOf(t time.Time) time.Time { return t.UTC().Truncate(time.Hour) }

// Neighbours finds, for each capture (in arrival order), its k nearest by
// cosine among the captures of strictly earlier hours that have a vector.
// The live system works in hourly batches, so what is known of a capture's
// neighbours is what earlier hours resolved. A capture without a vector has
// none. Brute force, in parallel.
func Neighbours(caps []Capture, k int) [][]Neighbour {
	out := make([][]Neighbour, len(caps))
	// hourStart[i]: the index of the first capture of i's hour; everything
	// before it is an earlier hour (caps are in arrival order).
	hourStart := make([]int, len(caps))
	for i := range caps {
		if i > 0 && hourOf(caps[i].Fetched).Equal(hourOf(caps[i-1].Fetched)) {
			hourStart[i] = hourStart[i-1]
		} else {
			hourStart[i] = i
		}
	}
	var wg sync.WaitGroup
	work := make(chan int, 256)
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				if caps[i].Vec == nil {
					continue
				}
				top := make([]Neighbour, 0, k+1)
				vi := caps[i].Vec
				for j := 0; j < hourStart[i]; j++ {
					vj := caps[j].Vec
					if vj == nil || len(vj) != len(vi) {
						continue
					}
					var s float32
					for d := range vi {
						s += vi[d] * vj[d]
					}
					if len(top) == k && float64(s) <= top[k-1].Cos {
						continue
					}
					n := Neighbour{Index: j, Cos: float64(s)}
					pos := sort.Search(len(top), func(x int) bool { return top[x].Cos < n.Cos })
					top = append(top, Neighbour{})
					copy(top[pos+1:], top[pos:])
					top[pos] = n
					if len(top) > k {
						top = top[:k]
					}
				}
				out[i] = top
			}
		}()
	}
	for i := range caps {
		work <- i
	}
	close(work)
	wg.Wait()
	return out
}

// ---- the rules, and what they return ----

// Cand is one existing entity a mention may be.
type Cand struct {
	Mention   string `json:"mention"`
	EntityID  string `json:"entity_id"`
	Name      string `json:"name"`
	Via       string `json:"via"`
	Ambiguous bool   `json:"ambiguous,omitempty"`
}

// Mention is one name found in a capture and the entities the rules shortlist for it.
type Mention struct {
	Key, Phrase string
	Hit         bool // the text matched a known label or alias
	InTitle     bool
	InLead      bool
	Count       int
	Score       float64
	Acronym     bool
	Ambiguous   bool
	Cands       []Cand
}

// Rules are the candidate rules. Prepare sets the as-of lexicon for a day
// (UTC midnight); Mentions lists a capture's mentions given the entity ids of
// its neighbours' subjects.
type Rules interface {
	Prepare(day time.Time, ents []Entity, stops []string)
	Mentions(title, lead string, known map[string]bool) []Mention
}

// Salient: the mention is a subject the capture is plausibly about, by the
// rules alone: it is in the title or occurs twice in the body.
func Salient(m Mention) bool { return m.InTitle || m.Count >= 2 }

// Prop is a name in a capture that no entity of the day's lexicon is a
// candidate for: source (b).
type Prop struct {
	Key     string  `json:"key"`
	Phrase  string  `json:"phrase"`
	Score   float64 `json:"score"`
	InTitle bool    `json:"in_title,omitempty"`
	Count   int     `json:"count"`
	Acronym bool    `json:"acronym,omitempty"`
}

// NSubj is a subject taken from the nearest earlier captures: source (c).
type NSubj struct {
	Key     string  `json:"key"` // "e:<entity id>" or "n:<name key>"
	Label   string  `json:"label"`
	Support int     `json:"support"`  // neighbours that hold it
	CosSum  float64 `json:"cos_sum"`  // sum of those neighbours' cosines
	CosBest float64 `json:"cos_best"` // best of them
}

// Result is what the rules propose for one capture.
type Result struct {
	ID       string    `json:"id"`
	Day      string    `json:"day"`
	Fetched  time.Time `json:"fetched"`
	Outlet   string    `json:"outlet,omitempty"`
	Title    string    `json:"title"`
	Lead     string    `json:"lead,omitempty"`
	HasVec   bool      `json:"has_vec"`
	Mentions int       `json:"mentions"`
	// A: entities found by a known label or alias (a hit mention's candidates).
	A []Cand `json:"a"`
	// F: entities a name not found as a label may be (near, subset, initials, neighbour forms).
	F []Cand `json:"f"`
	// B: names with no candidate.
	B []Prop `json:"b"`
	// C: subjects of the nearest earlier captures.
	C      []NSubj `json:"c"`
	NNeigh int     `json:"n_neigh"`
	MaxCos float64 `json:"max_cos"`
}

// MaxNeighbourSubjects bounds source (c) per capture, as the shortlist per mention is bounded.
const MaxNeighbourSubjects = 8

type subject struct{ key, label string }

func subjectsOf(ms []Mention) []subject {
	var out []subject
	seen := map[string]bool{}
	for _, m := range ms {
		if !Salient(m) {
			continue
		}
		var s subject
		switch {
		case len(m.Cands) > 0 && m.Cands[0].Via == "exact":
			s = subject{"e:" + m.Cands[0].EntityID, m.Cands[0].Name}
		default:
			s = subject{"n:" + m.Key, m.Phrase}
		}
		if !seen[s.key] {
			seen[s.key] = true
			out = append(out, s)
		}
	}
	return out
}

// Replay runs the rules over caps (already in arrival order, see Dedupe)
// one capture at a time. The lexicon is rebuilt at each new UTC day from the
// entities and stopwords that existed before it. Subjects of neighbours come
// only from captures of earlier hours the replay itself has already done.
func Replay(caps []Capture, ents []Entity, stops []Stop, rules Rules, policy AliasPolicy, k int) []Result {
	nb := Neighbours(caps, k)
	subj := make([][]subject, len(caps))
	out := make([]Result, len(caps))
	var day time.Time
	for i, c := range caps {
		d := c.Fetched.UTC().Truncate(24 * time.Hour)
		if !d.Equal(day) {
			day = d
			rules.Prepare(day, EntitiesAsOf(ents, day, policy), StopsAsOf(stops, day))
		}
		// the entities of the neighbours' subjects
		known := map[string]bool{}
		for _, n := range nb[i] {
			for _, s := range subj[n.Index] {
				if strings.HasPrefix(s.key, "e:") {
					known[strings.TrimPrefix(s.key, "e:")] = true
				}
			}
		}
		ms := rules.Mentions(c.Title, c.Lead, known)
		subj[i] = subjectsOf(ms)
		r := Result{ID: c.ID, Day: day.Format("2006-01-02"), Fetched: c.Fetched, Outlet: c.Host, Title: c.Title, Lead: c.Lead,
			HasVec: c.Vec != nil, Mentions: len(ms), A: []Cand{}, F: []Cand{}, B: []Prop{}, C: []NSubj{}, NNeigh: len(nb[i])}
		have := map[string]bool{}
		for _, m := range ms {
			if len(m.Cands) == 0 {
				r.B = append(r.B, Prop{Key: m.Key, Phrase: m.Phrase, Score: m.Score, InTitle: m.InTitle, Count: m.Count, Acronym: m.Acronym})
				have["n:"+m.Key] = true
				continue
			}
			for _, cd := range m.Cands {
				cd.Mention = m.Phrase
				if m.Hit {
					r.A = append(r.A, cd)
				} else {
					r.F = append(r.F, cd)
				}
				have["e:"+cd.EntityID] = true
			}
			if !m.Hit {
				have["n:"+m.Key] = true
			}
		}
		agg := map[string]*NSubj{}
		for _, n := range nb[i] {
			if n.Cos > r.MaxCos {
				r.MaxCos = n.Cos
			}
			for _, s := range subj[n.Index] {
				if have[s.key] {
					continue
				}
				a := agg[s.key]
				if a == nil {
					a = &NSubj{Key: s.key, Label: s.label}
					agg[s.key] = a
				}
				a.Support++
				a.CosSum += n.Cos
				if n.Cos > a.CosBest {
					a.CosBest = n.Cos
				}
			}
		}
		for _, a := range agg {
			r.C = append(r.C, *a)
		}
		sort.Slice(r.C, func(x, y int) bool {
			if r.C[x].CosSum != r.C[y].CosSum {
				return r.C[x].CosSum > r.C[y].CosSum
			}
			return r.C[x].Key < r.C[y].Key
		})
		if len(r.C) > MaxNeighbourSubjects {
			r.C = r.C[:MaxNeighbourSubjects]
		}
		out[i] = r
	}
	return out
}

// LeadRunes bounds the lead as the ready list bounds it (readyLeadRunes).
const LeadRunes = 2400

// LeadOf is a capture's lead as the ready list shows it: the first passage
// capturefts.Passages cuts from the text's prefix, invalid bytes repaired,
// clipped to LeadRunes. (The deployed captureLeadPassage reads a 256 KiB
// prefix; the export holds the first 32 KiB, and the first passage ends
// long before that.)
func LeadOf(prefix []byte) string {
	text := string(prefix)
	spans := capturefts.Passages(text)
	if len(spans) == 0 {
		return ""
	}
	p := text[spans[0].Start:spans[0].End]
	if !utf8.ValidString(p) {
		p = string([]rune(p))
	}
	if utf8.RuneCountInString(p) > LeadRunes {
		p = string([]rune(p)[:LeadRunes])
	}
	return p
}
