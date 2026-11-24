package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

// E5 analysis (ledger C232 to C240): reads the arms' JSONL written by e5-run.py.

type e5Rec struct {
	Batch   int      `json:"batch"`
	IDs     []string `json:"ids"`
	Arm     string   `json:"arm"`
	Seconds float64  `json:"seconds"`
	Error   *string  `json:"error"`
	Retried bool     `json:"retried_without_effort"`
	Usage   struct {
		Prompt  int `json:"prompt_tokens"`
		Compl   int `json:"completion_tokens"`
		Details struct {
			Cached int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
	Cost   float64           `json:"cost"`
	Parsed []json.RawMessage `json:"parsed"`
}

type e5Out struct {
	Subjects []string
	Asserts  string
	NewFlags []bool
}

func normName(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func loadItems(path string) []e5Item {
	f, err := os.Open(path)
	must(err)
	defer f.Close()
	var out []e5Item
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		var it e5Item
		must(json.Unmarshal(sc.Bytes(), &it))
		out = append(out, it)
	}
	return out
}

func loadArm(path string) (recs []e5Rec, outs map[string]e5Out) {
	outs = map[string]e5Out{}
	f, err := os.Open(path)
	if err != nil {
		return nil, outs
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<28)
	for sc.Scan() {
		var r e5Rec
		must(json.Unmarshal(sc.Bytes(), &r))
		recs = append(recs, r)
		for _, raw := range r.Parsed {
			var o struct {
				ID       string            `json:"id"`
				Subjects []json.RawMessage `json:"subjects"`
				Asserts  string            `json:"asserts"`
			}
			if json.Unmarshal(raw, &o) != nil || o.ID == "" {
				continue
			}
			var out e5Out
			out.Asserts = o.Asserts
			for _, s := range o.Subjects {
				var str string
				if json.Unmarshal(s, &str) == nil {
					out.Subjects = append(out.Subjects, str)
					continue
				}
				var obj struct {
					Name string `json:"name"`
					E    *bool  `json:"e"`
				}
				if json.Unmarshal(s, &obj) == nil && obj.Name != "" {
					out.Subjects = append(out.Subjects, obj.Name)
					out.NewFlags = append(out.NewFlags, obj.E != nil && !*obj.E)
				}
			}
			outs[o.ID] = out
		}
	}
	return recs, outs
}

// scoreArm is C235's scoring: I_sum x Ne from an arm's own subjects, over the day's items.
func scoreArm(items []e5Item, outs map[string]e5Out) []float64 {
	n := len(items)
	names := make([][]string, n)
	df := map[string]int{}
	for i, it := range items {
		seen := map[string]bool{}
		for _, s := range outs[it.ID].Subjects {
			k := normName(s)
			if k == "" || seen[k] {
				continue
			}
			seen[k] = true
			if i >= e1bFurnMin && float64(df[k])/float64(i) > e1bFurniture {
				continue
			}
			names[i] = append(names[i], k)
		}
		for k := range seen {
			df[k]++
		}
	}
	type mention struct {
		at     time.Time
		origin string
	}
	byKey := map[string][]mention{}
	subjSeen := map[string]bool{}
	pairSeen := map[string]bool{}
	scores := make([]float64, n)
	for i, it := range items {
		t, _ := time.Parse("2006-01-02T15:04:05Z", it.Time)
		type sr struct {
			k  string
			rs int
		}
		var srs []sr
		var isum float64
		for _, k := range names[i] {
			byKey[k] = append(byKey[k], mention{t, it.Origin})
			seen := map[string]bool{}
			for j := len(byKey[k]) - 1; j >= 0; j-- {
				if !byKey[k][j].at.After(t.Add(-3 * time.Hour)) {
					break
				}
				seen[byKey[k][j].origin] = true
			}
			srs = append(srs, sr{k, len(seen)})
			isum += math.Log2(1 + float64(len(seen)))
		}
		sort.SliceStable(srs, func(x, y int) bool { return srs[x].rs > srs[y].rs })
		top := srs
		if len(top) > 8 {
			top = top[:8]
		}
		pairs, newP, newS := 0, 0, 0
		for x := 0; x < len(top); x++ {
			if !subjSeen[top[x].k] {
				newS++
			}
			for y := x + 1; y < len(top); y++ {
				pairs++
				a, b := top[x].k, top[y].k
				if a > b {
					a, b = b, a
				}
				if !pairSeen[a+"|"+b] {
					newP++
				}
			}
		}
		if pairs+len(top) > 0 {
			scores[i] = isum * float64(newP+newS) / float64(pairs+len(top))
		}
		for _, k := range names[i] {
			subjSeen[k] = true
		}
		for x := 0; x < len(names[i]); x++ {
			for y := x + 1; y < len(names[i]); y++ {
				a, b := names[i][x], names[i][y]
				if a > b {
					a, b = b, a
				}
				pairSeen[a+"|"+b] = true
			}
		}
	}
	return scores
}

func hourBin(it e5Item) int {
	t, _ := time.Parse("2006-01-02T15:04:05Z", it.Time)
	day := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	return int(t.Add(-time.Nanosecond).Sub(day) / time.Hour)
}

func rankOf(scores []float64, idx []int, i int) int {
	r := 1
	for _, j := range idx {
		if scores[j] > scores[i]+1e-12 {
			r++
		}
	}
	return r
}

func jaccard(a, b []string) float64 {
	sa, sb := map[string]bool{}, map[string]bool{}
	for _, x := range a {
		sa[normName(x)] = true
	}
	for _, x := range b {
		sb[normName(x)] = true
	}
	inter := 0
	for k := range sa {
		if sb[k] {
			inter++
		}
	}
	u := len(sa) + len(sb) - inter
	if u == 0 {
		return 1
	}
	return float64(inter) / float64(u)
}

var rePetition = regexp.MustCompile(`(?i)petition`)

func runE5Report(dir, outDir string) {
	items := loadItems(filepath.Join(outDir, "e5-input.jsonl"))
	var b strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }
	sample := map[string]bool{}
	if data, err := os.ReadFile(filepath.Join(outDir, "e5-sample100.txt")); err == nil {
		for _, l := range strings.Fields(string(data)) {
			sample[l] = true
		}
	}
	arms := map[string]map[string]e5Out{}
	recsOf := map[string][]e5Rec{}
	for _, a := range []string{"A", "B", "D", "E", "A5", "A2"} {
		r, o := loadArm(filepath.Join(outDir, "e5-"+a+".jsonl"))
		if len(r) > 0 {
			recsOf[a], arms[a] = r, o
		}
	}
	w("== cost, time and parse, per arm ==\narm | calls | failed calls | retried without effort | input tokens | cached | output tokens | cost $ | seconds total | mean s/call | arrivals covered | missing or unparsed\n")
	totalCost := 0.0
	for _, a := range []string{"A", "B", "D", "E", "A5", "A2"} {
		rs := recsOf[a]
		if len(rs) == 0 {
			continue
		}
		var pin, cached, comp, failed, retried int
		var cost, secs float64
		want := 0
		for _, r := range rs {
			pin += r.Usage.Prompt
			cached += r.Usage.Details.Cached
			comp += r.Usage.Compl
			cost += r.Cost
			secs += r.Seconds
			want += len(r.IDs)
			if r.Error != nil || r.Parsed == nil {
				failed++
			}
			if r.Retried {
				retried++
			}
		}
		got := len(arms[a])
		totalCost += cost
		w("%s | %d | %d | %d | %d | %d | %d | %.4f | %.0f | %.1f | %d of %d | %d (%.2f%%)\n", a, len(rs), failed, retried, pin, cached, comp, cost, secs, secs/float64(len(rs)), got, want, want-got, 100*float64(want-got)/float64(max(want, 1)))
	}
	w("total measured cost (all arms) $%.4f\n", totalCost)
	// digest size and the cost of D over B
	if rb, rd := recsOf["B"], recsOf["D"]; len(rb) > 0 && len(rd) > 0 {
		n := min(len(rb), len(rd))
		var delta float64
		var dIn, bIn, dCached, dOut, bOut int
		var dCost, bCost float64
		for i := 0; i < n; i++ {
			delta += float64(rd[i].Usage.Prompt - rb[i].Usage.Prompt)
			dIn += rd[i].Usage.Prompt
			bIn += rb[i].Usage.Prompt
			dCached += rd[i].Usage.Details.Cached
			dOut += rd[i].Usage.Compl
			bOut += rb[i].Usage.Compl
			dCost += rd[i].Cost
			bCost += rb[i].Cost
		}
		w("\n== digest size and cost (D over B, %d paired calls) ==\n", n)
		w("prompt tokens D minus B per call, mean: %.0f (the digest, as the vendor counts it; the two prompts differ also in the model's own earlier names)\n", delta/float64(n))
		w("D input %d tokens, of which cached %d (%.1f%%); B input %d\n", dIn, dCached, 100*float64(dCached)/float64(max(dIn, 1)), bIn)
		w("D cost with the cache as measured $%.4f; B $%.4f; added $%.4f\n", dCost, bCost, dCost-bCost)
		unc := (float64(dIn)*PriceIn + float64(dOut)*PriceOut) / 1e6
		uncB := (float64(bIn)*PriceIn + float64(bOut)*PriceOut) / 1e6
		w("without any cache: D $%.4f, B $%.4f, added $%.4f\n", unc, uncB, unc-uncB)
	}
	// projections
	if rs := recsOf["B"]; len(rs) > 0 {
		var secs, cost float64
		for _, r := range rs {
			secs += r.Seconds
			cost += r.Cost
		}
		perCall, perCost := secs/float64(len(rs)), cost/float64(len(rs))
		rate := float64(len(items)) / 24
		w("\n== projection (arm B per call: %.1f s, $%.4f) ==\n", perCall, perCost)
		for _, mult := range []float64{1, 10} {
			calls := rate * mult / 25
			w("%.0fx today's rate: %.0f arrivals an hour -> %.1f calls an hour, %.0f s of calls an hour, $%.3f an hour (sequential: %.1f%% of wall clock)\n", mult, rate*mult, calls, calls*perCall, calls*perCost, 100*calls*perCall/3600)
		}
	}
	// scoring under each arm: C235, C236 and the C230 reference
	w("\n== importance x symbolic novelty (e) per arm: best Huang-pattern arrival in the hour ending 19:00, in the turn hour, and echo rank shares ==\n")
	idxByBin := map[int][]int{}
	for i, it := range items {
		idxByBin[hourBin(it)] = append(idxByBin[hourBin(it)], i)
	}
	for _, a := range []string{"A", "B", "D", "E"} {
		o := arms[a]
		if o == nil {
			continue
		}
		sc := scoreArm(items, o)
		for _, hr := range []int{19, 15} {
			idx := idxByBin[hr-1]
			var rs []int
			for _, i := range idx {
				if items[i].Huang {
					rs = append(rs, rankOf(sc, idx, i))
				}
			}
			sort.Ints(rs)
			if len(rs) > 12 {
				rs = rs[:12]
			}
			w("%s, hour ending %02d:00 (%d arrivals): Huang ranks %v\n", a, hr, len(idx), rs)
		}
		var shares []float64
		for hr := 0; hr < 24; hr++ {
			idx := idxByBin[hr]
			for _, i := range idx {
				if items[i].Echo {
					shares = append(shares, float64(rankOf(sc, idx, i))/float64(len(idx)))
				}
			}
		}
		sort.Float64s(shares)
		if len(shares) > 0 {
			w("%s: echoes n %d, median rank share %.3f, 10th percentile %.3f\n", a, len(shares), shares[len(shares)/2], shares[len(shares)/10])
		}
		// top ten at 19:00
		idx := idxByBin[18]
		o2 := append([]int(nil), idx...)
		sort.SliceStable(o2, func(x, y int) bool { return sc[o2[x]] > sc[o2[y]] })
		w("top 10 at 19:00 under %s (score | Huang | echo | text):\n", a)
		for k := 0; k < 10 && k < len(o2); k++ {
			it := items[o2[k]]
			t := it.Title
			if t == "" {
				t = "(post) " + it.Text
			}
			w("  %.2f | %v | %v | %s\n", sc[o2[k]], it.Huang, it.Echo, trunc(t, 80))
		}
	}
	// C234: reinstatement symbols
	w("\n== C234: reinstatement symbols (a model subject containing reinstat, return or restor) ==\n")
	var petition, turn []int
	for i, it := range items {
		if !it.Huang {
			continue
		}
		txt := it.Title + " " + it.Text
		if it.Time < "2026-11-23T11:00:00Z" && rePetition.MatchString(txt) {
			petition = append(petition, i)
		}
		if it.Time > "2026-11-23T14:00:00Z" && it.Time <= "2026-11-23T15:00:00Z" && strings.Contains(strings.ToLower(txt), "reinstat") {
			turn = append(turn, i)
		}
	}
	w("petition arrivals: %d; turn arrivals: %d\n", len(petition), len(turn))
	hasSym := func(o e5Out) bool {
		for _, s := range o.Subjects {
			n := normName(s)
			if strings.Contains(n, "reinstat") || strings.Contains(n, "return") || strings.Contains(n, "restor") {
				return true
			}
		}
		return false
	}
	for _, a := range []string{"A", "B", "D", "E"} {
		o := arms[a]
		if o == nil {
			continue
		}
		tc, pc := 0, 0
		for _, i := range turn {
			if hasSym(o[items[i].ID]) {
				tc++
			}
		}
		for _, i := range petition {
			if hasSym(o[items[i].ID]) {
				pc++
			}
		}
		w("%s: turn arrivals with a reinstatement symbol %d of %d; petition arrivals %d of %d\n", a, tc, len(turn), pc, len(petition))
		// pairs carried by turn arrivals and by no petition arrival
		pp := map[string]bool{}
		for _, i := range petition {
			ns := []string{}
			for _, s := range o[items[i].ID].Subjects {
				ns = append(ns, normName(s))
			}
			for x := 0; x < len(ns); x++ {
				for y := x + 1; y < len(ns); y++ {
					p := []string{ns[x], ns[y]}
					sort.Strings(p)
					pp[p[0]+"|"+p[1]] = true
				}
			}
		}
		cnt := map[string]int{}
		for _, i := range turn {
			ns := []string{}
			for _, s := range o[items[i].ID].Subjects {
				ns = append(ns, normName(s))
			}
			for x := 0; x < len(ns); x++ {
				for y := x + 1; y < len(ns); y++ {
					p := []string{ns[x], ns[y]}
					sort.Strings(p)
					if !pp[p[0]+"|"+p[1]] {
						cnt[p[0]+" + "+p[1]]++
					}
				}
			}
		}
		type kv struct {
			k string
			n int
		}
		var kvs []kv
		for k, n := range cnt {
			kvs = append(kvs, kv{k, n})
		}
		sort.Slice(kvs, func(i, j int) bool { return kvs[i].n > kvs[j].n || (kvs[i].n == kvs[j].n && kvs[i].k < kvs[j].k) })
		w("  %s: pairs in turn arrivals and in no petition arrival, commonest:", a)
		for i, x := range kvs {
			if i >= 6 {
				break
			}
			w(" [%s x%d]", x.k, x.n)
		}
		w("\n")
	}
	// C241, C242: arm E against arm D
	if re, rd := recsOf["E"], recsOf["D"]; len(re) > 0 {
		w("\n== arm E (grouped fan-out) against arm D: calls and tokens ==\n")
		tally := func(rs []e5Rec) (calls, arrivals, pin, comp int) {
			for _, r := range rs {
				calls++
				arrivals += len(r.IDs)
				pin += r.Usage.Prompt
				comp += r.Usage.Compl
			}
			return
		}
		ec, ea, ei, eo := tally(re)
		w("E: %d calls for %d arrivals (%.2f an arrival per call); input %d tokens = %.0f per arrival; output %d = %.0f per arrival\n", ec, ea, float64(ea)/float64(max(ec, 1)), ei, float64(ei)/float64(max(ea, 1)), eo, float64(eo)/float64(max(ea, 1)))
		if len(rd) > 0 {
			dc, da, di, do := tally(rd)
			w("D: %d calls for %d arrivals (%.2f per call); input %d = %.0f per arrival; output %d = %.0f per arrival\n", dc, da, float64(da)/float64(max(dc, 1)), di, float64(di)/float64(max(da, 1)), do, float64(do)/float64(max(da, 1)))
			w("calls an hour at today's rate (full day / 24): E %.1f, D %.1f; ratio E/D %.2f (scaled to the arrivals covered: E %.2f calls per 100 arrivals, D %.2f)\n", float64(ec)/24, float64(dc)/24, float64(ec)/float64(max(dc, 1)), 100*float64(ec)/float64(max(ea, 1)), 100*float64(dc)/float64(max(da, 1)))
		}
		kinds := map[string][2]int{}
		kindOf := map[string]string{}
		if data, err := os.ReadFile(filepath.Join(outDir, "e5-ecalls.jsonl")); err == nil {
			for _, l := range strings.Split(string(data), "\n") {
				var c e5Call
				if json.Unmarshal([]byte(l), &c) == nil {
					for _, id := range c.IDs {
						kindOf[id] = c.Kind
					}
					v := kinds[c.Kind]
					v[0]++
					v[1] += len(c.IDs)
					kinds[c.Kind] = v
				}
			}
		}
		for k, v := range kinds {
			w("E calls of kind %s: %d calls, %d arrivals\n", k, v[0], v[1])
		}
		// which call holds each arrival, under D (batch of 25 by index) and E
		callOfE := map[string]int{}
		for _, r := range re {
			for _, id := range r.IDs {
				callOfE[id] = r.Batch
			}
		}
		callOfD := map[string]int{}
		for _, r := range rd {
			for _, id := range r.IDs {
				callOfD[id] = r.Batch
			}
		}
		var petition, turn []string
		for _, it := range items {
			if !it.Huang {
				continue
			}
			txt := it.Title + " " + it.Text
			if it.Time < "2026-11-23T11:00:00Z" && rePetition.MatchString(txt) {
				petition = append(petition, it.ID)
			}
			if it.Time > "2026-11-23T14:00:00Z" && it.Time <= "2026-11-23T15:00:00Z" && strings.Contains(strings.ToLower(txt), "reinstat") {
				turn = append(turn, it.ID)
			}
		}
		distinct := func(ids []string, m map[string]int) int {
			set := map[int]bool{}
			for _, id := range ids {
				if c, ok := m[id]; ok {
					set[c] = true
				}
			}
			return len(set)
		}
		w("C242: the %d turn arrivals fall in %d distinct calls under D and %d under E; the %d petition arrivals in %d under D and %d under E\n", len(turn), distinct(turn, callOfD), distinct(turn, callOfE), len(petition), distinct(petition, callOfD), distinct(petition, callOfE))
		for _, id := range turn {
			w("  turn %s: E kind %s, call %d\n", id, kindOf[id], callOfE[id])
		}
	}
	// C237: batch size
	if a5, a2 := arms["A5"], arms["A2"]; a5 != nil || a2 != nil {
		w("\n== C237: 25 to a call against 5 to a call and against another 25-to-a-call run, on the 100-arrival sample ==\n")
		var j5, j2 []float64
		var ids []string
		for id := range sample {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		var len25, len5 []float64
		for _, id := range ids {
			a := arms["A"][id]
			if o, ok := a5[id]; ok {
				j5 = append(j5, jaccard(a.Subjects, o.Subjects))
				len25 = append(len25, float64(len(a.Subjects)))
				len5 = append(len5, float64(len(o.Subjects)))
			}
			if o, ok := a2[id]; ok {
				j2 = append(j2, jaccard(a.Subjects, o.Subjects))
			}
		}
		mean := func(v []float64) float64 {
			if len(v) == 0 {
				return math.NaN()
			}
			s := 0.0
			for _, x := range v {
				s += x
			}
			return s / float64(len(v))
		}
		w("mean Jaccard of the subject sets, A (25 a call) against 5 a call: %.3f (n %d); against A' (25 a call, other batches): %.3f (n %d)\n", mean(j5), len(j5), mean(j2), len(j2))
		w("mean subjects per arrival: 25 a call %.2f, 5 a call %.2f\n", mean(len25), mean(len5))
	}
	must(os.WriteFile(filepath.Join(outDir, "e5-report.txt"), []byte(b.String()), 0o644))
	fmt.Print(b.String())
}

const PriceIn, PriceOut = 0.12, 0.48

// runE5Sheet writes the hand-read worksheets: the drift groups and the cutoff sample.
func runE5Sheet(dir, outDir string) {
	items := loadItems(filepath.Join(outDir, "e5-input.jsonl"))
	posOf := map[int]string{}
	for _, it := range items {
		posOf[it.Idx] = it.ID
	}
	byID := map[string]e5Item{}
	for _, it := range items {
		byID[it.ID] = it
	}
	arr, _ := loadArrivals(dir)
	day := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	var idx []int
	for i := range arr {
		if !arr[i].Fetched.Before(day) {
			idx = append(idx, i)
		}
	}
	gs := groupArrivals(arr, idx)
	var cands []int
	for gi, g := range gs {
		o := map[string]bool{}
		for _, m := range g.Members {
			o[arr[m].Origin] = true
		}
		if len(g.Members) >= 3 && len(o) >= 2 {
			cands = append(cands, gi)
		}
	}
	r := rand.New(rand.NewSource(20261124 + 5))
	r.Shuffle(len(cands), func(i, j int) { cands[i], cands[j] = cands[j], cands[i] })
	if len(cands) > 45 {
		cands = cands[:45]
	}
	arms := map[string]map[string]e5Out{}
	for _, a := range []string{"A", "B", "D", "E"} {
		_, arms[a] = loadArm(filepath.Join(outDir, "e5-"+a+".jsonl"))
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "# E5 worksheet: %d candidate groups of 11-23 (3 or more members, 2 or more origins), seed 20261129; name the one real subject of each and a pattern for the names that denote it\n\n", len(cands))
	for n, gi := range cands {
		g := gs[gi]
		fmt.Fprintf(&sb, "## D%02d (%d members)\n", n+1, len(g.Members))
		shown := g.Members
		if len(shown) > 8 {
			shown = shown[:8]
		}
		for _, m := range shown {
			id := posOf[m]
			it := byID[id]
			t := it.Title
			if t == "" {
				t = "(post) " + it.Text
			}
			fmt.Fprintf(&sb, "- %s %s | %s\n", id, it.Origin, trunc(t, 100))
			for _, a := range []string{"A", "B", "D", "E"} {
				if o, ok := arms[a][id]; ok {
					fmt.Fprintf(&sb, "    %s: %s\n", a, strings.Join(o.Subjects, "; "))
				}
			}
		}
		sb.WriteString("\n")
	}
	must(os.WriteFile(filepath.Join(outDir, "e5-drift-sheet.md"), []byte(sb.String()), 0o644))
	// cutoff sample: one member per first 30 drift groups chosen by the drift file is written later; here the Huang reinstat arrivals
	fmt.Printf("wrote e5-drift-sheet.md with %d groups\n", len(cands))
}

func runE5Drift(dir, outDir string) {
	items := loadItems(filepath.Join(outDir, "e5-input.jsonl"))
	// e5-drift.tsv: group number TAB subject label TAB regex; the group's members come from the sheet's grouping
	posOf := map[int]string{}
	for _, it := range items {
		posOf[it.Idx] = it.ID
	}
	arr, _ := loadArrivals(dir)
	day := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	var idx []int
	for i := range arr {
		if !arr[i].Fetched.Before(day) {
			idx = append(idx, i)
		}
	}
	gs := groupArrivals(arr, idx)
	var cands []int
	for gi, g := range gs {
		o := map[string]bool{}
		for _, m := range g.Members {
			o[arr[m].Origin] = true
		}
		if len(g.Members) >= 3 && len(o) >= 2 {
			cands = append(cands, gi)
		}
	}
	r := rand.New(rand.NewSource(20261124 + 5))
	r.Shuffle(len(cands), func(i, j int) { cands[i], cands[j] = cands[j], cands[i] })
	arms := map[string]map[string]e5Out{}
	for _, a := range []string{"A", "B", "D", "E"} {
		_, arms[a] = loadArm(filepath.Join(outDir, "e5-"+a+".jsonl"))
	}
	f, err := os.Open(filepath.Join(outDir, "e5-drift.tsv"))
	must(err)
	sc := bufio.NewScanner(f)
	tot := map[string]float64{}
	nSub := 0
	var sb strings.Builder
	sb.WriteString("group | subject | members | distinct names A | B | D | E\n")
	for sc.Scan() {
		fs := strings.Split(sc.Text(), "\t")
		if len(fs) < 3 || strings.HasPrefix(fs[0], "#") {
			continue
		}
		var gnum int
		fmt.Sscanf(fs[0], "D%d", &gnum)
		re := regexp.MustCompile("(?i)" + fs[2])
		g := gs[cands[gnum-1]]
		row := fmt.Sprintf("%s | %s | %d |", fs[0], fs[1], len(g.Members))
		for _, a := range []string{"A", "B", "D", "E"} {
			names := map[string]bool{}
			for _, m := range g.Members {
				// only arrivals every arm compressed, so a failed call does not lower an arm's count
				missing := false
				for _, a2 := range []string{"A", "B", "D", "E"} {
					if _, ok := arms[a2][posOf[m]]; !ok {
						missing = true
					}
				}
				if missing {
					continue
				}
				for _, s := range arms[a][posOf[m]].Subjects {
					if re.MatchString(s) {
						names[normName(s)] = true
					}
				}
			}
			row += fmt.Sprintf(" %d |", len(names))
			tot[a] += float64(len(names))
		}
		nSub++
		sb.WriteString(row + "\n")
	}
	fmt.Fprintf(&sb, "subjects %d; mean distinct names per subject: A %.2f, B %.2f, D %.2f, E %.2f; B/A %.2f, D/B %.2f, E/D %.2f\n", nSub, tot["A"]/float64(nSub), tot["B"]/float64(nSub), tot["D"]/float64(nSub), tot["E"]/float64(nSub), tot["B"]/tot["A"], tot["D"]/tot["B"], tot["E"]/tot["D"])
	must(os.WriteFile(filepath.Join(outDir, "e5-drift-score.txt"), []byte(sb.String()), 0o644))
	fmt.Print(sb.String())
}

// runE5Cutoff writes the cutoff hand-read sheet (ledger C238, C243): 30 arrivals (one member of each of the
// first 30 drift candidates), up to 10 Huang-pattern turn arrivals that carry "reinstat", and 20 singletons of arm E's grouping.
func runE5Cutoff(dir, outDir string) {
	items := loadItems(filepath.Join(outDir, "e5-input.jsonl"))
	byID := map[string]e5Item{}
	posOf := map[int]string{}
	for _, it := range items {
		byID[it.ID] = it
		posOf[it.Idx] = it.ID
	}
	arr, _ := loadArrivals(dir)
	day := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	var idx []int
	for i := range arr {
		if !arr[i].Fetched.Before(day) {
			idx = append(idx, i)
		}
	}
	gs := groupArrivals(arr, idx)
	var cands []int
	for gi, g := range gs {
		o := map[string]bool{}
		for _, m := range g.Members {
			o[arr[m].Origin] = true
		}
		if len(g.Members) >= 3 && len(o) >= 2 {
			cands = append(cands, gi)
		}
	}
	r := rand.New(rand.NewSource(20261124 + 5))
	r.Shuffle(len(cands), func(i, j int) { cands[i], cands[j] = cands[j], cands[i] })
	var sample []string
	r2 := rand.New(rand.NewSource(20261124 + 7))
	for _, gi := range cands[:30] {
		g := gs[gi]
		sample = append(sample, posOf[g.Members[r2.Intn(len(g.Members))]])
	}
	var turn []string
	for _, it := range items {
		if it.Huang && it.Time > "2026-11-23T14:00:00Z" && it.Time <= "2026-11-23T15:00:00Z" && strings.Contains(strings.ToLower(it.Title+" "+it.Text), "reinstat") {
			turn = append(turn, it.ID)
		}
	}
	r2.Shuffle(len(turn), func(i, j int) { turn[i], turn[j] = turn[j], turn[i] })
	if len(turn) > 10 {
		turn = turn[:10]
	}
	inGroup := map[string]bool{}
	f, err := os.Open(filepath.Join(outDir, "e5-ecalls.jsonl"))
	must(err)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	var singles []string
	for sc.Scan() {
		var c e5Call
		must(json.Unmarshal(sc.Bytes(), &c))
		for _, id := range c.IDs {
			if c.Kind == "group" {
				inGroup[id] = true
			} else {
				singles = append(singles, id)
			}
		}
	}
	f.Close()
	r2.Shuffle(len(singles), func(i, j int) { singles[i], singles[j] = singles[j], singles[i] })
	singles = singles[:20]
	type keyRow struct {
		ID      string `json:"id"`
		Source  string `json:"source"`
		InGroup bool   `json:"in_group"`
	}
	var key []keyRow
	seen := map[string]bool{}
	add := func(ids []string, src string) {
		for _, id := range ids {
			if seen[id] {
				continue
			}
			seen[id] = true
			key = append(key, keyRow{id, src, inGroup[id]})
		}
	}
	add(sample, "drift-group member")
	add(turn, "Huang turn arrival")
	add(singles, "singleton of arm E")
	arms := map[string]map[string]e5Out{}
	for _, a := range []string{"B", "D", "E"} {
		_, arms[a] = loadArm(filepath.Join(outDir, "e5-"+a+".jsonl"))
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "# E5 cutoff sheet: %d arrivals. For each, mark B, D, E 1 if the compression is wrong because the model did not know something after its training cutoff (a reinstatement read as a first appointment, an out-of-date role, an unknown event or product), else 0.\n\n", len(key))
	for _, k := range key {
		it := byID[k.ID]
		t := it.Title
		if t == "" {
			t = "(post)"
		}
		fmt.Fprintf(&sb, "## %s [%s; in an E group: %v] %s %s\n%s\n%s\n", k.ID, k.Source, k.InGroup, it.Time[11:16], it.Origin, t, trunc(it.Text, 420))
		for _, a := range []string{"B", "D", "E"} {
			o := arms[a][k.ID]
			fmt.Fprintf(&sb, "  %s: %s || %s\n", a, strings.Join(o.Subjects, "; "), o.Asserts)
		}
		sb.WriteString("\n")
	}
	must(os.WriteFile(filepath.Join(outDir, "e5-cutoff-sheet.md"), []byte(sb.String()), 0o644))
	kb, _ := json.MarshalIndent(key, "", " ")
	must(os.WriteFile(filepath.Join(outDir, "e5-cutoff-key.json"), kb, 0o644))
	fmt.Printf("wrote e5-cutoff-sheet.md: %d arrivals (%d drift-group members, %d turn, %d singletons)\n", len(key), len(sample), len(turn), len(singles))
}

func runE5CutoffScore(outDir string) {
	var key []struct {
		ID      string `json:"id"`
		Source  string `json:"source"`
		InGroup bool   `json:"in_group"`
	}
	kb, err := os.ReadFile(filepath.Join(outDir, "e5-cutoff-key.json"))
	must(err)
	must(json.Unmarshal(kb, &key))
	lab := map[string][3]int{}
	f, err := os.Open(filepath.Join(outDir, "e5-cutoff.tsv"))
	must(err)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fs := strings.Split(sc.Text(), "\t")
		if len(fs) >= 4 && !strings.HasPrefix(fs[0], "#") {
			var v [3]int
			for k := 0; k < 3; k++ {
				fmt.Sscanf(fs[k+1], "%d", &v[k])
			}
			lab[fs[0]] = v
		}
	}
	var sb strings.Builder
	p := func(f string, a ...any) { fmt.Fprintf(&sb, f, a...) }
	names := []string{"B", "D", "E"}
	count := func(pred func(i int) bool) (n int, errs [3]int) {
		for i, k := range key {
			_ = k
			if pred(i) {
				n++
				v := lab[key[i].ID]
				for a := 0; a < 3; a++ {
					errs[a] += v[a]
				}
			}
		}
		return
	}
	for _, s := range []struct {
		name string
		pred func(i int) bool
	}{
		{"all", func(i int) bool { return true }},
		{"drift-group members", func(i int) bool { return key[i].Source == "drift-group member" }},
		{"Huang turn arrivals", func(i int) bool { return key[i].Source == "Huang turn arrival" }},
		{"arrivals in a group of arm E", func(i int) bool { return key[i].InGroup }},
		{"singletons of arm E", func(i int) bool { return !key[i].InGroup }},
	} {
		n, e := count(s.pred)
		p("%s: n %d;", s.name, n)
		for a := 0; a < 3; a++ {
			lo, hi := wilson(e[a], n)
			p(" %s errors %d (%.0f%%, %.0f to %.0f);", names[a], e[a], 100*float64(e[a])/float64(max(n, 1)), 100*lo, 100*hi)
		}
		p("\n")
	}
	must(os.WriteFile(filepath.Join(outDir, "e5-cutoff-score.txt"), []byte(sb.String()), 0o644))
	fmt.Print(sb.String())
}
