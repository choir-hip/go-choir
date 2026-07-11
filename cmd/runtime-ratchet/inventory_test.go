package main

import (
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
				writeFixture(t, root, "internal/runtime/added.go", "package runtime\n\nfunc added() {}\n")
			},
			diagnostic: `files: added item "internal/runtime/added.go [production]"`,
		},
		{
			name: "added export",
			mutate: func(t *testing.T, root string) {
				writeFixture(t, root, "internal/runtime/runtime.go", "package runtime\n\ntype Runtime struct{}\n\nfunc NewRuntime() *Runtime { return &Runtime{} }\n")
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
	writeFixture(t, root, "internal/runtime/registration.go", `package runtime

func register(s interface{ HandleFunc(string, any) }, r interface{ Register(any) error }) {
	s.HandleFunc("/api/example", handler)
	_ = r.Register(Tool{Name: "example_tool"})
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
	if len(inv.Tools) != 1 || !strings.Contains(inv.Tools[0].ID, "example_tool") {
		t.Fatalf("tools = %+v, want syntax-derived example_tool registration", inv.Tools)
	}
	if len(inv.ProductionImporters) != 0 {
		t.Fatalf("production importers = %+v, string literal must not count as import", inv.ProductionImporters)
	}
}
func TestInventoryUsesAuthoritativeFilesAndStableCiterIdentities(t *testing.T) {
	t.Run("ignored generated tree is excluded", func(t *testing.T) {
		root := fixtureRepository(t)
		writeFixture(t, root, ".gitignore", "frontend/dist/\n")
		initGitFixture(t, root)
		baseline := mustScan(t, root)
		writeFixture(t, root, "frontend/dist/assets/ghostty-web.js",
			strings.Repeat("x", 700_000)+" // internal/runtime\n")
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
func TestRebasedExportRequiresProductionCaller(t *testing.T) {
	t.Run("unused export fails after baseline rebase", func(t *testing.T) {
		root := fixtureRepository(t)
		original := mustScan(t, root)
		writeFixture(t, root, "internal/runtime/runtime.go",
			"package runtime\n\ntype Runtime struct{}\n\nfunc UnusedExport() {}\n")
		rebased := mustScan(t, root)
		rebaseInitialDebt(&rebased, original.UnusedExportDebt)
		err := compareInventory(rebased, mustScan(t, root))
		assertDiagnostic(t, err, "has no AST-derived non-test production caller")
	})

	t.Run("used export passes after baseline rebase", func(t *testing.T) {
		root := fixtureRepository(t)
		original := mustScan(t, root)
		writeFixture(t, root, "internal/runtime/runtime.go",
			"package runtime\n\ntype Runtime struct{}\n\nfunc UsedExport() {}\n")
		writeFixture(t, root, "cmd/caller/main.go", `package main

import runtime "github.com/yusefmosiah/go-choir/internal/runtime"

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
		writeFixture(t, root, "internal/runtime/runtime.go", `package runtime

type Runtime struct{}
func (*Runtime) Start() {}
`)
		writeFixture(t, root, "cmd/caller/main.go", `package main

import runtime "github.com/yusefmosiah/go-choir/internal/runtime"

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
		writeFixture(t, root, "internal/runtime/runtime.go", `package runtime

type Runtime struct{}
func (*Runtime) Start() {}
`)
		writeFixture(t, root, "cmd/caller/main.go", `package main

import runtime "github.com/yusefmosiah/go-choir/internal/runtime"

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
		writeFixture(t, root, "internal/runtime/runtime.go", `package runtime

type Runtime struct{}
func NewRuntime() *Runtime { return &Runtime{} }
func (*Runtime) Start() {}
`)
		writeFixture(t, root, "cmd/caller/main.go", `package main

import runtime "github.com/yusefmosiah/go-choir/internal/runtime"

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
		writeFixture(t, root, "internal/runtime/runtime.go", `package runtime

type Runtime struct{}
func (*Runtime) Start() {}
`)
		writeFixture(t, root, "cmd/caller/main.go", `package main

import runtime "github.com/yusefmosiah/go-choir/internal/runtime"

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
	writeFixture(t, root, "internal/runtime/runtime.go", "package runtime\n\ntype Runtime struct{}\n")
	writeFixture(t, root, "internal/runtime/runtime_test.go", "package runtime\n\nfunc ExampleRuntime() {}\n")
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
