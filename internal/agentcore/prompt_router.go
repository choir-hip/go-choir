package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/promptroute"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The prompt router in the runtime (plan MA4.8, section 9.2): one owner
// submission is one route, and one route is one precommitment record, frozen
// with the full distribution, the model and the chosen route before the app
// opens. The router asks System 1 one typed choice question with the owner's
// text as the only evidence about the input (it sees no focus, R31) and the
// registry's descriptions of the candidates. It performs no effect itself.
//
// The deterministic default route is the router's fallback and its rollback:
// no decision model, a failed or late call, an answer that names no candidate,
// or a record that cannot be written all take the default. Nothing here is a
// switch: the router runs on every owner submission, and when the default is
// the only candidate it asks nothing and spends nothing.

// promptRouterTimeout bounds the owner's wait for a route. It covers the
// System 1 service's lock too (a research call may hold it), which a context
// alone does not. A var so a test can shorten it.
var promptRouterTimeout = 8 * time.Second

// promptRouterInputRunes bounds the owner's text the judge is shown.
const promptRouterInputRunes = 2000

// Host evidence references of the router's question.
const (
	routerHostInputRef        = system1HostRefPrefix + "router_input"
	routerHostDestinationsRef = system1HostRefPrefix + "router_destinations"
)

// Run metadata keys that carry a prompt-bar run's route.
const (
	runMetadataRouteRecordID     = "route_record_id"
	runMetadataRouteDestination  = "route_destination"
	runMetadataRouteApp          = "route_app"
	runMetadataRouteRule         = "route_rule"
	runMetadataRouteAlternatives = "route_alternatives"
	runMetadataRouteReason       = "route_reason"
	runMetadataRouteDispatch     = "route_dispatch"
)

// Dispatch outcomes of a route (run metadata route_dispatch).
const (
	RouteDispatchDelivered = "delivered"
	RouteDispatchFailed    = "failed"
)

// routerLabel is the provenance label of the router's call and records: the
// owner's own text, private to the owner (plan 9.2 item 6).
var routerLabel = types.OwnerPrivateLabel("router")

// ownerTextLabel is the label of the owner's typed text on every hand-off to
// a desk (routed or focused): the owner's own input from the prompt bar. A
// focused hand-off also carries owner_private:<kind of the focused object>,
// the label of the custodian's data it points at. The router's own call and
// route record keep owner_private:router (plan 9.2 item 6), so the router's
// records and a desk's requests are told apart (review r1 item 4).
var ownerTextLabel = types.OwnerPrivateLabel("prompt_bar")

// promptRouterAsk is the router's System 1 call; a var so a test can make it
// fail in ways a judge cannot (a panic in the service).
var promptRouterAsk = func(rt *Runtime, ctx context.Context, exec toolregistry.ExecutionContext, owner, computer string, ask system1Ask) (json.RawMessage, error) {
	return rt.System1().Ask(ctx, exec, owner, computer, ask)
}

// PromptRoute is one route as the prompt bar reads it.
type PromptRoute struct {
	promptroute.Decision
	// RecordID is the route record's id on the commitment ledger, or empty
	// when it could not be written (RecordError says why).
	RecordID    string `json:"record_id,omitempty"`
	RecordError string `json:"record_error,omitempty"`
	Model       string `json:"model,omitempty"`
	ReceiptRef  string `json:"receipt_ref,omitempty"`
	// Replayed marks a route read back from an earlier submit of the same
	// command: no question was asked and no record written.
	Replayed bool `json:"replayed,omitempty"`
	// Dispatch is the hand-off's outcome for a route to a package desk:
	// delivered, or failed (the default was opened instead and a dispatch
	// record on the ledger says so).
	Dispatch string `json:"dispatch,omitempty"`
}

// Metadata is the route as prompt-bar run metadata.
func (r PromptRoute) Metadata() map[string]any {
	m := map[string]any{
		runMetadataRouteDestination: r.Destination,
		runMetadataRouteApp:         r.App,
		runMetadataRouteRule:        r.Rule,
	}
	if r.RecordID != "" {
		m[runMetadataRouteRecordID] = r.RecordID
	}
	if len(r.Alternatives) > 0 {
		m[runMetadataRouteAlternatives] = strings.Join(r.Alternatives, ",")
	}
	if r.Reason != "" {
		m[runMetadataRouteReason] = r.Reason
	}
	if r.Dispatch != "" {
		m[runMetadataRouteDispatch] = r.Dispatch
	}
	return m
}

