package offlineslice

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Story is a published story as the paper had it over time: its versions, each
// with the passage vectors of vec_story_passages.
type Story struct {
	ID       string
	First    time.Time      // creation of its first version, whether or not that version is in Versions
	Versions []StoryVersion // ascending by Created
}

// StoryVersion is one published version of a story.
type StoryVersion struct {
	ID      string
	Created time.Time
	Title   string
	Dek     string
	Vecs    [][]float32
}

// StoryEntity is an entity a story is linked to (mentions_entity), with its role.
type StoryEntity struct {
	EntityCanonical string
	Role            string
	At              time.Time
}

// DekRunes bounds a story's one-line description.
const DekRunes = 200

var sourceMarker = regexp.MustCompile(`\[source[^\]]*\]`)

// Dek is a story's first paragraph after its heading, citation markers
// removed, clipped: the "main point" is not stored where the host can read it,
// so the dek stands for it.
func Dek(text string) string {
	for _, p := range strings.Split(text, "\n\n") {
		p = strings.TrimSpace(p)
		if p == "" || strings.HasPrefix(p, "#") {
			continue
		}
		p = sourceMarker.ReplaceAllString(p, "")
		p = strings.Join(strings.Fields(p), " ")
		if p == "" {
			continue
		}
		if r := []rune(p); len(r) > DekRunes {
			p = string(r[:DekRunes]) + "…"
		}
		return p
	}
	return ""
}

// StoryHit is a story near a capture, as of the capture's arrival.
type StoryHit struct {
	Story   int // index into the stories slice
	Version int // index of the version that existed at arrival
	Cos     float64
}

// NearestStories returns the k stories nearest a capture by cosine, as the
// stories stood at its arrival: each story is read at its latest version
// created strictly before the capture's fetch time, scored by the best of that
// version's passages. A story with no such version, or no vector, is absent.
func NearestStories(c Capture, stories []Story, k int) []StoryHit {
	if c.Vec == nil {
		return nil
	}
	var top []StoryHit
	for si := range stories {
		vs := stories[si].Versions
		// first version with Created >= fetched; the one before it is current
		n := sort.Search(len(vs), func(i int) bool { return !vs[i].Created.Before(c.Fetched) })
		if n == 0 {
			continue
		}
		best := -2.0
		for _, v := range vs[n-1].Vecs {
			if len(v) != len(c.Vec) {
				continue
			}
			var s float32
			for d := range v {
				s += v[d] * c.Vec[d]
			}
			if float64(s) > best {
				best = float64(s)
			}
		}
		if best < -1 {
			continue
		}
		if len(top) == k && best <= top[k-1].Cos {
			continue
		}
		h := StoryHit{Story: si, Version: n - 1, Cos: best}
		pos := sort.Search(len(top), func(x int) bool { return top[x].Cos < h.Cos })
		top = append(top, StoryHit{})
		copy(top[pos+1:], top[pos:])
		top[pos] = h
		if len(top) > k {
			top = top[:k]
		}
	}
	return top
}

// Oracle is what the records say became of a capture.
type Oracle struct {
	// Status: "fed" (a story that existed before the capture arrived cites it),
	// "new" (only stories first published after it cite it) or "none" (no story cites it).
	Status   string       `json:"status"`
	Existing []string     `json:"existing,omitempty"` // story ids that existed at arrival and cite it
	HitRank  map[int]bool `json:"-"`
}

// OracleStatus classifies a capture by the stories that cite it.
func OracleStatus(citing []string, first map[string]time.Time, at time.Time) Oracle {
	var o Oracle
	seen := map[string]bool{}
	for _, s := range citing {
		if seen[s] {
			continue
		}
		seen[s] = true
		if f, ok := first[s]; ok && f.Before(at) {
			o.Existing = append(o.Existing, s)
		}
	}
	switch {
	case len(o.Existing) > 0:
		o.Status = "fed"
	case len(seen) > 0:
		o.Status = "new"
	default:
		o.Status = "none"
	}
	return o
}

