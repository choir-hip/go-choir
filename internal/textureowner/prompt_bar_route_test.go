package textureowner

// MA4.8, the prompt router at the prompt bar (plan section 9.2). Ways the
// owner's door could fail (written first):
//
//  D1 A routed submit to a package desk opens nothing, creates a Texture
//     anyway, or writes no hand-off record to that desk.
//  D2 A router failure (model error) fails the submit instead of taking the
//     default route: a new Texture, as before the router.
//  D3 An in-process desk caller (management's ProductAPI) is routed away from
//     the Texture it asked for, or spends a judge call.
//  D4 A focus reaches the conductor (a judge call on a focused submit), or a
//     focus may name a system desk, a role, an uninstalled package or a
//     candidate desk, or an in-process caller may submit a focus.
//  D5 A focused submit to a package desk carries no owner-origin request with
//     the focus, labelled for that app only.
//  D6 The decision run does not carry the route record it followed.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/promptroute"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// routeJudge answers the router's question with prefer at p, or fails.
type routeJudge struct {
	mu     sync.Mutex
	calls  int
	prefer string
	p      float64
	err    error
}

func (j *routeJudge) Judge(_ context.Context, _ json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.calls++
	if j.err != nil {
		return nil, j.err
	}
	answers := map[string]any{}
	for id, raw := range questions {
		var q struct {
			Choices []string `json:"choices"`
		}
		_ = json.Unmarshal(raw, &q)
		probs := map[string]float64{}
		for _, c := range q.Choices {
			probs[c] = (1 - j.p) / float64(len(q.Choices)-1)
		}
		probs[j.prefer] = j.p
		answers[id] = map[string]any{"type": "choice", "choice": j.prefer, "probabilities": probs}
	}
	return json.Marshal(map[string]any{"id": "gen-route", "model": "typesafe/jev-test", "answers": answers})
}

func (j *routeJudge) count() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.calls
}

func registerRouteDesk(t *testing.T, slug string, uid uint32, accepts ...string) string {
	t.Helper()
	d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: "app:" + slug, Verbs: []string{"Cast"},
		Taint: deskmanifest.TaintUntrustedInput, UID: uid, WorkerMaxBytes: 64 << 20, Accepts: accepts})
	if err != nil {
		t.Fatal(err)
	}
	if err := deskmanifest.RegisterApp(d); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deskmanifest.RetireApp("app:" + slug) })
	return "app:" + slug
}

func decisionOf(t *testing.T, handler *Handler, owner, submissionID string) ConductorDecision {
	t.Helper()
	rec, err := handler.Core.GetRun(context.Background(), submissionID, owner)
	if err != nil {
		t.Fatal(err)
	}
	var d ConductorDecision
	if err := json.Unmarshal([]byte(rec.Result), &d); err != nil {
		t.Fatalf("decision %q: %v", rec.Result, err)
	}
	return d
}

