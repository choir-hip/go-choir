package tapestore

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Record is one verified record handed to a Scan callback.
type Record struct {
	Segno  uint64
	Sealed bool
	NS     NS
	Digest string
	Body   []byte
}

// Scan visits every record of the computer's segments in segment and file
// order, verifying everything the point reader verifies plus the whole-file
// properties: a sealed segment must end exactly at its sealed size, hold
// exactly the records its index lists (each at the index's offset), and an
// unsealed segment may end in a torn tail only if it is the last one. It is
// the audit used by tests, the census and the future repack tool; it reads
// whole segments into memory.
func (r *Reader) Scan(computerID string, fn func(Record) error) error {
	dir, segs, err := r.list(computerID)
	if err != nil {
		return err
	}
	for i, f := range segs {
		isLast := i == len(segs)-1
		label := fmt.Sprintf("computer %s segment %d", computerID, f.segno)
		raw, err := os.ReadFile(filepath.Join(dir, SegmentFileName(f.segno)))
		if err != nil {
			return fmt.Errorf("tapestore: read segment %s: %w", label, err)
		}
		var idx map[openKey]idxEntry
		if f.hasIdx {
			s, err := openSealed(dir, computerID, f.segno)
			if err != nil {
				return err
			}
			if idx, err = readAllEntries(s); err != nil {
				return err
			}
			if int64(len(raw)) != s.segSize {
				return corruptf("%s: segment is %d bytes, index sealed %d", label, len(raw), s.segSize)
			}
		}
		if len(raw) < segHeaderSize {
			if f.hasIdx || !isLast {
				return corruptf("%s: truncated inside its header", label)
			}
			continue
		}
		if err := checkSegHeader(raw[:segHeaderSize], f.segno, computerID); err != nil {
			return err
		}
		pos := int64(segHeaderSize)
		size := int64(len(raw))
		seen := 0
		for pos < size {
			h, next, bad, err := scanRecord(bytes.NewReader(raw), pos, size)
			if err != nil {
				return err
			}
			if bad != "" {
				if f.hasIdx || !isLast {
					return corruptf("%s: bad record at %d: %s", label, pos, bad)
				}
				later, _ := validRecordAfter(bytes.NewReader(raw), pos, size)
				if later || size-pos > recHeaderSize+MaxRecordBytes {
					return corruptf("%s: bad record at %d is followed by valid records: %s", label, pos, bad)
				}
				break // torn tail of the last unsealed segment
			}
			body, derr := decodeBody(h.version, raw[pos+recHeaderSize:next], h.digest)
			if derr != nil {
				return corruptf("%s: record at %d: %v", label, pos, derr)
			}
			if f.hasIdx {
				e, ok := idx[openKey{h.ns, h.digest}]
				if !ok || int64(e.off) != pos || e.length != h.length {
					return corruptf("%s: record at %d is not in its index at that offset", label, pos)
				}
			}
			seen++
			if err := fn(Record{Segno: f.segno, Sealed: f.hasIdx, NS: h.ns, Digest: hexDigest(h.digest), Body: body}); err != nil {
				return err
			}
			pos = next
		}
		if f.hasIdx && seen != len(idx) {
			return corruptf("%s: index lists %d records, segment holds %d", label, len(idx), seen)
		}
	}
	return nil
}

// readAllEntries loads and sanity-checks a sealed index's entries.
func readAllEntries(s *sealedSeg) (map[openKey]idxEntry, error) {
	raw, err := os.ReadFile(s.idxPath)
	if err != nil {
		return nil, fmt.Errorf("tapestore: read index %s: %w", s.idxPath, err)
	}
	if want := idxFixedSize + idxEntrySize*int(s.count) + idxTrailerSize; len(raw) != want {
		return nil, corruptf("index %d: changed size under the reader", s.segno)
	}
	out := make(map[openKey]idxEntry, s.count)
	var prev *idxEntry
	var bucket [256]uint32
	for i := 0; i < int(s.count); i++ {
		e := decodeIdxEntry(raw[idxFixedSize+idxEntrySize*i:])
		if !e.ns.valid() {
			return nil, corruptf("index %d: entry %d has an unknown namespace", s.segno, i)
		}
		if prev != nil {
			if c := bytes.Compare(prev.digest[:], e.digest[:]); c > 0 || (c == 0 && prev.ns >= e.ns) {
				return nil, corruptf("index %d: entries not strictly sorted at %d", s.segno, i)
			}
		}
		cp := e
		prev = &cp
		bucket[e.digest[0]]++
		out[openKey{e.ns, e.digest}] = e
	}
	var cum uint32
	for i := 0; i < 256; i++ {
		cum += bucket[i]
		if s.fanout[i] != cum {
			return nil, corruptf("index %d: fanout disagrees with its entries at %d", s.segno, i)
		}
	}
	return out, nil
}

// sortedEntries returns the map's entries sorted by (digest, ns).
func sortedEntries(m map[openKey]idxEntry) []idxEntry {
	out := make([]idxEntry, 0, len(m))
	for _, e := range m {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if c := bytes.Compare(out[i].digest[:], out[j].digest[:]); c != 0 {
			return c < 0
		}
		return out[i].ns < out[j].ns
	})
	return out
}