// PromptBarRunID is the prompt-bar run a command id names: the decision run
// completePromptBarDecisionRun records, and the run the router's call and
// record are filed under.
func (rt *Runtime) PromptBarRunID(ownerID, commandID string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join([]string{"choir:prompt-bar", ownerID, rt.cfg.ComputerID, strings.TrimSpace(commandID)}, ":"))).String()
}

// RoutePrompt routes the owner's text among the candidates the registry
// derives, defaulting to defaultDesk. It never fails: every failure is the
// default route, recorded with its cause, and a panic in the router is the
// default route too (review r1 item 2).
func (rt *Runtime) RoutePrompt(ctx context.Context, ownerID, runID, text, defaultDesk string) (route PromptRoute) {
	defer func() {
		if p := recover(); p != nil {
			log.Printf("prompt router: panic routing %s: %v; taking the default route", runID, p)
			route = PromptRoute{Decision: promptroute.Decision{Destination: defaultDesk, App: defaultDesk,
				Rule: promptroute.RuleJudgeFailed, Reason: "the router panicked"}, RecordError: "the router panicked"}
		}
	}()
	candidates := promptroute.Candidates(defaultDesk)
	if replay, ok := rt.promptRouteReplay(ctx, ownerID, runID); ok {
		return replay
	}
	if len(candidates) < 2 {
		return rt.freezeRoute(ctx, ownerID, runID, candidates, PromptRoute{Decision: promptroute.Fallback(candidates, promptroute.RuleSoleCandidate, "the default is the only candidate")})
	}
	if rt == nil || rt.candidateJudge == nil {
		return rt.freezeRoute(ctx, ownerID, runID, candidates, PromptRoute{Decision: promptroute.Fallback(candidates, promptroute.RuleNoJudge, "no decision model is bound on this computer")})
	}
	asked := rt.askRouter(ctx, ownerID, runID, text, candidates)
	return rt.freezeRoute(ctx, ownerID, runID, candidates, asked)
}

// RecordRuleRoute freezes a rule route: the deterministic URL classifier's
// choice of a display app. It is a route record like a judge route, so the
// two are scored on the same footing.
func (rt *Runtime) RecordRuleRoute(ctx context.Context, ownerID, runID, app, defaultDesk string) PromptRoute {
	if replay, ok := rt.promptRouteReplay(ctx, ownerID, runID); ok {
		return replay
	}
	candidates := []promptroute.Candidate{{Desk: defaultDesk, App: defaultDesk, Default: true}, {Desk: app, App: app}}
	return rt.freezeRoute(ctx, ownerID, runID, candidates, PromptRoute{Decision: promptroute.Decision{
		Destination: app, App: app, Rule: promptroute.RuleContentURL, Probability: promptroute.ClampHigh,
		Distribution: map[string]float64{app: promptroute.ClampHigh, defaultDesk: promptroute.ClampLow},
	}})
}

// promptRouteReplay reads the route of an earlier submit of the same command
// back from its run: a retry asks nothing and writes nothing.
func (rt *Runtime) promptRouteReplay(ctx context.Context, ownerID, runID string) (PromptRoute, bool) {
	if rt == nil || rt.store == nil || strings.TrimSpace(runID) == "" {
		return PromptRoute{}, false
	}
	rec, err := rt.getRunForComputer(ctx, ownerID, runID)
	if err != nil {
		return PromptRoute{}, false
	}
	dest := metadataStringValue(rec.Metadata, runMetadataRouteDestination)
	if dest == "" {
		return PromptRoute{}, false
	}
	r := PromptRoute{Replayed: true, RecordID: metadataStringValue(rec.Metadata, runMetadataRouteRecordID), Decision: promptroute.Decision{
		Destination: dest, App: metadataStringValue(rec.Metadata, runMetadataRouteApp), Rule: metadataStringValue(rec.Metadata, runMetadataRouteRule),
		Reason: metadataStringValue(rec.Metadata, runMetadataRouteReason),
	}, Dispatch: metadataStringValue(rec.Metadata, runMetadataRouteDispatch)}
	if alts := metadataStringValue(rec.Metadata, runMetadataRouteAlternatives); alts != "" {
		r.Alternatives = strings.Split(alts, ",")
	}
	return r, true
}

