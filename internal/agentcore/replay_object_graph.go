package agentcore

import (
	"sort"

	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// ReplayObjectGraphComparison names the og_objects rows that differ between
// the live store and the replay projection. It is present only when they
// differ, so an equivalent report keeps its digest.
type ReplayObjectGraphComparison struct {
	LiveCount       int                           `json:"live_count"`
	ReplayCount     int                           `json:"replay_count"`
	LiveOnlyCount   int                           `json:"live_only_count"`
	ReplayOnlyCount int                           `json:"replay_only_count"`
	ChangedCount    int                           `json:"changed_count"`
	ByObjectKind    map[string]int                `json:"by_object_kind"`
	Samples         []ReplayObjectGraphDifference `json:"samples,omitempty"`
}

// ReplayObjectGraphDifference is one differing row.
type ReplayObjectGraphDifference struct {
	Kind        string   `json:"kind"` // live_only, replay_only, changed
	CanonicalID string   `json:"canonical_id"`
	ObjectKind  string   `json:"object_kind"`
	Fields      []string `json:"fields,omitempty"`
	LiveUpdated string   `json:"live_updated_at,omitempty"`
}

const replayObjectGraphSampleLimit = 48

func compareReplayObjectGraph(live, replay []choirstore.ObjectGraphFingerprint) *ReplayObjectGraphComparison {
	liveByID := make(map[string]choirstore.ObjectGraphFingerprint, len(live))
	for _, row := range live {
		liveByID[row.CanonicalID] = row
	}
	replayByID := make(map[string]choirstore.ObjectGraphFingerprint, len(replay))
	for _, row := range replay {
		replayByID[row.CanonicalID] = row
	}
	ids := make([]string, 0, len(liveByID)+len(replayByID))
	for id := range liveByID {
		ids = append(ids, id)
	}
	for id := range replayByID {
		if _, ok := liveByID[id]; !ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	c := &ReplayObjectGraphComparison{LiveCount: len(live), ReplayCount: len(replay), ByObjectKind: map[string]int{}}
	add := func(d ReplayObjectGraphDifference) {
		c.ByObjectKind[d.ObjectKind]++
		if len(c.Samples) < replayObjectGraphSampleLimit {
			c.Samples = append(c.Samples, d)
		}
	}
	for _, id := range ids {
		l, lok := liveByID[id]
		r, rok := replayByID[id]
		switch {
		case lok && !rok:
			c.LiveOnlyCount++
			add(ReplayObjectGraphDifference{Kind: "live_only", CanonicalID: id, ObjectKind: l.ObjectKind, LiveUpdated: l.UpdatedAt})
		case !lok && rok:
			c.ReplayOnlyCount++
			add(ReplayObjectGraphDifference{Kind: "replay_only", CanonicalID: id, ObjectKind: r.ObjectKind})
		default:
			var fields []string
			if l.ObjectKind != r.ObjectKind {
				fields = append(fields, "object_kind")
			}
			if l.VersionID != r.VersionID {
				fields = append(fields, "version_id")
			}
			if l.ContentHash != r.ContentHash {
				fields = append(fields, "content_hash")
			}
			if l.MetadataDigest != r.MetadataDigest {
				fields = append(fields, "metadata")
			}
			if l.BodyDigest != r.BodyDigest {
				fields = append(fields, "body")
			}
			if l.UpdatedAt != r.UpdatedAt {
				fields = append(fields, "updated_at")
			}
			if l.Tombstone != r.Tombstone {
				fields = append(fields, "tombstone")
			}
			if l.SupersededBy != r.SupersededBy {
				fields = append(fields, "superseded_by")
			}
			if len(fields) > 0 {
				c.ChangedCount++
				add(ReplayObjectGraphDifference{Kind: "changed", CanonicalID: id, ObjectKind: l.ObjectKind, Fields: fields, LiveUpdated: l.UpdatedAt})
			}
		}
	}
	if c.LiveOnlyCount+c.ReplayOnlyCount+c.ChangedCount == 0 {
		return nil
	}
	return c
}
