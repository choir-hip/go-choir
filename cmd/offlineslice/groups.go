package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/offlineslice"
)

type slim struct {
	cos    float64
	oracle string
}

func loadSlim(path string) map[string]slim {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	out := map[string]slim{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		var r offlineslice.ResultB
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			panic(err)
		}
		s := slim{cos: -1, oracle: r.Oracle}
		if len(r.Stories) > 0 {
			s.cos = r.Stories[0].Cos
		}
		out[r.ID] = s
	}
	return out
}

func dec(v []float64) string {
	if len(v) == 0 {
		return "n=0"
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	q := func(p float64) float64 { return s[int(p*float64(len(s)-1))] }
	return fmt.Sprintf("n=%d p10 %.3f p25 %.3f p50 %.3f p75 %.3f p90 %.3f", len(s), q(.1), q(.25), q(.5), q(.75), q(.9))
}

func dist(v []int) string {
	if len(v) == 0 {
		return "n=0"
	}
	s := append([]int(nil), v...)
	sort.Ints(s)
	sum := 0
	for _, x := range s {
		sum += x
	}
	q := func(p float64) int { return s[int(p*float64(len(s)-1))] }
	return fmt.Sprintf("n=%d mean %.2f p50 %d p90 %d p99 %d max %d", len(s), float64(sum)/float64(len(s)), q(.5), q(.9), q(.99), s[len(s)-1])
}

func pc(a, b int) string {
	if b == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(a)/float64(b))
}

