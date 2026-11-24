package platform

// Account deletion in corpusd (docs/design/account-create-delete-2026-11-23.md,
// the owner's rulings of 2026-11-23). Three host-internal routes, called by
// the auth service's purge runner:
//
//	POST /internal/platform/account/suspend    at once, reversible: the
//	     account's live takes stop being served (state suspended); restore
//	     serves them again (a cancelled deletion)
//	POST /internal/platform/account/purge      after the grace period:
//	     takes deleted, the author detached from published pieces, the
//	     account's captures and capture projections removed, and a count of
//	     what of the account's graph rows remains, by kind
//	POST /internal/platform/account/purge-tape after the grace period: the
//	     computers' tape, snapshot chunks, file chunks and rows through the
//	     destroyed sweep, scoped to those computers, with no archive
//
// Every destructive call here refuses a request that does not carry the
// deletion record's request time at least 72 hours old (purgeguard), the hard
// refusal list of tape_purge.go (the paper, the owner's computer), and an
// invalid id. What the sweep archives or keeps by its own rules is stated in
// the receipt, not hidden.
//
// Published pieces STAY: the publication object keeps its canonical id (which
// embeds the opaque user id; it cannot be rewritten without breaking every
// link) and loses its byline. The user row and email are deleted by auth, so
// nothing maps the opaque id to a person afterwards. Links in other versions
// that pinned the byline at the time they were made are not rewritten.
//
// Ways this could go wrong, written before the code (tests in
// account_purge_test.go):
//  1. a purge runs for a request with no time, a young time or a future time;
//  2. a purge removes another account's take, capture, publication or rows;
//  3. a suspended take is served, or a restored one is not;
//  4. a published piece is deleted, or keeps its byline, or loses its text;
//  5. a protected computer (the paper, the owner) is named and not refused;
//  6. the tape purge lists a computer the request did not name, or writes an
//     archive;
//  7. a second run fails.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/purgeguard"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	AccountSuspendPath   = "/internal/platform/account/suspend"
	AccountPurgePath     = "/internal/platform/account/purge"
	AccountPurgeTapePath = "/internal/platform/account/purge-tape"

	// TakeStateSuspended is a take held back while its publisher's account
	// deletion is pending. Reads serve only live takes, so a suspended take
	// is not served; restoring puts the state back.
	TakeStateSuspended = "suspended"

	// FormerAuthorLabel is the byline a published piece carries once its
	// author's account is deleted.
	FormerAuthorLabel = "former-writer"

	accountPurgeScan = 2000

	// accountTapeRetryAfter is how long a failed tape purge job is reported
	// as it is before a new one is started.
	accountTapeRetryAfter = 5 * time.Minute
)

// ErrAccountPurgeRefused is a refusal of the rule, not a failure to retry.
var ErrAccountPurgeRefused = errors.New("platform: account purge refused")

func accountPurgeGuard(userID string, computerIDs []string, requestedAt, now time.Time) error {
	if err := purgeguard.CheckClaim(requestedAt, now); err != nil {
		return fmt.Errorf("%w: %v", ErrAccountPurgeRefused, err)
	}
	userID = strings.TrimSpace(userID)
	if userID == "" || strings.ContainsAny(userID, "/\\") || len(userID) > 200 {
		return fmt.Errorf("%w: invalid user id", ErrAccountPurgeRefused)
	}
	for _, id := range computerIDs {
		if err := tapeSweepProtected(id); err != nil {
			return fmt.Errorf("%w: %v", ErrAccountPurgeRefused, err)
		}
		if !purgeIDOK(strings.TrimSpace(id)) {
			return fmt.Errorf("%w: invalid computer id", ErrAccountPurgeRefused)
		}
	}
	return nil
}

