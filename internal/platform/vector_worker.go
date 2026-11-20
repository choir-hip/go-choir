package platform

// The embed worker (station V2). Its work list is derived at the start of
// every pass from what should have a vector and does not: the versions of
// published stories newest first, then public captures newest first. Nothing
// is queued in memory beyond the pass in hand, nothing is dropped, nothing is
// refused and nothing is capped. A crash restart resumes nothing: the first
// pass re-derives the gap, as the full-text catch-up and the capture
// projection's reconcile do (CLAUDE.md, Restarts). It does not pace itself
// (see vectorBatch), logs remaining= and a rate, and when the embedder is
// down backs off after its real failure and says so once.
//
// One writer: corpusd alone writes vector rows. A vector made by this worker
// and one made on the Mac (vector_import.go) go through the same checks and
// the same write functions: the model's hash, the text_sha of the passage this
// host cuts itself, and, for an import, a sample re-embedded here.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/capturefts"
	"github.com/yusefmosiah/go-choir/internal/embedder"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Variables so tests can shrink them.
var (
	// vectorStoryBatch is how many publications one read of the story walk
	// takes.
	vectorStoryBatch = 200
	// vectorBatch is how many inputs go in one embedder call, a size and not a
	// limit. The worker sends one batch at a time and waits for its answer, so
	// a search's query, which goes to the same unit, waits behind at most one
	// batch: four passages of about 300 tokens is one to three seconds on the
	// unit's four threads (350 tokens a second measured on this CPU, batches
	// of four no slower per token than one). The worker does not rest for
	// searches or space its calls: the unit's low CPU weight is what lets
	// guests win (CLAUDE.md, no limits on our own model calls).
	vectorBatch = 4
	// vectorGapPage is how many gap rows one statement reads. The gap statement
	// scans every projected capture of the computer (about 2 s at 217,000), so it
	// runs once per page and the page is fed to the embedder vectorBatch at a
	// time; it is a size of one read, not a limit on the embedder's calls.
	vectorGapPage = 512
	// vectorInboxCheckEvery is how often, at most, a running pass looks between
	// embed batches for a file in the spool inbox, and yields to it.
	vectorInboxCheckEvery = 60 * time.Second
	// vectorStartDelay is how long after boot the first pass waits.
	vectorStartDelay = 30 * time.Second
	// vectorIdleInterval is how long a worker with nothing to do waits before
	// it looks again (a kick ends the wait).
	vectorIdleInterval = 5 * time.Minute
	// vectorProgressEvery is how often a pass logs progress.
	vectorProgressEvery = 30 * time.Second
	// vectorBackoffStart and vectorBackoffMax bound the wait after a failed pass.
	vectorBackoffStart = 5 * time.Second
	vectorBackoffMax   = 5 * time.Minute
)

// logEmbedGap writes the gap reader's line (scripts/status-vectors): the
// vectors held, the rows that should have one, and what remains, for stories
// (versions: a version's passages are written whole) or captures. rows is
// what the batch embedded. Written at the start of a pass, at each batch
// boundary and at the end of a phase, never per row. refused counts the
// passages (stories) or leads (captures) the embedder refused and that are
// recorded as such; they are not in the gap. want counts only what satisfies
// the one SQL predicate (captures) or the walk and that predicate (versions),
// so remaining is exact: a capture that can never be eligible is in neither
// have nor want. The exceptions are a capture whose text blob cannot be read
// now and a version the pass finds not public, which leave want when the pass
// reaches them.
func logEmbedGap(shape string, since time.Time, rows int, have, want, refused int64) {
	remaining := max(want-have, 0)
	log.Printf("corpusd retrieval op=embed_gap shape=%s ms=%.2f rows=%d have=%d want=%d remaining=%d model=%s refused=%d",
		shape, float64(time.Since(since).Microseconds())/1000, rows, have, want, remaining, embedder.ModelHash[:12], refused)
}

// vecIdleMark is what the last complete story walk saw (see storyMark): the
// next pass skips the walk while the tables still show the same.
type vecIdleMark struct {
	valid      bool
	mark       storyMarkT
	want, have int64
}

// storyMarkT is the cheap summary of the publications and versions: the newest
// publication's update time, the counts of live publications and versions, and
// the vector rows held.
type storyMarkT struct {
	newest         time.Time
	pubs, versions int64
	// latest is the newest creation time of a live version: a tombstone and a
	// new version in one interval leave the counts alone but move it.
	latest time.Time
	// held is the story vector rows of the current model, so a table that
	// lost or changed rows since the walk is walked again.
	held int64
}

