package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/offlineslice"
)

// Rung 2's grouping: the 0c rule (T from days 1-3, centroid, G = 0.84) with chaining bounded
// (a group takes no member once it is 48 h old or holds 25 members) and relay accounts collapsed
// to the outlet they relay. Writes the groups that reach 3 distinct origins for naming.
const (
	g2G       = 0.84
	g2MaxAge  = 48 * time.Hour
	g2MaxSize = 25
)

type g2Member struct {
	ID     string    `json:"id"`
	Time   time.Time `json:"time"`
	Title  string    `json:"title"`
	Origin string    `json:"origin"`
	Text   string    `json:"text"`
}

type g2Group struct {
	ID              string     `json:"id"`
	Opened          time.Time  `json:"opened"`
	Reach3          time.Time  `json:"reach3"`
	Size            int        `json:"size"`
	Origins         int        `json:"origins"`
	Members         []g2Member `json:"members"`           // the first 12, earliest first
	StoryAfterHours float64    `json:"story_after_hours"` // hours from opening to the first version of a story that cites a member and began after the opening; -1 none
	StoryStatus     string     `json:"story_status"`      // after | existing_only | none
}

func groups2Main(dataDir, resultsPath, outDir string) {
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
	var pos, neg []float64
	for _, c := range arr {
		if early(day(c)) {
			if s := sl[c.ID]; s.oracle == "fed" {
				pos = append(pos, s.cos)
			} else {
				neg = append(neg, s.cos)
			}
		}
	}
	T, _, _ := offlineslice.BestThreshold(pos, neg)
	var un []int
	for i, c := range arr {
		if sl[c.ID].cos < T {
			un = append(un, i)
		}
	}
	items := make([]offlineslice.GItem, len(un))
	for k, i := range un {
		items[k] = offlineslice.GItem{At: arr[i].Fetched, Vec: arr[i].Vec}
	}
	// outlet labels for relay hashtags: the first label of every outlet a non-social arrival or a linked article has
	base := make([]string, len(arr))
	labels := map[string]string{}
	for i, c := range arr {
		base[i] = offlineslice.OriginOf(c, urls[c.ID], wire[c.ID])
		if !strings.HasPrefix(c.SourceID, "social:") || !strings.Contains(base[i], "/") {
			if lab, _, ok := strings.Cut(base[i], "."); ok && !strings.HasPrefix(base[i], "wire:") {
				labels[strings.ToLower(lab)] = base[i]
			}
		}
	}
	origin := make([]string, len(arr))
	relayed := 0
	for i, c := range arr {
		origin[i] = offlineslice.OriginOfRelays(c, urls[c.ID], wire[c.ID], labels)
		if origin[i] != base[i] {
			relayed++
		}
	}
	fmt.Printf("T=%.3f G=%.2f; unmatched %d of %d arrivals; relay hashtags changed the origin of %d arrivals\n", T, g2G, len(un), len(arr), relayed)

	build := func(maxAge time.Duration, maxSize int, org []string) (gs []offlineslice.Group, stats string, three []g2Group) {
		gs = offlineslice.GroupArrivalsCapped(items, g2G, offlineslice.LinkCentroid, 48*time.Hour, maxAge, maxSize)
		return gs, "", nil
	}
	_ = build
	uncapped := offlineslice.GroupArrivalsCapped(items, g2G, offlineslice.LinkCentroid, 48*time.Hour, 0, 0)
	gs := offlineslice.GroupArrivalsCapped(items, g2G, offlineslice.LinkCentroid, 48*time.Hour, g2MaxAge, g2MaxSize)
	maxU, maxC := 0, 0
	for _, g := range uncapped {
		if len(g.Members) > maxU {
			maxU = len(g.Members)
		}
	}
	atCap := 0
	for _, g := range gs {
		if len(g.Members) > maxC {
			maxC = len(g.Members)
		}
		if len(g.Members) >= g2MaxSize {
			atCap++
		}
	}
	fmt.Printf("groups: uncapped %d (largest %d); capped %d (largest %d; %d hit the size cap)\n\n", len(uncapped), maxU, len(gs), maxC, atCap)

	type gi struct {
		g       offlineslice.Group
		ats     []time.Time
		origins []string
		final   int
	}
	var all []gi
	openedByDay := map[string]int{}
	arrByDay := map[string]int{}
	for _, c := range arr {
		arrByDay[day(c)]++
	}
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
		openedByDay[g.Opened.UTC().Format("2006-01-02")]++
	}
	fmt.Println("groups opened per day and per 1,000 arrivals:")
	var days []string
	for d := range arrByDay {
		days = append(days, d)
	}
	sort.Strings(days)
	var e12, l47 float64
	var n12, n47 int
	for _, d := range days {
		per := 1000 * float64(openedByDay[d]) / float64(arrByDay[d])
		fmt.Printf("  %s %5d groups, %6.1f per 1,000 arrivals\n", d, openedByDay[d], per)
		switch {
		case d <= "2026-11-16":
			e12 += float64(openedByDay[d])
			n12 += arrByDay[d]
		case d >= "2026-11-18":
			l47 += float64(openedByDay[d])
			n47 += arrByDay[d]
		}
	}
	fmt.Printf("  days 4-7 over days 1-2: %.2f (%.1f against %.1f per 1,000)\n\n", (l47/float64(n47))/(e12/float64(n12)), 1000*l47/float64(n47), 1000*e12/float64(n12))

	var d47 []gi
	for _, x := range all {
		if !early(x.g.Opened.UTC().Format("2006-01-02")) {
			d47 = append(d47, x)
		}
	}
	var sizes, origs []int
	for _, x := range d47 {
		sizes = append(sizes, len(x.g.Members))
		origs = append(origs, x.final)
	}
	fmt.Printf("groups opened on days 4-7: %d\n  size %s\n  origins %s\n", len(d47), distInt(sizes), distInt(origs))
	endOfData := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	for _, N := range []int{2, 3, 5} {
		for _, H := range []time.Duration{6 * time.Hour, 24 * time.Hour, 48 * time.Hour} {
			row := map[string]int{}
			tot, totObs, obs := 0, 0, 0
			for _, x := range d47 {
				d := x.g.Opened.UTC().Format("2006-01-02")
				observable := !x.g.Opened.Add(H).After(endOfData)
				if observable {
					obs++
				}
				if dur, ok := offlineslice.ReachTime(x.ats, x.origins, N); ok && dur <= H {
					row[d]++
					tot++
					if observable {
						totObs++
					}
				}
			}
			fmt.Printf("  >=%d origins within %2dh: per day (11-18..21) %d / %d / %d / %d; total %d; fully observable groups %d, of which %d\n", N, int(H.Hours()), row["2026-11-18"], row["2026-11-19"], row["2026-11-20"], row["2026-11-21"], tot, obs, totObs)
		}
	}
	cross := map[string]map[string]int{}
	for _, N := range []int{2, 3, 5} {
		cross[fmt.Sprint(N)] = map[string]int{}
		for _, x := range all {
			if dur, ok := offlineslice.ReachTime(x.ats, x.origins, N); ok {
				cross[fmt.Sprint(N)][x.g.Opened.Add(dur).UTC().Format("2006-01-02")]++
			}
		}
		var ds []string
		for _, d := range days {
			ds = append(ds, fmt.Sprintf("%s:%d", d[5:], cross[fmt.Sprint(N)][d]))
		}
		fmt.Printf("  wakes by the day the %dth origin arrived (any opening day): %s\n", N, strings.Join(ds, " "))
	}
	nLate, nSolo := 0, 0
	inG := make([]int, len(items))
	for gi2, x := range all {
		for _, m := range x.g.Members {
			inG[m] = gi2
		}
	}
	for k, i := range un {
		if early(day(arr[i])) {
			continue
		}
		nLate++
		if all[inG[k]].final < 2 {
			nSolo++
		}
	}
	fmt.Printf("  unmatched arrivals of days 4-7 in groups that never reach 2 origins: %d of %d = %s\n\n", nSolo, nLate, pc(nSolo, nLate))

	// groups reaching 3 origins, whole week, for naming
	hashes := map[string]string{}
	for _, c := range caps {
		hashes[c.ID] = c.TextHash
	}
	need := map[string]string{}
	var out []g2Group
	for gidx, x := range all {
		dur, ok := offlineslice.ReachTime(x.ats, x.origins, 3)
		if !ok {
			continue
		}
		g := g2Group{ID: fmt.Sprintf("g%04d", gidx), Opened: x.g.Opened, Reach3: x.g.Opened.Add(dur), Size: len(x.g.Members), Origins: x.final, StoryAfterHours: -1, StoryStatus: "none"}
		exist := false
		for k, m := range x.g.Members {
			c := arr[un[m]]
			if k < 12 {
				need[hashes[c.ID]] = c.ID
				t := strings.TrimSpace(c.Title)
				g.Members = append(g.Members, g2Member{ID: c.ID, Time: c.Fetched, Title: t, Origin: origin[un[m]]})
			}
			for _, sid := range sd.Cites[c.ID] {
				f, ok := sd.First[sid]
				if !ok {
					continue
				}
				if f.After(x.g.Opened) {
					if h := f.Sub(x.g.Opened).Hours(); g.StoryAfterHours < 0 || h < g.StoryAfterHours {
						g.StoryAfterHours = h
					}
				} else {
					exist = true
				}
			}
		}
		switch {
		case g.StoryAfterHours >= 0:
			g.StoryStatus = "after"
		case exist:
			g.StoryStatus = "existing_only"
		}
		out = append(out, g)
	}
	texts := readTexts(filepath.Join(dataDir, "bodies.bin"), need)
	f, err := os.Create(filepath.Join(outDir, "groups2.jsonl"))
	must(err)
	w := bufio.NewWriter(f)
	nDays := map[string]int{}
	var after, existing, none int
	var lags []float64
	for i := range out {
		for k := range out[i].Members {
			out[i].Members[k].Text = clipRunes(strings.Join(strings.Fields(offlineslice.CleanText(texts[out[i].Members[k].ID])), " "), 500)
		}
		b, _ := json.Marshal(out[i])
		w.Write(b)
		w.WriteByte('\n')
		nDays[out[i].Reach3.UTC().Format("2006-01-02")]++
		if !early(out[i].Reach3.UTC().Format("2006-01-02")) {
			switch out[i].StoryStatus {
			case "after":
				after++
				lags = append(lags, out[i].StoryAfterHours)
			case "existing_only":
				existing++
			default:
				none++
			}
		}
	}
	must(w.Flush())
	f.Close()
	fmt.Printf("groups reaching 3 distinct origins (whole week): %d, by the day the 3rd origin arrived %v\n", len(out), nDays)
	fmt.Printf("of those reaching 3 on days 4-7: a story first published after the opening cites a member %d; only an existing story cites a member %d; no story cites a member %d; hours to that story: %s\n", after, existing, none, dec(lags))
}
