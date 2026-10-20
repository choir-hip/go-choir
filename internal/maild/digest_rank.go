package maild

import (
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
)

// FrontRow is one row of corpusd's internal front list
// (/internal/platform/publications/front), as much of it as the daily digest
// reads. The counts are float64 pointers because the page reads them as
// JavaScript numbers; a nil pointer is "unknown", as undefined is there.
type FrontRow struct {
	PublicationID  string   `json:"publication_id"`
	Title          string   `json:"title"`
	Dek            string   `json:"dek"`
	UpdatedAt      string   `json:"updated_at"`
	VersionCount   float64  `json:"version_count"`
	LatestRevision int      `json:"latest_revision"`
	Importance     *float64 `json:"importance,omitempty"`
	Promo          bool     `json:"promo,omitempty"`
	SourceCount    *float64 `json:"source_count,omitempty"`
	Words          *float64 `json:"words,omitempty"`
	Section        string   `json:"section,omitempty"`
	Route          struct {
		Bare      string `json:"bare"`
		Permalink string `json:"permalink"`
	} `json:"route"`
}

// The rank below is a Go mirror of rankFrontStories in
// frontend/src/lib/autopaper.ts, the order the front page shows. It is a
// knowing duplicate of one rule (docs/problems/autopaper-daily-digest-
// phase1-2026-10-20.md, problem 1), held to the page by a parity test that
// ranks one fixture with both. Change the two together. The substrate fix is
// a server-side rank that both read. Like the page's, it reads no clock: age
// is measured from the newest update in the list (the edition time).
//
// Omitted on purpose: the reader's section habit (sectionShares), which is
// per-device state a mail has none of.
const (
	digestFreshWindowMs      = int64(6 * 60 * 60 * 1000)
	digestVersionLiftMs      = int64(45 * 60 * 1000)
	digestMaxLiftedVersions  = 4
	digestHalfLifeMs         = float64(24 * 60 * 60 * 1000)
	digestDefaultImportance  = 4.0
	digestThinWords          = 150
	digestLeadMinSources     = 2
	digestUnparsableUpdateMs = int64(math.MinInt64)
)

// updatedMs mirrors updatedMs in autopaper.ts: the update time in
// milliseconds, or ok=false when it does not parse (the page's -Infinity).
func (r FrontRow) updatedMs() (int64, bool) {
	raw := strings.TrimSpace(r.UpdatedAt)
	if raw == "" {
		return digestUnparsableUpdateMs, false
	}
	// corpusd emits RFC 3339. A date-only string is UTC on the page too; an
	// unzoned date-time is local time there, so it is left unparsed here.
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UnixMilli(), true
		}
	}
	return digestUnparsableUpdateMs, false
}

func (r FrontRow) importance() (float64, bool) {
	if r.Importance == nil {
		return 0, false
	}
	n := *r.Importance
	if math.IsNaN(n) || math.IsInf(n, 0) || n < 1 || n > 10 {
		return 0, false
	}
	return n, true
}

func finiteCount(p *float64) (float64, bool) {
	if p == nil {
		return 0, false
	}
	n := *p
	if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
		return 0, false
	}
	return n, true
}

// depthFactor mirrors depthFactor: how much of the story's importance our
// article earns, from its sources and length. 1 when unknown.
func (r FrontRow) depthFactor() float64 {
	factor := 1.0
	if sources, ok := finiteCount(r.SourceCount); ok {
		switch {
		case sources <= 1:
			factor *= 0.5
		case sources < 3:
			factor *= 0.75
		}
	}
	if words, ok := finiteCount(r.Words); ok && words < digestThinWords {
		factor *= 0.8
	}
	return factor
}

// isDemoted mirrors isDemotedStory: promotional, or a leaked instruction
// ("Open a story on ...") used as a title.
func (r FrontRow) isDemoted() bool {
	if r.Promo {
		return true
	}
	title := strings.TrimLeftFunc(r.Title, func(c rune) bool { return unicode.IsSpace(c) || c == 0xFEFF })
	return strings.HasPrefix(strings.ToLower(title), "open a story")
}

