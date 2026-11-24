package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	e3Seed       = 20261124
	e3Groups     = 50
	e3Singles    = 40
	e3ShownMax   = 12
	e3Neighbours = 3
)

type e3Key struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"` // group | single
	Members int      `json:"members"`
	Origins int      `json:"origins"`
	Arrival []string `json:"arrival_ids"`
	Nearest []string `json:"nearest_ids,omitempty"`
	Cos     []float64 `json:"nearest_cos,omitempty"`
}

func runE3(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: e3 sample DATA OUT | e3 score OUT")
		os.Exit(2)
	}
	switch args[0] {
	case "sample":
		e3Sample(args[1], args[2])
	case "score":
		e3Score(args[1])
	}
}

func e3Sample(dir, out string) {
	must(os.MkdirAll(out, 0o755))
	arr, _ := loadArrivals(dir)
	day := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	var idx []int
	for i := range arr {
		if !arr[i].Fetched.Before(day) && arr[i].Fetched.Before(day.Add(24*time.Hour)) {
			idx = append(idx, i)
		}
	}
	gs := groupArrivals(arr, idx)
	var multi, single []int
	sz := map[int]int{}
	for gi, g := range gs {
		if len(g.Members) >= 2 {
			multi = append(multi, gi)
		} else {
			single = append(single, gi)
		}
		sz[len(g.Members)]++
	}
	r := rand.New(rand.NewSource(e3Seed))
	r.Shuffle(len(multi), func(i, j int) { multi[i], multi[j] = multi[j], multi[i] })
	r.Shuffle(len(single), func(i, j int) { single[i], single[j] = single[j], single[i] })
	multi2, nSingle := len(multi), len(single)
	multi, single = multi[:e3Groups], single[:e3Singles]
	var md strings.Builder
	var keys []e3Key
	w := func(f string, a ...any) { fmt.Fprintf(&md, f, a...) }
	w("# E3 sample (seed %d): %d arrivals of 2026-11-23 in %d groups: %d singletons, %d groups of two or more, largest %d members.\n\n", e3Seed, len(idx), len(gs), nSingle, multi2, maxSize(gs))
	show := func(c *capture) string {
		return fmt.Sprintf("%s | %s | %s | %s", c.Fetched.Format("15:04"), c.Origin, trunc(c.Title, 120), trunc(c.Lead, 220))
	}
	for n, gi := range multi {
		g := gs[gi]
		origins := map[string]bool{}
		for _, m := range g.Members {
			origins[arr[m].Origin] = true
		}
		id := fmt.Sprintf("G%02d", n+1)
		k := e3Key{ID: id, Kind: "group", Members: len(g.Members), Origins: len(origins)}
		for _, m := range g.Members {
			k.Arrival = append(k.Arrival, arr[m].ID)
		}
		keys = append(keys, k)
		members := append([]int(nil), g.Members...)
		note := ""
		if len(members) > e3ShownMax {
			rr := rand.New(rand.NewSource(int64(e3Seed + n)))
			rr.Shuffle(len(members), func(i, j int) { members[i], members[j] = members[j], members[i] })
			members = members[:e3ShownMax]
			sort.Ints(members)
			note = fmt.Sprintf(", %d shown at random", e3ShownMax)
		}
		w("## %s: %d members%s\n", id, len(g.Members), note)
		for _, m := range members {
			w("- %s\n", show(&arr[m]))
		}
		w("\n")
	}
	for n, gi := range single {
		m := gs[gi].Members[0]
		a := &arr[m]
		type nb struct {
			i   int
			cos float64
		}
		var nbs []nb
		for j := range arr {
			if j == m || arr[j].Fetched.Sub(a.Fetched) > 48*time.Hour || a.Fetched.Sub(arr[j].Fetched) > 48*time.Hour {
				continue
			}
			nbs = append(nbs, nb{j, dot(a.Vec, arr[j].Vec)})
		}
		sort.Slice(nbs, func(x, y int) bool { return nbs[x].cos > nbs[y].cos })
		id := fmt.Sprintf("S%02d", n+1)
		k := e3Key{ID: id, Kind: "single", Members: 1, Origins: 1, Arrival: []string{a.ID}}
		w("## %s: one arrival\n- THIS: %s\n", id, show(a))
		for _, x := range nbs[:e3Neighbours] {
			k.Nearest = append(k.Nearest, arr[x.i].ID)
			k.Cos = append(k.Cos, x.cos)
			w("- near: %s\n", show(&arr[x.i]))
		}
		w("\n")
		keys = append(keys, k)
	}
	must(os.WriteFile(filepath.Join(out, "e3-sample.md"), []byte(md.String()), 0o644))
	kb, _ := json.MarshalIndent(keys, "", " ")
	must(os.WriteFile(filepath.Join(out, "e3-key.json"), kb, 0o644))
	fmt.Printf("wrote e3-sample.md (%d groups, %d singletons) and e3-key.json; group-size histogram of the day: singletons %d, 2: %d, 3 to 4: %d, 5 to 11: %d, 12 or more: %d\n",
		len(multi), len(single), sz[1], sz[2], sz[3]+sz[4], countRange(sz, 5, 11), countRange(sz, 12, 1<<30))
}

