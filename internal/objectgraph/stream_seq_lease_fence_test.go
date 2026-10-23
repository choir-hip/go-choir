package objectgraph

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// The stream_seq lease kind and its hard writer fence
// (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md, section 6;
// tests 27 to 30). Ways it can fail, written before the code:
//  1. The fence sits in the mode check, so the default observe mode (and off)
//     lets a desk-declared writer forge a lease: a forged R is the one input
//     that can burn the number space or, if low, reissue numbers.
//  2. The fence refuses the store itself: the capability does not admit the
//     lease write under enforce.
//  3. The kind is registered under system:runtime, which admits every
//     declared writer (writerAdmits), so the registry alone would not refuse.
//  4. system:store is not a built-in writer, so the kind fails the coverage
//     test that every registered kind names one known writer.
//  5. A tombstone of the kind is refused, so a retired lease cannot be removed
//     the way other historic rows can.
//  6. The capability leaks into a derived context that outlives the store
//     method, or is a plain string key a caller can forge.

func leaseObject(t *testing.T, key string) Object {
	t.Helper()
	id, err := BuildCanonicalID(KindStreamSeqLease, "__system__", StableSuffixFromKey(key))
	if err != nil {
		t.Fatal(err)
	}
	meta := json.RawMessage(`{"reserved_through":100,"allocated_through":50,"block":65536,"writer_epoch":3,"basis":"exact","head_sequence":1}`)
	at := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	return Object{CanonicalID: id, ObjectKind: KindStreamSeqLease, OwnerID: "__system__", ContentHash: ContentHash(KindStreamSeqLease, meta, meta), Body: meta, Metadata: meta, CreatedAt: at, UpdatedAt: at}
}

func TestStreamSeqLeaseKindIsRegisteredUnderTheStoreWriter(t *testing.T) {
	reg, err := DefaultRegistry().LookupKind(KindStreamSeqLease)
	if err != nil {
		t.Fatalf("lease kind not registered: %v", err)
	}
	if KindStreamSeqLease != "choir.stream_seq_lease" {
		t.Fatalf("kind = %q", KindStreamSeqLease)
	}
	if reg.Writer != WriterStore || WriterStore != "system:store" {
		t.Fatalf("writer = %q, want system:store (not system:runtime, which admits every writer)", reg.Writer)
	}
	if reg.IdentityMode != IdentityExternalKey {
		t.Fatalf("identity mode = %q, want external key", reg.IdentityMode)
	}
	if reg.Store != StoreTypeDolt || reg.Owner != OwnerVMStore {
		t.Fatalf("store/owner = %q/%q", reg.Store, reg.Owner)
	}
	if !BuiltinWriters[WriterStore] || !ValidKindWriter(WriterStore) {
		t.Fatal("system:store is not a built-in writer: TestEveryRegisteredKindNamesExactlyOneKnownWriter would fail")
	}
}

// Test 27: refused under all three gate modes, including a desk-declared
// writer and the kind's own writer without the capability.
func TestLeaseWriteWithoutTheCapabilityIsRefusedInEveryMode(t *testing.T) {
	obj := leaseObject(t, "stream_seq_lease")
	writers := map[string]context.Context{
		"undeclared":    context.Background(),
		"desk:texture":  WithWriter(context.Background(), WriterTexture),
		"desk:research": WithWriter(context.Background(), WriterResearch),
		"system:store":  WithWriter(context.Background(), WriterStore),
		"owner":         WithWriter(context.Background(), WriterOwner),
		"runtime":       WithWriter(context.Background(), WriterRuntime),
	}
	for _, mode := range []WriterGateMode{WriterGateOff, WriterGateObserve, WriterGateEnforce} {
		for name, ctx := range writers {
			func() {
				writerMode(t, mode)
				err := CheckWriteAs(ctx, []Object{obj}, nil)
				if !errors.Is(err, ErrLeaseFence) {
					t.Errorf("mode %s, writer %s: err = %v, want ErrLeaseFence", mode, name, err)
				}
			}()
		}
	}
}

// Test 27, in a mixed batch: the lease object inside a larger batch is found.
func TestLeaseWriteInsideABatchIsRefused(t *testing.T) {
	writerMode(t, WriterGateObserve)
	other := gateObject(t, "choir.citation_check", "a")
	err := CheckWriteAs(context.Background(), []Object{other, leaseObject(t, "stream_seq_lease")}, nil)
	if !errors.Is(err, ErrLeaseFence) {
		t.Fatalf("batch with a lease: err = %v, want ErrLeaseFence", err)
	}
	if err := CheckWriteAs(context.Background(), []Object{other}, nil); err != nil {
		t.Fatalf("a batch without a lease is refused: %v", err)
	}
}

