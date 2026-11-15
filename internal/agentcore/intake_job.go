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

	// Cut each live capture's options into parts: its candidate stories in its
	// own order, then every other capture of the poll (none is trimmed, so a
	// reworded or translated duplicate is always offered). A part is as large
	// as the vendor allows: at most intakeOptionsMax options (255 choices less
	// new and a margin), and the part's entries, the capture's own entry and
	// the question text inside the per-question token limit. A split between
	// parts is named by the limit that forced it when the job is planned.
	parts := make([][]intakePart, len(p.items))
	maxParts := 0
	for _, i := range live {
		type option struct {
			handle string
			story  bool
			tokens int
		}
		var options []option
		for _, h := range p.views[i].order {
			options = append(options, option{h, true, storyTokens[h] + 2*key(h)})
		}
		for _, h := range p.views[i].siblings {
			options = append(options, option{h, false, siblingTokens[siblingIndex(h)] + key(h)})
		}
		// What a part's options may add to the state a question reads: the
		// per-question limit less the intercept, the shared state, the
		// capture's own entry and the question text.
		avail := jevbudget.QuestionBudgetTokens - 412 - baseTokens - fullTokens[i] - 600
		for n, start := 0, 0; n == 0 || start < len(options); n++ {
			end, used := start, 0
			for end < len(options) && end-start < intakeOptionsMax {
				if used+options[end].tokens > avail && end > start {
					break
				}
				used += options[end].tokens
				end++
			}
			part := intakePart{eventName: "event_" + p.handles[i]}
			if n > 0 {
				part.eventName += fmt.Sprintf("_s%d", n+1)
			}
			var siblings []string
			for _, o := range options[start:end] {
				if o.story {
					part.storyChoices = append(part.storyChoices, o.handle)
				} else {
					siblings = append(siblings, o.handle)
				}
			}
			part.eventChoices = append(append(append([]string(nil), part.storyChoices...), siblings...), store.CaptureLabelEventNew)
			part.siblingChoices = siblings
			if len(part.storyChoices) > 0 {
				part.topicName = "topic_" + p.handles[i]
				if n > 0 {
					part.topicName += fmt.Sprintf("_s%d", n+1)
				}
			}
			parts[i] = append(parts[i], part)
			start = end
			if end >= len(options) {
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
			siblings := len(part.siblingChoices) > 0
			if siblings {
				needs = append([]jevRef(nil), topicNeeds...)
				for _, h := range part.siblingChoices {
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
	var mk func(jc jevCall) intakeCall
	mk = func(jc jevCall) intakeCall {
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
		// halve is the refusal fallback: the same questions in two calls.
		c.halve = func() []intakeCall {
			halves, ok := job.Halve(jc)
			if !ok {
				return nil
			}
			out := make([]intakeCall, 0, len(halves))
			for _, h := range halves {
				out = append(out, mk(h))
			}
			return out
		}
		return c
	}
	var calls []intakeCall
	for _, jc := range jobCalls {
		calls = append(calls, mk(jc))
	}
	return calls, nil
}
