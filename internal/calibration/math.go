package calibration

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// OutcomeValue maps an outcome to y in [0,1]. resolved is false for
// unresolvable (no Brier score); ok is false for an unknown outcome.
func OutcomeValue(outcome string) (y float64, resolved, ok bool) {
	switch outcome {
	case types.CalibrationOutcomeTrue:
		return 1, true, true
	case types.CalibrationOutcomeFalse:
		return 0, true, true
	case types.CalibrationOutcomePartial:
		return 0.5, true, true
	case types.CalibrationOutcomeUnresolvable:
		return 0, false, true
	}
	return 0, false, false
}

// BrierScore is (p - y)^2.
func BrierScore(p, y float64) float64 { return (p - y) * (p - y) }

func methodRank(method string) int {
	switch method {
	case types.CalibrationScorerRule:
		return 0
	case types.CalibrationScorerOwner:
		return 1
	case types.CalibrationScorerIndependent:
		return 2
	case types.CalibrationScorerSelf:
		return 3
	}
	return 4
}

// Governing picks the settlement that decides a claim's outcome. With
// method "" (governing) it takes the highest-ranked method, rule > owner >
// independent > self; with a method it takes that method's settlement.
// Ties go to the earliest settlement.
func Governing(settlements []SettlementView, method string) (SettlementView, bool) {
	var best SettlementView
	found := false
	for _, s := range settlements {
		if method != "" && s.Method != method {
			continue
		}
		if methodRank(s.Method) > 3 {
			continue
		}
		if !found || methodRank(s.Method) < methodRank(best.Method) ||
			(methodRank(s.Method) == methodRank(best.Method) && (s.SettledAt < best.SettledAt || (s.SettledAt == best.SettledAt && s.ID < best.ID))) {
			best, found = s, true
		}
	}
	return best, found
}

// SummaryOptions selects the scorer (""/"governing" or one method), the
// grouping keys (agent, model, type, project; default agent, model, type)
// and whether reconstructed claims count.
type SummaryOptions struct {
	Scorer               string
	GroupBy              []string
	IncludeReconstructed bool
	// Filters label the summary (the caller has already applied them).
	Filters Filters
}

// Bin is one calibration bin of width 0.1 by stated p: how many resolved
// claims fell in it, their mean stated p and the observed mean outcome.
type Bin struct {
	Lo       float64 `json:"lo"`
	Hi       float64 `json:"hi"`
	N        int     `json:"n"`
	MeanP    float64 `json:"mean_p"`
	Observed float64 `json:"observed"`
}

// SelfGap compares self settlements with the governing non-self settlement
// on claims that have both: self-favourable means the claimant scored its
// own claim as more true than the independent scorer did.
type SelfGap struct {
	N               int `json:"n"`
	Agree           int `json:"agree"`
	SelfFavorable   int `json:"self_favorable"`
	SelfUnfavorable int `json:"self_unfavorable"`
}

// GroupStats is the calibration of one group. Scores are nil when no claim
// is resolved, never NaN.
type GroupStats struct {
	Agent          string   `json:"agent,omitempty"`
	Model          string   `json:"model,omitempty"`
	Type           string   `json:"type,omitempty"`
	Project        string   `json:"project,omitempty"`
	Claims         int      `json:"claims"`
	Open           int      `json:"open"`
	Overdue        int      `json:"overdue"`
	Settled        int      `json:"settled"`
	Resolved       int      `json:"resolved"`
	Unresolvable   int      `json:"unresolvable"`
	Brier          *float64 `json:"brier"`
	BrierStrict    *float64 `json:"brier_strict"`
	BaseRate       *float64 `json:"base_rate"`
	MeanP          *float64 `json:"mean_p"`
	HitRate        *float64 `json:"hit_rate"`
	Overconfidence *float64 `json:"overconfidence"`
	Bins           []Bin    `json:"bins"`
	SelfGap        *SelfGap `json:"self_gap,omitempty"`
	// SelfOnly counts settled claims whose only settlement is the
	// claimant's own. They teach the agent (self-scoring is the default
	// learning signal) but are not oversight evidence: self never
	// governs anything that gates action (docs/precommitment-records.md §5.2).
	SelfOnly int `json:"self_only"`
}