// Test 28: with the capability, a write passes under enforce (and the others).
func TestLeaseWriteWithTheCapabilityPasses(t *testing.T) {
	obj := leaseObject(t, "stream_seq_lease")
	for _, mode := range []WriterGateMode{WriterGateOff, WriterGateObserve, WriterGateEnforce} {
		func() {
			writerMode(t, mode)
			ctx := WithWriter(WithStreamSeqLeaseCapability(context.Background()), WriterStore)
			if err := CheckWriteAs(ctx, []Object{obj}, nil); err != nil {
				t.Errorf("mode %s: the store's own lease write refused: %v", mode, err)
			}
		}()
	}
}

// Under enforce the capability alone is not enough: the declared writer must
// still be the kind's writer (the existing gate runs after the fence).
func TestLeaseCapabilityWithAWrongWriterIsRefusedUnderEnforce(t *testing.T) {
	writerMode(t, WriterGateEnforce)
	ctx := WithWriter(WithStreamSeqLeaseCapability(context.Background()), WriterTexture)
	err := CheckWriteAs(ctx, []Object{leaseObject(t, "stream_seq_lease")}, nil)
	if !errors.Is(err, ErrWrongWriter) {
		t.Fatalf("capability with desk:texture under enforce: err = %v, want ErrWrongWriter", err)
	}
}

// A tombstone passes, as in CheckWrite: a retired lease can be removed.
func TestLeaseTombstonePassesTheFence(t *testing.T) {
	writerMode(t, WriterGateEnforce)
	obj := leaseObject(t, "stream_seq_lease")
	obj.Tombstone = true
	if err := CheckWriteAs(context.Background(), []Object{obj}, nil); err != nil {
		t.Fatalf("tombstone of the lease refused: %v", err)
	}
}

// The capability is a context value under an unexported key: a string key
// carrying the same words does not forge it.
func TestLeaseCapabilityCannotBeForgedWithAStringKey(t *testing.T) {
	writerMode(t, WriterGateEnforce)
	type lookalike string
	ctx := context.WithValue(context.Background(), lookalike("streamSeqLeaseCapability"), true)
	ctx = context.WithValue(ctx, "streamSeqLeaseCapability", true) //nolint:staticcheck // the point of the test
	ctx = WithWriter(ctx, WriterStore)
	if err := CheckWriteAs(ctx, []Object{leaseObject(t, "stream_seq_lease")}, nil); !errors.Is(err, ErrLeaseFence) {
		t.Fatalf("a forged capability passed: err = %v", err)
	}
}

// The Dolt store runs the fence before its interceptor and its validator, so
// a refused lease write reaches no tape.
func TestDoltStoreRefusesAForgedLeaseBeforeTheInterceptor(t *testing.T) {
	for _, mode := range []WriterGateMode{WriterGateOff, WriterGateObserve, WriterGateEnforce} {
		func() {
			writerMode(t, mode)
			ctx := context.Background()
			store := NewDoltStore(openTestDoltDB(t))
			if err := store.EnsureSchema(ctx); err != nil {
				t.Fatal(err)
			}
			intercepted := 0
			store.SetMutationInterceptor(func(context.Context, []Object, []Edge) error {
				intercepted++
				return nil
			})
			obj := leaseObject(t, "stream_seq_lease")
			desk := WithWriter(ctx, WriterTexture)
			for name, write := range map[string]func() error{
				"PutObject":      func() error { return store.PutObject(desk, obj) },
				"PutBatch":       func() error { return store.PutBatch(desk, Batch{Objects: []Object{obj}}) },
				"PreflightWrite": func() error { return store.PreflightWrite(desk, []Object{obj}, nil) },
			} {
				if err := write(); !errors.Is(err, ErrLeaseFence) {
					t.Errorf("mode %s %s: err = %v, want ErrLeaseFence", mode, name, err)
				}
			}
			if intercepted != 0 {
				t.Errorf("mode %s: the interceptor saw %d refused lease writes", mode, intercepted)
			}
			// With the capability the same store writes it.
			store.SetMutationInterceptor(nil)
			ok := WithWriter(WithStreamSeqLeaseCapability(ctx), WriterStore)
			if err := store.PutObject(ok, obj); err != nil {
				t.Errorf("mode %s: the capability write refused: %v", mode, err)
			}
		}()
	}
}

// Replay does not call CheckWrite, so the fence cannot poison it (the
// projector is not exercised here; the contract is that CheckWrite, which
// replay's neighbours use, does not know the fence).
func TestCheckWriteDoesNotApplyTheLeaseFence(t *testing.T) {
	if err := CheckWrite([]Object{leaseObject(t, "stream_seq_lease")}, nil); err != nil {
		t.Fatalf("CheckWrite refused the lease kind: %v", err)
	}
}
