// Package apppackage is the app package seam of doctrine I27 (P2b of
// docs/design/app-packages-replan-2026-10-24.md): the package manifest, the
// kind schema subset, and the registration of a package's data kinds with the
// object graph's write gate.
//
// A package declares its kinds in choir-package.json (choir.app.<package>.<kind>)
// with their schemas. First-party manifests are read at build time by
// frontend/scripts/gen-package-kinds.mjs into firstparty_gen.go, and
// RegisterFirstParty registers them at boot through
// objectgraph.RegisterPackageKind. Installed packages (P5) will call the same
// registration from the fold of install events.
//
// The manifest is choir.package/v2 (MA2.3 of
// docs/design/modular-architecture-plan-2026-11-04.md section 5). The v1
// reader is deleted; there is no shim.
package apppackage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

const (
	// ManifestFile is the manifest's name at the root of a package directory.
	ManifestFile = "choir-package.json"

	ManifestSchemaV2 = "choir.package/v2"
	AppABISvelteV1   = "choir.app-frontend/svelte-inline/v1"
	// DefaultRequiresSDK is the SDK range of a manifest that names none: the
	// shell SDK the svelte-inline/v1 packages were written against
	// (frontend/src/lib/app-sdk.ts SDK_VERSION 1.0.0).
	DefaultRequiresSDK = "^1.0"

	// MaxManifestBytes bounds a manifest before it is parsed.
	MaxManifestBytes = 256 << 10
	maxJSONDepth     = 24
)

// Origins. The origin of a package is a fact of how the platform came by it,
// never a field the manifest states: a manifest that carries one is refused
// (ErrPublisherDeclared).
const (
	// OriginFirstParty is a package built into the platform's own tree.
	OriginFirstParty = "first_party"
	// OriginPublished is a package that came through the Library from a
	// publisher.
	OriginPublished = "published"
)

// ErrInvalidManifest refuses a manifest that is not a valid choir.package/v2.
var ErrInvalidManifest = errors.New("app package: invalid manifest")

// ErrPublisherDeclared refuses a manifest that states a fact only the
// platform may set (its origin, level, class, hash or import set). It wraps
// ErrInvalidManifest.
var ErrPublisherDeclared = fmt.Errorf("%w: publisher-declared platform field", ErrInvalidManifest)

// ErrPrivilegedGrant refuses a privileged grant from a non-first-party origin.
// It wraps ErrInvalidManifest.
var ErrPrivilegedGrant = fmt.Errorf("%w: privileged grant from a non-first-party origin", ErrInvalidManifest)

// ErrDeskNeedsSandbox refuses a desk-bearing package from a non-first-party
// origin while the platform lacks the Phase 6 isolation (R24). It wraps
// ErrInvalidManifest.
var ErrDeskNeedsSandbox = fmt.Errorf("%w: desk-bearing package from a non-first-party origin needs the Phase 6 sandbox", ErrInvalidManifest)

var (
	// A package name is also the app desk's name (desk:<name>), so it follows
	// objectgraph.ValidPackageName: 2 to 32 characters.
	versionPattern  = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$`)
	aliasPattern    = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)
	sdkRangePattern = regexp.MustCompile(`^\^(0|[1-9][0-9]{0,3})\.(0|[1-9][0-9]{0,3})$`)
	scheduleIDRe    = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	pathSegmentRe   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
)

// Manifest is choir.package/v2. The window, launcher, auth and theme fields of
// ChoirAppDefinition are carried as opaque JSON: the frontend check script
// owns their shape.
//
// Origin is not a manifest field. ParseManifest sets it from the caller, who
// knows how the platform came by the bytes.
type Manifest struct {
	Schema       string       `json:"schema"`
	Name         string       `json:"name"`
	Version      string       `json:"version"`
	Display      Display      `json:"display"`
	App          *App         `json:"app,omitempty"`
	Opens        []Opens      `json:"opens"`
	Kinds        []Kind       `json:"kinds"`
	UsesKinds    []string     `json:"uses_kinds"`
	Contributes  Contributes  `json:"contributes"`
	Capabilities []Capability `json:"capabilities"`

	Origin string `json:"-"`
}

// Contributes is what the package adds to the computer beside its window.
type Contributes struct {
	Desks       []Desk       `json:"desks"`
	Commands    []Command    `json:"commands"`
	Keybindings []Keybinding `json:"keybindings"`
	Themes      []ThemeFile  `json:"themes"`
	Links       []Link       `json:"links"`
	Menus       []Menu       `json:"menus"`
	// Settings are the sections of the Settings app the package adds (S6).
	Settings []SettingsSection `json:"settings"`
}

// Desk is one desk the package contributes (a persistent root agent desk,
// doctrine I26). Prompt, Code and Tools are source files of the package; the
// platform builds from source and a release carries no binary.
type Desk struct {
	Name string `json:"name"`
	// Prompt is the desk's prompt file (.md or .txt). A prompt is behaviour.
	Prompt string `json:"prompt"`
	// Code is the desk's Go source for its yaegi cell (.go), optional.
	Code string `json:"code,omitempty"`
	// Tools are further Go source files the desk's cells call (.go).
	Tools     []string   `json:"tools,omitempty"`
	Schedules []Schedule `json:"schedules,omitempty"`
	// Accepts is what the desk takes from the prompt bar. Its shape is fixed
	// by MA4.8; MA2.3 decodes it as a bounded list and hashes it.
	Accepts json.RawMessage `json:"accepts,omitempty"`
}

// Schedule is a standing intent of a desk.
type Schedule struct {
	ID           string `json:"id"`
	EverySeconds int    `json:"every_seconds"`
}

// Capability is one request in the package's ceiling: a closed grant name and
// its arguments (grants.go). The level of a grant is the catalogue's, never
// the manifest's.
type Capability struct {
	Grant string   `json:"grant"`
	Args  []string `json:"args,omitempty"`
}

type Display struct {
	Title       string `json:"title"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
}

