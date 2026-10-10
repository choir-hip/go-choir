package agentcore

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// requireServingVocabularyDeposits fails when any object-graph leaf on s is
// not a fixed point of the frozen vocabulary rule. A from-genesis replay
// upcasts such a leaf (and re-derives the canonical id from it) while the live
// store keeps the minted bytes, so one V1-spelled id makes the computer's
// apply checkpoint unreplayable
// (problems/selfdev-apply-checkpoint-starved-by-resumed-work-2026-10-10.md,
// rerun 8). Failure modes pinned: a lifecycle command or event id minted with
// the V1 infix (co-super-cancel:, co-super-capsule:, co-super-report:,
// co-super-restart-cancel:); an attestation or fate digest ref minted with it;
// a seed fixture carrying it, which would mask the first two.
func requireServingVocabularyDeposits(t *testing.T, s *store.Store) {
	t.Helper()
	rows, err := s.DB().QueryContext(context.Background(), `SELECT canonical_id, object_kind, body, metadata FROM og_objects`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	scanned := 0
	for rows.Next() {
		var id, kind, metadata string
		var body []byte
		if err := rows.Scan(&id, &kind, &body, &metadata); err != nil {
			t.Fatal(err)
		}
		scanned++
		for _, raw := range [][]byte{body, []byte(metadata)} {
			var decoded any
			if len(raw) == 0 || json.Unmarshal(raw, &decoded) != nil {
				continue
			}
			walkStringLeaves(decoded, func(v string) {
				if !store.IsServingVocabularyLeaf(v) {
					t.Errorf("%s %s: leaf %q is not serving vocabulary; replay would rewrite it", kind, id, v)
				}
			})
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if scanned == 0 {
		t.Fatal("no object-graph rows scanned")
	}
}

func walkStringLeaves(node any, fn func(string)) {
	switch v := node.(type) {
	case string:
		fn(v)
	case map[string]any:
		for _, child := range v {
			walkStringLeaves(child, fn)
		}
	case []any:
		for _, child := range v {
			walkStringLeaves(child, fn)
		}
	}
}
