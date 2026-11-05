package objectgraph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// The reader gate (MA3.3): a read declares its reader, and the gate checks it
// against the kind's custodian and the readers section 8.2 lists. Package
// kinds enforce now; built-in kinds observe.

func readerMode(t *testing.T, mode WriterGateMode) {
	t.Helper()
	t.Cleanup(SetReaderGateMode(mode))
}

func TestCheckReadAsAdmitsTheCustodianTheOwnerAndTheCommons(t *testing.T) {
	readerMode(t, WriterGateEnforce)
	cases := []struct {
		name     string
		declared Writer
		kind     ObjectKind
		wantErr  error
	}{
		{"texture reads its document", WriterTexture, "choir.texture_document", nil},
		{"research cannot read a texture document (X1)", WriterResearch, "choir.texture_document", ErrWrongReader},
		{"management cannot read a texture revision (X8)", WriterManagement, "choir.texture_revision", ErrWrongReader},
		{"the owner reads a texture document (E1)", WriterOwner, "choir.texture_document", nil},
		{"the kernel reads a texture document", WriterKernel, "choir.texture_document", nil},
		{"management reads engineering's kinds (supervisory)", WriterManagement, "choir.engineering_lineage", nil},
		{"research cannot read engineering's kinds", WriterResearch, "choir.engineering_lineage", ErrWrongReader},
		{"engineering cannot read management's kinds", WriterEngineering, "choir.capture_label", ErrWrongReader},
		{"every desk reads the commons", WriterResearch, "choir.inbox_head", nil},
		{"every desk reads a corpusd kind (E4)", WriterTexture, "choir.publication", nil},
		{"the owner reads a corpusd kind", WriterOwner, "choir.publication", nil},
		{"settings are the shell's: texture is a crossing", WriterTexture, "choir.computer_setting", ErrWrongReader},
	}
	for _, tc := range cases {
		err := CheckReadAs(WithReader(context.Background(), tc.declared), tc.kind)
		if tc.wantErr == nil {
			if _, err := writeRegistry.LookupKind(tc.kind); err != nil {
				t.Fatalf("%s: the test names a kind the registry lacks: %v", tc.name, err)
			}
		}
		// Built-in kinds only observe: in enforce mode the gate refuses, in
		// observe it counts (TestBuiltInKindsObserve).
		if !errors.Is(err, tc.wantErr) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.wantErr)
		}
	}
}

// A context that declares a writer and no reader reads as that writer; a
// declared reader wins; neither declares nothing.
func TestReaderFallsBackToTheDeclaredWriter(t *testing.T) {
	ctx := WithWriter(context.Background(), WriterTexture)
	if r, ok := ReaderFrom(ctx); !ok || r != WriterTexture {
		t.Fatalf("reader = %q, %v; want the writer", r, ok)
	}
	if r, _ := ReaderFrom(WithReader(ctx, WriterResearch)); r != WriterResearch {
		t.Fatalf("a declared reader lost to the writer: %q", r)
	}
	if _, ok := ReaderFrom(context.Background()); ok {
		t.Fatal("an empty context declared a reader")
	}
	if r, _ := ReaderFrom(WithReaderIfAbsent(WithReader(context.Background(), WriterResearch), WriterOwner)); r != WriterResearch {
		t.Fatalf("WithReaderIfAbsent overwrote %q", r)
	}
	if r, _ := ReaderFrom(WithReaderIfAbsent(ctx, WriterOwner)); r != WriterTexture {
		t.Fatalf("WithReaderIfAbsent overwrote the writer: %q", r)
	}
}

