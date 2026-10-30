package objectgraph

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// One writer per object kind (SDC ARCH-04,
// docs/problems/object-kind-writer-is-a-package-not-a-desk-2026-10-21.md).
// The registry names the one writer of each kind; the write gate, given a
// declared writer, refuses a kind another writer owns. The failure modes of
// the fix are listed in the problem record; each test below names the one it
// covers.

func writerMode(t *testing.T, mode WriterGateMode) {
	t.Helper()
	t.Cleanup(SetWriterGateMode(mode))
}

// Every registered kind names exactly one writer, and a writer is a known one.
func TestEveryRegisteredKindNamesExactlyOneKnownWriter(t *testing.T) {
	for _, reg := range DefaultRegistry().AllKinds() {
		if reg.Reserved {
			if reg.Writer != WriterAutoradio {
				t.Errorf("reserved kind %s: writer %q, want %q", reg.Kind, reg.Writer, WriterAutoradio)
			}
			continue
		}
		if !BuiltinWriters[reg.Writer] {
			t.Errorf("kind %s: writer %q is not a known built-in writer", reg.Kind, reg.Writer)
		}
	}
}

// The writer agrees with the package the kind's owner names.
func TestWriterAgreesWithOwnerPackage(t *testing.T) {
	for _, reg := range DefaultRegistry().AllKinds() {
		switch reg.Owner {
		case OwnerCorpusd:
			if reg.Writer != WriterCorpusd {
				t.Errorf("corpusd kind %s has writer %q, want %q", reg.Kind, reg.Writer, WriterCorpusd)
			}
		case OwnerVMStore:
			if reg.Writer == WriterCorpusd {
				t.Errorf("VM store kind %s has the corpusd writer", reg.Kind)
			}
		}
	}
}

// Texture is the sole agent writer of its kinds; the owner's own edit path is
// the second writer class of documents and revisions only (doctrine: canonical
// document versions have two writer classes, AuthorUser and AuthorAppAgent).
func TestTextureKindsHaveTextureAsTheOnlyDeskWriter(t *testing.T) {
	registry := DefaultRegistry()
	ownerEdits := map[ObjectKind]bool{"choir.texture_document": true, "choir.texture_revision": true}
	for _, kind := range []ObjectKind{
		"choir.texture_document", "choir.texture_revision", "choir.texture_decision",
		"choir.source_entity", "choir.source_ref", "choir.citation_check",
	} {
		reg, err := registry.LookupKind(kind)
		if err != nil {
			t.Fatal(err)
		}
		if reg.Writer != WriterTexture {
			t.Errorf("%s: writer %q, want %q", kind, reg.Writer, WriterTexture)
		}
		if reg.OwnerEdits != ownerEdits[kind] {
			t.Errorf("%s: OwnerEdits = %v, want %v", kind, reg.OwnerEdits, ownerEdits[kind])
		}
	}
}

// F7 (mixed batches): runtime kinds pass under any declared desk; a desk-owned
// kind passes only under its desk, or the owner where OwnerEdits; a corpusd
// kind only under corpusd.
func TestCheckWriteAsAdmitsOnlyTheKindsWriter(t *testing.T) {
	writerMode(t, WriterGateEnforce)
	cases := []struct {
		name     string
		declared Writer
		kind     ObjectKind
		wantErr  error
	}{
		{"texture writes its revision", WriterTexture, "choir.texture_revision", nil},
		{"research cannot write a texture revision", WriterResearch, "choir.texture_revision", ErrWrongWriter},
		{"management cannot write a texture document", WriterManagement, "choir.texture_document", ErrWrongWriter},
		{"the owner edits a document (second writer class)", WriterOwner, "choir.texture_revision", nil},
		{"the owner cannot write a texture decision", WriterOwner, "choir.texture_decision", ErrWrongWriter},
		{"texture writes runtime bookkeeping in its batch", WriterTexture, "choir.inbox_head", nil},
		{"research writes runtime bookkeeping in its batch", WriterResearch, "choir.worker_update", nil},
		{"texture cannot write a corpusd kind", WriterTexture, "choir.publication", ErrWrongWriter},
		{"corpusd writes its kind", WriterCorpusd, "choir.publication", nil},
		{"corpusd cannot write a texture kind", WriterCorpusd, "choir.texture_revision", ErrWrongWriter},
	}
	for _, tc := range cases {
		ctx := WithWriter(context.Background(), tc.declared)
		err := CheckWriteAs(ctx, []Object{gateObject(t, tc.kind, tc.name)}, nil)
		if !errors.Is(err, tc.wantErr) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.wantErr)
		}
		if tc.wantErr != nil && err != nil && !strings.Contains(err.Error(), string(tc.kind)) {
			t.Errorf("%s: the refusal does not name the kind: %v", tc.name, err)
		}
	}
}

