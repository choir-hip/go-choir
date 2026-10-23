package taperepack

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// class is the outcome for one legacy file. The zero value is unverified and
// is not deletable.
type class uint8

const (
	classUnverified class = iota
	classCovered
	classUncovered
	classCorrupt
	classBlocked
	classIneligible
	classUnreceipted
	classLegacyBad
	classTooLarge
	classVanished
	classOutOfScope
	classDroppedReceipt
)

var classNames = [...]string{"unverified", "covered", "uncovered", "corrupt", "blocked", "ineligible", "unreceipted", "legacy_bad", "too_large", "vanished", "out_of_scope", "dropped_receipt"}

func (c class) String() string {
	if int(c) < len(classNames) {
		return classNames[c]
	}
	return "unverified"
}

// Verdict is what the engine concluded about one legacy file in one pass.
//
// r4: it is the only thing a delete step may consult. Its fields are
// unexported and only the engine constructs one, so no caller can make a
// file deletable. Deletable is true only for class covered, which the engine
// assigns only after it has read every segment copy of the file for every
// computer that needs it, in the same pass, verified each (record header,
// crc, SHA-256, index) and compared its bytes with the legacy bytes it had
// just read and hashed. A corrupt copy, an unreadable store, a missing copy,
// a legacy file that does not hash, an ineligible file and an unreceipted
// file are all not deletable. The zero Verdict is not deletable.
type Verdict struct {
	ns        tapestore.NS
	digest    string
	class     class
	computers []string
	size      int64
	// listStart is when the pass listed the legacy tree: a file touched
	// after it was repinned after the attribution was frozen.
	listStart time.Time
}

// NS is the legacy namespace of the file.
func (v Verdict) NS() tapestore.NS { return v.ns }

// Digest is the file's name digest.
func (v Verdict) Digest() string { return v.digest }

// Class names the outcome: covered, uncovered, corrupt, blocked, ineligible,
// unreceipted, legacy_bad, too_large, vanished, out_of_scope, or unverified.
func (v Verdict) Class() string { return v.class.String() }

// Size is the legacy file's size in bytes, when it was read.
func (v Verdict) Size() int64 { return v.size }

// Computers are the computers whose segments were checked.
func (v Verdict) Computers() []string { return append([]string(nil), v.computers...) }

// Deletable is true only for a file whose every needed segment copy verified.
func (v Verdict) Deletable() bool { return v.class == classCovered }

// Tally counts files by outcome. Global tallies count each legacy file
// once, by its worst outcome across the computers that need it; per-computer
// tallies count a file once for each computer that needs it.
type Tally struct {
	Files       int64 `json:"files"`
	Covered     int64 `json:"covered"`
	Uncovered   int64 `json:"uncovered"`
	Corrupt     int64 `json:"corrupt"`
	Blocked     int64 `json:"blocked"`
	Ineligible  int64 `json:"ineligible"`
	Unreceipted int64 `json:"unreceipted"`
	LegacyBad   int64 `json:"legacy_bad"`
	TooLarge    int64 `json:"too_large"`
	Vanished    int64 `json:"vanished"`
	OutOfScope  int64 `json:"out_of_scope"`
	// DroppedReceipt counts files a receipt names that the join set aside
	// beside a kept one (rejected shape, ineligible, another length or
	// computer): refused, and the run is unhealthy.
	DroppedReceipt int64 `json:"dropped_receipt"`
}

func (t *Tally) count(c class) {
	t.Files++
	switch c {
	case classCovered:
		t.Covered++
	case classUncovered:
		t.Uncovered++
	case classCorrupt:
		t.Corrupt++
	case classBlocked:
		t.Blocked++
	case classIneligible:
		t.Ineligible++
	case classUnreceipted:
		t.Unreceipted++
	case classLegacyBad:
		t.LegacyBad++
	case classTooLarge:
		t.TooLarge++
	case classVanished:
		t.Vanished++
	case classOutOfScope:
		t.OutOfScope++
	case classDroppedReceipt:
		t.DroppedReceipt++
	}
}

func (t *Tally) add(o Tally) {
	t.Files += o.Files
	t.Covered += o.Covered
	t.Uncovered += o.Uncovered
	t.Corrupt += o.Corrupt
	t.Blocked += o.Blocked
	t.Ineligible += o.Ineligible
	t.Unreceipted += o.Unreceipted
	t.LegacyBad += o.LegacyBad
	t.TooLarge += o.TooLarge
	t.Vanished += o.Vanished
	t.OutOfScope += o.OutOfScope
	t.DroppedReceipt += o.DroppedReceipt
}

// CopyStats count records (an artifact or a receipt), not files.
type CopyStats struct {
	WouldCopyRecords int64 `json:"would_copy_records"`
	WouldCopyBytes   int64 `json:"would_copy_bytes"`
	CopiedRecords    int64 `json:"copied_records"`
	CopiedBytes      int64 `json:"copied_bytes"`
	AlreadyPresent   int64 `json:"already_present"`
	Appends          int64 `json:"appends"`
}

