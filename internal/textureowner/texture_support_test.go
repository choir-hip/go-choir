package textureowner

import (
	"context"
	"encoding/json"
	"strings"
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

// Support judgement, the Texture side (Cut 2 2a (v)): a committed revision
// lists its new citation pairs to management in the commit's own
// transaction, and records each citation's quote check where the reader
// reads it. Ways it can fail, written first:
//   - a commit with new citations lists nothing, mints one record per
//     citation, or addresses research instead of management;
//   - the list is a system record, and system records wake no one (D5), so
//     management never judges;
//   - an unchanged pair is listed again on every later commit, so
//     management wakes on every commit and judges the same pair twice;
//   - the pair names the citing block by id but hashes other text, so an
//     edited sentence keeps the verdict its old text earned;
//   - the quote check outcome is not recorded per citation, or a citation
//     of a source nobody captured is memoized and never rechecked once a
//     capture lands;
//   - listing mints a revision or edits text.

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
	return blocks[i].ID, texturewrite.PlainText(blocks[i].Node)
}

func TestCommitListsNewCitationPairsToManagementOnce(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "support-list")
	ctx := context.Background()
	base := headOf(t, s, rec)
	management := managementInbox(rec)
	write := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Write: &yaegikernel.TextureWrite{
		Edits: "@replace b2\nThe court ruled that the merger may proceed {{cite https://example.org/court \"the merger may proceed\"}}, and the regulator appealed {{cite https://example.org/court \"appeal was filed on Monday\"}}.",
		Why:   "state the ruling and the appeal",
	}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, write, "support-1"); err != nil {
		t.Fatalf("Write with two cites: %v", err)
	}
	v1 := headOf(t, s, rec)
	blockID, blockText := blockIDAt(t, v1, 1)

	got, err := s.ListInboxRecords(ctx, rec.OwnerID, rec.ComputerID, management, 0, 10)
	if err != nil || len(got) != 1 {
		t.Fatalf("management inbox after the commit = %d records, %v; want one list", len(got), err)
	}
	list := got[0]
	if list.EmitterKind != types.CommitmentEmitterSystem || list.SenderInbox != rec.AgentID || list.Support == nil || len(list.Support.Pairs) != 2 {
		t.Fatalf("the list = %+v; want one system record from %s with two pairs", list, rec.AgentID)
	}
	quotes := map[string]bool{}
	for _, p := range list.Support.Pairs {
		if p.DocID != rec.ChannelID || p.RevisionID != v1.RevisionID || p.BlockID != blockID || p.BlockHash != texturewrite.TextHash(blockText) || p.Text != blockText {
			t.Fatalf("pair %+v does not name the citing block %s (%q) of %s", p, blockID, blockText, v1.RevisionID)
		}
		if p.Cited.Doc != "cap-court" || p.Cited.Version != "doc-hash-1" || p.Cited.CID == "" || p.Cited.Hash == "" || p.Matcher != quotematch.Version {
			t.Fatalf("pair %+v does not carry the cited Ref", p)
		}
		if p.Key != texturewrite.PairKey(p.BlockHash, p.Cited.Hash, p.Cited.Quote) {
			t.Fatalf("pair key %s is not the hash pair's key", p.Key)
		}
		quotes[p.Cited.Quote] = true
		check, found, err := s.CitationCheck(ctx, rec.OwnerID, rec.ComputerID, p.Key)
		if err != nil || !found || check.ListedBy != list.RecordID || check.Outcome != string(quotematch.Matched) {
			t.Fatalf("check memo for %s = %+v, %v, %v; want matched, listed by %s", p.Key, check, found, err, list.RecordID)
		}
	}
	if !quotes["the merger may proceed"] || !quotes["appeal was filed on Monday"] {
		t.Fatalf("pair quotes = %v; want the sources' own spans", quotes)
	}

	// The list must wake management: it is the one system record that asks
	// a desk to act.
	wakes, err := s.ListUnprojectedActorWakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	woken := false
	for _, w := range wakes {
		if w.TargetAgentID == management && w.Kind == store.InboxRecordWakeKind {
			woken = true
		}
	}
	if !woken {
		t.Fatalf("no inbox wake for %s among %d wakes", management, len(wakes))
	}

	// Each citation's quote check is recorded on the revision.
	checks := texturewrite.ChecksFrom(v1)
	if len(checks) != 2 {
		t.Fatalf("revision checks = %+v; want two", checks)
	}
	for _, c := range checks {
		if c.Outcome != quotematch.Matched || c.Tier != quotematch.TierExact || c.BlockID != blockID || c.Pair == "" || c.RefID == "" {
			t.Fatalf("check %+v; want an exact match naming its pair", c)
		}
	}

	// A later commit that leaves the pairs unchanged lists nothing.
	other := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: v1.RevisionID, Write: &yaegikernel.TextureWrite{Edits: "@replace b3\nA new closing line.", Why: "close"}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, other, "support-2"); err != nil {
		t.Fatalf("unrelated Write: %v", err)
	}
	if head, _ := s.InboxHead(ctx, rec.OwnerID, rec.ComputerID, management); head != 1 {
		t.Fatalf("management inbox head = %d after an unrelated commit; unchanged pairs were listed again", head)
	}

	// Editing the citing sentence makes a new pair.
	v2 := headOf(t, s, rec)
	edit := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: v2.RevisionID, Write: &yaegikernel.TextureWrite{
		Edits: "@replace b2\nThe court said the merger may proceed {{cite https://example.org/court \"the merger may proceed\"}}.", Why: "tighten",
	}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, edit, "support-3"); err != nil {
		t.Fatalf("edit the citing sentence: %v", err)
	}
	got, _ = s.ListInboxRecords(ctx, rec.OwnerID, rec.ComputerID, management, 1, 10)
	if len(got) != 1 || got[0].Support == nil || len(got[0].Support.Pairs) != 1 {
		t.Fatalf("records after the edit = %+v; want one list with the new pair", got)
	}
	for _, p := range list.Support.Pairs {
		if got[0].Support.Pairs[0].Key == p.Key {
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
	got, err := s.ListInboxRecords(ctx, rec.OwnerID, rec.ComputerID, managementInbox(rec), 0, 10)
	if err != nil || len(got) != 1 || got[0].Support == nil || len(got[0].Support.Pairs) != 1 {
		t.Fatalf("management list = %+v, %v; want only the captured citation's pair", got, err)
	}
	if p := got[0].Support.Pairs[0]; p.Cited.Doc != "cap-court" || strings.Contains(p.Text, "nobody read") {
		t.Fatalf("listed pair %+v; the not-captured citation was listed", p)
	}
	if head := headOf(t, s, rec); head.RevisionID != v1.RevisionID {
		t.Fatal("listing minted a revision")
	}
}
