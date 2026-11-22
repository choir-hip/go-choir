package offlineslice

import (
	"strings"
	"sync"
	"time"
)

// ConfigB sets rung 0b.
type ConfigB struct {
	ListStories   int     // stories offered in the list (5)
	OracleDepth   int     // stories kept for the oracle (20)
	Cap           int     // list cap (12)
	DFMax         float64 // see Cleaner
	CommonRatio   float64
	CommonMin     int
	EntityStories int // top stories whose central entities are offered (3)
}

// DefaultConfigB is the stated configuration.
func DefaultConfigB() ConfigB {
	return ConfigB{ListStories: 5, OracleDepth: 20, Cap: 12, DFMax: 0.01, CommonRatio: 0.5, CommonMin: 20, EntityStories: 3}
}

// IsArrival: a capture a source brought, not a page a desk read for itself.
func IsArrival(c Capture) bool { return c.SourceID != "web:read" }

// Class is a capture's source class.
func Class(c Capture) string {
	switch {
	case c.SourceID == "web:read":
		return "web:read"
	case strings.HasPrefix(c.SourceID, "social:"):
		return "social"
	case strings.HasPrefix(c.SourceID, "rss:"):
		return "rss"
	case strings.HasPrefix(c.SourceID, "arxiv"):
		return "arxiv"
	case strings.HasPrefix(c.SourceID, "official"):
		return "official"
	}
	return "other:" + c.SourceID
}

// ResultB is what rung 0b proposes for one source arrival.
type ResultB struct {
	ID       string    `json:"id"`
	Day      string    `json:"day"`
	Fetched  time.Time `json:"fetched"`
	Class    string    `json:"class"`
	Source   string    `json:"source"`
	Host     string    `json:"host"`
	Title    string    `json:"title"`
	Lead     string    `json:"lead,omitempty"`
	Mentions int       `json:"mentions"`
	// RawCount: distinct entities and new names the deployed rules give before any cleaning.
	RawCount int `json:"raw_count"`
	// CleanCount: the same after cleaning (no stories).
	CleanCount  int            `json:"clean_count"`
	Dropped     map[string]int `json:"dropped,omitempty"`
	DroppedKeys []string       `json:"dropped_keys,omitempty"`
	KeptKeys    []string       `json:"kept_keys,omitempty"`
	// Stories: the nearest earlier stories, best first (OracleDepth of them).
	Stories []StoryCand `json:"stories"`
	// PreCap: distinct entries before the cap; Final: the list.
	PreCap int         `json:"pre_cap"`
	Final  []FinalCand `json:"final"`
	// StoryEntities offered (names).
	Oracle     string   `json:"oracle"`
	CitedStory []string `json:"cited_story,omitempty"`
	// OracleRank: the best rank (1-based) among the stories at arrival that cite the capture; 0 when none is among the OracleDepth.
	OracleRank int `json:"oracle_rank"`
}

// ReplayB runs the cleaned rules and the story source over the source
// arrivals. Every capture, arrivals or not, goes through the rules, so the
// cleaner's counts are of all captures of the earlier days.
func ReplayB(caps []Capture, ents []Entity, stops []Stop, rules Rules, policy AliasPolicy, sd *StoryData, entByCanon map[string]Entity, cfg ConfigB) []ResultB {
	// the stories nearest each arrival, in parallel (they depend only on the arrival and the stories)
	hits := make([][]StoryHit, len(caps))
	var wg sync.WaitGroup
	work := make(chan int, 256)
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				hits[i] = NearestStories(caps[i], sd.Stories, cfg.OracleDepth)
			}
		}()
	}
	for i, c := range caps {
		if IsArrival(c) {
			work <- i
		}
	}
	close(work)
	wg.Wait()

	cl := NewCleaner(cfg.DFMax, cfg.CommonRatio, cfg.CommonMin)
	var out []ResultB
	var day time.Time
	for i, c := range caps {
		d := c.Fetched.UTC().Truncate(24 * time.Hour)
		if !d.Equal(day) {
			day = d
			rules.Prepare(day, EntitiesAsOf(ents, day, policy), StopsAsOf(stops, day))
			cl.StartDay()
		}
		ms := rules.Mentions(c.Title, c.Lead, nil)
		if !IsArrival(c) {
			cl.Observe(ms, c.Title, c.Lead)
			continue
		}
		r := ResultB{ID: c.ID, Day: day.Format("2006-01-02"), Fetched: c.Fetched, Class: Class(c), Source: c.SourceID, Host: c.Host,
			Title: c.Title, Lead: c.Lead, Mentions: len(ms), Dropped: map[string]int{}}
		r.RawCount = distinctCount(ms)
		var kept []Mention
		for _, m := range ms {
			if why := cl.Drop(m, c.Host); why != "" {
				r.Dropped[why]++
				r.DroppedKeys = append(r.DroppedKeys, why+":"+m.Key)
				continue
			}
			kept = append(kept, m)
			r.KeptKeys = append(r.KeptKeys, m.Key)
		}
		cl.Observe(ms, c.Title, c.Lead)
		r.CleanCount = distinctCount(kept)
		in := ListInput{Mentions: kept}
		r.Stories = []StoryCand{}
		for rank, h := range hits[i] {
			st := sd.Stories[h.Story]
			v := st.Versions[h.Version]
			sc := StoryCand{StoryID: st.ID, Title: v.Title, Dek: v.Dek, Cos: h.Cos}
			r.Stories = append(r.Stories, sc)
			if rank < cfg.ListStories {
				in.Stories = append(in.Stories, sc)
			}
			if rank < cfg.EntityStories {
				var es []Cand
				for _, l := range sd.Links[st.ID] {
					if l.Role != "central" || !l.At.Before(c.Fetched) {
						continue
					}
					e, ok := entByCanon[l.EntityCanonical]
					if !ok || e.Tombstone || !e.Created.Before(c.Fetched) {
						continue
					}
					es = append(es, Cand{EntityID: e.EntityID, Name: e.Name})
				}
				in.StoryEntities = append(in.StoryEntities, es)
			}
		}
		r.Final, r.PreCap = RankAndCap(in, cfg.Cap)
		o := OracleStatus(sd.Cites[c.ID], sd.First, c.Fetched)
		r.Oracle, r.CitedStory = o.Status, o.Existing
		for rank, s := range r.Stories {
			for _, e := range o.Existing {
				if s.StoryID == e {
					r.OracleRank = rank + 1
					break
				}
			}
			if r.OracleRank != 0 {
				break
			}
		}
		out = append(out, r)
	}
	return out
}

func distinctCount(ms []Mention) int {
	seen := map[string]bool{}
	for _, m := range ms {
		if len(m.Cands) == 0 {
			seen["n:"+m.Key] = true
			continue
		}
		seen["e:"+m.Cands[0].EntityID] = true
		for _, c := range m.Cands[1:] {
			seen["e:"+c.EntityID] = true
		}
	}
	return len(seen)
}
