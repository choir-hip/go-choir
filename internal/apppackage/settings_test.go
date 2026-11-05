package apppackage

import (
	"errors"
	"testing"
)

func settingsPack(sections ...any) func(map[string]any) {
	return func(v map[string]any) { v["contributes"] = map[string]any{"settings": sections} }
}

func field(key, typ string, def any, extra map[string]any) map[string]any {
	f := map[string]any{"key": key, "type": typ, "default": def, "label": "Label " + key}
	for k, x := range extra {
		f[k] = x
	}
	return f
}

func section(name string, fields ...any) map[string]any {
	return map[string]any{"section": name, "fields": fields}
}

func TestSettingsPackAccepted(t *testing.T) {
	m, err := ParseManifest(packJSON(t, settingsPack(section("Notes",
		field("autosave", "boolean", true, nil),
		field("font", "string", "serif", map[string]any{"enum": []any{"serif", "mono"}}),
		field("size", "number", 14, nil),
	))), OriginPublished)
	if err != nil {
		t.Fatal(err)
	}
	f, ok := Field(m.Contributes.Settings, "font")
	if !ok || f.DefaultValue() != "serif" {
		t.Fatalf("field = %+v %v", f, ok)
	}
	if pkg, key, ok := SplitSettingKey(SettingPreferenceKey("paper-night", "font")); !ok || pkg != "paper-night" || key != "font" {
		t.Fatalf("split = %s %s %v", pkg, key, ok)
	}
	for _, bad := range []string{"theme", "pkg.", "pkg.x", "pkg.x.", "pkg.x.A", "pkg.x.a.b", "other.x.y"} {
		if _, _, ok := SplitSettingKey(bad); ok {
			t.Errorf("SplitSettingKey(%q) accepted", bad)
		}
	}
}

// G1, G2
func TestSettingsRefusals(t *testing.T) {
	long := make([]any, 33)
	for i := range long {
		long[i] = "o" + string(rune('a'+i%26)) + string(rune('a'+i/26))
	}
	cases := map[string]func(map[string]any){
		"unknown type":              settingsPack(section("S", field("a", "object", map[string]any{}, nil))),
		"default of another type":   settingsPack(section("S", field("a", "boolean", "yes", nil))),
		"no default":                settingsPack(section("S", map[string]any{"key": "a", "type": "boolean", "label": "A"})),
		"enum on a boolean":         settingsPack(section("S", field("a", "boolean", true, map[string]any{"enum": []any{"x"}}))),
		"default outside the enum":  settingsPack(section("S", field("a", "string", "z", map[string]any{"enum": []any{"x", "y"}}))),
		"enum option twice":         settingsPack(section("S", field("a", "string", "x", map[string]any{"enum": []any{"x", "x"}}))),
		"enum too long":             settingsPack(section("S", field("a", "string", "oaa", map[string]any{"enum": long}))),
		"key with a dot":            settingsPack(section("S", field("a.b", "boolean", true, nil))),
		"key upper case":            settingsPack(section("S", field("Aa", "boolean", true, nil))),
		"key twice":                 settingsPack(section("S", field("a", "boolean", true, nil), field("a", "boolean", false, nil))),
		"key twice across sections": settingsPack(section("S", field("a", "boolean", true, nil)), section("T", field("a", "boolean", true, nil))),
		"section twice":             settingsPack(section("S", field("a", "boolean", true, nil)), section("S", field("b", "boolean", true, nil))),
		"no fields":                 settingsPack(section("S")),
		"no label":                  settingsPack(section("S", map[string]any{"key": "a", "type": "boolean", "default": true})),
		"unknown field member":      settingsPack(section("S", field("a", "boolean", true, map[string]any{"min": 1}))),
		"unknown section member":    settingsPack(map[string]any{"section": "S", "fields": []any{field("a", "boolean", true, nil)}, "icon": "x"}),
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseManifest(packJSON(t, mutate), OriginPublished); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("want ErrInvalidManifest, got %v", err)
			}
		})
	}
}

// G3 at the field.
func TestSettingFieldCheck(t *testing.T) {
	enum := SettingField{Type: SettingString, Enum: []string{"a", "b"}}
	for in, ok := range map[string]bool{`"a"`: true, `"c"`: false, `1`: false, `null`: false, `"a" "b"`: false} {
		if _, err := enum.Check([]byte(in)); (err == nil) != ok {
			t.Errorf("enum.Check(%s) = %v", in, err)
		}
	}
	num := SettingField{Type: SettingNumber}
	for in, ok := range map[string]bool{`1`: true, `-2.5`: true, `1e999`: false, `"1"`: false, `true`: false} {
		if _, err := num.Check([]byte(in)); (err == nil) != ok {
			t.Errorf("num.Check(%s) = %v", in, err)
		}
	}
	if SettingsOf([]byte(`{"contributes":{"settings":[{"section":"S","fields":[{"key":"a","type":"boolean","default":true,"label":"A"}]}]}}`))[0].Fields[0].Key != "a" {
		t.Error("SettingsOf")
	}
	if SettingsOf([]byte(`not json`)) != nil || FirstPartySettings("no-such-package") != nil {
		t.Error("an unreadable manifest or unknown package declares none")
	}
}

func TestSettingsMoveTheSurfaceHash(t *testing.T) {
	hash := func(def any) string {
		m, err := ParseManifest(packJSON(t, settingsPack(section("S", field("a", "boolean", def, nil)))), OriginPublished)
		if err != nil {
			t.Fatal(err)
		}
		h, err := m.BehaviourSurface(MapSource(nil))
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	if hash(true) == hash(false) || hash(true) != hash(true) {
		t.Fatal("a changed default must move the hash and a repeat must not")
	}
}
