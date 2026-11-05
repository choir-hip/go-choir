// Package pkgruntime is runtime install (Library S12, app packages P5;
// docs/design/s12-runtime-install-build-2026-11-04.md): a published
// first-party package release becomes one content-addressed ES module in a
// per-computer store, outside the source tree, which the shell loads at run
// time. Installing or uninstalling one changes no source, builds no SPA and
// restarts nothing.
//
// The computer builds the module itself, from the verified release tree and
// its own locked frontend dependencies, with no network (Builder). Source
// crosses between computers; a binary never does.
package pkgruntime

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

const (
	// ModuleABI is the module the shell loads: one self-contained ES module
	// exporting mount(target, ctx), abi, requires_sdk and sdk_imports.
	ModuleABI = "choir.app-frontend/module/v1"
	// MaxModuleBytes bounds a built module (the Reader, with pdf.js inlined,
	// is about 3.6 MB).
	MaxModuleBytes = 16 << 20
	// BuildTimeout bounds one module build (about 1 to 3 seconds measured).
	BuildTimeout = 3 * time.Minute
	// maxBuildLog bounds the build output kept for a refusal.
	maxBuildLog = 4 << 10
)

//go:embed build-module.mjs
var buildScript []byte

//go:embed package-boundary-plugin.mjs
var boundaryPlugin []byte

// legacyEvents is the events list the module shim forwards (SDK 1.1.0: one
// source, frontend/src/lib/sdk-surface/legacy-events.json, which the SDK lock
// records; a test keeps this copy equal).
//
//go:embed legacy-events.json
var legacyEvents []byte

var (
	sha256Hex   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	sdkRange    = regexp.MustCompile(`^\^(0|[1-9][0-9]{0,3})\.(0|[1-9][0-9]{0,3})$`)
	ErrNotFound = errors.New("pkgruntime: module not found")
)

// ErrBuild is a build failure; its message carries the build's own output.
var ErrBuild = errors.New("pkgruntime: package build failed")

// Builder builds modules. Node is the node binary (empty: "node" on PATH);
// DepsDir holds the computer's locked frontend dependencies, package.json and
// node_modules ($CHOIR_FRONTEND_DEPS on a computer).
type Builder struct {
	Node    string
	DepsDir string
	Timeout time.Duration
}

// Module is one built module and what the shell needs to list it.
type Module struct {
	SHA256      string          `json:"sha256"`
	Size        int             `json:"size"`
	ABI         string          `json:"abi"`
	RequiresSDK string          `json:"requires_sdk"`
	Manifest    json.RawMessage `json:"manifest"`
	Bytes       []byte          `json:"-"`
}

// Available reports whether the builder can run: a node binary and the
// locked dependencies.
func (b Builder) Available() error {
	if strings.TrimSpace(b.DepsDir) == "" {
		return errors.New("no frontend dependencies (CHOIR_FRONTEND_DEPS is unset)")
	}
	for _, p := range []string{"package.json", "node_modules"} {
		if _, err := os.Stat(filepath.Join(b.DepsDir, p)); err != nil {
			return fmt.Errorf("frontend dependencies: %v", err)
		}
	}
	if _, err := exec.LookPath(b.node()); err != nil {
		return fmt.Errorf("node: %v", err)
	}
	return nil
}

func (b Builder) node() string {
	if strings.TrimSpace(b.Node) != "" {
		return b.Node
	}
	return "node"
}

