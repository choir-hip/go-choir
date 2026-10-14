package platform

// Model-judged importance on the Autopaper front list. A host timer
// (cmd/front-importance) asks Jev how important each front story is and
// writes the scores to a JSON file; corpusd only reads that file, so it
// gains no model access. Attaching is best effort: a missing, unreadable,
// corrupt or stale file attaches nothing and the front list is exactly
// what it was without scores.

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// frontImportanceMaxAge is how old the scores file may be before it is
// ignored: the timer rewrites it every ten minutes, so a day-old file means
// the scorer is not running.
const frontImportanceMaxAge = 24 * time.Hour

// frontImportanceFile is the scores file's schema (version 1).
type frontImportanceFile struct {
	Version int                           `json:"version"`
	Scores  map[string]frontImportanceRow `json:"scores"`
}

type frontImportanceRow struct {
	Revision   int  `json:"revision"`
	Importance int  `json:"importance"`
	Promo      bool `json:"promo"`
	// Section is one of frontSections; absent in rows scored before
	// sections existed (LF1).
	Section string `json:"section,omitempty"`
}

// frontSections are the Autopaper front-page sections a scores row may
// name; any other value is ignored.
var frontSections = map[string]bool{"world": true, "us_politics": true, "business": true, "tech_ai": true}

// frontImportanceCache holds the parsed scores file, reparsed only when its
// modification time or size changes.
type frontImportanceCache struct {
	mu      sync.Mutex
	path    string
	modTime time.Time
	size    int64
	scores  map[string]frontImportanceRow
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
	c.modTime, c.size, c.scores = time.Time{}, 0, nil
}

// load returns the current scores, or nil when there are none to use.
func (c *frontImportanceCache) load(now time.Time) map[string]frontImportanceRow {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.path == "" {
		return nil
	}
	info, err := os.Stat(c.path)
	if err != nil || info.IsDir() || now.Sub(info.ModTime()) > frontImportanceMaxAge {
		c.modTime, c.size, c.scores = time.Time{}, 0, nil
		return nil
	}
	if c.scores != nil && info.ModTime().Equal(c.modTime) && info.Size() == c.size {
		return c.scores
	}
	scores := map[string]frontImportanceRow{}
	raw, err := os.ReadFile(c.path)
	if err == nil {
		var file frontImportanceFile
		if err = json.Unmarshal(raw, &file); err == nil && file.Version == 1 {
			for id, row := range file.Scores {
				if strings.TrimSpace(id) != "" && row.Importance >= 1 && row.Importance <= 10 {
					scores[id] = row
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
	c.modTime, c.size, c.scores = info.ModTime(), info.Size(), scores
	return scores
}

type frontImportanceVersionError struct{}

func (frontImportanceVersionError) Error() string { return "unsupported scores file version" }

var errUnsupportedFrontImportanceVersion error = frontImportanceVersionError{}

// attachFrontImportance sets Importance, Promo and Section on rows the
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
			if frontSections[row.Section] {
				stories[i].Section = row.Section
			}
		}
	}
}
