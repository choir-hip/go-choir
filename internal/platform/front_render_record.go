package platform

// The front render record (R0, docs/definitions/choir-records-and-measurement-
// 2026-11-23.md): the stage "page_render" of the decision record. The front
// list is ranked by the page (web and phone) and by the daily digest from the
// list this service serves; the scorer keeps only each story's latest
// importance, so the order a reader saw could not be rebuilt afterwards.
//
// What a row is, and is not. It is a record of each change of the canonical
// first page: the first page of the whole list (no cursor, no section) as
// this service serves it, ranked by frontrank.RankScored without any
// reader's per-device section habit. A row is written when a first-page
// request finds that page's content different from the last one recorded. It is NOT a record of every render: cursor and
// section pages write nothing, a read served from a client's or a browser's
// cache never reaches this service, and what a reader's own habits did to the
// order they saw is not in it. The phone ranks its own pool with its own
// formula (mobile/AutoradioLab/paper-model.ts); a row says what the web rank
// and the digest would show for that list, not what the phone drew.
//
// Only the canonical first page is recorded: the one asked for with limit=60,
// which is what the web page's first load asks (FRONT_FIRST_PAGE_SIZE in
// frontend/src/lib/autopaper.ts), what the daily digest asks (digestFrontLimit
// in internal/maild/digest.go) and what the phone's radio asks (FRONT_LIMIT in
// mobile/AutoradioLab/paper-model.ts). A request for any other size writes
// nothing and spends nothing, so a stranger sweeping ?limit= cannot touch the
// record. The phone's list pool (limit=100, LIST_LIMIT) is not recorded: its
// page slices 60 and ranks them with its own formula, so a row for it would be
// the web rank over a count the phone does not show.
//
// A render never waits on or fails because of the row. The page is hashed
// inline, the dedupe is updated under a short lock, and the write is handed
// to one background writer over a bounded queue with its own timeout,
// detached from the request; a full queue or a failed write is logged and the
// row is lost (a failed write is retried by the next request that finds the
// same change unrecorded), and a panic in the writer is recovered and logged.
// The dedupe is the hash of the shown page, one hash held in memory: the first
// request after a restart writes one row even if nothing changed.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/frontrank"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	ogKindPageRender = objectgraph.ObjectKind("choir.page_render_record")
	frontRenderOwner = "corpusd"
	frontRenderRule  = "frontrank.RankScored, the web page's and the daily digest's rank (importance x depth x 24-hour half-life, or recency when under half the list is scored; demoted stories last under the importance rank; the first story that can lead moved to the front), without the reader's per-device section habit and not the phone's own ranking; the canonical first page (limit 60) of the whole list as served"
)

const (
	// frontRenderCanonicalLimit is the one page size recorded (see the header).
	frontRenderCanonicalLimit = 60
	// frontRenderQueue is how many rows wait for the writer; more are dropped.
	frontRenderQueue = 16
	// frontRenderWriteTimeout bounds one write, detached from any request.
	frontRenderWriteTimeout = 30 * time.Second
)

type frontRenderItem struct {
	obj  objectgraph.Object
	hash string
}

// frontRenderState is the hash of the last canonical page recorded and the
// one background writer.
type frontRenderState struct {
	mu    sync.Mutex
	last  string
	queue chan frontRenderItem
	start sync.Once
	wg    sync.WaitGroup
	// put writes one row; nil means the object graph store. Tests set it.
	put func(context.Context, objectgraph.Object) error
}

// FlushFrontRenders waits until every queued row has been written or has
// failed. A test seam; the route never calls it.
func (s *Service) FlushFrontRenders() {
	if s != nil {
		s.frontRender.wg.Wait()
	}
}

func (s *Service) frontRenderWriter(og *ObjectGraphStore) {
	f := &s.frontRender
	f.start.Do(func() {
		f.queue = make(chan frontRenderItem, frontRenderQueue)
		go func() {
			for it := range f.queue {
				f.writeOne(og, it)
			}
		}()
	})
}

// writeOne writes one row. A panic of the store is recovered and logged: the
// writer is a goroutine of the corpusd process, and the record is observation.
func (f *frontRenderState) writeOne(og *ObjectGraphStore, it frontRenderItem) {
	defer f.wg.Done()
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("the writer panicked: %v", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), frontRenderWriteTimeout)
		defer cancel()
		if f.put != nil {
			err = f.put(ctx, it.obj)
		} else {
			err = og.PutObject(ctx, it.obj)
		}
	}()
	if err != nil {
		f.mu.Lock()
		if f.last == it.hash {
			f.last = ""
		}
		f.mu.Unlock()
		log.Printf("corpusd: front render record not written: %v", err)
	}
}

func frontRowOf(s FrontStory) frontrank.Row {
	r := frontrank.Row{
		PublicationID: s.PublicationID, Title: s.Title, Dek: s.Dek, UpdatedAt: s.UpdatedAt.UTC().Format(time.RFC3339Nano),
		VersionCount: float64(s.VersionCount), LatestRevision: s.LatestRevision, Promo: s.Promo, Section: s.Section,
	}
	if s.Importance > 0 {
		v := float64(s.Importance)
		r.Importance = &v
	}
	if s.SourceCount != nil {
		v := float64(*s.SourceCount)
		r.SourceCount = &v
	}
	if s.Words != nil {
		v := float64(*s.Words)
		r.Words = &v
	}
	r.Route.Bare, r.Route.Permalink = s.Route.Bare, s.Route.Permalink
	return r
}