// Miss is a settled claim with a large Brier score, for the context text.
type Miss struct {
	ID      string  `json:"id"`
	Claim   string  `json:"claim"`
	P       float64 `json:"p"`
	Outcome string  `json:"outcome"`
	Brier   float64 `json:"brier"`
}

// Summary is GET /api/precommit/calibration. Context is the plain-text
// track record an agent pastes into its context.
type Summary struct {
	Scorer                string       `json:"scorer"`
	GroupBy               []string     `json:"group_by"`
	Filters               Filters      `json:"filters"`
	Overall               GroupStats   `json:"overall"`
	Groups                []GroupStats `json:"groups"`
	Misses                []Miss       `json:"largest_misses"`
	ReconstructedExcluded int          `json:"reconstructed_excluded"`
	ReconstructedIncluded int          `json:"reconstructed_included"`
	Truncated             bool         `json:"truncated"`
	Context               string       `json:"context"`
}

// Filters names the claim filters a summary or list was computed under.
type Filters struct {
	Agent   string `json:"agent,omitempty"`
	Model   string `json:"model,omitempty"`
	Type    string `json:"type,omitempty"`
	Project string `json:"project,omitempty"`
}

// Match reports whether a claim passes the filters.
func (f Filters) Match(c ClaimView) bool {
	return (f.Agent == "" || c.Agent == f.Agent) && (f.Model == "" || c.Model == f.Model) &&
		(f.Type == "" || c.Type == f.Type) && (f.Project == "" || c.Project == f.Project)
}

// GroupKeys are the accepted group_by keys.
var GroupKeys = map[string]bool{"agent": true, "model": true, "type": true, "project": true}

// DefaultGroupBy is agent x model x claim type.
var DefaultGroupBy = []string{"agent", "model", "type"}

type scored struct {
	claim    ClaimView
	status   string
	outcome  string
	y        float64
	resolved bool
}

func scoreClaim(c ClaimView, scorer string, now time.Time) scored {
	s := scored{claim: c, status: statusOf(c, scorer, now)}
	if g, ok := Governing(c.Settlements, scorer); ok {
		s.outcome = g.Outcome
		s.y, s.resolved, _ = OutcomeValue(g.Outcome)
	}
	return s
}

func ptr(v float64) *float64 { return &v }

func stats(items []scored) GroupStats {
	var g GroupStats
	var brier, strict, ys, ps, conf, hits float64
	bins := make([]struct{ n, p, y float64 }, 10)
	for _, it := range items {
		g.Claims++
		switch it.status {
		case StatusOpen:
			g.Open++
		case StatusOverdue:
			g.Open++
			g.Overdue++
		case StatusSettled:
			g.Settled++
		}
		if it.status != StatusSettled {
			continue
		}
		p := it.claim.P
		if !it.resolved {
			g.Unresolvable++
			strict += BrierScore(p, 0)
			continue
		}
		g.Resolved++
		b := BrierScore(p, it.y)
		brier += b
		strict += b
		ys += it.y
		ps += p
		hit := it.y
		c := p
		if p < 0.5 {
			hit, c = 1-it.y, 1-p
		}
		conf += c
		hits += hit
		idx := int(p*10 + 1e-9) // 0.7*10 must land in bin 7, not 6
		if idx > 9 {
			idx = 9
		}
		bins[idx].n++
		bins[idx].p += p
		bins[idx].y += it.y
	}
	if g.Resolved > 0 {
		n := float64(g.Resolved)
		g.Brier, g.BaseRate, g.MeanP = ptr(brier/n), ptr(ys/n), ptr(ps/n)
		g.HitRate, g.Overconfidence = ptr(hits/n), ptr(conf/n-hits/n)
	}
	if g.Resolved+g.Unresolvable > 0 {
		g.BrierStrict = ptr(strict / float64(g.Resolved+g.Unresolvable))
	}
	g.Bins = []Bin{}
	for i, b := range bins {
		if b.n == 0 {
			continue
		}
		g.Bins = append(g.Bins, Bin{Lo: float64(i) / 10, Hi: float64(i+1) / 10, N: int(b.n), MeanP: b.p / b.n, Observed: b.y / b.n})
	}
	g.SelfGap = selfGap(items)
	for _, it := range items {
		if it.status != StatusSettled {
			continue
		}
		if _, ok := Governing(it.claim.Settlements, ""); ok {
			selfOnly := true
			for _, s := range it.claim.Settlements {
				if s.Method != types.CalibrationScorerSelf {
					selfOnly = false
				}
			}
			if selfOnly {
				g.SelfOnly++
			}
		}
	}
	return g
}

