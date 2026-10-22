package tracelog

import (
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"sort"
	"time"
)

// strTab interns the strings of one segment's index (owners, runs,
// trajectories, kinds), so an entry is a fixed-size struct.
type strTab struct {
	ids  map[string]uint32
	list []string
}

func (t *strTab) id(s string) uint32 {
	if id, ok := t.ids[s]; ok {
		return id
	}
	if t.ids == nil {
		t.ids = map[string]uint32{}
	}
	id := uint32(len(t.list))
	t.ids[s] = id
	t.list = append(t.list, s)
	return id
}

func (t *strTab) lookup(s string) (uint32, bool) {
	id, ok := t.ids[s]
	return id, ok
}

// ent is one line in a segment's index.
type ent struct {
	off    int64
	n      uint32 // the whole record, header included
	seq    int64
	stream int64
	ts     int64
	run    uint32
	owner  uint32
	traj   uint32
	kind   uint32
}

const entSize = 8 + 4 + 8 + 8 + 8 + 4*4

type pair [2]uint32 // (owner, run) or (owner, trajectory)

// segIdx is the in-memory index of one segment: the entries in append order
// and lookups by run and trajectory. The open segment's is bounded by the
// record cap (F18).
type segIdx struct {
	strs      strTab
	ents      []ent
	byRun     map[pair][]int32
	byTraj    map[pair][]int32
	nBound    int // boundary records
	monotone  bool
	minStream int64
	maxStream int64
	minTs     int64
	maxTs     int64
	kindN     map[uint32]int64
	kindBytes map[uint32]int64
	lastStr   int64 // last stream_seq added, for the monotone check
}

func newIdx() *segIdx {
	return &segIdx{byRun: map[pair][]int32{}, byTraj: map[pair][]int32{}, monotone: true, kindN: map[uint32]int64{}, kindBytes: map[uint32]int64{}}
}

// add indexes one line.
func (x *segIdx) add(off int64, n int, m *Meta) {
	e := ent{
		off: off, n: uint32(n), seq: m.Seq, stream: m.StreamSeq, ts: timeNanos(m.Time),
		run: x.strs.id(m.RunID), owner: x.strs.id(m.OwnerID), traj: x.strs.id(m.TrajectoryID), kind: x.strs.id(m.Kind),
	}
	x.addEnt(e)
}

func (x *segIdx) addEnt(e ent) {
	i := int32(len(x.ents))
	if len(x.ents) == 0 {
		x.minStream, x.maxStream, x.minTs, x.maxTs = e.stream, e.stream, e.ts, e.ts
	} else {
		if e.stream < x.lastStr {
			x.monotone = false
		}
		x.minStream = min(x.minStream, e.stream)
		x.maxStream = max(x.maxStream, e.stream)
		if e.ts != noTime {
			if x.minTs == noTime || e.ts < x.minTs {
				x.minTs = e.ts
			}
			x.maxTs = max(x.maxTs, e.ts)
		}
	}
	x.lastStr = e.stream
	x.ents = append(x.ents, e)
	k := pair{e.owner, e.run}
	x.byRun[k] = append(x.byRun[k], i)
	t := pair{e.owner, e.traj}
	x.byTraj[t] = append(x.byTraj[t], i)
	x.kindN[e.kind]++
	x.kindBytes[e.kind] += int64(e.n)
}

func (x *segIdx) str(id uint32) string {
	if int(id) < len(x.strs.list) {
		return x.strs.list[id]
	}
	return ""
}

// Summary is what a query needs to skip a segment without loading its
// entries. It is stored in front of the entries in the .tidx.
type Summary struct {
	Records   int                 `json:"records"` // lines and boundaries
	Lines     int                 `json:"lines"`
	ValidEnd  int64               `json:"valid_end"`
	MinStream int64               `json:"min_stream_seq"`
	MaxStream int64               `json:"max_stream_seq"`
	MinTs     int64               `json:"min_ts_nanos"`
	MaxTs     int64               `json:"max_ts_nanos"`
	CreatedAt int64               `json:"created_at_nanos"`
	SealedAt  int64               `json:"sealed_at_nanos"`
	KindCount map[string]int64    `json:"kind_count"`
	KindBytes map[string]int64    `json:"kind_bytes"`
	Owners    []string            `json:"owners"`
	Runs      map[string][2]int64 `json:"runs"`         // owner NUL run to [min seq, max seq]
	Trajs     map[string][2]int64 `json:"trajectories"` // owner NUL trajectory to [min stream_seq, max stream_seq]
}

func runKey(owner, run string) string { return owner + "\x00" + run }

