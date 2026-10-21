package texturewrite

// Ways a Write can fail, written before the code (Cut 2 unit 2a, item iv):
//
//  1. A Write that adds one paragraph re-mints every block id (the old
//     whole-document path did, tools_texture.go ~800), so citations and
//     support judgements keyed on block ids churn. Untouched blocks must keep
//     their ids and their citations byte for byte.
//  2. @all re-addresses the document: unchanged blocks must keep their ids.
//  3. @all alignment carries a citation onto text that no longer supports it.
//     A carried citation whose quote is gone is dropped and listed.
//  4. A {{cite}} names a handle that has no capture (A2): refused at Write,
//     with the line of the cite.
//  5. An ellipsis quote whose segments sit in two blocks (A4): refused, with
//     a message to split it into two cites.
//  6. A bad alias (b9 in a five-block document) is refused with its line.
//  7. A quote that is not in the capture is refused with its line.
//  8. Re-writing an existing citation unchanged must not need the capture
//     store (the token round-trips from the frame).
//  9. A malformed token or an unterminated quote is refused with its line.
// 10. @all combined with other ops, or two ops on one block, is ambiguous and
//     refused.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/quotematch"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type fakeCaptures struct {
	captures map[string]Capture
	calls    int
}

func (f *fakeCaptures) CaptureBlocks(_ context.Context, _ string, handle string) (Capture, error) {
	f.calls++
	c, ok := f.captures[handle]
	if !ok {
		return Capture{}, ErrNotCaptured
	}
	return c, nil
}

func newFakeCaptures() *fakeCaptures {
	return &fakeCaptures{captures: map[string]Capture{
		"https://example.org/court": {
			Doc: "cap-court", Version: "v1", URL: "https://example.org/court", Title: "Court ruling",
			Blocks: []quotematch.Block{
				{ID: "c1", Text: "The court ruled that the merger may proceed."},
				{ID: "c2", Text: "The appeal was filed on Monday by the regulator."},
			},
		},
	}}
}

func paragraph(id, text string) texturedoc.Node {
	return texturedoc.Node{Type: "paragraph", Attrs: map[string]any{"id": id}, Content: []texturedoc.Node{{Type: "text", Text: text}}}
}

func heading(id string, level int, text string) texturedoc.Node {
	return texturedoc.Node{Type: "heading", Attrs: map[string]any{"id": id, "level": level}, Content: []texturedoc.Node{{Type: "text", Text: text}}}
}

func baseDoc(blocks ...texturedoc.Node) texturedoc.StructuredTextureDoc {
	return texturedoc.StructuredTextureDoc{Schema: texturedoc.SchemaV1, Doc: texturedoc.Node{Type: "doc", Attrs: map[string]any{"id": "root"}, Content: blocks}}
}

// citedBase is a document whose second block carries a citation committed
// by an earlier Write.
func citedBase(t *testing.T) (texturedoc.StructuredTextureDoc, []texturedoc.SourceEntity) {
	t.Helper()
	caps := newFakeCaptures()
	res, err := Apply(context.Background(), Input{
		OwnerID: "owner", DocID: "doc", Seed: "rev-0",
		Base:     baseDoc(heading("h1", 1, "Merger"), paragraph("p1", "Opening line."), paragraph("p2", "Closing line.")),
		Script:   "@replace b2\nThe court ruled that the merger may proceed {{cite https://example.org/court \"the court ruled that the merger may proceed\"}}.",
		Resolver: caps,
	})
	if err != nil {
		t.Fatalf("seed cited base: %v", err)
	}
	return res.Doc, res.Entities
}

func blockJSON(t *testing.T, doc texturedoc.StructuredTextureDoc) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, n := range doc.Doc.Content {
		raw, _ := json.Marshal(n)
		out[n.Attrs["id"].(string)] = string(raw)
	}
	return out
}

func lineOf(t *testing.T, err error) int {
	t.Helper()
	var errs Errors
	if !errors.As(err, &errs) || len(errs) == 0 {
		t.Fatalf("error %v is not line-located", err)
	}
	return errs[0].Line
}

