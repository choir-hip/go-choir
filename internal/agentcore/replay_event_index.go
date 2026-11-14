package agentcore

import (
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// ReplayEventIndexComparison (STUB).
type ReplayEventIndexComparison struct {
	LiveCount            int                          `json:"live_count"`
	ReplayCount          int                          `json:"replay_count"`
	LiveOnlyCount        int                          `json:"live_only_count"`
	ReplayOnlyCount      int                          `json:"replay_only_count"`
	DifferentCount       int                          `json:"different_count"`
	LiveDisposableRows   int                          `json:"live_disposable_rows"`
	ReplayDisposableRows int                          `json:"replay_disposable_rows"`
	TrimBoundary         uint64                       `json:"trim_boundary"`
	LiveOnlyByKind       map[string]int               `json:"live_only_by_kind,omitempty"`
	LiveOnlyByStatus     map[string]int               `json:"live_only_by_status,omitempty"`
	ReplayOnlyByKind     map[string]int               `json:"replay_only_by_kind,omitempty"`
	ReplayOnlyByStatus   map[string]int               `json:"replay_only_by_status,omitempty"`
	DifferentByKind      map[string]int               `json:"different_by_kind,omitempty"`
	DifferentByStatus    map[string]int               `json:"different_by_status,omitempty"`
	DifferentByField     map[string]int               `json:"different_by_field,omitempty"`
	LiveOnlySeqMin       int64                        `json:"live_only_seq_min,omitempty"`
	LiveOnlySeqMax       int64                        `json:"live_only_seq_max,omitempty"`
	ReplayOnlySeqMin     int64                        `json:"replay_only_seq_min,omitempty"`
	ReplayOnlySeqMax     int64                        `json:"replay_only_seq_max,omitempty"`
	Samples              []ReplayEventIndexDifference `json:"samples,omitempty"`
}

// ReplayEventIndexDifference is one sampled row difference.
type ReplayEventIndexDifference struct {
	Kind            string   `json:"kind"`
	EventDigest     string   `json:"event_digest"`
	Sequence        int64    `json:"sequence"`
	EventKind       string   `json:"event_kind"`
	Status          string   `json:"status"`
	ReplaySequence  int64    `json:"replay_sequence,omitempty"`
	ReplayStatus    string   `json:"replay_status,omitempty"`
	LiveRowDigest   string   `json:"live_row_digest,omitempty"`
	ReplayRowDigest string   `json:"replay_row_digest,omitempty"`
	DifferentFields []string `json:"different_fields,omitempty"`
}

const replayEventIndexSampleLimit = 32

// compareReplayEventIndex (STUB).
func compareReplayEventIndex(live, replay []choirstore.EventIndexFingerprint, liveDisposable, replayDisposable int, boundary uint64) *ReplayEventIndexComparison {
	return nil
}