// storyMark reads the mark: one row and three counts.
func (s *Service) storyMark(ctx context.Context) (storyMarkT, error) {
	var m storyMarkT
	rows, err := s.frontPublicationBatch(ctx, nil, 1)
	if err != nil {
		return m, err
	}
	if len(rows) > 0 {
		m.newest = rows[0].UpdatedAt
	}
	db := s.store.corpus()
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM og_objects WHERE object_kind = 'choir.publication' AND NOT tombstone`).Scan(&m.pubs); err != nil {
		return m, err
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(MAX(created_at), 'epoch'::timestamptz) FROM og_objects WHERE object_kind = 'choir.publication_version' AND NOT tombstone`).Scan(&m.versions, &m.latest); err != nil {
		return m, err
	}
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM vec_story_passages WHERE model = ?`, embedder.ModelHash).Scan(&m.held)
	return m, err
}

// errEmbedderDown marks a pass that stopped because the embedder cannot answer.
var errEmbedderDown = errors.New("embedder down")

// VectorPassResult is what one pass did.
type VectorPassResult struct {
	StoryVersions, StoryPassages int
	Captures                     int
	// Skipped are items that could not be embedded now (unreadable text, an
	// input the server refused); they stay in the gap and the next pass tries
	// them again.
	Skipped int
	// Refused are passages and leads the embedder refused in this pass; they
	// are recorded (no vector, the reason) and are not retried until the model
	// changes.
	Refused int
	// Imports are inbox files brought to an outcome (done/ or refused/) in this
	// pass, at most one; a file put back to wait is not counted.
	Imports int
	// Yielded says the pass stopped early because the spool inbox holds a file
	// the next pass would take; the worker starts that pass at once.
	Yielded bool
	// Removed are vectors deleted because their text is no longer public.
	Removed int
	// StoryGap and CaptureGap are the counts without a vector at the start.
	StoryGap, CaptureGap int64
}

// Worked says the pass embedded or removed something.
func (r VectorPassResult) Worked() bool {
	return r.StoryVersions+r.Captures+r.Removed+r.Refused+r.Imports > 0
}

// ---- what the text of a vector is ----

// captureVectorSource reads what the vector index needs of a capture object:
// its title and the text blob its lead passage is cut from (empty when it has
// none). ok is false for a capture that must not have a vector: a tombstoned
// or superseded one, one with no scope, and above all an owner-private one,
// the same exclusion the full-text index makes (capture_fulltext.go).
func captureVectorSource(obj objectgraph.Object) (title, textHash string, ok bool) {
	if obj.ObjectKind != objectgraph.WebCaptureObjectKind {
		return "", "", false
	}
	if strings.TrimSpace(obj.OwnerID) == "" || strings.TrimSpace(obj.ComputerID) == "" {
		return "", "", false
	}
	if obj.Tombstone || strings.TrimSpace(obj.SupersededBy) != "" || types.LabelsFromMetadata(obj.Metadata).OwnerPrivate() {
		return "", "", false
	}
	meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
	if err != nil {
		return "", "", false
	}
	hash := strings.TrimPrefix(strings.TrimSpace(meta.ExtractedTextBlobID), "sha256:")
	if len(hash) != 64 {
		hash = ""
	}
	return strings.TrimSpace(meta.Title), hash, true
}

// captureLeadPassage is a capture's first passage: the first span
// capturefts.Passages cuts over the first 256 KiB of its text. A capture with
// no text has an empty lead (its vector is its title's).
func captureLeadPassage(bodies *CaptureBodyStore, textHash string) (string, error) {
	if textHash == "" || bodies == nil {
		return "", nil
	}
	// The first 256 KiB plus 8 bytes, as the Mac's export reads
	// (scripts/embed-pass-export.py: blob(..., MAX_INDEXED_BYTES + 8)).
	raw, err := bodies.Prefix(textHash, capturefts.MaxIndexedBytes+8)
	if err != nil {
		return "", err
	}
	text := string(raw)
	spans := capturefts.Passages(text)
	if len(spans) == 0 {
		return "", nil
	}
	return passageText(text[spans[0].Start:spans[0].End]), nil
}

// passageText is a passage as the text the vector is made of: cut on the raw
// bytes, then each invalid byte becomes U+FFFD, as the export decodes a
// passage (bytes.decode("utf-8", "replace") after cutting). Nothing is
// stripped. (A truncated multi-byte sequence is one U+FFFD in the export and
// one per byte here; such a passage then differs in text_sha and the import
// drops its row rather than take it.)
func passageText(p string) string {
	if utf8.ValidString(p) {
		return p
	}
	return string([]rune(p))
}

// captureDay is the day a capture stands for: published, else fetched, else
// the object's update.
func captureDay(obj objectgraph.Object) time.Time {
	if meta, err := objectgraph.WebCaptureMetadataFromObject(obj); err == nil {
		if t := parseCaptureTime(meta.PublishedAt); t.Valid {
			return t.Time.UTC()
		}
		if t := parseCaptureTime(meta.FetchedAt); t.Valid {
			return t.Time.UTC()
		}
	}
	return obj.UpdatedAt.UTC()
}

// storyVersionRef names one published version of a story that should have
// vectors.
type storyVersionRef struct {
	StoryID, VersionID string
	Revision           int
	Title              string
}

// storyVersionText is a version's passages: what the vectors are made of.
type storyVersionText struct {
	Title    string
	Day      time.Time
	Passages []string
}

// loadStoryVersionText reads a published version's text and cuts its
// passages with capturefts.Passages (1,800 runes, at most 2,400). ok is false
// for a version that cannot or must not have vectors: one whose access policy
// is not public, or whose blob is unreadable (err says why).
func (s *Service) loadStoryVersionText(ctx context.Context, og *ObjectGraphStore, storyID, versionID string) (storyVersionText, bool, error) {
	var out storyVersionText
	storyObj, err := og.GetObject(ctx, storyID)
	if err != nil {
		return out, false, fmt.Errorf("story %s: %w", storyID, err)
	}
	var meta frontPublicationMeta
	if storyObj.Tombstone || json.Unmarshal(storyObj.Metadata, &meta) != nil || !s.looksSearchable(frontPublicationRow{Metadata: storyObj.Metadata}) {
		return out, false, nil
	}
	versionObj, err := og.GetObject(ctx, versionID)
	if err != nil {
		return out, false, fmt.Errorf("version %s: %w", versionID, err)
	}
	if versionObj.Tombstone {
		return out, false, nil
	}
	manifest, err := s.frontManifest(ctx, og, versionID)
	if err != nil {
		return out, false, err
	}
	if !publicationAccessPolicyAllowsPublicRoute(manifest.AccessPolicy) || manifest.StorageRef == "" {
		return out, false, nil
	}
	raw, err := s.readBlob(manifest.StorageRef)
	if err != nil {
		return out, false, err
	}
	content := string(raw)
	// The title as the Mac's export takes it (embed-pass-export.py:
	// str(man.get("title") or pmeta.get("title") or "").strip()): the
	// version's manifest title, else the publication's, then stripped. A
	// whitespace-only manifest title is a title (and strips to none).
	out.Title = strings.TrimSpace(manifest.Title)
	if manifest.Title == "" {
		out.Title = strings.TrimSpace(meta.Title)
	}
	out.Day = versionObj.CreatedAt.UTC()
	for _, sp := range capturefts.Passages(content) {
		out.Passages = append(out.Passages, passageText(content[sp.Start:sp.End]))
	}
	return out, true, nil
}

// ---- the derived work list ----

// eligibleStoryVersions lists every version of every published story or
// opinion piece, newest publication first and newest version first within a
// publication. The publications come in windows of vectorStoryBatch and their
// has_version edges in one statement per chunk of publications (the same
// reading of the edges as publishedVersions: first edge of a revision wins,
// revision above zero), not one query a publication. Whether a version's
// access policy is public is read when it is embedded.
func (s *Service) eligibleStoryVersions(ctx context.Context) ([]storyVersionRef, error) {
	if s.ogStore() == nil {
		return nil, errors.New("object graph store unavailable")
	}
	type pubRef struct{ id, title string }
	var pubs []pubRef
	seen := map[string]bool{}
	var after *frontCursor
	for {
		rows, err := s.frontPublicationBatch(ctx, after, vectorStoryBatch)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if seen[row.CanonicalID] {
				continue
			}
			seen[row.CanonicalID] = true
			if !s.looksSearchable(row) {
				continue
			}
			var meta frontPublicationMeta
			_ = json.Unmarshal(row.Metadata, &meta)
			pubs = append(pubs, pubRef{row.CanonicalID, meta.Title})
		}
		if len(rows) < vectorStoryBatch {
			break
		}
		after = frontCursorAt(rows[len(rows)-1])
	}
	var out []storyVersionRef
	for start := 0; start < len(pubs); start += 500 {
		chunk := pubs[start:min(start+500, len(pubs))]
		ids := make([]string, len(chunk))
		for i, p := range chunk {
			ids[i] = p.id
		}
		byPub, err := s.store.publishedVersionsOf(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, p := range chunk {
			vs := byPub[p.id]
			for i := len(vs) - 1; i >= 0; i-- {
				out = append(out, storyVersionRef{StoryID: p.id, VersionID: vs[i].CanonicalID, Revision: vs[i].Revision, Title: p.title})
			}
		}
	}
	return out, nil
}

// publishedVersionsOf reads the has_version edges of many publications in one
// statement and returns each publication's published versions, oldest
// revision first, exactly as publishedVersions reads one.
func (s *Store) publishedVersionsOf(ctx context.Context, pubIDs []string) (out map[string][]publishedVersion, err error) {
	done := s.retrievalTimer(retrievalPublishedVersionsOf)
	defer func() { done("chunk", len(out), err) }()
	out = map[string][]publishedVersion{}
	if len(pubIDs) == 0 {
		return out, nil
	}
	args := make([]any, len(pubIDs))
	for i, id := range pubIDs {
		args[i] = id
	}
	rows, err := s.corpus().QueryContext(ctx, `SELECT from_id, to_id, metadata FROM og_edges
		WHERE kind = 'has_version' AND tombstone = FALSE AND from_id IN (?`+strings.Repeat(",?", len(pubIDs)-1)+`) ORDER BY from_id, created_at, edge_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var from, to, meta string
		if err := rows.Scan(&from, &to, &meta); err != nil {
			return nil, err
		}
		var m struct {
			Revision int `json:"revision"`
		}
		if json.Unmarshal([]byte(meta), &m) != nil || m.Revision <= 0 {
			continue
		}
		key := from + "#" + strconv.Itoa(m.Revision)
		if seen[key] {
			continue
		}
		seen[key] = true
		out[from] = append(out[from], publishedVersion{Revision: m.Revision, CanonicalID: to})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for id := range out {
		vs := out[id]
		sort.Slice(vs, func(i, j int) bool { return vs[i].Revision < vs[j].Revision })
	}
	return out, nil
}

