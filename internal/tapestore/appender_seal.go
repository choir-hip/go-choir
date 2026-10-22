package tapestore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Seal seals the open segment this Appender created for the computer, if it
// still owns one: an index is written and the directory fsynced, as at
// rotation, and the next Append starts the next segment. It is for a tool
// that is done writing (the repack), so that a computer nobody writes again
// does not keep an unsealed tail.
//
// It takes the computer's write lock and applies the rules of every other
// call: a poisoned Appender returns ErrAppenderPoisoned and touches nothing;
// a segment this Appender did not create, or one whose size or seal state no
// longer matches what it wrote (another writer got there first), is left
// alone and Seal returns nil; a segment holding no record is left open. A
// failure after the seal began poisons the Appender, as at rotation.
func (a *Appender) Seal(computerID string) error {
	if !ValidComputerID(computerID) {
		return fmt.Errorf("tapestore: invalid computer id %q", computerID)
	}
	l := a.logFor(computerID)
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := a.check(); err != nil {
		return err
	}
	if !l.valid || len(l.entries) == 0 {
		return nil
	}
	dir := filepath.Join(a.root, SegDirName, computerID)
	for _, p := range []string{filepath.Join(a.root, SegDirName), dir} {
		if err := requirePlainDir(p); err != nil {
			return err
		}
	}
	d, err := os.Open(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			l.valid = false
			return nil
		}
		return fmt.Errorf("tapestore: open %s: %w", dir, err)
	}
	defer d.Close()
	unlock, err := lockDir(d)
	if err != nil {
		return fmt.Errorf("tapestore: lock %s: %w", dir, err)
	}
	defer unlock()
	if err := a.check(); err != nil {
		return err
	}
	if !a.cacheCurrent(l, dir) {
		l.valid = false
		return nil
	}
	l.valid = false
	if err := a.seal(dir, computerID, l.segno, l.size, l.entries); err != nil {
		return a.fail(err)
	}
	return nil
}
