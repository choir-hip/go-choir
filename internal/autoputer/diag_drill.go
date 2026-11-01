package autoputer

// The isolation drill load (diag_drill_test.go lists its failure modes): a
// host-peer-only POST that makes this guest a bounded noisy neighbour, so the
// guest isolation drills read a real guest's disk and CPU load through the
// host's per-VM bounds (docs/definitions/choir-guest-isolation-2026-11-01.md).
// The operator runs it on d@e only; the paper and owners' computers are never
// drill targets. The guest is told no computer class, so the bounds here, not
// a class check, keep it small: one drill at a time, at most five minutes and
// 2 GiB, a write refused under twice its size free, and the scratch file
// removed however the drill ends.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/server"
)

const (
	drillMaxSeconds = 300
	drillMaxMiB     = 2048
	drillMaxThreads = 64
	drillChunk      = 1 << 20
	// drillMaxSamples bounds the latencies an IO drill keeps (2 MiB).
	drillMaxSamples = 1 << 18
	// drillSyncEvery is how many chunks a write drill writes between
	// fdatasyncs: the writes reach the disk, not only the guest cache.
	drillSyncEvery = 16
	// drillScratchDir is under the persistent volume (data.img).
	drillScratchDir = "/mnt/persistent/.diag-drill"
)

// drillReport is what a drill did, from the guest's side.
type drillReport struct {
	Kind      string  `json:"kind"`
	Seconds   float64 `json:"seconds"`
	Bytes     int64   `json:"bytes"`
	Ops       int64   `json:"ops"`
	P50Micros int64   `json:"p50_us"`
	P99Micros int64   `json:"p99_us"`
	MaxMicros int64   `json:"max_us"`
}

type diagDrill struct {
	dir     string
	running atomic.Bool
	free    func(dir string) (int64, error)
}

func newDiagDrill(dir string) *diagDrill {
	return &diagDrill{dir: dir, free: freeBytes}
}

func (d *diagDrill) busy() bool { return d.running.Load() }

func freeBytes(dir string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}

func (d *diagDrill) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !server.HostPeerCaller(r) {
		http.Error(w, "host peer required", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	kind := q.Get("kind")
	seconds, err := strconv.Atoi(q.Get("seconds"))
	if err != nil || seconds < 1 || seconds > drillMaxSeconds {
		http.Error(w, fmt.Sprintf("seconds must be 1..%d", drillMaxSeconds), http.StatusBadRequest)
		return
	}
	var mib, threads int
	switch kind {
	case "io-write", "io-read":
		mib, err = strconv.Atoi(q.Get("mib"))
		if err != nil || mib < 1 || mib > drillMaxMiB {
			http.Error(w, fmt.Sprintf("mib must be 1..%d", drillMaxMiB), http.StatusBadRequest)
			return
		}
	case "cpu":
		threads = runtime.NumCPU()
		if raw := q.Get("threads"); raw != "" {
			threads, err = strconv.Atoi(raw)
			if err != nil || threads < 1 || threads > drillMaxThreads {
				http.Error(w, fmt.Sprintf("threads must be 1..%d", drillMaxThreads), http.StatusBadRequest)
				return
			}
		}
	default:
		http.Error(w, "kind must be io-write, io-read or cpu", http.StatusBadRequest)
		return
	}
	if !d.running.CompareAndSwap(false, true) {
		http.Error(w, "a drill is running", http.StatusConflict)
		return
	}
	defer d.running.Store(false)

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(seconds)*time.Second)
	defer cancel()
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(time.Duration(seconds)*time.Second + time.Minute))
	var rep drillReport
	if kind == "cpu" {
		rep = drillCPU(ctx, threads)
	} else {
		if err := os.MkdirAll(d.dir, 0o700); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		size := int64(mib) * drillChunk
		free, err := d.free(d.dir)
		if err != nil {
			http.Error(w, "free space: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if free < 2*size {
			http.Error(w, "not enough free space for the drill", http.StatusInsufficientStorage)
			return
		}
		rep, err = drillIO(ctx, d.dir, kind, size)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	rep.Kind = kind
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rep)
}

func drillCPU(ctx context.Context, threads int) drillReport {
	start := time.Now()
	var ops atomic.Int64
	var wg sync.WaitGroup
	for range threads {
		wg.Add(1)
		go func() {
			defer wg.Done()
			x := uint64(1)
			for n := int64(0); ; n++ {
				if n%(1<<16) == 0 && ctx.Err() != nil {
					ops.Add(n)
					return
				}
				x = x*6364136223846793005 + 1442695040888963407
			}
		}()
	}
	wg.Wait()
	// A spin has no per-operation latency; the latency fields stay zero.
	return drillReport{Seconds: time.Since(start).Seconds(), Ops: ops.Load()}
}

// drillIO writes (io-write) or reads (io-read) a scratch file of size bytes in
// 1 MiB chunks until ctx ends, timing each chunk. A read drill writes the file
// first, then drops it from the guest's page cache before each pass so every
// read reaches the disk. The file is removed however the drill ends.
func drillIO(ctx context.Context, dir, kind string, size int64) (drillReport, error) {
	f, err := os.CreateTemp(dir, "drill-*")
	if err != nil {
		return drillReport{}, err
	}
	path := f.Name()
	defer func() {
		_ = f.Close()
		_ = os.Remove(path)
	}()
	buf := make([]byte, drillChunk)
	for i := range buf {
		buf[i] = byte(i * 31)
	}
	if kind == "io-read" {
		for off := int64(0); off < size; off += drillChunk {
			if err := ctx.Err(); err != nil {
				return drillReport{}, err
			}
			if _, err := f.WriteAt(buf, off); err != nil {
				return drillReport{}, err
			}
		}
		if err := f.Sync(); err != nil {
			return drillReport{}, err
		}
	}
	start := time.Now()
	var lat []int64
	var total, ops int64
	var off int64
	for n := 0; ctx.Err() == nil; n++ {
		if off >= size {
			off = 0
		}
		if kind == "io-read" && off == 0 {
			dropFromCache(f, size)
		}
		t := time.Now()
		var err error
		if kind == "io-write" {
			_, err = f.WriteAt(buf, off)
			if err == nil && (n+1)%drillSyncEvery == 0 {
				err = fdatasync(f)
			}
		} else {
			_, err = f.ReadAt(buf, off)
		}
		if err != nil {
			return drillReport{}, err
		}
		// At most drillMaxSamples latencies are kept; past that each new one
		// replaces a slot in turn, a bounded sample of the whole run.
		if us := time.Since(t).Microseconds() + 1; len(lat) < drillMaxSamples {
			lat = append(lat, us)
		} else {
			lat[n%drillMaxSamples] = us
		}
		ops++
		total += drillChunk
		off += drillChunk
	}
	if kind == "io-write" {
		_ = fdatasync(f)
	}
	return drillReport{
		Seconds:   time.Since(start).Seconds(),
		Bytes:     total,
		Ops:       ops,
		P50Micros: percentile(lat, 50),
		P99Micros: percentile(lat, 99),
		MaxMicros: percentile(lat, 100),
	}, nil
}

func percentile(lat []int64, p int) int64 {
	if len(lat) == 0 {
		return 0
	}
	s := slices.Clone(lat)
	slices.Sort(s)
	i := (len(s) - 1) * p / 100
	return s[i]
}
