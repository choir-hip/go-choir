package platform

// Rung 0 of the offline slice: the deployed candidate rules (mentionsFor and
// finishMention of capture_ready.go, over the real gazetteer of
// internal/entitytext) driven by the replay harness of internal/offlineslice
// over a week of exported captures. It reads flat files and writes JSONL; it
// touches no database and calls no model. Skipped unless OFFLINE_SLICE_DATA
// names the export directory (scripts/offline-slice/export).
//
//   OFFLINE_SLICE_DATA=dir OFFLINE_SLICE_OUT=dir OFFLINE_ALIASES=all|asof|none \
//     go test ./internal/platform -run TestOfflineSliceReplay -count=1 -timeout 3h

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/entitytext"
	"github.com/yusefmosiah/go-choir/internal/offlineslice"
)

// offlineRules adapts the deployed lexicon to offlineslice.Rules.
type offlineRules struct{ lex *entityLexicon }

func (r *offlineRules) Prepare(day time.Time, ents []offlineslice.Entity, stops []string) {
	x := &entityLexicon{gaz: entitytext.NewGazetteer(), entities: map[string]lexiconEntity{}, stop: map[string]entityStopwordRow{}}
	for _, e := range ents {
		x.put(lexiconEntity{EntityID: e.EntityID, Canonical: e.Canonical, Name: e.Name, Type: e.Type, FirstSeen: e.Created, Aliases: e.Aliases})
	}
	for _, k := range stops {
		x.setStop(entityStopwordRow{Key: k, Active: true})
	}
	r.lex = x
}

func (r *offlineRules) Mentions(title, lead string, known map[string]bool) []offlineslice.Mention {
	var out []offlineslice.Mention
	for _, m := range r.lex.mentionsFor(title, lead, known) {
		// finishMention with no story-edge counts: the deployed ranking and
		// the 8-candidate bound, without today's mention counts (the future).
		rm := r.lex.finishMention(m, map[string]entityCount{}, known)
		om := offlineslice.Mention{Key: m.key, Phrase: m.phrase, Hit: m.hit, InTitle: m.stats.InTitle, InLead: m.stats.InLead,
			Count: m.stats.Count, Score: m.stats.Score, Acronym: m.acronym, Ambiguous: m.ambiguous}
		for _, c := range rm.Candidates {
			om.Cands = append(om.Cands, offlineslice.Cand{EntityID: c.EntityID, Name: c.Name, Via: c.Via, Ambiguous: c.Ambiguous})
		}
		out = append(out, om)
	}
	return out
}