// storyVersionsWithVectors is the set of version ids that hold vectors of the
// current model. A version is written whole in one transaction, so one row
// means all its passages.
func (s *Store) storyVersionsWithVectors(ctx context.Context, model string) (map[string]bool, error) {
	rows, err := s.corpus().QueryContext(ctx, `SELECT DISTINCT version_id FROM vec_story_passages WHERE model = ?`, model)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// captureCounts is the captures that should have a vector (want: projected
// and satisfying vecCaptureEligibleSQL, the one predicate the sweep and the
// search use) and the vectors held of the model among them (have; a row
// recording a refusal counts as held). The two are exact: a capture that can
// never be eligible is in neither.
func (s *Store) captureCounts(ctx context.Context, model string) (want, have int64, err error) {
	vectorWholeTableQueries.Add(1)
	if err = s.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM proj_capture_index v WHERE `+vecCaptureEligibleSQL).Scan(&want); err != nil {
		return
	}
	err = s.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM vec_captures v WHERE v.model = ? AND `+vecCaptureEligibleSQL, model).Scan(&have)
	return
}

// captureGapCount counts eligible projected captures without a row of the
// model.
func (s *Store) captureGapCount(ctx context.Context, model string) (int64, error) {
	vectorWholeTableQueries.Add(1)
	var n int64
	err := s.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM proj_capture_index v
		WHERE `+vecCaptureEligibleSQL+` AND NOT EXISTS (SELECT 1 FROM vec_captures x WHERE x.capture_id = v.capture_id AND x.model = ?)`, model).Scan(&n)
	return n, err
}

type captureGapRow struct {
	ID    string
	Owner string
	Eff   time.Time
}

// captureGapBatchHook is a test seam: it runs before each gap statement of the
// capture walk, so a test can count them. Set it only while no pass runs.
var captureGapBatchHook func()

// captureGapBatch is the next batch of eligible projected captures without a
// row of the model, newest first, after the keyset cursor (eff, id).
func (s *Store) captureGapBatch(ctx context.Context, model string, afterEff *time.Time, afterID string, n int) ([]captureGapRow, error) {
	vectorWholeTableQueries.Add(1)
	if captureGapBatchHook != nil {
		captureGapBatchHook()
	}
	q := `SELECT v.capture_id, v.owner_id, COALESCE(v.published_at, v.fetched_at, v.updated_at) AS eff FROM proj_capture_index v
		WHERE ` + vecCaptureEligibleSQL + ` AND NOT EXISTS (SELECT 1 FROM vec_captures x WHERE x.capture_id = v.capture_id AND x.model = ?)`
	args := []any{model}
	if afterEff != nil {
		q += ` AND (COALESCE(v.published_at, v.fetched_at, v.updated_at), v.capture_id) < (?::timestamptz, ?)`
		args = append(args, *afterEff, afterID)
	}
	q += ` ORDER BY eff DESC, v.capture_id DESC LIMIT ?`
	args = append(args, n)
	rows, err := s.corpus().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []captureGapRow
	for rows.Next() {
		var r captureGapRow
		if err := rows.Scan(&r.ID, &r.Owner, &r.Eff); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// embedInputs embeds inputs in batches. When a batch fails and the embedder
// still answers its health check, its inputs are tried one by one so one bad
// input does not hold the rest. What a failure of one input means is decided
// here, in one place, and by one rule:
//
//   - the server identifies the input as the problem (embedder.InputRefusedError:
//     the two "too large" messages): the vector is nil and refusals names the
//     reason; the caller may record it durably;
//   - a status error of any other kind (a 500, a 503, a 429, a 404) is the
//     server's state, not the input's: the error wraps errEmbedderDown, nothing
//     is recorded, the pass stops and backs off, the items are retried;
//   - a transport or decode failure of one input while the server is healthy:
//     the input is left out of this pass (failed names it) and the others go
//     on; it is tried again next pass.
//
// When the embedder cannot answer its health check the error wraps errEmbedderDown.
func (s *Service) embedInputs(ctx context.Context, shape string, inputs []string) (out [][]float32, refusals map[int]string, failed map[int]error, err error) {
	emb := s.store.embedder()
	if emb == nil {
		return nil, nil, nil, fmt.Errorf("%w: no embedder configured", errEmbedderDown)
	}
	out = make([][]float32, len(inputs))
	for start := 0; start < len(inputs); start += vectorBatch {
		end := min(start+vectorBatch, len(inputs))
		done := s.store.retrievalTimer(retrievalVectorEmbed)
		vecs, err := emb.Embed(ctx, inputs[start:end])
		done(shape, end-start, err)
		if err == nil {
			copy(out[start:end], vecs)
			continue
		}
		if ctx.Err() != nil {
			return nil, nil, nil, ctx.Err()
		}
		if herr := emb.Health(ctx); herr != nil {
			return nil, nil, nil, fmt.Errorf("%w: %v", errEmbedderDown, herr)
		}
		for i := start; i < end; i++ {
			one, err := emb.Embed(ctx, inputs[i:i+1])
			if err == nil {
				out[i] = one[0]
				continue
			}
			if ctx.Err() != nil {
				return nil, nil, nil, ctx.Err()
			}
			if herr := emb.Health(ctx); herr != nil {
				return nil, nil, nil, fmt.Errorf("%w: %v", errEmbedderDown, herr)
			}
			switch classifyEmbedOutcome(err) {
			case effectRefused:
				log.Printf("corpusd vectors embed refused shape=%s err=%v", shape, err)
				if refusals == nil {
					refusals = map[int]string{}
				}
				refusals[i] = clipRunes(strings.TrimSpace(err.Error()), 200)
			case effectDown:
				return nil, nil, nil, fmt.Errorf("%w: the server answered %v", errEmbedderDown, err)
			default:
				if failed == nil {
					failed = map[int]error{}
				}
				failed[i] = err
			}
		}
	}
	return out, refusals, failed, nil
}

// RunVectorPass derives the gap and embeds it: stories, then captures, then
// the sweeps that remove vectors whose text is no longer public. It returns
// when the gap it found is closed, the context ends, or the embedder fails.
func (s *Service) RunVectorPass(ctx context.Context) (VectorPassResult, error) {
	return s.runVectorPass(ctx, vectorInboxCheckEvery, vectorIdleInterval)
}

// runVectorPass is RunVectorPass with the two timings it needs given, so the
// worker passes the values it read once at start. Between embed batches, at most
// once per checkEvery, the pass asks whether the spool inbox holds a file the
// next pass would take (vectorInboxHasFileForPass; a waiter tried less than
// idleInterval ago does not count, which bounds the passes a permanently waiting
// file can cause to one per idleInterval) and, if so, returns early with
// Yielded set, so a file placed during a long pass is not left waiting for it.
func (s *Service) runVectorPass(ctx context.Context, checkEvery, idleInterval time.Duration) (VectorPassResult, error) {
	var res VectorPassResult
	if s == nil || s.store == nil {
		return res, errors.New("vectors: nil service")
	}
	if err := s.store.ensureVectorTables(ctx); err != nil && (!s.store.VectorsReady() || s.store.vec.unsupportedReason() != "") {
		return res, err
	}
	// The sweep comes first and needs no embedder: a vector whose text is no
	// longer public is gone before anything else runs, whether or not the
	// embedder is up.
	removed, err := s.store.sweepVectors(ctx)
	res.Removed += removed
	if err != nil {
		return res, err
	}
	emb := s.store.embedder()
	if emb == nil {
		return res, fmt.Errorf("%w: no embedder configured", errEmbedderDown)
	}
	if err := emb.Health(ctx); err != nil {
		return res, fmt.Errorf("%w: %v", errEmbedderDown, err)
	}
	// A file waiting in the spool inbox is imported before the gap work, so the
	// gap it leaves is what the worker then fills.
	taken, ierr := s.ProcessVectorInbox(ctx)
	if taken > 0 {
		s.vecIdle = vecIdleMark{}
		res.Imports += taken
	}
	if ierr != nil {
		if errors.Is(ierr, errEmbedderDown) {
			return res, ierr
		}
		log.Printf("corpusd vectors inbox: %v", ierr)
	}
	// The pass derives the gap itself and so covers every capture the write
	// path named so far: the set is emptied, and a capture written from here
	// on is named again.
	s.store.takePendingCaptures()
	model := embedder.ModelHash
	started := time.Now()
	lastLog := started

	// Stories, newest first. An idle pass costs one row and two counts: when
	// the newest publication and the counts of publications and versions are
	// what the last complete walk saw, the walk (a publication's edge read each)
	// is not repeated. The mark is a hint derived from the tables and held in
	// this process only; a restart walks once.
	mark, err := s.storyMark(ctx)
	if err != nil {
		return res, fmt.Errorf("vectors: story mark: %w", err)
	}
	refusedStories, refusedCaptures, err := s.store.vectorRefusedCounts(ctx, model)
	if err != nil {
		return res, err
	}
	var eligible, gap []storyVersionRef
	walk := !(s.vecIdle.valid && s.vecIdle.mark == mark)
	storyWant, storyHave := s.vecIdle.want, s.vecIdle.have
	if walk {
		eligible, err = s.eligibleStoryVersions(ctx)
		if err != nil {
			return res, fmt.Errorf("vectors: list stories: %w", err)
		}
		have, err := s.store.storyVersionsWithVectors(ctx, model)
		if err != nil {
			return res, err
		}
		okSQL, err := s.store.sqlEligibleStoryVersions(ctx, eligible)
		if err != nil {
			return res, err
		}
		var kept []storyVersionRef
		for _, ref := range eligible {
			if okSQL[ref.VersionID] {
				kept = append(kept, ref)
			}
		}
		eligible = kept
		for _, ref := range eligible {
			if !have[ref.VersionID] {
				gap = append(gap, ref)
			}
		}
		storyWant = int64(len(eligible))
		storyHave = storyWant - int64(len(gap))
	}
	res.StoryGap = int64(len(gap))
	captureGap, err := s.store.captureGapCount(ctx, model)
	if err != nil {
		return res, err
	}
	res.CaptureGap = captureGap
	s.store.vec.remaining.Store(res.StoryGap + res.CaptureGap)
	if res.StoryGap+res.CaptureGap > 0 {
		log.Printf("corpusd vectors pass start stories=%d captures=%d remaining=%d", res.StoryGap, res.CaptureGap, res.StoryGap+res.CaptureGap)
	}
	var embeddedTotal atomic.Int64
	progress := func(kind string) {
		if time.Since(lastLog) < vectorProgressEvery {
			return
		}
		lastLog = time.Now()
		el := time.Since(started).Seconds()
		rate := 0.0
		if el > 0 {
			rate = float64(embeddedTotal.Load()) / el
		}
		log.Printf("corpusd vectors embed kind=%s remaining=%d rate=%.2f/s elapsed=%.0fs", kind, s.store.vec.remaining.Load(), rate, el)
	}
	lastInboxCheck := time.Now()
	yieldToInbox := func() bool {
		if time.Since(lastInboxCheck) < checkEvery {
			return false
		}
		lastInboxCheck = time.Now()
		if !s.vectorInboxHasFileForPass(idleInterval) {
			return false
		}
		s.vecIdle = vecIdleMark{}
		res.Yielded = true
		log.Printf("corpusd vectors pass yields to the inbox remaining=%d", s.store.vec.remaining.Load())
		return true
	}
	og := s.ogStore()
	if walk {
		logEmbedGap("stories", started, 0, storyHave, storyWant, refusedStories)
	}
	batchStart, batchRows, batchUnits := time.Now(), 0, 0
	storyErrs := 0
	for _, ref := range gap {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		n, nref, err := s.embedStoryVersion(ctx, og, ref)
		s.store.vec.remaining.Add(-1)
		switch {
		case errors.Is(err, errEmbedderDown) || ctx.Err() != nil:
			if ctx.Err() != nil {
				return res, ctx.Err()
			}
			return res, err
		case err != nil:
			res.Skipped++
			storyErrs++
			storyWant--
			log.Printf("corpusd vectors story version=%s skipped err=%v", ref.VersionID, err)
		case n+nref > 0:
			res.StoryVersions++
			res.StoryPassages += n
			res.Refused += nref
			refusedStories += int64(nref)
			storyHave++
			batchRows += n
			embeddedTotal.Add(int64(n))
		default:
			res.Skipped++ // not public, or no passages
			storyWant--
		}
		if batchUnits++; batchUnits >= vectorBatch {
			logEmbedGap("stories", batchStart, batchRows, storyHave, storyWant, refusedStories)
			batchStart, batchRows, batchUnits = time.Now(), 0, 0
		}
		progress("story")
		if yieldToInbox() {
			return res, nil
		}
	}
	if batchUnits > 0 || len(gap) > 0 {
		logEmbedGap("stories", batchStart, batchRows, storyHave, storyWant, refusedStories)
	}
	if walk && storyErrs == 0 {
		// The held count is read after the writes of this walk; the rest of
		// the mark before it, so a publication that arrived meanwhile shows.
		if err := s.store.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM vec_story_passages WHERE model = ?`, model).Scan(&mark.held); err != nil {
			return res, err
		}
		s.vecIdle = vecIdleMark{valid: true, mark: mark, want: storyWant, have: storyHave}
	}

	// Captures, newest first. The keyset cursor passes over a capture that
	// cannot have a vector (owner-private, no title and no text) once.
	capWant, capHave, err := s.store.captureCounts(ctx, model)
	if err != nil {
		return res, err
	}
	if res.CaptureGap > 0 || walk {
		logEmbedGap("captures", started, 0, capHave, capWant, refusedCaptures)
	}
	var afterEff *time.Time
	afterID := ""
	for {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		page, err := s.store.captureGapBatch(ctx, model, afterEff, afterID, vectorGapPage)
		if err != nil {
			return res, err
		}
		if len(page) == 0 {
			break
		}
		last := page[len(page)-1]
		afterEff, afterID = &last.Eff, last.ID
		// The page is a snapshot. embedCaptureBatch re-checks each row's
		// eligibility (sqlEligibleCaptures) just before embedding and skips one
		// that is no longer eligible, and writeCaptureVectors upserts by capture
		// id, so a row that got a vector meanwhile is only overwritten with the
		// same model's row; a stale page cannot write a vector for an ineligible
		// capture.
		for start := 0; start < len(page); start += vectorBatch {
			if err := ctx.Err(); err != nil {
				return res, err
			}
			rows := page[start:min(start+vectorBatch, len(page))]
			batchAt := time.Now()
			n, skipped, nref, err := s.embedCaptureBatch(ctx, rows)
			s.store.vec.remaining.Add(-int64(len(rows)))
			res.Captures += n
			res.Skipped += skipped
			res.Refused += nref
			refusedCaptures += int64(nref)
			capHave += int64(n + nref)
			capWant -= int64(skipped)
			embeddedTotal.Add(int64(n))
			logEmbedGap("captures", batchAt, n, capHave, capWant, refusedCaptures)
			if err != nil {
				return res, err
			}
			progress("capture")
			if yieldToInbox() {
				return res, nil
			}
		}
		if len(page) < vectorGapPage {
			break // a short page is the end of the gap
		}
	}

	s.store.vec.remaining.Store(0)
	if res.Worked() || res.Skipped > 0 {
		el := time.Since(started).Seconds()
		rate := 0.0
		if el > 0 {
			rate = float64(res.StoryPassages+res.Captures) / el
		}
		log.Printf("corpusd vectors pass done story_versions=%d story_passages=%d captures=%d skipped=%d removed=%d remaining=0 rate=%.2f/s elapsed=%.0fs",
			res.StoryVersions, res.StoryPassages, res.Captures, res.Skipped, res.Removed, rate, el)
	}
	return res, nil
}

