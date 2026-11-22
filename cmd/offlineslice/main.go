// Command offlineslice reads the replay's results (internal/platform
// TestOfflineSliceReplay) and prints rung 0's numbers, or writes the folder
// an independent reader judges.
//
//	offlineslice report -results results-asof.jsonl
//	offlineslice sample -results results-asof.jsonl -data DIR -out FOLDER -key KEYFOLDER [-n 100] [-seed 20261122]
//
// It reads local files only.
package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/offlineslice"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: offlineslice report|sample ...")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "report":
		fs := flag.NewFlagSet("report", flag.ExitOnError)
		in := fs.String("results", "", "results jsonl")
		_ = fs.Parse(os.Args[2:])
		report(load(*in))
	case "sample":
		fs := flag.NewFlagSet("sample", flag.ExitOnError)
		in := fs.String("results", "", "results jsonl")
		data := fs.String("data", "", "export directory")
		out := fs.String("out", "", "reader folder")
		key := fs.String("key", "", "key folder (not for the reader)")
		n := fs.Int("n", 100, "sample size")
		seed := fs.Int64("seed", 20261122, "seed")
		_ = fs.Parse(os.Args[2:])
		sample(load(*in), *data, *out, *key, *n, *seed)
	case "sampleb":
		fs := flag.NewFlagSet("sampleb", flag.ExitOnError)
		in := fs.String("results", "", "results-b jsonl")
		data := fs.String("data", "", "export directory")
		out := fs.String("out", "", "reader folder")
		key := fs.String("key", "", "key folder (not for the reader)")
		n := fs.Int("n", 100, "sample size")
		seed := fs.Int64("seed", 20261123, "seed")
		_ = fs.Parse(os.Args[2:])
		sampleB(*in, *data, *out, *key, *n, *seed)
	case "groups":
		fs := flag.NewFlagSet("groups", flag.ExitOnError)
		in := fs.String("results", "", "results-b jsonl (nearest story and oracle)")
		data := fs.String("data", "", "export directory")
		out := fs.String("out", "", "reader folder")
		key := fs.String("key", "", "key folder (not for the reader)")
		n := fs.Int("n", 50, "groups in the reader folder")
		seed := fs.Int64("seed", 20261124, "seed")
		_ = fs.Parse(os.Args[2:])
		groupsMain(*data, *in, *out, *key, *seed, *n)
	default:
		fmt.Fprintln(os.Stderr, "usage: offlineslice report|sample|sampleb|groups ...")
		os.Exit(2)
	}
}

func load(path string) []offlineslice.Result {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	var out []offlineslice.Result
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		var r offlineslice.Result
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			panic(err)
		}
		out = append(out, r)
	}
	return out
}

// counts of a result by source: distinct entities of A, of F (not already in A), distinct B keys, C subjects.
func counts(r offlineslice.Result) (a, f, b, c int) {
	seen := map[string]bool{}
	for _, x := range r.A {
		if !seen[x.EntityID] {
			seen[x.EntityID] = true
			a++
		}
	}
	for _, x := range r.F {
		if !seen[x.EntityID] {
			seen[x.EntityID] = true
			f++
		}
	}
	bs := map[string]bool{}
	for _, x := range r.B {
		bs[x.Key] = true
	}
	return a, f, len(bs), len(r.C)
}

type dist []int

func (d dist) String() string {
	if len(d) == 0 {
		return "n=0"
	}
	s := append([]int(nil), d...)
	sort.Ints(s)
	sum := 0
	for _, x := range s {
		sum += x
	}
	q := func(p float64) int { return s[int(p*float64(len(s)-1))] }
	zero := 0
	for _, x := range s {
		if x == 0 {
			zero++
		}
	}
	return fmt.Sprintf("mean %.2f  p10 %d p25 %d p50 %d p75 %d p90 %d p99 %d max %d  zero %.1f%%", float64(sum)/float64(len(s)), q(.1), q(.25), q(.5), q(.75), q(.9), q(.99), s[len(s)-1], 100*float64(zero)/float64(len(s)))
}

