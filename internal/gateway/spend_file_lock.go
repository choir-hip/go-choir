package gateway

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// spendLockWait is how long a save waits for the file's lock. It is a
// deadline, not a switch: past it the save is skipped, the unsaved delta is
// kept, and the next save writes it.
var spendLockWait atomic.Int64 // nanoseconds; atomic because a flusher reads it

func init() { spendLockWait.Store(int64(5 * time.Second)) }

// spendRetryEvery is how soon a failed save is tried again with no new call
// to kick it; spendErrorLogEvery is how often a failing save logs.
const (
	spendRetryEvery    = time.Second
	spendErrorLogEvery = time.Minute
)

// spendSaveWindowHook, when set (tests only), runs after a save has taken its
// snapshot and before it folds the merged record back into memory.
var spendSaveWindowHook atomic.Pointer[func()]

// SpendPersistence says whether a ledger's spend is reaching its file. The
// unsaved calls still count against the caps (memory plus delta); the file
// is what the other gateway process and the next start read.
type SpendPersistence struct {
	UnsavedCalls int64  `json:"unsaved_calls"`
	LastError    string `json:"last_error,omitempty"`
	LastErrorAt  string `json:"last_error_at,omitempty"`
	LastSaveAt   string `json:"last_save_at,omitempty"`
	ErrorLogs    int64  `json:"error_log_lines"`
	// BadFilesMoved counts corrupt spend files set aside to
	// <file>.bad-<UTC time>; BadFileMovedTo is the last one.
	BadFilesMoved  int64  `json:"bad_files_moved,omitempty"`
	BadFileMovedTo string `json:"bad_file_moved_to,omitempty"`
	// DroppedCharges counts relay charges not saved because their day was
	// already finished in the file or in this process (nothing reads a
	// finished day).
	DroppedCharges int64 `json:"dropped_charges,omitempty"`
}

// spendSaver runs a ledger's saves. A guest's call only records into memory
// and kicks the flusher; the file lock, the read, the merge and the write all
// happen on the flusher (or in Flush), outside the ledger's mutex, so a call
// never waits for the file. Saves are serialised in the process by saveMu.
type spendSaver struct {
	saveMu sync.Mutex
	once   sync.Once
	kick   chan struct{}
	quit   chan struct{}
	done   chan struct{}
	stopMu sync.Once

	statusMu   sync.Mutex
	lastErr    string
	lastErrAt  time.Time
	lastSaveAt time.Time
	lastLogAt  time.Time
	errorLogs  int64
	badMoved   int64
	badMovedTo string
	dropped    int64
}

// kickSave asks the flusher to save; it never blocks. save is the ledger's
// one-shot save, which returns nil when there is nothing to save.
func (p *spendSaver) kickSave(save func() error) {
	p.once.Do(func() {
		p.kick = make(chan struct{}, 1)
		p.quit, p.done = make(chan struct{}), make(chan struct{})
		go p.loop(save)
	})
	select {
	case p.kick <- struct{}{}:
	default:
	}
}

// stop ends the flusher and waits for its save in flight (tests only: the
// gateway's flusher lives as long as the process).
func (p *spendSaver) stop() {
	p.once.Do(func() { p.quit, p.done = make(chan struct{}), make(chan struct{}); close(p.done) })
	p.stopMu.Do(func() { close(p.quit) })
	<-p.done
}

func (p *spendSaver) loop(save func() error) {
	defer close(p.done)
	var retry <-chan time.Time
	for {
		select {
		case <-p.quit:
			return
		case <-p.kick:
		case <-retry:
		}
		if err := p.run(save); err != nil {
			retry = time.After(spendRetryEvery)
		} else {
			retry = nil
		}
	}
}

// run is one save, with its result recorded in the status. The log line is
// written after statusMu is released, so a blocked journal never stalls a
// reader of the status.
func (p *spendSaver) run(save func() error) error {
	p.saveMu.Lock()
	defer p.saveMu.Unlock()
	err := save()
	p.statusMu.Lock()
	if err == nil {
		p.lastSaveAt, p.lastErr = time.Now(), ""
		p.statusMu.Unlock()
		return nil
	}
	now := time.Now()
	p.lastErr, p.lastErrAt = err.Error(), now
	logIt := now.Sub(p.lastLogAt) >= spendErrorLogEvery
	if logIt {
		p.lastLogAt = now
		p.errorLogs++
	}
	p.statusMu.Unlock()
	if logIt {
		log.Printf("gateway: spend file not saved (the calls are kept in memory and count against the caps; retrying each second): %v", err)
	}
	return err
}

