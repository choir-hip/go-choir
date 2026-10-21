package platform

import "github.com/yusefmosiah/go-choir/internal/tapestore"

// STUB for the test-first run.
type TapeDiagnostics struct {
	SegmentsMode        string                  `json:"segments_mode"`
	SegmentPins         uint64                  `json:"segment_pins"`
	LegacyPins          uint64                  `json:"legacy_pins"`
	BlobFilesCreated    uint64                  `json:"blob_files_created"`
	LegacyFilesCreated  uint64                  `json:"legacy_files_created"`
	SegmentFilesCreated uint64                  `json:"segment_files_created"`
	Reader              tapestore.Stats         `json:"reader"`
	Appender            tapestore.AppenderStats `json:"appender"`
}

func (s *Service) SetTapeSegmentsMode(string)       {}
func (s *Service) TapeDiagnostics() TapeDiagnostics { return TapeDiagnostics{SegmentsMode: "off"} }