// The production default: built-in kinds only observe, counting each crossing
// under kind|declared reader|outcome.
func TestBuiltInKindsObserve(t *testing.T) {
	readerMode(t, WriterGateObserve)
	ResetReaderGateCounts()
	ctx := WithReader(context.Background(), WriterResearch)
	for i := 0; i < 3; i++ {
		if err := CheckReadAs(ctx, "choir.texture_document"); err != nil {
			t.Fatalf("observe refused a read: %v", err)
		}
	}
	if err := CheckReadAs(context.Background(), "choir.texture_document"); err != nil {
		t.Fatalf("observe refused an undeclared read: %v", err)
	}
	counts := ReaderGateCounts()
	if counts["choir.texture_document|desk:research|wrong_reader"] != 3 {
		t.Errorf("counts = %v", counts)
	}
	if counts["choir.texture_document|undeclared|undeclared_reader"] != 1 {
		t.Errorf("undeclared read not counted: %v", counts)
	}
	if err := CheckReadAs(context.Background(), "choir.inbox_head"); err != nil || len(ReaderGateCounts()) != 2 {
		t.Errorf("a commons read was counted: %v %v", err, ReaderGateCounts())
	}
	writerOnly := WithWriter(context.Background(), WriterTexture)
	ResetReaderGateCounts()
	_ = CheckReadAs(writerOnly, "choir.texture_document")
	if len(ReaderGateCounts()) != 0 {
		t.Errorf("a write path reading its own kind was counted: %v", ReaderGateCounts())
	}
	// Off checks nothing.
	readerMode(t, WriterGateOff)
	ResetReaderGateCounts()
	_ = CheckReadAs(ctx, "choir.texture_document")
	if len(ReaderGateCounts()) != 0 {
		t.Errorf("off counted: %v", ReaderGateCounts())
	}
}

func storedObject(t *testing.T, kind ObjectKind, key string) Object {
	t.Helper()
	obj := gateObject(t, kind, key)
	obj.Body = json.RawMessage(`{"k":"v"}`)
	obj.Metadata = json.RawMessage(`{"key":"` + key + `"}`)
	obj.ContentHash = ContentHash(kind, obj.Body, obj.Metadata)
	obj.CreatedAt, obj.UpdatedAt = time.Now().UTC(), time.Now().UTC()
	return obj
}

