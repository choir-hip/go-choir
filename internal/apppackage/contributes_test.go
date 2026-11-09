package apppackage

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// Failure modes X1 to X7 are listed in contributes.go.

func packJSON(t *testing.T, mutate func(map[string]any)) []byte {
	t.Helper()
	v := map[string]any{
		"schema": ManifestSchemaV2, "name": "paper-night", "version": "1.2.0",
		"display": map[string]any{"title": "Paper Night", "icon": "N", "description": "A dark theme"},
		"contributes": map[string]any{
			"themes": []any{map[string]any{"id": "paper-night.dark", "file": "theme.json"}},
			"commands": []any{map[string]any{"id": "paper-night.use", "title": "Use Paper Night", "scope": "global",
				"run": map[string]any{"action": "setTheme", "theme": "paper-night.dark"}}},
			"keybindings": []any{map[string]any{"key": "cmd+shift+n", "command": "paper-night.use"}},
		},
	}
	if mutate != nil {
		mutate(v)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func contrib(v map[string]any) map[string]any { return v["contributes"].(map[string]any) }

func withApp(v map[string]any) {
	v["app"] = map[string]any{
		"entry": "App.svelte", "abi": AppABISvelteV1,
		"launcher": map[string]any{"menu": true, "desktopIcon": false, "order": 900}, "window": map[string]any{"singleton": true, "heavy": false},
		"auth": map[string]any{"preview": "private"}, "theme": map[string]any{"surface": "standard", "shellDataAttr": "data-paper-night", "contentClass": "paper-night-content"},
	}
}

// X7 and the pack: a data-only package of themes, commands and keybindings
// is accepted from a published origin and is not code-bearing.
func TestPackAcceptedFromAPublishedOrigin(t *testing.T) {
	for _, origin := range []string{OriginFirstParty, OriginPublished} {
		m, err := ParseManifest(packJSON(t, nil), origin)
		if err != nil {
			t.Fatalf("%s: %v", origin, err)
		}
		if m.Class() != ClassData || m.CodeBearing() || len(m.Contributes.Themes) != 1 || len(m.Contributes.Commands) != 1 {
			t.Fatalf("%s: %+v class=%s", origin, m.Contributes, m.Class())
		}
		if got := m.ThemeFiles(); len(got) != 1 || got[0] != "theme.json" {
			t.Fatalf("theme files = %v", got)
		}
	}
	if _, err := ParseManifest(packJSON(t, func(v map[string]any) { v["contributes"] = map[string]any{} }), OriginPublished); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("X7: an empty package must be refused, got %v", err)
	}
}

func TestContributesRefusals(t *testing.T) {
	cmd := func(v map[string]any) map[string]any { return contrib(v)["commands"].([]any)[0].(map[string]any) }
	cases := map[string]func(map[string]any){
		"X1 command outside the namespace": func(v map[string]any) { cmd(v)["id"] = "core.use" },
		"X1 command id shape":              func(v map[string]any) { cmd(v)["id"] = "paper-night" },
		"X1 command id twice":              func(v map[string]any) { c := contrib(v); c["commands"] = append(c["commands"].([]any), cmd(v)) },
		"X1 unknown action":                func(v map[string]any) { cmd(v)["run"] = map[string]any{"action": "eval", "theme": "x"} },
		"X1 action field missing":          func(v map[string]any) { cmd(v)["run"] = map[string]any{"action": "setTheme"} },
		"X1 action carries a stranger": func(v map[string]any) {
			cmd(v)["run"] = map[string]any{"action": "toast", "message": "hi", "theme": "x"}
		},
		"X1 run is a string":         func(v map[string]any) { cmd(v)["run"] = "setTheme" },
		"X1 scope":                   func(v map[string]any) { cmd(v)["scope"] = "everywhere" },
		"X1 empty title":             func(v map[string]any) { cmd(v)["title"] = " " },
		"X5 handler without an app":  func(v map[string]any) { cmd(v)["run"] = map[string]any{"handler": true} },
		"X1 handler with a stranger": func(v map[string]any) { withApp(v); cmd(v)["run"] = map[string]any{"handler": true, "action": "toast"} },
		"X2 locked key":              func(v map[string]any) { contrib(v)["keybindings"].([]any)[0].(map[string]any)["key"] = "ctrl+k" },
		"X2 other package's command": func(v map[string]any) {
			contrib(v)["keybindings"].([]any)[0].(map[string]any)["command"] = "core.search"
		},
		"X2 undeclared own command": func(v map[string]any) {
			contrib(v)["keybindings"].([]any)[0].(map[string]any)["command"] = "paper-night.gone"
		},
		"X2 no key":                     func(v map[string]any) { contrib(v)["keybindings"].([]any)[0].(map[string]any)["key"] = "+ +" },
		"X2 unknown keybinding field":   func(v map[string]any) { contrib(v)["keybindings"].([]any)[0].(map[string]any)["allowInInput"] = true },
		"X3 theme id not the package's": func(v map[string]any) { contrib(v)["themes"].([]any)[0].(map[string]any)["id"] = "futuristic-noir" },
		"X3 theme file escapes":         func(v map[string]any) { contrib(v)["themes"].([]any)[0].(map[string]any)["file"] = "../theme.json" },
		"X3 theme file absolute":        func(v map[string]any) { contrib(v)["themes"].([]any)[0].(map[string]any)["file"] = "/etc/passwd.json" },
		"X3 theme file is not json":     func(v map[string]any) { contrib(v)["themes"].([]any)[0].(map[string]any)["file"] = "theme.js" },
		"X3 theme file twice": func(v map[string]any) {
			c := contrib(v)
			c["themes"] = append(c["themes"].([]any), map[string]any{"id": "paper-night.light", "file": "theme.json"})
		},
		"X6 too many themes": func(v map[string]any) {
			l := []any{}
			for i := 0; i < 17; i++ {
				l = append(l, map[string]any{"id": "paper-night.t" + strings.Repeat("a", i), "file": strings.Repeat("a", i+1) + ".json"})
			}
			contrib(v)["themes"] = l
		},
		"X4 link without an app": func(v map[string]any) {
			contrib(v)["links"] = []any{map[string]any{"app": "paper-night", "params": []any{map[string]any{"name": "tag"}}}}
		},
		"unknown contributes key":             func(v map[string]any) { contrib(v)["hooks"] = []any{} },
		"unknown command key":                 func(v map[string]any) { cmd(v)["callback"] = "x" },
		"platform fact inside a contribution": func(v map[string]any) { cmd(v)["level"] = "privileged" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			for _, origin := range []string{OriginFirstParty, OriginPublished} {
				if _, err := ParseManifest(packJSON(t, mutate), origin); !errors.Is(err, ErrInvalidManifest) {
					t.Fatalf("%s: want ErrInvalidManifest, got %v", origin, err)
				}
			}
		})
	}
}

