package platform

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// Segment write path (stage A2 of
// docs/reports/tape-volume-forward-fix-and-surgery-2026-10-19.md, section
// 4.2). PLATFORM_TAPE_SEGMENTS=off|write, default off. With the flag off
// nothing here runs on a pin and the legacy files are written exactly as
// before. With the flag at write, only projection_batch_recorded events,
// their ProjectionBatchMediaType payloads and the pin receipts of those two
// pins are written to the computer's segment; every other pin is legacy. The
// read path never depends on the flag: it is segment first, legacy second,
// always (internal/tapestore).
//
// Dead weight (section 9.6 item 2). A legacy pin whose head CAS never lands
// is an orphan file that the tape sweep unlinks after its grace; a re-pin
// refreshes its mtime. A record has no mtime: a segmented pin whose CAS never
// lands stays in its segment until the segment is dropped (the repack and
// retention work of section 6). The bound is the pin itself: one record per
// artifact (a re-pin of the same bytes is a no-op) plus one receipt per pin
// call, each at most tapestore.MaxRecordBytes, written only through the
// pin endpoint's authenticated guest. No byte cap is enforced here: a cap
// that refused pins would turn a storage tidiness issue into a guest outage.
// It is a named residual of A2.

const (
	tapeSegmentsOff   = "off"
	tapeSegmentsWrite = "write"
)

// parseTapeSegmentsMode maps the flag to a mode. Unknown values are off.
func parseTapeSegmentsMode(raw string) (mode string, known bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", tapeSegmentsOff:
		return tapeSegmentsOff, true
	case tapeSegmentsWrite:
		return tapeSegmentsWrite, true
	}
	return tapeSegmentsOff, false
}

// SetTapeSegmentsMode records PLATFORM_TAPE_SEGMENTS. An unknown value is
// off, with a log line.
func (s *Service) SetTapeSegmentsMode(raw string) {
	if s == nil {
		return
	}
	mode, known := parseTapeSegmentsMode(raw)
	if !known {
		log.Printf("corpusd: PLATFORM_TAPE_SEGMENTS=%q is not off|write; using off", raw)
	}
	if mode == tapeSegmentsWrite {
		s.tapeSegMode.Store(1)
	} else {
		s.tapeSegMode.Store(0)
	}
}

// TapeSegmentsMode is the effective mode, "off" or "write".
func (s *Service) TapeSegmentsMode() string {
	if s != nil && s.tapeSegMode.Load() == 1 {
		return tapeSegmentsWrite
	}
	return tapeSegmentsOff
}

// tapeAppender is the one segment writer, following the artifacts root like
// tapeReader.
func (s *Service) tapeAppender() *tapestore.Appender {
	s.tapeReaderMu.Lock()
	defer s.tapeReaderMu.Unlock()
	root := filepath.Clean(s.artifactsRoot)
	if s.tapeAppenderVal == nil || s.tapeAppenderVal.Root() != root {
		s.tapeAppenderVal = tapestore.NewAppender(root)
	}
	return s.tapeAppenderVal
}

// tapeCounters are lifetime counters of the tape write path.
type tapeCounters struct {
	blobFiles   atomic.Uint64 // files created by writeBlob, any artifact
	legacyFiles atomic.Uint64 // of those, created by tape pins
	legacyPins  atomic.Uint64 // pin calls written to legacy
	segmentPins atomic.Uint64 // pin calls written to a segment
	// segmentFallbacks counts pin calls that went to the segment writer, which
	// failed, and were stored legacy instead (each is also a legacyPin).
	segmentFallbacks atomic.Uint64
}

// TapeDiagnostics is the tape write/read snapshot served on the health
// endpoint, so the forward-fix E2E needs no SSH: files created per 1,000 pin
// calls is (LegacyFilesCreated+SegmentFilesCreated)*1000/(LegacyPins+SegmentPins).
type TapeDiagnostics struct {
	SegmentsMode        string                  `json:"segments_mode"`
	SegmentPins         uint64                  `json:"segment_pins"`
	LegacyPins          uint64                  `json:"legacy_pins"`
	SegmentFallbacks    uint64                  `json:"segment_fallbacks"`
	BlobFilesCreated    uint64                  `json:"blob_files_created"`
	LegacyFilesCreated  uint64                  `json:"legacy_files_created"`
	SegmentFilesCreated uint64                  `json:"segment_files_created"`
	Reader              tapestore.Stats         `json:"reader"`
	Appender            tapestore.AppenderStats `json:"appender"`
}

// TapeDiagnostics returns the counters.
func (s *Service) TapeDiagnostics() TapeDiagnostics {
	if s == nil {
		return TapeDiagnostics{SegmentsMode: tapeSegmentsOff}
	}
	d := TapeDiagnostics{
		SegmentsMode:       s.TapeSegmentsMode(),
		SegmentPins:        s.tapeCount.segmentPins.Load(),
		LegacyPins:         s.tapeCount.legacyPins.Load(),
		SegmentFallbacks:   s.tapeCount.segmentFallbacks.Load(),
		BlobFilesCreated:   s.tapeCount.blobFiles.Load(),
		LegacyFilesCreated: s.tapeCount.legacyFiles.Load(),
		Reader:             s.tapeReader().Stats(),
	}
	s.tapeReaderMu.Lock()
	if s.tapeAppenderVal != nil {
		d.Appender = s.tapeAppenderVal.Stats()
	}
	s.tapeReaderMu.Unlock()
	d.SegmentFilesCreated = d.Appender.SegmentsCreated + d.Appender.IndexesCreated
	return d
}

// tapeSegmentable says whether a pin of this computer may go to a segment
// under the current flag: the flag is write and the id can name a segment
// directory (an id that cannot stays legacy, as it always read).
func (s *Service) tapeSegmentable(computerID string) bool {
	return s.TapeSegmentsMode() == tapeSegmentsWrite && tapestore.ValidComputerID(computerID)
}

// stageSegmented decides whether an artifact needs a new record and
// enforces the collision rule across both stores: the same bytes already in
// legacy (mtime refreshed, as writeImmutable does) or in a segment is a
// no-op; different bytes under the digest, in either, is an error. A segment
// error, including corruption, is an error: it is never answered from
// legacy or overwritten. The caller holds the CAS reuse lock.
func (s *EventArtifactService) stageSegmented(computerID string, ns tapestore.NS, body []byte) (*tapestore.Item, error) {
	digest := computerevent.DigestBytes(body)
	path, err := s.platform.artifactPath(ns.LegacyRel(digest))
	if err != nil {
		return nil, err
	}
	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		if !bytes.Equal(existing, body) {
			return nil, fmt.Errorf("event artifact service: immutable digest collision")
		}
		now := time.Now()
		if err := os.Chtimes(path, now, now); err != nil {
			return nil, fmt.Errorf("event artifact service: refresh reused artifact: %w", err)
		}
		return nil, nil
	case !errors.Is(err, os.ErrNotExist):
		return nil, err
	}
	have, found, err := s.platform.tapeReader().ReadSegment(computerID, ns, digest)
	if err != nil {
		return nil, fmt.Errorf("event artifact service: read segment: %w", err)
	}
	if found {
		if !bytes.Equal(have, body) {
			return nil, fmt.Errorf("event artifact service: immutable digest collision")
		}
		return nil, nil
	}
	return &tapestore.Item{NS: ns, Body: body}, nil
}
