package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// U4 slice 1 (docs/design/u4-stack-rebase-build-2026-10-26.md): the rebase
// notice is a system record that wakes the root Texture RLM, committed with
// command id equal to record id; the claim object is keyed by pair. Ways it
// can fail, written first:
//   - another system record starts waking its reader (the wake predicate is
//     a closed allowlist);
//   - a retry under a different command id mints a second record of one id;
//   - a replay of the same command is refused instead of returning the
//     committed receipt;
//   - a typo in the new switch stores a setting nobody reads;
//   - a claim is written twice for one pair, or one pair's claim is
//     read as another's.

const rebaseTestBase = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func rebaseNoticeRecord(t *testing.T, attempt int) types.CommitmentRecord {
	t.Helper()
	rec, ok := types.NewStackRebaseNotice(types.StackRebaseNotice{
		TextureInbox: "texture:doc-master", ComputerID: inboxTestComputer, OperationID: "op-a",
		BootedBase: rebaseTestBase, StackBase: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Attempt: attempt, DocID: "doc-master",
	})
	if !ok {
		t.Fatal("notice did not build")
	}
	return rec
}

func TestRebaseNoticeWakesItsReaderAndNoOtherClosureDoes(t *testing.T) {
	notice := rebaseNoticeRecord(t, 1)
	if !RecordWakesReader(notice) || !inboxRecordWakes(notice) {
		t.Fatal("a rebase notice must wake the root")
	}
	closure := notice
	closure.SenderInbox, closure.RecordID = "selfdev-rebase:other", "closure:x"
	closure.SubjectRefs = nil
	if RecordWakesReader(closure) {
		t.Fatal("a system Note that is not a rebase notice must not wake")
	}
	forged := notice
	forged.EmitterKind = types.CommitmentEmitterAgent
	forged.SenderInbox = "texture:x"
	if types.IsStackRebaseNotice(forged) {
		t.Fatal("an agent record passes for a notice")
	}
}

func TestRebaseNoticeCommitsOnceByCommandAndRecordId(t *testing.T) {
	s := openTestStore(t)
	notice := rebaseNoticeRecord(t, 1)

	first := mustCommitInbox(t, s, notice.RecordID, nil, notice)
	if len(first.Records) != 1 {
		t.Fatalf("first commit receipt %+v", first)
	}
	// The same command replays: no second record, no error.
	if _, err := commitInbox(t, s, notice.RecordID, nil, notice); err != nil {
		t.Fatalf("the same command must replay: %v", err)
	}
	// A retry under any other command id is refused by the record id.
	if _, err := commitInbox(t, s, "rebase-retry:other-command", nil, notice); !errors.Is(err, ErrInboxRefused) {
		t.Fatalf("a new command id with the same record id must be refused, got %v", err)
	}
	got, err := s.ListInboxRecords(context.Background(), inboxTestOwner, inboxTestComputer, "texture:doc-master", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !types.IsStackRebaseNotice(got[0]) {
		t.Fatalf("inbox holds %d records, want exactly the one notice", len(got))
	}
	// A later attempt is a new record with its own command.
	mustCommitInbox(t, s, rebaseNoticeRecord(t, 2).RecordID, nil, rebaseNoticeRecord(t, 2))
}

func TestSourceRebaseOfferSettingIsKnownAndDefaultsOff(t *testing.T) {
	if on, known := KnownComputerSettings[SettingSourceRebaseOffer]; !known || on {
		t.Fatalf("source.rebase_offer known=%v default=%v; want known and false", known, on)
	}
	if SettingSourceRebaseOffer != "source.rebase_offer" {
		t.Fatalf("setting key %q", SettingSourceRebaseOffer)
	}
	s := openTestStore(t)
	if err := s.PutComputerSetting(context.Background(), ComputerSetting{OwnerID: inboxTestOwner, ComputerID: inboxTestComputer, Key: "source.rebase_offr", Value: true}); err == nil {
		t.Fatal("a typo'd key must still be refused")
	}
	if err := s.PutComputerSetting(context.Background(), ComputerSetting{OwnerID: inboxTestOwner, ComputerID: inboxTestComputer, Key: SettingSourceRebaseOffer, Value: true}); err != nil {
		t.Fatal(err)
	}
}

func TestRebaseNoticeClaimIsOnePerPair(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	pair := types.StackRebasePairKey("op-a", rebaseTestBase)
	if _, err := s.GetRebaseNoticeClaim(ctx, inboxTestOwner, inboxTestComputer, pair); !errors.Is(err, ErrNotFound) {
		t.Fatalf("no claim yet: %v", err)
	}
	claim := types.StackRebaseClaim{
		PairKey: pair, OperationID: "op-a", BootedBase: rebaseTestBase, NoticeID: types.StackRebaseNoticeID("op-a", rebaseTestBase, 1),
		Attempt: 1, AssignmentID: "assignment-1", ClaimedAt: time.Now().UTC(),
	}
	if err := s.PutRebaseNoticeClaim(ctx, inboxTestOwner, inboxTestComputer, claim); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetRebaseNoticeClaim(ctx, inboxTestOwner, inboxTestComputer, pair)
	if err != nil || got.AssignmentID != "assignment-1" || got.NoticeID != claim.NoticeID || got.Attempt != 1 {
		t.Fatalf("claim = %+v, %v", got, err)
	}
	// A second claim for the pair loses; the first stands.
	second := claim
	second.AssignmentID, second.Attempt, second.NoticeID = "assignment-2", 2, types.StackRebaseNoticeID("op-a", rebaseTestBase, 2)
	if err := s.PutRebaseNoticeClaim(ctx, inboxTestOwner, inboxTestComputer, second); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("a second claim must lose with ErrConcurrentStateChange, got %v", err)
	}
	if again, _ := s.GetRebaseNoticeClaim(ctx, inboxTestOwner, inboxTestComputer, pair); again.AssignmentID != "assignment-1" {
		t.Fatalf("the first claim must stand: %+v", again)
	}
	// Another pair is untouched.
	if _, err := s.GetRebaseNoticeClaim(ctx, inboxTestOwner, inboxTestComputer, types.StackRebasePairKey("op-a", "cccc")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another pair reads as claimed: %v", err)
	}
	// A claim that names no pair, assignment or notice is refused.
	if err := s.PutRebaseNoticeClaim(ctx, inboxTestOwner, inboxTestComputer, types.StackRebaseClaim{PairKey: "x"}); err == nil {
		t.Fatal("an empty claim must be refused")
	}
}
