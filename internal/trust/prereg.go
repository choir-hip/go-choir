package trust

import (
	"sort"
	"time"
)

// The pre-registered units (MA7.9; docs/evidence/preregistration-2026-11-04.md
// section 1, P1 and P2). A unit is (s, k, t): a desk s and an action class k
// at impact I2 or above with at least one settled item before the cutoff t.
// Cutoffs are weekly; the window W is 14 days. The unit's label is a material
// failure event in [t, t+W): an E1 contradicted, an E2 omission, an E3
// fidelity event or an E7 platform event (an item that is a failure, of one
// of those classes; a class-less platform event counts in every class the
// desk holds). A unit with no act in k during W is dropped (nothing could
// fail), and so is a unit whose window has not closed by the latest
// evidence. The predictors are read from items strictly before t only: the
// trust row (lower bound and mean) and the baselines of P1 and P2 (success
// rate, the grant's impact, the count and the recency of the record). The
// units are the harness's stored input; the harness (internal/trustharness)
// scores them, so the server never computes an outcome metric for them.

// PreregWindow is W.
const PreregWindow = 14 * 24 * time.Hour

// PreregUnit is one scored unit.
type PreregUnit struct {
	Desk   string      `json:"desk"`
	Class  ActionClass `json:"class"`
	Cutoff time.Time   `json:"cutoff"`
	// PriorItems and PriorFailures count the desk's settled items in the
	// class before the cutoff.
	PriorItems    int `json:"prior_items"`
	PriorFailures int `json:"prior_failures"`
	// LCB and Mean are the desk's trust row in the class as of the cutoff
	// (HasRow false when the fold had none).
	HasRow  bool    `json:"has_row"`
	LCB     float64 `json:"lcb"`
	Mean    float64 `json:"mean"`
	Breaker bool    `json:"breaker"`
	// SuccessRate is the share of the prior items that were not failures
	// (P1 b1, P2 success-rate baseline: no calibration, no materiality).
	SuccessRate float64 `json:"success_rate"`
	// ImpactRank is the grant's impact (P2 b1, the fixed allowlist).
	ImpactRank int `json:"impact_rank"`
	// RecencyHours is the time from the latest prior item to the cutoff.
	RecencyHours float64 `json:"recency_hours"`
	// WindowActs counts the class's items in the window; Positive says one
	// of them (or a class-less platform event) was a material failure.
	WindowActs int  `json:"window_acts"`
	Positive   bool `json:"positive"`
}

// PreregUnitSet is the units and how they were drawn.
type PreregUnitSet struct {
	Window string `json:"window"`
	// Items is the number of extracted items about a desk; MaterialItems
	// those in a class at I2 or above (class-less platform events included).
	Items         int `json:"items"`
	MaterialItems int `json:"material_items"`
	// FirstEvidence and LastEvidence bound the material items; Cutoffs is
	// every weekly cutoff tried.
	FirstEvidence time.Time   `json:"first_evidence,omitempty"`
	LastEvidence  time.Time   `json:"last_evidence,omitempty"`
	Cutoffs       []time.Time `json:"cutoffs"`
	// DroppedNoAct counts candidate units with no act in the window;
	// DroppedOpen those whose window had not closed.
	DroppedNoAct int          `json:"dropped_no_act"`
	DroppedOpen  int          `json:"dropped_open"`
	Units        []PreregUnit `json:"units"`
}

// preregFailureClasses are the evidence classes of a material failure event.
var preregFailureClasses = map[EvidenceClass]bool{E1: true, E2: true, E3: true, E7: true}

// firstMonday is the first Monday 00:00 UTC strictly after t.
func firstMonday(t time.Time) time.Time {
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	for {
		d = d.AddDate(0, 0, 1)
		if d.Weekday() == time.Monday {
			return d
		}
	}
}

