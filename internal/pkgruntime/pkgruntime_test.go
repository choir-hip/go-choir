package pkgruntime

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/pkgruntime/runtimetest"
)

// Runtime install build and store, failure modes written before the code
// (docs/design/s12-runtime-install-build-2026-11-04.md):
//
//	R1  the embedded boundary plugin drifts from the frontend's;
//	R2  a module read from the store is not the bytes its digest names;
//	R3  a store write leaves a partial or writable module;
//	R4  a release whose bytes are not the record's is built;
//	R5  a package import outside its directory (other than the SDK) builds;
//	R6  the built module is not module/v1: no mount, no abi, no
//	    sdk_imports, or the SDK is bundled instead of read from ctx.sdk;
//	R7  requires_sdk is lost or malformed;
//	R8  a build with no dependencies or node hangs or half-succeeds;
//	R9  the shim's forwarded-event list drifts from the SDK lock's.

func TestEmbeddedBoundaryPluginIsTheFrontends(t *testing.T) {
	front, err := os.ReadFile(filepath.Join("..", "..", "frontend", "scripts", "package-boundary-plugin.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(front, boundaryPlugin) {
		t.Fatal("R1: internal/pkgruntime/package-boundary-plugin.mjs differs from frontend/scripts/package-boundary-plugin.mjs; copy it")
	}
}

// M4: the frontend build builds the base packages with the same script, from
// its own copy (the Nix frontend source holds only frontend/).
func TestEmbeddedBuildScriptIsTheFrontends(t *testing.T) {
	front, err := os.ReadFile(filepath.Join("..", "..", "frontend", "scripts", "build-module.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(front, buildScript) {
		t.Fatal("internal/pkgruntime/build-module.mjs differs from frontend/scripts/build-module.mjs; copy it")
	}
}

func TestEmbeddedCSSFetchFunctionsAreTheFrontends(t *testing.T) {
	front, err := os.ReadFile(filepath.Join("..", "..", "frontend", "scripts", "css-fetch-functions.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(front, cssFetchFunctions) {
		t.Fatal("internal/pkgruntime/css-fetch-functions.json differs from frontend/scripts/css-fetch-functions.json; copy it")
	}
}

func TestEmbeddedLegacyEventsAreTheFrontends(t *testing.T) {
	front, err := os.ReadFile(filepath.Join("..", "..", "frontend", "src", "lib", "sdk-surface", "legacy-events.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(front, legacyEvents) {
		t.Fatal("R9: internal/pkgruntime/legacy-events.json differs from frontend/src/lib/sdk-surface/legacy-events.json (the SDK lock's legacy_events); copy it")
	}
}

func TestStoreVerifiesEveryRead(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	sha, err := s.Put([]byte("export function mount() {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if again, err := s.Put([]byte("export function mount() {}\n")); err != nil || again != sha {
		t.Fatalf("repeat put = %s %v", again, err)
	}
	info, err := os.Stat(s.path(sha))
	if err != nil || info.Mode().Perm() != 0o444 {
		t.Fatalf("R3: mode %v (%v)", info.Mode(), err)
	}
	if raw, err := s.Read(sha); err != nil || string(raw) != "export function mount() {}\n" {
		t.Fatalf("read = %q %v", raw, err)
	}
	if _, err := s.Read(strings.Repeat("0", 64)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("absent = %v", err)
	}
	if _, err := s.Read("../../etc/passwd"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a path for a digest = %v", err)
	}
	// R2: tamper with the stored copy.
	if err := os.Chmod(s.path(sha), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.path(sha), []byte("alert(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Read(sha); err == nil {
		t.Fatal("R2: a tampered module was served")
	}
	// A put of the true bytes repairs it.
	if _, err := s.Put([]byte("export function mount() {}\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Read(sha); err != nil {
		t.Fatalf("after repair: %v", err)
	}
	leftovers, _ := filepath.Glob(filepath.Join(s.Dir, "modules", ".module-*"))
	if len(leftovers) != 0 {
		t.Fatalf("R3: temp files left: %v", leftovers)
	}
}

func TestBuilderRefusesWithoutDependencies(t *testing.T) {
	// R8
	if err := (Builder{DepsDir: t.TempDir()}).Available(); err == nil {
		t.Fatal("a builder with no node_modules is available")
	}
	if err := (Builder{}).Available(); err == nil {
		t.Fatal("a builder with no deps dir is available")
	}
}

const fixtureApp = `<script>
  import { onMount } from 'svelte';
  import { appTitle } from '../../lib/app-sdk';
  import { label } from './label.js';
  export let windowId = '';
  let mounted = false;
  onMount(() => { mounted = true; });
</script>

<section data-fixture-app data-window={windowId}>
  <h2>{label}</h2>
  <p data-fixture-title>{appTitle ? appTitle('x') : 'no sdk'}</p>
  {#if mounted}<span data-fixture-mounted>mounted</span>{/if}
</section>

<style>
  section { padding: 1rem; }
</style>
`

func builder(t *testing.T) Builder {
	t.Helper()
	deps := runtimetest.DepsDir(t)
	if deps == "" {
		t.Skip("frontend/node_modules is not installed (npm ci in frontend)")
	}
	b := Builder{DepsDir: deps}
	if err := b.Available(); err != nil {
		t.Skip(err)
	}
	return b
}

func TestBuildMakesAModuleV1(t *testing.T) {
	b := builder(t)
	rec, patch := runtimetest.Release(t, "owner-a", "fixture", map[string]string{
		"choir-package.json": runtimetest.Manifest("fixture", ` "requires_sdk": "^1.0",`),
		"App.svelte":         fixtureApp,
		"label.js":           "export const label = 'Fixture package';\n",
	})
	ctx := context.Background()

	// R4
	if _, err := b.Build(ctx, rec, patch[:len(patch)-1]); pkgrelease.ReasonOf(err) != pkgrelease.ReasonBlobMismatch {
		t.Fatalf("R4: build of other bytes = %v", err)
	}

	mod, err := b.Build(ctx, rec, patch)
	if err != nil {
		t.Fatal(err)
	}
	code := string(mod.Bytes)
	for _, want := range []string{`"choir.app-frontend/module/v1"`, `as mount`, `as sdk_imports`, `as requires_sdk`, `["appTitle"]`, `"^1.0"`, `Fixture package`, `padding`} {
		if !strings.Contains(code, want) {
			t.Errorf("R6/R7: module lacks %s", want)
		}
	}
	// The SDK is the shim, never the shell's file: nothing of media-utils is bundled.
	if strings.Contains(code, "MEDIA_FILE_ROUTES") {
		t.Error("R6: the shell SDK was bundled")
	}
	if mod.RequiresSDK != "^1.0" || mod.ABI != ModuleABI || mod.SHA256 != digest(mod.Bytes) || len(mod.Manifest) == 0 {
		t.Fatalf("module = %+v", Module{SHA256: mod.SHA256, ABI: mod.ABI, RequiresSDK: mod.RequiresSDK})
	}
}

func TestBuildRefusesAnImportOutsideThePackage(t *testing.T) {
	b := builder(t)
	// R5
	rec, patch := runtimetest.Release(t, "owner-a", "escaper", map[string]string{
		"choir-package.json": runtimetest.Manifest("escaper", ""),
		"App.svelte":         "<script>\n  import pkg from '../../../package.json';\n</script>\n<p>{pkg.name}</p>\n",
	})
	_, err := b.Build(context.Background(), rec, patch)
	if !errors.Is(err, ErrBuild) || !strings.Contains(err.Error(), "may not import") {
		t.Fatalf("R5: build = %v", err)
	}
}

// R9 (review): nothing outside the verified bytes runs. A literal data: (or
// any URL) import, which Vite leaves unresolved and Rollup would externalize,
// and an `export *` from the SDK, which would hide names from sdk_imports,
// fail the build.
func TestBuildRefusesURLImportsAndSDKExportStar(t *testing.T) {
	b := builder(t)
	for name, app := range map[string]string{
		"dataimport": "<script>\n  import('data:text/javascript,window.x=1');\n</script>\n<p>x</p>\n",
		"urlimport":  "<script>\n  import x from 'https://example.invalid/x.js';\n</script>\n<p>{x}</p>\n",
	} {
		rec, patch := runtimetest.Release(t, "owner-a", name, map[string]string{
			"choir-package.json": runtimetest.Manifest(name, ""),
			"App.svelte":         app,
		})
		if _, err := b.Build(context.Background(), rec, patch); !errors.Is(err, ErrBuild) || !strings.Contains(err.Error(), "may not import") {
			t.Fatalf("%s: build = %v", name, err)
		}
	}
	rec, patch := runtimetest.Release(t, "owner-a", "starsdk", map[string]string{
		"choir-package.json": runtimetest.Manifest("starsdk", ""),
		"App.svelte":         "<script>\n  import * as s from './sdk.js';\n</script>\n<p>{s.appTitle('x')}</p>\n",
		"sdk.js":             "export * from '../../lib/app-sdk';\n",
	})
	if _, err := b.Build(context.Background(), rec, patch); !errors.Is(err, ErrBuild) || !strings.Contains(err.Error(), "export *") {
		t.Fatalf("export * build = %v", err)
	}
}
