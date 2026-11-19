package yaegikernel

import (
	"fmt"
	"time"

	"github.com/yusefmosiah/go-choir/internal/searchquery"
)

// Search for what is lacking, not for what is new (docs/problems/
// search-volume-outruns-keenable-credits-2026-11-18.md, "Why a run searches
// again"). After search reuse deployed, 83% of the pages a run's searches
// returned were new to it, so "stop when wordings stop finding new pages" almost
// never stopped, and the runs read under 5% of the pages they were handed. What
// a question lacks is known only to a desk that has read what it holds.

// searchEnoughUnread is how many pages new to the run, returned for one
// question and not yet read, mean the question already holds more than the run
// will read: the runs of 2026-11-18 read about 19 pages a run in all, of every
// 40 returned 1 to 2, and a search returns up to 40. 15 is nearly a whole run's
// reading, spent on one question. It is the line at which a further wording of
// that question is reported to the desk instead of sent, not a cap: the desk
// reads, and then it can search again; fresh sends a wording any time.
const searchEnoughUnread = 15

// allCounts are a cell's SearchAll counts (SearchStats.AllCalls and the rest).
type allCounts struct {
	calls, offered, sent, freshOverUnread, untagged int
}

// tagQuestion records that search id was sent for the question with this key.
// It reports whether it did: false when the store does not hold the search
// (its start frame had not arrived), and then the gate cannot see that search.
func (st *SearchStore) tagQuestion(id, qk string) bool {
	if st == nil || qk == "" {
		return false
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	s := st.searches[id]
	if s == nil {
		return false
	}
	s.question = qk
	return true
}

// unreadPages lists the pages of this run's searches that were new to the run
// when they came and that the run has not yet tried to read, each once, in the
// order they were found. A page that will not load (a dead link, a 403, a
// paywall) is gone from the list once it was tried. With a question key only
// the searches sent for that question count; with none, every search of the
// run does.
func (st *SearchStore) unreadPages(qk string) []SearchHit {
	if st == nil {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	var tried map[string]bool
	if st.triedKeys != nil {
		tried = st.triedKeys()
	}
	read := st.readSet()
	seen := map[string]bool{}
	var out []SearchHit
	for i := st.runStart; i < len(st.order); i++ {
		if qk != "" && st.searches[st.order[i]].question != qk {
			continue
		}
		for _, h := range st.markOneLocked(i, read).Results {
			if key := searchHitKey(h.URL); !h.Held && !h.Read && !tried[key] && !seen[key] {
				seen[key] = true
				out = append(out, h)
			}
		}
	}
	return out
}

// settle waits until none of this run's searches is still running, at most d
// in all, so a search is judged against the pages the searches before it
// returned, not before they arrive. A search that has run longer than d is not
// waited on again: it had its wait.
func (st *SearchStore) settle(d time.Duration) {
	if st == nil {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	for {
		st.mu.Lock()
		pending := false
		for i := st.runStart; i < len(st.order); i++ {
			if c := st.searches[st.order[i]]; !c.Done && st.nowTime().Sub(c.startedAt) < d {
				pending = true
				break
			}
		}
		changed := st.changed
		st.mu.Unlock()
		if !pending {
			return
		}
		select {
		case <-changed:
		case <-timer.C:
			return
		}
	}
}

// knows waits up to d until the store holds search id (its start frame has
// arrived), and reports whether it does.
func (st *SearchStore) knows(id string, d time.Duration) bool {
	if st == nil {
		return false
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	for {
		st.mu.Lock()
		_, ok := st.searches[id]
		changed := st.changed
		st.mu.Unlock()
		if ok {
			return true
		}
		select {
		case <-changed:
		case <-timer.C:
			return false
		}
	}
}

// questionKey is the key of a SearchAll question: the terms of its words, as
// every in-run key is (searchquery.Key). Empty when the question has none.
func questionKey(question string) string {
	return searchquery.Key(question)
}

// refuseForUnread is the need test every web search passes, SearchAll's
// wordings and the plain verbs alike. It returns the pages the run holds for
// the question (qk; every search of the run when empty) that it has not tried
// to read, and whether they are enough to refuse a search: searchEnoughUnread
// or more. Unless override is set it first waits for the run's running searches
// (see settle). With override it never refuses; the caller counts a refusal as
// holds_unread, and a search sent over one as fresh_over_unread once the host
// has accepted it.
func (s *ChoirScope) refuseForUnread(qk string, override bool) (held []SearchHit, refuse bool) {
	if s == nil || s.searches == nil {
		return nil, false
	}
	if !override {
		s.searches.settle(time.Duration(searchWaitMs) * time.Millisecond)
	}
	held = s.searches.unreadPages(qk)
	if len(held) < searchEnoughUnread {
		return held, false
	}
	return held, !override
}

// unreadRefusal is what the desk is told when a search is not sent for the
// pages the run holds: why, how many, and the pages themselves (SearchStub and
// Search carry them).
func unreadRefusal(n int) string {
	return fmt.Sprintf("this run already holds %d pages new to this run that you have not tried to read, returned by its earlier searches (they may be from any thread of this run, not only this query's). Read the ones that matter, then search again for what is still missing, naming it. A page that will not load counts once you have tried it. To send a search for something these pages cannot hold, pass fresh: choir.Search(query, true)", n)
}