// A package kind is readable by its package desk and the owner alone, in the
// default (observe) mode; the store refuses every read path: by id, by list,
// by page, by metadata. A built-in kind read by a wrong reader is counted and
// served.
func TestPackageKindsEnforceInTheStoreWhileBuiltInKindsObserve(t *testing.T) {
	readerMode(t, WriterGateObserve)
	writerMode(t, WriterGateObserve)
	ResetReaderGateCounts()
	pkg := "ma33read"
	k := pkgKind(pkg, "note")
	if err := RegisterPackageKind(pkg, k); err != nil {
		t.Fatal(err)
	}
	st := NewSQLStore(openTestDB(t))
	ctx := context.Background()
	if err := st.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	own := WithWriter(ctx, Writer("desk:"+pkg))
	pkgObj := storedObject(t, k.Kind, "note-1")
	if err := st.PutObject(own, pkgObj); err != nil {
		t.Fatalf("seed package object: %v", err)
	}
	doc := storedObject(t, "choir.texture_document", "doc-1")
	if err := st.PutObject(WithWriter(ctx, WriterTexture), doc); err != nil {
		t.Fatalf("seed texture document: %v", err)
	}

	reads := map[string]func(c context.Context) error{
		"get": func(c context.Context) error { _, err := st.GetObject(c, pkgObj.CanonicalID); return err },
		"list": func(c context.Context) error {
			_, err := st.ListObjects(c, ListFilter{Kind: k.Kind, Limit: 10})
			return err
		},
		"page": func(c context.Context) error { _, err := st.ListObjectsPage(c, string(k.Kind), "", 10); return err },
		"metadata": func(c context.Context) error {
			_, err := st.GetObjectByMetadata(c, string(k.Kind), "$.key", "note-1")
			return err
		},
	}
	for name, read := range reads {
		if err := read(WithReader(ctx, Writer("desk:"+pkg))); err != nil {
			t.Errorf("%s: the package's desk: %v", name, err)
		}
		if err := read(WithReader(ctx, WriterOwner)); err != nil {
			t.Errorf("%s: the owner: %v", name, err)
		}
		for who, c := range map[string]context.Context{
			"another desk": WithReader(ctx, WriterManagement),
			"the kernel":   WithReader(ctx, WriterKernel),
			"undeclared":   ctx,
		} {
			if err := read(c); !errors.Is(err, ErrWrongReader) && !errors.Is(err, ErrUndeclaredReader) {
				t.Errorf("%s by %s: err = %v, want a refusal", name, who, err)
			}
		}
	}
	// A list with no kind filter drops the package rows the reader may not see.
	all, err := st.ListObjects(WithReader(ctx, WriterManagement), ListFilter{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range all {
		if o.ObjectKind == k.Kind {
			t.Errorf("an unfiltered list handed another desk the package object %s", o.CanonicalID)
		}
	}
	if own, err := st.ListObjects(WithReader(ctx, Writer("desk:"+pkg)), ListFilter{Limit: 50}); err != nil || len(own) == 0 {
		t.Errorf("the package's desk lost its own rows: %v %v", own, err)
	}

	// Built-in: counted and served.
	ResetReaderGateCounts()
	got, err := st.GetObject(WithReader(ctx, WriterResearch), doc.CanonicalID)
	if err != nil || got.CanonicalID != doc.CanonicalID {
		t.Fatalf("a built-in kind read by a wrong reader was refused: %v", err)
	}
	if n := ReaderGateCounts()["choir.texture_document|desk:research|wrong_reader"]; n != 1 {
		t.Errorf("counts = %v", ReaderGateCounts())
	}
	if _, err := st.GetObject(WithReader(ctx, WriterTexture), doc.CanonicalID); err != nil {
		t.Errorf("custodian: %v", err)
	}
	if n := ReaderGateCounts()["choir.texture_document|desk:research|wrong_reader"]; n != 1 || len(ReaderGateCounts()) != 1 {
		t.Errorf("custodian's read was counted: %v", ReaderGateCounts())
	}
}

// The oracle tallies an undeclared read of a desk kind from a non-test caller
// only; a commons read and a test-file caller are not tallied.
func TestUndeclaredReadOracleSkipsTestFilesAndTheCommons(t *testing.T) {
	stop := TrackUndeclaredReads()
	defer stop()
	ResetUndeclaredReads()
	_ = CheckReadAs(context.Background(), "choir.texture_document")
	_ = CheckReadAs(context.Background(), "choir.inbox_head")
	if got := UndeclaredReads(); len(got) != 0 {
		t.Fatalf("tallied a read called from a test file: %v", got)
	}
}

// MA3.7c (E4): world events, memberships and capture labels are shared world
// content, read by research and Texture without a count; Texture's documents
// are still not research's.
func TestSharedWorldContentKindsAreAdmittedToResearchAndTexture(t *testing.T) {
	readerMode(t, WriterGateObserve)
	for _, kind := range []ObjectKind{"choir.world_event", "choir.event_membership", "choir.capture_label"} {
		for _, reader := range []Writer{WriterResearch, WriterTexture, WriterManagement} {
			ResetReaderGateCounts()
			_ = CheckReadAs(WithReader(context.Background(), reader), kind)
			if len(ReaderGateCounts()) != 0 {
				t.Errorf("%s read of %s was counted: %v", reader, kind, ReaderGateCounts())
			}
		}
	}
	ResetReaderGateCounts()
	_ = CheckReadAs(WithReader(context.Background(), WriterEngineering), "choir.world_event")
	_ = CheckReadAs(WithReader(context.Background(), WriterResearch), "choir.capture_label_override")
	if len(ReaderGateCounts()) != 2 {
		t.Errorf("the admission is wider than E4's kinds and desks: %v", ReaderGateCounts())
	}
}

// A violating read records its caller's site: the first frame outside the
// graph and the store, here this test.
func TestReaderGateRecordsCallSite(t *testing.T) {
	readerMode(t, WriterGateObserve)
	ResetReaderGateCounts()
	if err := CheckReadAs(context.Background(), "choir.texture_document"); err != nil {
		t.Fatalf("observe refused: %v", err)
	}
	var found bool
	for k, n := range ReaderGateSites() {
		if strings.HasPrefix(k, "choir.texture_document|undeclared_reader|") &&
			strings.Contains(k, "TestReaderGateRecordsCallSite") &&
			strings.Contains(k, "internal/objectgraph/reader_gate_test.go:") && n == 1 {
			found = true
		}
	}
	if !found {
		t.Errorf("site not recorded: %v", ReaderGateSites())
	}
	for i := 0; i < maxGateSites+5; i++ {
		readerGateSites.tally(ObjectKind(fmt.Sprintf("k%d", i)), "x")
	}
	if got := ReaderGateSites(); len(got) > maxGateSites+1 || got[gateSitesOverflowKey] == 0 {
		t.Errorf("map unbounded or no overflow: %d keys", len(got))
	}
}
