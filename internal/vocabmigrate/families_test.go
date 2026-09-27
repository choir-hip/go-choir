package vocabmigrate

// R5a tests: per-family frozen decoder coverage, the single V1→V2
// normalization point, the pinned identity seeds, and the pre-migration
// fold-identical proof. Coverage is enforced by scanning the production
// sources for V1-spelled literals — a literal that appears outside its
// frozen table fails the test, so the freeze cannot silently drift.

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Go string literal: escape-aware so embedded \" or \\ inside a literal
// cannot misalign the pairing and swallow the next literal.
var stringLiteralRe = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)

func sourceLiterals(t *testing.T, rel string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	out := map[string]bool{}
	for _, m := range stringLiteralRe.FindAllStringSubmatch(string(raw), -1) {
		out[m[1]] = true
	}
	return out
}

// TestFrozenFamiliesCoverSourceVocabulary scans the production sources that
// mint or carry V1-spelled durable strings and asserts every such literal is
// registered in its family's frozen table. A new V1 spelling added to source
// without a table entry fails this test — the freeze cannot silently drift.
func TestFrozenFamiliesCoverSourceVocabulary(t *testing.T) {
	// Lifecycle kinds: every types/lifecycle.go string literal that is a
	// kind value must be in its frozen table (all are enumerated; the scan
	// keys on the const-block literal values).
	lifecycleSrc, err := os.ReadFile(filepath.Join("..", "types", "lifecycle.go"))
	if err != nil {
		t.Fatalf("read lifecycle.go: %v", err)
	}
	for _, m := range regexp.MustCompile(`Lifecycle\w+\s+Lifecycle\w+\s*=\s*"([^"]+)"`).FindAllStringSubmatch(string(lifecycleSrc), -1) {
		lit := m[1]
		cmdKnown := false
		evtKnown := false
		for k := range frozenLifecycleCommandKinds {
			if string(k) == lit {
				cmdKnown = true
			}
		}
		for k := range frozenLifecycleEventKinds {
			if string(k) == lit {
				evtKnown = true
			}
		}
		if !cmdKnown && !evtKnown {
			t.Errorf("lifecycle kind literal %q absent from frozen tables", lit)
		}
	}

	// OG kinds + schema strings: every V1-spelled "choir.*" literal in the
	// durable-bearing sources must be in frozenOGObjectKinds or
	// frozenSchemaStrings. Neutral choir.* names are OG kinds and must be
	// in the OG table regardless.
	for _, rel := range []string{
		"internal/store/engineering_assignments.go",
		"internal/store/engineering_evidence.go",
		"internal/store/lifecycle.go",
		"internal/store/vocab_migrate_og.go",
		"internal/types/engineering_assignment.go",
		"internal/types/lifecycle.go",
		"cmd/choir/main.go",
	} {
		for lit := range sourceLiterals(t, rel) {
			if len(lit) < 6 || lit[:6] != "choir." {
				continue
			}
			inOG := frozenOGObjectKinds[lit]
			inSchema := frozenSchemaStrings[lit]
			if !inOG && !inSchema {
				t.Errorf("%s: durable literal %q absent from frozen tables", rel, lit)
			}
		}
	}

	// Identity seeds: any choir:* literal surviving in the seed-bearing
	// sources must be a frozen identity seed (production code now uses the
	// constants, so a surviving literal is a table miss or a new seed
	// written outside the freeze).
	for _, rel := range []string{
		"internal/agentcore/engineering_assignment_runtime.go",
		"internal/agentcore/tools_engineering_assignment.go",
	} {
		for lit := range sourceLiterals(t, rel) {
			if len(lit) >= 6 && lit[:6] == "choir:" {
				if !frozenIdentitySeeds[lit] {
					t.Errorf("%s: identity-seed literal %q absent from frozen seed table", rel, lit)
				}
			}
		}
	}

	// SQL identifiers: the V1-spelled durable identifiers frozen in the
	// manifest must be exactly the ones the schema still carries.
	for _, ident := range FrozenSQLIdentifiers {
		found := false
		for lit := range sourceLiterals(t, "internal/store/store.go") {
			if lit == ident {
				found = true
			}
		}
		if !found {
			src, _ := os.ReadFile(filepath.Join("..", "store", "store.go"))
			if !regexp.MustCompile(regexp.QuoteMeta(ident)).Match(src) {
				t.Errorf("frozen SQL identifier %q not found in store.go", ident)
			}
		}
	}
}

// TestLifecycleKindDecoders pin the routed decode semantics: every known
// kind decodes to itself byte-identically; unknown kinds report not-frozen.
func TestLifecycleKindDecoders(t *testing.T) {
	for kind := range frozenLifecycleCommandKinds {
		got, ok := DecodeLifecycleCommandKind(kind)
		if !ok || got != kind {
			t.Errorf("command kind %q did not decode identically", kind)
		}
	}
	for kind := range frozenLifecycleEventKinds {
		got, ok := DecodeLifecycleEventKind(kind)
		if !ok || got != kind {
			t.Errorf("event kind %q did not decode identically", kind)
		}
	}
	if _, ok := DecodeLifecycleCommandKind(types.LifecycleCommandKind("not_a_kind")); ok {
		t.Error("unknown command kind decoded as frozen")
	}
	if _, ok := DecodeLifecycleEventKind(types.LifecycleEventKind("co_super_assignment_forged")); ok {
		t.Error("unknown event kind decoded as frozen")
	}
}