// embedStoryVersion embeds every passage of one published version and writes
// them in one transaction, replacing any rows the version had (another model's).
// A passage the embedder refuses is written as a row without a vector and with
// the reason, so the version is whole in the table, is not in the gap, and the
// model is not asked again until the model changes; a search finds the
// version's other passages and never that one. It returns the passages
// embedded, 0 for a version that must not have vectors.
func (s *Service) embedStoryVersion(ctx context.Context, og *ObjectGraphStore, ref storyVersionRef) (embedded, refused int, err error) {
	vt, ok, err := s.loadStoryVersionText(ctx, og, ref.StoryID, ref.VersionID)
	if err != nil || !ok || len(vt.Passages) == 0 {
		return 0, 0, err
	}
	inputs := make([]string, len(vt.Passages))
	for i, p := range vt.Passages {
		inputs[i] = embedder.DocumentInput(vt.Title, p)
	}
	vecs, refusals, failed, err := s.embedInputs(ctx, "story", inputs)
	if err != nil {
		return 0, 0, err
	}
	if len(failed) > 0 {
		for i, ferr := range failed {
			return 0, 0, fmt.Errorf("passage %d of the version could not be embedded now (tried again next pass): %w", i, ferr)
		}
	}
	rows := make([]storyVectorRow, len(vecs))
	for i := range vecs {
		rows[i] = storyVectorRow{StoryID: ref.StoryID, VersionID: ref.VersionID, Ordinal: i, TextSHA: embedder.TextSHA(vt.Passages[i]), Day: vt.Day, Vec: vecs[i], Refused: refusals[i]}
		if vecs[i] != nil {
			embedded++
		} else {
			refused++
		}
	}
	if err := s.store.writeStoryVersionVectors(ctx, ref.StoryID, ref.VersionID, rows); err != nil {
		return 0, 0, err
	}
	return embedded, refused, nil
}