// summarize computes the summary of an index.
func (x *segIdx) summarize(validEnd int64, created, sealed time.Time) *Summary {
	s := &Summary{
		Records: len(x.ents) + x.nBound, Lines: len(x.ents), ValidEnd: validEnd,
		MinStream: x.minStream, MaxStream: x.maxStream, MinTs: x.minTs, MaxTs: x.maxTs,
		CreatedAt: created.UnixNano(), SealedAt: sealed.UnixNano(),
		KindCount: map[string]int64{}, KindBytes: map[string]int64{},
		Runs: map[string][2]int64{}, Trajs: map[string][2]int64{},
	}
	for id, n := range x.kindN {
		s.KindCount[x.str(id)] = n
		s.KindBytes[x.str(id)] = x.kindBytes[id]
	}
	// From the per-run and per-trajectory lists, so no entry costs a string.
	owners := map[uint32]bool{}
	for k, list := range x.byRun {
		owners[k[0]] = true
		lo, hi := x.ents[list[0]].seq, x.ents[list[0]].seq
		for _, i := range list[1:] {
			lo, hi = min(lo, x.ents[i].seq), max(hi, x.ents[i].seq)
		}
		s.Runs[runKey(x.str(k[0]), x.str(k[1]))] = [2]int64{lo, hi}
	}
	for k, list := range x.byTraj {
		lo, hi := x.ents[list[0]].stream, x.ents[list[0]].stream
		for _, i := range list[1:] {
			lo, hi = min(lo, x.ents[i].stream), max(hi, x.ents[i].stream)
		}
		s.Trajs[runKey(x.str(k[0]), x.str(k[1]))] = [2]int64{lo, hi}
	}
	for o := range owners {
		s.Owners = append(s.Owners, x.str(o))
	}
	sort.Strings(s.Owners)
	return s
}

func (s *Summary) hasOwner(owner string) bool {
	i := sort.SearchStrings(s.Owners, owner)
	return i < len(s.Owners) && s.Owners[i] == owner
}

// --- .tidx -----------------------------------------------------------------
//
//	magic(8) version(4) segno(8) valid_end(8) tlog_size(8) summary_len(4)
//	strings_len(4) ent_count(4) header_crc(4)
//	summary JSON | summary_crc(4)
//	strings (uvarint length, bytes)... | entries (entSize each) | body_crc(4)
//
// The summary has its own checksum so a query can load it alone.

const idxHeaderSize = 8 + 4 + 8 + 8 + 8 + 4 + 4 + 4 + 4

type idxHeader struct {
	segno      uint64
	validEnd   int64
	tlogSize   int64
	summaryLen uint32
	stringsLen uint32
	entCount   uint32
}

func encodeIndex(segno uint64, validEnd, tlogSize int64, sum *Summary, x *segIdx) []byte {
	sj, _ := json.Marshal(sum)
	var strs []byte
	for _, s := range x.strs.list {
		strs = binary.AppendUvarint(strs, uint64(len(s)))
		strs = append(strs, s...)
	}
	b := make([]byte, 0, idxHeaderSize+len(sj)+4+len(strs)+len(x.ents)*entSize+4)
	b = append(b, idxMagic...)
	b = binary.BigEndian.AppendUint32(b, formatVersion)
	b = binary.BigEndian.AppendUint64(b, segno)
	b = binary.BigEndian.AppendUint64(b, uint64(validEnd))
	b = binary.BigEndian.AppendUint64(b, uint64(tlogSize))
	b = binary.BigEndian.AppendUint32(b, uint32(len(sj)))
	b = binary.BigEndian.AppendUint32(b, uint32(len(strs)))
	b = binary.BigEndian.AppendUint32(b, uint32(len(x.ents)))
	b = binary.BigEndian.AppendUint32(b, crc32.Checksum(b, crcTable))
	b = append(b, sj...)
	b = binary.BigEndian.AppendUint32(b, crc32.Checksum(sj, crcTable))
	body := len(b)
	b = append(b, strs...)
	for _, e := range x.ents {
		b = binary.BigEndian.AppendUint64(b, uint64(e.off))
		b = binary.BigEndian.AppendUint32(b, e.n)
		b = binary.BigEndian.AppendUint64(b, uint64(e.seq))
		b = binary.BigEndian.AppendUint64(b, uint64(e.stream))
		b = binary.BigEndian.AppendUint64(b, uint64(e.ts))
		b = binary.BigEndian.AppendUint32(b, e.run)
		b = binary.BigEndian.AppendUint32(b, e.owner)
		b = binary.BigEndian.AppendUint32(b, e.traj)
		b = binary.BigEndian.AppendUint32(b, e.kind)
	}
	return binary.BigEndian.AppendUint32(b, crc32.Checksum(b[body:], crcTable))
}

