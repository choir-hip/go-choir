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
package apppackage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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

	ManifestSchemaV1 = "choir.app-package/v1"
	OriginFirstParty = "first_party"
	AppABISvelteV1   = "choir.app-frontend/svelte-inline/v1"
)

// ErrInvalidManifest refuses a manifest that is not choir.app-package/v1.
var ErrInvalidManifest = errors.New("app package: invalid manifest")

var (
	// A package name is also the app desk's name (desk:<name>), so it follows
	// objectgraph.ValidPackageName: 2 to 32 characters.
	versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$`)
	aliasPattern   = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)
	// A shell name no package may take (the static app registry is checked by
	// the frontend script, which reads it).
	shellNames = map[string]bool{
		"desktop": true, "shell": true, "package": true, "packages": true,
		"app-packages": true, "system": true, "trace": true, "terminal": true,
		"features": true, "choir": true, "owner": true,
	}
)

// Manifest is choir.app-package/v1 as P1 fixes it (design section 3.1). The
// window, launcher, auth and theme fields of ChoirAppDefinition are carried as
// opaque JSON: the frontend check script owns their shape.
type Manifest struct {
	Schema    string          `json:"schema"`
	Name      string          `json:"name"`
	Version   string          `json:"version"`
	Origin    string          `json:"origin"`
	Display   Display         `json:"display"`
	App       App             `json:"app"`
	Opens     []Opens         `json:"opens"`
	Kinds     []Kind          `json:"kinds"`
	UsesKinds []string        `json:"uses_kinds"`
	Desk      json.RawMessage `json:"desk"`
	Backend   json.RawMessage `json:"backend"`
	Grants    []any           `json:"grants"`
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
}

type Opens struct {
	MediaType  string   `json:"media_type"`
	Extensions []string `json:"extensions"`
}

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

// ParseManifest decodes a manifest strictly (unknown fields and trailing data
// are refused) and validates it.
func ParseManifest(raw []byte) (Manifest, error) {
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
	if err := m.Validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidManifest, fmt.Sprintf(format, args...))
}

func isNull(raw json.RawMessage) bool {
	t := bytes.TrimSpace(raw)
	return len(t) == 0 || string(t) == "null"
}

// Validate enforces every v1 rule that does not need the static app registry.
func (m Manifest) Validate() error {
	if m.Schema != ManifestSchemaV1 {
		return invalid("schema %q is not %s", m.Schema, ManifestSchemaV1)
	}
	if err := checkName(m.Name); err != nil {
		return err
	}
	if !versionPattern.MatchString(m.Version) {
		return invalid("version %q is not MAJOR.MINOR.PATCH", m.Version)
	}
	if m.Origin != OriginFirstParty {
		return invalid("origin %q is not %s (installed packages are P5)", m.Origin, OriginFirstParty)
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
	if m.App.ABI != AppABISvelteV1 {
		return invalid("app.abi %q is not %s", m.App.ABI, AppABISvelteV1)
	}
	if err := checkEntry(m.App.Entry); err != nil {
		return invalid("app.entry: %v", err)
	}
	seenAlias := map[string]bool{m.Name: true}
	for _, a := range m.App.Aliases {
		if !aliasPattern.MatchString(a) || shellNames[a] {
			return invalid("app.aliases: %q is not a valid alias", a)
		}
		if seenAlias[a] {
			return invalid("app.aliases: %q is repeated or equals the package name", a)
		}
		seenAlias[a] = true
	}
	seenExt := map[string]bool{}
	for i, o := range m.Opens {
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
	if !isNull(m.Desk) {
		return invalid("desk must be null before P4")
	}
	if !isNull(m.Backend) {
		return invalid("backend must be null before P6")
	}
	if len(m.Grants) != 0 {
		return invalid("grants are not accepted before the egress grant model")
	}
	for _, u := range m.UsesKinds {
		if strings.TrimSpace(u) == "" || strings.ContainsAny(u, " \t\r\n:/\\") {
			return invalid("uses_kinds: %q is not a kind name", u)
		}
	}
	return m.validateKinds()
}

func checkName(name string) error {
	if !objectgraph.ValidPackageName(name) {
		return invalid("name %q must match ^[a-z][a-z0-9-]{1,31}$ and not be a built-in desk", name)
	}
	if shellNames[name] {
		return invalid("name %q is reserved by the shell", name)
	}
	if _, ok := deskmanifest.Lookup(name); ok {
		return invalid("name %q is a desk; desk:%s would not be the package's own", name, name)
	}
	return nil
}

func checkEntry(p string) error {
	switch {
	case p == "":
		return errors.New("empty path")
	case strings.HasPrefix(p, "/"), strings.Contains(p, `\`), strings.ContainsRune(p, 0), len(p) > 256:
		return fmt.Errorf("%q is not a clean relative path", p)
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." || strings.HasPrefix(part, ".") {
			return fmt.Errorf("%q is not a clean relative path", p)
		}
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