type App struct {
	Entry    string          `json:"entry"`
	ABI      string          `json:"abi"`
	Aliases  []string        `json:"aliases"`
	Launcher json.RawMessage `json:"launcher"`
	Window   json.RawMessage `json:"window"`
	Auth     json.RawMessage `json:"auth"`
	Theme    json.RawMessage `json:"theme"`
	// RequiresSDK is the shell SDK range the package was written against,
	// "^MAJOR.MINOR" (runtime install, S12). Absent means DefaultRequiresSDK.
	RequiresSDK string `json:"requires_sdk,omitempty"`
}

type Opens struct {
	MediaType  string   `json:"media_type"`
	Extensions []string `json:"extensions"`
	// Default false offers the package under Open with for these extensions
	// without making it their default opener; absent means true.
	Default *bool `json:"default,omitempty"`
}

// IsDefault reports whether the entry routes its extensions to the package by
// default (true unless "default": false).
func (o Opens) IsDefault() bool { return o.Default == nil || *o.Default }

// Kind is one data kind the package declares.
type Kind struct {
	Kind                string          `json:"kind"`
	Identity            string          `json:"identity"`
	Versioned           bool            `json:"versioned"`
	OwnerEdits          bool            `json:"owner_edits"`
	SchemaVersion       int             `json:"schema_version"`
	ReadsSchemaVersions []int           `json:"reads_schema_versions"`
	Schema              json.RawMessage `json:"schema"`
}

// ParseManifest decodes a manifest strictly and validates it for origin (how
// the platform came by the bytes). It refuses, before any field is read:
//
//   - a document over MaxManifestBytes or nested past maxJSONDepth;
//   - a duplicate key at any depth (two readers would disagree on the winner);
//   - a key that is not exactly a declared field (Go's decoder folds case, so
//     "Name" is checked by hand), with the platform's own facts named apart
//     (ErrPublisherDeclared);
//   - unknown fields and trailing data.
func ParseManifest(raw []byte, origin string) (Manifest, error) {
	if len(raw) > MaxManifestBytes {
		return Manifest{}, invalid("manifest exceeds %d bytes", MaxManifestBytes)
	}
	if err := scanJSON(raw); err != nil {
		return Manifest{}, err
	}
	if err := checkKeys(raw, reflect.TypeOf(Manifest{}), "manifest"); err != nil {
		return Manifest{}, err
	}
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("%w: %v", ErrInvalidManifest, err)
	}
	var extra json.RawMessage
	if dec.More() || dec.Decode(&extra) == nil {
		return Manifest{}, fmt.Errorf("%w: trailing data after the JSON value", ErrInvalidManifest)
	}
	m.Origin = origin
	if err := m.Validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidManifest, fmt.Sprintf(format, args...))
}

// Validate enforces every v2 rule that does not need the static app registry
// or the package's files, on the running platform (CurrentPlatform).
func (m Manifest) Validate() error { return m.ValidateOn(CurrentPlatform()) }

