package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/offlineslice"
)

// newsLike: the arrivals a reader can judge as news. RSS and official items
// are; a social post is when its text links an article (a post that states an
// event with no link cannot be told from chatter by a rule); arXiv is not.
func newsLike(r offlineslice.ResultB) bool {
	switch r.Class {
	case "rss", "official":
		return true
	case "social":
		return strings.Contains(r.Lead, "http://") || strings.Contains(r.Lead, "https://") || strings.Contains(r.Lead, "www.")
	}
	return false
}

func sampleB(in, dataDir, out, keyDir string, n int, seed int64) {
	f, err := os.Open(in)
	if err != nil {
		panic(err)
	}
	var pool []offlineslice.ResultB
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		var r offlineslice.ResultB
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			panic(err)
		}
		if r.Day >= "2026-11-18" && newsLike(r) {
			pool = append(pool, r)
		}
	}
	f.Close()
	rng := rand.New(rand.NewSource(seed))
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	if n > len(pool) {
		n = len(pool)
	}
	chosen := pool[:n]
	hashes := map[string]string{}
	cf, _ := os.Open(filepath.Join(dataDir, "captures.csv"))
	cr := csv.NewReader(bufio.NewReaderSize(cf, 1<<20))
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	for {
		rec, err := cr.Read()
		if err != nil {
			break
		}
		hashes[rec[0]] = strings.TrimPrefix(rec[9], "sha256:")
	}
	cf.Close()
	need := map[string]string{}
	for _, r := range chosen {
		need[hashes[r.ID]] = r.ID
	}
	texts := readTexts(filepath.Join(dataDir, "bodies.bin"), need)
	_ = os.MkdirAll(filepath.Join(out, "captures"), 0o755)
	_ = os.MkdirAll(keyDir, 0o755)
	type keyRow struct {
		Case       string                   `json:"case"`
		CaptureID  string                   `json:"capture_id"`
		Day        string                   `json:"day"`
		Class      string                   `json:"class"`
		Oracle     string                   `json:"oracle"`
		OracleRank int                      `json:"oracle_rank"`
		PreCap     int                      `json:"pre_cap"`
		Stories    []offlineslice.StoryCand `json:"stories_top20"`
		Final      []offlineslice.FinalCand `json:"final_in_rank_order"`
	}
	var keys []keyRow
	for k, r := range chosen {
		id := fmt.Sprintf("%03d", k+1)
		type line struct{ sortKey, text string }
		var ls []line
		for _, c := range r.Final {
			if c.Kind == "story" {
				d := c.Desc
				if d == "" {
					d = "(no description)"
				}
				ls = append(ls, line{strings.ToLower(c.Label), fmt.Sprintf("STORY: %s: %s", c.Label, d)})
			} else {
				ls = append(ls, line{strings.ToLower(c.Label), c.Label})
			}
		}
		sort.Slice(ls, func(a, b int) bool { return ls[a].sortKey < ls[b].sortKey })
		text := texts[r.ID]
		if rs := []rune(text); len(rs) > 3000 {
			text = string(rs[:3000])
		}
		title := r.Title
		if strings.TrimSpace(title) == "" {
			title = "(none: a social post)"
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "# Capture %s\n\n**Title:** %s\n\n**Arrived:** %s\n\n## Text (first 3,000 characters)\n\n%s\n\n## Candidate subjects (alphabetical)\n\n", id, title, r.Fetched.UTC().Format("2006-01-02 15:04 UTC"), text)
		for _, l := range ls {
			fmt.Fprintf(&sb, "- %s\n", l.text)
		}
		if len(ls) == 0 {
			sb.WriteString("(none)\n")
		}
		if err := os.WriteFile(filepath.Join(out, "captures", id+".md"), []byte(sb.String()), 0o644); err != nil {
			panic(err)
		}
		keys = append(keys, keyRow{Case: id, CaptureID: r.ID, Day: r.Day, Class: r.Class, Oracle: r.Oracle, OracleRank: r.OracleRank, PreCap: r.PreCap, Stories: r.Stories, Final: r.Final})
	}
	kb, _ := json.MarshalIndent(keys, "", " ")
	_ = os.WriteFile(filepath.Join(keyDir, "key.json"), kb, 0o644)
	var tpl strings.Builder
	tpl.WriteString("# Answers\n\nFor each case fill the five lines. Do not change an answer after seeing the list.\n\n")
	for k := 0; k < n; k++ {
		fmt.Fprintf(&tpl, "## %03d\n\nACTORS: \nHAPPENING: \nACTOR_IN_LIST: yes (which) / no\nSAME_AFFAIR_STORY_LISTED: yes (which STORY) / no\nUNRELATED_ENTRIES: <count of list entries with no connection to this capture>\nNEW: yes / no (yes: the capture reports an affair none of the listed stories covers)\n\n", k+1)
	}
	_ = os.WriteFile(filepath.Join(out, "answers.md"), []byte(tpl.String()), 0o644)
	instr := fmt.Sprintf(`# Reading task: do the candidate lists contain the actors and the happening?

captures/001.md to captures/%03d.md are news items that arrived at a news site's feeds. Each has a title (a social post has none),
the first 3,000 characters of its text, and a list of candidate subjects that a program proposed. Most entries are names of
people, organisations and places. Entries that begin with STORY: are stories the site had already published before this item
arrived, each with its title and a one-line description. You did not write the program; do not guess how it works.

For each capture, in order:

1. Read the title and text first. Without looking at the list, write the ACTORS (who or what the item is about: people,
   organisations, places) and the HAPPENING (what happened or was reported, in one line of your own words).
2. Then read the list and answer:
   - ACTOR_IN_LIST: is at least one of the actors in the list (same thing, under any name)? Say which, or no.
   - SAME_AFFAIR_STORY_LISTED: is any STORY: entry about the same affair as this item (the same happening, or a direct
     development of it)? Say which, or no. A story only about the same actors or topic is no.
   - UNRELATED_ENTRIES: how many entries of the list have no connection to this item.
   - NEW: if no listed story is about the same affair, does this item report an affair that none of the listed stories
     covers? yes. (You cannot know whether the site has such a story that is not listed; answer only from the list.)
3. If an item has no readable news content (a menu, a bot check, a bare link), write NO CONTENT in HAPPENING and
   leave the other lines blank.

Judge only from what is shown. Do not look anything up. Work in numerical order. Fill in answers.md.
`, n)
	_ = os.WriteFile(filepath.Join(out, "INSTRUCTIONS.md"), []byte(instr), 0o644)
	fmt.Printf("pool %d; wrote %d to %s; key in %s\n", len(pool), n, out, keyDir)
}
