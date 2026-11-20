package agentcore

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

// The event layer's call is handled as intake's is (jev_failure.go), through
// the production client against a gateway that fails as the real one does.
func TestEventLayerGateway502IsRetriedThenHalvedAndLosesNoCapture(t *testing.T) {
	fastRetries(t)
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	rt, owner, computer := eventRuntime(t, judge)
	gw, wired := newFakeGateway(t, judge, func(n, questions int) int {
		if questions > 6 {
			return http.StatusBadGateway
		}
		return 0
	})
	rt.SetCandidateJudge(wired)
	var inputs []eventInput
	for i := 0; i < 12; i++ {
		inputs = append(inputs, capItem(fmt.Sprintf("g%d", i), fmt.Sprintf("Gateway story number %d about topic", i), fmt.Sprintf("https://outlet%d.com/a", i), ""))
	}
	r, err := rt.resolveEvents(context.Background(), owner, computer, inputs)
	if err != nil {
		t.Fatal(err)
	}
	// 12 questions refused twice, then two halves of 6.
	if r.Unresolved != 0 || gw.count() != 4 {
		t.Fatalf("report = %+v, %d gateway requests; want every capture resolved in 4 requests", r, gw.count())
	}
}

func TestEventLayerGatewayPersistentFailureIsBoundedAndA429IsNeverHalved(t *testing.T) {
	fastRetries(t)
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	rt, owner, computer := eventRuntime(t, judge)
	gw, wired := newFakeGateway(t, judge, func(n, questions int) int { return http.StatusBadGateway })
	rt.SetCandidateJudge(wired)
	var inputs []eventInput
	for i := 0; i < 12; i++ {
		inputs = append(inputs, capItem(fmt.Sprintf("h%d", i), fmt.Sprintf("Persistent story number %d about topic", i), fmt.Sprintf("https://outlet%d.com/a", i), ""))
	}
	r, _ := rt.resolveEvents(context.Background(), owner, computer, inputs)
	if gw.count() != 8 || r.Unresolved != 12 {
		t.Fatalf("report = %+v, %d requests; want 8 requests and all 12 unresolved", r, gw.count())
	}
	rate, wired2 := newFakeGateway(t, judge, func(n, questions int) int { return http.StatusTooManyRequests })
	rt.SetCandidateJudge(wired2)
	var more []eventInput
	for i := 0; i < 12; i++ {
		more = append(more, capItem(fmt.Sprintf("k%d", i), fmt.Sprintf("Rate story number %d about topic", i), fmt.Sprintf("https://outlet%d.org/a", i), ""))
	}
	_, _ = rt.resolveEvents(context.Background(), owner, computer, more)
	if rate.count() != 3 {
		t.Fatalf("%d requests on a persistent 429; want 3, never halved", rate.count())
	}
}