// IsBuiltinPackage reports whether name is an in-tree first-party package. An
// installed package of that name replaces the built-in app in the shell, so an
// install of one is allowed only from the installing owner's own track.
func IsBuiltinPackage(name string) bool {
	for _, n := range builtinPackageNames {
		if n == name {
			return true
		}
	}
	return false
}

// ShellAppNames lists the names a published package may not take.
func ShellAppNames() []string { return append([]string{}, shellAppNames...) }

// ValidateOn is Validate on a stated platform.
func (m Manifest) ValidateOn(p Platform) error {
	if m.Schema != ManifestSchemaV2 {
		return invalid("schema %q is not %s", m.Schema, ManifestSchemaV2)
	}
	if m.Origin != OriginFirstParty && m.Origin != OriginPublished {
		return invalid("origin %q is not %s or %s", m.Origin, OriginFirstParty, OriginPublished)
	}
	if err := checkName(m.Name); err != nil {
		return err
	}
	// B3: the -candidate suffix names a self-development candidate package
	// (<pkg>-candidate); no published package takes it.
	if m.Origin != OriginFirstParty && strings.HasSuffix(m.Name, "-candidate") {
		return invalid("name %q uses the reserved -candidate suffix", m.Name)
	}
	// A published package never takes a name the shell holds as an app id or
	// reserves: installing it would shadow the built-in app (review B2).
	if m.Origin != OriginFirstParty {
		for _, n := range shellAppNames {
			if m.Name == n {
				return invalid("name %q is a shell app name; a published package cannot take it", m.Name)
			}
		}
	}
	if !versionPattern.MatchString(m.Version) {
		return invalid("version %q is not MAJOR.MINOR.PATCH", m.Version)
	}
	if strings.TrimSpace(m.Display.Title) == "" || utf8.RuneCountInString(m.Display.Title) > 60 {
		return invalid("display.title is required and at most 60 characters")
	}
	if utf8.RuneCountInString(m.Display.Icon) > 16 {
		return invalid("display.icon is at most 16 characters")
	}
	if utf8.RuneCountInString(m.Display.Description) > 200 {
		return invalid("display.description is at most 200 characters")
	}
	if m.App == nil && len(m.Contributes.Desks) == 0 && len(m.Kinds) == 0 && m.contributesNothingElse() {
		return invalid("the package contributes nothing: it needs an app, a desk, a kind, a command, a keybinding, a theme or a link")
	}
	if m.App != nil {
		if err := m.validateApp(); err != nil {
			return err
		}
	}
	seenExt := map[string]bool{}
	for i, o := range m.Opens {
		if m.App == nil {
			return invalid("opens needs an app to open the file")
		}
		if strings.TrimSpace(o.MediaType) == "" || strings.ContainsAny(o.MediaType, " \t\r\n") {
			return invalid("opens[%d].media_type is required and has no whitespace", i)
		}
		for _, e := range o.Extensions {
			if e == "" || e != strings.ToLower(e) || strings.ContainsAny(e, " \t\r\n/\\") {
				return invalid("opens[%d].extensions: %q is not a lower-case extension", i, e)
			}
			if seenExt[e] {
				return invalid("opens: extension %q is claimed twice", e)
			}
			seenExt[e] = true
		}
	}
	for _, u := range m.UsesKinds {
		if strings.TrimSpace(u) == "" || strings.ContainsAny(u, " \t\r\n:/\\") {
			return invalid("uses_kinds: %q is not a kind name", u)
		}
	}
	if err := m.validateDesks(); err != nil {
		return err
	}
	if err := m.validateContributes(); err != nil {
		return err
	}
	if err := m.validateCapabilities(); err != nil {
		return err
	}
	if m.Origin != OriginFirstParty {
		// R24: a desk is an autonomous agent even without code. Until the
		// platform reports the isolation, no other publisher's desk is
		// published or installed. This is a capability of the running
		// platform, not a switch.
		for _, c := range m.Capabilities {
			if spec, _ := lookupGrant(c.Grant); spec.level == LevelPrivileged {
				return fmt.Errorf("%w: %s", ErrPrivilegedGrant, c.Grant)
			}
		}
		if len(m.Contributes.Desks) > 0 && !p.AdmitsDesksFrom(m.Origin) {
			return fmt.Errorf("%w: %d desk(s) in package %q", ErrDeskNeedsSandbox, len(m.Contributes.Desks), m.Name)
		}
		// opens and aliases have no consumer for an installed package yet
		// (the shell builds no media routes or aliases from them), and the
		// clash checks that guard them exist only for in-tree packages. A
		// published manifest carries neither until they are wired (review N3).
		if len(m.Opens) > 0 || (m.App != nil && len(m.App.Aliases) > 0) {
			return invalid("opens and app.aliases are first-party only until an installed package's routes are wired")
		}
	}
	return m.validateKinds()
}

