// Package promptroute is the prompt router (modular architecture plan MA4.8,
// section 9.2): the conductor as a System 1 router. It is kernel-adjacent and
// has no custody: it owns no object kind, reads only the owner's submitted
// text (never a focus, R31), and writes only its route records.
//
// This package is the router's pure part: the shape of a package desk's
// `accepts` declaration, the candidate set derived from the desk registry,
// the typed choice question, and the decision rule over the distribution
// System 1 returns. The runtime (internal/agentcore/prompt_router.go) asks the
// question through the System 1 kernel service and freezes the route record;
// the prompt-bar handler (internal/textureowner/prompt_bar.go) hands off.
//
// The router can only choose among the candidates: the default destination
// (the caller's; today Texture) and the registered package (app) desks that
// declare what they accept. A system desk, a run role, a candidate desk (MA4.3:
// no inbound routing) and an unregistered name are never candidates, so no
// answer, whatever the owner's text asked for, routes to one. Every failure
// takes the default route.
package promptroute

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Service names the router as the committer and resolver of its route
// records, and as the agent its System 1 calls are booked to.
const Service = "system:router"

// QuestionID is the id of the router's one typed question.
const QuestionID = "route"

// Question is the router's typed choice question. The owner's text and the
// destinations' descriptions are evidence the host composes; the question's
// wording is not evidence.
const Question = "Which destination does the owner's input want? Choose the destination whose description fits the input. The default destination takes anything no other destination clearly fits."

// Theta is the starting threshold: the router takes the top candidate only
// when its probability is at least Theta, and otherwise abstains to the
// default with the top candidates as the one-tap choices. It is a starting
// parameter (plan 9.2 item 7), tuned from settled routes, not a switch.
const Theta = 0.6

// MaxCandidates bounds the choice set: System 1's host question bound
// (yaegikernel.System1HostMaxChoices; a runtime test pins the two equal).
const MaxCandidates = 8

// MaxAlternatives is how many choices an abstained route offers.
const MaxAlternatives = 3

// Probability clamp (docs/precommitment-records.md 5.3: clamp before logging).
const (
	ClampLow  = 0.01
	ClampHigh = 0.99
)

// Rules: how a route was decided. A record carries exactly one.
const (
	// RuleJudge: System 1's top candidate cleared Theta.
	RuleJudge = "judge"
	// RuleAbstain: no candidate cleared Theta; the default was taken and the
	// top candidates are offered as the one-tap choice.
	RuleAbstain = "abstain"
	// RuleSoleCandidate: the default is the only candidate, so there is no
	// question to ask and no call is made.
	RuleSoleCandidate = "sole_candidate"
	// RuleContentURL: the deterministic URL classifier's rule route.
	RuleContentURL = "content_url"
	// RuleNoJudge: no decision model is bound on this computer.
	RuleNoJudge = "no_judge"
	// RuleJudgeFailed: the call failed or its answer did not decode.
	RuleJudgeFailed = "judge_failed"
	// RuleJudgeTimeout: the call did not answer within the router's bound.
	RuleJudgeTimeout = "judge_timeout"
	// RuleNoAnswer: the answer named no candidate with a probability.
	RuleNoAnswer = "no_answer"
)

// Accept is one entry of a package desk's `accepts` (plan 9.2 item 1, a
// field of contributes.desks[]): a plain-language intent the desk takes from
// the prompt bar, and the request kind the router sends it. The manifest
// writes an entry as a string (the intent) or as an object.
type Accept struct {
	Intent string `json:"intent"`
	// RequestKind is "cast" (the default) or "ask".
	RequestKind string `json:"request_kind,omitempty"`
}

// Bounds of an accepts declaration.
const (
	MaxAccepts      = 64
	MaxIntentRunes  = 200
	RequestKindCast = "cast"
	RequestKindAsk  = "ask"
)

// UnmarshalJSON reads a string or a strict object.
func (a *Accept) UnmarshalJSON(raw []byte) error {
	t := bytes.TrimSpace(raw)
	if len(t) > 0 && t[0] == '"' {
		var s string
		if err := json.Unmarshal(t, &s); err != nil {
			return err
		}
		*a = Accept{Intent: s}
		return nil
	}
	type plain Accept
	var p plain
	dec := json.NewDecoder(bytes.NewReader(t))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return fmt.Errorf("an accepts entry is a string or {intent, request_kind}: %w", err)
	}
	*a = Accept(p)
	return nil
}