// askRouter asks System 1 the router's question within promptRouterTimeout
// and decides. A late answer is recorded on the tape by the System 1 service
// when it lands; the route has already taken the default.
func (rt *Runtime) askRouter(ctx context.Context, ownerID, runID, text string, candidates []promptroute.Candidate) PromptRoute {
	questions, err := yaegikernel.ValidateSystem1HostQuestions([]yaegikernel.System1Question{{
		ID: promptroute.QuestionID, Text: promptroute.Question, Choices: promptroute.Choices(candidates),
		ContextRefs: []string{routerHostInputRef, routerHostDestinationsRef},
	}})
	if err != nil {
		return PromptRoute{Decision: promptroute.Fallback(candidates, promptroute.RuleJudgeFailed, "the question did not validate: "+err.Error())}
	}
	type answer struct {
		raw   json.RawMessage
		model string
		err   error
		// late: the call ended at the router's deadline, whatever error
		// text the service wrapped it in (review r1 item 7).
		late bool
	}
	done := make(chan answer, 1)
	askCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), promptRouterTimeout)
	exec := toolregistry.ExecutionContext{RunID: runID, AgentID: promptroute.Service, OwnerID: ownerID, ComputerID: rt.cfg.ComputerID}
	go func() {
		defer cancel()
		// A panic in the service is an error answer, never a crashed guest.
		defer func() {
			if p := recover(); p != nil {
				log.Printf("prompt router: panic in the System 1 call of %s: %v", runID, p)
				done <- answer{err: fmt.Errorf("the System 1 call panicked")}
			}
		}()
		var model string
		raw, err := promptRouterAsk(rt, askCtx, exec, ownerID, rt.cfg.ComputerID, system1Ask{
			questions: questions, labels: types.NewLabelSet(routerLabel), model: &model,
			hostText: map[string]string{
				routerHostInputRef:        "The owner's input to the prompt bar: " + routerBound(text),
				routerHostDestinationsRef: promptroute.Destinations(candidates),
			},
		})
		done <- answer{raw: raw, model: model, err: err, late: errors.Is(askCtx.Err(), context.DeadlineExceeded)}
	}()
	timer := time.NewTimer(promptRouterTimeout)
	defer timer.Stop()
	var got answer
	select {
	case got = <-done:
	case <-timer.C:
		cancel()
		return PromptRoute{Decision: promptroute.Fallback(candidates, promptroute.RuleJudgeTimeout, fmt.Sprintf("System 1 did not answer within %s", promptRouterTimeout))}
	}
	if got.err != nil {
		rule := promptroute.RuleJudgeFailed
		if got.late {
			rule = promptroute.RuleJudgeTimeout
		}
		return PromptRoute{Decision: promptroute.Fallback(candidates, rule, system1Message(got.err))}
	}
	var result system1Result
	if err := json.Unmarshal(got.raw, &result); err != nil {
		return PromptRoute{Decision: promptroute.Fallback(candidates, promptroute.RuleJudgeFailed, "the answer did not decode")}
	}
	for _, a := range result.Answers {
		if a.ID != promptroute.QuestionID {
			continue
		}
		if a.Error != "" {
			r := PromptRoute{Decision: promptroute.Fallback(candidates, promptroute.RuleNoAnswer, a.Error)}
			r.Model, r.ReceiptRef = got.model, result.ReceiptRef
			return r
		}
		return PromptRoute{Decision: promptroute.Decide(candidates, a.Probabilities, promptroute.Theta), Model: got.model, ReceiptRef: result.ReceiptRef}
	}
	return PromptRoute{Decision: promptroute.Fallback(candidates, promptroute.RuleNoAnswer, "System 1 gave no answer to the route question"), Model: got.model, ReceiptRef: result.ReceiptRef}
}

// routerBound collapses whitespace and keeps the first runes of the owner's
// text the judge is shown.
func routerBound(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) > promptRouterInputRunes {
		text = string([]rune(text)[:promptRouterInputRunes]) + "…"
	}
	return text
}

