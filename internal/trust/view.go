package trust

import "time"

// View is the derived trust view: per subject and action class, never
// stored, recomputed from the records like a box score. It carries the
// parameters and the table digest it used, so it is reproducible.
type View struct {
	Params      Params     `json:"params"`
	TableDigest string     `json:"table_digest"`
	Rows        []Row      `json:"rows"`
	Excluded    []Excluded `json:"excluded,omitempty"`
	// WorldSurprises counted against no one.
	WorldSurprises []Surprise `json:"world_surprises,omitempty"`
	// Judgment is score 2 per subject, shown beside the posterior; it moves
	// nothing.
	Judgment map[Subject]JudgmentStat `json:"judgment,omitempty"`
}

// Derive computes the view of in. It is a pure function of in and p: record
// order does not matter, and nothing is written. A zero p.Now means the time
// of the latest evidence.
func Derive(in Inputs, p Params) View {
	if p.Lambda == 0 {
		d := DefaultParams()
		d.Now = p.Now
		p = d
	}
	ex := Extract(in, p)
	if p.Now.IsZero() {
		var last time.Time
		for _, it := range ex.Items {
			if it.At.After(last) {
				last = it.At
			}
		}
		p.Now = last
	}
	return View{Params: p, TableDigest: ImpactTableDigest(), Rows: Fold(ex.Items, in, p),
		Excluded: ex.Excluded, WorldSurprises: ex.Surprises, Judgment: ex.Judgment}
}
