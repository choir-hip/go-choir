package agentcore

import (
	"context"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/search"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// searchLeadBatch collects the results of every provider arrival of one search
// and hands them to leads once, when the search ends: one write per search, not
// one per arrival or per result.
type searchLeadBatch struct {
	query string
	leads func(query string, results []search.Result)
	all   []search.Result
}

func (b *searchLeadBatch) add(results []search.Result) {
	if b.leads != nil {
		b.all = append(b.all, results...)
	}
}

func (b *searchLeadBatch) flush() {
	if b.leads != nil && len(b.all) > 0 {
		b.leads(b.query, b.all)
		b.all = nil
	}
}

// searchLeadObserver returns the function a desk's search calls once, with all
// of the search's results when it ends, or nil when this computer has no corpusd that
// takes leads. Every web-search result the computer is handed becomes a row
// corpusd holds (platform/search_leads.go), so what was returned and never
// read survives the run. One call per search, off the search's path: the call
// is started in the background and the search goes on; a failure is logged
// (`search leads op=lead_write rows= ms= err= inflight=`) and dropped. No call
// is dropped for volume: each runs under a deadline (researchtools.RunLeadCall).
func (rt *Runtime) searchLeadObserver(ownerID, computerID, conversationID string) func(query string, results []search.Result) {
	if rt == nil || rt.researchDeps == nil || strings.TrimSpace(ownerID) == "" || strings.TrimSpace(computerID) == "" {
		return nil
	}
	w, ok := rt.researchDeps.Captures.(researchtools.SearchLeadWriter)
	if !ok || w == nil {
		return nil
	}
	return func(query string, results []search.Result) {
		if len(results) == 0 {
			return
		}
		hits := make([]researchtools.SearchLeadHit, 0, len(results))
		for _, r := range results {
			hits = append(hits, researchtools.SearchLeadHit{URL: r.URL, Title: r.Title, Snippet: r.Snippet, PublishedAt: r.PublishedAt})
		}
		req := researchtools.SearchLeadsWrite{OwnerID: ownerID, ComputerID: computerID, Query: query, ConversationID: conversationID, Results: hits}
		researchtools.RunLeadCall("lead_write", len(hits), func(ctx context.Context) error {
			_, err := w.WriteSearchLeads(ctx, req)
			return err
		})
	}
}

// leadConversationOf is the conversation a desk's search leads are filed
// under: its own channel, or, for a research child (which runs with none), the
// story its research tree serves. Without it a child's searches, which are the
// web searches, carried no story and no run could find what a story's research
// had read.
func leadConversationOf(ctx context.Context, exec toolregistry.ExecutionContext) string {
	if c := strings.TrimSpace(exec.ChannelID); c != "" {
		return c
	}
	if lineage, ok := ctx.Value(researchLineageKey{}).(researchLineage); ok {
		return lineage.channelID
	}
	return ""
}
