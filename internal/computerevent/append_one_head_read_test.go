package computerevent

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

// Station S5 part 2: a new event is bound to the platform head once, under
// the appender mutex, and that read serves the embedded-head comparison and
// the reducer too. The paper paid two head reads per append (6.03 corpusd
// calls per tape event: append, payload, 2 heads, 2 pins). The second pin is
// not redundant: the payload pin receipt is bound into the event's request
// commitment, and the event pin carries the event body, which holds that
// commitment, so the event pin cannot precede or merge with the payload pin.

func newHeadCountAppender(t *testing.T) (*ComputerEventAppender, *memoryCAS) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := SigningKey{SignerRef: SignerRef{SignerDomain: "platform-control", KeyID: "platform-1"}, PrivateKey: privateKey}
	cas := &memoryCAS{signer: signer}
	projection := &dryRunProjection{memoryProjection: &memoryProjection{}}
	appender, err := NewComputerEventAppender(testComputerID, memoryPinner{signer: signer}, projection, cas, EventHeadReceiptVerifier{Keys: staticKeyResolver{key: publicKey}})
	if err != nil {
		t.Fatal(err)
	}
	genesis := testEvent(t, nil, EventGenesisImported)
	genesis.ResultingEffectiveCommitment = testDigestA
	if _, err := appender.AppendNew(context.Background(), genesis, TransitionInput{TargetStateCommitment: testDigestA}, nil); err != nil {
		t.Fatal(err)
	}
	return appender, cas
}

func TestAppendNewPayloadReadsThePlatformHeadOnce(t *testing.T) {
	// One batch per appender: the batch projection double here does not move
	// its embedded head, so a second append would start from a gap.
	for i := 0; i < 3; i++ {
		appender, cas := newHeadCountAppender(t)
		cas.mu.Lock()
		before := cas.headCalls
		cas.mu.Unlock()
		event := testEvent(t, nil, EventProjectionBatchRecorded)
		if _, _, err := appender.AppendNewPayload(context.Background(), event, TransitionInput{}, testBatchPayloadFor(t, event.EventID), ProjectionBatchMediaType, "owner"); err != nil {
			t.Fatal(err)
		}
		cas.mu.Lock()
		got := cas.headCalls - before
		cas.mu.Unlock()
		if got != 1 {
			t.Fatalf("append %d read the platform head %d times, want 1", i, got)
		}
	}
}

// Append (an already bound event) has no earlier read: it still reads once.
func TestAppendReadsThePlatformHeadOnceWhenUnbound(t *testing.T) {
	appender, cas := newHeadCountAppender(t)
	cas.mu.Lock()
	before := cas.headCalls
	cas.mu.Unlock()
	event := testEvent(t, nil, EventProjectionBatchRecorded)
	_, _ = appender.Append(context.Background(), event, TransitionInput{}, nil) // refused on its binding; the read is what is counted
	cas.mu.Lock()
	got := cas.headCalls - before
	cas.mu.Unlock()
	if got > 1 {
		t.Fatalf("Append read the platform head %d times, want at most 1", got)
	}
}

// The single read must not weaken the guard it replaced: if the platform head
// moves between the bind and the CAS (a host-side event during the payload
// pin), the append fails, nothing of it is canonical, and the next append
// succeeds on the new head.
func TestAppendNewPayloadHeadMovedDuringPinIsRefused(t *testing.T) {
	appender, cas := newHeadCountAppender(t)
	moved := false
	appender.pins = movingPinner{memoryPinner: appender.pins.(memoryPinner), onPin: func() {
		if moved {
			return
		}
		moved = true
		cas.mu.Lock()
		h := cloneHead(cas.head)
		h.CanonicalEventHead = testDigestB
		h.Sequence++
		cas.head = h
		cas.mu.Unlock()
	}}
	event := testEvent(t, nil, EventProjectionBatchRecorded)
	if _, _, err := appender.AppendNewPayload(context.Background(), event, TransitionInput{}, testBatchPayloadFor(t, event.EventID), ProjectionBatchMediaType, "owner"); err == nil {
		t.Fatal("an append bound to a head that moved before its CAS succeeded")
	}
	cas.mu.Lock()
	for _, rec := range cas.records {
		if rec.Request.Event.EventID == event.EventID {
			t.Fatal("the stale-bound event is on the platform tape")
		}
	}
	cas.mu.Unlock()
}

type movingPinner struct {
	memoryPinner
	onPin func()
}

func (p movingPinner) PinNonPrivatePayload(ctx context.Context, computerID string, payload []byte, mediaType, privacyClass, intent string) (PinResult, error) {
	p.onPin()
	return p.memoryPinner.PinNonPrivatePayload(ctx, computerID, payload, mediaType, privacyClass, intent)
}
