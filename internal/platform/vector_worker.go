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
	// vectorBatch is how many inputs go in one embedder call, a size and not a
	// limit. The worker sends one batch at a time and waits for its answer, so
	// a search's query, which goes to the same unit, waits behind at most one
	// batch: four passages of about 300 tokens is one to three seconds on the
	// unit's four threads (350 tokens a second measured on this CPU, batches
	// of four no slower per token than one). The worker does not rest for
	// searches or space its calls: the unit's low CPU weight is what lets
	// guests win (CLAUDE.md, no limits on our own model calls).
	vectorBatch = 4
	// vectorStartDelay is how long after boot the first pass waits.
	vectorStartDelay = 30 * time.Second
	// vectorIdleInterval is how long a worker with nothing to do waits before
	// it looks again (a kick ends the wait).
	vectorIdleInterval = 5 * time.Minute
	// vectorProgressEvery is how often a pass logs progress.
	vectorProgressEvery = 30 * time.Second
	// vectorSweepEvery is how often the capture sweep (a vector whose capture
	// is gone, superseded or owner-private) runs.
	vectorSweepEvery = time.Hour
	// vectorBackoffStart and vectorBackoffMax bound the wait after a failed pass.
	vectorBackoffStart = 5 * time.Second
	vectorBackoffMax   = 5 * time.Minute
)

// logEmbedGap writes the gap reader's line (scripts/status-vectors): the
// vectors held, the rows that should have one, and what remains, for stories
// (versions: a version's passages are written whole) or captures. rows is
// what the batch embedded. Written at the start of a pass, at each batch
// boundary and at the end of a phase, never per row. want counts what the
// derivation could not yet tell cannot have a vector (an owner-private
// capture, a version that is not public), so remaining is an upper bound
// until the pass has passed over them; the end-of-phase line is exact.
func logEmbedGap(shape string, since time.Time, rows int, have, want int64) {
	remaining := max(want-have, 0)
	log.Printf("corpusd retrieval op=embed_gap shape=%s ms=%.2f rows=%d have=%d want=%d remaining=%d model=%s",
		shape, float64(time.Since(since).Microseconds())/1000, rows, have, want, remaining, embedder.ModelHash[:12])
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
	// Removed are vectors deleted because their text is no longer public.
	Removed int
	// StoryGap and CaptureGap are the counts without a vector at the start.
	StoryGap, CaptureGap int64
}

// Worked says the pass embedded or removed something.
func (r VectorPassResult) Worked() bool {
	return r.StoryVersions+r.Captures+r.Removed > 0
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
// publication. Whether a version's access policy is public is read when it is
// embedded.
func (s *Service) eligibleStoryVersions(ctx context.Context) ([]storyVersionRef, error) {
	og := s.ogStore()
	if og == nil {
		return nil, errors.New("object graph store unavailable")
	}
	var out []storyVersionRef
	seen := map[string]bool{}
	var after *frontCursor
	for {
		rows, err := s.frontPublicationBatch(ctx, after, 200)
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
			versions, err := publishedVersions(ctx, og, row.CanonicalID)
			if err != nil {
				return nil, err
			}
			for i := len(versions) - 1; i >= 0; i-- {
				out = append(out, storyVersionRef{StoryID: row.CanonicalID, VersionID: versions[i].CanonicalID, Revision: versions[i].Revision, Title: meta.Title})
			}
		}
		if len(rows) < 200 {
			return out, nil
		}
		last := rows[len(rows)-1]
		after = &frontCursor{UpdatedAt: last.UpdatedAt.Unix(), CanonicalID: last.CanonicalID}
	}
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

// captureCounts is the captures projected (want) and the vectors held of the
// model (have).
func (s *Store) captureCounts(ctx context.Context, model string) (want, have int64, err error) {
	if err = s.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM proj_capture_index`).Scan(&want); err != nil {
		return
	}
	err = s.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM vec_captures WHERE model = ?`, model).Scan(&have)
	return
}