// SetAccountTakesState moves the account's takes from one state to another
// and returns how many moved. Only suspend (live to suspended) and restore
// (suspended to live) are exposed to the route.
func (s *Service) SetAccountTakesState(ctx context.Context, userID, from, to string) (int, error) {
	og, err := s.takeStore()
	if err != nil {
		return 0, err
	}
	key := types.TakePublisherKey(userID)
	s.takeMu.Lock()
	defer s.takeMu.Unlock()
	objs, err := og.ListObjectsByMetadata(ctx, string(ogKindTake), "$.publisher_key", key, takeScan)
	if err != nil {
		return 0, err
	}
	moved := 0
	for _, o := range objs {
		var row takeRow
		if o.Tombstone || json.Unmarshal(o.Body, &row) != nil || row.PublisherKey != key || row.State != from {
			continue
		}
		row.State = to
		if err := s.putTakeRow(ctx, og, row, time.Now().UTC()); err != nil {
			return moved, err
		}
		moved++
	}
	return moved, nil
}

// AccountPurgeCounts is what a purge removed or changed, and what it did not
// reach.
type AccountPurgeCounts struct {
	TakesDeleted      int              `json:"takes_deleted"`
	AuthorsDetached   int              `json:"authors_detached"`
	CapturesDeleted   int              `json:"captures_deleted"`
	ProjectionRows    map[string]int64 `json:"projection_rows,omitempty"`
	RemainingByKind   map[string]int64 `json:"remaining_by_kind,omitempty"`
	RemainingTextures map[string]int64 `json:"remaining_texture_rows,omitempty"`
}

// purgeProjectionTables are the tables of the corpus database keyed by
// owner_id that hold what a computer's research read. They are created
// lazily, so a missing one counts zero.
var purgeProjectionTables = []string{"proj_capture_index", "proj_capture_terms", "proj_capture_resolution", "vec_captures", "search_leads"}

