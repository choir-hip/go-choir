package tapestore

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"slices"
	"sort"
)

// A digest index lets a view choose which sealed segments to probe for one
// namespace. The repack appends legacy files bucket by first digest byte
// (internal/taperepack/join.go), so consecutive events of a chain sit in
// unrelated segments and "probe from the last hit" costs about half the
// segments an event (1,789 segments, 935 probes an event on the paper).
//
// The index is one sorted []uint64: the top 44 bits of the first eight digest
// bytes, then 20 bits of segment ordinal, 8 bytes an entry (11 million events:
// 88 MB). It is built from the sealed index files the view reads anyway, for
// the namespace asked for, and only by OpenIndexedView.
//
// What the index may say. A sealed segment's index file never changes, so for
// every sealed segment that was in the directory listing when the index was
// built or last extended, "no entry has this prefix" proves that segment does
// not hold the digest (a longer digest can only match if its prefix does).
// The index is extended in load(), before the view's listing is replaced, for
// every sealed segment of the new listing; so the sealed segments of the
// view's listing are always all covered. A segment that is open (no index
// file) is never in the index and is probed on every read. A segment created
// or sealed since the listing is not in the view's listing, and absence is
// reported only when a fresh listing equals the view's (the rule ReadObject
// already has): otherwise the view reloads, which extends the index, and
// searches again. The index selects candidates; the record that is returned
// is still read and verified from its segment.

const (
	digestIndexOrdBits = 20
	digestIndexOrdMask = 1<<digestIndexOrdBits - 1
)

type digestIndex struct {
	ns     NS
	ords   map[uint64]uint32 // segno -> ordinal
	segnos []uint64          // ordinal -> segno
	keys   []uint64          // sorted: prefix44<<20 | ordinal
}

// OpenIndexedView is OpenView for a reader that will look up many objects of
// one namespace: it also builds the digest index of that namespace (see the
// comment above). If the tape has more than a million sealed segments the
// view is returned without the index and reads as OpenView's view does.
func (r *Reader) OpenIndexedView(computerID string, ns NS) (*SegmentView, error) {
	v := &SegmentView{r: r, computer: computerID, index: &digestIndex{ns: ns, ords: map[uint64]uint32{}}}
	if err := v.load(); err != nil {
		return nil, err
	}
	return v, nil
}

// IndexEntries is the number of entries of the digest index, 0 for a view
// without one. IndexBytes is the memory they take.
func (v *SegmentView) IndexEntries() int {
	if v.index == nil {
		return 0
	}
	return len(v.index.keys)
}

func (v *SegmentView) IndexBytes() int { return 8 * v.IndexEntries() }

// SealedProbes is the number of Probes that consulted a sealed segment.
func (v *SegmentView) SealedProbes() uint64 { return v.sealedProbes }

// indexAdditions reads the entries of the namespace from every sealed segment
// of segs the index does not cover yet. It changes nothing; commitIndex
// applies the result. A nil result with no error means nothing to add or that
// the index has been dropped (too many segments).
type indexAddition struct {
	segnos []uint64
	keys   []uint64
	drop   bool
}

func (v *SegmentView) indexAdditions(segs []segFiles, sealed map[uint64]*sealedSeg) (*indexAddition, error) {
	ix := v.index
	if ix == nil {
		return nil, nil
	}
	add := &indexAddition{}
	for _, f := range segs {
		if !f.hasIdx {
			continue
		}
		if _, ok := ix.ords[f.segno]; ok {
			continue
		}
		ord := len(ix.segnos) + len(add.segnos)
		if ord > digestIndexOrdMask {
			return &indexAddition{drop: true}, nil
		}
		prefixes, err := sealedPrefixes(sealed[f.segno], ix.ns)
		if err != nil {
			return nil, err
		}
		for _, p := range prefixes {
			add.keys = append(add.keys, p<<digestIndexOrdBits|uint64(ord))
		}
		add.segnos = append(add.segnos, f.segno)
	}
	return add, nil
}

func (v *SegmentView) commitIndex(add *indexAddition) {
	ix := v.index
	if ix == nil || add == nil {
		return
	}
	if add.drop {
		v.index = nil
		return
	}
	if len(add.segnos) == 0 {
		return
	}
	for _, s := range add.segnos {
		ix.ords[s] = uint32(len(ix.segnos))
		ix.segnos = append(ix.segnos, s)
	}
	ix.keys = append(ix.keys, add.keys...)
	slices.Sort(ix.keys)
}

// sealedPrefixes reads a sealed index file and returns the 44-bit digest
// prefix of every entry of the namespace. The file was validated by
// openSealed; the size and crc are checked again because the bytes are read
// again.
func sealedPrefixes(s *sealedSeg, ns NS) ([]uint64, error) {
	raw, err := os.ReadFile(s.idxPath)
	if err != nil {
		return nil, fmt.Errorf("tapestore: read index %s: %w", s.idxPath, err)
	}
	want := idxFixedSize + idxEntrySize*int(s.count) + idxTrailerSize
	if len(raw) != want {
		return nil, corruptf("index %d: size %d, expected %d", s.segno, len(raw), want)
	}
	body := len(raw) - idxTrailerSize
	if crc32.Checksum(raw[:body], castagnoli) != binary.BigEndian.Uint32(raw[body:]) {
		return nil, corruptf("index %d: crc mismatch", s.segno)
	}
	out := make([]uint64, 0, s.count)
	for i := 0; i < int(s.count); i++ {
		e := raw[idxFixedSize+idxEntrySize*i:]
		if NS(e[32]) != ns {
			continue
		}
		out = append(out, binary.BigEndian.Uint64(e[0:8])>>digestIndexOrdBits)
	}
	return out, nil
}

// candidates returns the indexes into v.segs of the sealed segments whose
// index holds an entry with the digest's prefix.
func (v *SegmentView) candidates(want [32]byte) []int {
	ix := v.index
	lo := binary.BigEndian.Uint64(want[0:8]) >> digestIndexOrdBits << digestIndexOrdBits
	hi := lo + 1<<digestIndexOrdBits
	i := sort.Search(len(ix.keys), func(k int) bool { return ix.keys[k] >= lo })
	var out []int
	for ; i < len(ix.keys) && ix.keys[i] < hi; i++ {
		segno := ix.segnos[ix.keys[i]&digestIndexOrdMask]
		if at, ok := v.segAt[segno]; ok {
			out = append(out, at)
		}
	}
	return out
}

// searchIndexed is search for the indexed namespace: the candidate sealed
// segments and the open segment, with the same rule as search (an intact copy
// returns at once; otherwise the newest damaged copy's error; a path error is
// returned at once).
func (v *SegmentView) searchIndexed(ns NS, want [32]byte) ([]byte, bool, error) {
	var damage error
	damageAt := -1
	try := func(i int) ([]byte, bool, error, bool) {
		body, found, err := v.probe(i, ns, want)
		switch {
		case found:
			return body, true, nil, true
		case err != nil && !isDamage(err):
			return nil, false, err, true
		case err != nil && i > damageAt:
			damage, damageAt = err, i
		}
		return nil, false, nil, false
	}
	for _, i := range v.candidates(want) {
		if body, found, err, done := try(i); done {
			return body, found, err
		}
	}
	for i, f := range v.segs {
		if !f.hasIdx {
			if body, found, err, done := try(i); done {
				return body, found, err
			}
		}
	}
	return nil, false, damage
}
