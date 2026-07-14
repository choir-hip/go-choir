package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInventoryBaselineAndRegressions(t *testing.T) {
	t.Run("clean baseline", func(t *testing.T) {
		root := fixtureRepository(t)
		baseline := mustScan(t, root)
		current := mustScan(t, root)
		if err := compareInventory(baseline, current); err != nil {
			t.Fatalf("clean baseline: %v", err)
		}
	})

	tests := []struct {
		name       string
		mutate     func(*testing.T, string)
		diagnostic string
	}{
		{
			name: "added production file",
			mutate: func(t *testing.T, root string) {
				writeFixture(t, root, "internal/agentcore/added.go", "package runtime\n\nfunc added() {}\n")
			},
			diagnostic: `files: added item "internal/agentcore/added.go [production]"`,
		},
		{
			name: "added export",
			mutate: func(t *testing.T, root string) {
				writeFixture(t, root, "internal/agentcore/runtime.go", "package runtime\n\ntype Runtime struct{}\n\nfunc NewRuntime() *Runtime { return &Runtime{} }\n")
			},
			diagnostic: "new production exports also require a production caller",
		},
		{
			name: "added production importer",
			mutate: func(t *testing.T, root string) {
				writeFixture(t, root, "cmd/newcaller/main.go", "package main\n\nimport runtime \"github.com/yusefmosiah/go-choir/internal/runtime\"\n\nvar _ *runtime.Runtime\n")
			},
			diagnostic: `production_importers: added item "cmd/newcaller/main.go"`,
		},
		{
			name: "added citer",
			mutate: func(t *testing.T, root string) {
				writeFixture(t, root, "docs/new-contract.md", "Active dependency: internal/runtime must remain.\n")
			},
			diagnostic: `citers: added item "docs/new-contract.md:Active dependency: internal/runtime must remain."`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := fixtureRepository(t)
			baseline := mustScan(t, root)
			tc.mutate(t, root)
			err := compareInventory(baseline, mustScan(t, root))
			assertDiagnostic(t, err, tc.diagnostic)
		})
	}
}

func TestInventoryRejectsDispositionErrors(t *testing.T) {
	t.Run("undispositioned item", func(t *testing.T) {
		root := fixtureRepository(t)
		baseline := mustScan(t, root)
		baseline.Files[0].Disposition = ""
		err := compareInventory(baseline, mustScan(t, root))
		assertDiagnostic(t, err, "undispositioned item")
	})

	t.Run("invalid item disposition", func(t *testing.T) {
		root := fixtureRepository(t)
		baseline := mustScan(t, root)
		baseline.Exports[0].Disposition = "later"
		err := compareInventory(baseline, mustScan(t, root))
		assertDiagnostic(t, err, `invalid disposition "later"`)
	})

	t.Run("invalid citer disposition", func(t *testing.T) {
		root := fixtureRepository(t)
		baseline := mustScan(t, root)
		baseline.Citers[0].Disposition = "core"
		err := compareInventory(baseline, mustScan(t, root))
		assertDiagnostic(t, err, `invalid disposition "core"`)
	})
}

