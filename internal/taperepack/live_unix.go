//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package taperepack

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// refuseLiveWriter tries, without blocking, the per-computer segment lock
// the production appender holds while it writes (an exclusive flock on
// <root>/tape-seg/<computer>, internal/tapestore lockDir) and releases it at
// once. A held lock means a writer is appending now: copy refuses rather
// than interleave with a live store (bunch 31, low). It detects a writer in
// the act; the appender's own lock is what keeps a concurrent copy correct.
func refuseLiveWriter(root, computer string) error {
	dir := filepath.Join(root, tapestore.SegDirName, computer)
	d, err := os.Open(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("taperepack: computer %s: open the segment directory: %w", computer, err)
	}
	defer d.Close()
	for {
		err = syscall.Flock(int(d.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err != syscall.EINTR {
			break
		}
	}
	switch {
	case err == nil:
		_ = syscall.Flock(int(d.Fd()), syscall.LOCK_UN)
		return nil
	case errors.Is(err, syscall.EWOULDBLOCK):
		return fmt.Errorf("taperepack: computer %s: a writer holds the segment lock; the store is live, copy refused", computer)
	default:
		return fmt.Errorf("taperepack: computer %s: try the segment lock: %w", computer, err)
	}
}