// StoryData is everything the story export holds.
type StoryData struct {
	Stories []Story
	Index   map[string]int           // story id -> index
	Links   map[string][]StoryEntity // story id -> its entity links
	First   map[string]time.Time     // story id -> first version time, all stories
	Cites   map[string][]string      // capture id -> story ids citing it
}

func readCSVFile(path string, fn func(rec []string) error) error {
	r, f, err := openCSV(path)
	if err != nil {
		return err
	}
	defer f.Close()
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := fn(rec); err != nil {
			return err
		}
	}
}

// ReadStories reads the story-*.csv and story-text.jsonl of the export.
func ReadStories(dir string) (*StoryData, error) {
	d := &StoryData{Index: map[string]int{}, Links: map[string][]StoryEntity{}, First: map[string]time.Time{}, Cites: map[string][]string{}}
	type verRef struct {
		story string
		v     StoryVersion
	}
	vers := map[string]*verRef{}
	var order []string
	if err := readCSVFile(filepath.Join(dir, "story-versions.csv"), func(rec []string) error {
		if len(rec) < 5 {
			return nil
		}
		vers[rec[1]] = &verRef{story: rec[0], v: StoryVersion{ID: rec[1], Created: parseTime(rec[2]), Title: rec[4]}}
		order = append(order, rec[1])
		return nil
	}); err != nil {
		return nil, err
	}
	if err := readCSVFile(filepath.Join(dir, "story-vectors.csv"), func(rec []string) error {
		vr := vers[rec[0]]
		if vr == nil || len(rec) < 3 {
			return nil
		}
		parts := strings.Split(strings.Trim(rec[2], "[]"), ",")
		v := make([]float32, len(parts))
		for i, p := range parts {
			x, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
			if err != nil {
				return err
			}
			v[i] = float32(x)
		}
		vr.v.Vecs = append(vr.v.Vecs, normalize(v))
		return nil
	}); err != nil {
		return nil, err
	}
	tf, err := os.Open(filepath.Join(dir, "story-text.jsonl"))
	if err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(tf)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var t struct{ VersionID, Title, Text string }
		var raw struct {
			VersionID string `json:"version_id"`
			Title     string `json:"title"`
			Text      string `json:"text"`
		}
		if json.Unmarshal(sc.Bytes(), &raw) != nil {
			continue
		}
		t.VersionID, t.Title, t.Text = raw.VersionID, raw.Title, raw.Text
		if vr := vers[t.VersionID]; vr != nil {
			if strings.TrimSpace(t.Title) != "" {
				vr.v.Title = strings.TrimSpace(t.Title)
			}
			vr.v.Dek = Dek(t.Text)
		}
	}
	tf.Close()
	for _, id := range order {
		vr := vers[id]
		if len(vr.v.Vecs) == 0 {
			continue
		}
		si, ok := d.Index[vr.story]
		if !ok {
			si = len(d.Stories)
			d.Index[vr.story] = si
			d.Stories = append(d.Stories, Story{ID: vr.story})
		}
		d.Stories[si].Versions = append(d.Stories[si].Versions, vr.v)
	}
	for i := range d.Stories {
		vs := d.Stories[i].Versions
		sort.SliceStable(vs, func(a, b int) bool { return vs[a].Created.Before(vs[b].Created) })
	}
	if err := readCSVFile(filepath.Join(dir, "story-first.csv"), func(rec []string) error {
		if len(rec) >= 2 {
			d.First[rec[0]] = parseTime(rec[1])
		}
		return nil
	}); err != nil {
		return nil, err
	}
	for i := range d.Stories {
		d.Stories[i].First = d.First[d.Stories[i].ID]
	}
	if err := readCSVFile(filepath.Join(dir, "story-entities.csv"), func(rec []string) error {
		if len(rec) >= 4 {
			d.Links[rec[0]] = append(d.Links[rec[0]], StoryEntity{EntityCanonical: rec[1], Role: rec[2], At: parseTime(rec[3])})
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if err := readCSVFile(filepath.Join(dir, "story-cites.csv"), func(rec []string) error {
		if len(rec) >= 3 {
			d.Cites[rec[2]] = append(d.Cites[rec[2]], rec[1])
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return d, nil
}

