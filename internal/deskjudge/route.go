package deskjudge

import (
	"context"
	"strings"
	"time"
)

// Route is where one engineering interim update goes (owner ruling 3,
// 2026-11-09): straight through to the Texture whose request the assignment
// serves, or to a wake of the management desk's model. Management always
// holds the Report as a record (it is addressed to management); the Route
// says whether management's model is also woken for it.
type Route string

const (
	RoutePass Route = "pass_through"
	RouteWake Route = "wake_model"
)

// Decision is the tape-shaped outcome for one update (owner ruling 8: which
// updates went straight through, which went by management, and whether any
// judgment ran, is data). ID derives from the update's own id so a replayed
// Report cannot hold two decisions (R3).
type Decision struct {
	ID       string `json:"decision_id"`
	ReportID string `json:"report_id"`
	Route    Route  `json:"route"`
	// Judged is false when no judgment ran; Why then names the cause (R1).
	Judged bool   `json:"judged"`
	Why    string `json:"why,omitempty"`
	Model  string `json:"model,omitempty"`
}

// DecisionID is the id of the decision of one update.
func DecisionID(reportID string) string { return "progress-route:" + strings.TrimSpace(reportID) }

const (
	routeQuestionID  = "route"
	routeTimeout     = 2 * time.Minute
	routeClaimRunes  = 4000
	routeTaskPrompt  = "An engineering desk sent an interim update about work in progress. Decide who should hear of it next."
	routeInstruction = "Is this interim update plain, factual progress that the owner's document can be told of directly (pass_through), or does it need the management desk's judgment first, because it reports a problem, a decision, a scope change or a risk, or is too technical to pass on as it stands (wake_model)?"
)

// Update is one interim engineering Report as the router reads it. Claim is
// delegate-authored text and untrusted; it reaches Jev as data only (R6).
type Update struct {
	ReportID string
	Claim    string
}

// DecideRoute asks Jev where the update goes. It never errors and never
// drops the update: when Jev is absent, slow, undecided or outside the two
// outcomes the update passes through to Texture and the decision says that no
// judgment ran (R1, R7). The caller runs it after the Report has committed,
// never inside the reducer's transaction (R2).
func DecideRoute(ctx context.Context, j Judge, u Update) Decision {
	d := Decision{ID: DecisionID(u.ReportID), ReportID: strings.TrimSpace(u.ReportID)}
	claim, _ := bound(strings.TrimSpace(u.Claim), routeClaimRunes)
	if claim == "" {
		d.Route, d.Why = RoutePass, "no judgment ran: the update carries no claim text"
		return d
	}
	a, err := askChoice(ctx, j, routeTimeout, routeTaskPrompt, routeQuestionID, routeInstruction,
		[]string{string(RoutePass), string(RouteWake)}, map[string]string{"update": claim})
	if err != nil {
		d.Route, d.Why = RoutePass, "no judgment ran: "+err.Error()
		return d
	}
	d.Route, d.Judged, d.Model = Route(a.Choice), true, a.Model
	return d
}
