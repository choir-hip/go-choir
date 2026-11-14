package selfdevprotocol

// Witness version 4 (canonical event index). Ways this could fail:
//
//  1. the form or the boundary is not inside what the checkpoint commits to,
//     so either can be swapped after the fact;
//  2. a version 3 witness changes its digest (old checkpoints stop verifying);
//  3. a version 3 witness carries a boundary, or a v3 and a v4 witness are
//     compared as if they were one form;
//  4. a release that reads only version 3 would accept a version 4
//     checkpoint (the reader here is this release: it must refuse any
//     version it does not know, and name ErrWitnessVersionRefused).

import (
	"errors"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

func testCanonicalDigests() computerversion.StateDigests {
	return computerversion.StateDigests{
		Database: "texture", SchemaHash: strings.Repeat("a", 64),
		Tables: map[string]string{"computer_event_index": strings.Repeat("b", 64)},
	}
}

func testWitnessCanonical(t *testing.T, boundary uint64) VMLocalContentWitness {
	t.Helper()
	w, err := WitnessFromDigestsCanonical(testCanonicalDigests(), boundary)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestCanonicalWitnessFormAndBoundaryAreCommitted(t *testing.T) {
	w := testWitnessCanonical(t, 40000)
	if w.WitnessVersion != WitnessVersionCanonical || w.EventIndexTrimBoundary != 40000 {
		t.Fatalf("witness = %+v", w)
	}
	if err := w.Validate(); err != nil {
		t.Fatal(err)
	}
	other := testWitnessCanonical(t, 30000)
	if other.DerivabilityDigest == w.DerivabilityDigest {
		t.Fatal("the boundary is not inside the derivability digest")
	}
	swapped := w
	swapped.EventIndexTrimBoundary = 30000
	if err := swapped.Validate(); err == nil {
		t.Fatal("a boundary swapped after the fact validates")
	}
	// The form is committed too: a v3 witness over the same digests is a
	// different digest, and cannot be passed off as version 4 or the reverse.
	strict, err := WitnessFromDigests(testCanonicalDigests())
	if err != nil {
		t.Fatal(err)
	}
	if strict.DerivabilityDigest == testWitnessCanonical(t, 0).DerivabilityDigest {
		t.Fatal("the strict and canonical forms share a derivability digest")
	}
	relabelled := strict
	relabelled.WitnessVersion = WitnessVersionCanonical
	if err := relabelled.Validate(); err == nil {
		t.Fatal("a strict witness relabelled as canonical validates")
	}
	relabelled = w
	relabelled.WitnessVersion = WitnessVersionSQLite
	if err := relabelled.Validate(); err == nil {
		t.Fatal("a canonical witness relabelled as strict validates")
	}
}

func TestCanonicalWitnessIsPartOfTheCheckpointDigest(t *testing.T) {
	// The checkpoint digest is over the request, witness included.
	a := testWitnessCanonical(t, 40000)
	b := testWitnessCanonical(t, 30000)
	ja, _ := computerevent.CanonicalJSON(a)
	jb, _ := computerevent.CanonicalJSON(b)
	if string(ja) == string(jb) {
		t.Fatal("the boundary is not in the witness's canonical JSON")
	}
	if !strings.Contains(string(ja), `"event_index_trim_boundary":40000`) {
		t.Fatalf("canonical JSON = %s", ja)
	}
	strict, _ := WitnessFromDigests(testCanonicalDigests())
	js, _ := computerevent.CanonicalJSON(strict)
	if strings.Contains(string(js), "event_index_trim_boundary") {
		t.Fatalf("a version 3 witness gained a field: %s", js)
	}
}

// Version 3's digest is the one every recorded checkpoint carries.
func TestStrictWitnessDigestIsUnchanged(t *testing.T) {
	w, err := NewWitnessV3("texture", strings.Repeat("a", 64), map[string]string{"agents": strings.Repeat("b", 64)})
	if err != nil {
		t.Fatal(err)
	}
	proof, err := computerevent.CanonicalJSON(struct {
		Version     int               `json:"witness_version"`
		Database    string            `json:"database"`
		ContentRoot string            `json:"content_root"`
		SchemaHash  string            `json:"schema_hash"`
		Tables      map[string]string `json:"tables"`
	}{3, w.Database, w.ContentRoot, w.SchemaHash, w.Tables})
	if err != nil {
		t.Fatal(err)
	}
	if computerevent.DigestBytes(proof) != w.DerivabilityDigest {
		t.Fatal("the strict form's derivability digest changed")
	}
}

func TestStrictWitnessWithABoundaryIsRefused(t *testing.T) {
	w := testWitnessV3("texture")
	w.EventIndexTrimBoundary = 40000
	if err := w.Validate(); err == nil {
		t.Fatal("a version 3 witness with a trim boundary validates")
	}
}

func TestWitnessContentMatchesComparesFormsAndNamesTheRefusal(t *testing.T) {
	canonical := testWitnessCanonical(t, 40000)
	if err := WitnessContentMatches(canonical, canonical); err != nil {
		t.Fatal(err)
	}
	strict, _ := WitnessFromDigests(testCanonicalDigests())
	for name, pair := range map[string][2]VMLocalContentWitness{
		"strict against canonical":       {strict, canonical},
		"canonical against strict":       {canonical, strict},
		"another boundary":               {testWitnessCanonical(t, 30000), canonical},
		"another table digest, same form": {func() VMLocalContentWitness {
			d := testCanonicalDigests()
			d.Tables = map[string]string{"computer_event_index": strings.Repeat("c", 64)}
			w, _ := WitnessFromDigestsCanonical(d, 40000)
			return w
		}(), canonical},
	} {
		err := WitnessContentMatches(pair[0], pair[1])
		if !errors.Is(err, ErrWitnessMismatch) {
			t.Fatalf("%s: err = %v, want ErrWitnessMismatch", name, err)
		}
	}
}

func TestUnknownWitnessVersionsAreRefusedWithTheirSentinel(t *testing.T) {
	for _, v := range []int{0, 1, 2, 5} {
		if err := RefuseWitnessVersion(v); !errors.Is(err, ErrWitnessVersionRefused) {
			t.Fatalf("version %d: %v", v, err)
		}
	}
	for _, v := range []int{3, 4} {
		if err := RefuseWitnessVersion(v); err != nil {
			t.Fatalf("version %d: %v", v, err)
		}
	}
}
