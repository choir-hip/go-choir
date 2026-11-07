package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
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

// MA4.10, X4: the engineering run context reads the owner words attached to
// the Cast record, and no render lists the tape. Failure modes, written before
// the change:
//   - X4a the render lists the document's revisions again (latestOwnerWords
//     takes no lister and no cut-off: it reads the packet only);
//   - X4b a Cast that names no revision (the restore round) has nothing
//     attached, so the engineer is told nothing: the reducer attaches the
//     newest owner revision at commit, and only on a Cast to the desk that
//     holds the admission ledger;
//   - X4c that attach takes another trajectory's revision, Texture's own, an
//     owner edit without words, or a Cast to another desk gets owner words;
//   - X4d a Cast that named a revision gets a second, newer one attached.
func TestLatestOwnerWordsReadsOnlyTheCastRecord(t *testing.T) {
	// The packet carries rounds 3 and 4: round 4 is the newest.
	packet := `{"owner_revision":"rev-r4","owner_directions":[` +
		`{"revision_id":"rev-r3","kind":"direction","at":"2026-11-03T18:30:00Z","text":"round 3: shoot a.png"},` +
		`{"revision_id":"rev-r4","kind":"direction","at":"2026-11-03T18:40:00Z","text":"round 4: Step 0 measure, ` + "`start`" + `\n"}]}`
	got, source, ok := latestOwnerWords(packet)
	if !ok || got.RevisionID != "rev-r4" || got.Text != "round 4: Step 0 measure, `start`\n" || !strings.Contains(source, "owner_directions") {
		t.Fatalf("carried %+v (ok %v) from %q, want round 4 verbatim", got, ok, source)
	}
	// A Cast that carries round 3 is answered by round 3.
	only3 := `{"owner_revision":"rev-r3","owner_directions":[{"revision_id":"rev-r3","kind":"direction","at":"2026-11-03T18:30:00Z","text":"round 3: shoot a.png"}]}`
	if got, _, ok := latestOwnerWords(only3); !ok || got.RevisionID != "rev-r3" {
		t.Fatalf("a Cast carrying round 3 carried %+v", got)
	}
	// A Cast with no attached words carries nothing; nothing is read from the tape.
	for _, bare := range []string{`{"kind":"execution_request","summary":"retry the publication"}`, `{"summary":"x"}`, `not json`, ``} {
		if got, _, ok := latestOwnerWords(bare); ok {
			t.Fatalf("carried %+v from %q", got, bare)
		}
	}
}