func TestSyntaxAwareInventory(t *testing.T) {
	root := fixtureRepository(t)
	writeFixture(t, root, "internal/toolregistry/toolregistry.go", `package toolregistry

type Tool struct { Name string }
`)
	writeFixture(t, root, "internal/agentcore/registration.go", `package runtime

import "github.com/yusefmosiah/go-choir/internal/toolregistry"

func register(s interface{ HandleFunc(string, any) }, r interface{ Register(any) error }) {
	s.HandleFunc("/api/example", handler)
	_ = r.Register(Tool{Name: "example_tool"})
	_ = r.Register(toolregistry.Tool{Name: "qualified_tool"})
}

var handler any
type Tool struct { Name string }
`)
	writeFixture(t, root, "cmd/string-only/main.go", `package main

const notAnImport = "github.com/yusefmosiah/go-choir/internal/runtime"
`)
	inv := mustScan(t, root)
	if len(inv.Routes) != 1 || !strings.Contains(inv.Routes[0].ID, "/api/example") {
		t.Fatalf("routes = %+v, want syntax-derived /api/example registration", inv.Routes)
	}
	if len(inv.Tools) != 2 || !strings.Contains(inv.Tools[0].ID, "example_tool") || !strings.Contains(inv.Tools[1].ID, "qualified_tool") {
		t.Fatalf("tools = %+v, want identical unqualified and qualified syntax-derived registrations", inv.Tools)
	}
	if len(inv.ProductionImporters) != 0 {
		t.Fatalf("production importers = %+v, string literal must not count as import", inv.ProductionImporters)
	}
}
func TestTypeAwareStateWriterInventory(t *testing.T) {
	root := fixtureRepository(t)
	writeFixture(t, root, "internal/store/store.go", `package store

type Store struct{}
func (*Store) CreateDocument() {}
func (*Store) CreateRevision() {}
func (*Store) UpdateDocument() {}
func (*Store) CreateWorkItem() {}
func (*Store) UpdateTrajectoryStatus() {}
func (*Store) PatchRevisionMetadata() {}
func (*Store) ClaimCoSuperSlot() {}
func (*Store) ReleaseCoSuperSlotClaim() {}
func (*Store) CancelAgentMutation() {}
func (*Store) UpsertAppAdoption() {}
func (*Store) UpsertComputerSourceLineage() {}
func (*Store) UpsertAppChangePackage() {}
func (*Store) UpdateAppAdoptionIfCurrent() {}
`)
	writeFixture(t, root, "internal/agentcore/writers.go", `package runtime

import "github.com/yusefmosiah/go-choir/internal/store"

func writeState(value *store.Store) {
	value.CreateDocument()
	value.CreateRevision()
	value.UpdateDocument()
	value.CreateWorkItem()
	value.UpdateTrajectoryStatus()
	value.PatchRevisionMetadata()
	value.ClaimCoSuperSlot()
	value.ReleaseCoSuperSlotClaim()
	value.CancelAgentMutation()
	value.UpsertAppAdoption()
	value.UpsertComputerSourceLineage()
	value.UpsertAppChangePackage()
	value.UpdateAppAdoptionIfCurrent()
}
`)
	inventory := mustScan(t, root)
	required := map[string]string{
		"CreateDocument":              "wire",
		"CreateRevision":              "wire",
		"UpdateDocument":              "wire",
		"CreateWorkItem":              "wire",
		"UpdateTrajectoryStatus":      "wire",
		"PatchRevisionMetadata":       "wire",
		"ClaimCoSuperSlot":            "lifecycle",
		"ReleaseCoSuperSlotClaim":     "lifecycle",
		"CancelAgentMutation":         "lifecycle",
		"UpsertAppAdoption":           "promotion",
		"UpsertComputerSourceLineage": "promotion",
		"UpsertAppChangePackage":      "promotion",
		"UpdateAppAdoptionIfCurrent":  "promotion",
	}
	for index := range inventory.StoreCalls {
		call := &inventory.StoreCalls[index]
		if !strings.Contains(call.ID, "internal/store.") {
			t.Errorf("store call %q is not an underlying store method", call.ID)
		}
		for name, disposition := range required {
			if strings.Contains(call.ID, "."+name) {
				call.Disposition = disposition
				delete(required, name)
				break
			}
		}
	}
	if len(required) > 0 {
		t.Fatalf("store calls = %+v, missing required calls %+v", inventory.StoreCalls, required)
	}
	if problems := validateStoreCalls(inventory.StoreCalls); len(problems) > 0 {
		t.Fatalf("classified store calls: %v", problems)
	}
}