// F2: a write that declares no writer. In enforce it is refused when the kind
// is desk or corpusd owned; the runtime's own kinds are written by the runtime
// and need no declaration.
func TestCheckWriteAsEnforceRefusesAnUndeclaredWriteOfADeskKind(t *testing.T) {
	writerMode(t, WriterGateEnforce)
	ctx := context.Background()
	if err := CheckWriteAs(ctx, []Object{gateObject(t, "choir.texture_revision", "u1")}, nil); !errors.Is(err, ErrUndeclaredWriter) {
		t.Fatalf("undeclared write of a desk kind: err = %v, want ErrUndeclaredWriter", err)
	}
	if err := CheckWriteAs(ctx, []Object{gateObject(t, "choir.publication", "u2")}, nil); !errors.Is(err, ErrUndeclaredWriter) {
		t.Fatalf("undeclared write of a corpusd kind: err = %v, want ErrUndeclaredWriter", err)
	}
	if err := CheckWriteAs(ctx, []Object{gateObject(t, "choir.inbox_head", "u3")}, nil); err != nil {
		t.Fatalf("undeclared runtime write refused: %v", err)
	}
}

// F1: the default mode observes. A mismatch and an undeclared write are
// counted, logged once per cause, and allowed, so a wrong table cannot stop a
// production write; enforce is a deliberate flip after the counts are clean.
func TestCheckWriteAsObserveCountsAndAllows(t *testing.T) {
	writerMode(t, WriterGateObserve)
	ResetWriterGateCounts()
	ctx := WithWriter(context.Background(), WriterResearch)
	for i := 0; i < 3; i++ {
		if err := CheckWriteAs(ctx, []Object{gateObject(t, "choir.texture_revision", "o"+string(rune('a'+i)))}, nil); err != nil {
			t.Fatalf("observe refused a write: %v", err)
		}
	}
	if err := CheckWriteAs(context.Background(), []Object{gateObject(t, "choir.texture_decision", "ou")}, nil); err != nil {
		t.Fatalf("observe refused an undeclared write: %v", err)
	}
	counts := WriterGateCounts()
	if got := counts["choir.texture_revision|desk:research|wrong_writer"]; got != 3 {
		t.Fatalf("wrong-writer count = %d, want 3 (counts: %v)", got, counts)
	}
	if got := counts["choir.texture_decision|undeclared|undeclared_writer"]; got != 1 {
		t.Fatalf("undeclared count = %d, want 1 (counts: %v)", got, counts)
	}
	// A correct write adds nothing.
	before := len(WriterGateCounts())
	if err := CheckWriteAs(WithWriter(context.Background(), WriterTexture), []Object{gateObject(t, "choir.texture_revision", "ok")}, nil); err != nil {
		t.Fatal(err)
	}
	if len(WriterGateCounts()) != before {
		t.Fatalf("a correct write was counted as a violation: %v", WriterGateCounts())
	}
}

func TestCheckWriteAsOffIgnoresWriters(t *testing.T) {
	writerMode(t, WriterGateOff)
	ResetWriterGateCounts()
	ctx := WithWriter(context.Background(), WriterResearch)
	if err := CheckWriteAs(ctx, []Object{gateObject(t, "choir.texture_revision", "off")}, nil); err != nil {
		t.Fatalf("off refused a write: %v", err)
	}
	if len(WriterGateCounts()) != 0 {
		t.Fatalf("off counted a violation: %v", WriterGateCounts())
	}
}

func TestWriterGateDefaultsToObserve(t *testing.T) {
	t.Setenv("CHOIR_OG_WRITER_GATE", "")
	if got := WriterGateModeFromEnv(); got != WriterGateObserve {
		t.Fatalf("default mode = %v, want observe", got)
	}
	t.Setenv("CHOIR_OG_WRITER_GATE", "enforce")
	if got := WriterGateModeFromEnv(); got != WriterGateEnforce {
		t.Fatalf("enforce env = %v", got)
	}
	t.Setenv("CHOIR_OG_WRITER_GATE", "nonsense")
	if got := WriterGateModeFromEnv(); got != WriterGateObserve {
		t.Fatalf("an unknown value must not turn the gate off: %v", got)
	}
}

