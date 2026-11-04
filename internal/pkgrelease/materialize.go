package pkgrelease

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Materialize (runtime install, S12; docs/design/s12-runtime-install-build-2026-11-04.md)
// verifies a release's content exactly as VerifyContent does and writes its
// files, at their full paths (frontend/src/packages/<name>/...), under dir,
// which must be empty or absent. Nothing is written unless every content
// check held, and the checks refuse symlinks, submodules and paths outside
// the package root, so the checkout writes only regular files under
// dir/<root>.
func Materialize(ctx context.Context, rec Record, treePatch []byte, dir string) error {
	rec = rec.Normalize()
	if err := rec.Validate(); err != nil {
		return err
	}
	if err := VerifyBlob(rec.Source.TreePatch, treePatch); err != nil {
		return err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if entries, err := os.ReadDir(abs); err == nil && len(entries) > 0 {
		return fmt.Errorf("pkgrelease: materialize into %s: not empty", abs)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return err
	}
	s, err := newScratch(ctx, "")
	if err != nil {
		return fmt.Errorf("pkgrelease: scratch repository: %w", err)
	}
	defer s.close()
	full, err := s.verifiedFull(ctx, rec, treePatch)
	if err != nil {
		return err
	}
	env := s.index()
	if _, err := s.git(ctx, env, nil, "read-tree", full); err != nil {
		return fmt.Errorf("pkgrelease: read the release tree: %w", err)
	}
	if _, err := s.git(ctx, env, nil, "checkout-index", "--all", "--prefix="+abs+string(filepath.Separator)); err != nil {
		return fmt.Errorf("pkgrelease: write the release tree: %w", err)
	}
	return nil
}
