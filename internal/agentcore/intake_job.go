package agentcore

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/yusefmosiah/go-choir/internal/jevbudget"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// planIntakeJobCalls is the flush's one Jev request on the job entry point
// (jevJob): every live capture's event and topic questions, every story and
// every capture once, sent as one call; split only where jevbudget's limits
// refuse the one call, each call carrying only the stories its questions
// read (owner, 2026-11-15: as many questions in the same call as possible).
//
// A capture's candidate stories are asked in parts, so that a question stays
// under the vendor's 255 choices and a part's stories, with the capture and
// the other captures, stay inside the per-question token limit. Parts are
// added part by part across captures, so the stories a split call carries are
// a contiguous run.
func (rt *Runtime) planIntakeJobCalls(lib *intakeLibrary, p *intakePoll, live []int) ([]intakeCall, error) {
	base := map[string]any{
		"task": "News captures have arrived (state.captures). Each question is about one of them: which story in state.stories or other new capture reports the same real-world event, and which story shares its topic. Titles and summaries may be in any language; read them all.",
	}
	if len(lib.examples) > 0 {
		var corrections []map[string]string
		for _, o := range lib.examples {
			corrections = append(corrections, map[string]string{"capture": o.CaptureTitle, "label_said": o.JevSaid, "editor_did": overrideDisposition(o), "reason": o.Reason})
		}
		base["corrections"] = corrections
	}
	job := newJevJob("intake_labels", base)
	job.List("stories")
	job.Map("captures")
	storyTokens := map[string]int{}
	for _, s := range p.stories {
		job.Entry("stories", s.handle, s.entry)
		storyTokens[s.handle] = jevbudget.Tokens([]byte(strconv.Quote(s.entry))) + 1
	}
	siblingTokens, fullTokens := make([]int, len(p.items)), make([]int, len(p.items))
	for j, other := range p.items {
		short := intakeCaptureState(sourcegraph.CapturedItem{Title: other.Title, Summary: other.Summary, Outlet: other.Outlet, Language: other.Language}, intakeSiblingSummaryRunes)
		full := intakeCaptureState(p.items[j], intakeSummaryRunes)
		job.Entry("captures", p.handles[j], short, full)
		rs, _ := json.Marshal(short)
		rf, _ := json.Marshal(full)
		siblingTokens[j], fullTokens[j] = jevbudget.Tokens(rs), jevbudget.Tokens(rf)
	}
	baseRaw, _ := json.Marshal(base)
	baseTokens := jevbudget.Tokens(baseRaw)
	key := func(h string) int { return jevbudget.Tokens([]byte(strconv.Quote(h) + ":null,")) }

	// Cut each live capture's candidates into parts.
	parts := make([][]intakePart, len(p.items))
	maxParts := 0
	for _, i := range live {
		own := p.views[i].order
		sibs := 0
		for j := range p.items {
			if j != i {
				sibs += siblingTokens[j] + key(p.handles[j])
			}
		}
		perCount := intakeStoryOptions(len(p.items))
		// What a part's stories may add to the state a question reads: the
		// per-question budget less the intercept, the shared state, the
		// capture's own entry, the question text and its keys.
		avail := jevbudget.QuestionBudgetTokens - 412 - baseTokens - fullTokens[i] - 600
		for n, start := 0, 0; n == 0 || start < len(own); n++ {
			room := avail
			if n == 0 {
				room -= sibs
			}
			end, used := start, 0
			for end < len(own) && end-start < perCount {
				c := storyTokens[own[end]] + 2*key(own[end])
				if used+c > room && end > start {
					break
				}
				used += c
				end++
			}
			part := intakePart{eventName: "event_" + p.handles[i], storyChoices: own[start:end]}
			if n > 0 {
				part.eventName += fmt.Sprintf("_s%d", n+1)
			}
			var siblings []string
			if n == 0 {
				siblings = p.views[i].siblings
			}
			part.eventChoices = append(append(append([]string(nil), part.storyChoices...), siblings...), store.CaptureLabelEventNew)
			if len(part.storyChoices) > 0 {
				part.topicName = "topic_" + p.handles[i]
				if n > 0 {
					part.topicName += fmt.Sprintf("_s%d", n+1)
				}
			}
			parts[i] = append(parts[i], part)
			start = end
			if end == len(own) {
				break
			}
		}
		maxParts = max(maxParts, len(parts[i]))
	}

	// Add the questions part by part across captures.
	type where struct{ capture, part int }
	at := map[string]where{}
	for n := 0; n < maxParts; n++ {
		for _, i := range live {
			if n >= len(parts[i]) {
				continue
			}
			part := parts[i][n]
			job.Unit()
			// What the questions read: the capture in full and the part's
			// stories; the event question also reads the other captures.
			topicNeeds := []jevRef{{"captures", p.handles[i], 1}}
			for _, h := range part.storyChoices {
				topicNeeds = append(topicNeeds, jevRef{"stories", h, 0})
			}
			needs := topicNeeds
			siblings := n == 0 && len(p.views[i].siblings) > 0
			if siblings {
				needs = append([]jevRef(nil), topicNeeds...)
				for _, h := range p.views[i].siblings {
					needs = append(needs, jevRef{"captures", h, 0})
				}
			}
			event, err := json.Marshal(map[string]any{
				"type":         "choice",
				"instructions": intakeEventInstructions(p.handles[i], len(part.storyChoices) > 0, siblings, len(lib.examples) > 0),
				"criteria":     intakeCriteria(part.eventChoices, map[string]string{store.CaptureLabelEventNew: "No story and no other new capture reports this event; a shared topic or shared actors alone do not count."}),
			})
			if err != nil {
				return nil, err
			}
			job.Ask(part.eventName, event, p.items[i].CaptureID, needs...)
			at[part.eventName] = where{i, n}
			if part.topicName != "" {
				topic, err := json.Marshal(map[string]any{
					"type":         "choice",
					"instructions": "Which story in state.stories shares state.captures." + p.handles[i] + "'s topic: the same ongoing subject (a war, an election, a company, a trial), even when it is a different event? The options are the story handles (c1, c2, … each at the start of its entry in state.stories) and none; answer none when no story does.",
					"criteria":     intakeCriteria(append(append([]string(nil), part.storyChoices...), "none"), map[string]string{"none": "No story shares the capture's topic."}),
				})
				if err != nil {
					return nil, err
				}
				job.Ask(part.topicName, topic, p.items[i].CaptureID, topicNeeds...)
				at[part.topicName] = where{i, n}
			}
		}
	}
	jobCalls, err := job.Plan()
	if err != nil {
		return nil, err
	}
	var calls []intakeCall
	for _, jc := range jobCalls {
		c := intakeCall{first: jc.Decisions[jc.Names[0]], state: jc.State, questions: jc.Questions, decisions: jc.Decisions}
		byCapture := map[int]*intakeAsk{}
		var order []int
		for _, name := range jc.Names {
			w := at[name]
			ask := byCapture[w.capture]
			if ask == nil {
				ask = &intakeAsk{capture: w.capture}
				byCapture[w.capture] = ask
				order = append(order, w.capture)
			}
			if parts[w.capture][w.part].eventName == name {
				ask.parts = append(ask.parts, parts[w.capture][w.part])
			}
		}
		for _, i := range order {
			c.asks = append(c.asks, *byCapture[i])
		}
		calls = append(calls, c)
	}
	return calls, nil
}
