package trust

import (
	"math"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
)

// The hierarchical prior and the change discount (trust draft 4.2, MA7.3).
//
//	platform base (per class)
//	  -> publisher   prior = platform posterior, shrunk to n0 pseudo-counts
//	    -> package   prior = publisher posterior, shrunk
//	      -> agent version   prior = package posterior, discounted by change
//
// A new subject inherits, so a new version is not a blank slate and a new
// publisher is not trusted by default: a publisher created yesterday lends the
// platform posterior, which for I3 and I4 starts deliberately low. The change
// discount d is computed from the behaviour-surface parts of the two versions
// (apppackage.SurfaceParts), never declared.

// ComponentDiscount is d for a change to one surface part, from 0.1 (assets,
// copy) to 0.9 (prompt rewrite, new code path, grant change). A part that is
// added or removed counts as changed.
var ComponentDiscount = map[string]float64{
	apppackage.PartAssets:    0.1,
	apppackage.PartApp:       0.3,
	apppackage.PartSchedules: 0.5,
	apppackage.PartAccepts:   0.5,
	apppackage.PartKinds:     0.7,
	apppackage.PartPrompt:    0.9,
	apppackage.PartCode:      0.9,
	apppackage.PartGrants:    0.9,
}

// MaxDiscount caps d: many small changes never keep less than 10% of a record.
const MaxDiscount = 0.9

// MinPriorCount is the least number of pseudo-counts a carried prior keeps, so
// a carried prior is always a valid Beta.
const MinPriorCount = 1.0

// ChangeDiscount is d for the change from prev to cur: the noisy-or of the
// discounts of the parts whose hashes differ, capped at MaxDiscount. Identical
// parts give 0 (a republish keeps the whole record). An unknown part name
// counts at the highest discount, so a new kind of change is never cheap.
func ChangeDiscount(prev, cur map[string]string) float64 {
	keep := 1.0
	seen := map[string]bool{}
	for k, h := range cur {
		seen[k] = true
		if prev[k] != h {
			keep *= 1 - componentDiscount(k)
		}
	}
	for k := range prev {
		if !seen[k] {
			keep *= 1 - componentDiscount(k)
		}
	}
	return math.Min(1-keep, MaxDiscount)
}

func componentDiscount(part string) float64 {
	if d, ok := ComponentDiscount[part]; ok {
		return d
	}
	return MaxDiscount
}

// CarryPrior keeps a posterior's mean and scales its pseudo-counts by (1 - d),
// never below MinPriorCount.
func CarryPrior(alpha, beta, d float64) (float64, float64) {
	n := math.Max((alpha+beta)*(1-d), MinPriorCount)
	mean := alpha / (alpha + beta)
	return mean * n, (1 - mean) * n
}

func shrink(alpha, beta, n0 float64) (float64, float64) {
	mean := alpha / (alpha + beta)
	return mean * n0, (1 - mean) * n0
}

// Lineage places a desk's current version: when it was installed, and the
// surface parts of the version before it (nil for a first version) and of
// this one.
type Lineage struct {
	Since    time.Time
	Previous map[string]string
	Current  map[string]string
}

// Hierarchy configures the hierarchical prior.
type Hierarchy struct {
	// N0Publisher, N0Package and N0Version are the pseudo-counts a level's
	// prior is shrunk to.
	N0Publisher, N0Package, N0Version float64
	// Lineage reports a desk's current version, when known.
	Lineage func(desk string) (Lineage, bool)
}

// DefaultHierarchy is the starting hierarchy: shrunk to 4, 6 and 8.
func DefaultHierarchy() Hierarchy {
	return Hierarchy{N0Publisher: 4, N0Package: 6, N0Version: 8}
}

// DeriveHierarchical derives the view with the hierarchical prior: each
// subject's prior comes from the level above it, computed from the other
// subjects' evidence (never its own, so nothing counts twice), and a new
// version's own record starts at its install.
func DeriveHierarchical(in Inputs, p Params, h Hierarchy) View {
	if p.Lambda == 0 {
		d := DefaultParams()
		d.Now = p.Now
		p = d
	}
	if h.N0Publisher <= 0 || h.N0Package <= 0 || h.N0Version <= 0 {
		d := DefaultHierarchy()
		d.Lineage = h.Lineage
		h = d
	}
	ex := Extract(in, p)
	if p.Now.IsZero() {
		for _, it := range ex.Items {
			if it.At.After(p.Now) {
				p.Now = it.At
			}
		}
	}
	hh := &hier{in: in, p: p, h: h, cache: map[priorKey][2]float64{}}
	for _, it := range ex.Items {
		if !it.At.After(p.Now) {
			hh.items = append(hh.items, it)
		}
	}
	in.Prior = hh.prior
	in.SubjectSince = hh.since
	return View{Params: p, TableDigest: ImpactTableDigest(), Rows: Fold(ex.Items, in, p),
		Excluded: ex.Excluded, WorldSurprises: ex.Surprises, Judgment: ex.Judgment}
}