// Failure mode 1: one Write adds a paragraph with an inline cite; untouched
// blocks keep ids and citations (revision JSON diff).
func TestWriteAddsCitedParagraphAndKeepsUntouchedBlocks(t *testing.T) {
	doc, entities := citedBase(t)
	before := blockJSON(t, doc)
	caps := newFakeCaptures()
	res, err := Apply(context.Background(), Input{
		OwnerID: "owner", DocID: "doc", Seed: "rev-1", Base: doc, Entities: entities,
		Script:   "@insert after b2\nThe regulator appealed {{cite https://example.org/court \"appeal was filed on Monday\"}}.",
		Resolver: caps,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	after := blockJSON(t, res.Doc)
	if len(after) != len(before)+1 {
		t.Fatalf("blocks %d -> %d, want one more", len(before), len(after))
	}
	for id, raw := range before {
		if after[id] != raw {
			t.Fatalf("untouched block %s changed:\nbefore %s\nafter  %s", id, raw, after[id])
		}
	}
	added := res.Doc.Doc.Content[2]
	refs := sourceRefs(added)
	if len(refs) != 1 {
		t.Fatalf("added block refs = %d, want 1: %+v", len(refs), added)
	}
	ref, ok := RefOf(refs[0])
	if !ok || ref.Doc != "cap-court" || ref.CID != "c2" || ref.Quote != "appeal was filed on Monday" || ref.Hash == "" {
		t.Fatalf("ref = %+v", ref)
	}
	if err := texturedoc.Validate(res.Doc, res.Entities); err != nil {
		t.Fatalf("result does not validate: %v", err)
	}
}

// Failure mode 2: @all rewrite keeps ids of unchanged blocks.
func TestAllKeepsIDsOfUnchangedBlocks(t *testing.T) {
	doc, entities := citedBase(t)
	blocks := DocBlocks(doc)
	// Keep b1 and b2 verbatim (as the frame shows them), rewrite b3, add one.
	script := "@all\n" + blocks[0].Text + "\n\n" + blocks[1].Text + "\n\nA new closing line.\n\nAnd a coda."
	caps := newFakeCaptures()
	res, err := Apply(context.Background(), Input{OwnerID: "owner", DocID: "doc", Seed: "rev-2", Base: doc, Entities: entities, Script: script, Resolver: caps})
	if err != nil {
		t.Fatalf("Apply @all: %v", err)
	}
	before := blockJSON(t, doc)
	after := blockJSON(t, res.Doc)
	for _, b := range blocks[:2] {
		if after[b.ID] != before[b.ID] {
			t.Fatalf("unchanged block %s (%s) changed under @all", b.Alias, b.ID)
		}
	}
	// The rewritten closing line pairs with b3 and keeps its id.
	if got := nodeID(res.Doc.Doc.Content[2]); got != blocks[2].ID {
		t.Fatalf("paired changed block id = %s, want %s", got, blocks[2].ID)
	}
	if caps.calls != 0 {
		t.Fatalf("round-tripped citation called the capture store %d times", caps.calls)
	}
}

// Failure mode 3: a carried citation whose quote is gone is dropped and
// listed in the next frame.
func TestAllDropsCarriedCitationWhoseQuoteIsGone(t *testing.T) {
	doc, entities := citedBase(t)
	blocks := DocBlocks(doc)
	script := "@all\n" + blocks[0].Text + "\n\nThe judges let the deal go ahead.\n\n" + blocks[2].Text
	res, err := Apply(context.Background(), Input{OwnerID: "owner", DocID: "doc", Seed: "rev-3", Base: doc, Entities: entities, Script: script, Resolver: newFakeCaptures()})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(sourceRefs(res.Doc.Doc.Content[1])) != 0 {
		t.Fatalf("citation was carried onto text without its quote: %+v", res.Doc.Doc.Content[1])
	}
	if len(res.Dropped) != 1 || res.Dropped[0].Quote == "" || res.Dropped[0].BlockID != blocks[1].ID {
		t.Fatalf("dropped = %+v, want the one citation of %s", res.Dropped, blocks[1].ID)
	}
	if len(res.Entities) != 0 {
		t.Fatalf("an uncited source entity stayed: %+v", res.Entities)
	}
	frame := Frame{DocID: "doc", RevisionID: "rev-3", Blocks: DocBlocks(res.Doc), Dropped: res.Dropped}.Render()
	if !strings.Contains(frame, "dropped") || !strings.Contains(frame, res.Dropped[0].Quote) {
		t.Fatalf("frame does not list the dropped citation:\n%s", frame)
	}

	// Kept quote: the same rewrite that still says the quoted words carries it.
	script = "@all\n" + blocks[0].Text + "\n\nOn Friday the court ruled that the merger may proceed.\n\n" + blocks[2].Text
	res, err = Apply(context.Background(), Input{OwnerID: "owner", DocID: "doc", Seed: "rev-3b", Base: doc, Entities: entities, Script: script, Resolver: newFakeCaptures()})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(sourceRefs(res.Doc.Doc.Content[1])) != 1 || len(res.Dropped) != 0 {
		t.Fatalf("citation whose quote still appears was not carried: %+v dropped %+v", res.Doc.Doc.Content[1], res.Dropped)
	}
}

// Failure mode 4 (A2): a cite whose handle names no capture is refused with
// its line.
func TestCiteOfUncapturedHandleIsRefusedWithLine(t *testing.T) {
	doc, entities := citedBase(t)
	script := "@insert end\nFirst new line.\n\nA claim {{cite https://example.org/snippet-only \"some words\"}}."
	_, err := Apply(context.Background(), Input{OwnerID: "owner", DocID: "doc", Seed: "rev-4", Base: doc, Entities: entities, Script: script, Resolver: newFakeCaptures()})
	if err == nil {
		t.Fatal("cite of an uncaptured source committed")
	}
	if got := lineOf(t, err); got != 4 {
		t.Fatalf("refusal line = %d, want 4: %v", got, err)
	}
	if !strings.Contains(err.Error(), "not captured") {
		t.Fatalf("refusal does not say not captured: %v", err)
	}
}

// Failure mode 5 (A4): a cross-block ellipsis quote is refused with the
// split message.
func TestCrossBlockEllipsisQuoteIsRefusedWithSplitMessage(t *testing.T) {
	doc, entities := citedBase(t)
	script := "@insert end\nBoth {{cite https://example.org/court \"the merger may proceed ... filed on Monday\"}}."
	_, err := Apply(context.Background(), Input{OwnerID: "owner", DocID: "doc", Seed: "rev-5", Base: doc, Entities: entities, Script: script, Resolver: newFakeCaptures()})
	if err == nil {
		t.Fatal("cross-block ellipsis quote committed")
	}
	if got := lineOf(t, err); got != 2 {
		t.Fatalf("refusal line = %d, want 2", got)
	}
	if !strings.Contains(err.Error(), "split") || !strings.Contains(err.Error(), "two cites") {
		t.Fatalf("refusal does not tell Texture to split the quote: %v", err)
	}
}

// Failure modes 6, 7, 9, 10: bad alias, quote not in capture, malformed
// token, ambiguous ops, each refused with its line.
func TestRefusalsAreLineLocated(t *testing.T) {
	doc, entities := citedBase(t)
	cases := []struct {
		name, script, want string
		line               int
	}{
		{"bad alias", "@replace b2\nfine\n\n@delete b9", "b9", 4},
		{"quote not in capture", "@insert end\nX {{cite https://example.org/court \"the court dismissed it\"}}.", "not found", 2},
		{"unterminated quote", "@insert end\nX {{cite https://example.org/court \"open quote}}.", "quote", 2},
		{"unclosed token", "@insert end\nok\nX {{cite https://example.org/court", "{{", 3},
		{"unknown directive", "@rewrite b1\nx", "@rewrite", 1},
		{"text before directive", "hello\n@delete b1", "directive", 1},
		{"all with other ops", "@delete b1\n@all\nx", "@all", 2},
		{"two ops on one block", "@delete b2\n@replace b2\nx", "b2", 2},
		{"section on a paragraph", "@section b2\nx", "heading", 1},
		{"empty replace", "@replace b2", "text", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Apply(context.Background(), Input{OwnerID: "owner", DocID: "doc", Seed: "rev-6", Base: doc, Entities: entities, Script: tc.script, Resolver: newFakeCaptures()})
			if err == nil {
				t.Fatalf("script committed:\n%s", tc.script)
			}
			if got := lineOf(t, err); got != tc.line {
				t.Fatalf("line = %d, want %d: %v", got, tc.line, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

// Check is the synchronous worker-side subset: grammar, aliases and frame
// handles, with no capture store.
func TestCheckRefusesBadAliasAndUnknownFrameHandle(t *testing.T) {
	if err := Check("@replace b2\nx", 3, map[string]bool{"r1.c1": true}); err != nil {
		t.Fatalf("valid script refused: %v", err)
	}
	err := Check("@replace b4\nx", 3, nil)
	if err == nil || lineOf(t, err) != 1 || !strings.Contains(err.Error(), "b4") {
		t.Fatalf("bad alias = %v", err)
	}
	err = Check("@insert end\nok\n\nX {{cite r2.c1 \"q\"}}", 3, map[string]bool{"r1.c1": true})
	if err == nil || lineOf(t, err) != 4 || !strings.Contains(err.Error(), "r2.c1") {
		t.Fatalf("unknown frame handle = %v", err)
	}
}

// Move, delete and section edit structure without touching other blocks.
func TestMoveDeleteSection(t *testing.T) {
	base := baseDoc(heading("h1", 1, "Title"), paragraph("p1", "One."), heading("h2", 2, "Part"), paragraph("p2", "Two."), paragraph("p3", "Three."), heading("h3", 2, "End"), paragraph("p4", "Four."))
	res, err := Apply(context.Background(), Input{OwnerID: "o", DocID: "d", Seed: "s", Base: base, Script: "@move b2 after b5\n@delete b7\n@section b3\n## Part\n\nTwo.\n\nNew three.", Resolver: newFakeCaptures()})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	var ids []string
	for _, n := range res.Doc.Doc.Content {
		ids = append(ids, nodeID(n))
	}
	got := strings.Join(ids, ",")
	if !strings.HasPrefix(got, "h1,h2,p2,") || !strings.HasSuffix(got, ",p1,h3") || len(ids) != 6 {
		t.Fatalf("order = %s", got)
	}
	if text := DocBlocks(res.Doc)[3].Text; text != "New three." {
		t.Fatalf("section rewrite block = %q", text)
	}
}

// A legacy citation (a source_ref without a Ref) round-trips through its
// {{ref}} token.
func TestLegacyRefTokenRoundTrips(t *testing.T) {
	entity := texturedoc.SourceEntity{SourceEntityID: "src_old", Target: texturedoc.SourceTarget{Kind: "web_url", URI: "https://old.example"},
		Display: texturedoc.SourceDisplay{Mode: "numbered_ref"}, Evidence: texturedoc.SourceEvidence{State: "available", OpenSurface: "source"}, Provenance: texturedoc.SourceEntityProvenance{CreatedBy: "runtime"}}
	p := paragraph("p1", "Old claim.")
	p.Content = append(p.Content, texturedoc.Node{Type: "source_ref", Attrs: map[string]any{"id": "r1", "source_entity_id": "src_old", "display_mode": "numbered_ref"}})
	base := baseDoc(p, paragraph("p2", "Other."))
	blocks := DocBlocks(base)
	if !strings.Contains(blocks[0].Text, "{{ref src_old}}") {
		t.Fatalf("legacy citation renders as %q", blocks[0].Text)
	}
	res, err := Apply(context.Background(), Input{OwnerID: "o", DocID: "d", Seed: "s", Base: base, Entities: []texturedoc.SourceEntity{entity}, Script: "@replace b1\nOld claim, restated {{ref src_old}}.", Resolver: newFakeCaptures()})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(sourceRefs(res.Doc.Doc.Content[0])) != 1 || len(res.Entities) != 1 {
		t.Fatalf("legacy ref lost: %+v", res.Doc.Doc.Content[0])
	}
}

// The frame shows blocks by alias with their citations as tokens, and the
// new records with their sources by handle.
func TestFrameRendersAliasesRecordsAndSources(t *testing.T) {
	doc, _ := citedBase(t)
	packet := `{"kind":"evidence_update","summary":"Ruling found","claims":[{"text":"The court allowed the merger","source_ids":["s1"]}],"sources":[{"source_id":"s1","kind":"web","target":{"uri":"https://example.org/court","title":"Court ruling"},"excerpt":"The court ruled that the merger may proceed."}]}`
	records := RecordsFrom([]RecordInput{{Seq: 7, RecordID: "rec-7", Kind: "report", From: "research:doc", Round: 1, Final: true, Body: packet}})
	frame := Frame{DocID: "doc", RevisionID: "rev-1", Blocks: DocBlocks(doc), Records: records}.Render()
	for _, want := range []string{"b1 ", "b2 ", "b3 ", "{{cite cap-court", "r1 report from research:doc", "r1.c1", "https://example.org/court", "The court allowed the merger [r1.c1]"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("frame missing %q:\n%s", want, frame)
		}
	}
	handles := Handles(records)
	if handles["r1.c1"].URL != "https://example.org/court" {
		t.Fatalf("handles = %+v", handles)
	}
	// A source research Read names its capture; the handle resolves by it.
	captured := RecordsFrom([]RecordInput{{Seq: 8, Kind: "report", From: "research:doc", Body: `{"kind":"evidence_update","summary":"s","sources":[{"source_id":"s1","target":{"uri":"https://example.org/court","capture_id":"cap-1"}}]}`}})
	if src := Handles(captured)["r1.c1"]; src.CaptureID != "cap-1" || src.Key() != "cap-1" {
		t.Fatalf("captured source = %+v, want it keyed by its capture id", src)
	}
}

// The not-captured hint sends a web source to research and keeps research
// away from an internal record it cannot Read (adopt-1418: Texture cast
// research to capture an engineering assignment report; research got
// "record not found").
func TestFrameNotCapturedHintDependsOnWhetherResearchCanRead(t *testing.T) {
	packet := `{"kind":"evidence_update","summary":"Adoption blocked","claims":[{"text":"Engineering could not copy the release tree","source_ids":["s1","s2"]}],"sources":[{"source_id":"s1","kind":"web","target":{"uri":"https://example.org/court","title":"Court ruling"}},{"source_id":"s2","kind":"report","target":{"uri":"assignment-report:report:sha256:abc","title":"Engineering final report"}}]}`
	records := RecordsFrom([]RecordInput{{Seq: 7, RecordID: "rec-7", Kind: "report", From: "management:o", Body: packet}})
	frame := Frame{DocID: "doc", RevisionID: "rev-1", Records: records, Captured: map[string]bool{}}.Render()
	var web, internal string
	for _, line := range strings.Split(frame, "\n") {
		switch {
		case strings.Contains(line, "https://example.org/court"):
			web = line
		case strings.Contains(line, "assignment-report:report:sha256:abc"):
			internal = line
		}
	}
	if !strings.Contains(web, "until research Reads it") {
		t.Fatalf("web source line = %q, want the research-Reads hint\n%s", web, frame)
	}
	if strings.Contains(internal, "until research Reads it") || !strings.Contains(internal, "research cannot Read it") || !strings.Contains(internal, "management owns one bounded re-cast") {
		t.Fatalf("internal record line = %q, want the cannot-Read hint naming management and the owner\n%s", internal, frame)
	}
}

// Support judgement (Cut 2 2a (v)): a committed citation's check carries its
// tier and pair, and management's failing verdict reaches Texture's frame on
// the block it fails. Ways it can fail: a tolerant match is reported exact;
// the pair hashes the sentence with its citation tokens in it, so the key
// changes whenever a citation is added beside it; the failure names a block
// id Texture cannot address, or drops why.
func TestChecksPairCitationsAndTheFrameShowsAFailure(t *testing.T) {
	doc, _ := citedBase(t)
	checks, pairs := Checks(doc, "doc", "rev-1")
	if len(checks) != 1 || len(pairs) != 1 {
		t.Fatalf("checks %+v, pairs %+v; want one of each", checks, pairs)
	}
	p := pairs[0]
	if checks[0].Outcome != quotematch.Matched || checks[0].Tier != quotematch.TierWSCase || checks[0].Pair != p.Key {
		t.Fatalf("check = %+v; want a tolerant match naming its pair", checks[0])
	}
	if p.Text != "The court ruled that the merger may proceed." || p.BlockHash != TextHash(p.Text) || p.Key != PairKey(p.BlockHash, p.Cited.Hash, p.Cited.Quote) {
		t.Fatalf("pair = %+v; want the sentence's words without tokens, hashed", p)
	}
	failure := types.SupportVerdict{Key: p.Key, Verdict: types.SupportUnsupported, Why: "the ruling was stayed", Judge: "management", Pair: &p}
	records := RecordsFrom([]RecordInput{{Seq: 3, RecordID: "rec-3", Kind: "support_failure", From: "management:owner",
		Support: &types.CommitmentSupport{Failures: []types.SupportVerdict{failure}}}})
	frame := Frame{DocID: "doc", RevisionID: "rev-1", Blocks: DocBlocks(doc), Records: records}.Render()
	for _, want := range []string{"r1 support_failure from management:owner", "unsupported citations", "- b2 cites cap-court", "the ruling was stayed"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("frame missing %q:\n%s", want, frame)
		}
	}
}

// A long document's frame carries a section digest with the recently edited
// and cited blocks in full, not every block.
func TestLongDocumentFrameDigests(t *testing.T) {
	var blocks []texturedoc.Node
	blocks = append(blocks, heading("h1", 1, "Long"))
	for i := 0; i < 60; i++ {
		blocks = append(blocks, paragraph("p"+strings.Repeat("x", i+1), strings.Repeat("Filler sentence for a long document. ", 12)))
	}
	doc := baseDoc(blocks...)
	full := 0
	for _, b := range DocBlocks(doc) {
		full += len(b.Text)
	}
	edited := DocBlocks(doc)[30]
	f := Frame{DocID: "d", RevisionID: "r", Blocks: DocBlocks(doc), Changed: map[string]bool{edited.ID: true}}
	digest := f.Render()
	if len(digest) >= full || len(digest) > 2*FrameDocBudget {
		t.Fatalf("long frame not digested: %d chars (document %d)", len(digest), full)
	}
	if !strings.Contains(digest, "choir.ReadDoc()") {
		t.Fatalf("digest does not point at ReadDoc:\n%s", digest[:400])
	}
	if !strings.Contains(digest, edited.Alias+" "+edited.Text) {
		t.Fatalf("the edited block %s is not in full", edited.Alias)
	}
}

// An owner draft is stored as one paragraph of lines joined by hard breaks.
// Read as one block, the whole draft is b1 and every Write rewrites all of
// it; read as markdown, the owner's headings and paragraphs are blocks.
func TestBaseReadsAnOwnerDraftAsItsBlocks(t *testing.T) {
	lines := []texturedoc.Node{{Type: "text", Text: "# Title"}, {Type: "hard_break"}, {Type: "hard_break"}, {Type: "text", Text: "First."}, {Type: "hard_break"}, {Type: "hard_break"}, {Type: "text", Text: "Second."}}
	body, _ := json.Marshal(baseDoc(texturedoc.Node{Type: "paragraph", Attrs: map[string]any{"id": "p"}, Content: lines}))
	doc, _, err := Base(types.Revision{RevisionID: "r", DocID: "d", AuthorKind: types.AuthorUser, Content: "# Title\n\nFirst.\n\nSecond.", BodyDoc: body})
	if err != nil {
		t.Fatal(err)
	}
	blocks := DocBlocks(doc)
	if len(blocks) != 3 || blocks[0].Text != "# Title" || blocks[2].Text != "Second." {
		t.Fatalf("owner draft blocks = %+v", blocks)
	}
	again, _, _ := Base(types.Revision{RevisionID: "r", DocID: "d", AuthorKind: types.AuthorUser, Content: "# Title\n\nFirst.\n\nSecond.", BodyDoc: body})
	if DocBlocks(again)[1].ID != blocks[1].ID {
		t.Fatal("owner draft block ids are not stable, so the frame and the commit would disagree")
	}
}

// A support pair judged a whole paragraph against one quote, so a
// paragraph that drew on two outlets could never pass (docs/problems/
// support-pair-judges-whole-paragraph-against-one-quote-2026-10-19.md). A
// pair's text is now the span its citation closes, with the paragraph as
// context. Ways it can fail: S1 the closing period lands in the next span;
// S2 side-by-side citations get different spans; S3 a lone citation at a
// paragraph's end does not cover the paragraph; S4 an edit to another
// sentence re-keys this citation; S7 the frame does not say which span
// failed.
func TestChecksPairEachCitationWithTheSpanItCloses(t *testing.T) {
	caps := newFakeCaptures()
	caps.captures["https://example.org/wire"] = Capture{
		Doc: "cap-wire", Version: "v1", URL: "https://example.org/wire", Title: "Wire report",
		Blocks: []quotematch.Block{{ID: "w1", Text: "The regulator appealed the ruling on Monday."}},
	}
	write := func(first string) texturedoc.StructuredTextureDoc {
		t.Helper()
		res, err := Apply(context.Background(), Input{
			OwnerID: "owner", DocID: "doc", Seed: "rev-0",
			Base: baseDoc(heading("h1", 1, "Merger"), paragraph("p1", "Opening line."), paragraph("p2", "Closing line.")),
			Script: "@replace b2\n" + first + " {{cite https://example.org/court \"the court ruled that the merger may proceed\"}}. " +
				"The regulator appealed on Monday {{cite https://example.org/court \"The appeal was filed on Monday by the regulator\"}} " +
				"{{cite https://example.org/wire \"The regulator appealed the ruling on Monday\"}}. An uncited closing remark.\n" +
				"@replace b3\nThe merger was announced in spring. It closes next year {{cite https://example.org/court \"the court ruled that the merger may proceed\"}}.",
			Resolver: caps,
		})
		if err != nil {
			t.Fatalf("write: %v", err)
		}
		return res.Doc
	}
	doc := write("The court ruled that the merger may proceed")
	_, pairs := Checks(doc, "doc", "rev-1")
	if len(pairs) != 4 {
		t.Fatalf("pairs = %+v; want four", pairs)
	}
	if pairs[0].Text != "The court ruled that the merger may proceed." {
		t.Errorf("S1: first span = %q", pairs[0].Text)
	}
	if pairs[1].Text != "The regulator appealed on Monday." || pairs[2].Text != pairs[1].Text {
		t.Errorf("S2: side-by-side spans = %q and %q; want the same sentence", pairs[1].Text, pairs[2].Text)
	}
	if pairs[3].Text != "The merger was announced in spring. It closes next year." {
		t.Errorf("S3: a lone closing citation spans %q; want the whole paragraph", pairs[3].Text)
	}
	whole := "The court ruled that the merger may proceed. The regulator appealed on Monday. An uncited closing remark."
	for i, p := range pairs[:3] {
		if p.Context != whole {
			t.Errorf("pair %d context = %q; want the paragraph", i, p.Context)
		}
		if p.BlockHash != TextHash(p.Text) || p.Key != PairKey(p.BlockHash, p.Cited.Hash, p.Cited.Quote) {
			t.Errorf("pair %d is not keyed by its span: %+v", i, p)
		}
	}
	_, again := Checks(write("A court ruled that the merger may proceed"), "doc", "rev-2")
	if again[0].Key == pairs[0].Key {
		t.Error("an edited span kept its key")
	}
	if again[1].Key != pairs[1].Key || again[2].Key != pairs[2].Key {
		t.Error("S4: an edit to another sentence re-keyed this citation")
	}

	failure := types.SupportVerdict{Key: pairs[1].Key, Verdict: types.SupportUnsupported, Why: "not said", Judge: "jev:test", Pair: &pairs[1]}
	frame := Frame{DocID: "doc", RevisionID: "rev-1", Blocks: DocBlocks(doc), Records: RecordsFrom([]RecordInput{{Seq: 3, RecordID: "rec-3", Kind: "report", From: "system:support",
		Support: &types.CommitmentSupport{Failures: []types.SupportVerdict{failure}}}})}.Render()
	if !strings.Contains(frame, `"The regulator appealed on Monday."`) {
		t.Errorf("S7: the frame does not quote the failing span:\n%s", frame)
	}
}

// A reader's request for more research is told to Texture in every frame
// while it is pending (docs/design/research-more-goes-to-research-2026-10-21.md
// failure mode 11), and never otherwise.
func TestFrameTellsAPendingReaderRequest(t *testing.T) {
	doc, _ := citedBase(t)
	plain := Frame{DocID: "doc", RevisionID: "rev-1", Blocks: DocBlocks(doc)}.Render()
	if strings.Contains(plain, "A reader asked for more research") {
		t.Fatalf("frame without a request tells one:\n%s", plain)
	}
	asked := Frame{DocID: "doc", RevisionID: "rev-1", Blocks: DocBlocks(doc), ReaderRequest: "expand:pub-1:rev-1"}.Render()
	for _, want := range []string{"A reader asked for more research", "expand:pub-1:rev-1", "keep its text, structure and citations"} {
		if !strings.Contains(asked, want) {
			t.Fatalf("frame missing %q:\n%s", want, asked)
		}
	}
}
