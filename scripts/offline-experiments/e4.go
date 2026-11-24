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
	"unicode"
)

// E4 (ledger C221 to C225): four novelty forms, each multiplied by the C218
// importance. See docs/evidence/offline-experiments-2026-11-24/README.md.

const (
	e4DFMax    = 0.01 // a word in more than 1% of arrivals so far is furniture
	e4Fast     = 2 * time.Hour
	e4SlowBack = 26 * time.Hour
	e4EchoCos  = 0.95
)

var (
	reURL    = regexp.MustCompile(`(?i)https?://\S+|www\.\S+`)
	reTagHdl = regexp.MustCompile(`[#@]\S+`)
)

func contentWords(c *capture) []string {
	lead := []rune(c.Lead)
	if len(lead) > 800 {
		lead = lead[:800]
	}
	t := strings.ToLower(c.Title + " " + string(lead))
	t = reURL.ReplaceAllString(t, " ")
	t = reTagHdl.ReplaceAllString(t, " ")
	words := strings.FieldsFunc(t, func(r rune) bool { return !unicode.IsLetter(r) })
	seen := map[string]bool{}
	var out []string
	for _, w := range words {
		if len([]rune(w)) >= 4 && !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

type e4Row struct {
	I, Na, Nb, Nc, NcShare, B, N10 float64
	Top                            string
	NewWords                       []string
	Story                          string
	Huang, Echo                    bool
	HuangEcho                      bool
}

type e4Form struct {
	Name string
	Get  func(r *e4Row) float64
}

func runE4(dir, out string) {
	must(os.MkdirAll(out, 0o755))
	d, _ := loadE1(dir)
	arr := d.arr
	n := len(arr)
	kept := keptSubjects(arr)
	day := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)

	// per-arrival cited-by-story lists, for the story-scoped form
	type citeRec struct {
		story *story
		pub   time.Time // version publish time
		made  time.Time // edge creation time
		arr   int
	}
	var cites []citeRec
	for _, s := range d.stories {
		for _, v := range s.Versions {
			for _, c := range d.cites[v.ID] {
				cites = append(cites, citeRec{s, v.Pub, c.Created, c.Arr})
			}
		}
	}
	hasSubj := func(i int, s string) bool {
		for _, k := range kept[i] {
			if k == s {
				return true
			}
		}
		return false
	}

	type mention struct {
		at     time.Time
		origin string
	}
	byKey := map[string][]mention{}
	subjArr := map[string][]int{}
	subjWords := map[string]map[string]bool{}
	wordDF := map[string]int{}
	rows := make([]e4Row, n)

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

	for i := range arr {
		a := &arr[i]
		t := a.Fetched
		r := &rows[i]
		r.Huang = isHuang(a)
		ks := kept[i]
		// C218 importance and the top subject
		bestR := -1
		for _, k := range ks {
			byKey[k] = append(byKey[k], mention{t, a.Origin})
			rs := distinct(byKey[k], t.Add(-3*time.Hour), t)
			r.I += math.Log2(1 + float64(rs))
			if rs > bestR {
				bestR, r.Top = rs, k
			}
		}
		words := contentWords(a)
		if t.Before(day) {
			// warm-up arrivals are not scored, but their state counts
		} else {
			// C218 form, for the side-by-side
			r.N10 = 1
			cs := make([]float64, 0, i)
			for j := 0; j < i; j++ {
				cs = append(cs, dot(a.Vec, arr[j].Vec))
			}
			sort.Sort(sort.Reverse(sort.Float64Slice(cs)))
			if len(cs) >= 10 {
				r.N10 = 1 - cs[9]
			}
			r.Echo = len(cs) > 0 && cs[0] >= e4EchoCos
			if r.Top != "" {
				// (a) subject-scoped sparsity
				best := -2.0
				for _, j := range subjArr[r.Top] {
					if c := dot(a.Vec, arr[j].Vec); c > best {
						best = c
					}
				}
				r.Na = 1
				if best > -2 {
					r.Na = 1 - best
				}
				// (b) story-scoped sparsity
				cnt := map[*story]map[int]bool{}
				for _, c := range cites {
					if c.pub.After(t) || c.made.After(t) || arr[c.arr].Fetched.After(t) || !hasSubj(c.arr, r.Top) {
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
					if len(m) > coverN || (len(m) == coverN && cover != nil && latest.Pub.After(coverPub)) || (len(m) == coverN && cover == nil && coverN >= 2) {
						cover, coverN, coverPub = s, len(m), latest.Pub
					}
				}
				r.Nb = 1
				if cover != nil {
					r.Story = trunc(cover.Title, 70)
					bc := -2.0
					for _, p := range d.passages[latestAt(cover, t).ID] {
						if c := dot(a.Vec, p); c > bc {
							bc = c
						}
					}
					if bc > -2 {
						r.Nb = 1 - bc
					}
				}
				// (c) new words
				elig := 0
				for _, w := range words {
					if float64(wordDF[w])/float64(i) <= e4DFMax {
						elig++
						if !subjWords[r.Top][w] {
							r.Nc++
							r.NewWords = append(r.NewWords, w)
						}
					}
				}
				if elig > 0 {
					r.NcShare = r.Nc / float64(elig)
				}
				// (d) burst
				ms := byKey[r.Top]
				fast := distinct(ms, t.Add(-e4Fast), t)
				slow := distinct(ms, t.Add(-e4SlowBack), t.Add(-e4Fast))
				r.B = float64(fast) / math.Max(float64(slow)/12, 0.25)
			}
		}
		// update state with this arrival
		for _, k := range ks {
			subjArr[k] = append(subjArr[k], i)
			if subjWords[k] == nil {
				subjWords[k] = map[string]bool{}
			}
			for _, w := range words {
				subjWords[k][w] = true
			}
		}
		for _, w := range words {
			wordDF[w]++
		}
	}
	// Huang echoes: near-copy (cos >= 0.95) of an earlier Huang-pattern arrival
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

	forms := []e4Form{
		{"I alone", func(r *e4Row) float64 { return r.I }},
		{"C218 I x N(10th neighbour, corpus-wide)", func(r *e4Row) float64 { return r.I * r.N10 }},
		{"(a) I x subject-scoped sparsity", func(r *e4Row) float64 { return r.I * r.Na }},
		{"(b) I x story-scoped sparsity", func(r *e4Row) float64 { return r.I * r.Nb }},
		{"(c) I x new words (count)", func(r *e4Row) float64 { return r.I * r.Nc }},
		{"(c') I x new words (share)", func(r *e4Row) float64 { return r.I * r.NcShare }},
		{"(d) I x burst", func(r *e4Row) float64 { return r.I * r.B }},
	}
	bin := func(i int) int { // hour ending X is bin X-1 (arrivals in (X-1:00, X:00])
		return int((arr[i].Fetched.Add(-time.Nanosecond).Sub(day)) / time.Hour)
	}
	byBin := map[int][]int{}
	for i := range arr {
		if !arr[i].Fetched.Before(day) {
			byBin[bin(i)] = append(byBin[bin(i)], i)
		}
	}
	rankIn := func(idx []int, f e4Form) map[int]int {
		rk := map[int]int{}
		for _, i := range idx {
			v := f.Get(&rows[i])
			c := 1
			for _, j := range idx {
				if f.Get(&rows[j]) > v+1e-12 {
					c++
				}
			}
			rk[i] = c
		}
		return rk
	}
	var b strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }
	nsub := 0
	for i := range arr {
		if !arr[i].Fetched.Before(day) && rows[i].Top != "" {
			nsub++
		}
	}
	nDay := 0
	for _, idx := range byBin {
		nDay += len(idx)
	}
	w("E4: %d arrivals, %d on 2026-11-23 of which %d have a top subject; hour bins are (X-1:00, X:00]\n\n", n, nDay, nsub)

	hourOf := func(end int) []int { return byBin[end-1] }
	// 1. Huang rank per form per hour (the whole day)
	w("== Best Huang-pattern arrival's rank in its hour, by form, for every hour of 11-23 with one (n Huang | N arrivals) ==\n")
	w("hour end |")
	for _, f := range forms {
		w(" %s |", f.Name)
	}
	w("\n")
	bestRank := map[string]map[int]int{}
	for _, f := range forms {
		bestRank[f.Name] = map[int]int{}
	}
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
			bestRank[f.Name][end] = best
			w(" %d |", best)
		}
		w("\n")
	}
	// day-wide rank share of Huang arrivals
	w("\n== Huang-pattern arrivals of the whole day ranked against all %d arrivals of the day: best rank and median rank share ==\n", nDay)
	var dayIdx []int
	for _, idx := range byBin {
		dayIdx = append(dayIdx, idx...)
	}
	for _, f := range forms {
		rk := rankIn(dayIdx, f)
		best := 1 << 30
		var shares []float64
		for _, i := range dayIdx {
			if rows[i].Huang {
				if rk[i] < best {
					best = rk[i]
				}
				shares = append(shares, float64(rk[i])/float64(len(dayIdx)))
			}
		}
		sort.Float64s(shares)
		w("%s: best rank %d, median rank share %.3f (n %d)\n", f.Name, best, shares[len(shares)/2], len(shares))
	}

	// 2. the primary hour in detail
	prim := hourOf(19)
	w("\n== PRIMARY hour (18:00, 19:00]: %d arrivals; Huang-pattern arrivals ==\n", len(prim))
	for _, f := range forms[2:] {
		rk := rankIn(prim, f)
		var rs []int
		for _, i := range prim {
			if rows[i].Huang {
				rs = append(rs, rk[i])
			}
		}
		sort.Ints(rs)
		w("%s: Huang ranks %v\n", f.Name, rs)
	}
	w("\nper Huang arrival: time | origin | I | top subject | Na | Nb (covering story) | Nc (share) | B | echo of an earlier Huang arrival | new eligible words\n")
	for _, i := range prim {
		r := &rows[i]
		if !r.Huang {
			continue
		}
		w("%s | %s | %.1f | %s | %.3f | %.3f (%s) | %.0f (%.2f) | %.2f | %v | %s\n", arr[i].Fetched.Format("15:04"), arr[i].Origin, r.I, r.Top, r.Na, r.Nb, r.Story, r.Nc, r.NcShare, r.B, r.HuangEcho, strings.Join(firstN(r.NewWords, 8), ","))
	}
	// reinstat* firing
	w("\ndoes a reinstat* word fire as new? (Huang-pattern arrivals of the day with a new eligible word starting 'reinstat' / 'return' / 'restor'): ")
	for _, i := range dayIdx {
		r := &rows[i]
		if !r.Huang {
			continue
		}
		for _, nw := range r.NewWords {
			if strings.HasPrefix(nw, "reinstat") || strings.HasPrefix(nw, "return") || strings.HasPrefix(nw, "restor") {
				w("[%s %s: %s] ", arr[i].Fetched.Format("15:04"), arr[i].Origin, nw)
			}
		}
	}
	w("\n")

	// 3. turn hour
	turn := hourOf(15)
	w("\n== TURN hour (14:00, 15:00]: %d arrivals; Huang-pattern arrivals, per arrival ==\n", len(turn))
	for _, f := range forms[2:] {
		rk := rankIn(turn, f)
		var rs []int
		for _, i := range turn {
			if rows[i].Huang {
				rs = append(rs, rk[i])
			}
		}
		sort.Ints(rs)
		w("%s: Huang ranks %v\n", f.Name, rs)
	}
	w("\nper Huang arrival of the turn hour: time | origin | I | top subject | Na | Nb (covering story) | Nc (share) | B | new eligible words\n")
	for _, i := range turn {
		r := &rows[i]
		if r.Huang {
			w("%s | %s | %.1f | %s | %.3f | %.3f (%s) | %.0f (%.2f) | %.2f | %s\n", arr[i].Fetched.Format("15:04"), arr[i].Origin, r.I, r.Top, r.Na, r.Nb, r.Story, r.Nc, r.NcShare, r.B, strings.Join(firstN(r.NewWords, 8), ","))
		}
	}

	// 4. echoes
	w("\n== ECHOES (nearest earlier arrival at cosine >= %.2f), rank share within the arrival's hour ==\n", e4EchoCos)
	nEcho := 0
	for _, i := range dayIdx {
		if rows[i].Echo {
			nEcho++
		}
	}
	w("echoes on 11-23: %d of %d\n", nEcho, nDay)
	for _, f := range forms {
		var shares []float64
		for end := 1; end <= 24; end++ {
			idx := hourOf(end)
			rk := rankIn(idx, f)
			for _, i := range idx {
				if rows[i].Echo {
					shares = append(shares, float64(rk[i])/float64(len(idx)))
				}
			}
		}
		sort.Float64s(shares)
		w("%s: median echo rank share %.3f (n %d); 10th percentile %.3f\n", f.Name, shares[len(shares)/2], len(shares), shares[len(shares)/10])
	}
	w("\nHuang-pattern echoes of an earlier Huang arrival (a repost of the petition or reinstatement text): count and median in-hour rank share by form\n")
	for _, f := range forms {
		var shares []float64
		for end := 1; end <= 24; end++ {
			idx := hourOf(end)
			rk := rankIn(idx, f)
			for _, i := range idx {
				if rows[i].HuangEcho {
					shares = append(shares, float64(rk[i])/float64(len(idx)))
				}
			}
		}
		sort.Float64s(shares)
		if len(shares) > 0 {
			w("%s: n %d, median rank share %.3f, best %.3f\n", f.Name, len(shares), shares[len(shares)/2], shares[0])
		}
	}

	// 5. top tens
	for _, end := range []int{19, 15, 3, 9, 21} {
		idx := hourOf(end)
		for _, f := range forms[1:5] {
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
				w("%d | %.2f | H=%v echo=%v | %s | %s | %s\n", k+1, f.Get(&rows[i]), rows[i].Huang, rows[i].Echo, arr[i].Origin, rows[i].Top, trunc(tt, 90))
			}
		}
	}
	must(os.WriteFile(filepath.Join(out, "e4-results.txt"), []byte(b.String()), 0o644))
	fmt.Print(b.String())
}

func latestAt(s *story, t time.Time) *version {
	var out *version
	for k := range s.Versions {
		if s.Versions[k].Pub.After(t) {
			break
		}
		out = &s.Versions[k]
	}
	return out
}
