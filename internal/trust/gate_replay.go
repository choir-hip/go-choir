package trust

import (
	"sort"
	"time"
)

// The act gate's replay (plan MA7.8 proof, trust draft 11 TEST, pre-registered
// P2): walk the settled evidence in time and, at each material act (an item
// of class I2 and up about a desk), ask the gate's rule of the desk's record
// from strictly earlier items, autonomous(s, k) iff LCB_05 >= tau(impact(k))
// with no open breaker. The gated acts are the owner taps the gate would have
// asked for; their rate per day is the tap rate. It reads the agent row only
// (the desk), as the runtime gate does when no model is known.

// GateReport is the replay's count.
type GateReport struct {
	// Acts is the number of material acts walked; Gated how many the rule
	// would have sent to the gate, Breaker how many of those by an open
	// breaker (reported apart, pre-registration P2 known edges).
	Acts    int `json:"acts"`
	Gated   int `json:"gated"`
	Breaker int `json:"breaker"`
	// Days is the span of the walked acts, at least one day.
	Days       float64 `json:"days"`
	TapsPerDay float64 `json:"taps_per_day"`
	// ByImpact counts the gated acts per impact class.
	ByImpact map[string]int `json:"by_impact,omitempty"`
}

// GateReplay replays the gate rule over in.
func GateReplay(in Inputs, base Params) GateReport {
	if base.Lambda == 0 {
		d := DefaultParams()
		d.Now = base.Now
		base = d
	}
	ex := Extract(in, base)
	type gk struct {
		desk string
		c    ActionClass
	}
	by := map[gk][]Item{}
	for _, it := range ex.Items {
		if it.Desk == "" || it.Action == wildcardClass {
			continue
		}
		by[gk{it.Desk, it.Action}] = append(by[gk{it.Desk, it.Action}], it)
	}
	rep := GateReport{ByImpact: map[string]int{}}
	var first, last time.Time
	for k, seq := range by {
		r, ok := k.c.Impact.Rank()
		if !ok || r < 2 {
			continue
		}
		tau, hasTau := Tau(k.c.Impact)
		sort.SliceStable(seq, func(i, j int) bool { return seq[i].At.Before(seq[j].At) })
		for j, act := range seq {
			rep.Acts++
			if first.IsZero() || act.At.Before(first) {
				first = act.At
			}
			if act.At.After(last) {
				last = act.At
			}
			var prior []Item
			for _, e := range seq[:j] {
				if e.At.Before(act.At) {
					prior = append(prior, e)
				}
			}
			autonomous, breaker := false, false
			if len(prior) > 0 && hasTau {
				p := base
				p.Now = act.At
				for _, row := range Fold(prior, in, p) {
					if row.Subject.Kind == SubjectAgent && row.Subject.ID == k.desk && row.Class == k.c {
						breaker = row.Breaker
						autonomous = !row.Breaker && row.LCB >= tau
					}
				}
			}
			if !autonomous {
				rep.Gated++
				rep.ByImpact[string(k.c.Impact)]++
				if breaker {
					rep.Breaker++
				}
			}
		}
	}
	rep.Days = 1
	if span := last.Sub(first).Hours() / 24; span > 1 {
		rep.Days = span
	}
	rep.TapsPerDay = float64(rep.Gated) / rep.Days
	if len(rep.ByImpact) == 0 {
		rep.ByImpact = nil
	}
	return rep
}
