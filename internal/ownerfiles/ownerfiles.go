// Package ownerfiles is the one write path for the computer's files root
// (docs/design/capsule-overlay-computer-state-2026-11-03.md, slice S1).
//
// Every write under the files root goes through it. A file is replaced by
// writing a temporary file beside it and renaming it into place, never by
// writing the old inode, so a hardlink taken earlier keeps the old bytes.
// Every write holds the read side of one process-wide lock; the engineering
// capsule's files snapshot holds the write side (Exclusive) while it walks
// and links, so the snapshot is the tree at one instant.
//
// Go's RWMutex forbids taking the read side twice in one goroutine (a waiting
// Exclusive blocks the second RLock), so a compound write takes one Tx and
// calls only its methods. The package-level functions each take their own.
//
// Lock order is fixed per writer mutex, and no code takes a writer's mutex
// while holding a Tx and also takes a Tx while holding that mutex: the
// maildir takes this lock, then its own mutex; file sync holds its own mutex
// across hydration and takes this lock inside it, and nothing under a Tx
// calls back into file sync.
//
// scripts/check-owner-files-writes refuses raw os write calls in the packages
// that hold the files root, outside this package.
package ownerfiles

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
)

var mu sync.RWMutex

// Sync fsyncs one open file or directory; tests replace it to see what
// reached the disk before a write is acknowledged.
var Sync = func(f *os.File) error { return f.Sync() }

// MailDir is the owner's mailbox under the files root (the maildir). The
// engineering capsule's files snapshot leaves it out by this name.
const MailDir = "mail"

// TempInfix marks the name pattern of the helper's temporary files: a dot,
// the target's base name, ".tmp-" and a random suffix. The files snapshot
// leaves these names out.
const TempInfix = ".tmp-"

// Tx is a held read side of the files lock.
type Tx struct{ done bool }

// Begin takes the read side. End releases it; End is idempotent.
func Begin() *Tx {
	mu.RLock()
	return &Tx{}
}

// End releases the read side.
func (t *Tx) End() {
	if t == nil || t.done {
		return
	}
	t.done = true
	mu.RUnlock()
}

// Exclusive runs fn holding the write side: no write under the files root
// runs while fn does. The files snapshot is its only caller. fn must not
// call any function of this package (each takes the read side, and would
// wait forever behind the held write side); it uses raw reads and links.
func Exclusive(fn func() error) error {
	mu.Lock()
	defer mu.Unlock()
	return fn()
}

// WriteFile replaces path with data: write, chmod, fsync, rename, fsync the
// directory. A kill leaves the old file or the new one, never an empty one.
func (t *Tx) WriteFile(path string, data []byte, mode os.FileMode) error {
	return t.WriteFrom(path, mode, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

// WriteFrom replaces path with what fill writes, as WriteFile does.
func (t *Tx) WriteFrom(path string, mode os.FileMode, fill func(io.Writer) error) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+TempInfix+"*")
	if err != nil {
		return err
	}
	fillErr := fill(tmp)
	chmodErr := tmp.Chmod(mode)
	syncErr := Sync(tmp)
	closeErr := tmp.Close()
	if err := errors.Join(fillErr, chmodErr, syncErr, closeErr); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return t.SyncDirs(dir)
}

// CreateExcl creates a new file that must not exist (O_EXCL), for a writer
// with its own temporary-name convention (the maildir's tmp/). The caller
// renames it into place within the same Tx.
func (t *Tx) CreateExcl(path string, mode os.FileMode) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
}

// Mkdir creates one directory.
func (t *Tx) Mkdir(path string, mode os.FileMode) error { return os.Mkdir(path, mode) }

// MkdirAll creates a directory and its parents.
func (t *Tx) MkdirAll(path string, mode os.FileMode) error { return os.MkdirAll(path, mode) }

// Remove removes one name.
func (t *Tx) Remove(path string) error { return os.Remove(path) }

// RemoveAll removes a tree.
func (t *Tx) RemoveAll(path string) error { return os.RemoveAll(path) }

// Rename renames, replacing newpath.
func (t *Tx) Rename(oldpath, newpath string) error { return os.Rename(oldpath, newpath) }

// RenameNoReplace moves src to dst and fails if dst exists.
func (t *Tx) RenameNoReplace(src, dst string) error { return renameNoReplace(src, dst) }

// SyncDirs fsyncs each distinct directory, so a created, removed or renamed
// entry in it survives a kill.
func (t *Tx) SyncDirs(dirs ...string) error {
	seen := make(map[string]bool, len(dirs))
	for _, dir := range dirs {
		if seen[dir] {
			continue
		}
		seen[dir] = true
		d, err := os.Open(dir)
		if err != nil {
			return err
		}
		if err := errors.Join(Sync(d), d.Close()); err != nil {
			return err
		}
	}
	return nil
}

func with(fn func(*Tx) error) error {
	t := Begin()
	defer t.End()
	return fn(t)
}

// WriteFile is Tx.WriteFile under its own read lock.
func WriteFile(path string, data []byte, mode os.FileMode) error {
	return with(func(t *Tx) error { return t.WriteFile(path, data, mode) })
}

// WriteFrom is Tx.WriteFrom under its own read lock.
func WriteFrom(path string, mode os.FileMode, fill func(io.Writer) error) error {
	return with(func(t *Tx) error { return t.WriteFrom(path, mode, fill) })
}

// Mkdir is Tx.Mkdir under its own read lock.
func Mkdir(path string, mode os.FileMode) error {
	return with(func(t *Tx) error { return t.Mkdir(path, mode) })
}

// MkdirAll is Tx.MkdirAll under its own read lock.
func MkdirAll(path string, mode os.FileMode) error {
	return with(func(t *Tx) error { return t.MkdirAll(path, mode) })
}

// Remove is Tx.Remove under its own read lock.
func Remove(path string) error { return with(func(t *Tx) error { return t.Remove(path) }) }

// RemoveAll is Tx.RemoveAll under its own read lock.
func RemoveAll(path string) error { return with(func(t *Tx) error { return t.RemoveAll(path) }) }

// Rename is Tx.Rename under its own read lock.
func Rename(oldpath, newpath string) error {
	return with(func(t *Tx) error { return t.Rename(oldpath, newpath) })
}

// RenameNoReplace is Tx.RenameNoReplace under its own read lock.
func RenameNoReplace(src, dst string) error {
	return with(func(t *Tx) error { return t.RenameNoReplace(src, dst) })
}

// SyncDirs is Tx.SyncDirs under its own read lock.
func SyncDirs(dirs ...string) error { return with(func(t *Tx) error { return t.SyncDirs(dirs...) }) }