func TestLinksNeedTheOwnAppAndNoExpressions(t *testing.T) {
	link := func(v map[string]any) map[string]any { return contrib(v)["links"].([]any)[0].(map[string]any) }
	good := func(v map[string]any) {
		withApp(v)
		contrib(v)["links"] = []any{map[string]any{
			"app": "paper-night", "params": []any{map[string]any{"name": "note", "context": "noteId"}, map[string]any{"name": "tag"}},
			"launch": map[string]any{"windowTitle": "Notes", "requireAny": []any{"note"}, "context": map[string]any{"noteId": "param:note", "pinned": true}},
		}}
	}
	if _, err := ParseManifest(packJSON(t, good), OriginPublished); err != nil {
		t.Fatal(err)
	}
	bad := map[string]func(map[string]any){
		"another app":     func(v map[string]any) { good(v); link(v)["app"] = "texture" },
		"shell parameter": func(v map[string]any) { good(v); link(v)["params"] = []any{map[string]any{"name": "doc"}} },
		"parameter twice": func(v map[string]any) {
			good(v)
			link(v)["params"] = []any{map[string]any{"name": "a"}, map[string]any{"name": "a"}}
		},
		"no parameters":         func(v map[string]any) { good(v); link(v)["params"] = []any{} },
		"requireAny undeclared": func(v map[string]any) { good(v); link(v)["launch"].(map[string]any)["requireAny"] = []any{"nope"} },
		"context reads undeclared": func(v map[string]any) {
			good(v)
			link(v)["launch"].(map[string]any)["context"] = map[string]any{"x": "param:nope"}
		},
		"context expression": func(v map[string]any) {
			good(v)
			link(v)["launch"].(map[string]any)["context"] = map[string]any{"x": map[string]any{"eval": "1"}}
		},
		"context key shape": func(v map[string]any) {
			good(v)
			link(v)["launch"].(map[string]any)["context"] = map[string]any{"X-y": 1}
		},
		"two rules for one app": func(v map[string]any) { good(v); c := contrib(v); c["links"] = append(c["links"].([]any), link(v)) },
	}
	for name, mutate := range bad {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseManifest(packJSON(t, mutate), OriginPublished); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("want ErrInvalidManifest, got %v", err)
			}
		})
	}
}

func TestHandlerCommandsAcceptedWithAnApp(t *testing.T) {
	_, err := ParseManifest(packJSON(t, func(v map[string]any) {
		withApp(v)
		contrib(v)["commands"].([]any)[0].(map[string]any)["run"] = map[string]any{"handler": true}
	}), OriginPublished)
	if err != nil {
		t.Fatal(err)
	}
}

