package textureowner

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/quotematch"
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Support judgement, the Texture side
// (docs/archive/reports/support-judgement-to-jev-design-2026-10-14.md): a committed
// revision lists its new citation pairs by their check memos in the
// commit's own transaction, and the commit path hands them to the
// runtime's Jev job, which judges them after the commit. Ways it can fail,
// written first:
//   - the commit still sends management a pairs record (management's
//     serial turn back in the path), or lists nothing;
//   - the commit is not handed to Jev, so pairs stay pending forever;
//   - Jev's failing verdict never reaches this Texture's inbox;
//   - an unchanged pair is judged again on every later commit;
//   - the pair names the citing block by id but hashes other text, so an
//     edited sentence keeps the verdict its old text earned;
//   - the quote check outcome is not recorded per citation, or a citation
//     of a source nobody captured is memoized and never rechecked once a
//     capture lands;
//   - listing or judging mints a revision or edits text.

func managementInbox(rec types.RunRecord) string {
	return agentprofile.Management + ":" + rec.OwnerID
}

func blockIDAt(t *testing.T, rev types.Revision, i int) (string, string) {
	t.Helper()
	doc, _, err := texturewrite.Base(rev)
	if err != nil {
		t.Fatalf("read revision: %v", err)
	}
	blocks := texturewrite.DocBlocks(doc)
	if i >= len(blocks) {
		t.Fatalf("revision has %d blocks, want block %d", len(blocks), i+1)
	}
	return blocks[i].ID, texturewrite.CitingText(blocks[i].Node)
}

// supportJev answers every pair whose quote it names as unsupported and the
// rest as supported, and counts the pairs it was asked about.
type supportJev struct {
	mu          sync.Mutex
	unsupported map[string]bool
	asked       []string
}

func (j *supportJev) Judge(_ context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	var parsed struct {
		Pairs map[string]struct {
			Quote string `json:"quote"`
		} `json:"pairs"`
	}
	if err := json.Unmarshal(state, &parsed); err != nil {
		return nil, err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	answers := map[string]any{}
	for name := range questions {
		quote := parsed.Pairs[name].Quote
		j.asked = append(j.asked, quote)
		choice, yes := "yes", 0.95
		if j.unsupported[quote] {
			choice, yes = "no", 0.05
		}
		answers[name] = map[string]any{"type": "choice", "choice": choice, "probabilities": map[string]float64{"yes": yes, "no": 1 - yes}}
	}
	return json.Marshal(map[string]any{"model": "typesafe/jev-1.13", "answers": answers})
}

func (j *supportJev) askedQuotes() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]string(nil), j.asked...)
}

