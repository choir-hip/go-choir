package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Intake labels (L2a A1 / R2-3 / R2-4,
// docs/archive/reports/l2a-synthesis-intake-design-2026-10-14.md). When the
// computer's intake.labels setting is on, a poll's new captures are each
// labelled by Jev against a shortlist of the computer's stories and the
// poll's other new captures, after the captures are written and before the
// arrival record is minted. One request per capture carries two questions:
// which candidate reports the same event (or new), and which story shares
// the topic (or none). Choices are short aliases (c1…, n1…) mapped back
// here. Labels run in parallel within a per-poll budget; a failure or
// timeout leaves the capture unlabelled, and labels never gate arrival.
// Each label is its own choir.capture_label object, never part of the
// arrival record.

// judgmentScopeIntakeLabels is the gateway rate scope of intake labels: its
// own bucket, so a busy poll never starves the support judge.
const judgmentScopeIntakeLabels = "intake_labels"

// ScopedCandidateJudge is a CandidateJudge that can draw on a named gateway
// rate bucket. A judge without it is called through Judge.
type ScopedCandidateJudge interface {
	JudgeScoped(ctx context.Context, scope string, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error)
}

// Budgets. Vars so a test can shorten them.
var (
	intakeLabelConcurrency = 4
	intakeLabelPollBudget  = 15 * time.Second
	intakeLabelCallTimeout = 5 * time.Second
)

const (
	intakeShortlistRecent     = 30
	intakeShortlistOverlap    = 10
	intakeShortlistHorizon    = 72 * time.Hour
	intakeOverrideExamples    = 12
	intakeSummaryRunes        = 600
	intakeSiblingSummaryRunes = 200
	// intakeNoCandidatesModel marks a label written without a call because
	// there was nothing to compare the capture with.
	intakeNoCandidatesModel = "none"
)

// intakeLibrary is the per-poll view of the computer's stories: the
// newest within the horizon, the rest within it for title overlap, and a
// cache of the summaries read.
type intakeLibrary struct {
	rt       *Runtime
	recent   []types.Document
	rest     []types.Document
	mu       sync.Mutex
	summary  map[string]store.StorySummary
	examples []store.CaptureLabelOverride
}

func (lib *intakeLibrary) summaryOf(ctx context.Context, doc types.Document) store.StorySummary {
	lib.mu.Lock()
	cached, ok := lib.summary[doc.DocID]
	lib.mu.Unlock()
	if ok {
		return cached
	}
	sum := lib.rt.store.StorySummaryOf(ctx, doc)
	lib.mu.Lock()
	lib.summary[doc.DocID] = sum
	lib.mu.Unlock()
	return sum
}

// intakeShortlistSplit splits stories (any order) into the newest within
// the horizon and the rest within it.
func intakeShortlistSplit(docs []types.Document, now time.Time) (recent, rest []types.Document) {
	live := make([]types.Document, 0, len(docs))
	for _, d := range docs {
		if now.Sub(d.UpdatedAt) <= intakeShortlistHorizon {
			live = append(live, d)
		}
	}
	sort.SliceStable(live, func(i, j int) bool { return live[i].UpdatedAt.After(live[j].UpdatedAt) })
	if len(live) <= intakeShortlistRecent {
		return live, nil
	}
	return live[:intakeShortlistRecent], live[intakeShortlistRecent:]
}

var intakeStopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "from": true, "after": true, "over": true, "says": true, "into": true,
	"that": true, "this": true, "are": true, "was": true, "has": true, "have": true, "its": true, "les": true, "des": true,
	"une": true, "los": true, "las": true, "del": true, "por": true, "con": true, "que": true, "der": true, "die": true,
	"das": true, "und": true, "mit": true, "von": true, "para": true, "com": true, "uma": true,
}

// titleTokens are a title's lower-cased words of three or more runes.
func titleTokens(title string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(title), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }) {
		if len([]rune(w)) >= 3 && !intakeStopwords[w] {
			out[w] = true
		}
	}
	return out
}

