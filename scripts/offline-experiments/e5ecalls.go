package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// E5 arm E preparation (ledger C241): the grouped fan-out. Per clock hour,
// vector groups joined by shared subjects; a group is one call; singletons are
// pooled into mixed calls of 25. Also the structure the digest slice is cut from.

type e5Call struct {
	Call    int      `json:"call"`
	Hour    int      `json:"hour"`
	Kind    string   `json:"kind"` // group | pool
	IDs     []string `json:"ids"`
	Terms   []string `json:"terms"`
	Stories []string `json:"stories"` // "title: one-line point"
	At      string   `json:"at"`      // time of the call's last arrival
}

type e5DigestStruct struct {
	Front    []string            `json:"front"`
	Subj     map[string]int      `json:"subj"`
	Stories  map[string][]string `json:"month_stories"`
	Subjects map[string][]string `json:"month_subjects"`
}

func runE5ECalls(dir, out string) {
	must(os.MkdirAll(out, 0o755))
	dd := loadDigestData(dir)
	d, _ := loadE1(dir)
	arr, kept := dd.arr, dd.kept
	items := loadItems(filepath.Join(out, "e5-input.jsonl"))
	posID := map[int]string{}
	for _, it := range items {
		posID[it.Idx] = it.ID
	}
	day := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	byHour := map[int][]int{}
	for i := range arr {
		if !arr[i].Fetched.Before(day) {
			h := int(arr[i].Fetched.Add(-time.Nanosecond).Sub(day) / time.Hour)
			byHour[h] = append(byHour[h], i)
		}
	}
	var calls []e5Call
	structs := map[string]e5DigestStruct{}
	nGroupCalls, nPool := 0, 0
	for h := 0; h < 24; h++ {
		idx := byHour[h]
		if len(idx) == 0 {
			continue
		}
		// digest structure at the start of the hour
		h0 := day.Add(time.Duration(h) * time.Hour)
		structs[h0.Format("2006-01-02T15")] = digestStructAt(dd, h0)
		// vector groups of the hour
		gs := groupArrivals(arr, idx)
		parent := map[int]int{}
		var find func(x int) int
		find = func(x int) int {
			if parent[x] == x {
				return x
			}
			parent[x] = find(parent[x])
			return parent[x]
		}
		for _, i := range idx {
			parent[i] = i
		}
		for _, g := range gs {
			for _, m := range g.Members[1:] {
				parent[find(m)] = find(g.Members[0])
			}
		}
		// joins by two shared subjects and cosine 0.6
		subjIdx := map[string][]int{}
		for _, i := range idx {
			for _, k := range kept[i] {
				subjIdx[k] = append(subjIdx[k], i)
			}
		}
		shared := map[[2]int]int{}
		for _, ms := range subjIdx {
			if len(ms) > 60 {
				continue // a subject held by most of the hour joins nothing
			}
			for x := 0; x < len(ms); x++ {
				for y := x + 1; y < len(ms); y++ {
					shared[[2]int{ms[x], ms[y]}]++
				}
			}
		}
		for p, c := range shared {
			if c >= 2 && dot(arr[p[0]].Vec, arr[p[1]].Vec) >= 0.60 {
				parent[find(p[0])] = find(p[1])
			}
		}
		comps := map[int][]int{}
		for _, i := range idx {
			comps[find(i)] = append(comps[find(i)], i)
		}
		var groups [][]int
		var singles []int
		for _, ms := range comps {
			sort.Ints(ms)
			if len(ms) == 1 {
				singles = append(singles, ms[0])
				continue
			}
			for len(ms) > 25 {
				groups = append(groups, ms[:25])
				ms = ms[25:]
			}
			groups = append(groups, ms)
		}
		sort.Slice(groups, func(x, y int) bool { return groups[x][0] < groups[y][0] })
		sort.Ints(singles)
		mk := func(ms []int, kind string) e5Call {
			c := e5Call{Hour: h + 1, Kind: kind, At: arr[ms[len(ms)-1]].Fetched.Format("2006-01-02T15:04:05Z")}
			cnt := map[string]int{}
			sum := make([]float32, len(arr[ms[0]].Vec))
			for _, m := range ms {
				c.IDs = append(c.IDs, posID[m])
				for _, k := range kept[m] {
					cnt[k]++
				}
				for k, x := range arr[m].Vec {
					sum[k] += x
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
			sort.Slice(kvs, func(a, b int) bool { return kvs[a].n > kvs[b].n || (kvs[a].n == kvs[b].n && kvs[a].k < kvs[b].k) })
			for k := 0; k < 15 && k < len(kvs); k++ {
				c.Terms = append(c.Terms, kvs[k].k)
			}
			if kind == "group" {
				cen := normalize(sum)
				t := arr[ms[len(ms)-1]].Fetched
				type sc struct {
					s   *story
					cos float64
				}
				var scs []sc
				for _, s := range d.stories {
					v := latestAt(s, t)
					if v == nil {
						continue
					}
					best := -2.0
					for _, p := range d.passages[v.ID] {
						if x := dot(cen, p); x > best {
							best = x
						}
					}
					scs = append(scs, sc{s, best})
				}
				sort.Slice(scs, func(a, b int) bool { return scs[a].cos > scs[b].cos })
				for k := 0; k < 3 && k < len(scs); k++ {
					v := latestAt(scs[k].s, t)
					title := dd.vtitle[v.ID]
					if title == "" {
						title = scs[k].s.Title
					}
					c.Stories = append(c.Stories, trunc(title, 110)+": "+firstSentence(dd.vtext[v.ID], title))
				}
			}
			return c
		}
		var hourCalls []e5Call
		for _, g := range groups {
			hourCalls = append(hourCalls, mk(g, "group"))
			nGroupCalls++
		}
		for len(singles) > 0 {
			n := min(25, len(singles))
			hourCalls = append(hourCalls, mk(singles[:n], "pool"))
			singles = singles[n:]
			nPool++
		}
		sort.SliceStable(hourCalls, func(x, y int) bool { return hourCalls[x].At < hourCalls[y].At })
		calls = append(calls, hourCalls...)
	}
	for i := range calls {
		calls[i].Call = i
	}
	f, err := os.Create(filepath.Join(out, "e5-ecalls.jsonl"))
	must(err)
	enc := json.NewEncoder(f)
	nIDs := 0
	for _, c := range calls {
		must(enc.Encode(c))
		nIDs += len(c.IDs)
	}
	must(f.Close())
	jb, _ := json.Marshal(structs)
	must(os.WriteFile(filepath.Join(out, "e5-digests-struct.json"), jb, 0o644))
	fmt.Printf("arm E: %d calls (%d group calls, %d pooled calls) for %d arrivals; %.1f calls an hour\n", len(calls), nGroupCalls, nPool, nIDs, float64(len(calls))/24)
}

func digestStructAt(dd *digestData, h0 time.Time) e5DigestStruct {
	st := e5DigestStruct{Subj: map[string]int{}, Stories: map[string][]string{}, Subjects: map[string][]string{}}
	type fr struct {
		s *dstory
		v dver
	}
	var front []fr
	for _, s := range dd.order {
		k := sort.Search(len(s.Vers), func(i int) bool { return s.Vers[i].Pub.After(h0) })
		if k > 0 {
			front = append(front, fr{s, s.Vers[k-1]})
		}
	}
	sort.Slice(front, func(i, j int) bool { return front[i].v.Pub.After(front[j].v.Pub) })
	if len(front) > 60 {
		front = front[:60]
	}
	for _, x := range front {
		t := dd.vtitle[x.v.ID]
		if t == "" {
			t = x.s.Title
		}
		st.Front = append(st.Front, fmt.Sprintf("%s: %s", trunc(t, 110), firstSentence(dd.vtext[x.v.ID], t)))
	}
	origins := map[string]map[string]bool{}
	for i := range dd.arr {
		if dd.arr[i].Fetched.After(h0.Add(-24*time.Hour)) && !dd.arr[i].Fetched.After(h0) {
			for _, k := range dd.kept[i] {
				if digestStop[k] {
					continue
				}
				if origins[k] == nil {
					origins[k] = map[string]bool{}
				}
				origins[k][dd.arr[i].Origin] = true
			}
		}
	}
	for k, v := range origins {
		st.Subj[k] = len(v)
	}
	for _, m := range []struct {
		label string
		from  time.Time
	}{{"2026-11", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}, {"2026-10", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}} {
		to := m.from.AddDate(0, 1, 0)
		var closed []*dstory
		for _, s := range dd.order {
			if s.First.Before(m.from) || !s.First.Before(to) || s.Vers[len(s.Vers)-1].Pub.After(h0) {
				continue
			}
			closed = append(closed, s)
		}
		sort.SliceStable(closed, func(i, j int) bool { return len(closed[i].Vers) > len(closed[j].Vers) })
		for i, s := range closed {
			if i >= 60 {
				break
			}
			st.Stories[m.label] = append(st.Stories[m.label], trunc(s.Title, 90))
		}
	}
	return st
}

var _ = strings.Join