func TestNewStoreWriterRequiresBaselineDisposition(t *testing.T) {
	root := fixtureRepository(t)
	writeFixture(t, root, "internal/store/store.go", `package store

type Store struct{}
func (*Store) UpsertAppAdoption() {}
`)
	writeFixture(t, root, "internal/agentcore/writer.go", `package runtime

import "github.com/yusefmosiah/go-choir/internal/store"
var stateStore *store.Store
`)
	baseline := mustScan(t, root)
	writeFixture(t, root, "internal/agentcore/writer.go", `package runtime

import "github.com/yusefmosiah/go-choir/internal/store"
var stateStore *store.Store
func writePromotion() { stateStore.UpsertAppAdoption() }
`)
	err := compareInventory(baseline, mustScan(t, root))
	assertDiagnostic(t, err, "store_calls: added item")
	assertDiagnostic(t, err, "UpsertAppAdoption")
}
func TestNewPatchWriterRequiresBaselineDisposition(t *testing.T) {
	root := fixtureRepository(t)
	writeFixture(t, root, "internal/store/store.go", `package store

type Store struct{}
func (*Store) PatchRevisionMetadata() {}
`)
	writeFixture(t, root, "internal/agentcore/writer.go", `package runtime

import "github.com/yusefmosiah/go-choir/internal/store"
var stateStore *store.Store
`)
	baseline := mustScan(t, root)
	writeFixture(t, root, "internal/agentcore/writer.go", `package runtime

import "github.com/yusefmosiah/go-choir/internal/store"
var stateStore *store.Store
func patchWire() { stateStore.PatchRevisionMetadata() }
`)
	err := compareInventory(baseline, mustScan(t, root))
	assertDiagnostic(t, err, "store_calls: added item")
	assertDiagnostic(t, err, "PatchRevisionMetadata")
}
func TestUnknownStoreMethodFailsClosed(t *testing.T) {
	root := fixtureRepository(t)
	writeFixture(t, root, "internal/store/store.go", `package store

type Store struct{}
func (*Store) TransmogrifyState() {}
`)
	writeFixture(t, root, "internal/agentcore/writer.go", `package runtime

import "github.com/yusefmosiah/go-choir/internal/store"
var stateStore *store.Store
`)
	baseline := mustScan(t, root)
	writeFixture(t, root, "internal/agentcore/writer.go", `package runtime

import "github.com/yusefmosiah/go-choir/internal/store"
var stateStore *store.Store
func mutateUnknown() { stateStore.TransmogrifyState() }
`)
	err := compareInventory(baseline, mustScan(t, root))
	assertDiagnostic(t, err, "store_calls: added item")
	assertDiagnostic(t, err, "TransmogrifyState")
}
func TestDeceptiveStoreMethodNamesRequireDisposition(t *testing.T) {
	for _, method := range []string{"GetAndDeleteState", "LoadOrCreateRun"} {
		t.Run(method, func(t *testing.T) {
			root := fixtureRepository(t)
			writeFixture(t, root, "internal/store/store.go", "package store\n\ntype Store struct{}\nfunc (*Store) "+method+"() {}\n")
			writeFixture(t, root, "internal/agentcore/writer.go", `package runtime

import "github.com/yusefmosiah/go-choir/internal/store"
var stateStore *store.Store
`)
			baseline := mustScan(t, root)
			writeFixture(t, root, "internal/agentcore/writer.go", "package runtime\n\nimport \"github.com/yusefmosiah/go-choir/internal/store\"\nvar stateStore *store.Store\nfunc callStore() { stateStore."+method+"() }\n")
			err := compareInventory(baseline, mustScan(t, root))
			assertDiagnostic(t, err, "store_calls: added item")
			assertDiagnostic(t, err, method)
		})
	}
}

func TestStoreWriterDispositionsCannotBeLaundered(t *testing.T) {
	required := map[string]string{
		"CreateBrowserSession":      "lifecycle",
		"UpdateBrowserSession":      "lifecycle",
		"UpsertPodcastSubscription": "lifecycle",
		"UpsertMediaProgress":       "lifecycle",
		"UpsertMediaRecent":         "lifecycle",
		"SaveUserPreference":        "lifecycle",
		"UpsertDocumentAlias":       "wire",
		"DeleteDocument":            "wire",
	}
	for method, expected := range required {
		t.Run(method, func(t *testing.T) {
			inventory := Inventory{StoreCalls: []Entry{{
				ID:          "internal/agentcore/fixture.go:use:internal/store.method:Store." + method,
				Disposition: "read",
			}}}
			assertDiagnostic(t, errors.New(strings.Join(validateStoreCalls(inventory.StoreCalls), "\n")), "must use "+expected+" disposition")
			applyAuthoritativeStoreDispositions(&inventory)
			if inventory.StoreCalls[0].Disposition != expected {
				t.Fatalf("authoritative regeneration disposition = %q, want %q", inventory.StoreCalls[0].Disposition, expected)
			}
			if problems := validateStoreCalls(inventory.StoreCalls); len(problems) > 0 {
				t.Fatalf("authoritative disposition: %v", problems)
			}
		})
	}
}

