package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// E5 arm D digest (ledger C238): built by plain code, once per clock hour of
// 2026-11-23, from what the paper held at the start of the hour. No model call.

var digestStop = map[string]bool{"article url": true, "comments url": true, "news": true, "world news": true, "url": true, "comments": true}

type dver struct {
	ID  string
	Pub time.Time
}
type dstory struct {
	ID, Title string
	First     time.Time
	Vers      []dver
}

var reMD = regexp.MustCompile(`[#*_>\[\]]|\(https?://[^)]*\)`)

func firstSentence(text, title string) string {
	text = strings.TrimSpace(text)
	if i := strings.Index(text, "\n\n"); i >= 0 && strings.HasPrefix(text, "#") {
		text = text[i+2:]
	}
	text = reMD.ReplaceAllString(text, "")
	text = strings.Join(strings.Fields(text), " ")
	r := []rune(text)
	end := len(r)
	for i := 40; i < len(r) && i < 400; i++ {
		if (r[i] == '.' || r[i] == '!' || r[i] == '?') && (i+1 == len(r) || r[i+1] == ' ') {
			end = i + 1
			break
		}
	}
	if end > 160 {
		end = 160
	}
	return string(r[:end])
}

func runE5Digest(dir, out string) {
	must(os.MkdirAll(out, 0o755))
	arr, _ := loadArrivals(dir)
	kept := keptSubjects(arr)
	// stories and versions
	stories := map[string]*dstory{}
	var order []*dstory
	seen := map[string]bool{}
	readCSV(filepath.Join(dir, "digest-versions.csv"), func(r []string) {
		if len(r) < 6 || seen[r[1]] {
			return
		}
		seen[r[1]] = true
		pub := parseTime(r[3])
		if pub.IsZero() {
			return
		}
		s := stories[r[0]]
		if s == nil {
			s = &dstory{ID: r[0], Title: r[4], First: parseTime(r[5])}
			stories[r[0]] = s
			order = append(order, s)
		}
		s.Vers = append(s.Vers, dver{r[1], pub})
	})
	for _, s := range order {
		sort.Slice(s.Vers, func(i, j int) bool { return s.Vers[i].Pub.Before(s.Vers[j].Pub) })
		s.First = s.Vers[0].Pub
	}
	// version text
	vtitle, vtext := map[string]string{}, map[string]string{}
	f, err := os.Open(filepath.Join(dir, "digest-story-text.jsonl"))
	must(err)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		var m struct{ VersionID, Title, Text string }
		var raw map[string]string
		if json.Unmarshal(sc.Bytes(), &raw) == nil {
			m.VersionID, m.Title, m.Text = raw["version_id"], raw["title"], raw["text"]
			vtitle[m.VersionID], vtext[m.VersionID] = m.Title, m.Text
		}
	}
	f.Close()
	// entities and story-entity links
	ename := map[string]string{}
	readCSV(filepath.Join(dir, "digest-entities.csv"), func(r []string) {
		if len(r) < 3 {
			return
		}
		var m struct {
			Name string `json:"name"`
		}
		if json.Unmarshal([]byte(r[2]), &m) == nil && m.Name != "" {
			ename[r[0]] = m.Name
		}
	})
	type link struct {
		ent     string
		created time.Time
	}
	links := map[string][]link{}
	readCSV(filepath.Join(dir, "digest-story-entities.csv"), func(r []string) {
		if len(r) >= 4 && r[2] == "central" {
			links[r[0]] = append(links[r[0]], link{r[1], parseTime(r[3])})
		}
	})
	digests := map[string]string{}
	var sizes []int
	for h := 0; h < 24; h++ {
		h0 := time.Date(2026, 11, 23, h, 0, 0, 0, time.UTC)
		var b strings.Builder
		w := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }
		// layer one: the front
		type fr struct {
			s *dstory
			v dver
		}
		var front []fr
		for _, s := range order {
			k := sort.Search(len(s.Vers), func(i int) bool { return s.Vers[i].Pub.After(h0) })
			if k > 0 {
				front = append(front, fr{s, s.Vers[k-1]})
			}
		}
		sort.Slice(front, func(i, j int) bool { return front[i].v.Pub.After(front[j].v.Pub) })
		if len(front) > 60 {
			front = front[:60]
		}
		w("LAYER 1, NOW. The paper's front page at %s UTC, newest first (title: first line):\n", h0.Format("2006-01-02 15:04"))
		for i, x := range front {
			t := vtitle[x.v.ID]
			if t == "" {
				t = x.s.Title
			}
			w("%d. %s: %s\n", i+1, trunc(t, 110), firstSentence(vtext[x.v.ID], t))
		}
		// subjects, last 24 hours
		origins := map[string]map[string]bool{}
		for i := range arr {
			if arr[i].Fetched.After(h0.Add(-24*time.Hour)) && !arr[i].Fetched.After(h0) {
				for _, k := range kept[i] {
					if digestStop[k] {
						continue
					}
					if origins[k] == nil {
						origins[k] = map[string]bool{}
					}
					origins[k][arr[i].Origin] = true
				}
			}
		}
		type kv struct {
			k string
			n int
		}
		var kvs []kv
		for k, v := range origins {
			kvs = append(kvs, kv{k, len(v)})
		}
		sort.Slice(kvs, func(i, j int) bool {
			if kvs[i].n != kvs[j].n {
				return kvs[i].n > kvs[j].n
			}
			return kvs[i].k < kvs[j].k
		})
		w("\nMost-reported subjects in the last 24 hours (distinct sources): ")
		for i, x := range kvs {
			if i >= 20 {
				break
			}
			w("%s (%d); ", x.k, x.n)
		}
		w("\n")
		// layer two: months; stories whose every version is at or before h0
		w("\nLAYER 2, SINCE YOUR TRAINING CUTOFF. The paper's record starts 2026-10-13. Per month: most-covered subjects (by number of stories centrally about them) and the stories with the most revisions. Finished stories only; the newest are in layer 1.\n")
		for _, m := range []struct {
			label string
			from  time.Time
			nSub  int
			nTit  int
		}{{"2026-11", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), 15, 12}, {"2026-10", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), 10, 8}} {
			to := m.from.AddDate(0, 1, 0)
			var closed []*dstory
			for _, s := range order {
				if s.First.Before(m.from) || !s.First.Before(to) || s.Vers[len(s.Vers)-1].Pub.After(h0) {
					continue
				}
				closed = append(closed, s)
			}
			cnt := map[string]int{}
			for _, s := range closed {
				seenE := map[string]bool{}
				for _, l := range links[s.ID] {
					if !l.created.After(h0) && ename[l.ent] != "" && !seenE[l.ent] {
						seenE[l.ent] = true
						cnt[ename[l.ent]]++
					}
				}
			}
			var es []kv
			for k, n := range cnt {
				es = append(es, kv{k, n})
			}
			sort.Slice(es, func(i, j int) bool {
				if es[i].n != es[j].n {
					return es[i].n > es[j].n
				}
				return es[i].k < es[j].k
			})
			sort.SliceStable(closed, func(i, j int) bool { return len(closed[i].Vers) > len(closed[j].Vers) })
			w("%s (%d finished stories): subjects: ", m.label, len(closed))
			for i, x := range es {
				if i >= m.nSub {
					break
				}
				w("%s (%d); ", x.k, x.n)
			}
			w("\n  stories: ")
			for i, s := range closed {
				if i >= m.nTit {
					break
				}
				w("%s; ", trunc(s.Title, 90))
			}
			w("\n")
		}
		digests[h0.Format("2006-01-02T15")] = b.String()
		sizes = append(sizes, len([]rune(b.String())))
	}
	jb, _ := json.Marshal(digests)
	must(os.WriteFile(filepath.Join(out, "e5-digests.json"), jb, 0o644))
	must(os.WriteFile(filepath.Join(out, "e5-digest-19.txt"), []byte(digests["2026-11-23T19"]), 0o644))
	mn, mx, sum := sizes[0], sizes[0], 0
	for _, s := range sizes {
		sum += s
		if s < mn {
			mn = s
		}
		if s > mx {
			mx = s
		}
	}
	fmt.Printf("digests: 24; characters min %d mean %d max %d (about %d tokens at 4 characters a token)\n", mn, sum/24, mx, sum/24/4)
}