// TestNormalizeHistoricProfile pins the single stratum-C normalization
// point: V2 live tokens canonicalize, V1 spellings join through
// ForwardV1ToV2, frozen protocol passes through, unknown fails closed.
func TestNormalizeHistoricProfile(t *testing.T) {
	cases := []struct {
		in    string
		want  string
		known bool
	}{
		{"management", "management", true},
		{"engineering", "engineering", true},
		{"engineering", "engineering", true},
		{"super", "management", true},     // V1 canonical management
		{"co-super", "engineering", true}, // V1 canonical engineering
		{"cosuper", "engineering", true},  // V1 spelling
		{"co_super", "engineering", true}, // V1 underscore spelling
		{"researcher", "research", true},  // V1 research
		{"owner", "owner", true},          // frozen protocol passthrough
		{"trusted-core", "trusted-core", true},
		{"not-a-desk", "", false}, // fail closed
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := NormalizeHistoricProfile(c.in)
		if ok != c.known || got != c.want {
			t.Errorf("NormalizeHistoricProfile(%q) = %q,%v want %q,%v", c.in, got, ok, c.want, c.known)
		}
	}
}

// TestIdentitySeedsFrozen pins keep-v3 and the neighbor seeds: the strings
// mint content-addressed IDs, so they are byte-frozen forever.
func TestIdentitySeedsFrozen(t *testing.T) {
	if IdentitySeedCoSuperAssignmentV3 != "choir:co-super-assignment:v3" {
		t.Fatalf("keep-v3 decision violated: seed = %q", IdentitySeedCoSuperAssignmentV3)
	}
	for _, seed := range []string{
		IdentitySeedCoSuperAssignmentV3,
		IdentitySeedCoSuperRequestV2,
		IdentitySeedCoSuperDecisionV3,
		IdentitySeedDelegatedCastRequestV1,
		IdentitySeedDelegatedDecisionV1,
		IdentitySeedPersistentSuperReportV1,
	} {
		if !IsFrozenIdentitySeed(seed) {
			t.Errorf("seed %q absent from frozen table", seed)
		}
	}
	if IsFrozenIdentitySeed("choir:co-super-assignment:v4") {
		t.Error("v4 seed registered — a rename would re-mint replayed IDs")
	}
}

// TestPreMigrationFixtureFoldsIdentically is the station's proof item: a
// fixture of pre-migration durable bytes carrying V1 spellings across the
// families folds to the same semantic values whether read through the
// frozen decoders or read raw — because the frozen vocabulary IS the
// pre-migration spelling, byte-identical by construction.
func TestPreMigrationFixtureFoldsIdentically(t *testing.T) {
	// Fixture: V1-spelled durable values as a pre-migration tape carries
	// them — every family, every member of the V1-bearing subset.
	fixture := struct {
		commandKinds []types.LifecycleCommandKind
		eventKinds   []types.LifecycleEventKind
		ogKinds      []string
		schemas      []string
		profiles     []string
		seeds        []string
	}{
		commandKinds: []types.LifecycleCommandKind{
			"open_co_super_assignment", "bind_co_super_assignment",
			"record_co_super_assignment", "cancel_co_super_assignment",
			"set_co_super_capsule_disposition",
			"start", "open_work", "settle_work", "terminalize_run",
		},
		eventKinds: []types.LifecycleEventKind{
			"co_super_assignment_opened", "co_super_assignment_bound",
			"co_super_assignment_reported", "co_super_assignment_cancelled",
			"co_super_capsule_disposition_set",
			"trajectory_started", "work_opened", "update_applied",
		},
		ogKinds: []string{
			"choir.co_super_assignment", "choir.co_super_assignment_report",
			"choir.co_super_subject_candidate", "choir.co_super_capability_claim",
			"choir.co_super_capsule_claim", "choir.co_super_run_claim",
			"choir.lifecycle_event", "choir.texture_revision",
		},
		schemas: []string{
			"choir.co_super_assignment/v1", "choir.co_super_capsule_evidence/v1",
			"choir.co_super_capsule_evidence_verifier/v1",
			"choir.co_super_execution_attestation/v1",
			"choir.lifecycle.persistent-super-report/v1",
			"choir.lifecycle.persistent-super-report/v2",
		},
		profiles: []string{"super", "co-super", "cosuper", "co_super", "researcher", "owner"},
		seeds: []string{
			"choir:co-super-assignment:v3", "choir:co-super-request:v2",
			"choir:co-super-decision:v3", "choir:delegated-cast-request:v1",
			"choir:delegated-decision:v1", "choir:persistent-super-report:v1",
		},
	}

	// Fold through the frozen decoders: every fixture member must decode
	// byte-identically (the fold output equals the pre-migration input).
	for _, k := range fixture.commandKinds {
		got, ok := DecodeLifecycleCommandKind(k)
		if !ok || got != k {
			t.Errorf("command kind %q does not fold identically", k)
		}
	}
	for _, k := range fixture.eventKinds {
		got, ok := DecodeLifecycleEventKind(k)
		if !ok || got != k {
			t.Errorf("event kind %q does not fold identically", k)
		}
	}
	for _, k := range fixture.ogKinds {
		got, ok := DecodeOGObjectKind(k)
		if !ok || got != k {
			t.Errorf("OG kind %q does not fold identically", k)
		}
	}
	for _, s := range fixture.schemas {
		got, ok := DecodeSchemaString(s)
		if !ok || got != s {
			t.Errorf("schema %q does not fold identically", s)
		}
	}
	for _, s := range fixture.seeds {
		if !IsFrozenIdentitySeed(s) {
			t.Errorf("seed %q absent from freeze", s)
		}
	}
	// Profiles fold to semantic live values through the one normalization
	// point — identical meaning, not identical bytes (that IS the point).
	for _, p := range fixture.profiles {
		got, ok := NormalizeHistoricProfile(p)
		if !ok {
			t.Errorf("historic profile %q did not fold", p)
		} else {
			t.Logf("profile %q -> %q", p, got)
		}
	}
}
