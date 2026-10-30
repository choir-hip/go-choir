package yaegikernel

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFileReplacing writes content to path like os.WriteFile and, when the
// target is an existing regular file that refuses the open with a permission
// error, replaces it with a temp file in the same directory renamed over it.
//
// Why: a capsule's source tree is an immutable overlay lower layer whose
// regular files are 0444 and owned by an uid the capsule namespace does not
// map (internal/capsule/source_snapshot.go makeSubjectTreeReadOnly), so
// open(O_WRONLY|O_TRUNC) on an existing file fails with EACCES even for the
// namespace's root. Directories are 0777, so creating and renaming entries is
// allowed; the replace edits the file without loosening the lower tree
// (docs/archive/problems/p5-go-served-0825-engineering-never-freezes-2026-10-15.md).
//
// perm is used for a new file. When explicitMode is false and the replace path
// runs, the replacement keeps the old file's permission bits plus owner-write,
// so a 0555 script stays executable and later writes take the plain path. A
// symlink, a directory, or an unwritable parent directory is never replaced:
// the original error is returned and the target is untouched.
func WriteFileReplacing(path string, content []byte, perm os.FileMode, explicitMode bool) error {
	err := os.WriteFile(path, content, perm)
	if err == nil || !errors.Is(err, fs.ErrPermission) {
		return err
	}
	info, statErr := os.Lstat(path)
	if statErr != nil || !info.Mode().IsRegular() {
		return err
	}
	mode := perm
	if !explicitMode {
		mode = info.Mode().Perm() | 0o200
	}
	tmp, tmpErr := os.CreateTemp(filepath.Dir(path), ".choir-write-*")
	if tmpErr != nil {
		return err
	}
	tmpName := tmp.Name()
	fail := func(step string, cause error) error {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("%w (replace %s failed: %v)", err, step, cause)
	}
	if _, writeErr := tmp.Write(content); writeErr != nil {
		return fail("write", writeErr)
	}
	if chmodErr := tmp.Chmod(mode); chmodErr != nil {
		return fail("chmod", chmodErr)
	}
	if closeErr := tmp.Close(); closeErr != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("%w (replace close failed: %v)", err, closeErr)
	}
	if renameErr := os.Rename(tmpName, path); renameErr != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("%w (replace rename failed: %v)", err, renameErr)
	}
	return nil
}