type priorKey struct {
	s Subject
	c ActionClass
}

type hier struct {
	in    Inputs
	p     Params
	h     Hierarchy
	items []Item
	cache map[priorKey][2]float64
}

func deskOfSubject(s Subject) string {
	d, _, _ := strings.Cut(s.ID, "@")
	return d
}

func (hh *hier) since(s Subject) time.Time {
	if hh.h.Lineage == nil || (s.Kind != SubjectAgent && s.Kind != SubjectAgentModel) {
		return time.Time{}
	}
	if l, ok := hh.h.Lineage(deskOfSubject(s)); ok && l.Previous != nil {
		return l.Since
	}
	return time.Time{}
}

func (hh *hier) itemPackage(it Item) string {
	if p := hh.in.info(it.Desk).Package; p != "" {
		return p
	}
	for _, s := range it.Direct {
		if s.Kind == SubjectPackage {
			return s.ID
		}
	}
	return ""
}

func (hh *hier) itemPublisher(it Item) string {
	if p := hh.in.info(it.Desk).Publisher; p != "" {
		return p
	}
	for _, s := range it.Direct {
		if s.Kind == SubjectPublisher {
			return s.ID
		}
	}
	return ""
}

// post is the posterior of the items in a class from a starting prior.
func (hh *hier) post(c ActionClass, a0, b0 float64, keep func(Item) bool) (float64, float64) {
	var its []Item
	for _, it := range hh.items {
		if it.Action == c && keep(it) {
			its = append(its, it)
		}
	}
	if len(its) == 0 {
		return a0, b0
	}
	r := foldGroup(groupKey{c: c}, its, a0, b0, hh.p)
	return r.Alpha, r.Beta
}

func (hh *hier) publisherPrior(pub string, c ActionClass) (float64, float64) {
	a0, b0 := PlatformBase(c)
	a, b := hh.post(c, a0, b0, func(it Item) bool { return pub == "" || hh.itemPublisher(it) != pub })
	return shrink(a, b, hh.h.N0Publisher)
}

func (hh *hier) packagePrior(pkg, pub string, c ActionClass) (float64, float64) {
	a0, b0 := hh.publisherPrior(pub, c)
	if pub == "" {
		return shrink(a0, b0, hh.h.N0Package)
	}
	a, b := hh.post(c, a0, b0, func(it Item) bool { return hh.itemPublisher(it) == pub && hh.itemPackage(it) != pkg })
	return shrink(a, b, hh.h.N0Package)
}

func (hh *hier) versionPrior(desk string, c ActionClass) (float64, float64) {
	info := hh.in.info(desk)
	if info.Package == "" && info.Publisher == "" {
		return PlatformBase(c)
	}
	pa, pb := hh.packagePrior(info.Package, info.Publisher, c)
	if info.Package == "" {
		return shrink(pa, pb, hh.h.N0Version)
	}
	if hh.h.Lineage != nil {
		if l, ok := hh.h.Lineage(desk); ok && l.Previous != nil && !l.Since.IsZero() {
			a, b := hh.post(c, pa, pb, func(it Item) bool {
				return hh.itemPackage(it) == info.Package && it.At.Before(l.Since)
			})
			return CarryPrior(a, b, ChangeDiscount(l.Previous, l.Current))
		}
	}
	a, b := hh.post(c, pa, pb, func(it Item) bool { return hh.itemPackage(it) == info.Package && it.Desk != desk })
	return shrink(a, b, hh.h.N0Version)
}

func (hh *hier) prior(s Subject, c ActionClass) (float64, float64) {
	k := priorKey{s, c}
	if v, ok := hh.cache[k]; ok {
		return v[0], v[1]
	}
	var a, b float64
	switch s.Kind {
	case SubjectPublisher:
		a, b = hh.publisherPrior(s.ID, c)
	case SubjectPackage:
		a, b = hh.packagePrior(s.ID, hh.publisherOfPackage(s.ID), c)
	case SubjectAgent, SubjectAgentModel:
		a, b = hh.versionPrior(deskOfSubject(s), c)
	default:
		a, b = PlatformBase(c)
	}
	hh.cache[k] = [2]float64{a, b}
	return a, b
}

// publisherOfPackage reads the publisher from the package's items, whose
// desks the Resolver places (itemPublisher); the Resolver is keyed by desk, so
// a package with no items yet has no desk to ask.
func (hh *hier) publisherOfPackage(pkg string) string {
	for _, it := range hh.items {
		if hh.itemPackage(it) == pkg {
			if p := hh.itemPublisher(it); p != "" {
				return p
			}
		}
	}
	return ""
}
