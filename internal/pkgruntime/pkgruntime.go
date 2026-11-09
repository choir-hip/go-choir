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
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
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

// cssFetchFunctions is the list of CSS functions that fetch, which the boundary
// plugin's CSS gate reads beside itself (frontend/scripts/css-fetch-functions.json,
// shared with the theme gate; a test keeps this copy equal).
//
//go:embed css-fetch-functions.json
var cssFetchFunctions []byte

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
	// Files are the data files the manifest names (a theme pack's themes),
	// read from the verified tree. A pack, a package with no app, has Files
	// and no Bytes: there is no code to build.
	Files []File `json:"files,omitempty"`
	// Uses are the SDK names the module imports, extracted by the build
	// (sorted; empty for a pack). An update compares them (F1).
	Uses []string `json:"uses,omitempty"`
}

// File is one data file of a package, by path in the package and digest.
type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  []byte `json:"-"`
}

// MaxFileBytes bounds one data file of a package (a theme).
const MaxFileBytes = apppackage.MaxThemeFileBytes

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
	return b.buildTree(ctx, tmp, tree, rec.Source.Root, rec.OwnShellUpdate())
}

// buildTree builds the module of a verified, materialized release tree
// (Build's second half; the surface-mount test lays a tree out by hand).
func (b Builder) buildTree(ctx context.Context, tmp, tree, root string, ownShell bool) (Module, error) {
	manifestRaw, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(root), apppackage.ManifestFile))
	if err != nil {
		return Module{}, fmt.Errorf("pkgruntime: read the manifest: %w", err)
	}
	manifest, err := apppackage.ParseManifestOwnShell(manifestRaw, apppackage.OriginPublished, apppackage.CurrentPlatform(), ownShell)
	if err != nil {
		return Module{}, err
	}
	files, err := readThemeFiles(manifest, filepath.Join(tree, filepath.FromSlash(root)))
	if err != nil {
		return Module{}, err
	}
	prompts, err := readDeskPrompts(manifest, filepath.Join(tree, filepath.FromSlash(root)))
	if err != nil {
		return Module{}, err
	}
	files = append(files, prompts...)
	if manifest.App == nil && len(manifest.Kinds) == 0 && len(manifest.Contributes.Desks) > 0 && len(manifest.Contributes.Desks) == len(prompts) {
		// A desk-only package from another publisher (R24): no window and no
		// code to build here. The desk's prompt rides as a data file, verified
		// with the rest of the release's bytes; the desk runs in the worker
		// sandbox, never in this build.
		return Module{RequiresSDK: apppackage.DefaultRequiresSDK, Manifest: json.RawMessage(manifestRaw), Files: files}, nil
	}
	if manifest.App == nil && len(manifest.Provides) == 0 {
		// A pack contributes data (themes, commands, keybindings) and has no
		// code to build. A desk or a kind with no app is not a pack.
		if len(manifest.Contributes.Desks) > 0 || len(manifest.Kinds) > 0 {
			return Module{}, fmt.Errorf("pkgruntime: package %q has no app to build", manifest.Name)
		}
		return Module{RequiresSDK: apppackage.DefaultRequiresSDK, Manifest: json.RawMessage(manifestRaw), Files: files}, nil
	}
	if err := b.Available(); err != nil {
		return Module{}, err
	}
	requires := apppackage.DefaultRequiresSDK
	if manifest.App != nil && manifest.App.RequiresSDK != "" {
		requires = manifest.App.RequiresSDK
	}
	// A surface package's module needs the highest SDK range its surfaces ask.
	if manifest.App == nil {
		for _, p := range manifest.Provides {
			if p.RequiresSDK != "" && sdkRange.MatchString(p.RequiresSDK) && sdkRangeLess(requires, p.RequiresSDK) {
				requires = p.RequiresSDK
			}
		}
	}
	if !sdkRange.MatchString(requires) {
		return Module{}, fmt.Errorf("pkgruntime: requires_sdk %q is not ^MAJOR.MINOR", requires)
	}
	front := filepath.Join(tree, "frontend")
	for name, body := range map[string][]byte{"build-module.mjs": buildScript, "package-boundary-plugin.mjs": boundaryPlugin, "css-fetch-functions.json": cssFetchFunctions} {
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
	uses, err := readUses(out + ".uses.json")
	if err != nil {
		return Module{}, err
	}
	return Module{SHA256: digest(raw), Size: len(raw), ABI: ModuleABI, RequiresSDK: requires, Manifest: json.RawMessage(manifestRaw), Bytes: raw, Files: files, Uses: uses}, nil
}