func TestCommitListsNewPairsAndJevJudgesThemOffTheCommit(t *testing.T) {
	core, h := testAPISetup(t)
	court := courtCapture["https://example.org/court"]
	core.SetTextureCaptures(fakeCaptureStore{"https://example.org/court": court, "cap-court": court})
	jev := &supportJev{unsupported: map[string]bool{"appeal was filed on Monday": true}}
	core.SetCandidateJudge(jev)
	s := core.Store()
	rec := seedCellAuthorLifecycle(t, s, "owner-cell-author", "support-list", "# Merger\n\nOpening line.\n\nClosing line.")
	h.createAgentMutationForRun(context.Background(), &rec)
	ctx := context.Background()
	base := headOf(t, s, rec)
	management, texture := managementInbox(rec), rec.AgentID
	write := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Write: &yaegikernel.TextureWrite{
		Edits: "@replace b2\nThe court ruled that the merger may proceed {{cite https://example.org/court \"the merger may proceed\"}}, and the regulator appealed {{cite https://example.org/court \"appeal was filed on Monday\"}}.",
		Why:   "state the ruling and the appeal",
	}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, write, "support-1"); err != nil {
		t.Fatalf("Write with two cites: %v", err)
	}
	core.WaitSupportJudgements()
	v1 := headOf(t, s, rec)
	blockID, blockText := blockIDAt(t, v1, 1)
	if blockText != "The court ruled that the merger may proceed, and the regulator appealed." {
		t.Fatalf("citing text = %q; want the sentence without its citation tokens", blockText)
	}
	if head, _ := s.InboxHead(ctx, rec.OwnerID, rec.ComputerID, management); head != 0 {
		t.Fatalf("management inbox head = %d; the commit must not send management its pairs", head)
	}

	// Each citation's quote check is recorded on the revision, and each
	// pair is listed by its check memo.
	checks := texturewrite.ChecksFrom(v1)
	if len(checks) != 2 {
		t.Fatalf("revision checks = %+v; want two", checks)
	}
	pairs := map[string]types.CitationCheck{}
	for _, c := range checks {
		if c.Outcome != quotematch.Matched || c.Tier != quotematch.TierExact || c.BlockID != blockID || c.Pair == "" || c.RefID == "" {
			t.Fatalf("check %+v; want an exact match naming its pair", c)
		}
		check, found, err := s.CitationCheck(ctx, rec.OwnerID, rec.ComputerID, c.Pair)
		if err != nil || !found || check.Outcome != string(quotematch.Matched) || !strings.HasPrefix(check.ListedBy, "texture-turn:") {
			t.Fatalf("check memo for %s = %+v, %v, %v; want matched, listed by the Texture turn", c.Pair, check, found, err)
		}
		p := check.Pair
		// A pair's text is the span its citation closes, with the block as
		// context (support-pair-judges-whole-paragraph-against-one-quote-
		// 2026-10-19).
		if p.DocID != rec.ChannelID || p.RevisionID != v1.RevisionID || p.BlockID != blockID || p.BlockHash != texturewrite.TextHash(p.Text) ||
			p.Text == "" || p.Text == blockText || !strings.Contains(blockText, strings.TrimSpace(strings.TrimRight(p.Text, ".,"))) || p.Context != blockText {
			t.Fatalf("pair %+v does not name its span of the citing block %s (%q) of %s", p, blockID, blockText, v1.RevisionID)
		}
		if p.Cited.Doc != "cap-court" || p.Cited.Version != "doc-hash-1" || p.Cited.CID == "" || p.Cited.Hash == "" || p.Matcher != quotematch.Version {
			t.Fatalf("pair %+v does not carry the cited Ref", p)
		}
		if p.Key != texturewrite.PairKey(p.BlockHash, p.Cited.Hash, p.Cited.Quote) {
			t.Fatalf("pair key %s is not the hash pair's key", p.Key)
		}
		pairs[p.Cited.Quote] = check
	}
	if len(pairs) != 2 {
		t.Fatalf("pairs = %v; want the sources' own spans", pairs)
	}

	// Jev judged both after the commit; the failure reached this Texture's
	// inbox, the pass only its memo.
	if asked := jev.askedQuotes(); len(asked) != 2 {
		t.Fatalf("Jev was asked about %v; want both pairs once", asked)
	}
	if v, found, _ := s.SupportVerdict(ctx, rec.OwnerID, rec.ComputerID, pairs["the merger may proceed"].Pair.Key); !found || v.Verdict != types.SupportSupported {
		t.Fatalf("ruling verdict = %+v, %v; want supported", v, found)
	}
	failures, err := s.ListInboxRecords(ctx, rec.OwnerID, rec.ComputerID, texture, 0, 10)
	if err != nil || len(failures) != 1 || failures[0].Support == nil || len(failures[0].Support.Failures) != 1 ||
		failures[0].Support.Failures[0].Key != pairs["appeal was filed on Monday"].Pair.Key || failures[0].SenderInbox != types.SupportSystemAgentID {
		t.Fatalf("texture inbox = %+v, %v; want one failure from %s on the appeal citation", failures, err, types.SupportSystemAgentID)
	}
	wakes, err := s.ListUnprojectedActorWakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	woken := false
	for _, w := range wakes {
		if w.TargetAgentID == texture && w.Kind == store.InboxRecordWakeKind {
			woken = true
		}
	}
	if !woken {
		t.Fatalf("no inbox wake for %s among %d wakes", texture, len(wakes))
	}
	if head := headOf(t, s, rec); head.RevisionID != v1.RevisionID {
		t.Fatal("judging minted a revision")
	}

	// A later commit that leaves the pairs unchanged asks Jev nothing.
	other := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: v1.RevisionID, Write: &yaegikernel.TextureWrite{Edits: "@replace b3\nA new closing line.", Why: "close"}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, other, "support-2"); err != nil {
		t.Fatalf("unrelated Write: %v", err)
	}
	core.WaitSupportJudgements()
	if asked := jev.askedQuotes(); len(asked) != 2 {
		t.Fatalf("Jev was asked about %v after an unrelated commit; unchanged pairs were judged again", asked)
	}

	// Editing the citing sentence makes a new pair, judged on its own.
	v2 := headOf(t, s, rec)
	edit := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: v2.RevisionID, Write: &yaegikernel.TextureWrite{
		Edits: "@replace b2\nThe court said the merger may proceed {{cite https://example.org/court \"the merger may proceed\"}}.", Why: "tighten",
	}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, edit, "support-3"); err != nil {
		t.Fatalf("edit the citing sentence: %v", err)
	}
	core.WaitSupportJudgements()
	if asked := jev.askedQuotes(); len(asked) != 3 || asked[2] != "the merger may proceed" {
		t.Fatalf("Jev was asked about %v; want the edited sentence's new pair", asked)
	}
	for _, c := range texturewrite.ChecksFrom(headOf(t, s, rec)) {
		if c.Pair == pairs["the merger may proceed"].Pair.Key {
			t.Fatal("an edited citing sentence kept its old pair key")
		}
	}
}

