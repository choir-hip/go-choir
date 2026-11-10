package platform

// Model-judged importance on the Autopaper front list. A host timer
// (cmd/front-importance) asks Jev how important each front story is and
// writes the scores to a JSON file; corpusd only reads that file, so it
// gains no model access. Attaching is best effort: a missing, unreadable,
// corrupt or stale file attaches nothing and the front list is exactly
// what it was without scores.
//
// Sections are questions (docs/design/sections-as-questions-2026-11-10.md):
// the file carries the section list (ids, labels, thresholds, question
// hashes) and each row's P(yes) per question. corpusd applies the
// thresholds here, at load, so a threshold change re-sorts without Jev.

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sectionlist"
)

// frontImportanceMaxAge is how old the scores file may be before it is
// ignored: the timer rewrites it every ten minutes, so a day-old file means
// the scorer is not running.
const frontImportanceMaxAge = 24 * time.Hour

// frontImportanceFile is the scores file's schema (version 1).
type frontImportanceFile struct {
	Version int                           `json:"version"`
	Scores  map[string]frontImportanceRow `json:"scores"`
	// SectionList is the section list the rows' SectionP answer, written by
	// the host scorer (internal/frontimportance); absent in files written
	// before section lists.
	SectionList *sectionlist.Published `json:"section_list,omitempty"`
}

type frontImportanceRow struct {
	Revision   int  `json:"revision"`
	Importance int  `json:"importance"`
	Promo      bool `json:"promo"`
	// Section is the row's primary section as the scorer wrote it. With a
	// section list it is recomputed at load from SectionP and the list's
	// thresholds; without one it is a legacy section (frontSections).
	Section string `json:"section,omitempty"`
	// SectionP is P(yes) per section question, keyed by the SHA-256 of the
	// normalized question text.
	SectionP map[string]float64 `json:"section_p,omitempty"`

	// Sections and SectionFallback are resolved at load: every section the
	// story belongs to, and whether it was placed by its top section
	// because none reached its threshold.
	Sections        []string `json:"-"`
	SectionFallback bool     `json:"-"`
}

// frontSections are the legacy sections a scores file written before section
// lists may name. They are also still accepted as a section filter (an empty
// page when the list does not have them), so an older client gets no error.
var frontSections = map[string]bool{"world": true, "us_politics": true, "business": true, "tech_ai": true}

// frontImportanceCache holds the parsed scores file, reparsed only when its
// modification time or size changes.
type frontImportanceCache struct {
	mu      sync.Mutex
	path    string
	modTime time.Time
	size    int64
	scores  map[string]frontImportanceRow
	list    *sectionlist.Published
}

// SetFrontImportancePath names the scores file the front list reads; empty
// turns scores off.
func (s *Service) SetFrontImportancePath(path string) {
	if s == nil {
		return
	}
	c := &s.frontImportance
	c.mu.Lock()
	defer c.mu.Unlock()
	c.path = strings.TrimSpace(path)
	c.modTime, c.size, c.scores, c.list = time.Time{}, 0, nil, nil
}

// load returns the current scores, or nil when there are none to use.
func (c *frontImportanceCache) load(now time.Time) map[string]frontImportanceRow {
	scores, _ := c.loadWithList(now)
	return scores
}

// sectionList returns the current section list, or nil when the file has
// none (or there is no usable file).
func (c *frontImportanceCache) sectionList(now time.Time) *sectionlist.Published {
	_, list := c.loadWithList(now)
	return list
}