func TestExactReadRequiresBaselineDisposition(t *testing.T) {
	root := fixtureRepository(t)
	writeFixture(t, root, "internal/store/store.go", `package store

type Store struct{}
func (*Store) GetDocument() {}
`)
	writeFixture(t, root, "internal/agentcore/reader.go", `package runtime

import "github.com/yusefmosiah/go-choir/internal/store"
func readStore(value *store.Store) { value.GetDocument() }
`)
	baseline := mustScan(t, root)
	current := mustScan(t, root)
	assertDiagnostic(t, compareInventory(baseline, current), "missing or invalid disposition")
	baseline.StoreCalls[0].Disposition = "read"
	if err := compareInventory(baseline, current); err != nil {
		t.Fatalf("baseline-dispositioned read: %v", err)
	}
}

func TestStoreMethodValuesAreInventoried(t *testing.T) {
	t.Run("mutating method value is new call identity", func(t *testing.T) {
		root := fixtureRepository(t)
		writeFixture(t, root, "internal/store/store.go", `package store

type Store struct{}
func (*Store) SaveDesktopState() {}
`)
		writeFixture(t, root, "internal/agentcore/persistence.go", `package runtime

import "github.com/yusefmosiah/go-choir/internal/store"
var stateStore *store.Store
`)
		baseline := mustScan(t, root)
		writeFixture(t, root, "internal/agentcore/persistence.go", `package runtime

import "github.com/yusefmosiah/go-choir/internal/store"
var stateStore *store.Store
var saveState = stateStore.SaveDesktopState
`)
		err := compareInventory(baseline, mustScan(t, root))
		assertDiagnostic(t, err, "store_calls: added item")
		assertDiagnostic(t, err, "SaveDesktopState")
	})

	t.Run("read method value requires read disposition", func(t *testing.T) {
		root := fixtureRepository(t)
		writeFixture(t, root, "internal/store/store.go", `package store

type Store struct{}
func (*Store) GetDocument() {}
`)
		writeFixture(t, root, "internal/agentcore/persistence.go", `package runtime

import "github.com/yusefmosiah/go-choir/internal/store"
var stateStore *store.Store
var readDocument = stateStore.GetDocument
`)
		baseline := mustScan(t, root)
		current := mustScan(t, root)
		assertDiagnostic(t, compareInventory(baseline, current), "missing or invalid disposition")
		baseline.StoreCalls[0].Disposition = "read"
		if err := compareInventory(baseline, current); err != nil {
			t.Fatalf("baseline-dispositioned method value read: %v", err)
		}
	})
}

func TestStoreBackedInterfaceCallIsInventoried(t *testing.T) {
	root := fixtureRepository(t)
	writeFixture(t, root, "internal/store/store.go", `package store

type Store struct{}
func (*Store) SaveUserPreference() {}
`)
	writeFixture(t, root, "internal/agentcore/persistence.go", `package runtime

import "github.com/yusefmosiah/go-choir/internal/store"
type persistence interface { SaveUserPreference() }
func adapt(source persistence) { persist(source) }
func persist(target persistence) {}
func begin(state *store.Store) { adapt(state) }
`)
	baseline := mustScan(t, root)
	writeFixture(t, root, "internal/agentcore/persistence.go", `package runtime

import "github.com/yusefmosiah/go-choir/internal/store"
type persistence interface { SaveUserPreference() }
func adapt(source persistence) { persist(source) }
func persist(target persistence) { target.SaveUserPreference() }
func begin(state *store.Store) { adapt(state) }
`)
	current := mustScan(t, root)
	err := compareInventory(baseline, current)
	assertDiagnostic(t, err, "interface_candidates: added item")
	assertDiagnostic(t, err, "store.interface:")
	current.InterfaceCandidates[0].Disposition = "lifecycle"
	applyInterfaceCandidateAuthority(&current, current)
	current.StoreCalls[0].Disposition = "lifecycle"
	if problems := validateInterfaceCandidates(current.InterfaceCandidates); len(problems) > 0 {
		t.Fatalf("store-backed candidate disposition: %v", problems)
	}
	if problems := validateInterfaceCandidateAuthority(current); len(problems) > 0 {
		t.Fatalf("store-backed candidate authority: %v", problems)
	}
	if problems := validateStoreCalls(current.StoreCalls); len(problems) > 0 {
		t.Fatalf("store-backed call disposition: %v", problems)
	}
}

