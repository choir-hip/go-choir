package platform

// Ways the section list on the front read path can fail
// (docs/design/sections-as-questions-2026-11-10.md):
//
//  1. A story in two sections shows in only one, or a section page lists a
//     story that is not a member.
//  2. A story no threshold admits is in no section (unreachable from a
//     section page) or is not marked as placed by fallback.
//  3. A threshold edit in the file does not re-sort at the next read.
//  4. A probability outside [0,1] in the file places a story.
//  5. A section of the list with no stories errors or lists null.
//  6. The front page does not carry the list, so the client cannot render
//     sections the house list added.

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/sectionlist"
)

func writeListScores(t *testing.T, path string, page *FrontPage, list sectionlist.List, probs map[string]map[string]float64) {
	t.Helper()
	pub := list.Published()
	hash := map[string]string{}
	for i, e := range pub.Entries {
		hash[list.Sections[i].ID] = e.Hash
	}
	scores := map[string]any{}
	for _, s := range page.Stories {
		bySection, ok := probs[s.Title]
		if !ok {
			continue
		}
		p := map[string]float64{}
		for id, v := range bySection {
			p[hash[id]] = v
		}
		scores[s.PublicationID] = map[string]any{"revision": s.LatestRevision, "importance": 5, "section_p": p}
	}
	raw, err := json.Marshal(map[string]any{"version": 1, "scores": scores, "section_list": pub})
	if err != nil {
		t.Fatal(err)
	}
	writeScores(t, path, string(raw))
}

func TestFrontSectionListMultiMembershipFallbackAndThresholds(t *testing.T) {
	f := newSectionFixture(t)
	page, err := f.svc.ListFrontStories(t.Context(), 100, "")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "scores.json")
	list := sectionlist.House()
	probs := map[string]map[string]float64{
		"Hotel":         {"world": 0.9, "politics": 0.8, "sports": 0.1},
		"Golf":          {"tech_ai": 0.9, "business": 0.6},
		"Foxtrot":       {"world": 0.2, "science": 0.4}, // below every threshold: placed by science
		"Delta":         {"world": 7.0, "sports": -1},   // not probabilities: ignored
		"Canal dredged": {"business": 0.95},
	}
	writeListScores(t, path, page, list, probs)
	f.svc.SetFrontImportancePath(path)

	titles := func(section string) string {
		return fmt.Sprint(storyIDs(sectionPage(t, f.svc, 50, "", section)))
	}
	if got := titles("world"); got != "[Hotel]" {
		t.Fatalf("world = %s", got)
	}
	if got := titles("politics"); got != "[Hotel]" {
		t.Fatalf("politics = %s (Hotel is in two sections)", got)
	}
	if got := titles("business"); got != "[Golf Canal dredged]" {
		t.Fatalf("business = %s", got)
	}
	science := sectionPage(t, f.svc, 50, "", "science")
	if fmt.Sprint(storyIDs(science)) != "[Foxtrot]" || !science.Stories[0].SectionFallback || science.Stories[0].Section != "science" {
		t.Fatalf("science = %+v, want Foxtrot placed by fallback", science.Stories)
	}
	if got := titles("sports"); got != "[]" {
		t.Fatalf("sports = %s, want an empty list", got)
	}
	if sectionPage(t, f.svc, 50, "", "sports").Stories == nil {
		t.Fatalf("empty section lists null")
	}
	front, err := f.svc.ListFrontStories(t.Context(), 100, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(front.Sections) != 10 || front.Sections[0].ID != "world" || front.Sections[4].Label != "Technology" {
		t.Fatalf("front page sections = %+v", front.Sections)
	}
	for _, s := range front.Stories {
		switch s.Title {
		case "Hotel":
			if len(s.Sections) != 2 || s.Section != "world" || s.SectionFallback {
				t.Fatalf("Hotel = %+v", s)
			}
		case "Delta":
			if s.Section != "" || len(s.Sections) != 0 {
				t.Fatalf("Delta placed by an invalid probability: %+v", s)
			}
		}
	}

	// A threshold edit in the file re-sorts at the next read, with no Jev.
	list.Sections[0].Threshold = 0.95
	writeListScores(t, path, page, list, probs)
	if got := titles("world"); got != "[]" {
		// Hotel (0.9) is under 0.95 now; with politics 0.8 it stays in politics.
		t.Fatalf("world after threshold edit = %s", got)
	}
	if got := titles("politics"); got != "[Hotel]" {
		t.Fatalf("politics after threshold edit = %s", got)
	}
}
