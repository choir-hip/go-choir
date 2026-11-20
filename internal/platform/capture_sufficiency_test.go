package platform

// The ranked answer's sufficiency block and leads (capture_sufficiency.go,
// landing L2 of the one search verb). Failure list, written before the code:
//
//   S1 an entry carries no cosine or no time, or the best cosine is not the
//      highest entry's, or a page found by text alone gets a cosine
//   S2 strong counts a weak page (below tau_s), a page older than H, or a page
//      with no time; the newest strong age is wrong
//   S3 an entry whose URL a lead marks tried still counts as unread strong, or
//      one with no lead does not
//   S4 leads by words miss a lead of the computer, list a tried one as
//      unread, find another computer's lead, or let a lead enter Entries
//   S5 the same terms inside the reuse window are not seen (and a search of
//      them 31 minutes ago, or of other terms, is)
//   S6 a lead table that is not made fails the answer instead of saying so in
//      Note, or a vector half that cannot answer drops the block
//   S7 the conversation count counts leads of another conversation

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func sufficiencyFixture(t *testing.T) *rankFixture {
	t.Helper()
	f := newRankFixture(t)
	if err := f.store.ensureSearchLeads(context.Background()); err != nil {
		t.Fatalf("ensureSearchLeads: %v", err)
	}
	return f
}

// rankS asks for the sufficiency block, as the host's measurement does.
func (f *rankFixture) rankS(t *testing.T, query string, mod ...func(*CaptureSearchRequest)) CaptureSearchResponse {
	t.Helper()
	return f.rank(t, query, append([]func(*CaptureSearchRequest){func(r *CaptureSearchRequest) { r.Sufficiency = true }}, mod...)...)
}

func (f *rankFixture) entry(t *testing.T, resp CaptureSearchResponse, key string) RankedCapture {
	t.Helper()
	for _, e := range resp.Ranked.Entries {
		if e.CaptureID == f.id(t, key) {
			return e
		}
	}
	t.Fatalf("no entry for %s in %v", key, entryIDs(resp))
	return RankedCapture{}
}

func TestSufficiencyEntriesCarryCosineAndTimeAndStrongIsCosineAndAge(t *testing.T) {
	f := sufficiencyFixture(t)
	now := time.Now().UTC()
	f.page(t, "hot", "harbor", 5, now.Add(-2*time.Hour))     // cosine 0.95, recent: strong
	f.page(t, "warm", "harbor", 15, now.Add(-30*time.Hour))  // 0.85, older: strong, not the newest
	f.page(t, "stale", "harbor", 3, now.Add(-30*24*time.Hour)) // 0.97 but beyond H
	f.page(t, "weak", "harbor", 40, now.Add(-time.Hour))     // 0.60: not strong
	f.pass(t)
	resp := f.rankS(t, "harbor")
	su := resp.Ranked.Sufficiency
	if su == nil {
		t.Fatal("no sufficiency block")
	}
	if got := f.entry(t, resp, "hot"); got.Cosine < 0.949 || got.Cosine > 0.951 || got.At == "" {
		t.Fatalf("hot entry %+v", got)
	}
	if su.BestCosine < 0.969 || su.BestCosine > 0.971 {
		t.Fatalf("best cosine %v, want 0.97 (the stale page's)", su.BestCosine)
	}
	if su.Strong != 2 {
		t.Fatalf("strong %d, want 2 (hot, warm); %+v", su.Strong, su)
	}
	if su.NewestStrongAgeSeconds < int64(2*time.Hour/time.Second) || su.NewestStrongAgeSeconds > int64(2*time.Hour/time.Second)+120 {
		t.Fatalf("newest strong age %ds, want about 2 h", su.NewestStrongAgeSeconds)
	}
	if su.Thresholds.StrongCosine != SufficiencyStrongCosine || su.Thresholds.HorizonHours != 168 {
		t.Fatalf("thresholds %+v", su.Thresholds)
	}
}

