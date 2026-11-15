package agentcore

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/jevbudget"
)

// The job entry point. Ways it can fail, written before the code:
//
//	E1. A job that fits the vendor's limits makes more than one call.
//	E2. A split call carries entries none of its questions read, so the state
//	    is sent in full more than once.
//	E3. A split is made with no limit named, or with a limit the job did not
//	    cross.
//	E4. A call is sized by a constant of ours (questions or items a call), not
//	    by the token limits: a 300-question job that fits is split.
//	E5. An entry is rendered at the short level when a question in the call
//	    needs the full one.
func newTestJob(stories, questions int, storyTokens int) *jevJob {
	j := newJevJob("test", map[string]any{"task": "decide"})
	j.List("stories")
	j.Map("captures")
	for i := 0; i < stories; i++ {
		j.Entry("stories", fmt.Sprintf("c%d", i+1), fmt.Sprintf("c%d: %s", i+1, strings.Repeat("7", storyTokens)))
	}
	for i := 0; i < questions; i++ {
		h := fmt.Sprintf("n%d", i+1)
		j.Entry("captures", h, "short "+h, "full "+h)
		q, _ := json.Marshal(map[string]string{"type": "choice", "instructions": "which story for " + h})
		j.Ask("event_"+h, q, "cap-"+h, jevRef{"captures", h, 1}, jevRef{"stories", fmt.Sprintf("c%d", i%max(stories, 1)+1), 0})
	}
	return j
}

func TestJevJobThatFitsIsOneCallWithAllItsQuestions(t *testing.T) {
	// E1, E4: 300 questions over 300 stories fit the budget and are one call.
	j := newTestJob(300, 300, 20)
	calls, err := j.Plan()
	if err != nil || len(calls) != 1 {
		t.Fatalf("%d calls, %v; a job that fits is exactly one call", len(calls), err)
	}
	if len(calls[0].Questions) != 300 || !jevbudget.Fits(calls[0].State, calls[0].Questions) {
		t.Fatalf("the call carries %d questions", len(calls[0].Questions))
	}
	var state struct {
		Task     string            `json:"task"`
		Stories  []string          `json:"stories"`
		Captures map[string]string `json:"captures"`
	}
	if err := json.Unmarshal(calls[0].State, &state); err != nil {
		t.Fatal(err)
	}
	if state.Task != "decide" || len(state.Stories) != 300 || state.Captures["n7"] != "full n7" {
		t.Fatalf("state: task %q, %d stories, capture n7 = %q (E5)", state.Task, len(state.Stories), state.Captures["n7"])
	}
	if state.Stories[0] != "c1: "+strings.Repeat("7", 20) {
		t.Fatalf("stories are not in entry order: %q", state.Stories[0])
	}
}

func TestJevJobSplitsOnlyOnANamedLimitAndEachCallCarriesOnlyWhatItReads(t *testing.T) {
	// 100 stories of 500 tokens is 50k tokens of state: over the 24k a
	// question may read together with its state, so the stories cannot be sent
	// together; each question reads one story, so each call carries only the
	// stories of its questions (E2).
	j := newTestJob(100, 100, 500)
	calls, err := j.Plan()
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) < 2 || len(calls) > 4 {
		t.Fatalf("%d calls for 100 one-story questions of 500 tokens; want 2 to 4", len(calls))
	}
	asked := 0
	for _, c := range calls {
		if !jevbudget.Fits(c.State, c.Questions) {
			t.Fatalf("a call is over the budget: %s", jevbudget.Limit(c.State, c.Questions))
		}
		var state struct {
			Stories []string `json:"stories"`
		}
		_ = json.Unmarshal(c.State, &state)
		if len(state.Stories) != len(c.Questions) {
			t.Fatalf("a call of %d questions carries %d stories; each question reads one (E2)", len(c.Questions), len(state.Stories))
		}
		asked += len(c.Questions)
	}
	if asked != 100 {
		t.Fatalf("%d of 100 questions asked", asked)
	}
}

func TestJevJobPlanNamesTheLimitBehindEverySplit(t *testing.T) {
	// E3: the planner returns one limit per split, and it is the vendor's
	// state-plus-question limit here.
	j := newTestJob(100, 100, 500)
	var plan jevSplit
	plan, err := planJevCalls("test", len(j.names), func(lo, hi int) (json.RawMessage, map[string]json.RawMessage, error) {
		c, err := j.build(lo, hi)
		return c.State, c.Questions, err
	})
	if err != nil || len(plan.Limits) != len(plan.Ranges)-1 {
		t.Fatalf("plan %+v, %v", plan, err)
	}
	for _, l := range plan.Limits {
		if l == "" {
			t.Fatal("a split with no limit named")
		}
	}
}
