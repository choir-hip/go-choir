package pkgrelease

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
)

// The tiered review (L3, review.go R1 to R5).

func nightTheme(mutate func(map[string]any)) []byte {
	v := map[string]any{
		"schema_version": 2, "id": "paper-night.dark", "name": "Paper night",
		"colors":  map[string]any{"bg": "#101418", "panel": "#1a2028", "fg": "#f4f4f4", "muted": "#b8c0c8", "accent": "#3a6ea5", "onAccent": "#ffffff", "danger": "#a52a2a", "onDanger": "#ffffff", "selected": "linear-gradient(#123456, #654321)"},
		"radii":   map[string]any{"window": "12px"},
		"motion":  map[string]any{"fast": "120ms"},
		"fonts":   map[string]any{"ui": "system-ui, sans-serif"},
		"effects": map[string]any{"shadow": "0 2px 8px rgba(0, 0, 0, 0.4)"},
		"layout":  map[string]any{"promptSurfacePlacement": "bottom", "promptSurfaceMinHeight": "56px", "deskSheetHeight": "70vh"},
	}
	if mutate != nil {
		mutate(v)
	}
	raw, _ := json.Marshal(v)
	return raw
}

func packManifest(t *testing.T) []byte {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{
		"schema": "choir.package/v2", "name": "paper-night", "version": "1.0.0",
		"display":      map[string]any{"title": "Paper night", "icon": "N", "description": "A dark theme"},
		"kinds":        []any{},
		"uses_kinds":   []any{},
		"contributes":  map[string]any{"desks": []any{}, "themes": []any{map[string]any{"id": "paper-night.dark", "file": "theme.json"}}},
		"capabilities": []any{},
	})
	return raw
}

// R1, R2, T0: a pack's theme is reviewed where it is published and installed.
func TestTierReviewRefusesAPackWithAnUnsafeValueOrLowContrast(t *testing.T) {
	check := func(theme []byte) error {
		_, _, err := CheckManifest("paper-night", packManifest(t), apppackage.MapSource(map[string]string{"theme.json": string(theme)}))
		return err
	}
	if err := check(nightTheme(nil)); err != nil {
		t.Fatalf("a safe theme is refused: %v", err)
	}
	for _, bad := range []string{
		`url(https://example.com/p.png)`,
		`url ( "https://example.com/p.png" )`,
		`IMAGE-SET("https://example.com/p.png" 1x)`,
		`imag\65-set("https://example.com/p.png" 1x)`,
		`cross-fade(a, "https://example.com/p.png")`,
		`red; background: blue`,
		`expression(alert(1))`,
		`@import "x.css"`,
	} {
		err := check(nightTheme(func(v map[string]any) { v["colors"].(map[string]any)["selected"] = bad }))
		if ReasonOf(err) != ReasonUnsafeValue {
			t.Errorf("%s: %v", bad, err)
		}
	}
	if err := check(nightTheme(func(v map[string]any) { v["layout"].(map[string]any)["deskSheetHeight"] = "src(x)" })); ReasonOf(err) != ReasonUnsafeValue {
		t.Errorf("layout value: %v", err)
	}
	if err := check(nightTheme(func(v map[string]any) { v["colors"].(map[string]any)["muted"] = "#202428" })); ReasonOf(err) != ReasonThemeContrast {
		t.Errorf("low contrast: %v", err)
	}
	if err := check(nightTheme(func(v map[string]any) { v["colors"].(map[string]any)["fg"] = "white" })); ReasonOf(err) != ReasonThemeContrast {
		t.Errorf("unreadable color: %v", err)
	}
	if err := check(nightTheme(func(v map[string]any) { v["id"] = "futuristic-noir" })); ReasonOf(err) != ReasonManifestInvalid {
		t.Errorf("theme id not the declared one: %v", err)
	}
	if _, _, err := CheckManifest("paper-night", packManifest(t), apppackage.MapSource(map[string]string{})); ReasonOf(err) != ReasonManifestInvalid {
		t.Errorf("missing theme file: %v", err)
	}
}

// R5, T2: a provider of a shell surface is refused for want of a platform
// review record, whatever else is right about it.
func TestTierReviewRefusesAT2Release(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"schema": "choir.package/v2", "name": "menu-compact", "version": "1.0.0",
		"display":      map[string]any{"title": "Compact menu", "icon": "M", "description": ""},
		"kinds":        []any{},
		"uses_kinds":   []any{},
		"contributes":  map[string]any{"desks": []any{}},
		"provides":     []any{map[string]any{"service": "contextMenu.renderer", "abi": "choir.service/contextMenu.renderer/v1", "entry": "Menu.surface.svelte"}},
		"capabilities": []any{map[string]any{"grant": "ui.shell"}},
	})
	if _, _, err := CheckManifest("menu-compact", raw, apppackage.MapSource(map[string]string{"Menu.surface.svelte": "<p/>"})); ReasonOf(err) != ReasonPlatformReviewRequired {
		t.Fatalf("T2 release: %v", err)
	}
}

// R3, T1: a window app is listed with its contract fixture; its draft says so
// first, and the host refuses a release without one.
func TestTierReviewListsAWindowAppOnlyWithItsContract(t *testing.T) {
	f := newFixture(t)
	record, blobs := f.release(f.tree(map[string]entry{testRoot + "/" + ContractFile: {"-", ""}}))
	if err := VerifyHost(context.Background(), f.input(record, blobs)); ReasonOf(err) != ReasonContractRequired {
		t.Fatalf("no contract: %v", err)
	}
	record, blobs = f.release(f.tree(map[string]entry{testRoot + "/" + ContractFile: {"100644", `{"schema":"choir.package-contract/v1"}`}}))
	if err := VerifyHost(context.Background(), f.input(record, blobs)); ReasonOf(err) != ReasonContractRequired {
		t.Fatalf("no ready selector: %v", err)
	}
	// Install does not re-judge a listed release for it (R3).
	if err := VerifyContent(context.Background(), record, blobs.TreePatch); err != nil {
		t.Fatalf("install refuses a listed release for its contract: %v", err)
	}
}

// R4, T1 at install: a shell-wide SDK name needs ui.shell.
func TestCheckUsesRefusesAShellCallWithoutTheGrant(t *testing.T) {
	plain := json.RawMessage(`{"name":"notes","capabilities":[]}`)
	if err := CheckUses(plain, []string{"fetchWithRenewal", "openApp"}); err != nil {
		t.Fatalf("ordinary uses: %v", err)
	}
	err := CheckUses(plain, []string{"windowList", "fetchWithRenewal", "selectProvider"})
	if ReasonOf(err) != ReasonUsesExceedGrants || !strings.Contains(err.Error(), "selectProvider (ui.shell), windowList (ui.shell)") {
		t.Fatalf("shell uses without the grant: %v", err)
	}
	if err := CheckUses(json.RawMessage(`{"capabilities":[{"grant":"ui.shell"}]}`), []string{"windowList"}); err != nil {
		t.Fatalf("shell uses with the grant: %v", err)
	}
}

// The shell and the review read one list of fetching functions.
func TestCSSFetchFunctionsAreTheShellsList(t *testing.T) {
	raw, err := os.ReadFile("../../frontend/scripts/css-fetch-functions.json")
	if err != nil {
		t.Fatal(err)
	}
	var front struct {
		Functions []string `json:"functions"`
	}
	if err := json.Unmarshal(raw, &front); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(front.Functions, CSSFetchFunctions) {
		t.Fatalf("css-fetch-functions.json %v, the review %v", front.Functions, CSSFetchFunctions)
	}
}
