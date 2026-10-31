package bundle

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// A format 2 bundle keeps its tape objects (events, payloads, pin receipts)
// in one pack instead of one file each: the paper's tape is millions of
// small objects, and a directory of millions of files is slow to write,
// copy and list on every filesystem the bundle crosses (the bundle review,
// docs/design/one-tape-durability-2026-10-31.md section 6a2).
//
//	objects.pack   the objects' bytes, appended
//	objects.idx    one JSON line per object: kind, digest, offset, length
//
// The pack is append-only. An export that stops part way leaves index lines
// the pack may not fully hold; reopening keeps the lines whose bytes are in
// the pack and truncates the pack after the last of them, so a resumed
// export continues where the durable part ends. Nothing trusts the index for
// content: every read hashes the bytes against the digest.
const (
	PackName      = "objects.pack"
	PackIndexName = "objects.idx"
)

// packEntry is one objects.idx line.
type packEntry struct {
	Kind   string `json:"kind"`
	Digest string `json:"digest"`
	Offset int64  `json:"offset"`
	Length int64  `json:"length"`
}

// objects is where a bundle's tape objects live: loose files (format 1) or
// the pack (format 2). sub is EventsDir, PayloadsDir or PinsDir.
type objects interface {
	// size reports the stored length of the object, if it is held.
	size(sub, digest string) (int64, bool)
	// get returns the object's bytes, checked against digest.
	get(sub, digest string) ([]byte, error)
	// put stores raw (already checked against digest) unless it is held.
	put(sub, digest string, raw []byte) error
	// digests lists the held objects of sub.
	digests(sub string) ([]string, error)
	// close makes what was put durable.
	close() error
}

func openObjects(dir string, format int, write bool) (objects, error) {
	switch format {
	case 1:
		return looseObjects{dir: dir}, nil
	case 2:
		return openPack(dir, write)
	}
	return nil, fmt.Errorf("bundle: format version %d has no object layout", format)
}

// looseObjects is format 1: <sub>/<digest>.
type looseObjects struct{ dir string }

func (l looseObjects) size(sub, digest string) (int64, bool) {
	info, err := os.Stat(filepath.Join(l.dir, sub, digest))
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return 0, false
	}
	return info.Size(), true
}

func (l looseObjects) get(sub, digest string) ([]byte, error) {
	raw, err := os.ReadFile(filepath.Join(l.dir, sub, digest))
	if err != nil {
		return nil, err
	}
	if computerevent.DigestBytes(raw) != digest {
		return nil, fmt.Errorf("bundle: %s/%s does not hash to its name", sub, digest)
	}
	return raw, nil
}

func (l looseObjects) put(sub, digest string, raw []byte) error {
	return writeAtomic(filepath.Join(l.dir, sub, digest), raw, 0o640)
}

func (l looseObjects) digests(sub string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(l.dir, sub))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && validDigest(entry.Name()) {
			out = append(out, entry.Name()) // skips temp files of an interrupted export
		}
	}
	return out, nil
}

func (looseObjects) close() error { return nil }

// packObjects is format 2.
type packObjects struct {
	pack    *os.File
	idx     *os.File
	idxW    *bufio.Writer
	end     int64
	entries map[string]packEntry // kind + "/" + digest
	order   map[string][]string  // kind -> digests in pack order
}