func TestKeyNormalizationMatchesTheShell(t *testing.T) {
	for in, want := range map[string]string{"cmd+shift+n": "Mod+Shift+N", "Ctrl+Alt+x": "Mod+Alt+X", "meta+k": "Mod+K", "escape": "Escape", "+": ""} {
		if got := NormalizeKey(in); got != want {
			t.Errorf("NormalizeKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// The behaviour-surface hash covers contributions, ignores their authoring
// order, and leaves every package without them unchanged.
func TestSurfaceHashCoversContributes(t *testing.T) {
	hash := func(mutate func(map[string]any)) string {
		m, err := ParseManifest(packJSON(t, mutate), OriginPublished)
		if err != nil {
			t.Fatal(err)
		}
		h, err := m.BehaviourSurface(MapSource(map[string]string{"theme.json": "{}"}))
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	base := hash(nil)
	if base != hash(nil) {
		t.Fatal("unstable")
	}
	for name, mutate := range map[string]func(map[string]any){
		"command target": func(v map[string]any) {
			contrib(v)["commands"].([]any)[0].(map[string]any)["run"] = map[string]any{"action": "openApp", "app": "files"}
		},
		"key":      func(v map[string]any) { contrib(v)["keybindings"].([]any)[0].(map[string]any)["key"] = "mod+shift+m" },
		"theme id": func(v map[string]any) { contrib(v)["themes"].([]any)[0].(map[string]any)["id"] = "paper-night.other" },
	} {
		if hash(mutate) == base {
			t.Errorf("%s did not move the hash", name)
		}
	}
	m, err := ParseManifest(packJSON(t, func(v map[string]any) { contrib(v)["commands"] = []any{}; contrib(v)["keybindings"] = []any{} }), OriginPublished)
	if err != nil {
		t.Fatal(err)
	}
	if raw, _ := m.surfaceContributes(); raw == nil {
		t.Fatal("a theme is still a contribution")
	}
	empty := Manifest{}
	if raw, _ := empty.surfaceContributes(); raw != nil {
		t.Fatal("no contributions must leave the hash input unchanged")
	}
}

func TestMenusNameTheOwnCommands(t *testing.T) {
	menu := func(v map[string]any) map[string]any { return contrib(v)["menus"].([]any)[0].(map[string]any) }
	good := func(v map[string]any) {
		contrib(v)["menus"] = []any{map[string]any{"target": "file", "command": "paper-night.use", "group": "theme", "order": 2, "when": "target.ext == json"}}
	}
	if _, err := ParseManifest(packJSON(t, good), OriginPublished); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(map[string]any){
		"X1 another package's command": func(v map[string]any) { good(v); menu(v)["command"] = "core.search" },
		"X1 undeclared command":        func(v map[string]any) { good(v); menu(v)["command"] = "paper-night.gone" },
		"target shape":                 func(v map[string]any) { good(v); menu(v)["target"] = "File Row" },
		"group shape":                  func(v map[string]any) { good(v); menu(v)["group"] = "A B" },
		"order is a string":            func(v map[string]any) { good(v); menu(v)["order"] = "first" },
		"callback member":              func(v map[string]any) { good(v); menu(v)["onSelect"] = "x" },
		"same entry twice":             func(v map[string]any) { good(v); c := contrib(v); c["menus"] = append(c["menus"].([]any), menu(v)) },
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseManifest(packJSON(t, mutate), OriginPublished); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("want ErrInvalidManifest, got %v", err)
			}
		})
	}
}

func TestLinkHomeIsFirstPartyAndNeedsNoParams(t *testing.T) {
	home := func(v map[string]any) {
		withApp(v)
		contrib(v)["links"] = []any{map[string]any{"app": "paper-night", "params": []any{}, "launch": map[string]any{"home": true, "windowTitle": "Home", "context": map[string]any{"preview": true}}}}
	}
	if _, err := ParseManifest(packJSON(t, home), OriginFirstParty); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseManifest(packJSON(t, home), OriginPublished); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("a published package must not claim the home, got %v", err)
	}
	noHome := func(v map[string]any) {
		withApp(v)
		contrib(v)["links"] = []any{map[string]any{"app": "paper-night", "params": []any{}}}
	}
	if _, err := ParseManifest(packJSON(t, noHome), OriginFirstParty); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("a link without parameters or home must be refused, got %v", err)
	}
}

// An app-less manifest that only opens files, or whose theme takes another
// package's namespace, is refused; neither app nor contributes stays refused.
func TestPackRefusals(t *testing.T) {
	cases := map[string]func(map[string]any){
		"opens without an app": func(v map[string]any) {
			contrib(v)["opens"] = []any{map[string]any{"media_type": "x/y", "extensions": []any{"zzq"}}}
		},
		"theme in a stranger's name":  func(v map[string]any) { contrib(v)["themes"].([]any)[0].(map[string]any)["id"] = "other.dark" },
		"neither app nor contributes": func(v map[string]any) { delete(v, "contributes") },
	}
	for name, mutate := range cases {
		if _, err := ParseManifest(packJSON(t, mutate), OriginPublished); !errors.Is(err, ErrInvalidManifest) {
			t.Errorf("%s: must be refused, got %v", name, err)
		}
	}
}