// captureGapCount counts projected captures without a vector of the model.
// It includes captures that cannot have one (owner-private), so it is an
// upper bound of the work; a pass reports what it embedded.
func (s *Store) captureGapCount(ctx context.Context, model string) (int64, error) {
	var n int64
	err := s.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM proj_capture_index p
		WHERE NOT EXISTS (SELECT 1 FROM vec_captures v WHERE v.capture_id = p.capture_id AND v.model = ?)`, model).Scan(&n)
	return n, err
}

type captureGapRow struct {
	ID    string
	Owner string
	Eff   time.Time
}

// captureGapBatch is the next batch of projected captures without a vector,
// newest first, after the keyset cursor (eff, id).
func (s *Store) captureGapBatch(ctx context.Context, model string, afterEff *time.Time, afterID string, n int) ([]captureGapRow, error) {
	q := `SELECT p.capture_id, p.owner_id, COALESCE(p.published_at, p.fetched_at, p.updated_at) AS eff FROM proj_capture_index p
		WHERE NOT EXISTS (SELECT 1 FROM vec_captures v WHERE v.capture_id = p.capture_id AND v.model = ?)`
	args := []any{model}
	if afterEff != nil {
		q += ` AND (COALESCE(p.published_at, p.fetched_at, p.updated_at), p.capture_id) < (?::timestamptz, ?)`
		args = append(args, *afterEff, afterID)
	}
	q += ` ORDER BY eff DESC, p.capture_id DESC LIMIT ?`
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
// still answers, its inputs are tried one by one so one bad input does not
// hold the rest; the ones that fail come back nil. When the embedder cannot
// answer the error wraps errEmbedderDown.
func (s *Service) embedInputs(ctx context.Context, shape string, inputs []string) ([][]float32, error) {
	emb := s.store.embedder()
	if emb == nil {
		return nil, fmt.Errorf("%w: no embedder configured", errEmbedderDown)
	}
	out := make([][]float32, len(inputs))
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
			return nil, ctx.Err()
		}
		if herr := emb.Health(ctx); herr != nil {
			return nil, fmt.Errorf("%w: %v", errEmbedderDown, herr)
		}
		for i := start; i < end; i++ {
			one, err := emb.Embed(ctx, inputs[i:i+1])
			if err != nil {
				if herr := emb.Health(ctx); herr != nil {
					return nil, fmt.Errorf("%w: %v", errEmbedderDown, herr)
				}
				log.Printf("corpusd vectors embed refused shape=%s err=%v", shape, err)
				continue
			}
			out[i] = one[0]
		}
	}
	return out, nil
}

// RunVectorPass derives the gap and embeds it: stories, then captures, then
// the sweeps that remove vectors whose text is no longer public. It returns
// when the gap it found is closed, the context ends, or the embedder fails.
func (s *Service) RunVectorPass(ctx context.Context) (VectorPassResult, error) {
	var res VectorPassResult
	if s == nil || s.store == nil {
		return res, errors.New("vectors: nil service")
	}
	emb := s.store.embedder()
	if emb == nil {
		return res, fmt.Errorf("%w: no embedder configured", errEmbedderDown)
	}
	if err := emb.Health(ctx); err != nil {
		return res, fmt.Errorf("%w: %v", errEmbedderDown, err)
	}
	if err := s.store.ensureVectorTables(ctx); err != nil && !s.store.VectorsReady() {
		return res, err
	}
	model := embedder.ModelHash
	started := time.Now()
	lastLog := started

	// Stories, newest first.
	eligible, err := s.eligibleStoryVersions(ctx)
	if err != nil {
		return res, fmt.Errorf("vectors: list stories: %w", err)
	}
	have, err := s.store.storyVersionsWithVectors(ctx, model)
	if err != nil {
		return res, err
	}
	var gap []storyVersionRef
	eligibleSet := make(map[string]bool, len(eligible))
	for _, ref := range eligible {
		eligibleSet[ref.VersionID] = true
		if !have[ref.VersionID] {
			gap = append(gap, ref)
		}
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
	og := s.ogStore()
	storyWant := int64(len(eligible))
	storyHave := storyWant - res.StoryGap
	logEmbedGap("stories", started, 0, storyHave, storyWant)
	batchStart, batchRows, batchUnits := time.Now(), 0, 0
	for _, ref := range gap {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		n, err := s.embedStoryVersion(ctx, og, ref)
		s.store.vec.remaining.Add(-1)
		switch {
		case errors.Is(err, errEmbedderDown) || ctx.Err() != nil:
			if ctx.Err() != nil {
				return res, ctx.Err()
			}
			return res, err
		case err != nil:
			res.Skipped++
			storyWant--
			log.Printf("corpusd vectors story version=%s skipped err=%v", ref.VersionID, err)
		case n > 0:
			res.StoryVersions++
			res.StoryPassages += n
			storyHave++
			batchRows += n
			embeddedTotal.Add(int64(n))
		default:
			res.Skipped++ // not public, or no passages
			storyWant--
		}
		if batchUnits++; batchUnits >= vectorBatch {
			logEmbedGap("stories", batchStart, batchRows, storyHave, storyWant)
			batchStart, batchRows, batchUnits = time.Now(), 0, 0
		}
		progress("story")
	}
	if batchUnits > 0 || len(gap) > 0 {
		logEmbedGap("stories", batchStart, batchRows, storyHave, storyWant)
	}

	// Captures, newest first. The keyset cursor passes over a capture that
	// cannot have a vector (owner-private, no title and no text) once.
	capWant, capHave, err := s.store.captureCounts(ctx, model)
	if err != nil {
		return res, err
	}
	logEmbedGap("captures", started, 0, capHave, capWant)
	var afterEff *time.Time
	afterID := ""
	for {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		rows, err := s.store.captureGapBatch(ctx, model, afterEff, afterID, vectorBatch)
		if err != nil {
			return res, err
		}
		if len(rows) == 0 {
			break
		}
		last := rows[len(rows)-1]
		afterEff, afterID = &last.Eff, last.ID
		batchAt := time.Now()
		n, skipped, err := s.embedCaptureBatch(ctx, rows)
		s.store.vec.remaining.Add(-int64(len(rows)))
		res.Captures += n
		res.Skipped += skipped
		capHave += int64(n)
		capWant -= int64(skipped)
		embeddedTotal.Add(int64(n))
		logEmbedGap("captures", batchAt, n, capHave, capWant)
		if err != nil {
			return res, err
		}
		progress("capture")
	}

	// Sweeps: a vector must not outlive its text's being public.
	removed, err := s.sweepStoryVectors(ctx, eligibleSet)
	res.Removed += removed
	if err != nil {
		return res, err
	}
	if s.store.captureSweepDue() {
		removed, err := s.sweepCaptureVectors(ctx)
		res.Removed += removed
		if err != nil {
			return res, err
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
// It returns the passages written, 0 for a version that must not have vectors.
func (s *Service) embedStoryVersion(ctx context.Context, og *ObjectGraphStore, ref storyVersionRef) (int, error) {
	vt, ok, err := s.loadStoryVersionText(ctx, og, ref.StoryID, ref.VersionID)
	if err != nil || !ok || len(vt.Passages) == 0 {
		return 0, err
	}
	inputs := make([]string, len(vt.Passages))
	for i, p := range vt.Passages {
		inputs[i] = embedder.DocumentInput(vt.Title, p)
	}
	vecs, err := s.embedInputs(ctx, "story", inputs)
	if err != nil {
		return 0, err
	}
	for i, v := range vecs {
		if v == nil {
			return 0, fmt.Errorf("passage %d was refused by the embedder", i)
		}
	}
	rows := make([]storyVectorRow, len(vecs))
	for i := range vecs {
		rows[i] = storyVectorRow{StoryID: ref.StoryID, VersionID: ref.VersionID, Ordinal: i, TextSHA: embedder.TextSHA(vt.Passages[i]), Day: vt.Day, Vec: vecs[i]}
	}
	if err := s.store.writeStoryVersionVectors(ctx, ref.StoryID, ref.VersionID, rows); err != nil {
		return 0, err
	}
	return len(rows), nil
}

type storyVectorRow struct {
	StoryID, VersionID string
	Ordinal            int
	TextSHA            string
	Day                time.Time
	Vec                []float32
}

// writeStoryVersionVectors replaces a version's rows with rows, whole.
func (s *Store) writeStoryVersionVectors(ctx context.Context, storyID, versionID string, rows []storyVectorRow) (err error) {
	done := s.retrievalTimer(retrievalVectorWrite)
	defer func() { done("story", len(rows), err) }()
	tx, err := s.corpus().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM vec_story_passages WHERE version_id = ?`, versionID); err != nil {
		return err
	}
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx, `INSERT INTO vec_story_passages (story_id, version_id, ordinal, text_sha, day, model, embedding, embedded_on)
			VALUES (?, ?, ?, ?, ?::date, ?, ?::halfvec(256), (now() AT TIME ZONE 'UTC')::date)`,
			storyID, versionID, r.Ordinal, r.TextSHA, r.Day.UTC().Format("2006-01-02"), embedder.ModelHash, vecLiteral(r.Vec)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// embedCaptureBatch embeds the captures of rows that may have a vector and