func (c *CopyStats) add(o CopyStats) {
	c.WouldCopyRecords += o.WouldCopyRecords
	c.WouldCopyBytes += o.WouldCopyBytes
	c.CopiedRecords += o.CopiedRecords
	c.CopiedBytes += o.CopiedBytes
	c.AlreadyPresent += o.AlreadyPresent
	c.Appends += o.Appends
}

// Report is a run's result. The namespaces are "event", "payload" and
// "receipt".
type Report struct {
	Version  int      `json:"version"`
	Mode     string   `json:"mode"` // census, copy-dry-run or copy-apply
	Root     string   `json:"root"`
	Filter   []string `json:"computers,omitempty"`
	Complete bool     `json:"complete"`
	Finished string   `json:"finished_at,omitempty"`

	// Listing counts the directory entries read: regular files with a
	// digest name per namespace, and everything else as other_files.
	Listing       map[string]int64            `json:"listing"`
	Global        map[string]Tally            `json:"files"`
	Computers     map[string]map[string]Tally `json:"computers_by_namespace"`
	SegmentErrors map[string]string           `json:"segment_errors,omitempty"`
	Copy          CopyStats                   `json:"copy"`

	// Receipt is the SHA-256 of the report without this field: the census
	// receipt digest a delete is gated on. The report is stored as
	// census-<digest>.json (copy-<digest>.json for a copy run).
	Receipt string `json:"receipt,omitempty"`
}

const reportVersion = 2

var namespaces = []string{"event", "payload", "receipt"}

func newReport() *Report {
	return &Report{
		Version:       reportVersion,
		Listing:       map[string]int64{},
		Global:        map[string]Tally{},
		Computers:     map[string]map[string]Tally{},
		SegmentErrors: map[string]string{},
	}
}

func nsName(ns tapestore.NS) string { return ns.String() }

func (r *Report) countGlobal(ns tapestore.NS, c class) {
	t := r.Global[nsName(ns)]
	t.count(c)
	r.Global[nsName(ns)] = t
}

func (r *Report) countComputer(computer string, ns tapestore.NS, c class) {
	m := r.Computers[computer]
	if m == nil {
		m = map[string]Tally{}
		r.Computers[computer] = m
	}
	t := m[nsName(ns)]
	t.count(c)
	m[nsName(ns)] = t
}

// merge adds a bucket's partial report.
func (r *Report) merge(o *Report) {
	for k, v := range o.Listing {
		r.Listing[k] += v
	}
	for k, v := range o.Global {
		t := r.Global[k]
		t.add(v)
		r.Global[k] = t
	}
	for c, m := range o.Computers {
		dst := r.Computers[c]
		if dst == nil {
			dst = map[string]Tally{}
			r.Computers[c] = dst
		}
		for k, v := range m {
			t := dst[k]
			t.add(v)
			dst[k] = t
		}
	}
	for c, e := range o.SegmentErrors {
		if _, ok := r.SegmentErrors[c]; !ok {
			r.SegmentErrors[c] = e
		}
	}
	r.Copy.add(o.Copy)
}

func (r *Report) total(f func(Tally) int64) int64 {
	var n int64
	for _, t := range r.Global {
		n += f(t)
	}
	return n
}

// Healthy is true when nothing the store holds is damaged: no corrupt copy,
// no blocked computer, no segment error, no legacy file that fails its hash,
// no pin receipt the tool had to set aside or could not read.
// It is the gate of the delete command; a file is still deleted only if its
// own Verdict is deletable.
func (r *Report) Healthy() bool {
	return r.Complete && len(r.SegmentErrors) == 0 &&
		r.total(func(t Tally) int64 { return t.Corrupt }) == 0 &&
		r.total(func(t Tally) int64 { return t.Blocked }) == 0 &&
		r.total(func(t Tally) int64 { return t.LegacyBad }) == 0 &&
		r.total(func(t Tally) int64 { return t.DroppedReceipt }) == 0 &&
		r.Listing["rejected_receipts"] == 0 && r.Listing["unnamable_receipts"] == 0
}

// OK is true when the run is healthy, found at least one file, and every
// file the tool is responsible for is covered (nothing uncovered).
func (r *Report) OK() bool {
	return r.Healthy() && r.total(func(t Tally) int64 { return t.Files }) > 0 &&
		r.total(func(t Tally) int64 { return t.Uncovered }) == 0
}

// Covered is the number of legacy files whose every segment copy verified.
func (r *Report) Covered() int64 { return r.total(func(t Tally) int64 { return t.Covered }) }

// canonical returns the report's JSON without its receipt digest, and the
// digest of those bytes.
func (r *Report) canonical() ([]byte, string, error) {
	c := *r
	c.Receipt = ""
	b, err := json.Marshal(&c)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(b)
	return b, hex.EncodeToString(sum[:]), nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