func TestOfflineSliceReplay(t *testing.T) {
	dir := os.Getenv("OFFLINE_SLICE_DATA")
	if dir == "" {
		t.Skip("OFFLINE_SLICE_DATA not set")
	}
	outDir := os.Getenv("OFFLINE_SLICE_OUT")
	if outDir == "" {
		outDir = dir
	}
	policy := map[string]offlineslice.AliasPolicy{"all": offlineslice.AliasesAll, "asof": offlineslice.AliasesAsOf, "none": offlineslice.AliasesNone}
	name := os.Getenv("OFFLINE_ALIASES")
	if name == "" {
		name = "asof"
	}
	pol, ok := policy[name]
	if !ok {
		t.Fatalf("OFFLINE_ALIASES %q", name)
	}
	start := time.Now()
	caps, err := offlineslice.ReadCaptures(filepath.Join(dir, "captures.csv"))
	if err != nil {
		t.Fatal(err)
	}
	vecs, err := offlineslice.ReadVectors(filepath.Join(dir, "vectors.csv"))
	if err != nil {
		t.Fatal(err)
	}
	ents, err := offlineslice.ReadEntities(filepath.Join(dir, "entities.csv"))
	if err != nil {
		t.Fatal(err)
	}
	stops, err := offlineslice.ReadStops(filepath.Join(dir, "stopwords.csv"))
	if err != nil {
		t.Fatal(err)
	}
	leads, missing, err := offlineslice.ReadLeads(filepath.Join(dir, "bodies.bin"))
	if err != nil {
		t.Fatal(err)
	}
	withLead := 0
	for i := range caps {
		caps[i].Vec = vecs[caps[i].ID]
		if caps[i].TextHash != "" {
			caps[i].Lead = leads[caps[i].TextHash]
		}
		if caps[i].Lead != "" {
			withLead++
		}
	}
	kept, dropped := offlineslice.Dedupe(caps)
	withVec := 0
	for _, c := range kept {
		if c.Vec != nil {
			withVec++
		}
	}
	t.Logf("loaded %d captures, %d dropped as repeats by URL, %d replayed; %d with a lead (%d body files missing), %d with a vector; %d entity rows, %d stop rows; load %s",
		len(caps), dropped, len(kept), withLead, missing, withVec, len(ents), len(stops), time.Since(start).Round(time.Second))
	t0 := time.Now()
	res := offlineslice.Replay(kept, ents, stops, &offlineRules{}, pol, 10)
	replay := time.Since(t0)
	f, err := os.Create(filepath.Join(outDir, "results-"+name+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	w := bufio.NewWriterSize(f, 1<<20)
	enc := json.NewEncoder(w)
	for _, r := range res {
		if err := enc.Encode(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	sum, _ := json.MarshalIndent(map[string]any{
		"aliases": name, "loaded": len(caps), "dropped_repeats": dropped, "replayed": len(kept), "with_lead": withLead,
		"body_files_missing": missing, "with_vector": withVec, "entity_rows": len(ents), "stop_rows": len(stops),
		"replay_seconds": replay.Seconds(), "total_seconds": time.Since(start).Seconds(),
	}, "", " ")
	if err := os.WriteFile(filepath.Join(outDir, "summary-"+name+".json"), sum, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("replay %s; total %s", replay.Round(time.Second), time.Since(start).Round(time.Second))
}

// TestOfflineSliceReplayB is rung 0b: source arrivals only, cleaned candidates,
// and the nearest earlier stories as a candidate source. It also needs the
// story export (scripts/offline-slice/export-stories). Env as above plus
// OFFLINE_DFMAX (default 0.01).
func TestOfflineSliceReplayB(t *testing.T) {
	dir := os.Getenv("OFFLINE_SLICE_DATA")
	if dir == "" {
		t.Skip("OFFLINE_SLICE_DATA not set")
	}
	outDir := os.Getenv("OFFLINE_SLICE_OUT")
	if outDir == "" {
		outDir = dir
	}
	cfg := offlineslice.DefaultConfigB()
	if v := os.Getenv("OFFLINE_DFMAX"); v != "" {
		if _, err := fmt.Sscanf(v, "%g", &cfg.DFMax); err != nil {
			t.Fatal(err)
		}
	}
	if v := os.Getenv("OFFLINE_DFMAX_KNOWN"); v != "" {
		if _, err := fmt.Sscanf(v, "%g", &cfg.DFMaxKnown); err != nil {
			t.Fatal(err)
		}
	}
	cfg.DFArrivalsOnly = os.Getenv("OFFLINE_DF_ARRIVALS") == "1"
	tag := os.Getenv("OFFLINE_TAG")
	if tag == "" {
		tag = "b"
	}
	start := time.Now()
	caps, err := offlineslice.ReadCaptures(filepath.Join(dir, "captures.csv"))
	if err != nil {
		t.Fatal(err)
	}
	vecs, err := offlineslice.ReadVectors(filepath.Join(dir, "vectors.csv"))
	if err != nil {
		t.Fatal(err)
	}
	ents, err := offlineslice.ReadEntities(filepath.Join(dir, "entities.csv"))
	if err != nil {
		t.Fatal(err)
	}
	stops, err := offlineslice.ReadStops(filepath.Join(dir, "stopwords.csv"))
	if err != nil {
		t.Fatal(err)
	}
	leads, _, err := offlineslice.ReadLeads(filepath.Join(dir, "bodies.bin"))
	if err != nil {
		t.Fatal(err)
	}
	sd, err := offlineslice.ReadStories(dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := range caps {
		caps[i].Vec = vecs[caps[i].ID]
		if caps[i].TextHash != "" {
			caps[i].Lead = leads[caps[i].TextHash]
		}
	}
	kept, dropped := offlineslice.Dedupe(caps)
	byCanon := map[string]offlineslice.Entity{}
	for _, e := range ents {
		byCanon[e.Canonical] = e
	}
	t.Logf("loaded %d captures (%d repeats dropped), %d stories, %d entity rows; load %s", len(caps), dropped, len(sd.Stories), len(ents), time.Since(start).Round(time.Second))
	t0 := time.Now()
	res := offlineslice.ReplayB(kept, ents, stops, &offlineRules{}, offlineslice.AliasesAsOf, sd, byCanon, cfg)
	f, err := os.Create(filepath.Join(outDir, "results-"+tag+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	w := bufio.NewWriterSize(f, 1<<20)
	enc := json.NewEncoder(w)
	for _, r := range res {
		if err := enc.Encode(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	sum, _ := json.MarshalIndent(map[string]any{"dedup_dropped": dropped, "replayed_all": len(kept), "arrivals": len(res), "dfmax": cfg.DFMax,
		"replay_seconds": time.Since(t0).Seconds(), "total_seconds": time.Since(start).Seconds()}, "", " ")
	_ = os.WriteFile(filepath.Join(outDir, "summary-"+tag+".json"), sum, 0o644)
	t.Logf("replay %s", time.Since(t0).Round(time.Second))
}