type storyVectorRow struct {
	StoryID, VersionID string
	Ordinal            int
	TextSHA            string
	Day                time.Time
	// Vec is nil for a passage the embedder refused; Refused says why.
	Vec     []float32
	Refused string
}

// vecParam is a vector as a statement parameter: its text form, or an empty
// string (NULLIF turns it into NULL) for a refused passage.
func vecParam(v []float32) string {
	if v == nil {
		return ""
	}
	return vecLiteral(v)
}

// writeStoryVersionVectors replaces a version's rows with rows, whole. Two
// writers of one version (the worker and an import, or two corpusd processes
// at a deploy overlap) do not abort each other: the second's inserts update
// the first's rows.
func (s *Store) writeStoryVersionVectors(ctx context.Context, storyID, versionID string, rows []storyVectorRow) (err error) {
	done := s.retrievalTimer(retrievalVectorWrite)
	defer func() { done("story", len(rows), err) }()
	tx, err := s.corpus().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// One writer of a version at a time: concurrent delete-and-insert of the
	// same rows can deadlock on lock order (seen with eight writers). The lock
	// ends with the transaction.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(?, 7330212470813))`, versionID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM vec_story_passages WHERE version_id = ?`, versionID); err != nil {
		return err
	}
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx, `INSERT INTO vec_story_passages (story_id, version_id, ordinal, text_sha, day, model, embedding, refused, embedded_on)
			VALUES (?, ?, ?, ?, ?::date, ?, NULLIF(?, '')::halfvec(256), ?, (now() AT TIME ZONE 'UTC')::date)
			ON CONFLICT (story_id, version_id, ordinal) DO UPDATE SET text_sha = EXCLUDED.text_sha, day = EXCLUDED.day, model = EXCLUDED.model,
				embedding = EXCLUDED.embedding, refused = EXCLUDED.refused, embedded_on = EXCLUDED.embedded_on`,
			storyID, versionID, r.Ordinal, r.TextSHA, r.Day.UTC().Format("2006-01-02"), embedder.ModelHash, vecParam(r.Vec), r.Refused); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// embedCaptureBatch embeds the captures of rows that may have a vector and
