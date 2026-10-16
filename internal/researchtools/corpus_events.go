package researchtools

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// The event layer's V2 (docs/reports/event-layer-at-ingestion-design-
// 2026-10-16.md §7): corpus search returns the world events it matches
// before captures. An event is what the paper knows about one occurrence:
// who reported it (outlets, counted origins), since when, and the captures
// to read. It is read from the computer's event memberships, so it needs no
// runtime state.
const (
	// corpusEventScan and corpusEventMemberScan bound the store reads: the
	// newest events and memberships (one per resolved capture).
	corpusEventScan       = 3000
	corpusEventMemberScan = 6000
	corpusEventCaptureIDs = 6
	corpusEventTitles     = 4
)

// CorpusEvent is one world event a corpus search found.
type CorpusEvent struct {
	EventID      string   `json:"event_id"`
	Headline     string   `json:"headline"`
	Outlets      []string `json:"outlets"`
	Origins      int      `json:"origins"`
	Corroborated bool     `json:"corroborated"`
	Members      int      `json:"members"`
	Tentative    int      `json:"tentative,omitempty"`
	FirstSeen    string   `json:"first_seen"`
	LastSeen     string   `json:"last_seen"`
	Titles       []string `json:"titles,omitempty"`
	CaptureIDs   []string `json:"capture_ids"`
	Matched      []string `json:"matched"`
}

type corpusEventAcc struct {
	out     CorpusEvent
	words   string
	outlets map[string]bool
	origins map[string]bool
	last    time.Time
}

// searchEvents returns the computer's events whose headline or member titles
// match terms: the most terms first, corroborated first among equals, then
// the newest.
func searchEvents(ctx context.Context, s *store.Store, ownerID, computerID string, terms []string, limit int) ([]CorpusEvent, error) {
	if s == nil || len(terms) == 0 || strings.TrimSpace(computerID) == "" {
		return []CorpusEvent{}, nil
	}
	events, err := s.RecentWorldEvents(ctx, ownerID, computerID, corpusEventScan)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return []CorpusEvent{}, nil
	}
	members, err := s.RecentEventMemberships(ctx, ownerID, computerID, corpusEventMemberScan)
	if err != nil {
		return nil, err
	}
	acc := make(map[string]*corpusEventAcc, len(events))
	for _, ev := range events {
		acc[ev.EventID] = &corpusEventAcc{out: CorpusEvent{EventID: ev.EventID, Headline: ev.Headline, FirstSeen: ev.FirstSeen.UTC().Format(time.RFC3339)},
			words: strings.ToLower(ev.Headline), outlets: map[string]bool{}, origins: map[string]bool{}, last: ev.FirstSeen}
	}
	for _, m := range members {
		a := acc[m.EventID]
		if a == nil {
			continue
		}
		if m.CapturedAt.After(a.last) {
			a.last = m.CapturedAt
		}
		if m.Tentative {
			a.out.Tentative++
			continue
		}
		a.out.Members++
		if len(a.out.CaptureIDs) < corpusEventCaptureIDs {
			a.out.CaptureIDs = append(a.out.CaptureIDs, m.CaptureID)
		}
		if t := strings.TrimSpace(m.Title); t != "" && t != a.out.Headline && len(a.out.Titles) < corpusEventTitles {
			a.out.Titles = append(a.out.Titles, t)
		}
		a.words += " " + strings.ToLower(m.Title)
		if origin := m.CountedOrigin(); origin != "" {
			a.outlets[m.Outlet], a.origins[origin] = true, true
		}
	}
	var hits []*corpusEventAcc
	for _, a := range acc {
		for _, term := range terms {
			if corpusTermIn(a.words, term) {
				a.out.Matched = append(a.out.Matched, term)
			}
		}
		if len(a.out.Matched) == 0 {
			continue
		}
		for o := range a.outlets {
			a.out.Outlets = append(a.out.Outlets, o)
		}
		sort.Strings(a.out.Outlets)
		a.out.Origins = len(a.origins)
		a.out.Corroborated = a.out.Origins >= store.EventMinOrigins
		a.out.LastSeen = a.last.UTC().Format(time.RFC3339)
		hits = append(hits, a)
	}
	sort.SliceStable(hits, func(i, j int) bool {
		hi, hj := hits[i].out, hits[j].out
		if len(hi.Matched) != len(hj.Matched) {
			return len(hi.Matched) > len(hj.Matched)
		}
		if hi.Corroborated != hj.Corroborated {
			return hi.Corroborated
		}
		if hi.Origins != hj.Origins {
			return hi.Origins > hj.Origins
		}
		return hits[i].last.After(hits[j].last)
	})
	out := make([]CorpusEvent, 0, limit)
	for i := 0; i < len(hits) && i < limit; i++ {
		out = append(out, hits[i].out)
	}
	return out, nil
}
