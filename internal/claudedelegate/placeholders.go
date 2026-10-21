package claudedelegate

import (
	"os"
	"path/filepath"
)

// claudePlaceholderFiles are the files Claude Code 2.1.300 creates empty in
// its working directory at startup when the subprocess env scrub is on, so
// its bubblewrap sandbox can mask them; claudePlaceholderDirs are the
// directories it creates there (node_modules/.bin and a stub .git). Left in
// the source tree they break the build: an empty /workspace/platform/package.json
// is read by esbuild (failure 29,
// docs/problems/engineering-completion-path-cluster-2026-10-20.md).
var (
	claudePlaceholderFiles = []string{
		".npmrc", ".yarnrc", ".yarnrc.yml", "bunfig.toml", "package.json", ".gitmodules",
		"package-lock.json", "yarn.lock", "pnpm-lock.yaml",
		".env", ".env.local", ".env.development", ".env.development.local", ".env.test",
		".env.test.local", ".env.production", ".env.production.local",
	}
	claudePlaceholderDirs = []string{".git", "node_modules"}
)

// absentPlaceholders lists the placeholder names missing from dir before a
// run; only those may be removed after it.
func absentPlaceholders(dir string) []string {
	var absent []string
	for _, name := range append(append([]string(nil), claudePlaceholderFiles...), claudePlaceholderDirs...) {
		if _, err := os.Lstat(filepath.Join(dir, name)); os.IsNotExist(err) {
			absent = append(absent, name)
		}
	}
	return absent
}

// removePlaceholders removes, of the names absent before the run, a file
// placeholder that is now empty or a symlink (a placeholder name the
// delegate filled with content is its work and stays), and a directory
// placeholder entirely. It never follows a symlink. It returns what it
// removed.
func removePlaceholders(dir string, absent []string) []string {
	dirs := map[string]bool{}
	for _, d := range claudePlaceholderDirs {
		dirs[d] = true
	}
	var removed []string
	for _, name := range absent {
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			err = os.Remove(path)
		case dirs[name]:
			err = os.RemoveAll(path)
		case info.Mode().IsRegular() && info.Size() == 0:
			err = os.Remove(path)
		default:
			continue
		}
		if err == nil {
			removed = append(removed, name)
		}
	}
	return removed
}
