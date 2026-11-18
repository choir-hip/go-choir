package runsheet

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const fixtureDir = "../../mobile/fixtures/runsheet"

type fixture struct {
	Name        string            `json:"name"`
	Start       RunSheet          `json:"start"`
	Ops         []json.RawMessage `json:"ops"`
	Want        RunSheet          `json:"want"`
	Results     []Result          `json:"results"`
	DecodeError bool              `json:"decode_error"`
}

func canon(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestFixtures runs every shared fixture. The Swift tests read the same files.
func TestFixtures(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(fixtureDir, "*.json"))
	if err != nil || len(files) < 20 {
		t.Fatalf("fixtures: %d files, err %v", len(files), err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(filepath.Base(f), func(t *testing.T) {
			var fx fixture
			if err := json.Unmarshal(raw, &fx); err != nil {
				t.Fatalf("decode fixture: %v", err)
			}
			var ops []Op
			var decodeErr error
			for _, r := range fx.Ops {
				var op Op
				if err := json.Unmarshal(r, &op); err != nil {
					decodeErr = err
					break
				}
				ops = append(ops, op)
			}
			if fx.DecodeError {
				if decodeErr == nil {
					t.Fatal("expected a decode error, got none")
				}
				return
			}
			if decodeErr != nil {
				t.Fatalf("decode ops: %v", decodeErr)
			}
			got, results, err := ApplyAll(fx.Start, ops)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(canon(t, got), canon(t, fx.Want)) {
				g, _ := json.MarshalIndent(got, "", " ")
				w, _ := json.MarshalIndent(fx.Want, "", " ")
				t.Fatalf("sheet mismatch\n got: %s\nwant: %s", g, w)
			}
			if !reflect.DeepEqual(canon(t, results), canon(t, fx.Results)) {
				t.Fatalf("results mismatch\n got: %v\nwant: %v", canon(t, results), canon(t, fx.Results))
			}
		})
	}
}

func entry(id string, state State) Entry {
	return Entry{ID: id, Kind: KindStory, PlacedBy: ByDesk, State: state, Title: id, Text: "words", EstSeconds: 10}
}

func TestApplyDoesNotMutateInput(t *testing.T) {
	s := RunSheet{Version: 1, Entries: []Entry{entry("a", StateAhead), entry("b", StateAhead)}}
	before := canon(t, s)
	e := entry("x", StateAhead)
	if _, r, _ := Apply(s, Op{OpID: "o1", By: ByListener, Op: OpPlace, At: AtNext, Entry: &e}); !r.Applied {
		t.Fatalf("not applied: %+v", r)
	}
	if _, r, _ := Apply(s, Op{OpID: "o2", By: ByListener, Op: OpClear, BaseVersion: 1}); !r.Applied {
		t.Fatalf("not applied: %+v", r)
	}
	if !reflect.DeepEqual(before, canon(t, s)) {
		t.Fatal("input sheet was modified")
	}
}

func TestUnknownOpIsAnError(t *testing.T) {
	var op Op
	if err := json.Unmarshal([]byte(`{"op_id":"o1","by":"phone","op":"teleport","base_version":0}`), &op); err == nil {
		t.Fatal("decode accepted an unknown op")
	}
	if _, _, err := Apply(RunSheet{}, Op{OpID: "o1", By: ByPhone, Op: "teleport"}); err == nil {
		t.Fatal("Apply accepted an unknown op")
	}
}

func TestAppliedOpsAreBounded(t *testing.T) {
	s := RunSheet{Entries: []Entry{}}
	for i := 0; i < MaxAppliedOps+10; i++ {
		var r Result
		s, r, _ = Apply(s, Op{OpID: "o" + strconv.Itoa(i), By: ByListener, Op: OpClear, BaseVersion: s.Version})
		if !r.Applied {
			t.Fatalf("op %d: %+v", i, r)
		}
	}
	if len(s.AppliedOps) != MaxAppliedOps || s.Version != int64(MaxAppliedOps+10) {
		t.Fatalf("applied ops %d version %d", len(s.AppliedOps), s.Version)
	}
	if _, r, _ := Apply(s, Op{OpID: "o" + strconv.Itoa(MaxAppliedOps+9), By: ByListener, Op: OpClear, BaseVersion: s.Version}); r.Dropped != DroppedDuplicate {
		t.Fatalf("recent op_id not remembered: %+v", r)
	}
}

func TestValidID(t *testing.T) {
	for _, ok := range []string{"a", "p-1a2b3c4d", "d-turn_7.2-3", strings.Repeat("a", 64)} {
		if !ValidID(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "-a", ".a", "a b", "a/b", strings.Repeat("a", 65), "é"} {
		if ValidID(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}

// A take is an entry kind: words to speak (the DJ's lead-in), a forecast like a
// story's, and the publisher's audio named by its source ref (design 11.5 of
// docs/design/autoradio-learned-document-2026-11-17.md); the Swift twin is
// RunSheetKind.take.
func TestTakeIsAnEntryKind(t *testing.T) {
	var e Entry
	raw := `{"id":"e1","kind":"take","placed_by":"desk","state":"ahead","title":"A voice","text":"A listener recorded this.","est_seconds":20,
		"source":{"ref":"take:take-0123456789abcdef","label":"voice-3fa2c1"},"forecast":{"p_heard":0.6,"caused_by":"explore","claim_id":"c1"}}`
	if err := json.Unmarshal([]byte(raw), &e); err != nil || e.Kind != KindTake || !e.Kind.TextRequired() {
		t.Fatalf("take entry: %+v %v", e, err)
	}
	if err := json.Unmarshal([]byte(`{"kind":"takes"}`), &e); err == nil {
		t.Fatal("an unknown kind decoded")
	}
}