// sdkRangeLess reports whether ^a.b is below ^c.d (both already match sdkRange).
func sdkRangeLess(a, b string) bool {
	var am, an, bm, bn int
	fmt.Sscanf(strings.TrimPrefix(a, "^"), "%d.%d", &am, &an)
	fmt.Sscanf(strings.TrimPrefix(b, "^"), "%d.%d", &bm, &bn)
	return am < bm || (am == bm && an < bn)
}

// readUses reads the SDK names the build extracted (the module's own
// sdk_imports, written beside it): the extracted uses an update compares
// (F1).
func readUses(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: the build wrote no uses: %v", ErrBuild, err)
	}
	var uses []string
	if err := json.Unmarshal(raw, &uses); err != nil {
		return nil, fmt.Errorf("%w: the build's uses: %v", ErrBuild, err)
	}
	if uses == nil {
		uses = []string{}
	}
	sort.Strings(uses)
	return uses, nil
}

// readThemeFiles reads the theme files the manifest names from the verified
// tree: each a regular file (no symlink), at most MaxFileBytes, a JSON object
// whose id is the one the manifest declares and whose schema_version is 2.
// The token, safe-value and contrast rules run in the shell when the theme
// registers (theme.ts); a theme carries no code.
func readThemeFiles(m apppackage.Manifest, root string) ([]File, error) {
	var out []File
	for _, t := range m.Contributes.Themes {
		full := filepath.Join(root, filepath.FromSlash(t.File))
		fi, err := os.Lstat(full)
		if err != nil || !fi.Mode().IsRegular() {
			return nil, fmt.Errorf("%w: theme file %q is not a file in the package", ErrBuild, t.File)
		}
		if fi.Size() > MaxFileBytes {
			return nil, fmt.Errorf("%w: theme file %q is over %d bytes", ErrBuild, t.File, MaxFileBytes)
		}
		raw, err := os.ReadFile(full)
		if err != nil {
			return nil, fmt.Errorf("%w: theme file %q: %w", ErrBuild, t.File, err)
		}
		var theme struct {
			ID            string `json:"id"`
			SchemaVersion int    `json:"schema_version"`
		}
		if json.Unmarshal(raw, &theme) != nil || theme.ID != t.ID || theme.SchemaVersion != 2 {
			return nil, fmt.Errorf("%w: theme file %q must be a schema 2 theme with id %q", ErrBuild, t.File, t.ID)
		}
		out = append(out, File{Path: t.File, SHA256: digest(raw), Bytes: raw})
	}
	return out, nil
}

// MaxDeskPromptBytes bounds one desk prompt file.
const MaxDeskPromptBytes = 64 << 10