// noteBadMoved records a corrupt spend file set aside, loudly: it is a loss
// of the other process's (or the last run's) spend from this ledger, once.
func (p *spendSaver) noteBadMoved(from, to string) {
	p.statusMu.Lock()
	p.badMoved++
	p.badMovedTo = to
	p.statusMu.Unlock()
	log.Printf("gateway: SPEND FILE CORRUPT: %s was moved to %s (bytes kept, never deleted); this ledger continues from its own memory and writes a fresh file; spend recorded only in the old file is not counted", from, to)
}

// noteDropped records charges not saved because their day was finished.
func (p *spendSaver) noteDropped(n int64, why string) {
	if n <= 0 {
		return
	}
	p.statusMu.Lock()
	p.dropped += n
	p.statusMu.Unlock()
	log.Printf("gateway: relay spend: %d unsaved charge(s) dropped: %s (a finished day is never read)", n, why)
}

// quarantineSpendFile moves a corrupt spend file to <file>.bad-<UTC time>,
// mode 0600. The bytes are kept; no code deletes them. The caller holds the
// file's lock and has just read the file as corrupt.
func quarantineSpendFile(path string, now time.Time) (string, error) {
	stamp := now.UTC().Format("20060102T150405Z")
	for i := 0; i < 100; i++ {
		to := path + ".bad-" + stamp
		if i > 0 {
			to += "-" + strconv.Itoa(i)
		}
		if _, err := os.Lstat(to); err == nil {
			continue
		}
		if err := os.Rename(path, to); err != nil {
			return "", fmt.Errorf("move the corrupt spend file aside: %w", err)
		}
		_ = os.Chmod(to, 0o600)
		return to, nil
	}
	return "", errors.New("move the corrupt spend file aside: no free name")
}

// setAsideIfCorrupt, at load, takes the file's lock, reads the file again
// (another process may have moved or mended it) and, when it is still
// corrupt, moves it aside. A lock not taken in time leaves the file for the
// first save to deal with.
func (p *spendSaver) setAsideIfCorrupt(path string, read func() error) {
	unlock, err := lockSpendFile(path)
	if err != nil {
		log.Printf("gateway: spend file %s is corrupt and could not be set aside yet: %v", path, err)
		return
	}
	defer unlock()
	if err := read(); !errors.Is(err, errSpendFileCorrupt) {
		return
	}
	to, err := quarantineSpendFile(path, time.Now())
	if err != nil {
		log.Printf("gateway: spend file %s is corrupt: %v", path, err)
		return
	}
	p.noteBadMoved(path, to)
}

func (p *spendSaver) status(unsaved int64) SpendPersistence {
	p.statusMu.Lock()
	defer p.statusMu.Unlock()
	out := SpendPersistence{UnsavedCalls: unsaved, LastError: p.lastErr, ErrorLogs: p.errorLogs,
		BadFilesMoved: p.badMoved, BadFileMovedTo: p.badMovedTo, DroppedCharges: p.dropped}
	if !p.lastErrAt.IsZero() && p.lastErr != "" {
		out.LastErrorAt = p.lastErrAt.UTC().Format(time.RFC3339)
	}
	if !p.lastSaveAt.IsZero() {
		out.LastSaveAt = p.lastSaveAt.UTC().Format(time.RFC3339)
	}
	return out
}

// errSpendFileBad says a spend file is present but cannot be used; it is
// never overwritten, because it may hold the other process's spend.
var errSpendFileBad = errors.New("the spend file is present but unreadable; not overwritten")

// errSpendFileCorrupt marks a file that was read but holds no usable record
// (it does not parse as one, or has no day). Such a file is set aside under
// the lock and replaced; a file that merely cannot be read is not.
var errSpendFileCorrupt = errors.New("the spend file's content is corrupt")

// errSpendLockTimeout says the lock was not taken within spendLockWait.
var errSpendLockTimeout = errors.New("the spend file's lock was not taken in time")

// lockSpendFile takes an exclusive flock on path+".lock" and returns the
// release. The kernel drops the lock when the process dies, so a killed
// writer never leaves it held; the lock file itself is left in place and is
// harmless.
func lockSpendFile(path string) (func(), error) {
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("spend lock file %s.lock: %w", path, err)
	}
	deadline := time.Now().Add(time.Duration(spendLockWait.Load()))
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() {
				_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
				_ = f.Close()
			}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EINTR) {
			_ = f.Close()
			return nil, err
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, errSpendLockTimeout
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// writeSpendFile writes raw to path by a uniquely named temp file in the same
// directory and an atomic rename, so two writers never share a temp name and
// a reader sees the old file or the new one whole.
func writeSpendFile(path string, raw []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err = tmp.Write(raw); err == nil {
		err = tmp.Chmod(0o600)
	}
	if err == nil {
		err = tmp.Sync() // a crash must not leave a 0-byte file under the final name
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