// PurgeAccountData removes what the platform holds for the account, except
// the published pieces, which stay with the author detached.
func (s *Service) PurgeAccountData(ctx context.Context, userID string, computerIDs []string, requestedAt, now time.Time) (AccountPurgeCounts, error) {
	var c AccountPurgeCounts
	if err := accountPurgeGuard(userID, computerIDs, requestedAt, now); err != nil {
		return c, err
	}
	userID = strings.TrimSpace(userID)
	og := s.ogStore()
	if og == nil {
		return c, errors.New("platform: object graph store unavailable")
	}
	var errs []error

	// 1. Takes: live and suspended ones are deleted (audio gone, transcript
	// blanked). Deleted and yanked ones are already removed.
	takeKey := types.TakePublisherKey(userID)
	if tobjs, err := og.ListObjectsByMetadata(ctx, string(ogKindTake), "$.publisher_key", takeKey, takeScan); err != nil {
		errs = append(errs, fmt.Errorf("takes: %w", err))
	} else {
		s.takeMu.Lock()
		for _, o := range tobjs {
			var row takeRow
			if o.Tombstone || json.Unmarshal(o.Body, &row) != nil || row.PublisherKey != takeKey {
				continue
			}
			if row.State != TakeStateLive && row.State != TakeStateSuspended {
				continue
			}
			if err := s.removeTake(ctx, og, &row, TakeStateDeleted); err != nil {
				errs = append(errs, fmt.Errorf("take %s: %w", row.TakeID, err))
				continue
			}
			c.TakesDeleted++
		}
		s.takeMu.Unlock()
	}

	// 2. Published pieces stay; their byline goes.
	n, err := s.detachPublicationAuthors(ctx, og, userID)
	c.AuthorsDetached = n
	if err != nil {
		errs = append(errs, err)
	}

	// 3. Captures the account's computers wrote: the rows go; the bodies
	// become unreferenced and the capture-bodies collector removes them past
	// its grace.
	for _, kind := range []objectgraph.ObjectKind{objectgraph.WebCaptureObjectKind} {
		for {
			objs, err := og.ListObjects(ctx, objectgraph.ListFilter{Kind: kind, OwnerID: userID, Limit: accountPurgeScan})
			if err != nil {
				errs = append(errs, fmt.Errorf("list %s: %w", kind, err))
				break
			}
			if len(objs) == 0 {
				break
			}
			progressed := false
			for _, o := range objs {
				if err := og.DeleteObject(ctx, o.CanonicalID); err != nil {
					errs = append(errs, fmt.Errorf("delete %s: %w", o.CanonicalID, err))
					continue
				}
				c.CapturesDeleted++
				progressed = true
			}
			if !progressed {
				break
			}
		}
	}
	c.ProjectionRows = map[string]int64{}
	for _, table := range purgeProjectionTables {
		var reg *string
		if err := s.store.corpus().QueryRowContext(ctx, `SELECT to_regclass($1)::text`, "public."+table).Scan(&reg); err != nil || reg == nil {
			continue
		}
		res, err := s.store.corpus().ExecContext(ctx, `DELETE FROM `+table+` WHERE owner_id = ?`, userID)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", table, err))
			continue
		}
		n, _ := res.RowsAffected()
		c.ProjectionRows[table] = n
	}

	// 4. What remains of the account's rows, by kind: named in the receipt,
	// not hidden.
	c.RemainingByKind = map[string]int64{}
	if rows, err := s.store.corpus().QueryContext(ctx, `SELECT object_kind, COUNT(*) FROM og_objects WHERE owner_id = ? GROUP BY object_kind`, userID); err != nil {
		errs = append(errs, fmt.Errorf("remaining: %w", err))
	} else {
		for rows.Next() {
			var kind string
			var n int64
			if err := rows.Scan(&kind, &n); err == nil {
				c.RemainingByKind[kind] = n
			}
		}
		_ = rows.Close()
	}
	c.RemainingTextures = map[string]int64{}
	for _, table := range []string{"platform_texture_documents", "platform_texture_revisions"} {
		var n int64
		if err := s.store.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE owner_id = ?`, userID).Scan(&n); err == nil {
			c.RemainingTextures[table] = n
		}
	}
	log.Printf("corpusd: operation=account_purge takes=%d authors_detached=%d captures=%d errors=%d", c.TakesDeleted, c.AuthorsDetached, c.CapturesDeleted, len(errs))
	return c, errors.Join(errs...)
}

// detachPublicationAuthors rewrites the byline of every publication the
// account owns. A publication already detached is left alone (idempotent).
func (s *Service) detachPublicationAuthors(ctx context.Context, og *ObjectGraphStore, userID string) (int, error) {
	objs, err := og.ListObjects(ctx, objectgraph.ListFilter{Kind: "choir.publication", OwnerID: userID, Limit: accountPurgeScan})
	if err != nil {
		return 0, fmt.Errorf("publications: %w", err)
	}
	detached := 0
	var errs []error
	for _, o := range objs {
		var meta map[string]any
		if json.Unmarshal(o.Metadata, &meta) != nil {
			errs = append(errs, fmt.Errorf("publication %s: metadata unreadable", o.CanonicalID))
			continue
		}
		label, _ := meta["author_label"].(string)
		if strings.TrimSpace(label) == "" || label == FormerAuthorLabel {
			continue
		}
		meta["author_label"] = FormerAuthorLabel
		raw, err := json.Marshal(meta)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		o.Metadata = raw
		o.UpdatedAt = time.Now().UTC()
		if err := og.PutObject(corpusdWriter(ctx), o); err != nil {
			errs = append(errs, fmt.Errorf("publication %s: %w", o.CanonicalID, err))
			continue
		}
		detached++
	}
	return detached, errors.Join(errs...)
}

// --- the tape ---

// AccountTapeJob is the state of one account's tape purge.
type AccountTapeJob struct {
	State            string   `json:"state"` // running | done | failed
	Computers        []string `json:"computers"`
	Error            string   `json:"error,omitempty"`
	Refusal          string   `json:"refusal,omitempty"`
	Permanent        bool     `json:"permanent,omitempty"`
	Deleted          int      `json:"deleted"`
	BytesGone        int64    `json:"bytes_deleted"`
	ComputersDeleted int      `json:"computers_deleted"`
	Archives         int      `json:"archives"`
	StartedAt        string   `json:"started_at"`
	FinishedAt       string   `json:"finished_at,omitempty"`
	RowsDeleted      int64    `json:"rows_deleted"`
}

type accountTapeState struct {
	mu   sync.Mutex
	jobs map[string]*AccountTapeJob // keyed by sorted computer ids
}

func accountTapeKey(ids []string) string {
	cp := append([]string(nil), ids...)
	sort.Strings(cp)
	return strings.Join(cp, ",")
}

// StartAccountTapePurge starts (or reports) the tape purge of the computers.
// One job per set of computers: a second call returns the job that exists
// until it has failed or finished, so the caller can poll with the same call.
func (s *Service) StartAccountTapePurge(userID string, computerIDs []string, requestedAt, now time.Time) (AccountTapeJob, error) {
	if err := accountPurgeGuard(userID, computerIDs, requestedAt, now); err != nil {
		return AccountTapeJob{}, err
	}
	if len(computerIDs) == 0 {
		return AccountTapeJob{}, fmt.Errorf("%w: no computer named", ErrAccountPurgeRefused)
	}
	ids := make([]string, 0, len(computerIDs))
	for _, id := range computerIDs {
		ids = append(ids, strings.TrimSpace(id))
	}
	key := accountTapeKey(ids)
	st := &s.acctTape
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.jobs == nil {
		st.jobs = map[string]*AccountTapeJob{}
	}
	if j := st.jobs[key]; j != nil {
		switch {
		case j.State == "running" || j.State == "done":
			return *j, nil
		case j.State == "failed":
			// A failure stays readable for a while, so the caller sees why
			// (a named refusal, an error) before the job is started again.
			if at, err := time.Parse(time.RFC3339, j.FinishedAt); err == nil && time.Since(at) < accountTapeRetryAfter {
				return *j, nil
			}
		}
	}
	job := &AccountTapeJob{State: "running", Computers: ids, StartedAt: time.Now().UTC().Format(time.RFC3339)}
	st.jobs[key] = job
	go func() {
		res := s.runAccountTapePurge(context.Background(), ids)
		st.mu.Lock()
		defer st.mu.Unlock()
		res.StartedAt = job.StartedAt
		res.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		*job = res
	}()
	return *job, nil
}

// AccountTapeJobFor reads the job of a set of computers.
func (s *Service) AccountTapeJobFor(ids []string) (AccountTapeJob, bool) {
	st := &s.acctTape
	st.mu.Lock()
	defer st.mu.Unlock()
	j := st.jobs[accountTapeKey(ids)]
	if j == nil {
		return AccountTapeJob{}, false
	}
	return *j, true
}

// runAccountTapePurge runs the destroyed sweep scoped to the computers, with
// no archive: a dry run, then the apply of that run's manifest. Evidence for
// "no archive": tape_sweep_apply.go archives only when the request does not
// say no_archive, and the manifest records the flag, so none is written.
func (s *Service) runAccountTapePurge(ctx context.Context, ids []string) AccountTapeJob {
	job := AccountTapeJob{State: "failed", Computers: ids}
	req := TapeSweepRequest{Action: TapeSweepActionDryRun, Sweeps: []string{TapeSweepDestroyed}, Computers: ids,
		Grace: TapeSweepMinGrace.String(), Quiet: TapeSweepMinGrace.String(), AcceptUnrecordedForkLineage: true, NoArchive: true}
	dry, err := s.RunTapeSweep(ctx, req)
	if err != nil {
		job.Error = err.Error()
		return job
	}
	if dry.DestroyedRefused != "" {
		job.Error = "destroyed sweep refused: " + dry.DestroyedRefused
		return job
	}
	proofs := map[string]TapeSweepComputerProof{}
	for _, p := range dry.NamedProofs {
		proofs[p.ComputerID] = p
	}
	for _, id := range ids {
		p, ok := proofs[id]
		switch {
		case !ok:
			// Every named computer has a proof; a missing one is a bug, and
			// the purge says so instead of calling the tape gone.
			job.Error = "no proof for " + id
			return job
		case p.Reason == "no tape head":
			// The computer never wrote a tape here; nothing to purge.
		case p.Reason == "already deleted":
		case !p.Deletable:
			job.Refusal = id + ": " + p.Reason
			// A route slot, a registry entry or an ownership record is not
			// going to change by waiting; a quiet window or a busy sweep is.
			job.Permanent = strings.Contains(p.Reason, "route slot") || strings.Contains(p.Reason, "protected")
			return job
		}
	}
	if dry.DestroyedListed == 0 {
		job.State = "done"
		return job
	}
	applyReq := req
	applyReq.Action, applyReq.RunID, applyReq.ManifestSHA256 = TapeSweepActionApply, dry.RunID, dry.ManifestSHA256
	applied, err := s.RunTapeSweep(ctx, applyReq)
	if applied.Apply != nil {
		job.Deleted, job.BytesGone, job.ComputersDeleted = applied.Apply.Deleted, applied.Apply.BytesDeleted, applied.Apply.ComputersDeleted
		job.Archives = len(applied.Apply.Archives)
		for _, cl := range applied.Apply.Classes {
			job.RowsDeleted += cl.Rows
		}
		if applied.Apply.Errors > 0 && err == nil {
			err = fmt.Errorf("%d errors during apply", applied.Apply.Errors)
		}
	}
	if err != nil {
		job.Error = err.Error()
		return job
	}
	job.State = "done"
	return job
}

// --- HTTP ---

type accountRequest struct {
	UserID      string   `json:"user_id"`
	ComputerIDs []string `json:"computer_ids,omitempty"`
	Action      string   `json:"action,omitempty"`
	RequestedAt string   `json:"deletion_requested_at,omitempty"`
}

func (h *Handler) accountRoute(w http.ResponseWriter, r *http.Request, method string) (accountRequest, time.Time, bool) {
	var in accountRequest
	if h.podcastRefuse(w, r, method) {
		return in, time.Time{}, false
	}
	if r.Method == http.MethodPost {
		if !decodeTakeBody(w, r, &in) {
			return in, time.Time{}, false
		}
	}
	requested, err := time.Parse(time.RFC3339, in.RequestedAt)
	if err != nil {
		requested = time.Time{}
	}
	return in, requested, true
}

func accountFail(w http.ResponseWriter, what string, err error) {
	if errors.Is(err, ErrAccountPurgeRefused) {
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
		return
	}
	log.Printf("corpusd: account %s: %v", what, err)
	writeJSON(w, http.StatusConflict, apiError{Error: "account " + what + " did not finish; it will be retried"})
}

// HandleAccountSuspend serves POST AccountSuspendPath {user_id, action}.
func (h *Handler) HandleAccountSuspend(w http.ResponseWriter, r *http.Request) {
	in, _, ok := h.accountRoute(w, r, http.MethodPost)
	if !ok {
		return
	}
	if strings.TrimSpace(in.UserID) == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "user_id is required"})
		return
	}
	from, to := TakeStateLive, TakeStateSuspended
	switch in.Action {
	case "suspend":
	case "restore":
		from, to = TakeStateSuspended, TakeStateLive
	default:
		writeJSON(w, http.StatusBadRequest, apiError{Error: "action is suspend or restore"})
		return
	}
	n, err := h.service.SetAccountTakesState(r.Context(), in.UserID, from, to)
	if err != nil {
		accountFail(w, "suspend", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"takes": n})
}

// HandleAccountPurge serves POST AccountPurgePath.
func (h *Handler) HandleAccountPurge(w http.ResponseWriter, r *http.Request) {
	in, requested, ok := h.accountRoute(w, r, http.MethodPost)
	if !ok {
		return
	}
	c, err := h.service.PurgeAccountData(r.Context(), in.UserID, in.ComputerIDs, requested, time.Now().UTC())
	if err != nil {
		accountFail(w, "purge", err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// HandleAccountPurgeTape serves POST AccountPurgeTapePath: it starts the job
// or reports the one running, and answers 202 while it runs.
func (h *Handler) HandleAccountPurgeTape(w http.ResponseWriter, r *http.Request) {
	in, requested, ok := h.accountRoute(w, r, http.MethodPost)
	if !ok {
		return
	}
	job, err := h.service.StartAccountTapePurge(in.UserID, in.ComputerIDs, requested, time.Now().UTC())
	if err != nil {
		accountFail(w, "tape purge", err)
		return
	}
	status := http.StatusOK
	if job.State == "running" {
		status = http.StatusAccepted
	}
	writeJSON(w, status, job)
}
