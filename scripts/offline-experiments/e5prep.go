package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// E5 preparation (ledger C232): the day's arrivals in time order with, for
// each, its ten nearest earlier arrivals of the day and the titles of its three
// nearest published stories at that moment; and the seeded 100-arrival sample.

type e5Item struct {
	ID        string   `json:"id"`
	Idx       int      `json:"idx"`
	Time      string   `json:"time"`
	Origin    string   `json:"origin"`
	Title     string   `json:"title"`
	Text      string   `json:"text"`
	Neighbors []string `json:"neighbors"` // ids of the ten nearest earlier arrivals of the day
	Stories   []string `json:"stories"`   // titles of the three nearest stories
	Huang     bool     `json:"huang,omitempty"`
	Echo      bool     `json:"echo,omitempty"`
}

func runE5Prep(dir, out string) {
	must(os.MkdirAll(out, 0o755))
	d, _ := loadE1(dir)
	arr := d.arr
	day := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	var idx []int
	for i := range arr {
		if !arr[i].Fetched.Before(day) {
			idx = append(idx, i)
		}
	}
	var items []e5Item
	for pos, i := range idx {
		a := &arr[i]
		t := a.Fetched
		text := []rune(strings.Join(strings.Fields(a.Lead), " "))
		if len(text) > 600 {
			text = text[:600]
		}
		it := e5Item{ID: fmt.Sprintf("a%04d", pos), Idx: i, Time: t.Format("2006-01-02T15:04:05Z"), Origin: a.Origin,
			Title: strings.TrimSpace(a.Title), Text: string(text), Huang: isHuang(a)}
		type nb struct {
			p   int
			cos float64
		}
		var nbs []nb
		for p2 := 0; p2 < pos; p2++ {
			nbs = append(nbs, nb{p2, dot(a.Vec, arr[idx[p2]].Vec)})
		}
		sort.Slice(nbs, func(x, y int) bool { return nbs[x].cos > nbs[y].cos })
		for k := 0; k < 10 && k < len(nbs); k++ {
			it.Neighbors = append(it.Neighbors, fmt.Sprintf("a%04d", nbs[k].p))
		}
		if len(nbs) > 0 && nbs[0].cos >= e4EchoCos {
			it.Echo = true
		}
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
				if c := dot(a.Vec, p); c > best {
					best = c
				}
			}
			if best > -2 {
				scs = append(scs, sc{s, best})
			}
		}
		sort.Slice(scs, func(x, y int) bool { return scs[x].cos > scs[y].cos })
		for k := 0; k < 3 && k < len(scs); k++ {
			it.Stories = append(it.Stories, trunc(scs[k].s.Title, 120))
		}
		items = append(items, it)
	}
	f, err := os.Create(filepath.Join(out, "e5-input.jsonl"))
	must(err)
	enc := json.NewEncoder(f)
	for _, it := range items {
		must(enc.Encode(it))
	}
	must(f.Close())
	// the 100-arrival sample: 12:00 to 20:00 UTC, seed 20261124
	var pool []string
	for _, it := range items {
		if it.Time >= "2026-11-23T12:00:00Z" && it.Time < "2026-11-23T20:00:00Z" {
			pool = append(pool, it.ID)
		}
	}
	r := rand.New(rand.NewSource(20261124))
	r.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	sample := pool[:100]
	sort.Strings(sample)
	must(os.WriteFile(filepath.Join(out, "e5-sample100.txt"), []byte(strings.Join(sample, "\n")+"\n"), 0o644))
	fmt.Printf("e5 input: %d items; sample 100 of %d in 12:00-20:00\n", len(items), len(pool))
}