func (m Manifest) validateApp() error {
	a := m.App
	if a.ABI != AppABISvelteV1 {
		return invalid("app.abi %q is not %s", a.ABI, AppABISvelteV1)
	}
	if a.RequiresSDK != "" && !sdkRangePattern.MatchString(a.RequiresSDK) {
		return invalid("app.requires_sdk %q is not ^MAJOR.MINOR", a.RequiresSDK)
	}
	if err := checkPath(a.Entry, ".svelte"); err != nil {
		return invalid("app.entry: %v", err)
	}
	if err := a.validateAppBlock(); err != nil {
		return err
	}
	seenAlias := map[string]bool{m.Name: true}
	for _, al := range a.Aliases {
		if !aliasPattern.MatchString(al) || objectgraph.IsShellName(al) {
			return invalid("app.aliases: %q is not a valid alias", al)
		}
		if seenAlias[al] {
			return invalid("app.aliases: %q is repeated or equals the package name", al)
		}
		seenAlias[al] = true
	}
	return nil
}

const (
	maxDesks     = 8
	maxSchedules = 16
	maxTools     = 32
	maxAccepts   = 64
	// minScheduleSeconds is the floor of a standing intent's period.
	minScheduleSeconds = 60
	maxAcceptsBytes    = 32 << 10
)

func (m Manifest) validateDesks() error {
	desks := m.Contributes.Desks
	if len(desks) > maxDesks {
		return invalid("contributes.desks: at most %d desks", maxDesks)
	}
	names := map[string]bool{}
	for i, d := range desks {
		w := fmt.Sprintf("contributes.desks[%d]", i)
		// A desk is named by its package (app:<package>) or extends it
		// (app:<package>-<suffix>); it cannot take another package's name,
		// a system desk's or a shell name (ValidPackageName holds the set).
		if d.Name != m.Name && !strings.HasPrefix(d.Name, m.Name+"-") {
			return invalid("%s.name %q is not the package name or %s-<suffix>", w, d.Name, m.Name)
		}
		if strings.HasSuffix(d.Name, "-") || strings.Contains(d.Name, "--") || !objectgraph.ValidPackageName(d.Name) {
			return invalid("%s.name %q is not a valid desk name", w, d.Name)
		}
		if names[d.Name] {
			return invalid("%s.name %q is declared twice", w, d.Name)
		}
		names[d.Name] = true
		files := map[string]bool{}
		file := func(field, p, ext string) error {
			if err := checkPath(p, ext); err != nil {
				return invalid("%s.%s: %v", w, field, err)
			}
			if files[strings.ToLower(p)] {
				return invalid("%s.%s: %q is named twice", w, field, p)
			}
			files[strings.ToLower(p)] = true
			return nil
		}
		if d.Prompt == "" {
			return invalid("%s.prompt is required: a desk's prompt is its behaviour", w)
		}
		if err := checkPathAny(d.Prompt, ".md", ".txt"); err != nil {
			return invalid("%s.prompt: %v", w, err)
		}
		files[strings.ToLower(d.Prompt)] = true
		if d.Code != "" {
			if err := file("code", d.Code, ".go"); err != nil {
				return err
			}
		}
		if len(d.Tools) > maxTools {
			return invalid("%s.tools: at most %d", w, maxTools)
		}
		for _, t := range d.Tools {
			if err := file("tools", t, ".go"); err != nil {
				return err
			}
		}
		if len(d.Schedules) > maxSchedules {
			return invalid("%s.schedules: at most %d", w, maxSchedules)
		}
		seen := map[string]bool{}
		wakesPerHour := 0.0
		for _, s := range d.Schedules {
			if !scheduleIDRe.MatchString(s.ID) || seen[s.ID] {
				return invalid("%s.schedules: %q is not a unique schedule id", w, s.ID)
			}
			seen[s.ID] = true
			if s.EverySeconds < minScheduleSeconds || s.EverySeconds > 31*24*3600 {
				return invalid("%s.schedules[%s].every_seconds must be from %d to 2678400", w, s.ID, minScheduleSeconds)
			}
			wakesPerHour += 3600 / float64(s.EverySeconds)
		}
		if wakesPerHour > float64(deskmanifest.MaxWakesPerHour) {
			return invalid("%s.schedules: together they wake the desk %.0f times an hour; at most %d", w, wakesPerHour, deskmanifest.MaxWakesPerHour)
		}
		if err := checkAccepts(d.Accepts); err != nil {
			return invalid("%s.accepts: %v", w, err)
		}
	}
	return nil
}