// PreregUnits draws the pre-registered units of in. It is a pure function of
// in and base (record order does not matter).
func PreregUnits(in Inputs, base Params) PreregUnitSet {
	if base.Lambda == 0 {
		d := DefaultParams()
		d.Now = base.Now
		base = d
	}
	ex := Extract(in, base)
	set := PreregUnitSet{Window: PreregWindow.String(), Cutoffs: []time.Time{}, Units: []PreregUnit{}}
	byDesk := map[string][]Item{}
	classes := map[string]map[ActionClass]bool{}
	for _, it := range ex.Items {
		if it.Desk == "" {
			continue
		}
		set.Items++
		byDesk[it.Desk] = append(byDesk[it.Desk], it)
		if it.Action == wildcardClass {
			set.MaterialItems++
		} else if r, ok := it.Action.Impact.Rank(); ok && r >= 2 {
			set.MaterialItems++
			if classes[it.Desk] == nil {
				classes[it.Desk] = map[ActionClass]bool{}
			}
			classes[it.Desk][it.Action] = true
		} else {
			continue
		}
		if set.FirstEvidence.IsZero() || it.At.Before(set.FirstEvidence) {
			set.FirstEvidence = it.At
		}
		if it.At.After(set.LastEvidence) {
			set.LastEvidence = it.At
		}
	}
	if set.MaterialItems == 0 {
		return set
	}
	horizon := set.LastEvidence
	if !base.Now.IsZero() {
		horizon = base.Now
	}
	for t := firstMonday(set.FirstEvidence); !t.After(horizon); t = t.AddDate(0, 0, 7) {
		set.Cutoffs = append(set.Cutoffs, t)
	}
	desks := make([]string, 0, len(classes))
	for d := range classes {
		desks = append(desks, d)
	}
	sort.Strings(desks)
	for _, t := range set.Cutoffs {
		end := t.Add(PreregWindow)
		for _, desk := range desks {
			var prior []Item
			for _, it := range byDesk[desk] {
				if it.At.Before(t) {
					prior = append(prior, it)
				}
			}
			if len(prior) == 0 {
				continue
			}
			p := base
			p.Now = t
			rows := map[ActionClass]Row{}
			for _, r := range Fold(prior, in, p) {
				if r.Subject.Kind == SubjectAgent && r.Subject.ID == desk {
					rows[r.Class] = r
				}
			}
			ks := make([]ActionClass, 0, len(classes[desk]))
			for k := range classes[desk] {
				ks = append(ks, k)
			}
			sort.Slice(ks, func(i, j int) bool {
				if ks[i].Grant != ks[j].Grant {
					return ks[i].Grant < ks[j].Grant
				}
				return ks[i].Impact < ks[j].Impact
			})
			for _, k := range ks {
				u := PreregUnit{Desk: desk, Class: k, Cutoff: t}
				var latest time.Time
				for _, it := range prior {
					if it.Action != k && it.Action != wildcardClass {
						continue
					}
					u.PriorItems++
					if it.Failure() {
						u.PriorFailures++
					}
					if it.At.After(latest) {
						latest = it.At
					}
				}
				if u.PriorItems == 0 {
					continue
				}
				if end.After(horizon) {
					set.DroppedOpen++
					continue
				}
				for _, it := range byDesk[desk] {
					if it.At.Before(t) || !it.At.Before(end) {
						continue
					}
					if it.Action == k {
						u.WindowActs++
					}
					if (it.Action == k || it.Action == wildcardClass) && preregFailureClasses[it.Class] && it.Failure() {
						u.Positive = true
					}
				}
				if u.WindowActs == 0 {
					set.DroppedNoAct++
					continue
				}
				if r, ok := rows[k]; ok {
					u.HasRow, u.LCB, u.Mean, u.Breaker = true, r.LCB, r.Mean, r.Breaker
				}
				u.SuccessRate = float64(u.PriorItems-u.PriorFailures) / float64(u.PriorItems)
				u.ImpactRank, _ = k.Impact.Rank()
				u.RecencyHours = t.Sub(latest).Hours()
				set.Units = append(set.Units, u)
			}
		}
	}
	return set
}
