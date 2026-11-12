package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/jevbudget"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The shadow comparison of intake labelling (cmd/intake-shadow): the old way,
// the stories cut into 15,000-byte slices and a call closed at 30,000 bytes
// and 6 captures, against the single state sized by the vendor's token limits.
// Both arms read the same captures and the same story window and are asked of
// the same Jev; the report gives calls, questions, request bytes, estimated
// and (when run) billed input tokens, and the label agreement per question.
// The plan-only form makes no call and costs nothing; the run form is paid and
// is meant for d@e, never the paper.

// intakeShadowPricePerMTokens is Jev's price in dollars per million input
// tokens (docs/reference/SYSTEM1.md section 7).
const intakeShadowPricePerMTokens = 0.042

// legacy limits, kept only as the shadow's baseline arm.
const (
	legacyIntakeChunkBytes   = 15000
	legacyIntakeRequestBytes = 30000
	legacyIntakeCallCaptures = 6
)

// intakeLegacyBytePolicy is the policy the intake labels ran under until
// 2026-11-12: slices of stories by bytes, a call closed by bytes.
var intakeLegacyBytePolicy = intakePolicy{
	storyCost: func(s intakeStory) (int, int) {
		c := len(s.entry) + 2*(len(s.handle)+8)
		return c, c
	},
	chunkRequest:  legacyIntakeChunkBytes,
	chunkQuestion: legacyIntakeChunkBytes,
	fits:          func(c intakeCall) bool { return c.bytes() <= legacyIntakeRequestBytes },
}

