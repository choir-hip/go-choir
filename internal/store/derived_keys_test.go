package store

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The Store's side of the og_key_index side table
// (docs/problems/store-hot-reads-need-a-rollback-safe-side-table-2026-10-21.md).
// Ways it can fail, written before the code:
//  1. A writer of og_objects the table does not know: the source guard below
//     and a behavioural test of the production writer, the projector.
//  2. The three reads still scan after validation, or leave the JSON path
//     before it (TestHotReadsLeaveTheJSONScansOnceValidated).
//  3. A replayed workspace reads stale keys (TestReplayProjectionWithdrawsTheFamilies).
//  4. A family with a malformed row stays on the JSON path after the row is
//     repaired, because the background loop never runs another pass
//     (TestRepairedMalformedRowRestoresTheSideRead).

var ogWriterStatement = regexp.MustCompile(`(?i)\b(INSERT(\s+IGNORE)?\s+INTO|REPLACE\s+INTO|UPDATE|DELETE\s+FROM)\s+og_objects\b`)

// maintainedOGWriters are the only guest functions that write og_objects. Each
// one must call the derived-key maintainer (Maintain or Drop), or use the
// shared upsert from a function that does. A new writer fails this test until
// it is added here with its maintenance.
var maintainedOGWriters = map[string][]string{
	"objectgraph/sql_store.go":  {"<package>", "DeleteObject"}, // <package>: upsertObjectSQL, used by PutObject and putBatchTx
	"store/project.go":          {"projectObject"},
	"store/object_retire.go":    {"projectObjectRetire"},
	"store/vocab_migrate_og.go": {"applyOGMigration", "revertOGMigration"},
}

