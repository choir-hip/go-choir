package textureowner

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The committer's receipt names the records its turn sent, with the round each
// opened: the runtime logs each Cast to research from it (story research cast,
// agentcore). Failure list: a turn that sends a Cast answers without it (a
// Wait-only turn or a Write turn); a record's round or opened flag is missing.
func TestTurnReceiptNamesTheInboxRecordsItSent(t *testing.T) {
	type receipt struct {
		InboxRecords []types.InboxRecordReceipt `json:"inbox_records"`
	}
	h, s, rec := cellAuthorSetup(t, "turn-receipt")
	ctx := context.Background()
	base := headOf(t, s, rec)
	wait := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "asked research", InboxRecords: []types.CommitmentRecord{researchCast(rec, "texture-cell:cast:receipt-1")}})
	out, err := h.CommitCellTextureAuthor(ctx, &rec, wait, "receipt-1")
	if err != nil {
		t.Fatalf("Wait with a Cast: %v", err)
	}
	var got receipt
	if err := json.Unmarshal([]byte(out), &got); err != nil || len(got.InboxRecords) != 1 {
		t.Fatalf("a Wait turn's receipt lacks its Cast's record: %s (%v)", out, err)
	}
	if r := got.InboxRecords[0]; r.Inbox != "research:"+rec.ChannelID || r.Round != 1 || !r.OpenedRound {
		t.Fatalf("record receipt = %+v, want research:%s, round 1, opened", r, rec.ChannelID)
	}
	// A turn that sends nothing carries no records.
	v1 := headOf(t, s, rec)
	quiet := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: v1.RevisionID, Wait: "still checking"})
	out, err = h.CommitCellTextureAuthor(ctx, &rec, quiet, "receipt-2")
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	got = receipt{}
	if json.Unmarshal([]byte(out), &got) != nil || len(got.InboxRecords) != 0 {
		t.Fatalf("a turn that sent nothing names records: %s", out)
	}
	// A Write turn with a Cast names it too.
	v1 = headOf(t, s, rec)
	write := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: v1.RevisionID, Wait: "second look", InboxRecords: []types.CommitmentRecord{researchCast(rec, "texture-cell:cast:receipt-3")}, Write: &yaegikernel.TextureWrite{
		Edits: "@replace b2\nThe court ruled that the merger may proceed {{cite https://example.org/court \"the court ruled that the merger may proceed\"}}.", Why: "state the ruling"}})
	out, err = h.CommitCellTextureAuthor(ctx, &rec, write, "receipt-3")
	if err != nil {
		t.Fatalf("Write with a Cast: %v", err)
	}
	got = receipt{}
	if err := json.Unmarshal([]byte(out), &got); err != nil || len(got.InboxRecords) != 1 || got.InboxRecords[0].Inbox != "research:"+rec.ChannelID {
		t.Fatalf("a Write turn's receipt lacks its Cast's record: %s (%v)", out, err)
	}
}

// A replayed turn (the same body and identity) names no records: they were named
// when it first committed, and the runtime logs a research Cast once.
func TestReplayedTurnReceiptNamesNoRecords(t *testing.T) {
	type receipt struct {
		InboxRecords []types.InboxRecordReceipt `json:"inbox_records"`
		Replay       bool                       `json:"replay"`
	}
	h, s, rec := cellAuthorSetup(t, "turn-replay")
	ctx := context.Background()
	base := headOf(t, s, rec)
	body := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "asked research", InboxRecords: []types.CommitmentRecord{researchCast(rec, "texture-cell:cast:replay-1")}})
	first, err := h.CommitCellTextureAuthor(ctx, &rec, body, "replay-1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.CommitCellTextureAuthor(ctx, &rec, body, "replay-1")
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	var a, b receipt
	_ = json.Unmarshal([]byte(first), &a)
	_ = json.Unmarshal([]byte(second), &b)
	if len(a.InboxRecords) != 1 || !b.Replay || len(b.InboxRecords) != 0 {
		t.Fatalf("first = %s\nreplay = %s\nwant one record, then a replay naming none", first, second)
	}
}