// IntakeShadowStory is one candidate story of the window.
type IntakeShadowStory struct {
	DocID     string    `json:"doc_id"`
	Headline  string    `json:"headline"`
	Lede      string    `json:"lede,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IntakeShadowCapture is one capture to label.
type IntakeShadowCapture struct {
	CaptureID string `json:"capture_id"`
	Title     string `json:"title"`
	Summary   string `json:"summary,omitempty"`
	Outlet    string `json:"outlet,omitempty"`
	Language  string `json:"language,omitempty"`
}

// IntakeShadowInput is one flush: the captures held and the story window.
type IntakeShadowInput struct {
	Stories  []IntakeShadowStory   `json:"stories"`
	Captures []IntakeShadowCapture `json:"captures"`
}

// IntakeShadowArm is one policy's plan, and its bill when it was run.
type IntakeShadowArm struct {
	Name      string `json:"name"`
	Calls     int    `json:"calls"`
	Questions int    `json:"questions"`
	// Bytes is the sum of the calls' state and questions.
	Bytes int `json:"bytes"`
	// EstTokens is the estimate (jevbudget) over the calls; InputTokens is
	// what Jev billed, set only by a run.
	EstTokens   int     `json:"est_input_tokens"`
	InputTokens int     `json:"input_tokens,omitempty"`
	CostUSD     float64 `json:"cost_usd,omitempty"`
	// EstCostUSD is EstTokens at the published price.
	EstCostUSD float64 `json:"est_cost_usd"`
	// MostQuestions is the largest call's question count.
	MostQuestions int `json:"most_questions"`
}

// IntakeShadowAgreement is the label agreement of a run, per question kind.
type IntakeShadowAgreement struct {
	Captures int `json:"captures"`
	// Event counts captures whose event answer (a story, a sibling or new)
	// is the same; Topic those whose topic story is the same.
	EventAgree int     `json:"event_agree"`
	TopicAgree int     `json:"topic_agree"`
	Event      float64 `json:"event_agreement"`
	Topic      float64 `json:"topic_agreement"`
	// MeanAbsEventP is the mean absolute difference of the event probability
	// over captures whose event answer agrees.
	MeanAbsEventP float64 `json:"mean_abs_event_p_diff"`
	// Disagreements names the captures whose event or topic answer differs.
	Disagreements []string `json:"disagreements,omitempty"`
}

// IntakeShadowReport compares the old arm with the new.
type IntakeShadowReport struct {
	Old       IntakeShadowArm        `json:"old"`
	New       IntakeShadowArm        `json:"new"`
	Run       bool                   `json:"run"`
	Agreement *IntakeShadowAgreement `json:"agreement,omitempty"`
}

func (in IntakeShadowInput) library(rt *Runtime) *intakeLibrary {
	lib := &intakeLibrary{rt: rt, summary: map[string]store.StorySummary{}}
	docs := make([]types.Document, 0, len(in.Stories))
	for _, s := range in.Stories {
		lib.summary[s.DocID] = store.StorySummary{DocID: s.DocID, Headline: s.Headline, Lede: s.Lede, UpdatedAt: s.UpdatedAt}
		docs = append(docs, types.Document{DocID: s.DocID, Title: s.Headline, UpdatedAt: s.UpdatedAt})
	}
	lib.recent, lib.rest = intakeShortlistSplit(docs, time.Now().UTC())
	return lib
}

func (in IntakeShadowInput) items() []sourcegraph.CapturedItem {
	out := make([]sourcegraph.CapturedItem, len(in.Captures))
	for i, c := range in.Captures {
		out[i] = sourcegraph.CapturedItem{CaptureID: c.CaptureID, Title: c.Title, Summary: c.Summary, Outlet: c.Outlet, Language: c.Language}
	}
	return out
}

// intakeShadowArms are the two policies and the captures a call may carry
// under each.
var intakeShadowArms = []struct {
	name   string
	policy intakePolicy
	batch  int
}{
	{"old: 15,000-byte slices, 30,000-byte guard, 6 captures a call", intakeLegacyBytePolicy, legacyIntakeCallCaptures},
	{"new: one state, vendor token limits", intakeTokenPolicy, intakeBatchCaptures},
}

func shadowPlanArm(rt *Runtime, in IntakeShadowInput, name string, policy intakePolicy, batch int) (IntakeShadowArm, error) {
	lib := in.library(rt)
	items := in.items()
	p, err := newIntakePoll(context.Background(), lib, items, policy)
	if err != nil {
		return IntakeShadowArm{}, err
	}
	var live []int
	for i := range items {
		if len(p.candidatesOf(i)) > 0 || len(p.views[i].siblings) > 0 {
			live = append(live, i)
		}
	}
	calls, err := rt.planIntakeCalls(lib, p, batch, live, policy)
	if err != nil {
		return IntakeShadowArm{}, err
	}
	arm := IntakeShadowArm{Name: name, Calls: len(calls)}
	for _, c := range calls {
		arm.Questions += len(c.questions)
		arm.Bytes += c.bytes()
		arm.EstTokens += jevbudget.Measure(c.state, c.questions).Request
		arm.MostQuestions = max(arm.MostQuestions, len(c.questions))
	}
	arm.EstCostUSD = float64(arm.EstTokens) * intakeShadowPricePerMTokens / 1e6
	return arm, nil
}

// PlanIntakeShadow plans both arms for one flush and makes no call. Its
// estimated tokens at the published price are what a run would cost to within
// the estimate's error (the estimate is weighted above the tokenizer, so it
// overstates).
func PlanIntakeShadow(in IntakeShadowInput) (IntakeShadowReport, error) {
	rt := &Runtime{}
	var rep IntakeShadowReport
	for n, arm := range intakeShadowArms {
		got, err := shadowPlanArm(rt, in, arm.name, arm.policy, arm.batch)
		if err != nil {
			return rep, err
		}
		if n == 0 {
			rep.Old = got
		} else {
			rep.New = got
		}
	}
	return rep, nil
}

// usageJudge counts the input tokens and cost Jev reports for the calls made
// through it. Calls through it are sequential.
type usageJudge struct {
	inner CandidateJudge
	mu    sync.Mutex
	input int
	cost  float64
}

func (j *usageJudge) Judge(ctx context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	return j.JudgeScoped(ctx, "", state, questions)
}

func (j *usageJudge) JudgeScoped(ctx context.Context, scope string, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	var raw json.RawMessage
	var err error
	if scoped, ok := j.inner.(ScopedCandidateJudge); ok && scope != "" {
		raw, err = scoped.JudgeScoped(ctx, scope, state, questions)
	} else {
		raw, err = j.inner.Judge(ctx, state, questions)
	}
	if err == nil {
		var usage struct {
			Usage struct {
				InputTokens int     `json:"input_tokens"`
				Cost        float64 `json:"cost"`
			} `json:"usage"`
		}
		if json.Unmarshal(raw, &usage) == nil {
			j.mu.Lock()
			j.input += usage.Usage.InputTokens
			j.cost += usage.Usage.Cost
			j.mu.Unlock()
		}
	}
	return raw, err
}

// RunIntakeShadow plans both arms and asks Jev through judge for each: the
// labels are the runtime's own labelPollWith, nothing is written. It is a paid
// run (two arms' worth of calls); the caller decides where it runs.
func RunIntakeShadow(ctx context.Context, judge CandidateJudge, in IntakeShadowInput) (IntakeShadowReport, error) {
	rep, err := PlanIntakeShadow(in)
	if err != nil {
		return rep, err
	}
	rep.Run = true
	var labels [2][]intakeLabelResult
	for n, arm := range intakeShadowArms {
		counter := &usageJudge{inner: judge}
		rt := &Runtime{}
		rt.SetCandidateJudge(counter)
		labels[n] = rt.labelPollWith(ctx, in.library(rt), "shadow", "shadow", "shadow", in.items(), arm.batch, arm.policy)
		target := &rep.Old
		if n == 1 {
			target = &rep.New
		}
		target.InputTokens, target.CostUSD = counter.input, counter.cost
	}
	rep.Agreement = intakeShadowAgreement(in, labels[0], labels[1])
	return rep, nil
}

func intakeEventKey(l store.CaptureLabel) string {
	switch l.Event {
	case store.CaptureLabelEventStory:
		return "story:" + l.EventDocID
	case store.CaptureLabelEventSibling:
		return "sibling:" + l.EventCaptureID
	}
	return l.Event
}

func intakeShadowAgreement(in IntakeShadowInput, old, cur []intakeLabelResult) *IntakeShadowAgreement {
	a := &IntakeShadowAgreement{}
	var diff float64
	for i, c := range in.Captures {
		if old[i].Err != nil || cur[i].Err != nil {
			a.Disagreements = append(a.Disagreements, fmt.Sprintf("%s: unlabelled (old: %v, new: %v)", c.CaptureID, old[i].Err, cur[i].Err))
			continue
		}
		a.Captures++
		o, n := old[i].Value, cur[i].Value
		var why []string
		if intakeEventKey(o) == intakeEventKey(n) {
			a.EventAgree++
			d := o.EventP - n.EventP
			if d < 0 {
				d = -d
			}
			diff += d
		} else {
			why = append(why, fmt.Sprintf("event %s -> %s", intakeEventKey(o), intakeEventKey(n)))
		}
		if o.TopicDocID == n.TopicDocID {
			a.TopicAgree++
		} else {
			why = append(why, fmt.Sprintf("topic %q -> %q", o.TopicDocID, n.TopicDocID))
		}
		if len(why) > 0 {
			a.Disagreements = append(a.Disagreements, c.CaptureID+": "+strings.Join(why, "; "))
		}
	}
	if a.Captures > 0 {
		a.Event = float64(a.EventAgree) / float64(a.Captures)
		a.Topic = float64(a.TopicAgree) / float64(a.Captures)
	}
	if a.EventAgree > 0 {
		a.MeanAbsEventP = diff / float64(a.EventAgree)
	}
	sort.Strings(a.Disagreements)
	return a
}
