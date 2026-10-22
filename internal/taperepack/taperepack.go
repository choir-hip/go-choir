// Package taperepack copies legacy tape pins (one file per digest under
// sha256/) into the per-computer segment store, and proves the copies. It is
// steps 2 to 4 of the repack in
// docs/reports/tape-volume-forward-fix-and-surgery-2026-10-19.md section 6.1
// (station FP9, L2-04): plan (a manifest, no writes), copy (append only,
// through the production appender) and census (segment-only reads compared
// with the legacy bytes). Nothing here deletes, moves or edits a legacy
// file, and nothing rewinds a head; the delete is a later, owner-gated step.
//
// Eligibility is the production pin path's (internal/platform
// event_artifacts.go): an event whose kind is projection_batch_recorded, or
// a payload whose media type is the projection batch type, together with
// its pin receipt. The manifest's edge: it sees only receipted pins; a pin
// with no receipt and whether the receipts are complete against the chain
// are checked by comparing the per-computer count with the platform's
// append receipts (FP9-C3).
package taperepack

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

const (
	nsEvent   = "event"
	nsPayload = "payload"

	eventNamespace   = "computer-event"
	payloadNamespace = "computer-event-payload"

	// maxReceiptBytes bounds a pin receipt read; receipts are about 1 KiB.
	maxReceiptBytes = 1 << 20
	// DefaultBatch is the number of pins per append (one fsync each).
	DefaultBatch = 64
)

// Entry is one manifest line: an eligible legacy artifact and the receipt
// that pinned it, both to be copied to the computer's segments.
type Entry struct {
	Computer      string `json:"computer"`
	NS            string `json:"ns"` // event or payload
	Digest        string `json:"digest"`
	Length        int64  `json:"length"`
	ReceiptDigest string `json:"receipt_digest"`
	ReceiptLength int64  `json:"receipt_length"`
}

func (e Entry) tapeNS() tapestore.NS {
	if e.NS == nsEvent {
		return tapestore.NSEvent
	}
	return tapestore.NSPayload
}

func (e Entry) validate() error {
	if !tapestore.ValidComputerID(e.Computer) {
		return fmt.Errorf("invalid computer %q", e.Computer)
	}
	if e.NS != nsEvent && e.NS != nsPayload {
		return fmt.Errorf("invalid ns %q", e.NS)
	}
	if !tapestore.ValidDigest(e.Digest) || !tapestore.ValidDigest(e.ReceiptDigest) {
		return fmt.Errorf("invalid digest")
	}
	if e.Length <= 0 || e.Length > tapestore.MaxRecordBytes || e.ReceiptLength <= 0 || e.ReceiptLength > maxReceiptBytes {
		return fmt.Errorf("invalid length")
	}
	return nil
}

// PlanOptions selects the computers to plan. At least one is required.
type PlanOptions struct {
	Root      string
	Computers []string
	// CountUnreceipted lists the legacy event and payload directories and
	// counts files no receipt names. It holds every receipted digest in
	// memory (32 bytes each).
	CountUnreceipted bool
}

// PlanCounts are the plan's tallies, by receipt.
type PlanCounts struct {
	Receipts            int            `json:"receipts"`
	Eligible            int            `json:"eligible"`
	OtherComputer       int            `json:"other_computer"`
	Ineligible          int            `json:"ineligible"`
	Dangling            int            `json:"dangling"`
	ArtifactMismatch    int            `json:"artifact_mismatch"`
	ReceiptNameMismatch int            `json:"receipt_name_mismatch"`
	ReceiptUndecodable  int            `json:"receipt_undecodable"`
	ScopeMismatch       int            `json:"scope_mismatch"`
	InvalidComputerID   int            `json:"invalid_computer_id"`
	TooLarge            int            `json:"too_large"`
	OtherFiles          int            `json:"other_files"`
	Unreceipted         int            `json:"unreceipted"`
	UnreceiptedCounted  bool           `json:"unreceipted_counted"`
	PerComputer         map[string]int `json:"per_computer"`
}

// PlanResult is the manifest and its counts.
type PlanResult struct {
	Entries []Entry    `json:"-"`
	Counts  PlanCounts `json:"counts"`
}

func computerSet(ids []string) (map[string]bool, error) {
	set := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if !tapestore.ValidComputerID(id) {
			return nil, fmt.Errorf("taperepack: invalid computer id %q", id)
		}
		set[id] = true
	}
	if len(set) == 0 {
		return nil, errors.New("taperepack: name at least one computer")
	}
	return set, nil
}

