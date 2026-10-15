// Package apppackage is the S7 app-package seam: the versioned package
// manifest, the host build receipt, the pure lifecycle reducer, and the
// content-addressed build store
// (docs/design/s7-0-app-packages-2026-10-14.md).
//
// A package is a frontend module plus an optional backend service, with its
// own build recipe. Only source is published; the host builds a package once
// and any computer installs that build by digest, whatever its own
// divergence. Which packages a computer has installed is the fold of its
// package lifecycle events on the computer's tape; this package holds the
// pure rules, and internal/store records them.
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

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

const (
	// ManifestFile is the manifest's name at the root of a package source tree.
	ManifestFile = "choir-package.json"

	ManifestSchemaV1 = "choir.app-package/v1"
	FrontendABIV1    = "choir.app-frontend/v1"
	BackendABIV1     = "choir.app-backend/v1"

	BackendKindService = "service"

	// RecipeStaticV1 copies the declared outputs verbatim from the source
	// tree; it is the only recipe S7-0a builds.
	RecipeStaticV1 = "static/v1"
	// RecipeViteModuleV1 bundles one self-contained ES module; the host
	// builds it from S7-0c (design §8).
	RecipeViteModuleV1 = "vite-module/v1"
	// RecipeGoServiceV1 builds a static backend executable against the base
	// image; the host builds it from S7-2 (design §7).
	RecipeGoServiceV1 = "go-service/v1"

	DataStoreNone = "none"
	DataStoreKVV1 = "package_kv/v1"

	// DataScopeComputerTape: the package's data lives on the computer's
	// tape, so a pinned-head restore rebuilds it by replay.
	DataScopeComputerTape = "computer_tape"

	OnPurgeDeleteData = "delete_data"
	OnPurgeRetainData = "retain_data"
)

var (
	ErrInvalidManifest = errors.New("app package: invalid manifest")
	ErrSourceMismatch  = errors.New("app package: source tree does not match the manifest's source digest")
)