// Validate checks one entry.
func (a Accept) Validate() error {
	intent := strings.TrimSpace(a.Intent)
	if n := utf8.RuneCountInString(intent); n == 0 || n > MaxIntentRunes {
		return fmt.Errorf("an intent must be 1-%d characters", MaxIntentRunes)
	}
	if intent != a.Intent {
		return errors.New("an intent has no leading or trailing space")
	}
	for _, r := range intent {
		if r < 0x20 || r == 0x7f {
			return errors.New("an intent has no control characters")
		}
	}
	switch a.RequestKind {
	case "", RequestKindCast, RequestKindAsk:
	default:
		return fmt.Errorf("request_kind %q is not cast or ask", a.RequestKind)
	}
	return nil
}

// ParseAccepts decodes and validates a desk's accepts declaration. An empty
// declaration accepts nothing from the prompt bar.
func ParseAccepts(raw json.RawMessage) ([]Accept, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var list []Accept
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("accepts must be a list of strings or {intent, request_kind} objects: %w", err)
	}
	if list == nil {
		return nil, errors.New("accepts must be a list")
	}
	if err := ValidateAccepts(list); err != nil {
		return nil, err
	}
	return list, nil
}

// ValidateAccepts checks a decoded declaration.
func ValidateAccepts(list []Accept) error {
	if len(list) > MaxAccepts {
		return fmt.Errorf("at most %d entries", MaxAccepts)
	}
	seen := map[string]bool{}
	for i, a := range list {
		if err := a.Validate(); err != nil {
			return fmt.Errorf("entry %d: %w", i, err)
		}
		if seen[a.Intent] {
			return fmt.Errorf("entry %d: intent %q is declared twice", i, a.Intent)
		}
		seen[a.Intent] = true
	}
	return nil
}

// Intents are the declaration's intents, in order.
func Intents(list []Accept) []string {
	out := make([]string, 0, len(list))
	for _, a := range list {
		out = append(out, a.Intent)
	}
	return out
}

// Candidate is one destination the router may choose.
type Candidate struct {
	// Desk is the destination's registry name; it is the choice System 1 sees.
	Desk string
	// App is the window the front end opens for it.
	App string
	// Intents describe what the desk takes; the default has none.
	Intents []string
	Default bool
}

// Candidates is the choice set: the default destination first, then every
// routable package desk in name order, at most MaxCandidates in all.
func Candidates(defaultDesk string) []Candidate {
	defaultDesk = strings.TrimSpace(defaultDesk)
	out := []Candidate{{Desk: defaultDesk, App: defaultDesk, Default: true}}
	for _, name := range deskmanifest.Names() {
		d, ok := routableDesk(name)
		if !ok || len(d.Accepts) == 0 || name == defaultDesk {
			continue
		}
		if len(out) == MaxCandidates {
			break
		}
		out = append(out, Candidate{Desk: d.Name, App: AppOf(d.Name), Intents: append([]string(nil), d.Accepts...)})
	}
	return out
}

// routableDesk is a registered, installed package desk that is not a
// candidate (a candidate has no inbound routing, plan MA4.3).
func routableDesk(name string) (deskmanifest.Desk, bool) {
	d, ok := deskmanifest.Lookup(name)
	if !ok || d.Kind != deskmanifest.KindApp {
		return deskmanifest.Desk{}, false
	}
	if objectgraph.IsCandidateName(AppOf(d.Name)) {
		return deskmanifest.Desk{}, false
	}
	return d, true
}

// Addressable reports whether a focused submit may name desk: the default
// destination or a routable package desk. A system desk, a role, a candidate
// and an unregistered name are refused.
func Addressable(defaultDesk, desk string) bool {
	desk = strings.TrimSpace(desk)
	if desk == "" {
		return false
	}
	if desk == strings.TrimSpace(defaultDesk) {
		return true
	}
	_, ok := routableDesk(desk)
	return ok
}

