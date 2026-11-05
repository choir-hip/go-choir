package trust

import (
	"math"
	"sort"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// WeightMode is how a weighted increment becomes evidence (trust draft 4.1).
// Two candidates, chosen by replay on settled records, never by taste.
type WeightMode string

const (
	// ModeCapped: each increment uses min(w*m, k).
	ModeCapped WeightMode = "capped"
	// ModeTempered: each increment uses w*m / mean(w*m), and the whole
	// likelihood is raised to a power eta (a power posterior).
	ModeTempered WeightMode = "tempered"
)

// Params are the parameters of a view. They are logged with every view so a
// view is reproducible. Policy numbers (tau, the impact numbers) live in the
// locked table.
type Params struct {
	// Lambda is the decay per day (0.995, about a 140-day half-life).
	Lambda float64    `json:"lambda"`
	Mode   WeightMode `json:"mode"`
	// Eta is the tempering power in (0,1]; Cap is the cap k.
	Eta float64 `json:"eta"`
	Cap float64 `json:"cap"`
	// LCBLevel is the credible-bound level (0.05).
	LCBLevel float64 `json:"lcb_level"`
	// PeerFactor scales E5 evidence ("small"); CostFactor scales E8.
	PeerFactor float64 `json:"peer_factor"`
	CostFactor float64 `json:"cost_factor"`
	// Now is the instant the view is as of. Evidence after it is not read.
	Now time.Time `json:"now"`
}

// DefaultParams are the starting parameters of the trust draft.
func DefaultParams() Params {
	return Params{Lambda: 0.995, Mode: ModeCapped, Eta: 1, Cap: 3, LCBLevel: 0.05, PeerFactor: 0.25, CostFactor: 0.25}
}

// mult is the evidence-strength multiplier of negative evidence: 1 for E1,
// E5, E6 and E8; 5 for E2 omissions; 20 for E3 fidelity and E7 platform
// evidence; 5 to 20 for E4 by the refused grant's impact.
func (p Params) mult(e EvidenceClass, c ActionClass) float64 {
	switch e {
	case E2:
		return 5
	case E3, E7:
		return 20
	case E4:
		switch c.Impact {
		case types.ImpactI1:
			return 5
		case types.ImpactI2:
			return 8
		case types.ImpactI3:
			return 14
		case types.ImpactI4:
			return 20
		}
		return 0
	}
	return 1
}

// Row is one subject's record in one action class.
type Row struct {
	Subject Subject     `json:"subject"`
	Class   ActionClass `json:"class"`
	Alpha0  float64     `json:"alpha0"`
	Beta0   float64     `json:"beta0"`
	Alpha   float64     `json:"alpha"`
	Beta    float64     `json:"beta"`
	// N is the number of evidence items that counted.
	N        int     `json:"n"`
	Mean     float64 `json:"mean"`
	LCB      float64 `json:"lcb"`
	Tau      float64 `json:"tau"`
	AboveTau bool    `json:"above_tau"`
	// Evidence counts items by evidence class.
	Evidence map[EvidenceClass]int `json:"evidence"`
	// Breaker is set by an E3 or E7 event with no later positive evidence in
	// the class: the class is gated until new evidence (circuit breaker).
	Breaker bool `json:"breaker"`
	// LastFidelityAt is the time of the latest E3 or E7 event.
	LastFidelityAt time.Time `json:"last_fidelity_at,omitempty"`
	LastEvidenceAt time.Time `json:"last_evidence_at,omitempty"`
}

type groupKey struct {
	s Subject
	c ActionClass
}

// subjectsOf lists the subjects an item is evidence about.
func subjectsOf(it Item, in Inputs) []Subject {
	var out []Subject
	if it.Desk != "" {
		out = append(out, Subject{SubjectAgent, it.Desk})
		if it.Model != "" {
			out = append(out, Subject{SubjectAgentModel, it.Desk + "@" + it.Model}, Subject{SubjectModel, it.Model})
		}
		info := in.info(it.Desk)
		if info.Package != "" {
			out = append(out, Subject{SubjectPackage, info.Package})
		}
		if info.Publisher != "" {
			out = append(out, Subject{SubjectPublisher, info.Publisher})
		}
	}
	out = append(out, it.Direct...)
	// de-duplicate (a desk's package may also be a direct subject)
	seen := map[Subject]bool{}
	uniq := out[:0]
	for _, s := range out {
		if !seen[s] {
			seen[s] = true
			uniq = append(uniq, s)
		}
	}
	return uniq
}

// Fold turns extracted items into rows as of p.Now.
func Fold(items []Item, in Inputs, p Params) []Row {
	groups := map[groupKey][]Item{}
	var wild []Item
	for _, it := range items {
		if !p.Now.IsZero() && it.At.After(p.Now) {
			continue
		}
		if it.Action == wildcardClass {
			wild = append(wild, it)
			continue
		}
		for _, s := range subjectsOf(it, in) {
			if in.SubjectSince != nil {
				if since := in.SubjectSince(s); !since.IsZero() && it.At.Before(since) {
					continue
				}
			}
			k := groupKey{s, it.Action}
			groups[k] = append(groups[k], it)
		}
	}
	// platform evidence with no class applies to every class its subjects hold
	for _, it := range wild {
		for _, s := range subjectsOf(it, in) {
			hit := false
			for k := range groups {
				if k.s == s && k.c != wildcardClass {
					cp := it
					cp.Action = k.c
					groups[k] = append(groups[k], cp)
					hit = true
				}
			}
			if !hit {
				k := groupKey{s, wildcardClass}
				groups[k] = append(groups[k], it)
			}
		}
	}
	var rows []Row
	for k, its := range groups {
		a0, b0 := PlatformBase(k.c)
		if in.Prior != nil {
			a0, b0 = in.Prior(k.s, k.c)
		}
		rows = append(rows, foldGroup(k, its, a0, b0, p))
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Subject.Kind != b.Subject.Kind {
			return a.Subject.Kind < b.Subject.Kind
		}
		if a.Subject.ID != b.Subject.ID {
			return a.Subject.ID < b.Subject.ID
		}
		if a.Class.Grant != b.Class.Grant {
			return a.Class.Grant < b.Class.Grant
		}
		return a.Class.Impact < b.Class.Impact
	})
	return rows
}

