package apppackage

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/gomodule"
)

// GoModuleRoot is the directory of a package's vendored Go modules: each
// module's whole content at GoModuleRoot/<escaped path>@<version>/ (the
// module cache's layout), so its h1: hash can be checked from the release's
// own bytes (plan MA6.5, R42; docs/design/ma65-yaegi-go-modules-2026-11-06.md).
const GoModuleRoot = "gomod"

// ErrGoModule refuses a package whose Go modules or desk Go imports fail the
// gate: not curated, not matching their checksum database hash, or an import
// closure that reaches a banned package, cgo, linkname, assembly or anything
// outside the desk's import set. It wraps ErrInvalidManifest; the refusal
// itself is a *gomodule.Refusal (gomodule.CodeOf names its code).
var ErrGoModule = fmt.Errorf("%w: go module refused", ErrInvalidManifest)

// curatedGoModules is the curated list the gate reads: the checked-in one.
// The package's own tests replace it to reach the closure checks with fixture
// modules; nothing else does.
var curatedGoModules = gomodule.Curated

func goModuleErr(err error) error { return fmt.Errorf("%w: %w", ErrGoModule, err) }

// validateGoModules is the shape check (no files): each reference is
// path@version, one version per path, within the caps; each desk's
// go_imports lie inside a declared module and are not internal packages.
func (m Manifest) validateGoModules() error {
	if len(m.GoModules) > gomodule.MaxModules {
		return invalid("go_modules: at most %d", gomodule.MaxModules)
	}
	var mods []gomodule.Module
	seen := map[string]bool{}
	for i, ref := range m.GoModules {
		mod, err := gomodule.ParseRef(ref)
		if err != nil {
			return invalid("go_modules[%d]: %v", i, err)
		}
		if seen[mod.Path] {
			return invalid("go_modules: %s is named twice", mod.Path)
		}
		seen[mod.Path] = true
		mods = append(mods, mod)
	}
	for i, d := range m.Contributes.Desks {
		if len(d.GoImports) > gomodule.MaxImports {
			return invalid("contributes.desks[%d].go_imports: at most %d", i, gomodule.MaxImports)
		}
		dup := map[string]bool{}
		for _, p := range d.GoImports {
			if err := gomodule.CheckPath(p); err != nil {
				return invalid("contributes.desks[%d].go_imports: %v", i, err)
			}
			if dup[p] {
				return invalid("contributes.desks[%d].go_imports: %q is named twice", i, p)
			}
			dup[p] = true
			covered := false
			for _, mod := range mods {
				if p == mod.Path || strings.HasPrefix(p, mod.Path+"/") {
					covered = true
				}
			}
			if !covered {
				return invalid("contributes.desks[%d].go_imports: %q is in no module of go_modules", i, p)
			}
		}
	}
	return nil
}

// goModuleSources reads every declared module's content from the release
// and checks each against the curated list (gomodule.Resolve's first half),
// refusing a file under GoModuleRoot that belongs to no declared module.
//
// It also returns the verified modules with their curated h1: sums, sorted by
// path: what the behaviour surface names.
func (m Manifest) goModuleSources(src Source) ([]gomodule.ModuleSource, []gomodule.Module, error) {
	if src == nil {
		if len(m.GoModules) > 0 {
			return nil, nil, invalid("go_modules: no source to read the modules from")
		}
		return nil, nil, nil
	}
	paths, err := src.List()
	if err != nil {
		return nil, nil, invalid("listing the package files: %v", err)
	}
	var out []gomodule.ModuleSource
	byDir := map[string]*gomodule.ModuleSource{}
	for _, ref := range m.GoModules {
		mod, err := gomodule.ParseRef(ref)
		if err != nil {
			return nil, nil, invalid("go_modules: %v", err)
		}
		out = append(out, gomodule.ModuleSource{Module: mod, Files: map[string][]byte{}})
	}
	for i := range out {
		byDir[GoModuleRoot+"/"+out[i].Module.Dir()+"/"] = &out[i]
	}
	sort.Strings(paths)
	for _, p := range paths {
		if !strings.HasPrefix(p, GoModuleRoot+"/") {
			continue
		}
		var into *gomodule.ModuleSource
		var rel string
		for dir, ms := range byDir {
			if strings.HasPrefix(p, dir) {
				into, rel = ms, strings.TrimPrefix(p, dir)
			}
		}
		if into == nil {
			return nil, nil, goModuleErr(&gomodule.Refusal{Code: gomodule.CodeStrayFile, Detail: fmt.Sprintf("%s is under %s/ but in no module of go_modules", p, GoModuleRoot)})
		}
		raw, err := src.Read(p)
		if err != nil {
			return nil, nil, invalid("%s: %v", p, err)
		}
		into.Files[rel] = raw
	}
	curated, err := curatedGoModules()
	if err != nil {
		return nil, nil, goModuleErr(err)
	}
	// Verify every declared module (curated, hash) even when no desk
	// imports it: the release carries it, so it must be what it claims.
	verified, err := gomodule.Verify(curated, out)
	if err != nil {
		return nil, nil, goModuleErr(err)
	}
	return out, verified, nil
}

// resolveDesk walks one desk's import closure over the verified modules.
func (m Manifest) resolveDesk(d Desk, mods []gomodule.ModuleSource) (gomodule.Bundle, error) {
	if len(d.GoImports) == 0 {
		return gomodule.Bundle{}, nil
	}
	curated, err := curatedGoModules()
	if err != nil {
		return gomodule.Bundle{}, goModuleErr(err)
	}
	b, err := gomodule.Resolve(curated, mods, d.GoImports)
	if err != nil {
		return gomodule.Bundle{}, goModuleErr(fmt.Errorf("desk %s: %w", d.Name, err))
	}
	return b, nil
}

// DeskGoBundle is the verified bundle the named desk's worker loads: the
// modules checked against the curated list and the desk's import closure
// walked, from the release's own bytes. An empty bundle means the desk loads
// no module.
func (m Manifest) DeskGoBundle(desk string, src Source) (gomodule.Bundle, error) {
	for _, d := range m.Contributes.Desks {
		if d.Name != desk {
			continue
		}
		mods, _, err := m.goModuleSources(src)
		if err != nil {
			return gomodule.Bundle{}, err
		}
		return m.resolveDesk(d, mods)
	}
	return gomodule.Bundle{}, invalid("no desk %q in package %q", desk, m.Name)
}