// promptRouteRecordID is the route record of a prompt-bar run: one per
// submission, so a retry re-mints nothing.
func promptRouteRecordID(runID string) string { return "route:" + strings.TrimSpace(runID) }

// freezeRoute writes the route record before the app opens. A record that
// cannot be written takes the default route: the owner's door stays open, and
// the run's metadata says the record is missing and why.
func (rt *Runtime) freezeRoute(ctx context.Context, ownerID, runID string, candidates []promptroute.Candidate, route PromptRoute) PromptRoute {
	if rt == nil || rt.store == nil {
		route.Decision = promptroute.Fallback(candidates, route.Rule, route.Reason)
		route.RecordError = "no store"
		return route
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	dist := route.Distribution
	if len(dist) == 0 {
		// A rule route states its own certainty: the destination it took.
		dist = map[string]float64{route.Destination: promptroute.ClampHigh}
	}
	model := route.Model
	if model == "" {
		model = "rule:" + route.Rule
	}
	evidence := []string{"prompt_bar_run:" + runID}
	if route.ReceiptRef != "" {
		evidence = append(evidence, route.ReceiptRef)
	}
	subject := map[string]string{
		"destination": route.Destination, "app": route.App, "rule": route.Rule,
		"theta": strconv.FormatFloat(promptroute.Theta, 'f', 2, 64), "prompt_bar_run": runID,
	}
	if len(route.Alternatives) > 0 {
		subject["alternatives"] = strings.Join(route.Alternatives, ",")
	}
	if route.Reason != "" {
		subject["reason"] = route.Reason
	}
	hypothesis := fmt.Sprintf("the owner's input wants %s", route.Destination)
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: promptRouteRecordID(runID), Kind: types.CommitmentKindPrecommit,
		Discrepancy: types.DiscrepancyUnresolved,
		Precommit:   &types.CommitmentPrecommit{Question: promptroute.Question, Distribution: dist, Resolver: promptroute.Service},
		Provenance: types.CommitmentProvenance{
			AgentID: promptroute.Service, ModelID: model, RunID: runID, Origin: types.OriginSystem,
			CommittedAt: now, Labels: types.NewLabelSet(routerLabel).Strings(),
		},
		Prediction: types.CommitmentPrediction{
			Hypothesis: hypothesis, CommittedAt: now,
			Alternatives: promptroute.Choices(candidates),
			Questions:    []types.TypedQuestion{{Question: promptroute.Question, Answers: promptroute.Choices(candidates), Probabilities: dist}},
		},
		Action:       &types.CommitmentAction{Intent: fmt.Sprintf("route to %s (%s)", route.Destination, route.Rule)},
		EvidenceRefs: evidence,
		Addressee:    promptroute.Service,
		EmitterKind:  types.CommitmentEmitterSystem, ComputerID: rt.cfg.ComputerID, SubjectRefs: subject,
	}
	if _, err := rt.store.AppendCommitmentRecord(context.WithoutCancel(ctx), ownerID, rt.cfg.ComputerID, rec); err != nil {
		route.RecordError = err.Error()
		if route.Rule == promptroute.RuleContentURL {
			// The URL rule is deterministic and opens no desk: its app opens
			// whether or not its record could be written.
			log.Printf("prompt router: record the rule route of %s: %v", runID, err)
			return route
		}
		log.Printf("prompt router: record the route of %s: %v; taking the default route", runID, err)
		route.Decision = promptroute.Fallback(candidates, route.Rule, "the route record could not be written")
		route.RecordError = err.Error()
		return route
	}
	route.RecordID = rec.RecordID
	return route
}

// OwnerFocus is a focused submit's focus (plan 9.2, "Focus-aware request"):
// the focused window's app agent, the ref of its focused object, the object's
// kind (checked by promptroute.FocusKind: the owner's object, of a kind the
// desk is custodian of) and an optional bounded selection inside it. It goes
// only to that app agent.
type OwnerFocus struct {
	AppAgentID string
	ObjectRef  string
	Kind       string
	Selection  string
}