var (
	namePattern    = regexp.MustCompile(`^[a-z][a-z0-9-]{1,39}$`)
	versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$`)
	digestPattern  = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

var knownRecipes = map[string]bool{
	RecipeStaticV1:     true,
	RecipeViteModuleV1: true,
	RecipeGoServiceV1:  true,
}

// reservedNames are every static APP_REGISTRY id
// (frontend/src/lib/apps/registry.ts) plus shell and retired names. A
// package may never take one; TestReservedNamesCoverStaticRegistry keeps the
// list in step with the registry.
var reservedNames = map[string]bool{
	"files": true, "browser": true, "email": true, "compute-monitor": true,
	"pulse": true, "texture": true, "autopaper": true, "podcast": true,
	"image": true, "audio": true, "video": true, "pdf": true, "epub": true,
	"slides": true, "calendar": true, "content": true, "super-console": true,
	"settings": true,
	// Shell and retired names.
	"desktop": true, "shell": true, "package": true, "packages": true,
	"app-packages": true, "system": true, "trace": true, "terminal": true,
	"features": true, "choir": true,
}

// IsReservedName reports whether name is held by a static app or the shell.
func IsReservedName(name string) bool { return reservedNames[name] }

// IsDigest reports whether s is a "sha256:" + 64 lowercase hex digest.
func IsDigest(s string) bool { return digestPattern.MatchString(s) }

// Manifest is choir.app-package/v1.
type Manifest struct {
	Schema   string            `json:"schema"`
	Name     string            `json:"name"`
	Version  string            `json:"version"`
	Display  Display           `json:"display"`
	Source   SourceRef         `json:"source"`
	Frontend Frontend          `json:"frontend"`
	Backend  *Backend          `json:"backend"`
	Build    BuildSpec         `json:"build"`
	Data     DataSpec          `json:"data"`
	Restore  RestoreSpec       `json:"restore"`
	Grants   []json.RawMessage `json:"grants"`
}

type Display struct {
	Title       string `json:"title"`
	Icon        string `json:"icon,omitempty"`
	Description string `json:"description,omitempty"`
}

type SourceRef struct {
	Digest string `json:"digest"`
}

type Frontend struct {
	Entry  string     `json:"entry"`
	ABI    string     `json:"abi"`
	Window WindowSpec `json:"window"`
}

type WindowSpec struct {
	Singleton bool `json:"singleton,omitempty"`
	Width     int  `json:"width,omitempty"`
	Height    int  `json:"height,omitempty"`
}

type Backend struct {
	Kind  string `json:"kind"`
	Entry string `json:"entry"`
	ABI   string `json:"abi"`
}

type BuildSpec struct {
	Recipe  string   `json:"recipe"`
	Outputs []string `json:"outputs"`
}

type DataSpec struct {
	Store               string `json:"store"`
	SchemaVersion       int    `json:"schema_version,omitempty"`
	ReadsSchemaVersions []int  `json:"reads_schema_versions,omitempty"`
}

type RestoreSpec struct {
	DataScope string `json:"data_scope"`
	OnPurge   string `json:"on_purge"`
}

// ParseManifest decodes a manifest strictly (unknown fields and trailing
// data are refused) and validates it.
func ParseManifest(raw []byte) (Manifest, error) {
	var m Manifest
	if err := strictDecode(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("%w: %v", ErrInvalidManifest, err)
	}
	if err := m.Validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// MarshalManifest renders a manifest as indented JSON for a source tree. It
// does not validate; ParseManifest does on the way back in.
func MarshalManifest(m Manifest) ([]byte, error) {
	if m.Grants == nil {
		m.Grants = []json.RawMessage{}
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func strictDecode(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("trailing data after the JSON value")
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err == nil {
		return errors.New("trailing data after the JSON value")
	}
	return nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidManifest, fmt.Sprintf(format, args...))
}

// Validate enforces every v1 field rule (design §3).
func (m Manifest) Validate() error {
	if m.Schema != ManifestSchemaV1 {
		return invalid("schema %q is not %s", m.Schema, ManifestSchemaV1)
	}
	if !namePattern.MatchString(m.Name) {
		return invalid("name %q must match %s", m.Name, namePattern)
	}
	if IsReservedName(m.Name) {
		return invalid("name %q is reserved by a static app or the shell", m.Name)
	}
	if !versionPattern.MatchString(m.Version) {
		return invalid("version %q is not MAJOR.MINOR.PATCH", m.Version)
	}
	if strings.TrimSpace(m.Display.Title) == "" || utf8.RuneCountInString(m.Display.Title) > 60 {
		return invalid("display.title is required and at most 60 characters")
	}
	if utf8.RuneCountInString(m.Display.Icon) > 8 {
		return invalid("display.icon is at most 8 characters")
	}
	if utf8.RuneCountInString(m.Display.Description) > 200 {
		return invalid("display.description is at most 200 characters")
	}
	if !IsDigest(m.Source.Digest) {
		return invalid("source.digest %q is not sha256:<64 hex>", m.Source.Digest)
	}
	if !knownRecipes[m.Build.Recipe] {
		return invalid("build.recipe %q is unknown", m.Build.Recipe)
	}
	if len(m.Build.Outputs) == 0 {
		return invalid("build.outputs is empty")
	}
	for i, out := range m.Build.Outputs {
		if err := checkRelPath(out); err != nil {
			return invalid("build.outputs[%d]: %v", i, err)
		}
		if i > 0 && m.Build.Outputs[i-1] >= out {
			return invalid("build.outputs must be sorted and unique")
		}
	}
	if err := checkRelPath(m.Frontend.Entry); err != nil {
		return invalid("frontend.entry: %v", err)
	}
	if !m.hasOutput(m.Frontend.Entry) {
		return invalid("frontend.entry %q is not a build output", m.Frontend.Entry)
	}
	if m.Frontend.ABI != FrontendABIV1 {
		return invalid("frontend.abi %q is not %s", m.Frontend.ABI, FrontendABIV1)
	}
	if w := m.Frontend.Window; w.Width < 0 || w.Height < 0 || w.Width > 4096 || w.Height > 4096 {
		return invalid("frontend.window size is out of range")
	}
	if b := m.Backend; b != nil {
		if b.Kind != BackendKindService {
			return invalid("backend.kind %q is not %s", b.Kind, BackendKindService)
		}
		if b.ABI != BackendABIV1 {
			return invalid("backend.abi %q is not %s", b.ABI, BackendABIV1)
		}
		if err := checkRelPath(b.Entry); err != nil {
			return invalid("backend.entry: %v", err)
		}
		if !m.hasOutput(b.Entry) {
			return invalid("backend.entry %q is not a build output", b.Entry)
		}
	}
	switch m.Data.Store {
	case DataStoreNone:
		if m.Data.SchemaVersion != 0 || len(m.Data.ReadsSchemaVersions) != 0 {
			return invalid("data.store none declares no schema versions")
		}
	case DataStoreKVV1:
		if m.Data.SchemaVersion < 1 {
			return invalid("data.schema_version must be at least 1")
		}
		for i, v := range m.Data.ReadsSchemaVersions {
			if v < 1 || (i > 0 && m.Data.ReadsSchemaVersions[i-1] >= v) {
				return invalid("data.reads_schema_versions must be sorted, unique and at least 1")
			}
		}
		if !m.ReadsSchema(m.Data.SchemaVersion) {
			return invalid("data.reads_schema_versions must include schema_version %d", m.Data.SchemaVersion)
		}
	default:
		return invalid("data.store %q is unknown", m.Data.Store)
	}
	if m.Restore.DataScope != DataScopeComputerTape {
		return invalid("restore.data_scope %q is not %s", m.Restore.DataScope, DataScopeComputerTape)
	}
	if m.Restore.OnPurge != OnPurgeDeleteData && m.Restore.OnPurge != OnPurgeRetainData {
		return invalid("restore.on_purge %q is unknown", m.Restore.OnPurge)
	}
	if len(m.Grants) != 0 {
		return invalid("grants are not accepted before S4's egress grant model")
	}
	return nil
}

// ReadsSchema reports whether the package can read data written at schema
// version v.
func (m Manifest) ReadsSchema(v int) bool {
	for _, readable := range m.Data.ReadsSchemaVersions {
		if readable == v {
			return true
		}
	}
	return false
}

func (m Manifest) hasOutput(path string) bool {
	i := sort.SearchStrings(m.Build.Outputs, path)
	return i < len(m.Build.Outputs) && m.Build.Outputs[i] == path
}

// Digest is the SHA-256 of the manifest's canonical JSON.
func (m Manifest) Digest() (string, error) {
	return canonicalDigest(m)
}

func canonicalDigest(v any) (string, error) {
	raw, err := computerevent.CanonicalJSON(v)
	if err != nil {
		return "", err
	}
	return "sha256:" + computerevent.DigestBytes(raw), nil
}

// checkRelPath accepts a clean relative slash path inside the package.
func checkRelPath(p string) error {
	switch {
	case p == "":
		return errors.New("empty path")
	case strings.HasPrefix(p, "/"):
		return fmt.Errorf("%q is absolute", p)
	case strings.Contains(p, `\`):
		return fmt.Errorf("%q contains a backslash", p)
	case strings.ContainsRune(p, 0):
		return fmt.Errorf("%q contains NUL", p)
	case len(p) > 256:
		return fmt.Errorf("%q is too long", p)
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("%q is not a clean relative path", p)
		}
		if strings.HasPrefix(part, ".") {
			return fmt.Errorf("%q names a dot-file", p)
		}
	}
	return nil
}
