package release

import (
	"errors"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Failure modes from docs/archive/reports/p6-p8-release-adopt-2026-10-15.md pinned
// here: R1 (a record whose id is not its canonical digest), R6 (reserved
// fields, executable bytes), A1 (a tampered record with its id kept), A4
// (patch bytes that do not hash to the record).

func validRecord(patch []byte) Record {
	return Record{
		Schema:    Schema,
		Publisher: Publisher{ComputerID: "computer-a", OwnerID: "owner-a"},
		Operation: Operation{
			OperationID: "op-1", BundleDigest: strings.Repeat("b", 64), ReleaseDigest: strings.Repeat("c", 64),
			CodeRef: "code:sha256:" + strings.Repeat("d", 64), DecisionEvent: strings.Repeat("e", 64),
			VerifierRefs: []string{"verifier:x"},
		},
		Source: Source{
			BaseCommit: strings.Repeat("1", 40), PatchSHA256: computerevent.DigestBytes(patch), PatchBytes: len(patch),
			TreeHash: strings.Repeat("2", 40), ChangedPaths: []string{"frontend/public/x.txt"},
		},
		Precommitments: []Precommitment{{RecordID: "pc-1", Kind: "tests_pass", Question: "tests pass?", PYesPermille: 900, Settled: "yes", ResolveRecordID: "pc-1:resolve:rule"}},
	}
}

func TestReleaseIDIsTheCanonicalDigestAndIgnoresNilVersusEmpty(t *testing.T) {
	patch := []byte("diff --git a/x b/x\n")
	a := validRecord(patch)
	b := validRecord(patch)
	b.Declarations.Packages = []string{}
	b.Operation.VerifierRefs = append([]string(nil), a.Operation.VerifierRefs...)
	idA, err := a.ReleaseID()
	if err != nil {
		t.Fatal(err)
	}
	idB, err := b.ReleaseID()
	if err != nil {
		t.Fatal(err)
	}
	if idA != idB || !computerevent.IsSHA256(idA) {
		t.Fatalf("ids differ or malformed: %s %s", idA, idB)
	}
	raw, err := a.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if computerevent.DigestBytes(raw) != idA {
		t.Fatal("release id is not the digest of the canonical bytes")
	}
	decoded, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyID(decoded, idA); err != nil {
		t.Fatalf("round trip: %v", err)
	}
}

func TestVerifyIDRefusesEveryTamperedField(t *testing.T) {
	patch := []byte("diff --git a/x b/x\n")
	base := validRecord(patch)
	id, err := base.ReleaseID()
	if err != nil {
		t.Fatal(err)
	}
	tampers := map[string]func(*Record){
		"patch sha":     func(r *Record) { r.Source.PatchSHA256 = strings.Repeat("f", 64) },
		"base commit":   func(r *Record) { r.Source.BaseCommit = strings.Repeat("3", 40) },
		"publisher":     func(r *Record) { r.Publisher.ComputerID = "computer-z" },
		"test command":  func(r *Record) { r.Declarations.TestCommand = "true" },
		"precommitment": func(r *Record) { r.Precommitments[0].Settled = "no" },
		"host build": func(r *Record) {
			r.HostBuild = &HostBuild{BuilderReceiptDigest: strings.Repeat("4", 64), ClosureDigest: strings.Repeat("5", 64), CodeCommit: strings.Repeat("6", 40)}
		},
	}
	for name, tamper := range tampers {
		t.Run(name, func(t *testing.T) {
			r := validRecord(patch)
			tamper(&r)
			if err := VerifyID(r, id); !errors.Is(err, ErrDigestMismatch) {
				t.Fatalf("tampered %s with the id kept: err = %v, want ErrDigestMismatch", name, err)
			}
		})
	}
	if err := VerifyID(base, strings.ToUpper(id[:1])+id[1:]); err != nil {
		t.Fatalf("an upper-cased id names the same release: %v", err)
	}
}

func TestValidateRefusesReservedAndMissingBindings(t *testing.T) {
	patch := []byte("p")
	cases := map[string]func(*Record){
		"packages reserved":   func(r *Record) { r.Declarations.Packages = []string{"slides"} },
		"schema":              func(r *Record) { r.Schema = "other" },
		"no publisher":        func(r *Record) { r.Publisher.OwnerID = "" },
		"no bundle":           func(r *Record) { r.Operation.BundleDigest = "" },
		"bad base commit":     func(r *Record) { r.Source.BaseCommit = "main" },
		"no tree":             func(r *Record) { r.Source.TreeHash = "" },
		"patch too large":     func(r *Record) { r.Source.PatchBytes = MaxPatchBytes + 1 },
		"no changed paths":    func(r *Record) { r.Source.ChangedPaths = nil },
		"escaping path":       func(r *Record) { r.Source.ChangedPaths = []string{"../etc/passwd"} },
		"absolute path":       func(r *Record) { r.Source.ChangedPaths = []string{"/etc/passwd"} },
		"multi-line test":     func(r *Record) { r.Declarations.TestCommand = "true\nrm -rf /" },
		"host build no bytes": func(r *Record) { r.HostBuild = &HostBuild{ClosureDigest: "not-a-digest"} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := validRecord(patch)
			mutate(&r)
			if err := r.Validate(); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Validate = %v, want ErrInvalid", err)
			}
		})
	}
	if err := validRecord(patch).Validate(); err != nil {
		t.Fatalf("valid record refused: %v", err)
	}
}

func TestVerifyPatchRefusesOtherBytesAndOversize(t *testing.T) {
	patch := []byte("diff --git a/x b/x\n+one\n")
	r := validRecord(patch)
	if err := VerifyPatch(r, patch); err != nil {
		t.Fatalf("matching patch refused: %v", err)
	}
	if err := VerifyPatch(r, []byte("diff --git a/x b/x\n+two\n")); !errors.Is(err, ErrPatchMismatch) {
		t.Fatalf("other bytes: %v", err)
	}
	if err := VerifyPatch(r, make([]byte, MaxPatchBytes+1)); !errors.Is(err, ErrPatchTooLarge) {
		t.Fatalf("oversize: %v", err)
	}
}

func TestDecodeRefusesUnknownFieldsAndTrailingContent(t *testing.T) {
	raw, err := validRecord([]byte("p")).Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(append(append([]byte(nil), raw...), []byte(" {}")...)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("trailing content: %v", err)
	}
	withExtra := strings.Replace(string(raw), `{"declarations"`, `{"binary_base64":"AAAA","declarations"`, 1)
	if _, err := Decode([]byte(withExtra)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("an unknown field (executable bytes smuggled in) must be refused: %v", err)
	}
}
