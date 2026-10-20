package types

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// eventKindConstants parses this package's non-test sources and returns every
// constant declared with the type EventKind, by value. The source, not a
// hand-kept list, is the authority: a kind added to task.go is found here
// whether or not anyone remembered to classify it.
func eventKindConstants(t *testing.T) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				ident, ok := vs.Type.(*ast.Ident)
				if !ok || ident.Name != "EventKind" {
					continue
				}
				for i, n := range vs.Names {
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						t.Fatalf("%s: EventKind constant %s is not a string literal; the classification test cannot read it", name, n.Name)
					}
					value, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatal(err)
					}
					found[value] = n.Name
				}
			}
		}
	}
	return found
}

// P0-1: a new EventKind cannot ship unclassified, and the table cannot keep
// an entry for a kind that no longer exists.
func TestEveryEventKindIsClassified(t *testing.T) {
	consts := eventKindConstants(t)
	if len(consts) < 40 {
		t.Fatalf("parsed only %d EventKind constants; the parser test is not reading task.go", len(consts))
	}
	var missing, stale []string
	for value, name := range consts {
		if _, ok := eventDurability[EventKind(value)]; !ok {
			missing = append(missing, name+" ("+value+")")
		}
	}
	for kind := range eventDurability {
		if _, ok := consts[string(kind)]; !ok {
			stale = append(stale, string(kind))
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	if len(missing) > 0 {
		t.Fatalf("EventKind constants with no entry in eventDurability (classify each as batched or acknowledged in event_durability.go): %v", missing)
	}
	if len(stale) > 0 {
		t.Fatalf("eventDurability has entries with no EventKind constant: %v", stale)
	}
}

// P0-2: default for anything the table does not name is acknowledged.
func TestDurabilityUnknownKindIsAcknowledged(t *testing.T) {
	for _, kind := range []EventKind{"", "loop.unknown", "LOOP.DELTA", "loop.delta ", "tool.result.v2"} {
		if got := kind.Durability(); got != DurabilityAcknowledged {
			t.Errorf("Durability(%q) = %q, want acknowledged", kind, got)
		}
		if kind.Batchable() {
			t.Errorf("Batchable(%q) = true for a kind outside the table", kind)
		}
	}
}

// P0-3: exactly the ten kinds of design section 3.1 batch.
func TestDurabilityTableIsTheDesignedTen(t *testing.T) {
	want := []string{
		"loop.delta", "loop.progress", "loop.retry", "loop.compaction.started", "loop.compaction.completed",
		"tool.invoked", "tool.result", "desk.cell", "desk.egress", "texture.agent_revision.progress",
	}
	var got []string
	for kind, d := range eventDurability {
		if d == DurabilityBatched {
			got = append(got, string(kind))
		}
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("batched kinds = %v\nwant %v (design telemetry-batching-design-2026-10-20.md section 3.1; changing this set needs the design and the panel)", got, want)
	}
	if n := len(eventDurability); n != 42 {
		t.Fatalf("eventDurability has %d entries, design accounts for 42 runtime kinds", n)
	}
}
