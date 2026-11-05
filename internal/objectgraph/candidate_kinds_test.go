package objectgraph

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Failure modes (MA4.3, B3, written first):
//
//	C1 A candidate's shadow kind cannot register: the kind grammar refuses a
//	   dot inside the kind part, so choir.app.<pkg>.candidate.<kind> is not a
//	   kind. -> the candidate is the package <pkg>-candidate; the grammar is
//	   unchanged and the dash is already legal in the package segment.
//	C2 An installed package takes the suffix (a publisher names a package
//	   "x-candidate" to shadow another's candidate, or to be mistaken for one).
//	   -> ValidPackageName refuses it, so every install path does.
//	C3 The installed package's name is so long its candidate's desk name
//	   exceeds 32 characters. -> installed names are limited to 22.
//	C4 The candidate writes the installed kind, or the installed desk writes
//	   the shadow. -> each kind has its own desk as the only writer.
//	C5 Registering a candidate through the installed path, or an installed
//	   package through the candidate path. -> each path refuses the other's names.
func TestCandidateNamespaceDoesNotChangeTheKindGrammar(t *testing.T) {
	if err := ValidPackageKindName("slides-candidate", "choir.app.slides-candidate.deck"); err != nil {
		t.Fatalf("a shadow kind is refused by the unchanged grammar: %v", err)
	}
	if err := ValidPackageKindName("slides", "choir.app.slides.candidate.deck"); err == nil {
		t.Fatal("the dotted spelling is still refused: a dot never enters the kind part")
	}
	if CandidateName("slides") != "slides-candidate" || !IsCandidateName("slides-candidate") || IsCandidateName("slides") {
		t.Fatal("candidate naming")
	}
	if base, ok := CandidateBase("slides-candidate"); !ok || base != "slides" {
		t.Fatalf("base %q %v", base, ok)
	}
}

func TestTheCandidateSuffixAndTheLengthAreReserved(t *testing.T) {
	for _, name := range []string{"slides-candidate", "x-candidate", "a-candidate-candidate"} {
		if ValidPackageName(name) {
			t.Errorf("installed package %q ends with the reserved suffix", name)
		}
	}
	long := strings.Repeat("a", 23)
	if ValidPackageName(long) {
		t.Errorf("a 23-character name leaves its candidate a 33-character desk name")
	}
	if !ValidPackageName(strings.Repeat("a", 22)) {
		t.Error("a 22-character name is the longest installed name")
	}
	if !ValidCandidateName(CandidateName(strings.Repeat("a", 22))) || len(CandidateName(strings.Repeat("a", 22))) != 32 {
		t.Error("the longest candidate must be 32 characters and valid")
	}
	if ValidCandidateName("candidate") || ValidCandidateName("texture-candidate") {
		t.Error("a candidate of nothing, or of a built-in desk, is not a candidate")
	}
	if ValidCandidateName(CandidateName("slides") + "-candidate") {
		t.Error("a candidate of a candidate")
	}
}

func TestCandidateKindsRegisterApartFromTheInstalledOnes(t *testing.T) {
	writerMode(t, WriterGateEnforce)
	const pkg = "cand4test"
	cand := CandidateName(pkg)
	installed := pkgKind(pkg, "note")
	shadow := pkgKind(cand, "note")
	if err := RegisterPackageKind(pkg, installed); err != nil {
		t.Fatal(err)
	}
	// C5: each path refuses the other's names.
	if err := RegisterPackageKind(cand, shadow); !errors.Is(err, ErrPackageKindRefused) {
		t.Errorf("the installed path took a candidate: %v", err)
	}
	if err := RegisterCandidateKind(pkg, installed); !errors.Is(err, ErrPackageKindRefused) {
		t.Errorf("the candidate path took an installed package: %v", err)
	}
	if err := RegisterCandidateKind(cand, shadow); err != nil {
		t.Fatalf("shadow kind: %v", err)
	}
	if err := RegisterCandidateKind(cand, shadow); err != nil {
		t.Fatalf("an identical re-registration is a no-op: %v", err)
	}
	// C4: each desk writes only its own kinds.
	write := func(w Writer, k ObjectKind, name string) error {
		return CheckWriteAs(WithWriter(context.Background(), w), []Object{gateObject(t, k, name)}, nil)
	}
	if err := write(Writer("desk:"+cand), shadow.Kind, "a"); err != nil {
		t.Errorf("the candidate cannot write its shadow kind: %v", err)
	}
	if err := write(Writer("desk:"+cand), installed.Kind, "b"); !errors.Is(err, ErrWrongWriter) {
		t.Errorf("the candidate wrote the installed kind: %v", err)
	}
	if err := write(Writer("desk:"+pkg), shadow.Kind, "c"); !errors.Is(err, ErrWrongWriter) {
		t.Errorf("the installed desk wrote the shadow kind: %v", err)
	}
	if err := write(Writer("desk:"+pkg), installed.Kind, "d"); err != nil {
		t.Errorf("the installed desk cannot write its kind: %v", err)
	}
	// A shadow kind may not name the installed desk as its writer.
	bad := pkgKind(cand, "other")
	bad.Writer = Writer("desk:" + pkg)
	if err := RegisterCandidateKind(cand, bad); !errors.Is(err, ErrPackageKindRefused) {
		t.Errorf("a shadow kind written by the installed desk: %v", err)
	}
}