func groupsMain(dataDir, resultsPath, outDir, keyDir string, seed int64, n int) {
	caps, err := offlineslice.ReadCaptures(filepath.Join(dataDir, "captures.csv"))
	must(err)
	vecs, err := offlineslice.ReadVectors(filepath.Join(dataDir, "vectors.csv"))
	must(err)
	leads, _, err := offlineslice.ReadLeads(filepath.Join(dataDir, "bodies.bin"))
	must(err)
	urls := map[string]string{}
	must(readCSV(filepath.Join(dataDir, "urls.csv"), func(r []string) { urls[r[0]] = r[1] }))
	wire := map[string]string{}
	if f, err := os.Open(filepath.Join(dataDir, "wire.tsv")); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if a, b, ok := strings.Cut(sc.Text(), "\t"); ok {
				wire[a] = b
			}
		}
		f.Close()
	}
	sd, err := offlineslice.ReadStories(dataDir)
	must(err)
	sl := loadSlim(resultsPath)
	for i := range caps {
		caps[i].Vec = vecs[caps[i].ID]
		caps[i].Lead = leads[caps[i].TextHash]
	}
	kept, _ := offlineslice.Dedupe(caps)
	var arr []offlineslice.Capture
	for _, c := range kept {
		if _, ok := sl[c.ID]; ok && c.Vec != nil {
			arr = append(arr, c)
		}
	}
	day := func(c offlineslice.Capture) string { return c.Fetched.UTC().Format("2006-01-02") }
	early := func(d string) bool { return d <= "2026-11-17" }
	fmt.Printf("arrivals %d (wire-tagged: %d)\n\n", len(arr), len(wire))

	// 1. T from days 1-3
	var pos, neg []float64
	for _, c := range arr {
		if !early(day(c)) {
			continue
		}
		s := sl[c.ID]
		if s.oracle == "fed" {
			pos = append(pos, s.cos)
		} else {
			neg = append(neg, s.cos)
		}
	}
	T, tpr, fpr := offlineslice.BestThreshold(pos, neg)
	fmt.Println("== 1. unmatched threshold T (days 1-3)")
	fmt.Println("nearest-story cosine, arrivals later cited by a story that existed (fed):", dec(pos))
	fmt.Println("nearest-story cosine, all other arrivals (cited only by a later story, or by none):", dec(neg))
	fmt.Printf("T = %.3f (Youden J = %.3f): fed matched %s, others matched %s; fed wrongly called unmatched: %.1f%% (%d of %d)\n", T, tpr-fpr,
		pcf(tpr), pcf(fpr), 100*(1-tpr), int(math.Round((1-tpr)*float64(len(pos)))), len(pos))
	type cell struct{ n, un int }
	byDay, byClass := map[string]*cell{}, map[string]*cell{}
	var un []int // indexes into arr
	for i, c := range arr {
		d, k := day(c), offlineslice.Class(c)
		for _, m := range []struct {
			mp map[string]*cell
			k  string
		}{{byDay, d}, {byClass, k}} {
			if m.mp[m.k] == nil {
				m.mp[m.k] = &cell{}
			}
			m.mp[m.k].n++
		}
		if sl[c.ID].cos < T {
			byDay[d].un++
			byClass[k].un++
			un = append(un, i)
		}
	}
	fmt.Println("unmatched share by day:")
	for _, d := range sortedKeys(byDay) {
		fmt.Printf("  %s %5d arrivals, %5d unmatched = %s\n", d, byDay[d].n, byDay[d].un, pc(byDay[d].un, byDay[d].n))
	}
	fmt.Println("unmatched share by class (week):")
	for _, k := range sortedKeys(byClass) {
		fmt.Printf("  %-9s %5d arrivals, %5d unmatched = %s\n", k, byClass[k].n, byClass[k].un, pc(byClass[k].un, byClass[k].n))
	}
	var late int
	for _, i := range un {
		if !early(day(arr[i])) {
			late++
		}
	}
	fmt.Printf("unmatched on days 4-7: %d\n\n", late)

	// 2. G from days 1-3, both modes
	items := make([]offlineslice.GItem, len(un))
	for k, i := range un {
		items[k] = offlineslice.GItem{At: arr[i].Fetched, Vec: arr[i].Vec}
	}
	nEarly := 0
	for _, i := range un {
		if early(day(arr[i])) {
			nEarly++
		}
	}
	fmt.Println("== 2. grouping (48 h close), G chosen on days 1-3 (the first", nEarly, "unmatched arrivals)")
	chosen := map[offlineslice.LinkMode]float64{}
	for _, mode := range []offlineslice.LinkMode{offlineslice.LinkCentroid, offlineslice.LinkAnyMember} {
		var xs, ys []float64
		line := ""
		for g := 0.60; g <= 0.951; g += 0.01 {
			gs := offlineslice.GroupArrivals(items[:nEarly], g, mode, 48*time.Hour)
			xs, ys = append(xs, g), append(ys, float64(len(gs)))
			if int(math.Round(g*100))%5 == 0 {
				line += fmt.Sprintf(" %.2f:%d", g, len(gs))
			}
		}
		k := offlineslice.Knee(xs, ys)
		chosen[mode] = xs[k]
		fmt.Printf("%s: groups opened on days 1-3 by G:%s  -> knee at G=%.2f (%d groups)\n", modeName(mode), line, xs[k], int(ys[k]))
	}
	fmt.Println()

	// 3-5 report on the centroid mode (primary) and any-member (secondary)
	origin := make([]string, len(arr))
	for i, c := range arr {
		origin[i] = offlineslice.OriginOf(c, urls[c.ID], wire[c.ID])
	}
	endOfData := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	for _, mode := range []offlineslice.LinkMode{offlineslice.LinkCentroid, offlineslice.LinkAnyMember} {
		G := chosen[mode]
		gs := offlineslice.GroupArrivals(items, G, mode, 48*time.Hour)
		fmt.Printf("== 3-5. %s, G=%.2f, T=%.3f: %d groups from %d unmatched arrivals (all days)\n", modeName(mode), G, T, len(gs), len(un))
		type gi struct {
			g       offlineslice.Group
			ats     []time.Time
			origins []string
			final   int
		}
		var all []gi
		for _, g := range gs {
			x := gi{g: g}
			seen := map[string]bool{}
			for _, m := range g.Members {
				x.ats = append(x.ats, items[m].At)
				o := origin[un[m]]
				x.origins = append(x.origins, o)
				seen[o] = true
			}
			x.final = len(seen)
			all = append(all, x)
		}
		var d47 []gi
		for _, x := range all {
			if !early(x.g.Opened.UTC().Format("2006-01-02")) {
				d47 = append(d47, x)
			}
		}
		fmt.Printf("groups opened on days 4-7: %d\n", len(d47))
		perDay := map[string]int{}
		var sizes, origs []int
		for _, x := range d47 {
			perDay[x.g.Opened.UTC().Format("2006-01-02")]++
			sizes = append(sizes, len(x.g.Members))
			origs = append(origs, x.final)
		}
		fmt.Println("  opened per day:", perDay)
		fmt.Println("  group size:    ", dist(sizes))
		fmt.Println("  distinct origins:", dist(origs))
		// size histogram
		hist := map[string]int{}
		for _, s := range sizes {
			k := "1"
			switch {
			case s >= 50:
				k = "50+"
			case s >= 10:
				k = "10-49"
			case s >= 5:
				k = "5-9"
			case s >= 2:
				k = "2-4"
			}
			hist[k]++
		}
		fmt.Println("  size buckets:", hist)
		fmt.Println("  groups reaching N distinct origins within H of opening, by opening day (a '*' = the horizon runs past the data's end for the days after it; counts shown are what was observed):")
		for _, N := range []int{2, 3, 5} {
			for _, H := range []time.Duration{6 * time.Hour, 24 * time.Hour, 48 * time.Hour} {
				row := map[string]int{}
				tot, totObs, obsGroups := 0, 0, 0
				for _, x := range d47 {
					d := x.g.Opened.UTC().Format("2006-01-02")
					observable := !x.g.Opened.Add(H).After(endOfData)
					if observable {
						obsGroups++
					}
					if dur, ok := offlineslice.ReachTime(x.ats, x.origins, N); ok && dur <= H {
						row[d]++
						tot++
						if observable {
							totObs++
						}
					}
				}
				var days []string
				for _, d := range []string{"2026-11-18", "2026-11-19", "2026-11-20", "2026-11-21"} {
					days = append(days, fmt.Sprintf("%d", row[d]))
				}
				fmt.Printf("    >=%d origins within %2dh: per day (11-18..21) %s; total %d; of groups with the full horizon in the data (%d): %d\n", N, int(H.Hours()), strings.Join(days, " / "), tot, obsGroups, totObs)
			}
		}
		// unmatched arrivals of days 4-7 ending in groups that never reach 2 origins
		inG := make([]int, len(items)) // group index per unmatched item
		for gi2, x := range all {
			for _, m := range x.g.Members {
				inG[m] = gi2
			}
		}
		nLate, nSolo := 0, 0
		for k, i := range un {
			if early(day(arr[i])) {
				continue
			}
			nLate++
			if all[inG[k]].final < 2 {
				nSolo++
			}
		}
		fmt.Printf("  unmatched arrivals of days 4-7 in groups that never reach 2 origins: %d of %d = %s\n", nSolo, nLate, pc(nSolo, nLate))
		// largest 20
		sort.SliceStable(d47, func(a, b int) bool { return len(d47[a].g.Members) > len(d47[b].g.Members) })
		fmt.Println("  largest 20 groups opened on days 4-7:")
		for i := 0; i < 20 && i < len(d47); i++ {
			x := d47[i]
			var ts []string
			for _, m := range x.g.Members {
				if len(ts) == 3 {
					break
				}
				ts = append(ts, headline(arr[un[m]]))
			}
			fmt.Printf("    %d members, %d origins, opened %s: %s\n", len(x.g.Members), x.final, x.g.Opened.UTC().Format("01-02 15:04"), strings.Join(ts, " | "))
		}
		// 5. cross-check on groups with >=3 origins
		var three []gi
		for _, x := range d47 {
			if x.final >= 3 {
				three = append(three, x)
			}
		}
		var lags []float64
		after, existingOnly, never := 0, 0, 0
		for _, x := range three {
			bestLag := time.Duration(-1)
			exist := false
			for _, m := range x.g.Members {
				for _, sid := range sd.Cites[arr[un[m]].ID] {
					f, ok := sd.First[sid]
					if !ok {
						continue
					}
					if f.After(x.g.Opened) {
						if l := f.Sub(x.g.Opened); bestLag < 0 || l < bestLag {
							bestLag = l
						}
					} else {
						exist = true
					}
				}
			}
			switch {
			case bestLag >= 0:
				after++
				lags = append(lags, bestLag.Hours())
			case exist:
				existingOnly++
			default:
				never++
			}
		}
		fmt.Printf("  cross-check, %d groups with >=3 origins (days 4-7): a story first published after the group opened cites a member: %d; only a story that already existed cites a member: %d; no story cites any member: %d\n", len(three), after, existingOnly, never)
		fmt.Println("  hours from group opening to that story's first version:", dec(lags))
		fmt.Println()

		if mode != offlineslice.LinkCentroid {
			continue
		}
		// 6. reader folder from the centroid grouping
		rng := rand.New(rand.NewSource(seed))
		rng.Shuffle(len(three), func(i, j int) { three[i], three[j] = three[j], three[i] })
		if n > len(three) {
			n = len(three)
		}
		pick := three[:n]
		need := map[string]string{}
		hashes := map[string]string{}
		for _, c := range caps {
			hashes[c.ID] = c.TextHash
		}
		for _, x := range pick {
			for k, m := range x.g.Members {
				if k >= 12 {
					break
				}
				id := arr[un[m]].ID
				need[hashes[id]] = id
			}
		}
		texts := readTexts(filepath.Join(dataDir, "bodies.bin"), need)
		must(os.MkdirAll(filepath.Join(outDir, "groups"), 0o755))
		must(os.MkdirAll(keyDir, 0o755))
		type krow struct {
			Case       string   `json:"case"`
			Members    int      `json:"members"`
			Origins    int      `json:"origins"`
			Opened     string   `json:"opened"`
			IDs        []string `json:"member_ids"`
			OriginOf   []string `json:"member_origins"`
			StoryAfter string   `json:"story_after_opening"`
		}
		var keys []krow
		for k, x := range pick {
			id := fmt.Sprintf("%02d", k+1)
			var sb strings.Builder
			fmt.Fprintf(&sb, "# Group %s\n\n%d items, earliest first", id, len(x.g.Members))
			if len(x.g.Members) > 12 {
				fmt.Fprintf(&sb, "; the first 12 are shown, %d more are not", len(x.g.Members)-12)
			}
			sb.WriteString(".\n\n")
			kr := krow{Case: id, Members: len(x.g.Members), Origins: x.final, Opened: x.g.Opened.UTC().Format(time.RFC3339)}
			for j, m := range x.g.Members {
				if j >= 12 {
					break
				}
				c := arr[un[m]]
				text := texts[c.ID]
				if rs := []rune(text); len(rs) > 600 {
					text = string(rs[:600])
				}
				title := strings.TrimSpace(c.Title)
				if title == "" {
					title = "(a social post, no title)"
				}
				fmt.Fprintf(&sb, "## Item %d\n\n- Title: %s\n- Outlet: %s\n- Time: %s\n\n%s\n\n", j+1, title, origin[un[m]], c.Fetched.UTC().Format("2006-01-02 15:04 UTC"), text)
				kr.IDs = append(kr.IDs, c.ID)
				kr.OriginOf = append(kr.OriginOf, origin[un[m]])
			}
			must(os.WriteFile(filepath.Join(outDir, "groups", id+".md"), []byte(sb.String()), 0o644))
			keys = append(keys, kr)
		}
		kb, _ := json.MarshalIndent(map[string]any{"seed": seed, "T": T, "G": G, "mode": "centroid", "groups": keys}, "", " ")
		must(os.WriteFile(filepath.Join(keyDir, "key.json"), kb, 0o644))
		var tpl strings.Builder
		tpl.WriteString("# Answers\n\n")
		for k := range pick {
			fmt.Fprintf(&tpl, "## %02d\n\nKIND: ONE / MOSTLY ONE (odd ones: item numbers) / THEME / NOISE\nGENERAL_PAPER_NEWS: yes / no / marginal\nNOTE (one line, optional): \n\n", k+1)
		}
		must(os.WriteFile(filepath.Join(outDir, "answers.md"), []byte(tpl.String()), 0o644))
		instr := fmt.Sprintf(`# Reading task: are these groups of news items one happening?

groups/01.md to groups/%02d.md are groups of items that a program put together because they read alike. Each item shows its
title (a social post has none), its outlet or account, its time, and the first 600 characters of its text. A group shows up to
12 items, earliest first; if it has more, the file says how many were left out. You did not write the program; do not guess how it works.

For each group, read the items, then fill in answers.md:

1. KIND, exactly one of:
   - ONE: every item reports the same affair (the same happening, or successive reports of it).
   - MOSTLY ONE: most items report one affair; name the odd ones by item number.
   - THEME: the items share actors or a topic but report different happenings (for example different crimes in one city, different bills in one legislature).
   - NOISE: no common affair; or the items are page furniture, spam, feeds of listings or automatic posts with no news in them.
2. GENERAL_PAPER_NEWS: is the group's affair news a general newspaper would cover? yes / no / marginal.
   (Judge the affair itself, not whether it is well reported.)

Judge only from what is shown. Do not look anything up. Work in numerical order.
`, len(pick))
		must(os.WriteFile(filepath.Join(outDir, "INSTRUCTIONS.md"), []byte(instr), 0o644))
		fmt.Printf("wrote %d groups (of %d with >=3 origins) to %s; key in %s\n\n", len(pick), len(three), outDir, keyDir)
	}
}

func headline(c offlineslice.Capture) string {
	t := strings.TrimSpace(c.Title)
	if t == "" {
		t = strings.Join(strings.Fields(c.Lead), " ")
	}
	if r := []rune(t); len(r) > 90 {
		t = string(r[:90]) + "…"
	}
	return t
}

func modeName(m offlineslice.LinkMode) string {
	if m == offlineslice.LinkAnyMember {
		return "any-member (single link)"
	}
	return "centroid"
}

func pcf(x float64) string { return fmt.Sprintf("%.1f%%", 100*x) }

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func sortedKeys[V any](m map[string]V) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func readCSV(path string, fn func([]string)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	r := csv.NewReader(bufio.NewReaderSize(f, 1<<20))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	for {
		rec, err := r.Read()
		if err != nil {
			return nil
		}
		fn(rec)
	}
}