func TestOnlyMaintainedWritersTouchOgObjects(t *testing.T) {
	root := filepath.Join("..") // internal/
	type site struct{ file, fn string }
	var found []site
	callsMaintainer := map[site]bool{}
	usesUpsertFn := map[site]bool{}
	fset := token.NewFileSet()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if info.IsDir() {
			// The corpusd platform store is a different database; scripts
			// and tests are not guest writers.
			if rel == "platform" || strings.HasPrefix(rel, "platform"+string(filepath.Separator)) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		rel = filepath.ToSlash(rel)
		for _, decl := range file.Decls {
			name := "<package>"
			if fd, ok := decl.(*ast.FuncDecl); ok {
				name = fd.Name.Name
			}
			hit, maintains, usesUpsert := false, false, false
			ast.Inspect(decl, func(n ast.Node) bool {
				switch v := n.(type) {
				case *ast.BasicLit:
					if v.Kind == token.STRING && ogWriterStatement.MatchString(v.Value) {
						hit = true
					}
				case *ast.Ident:
					if v.Name == "Maintain" || v.Name == "Drop" {
						maintains = true
					}
					if v.Name == "upsertObjectSQL" {
						usesUpsert = true
					}
				case *ast.SelectorExpr:
					if v.Sel.Name == "Maintain" || v.Sel.Name == "Drop" {
						maintains = true
					}
				}
				return true
			})
			if hit {
				s := site{rel, name}
				found = append(found, s)
				callsMaintainer[s] = maintains
			}
			if usesUpsert && name != "<package>" {
				// A user of the shared upsert is a writer too.
				s := site{rel, name}
				if !hit {
					found = append(found, s)
				}
				callsMaintainer[s] = maintains
				usesUpsertFn[s] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var problems []string
	for _, s := range found {
		allowed := usesUpsertFn[s] && s.file == "objectgraph/sql_store.go" // PutObject, putBatchTx
		for _, fn := range maintainedOGWriters[s.file] {
			if fn == s.fn {
				allowed = true
			}
		}
		if !allowed {
			problems = append(problems, fmt.Sprintf("%s: %s writes og_objects and is not a maintained writer; give it Maintain/Drop and list it in maintainedOGWriters", s.file, s.fn))
		} else if s.fn != "<package>" && !callsMaintainer[s] {
			problems = append(problems, fmt.Sprintf("%s: %s writes og_objects and never calls Maintain or Drop", s.file, s.fn))
		}
	}
	sort.Strings(problems)
	if len(problems) > 0 {
		t.Fatalf("%s", strings.Join(problems, "\n"))
	}
	have := map[string]bool{}
	for _, s := range found {
		have[s.file+"|"+s.fn] = true
	}
	for _, want := range []string{"objectgraph/sql_store.go|<package>", "objectgraph/sql_store.go|PutObject", "objectgraph/sql_store.go|putBatchTx", "objectgraph/sql_store.go|DeleteObject", "store/project.go|projectObject", "store/vocab_migrate_og.go|applyOGMigration", "store/vocab_migrate_og.go|revertOGMigration"} {
		if !have[want] {
			t.Errorf("the guard did not find the known writer %s (found %d sites)", want, len(found))
		}
	}
}

func opCallsContaining(s *Store, substrs ...string) uint64 {
	var n uint64
	for op, st := range s.EngineMutexStats() {
		match := true
		for _, sub := range substrs {
			if !strings.Contains(op, sub) {
				match = false
			}
		}
		if match {
			n += st.Calls
		}
	}
	return n
}

func TestProjectorKeepsDerivedKeysInItsTransaction(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-derived-keys"
	prepareGenesis(t, productStore, computerID, "genesis-derived")
	head, err := productStore.Head(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	runID, _ := objectgraph.BuildCanonicalID(objectgraph.ObjectKind("choir.run"), "owner-1", "run-1")
	recID, _ := objectgraph.BuildCanonicalID(objectgraph.ObjectKind("choir.commitment_record"), "owner-1", "rec-1")
	object := func(id string, kind objectgraph.ObjectKind, meta map[string]any, body string, at time.Time) json.RawMessage {
		m, _ := json.Marshal(meta)
		b, err := json.Marshal(objectgraph.Object{CanonicalID: id, ObjectKind: kind, OwnerID: "owner-1", ComputerID: computerID, VersionID: "v",
			ContentHash: objectgraph.ContentHash(kind, []byte(body), m), Body: []byte(body), Metadata: m, CreatedAt: at, UpdatedAt: at})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	finalize := func(idem string, ops ...computerevent.ProjectionOp) {
		eventID, err := computerevent.NewEventID()
		if err != nil {
			t.Fatal(err)
		}
		event, digest := prepareProjectionEvent(t, productStore, *head, eventID, idem)
		batch := computerevent.ProjectionBatch{Version: computerevent.ProjectionBatchV1, ProjectorVersion: computerevent.ProjectorVersionV1,
			ComputerID: computerID, EventID: event.EventID, EventDigest: digest, Ops: ops}
		if err := productStore.FinalizeBatch(ctx, computerID, digest, signedReceipt(t, computerID, digest, event.Sequence), &batch); err != nil {
			t.Fatal(err)
		}
		if head, err = productStore.Head(ctx, computerID); err != nil {
			t.Fatal(err)
		}
	}
	key := func(id, name string) string {
		var v string
		if err := productStore.db.QueryRow(`SELECT COALESCE(key_value, '<null>') FROM og_key_index WHERE canonical_id = ? AND key_name = ?`, id, name).Scan(&v); err != nil {
			return "<missing>"
		}
		return v
	}
	finalize("derived-1",
		computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObject, CanonicalID: runID, Body: object(runID, "choir.run", map[string]any{"agent_id": "texture:a"}, `{}`, now)},
		computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObject, CanonicalID: recID, Body: object(recID, "choir.commitment_record", map[string]any{}, `{"addressee":"management:o"}`, now)})
	if got := key(runID, objectgraph.KeyRunAgentID); got != "texture:a" {
		t.Fatalf("the projector left run key %q, want texture:a", got)
	}
	if got := key(recID, objectgraph.KeyCommitmentAddressee); got != "management:o" {
		t.Fatalf("the projector left record key %q, want management:o", got)
	}
	finalize("derived-2",
		computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObject, CanonicalID: runID, Body: object(runID, "choir.run", map[string]any{"agent_id": "texture:b"}, `{}`, now.Add(time.Minute))})
	if got := key(runID, objectgraph.KeyRunAgentID); got != "texture:b" {
		t.Fatalf("an update through the projector left run key %q, want texture:b", got)
	}
}

// A replay writes og_objects through projectBatchForReplay, which does not
// maintain the side table: the families must leave the table's reads at once
// and the online pass must put the rows back.
func TestReplayProjectionWithdrawsTheFamilies(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-derived-replay"
	prepareGenesis(t, productStore, computerID, "genesis-derived-replay")
	head, err := productStore.Head(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	if err := productStore.ValidateDerivedKeys(ctx, objectgraph.DerivedKeyValidation{}); err != nil {
		t.Fatal(err)
	}
	d := productStore.derivedKeys()
	if !d.Validated(objectgraph.KeyRunAgentID) {
		t.Fatal("test premise: an empty store validates")
	}
	now := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	runID, _ := objectgraph.BuildCanonicalID(objectgraph.ObjectKind("choir.run"), "owner-1", "run-replayed")
	meta, _ := json.Marshal(map[string]any{"agent_id": "texture:r"})
	body, _ := json.Marshal(objectgraph.Object{CanonicalID: runID, ObjectKind: "choir.run", OwnerID: "owner-1", ComputerID: computerID, VersionID: "v",
		ContentHash: objectgraph.ContentHash("choir.run", []byte(`{}`), meta), Body: []byte(`{}`), Metadata: meta, CreatedAt: now, UpdatedAt: now})
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event, digest := prepareProjectionEvent(t, productStore, *head, eventID, "replayed-1")
	batch := computerevent.ProjectionBatch{Version: computerevent.ProjectionBatchV1, ProjectorVersion: computerevent.ProjectorVersionV1,
		ComputerID: computerID, EventID: event.EventID, EventDigest: digest,
		Ops: []computerevent.ProjectionOp{{Kind: computerevent.ProjectionOpObject, CanonicalID: runID, Body: body}}}
	if err := productStore.FinalizeReplayBatch(ctx, computerID, digest, signedReceipt(t, computerID, digest, event.Sequence), &batch); err != nil {
		t.Fatal(err)
	}
	if d.Validated(objectgraph.KeyRunAgentID) {
		t.Fatal("a replayed object left the run family validated")
	}
	var n int
	if err := productStore.db.QueryRow(`SELECT COUNT(*) FROM og_key_index`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("replay maintained the side table (%d rows, %v); it is filled online after boot", n, err)
	}
	if err := productStore.ValidateDerivedKeys(ctx, objectgraph.DerivedKeyValidation{}); err != nil {
		t.Fatal(err)
	}
	objs, err := productStore.ogListAllByMetadata(ctx, objectgraph.ObjectKind("choir.run"), "agent_id", "texture:r")
	if err != nil || len(objs) != 1 || !d.Validated(objectgraph.KeyRunAgentID) {
		t.Fatalf("after the pass: %d objects, %v, validated %t", len(objs), err, d.Validated(objectgraph.KeyRunAgentID))
	}
}
