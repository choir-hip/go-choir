package projectionbase

import (
	"errors"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

func validTestDescriptor() Descriptor {
	sha := func(c string) string { return strings.Repeat(c, 64) }
	return Descriptor{
		ComputerID:            "computer-base-test",
		Sequence:              10,
		CanonicalHead:         sha("a"),
		BlobSHA256:            sha("b"),
		BlobSizeBytes:         1024,
		ReducerVersion:        computerevent.ReducerVersionV1,
		SchemaVersion:         computerevent.SchemaVersionV1,
		VocabularyVersion:     CurrentVocabularyVersion,
		VMLocalContentWitness: validTestWitness(),
	}
}

func validTestWitness() selfdevprotocol.VMLocalContentWitness {
	sha := func(c string) string { return strings.Repeat(c, 64) }
	return selfdevprotocol.VMLocalContentWitness{
		Database:           "texture",
		ContentRoot:        sha("c"),
		Schema:             map[string]string{"t": sha("d")},
		Tables:             map[string]string{"t": sha("e")},
		DoltHead:           sha("f"),
		DerivabilityDigest: sha("0"),
	}
}

func TestDescriptorBindsEveryField(t *testing.T) {
	base := validTestDescriptor()
	if err := base.Validate(); err != nil {
		t.Fatalf("valid descriptor refused: %v", err)
	}
	sha := func(c string) string { return strings.Repeat(c, 64) }
	cases := map[string]func(*Descriptor){
		"empty computer":     func(d *Descriptor) { d.ComputerID = "" },
		"zero sequence":      func(d *Descriptor) { d.Sequence = 0 },
		"empty head":         func(d *Descriptor) { d.CanonicalHead = "" },
		"non-sha head":       func(d *Descriptor) { d.CanonicalHead = "not-a-digest" },
		"empty blob":         func(d *Descriptor) { d.BlobSHA256 = "" },
		"uppercase blob":     func(d *Descriptor) { d.BlobSHA256 = strings.ToUpper(sha("b")) },
		"zero blob size":     func(d *Descriptor) { d.BlobSizeBytes = 0 },
		"negative blob size": func(d *Descriptor) { d.BlobSizeBytes = -1 },
		"zero reducer":       func(d *Descriptor) { d.ReducerVersion = 0 },
		"zero schema":        func(d *Descriptor) { d.SchemaVersion = 0 },
		"missing vocabulary": func(d *Descriptor) { d.VocabularyVersion = "" },
		"unknown vocabulary": func(d *Descriptor) { d.VocabularyVersion = "v3" },
		"empty witness":      func(d *Descriptor) { d.VMLocalContentWitness = selfdevprotocol.VMLocalContentWitness{} },
	}
	for name, mutate := range cases {
		d := validTestDescriptor()
		mutate(&d)
		if err := d.Validate(); err == nil {
			t.Errorf("%s: mutated descriptor was accepted", name)
		}
		if err := d.VerifyForRecovery("computer-base-test", sha("9"), 12); !errors.Is(err, ErrBaseRefused) {
			t.Errorf("%s: expected ErrBaseRefused, got %v", name, err)
		}
	}
	// Well-formed but incompatible with this live build: integrity passes,
	// recovery refuses. No live alias or fallback exists.
	compat := map[string]func(*Descriptor){
		"future reducer": func(d *Descriptor) { d.ReducerVersion = computerevent.ReducerVersionV1 + 1 },
		"future schema":  func(d *Descriptor) { d.SchemaVersion = computerevent.SchemaVersionV1 + 1 },
	}
	for name, mutate := range compat {
		d := validTestDescriptor()
		mutate(&d)
		if err := d.Validate(); err != nil {
			t.Errorf("%s: well-formed descriptor refused at integrity: %v", name, err)
		}
		if err := d.VerifyForRecovery("computer-base-test", sha("9"), 12); !errors.Is(err, ErrBaseRefused) {
			t.Errorf("%s: expected ErrBaseRefused, got %v", name, err)
		}
	}
	// Out-of-scope witness databases refuse even when every digest is valid.
	scoped := validTestDescriptor()
	w := scoped.VMLocalContentWitness
	w.Database = "platform"
	scoped.VMLocalContentWitness = w
	if err := scoped.Validate(); err == nil {
		t.Errorf("foreign witness db: out-of-scope database was accepted")
	}
}

// TestKnownVocabularySetIsV1V2 pins the mission-2 never-reverted widening:
// {v1, v2} exactly. A v1-stamped base stays installable after cutover;
// anything outside the set fails closed.
func TestKnownVocabularySetIsV1V2(t *testing.T) {
	for _, v := range []string{"v1", "v2", " v1 ", " v2 "} {
		if !IsKnownVocabularyVersion(v) {
			t.Errorf("IsKnownVocabularyVersion(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"", "v3", "V1", "v12", "legacy"} {
		if IsKnownVocabularyVersion(v) {
			t.Errorf("IsKnownVocabularyVersion(%q) = true, want false", v)
		}
	}
}

func TestVerifyForRecoveryBindsComputerAndOrder(t *testing.T) {
	sha := func(c string) string { return strings.Repeat(c, 64) }
	base := validTestDescriptor()

	if err := base.VerifyForRecovery("computer-base-test", sha("9"), 12); err != nil {
		t.Fatalf("valid recovery claim refused: %v", err)
	}
	// Degenerate zero-length tail W=H is admitted: install still verifies.
	if err := base.VerifyForRecovery("computer-base-test", sha("a"), 10); err != nil {
		t.Fatalf("W=H claim refused: %v", err)
	}

	type claim struct {
		computer, head string
		seq            uint64
	}
	cases := map[string]claim{
		"foreign computer":  {"computer-other", sha("9"), 12},
		"empty computer":    {"", sha("9"), 12},
		"malformed target":  {"computer-base-test", "not-a-digest", 12},
		"zero target seq":   {"computer-base-test", sha("9"), 0},
		"base after target": {"computer-base-test", sha("9"), 9},
		"same seq new head": {"computer-base-test", sha("9"), 10},
	}
	for name, tc := range cases {
		if err := base.VerifyForRecovery(tc.computer, tc.head, tc.seq); !errors.Is(err, ErrBaseRefused) {
			t.Errorf("%s: expected ErrBaseRefused, got %v", name, err)
		}
	}
}

func TestVerifyTailHeadProvesAncestry(t *testing.T) {
	base := validTestDescriptor()
	sha := func(c string) string { return strings.Repeat(c, 64) }
	first := computerevent.Event{
		ComputerID:   "computer-base-test",
		Sequence:     11,
		PreviousHead: sha("a"),
	}
	if err := base.VerifyTailHead(first); err != nil {
		t.Fatalf("true tail start refused: %v", err)
	}

	nonAncestor := first
	nonAncestor.PreviousHead = sha("9")
	if err := base.VerifyTailHead(nonAncestor); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("non-ancestor W admitted: %v", err)
	}
	gap := first
	gap.Sequence = 12
	if err := base.VerifyTailHead(gap); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("gapped tail admitted: %v", err)
	}
	rewind := first
	rewind.Sequence = 10
	if err := base.VerifyTailHead(rewind); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("prefix event admitted as tail: %v", err)
	}
	foreign := first
	foreign.ComputerID = "computer-other"
	if err := base.VerifyTailHead(foreign); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("foreign computer tail admitted: %v", err)
	}
}