func selfGap(items []scored) *SelfGap {
	var gap SelfGap
	for _, it := range items {
		self, ok := Governing(it.claim.Settlements, types.CalibrationScorerSelf)
		if !ok {
			continue
		}
		var others []SettlementView
		for _, s := range it.claim.Settlements {
			if s.Method != types.CalibrationScorerSelf {
				others = append(others, s)
			}
		}
		other, ok := Governing(others, "")
		if !ok {
			continue
		}
		sy, sr, _ := OutcomeValue(self.Outcome)
		oy, or, _ := OutcomeValue(other.Outcome)
		if !sr || !or {
			continue
		}
		gap.N++
		switch {
		case sy == oy:
			gap.Agree++
		case sy > oy:
			gap.SelfFavorable++
		default:
			gap.SelfUnfavorable++
		}
	}
	if gap.N == 0 {
		return nil
	}
	return &gap
}

func groupKey(c ClaimView, by []string) (string, GroupStats) {
	var parts []string
	var g GroupStats
	for _, k := range by {
		switch k {
		case "agent":
			parts, g.Agent = append(parts, c.Agent), c.Agent
		case "model":
			parts, g.Model = append(parts, c.Model), c.Model
		case "type":
			parts, g.Type = append(parts, c.Type), c.Type
		case "project":
			parts, g.Project = append(parts, c.Project), c.Project
		}
	}
	return strings.Join(parts, "\x1f"), g
}

// Summarize computes the calibration summary of claims (already filtered
// by the caller's Filters, which only label the result here).
func Summarize(claims []ClaimView, opts SummaryOptions, now time.Time) Summary {
	scorer := opts.Scorer
	if scorer == "governing" {
		scorer = ""
	}
	by := opts.GroupBy
	if len(by) == 0 {
		by = DefaultGroupBy
	}
	sum := Summary{Scorer: "governing", GroupBy: by, Filters: opts.Filters, Groups: []GroupStats{}, Misses: []Miss{}}
	if scorer != "" {
		sum.Scorer = scorer
	}
	var all []scored
	groups := map[string][]scored{}
	labels := map[string]GroupStats{}
	var order []string
	for _, c := range claims {
		if c.Reconstructed {
			if !opts.IncludeReconstructed {
				sum.ReconstructedExcluded++
				continue
			}
			sum.ReconstructedIncluded++
		}
		it := scoreClaim(c, scorer, now)
		all = append(all, it)
		key, label := groupKey(c, by)
		if _, ok := groups[key]; !ok {
			order = append(order, key)
			labels[key] = label
		}
		groups[key] = append(groups[key], it)
	}
	sum.Overall = stats(all)
	sort.Strings(order)
	for _, key := range order {
		g := stats(groups[key])
		label := labels[key]
		g.Agent, g.Model, g.Type, g.Project = label.Agent, label.Model, label.Type, label.Project
		sum.Groups = append(sum.Groups, g)
	}
	for _, it := range all {
		if it.status == StatusSettled && it.resolved {
			if b := BrierScore(it.claim.P, it.y); b >= 0.25 {
				sum.Misses = append(sum.Misses, Miss{ID: it.claim.ID, Claim: it.claim.Claim, P: it.claim.P, Outcome: it.outcome, Brier: b})
			}
		}
	}
	sort.SliceStable(sum.Misses, func(i, j int) bool {
		if sum.Misses[i].Brier != sum.Misses[j].Brier {
			return sum.Misses[i].Brier > sum.Misses[j].Brier
		}
		return sum.Misses[i].ID < sum.Misses[j].ID
	})
	if len(sum.Misses) > 5 {
		sum.Misses = sum.Misses[:5]
	}
	sum.Context = ContextText(sum)
	return sum
}

// minReadable is the fewest resolved claims before the context text reads
// anything into the scores.
const minReadable = 5

func f3(p *float64) string {
	if p == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.3f", *p)
}