// RecordFrontRender records page as the front when it is the canonical first
// page (limit 60, no cursor, no section) and differs from the last one
// recorded. Any other request writes nothing. It reports whether a row was
// handed to the background writer; the write itself happens later and may
// fail (it is logged).
func (s *Service) RecordFrontRender(ctx context.Context, limit int, cursor, section string, page *FrontPage) (bool, error) {
	if s == nil || page == nil || len(page.Stories) == 0 || limit != frontRenderCanonicalLimit || strings.TrimSpace(cursor) != "" || strings.TrimSpace(section) != "" {
		return false, nil
	}
	og := s.ogStore()
	if og == nil && s.frontRender.put == nil {
		return false, fmt.Errorf("front render record: object graph store unavailable")
	}
	rows := make([]frontrank.Row, 0, len(page.Stories))
	for _, st := range page.Stories {
		if st.Kind == "opinion" {
			continue
		}
		rows = append(rows, frontRowOf(st))
	}
	scored := frontrank.RankScored(rows)
	shown := make([]types.DecisionShown, len(scored))
	ids := make([]string, len(scored))
	for i, sc := range scored {
		var score *float64
		if sc.Known {
			v := sc.Score
			score = &v
		}
		shown[i] = types.DecisionShown{
			Rank: i + 1, PublicationID: sc.Row.PublicationID, Score: score, Demoted: sc.Demoted,
			Importance: sc.Row.Importance, Promo: sc.Row.Promo, SourceCount: sc.Row.SourceCount, Words: sc.Row.Words,
			VersionCount: sc.Row.VersionCount, UpdatedAt: sc.Row.UpdatedAt, Section: sc.Row.Section, Title: sc.Row.Title,
		}
		ids[i] = sc.Row.PublicationID
	}
	raw, _ := json.Marshal(shown)
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:16])

	now := time.Now().UTC()
	rec := types.DecisionRecord{
		OwnerID: frontRenderOwner, ComputerID: frontRenderOwner, CaptureID: types.DecisionPageID,
		Stage: types.DecisionStagePageRender, Decision: types.DecisionRenderedPage,
		DeciderKind: types.DeciderRule, Decider: "frontrank", CommandID: hash,
		Candidates: ids, CandidatesShown: len(ids), At: now, Shown: shown, Rule: frontRenderRule,
	}
	body, err := json.Marshal(rec)
	if err != nil {
		return false, err
	}
	// The time is in the key: a list that returns to an earlier state is a
	// new row, and a row is never written twice.
	key := fmt.Sprintf("%s\x00%s\x00%d", types.DecisionPageID, hash, now.UnixNano())
	id, err := objectgraph.BuildCanonicalID(ogKindPageRender, frontRenderOwner, objectgraph.StableSuffixFromKey(key))
	if err != nil {
		return false, err
	}
	meta := mustJSONRaw(map[string]any{"capture_id": rec.CaptureID, "stage": rec.Stage, "stories": len(ids)})
	obj := objectgraph.Object{
		CanonicalID: id, ObjectKind: ogKindPageRender, OwnerID: frontRenderOwner, ComputerID: frontRenderOwner,
		ContentHash: objectgraph.ContentHash(ogKindPageRender, body, meta), Body: body, Metadata: meta, CreatedAt: now, UpdatedAt: now,
	}

	f := &s.frontRender
	s.frontRenderWriter(og)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.last == hash {
		return false, nil
	}
	f.wg.Add(1)
	select {
	case f.queue <- frontRenderItem{obj: obj, hash: hash}:
		f.last = hash
		return true, nil
	default:
		f.wg.Done()
		log.Printf("corpusd: front render record dropped: the writer's queue is full (%d stories)", len(ids))
		return false, nil
	}
}

// ListFrontRenders reads the recorded renders, newest first, from since (the
// zero time for all of them), at most limit.
func (s *Service) ListFrontRenders(ctx context.Context, since time.Time, limit int) ([]types.DecisionRecord, error) {
	og := s.ogStore()
	if og == nil {
		return nil, fmt.Errorf("front render record: object graph store unavailable")
	}
	objs, err := og.ListObjects(ctx, objectgraph.ListFilter{Kind: ogKindPageRender, OwnerID: frontRenderOwner, Limit: limit})
	if err != nil {
		return nil, err
	}
	out := make([]types.DecisionRecord, 0, len(objs))
	for _, o := range objs {
		if o.Tombstone {
			continue
		}
		var r types.DecisionRecord
		if err := json.Unmarshal(o.Body, &r); err != nil {
			return nil, fmt.Errorf("front render record %s: %w", o.CanonicalID, err)
		}
		if !since.IsZero() && r.At.Before(since) {
			continue
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out, nil
}
