package computerevent

// Release 1 of docs/design/object-bodies-inline-on-the-tape-2026-11-15.md
// (the reader), the batch validator. Written before the code. Failure list,
// from sections 2, 4 and 8 of the design, restricted to what the reader
// release can fail here:
//
//  V1. Validate accepts the pair (batch 3, projector 2), so a version 3 batch
//      that holds an inline object op replays. Without it the first version 3
//      batch is refused by the reader release itself. (section 5, "Release 1")
//  V2. Validate still refuses every other pair; (3,2) is added without
//      loosening anything: (3,1), (3,3), (1,2), (2,1), (2,3), (1,3), (0,x),
//      (4,x). (failure 14)
//  V3. An old binary refuses a version 3 batch with ErrProjectionBatchInvalid
//      "unsupported version" and never applies it as a bodyless object. The
//      pre-change Validate predicate is vendored below; this simulates an old
//      binary, it is not one. (failure 1, F1; section 9 "Old-binary refusal")
//  V4. DecodeProjectionBatch of a version 3 batch whose op body carries
//      body_json returns the op body bytes verbatim, so the reader sees the
//      inline form. A version 1 batch still refuses a V2-only kind. (failure 11)
//  V5. A version 3 batch with the version 2 projector's kinds validates the same
//      as a version 2 batch: nothing else in Validate reads the batch version.
//  V6. Historical fixtures stay valid forever: a batch of each historical
//      version decodes and validates. (failure 28)
//  V7. NO WRITER EMITS body_json OR VERSION 3 IN RELEASE 1 (section 5): the
//      only non-test mention of ProjectionBatchV3 is its declaration and its
//      Validate use in this package, and no non-test source outside
//      internal/objectgraph/object.go names body_json. When release 2 adds the
//      constructor this test is the one it deliberately edits. (failure 15)
//  V8. The reader release writes no store floor: the four existing WriteFloor
//      call sites (telemetry partitions, header/body split, event index trim,
//      event table) are the only ones; a fifth is release 2's and edits this
//      list. (section 4, "It writes no floor")

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// oldValidVersion is the version predicate of Validate before release 1.
func oldValidVersion(version, projector int) bool {
	return (version == 1 && projector == 1) || (version == 2 && projector == 2)
}

func TestValidateAcceptsExactlyTheKnownVersionPairs(t *testing.T) { // V1, V2
	accepted := map[[2]int]bool{{1, 1}: true, {2, 2}: true, {3, 2}: true}
	for version := 0; version <= 4; version++ {
		for projector := 0; projector <= 4; projector++ {
			batch := ProjectionBatch{Version: version, ProjectorVersion: projector, ComputerID: "c", EventID: "e",
				Ops: []ProjectionOp{{Kind: ProjectionOpObject, CanonicalID: "x", Body: []byte(`{"canonical_id":"x"}`)}}}
			err := batch.Validate()
			want := accepted[[2]int{version, projector}]
			if want && err != nil {
				t.Errorf("pair (%d,%d) refused: %v", version, projector, err)
			}
			if !want {
				if err == nil {
					t.Errorf("pair (%d,%d) accepted", version, projector)
				} else if !errors.Is(err, ErrProjectionBatchInvalid) || !strings.Contains(err.Error(), "unsupported version") {
					t.Errorf("pair (%d,%d) refused with %v, want ErrProjectionBatchInvalid unsupported version", version, projector, err)
				}
			}
		}
	}
}

const v3InlineBatch = `{"version":3,"projector_version":2,"computer_id":"c","event_id":"e","ops":[` +
	`{"kind":"object_recorded","canonical_id":"obj:k:o:1","body":{"canonical_id":"obj:k:o:1","object_kind":"k","owner_id":"o","content_hash":"sha256:x","body_json":{"event_id":"e1","a":[1, 2]},"metadata":{},"created_at":"2026-11-15T01:02:03Z","updated_at":"2026-11-15T01:02:03Z"}}]}`

func TestOldBinarySimulationRefusesVersion3(t *testing.T) { // V3
	if oldValidVersion(ProjectionBatchV3, ProjectorVersionV2) {
		t.Fatal("the vendored pre-change predicate accepts (3,2); the simulation is wrong")
	}
	batch, err := DecodeProjectionBatch([]byte(v3InlineBatch))
	if err != nil {
		t.Fatalf("the reader refuses a version 3 batch: %v", err)
	}
	if oldValidVersion(batch.Version, batch.ProjectorVersion) {
		t.Fatalf("an old binary would accept (%d,%d) and read the inline op as a bodyless object", batch.Version, batch.ProjectorVersion)
	}
}