// writes them. skipped counts the ones that may not or could not.
func (s *Service) embedCaptureBatch(ctx context.Context, rows []captureGapRow) (written, skipped, refused int, err error) {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	objs, err := s.store.captureObjectsByIDAny(ctx, ids)
	if err != nil {
		return 0, 0, 0, err
	}
	type item struct {
		obj           objectgraph.Object
		lead, title   string
		input, textID string
	}
	var batchObjs []objectgraph.Object
	for _, r := range rows {
		if obj, ok := objs[r.ID]; ok {
			batchObjs = append(batchObjs, obj)
		}
	}
	okSQL, err := s.store.sqlEligibleCaptures(ctx, batchObjs)
	if err != nil {
		return 0, 0, 0, err
	}
	var items []item
	// Captures the predicate keeps but that have nothing to embed are recorded
	// as refused, so they leave the gap (and the remaining count) for good.
	var norow []captureVectorRow
	record := func(obj objectgraph.Object, why string) {
		norow = append(norow, captureVectorRow{ID: obj.CanonicalID, Owner: obj.OwnerID, Computer: obj.ComputerID, TextSHA: embedder.TextSHA(""), Day: captureDay(obj), Refused: why})
	}
	for _, r := range rows {
		obj, ok := objs[r.ID]
		if !ok || !okSQL[r.ID] {
			skipped++
			continue
		}
		title, hash, ok := captureVectorSource(obj)
		if !ok {
			// The predicate keeps it and the Go reader cannot read its metadata:
			// unreadable is not an identified refusal, so it is skipped and read
			// again next pass.
			skipped++
			log.Printf("corpusd vectors capture=%s skipped: its metadata cannot be read now", r.ID)
			continue
		}
		lead, err := captureLeadPassage(s.captureBodies, hash)
		if err != nil {
			skipped++
			log.Printf("corpusd vectors capture=%s skipped: read text: %v", r.ID, err)
			continue
		}
		if title == "" && strings.TrimSpace(lead) == "" {
			record(obj, "empty: no title and no text")
			continue
		}
		items = append(items, item{obj: obj, lead: lead, title: title, input: embedder.DocumentInput(title, lead), textID: embedder.TextSHA(lead)})
	}
	if len(items) == 0 {
		if err := s.store.writeCaptureVectors(ctx, norow); err != nil {
			return 0, skipped, 0, err
		}
		return 0, skipped, len(norow), nil
	}
	inputs := make([]string, len(items))
	for i, it := range items {
		inputs[i] = it.input
	}
	vecs, refusals, failed, err := s.embedInputs(ctx, "capture", inputs)
	if err != nil {
		return 0, skipped, 0, err
	}
	out := norow
	refused = len(norow)
	embedded := 0
	for i, it := range items {
		if ferr, bad := failed[i]; bad {
			// Left out of this pass, tried again next: one line per capture per
			// pass, and passes that do no other work are 5 minutes apart.
			skipped++
			log.Printf("corpusd vectors capture=%s could not be embedded now (tried again next pass): %v", it.obj.CanonicalID, ferr)
			continue
		}
		out = append(out, captureVectorRow{ID: it.obj.CanonicalID, Owner: it.obj.OwnerID, Computer: it.obj.ComputerID, TextSHA: it.textID, Day: captureDay(it.obj), Vec: vecs[i], Refused: refusals[i]})
		if vecs[i] != nil {
			embedded++
		} else {
			refused++
		}
	}
	if err := s.store.writeCaptureVectors(ctx, out); err != nil {
		return 0, skipped, 0, err
	}
	return embedded, skipped, refused, nil
}

type captureVectorRow struct {
	ID, Owner, Computer, TextSHA string
	Day                          time.Time
	Vec                          []float32
	Refused                      string
}