// eachDirEntry streams a directory's names in batches, so a directory of
// millions of files is never held whole.
func eachDirEntry(dir string, fn func(os.DirEntry) error) error {
	d, err := os.Open(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer d.Close()
	for {
		batch, err := d.ReadDir(1024)
		for _, de := range batch {
			if ferr := fn(de); ferr != nil {
				return ferr
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// readRegular reads a regular file (not a link) of at most limit bytes.
// tooLarge is true when it is larger.
func readRegular(path string, limit int64) (body []byte, tooLarge bool, err error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, false, err
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("taperepack: %s is not a regular file", path)
	}
	if info.Size() > limit {
		return nil, true, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	body, err = io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(body)) > limit {
		return nil, true, nil
	}
	return body, false, nil
}

func digestHex(b []byte) string { return computerevent.DigestBytes(b) }

// Plan reads the legacy pin receipts and lists the eligible, verified pins
// of the named computers. It writes nothing.
func Plan(opts PlanOptions) (PlanResult, error) {
	want, err := computerSet(opts.Computers)
	if err != nil {
		return PlanResult{}, err
	}
	root := filepath.Clean(opts.Root)
	counts := PlanCounts{PerComputer: map[string]int{}, UnreceiptedCounted: opts.CountUnreceipted}
	var entries []Entry
	receipted := map[[32]byte]struct{}{}
	receiptsDir := filepath.Join(root, "sha256", "pin-receipts")
	err = eachDirEntry(receiptsDir, func(de os.DirEntry) error {
		name := de.Name()
		receiptDigest, ok := strings.CutSuffix(name, ".json")
		if !ok || !tapestore.ValidDigest(receiptDigest) || !de.Type().IsRegular() {
			counts.OtherFiles++
			return nil
		}
		counts.Receipts++
		raw, tooLarge, err := readRegular(filepath.Join(receiptsDir, name), maxReceiptBytes)
		if err != nil {
			return fmt.Errorf("taperepack: read receipt %s: %w", receiptDigest, err)
		}
		if tooLarge {
			counts.TooLarge++
			return nil
		}
		if digestHex(raw) != receiptDigest {
			counts.ReceiptNameMismatch++
			return nil
		}
		var receipt computerevent.Receipt
		if err := json.Unmarshal(raw, &receipt); err != nil || receipt.ReceiptKind != "PinReceipt" || receipt.Issuer != "corpusd" {
			counts.ReceiptUndecodable++
			return nil
		}
		computer, _ := receipt.KindFields["computer_id"].(string)
		artifact, _ := receipt.KindFields["artifact_digest"].(string)
		namespace, _ := receipt.KindFields["pin_namespace"].(string)
		mediaType, _ := receipt.KindFields["media_type"].(string)
		lengthNum, _ := receipt.KindFields["length"].(json.Number)
		length, lengthErr := lengthNum.Int64()
		if !tapestore.ValidDigest(artifact) || lengthErr != nil {
			counts.ReceiptUndecodable++
			return nil
		}
		if opts.CountUnreceipted {
			var key [32]byte
			_, _ = hex.Decode(key[:], []byte(artifact))
			receipted[key] = struct{}{}
		}
		if !want[computer] {
			counts.OtherComputer++
			return nil
		}
		if !tapestore.ValidComputerID(computer) {
			counts.InvalidComputerID++
			return nil
		}
		var ns string
		switch {
		case namespace == eventNamespace:
			ns = nsEvent
		case namespace == payloadNamespace && mediaType == computerevent.ProjectionBatchMediaType:
			ns = nsPayload
		default:
			counts.Ineligible++
			return nil
		}
		body, tooLarge, err := readRegular(filepath.Join(root, "sha256", namespace, artifact), tapestore.MaxRecordBytes)
		switch {
		case errors.Is(err, os.ErrNotExist):
			counts.Dangling++
			return nil
		case err != nil:
			return fmt.Errorf("taperepack: read artifact %s: %w", artifact, err)
		case tooLarge:
			counts.TooLarge++
			return nil
		}
		if digestHex(body) != artifact || int64(len(body)) != length {
			counts.ArtifactMismatch++
			return nil
		}
		if ns == nsEvent {
			event, err := computerevent.DecodeHistoricEvent(body)
			if err != nil || event.EventKind != computerevent.EventProjectionBatchRecorded {
				counts.Ineligible++
				return nil
			}
			if event.ComputerID != computer {
				counts.ScopeMismatch++
				return nil
			}
		}
		counts.Eligible++
		counts.PerComputer[computer]++
		entries = append(entries, Entry{Computer: computer, NS: ns, Digest: artifact, Length: int64(len(body)), ReceiptDigest: receiptDigest, ReceiptLength: int64(len(raw))})
		return nil
	})
	if err != nil {
		return PlanResult{}, err
	}
	if opts.CountUnreceipted {
		for _, namespace := range []string{eventNamespace, payloadNamespace} {
			err := eachDirEntry(filepath.Join(root, "sha256", namespace), func(de os.DirEntry) error {
				if !tapestore.ValidDigest(de.Name()) {
					return nil
				}
				var key [32]byte
				_, _ = hex.Decode(key[:], []byte(de.Name()))
				if _, ok := receipted[key]; !ok {
					counts.Unreceipted++
				}
				return nil
			})
			if err != nil {
				return PlanResult{}, err
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Computer != entries[j].Computer {
			return entries[i].Computer < entries[j].Computer
		}
		return entries[i].ReceiptDigest < entries[j].ReceiptDigest
	})
	return PlanResult{Entries: entries, Counts: counts}, nil
}

// WriteManifest writes entries as JSON lines.
func WriteManifest(w io.Writer, entries []Entry) error {
	bw := bufio.NewWriter(w)
	enc := json.NewEncoder(bw)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// ReadManifest reads JSON lines and refuses any malformed or invalid line.
func ReadManifest(r io.Reader) ([]Entry, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	var out []Entry
	line := 0
	for sc.Scan() {
		line++
		text := bytes.TrimSpace(sc.Bytes())
		if len(text) == 0 {
			continue
		}
		dec := json.NewDecoder(bytes.NewReader(text))
		dec.DisallowUnknownFields()
		var e Entry
		if err := dec.Decode(&e); err != nil {
			return nil, fmt.Errorf("taperepack: manifest line %d: %v", line, err)
		}
		if err := e.validate(); err != nil {
			return nil, fmt.Errorf("taperepack: manifest line %d: %v", line, err)
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("taperepack: read manifest: %w", err)
	}
	return out, nil
}

// CopyOptions names the computers to copy; manifest lines of other
// computers are skipped and counted.
type CopyOptions struct {
	Root      string
	Entries   []Entry
	Computers []string
	Batch     int
}

// CopyCounts are the copy's tallies, by record (an artifact or a receipt)
// except Pins and SkippedOtherComputer, which count manifest lines.
type CopyCounts struct {
	Pins                 int `json:"pins"`
	SkippedOtherComputer int `json:"skipped_other_computer"`
	RecordsWritten       int `json:"records_written"`
	AlreadyInSegment     int `json:"already_in_segment"`
	Verified             int `json:"verified"`
}

type record struct {
	ns     tapestore.NS
	digest string
	body   []byte
}

// readLegacyRecord reads and verifies one legacy record against the
// manifest: it must exist, hash to its digest and have the manifest length.
func readLegacyRecord(rd *tapestore.Reader, ns tapestore.NS, digest string, length int64) ([]byte, error) {
	body, err := rd.ReadLegacy(ns, digest)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) != length {
		return nil, fmt.Errorf("taperepack: legacy %s %s is %d bytes, manifest says %d", ns, digest, len(body), length)
	}
	return body, nil
}

// Plan and Census only read (legacy files and segments) and take no lock.
//
// Copy appends the named computers' manifest records to their segments
// through the production appender (per-computer lock, one fsync per batch,
// fail-stop), skipping records already in a segment, then reads every
// record of the batch back from the segments only. Before each batch it
// refuses if a writer holds the computer's segment lock (refuseLiveWriter).
// It never writes, moves or deletes a legacy file.
func Copy(opts CopyOptions) (CopyCounts, error) {
	want, err := computerSet(opts.Computers)
	if err != nil {
		return CopyCounts{}, err
	}
	batch := opts.Batch
	if batch <= 0 {
		batch = DefaultBatch
	}
	root := filepath.Clean(opts.Root)
	rd := tapestore.NewReader(root)
	app := tapestore.NewAppender(root)
	var counts CopyCounts
	byComputer := map[string][]Entry{}
	var order []string
	for _, e := range opts.Entries {
		if err := e.validate(); err != nil {
			return counts, fmt.Errorf("taperepack: manifest entry: %v", err)
		}
		if !want[e.Computer] {
			counts.SkippedOtherComputer++
			continue
		}
		if _, ok := byComputer[e.Computer]; !ok {
			order = append(order, e.Computer)
		}
		byComputer[e.Computer] = append(byComputer[e.Computer], e)
	}
	for _, computer := range order {
		list := byComputer[computer]
		for start := 0; start < len(list); start += batch {
			if err := refuseLiveWriter(root, computer); err != nil {
				return counts, err
			}
			end := min(start+batch, len(list))
			var recs []record
			for _, e := range list[start:end] {
				body, err := readLegacyRecord(rd, e.tapeNS(), e.Digest, e.Length)
				if err != nil {
					return counts, err
				}
				receipt, err := readLegacyRecord(rd, tapestore.NSReceipt, e.ReceiptDigest, e.ReceiptLength)
				if err != nil {
					return counts, err
				}
				recs = append(recs, record{e.tapeNS(), e.Digest, body}, record{tapestore.NSReceipt, e.ReceiptDigest, receipt})
			}
			written, present := 0, 0
			err := app.AppendFunc(computer, func() ([]tapestore.Item, error) {
				written, present = 0, 0
				var items []tapestore.Item
				seen := map[string]bool{}
				for _, r := range recs {
					key := r.ns.String() + "/" + r.digest
					if seen[key] {
						present++
						continue
					}
					seen[key] = true
					// Segment-only read: the bytes hash to the digest, so a
					// hit is the same record.
					_, found, err := rd.ReadSegment(computer, r.ns, r.digest)
					if err != nil {
						return nil, fmt.Errorf("taperepack: computer %s: segment read before copy: %w", computer, err)
					}
					if found {
						present++
						continue
					}
					items = append(items, tapestore.Item{NS: r.ns, Body: r.body})
				}
				written = len(items)
				return items, nil
			})
			if err != nil {
				return counts, fmt.Errorf("taperepack: computer %s: append: %w", computer, err)
			}
			counts.Pins += end - start
			counts.RecordsWritten += written
			counts.AlreadyInSegment += present
			for _, r := range recs {
				got, found, err := rd.ReadSegment(computer, r.ns, r.digest)
				if err != nil || !found || !bytes.Equal(got, r.body) {
					return counts, fmt.Errorf("taperepack: computer %s: %s %s does not read back from a segment (found=%v err=%v)", computer, r.ns, r.digest, found, err)
				}
				counts.Verified++
			}
		}
	}
	return counts, nil
}

// CensusOptions: SegmentRoot holds tape-seg (live, or a restored tree);
// LegacyRoot holds the legacy files (default SegmentRoot). Computers, when
// given, limits the census.
type CensusOptions struct {
	SegmentRoot string
	LegacyRoot  string
	Entries     []Entry
	Computers   []string
}

// CensusCounts are by record (an artifact or a receipt).
type CensusCounts struct {
	Records       int `json:"records"`
	Equal         int `json:"equal"`
	Missing       int `json:"missing"`
	Corrupt       int `json:"corrupt"`
	Different     int `json:"different"`
	LegacyMissing int `json:"legacy_missing"`
	LegacyBad     int `json:"legacy_bad"`
}

// OK is true only when there was something to prove and all of it is equal.
func (c CensusCounts) OK() bool { return c.Records > 0 && c.Equal == c.Records }

// Census reads every manifest record from the segments only (no legacy
// fallback) and compares it with the legacy bytes.
func Census(opts CensusOptions) (CensusCounts, error) {
	var want map[string]bool
	if len(opts.Computers) > 0 {
		var err error
		if want, err = computerSet(opts.Computers); err != nil {
			return CensusCounts{}, err
		}
	}
	legacyRoot := opts.LegacyRoot
	if legacyRoot == "" {
		legacyRoot = opts.SegmentRoot
	}
	seg := tapestore.NewReader(filepath.Clean(opts.SegmentRoot))
	legacy := tapestore.NewReader(filepath.Clean(legacyRoot))
	var c CensusCounts
	check := func(computer string, ns tapestore.NS, digest string, length int64) {
		c.Records++
		got, found, err := seg.ReadSegment(computer, ns, digest)
		switch {
		case err != nil:
			c.Corrupt++
			return
		case !found:
			c.Missing++
			return
		case int64(len(got)) != length:
			c.Different++
			return
		}
		want, err := legacy.ReadLegacy(ns, digest)
		switch {
		case errors.Is(err, tapestore.ErrNotFound):
			c.LegacyMissing++
		case err != nil:
			c.LegacyBad++
		case bytes.Equal(got, want):
			c.Equal++
		default:
			c.Different++
		}
	}
	for _, e := range opts.Entries {
		if err := e.validate(); err != nil {
			return c, fmt.Errorf("taperepack: manifest entry: %v", err)
		}
		if want != nil && !want[e.Computer] {
			continue
		}
		check(e.Computer, e.tapeNS(), e.Digest, e.Length)
		check(e.Computer, tapestore.NSReceipt, e.ReceiptDigest, e.ReceiptLength)
	}
	return c, nil
}
