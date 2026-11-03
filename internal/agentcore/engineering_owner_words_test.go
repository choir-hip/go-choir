package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The owner's words reach the engineer verbatim (docs/problems/owner-words-
// restated-away-before-engineering-2026-11-03.md). Ways this can fail,
// written before the code:
//   - the run context carries management's restatement and no owner words;
//   - an older revision is carried as the operative one (pool r4 carried
//     round 3's text for round 4);
//   - a revision written after the Texture Cast, or another trajectory's,
//     Texture's own, or an owner edit without words, is carried;
//   - backticks, fences or newlines are escaped or trimmed;
//   - a text over the bound floods the context, or is cut without a mark, or
//     is cut mid-rune;
//   - a Texture Cast that named no owner revision (deepseek's restore round)
//     carries nothing though the owner wrote one;
//   - owner_directions comes out of order when Texture names the ids out of
//     order, or marks no entry (or more than one) as current;
//   - the objective management wrote is dropped.

func ownerWordsRevision(id string, minutes int, prompt string) types.Revision {
	rev := ownerCastRevision(id, types.AuthorUser, "rev-texture", map[string]any{"owner_prompt": prompt})
	rev.CreatedAt = time.Date(2026, 11, 3, 18, minutes, 0, 0, time.UTC)
	return rev
}

func TestOwnerDirectionsAreChronologicalAndMarkTheCurrentOne(t *testing.T) {
	revs := ownerCastRevisions()
	delete(revs, "rev-direction")
	root := revs["rev-root"]
	root.CreatedAt = time.Date(2026, 11, 3, 18, 0, 0, 0, time.UTC)
	revs["rev-root"] = root
	for i, id := range []string{"rev-r1", "rev-r2", "rev-r3", "rev-r4"} {
		revs[id] = ownerWordsRevision(id, 10*(i+1), "round "+string(rune('1'+i))+" words")
	}
	list := func() ([]types.Revision, error) {
		out := make([]types.Revision, 0, len(revs))
		for _, rev := range revs {
			out = append(out, rev)
		}
		return out, nil
	}
	// Texture names the newest first and the request after it: the list
	// must still read oldest to newest, and only round 4 is current.
	out, err := attachCastOwnerHistory([]yaegikernel.StagedIntent{ownerCast(`{"kind":"execution_request","owner_revision":["rev-r4","rev-root"]}`)}, ownerCastScope, ownerCastLoader(revs), list)
	if err != nil {
		t.Fatal(err)
	}
	_, directions := castOwnerDirectionsOf(t, out[0].Statement)
	var ids []string
	current := 0
	for _, d := range directions {
		ids = append(ids, d.RevisionID)
		if d.At == "" {
			t.Fatalf("entry %s has no time", d.RevisionID)
		}
		if d.Current {
			current++
		}
	}
	if strings.Join(ids, ",") != "rev-root,rev-r1,rev-r2,rev-r3,rev-r4" {
		t.Fatalf("owner_directions order %v, want oldest to newest", ids)
	}
	if current != 1 || !directions[len(directions)-1].Current || directions[len(directions)-1].Text != "round 4 words" {
		t.Fatalf("current marks %d entries; newest %+v", current, directions[len(directions)-1])
	}
	// The packet says so in words a model reads, not only as a flag.
	if !strings.Contains(out[0].Statement, `"current":true`) {
		t.Fatalf("no current flag on the packet: %s", out[0].Statement)
	}
}