func TestDecodeKeepsInlineOpBytesVerbatim(t *testing.T) { // V4, V5
	batch, err := DecodeProjectionBatch([]byte(v3InlineBatch))
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Ops) != 1 || !strings.Contains(string(batch.Ops[0].Body), `"body_json":{"event_id":"e1","a":[1, 2]}`) {
		t.Fatalf("op body = %s", batch.Ops[0].Body)
	}
	v1 := ProjectionBatch{Version: ProjectionBatchV1, ProjectorVersion: ProjectorVersionV1, ComputerID: "c", EventID: "e",
		Ops: []ProjectionOp{{Kind: ProjectionOpRunMemoryEntry, Body: []byte(`{}`)}}}
	if err := v1.Validate(); err == nil {
		t.Fatal("a version 1 batch accepted a version 2 only kind")
	}
	for _, version := range []int{ProjectionBatchV2, ProjectionBatchV3} {
		b := ProjectionBatch{Version: version, ProjectorVersion: ProjectorVersionV2, ComputerID: "c", EventID: "e",
			Ops: []ProjectionOp{{Kind: ProjectionOpObjectRetire, Body: []byte(`{}`)}}}
		if err := b.Validate(); err != nil && strings.Contains(err.Error(), "unsupported version") {
			t.Fatalf("version %d: %v", version, err)
		}
	}
}

func TestHistoricalVersionsStillDecode(t *testing.T) { // V6
	for name, raw := range map[string]string{
		"v1": `{"version":1,"projector_version":1,"computer_id":"c","event_id":"e","ops":[{"kind":"object_recorded","canonical_id":"x","body":{"canonical_id":"x","object_kind":"k","owner_id":"o","content_hash":"h","body":"eyJhIjoxfQ==","metadata":{},"created_at":"2026-11-15T01:02:03Z","updated_at":"2026-11-15T01:02:03Z"}}]}`,
		"v2": `{"version":2,"projector_version":2,"computer_id":"c","event_id":"e","ops":[{"kind":"object_recorded","canonical_id":"x","body":{"canonical_id":"x","object_kind":"k","owner_id":"o","content_hash":"h","body":"eyJhIjoxfQ==","metadata":{},"created_at":"2026-11-15T01:02:03Z","updated_at":"2026-11-15T01:02:03Z"}}]}`,
	} {
		if _, err := DecodeProjectionBatch([]byte(raw)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// internalSources walks the repo's internal and cmd trees for non-test .go
// files and parses them.
func internalSources(t *testing.T) map[string]*ast.File {
	t.Helper()
	out := map[string]*ast.File{}
	root := filepath.Join("..", "..")
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, perr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if perr != nil {
				t.Fatalf("parse %s: %v", path, perr)
			}
			out[filepath.ToSlash(path)] = file
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(out) < 100 {
		t.Fatalf("source walk found only %d files; the guard would pass vacuously", len(out))
	}
	return out
}

func TestNoWriterEmitsBodyJSONOrVersion3InTheReaderRelease(t *testing.T) { // V7
	for path, file := range internalSources(t) {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.Ident:
				if node.Name == "ProjectionBatchV3" && !strings.HasSuffix(path, "internal/computerevent/projection_batch.go") {
					t.Errorf("%s: ProjectionBatchV3 used outside its validator; release 1 has no writer", path)
				}
			case *ast.BasicLit:
				if node.Kind != token.STRING {
					return true
				}
				s, err := strconv.Unquote(node.Value)
				if err != nil {
					return true
				}
				if strings.Contains(s, "body_json") && !strings.HasSuffix(path, "internal/objectgraph/object.go") {
					t.Errorf("%s: string literal %q names body_json; only the reader in internal/objectgraph/object.go may", path, s)
				}
			case *ast.StructType:
				for _, field := range node.Fields.List {
					if field.Tag != nil && strings.Contains(field.Tag.Value, "body_json") && !strings.HasSuffix(path, "internal/objectgraph/object.go") {
						t.Errorf("%s: a struct tag names body_json", path)
					}
				}
			case *ast.KeyValueExpr:
				if key, ok := node.Key.(*ast.Ident); ok && key.Name == "Version" {
					if lit, ok := node.Value.(*ast.BasicLit); ok && lit.Value == "3" {
						t.Errorf("%s: a literal Version: 3", path)
					}
				}
			}
			return true
		})
	}
}

func TestReaderReleaseWritesNoStoreFloor(t *testing.T) { // V8
	existing := map[string]bool{
		"internal/store/telemetry_partition.go": true,
		"internal/store/header_body_split.go":   true,
		"internal/store/event_index_trim.go":    true,
		"internal/store/event_table.go":         true,
	}
	for path, file := range internalSources(t) {
		path = strings.TrimPrefix(path, "../../")
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "WriteFloor" && !existing[path] && !strings.HasPrefix(path, "internal/storeschema/") {
				t.Errorf("%s: a new WriteFloor call; release 1 writes no floor", path)
			}
			return true
		})
	}
}
