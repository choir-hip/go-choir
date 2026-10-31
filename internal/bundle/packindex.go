package bundle

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// packIndex is a format 2 pack opened to read (verify, import). Its index is
// a sorted slice of fixed-size slots, 48 bytes an object, instead of the
// writer's string-keyed maps (a few hundred bytes an object): the paper's
// tape is about 20 million objects. A slot's position gives verify a seen
// bitset of one bit an object.
type packIndex struct {
	pack  *os.File
	slotv []packSlot
	// mapped is the pack mapped read-only (nil: read with ReadAt).
	mapped []byte
	unmap  func() error
}

type packSlot struct {
	digest [32]byte
	offset int64
	length uint32
	kind   uint8
}

var packKinds = []string{EventsDir, PayloadsDir, PinsDir}

func packKind(sub string) (uint8, bool) {
	for i, k := range packKinds {
		if k == sub {
			return uint8(i), true
		}
	}
	return 0, false
}

func slotCmp(a, b packSlot) int {
	if a.kind != b.kind {
		return int(a.kind) - int(b.kind)
	}
	return bytes.Compare(a.digest[:], b.digest[:])
}

func openPackIndex(dir string) (*packIndex, error) {
	pack, err := os.Open(filepath.Join(dir, PackName))
	if err != nil {
		return nil, err
	}
	info, err := pack.Stat()
	if err != nil {
		pack.Close()
		return nil, err
	}
	packSize := info.Size()
	idx, err := os.Open(filepath.Join(dir, PackIndexName))
	if err != nil {
		pack.Close()
		return nil, err
	}
	defer idx.Close()
	p := &packIndex{pack: pack}
	var end int64
	scanner := bufio.NewScanner(idx)
	scanner.Buffer(make([]byte, 4096), 1<<16)
	for scanner.Scan() {
		// The durable-prefix rule of openPack: the first torn, malformed or
		// out-of-pack line ends what the index names.
		var e packEntry
		if json.Unmarshal(scanner.Bytes(), &e) != nil || !validDigest(e.Digest) || e.Offset != end || e.Length <= 0 || e.Length > math.MaxUint32 || e.Offset+e.Length > packSize {
			break
		}
		kind, ok := packKind(e.Kind)
		if !ok {
			break
		}
		s := packSlot{offset: e.Offset, length: uint32(e.Length), kind: kind}
		if _, err := hex.Decode(s.digest[:], []byte(e.Digest)); err != nil {
			break
		}
		p.slotv = append(p.slotv, s)
		end = e.Offset + e.Length
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, bufio.ErrTooLong) {
		pack.Close()
		return nil, err
	}
	p.mapped, p.unmap = mapPack(pack, packSize)
	slices.SortStableFunc(p.slotv, slotCmp)
	p.slotv = slices.Clip(slices.CompactFunc(p.slotv, func(a, b packSlot) bool { return slotCmp(a, b) == 0 }))
	return p, nil
}

func (p *packIndex) find(sub, digest string) (int, bool) {
	kind, ok := packKind(sub)
	if !ok || len(digest) != 64 {
		return -1, false
	}
	key := packSlot{kind: kind}
	if _, err := hex.Decode(key.digest[:], []byte(digest)); err != nil {
		return -1, false
	}
	return slices.BinarySearchFunc(p.slotv, key, slotCmp)
}

func (p *packIndex) size(sub, digest string) (int64, bool) {
	i, ok := p.find(sub, digest)
	if !ok {
		return 0, false
	}
	return int64(p.slotv[i].length), true
}

func (p *packIndex) get(sub, digest string) ([]byte, error) {
	i, ok := p.find(sub, digest)
	if !ok {
		return nil, fmt.Errorf("bundle: %s/%s: %w", sub, digest, os.ErrNotExist)
	}
	s := p.slotv[i]
	raw := make([]byte, s.length)
	if p.mapped != nil {
		// A copy: the caller may keep it past close.
		copy(raw, p.mapped[s.offset:s.offset+int64(s.length)])
	} else if _, err := p.pack.ReadAt(raw, s.offset); err != nil {
		return nil, fmt.Errorf("bundle: %s/%s: %w", sub, digest, err)
	}
	if computerevent.DigestBytes(raw) != digest {
		return nil, fmt.Errorf("bundle: %s/%s does not hash to its name", sub, digest)
	}
	return raw, nil
}

func (p *packIndex) put(string, string, []byte) error {
	return fmt.Errorf("bundle: pack is open read-only")
}

func (p *packIndex) digests(sub string) ([]string, error) {
	kind, ok := packKind(sub)
	if !ok {
		return nil, nil
	}
	var out []string
	for _, s := range p.slotv {
		if s.kind == kind {
			out = append(out, hex.EncodeToString(s.digest[:]))
		}
	}
	return out, nil
}

func (p *packIndex) slot(sub, digest string) (int, bool) { return p.find(sub, digest) }

func (p *packIndex) slots() int { return len(p.slotv) }

func (p *packIndex) close() error {
	if p.unmap != nil {
		_ = p.unmap()
		p.mapped, p.unmap = nil, nil
	}
	return p.pack.Close()
}