func parseIdxHeader(b []byte) (idxHeader, error) {
	if len(b) < idxHeaderSize {
		return idxHeader{}, corruptf("index header of %d bytes", len(b))
	}
	if string(b[:8]) != idxMagic {
		return idxHeader{}, corruptf("index magic")
	}
	if crc32.Checksum(b[:idxHeaderSize-4], crcTable) != binary.BigEndian.Uint32(b[idxHeaderSize-4:idxHeaderSize]) {
		return idxHeader{}, corruptf("index header checksum")
	}
	if v := binary.BigEndian.Uint32(b[8:12]); v != formatVersion {
		return idxHeader{}, corruptf("index version %d: %w", v, ErrUnsupportedVersion)
	}
	return idxHeader{
		segno:      binary.BigEndian.Uint64(b[12:20]),
		validEnd:   int64(binary.BigEndian.Uint64(b[20:28])),
		tlogSize:   int64(binary.BigEndian.Uint64(b[28:36])),
		summaryLen: binary.BigEndian.Uint32(b[36:40]),
		stringsLen: binary.BigEndian.Uint32(b[40:44]),
		entCount:   binary.BigEndian.Uint32(b[44:48]),
	}, nil
}

// decodeSummary reads the header and summary from the front of a .tidx.
func decodeSummary(front []byte) (idxHeader, *Summary, error) {
	h, err := parseIdxHeader(front)
	if err != nil {
		return h, nil, err
	}
	end := idxHeaderSize + int(h.summaryLen) + 4
	if h.summaryLen > 1<<28 || len(front) < end {
		return h, nil, corruptf("index summary of %d bytes", h.summaryLen)
	}
	sj := front[idxHeaderSize : idxHeaderSize+int(h.summaryLen)]
	if crc32.Checksum(sj, crcTable) != binary.BigEndian.Uint32(front[end-4:end]) {
		return h, nil, corruptf("index summary checksum")
	}
	var s Summary
	if err := json.Unmarshal(sj, &s); err != nil {
		return h, nil, corruptf("index summary: %v", err)
	}
	return h, &s, nil
}

// decodeIndex reads a whole .tidx.
func decodeIndex(b []byte) (idxHeader, *Summary, *segIdx, error) {
	h, s, err := decodeSummary(b)
	if err != nil {
		return h, nil, nil, err
	}
	body := idxHeaderSize + int(h.summaryLen) + 4
	want := body + int(h.stringsLen) + int(h.entCount)*entSize + 4
	if int(h.stringsLen) < 0 || int(h.entCount) < 0 || len(b) != want {
		return h, nil, nil, corruptf("index of %d bytes, header says %d", len(b), want)
	}
	if crc32.Checksum(b[body:want-4], crcTable) != binary.BigEndian.Uint32(b[want-4:]) {
		return h, nil, nil, corruptf("index body checksum")
	}
	x := newIdx()
	strs := b[body : body+int(h.stringsLen)]
	for len(strs) > 0 {
		n, w := binary.Uvarint(strs)
		if w <= 0 || uint64(len(strs)-w) < n {
			return h, nil, nil, corruptf("index string table")
		}
		x.strs.id(string(strs[w : w+int(n)]))
		strs = strs[w+int(n):]
	}
	p := body + int(h.stringsLen)
	x.ents = make([]ent, 0, h.entCount)
	for i := 0; i < int(h.entCount); i++ {
		e := ent{
			off: int64(binary.BigEndian.Uint64(b[p:])), n: binary.BigEndian.Uint32(b[p+8:]),
			seq: int64(binary.BigEndian.Uint64(b[p+12:])), stream: int64(binary.BigEndian.Uint64(b[p+20:])), ts: int64(binary.BigEndian.Uint64(b[p+28:])),
			run: binary.BigEndian.Uint32(b[p+36:]), owner: binary.BigEndian.Uint32(b[p+40:]),
			traj: binary.BigEndian.Uint32(b[p+44:]), kind: binary.BigEndian.Uint32(b[p+48:]),
		}
		if int(e.run) >= len(x.strs.list) || int(e.owner) >= len(x.strs.list) || int(e.traj) >= len(x.strs.list) || int(e.kind) >= len(x.strs.list) {
			return h, nil, nil, corruptf("index entry %d names a string that is not there", i)
		}
		x.addEnt(e)
		p += entSize
	}
	x.nBound = s.Records - s.Lines
	return h, s, x, nil
}