func TestInterfaceCandidateAuthorityCoversFlowShapes(t *testing.T) {
	flows := map[string]string{
		"return": `func source(state *store.Store) persistence { return state }
func use() { source(&store.Store{}).SaveUserPreference() }`,
		"conversion": `func use() { persistence(&store.Store{}).SaveUserPreference() }`,
		"composite": `type holder struct { state persistence }
func use() { holder{state: &store.Store{}}.state.SaveUserPreference() }`,
		"closure": `func use() { (func() persistence { return &store.Store{} })().SaveUserPreference() }`,
	}
	for name, flow := range flows {
		t.Run(name, func(t *testing.T) {
			root := fixtureRepository(t)
			writeFixture(t, root, "internal/store/store.go", `package store

type Store struct{}
func (*Store) SaveUserPreference() {}
`)
			writeFixture(t, root, "internal/agentcore/persistence.go", `package runtime

import "github.com/yusefmosiah/go-choir/internal/store"
type persistence interface { SaveUserPreference() }
`+flow)
			inventory := mustScan(t, root)
			if len(inventory.InterfaceCandidates) != 1 {
				t.Fatalf("interface candidates = %+v, want one", inventory.InterfaceCandidates)
			}
			if len(inventory.StoreCalls) != 0 {
				t.Fatalf("candidate became Store authority before disposition: %+v", inventory.StoreCalls)
			}
			inventory.InterfaceCandidates[0].Disposition = "read"
			applyInterfaceCandidateAuthority(&inventory, inventory)
			if len(inventory.StoreCalls) != 1 {
				t.Fatalf("store-backed candidate calls = %+v, want one", inventory.StoreCalls)
			}
			assertDiagnostic(t, errors.New(strings.Join(validateInterfaceCandidates(inventory.InterfaceCandidates), "\n")), `must use authoritative "lifecycle" disposition`)
			assertDiagnostic(t, errors.New(strings.Join(validateStoreCalls(inventory.StoreCalls), "\n")), "must use lifecycle disposition")
			inventory.InterfaceCandidates[0].Disposition = "lifecycle"
			inventory.StoreCalls[0].Disposition = "lifecycle"
			if problems := validateInterfaceCandidateAuthority(inventory); len(problems) > 0 {
				t.Fatalf("candidate authority: %v", problems)
			}
		})
	}
}

func TestPromotedEmbeddedInterfaceSelectionIsCandidate(t *testing.T) {
	root := fixtureRepository(t)
	writeFixture(t, root, "internal/store/store.go", `package store

type Store struct{}
func (*Store) SaveUserPreference() {}
`)
	writeFixture(t, root, "internal/agentcore/persistence.go", `package runtime

import "github.com/yusefmosiah/go-choir/internal/store"
type persistence interface { SaveUserPreference() }
type embedded struct { persistence }
var _ *store.Store
func persist(value embedded) { value.SaveUserPreference() }
`)
	inventory := mustScan(t, root)
	if len(inventory.InterfaceCandidates) != 1 {
		t.Fatalf("promoted interface candidates = %+v, want one", inventory.InterfaceCandidates)
	}
	if !strings.Contains(inventory.InterfaceCandidates[0].ID, "internal/agentcore.persistence.SaveUserPreference") {
		t.Fatalf("candidate identity does not name declaring interface: %q", inventory.InterfaceCandidates[0].ID)
	}
	inventory.InterfaceCandidates[0].Disposition = "lifecycle"
	applyInterfaceCandidateAuthority(&inventory, inventory)
	if len(inventory.StoreCalls) != 1 {
		t.Fatalf("promoted potential Store calls = %+v, want one", inventory.StoreCalls)
	}
}