func maxSize(gs []group) int {
	m := 0
	for _, g := range gs {
		if len(g.Members) > m {
			m = len(g.Members)
		}
	}
	return m
}

// e3Score reads e3-labels.tsv (id TAB label) and e3-key.json from OUT.
func e3Score(out string) {
	var keys []e3Key
	kb, err := os.ReadFile(filepath.Join(out, "e3-key.json"))
	must(err)
	must(json.Unmarshal(kb, &keys))
	lab := map[string]string{}
	f, err := os.Open(filepath.Join(out, "e3-labels.tsv"))
	must(err)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if a, b, ok := strings.Cut(sc.Text(), "\t"); ok {
			lab[strings.TrimSpace(a)] = strings.TrimSpace(strings.Fields(b)[0])
		}
	}
	var sb strings.Builder
	p := func(f string, a ...any) { fmt.Fprintf(&sb, f, a...) }
	count := func(kind string, pred func(k e3Key) bool) map[string]int {
		m := map[string]int{"_n": 0}
		for _, k := range keys {
			if k.Kind == kind && pred(k) {
				m[lab[k.ID]]++
				m["_n"]++
			}
		}
		return m
	}
	rate := func(name string, k, n int) {
		lo, hi := wilson(k, n)
		p("  %-8s %2d of %2d = %5.1f%%  (Wilson 95%% %.0f%% to %.0f%%)\n", name, k, n, 100*float64(k)/float64(max(n, 1)), 100*lo, 100*hi)
	}
	for _, sub := range []struct {
		name string
		pred func(k e3Key) bool
	}{
		{"groups of two or more arrivals (all sampled)", func(k e3Key) bool { return true }},
		{"of those, two or more independent origins", func(k e3Key) bool { return k.Origins >= 2 }},
		{"of those, one origin only", func(k e3Key) bool { return k.Origins < 2 }},
	} {
		m := count("group", sub.pred)
		p("%s: n = %d\n", sub.name, m["_n"])
		for _, l := range []string{"ONE", "MOSTLY", "MERGE", "NOISE"} {
			rate(l, m[l], m["_n"])
		}
	}
	m := count("single", func(k e3Key) bool { return true })
	p("singletons: n = %d\n", m["_n"])
	for _, l := range []string{"SPLIT", "ALONE"} {
		rate(l, m[l], m["_n"])
	}
	missing := 0
	for _, k := range keys {
		if lab[k.ID] == "" {
			missing++
		}
	}
	p("unlabelled: %d\n", missing)
	must(os.WriteFile(filepath.Join(out, "e3-score.txt"), []byte(sb.String()), 0o644))
	fmt.Print(sb.String())
}
