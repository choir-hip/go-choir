package yaegikernel

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// Async search, worker side (Cut 2 unit 2a,
// docs/archive/definitions/choir-reeng-cut2-evidence-to-publication-2026-10-12.md;
// pre-flight item 6: a cell's host calls end with the cell, so arrivals are
// written by the worker's single reader loop into a store the cell reads).
// Ways the store can fail:
//  1. Arrivals race the cell: the reader loop writes while a cell reads
//     (go test -race catches it).
//  2. A read hands the cell the store's own slices, so a cell that edits
//     what it read corrupts later reads.
//  3. An arrival lands on another search's entry.
//  4. A count line repeats a search with nothing new, or misses one that
//     has new arrivals, or counts a cooling provider in N.
//  5. SearchWait returns before the search is done when time remains, or
//     blocks past its bound.

func providerFrame(id, provider string, answered bool, urls ...string) SearchFrame {
	arrival := SearchArrival{Provider: provider, Status: "success", LatencyMs: 5}
	if !answered {
		arrival.Status, arrival.Error = "timeout", "provider timed out"
	}
	for _, u := range urls {
		arrival.Results = append(arrival.Results, SearchHit{Title: id + " " + u, URL: u, Provider: provider})
	}
	return SearchFrame{ID: id, Kind: SearchFrameProvider, Arrival: &arrival}
}

func TestSearchStoreCellReadsWhileArrivalsLand(t *testing.T) {
	store := newSearchStore()
	const searches, providers = 4, 50
	for i := 0; i < searches; i++ {
		store.apply(SearchFrame{ID: fmt.Sprintf("s%d", i+1), Kind: SearchFrameStart, Query: fmt.Sprintf("q%d", i+1), Providers: make([]string, providers)})
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // the reader loop: the only writer
		defer wg.Done()
		for p := 0; p < providers; p++ {
			for i := 0; i < searches; i++ {
				id := fmt.Sprintf("s%d", i+1)
				store.apply(providerFrame(id, fmt.Sprintf("p%d", p), true, fmt.Sprintf("https://%s.example/%d", id, p)))
			}
		}
		for i := 0; i < searches; i++ {
			store.apply(SearchFrame{ID: fmt.Sprintf("s%d", i+1), Kind: SearchFrameDone})
		}
	}()
	for r := 0; r < 4; r++ { // cells reading meanwhile
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				all := store.List()
				for _, s := range all {
					if len(s.Arrivals) > 0 {
						s.Arrivals[0].Provider = "edited by the cell"
						s.Results = nil
					}
				}
				one, err := store.Get("s1")
				if err != nil {
					t.Error(err)
					return
				}
				if one.Done {
					return
				}
			}
		}()
	}
	wg.Wait()
	for i := 0; i < searches; i++ {
		id := fmt.Sprintf("s%d", i+1)
		s, err := store.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if !s.Done || len(s.Arrivals) != providers || s.Answered != providers || len(s.Results) != providers {
			t.Fatalf("%s = done %v, %d arrivals, %d answered, %d results; want all %d", id, s.Done, len(s.Arrivals), s.Answered, len(s.Results), providers)
		}
		for _, a := range s.Arrivals {
			if a.Provider == "edited by the cell" {
				t.Fatalf("%s: a cell's edit of a read reached the store", id)
			}
			for _, hit := range a.Results {
				if !strings.HasPrefix(hit.Title, id+" ") {
					t.Fatalf("%s holds another search's arrival: %+v", id, hit)
				}
			}
		}
	}
}

func TestSearchResultsDedupAcrossProvidersInArrivalOrder(t *testing.T) {
	store := newSearchStore()
	store.apply(SearchFrame{ID: "s1", Kind: SearchFrameStart, Query: "q", Providers: []string{"a", "b"}})
	store.apply(providerFrame("s1", "b", true, "https://x.example/1", "https://x.example/2"))
	store.apply(providerFrame("s1", "a", true, "https://x.example/2/", "https://x.example/3"))
	s, _ := store.Get("s1")
	var urls []string
	for _, hit := range s.Results {
		urls = append(urls, hit.URL)
	}
	if got := strings.Join(urls, " "); got != "https://x.example/1 https://x.example/2 https://x.example/3" {
		t.Fatalf("results = %s, want each URL once in arrival order", got)
	}
}

