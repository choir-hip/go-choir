package platform

// The ranked answer's sufficiency block and held leads (landing L2 of
// docs/definitions/corpus-first-search-one-verb-2026-11-19.md).
//
// L2 is a measurement. It says how strong and how recent the best answer is
// and which held web-search results (search leads) match the query, from what
// the one ranked call already has: the entries' cosines and times, and reads
// of the leads table. It is computed only for a request that asks
// (CaptureSearchRequest.Sufficiency, the host's measurement call), after the
// ranked answer's own timer has ended, so no other caller waits for it or sees
// its cost. It decides nothing: no caller holds or sends a
// search differently because of it. The thresholds below are provisional; the
// replay (scripts/search-sufficiency-replay) sets them from a logged day, and L3 reads
// them. They gate nothing here.
//
// What is smaller than the design says, and why:
//   - "unread" strong captures are those whose URL no lead of this computer
//     has had a Read mark (tried_at or a capture id). The design sends the
//     URL keys and capture ids the run has tried; the host does not hold a
//     run's reads in one place, and passing them would widen the desk protocol
//     (a red change), so the per-run set is not used. The computer-wide mark
//     overstates "unread" for a page read by a path that makes no lead.
//   - leads are matched by words (at least half of the prepared query words,
//     rounded up, as the guest's corpus search needs: lower case, function
//     words dropped, in the title and snippet), by the conversation that was returned them, and by the
//     exact query key of an earlier search, not by cosine: leads are not
//     embedded (search_leads.go). Unread leads are the untried union of the
//     conversation's leads and the word-matched ones, deduplicated by URL key.
//   - a count that could not be read is -1 (and same_terms nil), never 0.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/captureterms"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Provisional thresholds of the one search verb's table (the design gives no
// numbers for the first three; these are guesses the replay replaces). They
// are named so the replay and L3 read one place, and they gate nothing in L2.
const (
	// SufficiencyStrongCosine is τs: a capture with at least this cosine to the
	// query is strong.
	SufficiencyStrongCosine = 0.80
	// SufficiencyHorizon is H: a strong capture is no older than this.
	SufficiencyHorizon = 7 * 24 * time.Hour
	// SufficiencyLag is how far before the newest strong capture a sent
	// search's `since` would reach (row 5).
	SufficiencyLag = 6 * time.Hour
	// SufficiencyReuse is the design's reuse window for the same terms (row 0).
	SufficiencyReuse = 30 * time.Minute
	// sufficiencyLeadRead bounds the leads read by words and the leads listed.
	sufficiencyLeadRead = 200
	// sufficiencyLeadWindow bounds how far back the word and conversation
	// reads look: a lead older than the horizon is not a lead for this search.
	sufficiencyLeadWindow = SufficiencyHorizon
	sufficiencyLeadList = 20
	sufficiencySnippet  = 300
)

// SufficiencyThresholds echoes the constants a block was computed with.
type SufficiencyThresholds struct {
	StrongCosine float64 `json:"strong_cosine"`
	HorizonHours float64 `json:"horizon_hours"`
	LagHours     float64 `json:"lag_hours"`
	ReuseMinutes float64 `json:"reuse_minutes"`
}

// Sufficiency is how strong and recent the best answer is.
type Sufficiency struct {
	// BestCosine is the highest cosine of any entry (0 when the vector half
	// did not answer or reached no entry).
	BestCosine float64 `json:"best_cosine"`
	// Strong counts entries with cosine at least τs and a time no older than H.
	Strong int `json:"strong"`
	// NewestStrongAt and NewestStrongAgeSeconds describe the newest strong
	// entry; empty and 0 when Strong is 0.
	NewestStrongAt         string `json:"newest_strong_at,omitempty"`
	NewestStrongAgeSeconds int64  `json:"newest_strong_age_seconds,omitempty"`
	// UnreadStrong counts strong entries no lead marks read; -1 when the
	// leads could not be read.
	UnreadStrong int `json:"unread_strong"`
	// LeadsMatched is how many leads (at most 200, returned within the
	// horizon) hold at least half of the prepared query words. UnreadLeadsWords are those no
	// run tried; UnreadLeadsConversation the untried leads a search of the
	// request's conversation returned, whatever their words; UnreadLeads the
	// untried union of the two, once per URL key (the design's row 4 count).
	// Each is -1 when it could not be read (the union when either part could
	// not be).
	LeadsMatched            int `json:"leads_matched"`
	UnreadLeads             int `json:"unread_leads"`
	UnreadLeadsConversation int `json:"unread_leads_conversation"`
	UnreadLeadsWords        int `json:"unread_leads_words"`
	// SameTerms is true when this computer searched the same words (the query
	// key, lower case and spaces collapsed) within the reuse window, by the
	// time of that search itself (search_query_times); nil when it could not
	// be read. SameTermsAgeSeconds is how long ago that search was.
	SameTerms           *bool `json:"same_terms,omitempty"`
	SameTermsAgeSeconds int64 `json:"same_terms_age_seconds,omitempty"`
	// Thresholds are the provisional constants above.
	Thresholds SufficiencyThresholds `json:"thresholds"`
	// Note names a part that could not be read (the leads table, the lookup).
	Note string `json:"note,omitempty"`
}