func TestSufficiencyUnreadStrongFollowsTheLeadsReadMark(t *testing.T) {
	f := sufficiencyFixture(t)
	now := time.Now().UTC()
	f.page(t, "readpage", "harbor", 5, now.Add(-time.Hour))
	f.page(t, "freshpage", "harbor", 6, now.Add(-time.Hour))
	f.pass(t)
	ctx := context.Background()
	if _, err := f.store.WriteSearchLeads(ctx, SearchLeadsWrite{OwnerID: f.owner, ComputerID: f.computer, Query: "x", Results: []SearchLeadHit{{URL: "https://example.com/readpage", Title: "t"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.MarkSearchLeadsTried(ctx, f.owner, f.computer, []string{"https://example.com/readpage"}, ""); err != nil {
		t.Fatal(err)
	}
	resp := f.rankS(t, "harbor")
	su := resp.Ranked.Sufficiency
	if su.Strong != 2 || su.UnreadStrong != 1 {
		t.Fatalf("strong %d unread %d, want 2 and 1: %+v", su.Strong, su.UnreadStrong, su)
	}
	if !f.entry(t, resp, "readpage").Read || f.entry(t, resp, "freshpage").Read {
		t.Fatal("the read mark is on the wrong entry")
	}
}

func TestSufficiencyLeadsByWordsSameTermsAndConversation(t *testing.T) {
	f := sufficiencyFixture(t)
	ctx := context.Background()
	f.page(t, "p1", "harbor", 5, time.Now().UTC().Add(-time.Hour))
	f.pass(t)
	w := func(computer, query, conv string, urls ...string) {
		var hits []SearchLeadHit
		for _, u := range urls {
			hits = append(hits, SearchLeadHit{URL: u, Title: "Harbor strike report " + u, Snippet: "the harbor was closed"})
		}
		if _, err := f.store.WriteSearchLeads(ctx, SearchLeadsWrite{OwnerID: f.owner, ComputerID: computer, Query: query, ConversationID: conv, Results: hits}); err != nil {
			t.Fatal(err)
		}
	}
	w(f.computer, "Harbor  strike", "conv-a", "https://n.example/1", "https://n.example/2", "https://n.example/3")
	w(f.computer, "other words", "conv-b", "https://n.example/4")
	w("c-other", "harbor strike", "conv-a", "https://n.example/5")
	if _, err := f.store.MarkSearchLeadsTried(ctx, f.owner, f.computer, []string{"https://n.example/3"}, ""); err != nil {
		t.Fatal(err)
	}
	resp := f.rankS(t, "harbor strike", func(r *CaptureSearchRequest) { r.ConversationID = "conv-a" })
	su := resp.Ranked.Sufficiency
	if su.LeadsMatched != 4 || su.UnreadLeads != 3 || su.UnreadLeadsConversation != 2 {
		t.Fatalf("leads matched %d unread %d unread in conversation %d, want 4, 3, 2: %+v", su.LeadsMatched, su.UnreadLeads, su.UnreadLeadsConversation, su)
	}
	if su.SameTerms == nil || !*su.SameTerms || su.SameTermsAgeSeconds > 60 {
		t.Fatalf("same terms %v age %d: %+v", su.SameTerms, su.SameTermsAgeSeconds, su)
	}
	if len(resp.Ranked.Leads) != 4 {
		t.Fatalf("%d leads listed, want 4 (none of c-other's): %+v", len(resp.Ranked.Leads), resp.Ranked.Leads)
	}
	for _, l := range resp.Ranked.Leads {
		if l.URL == "https://n.example/5" {
			t.Fatal("another computer's lead was listed")
		}
		if l.URL == "https://n.example/3" && !l.Tried {
			t.Fatal("a tried lead is listed untried")
		}
	}
	for _, e := range resp.Ranked.Entries {
		if e.CaptureID != f.id(t, "p1") {
			t.Fatalf("a lead or another page is an entry: %s", e.CaptureID)
		}
	}
	// Other terms: no earlier search of them.
	if su2 := f.rankS(t, "lighthouse keeper").Ranked.Sufficiency; (su2.SameTerms != nil && *su2.SameTerms) || su2.LeadsMatched != 0 {
		t.Fatalf("other terms: %+v", su2)
	}
	// 31 minutes ago is outside the reuse window.
	if _, err := f.store.corpus().Exec(`UPDATE search_query_times SET last_searched_at = ? WHERE computer_id = ?`, time.Now().UTC().Add(-31*time.Minute), f.computer); err != nil {
		t.Fatal(err)
	}
	if su3 := f.rankS(t, "harbor strike").Ranked.Sufficiency; su3.SameTerms != nil && *su3.SameTerms {
		t.Fatalf("a search 31 minutes ago counted as the same terms: %+v", su3)
	}
}

func TestSufficiencyDoesNotFailTheAnswerWhenLeadsOrVectorsAreMissing(t *testing.T) {
	f := newRankFixture(t) // no leads table made
	f.page(t, "p1", "zorblax", 5, time.Now().UTC().Add(-time.Hour))
	f.pass(t)
	resp := f.rankS(t, "zorblax")
	su := resp.Ranked.Sufficiency
	if su == nil || su.Note == "" || su.Strong != 1 || su.UnreadStrong != -1 || su.UnreadLeads != -1 || su.LeadsMatched != -1 || su.SameTerms != nil {
		t.Fatalf("without the leads table: %+v", su)
	}
	f.emb.Down.Store(true)
	resp = f.rankS(t, "zorblax")
	if resp.Ranked.VectorAnswered || resp.Ranked.Sufficiency == nil || resp.Ranked.Sufficiency.BestCosine != 0 || resp.Ranked.Sufficiency.Strong != 0 {
		t.Fatalf("without the vector half: %+v / %+v", resp.Ranked, resp.Ranked.Sufficiency)
	}
	for _, e := range resp.Ranked.Entries {
		if e.Cosine != 0 {
			t.Fatalf("a text-only page has cosine %v", e.Cosine)
		}
		if e.At == "" {
			t.Fatal("a text-only page has no time")
		}
	}
}

func TestSufficiencyWordsAreThePreparedWordsNotTheRawQuery(t *testing.T) {
	f := sufficiencyFixture(t)
	ctx := context.Background()
	f.page(t, "p1", "harbor", 5, time.Now().UTC().Add(-time.Hour))
	f.pass(t)
	if _, err := f.store.WriteSearchLeads(ctx, SearchLeadsWrite{OwnerID: f.owner, ComputerID: f.computer, Query: "docks", ConversationID: "c1",
		Results: []SearchLeadHit{{URL: "https://n.example/d", Title: "Harbor strike closes docks", Snippet: "port"}}}); err != nil {
		t.Fatal(err)
	}
	su := f.rankS(t, "what is the latest news on the harbor strike").Ranked.Sufficiency
	if su.LeadsMatched != 1 || su.UnreadLeads != 1 {
		t.Fatalf("a question with function words found %d leads, %d unread, want 1 and 1: %+v", su.LeadsMatched, su.UnreadLeads, su)
	}
}

func TestSufficiencyCountsTheConversationsLeadsWhateverTheWords(t *testing.T) {
	f := sufficiencyFixture(t)
	ctx := context.Background()
	f.page(t, "p1", "vegas", 5, time.Now().UTC().Add(-time.Hour))
	f.pass(t)
	var hits []SearchLeadHit
	for i := 0; i < 5; i++ {
		hits = append(hits, SearchLeadHit{URL: fmt.Sprintf("https://n.example/cv%d", i), Title: fmt.Sprintf("Casino evacuation report %d", i)})
	}
	if _, err := f.store.WriteSearchLeads(ctx, SearchLeadsWrite{OwnerID: f.owner, ComputerID: f.computer, Query: "Las Vegas shooting latest news", ConversationID: "conv-v", Results: hits}); err != nil {
		t.Fatal(err)
	}
	// One of them tried; one more lead matches the words but came from another conversation; one lead of another computer.
	if _, err := f.store.MarkSearchLeadsTried(ctx, f.owner, f.computer, []string{"https://n.example/cv0"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.WriteSearchLeads(ctx, SearchLeadsWrite{OwnerID: f.owner, ComputerID: f.computer, Query: "other", ConversationID: "conv-x",
		Results: []SearchLeadHit{{URL: "https://n.example/w1", Title: "Las Vegas shooting victims named"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.WriteSearchLeads(ctx, SearchLeadsWrite{OwnerID: f.owner, ComputerID: "c-other", Query: "x", ConversationID: "conv-v",
		Results: []SearchLeadHit{{URL: "https://n.example/z", Title: "Las Vegas shooting"}}}); err != nil {
		t.Fatal(err)
	}
	su := f.rankS(t, "Las Vegas shooting", func(r *CaptureSearchRequest) { r.ConversationID = "conv-v" }).Ranked.Sufficiency
	if su.UnreadLeadsConversation != 4 || su.UnreadLeadsWords != 1 || su.UnreadLeads != 5 {
		t.Fatalf("conversation %d words %d union %d, want 4, 1, 5: %+v", su.UnreadLeadsConversation, su.UnreadLeadsWords, su.UnreadLeads, su)
	}
	// A lead both in the conversation and matching the words counts once.
	if _, err := f.store.WriteSearchLeads(ctx, SearchLeadsWrite{OwnerID: f.owner, ComputerID: f.computer, Query: "again", ConversationID: "conv-v",
		Results: []SearchLeadHit{{URL: "https://n.example/w1", Title: "Las Vegas shooting victims named"}}}); err != nil {
		t.Fatal(err)
	}
	su = f.rankS(t, "Las Vegas shooting", func(r *CaptureSearchRequest) { r.ConversationID = "conv-v" }).Ranked.Sufficiency
	if su.UnreadLeads != 5 || su.UnreadLeadsConversation != 5 {
		t.Fatalf("union %d conversation %d, want 5 and 5 (deduped by URL key): %+v", su.UnreadLeads, su.UnreadLeadsConversation, su)
	}
}

func TestSufficiencySameTermsIsTheTimeOfThatQueryNotTheLeadsLatestReturn(t *testing.T) {
	f := sufficiencyFixture(t)
	ctx := context.Background()
	f.page(t, "p1", "harbor", 5, time.Now().UTC().Add(-time.Hour))
	f.pass(t)
	lead := []SearchLeadHit{{URL: "https://n.example/shared", Title: "Harbor strike"}}
	w := func(q string) {
		if _, err := f.store.WriteSearchLeads(ctx, SearchLeadsWrite{OwnerID: f.owner, ComputerID: f.computer, Query: q, ConversationID: "c", Results: lead}); err != nil {
			t.Fatal(err)
		}
	}
	w("harbor strike")   // Q
	w("dock workers")    // R, shares the lead, later
	if _, err := f.store.corpus().Exec(`UPDATE search_query_times SET last_searched_at = ? WHERE query_key = 'harbor strike'`, time.Now().UTC().Add(-40*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.corpus().Exec(`UPDATE search_query_times SET last_searched_at = ? WHERE query_key = 'dock workers'`, time.Now().UTC().Add(-5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.corpus().Exec(`UPDATE search_leads SET last_returned_at = ?`, time.Now().UTC().Add(-5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	q := f.rankS(t, "harbor strike").Ranked.Sufficiency
	if q.SameTerms == nil || *q.SameTerms {
		t.Fatalf("Q searched 40 minutes ago counts as the same terms: %+v", q)
	}
	r := f.rankS(t, "dock workers").Ranked.Sufficiency
	if r.SameTerms == nil || !*r.SameTerms || r.SameTermsAgeSeconds < 290 || r.SameTermsAgeSeconds > 400 {
		t.Fatalf("R searched 5 minutes ago: %+v", r)
	}
}

func TestSufficiencyIsComputedOnlyWhenAsked(t *testing.T) {
	f := sufficiencyFixture(t)
	f.page(t, "p1", "harbor", 5, time.Now().UTC().Add(-time.Hour))
	f.pass(t)
	resp := f.rank(t, "harbor")
	if resp.Ranked.Sufficiency != nil || resp.Ranked.Leads != nil {
		t.Fatalf("a caller that did not ask got the block: %+v", resp.Ranked.Sufficiency)
	}
	if e := resp.Ranked.Entries[0]; e.Cosine == 0 || e.Read {
		t.Fatalf("entry %+v: cosine is free, the read mark is not", e)
	}
}

func TestSearchQueryTimesArePrunedWithTheirLeadsAndStayWithinOwnerAndComputer(t *testing.T) {
	f := leadFixture(t)
	ctx := context.Background()
	hit := []SearchLeadHit{{URL: "https://n.example/a", Title: "t"}}
	for _, c := range []string{"comp-1", "comp-2"} {
		if _, err := f.store.WriteSearchLeads(ctx, SearchLeadsWrite{OwnerID: "o1", ComputerID: c, Query: "Same  Query", Results: hit}); err != nil {
			t.Fatal(err)
		}
	}
	at, ok, err := f.store.SearchQueryTime(ctx, "o1", "comp-1", "same query")
	if err != nil || !ok || time.Since(at) > time.Minute {
		t.Fatalf("query time %v %v %v", at, ok, err)
	}
	if _, ok, _ := f.store.SearchQueryTime(ctx, "o2", "comp-1", "same query"); ok {
		t.Fatal("another owner's query time was read")
	}
	if _, err := f.store.corpus().Exec(`UPDATE search_query_times SET last_searched_at = now() - interval '31 days' WHERE computer_id = 'comp-2'`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.PruneSearchLeads(ctx, searchLeadRetention, 100); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := f.store.SearchQueryTime(ctx, "o1", "comp-2", "same query"); ok {
		t.Fatal("an old query time was kept")
	}
	if _, ok, _ := f.store.SearchQueryTime(ctx, "o1", "comp-1", "same query"); !ok {
		t.Fatal("a young query time was pruned")
	}
}

func TestConversationIndexIsRebuiltWhenInvalidAndRetriedAfterAFailure(t *testing.T) {
	f := leadFixture(t)
	ctx := context.Background()
	valid := func() bool {
		var v bool
		if err := f.store.corpus().QueryRowContext(ctx, `SELECT indisvalid FROM pg_index WHERE indexrelid = to_regclass('idx_sl_conv')`).Scan(&v); err != nil {
			t.Fatalf("read idx_sl_conv: %v", err)
		}
		return v
	}
	if !valid() || !f.store.leads.convIndex.Load() {
		t.Fatal("the index is not built by ensureSearchLeads")
	}
	// A failed concurrent build leaves an invalid index and no flag.
	if _, err := f.store.corpus().ExecContext(ctx, `UPDATE pg_index SET indisvalid = false WHERE indexrelid = to_regclass('idx_sl_conv')`); err != nil {
		t.Skipf("cannot mark an index invalid here: %v", err)
	}
	f.store.leads.convIndex.Store(false)
	// A build that fails (the context is gone) leaves the flag off and says so.
	dead, cancel := context.WithCancel(ctx)
	cancel()
	if err := f.store.ensureConvIndex(dead); err == nil || f.store.leads.convIndex.Load() {
		t.Fatalf("a failed build: err=%v flag=%v", err, f.store.leads.convIndex.Load())
	}
	// The retry drops the invalid index, builds it again and sets the flag.
	if err := f.store.ensureConvIndex(ctx); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if !valid() || !f.store.leads.convIndex.Load() {
		t.Fatal("the retry did not leave a valid, flagged index")
	}
	// The retry step the maintenance loop takes: nothing to do once built, a wait that doubles while not.
	if wait, built := f.store.retryConvIndex(ctx, 5*time.Second); !built || wait != 0 {
		t.Fatalf("built: wait=%v built=%v", wait, built)
	}
	f.store.leads.convIndex.Store(false)
	if wait, built := f.store.retryConvIndex(dead, 5*time.Second); built || wait != 10*time.Second {
		t.Fatalf("failing: wait=%v built=%v, want 10s and not built", wait, built)
	}
	if wait, built := f.store.retryConvIndex(dead, 4*time.Minute); built || wait != 5*time.Minute {
		t.Fatalf("capped: wait=%v", wait)
	}
}