func TestSearchCountLinesOncePerSearchWithNewArrivals(t *testing.T) {
	scope := &ChoirScope{searches: newSearchStore()}
	store := scope.searches
	store.apply(SearchFrame{ID: "s1", Kind: SearchFrameStart, Query: "q1", Providers: []string{"a", "b", "c", "d"}, Cooling: []string{"e"}})
	store.apply(SearchFrame{ID: "s2", Kind: SearchFrameStart, Query: "q2", Providers: []string{"a"}})
	store.apply(providerFrame("s1", "a", true, "https://a.example"))
	store.apply(providerFrame("s1", "b", false))
	if got := strings.Join(scope.searchCountLines(), "|"); got != "s1: 1/4 answered, +2 new" {
		t.Fatalf("first cell's count lines = %q", got)
	}
	if got := scope.searchCountLines(); len(got) != 0 {
		t.Fatalf("a cell with no new arrivals got count lines %q", got)
	}
	store.apply(providerFrame("s2", "a", true, "https://a.example"))
	store.apply(SearchFrame{ID: "s2", Kind: SearchFrameDone})
	store.apply(providerFrame("s1", "c", true))
	if got := strings.Join(scope.searchCountLines(), "|"); got != "s1: 2/4 answered, +1 new|s2: 1/1 answered, +1 new, done" {
		t.Fatalf("third cell's count lines = %q", got)
	}
	for _, line := range []string{"https://a.example"} {
		for _, l := range scope.searchCountLines() {
			if strings.Contains(l, line) {
				t.Fatalf("a count line carries a result: %q", l)
			}
		}
	}
}

func TestSearchWaitReturnsAtDoneOrBound(t *testing.T) {
	store := newSearchStore()
	store.apply(SearchFrame{ID: "s1", Kind: SearchFrameStart, Query: "q", Providers: []string{"a", "b"}})
	start := time.Now()
	c, err := store.wait("s1", 80*time.Millisecond)
	if err != nil || c.Done || c.Arrived != 0 || c.Total != 2 {
		t.Fatalf("wait on a quiet search = %+v, %v", c, err)
	}
	if took := time.Since(start); took < 70*time.Millisecond || took > time.Second {
		t.Fatalf("wait took %s, want about its bound", took)
	}
	go func() {
		time.Sleep(30 * time.Millisecond)
		store.apply(providerFrame("s1", "a", true, "https://a.example"))
		time.Sleep(30 * time.Millisecond)
		store.apply(providerFrame("s1", "b", true, "https://b.example"))
		store.apply(SearchFrame{ID: "s1", Kind: SearchFrameDone})
	}()
	start = time.Now()
	c, err = store.wait("s1", 5*time.Second)
	if err != nil || !c.Done || c.Answered != 2 || c.Arrived != 2 {
		t.Fatalf("wait through to done = %+v, %v", c, err)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("wait took %s after the search was done", took)
	}
	if _, err := store.wait("s9", time.Millisecond); err == nil || !strings.Contains(err.Error(), "s1") {
		t.Fatalf("wait on an unknown id = %v, want an error naming the known searches", err)
	}
}

// Budget routing (docs/archive/reports/search-efficiency-and-reuse-2026-10-14.md
// §4.1) adds a paid provider after the start frame when the free answers
// fall short. Its arrival must count in N, or the count reads "2/1
// answered, -1 pending".
func TestSearchEscalatedProviderJoinsN(t *testing.T) {
	store := newSearchStore()
	store.apply(SearchFrame{ID: "s1", Kind: SearchFrameStart, Query: "q", Providers: []string{"keenable"}})
	store.apply(providerFrame("s1", "keenable", true, "https://k.example/1"))
	store.apply(providerFrame("s1", "serper", true, "https://s.example/1"))
	count, _ := store.wait("s1", time.Millisecond)
	if count.String() != "s1: 2/2 answered, 0 pending" {
		t.Fatalf("count = %q", count.String())
	}
	s, _ := store.Get("s1")
	if strings.Join(s.Providers, ",") != "keenable,serper" || s.Total != 2 {
		t.Fatalf("providers=%v total=%d", s.Providers, s.Total)
	}
	// A provider named on the start frame is not added twice.
	store.apply(providerFrame("s1", "keenable", true))
	if s, _ := store.Get("s1"); s.Total != 2 {
		t.Fatalf("total = %d after a repeated provider", s.Total)
	}
}