func checkAccepts(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	if len(raw) > maxAcceptsBytes {
		return fmt.Errorf("over %d bytes", maxAcceptsBytes)
	}
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil || list == nil {
		return errors.New("must be a list")
	}
	if len(list) > maxAccepts {
		return fmt.Errorf("at most %d entries", maxAccepts)
	}
	for _, e := range list {
		t := bytes.TrimSpace(e)
		if len(t) == 0 || (t[0] != '{' && t[0] != '"') {
			return errors.New("each entry is a string or an object")
		}
	}
	return nil
}

// checkPath admits a clean relative path of plain segments with one of the
// extensions: no absolute path, no "..", no dot-file, no backslash, no NUL, no
// non-ASCII, no more than eight segments. Because only source extensions
// pass, a package cannot name a binary (red-team 7).
func checkPath(p string, exts ...string) error { return checkPathAny(p, exts...) }

func checkPathAny(p string, exts ...string) error {
	if p == "" {
		return errors.New("empty path")
	}
	if len(p) > 256 {
		return errors.New("path over 256 bytes")
	}
	parts := strings.Split(p, "/")
	if len(parts) > 8 {
		return fmt.Errorf("%q is nested deeper than 8 segments", p)
	}
	for _, part := range parts {
		if !pathSegmentRe.MatchString(part) || strings.Contains(part, "..") {
			return fmt.Errorf("%q is not a clean relative path", p)
		}
	}
	ok := false
	for _, e := range exts {
		if strings.HasSuffix(p, e) && len(p) > len(e) {
			ok = true
		}
	}
	if !ok {
		return fmt.Errorf("%q must end in %s (source only)", p, strings.Join(exts, " or "))
	}
	return nil
}

// checkName defers to the seam, which holds the whole reserved set (built-in
// writers, the desk manifest, frozen V1 spellings, the shell's names) so the
// manifest layer and the generated first-party table cannot disagree with it.
func checkName(name string) error {
	if !objectgraph.ValidPackageName(name) {
		return invalid("name %q must match ^[a-z][a-z0-9-]{1,31}$ and not be reserved (a desk, an agent role, a built-in writer or a shell name)", name)
	}
	return nil
}

func (m Manifest) validateKinds() error {
	seen := map[string]bool{}
	for i, k := range m.Kinds {
		if err := objectgraph.ValidPackageKindName(m.Name, objectgraph.ObjectKind(k.Kind)); err != nil {
			return invalid("kinds[%d]: %v", i, err)
		}
		if seen[k.Kind] {
			return invalid("kinds[%d]: %q is declared twice", i, k.Kind)
		}
		seen[k.Kind] = true
		switch k.Identity {
		case string(objectgraph.IdentityExternalKey), string(objectgraph.IdentityContentHash):
		default:
			return invalid("kinds[%d] %q: identity %q is not external_key or content_hash", i, k.Kind, k.Identity)
		}
		if k.OwnerEdits && k.Identity != string(objectgraph.IdentityExternalKey) {
			return invalid("kinds[%d] %q: owner_edits needs identity external_key (the owner writes by id)", i, k.Kind)
		}
		if k.SchemaVersion < 1 {
			return invalid("kinds[%d] %q: schema_version must be at least 1", i, k.Kind)
		}
		if !sort.IntsAreSorted(k.ReadsSchemaVersions) {
			return invalid("kinds[%d] %q: reads_schema_versions must be sorted", i, k.Kind)
		}
		reads := false
		for j, v := range k.ReadsSchemaVersions {
			if v < 1 || (j > 0 && k.ReadsSchemaVersions[j-1] == v) {
				return invalid("kinds[%d] %q: reads_schema_versions must be unique and at least 1", i, k.Kind)
			}
			if v == k.SchemaVersion {
				reads = true
			}
		}
		if !reads {
			return invalid("kinds[%d] %q: reads_schema_versions must include schema_version %d", i, k.Kind, k.SchemaVersion)
		}
		if _, err := CompileSchema(k.Schema); err != nil {
			return invalid("kinds[%d] %q: %v", i, k.Kind, err)
		}
	}
	return nil
}
