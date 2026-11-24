package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	e2Seed  = 20261124
	e2Draws = 100
	e2Boots = 1000
)

var e2Fractions = []float64{0.10, 0.25, 0.50, 0.75, 1.00}

type e2Counts struct{ Any, Two, Three, Arrivals int }

// e2Count regroups the arrivals whose unit is kept and counts happenings by
// distinct independent origins (always the arrival's origin, whatever the unit dropped).
func e2Count(arr []capture, keep []bool, unit func(c *capture) string, units map[string]int) e2Counts {
	var idx []int
	for i := range arr {
		if keep[units[unit(&arr[i])]] {
			idx = append(idx, i)
		}
	}
	gs := groupArrivals(arr, idx)
	var out e2Counts
	out.Arrivals = len(idx)
	for _, g := range gs {
		seen := map[string]bool{}
		for _, m := range g.Members {
			seen[arr[m].Origin] = true
		}
		out.Any++
		if len(seen) >= 2 {
			out.Two++
		}
		if len(seen) >= 3 {
			out.Three++
		}
	}
	return out
}

func slopeOf(xs, ys []float64) float64 {
	var mx, my float64
	for i := range xs {
		mx += xs[i]
		my += ys[i]
	}
	mx /= float64(len(xs))
	my /= float64(len(xs))
	var sxy, sxx float64
	for i := range xs {
		sxy += (xs[i] - mx) * (ys[i] - my)
		sxx += (xs[i] - mx) * (xs[i] - mx)
	}
	return sxy / sxx
}

func pct(v []float64, p float64) float64 {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	return s[int(p*float64(len(s)-1))]
}

func runE2(dir, out string) {
	must(os.MkdirAll(out, 0o755))
	arr, dropped := loadArrivals(dir)
	var b strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }
	w("E2 replay: %d arrivals after the copy check (%d dropped), %d draws a fraction, seed %d\n\n", len(arr), dropped, e2Draws, e2Seed)
	type unitDef struct {
		name string
		fn   func(c *capture) string
	}
	for _, u := range []unitDef{
		{"origin (primary)", func(c *capture) string { return c.Origin }},
		{"feed (source_id)", func(c *capture) string { return c.SourceID }},
	} {
		units := map[string]int{}
		for i := range arr {
			k := u.fn(&arr[i])
			if _, ok := units[k]; !ok {
				units[k] = len(units)
			}
		}
		n := len(units)
		w("== unit dropped: %s; %d units ==\n", u.name, n)
		res := map[float64][]e2Counts{}
		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, 6)
		for fi, f := range e2Fractions {
			draws := e2Draws
			if f == 1 {
				draws = 1
			}
			for dr := 0; dr < draws; dr++ {
				wg.Add(1)
				sem <- struct{}{}
				go func(fi int, f float64, dr int) {
					defer wg.Done()
					defer func() { <-sem }()
					keep := make([]bool, n)
					if f == 1 {
						for i := range keep {
							keep[i] = true
						}
					} else {
						r := rand.New(rand.NewSource(int64(e2Seed + fi*100003 + dr)))
						perm := r.Perm(n)
						k := int(math.Round(f * float64(n)))
						for _, p := range perm[:k] {
							keep[p] = true
						}
					}
					c := e2Count(arr, keep, u.fn, units)
					mu.Lock()
					res[f] = append(res[f], c)
					mu.Unlock()
				}(fi, f, dr)
			}
		}
		wg.Wait()
		type series struct {
			name string
			get  func(c e2Counts) float64
		}
		for _, s := range []series{
			{"happenings with 2 or more independent origins (the claim)", func(c e2Counts) float64 { return float64(c.Two) }},
			{"happenings of any size (singletons included)", func(c e2Counts) float64 { return float64(c.Any) }},
			{"happenings with 3 or more origins", func(c e2Counts) float64 { return float64(c.Three) }},
			{"arrivals kept (check: should scale like f)", func(c e2Counts) float64 { return float64(c.Arrivals) }},
		} {
			w("\n-- %s --\nf | units kept | mean | 2.5%% | 97.5%% | mean arrivals\n", s.name)
			var lx, ly []float64
			byF := map[float64][]float64{}
			for _, f := range e2Fractions {
				var vals []float64
				var arrMean float64
				for _, c := range res[f] {
					vals = append(vals, s.get(c))
					arrMean += float64(c.Arrivals)
				}
				arrMean /= float64(len(res[f]))
				byF[f] = vals
				m := mean(vals)
				w("%.2f | %d | %.2f | %.1f | %.1f | %.0f\n", f, int(math.Round(f*float64(n))), m, pct(vals, .025), pct(vals, .975), arrMean)
				lx = append(lx, math.Log(f))
				ly = append(ly, math.Log(m))
			}
			b0 := slopeOf(lx, ly)
			// bootstrap of draws within each f
			r := rand.New(rand.NewSource(e2Seed))
			var slopes []float64
			for k := 0; k < e2Boots; k++ {
				var by []float64
				for _, f := range e2Fractions {
					vals := byF[f]
					var m float64
					for j := 0; j < len(vals); j++ {
						m += vals[r.Intn(len(vals))]
					}
					by = append(by, math.Log(m/float64(len(vals))))
				}
				slopes = append(slopes, slopeOf(lx, by))
			}
			// local slope between f=0.5 and f=1
			half := math.Log(mean(byF[1.0])/mean(byF[0.5])) / math.Log(2)
			w("exponent b (log mean N on log f over the five fractions): %.3f, 95%% bootstrap interval [%.3f, %.3f]; between f=0.5 and f=1: %.3f\n", b0, pct(slopes, .025), pct(slopes, .975), half)
		}
		w("\n")
	}
	must(os.WriteFile(filepath.Join(out, "e2-results.txt"), []byte(b.String()), 0o644))
	fmt.Print(b.String())
}

func mean(v []float64) float64 {
	var s float64
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}