func TestSameNameFakeInterfaceIsConservativePotentialStoreCall(t *testing.T) {
	root := fixtureRepository(t)
	writeFixture(t, root, "internal/store/store.go", `package store

type Store struct{}
func (*Store) GetRun() {}
`)
	writeFixture(t, root, "internal/agentcore/reporter.go", `package runtime

import "github.com/yusefmosiah/go-choir/internal/store"
type reporter interface { GetRun() }
type fakeReporter struct{}
func (fakeReporter) GetRun() {}
var activeReporter reporter = fakeReporter{}
var _ *store.Store
func report() { activeReporter.GetRun() }
`)
	inventory := mustScan(t, root)
	if len(inventory.InterfaceCandidates) != 1 {
		t.Fatalf("fake-only same-name interface candidates = %+v, want one", inventory.InterfaceCandidates)
	}
	inventory.InterfaceCandidates[0].Disposition = "later"
	assertDiagnostic(t, errors.New(strings.Join(validateInterfaceCandidates(inventory.InterfaceCandidates), "\n")), `must use authoritative "read" disposition`)
	inventory.InterfaceCandidates[0].Disposition = "read"
	applyInterfaceCandidateAuthority(&inventory, inventory)
	if problems := validateInterfaceCandidateAuthority(inventory); len(problems) > 0 {
		t.Fatalf("fake-only potential Store authority: %v", problems)
	}
	if len(inventory.StoreCalls) != 1 || inventory.StoreCalls[0].Disposition != "read" {
		t.Fatalf("fake-only potential Store call = %+v, want one read", inventory.StoreCalls)
	}
}

func TestPackageStoreHelperIsNotStoreMethod(t *testing.T) {
	root := fixtureRepository(t)
	writeFixture(t, root, "internal/store/store.go", `package store

type Store struct{}
func SaveDesktopState() {}
`)
	writeFixture(t, root, "internal/agentcore/persistence.go", `package runtime

import "github.com/yusefmosiah/go-choir/internal/store"
func persist() { store.SaveDesktopState() }
`)
	inventory := mustScan(t, root)
	if len(inventory.StoreCalls) != 0 {
		t.Fatalf("package helper produced store method calls: %+v", inventory.StoreCalls)
	}
}

func TestInventoryUsesAuthoritativeFilesAndStableCiterIdentities(t *testing.T) {
	t.Run("ignored generated tree is excluded", func(t *testing.T) {
		root := fixtureRepository(t)
		writeFixture(t, root, ".gitignore", "frontend/dist/\n")
		initGitFixture(t, root)
		baseline := mustScan(t, root)
		writeFixture(t, root, "frontend/dist/assets/ghostty-web.js",
			strings.Repeat("x", 700_000)+" // internal/agentcore\n")
		current := mustScan(t, root)
		if err := compareInventory(baseline, current); err != nil {
			t.Fatalf("ignored generated tree changed inventory: %v", err)
		}
	})

	t.Run("unrelated preceding line does not change citer identity", func(t *testing.T) {
		root := fixtureRepository(t)
		baseline := mustScan(t, root)
		writeFixture(t, root, "docs/evidence/history.md",
			"Unrelated ledger entry.\nRemoved dependency: internal/runtime.\n")
		current := mustScan(t, root)
		if err := compareInventory(baseline, current); err != nil {
			t.Fatalf("preceding line changed citer identity: %v", err)
		}
	})
}

