package objectgraph

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
)

// docs/problems/research-more-slow-and-hidden-2026-10-21.md, E6 (steady
// state after c038df7c6): four equalities on unindexed fields held the
// paper's engine 70% of the time: runs by $.agent_id, capture labels by
// $.capture_id, source refs by $.doc_id (metadata), and commitment records
// by the body's $.addressee. Failure modes, as for the 10-14 columns:
//  1. the indexed read returns other rows or another order than the JSON
//     read (case, trailing spaces, JSON null, numbers, missing, long);
//  2. the planner scans instead of seeking the index;
//  3. a column for one kind changes another kind's reads;
//  4. a record whose body is not JSON refuses the write.

func TestHotFieldColumnsMatchJSONReads(t *testing.T) {
	db := openTestDoltDB(t)
	ctx := context.Background()
	indexed := NewDoltStore(db)
	if err := indexed.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	unindexed := NewDoltStore(db)
	unindexed.ShareEngineMutex(indexed)
	for _, column := range []string{"og_meta_agent_id", "og_meta_capture_id", "og_meta_doc_id", recordAddresseeIndex} {
		if !indexed.live(column) {
			t.Fatalf("column %s not live: %v", column, indexed.liveNames())
		}
	}

	p255 := strings.Repeat("p", generatedColumnMaxChars)
	values := []any{"texture:doc-1", "Texture:doc-1", "texture:doc-1 ", "management:o", "texture:dóc-ü", "", "null", 12, true, nil,
		p255, p255 + "z", p255[:254]}
	lookups := []string{"texture:doc-1", "Texture:doc-1", "texture:doc-1 ", "texture:dóc-ü", "", "null", "12", "true", p255, p255 + "z", p255[:254], "absent"}
	kinds := []ObjectKind{"choir.run", "choir.capture_label", "choir.source_ref", commitmentRecordKind, "choir.event"}
	fields := []string{"agent_id", "capture_id", "doc_id"}

	rng := rand.New(rand.NewSource(1021))
	base := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	for i := 0; i < 300; i++ {
		metadata := map[string]any{}
		for _, field := range fields {
			if rng.Intn(5) > 0 {
				metadata[field] = values[rng.Intn(len(values))]
			}
		}
		body := `{}`
		if rng.Intn(6) > 0 {
			switch v := values[rng.Intn(len(values))].(type) {
			case string:
				body = fmt.Sprintf(`{"addressee": %q}`, v)
			case nil:
				body = `{"addressee": null}`
			default:
				body = fmt.Sprintf(`{"addressee": %v}`, v)
			}
		}
		kind := kinds[rng.Intn(len(kinds))]
		putTestObject(t, indexed, fmt.Sprintf("obj-%03d-%d", rng.Intn(1000), i), kind, metadata, body, base.Add(time.Duration(i)*time.Second))
	}
	for i, kind := range kinds {
		obj := Object{CanonicalID: fmt.Sprintf("raw-%d", i), ObjectKind: kind, OwnerID: "owner-2", ComputerID: "computer", VersionID: "v",
			ContentHash: fmt.Sprintf("h-raw-%d", i), Body: []byte("not json"), Metadata: []byte(`{"agent_id":"texture:doc-1"}`), CreatedAt: base, UpdatedAt: base}
		if err := indexed.PutObject(ctx, obj); err != nil {
			t.Fatalf("a body that is not JSON refused the write: %v", err)
		}
	}
	ids := func(objs []Object) string {
		out := make([]string, len(objs))
		for i, o := range objs {
			out[i] = o.CanonicalID
		}
		return fmt.Sprint(out)
	}
	matched := 0
	for _, kind := range kinds {
		for _, field := range fields {
			for _, value := range lookups {
				for after := ""; ; {
					want, err := unindexed.ListObjectsByMetadataPage(ctx, string(kind), "$."+field, value, after, 9)
					if err != nil {
						t.Fatal(err)
					}
					got, err := indexed.ListObjectsByMetadataPage(ctx, string(kind), "$."+field, value, after, 9)
					if err != nil {
						t.Fatal(err)
					}
					if ids(got) != ids(want) {
						t.Fatalf("page %s %s=%q after %q:\n got %v\nwant %v", kind, field, value, after, ids(got), ids(want))
					}
					if len(want) == 0 {
						break
					}
					matched += len(want)
					after = want[len(want)-1].CanonicalID
				}
				wantList, _ := unindexed.ListObjectsByMetadata(ctx, string(kind), "$."+field, value, 40)
				gotList, _ := indexed.ListObjectsByMetadata(ctx, string(kind), "$."+field, value, 40)
				if ids(gotList) != ids(wantList) {
					t.Fatalf("list %s %s=%q:\n got %v\nwant %v", kind, field, value, ids(gotList), ids(wantList))
				}
			}
		}
		for _, value := range lookups {
			match := []JSONFieldMatch{{JSONPath: "$.addressee", Value: value}}
			want, err := unindexed.ListObjectsByOwnerAndBody(ctx, string(kind), "owner", match, 25)
			if err != nil {
				t.Fatal(err)
			}
			got, err := indexed.ListObjectsByOwnerAndBody(ctx, string(kind), "owner", match, 25)
			if err != nil {
				t.Fatal(err)
			}
			if ids(got) != ids(want) {
				t.Fatalf("body %s addressee=%q:\n got %v\nwant %v", kind, value, ids(got), ids(want))
			}
			matched += len(want)
		}
	}
	if matched < 100 {
		t.Fatalf("only %d matches: the fixture proves little", matched)
	}
}

func TestHotFieldColumnPlansSeekTheirIndex(t *testing.T) {
	db := openTestDoltDB(t)
	store := NewDoltStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	for i := 0; i < 20; i++ {
		putTestObject(t, store, fmt.Sprintf("run-%02d", i), "choir.run", map[string]any{"agent_id": fmt.Sprintf("texture:d%d", i%3)}, `{}`, at)
	}
	for kind, field := range map[string]string{"choir.run": "agent_id", "choir.capture_label": "capture_id", "choir.source_ref": "doc_id"} {
		query, args := store.metadataPageSQL(kind, "$."+field, "v", "", 256)
		assertIndexedPlan(t, explainPlan(t, db, query, args...), recordIndex("og_meta_"+field), true)
	}
	query, args, ok := store.addresseeSQL(string(commitmentRecordKind), "owner", "texture:d1", 12)
	if !ok {
		t.Fatal("addressee column path not taken")
	}
	assertIndexedPlan(t, explainPlan(t, db, query, args...), "og_objects.object_kind,og_objects.owner_id,og_objects.og_rec_addressee,og_objects.updated_at,og_objects.canonical_id", true)
}