// F6: tombstones pass, as in CheckWrite; the unregistered-kind refusal still
// comes first.
func TestCheckWriteAsKeepsTombstonesAndTheRegistrationRefusal(t *testing.T) {
	writerMode(t, WriterGateEnforce)
	ctx := WithWriter(context.Background(), WriterResearch)
	retired := gateObject(t, "choir.texture_revision", "t")
	retired.Tombstone = true
	if err := CheckWriteAs(ctx, []Object{retired}, nil); err != nil {
		t.Fatalf("tombstone refused by the writer gate: %v", err)
	}
	if err := CheckWriteAs(ctx, []Object{gateObject(t, "choir.not_a_kind", "n")}, nil); !errors.Is(err, ErrUnregisteredKind) {
		t.Fatalf("unregistered kind: err = %v, want ErrUnregisteredKind", err)
	}
}

// F4: the package install seam. A kind needs exactly one valid writer, and a
// kind already registered cannot be taken by another writer.
func TestRegisterKindCheckedRefusesASecondWriter(t *testing.T) {
	r := DefaultRegistry()
	if err := r.RegisterKindChecked(KindRegistration{Kind: "choir.podcast_episode_note", Store: StoreTypeDolt, Writer: "desk:podcast"}); err != nil {
		t.Fatalf("an app desk's new kind: %v", err)
	}
	if err := r.RegisterKindChecked(KindRegistration{Kind: "choir.podcast_episode_note", Store: StoreTypeDolt, Writer: "desk:podcast"}); err != nil {
		t.Fatalf("an identical re-registration must be idempotent: %v", err)
	}
	if err := r.RegisterKindChecked(KindRegistration{Kind: "choir.podcast_episode_note", Store: StoreTypeDolt, Writer: "desk:reader"}); !errors.Is(err, ErrWriterConflict) {
		t.Fatalf("a second writer for a registered kind: err = %v, want ErrWriterConflict", err)
	}
	if err := r.RegisterKindChecked(KindRegistration{Kind: "choir.texture_revision", Store: StoreTypeDolt, Writer: "desk:podcast"}); !errors.Is(err, ErrWriterConflict) {
		t.Fatalf("taking Texture's kind: err = %v, want ErrWriterConflict", err)
	}
	for _, bad := range []Writer{"", "podcast", "desk:", "desk:Has Space", "system:other", "owner"} {
		if err := r.RegisterKindChecked(KindRegistration{Kind: "choir.x_kind", Store: StoreTypeDolt, Writer: bad}); !errors.Is(err, ErrInvalidWriter) {
			t.Errorf("writer %q: err = %v, want ErrInvalidWriter", bad, err)
		}
	}
	// The default registry is untouched by a package's registration on a copy.
	if _, err := DefaultRegistry().LookupKind("choir.podcast_episode_note"); err == nil {
		t.Fatal("a registration leaked into a fresh DefaultRegistry")
	}
}

// The SQL store gates a declared writer before the event tape: a refused
// write reaches no interceptor.
func TestSQLStoreEnforcesTheDeclaredWriterBeforeTheInterceptor(t *testing.T) {
	writerMode(t, WriterGateEnforce)
	ctx := context.Background()
	store := NewSQLStore(openTestDB(t))
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	intercepted := 0
	store.SetMutationInterceptor(func(context.Context, []Object, []Edge) error {
		intercepted++
		return nil
	})
	revision := gateObject(t, "choir.texture_revision", "w")
	wrong := WithWriter(ctx, WriterResearch)
	for name, write := range map[string]func() error{
		"PutObject":      func() error { return store.PutObject(wrong, revision) },
		"PutBatch":       func() error { return store.PutBatch(wrong, Batch{Objects: []Object{revision}}) },
		"PreflightWrite": func() error { return store.PreflightWrite(wrong, []Object{revision}, nil) },
		"PutBatch undeclared": func() error {
			return store.PutBatch(ctx, Batch{Objects: []Object{revision}})
		},
	} {
		err := write()
		if !errors.Is(err, ErrWrongWriter) && !errors.Is(err, ErrUndeclaredWriter) {
			t.Errorf("%s: err = %v, want a writer refusal", name, err)
		}
	}
	if intercepted != 0 {
		t.Fatalf("interceptor saw %d refused writes; the writer gate must run before the event tape", intercepted)
	}
	if _, err := store.GetObject(ctx, revision.CanonicalID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("refused object was stored: err = %v", err)
	}
	if err := store.PutBatch(WithWriter(ctx, WriterTexture), Batch{Objects: []Object{revision}}); err != nil {
		t.Fatalf("texture's own write refused: %v", err)
	}
	if intercepted != 1 {
		t.Fatalf("the declared writer's write reached the interceptor %d times, want 1", intercepted)
	}
}
