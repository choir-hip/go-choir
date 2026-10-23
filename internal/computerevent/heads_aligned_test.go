package computerevent

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
)

// The stream_seq lease's seeder precondition and the head-rewind fence
// (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md, sections 2
// and 3; tests 25 and 33). Ways it can fail, written before the code:
//  1. The seeder scans while replay can still add events: the local head is
//     not the platform head, and an event arrives mid-scan.
//  2. HeadsAligned says "aligned" for a local head ahead of the platform
//     (a rewind), so a lease is written onto a state the tape does not hold.
//  3. A boot whose local head is ahead of the platform head resumes: the
//     reconstruct loop applies no tail and nothing refuses it.

func alignedFixture(t *testing.T) (*ComputerEventAppender, *memoryProjection, *memoryCAS) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := SigningKey{SignerRef: SignerRef{SignerDomain: "platform-control", KeyID: "platform-1"}, PrivateKey: privateKey}
	projection := &memoryProjection{}
	cas := &memoryCAS{signer: signer}
	appender, err := NewComputerEventAppender(testComputerID, memoryPinner{signer: signer}, projection, cas, EventHeadReceiptVerifier{Keys: staticKeyResolver{key: publicKey}})
	if err != nil {
		t.Fatal(err)
	}
	genesis := testEvent(t, nil, EventGenesisImported)
	genesis.ResultingEffectiveCommitment = testDigestA
	if _, err := appender.AppendNew(context.Background(), genesis, TransitionInput{TargetStateCommitment: testDigestA}, nil); err != nil {
		t.Fatal(err)
	}
	return appender, projection, cas
}

func TestHeadsAlignedAgreesAfterAnAppend(t *testing.T) {
	appender, _, _ := alignedFixture(t)
	if err := appender.HeadsAligned(context.Background()); err != nil {
		t.Fatalf("heads agree after an append and HeadsAligned refused: %v", err)
	}
}

func TestHeadsAlignedAgreesOnAnEmptyChain(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := SigningKey{SignerRef: SignerRef{SignerDomain: "platform-control", KeyID: "platform-1"}, PrivateKey: privateKey}
	appender, err := NewComputerEventAppender(testComputerID, memoryPinner{signer: signer}, &memoryProjection{}, &memoryCAS{signer: signer}, EventHeadReceiptVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	if err := appender.HeadsAligned(context.Background()); err != nil {
		t.Fatalf("two empty heads are aligned: %v", err)
	}
}

func TestHeadsAlignedRefusesALocalHeadBehind(t *testing.T) {
	appender, projection, _ := alignedFixture(t)
	projection.head = nil
	if err := appender.HeadsAligned(context.Background()); !errors.Is(err, ErrNeedsProjectionRepair) {
		t.Fatalf("local head behind: err = %v, want ErrNeedsProjectionRepair", err)
	}
}

func TestHeadsAlignedRefusesALocalHeadAhead(t *testing.T) {
	appender, projection, _ := alignedFixture(t)
	ahead := cloneHead(projection.head)
	ahead.Sequence++
	projection.head = ahead
	if err := appender.HeadsAligned(context.Background()); !errors.Is(err, ErrNeedsProjectionRepair) {
		t.Fatalf("local head ahead: err = %v, want ErrNeedsProjectionRepair", err)
	}
}

// Test 33. A boot whose local head is ahead of the platform head refuses: the
// reconstruct loop applies no tail and fails its final sameHead check, and an
// append is refused while the heads differ. Pinned so the rewind fence cannot
// quietly go away: the lease design relies on it instead of a carry-forward.
func TestBootWithALocalHeadAheadOfThePlatformRefuses(t *testing.T) {
	appender, projection, cas := alignedFixture(t)
	ahead := cloneHead(projection.head)
	ahead.Sequence++
	projection.head = ahead
	if err := appender.Reconstruct(context.Background(), cas); !errors.Is(err, ErrNeedsProjectionRepair) {
		t.Fatalf("reconstruct with the local head ahead: err = %v, want ErrNeedsProjectionRepair", err)
	}
	next := testEvent(t, cas.head, EventResearchUpdate)
	next.ResultingEffectiveCommitment = testDigestB
	input := TransitionInput{TargetStateCommitment: testDigestB}
	bindTestRequest(t, &next, input, nil)
	if _, err := appender.Append(context.Background(), next, input, nil); !errors.Is(err, ErrNeedsProjectionRepair) {
		t.Fatalf("append with the heads apart: err = %v, want ErrNeedsProjectionRepair", err)
	}
}