// writes them. skipped counts the ones that may not or could not.
func (s *Service) embedCaptureBatch(ctx context.Context, rows []captureGapRow) (written, skipped int, err error) {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	objs, err := s.store.captureObjectsByIDAny(ctx, ids)
	if err != nil {
		return 0, 0, err
	}
	type item struct {
		obj           objectgraph.Object
		lead, title   string
		input, textID string
	}
	var items []item
	for _, r := range rows {
		obj, ok := objs[r.ID]
		if !ok {
			skipped++
			continue
		}
		title, hash, ok := captureVectorSource(obj)
		if !ok {
			skipped++
			continue
		}
		lead, err := captureLeadPassage(s.captureBodies, hash)
		if err != nil {
			skipped++
			log.Printf("corpusd vectors capture=%s skipped: read text: %v", r.ID, err)
			continue
		}
		if title == "" && strings.TrimSpace(lead) == "" {
			skipped++
			continue
		}
		items = append(items, item{obj: obj, lead: lead, title: title, input: embedder.DocumentInput(title, lead), textID: embedder.TextSHA(lead)})
	}
	if len(items) == 0 {
		return 0, skipped, nil
	}
	inputs := make([]string, len(items))
	for i, it := range items {
		inputs[i] = it.input
	}
	vecs, err := s.embedInputs(ctx, "capture", inputs)
	if err != nil {
		return 0, skipped, err
	}
	var out []captureVectorRow
	for i, it := range items {
		if vecs[i] == nil {
			skipped++
			continue
		}
		out = append(out, captureVectorRow{ID: it.obj.CanonicalID, Owner: it.obj.OwnerID, Computer: it.obj.ComputerID, TextSHA: it.textID, Day: captureDay(it.obj), Vec: vecs[i]})
	}
	if err := s.store.writeCaptureVectors(ctx, out); err != nil {
		return 0, skipped, err
	}
	return len(out), skipped, nil
}

