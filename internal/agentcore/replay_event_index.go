package agentcore

import (
	"sort"

	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// ReplayEventIndexComparison names the computer_event_index rows that differ
// between the live store (read in the probe's pinned snapshot) and the replay
// projection. It is present only when the result is not equivalent, so an
// equivalent report keeps its digest. Rows are keyed by event_digest and
// compared over the columns the content witness hashes (all but prepared_at
// and finalized_at); a null pin list reads as "[]". Finalized
// projection_batch_recorded rows at or below TrimBoundary are not compared
// (store.EventIndexWitnessFilter) and are counted per side instead, so a
// backlog the catch-up trim has not finished is visible but is not a
// difference.
type ReplayEventIndexComparison struct {
	LiveCount            int    `json:"live_count"`
	ReplayCount          int    `json:"replay_count"`
	TrimBoundary         uint64 `json:"trim_boundary"`
	LiveDisposableRows   int    `json:"live_disposable_rows"`
	ReplayDisposableRows int    `json:"replay_disposable_rows"`
	LiveOnlyCount        int    `json:"live_only_count"`
	ReplayOnlyCount      int    `json:"replay_only_count"`
	DifferentCount       int    `json:"different_count"`

	LiveOnlyByKind     map[string]int `json:"live_only_by_kind,omitempty"`
	LiveOnlyByStatus   map[string]int `json:"live_only_by_status,omitempty"`
	ReplayOnlyByKind   map[string]int `json:"replay_only_by_kind,omitempty"`
	ReplayOnlyByStatus map[string]int `json:"replay_only_by_status,omitempty"`
	DifferentByKind    map[string]int `json:"different_by_kind,omitempty"`
	DifferentByStatus  map[string]int `json:"different_by_status,omitempty"`
	// DifferentByField counts, per column, the rows that differ in it.
	DifferentByField map[string]int `json:"different_by_field,omitempty"`

	LiveOnlySeqMin   int64 `json:"live_only_seq_min,omitempty"`
	LiveOnlySeqMax   int64 `json:"live_only_seq_max,omitempty"`
	ReplayOnlySeqMin int64 `json:"replay_only_seq_min,omitempty"`
	ReplayOnlySeqMax int64 `json:"replay_only_seq_max,omitempty"`

	Samples []ReplayEventIndexDifference `json:"samples,omitempty"`
}

// ReplayEventIndexDifference is one sampled row difference.
type ReplayEventIndexDifference struct {
	Kind            string   `json:"kind"` // live_only, replay_only, different
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

// compareReplayEventIndex returns nil when the compared rows are equal.
func compareReplayEventIndex(live, replay []choirstore.EventIndexFingerprint, liveDisposable, replayDisposable int, boundary uint64) *ReplayEventIndexComparison {
	c := &ReplayEventIndexComparison{
		LiveCount: len(live), ReplayCount: len(replay), TrimBoundary: boundary,
		LiveDisposableRows: liveDisposable, ReplayDisposableRows: replayDisposable,
		LiveOnlyByKind: map[string]int{}, LiveOnlyByStatus: map[string]int{},
		ReplayOnlyByKind: map[string]int{}, ReplayOnlyByStatus: map[string]int{},
		DifferentByKind: map[string]int{}, DifferentByStatus: map[string]int{}, DifferentByField: map[string]int{},
	}
	liveByDigest := make(map[string]choirstore.EventIndexFingerprint, len(live))
	for _, row := range live {
		liveByDigest[row.EventDigest] = row
	}
	replayByDigest := make(map[string]choirstore.EventIndexFingerprint, len(replay))
	for _, row := range replay {
		replayByDigest[row.EventDigest] = row
	}
	digests := make([]string, 0, len(liveByDigest)+len(replayByDigest))
	for d := range liveByDigest {
		digests = append(digests, d)
	}
	for d := range replayByDigest {
		if _, ok := liveByDigest[d]; !ok {
			digests = append(digests, d)
		}
	}
	sort.Slice(digests, func(i, j int) bool {
		a, b := rowOf(liveByDigest, replayByDigest, digests[i]), rowOf(liveByDigest, replayByDigest, digests[j])
		if a.Sequence != b.Sequence {
			return a.Sequence < b.Sequence
		}
		return digests[i] < digests[j]
	})
	addSample := func(s ReplayEventIndexDifference) {
		if len(c.Samples) < replayEventIndexSampleLimit {
			c.Samples = append(c.Samples, s)
		}
	}
	for _, d := range digests {
		l, lok := liveByDigest[d]
		r, rok := replayByDigest[d]
		switch {
		case lok && !rok:
			c.LiveOnlyCount++
			c.LiveOnlyByKind[l.EventKind]++
			c.LiveOnlyByStatus[l.Status]++
			if c.LiveOnlySeqMin == 0 || l.Sequence < c.LiveOnlySeqMin {
				c.LiveOnlySeqMin = l.Sequence
			}
			if l.Sequence > c.LiveOnlySeqMax {
				c.LiveOnlySeqMax = l.Sequence
			}
			addSample(ReplayEventIndexDifference{Kind: "live_only", EventDigest: d, Sequence: l.Sequence, EventKind: l.EventKind, Status: l.Status, LiveRowDigest: l.RowDigest})
		case !lok && rok:
			c.ReplayOnlyCount++
			c.ReplayOnlyByKind[r.EventKind]++
			c.ReplayOnlyByStatus[r.Status]++
			if c.ReplayOnlySeqMin == 0 || r.Sequence < c.ReplayOnlySeqMin {
				c.ReplayOnlySeqMin = r.Sequence
			}
			if r.Sequence > c.ReplayOnlySeqMax {
				c.ReplayOnlySeqMax = r.Sequence
			}
			addSample(ReplayEventIndexDifference{Kind: "replay_only", EventDigest: d, Sequence: r.Sequence, EventKind: r.EventKind, Status: r.Status, ReplayRowDigest: r.RowDigest})
		default:
			if l.RowDigest == r.RowDigest {
				continue
			}
			fields := differingReplayRunMemoryFields(l.FieldDigests, r.FieldDigests)
			if len(fields) == 0 {
				fields = []string{"row"}
			}
			c.DifferentCount++
			c.DifferentByKind[l.EventKind]++
			c.DifferentByStatus[l.Status]++
			for _, f := range fields {
				c.DifferentByField[f]++
			}
			addSample(ReplayEventIndexDifference{
				Kind: "different", EventDigest: d, Sequence: l.Sequence, EventKind: l.EventKind, Status: l.Status,
				ReplaySequence: r.Sequence, ReplayStatus: r.Status,
				LiveRowDigest: l.RowDigest, ReplayRowDigest: r.RowDigest, DifferentFields: fields,
			})
		}
	}
	if c.LiveOnlyCount+c.ReplayOnlyCount+c.DifferentCount == 0 {
		return nil
	}
	return c
}

func rowOf(live, replay map[string]choirstore.EventIndexFingerprint, digest string) choirstore.EventIndexFingerprint {
	if row, ok := live[digest]; ok {
		return row
	}
	return replay[digest]
}
