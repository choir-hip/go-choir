package platform

// The cites_capture backfill (related-stories design 5.2). Versions
// published before the edge existed have none, and their manifests are
// immutable and hold the anchors, so the edges are a pure function of
// stored data. The act walks the latest version of every Autopaper story
// and writes the edges it lacks. It is an explicit owner/host act behind an
// internal route, never a boot-time sweep (CLAUDE.md: no boot-time
// resumers). Only the latest version needs it: the walk never reads an
// older version's edges.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// backfillPageSize is how many publications one backfill read takes.
// A variable so a test can shrink it.
var backfillPageSize = 100

// backfillMismatchSample bounds the mismatches a report names.
const backfillMismatchSample = 20

// RelatedBackfillMismatch names a story whose latest version carries a
// number of cites_capture edges other than its manifest's count of distinct
// cited captures that Store B still has.
type RelatedBackfillMismatch struct {
	PublicationID string `json:"publication_id"`
	Revision      int    `json:"revision"`
	Want          int    `json:"want"`
	Got           int    `json:"got"`
}

// RelatedBackfillReport is what a backfill did, or in a dry run would do.
type RelatedBackfillReport struct {
	DryRun bool `json:"dry_run"`
	// Stories is the Autopaper stories whose latest version was read.
	Stories int `json:"stories"`
	// EdgesExisting were already there; EdgesWritten were missing and were
	// written (in a dry run, would be).
	EdgesExisting int `json:"edges_existing"`
	EdgesWritten  int `json:"edges_written"`
	// MissingCaptures are anchors whose capture Store B no longer has: no
	// edge is written for them.
	MissingCaptures int                       `json:"missing_captures"`
	MismatchCount   int                       `json:"mismatch_count"`
	Mismatches      []RelatedBackfillMismatch `json:"mismatches,omitempty"`
}

// BackfillCitesCapture writes the cites_capture edges the latest version of
// every Autopaper story lacks. Running it again writes nothing. After the
// writes it checks each story's edge count against the distinct captures its
// manifest cites and reports every disagreement.
func (s *Service) BackfillCitesCapture(ctx context.Context, dryRun bool) (*RelatedBackfillReport, error) {
	if s == nil || s.store == nil || s.store.db == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	og := s.ogStore()
	if og == nil {
		return nil, fmt.Errorf("platform service: object graph store unavailable")
	}
	if !s.backfillMu.TryLock() {
		return nil, errBackfillRunning
	}
	defer s.backfillMu.Unlock()

	rep := &RelatedBackfillReport{DryRun: dryRun}
	after := objectIDPrefix("choir.publication")
	end := objectIDEnd("choir.publication")
	for {
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		rows, err := s.store.corpus().QueryContext(ctx,
			`SELECT canonical_id, metadata, updated_at FROM og_objects
			 WHERE canonical_id > ? AND canonical_id < ? AND object_kind = 'choir.publication' AND tombstone = FALSE
			 ORDER BY canonical_id LIMIT ?`, after, end, backfillPageSize)
		if err != nil {
			return rep, fmt.Errorf("related backfill: list publications: %w", err)
		}
		var page []frontPublicationRow
		for rows.Next() {
			var row frontPublicationRow
			var metadata string
			if err := rows.Scan(&row.CanonicalID, &metadata, &row.UpdatedAt); err != nil {
				rows.Close()
				return rep, fmt.Errorf("related backfill: scan publication: %w", err)
			}
			row.Metadata = json.RawMessage(metadata)
			page = append(page, row)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return rep, fmt.Errorf("related backfill: iterate publications: %w", err)
		}
		for _, pub := range page {
			after = pub.CanonicalID
			if err := s.backfillStory(ctx, og, pub, dryRun, rep); err != nil {
				return rep, err
			}
		}
		if len(page) < backfillPageSize {
			break
		}
	}
	log.Printf("corpusd: related backfill (dry_run=%t): %d stories, %d edges existing, %d written, %d missing captures, %d mismatches",
		dryRun, rep.Stories, rep.EdgesExisting, rep.EdgesWritten, rep.MissingCaptures, rep.MismatchCount)
	return rep, nil
}

// objectIDPrefix and objectIDEnd bound every canonical id of a kind
// ("obj:<kind>:<owner>:<suffix>"; ";" follows ":"), so a page walks the
// primary key.
func objectIDPrefix(kind string) string { return "obj:" + kind + ":" }
func objectIDEnd(kind string) string    { return "obj:" + kind + ";" }

