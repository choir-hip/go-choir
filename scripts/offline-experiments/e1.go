package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type version struct {
	ID  string
	Rev string
	Pub time.Time
}

type story struct {
	ID       string
	Title    string
	Versions []version // by publish time
}

type e1Data struct {
	arr      []capture
	groups   []group
	groupOf  []int // arrival index -> group index
	stories  []*story
	passages map[string][][]float32 // version id -> passage vectors
	cites    map[string][]cite      // version id -> cited arrivals
}

type cite struct {
	Arr     int
	Created time.Time
}

func loadE1(dir string) (*e1Data, int) {
	arr, dropped := loadArrivals(dir)
	d := &e1Data{arr: arr, passages: map[string][][]float32{}, cites: map[string][]cite{}}
	d.groups = groupArrivals(arr, allIdx(len(arr)))
	d.groupOf = make([]int, len(arr))
	for gi, g := range d.groups {
		for _, m := range g.Members {
			d.groupOf[m] = gi
		}
	}
	byID := map[string]int{}
	for i, c := range arr {
		byID[c.ID] = i
	}
	sByID := map[string]*story{}
	seenV := map[string]bool{}
	readCSV(filepath.Join(dir, "versions.csv"), func(r []string) {
		if len(r) < 5 || seenV[r[1]] {
			return
		}
		seenV[r[1]] = true
		pub := parseTime(r[3])
		if pub.IsZero() {
			return
		}
		s := sByID[r[0]]
		if s == nil {
			s = &story{ID: r[0], Title: r[4]}
			sByID[r[0]] = s
			d.stories = append(d.stories, s)
		}
		s.Versions = append(s.Versions, version{ID: r[1], Rev: r[2], Pub: pub})
	})
	for _, s := range d.stories {
		sort.Slice(s.Versions, func(i, j int) bool { return s.Versions[i].Pub.Before(s.Versions[j].Pub) })
	}
	readCSV(filepath.Join(dir, "story-vectors.csv"), func(r []string) {
		if len(r) >= 4 {
			d.passages[r[1]] = append(d.passages[r[1]], parseVec(r[3]))
		}
	})
	readCSV(filepath.Join(dir, "cites.csv"), func(r []string) {
		if len(r) < 3 {
			return
		}
		i, ok := byID[r[1]]
		if !ok {
			return
		}
		d.cites[r[0]] = append(d.cites[r[0]], cite{Arr: i, Created: parseTime(r[2])})
	})
	return d, dropped
}

// membersUpTo is how many of the group's members arrived at or before t.
func (d *e1Data) membersUpTo(g *group, t time.Time) int {
	return sort.Search(len(g.Members), func(i int) bool { return d.arr[g.Members[i]].Fetched.After(t) })
}

func (d *e1Data) centroidAt(g *group, k int) []float32 {
	sum := make([]float32, len(d.arr[g.Members[0]].Vec))
	for _, m := range g.Members[:k] {
		for i, x := range d.arr[m].Vec {
			sum[i] += x
		}
	}
	return normalize(sum)
}

type frontRow struct {
	Story *story
	Rank  int
	Mass  float64
	Ver   *version // latest version at t
}

func (d *e1Data) frontAt(t time.Time) []frontRow {
	type cand struct {
		s *story
		v *version
	}
	var cs []cand
	for _, s := range d.stories {
		k := sort.Search(len(s.Versions), func(i int) bool { return s.Versions[i].Pub.After(t) })
		if k > 0 {
			cs = append(cs, cand{s, &s.Versions[k-1]})
		}
	}
	sort.Slice(cs, func(i, j int) bool {
		if !cs[i].v.Pub.Equal(cs[j].v.Pub) {
			return cs[i].v.Pub.After(cs[j].v.Pub)
		}
		return cs[i].s.ID > cs[j].s.ID
	})
	if len(cs) > frontSize {
		cs = cs[:frontSize]
	}
	var norm float64
	rows := make([]frontRow, len(cs))
	for i, c := range cs {
		m := 1 / math.Log2(float64(i+2))
		rows[i] = frontRow{Story: c.s, Rank: i + 1, Mass: m, Ver: c.v}
		norm += m
	}
	for i := range rows {
		rows[i].Mass /= norm
	}
	return rows
}