func report(rs []offlineslice.Result) {
	n := len(rs)
	var da, df, db, dc, dall, dtext dist
	byDay := map[string]*struct{ n, b, newB, zero, zeroText, hasVec int }{}
	var days []string
	vocab := map[string]bool{}
	topB := map[string]int{}
	topA := map[string]int{}
	topC := map[string]int{}
	label := map[string]string{}
	noMention, noVec := 0, 0
	for _, r := range rs {
		a, f, b, c := counts(r)
		da, df, db, dc = append(da, a), append(df, f), append(db, b), append(dc, c)
		all := a + f + b + c
		dall = append(dall, all)
		dtext = append(dtext, a+f+b)
		d := byDay[r.Day]
		if d == nil {
			d = &struct{ n, b, newB, zero, zeroText, hasVec int }{}
			byDay[r.Day] = d
			days = append(days, r.Day)
		}
		d.n++
		d.b += b
		if all == 0 {
			d.zero++
		}
		if a+f+b == 0 {
			d.zeroText++
		}
		if r.HasVec {
			d.hasVec++
		} else {
			noVec++
		}
		if r.Mentions == 0 {
			noMention++
		}
		seenB := map[string]bool{}
		for _, x := range r.B {
			if seenB[x.Key] {
				continue
			}
			seenB[x.Key] = true
			topB[x.Key]++
			label[x.Key] = x.Phrase
			if !vocab[x.Key] {
				vocab[x.Key] = true
				d.newB++
			}
		}
		seenA := map[string]bool{}
		for _, x := range append(append([]offlineslice.Cand{}, r.A...), r.F...) {
			if !seenA[x.EntityID] {
				seenA[x.EntityID] = true
				topA[x.EntityID]++
				label[x.EntityID] = x.Name
			}
		}
		for _, x := range r.C {
			topC[x.Key]++
			label[x.Key] = x.Label
		}
	}
	sort.Strings(days)
	fmt.Printf("captures replayed: %d (no vector: %d; no mention at all: %d = %.1f%%)\n\n", n, noVec, noMention, 100*float64(noMention)/float64(n))
	fmt.Println("candidates per capture (distinct):")
	fmt.Println("  (a) known label/alias  ", da)
	fmt.Println("  (a2) fuzzy to entities ", df)
	fmt.Println("  (b) names not known    ", db)
	fmt.Println("  (c) neighbours' subj.  ", dc)
	fmt.Println("  text only (a+a2+b)     ", dtext)
	fmt.Println("  all sources            ", dall)
	fmt.Println()
	fmt.Println("by day: captures | zero candidates (all) | zero from text alone | (b) per 1000 | first-seen (b) names per 1000 | share with vector")
	for _, day := range days {
		d := byDay[day]
		fmt.Printf("  %s %6d | %5.1f%% | %5.1f%% | %7.0f | %7.0f | %5.1f%%\n", day, d.n, 100*float64(d.zero)/float64(d.n), 100*float64(d.zeroText)/float64(d.n),
			1000*float64(d.b)/float64(d.n), 1000*float64(d.newB)/float64(d.n), 100*float64(d.hasVec)/float64(d.n))
	}
	top := func(title string, m map[string]int) {
		type kv struct {
			k string
			v int
		}
		var l []kv
		for k, v := range m {
			l = append(l, kv{k, v})
		}
		sort.Slice(l, func(i, j int) bool {
			if l[i].v != l[j].v {
				return l[i].v > l[j].v
			}
			return l[i].k < l[j].k
		})
		fmt.Printf("\n%s\n", title)
		for i := 0; i < 30 && i < len(l); i++ {
			fmt.Printf("  %6d  %s\n", l[i].v, label[l[i].k])
		}
	}
	top("30 most-proposed (b) names (captures proposing)", topB)
	top("30 most-found known entities (a/a2) (captures)", topA)
	top("30 most-offered neighbour subjects (c) (captures)", topC)
}

