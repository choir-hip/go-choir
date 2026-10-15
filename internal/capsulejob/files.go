package capsulejob

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// WriteJSONAtomic replaces dir/name with v as indented JSON, through a
// temporary file and a rename, so a reader never sees half a file.
func WriteJSONAtomic(dir, name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, "."+name+".tmp")
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, name))
}

// Tail keeps the last limit bytes written to it. It is safe for concurrent
// use: a pump writes while a status read takes the string.
type Tail struct {
	mu    sync.Mutex
	limit int
	buf   []byte
}

// NewTail returns a tail of at most limit bytes (TailBytes when limit <= 0).
func NewTail(limit int) *Tail {
	if limit <= 0 {
		limit = TailBytes
	}
	return &Tail{limit: limit}
}

func (t *Tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.limit <= 0 {
		t.limit = TailBytes
	}
	if len(p) >= t.limit {
		t.buf = append(t.buf[:0], p[len(p)-t.limit:]...)
		return len(p), nil
	}
	t.buf = append(t.buf, p...)
	if over := len(t.buf) - t.limit; over > 0 {
		t.buf = append(t.buf[:0], t.buf[over:]...)
	}
	return len(p), nil
}

// String returns the tail.
func (t *Tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}

// KillGroup ends the process group pgid: SIGTERM now, SIGKILL after grace
// unless done closes first. It returns at once.
func KillGroup(pgid int, grace time.Duration, done <-chan struct{}) {
	if pgid <= 0 {
		return
	}
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	go func() {
		select {
		case <-done:
		case <-time.After(grace):
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		}
	}()
}