func openPack(dir string, write bool) (*packObjects, error) {
	flags := os.O_RDONLY
	if write {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, err
		}
		flags = os.O_RDWR | os.O_CREATE
	}
	pack, err := os.OpenFile(filepath.Join(dir, PackName), flags, 0o640)
	if err != nil {
		return nil, err
	}
	p := &packObjects{pack: pack, entries: map[string]packEntry{}, order: map[string][]string{}}
	info, err := pack.Stat()
	if err != nil {
		pack.Close()
		return nil, err
	}
	packSize := info.Size()
	idxFile, err := os.OpenFile(filepath.Join(dir, PackIndexName), flags, 0o640)
	if err != nil {
		pack.Close()
		return nil, err
	}
	// Keep the index lines whose bytes the pack holds, in order; the first
	// line that is torn, malformed or past the pack ends the durable part.
	var kept []packEntry
	scanner := bufio.NewScanner(idxFile)
	scanner.Buffer(make([]byte, 4096), 1<<16)
	for scanner.Scan() {
		var e packEntry
		if json.Unmarshal(scanner.Bytes(), &e) != nil || !validDigest(e.Digest) || e.Offset != p.end || e.Length <= 0 || e.Offset+e.Length > packSize {
			break
		}
		kept = append(kept, e)
		p.end = e.Offset + e.Length
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, bufio.ErrTooLong) {
		idxFile.Close()
		pack.Close()
		return nil, err
	}
	for _, e := range kept {
		key := e.Kind + "/" + e.Digest
		if _, dup := p.entries[key]; !dup {
			p.order[e.Kind] = append(p.order[e.Kind], e.Digest)
		}
		p.entries[key] = e
	}
	if !write {
		idxFile.Close()
		return p, nil
	}
	// Drop what is past the durable part, then append after it.
	if err := pack.Truncate(p.end); err != nil {
		idxFile.Close()
		pack.Close()
		return nil, err
	}
	if err := rewriteIndex(idxFile, kept); err != nil {
		idxFile.Close()
		pack.Close()
		return nil, err
	}
	p.idx = idxFile
	p.idxW = bufio.NewWriter(idxFile)
	return p, nil
}

// rewriteIndex leaves idx holding exactly kept, positioned at its end.
func rewriteIndex(idx *os.File, kept []packEntry) error {
	if err := idx.Truncate(0); err != nil {
		return err
	}
	if _, err := idx.Seek(0, io.SeekStart); err != nil {
		return err
	}
	w := bufio.NewWriter(idx)
	enc := json.NewEncoder(w)
	for _, e := range kept {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return w.Flush()
}

func (p *packObjects) size(sub, digest string) (int64, bool) {
	e, ok := p.entries[sub+"/"+digest]
	return e.Length, ok
}

func (p *packObjects) get(sub, digest string) ([]byte, error) {
	e, ok := p.entries[sub+"/"+digest]
	if !ok {
		return nil, fmt.Errorf("bundle: %s/%s: %w", sub, digest, os.ErrNotExist)
	}
	raw := make([]byte, e.Length)
	if _, err := p.pack.ReadAt(raw, e.Offset); err != nil {
		return nil, fmt.Errorf("bundle: %s/%s: %w", sub, digest, err)
	}
	if computerevent.DigestBytes(raw) != digest {
		return nil, fmt.Errorf("bundle: %s/%s does not hash to its name", sub, digest)
	}
	return raw, nil
}

func (p *packObjects) put(sub, digest string, raw []byte) error {
	if p.idxW == nil {
		return fmt.Errorf("bundle: pack is open read-only")
	}
	key := sub + "/" + digest
	if _, ok := p.entries[key]; ok {
		return nil
	}
	if len(raw) == 0 {
		return fmt.Errorf("bundle: %s/%s is empty", sub, digest)
	}
	if _, err := p.pack.WriteAt(raw, p.end); err != nil {
		return err
	}
	e := packEntry{Kind: sub, Digest: digest, Offset: p.end, Length: int64(len(raw))}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if _, err := p.idxW.Write(append(line, '\n')); err != nil {
		return err
	}
	p.end += e.Length
	p.entries[key] = e
	p.order[sub] = append(p.order[sub], digest)
	return nil
}

func (p *packObjects) digests(sub string) ([]string, error) {
	return append([]string(nil), p.order[sub]...), nil
}

// close syncs the pack before the index, so a durable index line never
// names bytes the pack lost.
func (p *packObjects) close() error {
	var err error
	if p.idxW != nil {
		if err = p.pack.Sync(); err == nil {
			if err = p.idxW.Flush(); err == nil {
				err = p.idx.Sync()
			}
		}
		if cerr := p.idx.Close(); err == nil {
			err = cerr
		}
	}
	if cerr := p.pack.Close(); err == nil {
		err = cerr
	}
	return err
}
