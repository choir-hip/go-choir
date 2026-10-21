package pkgmirror

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const tempPrefix = ".tmp-"

// diskCache is the append-only cache tree, the goproxy cache's design
// (internal/goproxy/cache.go). Entries are written to a temp
// file in their directory and linked into place, so an existing entry is
// never replaced and a crash never leaves a half-written entry visible.
type diskCache struct {
	root     string
	maxBytes int64

	mu       sync.Mutex
	size     int64
	evicting bool
}

func openDiskCache(root string, maxBytes int64) (*diskCache, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	// A systemd DynamicUser StateDirectory is a symlink into /var/lib/private.
	// WalkDir does not follow a symlink root, so the size walk and eviction
	// must run on the resolved directory (else they see only the link).
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	root = resolved
	c := &diskCache{root: root, maxBytes: maxBytes}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.HasPrefix(d.Name(), tempPrefix) {
			// A temp file from a fetch that never finished.
			_ = os.Remove(path)
			return nil
		}
		if info, err := d.Info(); err == nil {
			c.size += info.Size()
		}
		return nil
	})
	return c, err
}

func (c *diskCache) path(rel string) string { return cacheFile(c.root, rel) }

// lookup returns the entry's path when it is present.
func (c *diskCache) lookup(rel string) (string, bool) {
	p := c.path(rel)
	info, err := os.Stat(p)
	return p, err == nil && info.Mode().IsRegular()
}

// create opens a temp file next to rel's final path.
func (c *diskCache) create(rel string) (*os.File, error) {
	final := c.path(rel)
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		return nil, err
	}
	return os.CreateTemp(filepath.Dir(final), tempPrefix+"*")
}

// commit links tmp into rel's place unless an entry is already there, and
// removes tmp either way.
func (c *diskCache) commit(tmp, rel string, n int64) error {
	defer os.Remove(tmp)
	if err := os.Chmod(tmp, 0o644); err != nil {
		return err
	}
	if err := os.Link(tmp, c.path(rel)); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return nil
		}
		return err
	}
	c.mu.Lock()
	c.size += n
	over := c.maxBytes > 0 && c.size > c.maxBytes && !c.evicting
	if over {
		c.evicting = true
	}
	c.mu.Unlock()
	if over {
		c.evict()
	}
	return nil
}

func (c *diskCache) bytes() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.size
}

// evict deletes the oldest entries by modification time until the cache is
// at 90% of its cap.
func (c *diskCache) evict() {
	defer func() {
		c.mu.Lock()
		c.evicting = false
		c.mu.Unlock()
	}()
	type entry struct {
		path string
		size int64
		mod  time.Time
	}
	var entries []entry
	_ = filepath.WalkDir(c.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasPrefix(d.Name(), tempPrefix) {
			return nil
		}
		if info, err := d.Info(); err == nil {
			entries = append(entries, entry{path, info.Size(), info.ModTime()})
		}
		return nil
	})
	sort.Slice(entries, func(i, j int) bool { return entries[i].mod.Before(entries[j].mod) })
	target := c.maxBytes / 10 * 9
	for _, e := range entries {
		if c.bytes() <= target {
			return
		}
		if os.Remove(e.path) == nil {
			c.mu.Lock()
			c.size -= e.size
			c.mu.Unlock()
		}
	}
}

// copyCapped copies at most limit bytes and fails past it.
func copyCapped(dst io.Writer, src io.Reader, limit int64) (int64, error) {
	n, err := io.Copy(dst, io.LimitReader(src, limit+1))
	if err != nil {
		return n, err
	}
	if n > limit {
		return n, errTooLarge
	}
	return n, nil
}

var errTooLarge = errors.New("upstream response exceeds the size cap")

// cacheFile joins a checked relative cache path under root.
func cacheFile(root, rel string) string {
	return filepath.Join(root, filepath.FromSlash(rel))
}
