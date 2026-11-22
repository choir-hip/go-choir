package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/jevbudget"
	"github.com/yusefmosiah/go-choir/internal/offlineslice"
)

func readResultsB(path string) []offlineslice.ResultB {
	f, err := os.Open(path)
	must(err)
	defer f.Close()
	var out []offlineslice.ResultB
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		var r offlineslice.ResultB
		must(json.Unmarshal(sc.Bytes(), &r))
		out = append(out, r)
	}
	return out
}

func clipRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func s1Items(rs []offlineslice.ResultB, k int) []offlineslice.S1Item {
	var out []offlineslice.S1Item
	for _, r := range rs {
		it := offlineslice.S1Item{ID: r.ID, Title: strings.TrimSpace(r.Title), Text: clipRunes(offlineslice.CleanText(r.Lead), 400)}
		for i, s := range r.Stories {
			if i >= k {
				break
			}
			it.Stories = append(it.Stories, offlineslice.S1Story{ID: s.StoryID, Title: s.Title, Summary: s.Dek})
		}
		out = append(out, it)
	}
	return out
}

func writeJobs(path string, jobs []offlineslice.S1Job) (tokens int) {
	f, err := os.Create(path)
	must(err)
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, j := range jobs {
		tokens += jevbudget.Measure(j.State, j.Questions).Request
		b, _ := json.Marshal(j)
		w.Write(b)
		w.WriteByte('\n')
	}
	must(w.Flush())
	return
}

// s1jobs builds the rung 1 requests: every arrival in as few calls as the
// vendor's limits allow (the main run) and, for the same-captures comparison,
// a uniform sample of days 4-7 in calls of at most 12.
func s1jobsMain(resultsPath, outDir string, seed int64, nSmall int) {
	rs := readResultsB(resultsPath)
	main, err := offlineslice.BuildS1Jobs(s1Items(rs, 5), 0)
	must(err)
	tk := writeJobs(outDir+"/s1-jobs-main.jsonl", main)
	var late []offlineslice.ResultB
	for _, r := range rs {
		if r.Day >= "2026-11-18" {
			late = append(late, r)
		}
	}
	rng := rand.New(rand.NewSource(seed))
	rng.Shuffle(len(late), func(i, j int) { late[i], late[j] = late[j], late[i] })
	if nSmall > len(late) {
		nSmall = len(late)
	}
	sample := late[:nSmall]
	sort.Slice(sample, func(i, j int) bool { return sample[i].Fetched.Before(sample[j].Fetched) })
	small, err := offlineslice.BuildS1Jobs(s1Items(sample, 5), 12)
	must(err)
	tk2 := writeJobs(outDir+"/s1-jobs-small.jsonl", small)
	fmt.Printf("main: %d arrivals in %d calls, estimated %d input tokens (jevbudget estimate, an upper bound)\nsmall (12 a call): %d arrivals in %d calls, estimated %d input tokens\n", len(rs), len(main), tk, len(sample), len(small), tk2)
	var sizes []int
	for _, j := range main {
		sizes = append(sizes, len(j.Items))
	}
	fmt.Println("arrivals per main call:", distInt(sizes))
}