// DeliverOwnerRequest is the hand-off to a package desk: an owner-origin
// request record (a Cast directive) addressed to the desk, carrying the
// owner's text (labelled owner_private:prompt_bar) and, for a focused submit,
// the focus, labelled owner_private:<kind of the focused object> as well. The
// desk decides what the text is for. The record id
// is one per prompt-bar run, so a retry re-mints nothing.
func (rt *Runtime) DeliverOwnerRequest(ctx context.Context, ownerID, runID, desk, text string, focus *OwnerFocus, routeRecordID string) (string, error) {
	if rt == nil || rt.store == nil {
		return "", fmt.Errorf("owner request: no store")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if focus != nil && strings.TrimSpace(focus.Kind) == "" {
		return "", fmt.Errorf("owner request to %s: a focus needs its object's checked kind", desk)
	}
	labels := types.NewLabelSet(ownerTextLabel)
	directive := &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: text}
	subject := map[string]string{"prompt_bar_run": runID, "desk": desk}
	if focus != nil {
		labels = types.NewLabelSet(ownerTextLabel, types.OwnerPrivateLabel(focus.Kind))
		directive.TargetRef = focus.ObjectRef
		directive.Body = focus.Selection
		subject["focus_object_ref"] = focus.ObjectRef
	}
	var related []string
	if routeRecordID != "" {
		related = []string{routeRecordID}
	}
	inbox := desk + ":" + ownerID
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "owner-request:" + runID, Kind: types.CommitmentKindDirective,
		Discrepancy: types.DiscrepancyUnresolved, Directive: directive,
		Provenance: types.CommitmentProvenance{
			AgentID: ownerID, ModelID: "owner", RunID: runID, Origin: types.OriginOwner, CommittedAt: now, Labels: labels.Strings(),
		},
		Prediction:  types.CommitmentPrediction{Hypothesis: "the owner's request to " + desk, CommittedAt: now},
		EmitterKind: types.CommitmentEmitterOwner, ComputerID: rt.cfg.ComputerID, SubjectRefs: subject,
		Inbox: inbox, Addressee: inbox, RelatedIDs: related,
	}
	if _, err := rt.store.AppendCommitmentRecord(context.WithoutCancel(ctx), ownerID, rt.cfg.ComputerID, rec); err != nil {
		return "", fmt.Errorf("owner request to %s: %w", desk, err)
	}
	return rec.RecordID, nil
}

// RecordRouteDispatchFailure records that a frozen route to a package desk
// was not delivered (its hand-off record could not be written) and the
// default opened instead (review r1 item 3): a report on the ledger, linked
// to the route record, so the ledger never claims a delivered route. The run
// metadata carries the same outcome (PromptRoute.Dispatch and Reason).
func (rt *Runtime) RecordRouteDispatchFailure(ctx context.Context, ownerID, runID string, route PromptRoute, cause error) error {
	if rt == nil || rt.store == nil {
		return fmt.Errorf("route dispatch: no store")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	reason := "the hand-off could not be written"
	if cause != nil {
		reason += ": " + cause.Error()
	}
	var related []string
	if route.RecordID != "" {
		related = []string{route.RecordID}
	}
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: promptRouteRecordID(runID) + ":dispatch", Kind: types.CommitmentKindReport,
		Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{AgentID: promptroute.Service, ModelID: "rule:dispatch", RunID: runID, Origin: types.OriginSystem,
			CommittedAt: now, Labels: types.NewLabelSet(routerLabel).Strings()},
		Prediction:  types.CommitmentPrediction{Hypothesis: "the route to " + route.Destination + " was not delivered", CommittedAt: now},
		Observation: types.CommitmentObservation{Excerpt: reason, SourceRef: "prompt_bar_run:" + runID, ObservedAt: now},
		Addressee:   promptroute.Service, EmitterKind: types.CommitmentEmitterSystem, ComputerID: rt.cfg.ComputerID,
		SubjectRefs: map[string]string{"dispatch": RouteDispatchFailed, "routed_to": route.Destination, "prompt_bar_run": runID, "reason": reason},
		RelatedIDs:  related,
	}
	_, err := rt.store.AppendCommitmentRecord(context.WithoutCancel(ctx), ownerID, rt.cfg.ComputerID, rec)
	return err
}