func sample(rs []offlineslice.Result, dataDir, out, keyDir string, n int, seed int64) {
	hashes := map[string]string{}
	f, err := os.Open(filepath.Join(dataDir, "captures.csv"))
	if err != nil {
		panic(err)
	}
	r := csv.NewReader(bufio.NewReaderSize(f, 1<<20))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		hashes[rec[0]] = strings.TrimPrefix(rec[9], "sha256:")
	}
	f.Close()
	var pool []int
	for i, x := range rs {
		if x.Mentions > 0 {
			pool = append(pool, i)
		}
	}
	rng := rand.New(rand.NewSource(seed))
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	if n > len(pool) {
		n = len(pool)
	}
	chosen := pool[:n]
	need := map[string]string{} // hash -> capture id
	for _, i := range chosen {
		need[hashes[rs[i].ID]] = rs[i].ID
	}
	texts := readTexts(filepath.Join(dataDir, "bodies.bin"), need)
	_ = os.MkdirAll(filepath.Join(out, "captures"), 0o755)
	_ = os.MkdirAll(keyDir, 0o755)
	type keyRow struct {
		Case       string              `json:"case"`
		CaptureID  string              `json:"capture_id"`
		Day        string              `json:"day"`
		Candidates []map[string]any    `json:"candidates"`
		Result     offlineslice.Result `json:"-"`
	}
	var keys []keyRow
	for k, i := range chosen {
		x := rs[i]
		id := fmt.Sprintf("%03d", k+1)
		type cand struct {
			label, src, detail string
		}
		var cs []cand
		seen := map[string]bool{}
		add := func(label, src, detail string) {
			l := strings.ToLower(strings.TrimSpace(label))
			if l == "" || seen[l] {
				return
			}
			seen[l] = true
			cs = append(cs, cand{label, src, detail})
		}
		for _, c := range x.A {
			add(c.Name, "a", "known label '"+c.Mention+"' ("+c.Via+")")
		}
		for _, c := range x.F {
			add(c.Name, "a2", "fuzzy from '"+c.Mention+"' ("+c.Via+")")
		}
		for _, c := range x.B {
			add(c.Phrase, "b", "new name")
		}
		for _, c := range x.C {
			add(c.Label, "c", fmt.Sprintf("neighbours %d, best cos %.2f", c.Support, c.CosBest))
		}
		// the reader sees labels in an order that carries no source: alphabetical
		sort.Slice(cs, func(a, b int) bool { return strings.ToLower(cs[a].label) < strings.ToLower(cs[b].label) })
		text := texts[need[hashes[x.ID]]]
		if r := []rune(text); len(r) > 3000 {
			text = string(r[:3000])
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "# Capture %s\n\n**Title:** %s\n\n## Text (first 3,000 characters)\n\n%s\n\n## Candidate subjects\n\n", id, x.Title, text)
		if len(cs) == 0 {
			sb.WriteString("(none)\n")
		}
		kr := keyRow{Case: id, CaptureID: x.ID, Day: x.Day}
		for _, c := range cs {
			fmt.Fprintf(&sb, "- %s\n", c.label)
			kr.Candidates = append(kr.Candidates, map[string]any{"label": c.label, "source": c.src, "detail": c.detail})
		}
		keys = append(keys, kr)
		if err := os.WriteFile(filepath.Join(out, "captures", id+".md"), []byte(sb.String()), 0o644); err != nil {
			panic(err)
		}
	}
	kb, _ := json.MarshalIndent(keys, "", " ")
	_ = os.WriteFile(filepath.Join(keyDir, "key.json"), kb, 0o644)
	var tpl strings.Builder
	tpl.WriteString("# Answers\n\nFor each case, name the two or three subjects the capture is mainly about, in your own words, and after each, one of:\n`EXACT` (a candidate has that name), `OTHER NAME: <the candidate>` (a candidate is the same thing under another name), `ABSENT` (nothing in the list is it).\nIf you cannot tell what a capture is about, write `NO SUBJECT`.\n\n")
	for k := range chosen {
		fmt.Fprintf(&tpl, "## %03d\n\n1. \n2. \n3. \n\n", k+1)
	}
	_ = os.WriteFile(filepath.Join(out, "answers.md"), []byte(tpl.String()), 0o644)
	instr := `# Reading task: do the candidate lists contain what each capture is about?

You have ` + fmt.Sprint(n) + ` news items in captures/ (001.md to ` + fmt.Sprintf("%03d", n) + `.md). Each has a title, the first 3,000 characters of its text, and a list of
candidate subjects that a program proposed. You did not write the program and you should not guess how it works.

For each capture, in this order:

1. Read the title and text. Without looking at the candidate list, decide what the capture is mainly about:
   the two or three subjects (a person, an organisation, a place, an event, a law, a product) a reader would
   say it is about. Write them in your own words in answers.md, under that capture's number.
2. Then read the candidate list, and after each subject write exactly one of:
   - EXACT: a candidate has that name.
   - OTHER NAME: <the candidate>: a candidate is the same thing under another name (say which).
   - ABSENT: nothing in the list is that subject.
3. If a capture has no clear subject (a list page, a menu, a paywall notice, an error page), write NO SUBJECT.

Rules: judge only from the title and text shown. Do not look anything up. Do not change a subject after you have seen
the list. A candidate that is only a part of a name, or a more general or more specific thing than your subject, is
ABSENT, not OTHER NAME, unless a reasonable editor would call the two the same. Work through the captures in numerical order.
Fill in answers.md (one file). When done, add one line at the bottom: how many captures you judged NO SUBJECT.
`
	_ = os.WriteFile(filepath.Join(out, "INSTRUCTIONS.md"), []byte(instr), 0o644)
	fmt.Printf("wrote %d captures to %s; key in %s\n", n, out, keyDir)
}

func readTexts(path string, need map[string]string) map[string]string {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	br := bufio.NewReaderSize(f, 1<<20)
	out := map[string]string{}
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return out
		}
		var h string
		var sz int
		if _, e := fmt.Sscanf(line, "H %s %d", &h, &sz); e != nil {
			panic(e)
		}
		if sz < 0 {
			continue
		}
		buf := make([]byte, sz)
		if _, err := readFull(br, buf); err != nil {
			return out
		}
		if _, ok := need[h]; ok {
			out[need[h]] = strings.ToValidUTF8(string(buf), "�")
		}
	}
}

func readFull(br *bufio.Reader, b []byte) (int, error) {
	n := 0
	for n < len(b) {
		m, err := br.Read(b[n:])
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
