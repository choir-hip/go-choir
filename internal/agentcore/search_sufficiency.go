package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// The sufficiency measurement of landing L2 of the one search verb
// (docs/definitions/corpus-first-search-one-verb-2026-11-19.md). For every web
// search that reaches the host (the ones the guest's own gate let through) the
// host asks corpusd one ranked question with the same query, off the
// search's path, and logs one line with what a threshold needs. It decides
// nothing: the search is already on its way when the question is asked, and
// nothing reads the answer but the line. A search the guest holds (the unread
// gate, a re-ask) never reaches the host and is not in these lines.
//
// The line (greppable, one per search):
//
//	search sufficiency op=search_sufficiency run=<id> desk=<role> conv=<id> q=<8 hex of sha256 of the query> words=<n> ms=<n> corpus=<answered|vector_unavailable|failed> best=<cosine> strong=<n> newest_strong_age_s=<n> unread_strong=<n> leads_matched=<n> unread_leads=<n> unread_leads_conv=<n> unread_leads_words=<n> same_terms=<true|false|unknown> same_terms_age_s=<n> reuse_min=<n> tau_s=<c> h_h=<hours> lag_h=<hours> top=<cosine>:<age_s>:<r|u|?>,... decision=sent why=guest_gate_passed err=<none|text>
//
// A count that could not be read is -1 and same_terms is unknown (err says
// which read failed); the replay does not decide those searches. top lists
// the answer's entries best first: cosine, age in seconds at the time of the
// call (-1 when the page has no time), r when a lead marks the page read, u
// when not and ? when the leads could not be read. unread_leads is the
// untried union of the conversation's leads and the word-matched ones (the
// design's row 4 count); the other two are its parts. When the vector half
// could not answer, corpus=vector_unavailable and err carries why. The line
// is written once the search has started (a search that failed to start has
// none); decision is always sent, since the host sees only searches it sends,
// and why is "guest_gate_passed" (the guest's override flag is not visible
// here).
const sufficiencyDeadline = 10 * time.Second

// measureSearchSufficiency starts the measurement for a search's payload; it
// returns at once.
func (rt *Runtime) measureSearchSufficiency(ctx context.Context, exec toolregistry.ExecutionContext, query string) {
	if rt == nil || rt.researchDeps == nil || strings.TrimSpace(query) == "" {
		return
	}
	searcher, ok := rt.researchDeps.Captures.(researchtools.CaptureSearcher)
	if !ok || searcher == nil || strings.TrimSpace(exec.OwnerID) == "" || strings.TrimSpace(exec.ComputerID) == "" {
		return
	}
	go func() {
		callCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sufficiencyDeadline)
		defer cancel()
		start := time.Now()
		answer, err := searcher.SearchCaptures(callCtx, researchtools.CaptureSearchQuery{
			OwnerID: exec.OwnerID, ComputerID: exec.ComputerID, Query: query,
			Candidates: 1, Limit: 20, Size: 10, ConversationID: exec.ChannelID, Sufficiency: true,
		})
		log.Print(sufficiencyLine(exec, query, answer.Ranked, time.Since(start), err, time.Now().UTC()))
	}()
}

func sufficiencyLine(exec toolregistry.ExecutionContext, query string, rk *researchtools.CaptureRanking, took time.Duration, err error, now time.Time) string {
	sum := sha256.Sum256([]byte(strings.Join(strings.Fields(strings.ToLower(query)), " ")))
	state, errText := "answered", "none"
	su := researchtools.Sufficiency{UnreadStrong: -1, LeadsMatched: -1, UnreadLeads: -1, UnreadLeadsConversation: -1, UnreadLeadsWords: -1}
	var top []string
	switch {
	case err != nil:
		state, errText = "failed", strings.ReplaceAll(err.Error(), "\n", " ")
	case rk == nil || rk.Sufficiency == nil:
		state, errText = "failed", "the answer has no ranking or sufficiency"
	default:
		su = *rk.Sufficiency
		if !rk.VectorAnswered {
			state = "vector_unavailable"
		}
		for _, e := range rk.Entries {
			age := int64(-1)
			if at, perr := time.Parse(time.RFC3339, e.At); perr == nil {
				age = max(int64(now.Sub(at)/time.Second), 0)
			}
			mark := "u"
			switch {
			case su.UnreadStrong < 0:
				mark = "?"
			case e.Read:
				mark = "r"
			}
			top = append(top, fmt.Sprintf("%.3f:%d:%s", e.Cosine, age, mark))
		}
		var errs []string
		if !rk.VectorAnswered {
			errs = append(errs, "vector_unavailable:"+rk.VectorUnavailable)
		}
		if su.Note != "" {
			errs = append(errs, su.Note)
		}
		if len(errs) > 0 {
			errText = strings.Join(strings.Fields(strings.Join(errs, ";")), "_")
		}
	}
	topText := "none"
	if len(top) > 0 {
		topText = strings.Join(top, ",")
	}
	sameTerms := "unknown"
	if su.SameTerms != nil {
		sameTerms = fmt.Sprintf("%t", *su.SameTerms)
	}
	return fmt.Sprintf("search sufficiency op=search_sufficiency run=%s desk=%s conv=%s q=%s words=%d ms=%d corpus=%s best=%.3f strong=%d newest_strong_age_s=%d unread_strong=%d leads_matched=%d unread_leads=%d unread_leads_conv=%d unread_leads_words=%d same_terms=%s same_terms_age_s=%d reuse_min=%.0f tau_s=%.2f h_h=%.0f lag_h=%.0f top=%s decision=sent why=guest_gate_passed err=%s",
		exec.RunID, exec.Role, exec.ChannelID, hex.EncodeToString(sum[:4]), len(strings.Fields(query)), took.Milliseconds(), state,
		su.BestCosine, su.Strong, su.NewestStrongAgeSeconds, su.UnreadStrong, su.LeadsMatched, su.UnreadLeads, su.UnreadLeadsConversation, su.UnreadLeadsWords,
		sameTerms, su.SameTermsAgeSeconds, su.Thresholds.ReuseMinutes, su.Thresholds.StrongCosine, su.Thresholds.HorizonHours, su.Thresholds.LagHours, topText, errText)
}
