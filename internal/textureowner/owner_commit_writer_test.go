package textureowner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// A write set that carries a source entity or ref is committed by the writer
// the registry names for those kinds, which also writes the revision; one
// that carries none is the owner's.
func TestOwnerCommitContextPicksTheWriterFromTheWriteSet(t *testing.T) {
	declared := func(ctx context.Context) objectgraph.Writer {
		w, _ := objectgraph.WriterFrom(ctx)
		return w
	}
	if got := declared(ownerCommitContext(context.Background(), store.TextureSourceGraphWriteSet{})); got != objectgraph.WriterOwner {
		t.Fatalf("empty write set declares %q, want the owner", got)
	}
	for name, graph := range map[string]struct {
		set  store.TextureSourceGraphWriteSet
		kind objectgraph.ObjectKind
	}{
		"entities": {store.TextureSourceGraphWriteSet{SourceEntities: make([]store.TextureSourceEntityGraphRecord, 1)}, store.TextureSourceEntityObjectKind},
		"refs":     {store.TextureSourceGraphWriteSet{SourceRefs: make([]store.TextureSourceRefGraphRecord, 1)}, store.TextureSourceRefObjectKind},
	} {
		want, ok := objectgraph.CustodianOf(graph.kind)
		if !ok {
			t.Fatalf("%s: %s has no custodian", name, graph.kind)
		}
		got := declared(ownerCommitContext(context.Background(), graph.set))
		if got != want {
			t.Fatalf("%s: declares %q, want the kind's writer %q", name, got, want)
		}
		if rev, _ := objectgraph.CustodianOf("choir.texture_revision"); rev != got {
			t.Fatalf("%s: the writer %q does not also write choir.texture_revision (%q), so one commit could not hold both", name, got, rev)
		}
	}
}

// Every call in this package that commits a source graph names where its
// writer comes from. A new commit path that passes a bare context fails here
// until it uses ownerCommitContext (owner routes) or is listed with the reason
// its context is already a desk's.
func TestEverySourceGraphCommitNamesItsWriter(t *testing.T) {
	deskContext := map[string]string{
		"texture_turn_runtime.go": "a desk's turn: the run's context declares the desk",
		"texture_cell_author.go":  "a desk's authored revision: the run's context declares the desk",
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if !strings.Contains(line, "WithSourceGraph(") || strings.Contains(line, "Digest(") || strings.HasPrefix(strings.TrimSpace(line), "//") || strings.Contains(line, "func ") {
				continue
			}
			found++
			if _, ok := deskContext[file]; ok {
				continue
			}
			if !strings.Contains(line, "ownerCommitContext(") && !strings.Contains(line, "asTextureCustodian(") {
				t.Errorf("%s:%d commits a source graph without ownerCommitContext or asTextureCustodian: %s", file, i+1, strings.TrimSpace(line))
			}
		}
	}
	if found < 4 {
		t.Fatalf("saw %d source-graph commits, want at least 4: the scan is not finding the call sites", found)
	}
}
