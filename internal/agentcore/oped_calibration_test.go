package agentcore

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Calibration comes from the desk's own reading (handoff 4.C): a rejected
// op-ed the desk goes on to use is a false reject; an accepted one that ends
// in "read it, no change" is a false accept. Nothing here asks a model.

func provWithSources(t *testing.T, ids ...string) json.RawMessage {
	t.Helper()
	var links []types.ParentLink
	for _, id := range ids {
		links = append(links, types.ParentLink{Role: types.ParentLinkSource, PublicationVersionID: id, ManifestHash: "m-" + id})
	}
	raw, err := types.Provenance{SchemaVersion: 1, ParentLinks: links}.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestCalibrationFromDeskReading(t *testing.T) {
	t0 := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	meta := map[string]*opedMeta{
		"acc-used":   {VersionID: "acc-used", Verdict: types.OpedAccepted, VerdictRun: 1},
		"acc-unused": {VersionID: "acc-unused", Verdict: types.OpedAccepted, VerdictRun: 1},
		"rej-used":   {VersionID: "rej-used", Verdict: types.OpedRejectedUnclear, VerdictRun: 1},
		"rej-unused": {VersionID: "rej-unused", Verdict: types.OpedRejectedRedundant, VerdictRun: 1},
		"unjudged":   {VersionID: "unjudged"},
	}
	wakes := []opedWakeAt{
		{At: t0, VersionIDs: []string{"acc-used"}, Read: true},
		{At: t0.Add(2 * time.Hour), VersionIDs: []string{"acc-unused"}, Read: true},
		{At: t0.Add(4 * time.Hour), VersionIDs: []string{"rej-used"}, Read: true},
	}
	revs := []types.Revision{
		{RevisionID: "r0", AuthorKind: types.AuthorAppAgent, Content: "v0", CreatedAt: t0.Add(-time.Hour)},
		// After the first wake: changed, and cites acc-used.
		{RevisionID: "r1", AuthorKind: types.AuthorAppAgent, Content: "v1", CreatedAt: t0.Add(time.Hour), Provenance: provWithSources(t, "acc-used")},
		// After the second wake: the desk wrote nothing new.
		// After the third: a changed revision citing the rejected op-ed it was shown.
		{RevisionID: "r2", AuthorKind: types.AuthorAppAgent, Content: "v2", CreatedAt: t0.Add(5 * time.Hour), Provenance: provWithSources(t, "rej-used")},
	}
	got := map[string]string{}
	for _, s := range deriveOpedCalibration(meta, wakes, revs) {
		got[s.VersionID] = s.Kind
	}
	want := map[string]string{"acc-unused": "false_accept", "rej-used": "false_reject"}
	if len(got) != len(want) {
		t.Fatalf("signals = %v, want %v", got, want)
	}
	for v, kind := range want {
		if got[v] != kind {
			t.Fatalf("signals = %v, want %v", got, want)
		}
	}
	// A wake the desk has not read yet is not a false accept: it may not have run.
	unread := []opedWakeAt{{At: t0, VersionIDs: []string{"acc-unused"}}}
	if s := deriveOpedCalibration(meta, unread, nil); len(s) != 0 {
		t.Fatalf("signals with no desk turn = %v", s)
	}
}
