package agentcore

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/jevbudget"
)

// Jev calls are minimized (owner, 2026-11-15: "jev should run as many
// questions in the same call as possible. not multiple calls to the jev api.
// not concurrently or sequentially"). The planner takes the fewest calls the
// vendor's limits allow, and names the limit behind each split.
func splitFixture(sizes []int) func(lo, hi int) (json.RawMessage, map[string]json.RawMessage, error) {
	return func(lo, hi int) (json.RawMessage, map[string]json.RawMessage, error) {
		items := map[string]string{}
		questions := map[string]json.RawMessage{}
		for i := lo; i < hi; i++ {
			items[fmt.Sprintf("i%d", i)] = strings.Repeat("7", sizes[i]) // a digit is about one token
			questions[fmt.Sprintf("q%d", i)] = json.RawMessage(`{"type":"choice"}`)
		}
		state, _ := json.Marshal(items)
		return state, questions, nil
	}
}

func TestPlanJevCallsIsOneCallWhenItFits(t *testing.T) {
	sizes := make([]int, 20)
	for i := range sizes {
		sizes[i] = 500 // 10k tokens in all
	}
	plan, err := planJevCalls("test", len(sizes), splitFixture(sizes))
	if err != nil || len(plan.Ranges) != 1 || plan.Ranges[0] != [2]int{0, 20} || len(plan.Limits) != 0 {
		t.Fatalf("plan = %+v, %v; want one call over all 20", plan, err)
	}
}

func TestPlanJevCallsSplitsOnlyWhereALimitForcesItAndNamesIt(t *testing.T) {
	sizes := make([]int, 100)
	for i := range sizes {
		sizes[i] = 1000 // 100k tokens in all: over the 24k a question may read
	}
	plan, err := planJevCalls("test", len(sizes), splitFixture(sizes))
	if err != nil {
		t.Fatal(err)
	}
	// About 30 items fit under the 32k state-plus-question limit, so 100 items
	// are the fewest calls that cover them, in order and with no gap.
	if len(plan.Ranges) < 3 || len(plan.Ranges) > 4 {
		t.Fatalf("%d calls for 100k tokens under a 32k limit; the fewest is 4", len(plan.Ranges))
	}
	next := 0
	for _, r := range plan.Ranges {
		if r[0] != next {
			t.Fatalf("ranges %v leave a gap or overlap at %d", plan.Ranges, next)
		}
		next = r[1]
		state, qs, _ := splitFixture(sizes)(r[0], r[1])
		if !jevbudget.Fits(state, qs) {
			t.Fatalf("range %v is over the budget", r)
		}
	}
	if next != 100 {
		t.Fatalf("ranges %v cover %d of 100", plan.Ranges, next)
	}
	if len(plan.Limits) != len(plan.Ranges)-1 {
		t.Fatalf("limits %v: one named for each split", plan.Limits)
	}
	for _, l := range plan.Limits {
		if l != jevbudget.LimitQuestion {
			t.Fatalf("limit %q; the 32k state-plus-question limit forced these splits", l)
		}
	}
}

func TestPlanJevCallsALoneOversizeItemStandsAlone(t *testing.T) {
	sizes := []int{100, 40000, 100}
	plan, err := planJevCalls("test", 3, splitFixture(sizes))
	if err != nil || len(plan.Ranges) != 3 {
		t.Fatalf("plan = %+v, %v; want the oversize item alone, three calls", plan, err)
	}
}