// legacyCitationBody is an owner revision with one citation of a source no
// one captured (no Ref): it can be shown, never checked or judged.
func legacyCitationBody(r *types.Revision) {
	doc := texturedoc.StructuredTextureDoc{Schema: texturedoc.SchemaV1, Doc: texturedoc.Node{Type: "doc", Attrs: map[string]any{"id": "doc-legacy"}, Content: []texturedoc.Node{
		{Type: "heading", Attrs: map[string]any{"id": "h-legacy", "level": 1}, Content: []texturedoc.Node{{Type: "text", Text: "Merger"}}},
		{Type: "paragraph", Attrs: map[string]any{"id": "p-legacy"}, Content: []texturedoc.Node{
			{Type: "text", Text: "A claim from a page nobody read"},
			{Type: "source_ref", Attrs: map[string]any{"id": "ref-legacy", "source_entity_id": "src-legacy", "display_mode": "numbered_ref"}},
		}},
		{Type: "paragraph", Attrs: map[string]any{"id": "p-close"}, Content: []texturedoc.Node{{Type: "text", Text: "Closing line."}}},
	}}}
	entities := []texturedoc.SourceEntity{{
		SourceEntityID: "src-legacy",
		Target:         texturedoc.SourceTarget{Kind: "web_url", URI: "https://example.org/unread"},
		Selectors:      []texturedoc.SourceSelector{{Kind: sourcecontract.SelectorKindWholeResource}},
		Display:        texturedoc.SourceDisplay{Mode: "numbered_ref", Title: "Unread page"},
		Evidence:       texturedoc.SourceEvidence{State: sourcecontract.EvidenceStateAvailable, OpenSurface: sourcecontract.OpenSurfaceSource},
		Provenance:     texturedoc.SourceEntityProvenance{CreatedBy: "owner"},
	}}
	r.BodyDoc, _ = json.Marshal(doc)
	r.SourceEntities, _ = json.Marshal(entities)
	r.Content = ""
}

func TestNotCapturedCitationIsCheckedButNeverMemoized(t *testing.T) {
	core, h := testAPISetup(t)
	core.SetTextureCaptures(courtCapture)
	s := core.Store()
	rec := seedCellAuthorLifecycle(t, s, "owner-cell-author", "support-uncaptured", "", legacyCitationBody)
	h.createAgentMutationForRun(context.Background(), &rec)
	ctx := context.Background()
	base := headOf(t, s, rec)
	write := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Write: &yaegikernel.TextureWrite{
		Edits: "@replace b3\nThe merger may proceed {{cite https://example.org/court \"the merger may proceed\"}}.", Why: "cite the ruling",
	}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, write, "uncaptured-1"); err != nil {
		t.Fatalf("Write beside a legacy citation: %v", err)
	}
	core.WaitSupportJudgements()
	v1 := headOf(t, s, rec)
	checks := texturewrite.ChecksFrom(v1)
	byRef := map[string]texturewrite.QuoteCheck{}
	for _, c := range checks {
		byRef[c.RefID] = c
	}
	legacy, ok := byRef["ref-legacy"]
	if !ok || legacy.Outcome != quotematch.NotCaptured || legacy.Pair != "" {
		t.Fatalf("legacy citation check = %+v (all %+v); want not_captured with no pair", legacy, checks)
	}
	listed := 0
	for _, c := range checks {
		if c.Pair == "" {
			continue
		}
		check, found, err := s.CitationCheck(ctx, rec.OwnerID, rec.ComputerID, c.Pair)
		if err != nil || !found || check.Pair.Cited.Doc != "cap-court" || strings.Contains(check.Pair.Text, "nobody read") {
			t.Fatalf("listed pair %+v, %v, %v; want only the captured citation's pair", check, found, err)
		}
		listed++
	}
	if listed != 1 {
		t.Fatalf("listed %d pairs; want only the captured citation's", listed)
	}
	if head, _ := s.InboxHead(ctx, rec.OwnerID, rec.ComputerID, managementInbox(rec)); head != 0 {
		t.Fatalf("management inbox head = %d; the commit sent management a record", head)
	}
	if head := headOf(t, s, rec); head.RevisionID != v1.RevisionID {
		t.Fatal("listing minted a revision")
	}
}