// s1eval joins answers to the oracle.
func s1eval(resultsPath, jobsPath, respPath string) {
	rs := readResultsB(resultsPath)
	byID := map[string]offlineslice.ResultB{}
	for _, r := range rs {
		byID[r.ID] = r
	}
	jobs := map[string]offlineslice.S1Job{}
	f, err := os.Open(jobsPath)
	must(err)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		var j offlineslice.S1Job
		must(json.Unmarshal(sc.Bytes(), &j))
		jobs[j.ID] = j
	}
	f.Close()
	type resp struct {
		Job     string          `json:"job"`
		Status  int             `json:"status"`
		Seconds float64         `json:"seconds"`
		Body    json.RawMessage `json:"body"`
		Err     string          `json:"err"`
	}
	rf, err := os.Open(respPath)
	must(err)
	defer rf.Close()
	rsc := bufio.NewScanner(rf)
	rsc.Buffer(make([]byte, 1<<20), 1<<27)
	type row struct {
		r    offlineslice.ResultB
		a    offlineslice.S1Answer
		call string
	}
	var rows []row
	var calls, failed int
	var cost, secs float64
	var tin, tout int
	for rsc.Scan() {
		var x resp
		must(json.Unmarshal(rsc.Bytes(), &x))
		calls++
		secs += x.Seconds
		if x.Status != 200 {
			failed++
			continue
		}
		res, err := offlineslice.ParseS1(jobs[x.Job], x.Body)
		must(err)
		cost += res.Cost
		tin += res.InputTokens
		tout += res.OutputTokens
		for id, a := range res.Answers {
			rows = append(rows, row{r: byID[id], a: a, call: x.Job})
		}
	}
	fmt.Printf("calls %d (failed %d), arrivals answered %d, input tokens %d, output tokens %d, cost $%.4f, total call seconds %.0f (%.2f s a call, %.1f ms an arrival)\n",
		calls, failed, len(rows), tin, tout, cost, secs, secs/float64(max(calls, 1)), 1000*secs/float64(max(len(rows), 1)))
	if len(rows) > 0 {
		fmt.Printf("cost per 1,000 arrivals $%.4f\n", 1000*cost/float64(len(rows)))
	}
	report := func(name string, sel func(row) bool) {
		var n, story, other, idk, missing, lowConf int
		var fedN, fedReach, fedRight, fedWrong, fedOther int
		var newN, newPickedExisting int
		var noneN, nonePicked int
		var pickedN, pickedCited int
		for _, x := range rows {
			if !sel(x) {
				continue
			}
			n++
			switch x.a.Verdict {
			case "story":
				story++
				if x.a.Confidence < 0.5 {
					lowConf++
				}
			case "other":
				other++
			case "idk":
				idk++
			default:
				missing++
			}
			cited := map[string]bool{}
			for _, c := range x.r.CitedStory {
				cited[c] = true
			}
			switch x.r.Oracle {
			case "fed":
				fedN++
				reach := false
				for i, s := range x.r.Stories {
					if i < 5 && cited[s.StoryID] {
						reach = true
					}
				}
				if reach {
					fedReach++
				}
				switch {
				case x.a.Verdict == "story" && cited[x.a.Story]:
					fedRight++
				case x.a.Verdict == "story":
					fedWrong++
				default:
					fedOther++
				}
				if x.a.Verdict == "story" {
					pickedN++
					if cited[x.a.Story] {
						pickedCited++
					}
				}
			case "new":
				newN++
				if x.a.Verdict == "story" {
					newPickedExisting++
				}
			default:
				noneN++
				if x.a.Verdict == "story" {
					nonePicked++
				}
			}
		}
		fmt.Printf("\n[%s] arrivals %d: story %s, other %s, idk %s, missing %d; escalated (other+idk+missing) %s; story picks with confidence < 0.5: %s of picks\n", name, n, pc(story, n), pc(other, n), pc(idk, n), missing, pc(other+idk+missing, n), pc(lowConf, story))
		fmt.Printf("  fed arrivals %d (the cited story is among the 5 offered for %d = %s): picked the cited story %d (%s of fed; recall of reachable %s); picked another story %d; answered other/idk %d\n",
			fedN, fedReach, pc(fedReach, fedN), fedRight, pc(fedRight, fedN), pc(fedRight, fedReach), fedWrong, fedOther)
		fmt.Printf("  precision lower bound on picks among fed arrivals: %s (%d of %d); on arrivals cited only by a later story (%d): picked an existing story %s; on arrivals no story cites (%d): picked a story %s\n",
			pc(pickedCited, pickedN), pickedCited, pickedN, newN, pc(newPickedExisting, newN), noneN, pc(nonePicked, noneN))
	}
	report("all", func(row) bool { return true })
	for _, d := range []string{"2026-11-15", "2026-11-16", "2026-11-17", "2026-11-18", "2026-11-19", "2026-11-20", "2026-11-21"} {
		var n, esc int
		for _, x := range rows {
			if x.r.Day == d {
				n++
				if x.a.Verdict != "story" {
					esc++
				}
			}
		}
		fmt.Printf("  %s escalated %s of %d\n", d, pc(esc, n), n)
	}
	report("days 4-7", func(x row) bool { return x.r.Day >= "2026-11-18" })
}
