package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The run-end check reads open rounds from the conversation heads
// (OpenRoundsAddressedTo) instead of folding every record of the inbox
// (OpenRounds). Ways it can fail, written before the code:
//
//  1. A round the fold reports open, addressed to the inbox, is missing
//     from the head read (a conversation whose head is not found, an
//     inbox on the other side of the conversation id, a tombstone).
//  2. A round is reported that the fold says closed, or addressed to the
//     other party, or of another owner or computer.
//  3. The two agree on the set but not the content (round, caster,
//     hypotheses) or the order.
//
// The test drives random casts, reports, finals, notes and system closures
// through the real commit path, including refused ones, and compares both
// reads for every inbox after every step.
func TestOpenRoundsAddressedToMatchesRecordFold(t *testing.T) {
	ctx := context.Background()
	for _, seed := range []int64{1, 1014} {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			s := openTestStore(t)
			rng := rand.New(rand.NewSource(seed))
			management := "management:" + inboxTestOwner
			inboxes := []string{management, "texture:d1", "texture:d2", "research:d1", "engineering:a1", "source:s1", "research:d|2"}
			committed, refused := 0, 0
			for step := 0; step < 100; step++ {
				from, to := inboxes[rng.Intn(len(inboxes))], inboxes[rng.Intn(len(inboxes))]
				if from == to {
					continue
				}
				id := fmt.Sprintf("s%d-r%03d", seed, step)
				var rec types.CommitmentRecord
				switch rng.Intn(7) {
				case 0, 1:
					rec = inboxCast(id, from, to, types.CommitmentCast{Hypotheses: []types.CastHypothesis{hyp("h "+id, 0.4)}})
				case 2:
					rec = inboxReport(id, from, to, true, types.CommitmentReport{StopReason: "done",
						Settled: []types.HypothesisSettlement{{ID: fmt.Sprintf("H%d", 1+rng.Intn(3)), Outcome: "held"}}})
				case 3:
					rec = inboxReport(id, from, to, false, types.CommitmentReport{Claims: []types.ReportClaim{{Text: "claim " + id}}})
				case 4:
					rec = inboxNote(id, from, to)
				case 5:
					rec = inboxReport(id, from, to, true, types.CommitmentReport{StopReason: "done"})
				default:
					// The system closure the run-end check itself writes.
					head, err := s.ConversationState(ctx, inboxTestOwner, inboxTestComputer, types.InboxConversationID(from, to))
					if err != nil {
						t.Fatal(err)
					}
					if !head.Open {
						continue
					}
					rec = types.NewRoundClosureRecord(head.CasterInbox, head.RecipientInbox, head.Round, "run_completed", "")
				}
				if _, err := commitInbox(t, s, "cmd-"+id, nil, rec); err != nil {
					if !errors.Is(err, ErrInboxRefused) {
						t.Fatalf("commit %s: %v", id, err)
					}
					refused++
					continue
				}
				committed++
				// A commit changes only its own conversation, so the two
				// parties are where the reads can diverge; every inbox is
				// compared at the end.
				assertOpenRoundsAgree(t, s, rec.Inbox)
				assertOpenRoundsAgree(t, s, rec.SenderInbox)
			}
			for _, inbox := range inboxes {
				assertOpenRoundsAgree(t, s, inbox)
			}
			assertInboxHeadsMatchRebuild(t, s)
			t.Logf("seed %d: %d committed, %d refused", seed, committed, refused)
			if committed < 30 || refused == 0 {
				t.Fatalf("fixture too thin: %d committed, %d refused", committed, refused)
			}
		})
	}
}

func assertOpenRoundsAgree(t *testing.T, s *Store, inbox string) {
	t.Helper()
	ctx := context.Background()
	folded, err := s.OpenRounds(ctx, inboxTestOwner, inboxTestComputer, inbox)
	if err != nil {
		t.Fatal(err)
	}
	var want []types.ConversationHead
	for _, round := range folded {
		if round.RecipientInbox == inbox {
			want = append(want, round)
		}
	}
	got, err := s.OpenRoundsAddressedTo(ctx, inboxTestOwner, inboxTestComputer, inbox)
	if err != nil {
		t.Fatal(err)
	}
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("open rounds addressed to %s:\n heads %s\n  fold %s", inbox, gotJSON, wantJSON)
	}
	// Another computer's heads never leak in.
	other, err := s.OpenRoundsAddressedTo(ctx, inboxTestOwner, "computer-other", inbox)
	if err != nil || len(other) != 0 {
		t.Fatalf("other computer's open rounds for %s = %v, %v", inbox, other, err)
	}
}
