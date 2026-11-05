package objectgraph

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/kernelservice"
)

// Ways custody could fail (written before the code, per the testing rule):
//
//  1. A kind keeps the retired placeholder writer, so "the runtime" is still
//     the custodian of something no desk or service declared.
//  2. A kind is claimed by two manifests, or twice by one, and the later claim
//     silently wins.
//  3. A kind is claimed by none and keeps the zero writer, which no write can
//     match.
//  4. A manifest claims a name the registry does not hold (a logical name, a
//     typo), so its custody is declared but never reaches a write.
//  5. The load reports only the first problem, and each fix reveals the next.
//  6. A service writer constant drifts from its registration's name, so the
//     gate admits a writer the manifest never derived.
//  7. The drill passes because the writer is derived from the same table it is
//     checked against: the drill reads the claims independently.

// retiredPlaceholderWriter is the writer the registry named for every kind no
// desk had been assigned (MA3.1 deleted it).
const retiredPlaceholderWriter Writer = "system:runtime"

func TestNoKindHasThePlaceholderAndEveryKindHasExactlyOneCustodian(t *testing.T) {
	claimed := map[string][]Writer{}
	for _, c := range ManifestClaims() {
		for _, k := range c.Kinds {
			claimed[k] = append(claimed[k], c.Custodian)
		}
	}
	for _, reg := range DefaultRegistry().AllKinds() {
		if reg.Writer == retiredPlaceholderWriter {
			t.Errorf("kind %s still names the placeholder writer", reg.Kind)
		}
		who := claimed[string(reg.Kind)]
		if len(who) != 1 {
			t.Errorf("kind %s has %d custodians %v, want exactly 1", reg.Kind, len(who), who)
			continue
		}
		if reg.Writer != who[0] {
			t.Errorf("kind %s: registry writer %q is not its custodian %q", reg.Kind, reg.Writer, who[0])
		}
	}
	for k := range claimed {
		if _, err := DefaultRegistry().LookupKind(ObjectKind(k)); err != nil {
			t.Errorf("a manifest claims %q, which the registry does not hold", k)
		}
	}
	if BuiltinWriters[retiredPlaceholderWriter] {
		t.Error("the placeholder is still a built-in writer")
	}
}

func TestDeriveCustodyReportsEveryProblemAtOnce(t *testing.T) {
	kinds := []KindRegistration{{Kind: "choir.a"}, {Kind: "choir.b"}, {Kind: "choir.c"}, {Kind: "choir.d"}}
	claims := []Claim{
		{Custodian: "desk:one", Source: "one", Kinds: []string{"choir.a", "choir.b", "logical_name"}},
		{Custodian: WriterCorpusd, Source: "two", Kinds: []string{"choir.b", "choir.d", "choir.d"}},
		{Custodian: "owner", Source: "three", Kinds: nil},
		{Custodian: "nonsense", Source: "four", Kinds: []string{"choir.a"}},
	}
	_, err := DeriveCustody(kinds, claims)
	if !errors.Is(err, ErrCustody) {
		t.Fatalf("err = %v, want ErrCustody", err)
	}
	for _, want := range []string{
		"unclaimed: choir.c",
		"double-claimed: choir.b by one, two",
		"double-claimed: choir.d by two, two",
		`one claims "logical_name"`,
		`four derives writer "nonsense"`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not report %q:\n%v", want, err)
		}
	}
	if strings.Contains(err.Error(), "unclaimed: choir.a") {
		t.Errorf("choir.a is claimed once by a valid writer, but reported unclaimed:\n%v", err)
	}
}

func TestDeriveCustodySetsTheWriterFromTheClaim(t *testing.T) {
	out, err := DeriveCustody([]KindRegistration{{Kind: "choir.a", OwnerEdits: true}}, []Claim{{Custodian: "desk:one", Source: "one", Kinds: []string{"choir.a"}}})
	if err != nil || len(out) != 1 || out[0].Writer != "desk:one" || !out[0].OwnerEdits {
		t.Fatalf("out = %+v, err = %v", out, err)
	}
}

