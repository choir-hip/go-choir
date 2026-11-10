package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Failure list: docs/design/autoradio-system1-route-2026-11-10.md.

type limitedJudge struct{}

func (limitedJudge) Judge(context.Context, json.RawMessage, map[string]json.RawMessage) (json.RawMessage, error) {
	return nil, errors.New("gateway client: Jev judgment rate limit exceeded")
}

func system1RoutePost(rt *Runtime, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/system1/judgments", strings.NewReader(body))
	req.Header.Set("X-Authenticated-User", "owner-1")
	rec := httptest.NewRecorder()
	NewAPIHandler(rt).HandleSystem1Judgments(rec, req)
	return rec
}

func system1RouteQuestions(n, choices int) string {
	var cs []string
	for i := 0; i < choices; i++ {
		cs = append(cs, fmt.Sprintf(`"c%d"`, i))
	}
	var qs []string
	for i := 0; i < n; i++ {
		qs = append(qs, fmt.Sprintf(`"q%d":{"type":"choice","instructions":"Pick.","choices":[%s]}`, i, strings.Join(cs, ",")))
	}
	return "{" + strings.Join(qs, ",") + "}"
}

func TestSystem1RouteRefusesPastItsLimits(t *testing.T) {
	rt, _ := testRuntime(t)
	judge := &scopeJudge{}
	rt.SetCandidateJudge(judge)
	for name, c := range map[string]struct {
		body string
		want int
	}{
		"oversized body":     {`{"state":{"x":"` + strings.Repeat("a", 65<<10) + `"},"questions":` + system1RouteQuestions(1, 2) + `}`, http.StatusRequestEntityTooLarge},
		"oversized state":    {`{"state":{"x":"` + strings.Repeat("a", 50<<10) + `"},"questions":` + system1RouteQuestions(1, 2) + `}`, http.StatusBadRequest},
		"too many questions": {`{"state":{},"questions":` + system1RouteQuestions(13, 2) + `}`, http.StatusBadRequest},
		"too many choices":   {`{"state":{},"questions":` + system1RouteQuestions(1, 9) + `}`, http.StatusBadRequest},
		"no questions":       {`{"state":{},"questions":{}}`, http.StatusBadRequest},
		"unknown field":      {`{"state":{},"questions":` + system1RouteQuestions(1, 2) + `,"model":"x"}`, http.StatusBadRequest},
		"score type":         {`{"state":{},"questions":{"q":{"type":"score","instructions":"x","choices":["a","b"]}}}`, http.StatusBadRequest},
		"bad label":          {`{"state":{},"label":"Bad Label","questions":` + system1RouteQuestions(1, 2) + `}`, http.StatusBadRequest},
		"duplicate choices":  {`{"state":{},"questions":{"q":{"type":"choice","instructions":"x","choices":["a","a"]}}}`, http.StatusBadRequest},
	} {
		if got := system1RoutePost(rt, c.body).Code; got != c.want {
			t.Errorf("%s: status %d, want %d", name, got, c.want)
		}
	}
	judge.mu.Lock()
	called := len(judge.scopes)
	judge.mu.Unlock()
	if called != 0 {
		t.Fatalf("a refused request reached the judge %d times", called)
	}
}

func TestSystem1RouteAsksOnTheInteractiveScopeAndMapsALimitTo429(t *testing.T) {
	rt, _ := testRuntime(t)
	judge := &scopeJudge{}
	rt.SetCandidateJudge(judge)
	rec := system1RoutePost(rt, `{"state":{},"label":"lab","questions":`+system1RouteQuestions(2, 3)+`}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	judge.mu.Lock()
	scopes := append([]string(nil), judge.scopes...)
	judge.mu.Unlock()
	if len(scopes) != 1 || scopes[0] != judgmentScopeInteractive {
		t.Fatalf("scopes %v, want one interactive call", scopes)
	}
	rt.SetCandidateJudge(limitedJudge{})
	rec = system1RoutePost(rt, `{"state":{},"questions":`+system1RouteQuestions(1, 2)+`}`)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("limiter refusal: status %d retry-after %q", rec.Code, rec.Header().Get("Retry-After"))
	}
}

// capturingJudge records what the route hands the gateway client.
type capturingJudge struct {
	questions map[string]json.RawMessage
}

func (j *capturingJudge) Judge(_ context.Context, _ json.RawMessage, q map[string]json.RawMessage) (json.RawMessage, error) {
	j.questions = q
	return json.RawMessage(`{"answers":{}}`), nil
}

// The vendor reads a choice question's criteria map (choice -> description)
// and rejects a choice question without one (400 on staging, 2026-11-10). The
// route must send the fields the in-guest verb sends for the same choices.
func TestSystem1RouteSendsTheShapeTheInGuestVerbSends(t *testing.T) {
	rt, _ := testRuntime(t)
	judge := &capturingJudge{}
	rt.SetCandidateJudge(judge)
	rec := system1RoutePost(rt, `{"state":{},"questions":{"mood":{"type":"choice","instructions":"Which mood?","choices":["calm","somber"]},"own":{"type":"choice","instructions":"Pick.","criteria":{"a":"Alpha.","b":"Beta."},"choices":["a","b"]}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	_, inGuest, err := system1Request([]yaegikernel.System1Question{{ID: "mood", Text: "Which mood?", Choices: []string{"calm", "somber"}}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var sent, want map[string]any
	if err := json.Unmarshal(judge.questions["mood"], &sent); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(inGuest["mood"], &want); err != nil {
		t.Fatal(err)
	}
	if len(sent) != len(want) {
		t.Errorf("sent fields %v, in-guest fields %v", sent, want)
	}
	for f := range want {
		if _, ok := sent[f]; !ok {
			t.Errorf("field %s is sent in-guest but not by the route", f)
		}
	}
	crit, ok := sent["criteria"].(map[string]any)
	if !ok || len(crit) != 2 || crit["calm"] == nil || crit["somber"] == nil {
		t.Fatalf("criteria %v, want one description per choice", sent["criteria"])
	}
	var own map[string]any
	_ = json.Unmarshal(judge.questions["own"], &own)
	if c, _ := own["criteria"].(map[string]any); c["a"] != "Alpha." || c["b"] != "Beta." {
		t.Fatalf("a client's own criteria were not kept: %v", own["criteria"])
	}
}

func TestSystem1RouteRefusesCriteriaThatDoNotMatchTheChoices(t *testing.T) {
	rt, _ := testRuntime(t)
	judge := &capturingJudge{}
	rt.SetCandidateJudge(judge)
	for name, crit := range map[string]string{
		"missing key": `{"a":"x"}`,
		"extra key":   `{"a":"x","b":"y","c":"z"}`,
		"not strings": `{"a":"x","b":3}`,
		"not a map":   `["a","b"]`,
	} {
		rec := system1RoutePost(rt, `{"state":{},"questions":{"q":{"type":"choice","instructions":"x","criteria":`+crit+`,"choices":["a","b"]}}}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, rec.Code)
		}
	}
	if judge.questions != nil {
		t.Fatal("a refused request reached the judge")
	}
}