func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// ContextText renders the summary as the plain-text track record an agent
// pastes into its context before stating its next probability. It names
// only settled outcomes and counts of open claims, never a pending verdict.
func ContextText(sum Summary) string {
	var b strings.Builder
	o := sum.Overall
	who := []string{}
	for _, kv := range [][2]string{{"agent", sum.Filters.Agent}, {"model", sum.Filters.Model}, {"type", sum.Filters.Type}, {"project", sum.Filters.Project}} {
		if kv[1] != "" {
			who = append(who, kv[0]+"="+kv[1])
		}
	}
	scope := "all agents"
	if len(who) > 0 {
		scope = strings.Join(who, " ")
	}
	basis := "prospective claims"
	switch {
	case sum.ReconstructedIncluded > 0:
		basis = fmt.Sprintf("includes %d claims reconstructed after the fact", sum.ReconstructedIncluded)
	case sum.ReconstructedExcluded > 0:
		basis = fmt.Sprintf("prospective claims only; %d reconstructed excluded", sum.ReconstructedExcluded)
	}
	fmt.Fprintf(&b, "Calibration track record (%s; scorer: %s; %s).\n", scope, sum.Scorer, basis)
	fmt.Fprintf(&b, "Settled %d (%d resolved, %d unresolvable); open %d, of which %d past deadline.\n",
		o.Settled, o.Resolved, o.Unresolvable, o.Open, o.Overdue)
	if o.Resolved == 0 {
		b.WriteString("No resolved claims yet: nothing to read into calibration.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "Brier %s (0 is perfect; always saying 0.5 scores 0.250); strict Brier %s (unresolvable counted as false).\n", f3(o.Brier), f3(o.BrierStrict))
	fmt.Fprintf(&b, "Overconfidence %+.2f: mean confidence in the stated direction %.2f, hit rate %.2f.\n",
		*o.Overconfidence, *o.HitRate+*o.Overconfidence, *o.HitRate)
	if len(o.Bins) > 0 {
		b.WriteString("By stated p:")
		for i := len(o.Bins) - 1; i >= 0; i-- {
			bin := o.Bins[i]
			fmt.Fprintf(&b, " %.1f-%.1f n=%d held %.0f%%;", bin.Lo, bin.Hi, bin.N, bin.Observed*100)
		}
		b.WriteString("\n")
	}
	var scoredGroups []string
	for _, g := range sum.Groups {
		if g.Resolved == 0 {
			continue
		}
		var parts []string
		for _, v := range []string{g.Agent, g.Model, g.Type, g.Project} {
			if v != "" {
				parts = append(parts, v)
			}
		}
		scoredGroups = append(scoredGroups, fmt.Sprintf(" %s n=%d Brier %s overconfidence %+.2f;", strings.Join(parts, " "), g.Resolved, f3(g.Brier), *g.Overconfidence))
	}
	if len(scoredGroups) > 1 {
		b.WriteString("By group:" + strings.Join(scoredGroups, "") + "\n")
	}
	if o.SelfOnly > 0 {
		fmt.Fprintf(&b, "%d of the %d settled claims are settled only by yourself: a learning signal, not independent evidence.\n", o.SelfOnly, o.Settled)
	}
	if o.SelfGap != nil {
		fmt.Fprintf(&b, "Self-scoring: of %d claims you and an independent scorer both settled, you agreed on %d and scored yourself more favourably on %d.\n",
			o.SelfGap.N, o.SelfGap.Agree, o.SelfGap.SelfFavorable)
	}
	if len(sum.Misses) > 0 {
		b.WriteString("Largest misses:")
		for i, m := range sum.Misses {
			if i == 3 {
				break
			}
			fmt.Fprintf(&b, " \"%s\" (p %.2f, %s);", strings.ReplaceAll(clip(m.Claim, 90), `"`, "'"), m.P, m.Outcome)
		}
		b.WriteString("\n")
	}
	switch {
	case o.Resolved < minReadable:
		fmt.Fprintf(&b, "Only %d resolved: too few to read calibration; keep recording.\n", o.Resolved)
	case *o.Overconfidence > 0.1:
		b.WriteString("Reading: overconfident. State lower probabilities, or check under production conditions before claiming.\n")
	case *o.Overconfidence < -0.1:
		b.WriteString("Reading: underconfident. Your claims hold more often than you say; state higher probabilities where you have checked.\n")
	default:
		b.WriteString("Reading: within 0.1 of calibrated in the stated direction.\n")
	}
	return b.String()
}