func foldGroup(k groupKey, its []Item, a0, b0 float64, p Params) Row {
	sort.SliceStable(its, func(i, j int) bool {
		a, b := its[i], its[j]
		if !a.At.Equal(b.At) {
			return a.At.Before(b.At)
		}
		if a.RecordID != b.RecordID {
			return a.RecordID < b.RecordID
		}
		return a.Class < b.Class
	})
	row := Row{Subject: k.s, Class: k.c, Alpha0: a0, Beta0: b0, Evidence: map[EvidenceClass]int{}}
	wbar := 0.0
	for _, it := range its {
		wbar += it.W * it.M
	}
	wbar /= float64(len(its))
	eta := p.Eta
	if eta <= 0 || eta > 1 {
		eta = 1
	}
	alpha, beta := a0, b0
	var lastPositive time.Time
	for _, it := range its {
		age := 0.0
		if !p.Now.IsZero() && !it.At.IsZero() {
			age = math.Max(0, p.Now.Sub(it.At).Hours()/24)
		}
		decay := math.Pow(p.Lambda, age)
		var wa, wb float64
		switch p.Mode {
		case ModeTempered:
			if wbar > 0 {
				wa, wb = eta*it.W/wbar, eta*it.W*it.M/wbar
			}
		default:
			wa, wb = math.Min(it.W, p.Cap), math.Min(it.W*it.M, p.Cap)
		}
		alpha += decay * wa * it.QPos()
		beta += decay * wb * it.QNeg()
		row.N++
		row.Evidence[it.Class]++
		if it.At.After(row.LastEvidenceAt) {
			row.LastEvidenceAt = it.At
		}
		if it.Class == E3 || it.Class == E7 {
			if it.At.After(row.LastFidelityAt) || row.LastFidelityAt.IsZero() {
				row.LastFidelityAt = it.At
			}
		} else if !it.Failure() && (it.Class == E1 || it.Class == E5) && it.At.After(lastPositive) {
			lastPositive = it.At
		}
	}
	row.Alpha, row.Beta = alpha, beta
	row.Mean = alpha / (alpha + beta)
	level := p.LCBLevel
	if level <= 0 || level >= 1 {
		level = 0.05
	}
	row.LCB = betaQuantile(level, alpha, beta)
	if tau, ok := Tau(k.c.Impact); ok {
		row.Tau = tau
		row.AboveTau = row.LCB >= tau
	} else {
		row.AboveTau = true // I0: never gated
	}
	row.Breaker = !row.LastFidelityAt.IsZero() && !lastPositive.After(row.LastFidelityAt)
	return row
}

// betaQuantile is the q-quantile of Beta(a, b), by bisection on the
// regularized incomplete beta function.
func betaQuantile(q, a, b float64) float64 {
	lo, hi := 0.0, 1.0
	for i := 0; i < 80; i++ {
		mid := (lo + hi) / 2
		if betaInc(mid, a, b) < q {
			lo = mid
		} else {
			hi = mid
		}
	}
	return (lo + hi) / 2
}

// betaInc is the regularized incomplete beta function I_x(a, b).
func betaInc(x, a, b float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	la, _ := math.Lgamma(a + b)
	lb, _ := math.Lgamma(a)
	lc, _ := math.Lgamma(b)
	front := math.Exp(la - lb - lc + a*math.Log(x) + b*math.Log(1-x))
	if x < (a+1)/(a+b+2) {
		return front * betaCF(x, a, b) / a
	}
	return 1 - front*betaCF(1-x, b, a)/b
}

// betaCF evaluates the continued fraction of the incomplete beta function
// (modified Lentz).
func betaCF(x, a, b float64) float64 {
	const tiny = 1e-300
	qab, qap, qam := a+b, a+1, a-1
	c := 1.0
	d := 1 - qab*x/qap
	if math.Abs(d) < tiny {
		d = tiny
	}
	d = 1 / d
	h := d
	for m := 1; m <= 300; m++ {
		fm := float64(m)
		m2 := 2 * fm
		aa := fm * (b - fm) * x / ((qam + m2) * (a + m2))
		d = 1 + aa*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = 1 + aa/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1 / d
		h *= d * c
		aa = -(a + fm) * (qab + fm) * x / ((a + m2) * (qap + m2))
		d = 1 + aa*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = 1 + aa/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1 / d
		del := d * c
		h *= del
		if math.Abs(del-1) < 1e-14 {
			break
		}
	}
	return h
}