// The writer constants the gate and the stores declare are the ones the service
// registrations derive.
func TestServiceWriterConstantsAreDerivedFromServiceNames(t *testing.T) {
	want := map[string]Writer{
		kernelservice.Kernel: WriterKernel, kernelservice.Corpusd: WriterCorpusd, kernelservice.Store: WriterStore,
		kernelservice.OwnerFiles: WriterOwnerFiles, kernelservice.Shell: WriterShell, kernelservice.Autoradio: WriterAutoradio, kernelservice.Lease: WriterLease,
	}
	for _, m := range kernelservice.All() {
		w, ok := want[m.Name]
		if !ok {
			t.Errorf("service %s has no writer constant", m.Name)
			continue
		}
		if derived := Writer("system:" + m.Name); derived != w {
			t.Errorf("service %s derives %q, the constant is %q", m.Name, derived, w)
		}
		if !BuiltinWriters[w] {
			t.Errorf("service writer %q is not a built-in writer", w)
		}
	}
	if len(want) != len(kernelservice.All()) {
		t.Errorf("%d writer constants for %d services", len(want), len(kernelservice.All()))
	}
}

// The drill: every kind is written from its custodian without a count, and from
// another writer it is counted by the gate and allowed (observe; MA3.2 flips
// enforcement). The commons is the exception that stays until MA3.2 declares the
// writer at the reducer; the drill proves nothing about it (a wrong-writer
// commons write cannot be counted yet, so it is not coverage): a kernel kind is written on behalf of any desk in a
// mixed batch, so it admits every declared writer.
func TestDrillEveryKindFromItsCustodianAndFromAnotherDesk(t *testing.T) {
	writerMode(t, WriterGateObserve)
	others := []Writer{WriterTexture, WriterManagement, WriterEngineering, WriterResearch, WriterCorpusd}
	for _, reg := range DefaultRegistry().AllKinds() {
		reg := reg
		t.Run(string(reg.Kind), func(t *testing.T) {
			ResetWriterGateCounts()
			base := context.Background()
			if reg.Kind == KindStreamSeqLease {
				base = WithStreamSeqLeaseCapability(base)
			}
			obj := gateObject(t, reg.Kind, "drill-"+string(reg.Kind))
			if err := CheckWriteAs(WithWriter(base, reg.Writer), []Object{obj}, nil); err != nil {
				t.Fatalf("custodian %q refused: %v", reg.Writer, err)
			}
			if n := len(WriterGateCounts()); n != 0 {
				t.Fatalf("custodian %q was counted: %v", reg.Writer, WriterGateCounts())
			}
			other := others[0]
			for _, w := range others {
				if w != reg.Writer {
					other = w
					break
				}
			}
			if err := CheckWriteAs(WithWriter(base, other), []Object{obj}, nil); err != nil {
				t.Fatalf("observe refused %q writing %s: %v", other, reg.Kind, err)
			}
			counted := WriterGateCounts()[string(reg.Kind)+"|"+string(other)+"|wrong_writer"]
			switch {
			case reg.Writer == WriterKernel:
				if counted != 0 {
					t.Fatalf("the commons kind was counted for %q: %v", other, WriterGateCounts())
				}
			case counted != 1:
				t.Fatalf("%q writing %s counted %d times, want 1 (counts: %v)", other, reg.Kind, counted, WriterGateCounts())
			}
		})
	}
}

// The desk writer constants the gate and the stores declare are the manifest
// names they derive; a drifted constant would otherwise show only as counts.
func TestDeskWriterConstantsAreTheManifestNames(t *testing.T) {
	for name, w := range map[string]Writer{
		deskmanifest.Texture: WriterTexture, deskmanifest.Management: WriterManagement,
		deskmanifest.Engineering: WriterEngineering, deskmanifest.Research: WriterResearch,
	} {
		d, ok := deskmanifest.Lookup(name)
		if !ok {
			t.Fatalf("desk %s is not registered", name)
		}
		if derived := Writer("desk:" + d.Name); derived != w {
			t.Errorf("desk %s derives %q, the constant is %q", name, derived, w)
		}
	}
}
