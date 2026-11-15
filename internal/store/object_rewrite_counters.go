package store

import (
	"math"
	"math/bits"
)

// Object rewrite counters (store-once design R1, first step): per object kind,
// how many object_recorded ops the tape carried, their payload bytes, and how
// many DISTINCT objects those ops were for, so "this kind rewrites itself N
// times" is a number read from the observability endpoint. They are always on
// and never gate behaviour. They live inside tapeWriteMaps and are updated
// under telemetryCounters.mu, the lock the other tape counters already use
// after the append returned; no lock is added and none is held across I/O.
//
// Memory bound: at most maxCounterKeys kinds (the 64th key is "other"), and
// per kind one sketch of at most rewriteExactMax 8-byte hashes (exact) or
// 1<<hllPrecision one-byte registers (estimate). That is under 64 * 1 KiB =
// 64 KiB however many objects are written. Cost per op is O(1): one hash
// (computed before the lock) and one map insert or register update.
//
// Item 3 of the station (bytes of unchanged top-level fields) is not counted:
// the previous revision of an object is not at hand where the op is built
// (objectgraph/sql_store.go putBatchChecked hands the interceptor only the new
// objects, store/projection_tape.go interceptOG marshals them), and getting it
// would be a store read per write.

const (
	// rewriteExactMax is how many distinct hashes a kind keeps exactly before
	// it folds them into the HLL registers and reports an estimate.
	rewriteExactMax = 64
	hllPrecision    = 10 // 1024 registers, standard error about 1.04/sqrt(1024) = 3.3%
	hllRegisters    = 1 << hllPrecision
)

// hashID is a 64-bit hash of a canonical id: FNV-1a with a splitmix finalizer
// so the high bits the sketch indexes on are well mixed.
func hashID(id string) uint64 {
	h := uint64(14695981039346656037)
	for i := 0; i < len(id); i++ {
		h ^= uint64(id[i])
		h *= 1099511628211
	}
	h ^= h >> 30
	h *= 0xbf58476d1ce4e5b9
	h ^= h >> 27
	h *= 0x94d049bb133111eb
	h ^= h >> 31
	return h
}

// distinctSketch counts distinct hashes exactly up to rewriteExactMax, then as
// a HyperLogLog.
type distinctSketch struct {
	exact map[uint64]struct{}
	regs  []uint8 // nil until the exact set overflows
}

func (d *distinctSketch) add(h uint64) {
	if d.regs == nil {
		if d.exact == nil {
			d.exact = map[uint64]struct{}{}
		}
		if _, ok := d.exact[h]; ok {
			return
		}
		if len(d.exact) < rewriteExactMax {
			d.exact[h] = struct{}{}
			return
		}
		d.regs = make([]uint8, hllRegisters)
		for e := range d.exact {
			d.addReg(e)
		}
		d.exact = nil
	}
	d.addReg(h)
}

func (d *distinctSketch) addReg(h uint64) {
	idx := h >> (64 - hllPrecision)
	rho := uint8(bits.LeadingZeros64(h<<hllPrecision|1<<(hllPrecision-1))) + 1
	if rho > d.regs[idx] {
		d.regs[idx] = rho
	}
}

// count returns the distinct count and whether it is an estimate.
func (d *distinctSketch) count() (int64, bool) {
	if d.regs == nil {
		return int64(len(d.exact)), false
	}
	var sum float64
	zeros := 0
	for _, r := range d.regs {
		sum += math.Ldexp(1, -int(r))
		if r == 0 {
			zeros++
		}
	}
	m := float64(hllRegisters)
	est := 0.7213 / (1 + 1.079/m) * m * m / sum
	if est <= 2.5*m && zeros > 0 {
		est = m * math.Log(m/float64(zeros))
	}
	return int64(math.Round(est)), true
}

// ObjectKindRewrites is one kind's line on the wire.
type ObjectKindRewrites struct {
	Ops            int64   `json:"ops"`
	Bytes          int64   `json:"bytes"`
	MeanBytesPerOp float64 `json:"mean_bytes_per_op"`
	// DistinctObjects is the number of different objects written since boot;
	// exact until a kind passes rewriteExactMax objects, then a HyperLogLog
	// estimate (about 3% error) and Estimated is true.
	DistinctObjects int64   `json:"distinct_objects"`
	WritesPerObject float64 `json:"writes_per_object"`
	Estimated       bool    `json:"estimated"`
}

type rewriteKind struct {
	ops, bytes int64
	distinct   distinctSketch
}

// addRewrite counts one object op. The caller holds telemetryCounters.mu.
func (m *tapeWriteMaps) addRewrite(kind string, bytes int64, hash uint64, hasHash bool) {
	if m.rewrites == nil {
		m.rewrites = map[string]*rewriteKind{}
	}
	rk, ok := m.rewrites[kind]
	if !ok {
		if kind != "other" && len(m.rewrites) >= maxCounterKeys-1 {
			kind = "other"
			rk = m.rewrites[kind]
		}
		if rk == nil {
			rk = &rewriteKind{}
			m.rewrites[kind] = rk
		}
	}
	rk.ops++
	rk.bytes += bytes
	if hasHash {
		rk.distinct.add(hash)
	}
}

func (m *tapeWriteMaps) rewriteStats() map[string]ObjectKindRewrites {
	out := make(map[string]ObjectKindRewrites, len(m.rewrites))
	for kind, rk := range m.rewrites {
		n, est := rk.distinct.count()
		r := ObjectKindRewrites{Ops: rk.ops, Bytes: rk.bytes, DistinctObjects: n, Estimated: est}
		if rk.ops > 0 {
			r.MeanBytesPerOp = float64(rk.bytes) / float64(rk.ops)
		}
		if n > 0 {
			r.WritesPerObject = float64(rk.ops) / float64(n)
		}
		out[kind] = r
	}
	return out
}
