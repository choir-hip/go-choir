package agentcore

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/yusefmosiah/go-choir/internal/jevbudget"
)

// jevJob is the one entry point for asking Jev about a job: one unit of work
// (a capture flush, a judge pass) builds ONE request here and every question
// of it is asked in as few calls as the vendor's limits allow (owner,
// 2026-11-15: "as many questions in the same call as possible. not multiple
// calls to the jev api. not concurrently or sequentially";
// docs/problems/jev-misuse-structural-assessment-2026-11-15.md).
//
// A site adds the job's state as keyed entries and its questions, each naming
// the entries it reads. Plan returns the calls: one when everything fits
// jevbudget's limits, otherwise the fewest, each carrying only the entries
// its questions read, and every split names the limit that forced it. A site
// does not size, chunk, batch, queue or fan out; it reads its answers back by
// question name. Nothing here bounds questions per call but the vendor's
// token limits.
type jevJob struct {
	site string
	// base is the state every call carries (the task text, shared context).
	base map[string]any
	// groups are the entry groups in rendering order; a list group renders as
	// an array, a map group as an object keyed by entry key.
	groups  []string
	isList  map[string]bool
	entries map[string][]*jevEntry
	index   map[jevKey]*jevEntry

	names     []string
	questions map[string]json.RawMessage
	decisions map[string]string
	needs     map[string][]jevRef
	// starts are the indexes in names where a unit begins; questions of a unit
	// read the same entries, so a split keeps them together unless one unit
	// alone is over the limits. With none, every question is its own unit.
	starts []int
}

type jevKey struct{ group, key string }

// jevEntry is one piece of the job's state. Its levels are renderings from
// short to full; a call renders the highest level any of its questions needs.
type jevEntry struct {
	key    string
	levels []any
}

// jevRef names an entry a question reads, at a level.
type jevRef struct {
	Group, Key string
	Level      int
}

func newJevJob(site string, base map[string]any) *jevJob {
	return &jevJob{site: site, base: base, isList: map[string]bool{}, entries: map[string][]*jevEntry{}, index: map[jevKey]*jevEntry{},
		questions: map[string]json.RawMessage{}, decisions: map[string]string{}, needs: map[string][]jevRef{}}
}

// List declares a group rendered as an array in entry order.
func (j *jevJob) List(group string) { j.declare(group, true) }

// Map declares a group rendered as an object by entry key.
func (j *jevJob) Map(group string) { j.declare(group, false) }

func (j *jevJob) declare(group string, list bool) {
	if _, ok := j.isList[group]; !ok {
		j.groups = append(j.groups, group)
	}
	j.isList[group] = list
}

// Entry adds a piece of state in group under key, with its renderings from
// short to full.
func (j *jevJob) Entry(group, key string, levels ...any) {
	if _, ok := j.isList[group]; !ok {
		j.declare(group, false)
	}
	e := &jevEntry{key: key, levels: levels}
	j.entries[group] = append(j.entries[group], e)
	j.index[jevKey{group, key}] = e
}

// Unit starts a new unit of questions: the ones added next, until the next
// Unit, are kept in one call when they can be.
func (j *jevJob) Unit() { j.starts = append(j.starts, len(j.names)) }

// Ask adds a question under name, with the decision it feeds and the entries
// it reads. Questions keep the order they are added in; a split cuts that
// order, so a site adds questions whose entries overlap next to each other.
func (j *jevJob) Ask(name string, question json.RawMessage, decision string, needs ...jevRef) {
	j.names = append(j.names, name)
	j.questions[name] = question
	j.decisions[name] = decision
	j.needs[name] = needs
}

// jevCall is one request of a job: its state, its questions and the names
// asked, in the order added.
type jevCall struct {
	State     json.RawMessage
	Questions map[string]json.RawMessage
	Decisions map[string]string
	Names     []string
	// Lo and Hi are the range of the job's questions this call carries.
	Lo, Hi int
}