// RankedLead is a held search result that matches the query. It is never
// evidence and never an entry.
type RankedLead struct {
	URL            string `json:"url"`
	Title          string `json:"title,omitempty"`
	Snippet        string `json:"snippet,omitempty"`
	PublishedAt    string `json:"published_at,omitempty"`
	LastReturnedAt string `json:"last_returned_at"`
	TimesReturned  int    `json:"times_returned"`
	Tried          bool   `json:"tried"`
	// Conversation: an earlier search of the request's conversation returned it.
	Conversation bool `json:"conversation,omitempty"`
	// SameTerms: returned by a search of the same words inside the reuse window.
	SameTerms bool `json:"same_terms,omitempty"`
}

// entryTime is a capture's window time (published, else fetched) in RFC 3339.
func entryTime(byID map[string]objectgraph.Object, id string) string {
	o, ok := byID[id]
	if !ok {
		return ""
	}
	if t, ok := captureWindowTime(o); ok {
		return t.Format(time.RFC3339)
	}
	return ""
}

func leadTried(l SearchLead) bool { return l.TriedAt != nil || l.CaptureID != "" }

// addSufficiency fills ranking.Sufficiency and ranking.Leads. It never fails
// the answer: a part it cannot read is named in Note and its counts are -1.
func (s *Store) addSufficiency(ctx context.Context, req CaptureSearchRequest, resp *CaptureSearchResponse, rk *CaptureRanking) {
	owner, computer := strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID)
	now := time.Now().UTC()
	objs := map[string]objectgraph.Object{}
	for _, o := range resp.HitObjects {
		objs[o.CanonicalID] = o
	}
	su := &Sufficiency{UnreadStrong: -1, LeadsMatched: -1, UnreadLeads: -1, UnreadLeadsConversation: -1, UnreadLeadsWords: -1,
		Thresholds: SufficiencyThresholds{
			StrongCosine: SufficiencyStrongCosine, HorizonHours: SufficiencyHorizon.Hours(),
			LagHours: SufficiencyLag.Hours(), ReuseMinutes: SufficiencyReuse.Minutes()}}
	rk.Sufficiency = su
	note := func(format string, args ...any) {
		su.Note = strings.TrimSpace(su.Note + " " + fmt.Sprintf(format, args...))
	}

	var newest time.Time
	var strong []string
	for i := range rk.Entries {
		e := &rk.Entries[i]
		if e.At == "" {
			e.At = entryTime(objs, e.CaptureID)
		}
		if e.Cosine > su.BestCosine {
			su.BestCosine = e.Cosine
		}
		at, err := time.Parse(time.RFC3339, e.At)
		if err != nil || e.Cosine < SufficiencyStrongCosine || now.Sub(at) > SufficiencyHorizon {
			continue
		}
		su.Strong++
		strong = append(strong, e.CaptureID)
		if at.After(newest) {
			newest = at
		}
	}
	if su.Strong > 0 {
		su.NewestStrongAt = newest.Format(time.RFC3339)
		if age := now.Sub(newest); age > 0 {
			su.NewestStrongAgeSeconds = int64(age / time.Second)
		}
	}
	if owner == "" || computer == "" {
		return
	}

	// Read: a lead of this computer for the page's URL has a mark (a Read
	// tried it, or a capture of the URL was stored from a search result).
	// Unread strong: strong and not Read.
	if len(rk.Entries) == 0 {
		su.UnreadStrong = 0
	} else {
		var urls []string
		for _, e := range rk.Entries {
			if meta, err := objectgraph.WebCaptureMetadataFromObject(objs[e.CaptureID]); err == nil {
				urls = append(urls, meta.URL, meta.CanonicalURL)
			}
		}
		tried := map[string]bool{}
		got, err := s.SearchLeadsByURL(ctx, owner, computer, urls, 4*sufficiencyLeadRead)
		if err != nil {
			note("leads_by_url:%v;", err)
		} else {
			for _, l := range got {
				if leadTried(l) {
					tried[l.URLKey] = true
				}
			}
			su.UnreadStrong = 0
			isStrong := map[string]bool{}
			for _, id := range strong {
				isStrong[id] = true
			}
			for i := range rk.Entries {
				e := &rk.Entries[i]
				if meta, err := objectgraph.WebCaptureMetadataFromObject(objs[e.CaptureID]); err == nil {
					e.Read = tried[captureURLKey(meta.URL)] || (strings.TrimSpace(meta.CanonicalURL) != "" && tried[captureURLKey(meta.CanonicalURL)])
				}
				if isStrong[e.CaptureID] && !e.Read {
					su.UnreadStrong++
				}
			}
		}
	}

	listed := map[string]*RankedLead{}
	var order []string
	add := func(l SearchLead) *RankedLead {
		r := listed[l.URLKey]
		if r == nil {
			r = &RankedLead{URL: l.URL, Title: l.Title, Snippet: clipRunes(l.Snippet, sufficiencySnippet),
				LastReturnedAt: l.LastReturnedAt.Format(time.RFC3339), TimesReturned: l.TimesReturned, Tried: leadTried(l)}
			if l.PublishedAt != nil {
				r.PublishedAt = l.PublishedAt.Format(time.RFC3339)
			}
			listed[l.URLKey] = r
			order = append(order, l.URLKey)
		}
		return r
	}

	// The earlier search of the same words, by the time of that search.
	if key := leadQueryKey(req.Query); key != "" {
		at, ok, err := s.SearchQueryTime(ctx, owner, computer, key)
		switch {
		case err != nil:
			note("query_time:%v;", err)
		case ok && now.Sub(at) <= SufficiencyReuse:
			yes := true
			su.SameTerms = &yes
			if age := now.Sub(at); age > 0 {
				su.SameTermsAgeSeconds = int64(age / time.Second)
			}
			same, err := s.SearchLeadsBySameQuery(ctx, owner, computer, key, at, sufficiencyLeadRead)
			if err != nil {
				note("leads_same_query:%v;", err)
			}
			for _, l := range same {
				add(l).SameTerms = true
			}
		default:
			no := false
			su.SameTerms = &no
		}
	}

	// Untried leads: the conversation's, whatever their words, and those
	// holding every prepared word, once per URL key.
	leadSince := now.Add(-sufficiencyLeadWindow)
	unread := map[string]bool{}
	var wordsOK, convOK = true, true
	inConv := map[string]bool{}
	if c := strings.TrimSpace(req.ConversationID); c != "" {
		convLeads, err := s.SearchLeadsByConversation(ctx, owner, computer, c, leadSince, sufficiencyLeadRead)
		if err != nil {
			convOK = false
			note("leads_by_conversation:%v;", err)
		}
		su.UnreadLeadsConversation = 0
		for _, l := range convLeads {
			add(l).Conversation = true
			inConv[l.URLKey] = true
			if !leadTried(l) {
				unread[l.URLKey] = true
				su.UnreadLeadsConversation++
			}
		}
	} else {
		su.UnreadLeadsConversation = 0
	}
	if !convOK {
		su.UnreadLeadsConversation = -1
	}
	wordLeads, err := s.SearchLeadsByWords(ctx, owner, computer, captureterms.QueryWords(req.Query, 8), leadSince, sufficiencyLeadRead)
	if err != nil {
		wordsOK = false
		note("leads_by_words:%v;", err)
	} else {
		su.LeadsMatched, su.UnreadLeadsWords = len(wordLeads), 0
		for _, l := range wordLeads {
			r := add(l)
			if inConv[l.URLKey] {
				r.Conversation = true
			}
			if !leadTried(l) {
				unread[l.URLKey] = true
				su.UnreadLeadsWords++
			}
		}
	}
	if wordsOK && convOK {
		su.UnreadLeads = len(unread)
	}
	for i, k := range order {
		if i == sufficiencyLeadList {
			break
		}
		rk.Leads = append(rk.Leads, *listed[k])
	}
}