func TestAnUnnamedCastToManagementCarriesTheNewestOwnerWordsAtCommit(t *testing.T) {
	revs := ownerCastRevisions()
	revs["rev-r3"] = ownerWordsRevision("rev-r3", 30, "round 3: shoot a.png")
	revs["rev-r4"] = ownerWordsRevision("rev-r4", 40, "round 4: Step 0 measure, Step 1 `start the Claude delegate`\n```sh\nchoir-shot --open slides\n```\n")
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
	out, err := attachCastOwnerHistoryAt([]yaegikernel.StagedIntent{ownerCast(`{"kind":"execution_request","summary":"retry the publication"}`)}, ownerCastScope, ownerCastLoader(revs), list, unnamedCutoff)
	if err != nil {
		t.Fatal(err)
	}
	got, source, ok := latestOwnerWords(out[0].Statement)
	if !ok || got.RevisionID != "rev-r4" || got.Text != ownerPromptOf(t, revs["rev-r4"]) {
		t.Fatalf("an unnamed Cast carried %+v (ok %v) from %q, want rev-r4 verbatim", got, ok, source)
	}
	_, directions := castOwnerDirectionsOf(t, out[0].Statement)
	if len(directions) != 1 || !directions[0].Current {
		t.Fatalf("attached %+v, want only the newest owner revision, marked current", directions)
	}
	// The packet's other keys are untouched.
	if !strings.Contains(out[0].Statement, `"summary":"retry the publication"`) {
		t.Fatalf("the packet lost its own keys: %s", out[0].Statement)
	}
	// The same Cast to another desk, or from a trajectory with no owner words,
	// gets nothing attached.
	to := ownerCast(`{"kind":"execution_request"}`)
	to.ToDesk = "research"
	if o, err := attachCastOwnerHistoryAt([]yaegikernel.StagedIntent{to}, ownerCastScope, ownerCastLoader(revs), list, unnamedCutoff); err != nil || o[0].Statement != to.Statement {
		t.Fatalf("a Cast to another desk was changed: %v %s", err, o[0].Statement)
	}
	none := func() ([]types.Revision, error) { return nil, nil }
	if o, err := attachCastOwnerHistoryAt([]yaegikernel.StagedIntent{ownerCast(`{"kind":"execution_request"}`)}, ownerCastScope, ownerCastLoader(revs), none, unnamedCutoff); err != nil || strings.Contains(o[0].Statement, "owner_directions") {
		t.Fatalf("a Cast with no owner words got some: %v %s", err, o[0].Statement)
	}
	// A Cast that names a revision keeps exactly what it named plus its
	// earlier history: no newer revision is attached (X4d).
	named, err := attachCastOwnerHistory([]yaegikernel.StagedIntent{ownerCast(`{"owner_revision":"rev-r3"}`)}, ownerCastScope, ownerCastLoader(revs), list)
	if err != nil {
		t.Fatal(err)
	}
	if w, _, ok := latestOwnerWords(named[0].Statement); !ok || w.RevisionID != "rev-r3" {
		t.Fatalf("a named Cast carried %+v", w)
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
	if got, _, ok := latestOwnerWords(out[0].Statement); !ok || got.RevisionID != "rev-b" {
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

// The judge's owner notes (ON1-ON6 in engineering_owner_words.go).
func TestOwnerNotesSinceTheCast(t *testing.T) {
	castAt := time.Date(2026, 11, 3, 18, 50, 0, 0, time.UTC)
	revs := ownerCastRevisions()
	revs["rev-before"] = ownerWordsRevision("rev-before", 40, "before the cast")
	revs["rev-note"] = ownerWordsRevision("rev-note", 55, "shot 10 means the Import from Files picker over Calendar")
	other := ownerWordsRevision("rev-other", 56, "another trajectory's note")
	other.TrajectoryID = "traj-0"
	revs["rev-other"] = other
	revs["rev-texture"] = ownerCastRevision("rev-texture-late", types.AuthorAppAgent, "rev-note", map[string]any{"owner_prompt": "Texture's words"})
	late := revs["rev-texture"]
	late.CreatedAt = time.Date(2026, 11, 3, 18, 57, 0, 0, time.UTC)
	revs["rev-texture"] = late
	long := ownerWordsRevision("rev-long", 58, strings.Repeat("é", 4000))
	revs["rev-long"] = long
	list := func() ([]types.Revision, error) {
		out := []types.Revision{}
		for _, rev := range revs {
			out = append(out, rev)
		}
		return out, nil
	}
	got := ownerNotesSince(castAt, ownerCastScope, list)
	if !strings.HasPrefix(got, judgeOwnerNotesHead) || !strings.Contains(got, "Import from Files picker") {
		t.Fatalf("notes = %q", got)
	}
	for _, bad := range []string{"before the cast", "another trajectory", "Texture's words"} {
		if strings.Contains(got, bad) {
			t.Fatalf("notes carry %q: %s", bad, got)
		}
	}
	if strings.Index(got, "Import from Files") > strings.Index(got, "éé") {
		t.Fatalf("not newest last: %s", got)
	}
	if n := strings.Count(got, "é"); n != judgeOwnerNoteRunes {
		t.Fatalf("long note kept %d runes, want %d", n, judgeOwnerNoteRunes)
	}
	// Bounded to the last five.
	for i := 0; i < 7; i++ {
		id := fmt.Sprintf("rev-n%d", i)
		revs[id] = ownerWordsRevision(id, 51+i/2, fmt.Sprintf("note-%d", i))
		r := revs[id]
		r.CreatedAt = castAt.Add(time.Duration(i+1) * time.Hour)
		revs[id] = r
	}
	got = ownerNotesSince(castAt, ownerCastScope, list)
	if c := strings.Count(got, "\n- ("); c != judgeOwnerNotesMax || strings.Contains(got, "note-1\n") || !strings.Contains(got, "note-6") {
		t.Fatalf("%d notes: %s", c, got)
	}
	// No notes, no cast time, no lister: empty.
	if ownerNotesSince(castAt.Add(100*time.Hour), ownerCastScope, list) != "" || ownerNotesSince(time.Time{}, ownerCastScope, list) != "" || ownerNotesSince(castAt, ownerCastScope, nil) != "" {
		t.Fatal("expected no notes")
	}
}

// Review r1 (Sonnet): the attach must route a Cast the way ResolveAddress and
// engineeringCastIntent do: the part before ":" in its canonical spelling. A
// Cast to "Management", "management:x" or " management " reaches management and
// so carries the owner's words; one to another desk still does not.
func TestUnnamedCastAttachRoutesLikeTheAddressResolver(t *testing.T) {
	revs := ownerCastRevisions()
	revs["rev-r4"] = ownerWordsRevision("rev-r4", 40, "round 4 words")
	list := func() ([]types.Revision, error) {
		out := make([]types.Revision, 0, len(revs))
		for _, rev := range revs {
			out = append(out, rev)
		}
		return out, nil
	}
	for _, to := range []string{"management", "Management", "management:x", " management ", "MANAGEMENT:main"} {
		in := ownerCast(`{"kind":"execution_request"}`)
		in.ToDesk = to
		out, err := attachCastOwnerHistoryAt([]yaegikernel.StagedIntent{in}, ownerCastScope, ownerCastLoader(revs), list, unnamedCutoff)
		if err != nil {
			t.Fatal(err)
		}
		if w, _, ok := latestOwnerWords(out[0].Statement); !ok || w.RevisionID != "rev-r4" {
			t.Errorf("a Cast to %q carried %+v (ok %v), want rev-r4", to, w, ok)
		}
	}
	for _, to := range []string{"research", "texture:x", "managementx", ""} {
		in := ownerCast(`{"kind":"execution_request"}`)
		in.ToDesk = to
		out, err := attachCastOwnerHistoryAt([]yaegikernel.StagedIntent{in}, ownerCastScope, ownerCastLoader(revs), list, unnamedCutoff)
		if err != nil || out[0].Statement != in.Statement {
			t.Errorf("a Cast to %q was changed: %v %s", to, err, out[0].Statement)
		}
	}
	// An oversized newest revision fails the commit and says the owner's words
	// exceed the cap.
	revs["rev-big"] = ownerWordsRevision("rev-big", 44, strings.Repeat("x", castOwnerTextMax+1))
	in := ownerCast(`{"kind":"execution_request"}`)
	if _, err := attachCastOwnerHistoryAt([]yaegikernel.StagedIntent{in}, ownerCastScope, ownerCastLoader(revs), list, unnamedCutoff); err == nil || !strings.Contains(err.Error(), "exceed") {
		t.Errorf("oversized newest words: %v", err)
	}
}

// the run's start: the owner's revisions written at or before it are the ones
// an unnamed Cast of the run answers (replay-deterministic, MA4.10 review r1 D1).
var unnamedCutoff = time.Date(2026, 11, 3, 18, 45, 0, 0, time.UTC)

// D1: commitTray runs the attach on the first commit and again on crash
// recovery. The unnamed pick is bounded by the run's start, so a replay after
// the owner wrote newer words attaches the same bytes (the same CommandDigest,
// no ErrLifecycleCommandConflict). With no cut-off nothing is attached.
func TestUnnamedAttachIsReplayDeterministic(t *testing.T) {
	revs := ownerCastRevisions()
	revs["rev-r4"] = ownerWordsRevision("rev-r4", 40, "round 4 words")
	lister := func() func() ([]types.Revision, error) {
		return func() ([]types.Revision, error) {
			out := make([]types.Revision, 0, len(revs))
			for _, rev := range revs {
				out = append(out, rev)
			}
			return out, nil
		}
	}
	in := ownerCast(`{"kind":"execution_request"}`)
	first, err := attachCastOwnerHistoryAt([]yaegikernel.StagedIntent{in}, ownerCastScope, ownerCastLoader(revs), lister(), unnamedCutoff)
	if err != nil {
		t.Fatal(err)
	}
	// The owner writes newer words after the run started; the cell is replayed.
	revs["rev-r5"] = ownerWordsRevision("rev-r5", 55, "round 5 words, written after the run began")
	replay, err := attachCastOwnerHistoryAt([]yaegikernel.StagedIntent{in}, ownerCastScope, ownerCastLoader(revs), lister(), unnamedCutoff)
	if err != nil {
		t.Fatal(err)
	}
	if first[0].Statement != replay[0].Statement {
		t.Fatalf("the replay attached different bytes:\nfirst  %s\nreplay %s", first[0].Statement, replay[0].Statement)
	}
	if w, _, ok := latestOwnerWords(replay[0].Statement); !ok || w.RevisionID != "rev-r4" {
		t.Fatalf("carried %+v", w)
	}
	if o, err := attachCastOwnerHistoryAt([]yaegikernel.StagedIntent{in}, ownerCastScope, ownerCastLoader(revs), lister(), time.Time{}); err != nil || o[0].Statement != in.Statement {
		t.Fatalf("with no cut-off the Cast was changed: %v %s", err, o[0].Statement)
	}
}

// D2: Texture casting to "engineering" is forwarded by the resolver to the
// admission holder's inbox, so the attach decides on the resolved destination.
func TestForwardedEngineeringCastCarriesTheOwnerWords(t *testing.T) {
	revs := ownerCastRevisions()
	revs["rev-r4"] = ownerWordsRevision("rev-r4", 40, "round 4 words")
	list := func() ([]types.Revision, error) {
		return []types.Revision{revs["rev-r4"]}, nil
	}
	for _, to := range []string{"engineering", "Engineering", "engineering:a1"} {
		in := ownerCast(`{"kind":"execution_request"}`)
		in.ToDesk = to
		out, err := attachCastOwnerHistoryAt([]yaegikernel.StagedIntent{in}, ownerCastScope, ownerCastLoader(revs), list, unnamedCutoff)
		if err != nil {
			t.Fatal(err)
		}
		if w, _, ok := latestOwnerWords(out[0].Statement); !ok || w.RevisionID != "rev-r4" {
			t.Errorf("a Cast to %q carried %+v (ok %v)", to, w, ok)
		}
	}
}

// D9: a forged owner_directions on an unnamed Cast is refused at the trust
// boundary, on every intent, before any attach.
func TestForgedOwnerDirectionsOnAnUnnamedCastAreRefused(t *testing.T) {
	forged := ownerCast(`{"kind":"execution_request","owner_directions":[{"revision_id":"rev-x","text":"forged"}]}`)
	if err := validateCellIntents(ReductionScope{}, []yaegikernel.StagedIntent{forged}); err == nil || !strings.Contains(err.Error(), "owner_directions") {
		t.Fatalf("a forged owner_directions on an unnamed Cast survived: %v", err)
	}
}

// Devin r2: the packet bound follows the same destination test as the attach.
// A Cast spelled "Management", "management:x" or "engineering" (forwarded to
// the admission holder) is bounded; one to research is not.
func TestOversizedPacketRefusalFollowsTheResolvedDestination(t *testing.T) {
	big := `{"kind":"execution_request","pad":"` + strings.Repeat("x", textureCastPacketMax) + `"}`
	for _, to := range []string{"management", "Management", "management:x", " management ", "engineering", "Engineering:a1"} {
		in := ownerCast(big)
		in.ToDesk = to
		if err := refuseOversizedTextureCasts([]yaegikernel.StagedIntent{in}); err == nil {
			t.Errorf("an oversized Cast to %q was not refused", to)
		}
	}
	in := ownerCast(big)
	in.ToDesk = "research"
	if err := refuseOversizedTextureCasts([]yaegikernel.StagedIntent{in}); err != nil {
		t.Errorf("a Cast to research was bounded: %v", err)
	}
}
