package runsheet

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The deck's journal becomes ops on the phone (mobile/AutoradioLab/deck-ops.ts). Each fixture in mobile/fixtures/deck carries a journal,
// the ops the phone must build from it and what the sheet must look like when the reducer applies them. This test holds the server's reducer
// to the ops the fixtures carry: an op it would refuse, or one that leaves an entry other than as the fixture says, is the silent failure
// (a verdict lost, or a verdict counted for sound nobody heard). The TypeScript test builds the ops from the journal and compares.
const deckFixtureDir = "../../mobile/fixtures/deck"

type deckFixture struct {
	Name    string            `json:"name"`
	Start   RunSheet          `json:"start"`
	Ops     []json.RawMessage `json:"ops"`
	Want    []deckWant        `json:"want"`
	Dropped map[string]string `json:"dropped"`
}

type deckWant struct {
	ID        string `json:"id"`
	State     State  `json:"state"`
	Outcome   string `json:"outcome"`
	At        string `json:"at"`
	LastBlock int    `json:"last_block"`
}

func TestDeckFixtures(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(deckFixtureDir, "*.json"))
	if err != nil || len(files) < 10 {
		t.Fatalf("deck fixtures: %d files, err %v", len(files), err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(filepath.Base(f), func(t *testing.T) {
			var fx deckFixture
			if err := json.Unmarshal(raw, &fx); err != nil {
				t.Fatalf("decode fixture: %v", err)
			}
			var ops []Op
			for _, r := range fx.Ops {
				var op Op
				if err := json.Unmarshal(r, &op); err != nil {
					t.Fatalf("decode op: %v", err)
				}
				if !ValidID(op.OpID) {
					t.Fatalf("op_id %q is not a valid id", op.OpID)
				}
				ops = append(ops, op)
			}
			got, results, err := ApplyAll(fx.Start, ops)
			if err != nil {
				t.Fatal(err)
			}
			for i, r := range results {
				why := fx.Dropped[r.OpID]
				if r.Dropped != why {
					t.Errorf("op %d (%s): dropped %q, want %q", i, r.OpID, r.Dropped, why)
				}
			}
			for _, w := range fx.Want {
				var e *Entry
				for i := range got.Entries {
					if got.Entries[i].ID == w.ID {
						e = &got.Entries[i]
					}
				}
				if e == nil {
					t.Fatalf("entry %q is gone", w.ID)
				}
				have := deckWant{ID: e.ID, State: e.State}
				if e.Ended != nil {
					have.Outcome, have.At, have.LastBlock = string(e.Ended.Outcome), e.Ended.At, e.Ended.LastBlock
				}
				if !reflect.DeepEqual(have, w) {
					t.Errorf("entry %s: got %+v, want %+v", w.ID, have, w)
				}
			}
			// A resend (a post that was answered but whose answer was lost) changes nothing and settles nothing twice.
			again, results2, err := ApplyAll(got, ops)
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range results2 {
				if r.Applied {
					t.Errorf("op %s applied twice", r.OpID)
				}
			}
			if !reflect.DeepEqual(canon(t, again.Entries), canon(t, got.Entries)) {
				t.Error("a resend changed the sheet")
			}
		})
	}
}