type captureVectorRow struct {
	ID, Owner, Computer, TextSHA string
	Day                          time.Time
	Vec                          []float32
}

func (s *Store) writeCaptureVectors(ctx context.Context, rows []captureVectorRow) (err error) {
	if len(rows) == 0 {
		return nil
	}
	done := s.retrievalTimer(retrievalVectorWrite)
	defer func() { done("capture", len(rows), err) }()
	tx, err := s.corpus().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx, `INSERT INTO vec_captures (capture_id, owner_id, computer_id, text_sha, day, model, embedding, embedded_on)
			VALUES (?, ?, ?, ?, ?::date, ?, ?::halfvec(256), (now() AT TIME ZONE 'UTC')::date)
			ON CONFLICT (capture_id) DO UPDATE SET owner_id = EXCLUDED.owner_id, computer_id = EXCLUDED.computer_id, text_sha = EXCLUDED.text_sha,
				day = EXCLUDED.day, model = EXCLUDED.model, embedding = EXCLUDED.embedding, embedded_on = EXCLUDED.embedded_on`,
			r.ID, r.Owner, r.Computer, r.TextSHA, r.Day.UTC().Format("2006-01-02"), embedder.ModelHash, vecLiteral(r.Vec)); err != nil {
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
	rows, err := s.corpus().QueryContext(ctx, `SELECT canonical_id, owner_id, computer_id, metadata, tombstone, superseded_by, created_at, updated_at
		FROM og_objects WHERE canonical_id IN (?`+strings.Repeat(",?", len(ids)-1)+`)`, args...)
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

// ---- sweeps ----

// sweepStoryVectors removes the vectors of versions that are no longer
// eligible: a withdrawn or deleted story.
func (s *Service) sweepStoryVectors(ctx context.Context, eligible map[string]bool) (int, error) {
	rows, err := s.store.corpus().QueryContext(ctx, `SELECT DISTINCT version_id FROM vec_story_passages`)
	if err != nil {
		return 0, err
	}
	var gone []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, err
		}
		if !eligible[id] {
			gone = append(gone, id)
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return 0, err
	}
	for _, id := range gone {
		if _, err := s.store.corpus().ExecContext(ctx, `DELETE FROM vec_story_passages WHERE version_id = ?`, id); err != nil {
			return 0, err
		}
	}
	return len(gone), nil
}