// loadWithList returns the current scores and the section list they answer.
func (c *frontImportanceCache) loadWithList(now time.Time) (map[string]frontImportanceRow, *sectionlist.Published) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.path == "" {
		return nil, nil
	}
	info, err := os.Stat(c.path)
	if err != nil || info.IsDir() || now.Sub(info.ModTime()) > frontImportanceMaxAge {
		c.modTime, c.size, c.scores, c.list = time.Time{}, 0, nil, nil
		return nil, nil
	}
	if c.scores != nil && info.ModTime().Equal(c.modTime) && info.Size() == c.size {
		return c.scores, c.list
	}
	scores := map[string]frontImportanceRow{}
	var list *sectionlist.Published
	raw, err := os.ReadFile(c.path)
	if err == nil {
		var file frontImportanceFile
		if err = json.Unmarshal(raw, &file); err == nil && file.Version == 1 {
			if file.SectionList != nil && file.SectionList.Valid() {
				list = file.SectionList
			}
			for id, row := range file.Scores {
				if strings.TrimSpace(id) != "" && row.Importance >= 1 && row.Importance <= 10 {
					scores[id] = resolveSections(row, list)
				}
			}
		} else if err == nil {
			err = errUnsupportedFrontImportanceVersion
		}
	}
	if err != nil {
		log.Printf("corpusd: front importance scores %s ignored: %v", c.path, err)
	}
	// A bad file is cached as empty so it is not reparsed on every request.
	c.modTime, c.size, c.scores, c.list = info.ModTime(), info.Size(), scores, list
	return scores, list
}

// resolveSections applies the list's thresholds to a row's stored
// probabilities. Probabilities that are not in [0,1] are dropped (the file
// is host-written, but a corrupt one must not place a story). Without a
// list, a row keeps only a legacy section.
func resolveSections(row frontImportanceRow, list *sectionlist.Published) frontImportanceRow {
	row.Sections, row.SectionFallback = nil, false
	if list == nil {
		if !frontSections[row.Section] {
			row.Section = ""
		} else {
			row.Sections = []string{row.Section}
		}
		return row
	}
	if len(row.SectionP) > 0 {
		probs := make(map[string]float64, len(row.SectionP))
		for h, v := range row.SectionP {
			if v >= 0 && v <= 1 {
				probs[h] = v
			}
		}
		pl := list.Place(probs)
		row.Section, row.Sections, row.SectionFallback = pl.Primary, pl.Sections, pl.Fallback
		return row
	}
	// A legacy row under a list: it keeps its section only when the list
	// has it; the scorer's next run asks it the list's questions.
	if list.Has(row.Section) {
		row.Sections = []string{row.Section}
	} else {
		row.Section = ""
	}
	return row
}

type frontImportanceVersionError struct{}

func (frontImportanceVersionError) Error() string { return "unsupported scores file version" }

var errUnsupportedFrontImportanceVersion error = frontImportanceVersionError{}

// FrontSectionInfo is a section of the paper as the front page lists it.
type FrontSectionInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// frontSectionInfos is the section list for the front page's switcher, in
// the list's order; nil when the scorer has published none (the client then
// uses its default).
func (s *Service) frontSectionInfos() []FrontSectionInfo {
	if s == nil {
		return nil
	}
	list := s.frontImportance.sectionList(time.Now())
	if list == nil {
		return nil
	}
	out := make([]FrontSectionInfo, 0, len(list.Entries))
	for _, e := range list.Entries {
		out = append(out, FrontSectionInfo{ID: e.ID, Label: e.Label})
	}
	return out
}

// frontSectionValid reports whether id may filter the front list: a section
// of the published list, or a legacy section.
func (s *Service) frontSectionValid(id string) bool {
	if frontSections[id] {
		return true
	}
	list := s.frontImportance.sectionList(time.Now())
	return list != nil && list.Has(id)
}

// attachFrontImportance sets Importance, Promo and the sections on rows the
// scores file scores. A row scored at an older revision keeps that score until the
// scorer rescores it.
func (s *Service) attachFrontImportance(stories []FrontStory) {
	if s == nil || len(stories) == 0 {
		return
	}
	scores := s.frontImportance.load(time.Now())
	if len(scores) == 0 {
		return
	}
	for i := range stories {
		if row, ok := scores[stories[i].PublicationID]; ok {
			stories[i].Importance = row.Importance
			stories[i].Promo = row.Promo
			stories[i].Section = row.Section
			stories[i].Sections = row.Sections
			stories[i].SectionFallback = row.SectionFallback
		}
	}
}
