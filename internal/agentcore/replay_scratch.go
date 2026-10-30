package agentcore

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/persistentdisk"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// The replay probe's scratch: a whole installed snapshot plus the tail
// replay's writes. It lives beside the live store on the persistent volume
// (rematerialize stages there too), never in /tmp, which in a guest is a RAM
// tmpfs of half the guest's memory. A probe that cannot fit refuses by name
// before it starts; one that runs short while it replays stops by name. A full
// volume fails the live store's writes, so the probe must never be the writer
// that fills it
// (docs/problems/apply-checkpoint-replay-fills-guest-tmp-2026-10-26.md).

// ErrReplayScratchInsufficient means the persistent volume cannot hold the
// replay probe's scratch and still keep the live store's reserve. The probe
// refused or stopped; nothing was replayed into the live store and the
// scratch is removed.
var ErrReplayScratchInsufficient = errors.New("replay scratch insufficient")

const (
	// replayScratchDirName is the probe's parent beside the live store.
	replayScratchDirName = "replay-probe"
	// replayScratchPrefix names one probe's root, in the parent and, for
	// roots older runtimes left, in os.TempDir().
	replayScratchPrefix = "choir-replay-completeness-"
	// replayScratchPerEventBytes bounds the journal one replayed event
	// writes: 220 to 310 KiB per event measured on the paper, 141 KiB on the
	// local tape harness.
	replayScratchPerEventBytes = uint64(320) << 10
	// replayScratchMinReserveBytes is the floor of the free space the probe
	// leaves to the live store, which shares the volume and keeps writing
	// while the probe replays (the paper's journal grows up to 1.6 GiB in 10
	// minutes). The reserve is max(this, 10% of the volume).
	replayScratchMinReserveBytes = uint64(2) << 30
	// replayScratchPollInterval is how often a running probe reads free
	// space. A replay page writes at most a few hundred MiB per second.
	replayScratchPollInterval = time.Second
)

// replayScratchReserve is the free space a probe leaves to the live store on a
// volume of totalBytes: max(2 GiB, 10% of the volume). Admission and the
// in-flight watch both derive it from the same Usage.
func replayScratchReserve(totalBytes uint64) uint64 {
	if tenth := totalBytes / 10; tenth > replayScratchMinReserveBytes {
		return tenth
	}
	return replayScratchMinReserveBytes
}

// activeReplayScratch holds the roots of probes running in this process, so
// the start-time sweep never removes a live probe's root.
var activeReplayScratch sync.Map

// replayScratchParent is the directory probe roots are made in: beside the
// live store's marker.
func (rt *Runtime) replayScratchParent() string {
	if rt == nil {
		return ""
	}
	marker := ""
	if rt.store != nil {
		marker = strings.TrimSpace(rt.store.Path())
	}
	if marker == "" {
		marker = strings.TrimSpace(rt.cfg.StorePath)
	}
	if marker == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(marker), replayScratchDirName)
}

func (rt *Runtime) replayScratchFree(dir string) (persistentdisk.Usage, error) {
	if rt != nil && rt.replayScratchUsage != nil {
		return rt.replayScratchUsage(dir)
	}
	return defaultReplayScratchUsage(dir)
}

// defaultReplayScratchUsage reads the volume's free space. Tests replace it:
// the reserve is a percentage of the volume, so a test must not depend on how
// full the machine running it is.
var defaultReplayScratchUsage = persistentdisk.Statfs

// replayScratchEstimate is the scratch a probe at the current head needs: the
// installed snapshot and its replayed copy coexist, so twice the live state
// file plus its WAL (the store a replay rebuilds is about the live store's
// size).
func (rt *Runtime) replayScratchEstimate(ctx context.Context, computerID string) (uint64, error) {
	head, err := rt.store.Head(ctx, computerID)
	if err != nil {
		return 0, err
	}
	if head == nil || head.Sequence == 0 {
		return 0, nil
	}
	state := choirstore.StateDBPath(rt.store.Path())
	var size uint64
	for _, name := range []string{state, state + "-wal"} {
		if info, err := os.Stat(name); err == nil && info.Size() > 0 {
			size += uint64(info.Size())
		}
	}
	return 2 * size, nil
}

