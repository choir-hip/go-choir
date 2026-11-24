package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// E4 over the whole subject set of each arrival (ledger C226 to C231).

type e4sRow struct {
	Isum, Itop5            float64
	Na2, Nb2, Nc2, NcShare float64
	Bset, Ne, NePair       float64
	NSubj                  int
	Huang, Echo, HuangEcho bool
	NewWords               []string
	NewPairs               int
}

type e4sForm struct {
	Name string
	Get  func(r *e4sRow) float64
}

func runE4Set(dir, out string) {
	must(os.MkdirAll(out, 0o755))
	d, _ := loadE1(dir)
	arr := d.arr
	n := len(arr)
	kept := keptSubjects(arr)
	day := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)

	type citeRec struct {
		story     *story
		pub, made time.Time
		arr       int
	}
	var cites []citeRec
	for _, s := range d.stories {
		for _, v := range s.Versions {
			for _, c := range d.cites[v.ID] {
				cites = append(cites, citeRec{s, v.Pub, c.Created, c.Arr})
			}
		}
	}
	type mention struct {
		at     time.Time
		origin string
	}
	byKey := map[string][]mention{}
	subjArr := map[string][]int{}
	subjWords := map[string]map[string]bool{}
	wordDF := map[string]int{}
	pairSeen := map[string]bool{}
	distinct := func(ms []mention, from, to time.Time) int {
		seen := map[string]bool{}
		for j := len(ms) - 1; j >= 0; j-- {
			if ms[j].at.After(to) {
				continue
			}
			if !ms[j].at.After(from) {
				break
			}
			seen[ms[j].origin] = true
		}
		return len(seen)
	}
	pairKey := func(a, b string) string {
		if a > b {
			a, b = b, a
		}
		return a + "|" + b
	}
	rows := make([]e4sRow, n)
	for i := range arr {
		a := &arr[i]
		t := a.Fetched
		r := &rows[i]
		r.Huang = isHuang(a)
		ks := kept[i]
		r.NSubj = len(ks)
		type sr struct {
			k  string
			rs int
		}
		var srs []sr
		for _, k := range ks {
			byKey[k] = append(byKey[k], mention{t, a.Origin})
			srs = append(srs, sr{k, distinct(byKey[k], t.Add(-3*time.Hour), t)})
		}
		sort.SliceStable(srs, func(x, y int) bool { return srs[x].rs > srs[y].rs })
		var logs []float64
		for _, s := range srs {
			l := math.Log2(1 + float64(s.rs))
			r.Isum += l
			logs = append(logs, l)
		}
		for k := 0; k < len(logs) && k < 5; k++ {
			r.Itop5 += logs[k]
		}
		if m := len(logs); m > 0 {
			if m > 5 {
				m = 5
			}
			r.Itop5 /= float64(m)
		}
		words := contentWords(a)
		if !t.Before(day) {
			cs := make([]float64, 0, i)
			for j := 0; j < i; j++ {
				cs = append(cs, dot(a.Vec, arr[j].Vec))
			}
			sort.Sort(sort.Reverse(sort.Float64Slice(cs)))
			r.Echo = len(cs) > 0 && cs[0] >= e4EchoCos
			// shared-subject counts with earlier arrivals
			shared := map[int]int{}
			for _, k := range ks {
				for _, j := range subjArr[k] {
					shared[j]++
				}
			}
			// (a2)
			best := -2.0
			for j, c := range shared {
				if c >= 2 {
					if v := dot(a.Vec, arr[j].Vec); v > best {
						best = v
					}
				}
			}
			r.Na2 = 1
			if best > -2 {
				r.Na2 = 1 - best
			}
			// (b2)
			cnt := map[*story]map[int]bool{}
			for _, c := range cites {
				if c.pub.After(t) || c.made.After(t) || arr[c.arr].Fetched.After(t) || shared[c.arr] < 2 {
					continue
				}
				if cnt[c.story] == nil {
					cnt[c.story] = map[int]bool{}
				}
				cnt[c.story][c.arr] = true
			}
			var cover *story
			coverN := 1
			var coverPub time.Time
			for s, m := range cnt {
				latest := latestAt(s, t)
				if latest == nil {
					continue
				}
				if len(m) > coverN || (len(m) == coverN && coverN >= 2 && (cover == nil || latest.Pub.After(coverPub))) {
					cover, coverN, coverPub = s, len(m), latest.Pub
				}
			}
			r.Nb2 = 1
			if cover != nil {
				bc := -2.0
				for _, p := range d.passages[latestAt(cover, t).ID] {
					if c := dot(a.Vec, p); c > bc {
						bc = c
					}
				}
				if bc > -2 {
					r.Nb2 = 1 - bc
				}
			}
			// (c2)
			elig := 0
			for _, w := range words {
				if float64(wordDF[w])/float64(i) <= e4DFMax {
					elig++
					known := false
					for _, k := range ks {
						if subjWords[k][w] {
							known = true
							break
						}
					}
					if !known && len(ks) > 0 {
						r.Nc2++
						r.NewWords = append(r.NewWords, w)
					}
				}
			}
			if elig > 0 {
				r.NcShare = r.Nc2 / float64(elig)
			}
			// (d2)
			var bs float64
			m := 0
			for k := 0; k < len(srs) && k < 5; k++ {
				ms := byKey[srs[k].k]
				fast := distinct(ms, t.Add(-e4Fast), t)
				slow := distinct(ms, t.Add(-e4SlowBack), t.Add(-e4Fast))
				bs += float64(fast) / math.Max(float64(slow)/12, 0.25)
				m++
			}
			if m > 0 {
				r.Bset = bs / float64(m)
			}
			// (e)
			top := srs
			if len(top) > 8 {
				top = top[:8]
			}
			pairs, newPairs, newSubj := 0, 0, 0
			for x := 0; x < len(top); x++ {
				if len(subjArr[top[x].k]) == 0 {
					newSubj++
				}
				for y := x + 1; y < len(top); y++ {
					pairs++
					if !pairSeen[pairKey(top[x].k, top[y].k)] {
						newPairs++
					}
				}
			}
			r.NewPairs = newPairs
			if pairs > 0 {
				r.NePair = float64(newPairs) / float64(pairs)
			}
			if pairs+len(top) > 0 {
				r.Ne = float64(newPairs+newSubj) / float64(pairs+len(top))
			}
		}
		for _, k := range ks {
			subjArr[k] = append(subjArr[k], i)
			if subjWords[k] == nil {
				subjWords[k] = map[string]bool{}
			}
			for _, w := range words {
				subjWords[k][w] = true
			}
		}
		for x := 0; x < len(ks); x++ {
			for y := x + 1; y < len(ks); y++ {
				pairSeen[pairKey(ks[x], ks[y])] = true
			}
		}
		for _, w := range words {
			wordDF[w]++
		}
	}
	var huangSoFar []int
	for i := range arr {
		if arr[i].Fetched.Before(day) {
			if rows[i].Huang {
				huangSoFar = append(huangSoFar, i)
			}
			continue
		}
		if rows[i].Huang {
			for _, j := range huangSoFar {
				if dot(arr[i].Vec, arr[j].Vec) >= e4EchoCos {
					rows[i].HuangEcho = true
					break
				}
			}
			huangSoFar = append(huangSoFar, i)
		}
	}
	forms := []e4sForm{
		{"I_sum alone", func(r *e4sRow) float64 { return r.Isum }},
		{"I_top5 alone", func(r *e4sRow) float64 { return r.Itop5 }},
		{"(a2) I_sum x nearest sharing 2 subjects", func(r *e4sRow) float64 { return r.Isum * r.Na2 }},
		{"(b2) I_sum x story-scoped (set)", func(r *e4sRow) float64 { return r.Isum * r.Nb2 }},
		{"(c2) I_sum x new words (count)", func(r *e4sRow) float64 { return r.Isum * r.Nc2 }},
		{"(c2') I_sum x new words (share)", func(r *e4sRow) float64 { return r.Isum * r.NcShare }},
		{"(d2) I_sum x burst (set)", func(r *e4sRow) float64 { return r.Isum * r.Bset }},
		{"(e) I_sum x symbolic novelty", func(r *e4sRow) float64 { return r.Isum * r.Ne }},
		{"(e') I_sum x pair novelty only", func(r *e4sRow) float64 { return r.Isum * r.NePair }},
		{"(a2) I_top5 x nearest sharing 2", func(r *e4sRow) float64 { return r.Itop5 * r.Na2 }},
		{"(b2) I_top5 x story-scoped", func(r *e4sRow) float64 { return r.Itop5 * r.Nb2 }},
		{"(c2) I_top5 x new words", func(r *e4sRow) float64 { return r.Itop5 * r.Nc2 }},
		{"(d2) I_top5 x burst", func(r *e4sRow) float64 { return r.Itop5 * r.Bset }},
		{"(e) I_top5 x symbolic novelty", func(r *e4sRow) float64 { return r.Itop5 * r.Ne }},
	}
	bin := func(i int) int { return int((arr[i].Fetched.Add(-time.Nanosecond).Sub(day)) / time.Hour) }
	byBin := map[int][]int{}
	var dayIdx []int
	for i := range arr {
		if !arr[i].Fetched.Before(day) {
			byBin[bin(i)] = append(byBin[bin(i)], i)
			dayIdx = append(dayIdx, i)
		}
	}
	hourOf := func(end int) []int { return byBin[end-1] }
	rankIn := func(idx []int, f e4sForm) map[int]int {
		vals := make([]float64, len(idx))
		for k, i := range idx {
			vals[k] = f.Get(&rows[i])
		}
		rk := map[int]int{}
		for k, i := range idx {
			c := 1
			for _, v := range vals {
				if v > vals[k]+1e-12 {
					c++
				}
			}
			rk[i] = c
		}
		return rk
	}
	var b strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }
	w("E4 set forms: %d arrivals on 11-23; mean subjects per arrival %.1f; hour bins (X-1:00, X:00]\n\n", len(dayIdx), func() float64 {
		s := 0
		for _, i := range dayIdx {
			s += rows[i].NSubj
		}
		return float64(s) / float64(len(dayIdx))
	}())
	w("== Best Huang-pattern arrival's rank in its hour, by form ==\nhour end (n Huang | N) |")
	for _, f := range forms {
		w(" %s |", f.Name)
	}
	w("\n")
	for end := 1; end <= 24; end++ {
		idx := hourOf(end)
		nh := 0
		for _, i := range idx {
			if rows[i].Huang {
				nh++
			}
		}
		if nh == 0 {
			continue
		}
		w("%02d:00 (%d | %d) |", end%24, nh, len(idx))
		for _, f := range forms {
			rk := rankIn(idx, f)
			best := 1 << 30
			for _, i := range idx {
				if rows[i].Huang && rk[i] < best {
					best = rk[i]
				}
			}
			w(" %d |", best)
		}
		w("\n")
	}
	w("\n== whole day: Huang arrivals against all arrivals of the day: best rank, median rank share ==\n")
	for _, f := range forms {
		rk := rankIn(dayIdx, f)
		best := 1 << 30
		var sh []float64
		for _, i := range dayIdx {
			if rows[i].Huang {
				if rk[i] < best {
					best = rk[i]
				}
				sh = append(sh, float64(rk[i])/float64(len(dayIdx)))
			}
		}
		sort.Float64s(sh)
		w("%s: best %d, median share %.3f\n", f.Name, best, sh[len(sh)/2])
	}
	for _, spec := range []struct {
		name string
		end  int
	}{{"PRIMARY hour (18:00, 19:00]", 19}, {"TURN hour (14:00, 15:00]", 15}} {
		idx := hourOf(spec.end)
		w("\n== %s: %d arrivals; Huang ranks by form ==\n", spec.name, len(idx))
		for _, f := range forms {
			rk := rankIn(idx, f)
			var rs []int
			for _, i := range idx {
				if rows[i].Huang {
					rs = append(rs, rk[i])
				}
			}
			sort.Ints(rs)
			if len(rs) > 12 {
				rs = rs[:12]
			}
			w("%s: %v\n", f.Name, rs)
		}
	}
	w("\n== Huang arrivals of the primary hour: time | origin | nsubj | I_sum | I_top5 | Na2 | Nb2 | Nc2 (share) | Bset | Ne | pair share | new words ==\n")
	for _, i := range hourOf(19) {
		r := &rows[i]
		if r.Huang {
			w("%s | %s | %d | %.1f | %.2f | %.3f | %.3f | %.0f (%.2f) | %.1f | %.2f | %.2f | %s\n", arr[i].Fetched.Format("15:04"), arr[i].Origin, r.NSubj, r.Isum, r.Itop5, r.Na2, r.Nb2, r.Nc2, r.NcShare, r.Bset, r.Ne, r.NePair, strings.Join(firstN(r.NewWords, 6), ","))
		}
	}
	w("\n== Huang arrivals of the turn hour: same columns ==\n")
	for _, i := range hourOf(15) {
		r := &rows[i]
		if r.Huang {
			w("%s | %s | %d | %.1f | %.2f | %.3f | %.3f | %.0f (%.2f) | %.1f | %.2f | %.2f | %s\n", arr[i].Fetched.Format("15:04"), arr[i].Origin, r.NSubj, r.Isum, r.Itop5, r.Na2, r.Nb2, r.Nc2, r.NcShare, r.Bset, r.Ne, r.NePair, strings.Join(firstN(r.NewWords, 6), ","))
		}
	}
	w("\nreinstat*/return*/restor* new-word firings among Huang arrivals of the day (time origin word): ")
	for _, i := range dayIdx {
		if !rows[i].Huang {
			continue
		}
		for _, nw := range rows[i].NewWords {
			if strings.HasPrefix(nw, "reinstat") || strings.HasPrefix(nw, "return") || strings.HasPrefix(nw, "restor") {
				w("[%s %s %s] ", arr[i].Fetched.Format("15:04"), arr[i].Origin, nw)
			}
		}
	}
	nEcho := 0
	for _, i := range dayIdx {
		if rows[i].Echo {
			nEcho++
		}
	}
	w("\n\n== ECHOES: %d of %d arrivals of 11-23; median rank share within the hour; Huang echoes (n and median) ==\n", nEcho, len(dayIdx))
	for _, f := range forms {
		var sh, hsh []float64
		for end := 1; end <= 24; end++ {
			idx := hourOf(end)
			rk := rankIn(idx, f)
			for _, i := range idx {
				if rows[i].Echo {
					sh = append(sh, float64(rk[i])/float64(len(idx)))
				}
				if rows[i].HuangEcho {
					hsh = append(hsh, float64(rk[i])/float64(len(idx)))
				}
			}
		}
		sort.Float64s(sh)
		sort.Float64s(hsh)
		hm := math.NaN()
		if len(hsh) > 0 {
			hm = hsh[len(hsh)/2]
		}
		w("%s: echo median %.3f (n %d), 10th pct %.3f; Huang echoes n %d median %.3f\n", f.Name, sh[len(sh)/2], len(sh), sh[len(sh)/10], len(hsh), hm)
	}
	for _, end := range []int{19, 15, 3, 9, 21} {
		idx := hourOf(end)
		for _, f := range []e4sForm{forms[0], forms[2], forms[3], forms[4], forms[6], forms[7]} {
			rk := rankIn(idx, f)
			o := append([]int(nil), idx...)
			sort.SliceStable(o, func(x, y int) bool { return rk[o[x]] < rk[o[y]] })
			w("\n== top 10, hour ending %02d:00, %s ==\n", end%24, f.Name)
			for k, i := range o {
				if k >= 10 {
					break
				}
				tt := strings.TrimSpace(arr[i].Title)
				if tt == "" {
					tt = "(post) " + arr[i].Lead
				}
				w("%d | %.2f | H=%v echo=%v | %s | nsubj %d | %s\n", k+1, f.Get(&rows[i]), rows[i].Huang, rows[i].Echo, arr[i].Origin, rows[i].NSubj, trunc(tt, 90))
			}
		}
	}
	must(os.WriteFile(filepath.Join(out, "e4set-results.txt"), []byte(b.String()), 0o644))
	fmt.Print(b.String())
}