func TestLatestOwnerWordsPicksTheNewestTheChainAnswers(t *testing.T) {
	revs := ownerCastRevisions()
	revs["rev-r3"] = ownerWordsRevision("rev-r3", 30, "round 3: shoot a.png")
	revs["rev-r4"] = ownerWordsRevision("rev-r4", 40, "round 4: Step 0 measure, Step 1 `start the Claude delegate`, 15 minute budget\n```sh\nchoir-shot --open slides\n```\n")
	revs["rev-after"] = ownerWordsRevision("rev-after", 55, "a direction after the Cast")
	other := ownerWordsRevision("rev-other-traj-new", 41, "another trajectory's newer words")
	other.TrajectoryID = "traj-0"
	revs["rev-other-traj-new"] = other
	texture := ownerCastRevision("rev-texture-new", types.AuthorAppAgent, "rev-r4", map[string]any{"owner_prompt": "Texture's newer words"})
	texture.CreatedAt = time.Date(2026, 11, 3, 18, 42, 0, 0, time.UTC)
	revs["rev-texture-new"] = texture
	edit := ownerCastRevision("rev-edit-new", types.AuthorUser, "rev-r4", map[string]any{"input_origin": "editor"})
	edit.CreatedAt = time.Date(2026, 11, 3, 18, 43, 0, 0, time.UTC)
	revs["rev-edit-new"] = edit
	list := func() ([]types.Revision, error) {
		out := make([]types.Revision, 0, len(revs))
		for _, rev := range revs {
			out = append(out, rev)
		}
		return out, nil
	}
	castAt := time.Date(2026, 11, 3, 18, 50, 0, 0, time.UTC)

	// The packet carries rounds 3 and 4 (rounds as the runtime attaches
	// them): round 4 is carried, not round 3.
	packet := `{"owner_revision":"rev-r4","owner_directions":[` +
		`{"revision_id":"rev-r3","kind":"direction","at":"2026-11-03T18:30:00Z","text":"round 3: shoot a.png"},` +
		`{"revision_id":"rev-r4","kind":"direction","at":"2026-11-03T18:40:00Z","text":` + ownerPromptJSON(t, revs["rev-r4"]) + `}]}`
	got, source, ok := latestOwnerWords(packet, castAt, ownerCastScope, list)
	if !ok || got.RevisionID != "rev-r4" || got.Text != ownerPromptOf(t, revs["rev-r4"]) {
		t.Fatalf("carried %+v (ok %v), want round 4 verbatim", got, ok)
	}
	if source == "" {
		t.Fatal("no source named")
	}

	// Deepseek's restore round: the Texture Cast named no revision. The
	// newest owner revision on the trajectory at or before the Cast is
	// carried; the later one, another trajectory's, Texture's and an
	// owner edit are not.
	got, source, ok = latestOwnerWords(`{"kind":"execution_request","summary":"retry the publication"}`, castAt, ownerCastScope, list)
	if !ok || got.RevisionID != "rev-r4" {
		t.Fatalf("no-name Cast carried %+v (ok %v), want rev-r4", got, ok)
	}
	if !strings.Contains(source, "carried no owner words") {
		t.Fatalf("source %q does not say the Cast carried none", source)
	}

	// Review c1-r1 (DeepSeek F1): a Cast that carries round 3 is answered
	// by round 3, even when the owner wrote round 4 before it committed;
	// round 4's own Cast carries round 4.
	only3 := `{"owner_revision":"rev-r3","owner_directions":[{"revision_id":"rev-r3","kind":"direction","at":"2026-11-03T18:30:00Z","text":"round 3: shoot a.png"}]}`
	if got, source, ok := latestOwnerWords(only3, castAt, ownerCastScope, list); !ok || got.RevisionID != "rev-r3" || !strings.Contains(source, "owner_directions") {
		t.Fatalf("a Cast carrying round 3 carried %+v from %q", got, source)
	}

	// A Cast with no time and no owner words carries nothing.
	if got, _, ok := latestOwnerWords(`{"summary":"x"}`, time.Time{}, ownerCastScope, list); ok {
		t.Fatalf("carried %+v with no cut-off and no packet words", got)
	}
}

// Review c1-r1 (DeepSeek F2): two revisions in one second, named newest
// first, still order oldest first and mark the newer current.
func TestOwnerDirectionsOrderWithinOneSecond(t *testing.T) {
	revs := ownerCastRevisions()
	a := ownerWordsRevision("rev-a", 30, "the earlier words")
	a.CreatedAt = a.CreatedAt.Add(100 * time.Millisecond)
	b := ownerWordsRevision("rev-b", 30, "the later words")
	b.CreatedAt = b.CreatedAt.Add(900 * time.Millisecond)
	revs["rev-a"], revs["rev-b"] = a, b
	out, err := attachCastOwnerDirections([]yaegikernel.StagedIntent{ownerCast(`{"owner_revision":["rev-b","rev-a"]}`)}, ownerCastScope, ownerCastLoader(revs))
	if err != nil {
		t.Fatal(err)
	}
	_, directions := castOwnerDirectionsOf(t, out[0].Statement)
	if len(directions) != 2 || directions[0].RevisionID != "rev-a" || directions[0].Current || directions[1].RevisionID != "rev-b" || !directions[1].Current {
		t.Fatalf("within one second: %+v", directions)
	}
	if got, _, ok := latestOwnerWords(out[0].Statement, time.Time{}, ownerCastScope, nil); !ok || got.RevisionID != "rev-b" {
		t.Fatalf("carried %+v, want rev-b", got)
	}
}

func ownerPromptJSON(t *testing.T, rev types.Revision) string {
	t.Helper()
	raw, err := marshalVerbatim(ownerPromptOf(t, rev))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func ownerPromptOf(t *testing.T, rev types.Revision) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rev.Metadata, &m); err != nil {
		t.Fatal(err)
	}
	s, _ := m["owner_prompt"].(string)
	return s
}