func TestCiterDispositionSeparatesHistoricalAndLiveDocs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		path string
		want string
	}{
		{path: "docs/evidence/history.md", want: "historical_evidence"},
		{path: "docs/archive/mission-v0.md", want: "historical_evidence"},
		{path: "docs/definitions/active.md", want: "block"},
	} {
		if got := citerDisposition(tc.path); got != tc.want {
			t.Errorf("citerDisposition(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}
func TestRebasedExportRequiresProductionCaller(t *testing.T) {
	t.Run("unused export fails after baseline rebase", func(t *testing.T) {
		root := fixtureRepository(t)
		original := mustScan(t, root)
		writeFixture(t, root, "internal/agentcore/runtime.go",
			"package runtime\n\ntype Runtime struct{}\n\nfunc UnusedExport() {}\n")
		rebased := mustScan(t, root)
		rebaseInitialDebt(&rebased, original.UnusedExportDebt)
		err := compareInventory(rebased, mustScan(t, root))
		assertDiagnostic(t, err, "has no AST-derived non-test production caller")
	})

	t.Run("used export passes after baseline rebase", func(t *testing.T) {
		root := fixtureRepository(t)
		original := mustScan(t, root)
		writeFixture(t, root, "internal/agentcore/runtime.go",
			"package runtime\n\ntype Runtime struct{}\n\nfunc UsedExport() {}\n")
		writeFixture(t, root, "cmd/caller/main.go", `package main

import runtime "github.com/yusefmosiah/go-choir/internal/agentcore"

func useRuntime() { runtime.UsedExport() }
`)
		rebased := mustScan(t, root)
		rebaseInitialDebt(&rebased, original.UnusedExportDebt)
		if err := compareInventory(rebased, mustScan(t, root)); err != nil {
			t.Fatalf("used rebased export: %v", err)
		}
	})

	t.Run("typed receiver method call passes", func(t *testing.T) {
		root := fixtureRepository(t)
		original := mustScan(t, root)
		writeFixture(t, root, "internal/agentcore/runtime.go", `package runtime

type Runtime struct{}
func (*Runtime) Start() {}
`)
		writeFixture(t, root, "cmd/caller/main.go", `package main

import runtime "github.com/yusefmosiah/go-choir/internal/agentcore"

func useRuntime(rt *runtime.Runtime) { rt.Start() }
`)
		rebased := mustScan(t, root)
		rebaseInitialDebt(&rebased, original.UnusedExportDebt)
		if err := compareInventory(rebased, mustScan(t, root)); err != nil {
			t.Fatalf("typed receiver caller: %v", err)
		}
	})

	t.Run("unrelated same-name method does not satisfy", func(t *testing.T) {
		root := fixtureRepository(t)
		original := mustScan(t, root)
		writeFixture(t, root, "internal/agentcore/runtime.go", `package runtime

type Runtime struct{}
func (*Runtime) Start() {}
`)
		writeFixture(t, root, "cmd/caller/main.go", `package main

import runtime "github.com/yusefmosiah/go-choir/internal/agentcore"

type other struct{}
func (*other) Start() {}
func useOther(value *other) { value.Start() }
var _ *runtime.Runtime
`)
		rebased := mustScan(t, root)
		rebaseInitialDebt(&rebased, original.UnusedExportDebt)
		err := compareInventory(rebased, mustScan(t, root))
		assertDiagnostic(t, err, "has no AST-derived non-test production caller")
	})

	t.Run("constructor field flow passes", func(t *testing.T) {
		root := fixtureRepository(t)
		original := mustScan(t, root)
		writeFixture(t, root, "internal/agentcore/runtime.go", `package runtime

type Runtime struct{}
func NewRuntime() *Runtime { return &Runtime{} }
func (*Runtime) Start() {}
`)
		writeFixture(t, root, "cmd/caller/main.go", `package main

import runtime "github.com/yusefmosiah/go-choir/internal/agentcore"

type holder struct { runtime *runtime.Runtime }
func useRuntime() {
	value := holder{runtime: runtime.NewRuntime()}
	value.runtime.Start()
}
`)
		rebased := mustScan(t, root)
		rebaseInitialDebt(&rebased, original.UnusedExportDebt)
		if err := compareInventory(rebased, mustScan(t, root)); err != nil {
			t.Fatalf("constructor field caller: %v", err)
		}
	})

	t.Run("promoted alias receiver flow passes", func(t *testing.T) {
		root := fixtureRepository(t)
		original := mustScan(t, root)
		writeFixture(t, root, "internal/agentcore/runtime.go", `package runtime

type Runtime struct{}
func (*Runtime) Start() {}
`)
		writeFixture(t, root, "cmd/caller/main.go", `package main

import runtime "github.com/yusefmosiah/go-choir/internal/agentcore"

type runtimeAlias = runtime.Runtime
type holder struct { *runtimeAlias }
func useRuntime(value holder) { value.Start() }
`)
		rebased := mustScan(t, root)
		rebaseInitialDebt(&rebased, original.UnusedExportDebt)
		if err := compareInventory(rebased, mustScan(t, root)); err != nil {
			t.Fatalf("promoted alias caller: %v", err)
		}
	})
}

func TestInitialUnusedExportDebtCannotGrow(t *testing.T) {
	t.Run("manual debt and count rebase fails", func(t *testing.T) {
		root := fixtureRepository(t)
		prior := mustScan(t, root)
		writeFixture(t, root, "internal/agentcore/runtime.go",
			"package runtime\n\ntype Runtime struct{}\n\nfunc NewlyUnused() {}\n")
		rebased := mustScan(t, root)
		err := validateDebtNoGrowth(prior, rebased)
		assertDiagnostic(t, err, "initial unused export debt grew beyond prior canonical Git authority")
	})

	t.Run("debt removal passes", func(t *testing.T) {
		prior := mustScan(t, fixtureRepository(t))
		current := prior
		current.UnusedExportDebt = nil
		if err := validateDebtNoGrowth(prior, current); err != nil {
			t.Fatalf("debt removal: %v", err)
		}
	})
}

func TestRebasePackageCutoverDebtPreservesOnlyMovedExports(t *testing.T) {
	exports := []Entry{
		{ID: "internal/agentcore/runtime.go:method(*Runtime):StartRun"},
		{ID: "internal/textureowner/texture_workflow_verifier.go:method(*Handler):VerifyTextureWorkflow"},
		{ID: "internal/agentcore/runtime.go:func:NewlyUnused"},
	}
	previous := []Entry{
		{ID: "internal/runtime/runtime.go:method(*Runtime):StartRun"},
		{ID: "internal/runtime/texture_workflow_verifier.go:method(*Runtime):VerifyTextureWorkflow"},
		{ID: "internal/runtime/runtime.go:method(*Runtime):Removed"},
	}
	got := rebasePackageCutoverDebt(exports, previous)
	if len(got) != 2 ||
		got[0].ID != "internal/agentcore/runtime.go:method(*Runtime):StartRun" ||
		got[1].ID != "internal/textureowner/texture_workflow_verifier.go:method(*Handler):VerifyTextureWorkflow" {
		t.Fatalf("rebased debt = %#v", got)
	}
}

func TestCiterSuffixDriftChangesDigestIdentity(t *testing.T) {
	root := fixtureRepository(t)
	prefix := strings.Repeat("long-prefix-", 30)
	writeFixture(t, root, "docs/evidence/long.md",
		prefix+" internal/runtime retained suffix-one\n")
	baseline := mustScan(t, root)
	writeFixture(t, root, "docs/evidence/long.md",
		prefix+" internal/runtime retained suffix-two\n")
	err := compareInventory(baseline, mustScan(t, root))
	assertDiagnostic(t, err, "citers: added item")
}

func rebaseInitialDebt(inventory *Inventory, initial []Entry) {
	current := make(map[string]Entry, len(inventory.Exports))
	for _, export := range inventory.Exports {
		current[export.ID] = export
	}
	inventory.UnusedExportDebt = nil
	for _, debt := range initial {
		export, exists := current[debt.ID]
		if exists && len(export.ProductionCallers) == 0 {
			inventory.UnusedExportDebt = append(inventory.UnusedExportDebt, debt)
		}
	}
	setCounts(inventory)
}

func fixtureRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, "go.mod", "module github.com/yusefmosiah/go-choir\n\ngo 1.25.6\n")
	writeFixture(t, root, "internal/agentcore/runtime.go", "package runtime\n\ntype Runtime struct{}\n")
	writeFixture(t, root, "internal/agentcore/runtime_test.go", "package runtime\n\nfunc ExampleRuntime() {}\n")
	writeFixture(t, root, "docs/evidence/history.md", "Removed dependency: internal/runtime.\n")
	return root
}

func initGitFixture(t *testing.T, root string) {
	t.Helper()
	if output, err := exec.Command("git", "init", "--quiet", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
}

func writeFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustScan(t *testing.T, root string) Inventory {
	t.Helper()
	inv, err := scanRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	return inv
}

func assertDiagnostic(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("compareInventory error = nil, want diagnostic containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("compareInventory error = %q, want diagnostic containing %q", err, want)
	}
}
