package taperepack

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// This file is the only place the package changes or removes a file by
// name, and it never names a path under the artifacts root: the work
// directory (spill files, state, reports) is refused when it is inside the
// root, and every removal below is of a path under the work directory. A
// guard test (P4) fails if another source file calls os.Remove, os.RemoveAll
// or os.Rename.

// fsyncWork says whether work-directory files (spill, state, results, the
// report) are fsynced before they count. Tests turn it off: on some hosts a
// sync costs tens of milliseconds and a run writes a thousand files.
var fsyncWork = true

// resolvePath makes path absolute and resolves symlinks in its longest
// existing prefix, so a path that does not exist yet can still be compared.
func resolvePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rest := ""
	cur := abs
	for {
		resolved, err := filepath.EvalSymlinks(cur)
		if err == nil {
			return filepath.Join(resolved, rest), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs, nil
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// isUnder reports whether path is root or inside it.
func isUnder(path, root string) (bool, error) {
	p, err := resolvePath(path)
	if err != nil {
		return false, err
	}
	r, err := resolvePath(root)
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(r, p)
	if err != nil {
		return false, err
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))), nil
}

// checkWorkDir refuses a work directory inside the artifacts root.
func checkWorkDir(workDir, root string) error {
	if workDir == "" {
		return errors.New("taperepack: --work-dir is required")
	}
	inside, err := isUnder(workDir, root)
	if err != nil {
		return err
	}
	if inside {
		return errors.New("taperepack: the work directory must be outside the artifacts root")
	}
	return nil
}

// wipeDir removes a directory of the work directory (spill files, state).
func wipeDir(workDir, dir string) error {
	inside, err := isUnder(dir, workDir)
	if err != nil {
		return err
	}
	if !inside || filepath.Clean(dir) == filepath.Clean(workDir) {
		return fmt.Errorf("taperepack: refusing to remove %s: not inside the work directory", dir)
	}
	return os.RemoveAll(dir)
}

// writeFileAtomic writes b to path through a synced temp file and a rename.
func writeFileAtomic(path string, b []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(b)
	if werr == nil && fsyncWork {
		werr = tmp.Sync()
	}
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		_ = os.Remove(name)
		return werr
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}

func writeJSONAtomic(path string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, b)
}