// build is the request for the questions names[lo:hi].
func (j *jevJob) build(lo, hi int) (jevCall, error) {
	level := map[jevKey]int{}
	for _, name := range j.names[lo:hi] {
		for _, r := range j.needs[name] {
			k := jevKey{r.Group, r.Key}
			if cur, ok := level[k]; !ok || r.Level > cur {
				level[k] = r.Level
			}
		}
	}
	state := make(map[string]any, len(j.base)+len(j.groups))
	for k, v := range j.base {
		state[k] = v
	}
	for _, g := range j.groups {
		var list []any
		obj := map[string]any{}
		for _, e := range j.entries[g] {
			l, ok := level[jevKey{g, e.key}]
			if !ok || len(e.levels) == 0 {
				continue
			}
			v := e.levels[min(l, len(e.levels)-1)]
			if j.isList[g] {
				list = append(list, v)
			} else {
				obj[e.key] = v
			}
		}
		if j.isList[g] && len(list) > 0 {
			state[g] = list
		} else if !j.isList[g] && len(obj) > 0 {
			state[g] = obj
		}
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return jevCall{}, err
	}
	c := jevCall{State: raw, Questions: map[string]json.RawMessage{}, Decisions: map[string]string{}, Names: append([]string(nil), j.names[lo:hi]...), Lo: lo, Hi: hi}
	for _, name := range c.Names {
		c.Questions[name] = j.questions[name]
		c.Decisions[name] = j.decisions[name]
	}
	return c, nil
}

// Plan returns the job's calls: one when the whole job fits the vendor's
// limits, otherwise the fewest. It logs calls, questions and the largest
// estimated request for the job, and each split with the limit that forced it.
func (j *jevJob) Plan() ([]jevCall, error) {
	if len(j.names) == 0 {
		return nil, nil
	}
	// Plan over units: the index ranges of questions a unit holds.
	bounds := append([]int(nil), j.starts...)
	if len(bounds) == 0 || bounds[0] != 0 {
		bounds = append([]int{0}, bounds...)
	}
	if len(j.starts) == 0 {
		bounds = bounds[:0]
		for i := range j.names {
			bounds = append(bounds, i)
		}
	}
	bounds = append(bounds, len(j.names))
	plan, err := planJevCalls(j.site, len(bounds)-1, func(lo, hi int) (json.RawMessage, map[string]json.RawMessage, error) {
		c, err := j.build(bounds[lo], bounds[hi])
		return c.State, c.Questions, err
	})
	if err != nil {
		return nil, err
	}
	for i := range plan.Ranges {
		plan.Ranges[i] = [2]int{bounds[plan.Ranges[i][0]], bounds[plan.Ranges[i][1]]}
	}
	calls := make([]jevCall, 0, len(plan.Ranges))
	maxTokens := 0
	for _, r := range plan.Ranges {
		c, err := j.build(r[0], r[1])
		if err != nil {
			return nil, err
		}
		maxTokens = max(maxTokens, jevbudget.Measure(c.State, c.Questions).Request)
		calls = append(calls, c)
	}
	log.Printf("jev job: site=%s calls=%d questions=%d est_tokens_max=%d", j.site, len(calls), len(j.names), maxTokens)
	if len(calls) > 1 && len(plan.Limits) != len(calls)-1 {
		return nil, fmt.Errorf("jev job %s: %d calls but %d named limits", j.site, len(calls), len(plan.Limits))
	}
	return calls, nil
}

// Halve splits a call at the unit boundary nearest its middle, for a request
// the vendor refused for size although the estimate said it fit. It reports
// false when the call is one unit and cannot be split.
func (j *jevJob) Halve(c jevCall) ([]jevCall, bool) {
	best, bestDist := -1, 1<<30
	mid := (c.Lo + c.Hi) / 2
	starts := j.starts
	if len(starts) == 0 {
		for i := range j.names {
			starts = append(starts, i)
		}
	}
	for _, b := range starts {
		if b > c.Lo && b < c.Hi {
			if d := abs(b - mid); d < bestDist {
				best, bestDist = b, d
			}
		}
	}
	if best < 0 {
		return nil, false
	}
	left, err := j.build(c.Lo, best)
	if err != nil {
		return nil, false
	}
	right, err := j.build(best, c.Hi)
	if err != nil {
		return nil, false
	}
	return []jevCall{left, right}, true
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
