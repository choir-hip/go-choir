package maild

// Failure modes this file guards (docs/reports/autopaper-daily-digest-design-
// 2026-10-20.md section 8; docs/problems/autopaper-daily-digest-phase1-2026-
// 10-20.md). Written before digest_rank.go.
//
//	M13 The Go copy of the front-page rank drifts from the TypeScript the page
//	    runs (frontend/src/lib/autopaper.ts rankFrontStories). The digest would
//	    then lead with a story the front page does not, and nothing would say
//	    so. Guard: one JSON fixture of front rows is ranked by the TypeScript
//	    function under node and by the Go function; the orders must be equal,
//	    for every case. The orders the TypeScript gave are also committed as a
//	    golden (testdata/digest_rank_golden.json) so a CI shell without node
//	    still checks the Go copy against what the page produced, and a shell
//	    with node checks the golden itself is not stale.
//	M13a The mirror reads a clock. The page's rank reads none (age is measured
//	    from the newest update in the list, the edition time), so the same list
//	    must rank the same at any wall-clock time. Guard: the same rows ranked
//	    at two different "now" values give one order (the rank takes no clock).
//	M13b Hostile or odd rows panic the ranker (missing fields, an unparsable
//	    date, an importance outside 1..10, a negative count). Guard: the
//	    fixture carries each of those, and an empty list ranks to an empty
//	    list.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

type digestRankFixture struct {
	Cases []struct {
		Name    string     `json:"name"`
		Stories []FrontRow `json:"stories"`
	} `json:"cases"`
}

func loadDigestRankFixture(t *testing.T) digestRankFixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "digest_front_fixture.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fx digestRankFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if len(fx.Cases) < 5 {
		t.Fatalf("fixture has %d cases, want at least 5", len(fx.Cases))
	}
	return fx
}

func goRankedIDs(rows []FrontRow) []string {
	ranked := rankFrontRows(rows)
	ids := make([]string, 0, len(ranked))
	for _, row := range ranked {
		ids = append(ids, row.PublicationID)
	}
	return ids
}

func loadDigestRankGolden(t *testing.T) map[string][]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "digest_rank_golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	golden := map[string][]string{}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatalf("decode golden: %v", err)
	}
	return golden
}

// The Go rank equals the order the front page's TypeScript produced.
func TestDigestRankMatchesCommittedGolden(t *testing.T) {
	fx := loadDigestRankFixture(t)
	golden := loadDigestRankGolden(t)
	for _, c := range fx.Cases {
		want, ok := golden[c.Name]
		if !ok {
			t.Fatalf("golden has no case %q", c.Name)
		}
		got := goRankedIDs(c.Stories)
		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("case %s: Go rank\n  got  %v\n  want %v", c.Name, got, want)
		}
	}
}

// Parity against the live TypeScript (M13): run rankFrontStories under node on
// the same fixture. Skipped when node is absent unless CHOIR_REQUIRE_NODE=1
// (set it in a CI shell that has node, so drift is a failure there).
func TestDigestRankParityWithFrontPageTypeScript(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("CHOIR_REQUIRE_NODE") == "1" {
			t.Fatalf("CHOIR_REQUIRE_NODE=1 but node is not on PATH: %v", err)
		}
		t.Skip("node not on PATH; the committed-golden test still guards the Go rank")
	}
	script := filepath.Join("..", "..", "frontend", "scripts", "rank-front-fixture.mjs")
	fixture := filepath.Join("testdata", "digest_front_fixture.json")
	out, err := exec.Command(node, script, fixture).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("node ranking failed: %v\n%s", err, ee.Stderr)
		}
		t.Fatalf("node ranking failed: %v", err)
	}
	var tsOrders map[string][]string
	if err := json.Unmarshal(out, &tsOrders); err != nil {
		t.Fatalf("decode node output: %v\n%s", err, out)
	}
	fx := loadDigestRankFixture(t)
	golden := loadDigestRankGolden(t)
	for _, c := range fx.Cases {
		ts := tsOrders[c.Name]
		got := goRankedIDs(c.Stories)
		if len(ts) == 0 && len(got) == 0 && len(golden[c.Name]) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, ts) {
			t.Errorf("case %s: Go rank differs from the front page's TypeScript rank\n  go %v\n  ts %v", c.Name, got, ts)
		}
		if !reflect.DeepEqual(golden[c.Name], ts) {
			t.Errorf("case %s: committed golden is stale; regenerate testdata/digest_rank_golden.json\n  golden %v\n  ts     %v", c.Name, golden[c.Name], ts)
		}
	}
}

// The rank takes no clock (M13a): ranking is a pure function of the list.
func TestDigestRankIsPureOfTheList(t *testing.T) {
	fx := loadDigestRankFixture(t)
	for _, c := range fx.Cases {
		first := goRankedIDs(c.Stories)
		second := goRankedIDs(c.Stories)
		if !reflect.DeepEqual(first, second) {
			t.Errorf("case %s: two rankings of one list differ: %v vs %v", c.Name, first, second)
		}
	}
}

// Ranking does not reorder or mutate the caller's slice.
func TestDigestRankDoesNotMutateInput(t *testing.T) {
	fx := loadDigestRankFixture(t)
	for _, c := range fx.Cases {
		before := make([]string, 0, len(c.Stories))
		for _, s := range c.Stories {
			before = append(before, s.PublicationID)
		}
		_ = rankFrontRows(c.Stories)
		after := make([]string, 0, len(c.Stories))
		for _, s := range c.Stories {
			after = append(after, s.PublicationID)
		}
		if !reflect.DeepEqual(before, after) {
			t.Errorf("case %s: input order changed", c.Name)
		}
	}
}

func TestDigestRankEmptyAndNil(t *testing.T) {
	if got := rankFrontRows(nil); len(got) != 0 {
		t.Fatalf("nil list ranked to %v", got)
	}
	if got := rankFrontRows([]FrontRow{}); len(got) != 0 {
		t.Fatalf("empty list ranked to %v", got)
	}
}
