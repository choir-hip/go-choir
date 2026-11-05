package apppackage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

// contributes.settings (slice S6; system modularity design sections 2.2 and
// 2.6). A package declares sections of fields; the base Settings app renders
// them from the schema and the owner's values are stored as preferences named
// pkg.<package>.<key>. The shell custodian (mediastate) serves
// /api/preferences/{key} and takes only a key an installed or in-tree manifest
// declares, a value of the declared type, and a bounded size.
//
// Failure modes (tests in settings_test.go and internal/mediastate):
//
//	G1 a field with an unknown type, a default of another type, an enum on a
//	   non-string, or a default outside its enum: refused at the manifest;
//	G2 a key outside [a-z][a-z0-9_-]{0,39}, repeated within the package: refused
//	   (the preference name is pkg.<package>.<key>, so a dot would blur it);
//	G3 a value that is not the declared type, outside the enum, non-finite,
//	   over the size cap: refused at the route, nothing stored;
//	G4 a stored value that no longer fits the declaration (the package updated
//	   its field): read as the default, never as an error.

const (
	maxSettingSections = 16
	maxSettingFields   = 32
	maxSettingEnum     = 32
	// MaxSettingStringLen bounds a string setting and an enum option.
	MaxSettingStringLen = 1024
	maxSettingEnumLen   = 64
	// SettingPrefix starts every package setting's preference key.
	SettingPrefix = "pkg."
)

// Setting field types.
const (
	SettingBoolean = "boolean"
	SettingString  = "string"
	SettingNumber  = "number"
)

// SettingsSection is one section of the Settings app a package contributes.
type SettingsSection struct {
	Section string         `json:"section"`
	Fields  []SettingField `json:"fields"`
}

// SettingField is one setting: its key within the package, type, default,
// label, and for a string the closed set of values.
type SettingField struct {
	Key     string          `json:"key"`
	Type    string          `json:"type"`
	Default json.RawMessage `json:"default"`
	Label   string          `json:"label"`
	Enum    []string        `json:"enum,omitempty"`
}

var packageSegmentRe = regexp.MustCompile(`^[a-z][a-z0-9-]{1,39}$`)

var settingKeyRe = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,39}$`)

// SettingPreferenceKey is the preference key of a package's setting.
func SettingPreferenceKey(pkg, key string) string { return SettingPrefix + pkg + "." + key }

// SplitSettingKey reads pkg.<package>.<key>; ok is false for any other key.
func SplitSettingKey(pref string) (pkg, key string, ok bool) {
	rest, found := strings.CutPrefix(pref, SettingPrefix)
	if !found {
		return "", "", false
	}
	pkg, key, found = strings.Cut(rest, ".")
	if !found || !packageSegmentRe.MatchString(pkg) || !settingKeyRe.MatchString(key) {
		return "", "", false
	}
	return pkg, key, true
}

func (m Manifest) validateSettings() error {
	sections := m.Contributes.Settings
	if len(sections) > maxSettingSections {
		return invalid("contributes.settings: at most %d sections", maxSettingSections)
	}
	titles, keys := map[string]bool{}, map[string]bool{}
	for i, sec := range sections {
		w := fmt.Sprintf("contributes.settings[%d]", i)
		if strings.TrimSpace(sec.Section) == "" || tooLong(sec.Section, 60) {
			return invalid("%s.section is required and at most 60 characters", w)
		}
		if titles[sec.Section] {
			return invalid("%s.section %q is declared twice", w, sec.Section)
		}
		titles[sec.Section] = true
		if len(sec.Fields) == 0 || len(sec.Fields) > maxSettingFields {
			return invalid("%s.fields must list one to %d fields", w, maxSettingFields)
		}
		for j, f := range sec.Fields {
			fw := fmt.Sprintf("%s.fields[%d]", w, j)
			if !settingKeyRe.MatchString(f.Key) {
				return invalid("%s.key %q must match %s", fw, f.Key, settingKeyRe)
			}
			if keys[f.Key] {
				return invalid("%s.key %q is declared twice in the package", fw, f.Key)
			}
			keys[f.Key] = true
			if strings.TrimSpace(f.Label) == "" || tooLong(f.Label, 80) {
				return invalid("%s.label is required and at most 80 characters", fw)
			}
			if err := f.validateDeclaration(); err != nil {
				return invalid("%s: %v", fw, err)
			}
		}
	}
	return nil
}

func (f SettingField) validateDeclaration() error {
	switch f.Type {
	case SettingBoolean, SettingNumber:
		if len(f.Enum) != 0 {
			return fmt.Errorf("enum is for string fields")
		}
	case SettingString:
		if len(f.Enum) > maxSettingEnum {
			return fmt.Errorf("enum has at most %d options", maxSettingEnum)
		}
		seen := map[string]bool{}
		for _, o := range f.Enum {
			if o == "" || tooLong(o, maxSettingEnumLen) || seen[o] {
				return fmt.Errorf("enum option %q must be unique, non-empty and at most %d characters", o, maxSettingEnumLen)
			}
			seen[o] = true
		}
	default:
		return fmt.Errorf("type %q must be boolean, string or number", f.Type)
	}
	if len(bytes.TrimSpace(f.Default)) == 0 {
		return fmt.Errorf("default is required")
	}
	if _, err := f.Check(f.Default); err != nil {
		return fmt.Errorf("default: %v", err)
	}
	return nil
}

// Check validates a JSON value against the field's declared type, enum and
// size caps and returns the decoded value.
func (f SettingField) Check(raw json.RawMessage) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("not JSON: %v", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("trailing data after the value")
	}
	switch f.Type {
	case SettingBoolean:
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("must be a boolean")
		}
		return b, nil
	case SettingNumber:
		n, ok := v.(json.Number)
		if !ok {
			return nil, fmt.Errorf("must be a number")
		}
		x, err := n.Float64()
		if err != nil || math.IsNaN(x) || math.IsInf(x, 0) {
			return nil, fmt.Errorf("must be a finite number")
		}
		return x, nil
	case SettingString:
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("must be a string")
		}
		if tooLong(s, MaxSettingStringLen) {
			return nil, fmt.Errorf("is over %d characters", MaxSettingStringLen)
		}
		if len(f.Enum) > 0 {
			for _, o := range f.Enum {
				if o == s {
					return s, nil
				}
			}
			return nil, fmt.Errorf("must be one of the declared options")
		}
		return s, nil
	}
	return nil, fmt.Errorf("type %q is unknown", f.Type)
}

// DefaultValue is the field's default, decoded.
func (f SettingField) DefaultValue() any {
	v, _ := f.Check(f.Default)
	return v
}

// Field finds a declared field by key in the package's sections.
func Field(sections []SettingsSection, key string) (SettingField, bool) {
	for _, s := range sections {
		for _, f := range s.Fields {
			if f.Key == key {
				return f, true
			}
		}
	}
	return SettingField{}, false
}

// FirstPartySettings is the settings the in-tree package declares, from the
// generated table (frontend/scripts/gen-package-kinds.mjs).
func FirstPartySettings(pkg string) []SettingsSection {
	raw, ok := firstPartySettings[pkg]
	if !ok {
		return nil
	}
	var out []SettingsSection
	if json.Unmarshal([]byte(raw), &out) != nil {
		return nil
	}
	return out
}

// SettingsOf reads the settings sections of a manifest's raw bytes (an
// install record's manifest); an unreadable manifest declares none.
func SettingsOf(raw json.RawMessage) []SettingsSection {
	var m struct {
		Contributes struct {
			Settings []SettingsSection `json:"settings"`
		} `json:"contributes"`
	}
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return m.Contributes.Settings
}