func TestOwnerWordsBlockIsVerbatimAndBounded(t *testing.T) {
	d := castOwnerDirection{RevisionID: "rev-r4", Kind: "direction", At: "2026-11-03T18:40:00Z", Text: ownerDirectionWithCode}
	block := ownerWordsBlock(d, "named on Texture's Cast", "texture-cell:Cast:x")
	if !strings.Contains(block, ownerWordsLabel) || !strings.Contains(block, ownerDirectionWithCode) || !strings.Contains(block, "rev-r4") || !strings.Contains(block, "2026-11-03T18:40:00Z") {
		t.Fatalf("block does not carry the label, the words verbatim, the id and the time:\n%s", block)
	}
	if strings.Contains(block, "truncated") {
		t.Fatal("a small text was marked truncated")
	}
	big := castOwnerDirection{RevisionID: "rev-big", Kind: "direction", Text: strings.Repeat("é", ownerWordsCarryMax)} // twice the bound in bytes
	block = ownerWordsBlock(big, "named on Texture's Cast", "texture-cell:Cast:x")
	if !utf8.ValidString(block) || len(block) > ownerWordsCarryMax+2048 {
		t.Fatalf("bounded block is %d bytes, valid %v", len(block), utf8.ValidString(block))
	}
	if !strings.Contains(block, "truncated") || !strings.Contains(block, "texture-cell:Cast:x") {
		t.Fatal("a cut text is not marked, or the mark does not say where the rest is")
	}
}

// End to end through the store: the engineering run's context carries the
// newest owner revision on the Texture request verbatim, beside the
// objective management wrote, and a fresh runtime reads the same.
func TestEngineeringRunContextCarriesTheOwnersWordsVerbatim(t *testing.T) {
	f := newRequestFixture(t)
	ctx := context.Background()
	round3 := "round 3: shoot a.png"
	round4 := "round 4: Step 0 measure first.\nStep 1 `start the Claude delegate` for the edits and shots.\n```sh\nchoir-shot --open slides\n```\n"
	body, err := json.Marshal(map[string]any{
		"kind":           "execution_request",
		"owner_revision": "rev-r4",
		"owner_directions": []castOwnerDirection{
			{RevisionID: "rev-r3", Kind: "direction", At: "2026-11-03T18:30:00Z", Text: round3},
			{RevisionID: "rev-r4", Kind: "direction", At: "2026-11-03T18:40:00Z", Text: round4, Current: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	parent := f.textureCast(t, f.seed.TextureInbox, "words", "retry the publication", string(body))
	f.open(t, &parent, "capability-words", boundDigest(parent))
	assignment, err := f.s.GetEngineeringAssignment(ctx, f.seed.OwnerID, f.seed.ComputerID, f.seed.AssignmentIDs[0], 1)
	if err != nil {
		t.Fatal(err)
	}
	objective, err := f.rt.assignmentObjective(ctx, assignment)
	if err != nil {
		t.Fatal(err)
	}
	run := buildAssignedEngineeringRun(assignment, "run-words", objective, "capsule-source-git:commit:x")
	for name, rt := range map[string]*Runtime{"running": f.rt, "fresh": {store: f.s}} {
		got := rt.runContextForRun(&run)
		if !strings.Contains(got, ownerWordsLabel) || !strings.Contains(got, round4) {
			t.Fatalf("%s: run context lacks the owner's words verbatim:\n%s", name, got)
		}
		if strings.Contains(got, round3) {
			t.Fatalf("%s: run context carries the older round's words:\n%s", name, got)
		}
	}
	if run.Prompt != objective || objective == "" {
		t.Fatalf("the run prompt %q is not management's objective %q", run.Prompt, objective)
	}

	// A binding whose digest does not match the tape carries nothing.
	g := newRequestFixture(t)
	parent = g.textureCast(t, g.seed.TextureInbox, "drift", "drift", string(body))
	g.open(t, &parent, "capability-drift", func(string, string, string) string { return objectgraph.SHA256([]byte("not the parent")) })
	assignment, err = g.s.GetEngineeringAssignment(ctx, g.seed.OwnerID, g.seed.ComputerID, g.seed.AssignmentIDs[0], 1)
	if err != nil {
		t.Fatal(err)
	}
	run = buildAssignedEngineeringRun(assignment, "run-drift", "x", "capsule-source-git:commit:x")
	if got := g.rt.runContextForRun(&run); strings.Contains(got, round4) {
		t.Fatalf("an unbound request was carried:\n%s", got)
	}
}
