package agentcore

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Durable writes for state a hard kill must not tear: a vmctl refresh
// SIGKILLs the guest, so a file written without fsync, or a rename whose
// directory was not fsynced, can come back short, empty or missing
// (docs/problems/awaiting-candidate-unreadable-after-refresh-2026-10-26.md).

// durableSync fsyncs one open file or directory; tests replace it to see
// what reached the disk, and in what order.
var durableSync = func(f *os.File) error { return f.Sync() }

// syncPath fsyncs the file or directory at path.
func syncPath(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	syncErr := durableSync(f)
	return errors.Join(syncErr, f.Close())
}

// syncTree fsyncs every regular file and directory under root, root
// included, so a rename of root publishes only bytes already on disk.
func syncTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && !entry.Type().IsRegular() {
			return nil
		}
		return syncPath(path)
	})
}

// writeFileDurable replaces path with data through a temporary file in the
// same directory: write, chmod, fsync, rename, fsync the directory. A kill
// leaves the old file or the new one, never a torn one.
func writeFileDurable(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-")
	if err != nil {
		return err
	}
	_, writeErr := tmp.Write(data)
	chmodErr := tmp.Chmod(mode)
	syncErr := durableSync(tmp)
	closeErr := tmp.Close()
	if err := errors.Join(writeErr, chmodErr, syncErr, closeErr); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return syncPath(dir)
}