func ownerRecords(t *testing.T, handler *Handler, owner, addressee string) []types.CommitmentRecord {
	t.Helper()
	recs, err := handler.Core.Store().ListCommitmentRecords(context.Background(), owner, handler.Core.TextureComputerID(), addressee, 100)
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

func submitted(t *testing.T, body string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return out
}

// D1, D6
func TestPromptBarRoutesToAnInstalledPackageDesk(t *testing.T) {
	core, handler := testAPISetup(t)
	desk := registerRouteDesk(t, "routebar", 62951, "read, find or summarize the owner's email")
	judge := &routeJudge{prefer: desk, p: 0.9}
	core.SetCandidateJudge(judge)
	owner := "user-route"
	w := postPromptBar(t, handler, owner, map[string]any{"text": "summarize my inbox from today", "command_id": "route-mail"}, false)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	resp := submitted(t, w.Body.String())
	if resp["doc_id"] != nil && resp["doc_id"] != "" {
		t.Fatalf("D1: a routed submit created a Texture: %v", resp)
	}
	d := decisionOf(t, handler, owner, resp["submission_id"].(string))
	if d.Action != "open_app" || d.App != "routebar" || d.RouteRule != promptroute.RuleJudge || d.RouteRecordID == "" {
		t.Fatalf("D1/D6: decision = %+v", d)
	}
	reqs := ownerRecords(t, handler, owner, desk+":"+owner)
	if len(reqs) != 1 || reqs[0].Provenance.Origin != types.OriginOwner || reqs[0].Directive == nil ||
		reqs[0].Directive.Objective != "summarize my inbox from today" || len(reqs[0].RelatedIDs) != 1 || reqs[0].RelatedIDs[0] != d.RouteRecordID {
		t.Fatalf("D1: hand-off records = %+v", reqs)
	}
	// A retry of the same command pays nothing and opens the same app.
	calls := judge.count()
	again := postPromptBar(t, handler, owner, map[string]any{"text": "summarize my inbox from today", "command_id": "route-mail"}, false)
	if again.Code != http.StatusAccepted || judge.count() != calls {
		t.Fatalf("retry status=%d calls %d -> %d", again.Code, calls, judge.count())
	}
}

// D2
func TestPromptBarRouterFailureTakesTheDefaultRoute(t *testing.T) {
	core, handler := testAPISetup(t)
	registerRouteDesk(t, "routefail", 62952, "email")
	core.SetCandidateJudge(&routeJudge{err: errors.New("provider 500")})
	owner := "user-route-fail"
	w := postPromptBar(t, handler, owner, map[string]any{"text": "write one sentence about tides", "command_id": "route-fail"}, false)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	resp := submitted(t, w.Body.String())
	if resp["doc_id"] == nil || resp["doc_id"] == "" {
		t.Fatalf("D2: the default route created no Texture: %v", resp)
	}
	d := decisionOf(t, handler, owner, resp["submission_id"].(string))
	if d.App != agentprofile.Texture || d.RouteRule != promptroute.RuleJudgeFailed || d.RouteRecordID == "" {
		t.Fatalf("D2: decision = %+v", d)
	}
}

// D3
func TestPromptBarInProcessCallerIsNotRouted(t *testing.T) {
	core, handler := testAPISetup(t)
	desk := registerRouteDesk(t, "routeinproc", 62953, "email")
	judge := &routeJudge{prefer: desk, p: 0.99}
	core.SetCandidateJudge(judge)
	w := postPromptBar(t, handler, "user-inproc", map[string]any{"text": "a Texture about my email", "command_id": "inproc"}, true)
	if w.Code != http.StatusAccepted || judge.count() != 0 {
		t.Fatalf("D3: status=%d judge calls=%d body=%s", w.Code, judge.count(), w.Body.String())
	}
}

// D4, D5
func TestPromptBarFocusGoesToTheFocusedAppAgentOnly(t *testing.T) {
	core, handler := testAPISetup(t)
	desk := registerRouteDesk(t, "routefocus", 62954, "email")
	registerRouteDesk(t, "routefocus-candidate", 62955, "email")
	judge := &routeJudge{prefer: desk, p: 0.99}
	core.SetCandidateJudge(judge)
	owner := "user-focus"
	focus := map[string]string{"app_agent_id": desk, "object_ref": "msg-42", "selection": "the second paragraph"}
	w := postPromptBar(t, handler, owner, map[string]any{"text": "reply yes", "command_id": "focus-1", "focus": focus}, false)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if judge.count() != 0 {
		t.Fatalf("D4: a focused submit asked the conductor (%d calls)", judge.count())
	}
	reqs := ownerRecords(t, handler, owner, desk+":"+owner)
	if len(reqs) != 1 || reqs[0].Directive.TargetRef != "msg-42" || reqs[0].Directive.Body != "the second paragraph" ||
		strings.Join(reqs[0].Provenance.Labels, ",") != types.OwnerPrivateLabel("routefocus") {
		t.Fatalf("D5: focused request = %+v", reqs)
	}
	if recs := ownerRecords(t, handler, owner, promptroute.Service); len(recs) != 0 {
		t.Fatalf("D4: a focused submit wrote a route record: %+v", recs)
	}
	for i, name := range []string{agentprofile.Engineering, agentprofile.Management, agentprofile.Research, agentprofile.Conductor,
		"research_web", "app:not-installed", "app:routefocus-candidate", ""} {
		w := postPromptBar(t, handler, owner, map[string]any{"text": "do it", "command_id": "bad-focus-" + string(rune('a'+i)),
			"focus": map[string]string{"app_agent_id": name, "object_ref": "x"}}, false)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("D4: focus on %q: status=%d body=%s", name, w.Code, w.Body.String())
		}
	}
	if w := postPromptBar(t, handler, owner, map[string]any{"text": "do it", "command_id": "inproc-focus", "focus": focus}, true); w.Code != http.StatusForbidden {
		t.Fatalf("D4: in-process focus status=%d", w.Code)
	}
	if w := postPromptBar(t, handler, owner, map[string]any{"text": "do it", "command_id": "unknown-field",
		"focus": map[string]string{"app_agent_id": desk, "object_ref": "x", "desk": "engineering"}}, false); w.Code != http.StatusBadRequest {
		t.Fatalf("D4: an unknown focus field status=%d", w.Code)
	}
}