// canLead mirrors canLead: not demoted, and at least two sources (or depth
// unknown).
func (r FrontRow) canLead() bool {
	if r.isDemoted() {
		return false
	}
	sources, ok := finiteCount(r.SourceCount)
	return !ok || sources >= digestLeadMinSources
}

// frontRowsUseImportance mirrors usesImportanceRank: at least half the rows
// carry a usable importance.
func frontRowsUseImportance(rows []FrontRow) bool {
	if len(rows) == 0 {
		return false
	}
	scored := 0
	for _, r := range rows {
		if _, ok := r.importance(); ok {
			scored++
		}
	}
	return scored*2 >= len(rows)
}

func frontEditionMs(rows []FrontRow) (int64, bool) {
	var newest int64
	found := false
	for _, r := range rows {
		if ms, ok := r.updatedMs(); ok && (!found || ms > newest) {
			newest, found = ms, true
		}
	}
	return newest, found
}

// recencyProminence mirrors frontProminence: recency plus a lift for versions
// when the update is inside the fresh window of the edition. ok=false stands
// for the page's -Infinity.
func (r FrontRow) recencyProminence(edition int64, haveEdition bool) (float64, bool) {
	updated, ok := r.updatedMs()
	if !ok {
		return 0, false
	}
	extra := math.Floor(r.VersionCount) - 1
	if math.IsNaN(extra) || extra < 0 {
		extra = 0
	}
	lift := int64(0)
	if haveEdition && edition-updated <= digestFreshWindowMs {
		lift = int64(math.Min(extra, digestMaxLiftedVersions)) * digestVersionLiftMs
	}
	return float64(updated + lift), true
}

// importanceProminence mirrors importanceProminence without the section lift.
func (r FrontRow) importanceProminence(edition int64, haveEdition bool) (float64, bool) {
	updated, ok := r.updatedMs()
	if !ok {
		return 0, false
	}
	age := 0.0
	if haveEdition {
		age = math.Max(0, float64(edition-updated))
	}
	importance, scored := r.importance()
	if !scored {
		importance = digestDefaultImportance
	}
	return importance * r.depthFactor() * math.Pow(0.5, age/digestHalfLifeMs), true
}

type rankedRow struct {
	row     FrontRow
	demoted int
	score   float64 // -Inf when the date does not parse
	updated float64 // -Inf when the date does not parse
}

func sortKey(r FrontRow, score func(FrontRow) (float64, bool)) (float64, float64) {
	s, ok := score(r)
	if !ok {
		s = math.Inf(-1)
	}
	u := math.Inf(-1)
	if ms, ok := r.updatedMs(); ok {
		u = float64(ms)
	}
	return s, u
}

// rankFrontRows is rankFrontStories: the rows in front-page order, most
// prominent first. Pure and stable; the input is not modified.
func rankFrontRows(rows []FrontRow) []FrontRow {
	if len(rows) == 0 {
		return []FrontRow{}
	}
	edition, haveEdition := frontEditionMs(rows)
	byImportance := frontRowsUseImportance(rows)
	ranked := make([]rankedRow, len(rows))
	for i, r := range rows {
		score, updated := sortKey(r, func(r FrontRow) (float64, bool) {
			if byImportance {
				return r.importanceProminence(edition, haveEdition)
			}
			return r.recencyProminence(edition, haveEdition)
		})
		demoted := 0
		if byImportance && r.isDemoted() {
			demoted = 1
		}
		ranked[i] = rankedRow{row: r, demoted: demoted, score: score, updated: updated}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		if a.demoted != b.demoted {
			return a.demoted < b.demoted
		}
		if a.score != b.score {
			return a.score > b.score
		}
		if a.updated != b.updated {
			return a.updated > b.updated
		}
		return a.row.PublicationID < b.row.PublicationID
	})
	out := make([]FrontRow, len(ranked))
	for i, r := range ranked {
		out[i] = r.row
	}
	if byImportance {
		// The lead is the first story that may lead; the stories it passes
		// keep their order behind it. With none eligible the order stands.
		for i, r := range out {
			if r.canLead() {
				if i > 0 {
					lead := out[i]
					copy(out[1:i+1], out[0:i])
					out[0] = lead
				}
				break
			}
		}
	}
	return out
}