// admitReplayScratch refuses a probe whose estimated scratch plus the reserve
// does not fit on the parent's volume. An estimate the probe cannot make
// (base source unreachable) still requires the reserve; the probe's own base
// path then reports the source error.
func (rt *Runtime) admitReplayScratch(ctx context.Context, computerID, parent string) error {
	need, err := rt.replayScratchEstimate(ctx, computerID)
	if err != nil {
		need = 0
	}
	usage, err := rt.replayScratchFree(parent)
	if err != nil {
		return fmt.Errorf("%w: read free space on %s: %v", ErrReplayScratchInsufficient, parent, err)
	}
	reserve := replayScratchReserve(usage.TotalBytes)
	if usage.AvailBytes < need+reserve {
		return fmt.Errorf("%w: the probe needs about %d bytes plus a %d-byte reserve on %s; %d are free",
			ErrReplayScratchInsufficient, need, reserve, parent, usage.AvailBytes)
	}
	return nil
}

// watchReplayScratch returns a context that is cancelled, with an
// ErrReplayScratchInsufficient cause, when free space on dir falls under the
// reserve. Replay checks its context after every event. stop ends the watch.
func (rt *Runtime) watchReplayScratch(ctx context.Context, dir string) (context.Context, func()) {
	guarded, cancel := context.WithCancelCause(ctx)
	interval := replayScratchPollInterval
	if rt.replayScratchPoll > 0 {
		interval = rt.replayScratchPoll
	}
	done := make(chan struct{})
	var once sync.Once
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-guarded.Done():
				return
			case <-ticker.C:
				usage, err := rt.replayScratchFree(dir)
				if err != nil {
					continue
				}
				reserve := replayScratchReserve(usage.TotalBytes)
				if usage.AvailBytes >= reserve {
					continue
				}
				cause := fmt.Errorf("%w: free space on %s fell to %d bytes, under the %d-byte reserve, while the probe replayed",
					ErrReplayScratchInsufficient, dir, usage.AvailBytes, reserve)
				log.Printf("replay completeness: %v; stopping the probe", cause)
				cancel(cause)
				return
			}
		}
	}()
	return guarded, func() {
		once.Do(func() {
			close(done)
			cancel(nil)
		})
	}
}

// replayScratchCause returns the named scratch error when the guard stopped
// the probe, else err unchanged.
func replayScratchCause(guarded context.Context, err error) error {
	if err == nil || guarded == nil {
		return err
	}
	if cause := context.Cause(guarded); cause != nil && errors.Is(cause, ErrReplayScratchInsufficient) && !errors.Is(err, ErrReplayScratchInsufficient) {
		return fmt.Errorf("%w (%v)", cause, err)
	}
	return err
}

// removeStaleReplayScratch removes every probe root no running probe holds:
// in the parent beside the store, and in os.TempDir() where older runtimes
// made them. It runs first in Start.
func (rt *Runtime) removeStaleReplayScratch() {
	dirs := []string{os.TempDir()}
	if parent := rt.replayScratchParent(); parent != "" {
		dirs = append(dirs, parent)
	}
	for _, dir := range dirs {
		matches, err := filepath.Glob(filepath.Join(dir, replayScratchPrefix+"*"))
		if err != nil {
			continue
		}
		for _, root := range matches {
			if _, active := activeReplayScratch.Load(root); active {
				continue
			}
			if err := os.RemoveAll(root); err != nil {
				log.Printf("runtime: remove stale replay scratch %s: %v", root, err)
				continue
			}
			log.Printf("runtime: removed stale replay scratch %s", root)
		}
	}
}