type happening struct {
	G       int
	Members int // arrivals up to t
	Origins int // distinct origins in the window
	P       float64
	Q       float64 // mass assigned by covering stories
	Covers  []int   // front ranks of the stories assigned to it
	Term    float64 // p log2(p/q)
	Covered bool
}

type atT struct {
	T          time.Time
	Window     time.Duration
	Hs         []happening // groups with p > 0, by p desc
	D          float64
	Missing    float64
	Misranked  float64
	NOrigins   int
	Front      []frontRow
	Assign     map[int]int // front rank -> group index it was assigned to (absent: elsewhere)
	HuangGroup int         // -1 when not identified
	HuangN     int
}

var (
	reHuang = regexp.MustCompile(`huang`)
	reNv    = regexp.MustCompile(`nvidia`)
	reRe    = regexp.MustCompile(`reinstat|return|back as|restor`)
)

func isHuang(c *capture) bool {
	s := strings.ToLower(c.Title + " " + c.Lead)
	return reHuang.MatchString(s) && reNv.MatchString(s) && reRe.MatchString(s)
}

// computeAt evaluates the measure at t with a trailing window.
func (d *e1Data) computeAt(t time.Time, window time.Duration) *atT {
	res := &atT{T: t, Window: window, HuangGroup: -1}
	from := t.Add(-window)
	// groups as of t
	type live struct {
		gi, k  int
		centre []float32
	}
	var alive []live
	pOrig := map[int]int{}
	huangCount := map[int]int{}
	for gi := range d.groups {
		g := &d.groups[gi]
		if g.Opened.After(t) {
			continue
		}
		k := d.membersUpTo(g, t)
		if k == 0 {
			continue
		}
		last := d.arr[g.Members[k-1]].Fetched
		for _, m := range g.Members[:k] {
			if isHuang(&d.arr[m]) {
				huangCount[gi]++
			}
		}
		if n := len(originsOf(d.arr, g.Members[:k], from, t)); n > 0 {
			pOrig[gi] = n
		}
		if t.Sub(last) <= groupQuiet {
			alive = append(alive, live{gi: gi, k: k})
		}
	}
	best, bestN, total := -1, 0, 0
	for gi, n := range huangCount {
		total += n
		if n > bestN || (n == bestN && gi < best) {
			best, bestN = gi, n
		}
	}
	if bestN >= 5 {
		res.HuangGroup, res.HuangN = best, bestN
	}
	// centroids as of t, lazily, for groups that are alive
	for i := range alive {
		alive[i].centre = d.centroidAt(&d.groups[alive[i].gi], alive[i].k)
	}
	aliveIdx := map[int]int{}
	for i, a := range alive {
		aliveIdx[a.gi] = i
	}
	// front and assignment
	res.Front = d.frontAt(t)
	res.Assign = map[int]int{}
	qMass := map[int]float64{}
	covers := map[int][]int{}
	for _, row := range res.Front {
		cited := map[int]int{}
		for _, v := range row.Story.Versions {
			if v.Pub.After(t) {
				break
			}
			for _, c := range d.cites[v.ID] {
				if c.Created.After(t) || d.arr[c.Arr].Fetched.After(t) {
					continue
				}
				cited[d.groupOf[c.Arr]]++
			}
		}
		bestG, bestC, bestCos := -1, 0, -2.0
		consider := func(gi, nc int, cos float64) {
			if nc > bestC || (nc == bestC && cos > bestCos) {
				bestG, bestC, bestCos = gi, nc, cos
			}
		}
		for gi, nc := range cited {
			cos := -2.0
			if ai, ok := aliveIdx[gi]; ok {
				for _, p := range d.passages[row.Ver.ID] {
					if c := dot(p, alive[ai].centre); c > cos {
						cos = c
					}
				}
			}
			consider(gi, nc, cos)
		}
		if bestG < 0 {
			for _, a := range alive {
				cos := -2.0
				for _, p := range d.passages[row.Ver.ID] {
					if c := dot(p, a.centre); c > cos {
						cos = c
					}
				}
				if cos >= coverCos {
					consider(a.gi, 0, cos)
				}
			}
		}
		if bestG >= 0 {
			res.Assign[row.Rank] = bestG
			qMass[bestG] += row.Mass
			covers[bestG] = append(covers[bestG], row.Rank)
		}
	}
	// p, terms
	tot := 0
	for _, n := range pOrig {
		tot += n
	}
	res.NOrigins = tot
	for gi, n := range pOrig {
		p := float64(n) / float64(tot)
		q := qMass[gi]
		cov := q > 0
		if !cov {
			q = epsilonQ
		}
		h := happening{G: gi, Members: d.membersUpTo(&d.groups[gi], t), Origins: n, P: p, Q: q, Covers: covers[gi], Covered: cov}
		h.Term = p * math.Log2(p/q)
		res.D += h.Term
		if cov {
			res.Misranked += h.Term
		} else {
			res.Missing += h.Term
		}
		res.Hs = append(res.Hs, h)
	}
	sort.Slice(res.Hs, func(i, j int) bool {
		if res.Hs[i].Origins != res.Hs[j].Origins {
			return res.Hs[i].Origins > res.Hs[j].Origins
		}
		return res.Hs[i].Members > res.Hs[j].Members
	})
	return res
}