// readDeskPrompts reads the prompt file of each desk a package from another
// publisher contributes, from the verified tree: a regular file inside the
// package (no symlink), non-empty, at most MaxDeskPromptBytes. A first-party
// package's desk runs on the image's prompt (appdesk.FirstPartyPrompt), so its
// prompt file is not read.
func readDeskPrompts(m apppackage.Manifest, root string) ([]File, error) {
	if deskmanifest.FirstPartyAppSlug(m.Name) {
		return nil, nil
	}
	var out []File
	for _, d := range m.Contributes.Desks {
		if !filepath.IsLocal(filepath.FromSlash(d.Prompt)) {
			return nil, fmt.Errorf("%w: prompt %q of desk %q is not inside the package", ErrBuild, d.Prompt, d.Name)
		}
		full := filepath.Join(root, filepath.FromSlash(d.Prompt))
		fi, err := os.Lstat(full)
		if err != nil || !fi.Mode().IsRegular() {
			return nil, fmt.Errorf("%w: prompt %q of desk %q is not a file in the package", ErrBuild, d.Prompt, d.Name)
		}
		if fi.Size() == 0 || fi.Size() > MaxDeskPromptBytes {
			return nil, fmt.Errorf("%w: prompt %q of desk %q must be 1 to %d bytes", ErrBuild, d.Prompt, d.Name, MaxDeskPromptBytes)
		}
		raw, err := os.ReadFile(full)
		if err != nil {
			return nil, fmt.Errorf("%w: prompt %q: %w", ErrBuild, d.Prompt, err)
		}
		if strings.TrimSpace(string(raw)) == "" {
			return nil, fmt.Errorf("%w: prompt %q of desk %q is blank", ErrBuild, d.Prompt, d.Name)
		}
		out = append(out, File{Path: d.Prompt, SHA256: digest(raw), Bytes: raw})
	}
	return out, nil
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

func (s Store) filePath(sha string) string { return filepath.Join(s.Dir, "files", sha+".json") }

// PutFile writes a data file (a theme) by its digest beside the modules.
func (s Store) PutFile(raw []byte) (string, error) {
	if strings.TrimSpace(s.Dir) == "" {
		return "", errors.New("pkgruntime: no store directory")
	}
	if len(raw) == 0 || len(raw) > MaxFileBytes {
		return "", fmt.Errorf("pkgruntime: a file of %d bytes", len(raw))
	}
	sha := digest(raw)
	if _, err := s.ReadFile(sha); err == nil {
		return sha, nil
	}
	return sha, writeOnce(s.filePath(sha), ".file-", raw)
}

func (s Store) patchPath(sha string) string { return filepath.Join(s.Dir, "patches", sha+".patch") }

// MaxPatchBytes caps an installed release's tree patch (a release blob's cap).
const MaxPatchBytes = 2 << 20

// PutPatch keeps an installed release's tree patch by its digest, so the
// installed tree can be rebuilt as a capsule's seed.
func (s Store) PutPatch(raw []byte) (string, error) {
	if strings.TrimSpace(s.Dir) == "" {
		return "", errors.New("pkgruntime: no store directory")
	}
	if len(raw) == 0 || len(raw) > MaxPatchBytes {
		return "", fmt.Errorf("pkgruntime: a patch of %d bytes", len(raw))
	}
	sha := digest(raw)
	if _, err := s.ReadPatch(sha); err == nil {
		return sha, nil
	}
	return sha, writeOnce(s.patchPath(sha), ".patch-", raw)
}

// ReadPatch returns a kept tree patch, refusing any whose digest is not its name.
func (s Store) ReadPatch(sha string) ([]byte, error) {
	if !sha256Hex.MatchString(sha) || strings.TrimSpace(s.Dir) == "" {
		return nil, ErrNotFound
	}
	raw, err := os.ReadFile(s.patchPath(sha))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if digest(raw) != sha {
		return nil, fmt.Errorf("pkgruntime: patch %s does not match its digest", sha)
	}
	return raw, nil
}

// ReadFile returns a data file's bytes, refusing any whose digest is not its name.
func (s Store) ReadFile(sha string) ([]byte, error) {
	if !sha256Hex.MatchString(sha) || strings.TrimSpace(s.Dir) == "" {
		return nil, ErrNotFound
	}
	raw, err := os.ReadFile(s.filePath(sha))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if digest(raw) != sha {
		return nil, fmt.Errorf("pkgruntime: file %s does not match its digest", sha)
	}
	return raw, nil
}

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
	return sha, writeOnce(s.path(sha), ".module-", raw)
}

// writeOnce writes raw to path read only, through a synced temporary file, and
// replaces a tampered copy a read refused.
func writeOnce(path, tmpPrefix string, raw []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, tmpPrefix)
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o444); err != nil {
		return err
	}
	_ = os.Remove(path)
	return os.Rename(tmp, path)
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
