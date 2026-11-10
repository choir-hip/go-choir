package frontimportance

// Ways the section questions can fail (docs/design/sections-as-questions-2026-11-10.md):
//
//  1. A story's questions do not fit one call (more than the host cap), or a
//     new story costs more than one call.
//  2. A question that matches everything goes unnoticed.
//  3. A story is in no section, or a threshold change needs Jev again.
//  4. A bad probability (out of range) is stored and poisons placement.
//  5. Renaming a section loses its cache; editing its question keeps a
//     stale answer; adding a section asks more than that one question.
//  6. Jev down: the previous scores and list are lost.
//  7. Two runs over the same cache disagree (reorder flapping).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sectionlist"
)

func oneStory() []frontStory {
	return []frontStory{{PublicationID: "pub-a", Title: "A", LatestRevision: 1}}
}

func runOnce(t *testing.T, cfg config, stories []frontStory, jev *fakeJev) runStats {
	t.Helper()
	srv := frontServer(t, http.StatusOK, stories)
	cfg.FrontURL = srv.URL
	stats, err := run(context.Background(), cfg, srv.Client(), jev, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return stats
}

func TestNewStoryIsOneCallOfTwelveQuestions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	jev := &fakeJev{answers: map[string]string{"A": "6"}}
	runOnce(t, testConfig("", path), oneStory(), jev)
	if len(jev.calls) != 1 || len(jev.asked[0]) != 10 {
		t.Fatalf("calls = %d asked = %v, want one call with ten section questions (and importance, promo)", len(jev.calls), jev.asked)
	}
	file := readScores(path)
	row := file.Scores["pub-a"]
	if row.Importance != 6 || row.Judge != sectionJudgeVersion || len(row.SectionP) != 10 || row.Section != "world" {
		t.Fatalf("row = %+v", row)
	}
	if file.SectionList == nil || len(file.SectionList.Entries) != 10 {
		t.Fatalf("scores file carries no section list: %+v", file.SectionList)
	}
}

func TestScorerRefusesTooLongList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	cfg := testConfig("", path)
	cfg.Sections = sectionlist.List{Version: 1, ID: "x"}
	for i := 0; i < 11; i++ {
		cfg.Sections.Sections = append(cfg.Sections.Sections, sectionlist.Section{ID: fmt.Sprintf("s%d", i), Label: "S", Question: fmt.Sprintf("Is it %d?", i), Threshold: 0.5})
	}
	srv := frontServer(t, http.StatusOK, oneStory())
	cfg.FrontURL = srv.URL
	if _, err := run(context.Background(), cfg, srv.Client(), &fakeJev{}, time.Now); err == nil {
		t.Fatalf("an 11-section list was accepted")
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("scores file written for a refused list")
	}
}

// A legacy row (importance current, no section answers) is asked only the
// section questions: importance is not rescored.
func TestLegacyRowAsksOnlySections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	old := scoreRow{Revision: 1, Importance: 8, Criteria: scoringCriteria, Section: "world"}
	if err := writeScores(path, scoresFile{Version: 1, Scores: map[string]scoreRow{"pub-a": old}}); err != nil {
		t.Fatal(err)
	}
	jev := &fakeJev{answers: map[string]string{"A": "2"}}
	runOnce(t, testConfig("", path), oneStory(), jev)
	row := readScores(path).Scores["pub-a"]
	if row.Importance != 8 || len(row.SectionP) != 10 {
		t.Fatalf("row = %+v, want importance 8 kept and ten answers", row)
	}
}

func TestNewQuestionAsksOnlyThatQuestion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	jev := &fakeJev{answers: map[string]string{"A": "6"}}
	cfg := testConfig("", path)
	runOnce(t, cfg, oneStory(), jev)
	// Add a section and edit another's question.
	list := sectionlist.House()
	list.Sections[9] = sectionlist.Section{ID: "sports", Label: "Sports", Question: "Is this story about a team sport played professionally?", Threshold: 0.5}
	list.Sections = list.Sections[:10]
	list.Version = 2
	cfg.Sections = list
	jev.calls, jev.asked = nil, nil
	runOnce(t, cfg, oneStory(), jev)
	if len(jev.calls) != 1 || len(jev.asked[0]) != 1 {
		t.Fatalf("calls = %d asked = %v, want one call with only the edited question", len(jev.calls), jev.asked)
	}
	row := readScores(path).Scores["pub-a"]
	if len(row.SectionP) != 10 {
		t.Fatalf("answers = %d, want the nine cached and the edited one; the old sports answer pruned", len(row.SectionP))
	}
	if _, stale := row.SectionP[sectionlist.Hash(sectionlist.House().Sections[9].Question)]; stale {
		t.Fatalf("the edited question's old answer is still stored")
	}
}

func TestRenamedSectionKeepsCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	jev := &fakeJev{answers: map[string]string{"A": "6"}}
	cfg := testConfig("", path)
	runOnce(t, cfg, oneStory(), jev)
	list := sectionlist.House()
	list.Version = 2
	for i := range list.Sections {
		list.Sections[i].ID = "renamed_" + list.Sections[i].ID
		list.Sections[i].Label = "New " + list.Sections[i].Label
		list.Sections[i].Question = "  " + strings.ReplaceAll(list.Sections[i].Question, " ", "  ") + " "
	}
	cfg.Sections = list
	jev.calls = nil
	stats := runOnce(t, cfg, oneStory(), jev)
	if len(jev.calls) != 0 || stats.Kept != 1 {
		t.Fatalf("calls = %v stats = %+v, want the cache to hold across a rename and whitespace", jev.calls, stats)
	}
	if got := readScores(path).Scores["pub-a"].Section; got != "renamed_world" {
		t.Fatalf("primary = %q, want the renamed id", got)
	}
}

func TestThresholdChangeAsksNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	jev := &fakeJev{answers: map[string]string{"A": "6"}, prob: map[string]map[string]float64{"A": {"world": 0.6}}}
	cfg := testConfig("", path)
	runOnce(t, cfg, oneStory(), jev)
	list := sectionlist.House()
	list.Version = 2
	list.Sections[0].Threshold = 0.9
	cfg.Sections = list
	jev.calls = nil
	runOnce(t, cfg, oneStory(), jev)
	if len(jev.calls) != 0 {
		t.Fatalf("a threshold change asked Jev %d times", len(jev.calls))
	}
	file := readScores(path)
	if pl := file.SectionList.Place(file.Scores["pub-a"].SectionP); !pl.Fallback || pl.Primary != "world" {
		t.Fatalf("placement = %+v, want world by fallback (0.6 < 0.9)", pl)
	}
}

func TestScorerRefusesBadProbability(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	jev := &fakeJev{answers: map[string]string{"A": "6"}, prob: map[string]map[string]float64{"A": {"sports": 1.5, "health": -0.2}}}
	runOnce(t, testConfig("", path), oneStory(), jev)
	row := readScores(path).Scores["pub-a"]
	house := sectionlist.House()
	for _, s := range house.Sections {
		_, has := row.SectionP[sectionlist.Hash(s.Question)]
		if bad := s.ID == "sports" || s.ID == "health"; bad == has {
			t.Fatalf("section %s stored = %v", s.ID, has)
		}
	}
	// The refused two are asked again next run, and only those.
	jev.calls, jev.asked, jev.prob = nil, nil, nil
	runOnce(t, testConfig("", path), oneStory(), jev)
	if len(jev.calls) != 1 || len(jev.asked[0]) != 2 {
		t.Fatalf("second run asked %v, want the two refused questions", jev.asked)
	}
}

func TestScorerFlagsBroadSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	var stories []frontStory
	jev := &fakeJev{answers: map[string]string{}, yes: map[string][]string{}}
	for i := 0; i < 25; i++ {
		title := fmt.Sprintf("T%d", i)
		stories = append(stories, frontStory{PublicationID: "pub-" + title, Title: title, LatestRevision: 1})
		jev.answers[title] = "5"
		jev.yes[title] = []string{"us", "culture"}
	}
	stats := runOnce(t, testConfig("", path), stories, jev)
	if strings.Join(stats.Broad, ",") != "us,culture" {
		t.Fatalf("broad = %v, want us and culture", stats.Broad)
	}
}

func TestStoryBelowEveryThresholdIsStillPlaced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	jev := &fakeJev{answers: map[string]string{"A": "5"}, yes: map[string][]string{"A": {}}, prob: map[string]map[string]float64{"A": {"science": 0.3}}}
	stats := runOnce(t, testConfig("", path), oneStory(), jev)
	row := readScores(path).Scores["pub-a"]
	if row.Section != "science" || stats.Fallback != 1 {
		t.Fatalf("row section = %q, fallback = %d, want science placed by fallback", row.Section, stats.Fallback)
	}
}

func TestScorerJevDownKeepsLastScores(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	jev := &fakeJev{answers: map[string]string{"A": "6"}}
	cfg := testConfig("", path)
	runOnce(t, cfg, oneStory(), jev)
	before := readScores(path)
	// A new section arrives while Jev is down for section-only calls.
	list := sectionlist.House()
	list.Version = 2
	list.Sections[0].Question = "Is this story about a country other than the United States?"
	cfg.Sections = list
	jev.sectionsDown = true
	stats := runOnce(t, cfg, oneStory(), jev)
	after := readScores(path)
	if stats.Failed != 1 {
		t.Fatalf("stats = %+v", stats)
	}
	if after.SectionList == nil {
		t.Fatalf("list lost")
	}
	row := after.Scores["pub-a"]
	if row.Importance != 6 || row.Revision != 1 {
		t.Fatalf("row = %+v, want the last scores", row)
	}
	// The still-valid answers of the other nine are kept for the front page.
	if len(row.SectionP) != 9 || len(before.Scores["pub-a"].SectionP) != 10 {
		t.Fatalf("answers before %d after %d, want 10 then the nine still valid", len(before.Scores["pub-a"].SectionP), len(row.SectionP))
	}
}

func TestScorerSecondRunIsStable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	jev := &fakeJev{answers: map[string]string{"A": "6", "B": "3"}, yes: map[string][]string{"A": {"world", "politics"}, "B": {"sports"}}}
	stories := []frontStory{{PublicationID: "pub-a", Title: "A", LatestRevision: 1}, {PublicationID: "pub-b", Title: "B", LatestRevision: 2}}
	cfg := testConfig("", path)
	runOnce(t, cfg, stories, jev)
	first := stableJSON(t, readScores(path))
	jev.calls = nil
	runOnce(t, cfg, stories, jev)
	if len(jev.calls) != 0 {
		t.Fatalf("second run asked Jev %d times", len(jev.calls))
	}
	if second := stableJSON(t, readScores(path)); first != second {
		t.Fatalf("scores differ between runs:\n%s\n%s", first, second)
	}
}

func stableJSON(t *testing.T, f scoresFile) string {
	t.Helper()
	f.ScoredAt = ""
	for id, r := range f.Scores {
		r.ScoredAt = ""
		f.Scores[id] = r
	}
	raw, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