// rankBy returns 1 + the number of happenings strictly greater by key, for group gi (0 when absent).
func rankBy(hs []happening, gi int, key func(h happening) float64) int {
	var v float64
	found := false
	for _, h := range hs {
		if h.G == gi {
			v, found = key(h), true
		}
	}
	if !found {
		return 0
	}
	r := 1
	for _, h := range hs {
		if key(h) > v+1e-12 {
			r++
		}
	}
	return r
}

// label is the first non-empty title among the group's members, else the start of the first lead.
func (d *e1Data) label(gi int) string {
	g := &d.groups[gi]
	for _, m := range g.Members {
		if t := strings.TrimSpace(d.arr[m].Title); t != "" {
			return trunc(t, 90)
		}
	}
	return "(post) " + trunc(d.arr[g.Members[0]].Lead, 80)
}

func runE1(dir, out string) {
	must(os.MkdirAll(out, 0o755))
	d, dropped := loadE1(dir)
	var b strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }
	span := d.arr[len(d.arr)-1].Fetched.Sub(d.arr[0].Fetched)
	w("E1 replay: %d arrivals after the copy check (%d dropped as copies), %s to %s (%.1f h); %d groups; %d stories with versions, %d versions, %d versions with passage vectors.\n",
		len(d.arr), dropped, d.arr[0].Fetched.Format("2006-01-02T15:04Z"), d.arr[len(d.arr)-1].Fetched.Format("2006-01-02T15:04Z"), span.Hours(), len(d.groups), len(d.stories), nVersions(d), len(d.passages))
	sz := map[int]int{}
	maxSz := 0
	for _, g := range d.groups {
		sz[len(g.Members)]++
		if len(g.Members) > maxSz {
			maxSz = len(g.Members)
		}
	}
	w("group sizes: singletons %d, 2 to 4 members %d, 5 or more %d, largest %d\n\n", sz[1], countRange(sz, 2, 4), countRange(sz, 5, 1<<30), maxSz)

	day := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	windows := []time.Duration{3 * time.Hour, time.Hour, 6 * time.Hour, 24 * time.Hour}
	for _, win := range windows {
		primary := win == defaultWindw
		if primary {
			w("== PRIMARY, window %s ==\n", win)
		} else {
			w("== SENSITIVITY, window %s ==\n", win)
		}
		w("hour(UTC) | origins in window | happenings | D bits | of which happenings with 2+ origins | Missing | Misranked | h* identified (arrivals) | h* rank by p | h* cover story front rank | h* rank by term\n")
		maxMis := math.Inf(-1)
		maxHour := -1
		for h := 0; h < 24; h++ {
			t := day.Add(time.Duration(h) * time.Hour)
			r := d.computeAt(t, win)
			hr, cr, tr, cover := "-", "-", "-", "-"
			if r.HuangGroup >= 0 {
				hr = fmt.Sprint(rankBy(r.Hs, r.HuangGroup, func(x happening) float64 { return x.P }))
				tr = fmt.Sprint(rankBy(r.Hs, r.HuangGroup, func(x happening) float64 { return x.Term }))
				for _, x := range r.Hs {
					if x.G == r.HuangGroup && len(x.Covers) > 0 {
						cover = fmt.Sprint(x.Covers)
					}
				}
				_ = cr
			}
			id := "no"
			if r.HuangGroup >= 0 {
				id = fmt.Sprintf("yes (%d)", r.HuangN)
			}
			d2 := 0.0
			for _, x := range r.Hs {
				if x.Origins >= 2 {
					d2 += x.Term
				}
			}
			w("%02d:00 | %d | %d | %.3f | %.3f | %.3f | %.3f | %s | %s | %s | %s\n", h, r.NOrigins, len(r.Hs), r.D, d2, r.Missing, r.Misranked, id, hr, cover, tr)
			if r.Misranked > maxMis {
				maxMis, maxHour = r.Misranked, h
			}
		}
		w("hour of highest Misranked (covered happenings only, a signed sum) on 11-23: %02d:00 (%.3f bits)\n\n", maxHour, maxMis)
	}

	// the 19:00 detail, primary window
	t := time.Date(2026, 11, 23, 19, 0, 0, 0, time.UTC)
	r := d.computeAt(t, defaultWindw)
	w("== 2026-11-23T19:00Z detail, window 3 h ==\n")
	if r.HuangGroup < 0 {
		w("h* not identified (fewer than five arrivals matching huang, nvidia and reinstat|return|back as|restor before 19:00)\n")
	} else {
		g := &d.groups[r.HuangGroup]
		w("h* = group %d: %d members up to 19:00 (%d match the pattern), opened %s, title of first member: %q\n", r.HuangGroup, r.Hs[findH(r.Hs, r.HuangGroup)].Members, r.HuangN, g.Opened.Format("15:04Z"), d.label(r.HuangGroup))
		hh := r.Hs[findH(r.Hs, r.HuangGroup)]
		w("h*: distinct origins in the window %d of %d (p = %.4f), p rank %d of %d happenings, q = %.5f (covered: %v, front ranks %v), term %.4f bits, term rank %d\n",
			hh.Origins, r.NOrigins, hh.P, rankBy(r.Hs, r.HuangGroup, func(x happening) float64 { return x.P }), len(r.Hs), hh.Q, hh.Covered, hh.Covers, hh.Term,
			rankBy(r.Hs, r.HuangGroup, func(x happening) float64 { return x.Term }))
	}
	w("\nfront at 19:00 (rank | published | revisions | assigned to group (origins in window) | title)\n")
	for _, row := range r.Front {
		as := "elsewhere"
		if gi, ok := r.Assign[row.Rank]; ok {
			n := 0
			for _, h := range r.Hs {
				if h.G == gi {
					n = h.Origins
				}
			}
			as = fmt.Sprintf("group %d (%d origins in window, %d members)", gi, n, d.membersUpTo(&d.groups[gi], t))
		}
		hl := ""
		if strings.Contains(strings.ToLower(row.Story.Title), "huang") || strings.Contains(strings.ToLower(row.Story.Title), "nvidia") {
			hl = "   <== Huang/Nvidia in the title"
		}
		w("%2d | %s | %s | %s | %s%s\n", row.Rank, row.Ver.Pub.Format("15:04:05Z"), row.Ver.Rev, as, trunc(row.Story.Title, 80), hl)
	}
	w("\ntop 15 happenings by world attention at 19:00 (rank | origins | members | q | cover ranks | term | first title)\n")
	for i, h := range r.Hs {
		if i >= 15 {
			break
		}
		w("%2d | %d | %d | %.5f | %v | %.4f | %s\n", i+1, h.Origins, h.Members, h.Q, h.Covers, h.Term, d.label(h.G))
	}
	ts := append([]happening(nil), r.Hs...)
	sort.Slice(ts, func(i, j int) bool { return ts[i].Term > ts[j].Term })
	w("\ntop 10 terms p*log2(p/q) at 19:00 (term | origins | q | cover ranks | first title)\n")
	for i, h := range ts {
		if i >= 10 {
			break
		}
		w("%.4f | %d | %.5f | %v | %s\n", h.Term, h.Origins, h.Q, h.Covers, d.label(h.G))
	}
	if r.HuangGroup >= 0 {
		g := &d.groups[r.HuangGroup]
		k := d.membersUpTo(g, t)
		w("\nh* members (first 12 of %d up to 19:00): time | origin | title\n", k)
		for i, m := range g.Members[:k] {
			if i >= 12 {
				break
			}
			a := d.arr[m]
			w("%s | %s | %s\n", a.Fetched.Format("15:04Z"), a.Origin, trunc(a.Title, 100))
		}
		w("\nall Huang-pattern arrivals before 19:00, by group: ")
		by := map[int]int{}
		for i := range d.arr {
			if d.arr[i].Fetched.After(t) {
				break
			}
			if isHuang(&d.arr[i]) {
				by[d.groupOf[i]]++
			}
		}
		var ks []int
		for k := range by {
			ks = append(ks, k)
		}
		sort.Slice(ks, func(i, j int) bool { return by[ks[i]] > by[ks[j]] })
		for _, k := range ks {
			w("[group %d: %d] ", k, by[k])
		}
		w("\n")
		first := time.Time{}
		for i := range d.arr {
			if isHuang(&d.arr[i]) {
				first = d.arr[i].Fetched
				break
			}
		}
		w("first Huang-pattern arrival of the export: %s\n", first.Format("2006-01-02T15:04Z"))
	}
	// minute-resolution trace of h* (not a pass line): the replayed front is recency order
	w("\n== trace of h*, every 5 minutes 14:00Z to 23:55Z, window 3 h ==\n")
	w("time | h* p rank | best front rank of a story covering it | h* term | h* term rank | D\n")
	var sumTerm, sumQ, sumP float64
	nT, inTop10, inTop3p, termTop3 := 0, 0, 0, 0
	for m := 14 * 60; m < 24*60; m += 5 {
		t := day.Add(time.Duration(m) * time.Minute)
		r := d.computeAt(t, defaultWindw)
		if r.HuangGroup < 0 {
			continue
		}
		hh := r.Hs[findH(r.Hs, r.HuangGroup)]
		pr := rankBy(r.Hs, r.HuangGroup, func(x happening) float64 { return x.P })
		tr := rankBy(r.Hs, r.HuangGroup, func(x happening) float64 { return x.Term })
		best := 0
		for _, c := range hh.Covers {
			if best == 0 || c < best {
				best = c
			}
		}
		if m%30 == 0 || (t.Hour() == 19 && t.Minute() <= 30) {
			w("%s | %d | %d | %.4f | %d | %.3f\n", t.Format("15:04"), pr, best, hh.Term, tr, r.D)
		}
		nT++
		sumTerm += hh.Term
		sumQ += hh.Q
		sumP += hh.P
		if best > 0 && best <= 10 {
			inTop10++
		}
		if pr <= 3 {
			inTop3p++
		}
		if tr <= 3 {
			termTop3++
		}
	}
	w("over %d five-minute steps from the first with h* identified: mean p %.4f, mean q %.4f, mean term %.4f bits; h* in the world's top 3 by p at %d of %d steps; its best covering story in the front's top 10 at %d of %d steps; its term in the top 3 terms at %d of %d steps\n",
		nT, sumP/float64(nT), sumQ/float64(nT), sumTerm/float64(nT), inTop3p, nT, inTop10, nT, termTop3, nT)
	must(os.WriteFile(filepath.Join(out, "e1-results.txt"), []byte(b.String()), 0o644))
	fmt.Print(b.String())
}

func findH(hs []happening, gi int) int {
	for i, h := range hs {
		if h.G == gi {
			return i
		}
	}
	return 0
}

func nVersions(d *e1Data) int {
	n := 0
	for _, s := range d.stories {
		n += len(s.Versions)
	}
	return n
}

func countRange(m map[int]int, lo, hi int) int {
	n := 0
	for k, v := range m {
		if k >= lo && k <= hi {
			n += v
		}
	}
	return n
}