// Build verifies the release (pkgrelease.Materialize: the record's bytes,
// confined to the package root, no secret, a valid manifest), lays out a
// private frontend root holding only the package and an SDK shim, and runs
// the embedded build with the computer's locked dependencies. The
// publisher's declared commands are never run, and no package code runs at
// build time.
func (b Builder) Build(ctx context.Context, rec pkgrelease.Record, treePatch []byte) (Module, error) {
	if err := b.Available(); err != nil {
		return Module{}, err
	}
	rec = rec.Normalize()
	tmp, err := os.MkdirTemp("", "choir-pkgmodule-")
	if err != nil {
		return Module{}, err
	}
	defer os.RemoveAll(tmp)
	tree := filepath.Join(tmp, "tree")
	if err := pkgrelease.Materialize(ctx, rec, treePatch, tree); err != nil {
		return Module{}, err
	}
	manifestRaw, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(rec.Source.Root), apppackage.ManifestFile))
	if err != nil {
		return Module{}, fmt.Errorf("pkgruntime: read the manifest: %w", err)
	}
	manifest, err := apppackage.ParseManifest(manifestRaw, apppackage.OriginPublished)
	if err != nil {
		return Module{}, err
	}
	if manifest.App == nil {
		return Module{}, fmt.Errorf("pkgruntime: package %q has no app to build", manifest.Name)
	}
	requires := manifest.App.RequiresSDK
	if requires == "" {
		requires = apppackage.DefaultRequiresSDK
	}
	if !sdkRange.MatchString(requires) {
		return Module{}, fmt.Errorf("pkgruntime: requires_sdk %q is not ^MAJOR.MINOR", requires)
	}
	front := filepath.Join(tree, "frontend")
	for name, body := range map[string][]byte{"build-module.mjs": buildScript, "package-boundary-plugin.mjs": boundaryPlugin, "legacy-events.json": legacyEvents} {
		if err := os.WriteFile(filepath.Join(front, name), body, 0o444); err != nil {
			return Module{}, err
		}
	}
	pkgJSON, err := os.ReadFile(filepath.Join(b.DepsDir, "package.json"))
	if err != nil {
		return Module{}, err
	}
	if err := os.WriteFile(filepath.Join(front, "package.json"), pkgJSON, 0o444); err != nil {
		return Module{}, err
	}
	deps, err := filepath.Abs(filepath.Join(b.DepsDir, "node_modules"))
	if err != nil {
		return Module{}, err
	}
	if err := os.Symlink(deps, filepath.Join(front, "node_modules")); err != nil {
		return Module{}, err
	}
	out := filepath.Join(tmp, "module.js")
	timeout := b.Timeout
	if timeout <= 0 {
		timeout = BuildTimeout
	}
	bctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(bctx, b.node(), filepath.Join(front, "build-module.mjs"), front, manifest.Name, out, requires)
	cmd.Dir = front
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + tmp, "NODE_ENV=production", "TMPDIR=" + tmp}
	logs, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return Module{}, fmt.Errorf("%w: %v: %s", ErrBuild, runErr, tail(logs))
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		return Module{}, fmt.Errorf("%w: no module written: %s", ErrBuild, tail(logs))
	}
	if len(raw) == 0 || len(raw) > MaxModuleBytes {
		return Module{}, fmt.Errorf("%w: the module is %d bytes (at most %d)", ErrBuild, len(raw), MaxModuleBytes)
	}
	return Module{SHA256: digest(raw), Size: len(raw), ABI: ModuleABI, RequiresSDK: requires, Manifest: json.RawMessage(manifestRaw), Bytes: raw}, nil
}

func tail(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > maxBuildLog {
		s = "..." + s[len(s)-maxBuildLog:]
	}
	return s
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Store is the per-computer module store: <dir>/modules/<sha256>.js, written
// once (read only) and re-verified on every read. It sits beside the
// computer's state, outside the source tree, so a base move or a deploy
// leaves it untouched.
type Store struct {
	Dir string
}

func (s Store) path(sha string) string { return filepath.Join(s.Dir, "modules", sha+".js") }

// Put writes a module by its digest. Writing the same bytes again is a no-op.
func (s Store) Put(raw []byte) (string, error) {
	if strings.TrimSpace(s.Dir) == "" {
		return "", errors.New("pkgruntime: no store directory")
	}
	if len(raw) == 0 || len(raw) > MaxModuleBytes {
		return "", fmt.Errorf("pkgruntime: a module of %d bytes", len(raw))
	}
	sha := digest(raw)
	if _, err := s.Read(sha); err == nil {
		return sha, nil
	}
	dir := filepath.Dir(s.path(sha))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, ".module-")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(raw); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp, 0o444); err != nil {
		return "", err
	}
	_ = os.Remove(s.path(sha)) // a tampered copy Read refused
	if err := os.Rename(tmp, s.path(sha)); err != nil {
		return "", err
	}
	return sha, nil
}

// Read returns a module's bytes, refusing any whose digest is not its name.
func (s Store) Read(sha string) ([]byte, error) {
	if !sha256Hex.MatchString(sha) || strings.TrimSpace(s.Dir) == "" {
		return nil, ErrNotFound
	}
	raw, err := os.ReadFile(s.path(sha))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if digest(raw) != sha {
		return nil, fmt.Errorf("pkgruntime: module %s does not match its digest", sha)
	}
	return raw, nil
}
