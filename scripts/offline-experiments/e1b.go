package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/entitytext"
)

// E1, second form (ledger C218): importance of an arrival is the counted rate
// of independent origins on the subjects it mentions; novelty is the sparsity
// of the arrivals around its vector when it arrived; score is their product.
// No happenings (groups) are used.

const (
	e1bWindow    = 3 * time.Hour
	e1bK         = 10
	e1bMaxSubj   = 12
	e1bFurniture = 0.02
	e1bFurnMin   = 200
)

// subjectsOf is the keys of the names the deployed extractor proposes in the arrival's title and lead.
func subjectsOf(c *capture) []string {
	doc := entitytext.Parse(c.Title, c.Lead)
	ps := entitytext.Propose(doc, nil)
	var out []string
	seen := map[string]bool{}
	for _, p := range ps {
		if p.Key == "" || seen[p.Key] {
			continue
		}
		seen[p.Key] = true
		out = append(out, p.Key)
		if len(out) == e1bMaxSubj {
			break
		}
	}
	return out
}

type scored struct {
	I, N, Score float64
	Subj        []string
	Huang       bool
}

func runE1b(dir, out string) {
	must(os.MkdirAll(out, 0o755))
	arr, _ := loadArrivals(dir)
	n := len(arr)
	subj := make([][]string, n)
	for i := range arr {
		subj[i] = subjectsOf(&arr[i])
	}
	// furniture: keys seen in more than 2% of arrivals so far
	df := map[string]int{}
	type mention struct {
		at     time.Time
		origin string
	}
	byKey := map[string][]mention{}
	day := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	sc := make([]scored, n)
	var kept [][]string = make([][]string, n)
	dropped := 0
	for i := range arr {
		var ks []string
		for _, k := range subj[i] {
			if i >= e1bFurnMin && float64(df[k])/float64(i) > e1bFurniture {
				dropped++
				continue
			}
			ks = append(ks, k)
		}
		for _, k := range subj[i] {
			df[k]++
		}
		kept[i] = ks
		// R_s: distinct origins on s in the window up to and including this arrival
		var I float64
		for _, k := range ks {
			byKey[k] = append(byKey[k], mention{arr[i].Fetched, arr[i].Origin})
			seen := map[string]bool{}
			for j := len(byKey[k]) - 1; j >= 0; j-- {
				m := byKey[k][j]
				if arr[i].Fetched.Sub(m.at) > e1bWindow {
					break
				}
				seen[m.origin] = true
			}
			I += math.Log2(1 + float64(len(seen)))
		}
		// N: 1 - cos to the K-th nearest earlier arrival
		var N float64 = 1
		if !arr[i].Fetched.Before(day) {
			cs := make([]float64, 0, i)
			for j := 0; j < i; j++ {
				cs = append(cs, dot(arr[i].Vec, arr[j].Vec))
			}
			sort.Sort(sort.Reverse(sort.Float64Slice(cs)))
			if len(cs) >= e1bK {
				N = 1 - cs[e1bK-1]
			}
		}
		sc[i] = scored{I: I, N: N, Score: I * N, Subj: ks, Huang: isHuang(&arr[i])}
	}
	var b strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }
	nSubj := 0
	for i := range arr {
		if len(kept[i]) > 0 {
			nSubj++
		}
	}
	w("E1 second form: %d arrivals, %d with at least one subject kept (%d subject mentions dropped as furniture); window %s, k=%d\n\n", n, nSubj, dropped, e1bWindow, e1bK)

	report := func(title string, from, to time.Time) {
		var idx []int
		for i := range arr {
			if arr[i].Fetched.After(from) && !arr[i].Fetched.After(to) {
				idx = append(idx, i)
			}
		}
		w("== %s: %d arrivals in (%s, %s] ==\n", title, len(idx), from.Format("15:04"), to.Format("15:04Z"))
		nh := 0
		for _, i := range idx {
			if sc[i].Huang {
				nh++
			}
		}
		w("Huang-pattern arrivals in the hour: %d\n", nh)
		for _, key := range []struct {
			name string
			f    func(s scored) float64
		}{
			{"score I x N (the claim)", func(s scored) float64 { return s.Score }},
			{"importance I alone", func(s scored) float64 { return s.I }},
			{"novelty N alone", func(s scored) float64 { return s.N }},
		} {
			o := append([]int(nil), idx...)
			sort.SliceStable(o, func(a, c int) bool { return key.f(sc[o[a]]) > key.f(sc[o[c]]) })
			best := 0
			var inTop3 []int
			for r, i := range o {
				if sc[i].Huang {
					if best == 0 {
						best = r + 1
					}
					if r < 3 {
						inTop3 = append(inTop3, r+1)
					}
				}
			}
			w("by %s: best Huang-pattern arrival at rank %d of %d (top %.1f%%); in the top three: %v\n", key.name, best, len(o), 100*float64(best)/float64(len(o)), inTop3)
		}
		o := append([]int(nil), idx...)
		sort.SliceStable(o, func(a, c int) bool { return sc[o[a]].Score > sc[o[c]].Score })
		w("top 10 by score (rank | score | I | N | Huang | origin | title or lead | subjects)\n")
		for r, i := range o {
			if r >= 10 {
				break
			}
			t := strings.TrimSpace(arr[i].Title)
			if t == "" {
				t = "(post) " + arr[i].Lead
			}
			w("%d | %.2f | %.2f | %.3f | %v | %s | %s | %s\n", r+1, sc[i].Score, sc[i].I, sc[i].N, sc[i].Huang, arr[i].Origin, trunc(t, 90), strings.Join(firstN(sc[i].Subj, 4), "; "))
		}
		w("Huang-pattern arrivals by score rank: ")
		for r, i := range o {
			if sc[i].Huang {
				w("%d(I %.1f N %.3f) ", r+1, sc[i].I, sc[i].N)
				if r > 400 {
					break
				}
			}
		}
		w("\n\n")
	}
	t19 := time.Date(2026, 11, 23, 19, 0, 0, 0, time.UTC)
	report("PRIMARY, the hour before 19:00Z", t19.Add(-time.Hour), t19)
	report("sensitivity, the hour 19:00Z to 20:00Z", t19, t19.Add(time.Hour))
	report("sensitivity, the hour 15:00Z to 16:00Z", t19.Add(-4*time.Hour), t19.Add(-3*time.Hour))

	// subjects at 19:00 against groups at 19:00 (3 h window)
	w("== subjects at 19:00Z: top 15 by distinct origins in (16:00, 19:00] ==\n")
	type sr struct {
		k string
		n int
	}
	rs := map[string]map[string]bool{}
	for i := range arr {
		if arr[i].Fetched.After(t19.Add(-e1bWindow)) && !arr[i].Fetched.After(t19) {
			for _, k := range kept[i] {
				if rs[k] == nil {
					rs[k] = map[string]bool{}
				}
				rs[k][arr[i].Origin] = true
			}
		}
	}
	var srs []sr
	for k, v := range rs {
		srs = append(srs, sr{k, len(v)})
	}
	sort.Slice(srs, func(i, j int) bool {
		if srs[i].n != srs[j].n {
			return srs[i].n > srs[j].n
		}
		return srs[i].k < srs[j].k
	})
	for i, s := range srs {
		if i >= 15 {
			break
		}
		w("%2d | %d origins | %s\n", i+1, s.n, s.k)
	}
	// the same population grouped: top groups by origins in the window (the C215 rule), for side by side
	d, _ := loadE1(dir)
	r := d.computeAt(t19, defaultWindw)
	w("\n== groups at 19:00Z (C215 rule): top 15 by distinct origins in (16:00, 19:00] ==\n")
	for i, h := range r.Hs {
		if i >= 15 {
			break
		}
		w("%2d | %d origins | %d members | %s\n", i+1, h.Origins, h.Members, d.label(h.G))
	}
	// how much of a group a single subject accounts for (mechanical, for groups with 3+ origins in the window)
	w("\n== do groups with 3 or more origins in the window share a subject? (mechanical) ==\n")
	idxOf := map[string]int{}
	for i := range arr {
		idxOf[arr[i].ID] = i
	}
	nG, share60, share30 := 0, 0, 0
	for _, h := range r.Hs {
		if h.Origins < 3 {
			continue
		}
		g := &d.groups[h.G]
		k := d.membersUpTo(g, t19)
		cnt := map[string]int{}
		for _, m := range g.Members[:k] {
			for _, s := range kept[m] {
				cnt[s]++
			}
		}
		best := 0
		for _, c := range cnt {
			if c > best {
				best = c
			}
		}
		nG++
		if float64(best) >= 0.6*float64(k) {
			share60++
		}
		if float64(best) >= 0.3*float64(k) {
			share30++
		}
	}
	w("groups with 3+ origins in the window: %d; with one subject in 60%% or more of members: %d; in 30%% or more: %d\n", nG, share60, share30)
	must(os.WriteFile(filepath.Join(out, "e1b-results.txt"), []byte(b.String()), 0o644))
	fmt.Print(b.String())
}

func firstN(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// keptSubjects applies the C218 subject rule to every arrival in order: the
// extractor's names, minus a key seen in more than 2% of arrivals so far.
func keptSubjects(arr []capture) [][]string {
	df := map[string]int{}
	out := make([][]string, len(arr))
	for i := range arr {
		subj := subjectsOf(&arr[i])
		for _, k := range subj {
			if i >= e1bFurnMin && float64(df[k])/float64(i) > e1bFurniture {
				continue
			}
			out[i] = append(out[i], k)
		}
		for _, k := range subj {
			df[k]++
		}
	}
	return out
}