func (s *Store) captureSweepDue() bool {
	s.vec.sweepMu.Lock()
	defer s.vec.sweepMu.Unlock()
	return time.Since(s.vec.lastCaptureSweep) >= vectorSweepEvery
}

// sweepCaptureVectors removes the vectors of captures that are gone,
// tombstoned, superseded or owner-private, paging through the table.
func (s *Service) sweepCaptureVectors(ctx context.Context) (int, error) {
	removed := 0
	after := ""
	for {
		rows, err := s.store.corpus().QueryContext(ctx, `SELECT capture_id FROM vec_captures WHERE capture_id > ? ORDER BY capture_id LIMIT 400`, after)
		if err != nil {
			return removed, err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return removed, err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return removed, err
		}
		if len(ids) == 0 {
			break
		}
		after = ids[len(ids)-1]
		objs, err := s.store.captureObjectsByIDAny(ctx, ids)
		if err != nil {
			return removed, err
		}
		for _, id := range ids {
			obj, ok := objs[id]
			if ok {
				if _, _, live := captureVectorSource(obj); live {
					continue
				}
			}
			if _, err := s.store.corpus().ExecContext(ctx, `DELETE FROM vec_captures WHERE capture_id = ?`, id); err != nil {
				return removed, err
			}
			removed++
		}
		if len(ids) < 400 {
			break
		}
	}
	s.store.vec.sweepMu.Lock()
	s.store.vec.lastCaptureSweep = time.Now()
	s.store.vec.sweepMu.Unlock()
	return removed, nil
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
		go func() {
			select {
			case <-ctx.Done():
				return
			case <-time.After(vectorStartDelay):
			}
			backoff := vectorBackoffStart
			down := false
			for {
				res, err := s.RunVectorPass(ctx)
				if ctx.Err() != nil {
					return
				}
				wait := vectorIdleInterval
				switch {
				case errors.Is(err, errEmbedderDown):
					if !down {
						log.Printf("corpusd vectors embedder down, backing off (retry in %s, doubling to %s): %v", backoff, vectorBackoffMax, err)
						down = true
					}
					wait = backoff
					backoff = min(2*backoff, vectorBackoffMax)
				case err != nil:
					log.Printf("corpusd vectors pass err=%v (retry in %s)", err, backoff)
					wait = backoff
					backoff = min(2*backoff, vectorBackoffMax)
				default:
					if down {
						log.Printf("corpusd vectors embedder is back")
						down = false
					}
					backoff = vectorBackoffStart
					if res.Worked() {
						wait = 0
					}
				}
				if wait > 0 {
					select {
					case <-ctx.Done():
						return
					case <-time.After(wait):
					case <-s.store.vec.kickChan():
					}
				}
			}
		}()
	})
}