func (s *Store) writeCaptureVectors(ctx context.Context, rows []captureVectorRow) (err error) {
	if len(rows) == 0 {
		return nil
	}
	done := s.retrievalTimer(retrievalVectorWrite)
	defer func() { done("capture", len(rows), err) }()
	// Rows in key order, so two writers of overlapping batches lock in the
	// same order.
	rows = append([]captureVectorRow(nil), rows...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	tx, err := s.corpus().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx, `INSERT INTO vec_captures (capture_id, owner_id, computer_id, text_sha, day, model, embedding, refused, embedded_on)
			VALUES (?, ?, ?, ?, ?::date, ?, NULLIF(?, '')::halfvec(256), ?, (now() AT TIME ZONE 'UTC')::date)
			ON CONFLICT (capture_id) DO UPDATE SET owner_id = EXCLUDED.owner_id, computer_id = EXCLUDED.computer_id, text_sha = EXCLUDED.text_sha,
				day = EXCLUDED.day, model = EXCLUDED.model, embedding = EXCLUDED.embedding, refused = EXCLUDED.refused, embedded_on = EXCLUDED.embedded_on`,
			r.ID, r.Owner, r.Computer, r.TextSHA, r.Day.UTC().Format("2006-01-02"), embedder.ModelHash, vecParam(r.Vec), r.Refused); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// captureObjectsByIDAny reads capture objects by id whatever their scope (the
// worker has no request to scope by; each object carries its own).
func (s *Store) captureObjectsByIDAny(ctx context.Context, ids []string) (map[string]objectgraph.Object, error) {
	out := map[string]objectgraph.Object{}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	args = append(args, string(objectgraph.WebCaptureObjectKind))
	rows, err := s.corpus().QueryContext(ctx, `SELECT canonical_id, owner_id, computer_id, metadata, tombstone, superseded_by, created_at, updated_at
		FROM og_objects WHERE canonical_id IN (?`+strings.Repeat(",?", len(ids)-1)+`) AND object_kind = ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		obj := objectgraph.Object{ObjectKind: objectgraph.WebCaptureObjectKind}
		var meta string
		var superseded sql.NullString
		if err := rows.Scan(&obj.CanonicalID, &obj.OwnerID, &obj.ComputerID, &meta, &obj.Tombstone, &superseded, &obj.CreatedAt, &obj.UpdatedAt); err != nil {
			return nil, err
		}
		obj.Metadata = json.RawMessage(meta)
		obj.SupersededBy = superseded.String
		out[obj.CanonicalID] = obj
	}
	return out, rows.Err()
}

// ---- eligibility and sweeps ----

// The predicates below are the one statement of which vectors may be returned
// and which must be removed. A search filters by them in the same statement
// that reads the index, so removal latency cannot leak a private capture's id
// or a withdrawn story's passages; the sweep deletes by their negation, at the
// start of every pass and without the embedder. A capture's vector is
// eligible while its object is not tombstoned or superseded, belongs to the
// row's owner and computer, and carries no owner-private label (the full-text
// index's exclusion, taken conservatively: the label's prefix anywhere in the
// metadata text hides the capture). A story passage's is eligible while its
// version and publication are not tombstoned and the publication is published
// with a route, and no policy object of the version fails the read route's test
// (service_publication_read.go:465-481 with publicationAccessPolicyAllowsPublicRoute:
// access_policy_json absent, empty, malformed or not public is NOT public; a
// version with no policy object is the default, public). The CASE keeps
// malformed data from raising; TestVectorSQLPolicyAgreesWithTheReadRoute runs
// the same inputs through both so they cannot drift. The manifest's policy,
// the Mac export's source, is checked in Go when vectors are written.
//
// The capture predicate ends in OFFSET 0, which changes no result: it is
// PostgreSQL's fence against pulling an EXISTS up into a join. Pulled up, the
// planner drove the worker's gap count on the paper from og_objects (it
// estimated 2 eligible web captures, there were 221,335) into an index range
// of proj_capture_index holding every capture of the computer, a quadratic
// nested loop that ran 25 minutes without ending (2026-11-19). Fenced, the
// predicate is one primary-key read of og_objects per driving row: the same
// count took 2.3 s there. TestCaptureEligibilityIsProbedPerRowNotJoined holds
// the shape. The cost that stays: a count or a sweep over a computer's
// captures is one probe per capture, every pass.
var (
	vecCaptureEligibleSQL = `EXISTS (SELECT 1 FROM og_objects o WHERE o.canonical_id = v.capture_id AND o.object_kind = '` + string(objectgraph.WebCaptureObjectKind) + `'
		AND NOT o.tombstone AND COALESCE(o.superseded_by, '') = '' AND o.owner_id = v.owner_id AND o.computer_id = v.computer_id AND v.owner_id <> '' AND v.computer_id <> ''
		AND position('` + types.LabelOwnerPrivatePrefix + `' in o.metadata) = 0 OFFSET 0)`
	vecStoryEligibleSQL = `(EXISTS (SELECT 1 FROM og_objects ver WHERE ver.canonical_id = v.version_id AND NOT ver.tombstone)
		AND EXISTS (SELECT 1 FROM og_objects pub WHERE pub.canonical_id = v.story_id AND NOT pub.tombstone
			AND pub.metadata::jsonb->>'state' = 'published' AND btrim(COALESCE(pub.metadata::jsonb->>'route_path', '')) <> '')
		AND NOT EXISTS (SELECT 1 FROM og_edges pe JOIN og_objects po ON po.canonical_id = pe.to_id
			WHERE pe.from_id = v.version_id AND pe.kind = 'has_policy' AND NOT pe.tombstone
			AND CASE WHEN NOT pg_input_is_valid(po.metadata, 'jsonb') THEN true
				WHEN NOT pg_input_is_valid(COALESCE(po.metadata::jsonb->>'access_policy_json', ''), 'jsonb') THEN true
				WHEN COALESCE(po.metadata::jsonb->>'access_policy_json', '') = '' THEN true
				ELSE NOT (lower(btrim(COALESCE((po.metadata::jsonb->>'access_policy_json')::jsonb->>'visibility', ''))) = 'public'
					AND lower(btrim(COALESCE((po.metadata::jsonb->>'access_policy_json')::jsonb->>'route', ''))) IN ('', 'public')) END))`
)

// sqlEligibleCaptures returns which of objs satisfy vecCaptureEligibleSQL, the
// one predicate the sweep and the search use, so that the worker never writes
// a vector those would remove at once (a label the Go reader drops but the
// predicate keeps would otherwise be embedded and swept in turn).
func (s *Store) sqlEligibleCaptures(ctx context.Context, objs []objectgraph.Object) (map[string]bool, error) {
	out := map[string]bool{}
	for start := 0; start < len(objs); start += 200 {
		chunk := objs[start:min(start+200, len(objs))]
		args := make([]any, 0, 3*len(chunk))
		vals := make([]string, len(chunk))
		for i, o := range chunk {
			vals[i] = "(?::text, ?::text, ?::text)"
			args = append(args, o.CanonicalID, o.OwnerID, o.ComputerID)
		}
		rows, err := s.corpus().QueryContext(ctx, `SELECT v.capture_id FROM (VALUES `+strings.Join(vals, ",")+`) AS v(capture_id, owner_id, computer_id) WHERE `+vecCaptureEligibleSQL, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return nil, err
			}
			out[id] = true
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// sqlEligibleStoryVersions returns the version ids among refs that satisfy
// vecStoryEligibleSQL.
func (s *Store) sqlEligibleStoryVersions(ctx context.Context, refs []storyVersionRef) (map[string]bool, error) {
	out := map[string]bool{}
	for start := 0; start < len(refs); start += 200 {
		chunk := refs[start:min(start+200, len(refs))]
		args := make([]any, 0, 2*len(chunk))
		vals := make([]string, len(chunk))
		for i, r := range chunk {
			vals[i] = "(?::text, ?::text)"
			args = append(args, r.StoryID, r.VersionID)
		}
		rows, err := s.corpus().QueryContext(ctx, `SELECT v.version_id FROM (VALUES `+strings.Join(vals, ",")+`) AS v(story_id, version_id) WHERE `+vecStoryEligibleSQL, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return nil, err
			}
			out[id] = true
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// sweepVectors deletes the vectors whose text is no longer public, of both
// kinds, and returns the rows deleted.
func (s *Store) sweepVectors(ctx context.Context) (n int, err error) {
	done := s.retrievalTimer(retrievalVectorSweep)
	defer func() { done("both", n, err) }()
	removed := 0
	for _, q := range []struct{ table, pred string }{
		{vecStoryTable, vecStoryEligibleSQL}, {vecCaptureTable, vecCaptureEligibleSQL},
	} {
		res, err := s.corpus().ExecContext(ctx, `DELETE FROM `+q.table+` v WHERE NOT `+q.pred)
		if err != nil {
			return removed, fmt.Errorf("vectors: sweep %s: %w", q.table, err)
		}
		if n, err := res.RowsAffected(); err == nil {
			removed += int(n)
		}
	}
	if removed > 0 {
		log.Printf("corpusd vectors sweep removed=%d (text no longer public)", removed)
	}
	return removed, nil
}

// runCaptureIDStep embeds the captures the write path named since the last
// look: those that are eligible and have no vector of the model. It reads no
// count and walks no gap (a full pass does that on its own interval), takes
// the ids in one set whatever their number, and returns when they are done.
// A capture it cannot embed now (a transport failure of one input) is left
// for the full pass. An embedder that cannot answer returns an error wrapping
// errEmbedderDown; the caller leaves the retry to the full pass and its
// back-off.
func (s *Service) runCaptureIDStep(ctx context.Context) (embedded int, err error) {
	defer func() {
		if err != nil && ctx.Err() == nil {
			log.Printf("corpusd vectors ids step err=%v (the full pass takes the retry)", err)
		}
	}()
	if s == nil || s.store == nil || !s.store.VectorsReady() || s.store.embedder() == nil {
		return 0, nil
	}
	ids := s.store.takePendingCaptures()
	if len(ids) == 0 {
		return 0, nil
	}
	started := time.Now()
	have, err := s.store.captureIDsWithVector(ctx, ids, embedder.ModelHash)
	if err != nil {
		return 0, err
	}
	var rows []captureGapRow
	for _, id := range ids {
		if !have[id] {
			rows = append(rows, captureGapRow{ID: id})
		}
	}
	skipped, refused := 0, 0
	for start := 0; start < len(rows); start += vectorBatch {
		if err := ctx.Err(); err != nil {
			return embedded, err
		}
		n, sk, nref, err := s.embedCaptureBatch(ctx, rows[start:min(start+vectorBatch, len(rows))])
		embedded += n
		skipped += sk
		refused += nref
		if err != nil {
			return embedded, err
		}
	}
	s.store.vec.stepLog.add(len(ids), embedded, skipped, refused, time.Since(started))
	return embedded, nil
}

// captureIDsWithVector returns which of ids have a row of the model, by
// primary key.
func (s *Store) captureIDsWithVector(ctx context.Context, ids []string, model string) (map[string]bool, error) {
	out := map[string]bool{}
	for start := 0; start < len(ids); start += 500 {
		chunk := ids[start:min(start+500, len(ids))]
		args := []any{model}
		ph := make([]string, len(chunk))
		for i, id := range chunk {
			ph[i] = "?"
			args = append(args, id)
		}
		rows, err := s.corpus().QueryContext(ctx, `SELECT capture_id FROM vec_captures WHERE model = ? AND capture_id IN (`+strings.Join(ph, ",")+`)`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return nil, err
			}
			out[id] = true
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ---- the loop ----

// StartVectorWorker runs passes in the background: one a little after boot,
// then whenever one did work, when kicked, and every vectorIdleInterval. It
// returns at once. An embedder that cannot answer is logged once and retried
// with a doubling wait; stored vectors and searches are untouched.
func (s *Service) StartVectorWorker(ctx context.Context) {
	if s == nil || s.store == nil {
		return
	}
	s.store.vec.started.Do(func() {
		// These five timings are read once, here, before the goroutine exists:
		// the loop reads the locals, so a test that changes the four and puts
		// them back does not race with a worker that is still stopping. Keep
		// the capture outside the goroutine. (The batch sizes are still read
		// by a pass as package variables; a test that changes one must not
		// overlap a running worker.)
		startDelay, backoffStart, backoffMax, idle, checkEvery := vectorStartDelay, vectorBackoffStart, vectorBackoffMax, vectorIdleInterval, vectorInboxCheckEvery
		s.removeStaleVectorSpool()
		s.recoverVectorInbox()
		go func() {
			select {
			case <-ctx.Done():
				return
			case <-time.After(startDelay):
			}
			backoff := backoffStart
			down := false
			for {
				res, err := s.runVectorPass(ctx, checkEvery, idle)
				if ctx.Err() != nil {
					return
				}
				wait := idle
				backingOff := false
				switch {
				case errors.Is(err, errEmbedderDown):
					if !down {
						log.Printf("corpusd vectors embedder down, backing off (retry in %s, doubling to %s): %v", backoff, backoffMax, err)
						down = true
					}
					wait = backoff
					backingOff = true
					backoff = min(2*backoff, backoffMax)
				case err != nil:
					log.Printf("corpusd vectors pass err=%v (retry in %s)", err, backoff)
					wait = backoff
					backingOff = true
					backoff = min(2*backoff, backoffMax)
				default:
					if down {
						log.Printf("corpusd vectors embedder is back")
						down = false
					}
					backoff = backoffStart
					if res.Worked() || res.Yielded {
						wait = 0 // a yielded pass is followed at once by the one that takes the file
					}
				}
				if wait > 0 {
					timer := time.NewTimer(wait)
				waiting:
					for {
						select {
						case <-ctx.Done():
							timer.Stop()
							return
						case <-timer.C:
							break waiting
						case <-s.store.vec.kickChan():
							timer.Stop()
							break waiting
						case <-s.store.vec.wakeChan():
							// Captures were written. While backing off the ids
							// wait in their set (the full pass at the back-off's
							// end covers them) and the wake changes nothing.
							if backingOff {
								continue
							}
							if _, err := s.runCaptureIDStep(ctx); err != nil && ctx.Err() == nil {
								// Hand the retry to a full pass, which tells an
								// embedder that is down from a failure of one
								// input and owns the back-off.
								timer.Stop()
								break waiting
							}
						}
					}
				}
			}
		}()
	})
}