func (s *Service) backfillStory(ctx context.Context, og *ObjectGraphStore, pub frontPublicationRow, dryRun bool, rep *RelatedBackfillReport) error {
	var meta frontPublicationMeta
	if err := json.Unmarshal(pub.Metadata, &meta); err != nil {
		return nil
	}
	if meta.State != "published" || !s.isAutopaperStory(meta) {
		return nil
	}
	versions, err := publishedVersions(ctx, og, pub.CanonicalID)
	if err != nil {
		return fmt.Errorf("related backfill: versions of %s: %w", pub.CanonicalID, err)
	}
	if len(versions) == 0 {
		return nil
	}
	latest := versions[len(versions)-1]
	manifest, err := s.frontManifest(ctx, og, latest.CanonicalID)
	if err != nil {
		// A story whose manifest cannot be read is left for the report
		// of the next run; one bad story does not stop the rest.
		log.Printf("corpusd: related backfill: skip %s v%d: %v", pub.CanonicalID, latest.Revision, err)
		return nil
	}
	rep.Stories++
	var trs []GraphTransclusion
	for _, a := range manifest.Anchors {
		ref := a.Ref
		trs = append(trs, GraphTransclusion{Ref: &ref})
	}
	cited := citedCaptureIDs(trs)
	existing, err := og.ListEdgesByKind(ctx, latest.CanonicalID, edgeCitesCapture)
	if err != nil {
		return fmt.Errorf("related backfill: edges of %s: %w", latest.CanonicalID, err)
	}
	have := map[string]bool{}
	for _, e := range existing {
		have[e.ToID] = true
	}
	var missing []string
	for _, id := range cited {
		if !have[id] {
			missing = append(missing, id)
		}
	}
	// An edge only to a capture Store B still has: never a dangling edge.
	present := map[string]bool{}
	if len(missing) > 0 {
		objs, err := og.GetObjects(ctx, missing)
		if err != nil {
			return fmt.Errorf("related backfill: captures of %s: %w", latest.CanonicalID, err)
		}
		for id, obj := range objs {
			if !obj.Tombstone {
				present[id] = true
			}
		}
	}
	at := time.Now().UTC()
	if versionObj, err := og.GetObject(ctx, latest.CanonicalID); err == nil {
		if vm, err := decodeVersionMetadata(versionObj); err == nil {
			if t, err := time.Parse(time.RFC3339Nano, vm.PublishedAt); err == nil {
				at = t.UTC()
			}
		}
	}
	var write []objectgraph.Edge
	for _, id := range missing {
		if !present[id] {
			rep.MissingCaptures++
			continue
		}
		write = append(write, makeEdge(latest.CanonicalID, id, edgeCitesCapture, at, []byte(citesCaptureMetadata)))
	}
	rep.EdgesExisting += len(have)
	rep.EdgesWritten += len(write)
	if len(write) > 0 && !dryRun {
		if err := og.PutBatch(ctx, objectgraph.Batch{Edges: write}); err != nil {
			return fmt.Errorf("related backfill: write edges of %s: %w", latest.CanonicalID, err)
		}
	}
	// Verify against the manifest: a latest version carries one edge per
	// distinct capture it cites. A capture Store B lost shows here as a
	// mismatch, counted apart in MissingCaptures.
	got := len(have) + len(write)
	if len(cited) != got {
		rep.MismatchCount++
		if len(rep.Mismatches) < backfillMismatchSample {
			rep.Mismatches = append(rep.Mismatches, RelatedBackfillMismatch{PublicationID: meta.PublicationID, Revision: latest.Revision, Want: len(cited), Got: got})
		}
	}
	return nil
}

var errBackfillRunning = errors.New("a related backfill is already running")

// HandleInternalRelatedBackfill handles POST
// /internal/platform/publications/related-backfill: the host/owner act that
// writes the cites_capture edges existing latest versions lack. Body
// {"dry_run": true} reports without writing. Internal callers only.
func (h *Handler) HandleInternalRelatedBackfill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	var req struct {
		DryRun bool `json:"dry_run"`
	}
	if r.Body != nil {
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
		if err := dec.Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
			return
		}
	}
	rep, err := h.service.BackfillCitesCapture(r.Context(), req.DryRun)
	if err != nil {
		if errors.Is(err, errBackfillRunning) {
			writeJSON(w, http.StatusConflict, apiError{Error: err.Error()})
			return
		}
		log.Printf("corpusd: related backfill: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "related backfill failed"})
		return
	}
	writeJSON(w, http.StatusOK, rep)
}