// intakeOverlapPicks returns up to n of rest that share the most title
// tokens with title, at least one; ties keep rest's (newest first) order.
func intakeOverlapPicks(rest []types.Document, title string, n int) []types.Document {
	want := titleTokens(title)
	if len(want) == 0 || n <= 0 {
		return nil
	}
	type scored struct {
		doc   types.Document
		score int
	}
	var hits []scored
	for _, d := range rest {
		score := 0
		for w := range titleTokens(d.Title) {
			if want[w] {
				score++
			}
		}
		if score > 0 {
			hits = append(hits, scored{d, score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	out := make([]types.Document, 0, n)
	for _, h := range hits {
		if len(out) == n {
			break
		}
		out = append(out, h.doc)
	}
	return out
}

func intakeClip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if runes := []rune(s); len(runes) > n {
		return strings.TrimSpace(string(runes[:n])) + "…"
	}
	return s
}

// labelCaptures labels a poll's new captures and reports how many were
// labelled and how many were left unlabelled. It returns within the poll
// budget whatever Jev does.
func (rt *Runtime) labelCaptures(ctx context.Context, ownerID, computerID, sourceID string, captured []sourcegraph.CapturedItem) (labelled, unlabelled int) {
	var items []sourcegraph.CapturedItem
	for _, c := range captured {
		if strings.TrimSpace(c.CaptureID) != "" {
			items = append(items, c)
		}
	}
	if len(items) == 0 {
		return 0, 0
	}
	judge := rt.candidateJudge
	if judge == nil || rt.store == nil {
		return 0, len(items)
	}
	budget, cancel := context.WithTimeout(ctx, intakeLabelPollBudget)
	defer cancel()
	lib := &intakeLibrary{rt: rt, summary: map[string]store.StorySummary{}}
	if docs, err := rt.store.ListStoryDocuments(budget, ownerID, computerID, time.Time{}, "", 0); err != nil {
		log.Printf("intake labels: %s: list stories: %v; labelling against siblings only", sourceID, err)
	} else {
		lib.recent, lib.rest = intakeShortlistSplit(docs, time.Now().UTC())
	}
	if examples, err := rt.store.RecentCaptureLabelOverrides(budget, ownerID, computerID, intakeOverrideExamples); err != nil {
		log.Printf("intake labels: %s: read corrections: %v", sourceID, err)
	} else {
		lib.examples = examples
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, intakeLabelConcurrency)
	for i := range items {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-budget.Done():
				return
			}
			defer func() { <-sem }()
			label, err := rt.labelOneCapture(budget, judge, lib, ownerID, computerID, sourceID, items, i)
			if err == nil {
				_, err = rt.store.AppendCaptureLabel(ctx, label)
			}
			if err != nil {
				log.Printf("intake labels: %s: %s unlabelled: %v", sourceID, items[i].CaptureID, err)
				return
			}
			mu.Lock()
			labelled++
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	return labelled, len(items) - labelled
}

// intakeCaptureState is what Jev reads of one capture.
func intakeCaptureState(c sourcegraph.CapturedItem, summaryRunes int) map[string]any {
	state := map[string]any{"title": strings.TrimSpace(c.Title)}
	for k, v := range map[string]string{"summary": intakeClip(c.Summary, summaryRunes), "outlet": c.Outlet, "language": c.Language, "script": c.Script, "url": c.URL, "published_at": c.PublishedAt} {
		if v = strings.TrimSpace(v); v != "" {
			state[k] = v
		}
	}
	return state
}

// labelOneCapture asks Jev about items[i] and returns its label.
func (rt *Runtime) labelOneCapture(ctx context.Context, judge CandidateJudge, lib *intakeLibrary, ownerID, computerID, sourceID string, items []sourcegraph.CapturedItem, i int) (store.CaptureLabel, error) {
	capture := items[i]
	label := store.CaptureLabel{OwnerID: ownerID, ComputerID: computerID, CaptureID: capture.CaptureID, SourceID: sourceID,
		Title: strings.TrimSpace(capture.Title), Outlet: capture.Outlet, Language: capture.Language, Script: capture.Script}

	// Candidates: the newest stories, then the rest by title overlap.
	candidates := append([]types.Document(nil), lib.recent...)
	candidates = append(candidates, intakeOverlapPicks(lib.rest, capture.Title, intakeShortlistOverlap)...)
	storyAlias := map[string]store.StorySummary{}
	stories := map[string]any{}
	var storyChoices []string
	for n, doc := range candidates {
		if ctx.Err() != nil {
			return label, ctx.Err()
		}
		sum := lib.summaryOf(ctx, doc)
		alias := fmt.Sprintf("c%d", n+1)
		storyAlias[alias] = sum
		entry := map[string]any{"headline": sum.Headline, "updated_at": sum.UpdatedAt.UTC().Format(time.RFC3339)}
		if sum.Lede != "" {
			entry["lede"] = sum.Lede
		}
		stories[alias] = entry
		storyChoices = append(storyChoices, alias)
		label.CandidateDocIDs = append(label.CandidateDocIDs, sum.DocID)
	}
	siblingAlias := map[string]string{}
	siblings := map[string]any{}
	var siblingChoices []string
	for j, other := range items {
		if j == i {
			continue
		}
		alias := fmt.Sprintf("n%d", len(siblingChoices)+1)
		siblingAlias[alias] = other.CaptureID
		siblings[alias] = intakeCaptureState(sourcegraph.CapturedItem{Title: other.Title, Summary: other.Summary, Outlet: other.Outlet, Language: other.Language}, intakeSiblingSummaryRunes)
		siblingChoices = append(siblingChoices, alias)
		label.SiblingCaptureIDs = append(label.SiblingCaptureIDs, other.CaptureID)
	}
	if len(storyChoices) == 0 && len(siblingChoices) == 0 {
		label.Event, label.EventP, label.Model = store.CaptureLabelEventNew, 1, intakeNoCandidatesModel
		return label, nil
	}

	state := map[string]any{
		"task":    "A news capture has arrived. Decide which story or other new capture, if any, reports the same real-world event, and which story, if any, shares its topic. Titles and summaries may be in any language; read them all.",
		"capture": intakeCaptureState(capture, intakeSummaryRunes),
	}
	if len(stories) > 0 {
		state["stories"] = stories
	}
	if len(siblings) > 0 {
		state["new_captures"] = siblings
	}
	if len(lib.examples) > 0 {
		var corrections []map[string]string
		for _, o := range lib.examples {
			corrections = append(corrections, map[string]string{"capture": o.CaptureTitle, "label_said": o.JevSaid, "editor_did": overrideDisposition(o), "reason": o.Reason})
		}
		state["corrections"] = corrections
	}
	eventChoices := append(append(append([]string(nil), storyChoices...), siblingChoices...), store.CaptureLabelEventNew)
	eventCriteria := map[string]string{store.CaptureLabelEventNew: "No story and no other new capture reports this event; a shared topic or shared actors alone do not count."}
	for _, a := range storyChoices {
		eventCriteria[a] = "state.stories." + a + " reports the same event as the capture, or its direct development."
	}
	for _, a := range siblingChoices {
		eventCriteria[a] = "state.new_captures." + a + " reports the same event, and no story does."
	}
	questions := map[string]any{
		"event": map[string]any{
			"type":         "choice",
			"instructions": "Which entry reports the same real-world event as state.capture? A story (c1, c2, … in state.stories) or another new capture (n1, n2, … in state.new_captures) reports the same event when it covers the same occurrence or its direct development (a later announcement, reaction or outcome of that occurrence), in any language. Prefer a story over a new capture when both match. The same topic or the same actors in a different event is new. state.corrections, when present, are earlier labels the editor corrected, with the reason.",
			"criteria":     eventCriteria,
			"choices":      eventChoices,
		},
	}
	if len(storyChoices) > 0 {
		topicCriteria := map[string]string{"none": "No story shares the capture's topic."}
		for _, a := range storyChoices {
			topicCriteria[a] = "state.stories." + a + " covers the same ongoing subject as the capture."
		}
		questions["topic"] = map[string]any{
			"type":         "choice",
			"instructions": "Which story in state.stories shares state.capture's topic: the same ongoing subject (a war, an election, a company, a trial), even when it is a different event? Answer none when no story does.",
			"criteria":     topicCriteria,
			"choices":      append(append([]string(nil), storyChoices...), "none"),
		}
	}
	stateRaw, err := json.Marshal(state)
	if err != nil {
		return label, err
	}
	rawQuestions := make(map[string]json.RawMessage, len(questions))
	for name, q := range questions {
		if rawQuestions[name], err = json.Marshal(q); err != nil {
			return label, err
		}
	}
	raw, err := callIntakeJudge(ctx, judge, stateRaw, rawQuestions)
	if err != nil {
		return label, err
	}
	var distribution struct {
		Model   string `json:"model"`
		Answers map[string]struct {
			Choice        string             `json:"choice"`
			Probabilities map[string]float64 `json:"probabilities"`
		} `json:"answers"`
		Usage struct {
			InputTokens  int     `json:"input_tokens"`
			OutputTokens int     `json:"output_tokens"`
			Cost         float64 `json:"cost"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &distribution); err != nil {
		return label, fmt.Errorf("decode Jev's answer: %w", err)
	}
	event, ok := distribution.Answers["event"]
	if !ok {
		return label, fmt.Errorf("Jev gave no event answer")
	}
	choice := strings.TrimSpace(event.Choice)
	switch {
	case choice == store.CaptureLabelEventNew:
		label.Event = store.CaptureLabelEventNew
	case storyAlias[choice].DocID != "":
		label.Event, label.EventDocID, label.EventHeadline = store.CaptureLabelEventStory, storyAlias[choice].DocID, storyAlias[choice].Headline
	case siblingAlias[choice] != "":
		label.Event, label.EventCaptureID = store.CaptureLabelEventSibling, siblingAlias[choice]
	default:
		return label, fmt.Errorf("Jev chose %q, which is not a choice", choice)
	}
	label.EventP = event.Probabilities[choice]
	if topic, ok := distribution.Answers["topic"]; ok {
		if sum := storyAlias[strings.TrimSpace(topic.Choice)]; sum.DocID != "" {
			label.TopicDocID, label.TopicHeadline, label.TopicP = sum.DocID, sum.Headline, topic.Probabilities[strings.TrimSpace(topic.Choice)]
		}
	}
	label.Model = firstNonEmpty(distribution.Model, candidateJudgmentModel)
	label.InputTokens, label.OutputTokens, label.CostUSD = distribution.Usage.InputTokens, distribution.Usage.OutputTokens, distribution.Usage.Cost
	return label, nil
}

// callIntakeJudge calls Jev on the intake rate scope within the per-call
// timeout. It returns at the deadline even when the judge ignores it.
func callIntakeJudge(ctx context.Context, judge CandidateJudge, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	return callJudgeWithin(ctx, judge, judgmentScopeIntakeLabels, intakeLabelCallTimeout, state, questions)
}

// callJudgeWithin calls Jev on a rate scope ("" for the judge's default
// bucket) within timeout. It returns at the deadline even when the judge
// ignores it.
func callJudgeWithin(ctx context.Context, judge CandidateJudge, scope string, timeout time.Duration, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	type result struct {
		raw json.RawMessage
		err error
	}
	done := make(chan result, 1)
	go func() {
		var r result
		if scoped, ok := judge.(ScopedCandidateJudge); ok && scope != "" {
			r.raw, r.err = scoped.JudgeScoped(callCtx, scope, state, questions)
		} else {
			r.raw, r.err = judge.Judge(callCtx, state, questions)
		}
		done <- r
	}()
	select {
	case r := <-done:
		return r.raw, r.err
	case <-callCtx.Done():
		return nil, fmt.Errorf("Jev: %w", callCtx.Err())
	}
}