// AppOf is the app (package) a package desk belongs to: app:<slug> is <slug>.
// Any other name is its own app.
func AppOf(desk string) string {
	if slug, ok := strings.CutPrefix(desk, "app:"); ok {
		return slug
	}
	return desk
}

// Decision is one route.
type Decision struct {
	Destination string `json:"destination"`
	App         string `json:"app"`
	Rule        string `json:"rule"`
	// Probability is the chosen destination's clamped probability, or 0 for
	// a fallback.
	Probability float64 `json:"probability,omitempty"`
	// Distribution is the full clamped distribution over the candidates.
	Distribution map[string]float64 `json:"distribution,omitempty"`
	// Alternatives are the one-tap choices of an abstained route, best first.
	Alternatives []string `json:"alternatives,omitempty"`
	Reason       string   `json:"reason,omitempty"`
}

// Fallback is the default route, taken by rule.
func Fallback(candidates []Candidate, rule, reason string) Decision {
	d := defaultOf(candidates)
	return Decision{Destination: d.Desk, App: d.App, Rule: rule, Reason: reason}
}

func defaultOf(candidates []Candidate) Candidate {
	for _, c := range candidates {
		if c.Default {
			return c
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return Candidate{}
}

// Decide applies the decision rule to System 1's distribution: probabilities
// for names outside the candidates are dropped (an answer cannot add a
// destination), the rest are clamped, and the top candidate is taken when it
// clears theta. Otherwise the route abstains to the default and offers the
// top candidates.
func Decide(candidates []Candidate, given map[string]float64, theta float64) Decision {
	byDesk := map[string]Candidate{}
	for _, c := range candidates {
		byDesk[c.Desk] = c
	}
	dist := map[string]float64{}
	for name, p := range given {
		if _, ok := byDesk[name]; !ok || math.IsNaN(p) || math.IsInf(p, 0) {
			continue
		}
		dist[name] = Clamp(p)
	}
	if len(dist) == 0 {
		return Fallback(candidates, RuleNoAnswer, "the answer named no candidate with a probability")
	}
	ranked := make([]string, 0, len(dist))
	for name := range dist {
		ranked = append(ranked, name)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if dist[ranked[i]] != dist[ranked[j]] {
			return dist[ranked[i]] > dist[ranked[j]]
		}
		// A tie goes to the default, then to name order.
		if byDesk[ranked[i]].Default != byDesk[ranked[j]].Default {
			return byDesk[ranked[i]].Default
		}
		return ranked[i] < ranked[j]
	})
	top := byDesk[ranked[0]]
	if dist[top.Desk] >= theta {
		return Decision{Destination: top.Desk, App: top.App, Rule: RuleJudge, Probability: dist[top.Desk], Distribution: dist}
	}
	def := defaultOf(candidates)
	alts := ranked
	if len(alts) > MaxAlternatives {
		alts = alts[:MaxAlternatives]
	}
	return Decision{Destination: def.Desk, App: def.App, Rule: RuleAbstain, Probability: dist[def.Desk], Distribution: dist,
		Alternatives: append([]string(nil), alts...),
		Reason:       fmt.Sprintf("the top candidate %s at %.2f is below theta %.2f", top.Desk, dist[top.Desk], theta)}
}

// Clamp bounds a probability to [ClampLow, ClampHigh].
func Clamp(p float64) float64 {
	return math.Min(ClampHigh, math.Max(ClampLow, p))
}

// Choices are the candidates' desk names, the default first.
func Choices(candidates []Candidate) []string {
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c.Desk)
	}
	return out
}

// Destinations is the evidence text describing each candidate, composed by
// the host from the registry (never from the owner's text).
func Destinations(candidates []Candidate) string {
	var b strings.Builder
	b.WriteString("The destinations:")
	for _, c := range candidates {
		b.WriteString("\n- ")
		b.WriteString(c.Desk)
		b.WriteString(": ")
		if c.Default {
			b.WriteString("the default; writing, research and documents, and anything no other destination clearly fits")
			continue
		}
		b.WriteString(strings.Join(c.Intents, "; "))
	}
	return b.String()
}
