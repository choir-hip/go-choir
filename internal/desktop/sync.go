package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/base/model"
	"github.com/yusefmosiah/go-choir/internal/base/planner"
	"github.com/yusefmosiah/go-choir/internal/base/tree"
)

// SyncedState is the persisted local record of the last-synced tree and
// cursor. It is the "synced ancestor" the planner compares against. The
// state is stored as JSON in the Choir config directory.
type SyncedState struct {
	Cursor    int64           `json:"cursor"`
	Items     []model.Item    `json:"items"`
	Versions  []model.Version `json:"versions"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// SyncedStateStore persists the synced state (cursor + synced tree) across
// sync cycles. The default implementation is a JSON file.
type SyncedStateStore interface {
	Load() (SyncedState, error)
	Save(SyncedState) error
}

// FileSyncedStateStore stores the synced state in a JSON file.
type FileSyncedStateStore struct {
	path string
}

// NewFileSyncedStateStore returns a file-backed synced-state store at path.
func NewFileSyncedStateStore(path string) *FileSyncedStateStore {
	return &FileSyncedStateStore{path: path}
}

// Load reads the synced state. A missing file yields an empty state with
// cursor 0 (full sync on first run).
func (f *FileSyncedStateStore) Load() (SyncedState, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return SyncedState{}, nil
		}
		return SyncedState{}, fmt.Errorf("synced state: read %s: %w", f.path, err)
	}
	var s SyncedState
	if err := json.Unmarshal(data, &s); err != nil {
		return SyncedState{}, fmt.Errorf("synced state: parse %s: %w", f.path, err)
	}
	return s, nil
}

// Save writes the synced state with 0600 permissions.
func (f *FileSyncedStateStore) Save(s SyncedState) error {
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("synced state: mkdir %s: %w", dir, err)
	}
	s.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("synced state: marshal: %w", err)
	}
	if err := os.WriteFile(f.path, data, 0o600); err != nil {
		return fmt.Errorf("synced state: write %s: %w", f.path, err)
	}
	return nil
}

// SyncConfig configures the sync engine.
type SyncConfig struct {
	// BaseURL is the Choir backend URL (e.g. https://choir.news).
	BaseURL string
	// LocalRoot is the absolute path to the local folder being synced.
	LocalRoot string
	// DeviceID identifies this desktop device in journal events.
	DeviceID string
	// Interval is the delay between automatic sync cycles. If zero, the loop
	// only syncs when SyncNow is called.
	Interval time.Duration
	// OwnerID is the user ID owning the synced items. Filled from the API key
	// identity; may be empty (the server defaults it to the authenticated
	// user).
	OwnerID string
}

// SyncEngine is the cancellable background sync loop. It scans the local
// folder, fetches the remote delta, runs the planner, executes non-conflicting
// actions, surfaces conflicts to the ConflictManager, and updates the synced
// cursor. The loop is cancellable via the context passed to Start.
type SyncEngine struct {
	cfg    SyncConfig
	client *BaseClient
	scan   *LocalTreeBuilder
	state  SyncedStateStore
	status *StatusTracker
	conf   *ConflictManager

	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
	syncNow chan struct{}

	// syncedMu guards lastSynced, the cached synced tree from the most recent
	// cycle. deleteLocal and stateSnapshotItem read it to resolve paths and
	// decide create-vs-update.
	syncedMu   sync.RWMutex
	lastSynced tree.Tree
}

// NewSyncEngine constructs a sync engine. The apiKey is the Choir API key
// secret (choir_sk_...) used to authenticate Base API calls.
func NewSyncEngine(cfg SyncConfig, apiKey string) *SyncEngine {
	client := NewBaseClient(cfg.BaseURL, apiKey)
	scan := NewLocalTreeBuilder(cfg.LocalRoot, cfg.DeviceID)
	return &SyncEngine{
		cfg:     cfg,
		client:  client,
		scan:    scan,
		status:  NewStatusTracker(),
		conf:    NewConflictManager(),
		syncNow: make(chan struct{}, 1),
	}
}

// SetSyncedStateStore replaces the default in-memory synced-state store with
// a persistent one. If not called, the engine uses an in-memory store (empty
// synced tree on every restart — full resync).
func (e *SyncEngine) SetSyncedStateStore(s SyncedStateStore) {
	if s != nil {
		e.state = s
	}
}

// Status returns the StatusTracker for reading sync progress from the UI.
func (e *SyncEngine) Status() *StatusTracker { return e.status }

// Conflicts returns the ConflictManager for surfacing/resolving conflicts.
func (e *SyncEngine) Conflicts() *ConflictManager { return e.conf }

// SetHTTPClient replaces the Base API client's underlying *http.Client.
// Intended for tests that want to route through an httptest.Server's client.
func (e *SyncEngine) SetHTTPClient(h *http.Client) {
	e.client.SetHTTPClient(h)
}

// Start launches the background sync loop. It returns immediately. The loop
// runs until ctx is cancelled or Stop is called. When Interval is zero the
// loop waits on the syncNow channel; otherwise it ticks at Interval.
func (e *SyncEngine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return ErrAlreadyRunning
	}
	runCtx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	e.running = true
	e.mu.Unlock()

	if e.state == nil {
		// In-memory default: empty synced state.
		e.state = NewFileSyncedStateStore(filepath.Join(e.cfg.LocalRoot, ".choir-synced-state.json"))
	}

	go e.loop(runCtx)
	return nil
}

// Stop cancels the sync loop and waits for it to drain. It is safe to call
// from any goroutine.
func (e *SyncEngine) Stop() {
	e.mu.Lock()
	if e.cancel != nil {
		e.cancel()
	}
	e.running = false
	e.mu.Unlock()
}

// SyncNow triggers an immediate sync cycle (non-blocking). If a cycle is
// already running, the request is coalesced (buffered channel of size 1).
func (e *SyncEngine) SyncNow() {
	select {
	case e.syncNow <- struct{}{}:
	default:
	}
}

// IsRunning reports whether the sync loop is active.
func (e *SyncEngine) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

// loop is the background sync loop. It runs until ctx is cancelled.
func (e *SyncEngine) loop(ctx context.Context) {
	defer e.status.MarkCancelled()
	// Run one cycle immediately on start.
	e.runCycle(ctx)

	if e.cfg.Interval > 0 {
		ticker := time.NewTicker(e.cfg.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.runCycle(ctx)
			case <-e.syncNow:
				e.runCycle(ctx)
			}
		}
	}
	// Manual mode: wait for SyncNow or cancellation.
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.syncNow:
			e.runCycle(ctx)
		}
	}
}

// runCycle performs one full sync pass. It is the heart of the engine:
// scan → fetch delta → derive remote tree → load synced tree → plan →
// execute non-conflicting actions → surface conflicts → persist cursor.
func (e *SyncEngine) runCycle(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}

	// 1. Scan local folder.
	e.status.SetPhase(PhaseScanning)
	local, err := e.scan.Scan()
	if err != nil {
		e.status.SetError(err.Error())
		return
	}

	// 2. Fetch remote delta.
	e.status.SetPhase(PhaseFetching)
	state, err := e.state.Load()
	if err != nil {
		e.status.SetError("load synced state: " + err.Error())
		return
	}
	delta, err := e.client.FetchDelta(state.Cursor)
	if err != nil {
		e.status.SetError("fetch delta: " + err.Error())
		return
	}
	e.status.SetCursor(delta.Cursor, delta.Head)

	// 3. Derive the remote tree from the synced tree + delta events.
	//    We rebuild the remote tree by applying the delta events on top of
	//    the synced tree's events. Since the synced tree is a snapshot, we
	//    reconstruct it as a tree.Tree and then apply delta events via
	//    tree.Derive over the combined event set.
	syncedTree := snapshotToTree(state)
	// Cache the synced tree for deleteLocal / create-vs-update decisions.
	e.syncedMu.Lock()
	e.lastSynced = syncedTree
	e.syncedMu.Unlock()
	remoteTree := applyDelta(syncedTree, delta.Events)

	// 4. Plan.
	e.status.SetPhase(PhasePlanning)
	remotePTree := treeToPlanner(remoteTree)
	syncedPTree := snapshotToPlanner(state)
	actions, conflicts := planner.Plan(remotePTree, local, syncedPTree)

	// 5. Surface conflicts.
	e.conf.SetConflicts(conflicts, local, remotePTree)
	e.status.UpdateFromPlan(actions, conflicts, local, remotePTree)

	if len(conflicts) > 0 {
		e.status.SetPhase(PhaseConflicts)
	}

	// 6. If there are unresolved conflicts, pause before executing: the
	//    user must choose keep-local/keep-remote/keep-both. Non-conflicting
	//    actions are still executed so the rest of the tree converges.
	e.status.SetPhase(PhaseExecuting)
	e.status.SetActionTotals(len(actions))
	executed, err := e.executeActions(ctx, actions, local, remotePTree)
	if err != nil {
		e.status.SetError("execute actions: " + err.Error())
		// Persist whatever progress we made before the error.
		_ = e.persistState(state, delta, executed)
		return
	}

	// 7. Apply conflict resolutions (for conflicts the user has already
	//    resolved). Unresolved conflicts block cursor advancement.
	resolvedExec, err := e.applyResolutions(ctx, conflicts, local, remotePTree)
	if err != nil {
		e.status.SetError("apply resolutions: " + err.Error())
		_ = e.persistState(state, delta, executed)
		return
	}
	for id := range resolvedExec {
		executed[id] = struct{}{}
	}

	if e.conf.HasUnresolved() {
		// Pause: do not advance the cursor past unresolved conflicts. The
		// local and remote sides remain divergent until the user chooses.
		// We persist only the contiguous remote delta prefix before the first
		// unresolved conflict item. The cursor is a linear acknowledgement, so
		// it cannot skip an unresolved event even when later actions succeeded.
		e.status.SetPhase(PhaseConflicts)
		safeCursor := safeCursorBeforeUnresolved(state.Cursor, delta.Events, unresolvedItemIDs(e.conf))
		_ = e.persistState(state, deltaUpToCursor(delta, safeCursor), executed)
		return
	}

	// 8. Persist the synced state with the new cursor.
	if err := e.persistState(state, delta, executed); err != nil {
		e.status.SetError("persist state: " + err.Error())
		return
	}

	// All conflicts resolved and applied: clear the conflict set.
	e.conf.Clear()

	e.status.SetCursor(delta.Cursor, delta.Head)
	e.status.MarkSynced()
}

// executeActions runs the planner's actions for items that have no conflict.
// Items with an unresolved OR resolved conflict are skipped here; resolved
// conflicts are handled by applyResolutions.
func (e *SyncEngine) executeActions(ctx context.Context, actions []planner.Action, local, remote planner.Tree) (map[model.ItemID]struct{}, error) {
	executed := make(map[model.ItemID]struct{})
	for _, a := range actions {
		if err := ctx.Err(); err != nil {
			return executed, err
		}
		// Skip items that have a conflict record (resolved or not); the
		// conflict manager owns their reconciliation.
		if hasConflictRecord(e.conf, a.ItemID) {
			continue
		}
		if err := e.executeAction(a, local, remote); err != nil {
			return executed, fmt.Errorf("action %s for %s: %w", a.Type, a.ItemID, err)
		}
		executed[a.ItemID] = struct{}{}
		e.status.ActionDone()
	}
	return executed, nil
}

// applyResolutions converts each resolved conflict into a concrete action and
// executes it. Unresolved conflicts are skipped (they block cursor advance
// upstream). The returned set is the items whose resolution was applied.
func (e *SyncEngine) applyResolutions(ctx context.Context, conflicts []planner.Conflict, local, remote planner.Tree) (map[model.ItemID]struct{}, error) {
	applied := make(map[model.ItemID]struct{})
	for _, c := range conflicts {
		if err := ctx.Err(); err != nil {
			return applied, err
		}
		res, ok := e.conf.Resolution(c.ItemID)
		if !ok {
			continue // unresolved; skipped
		}
		localID := conflictLocalItemID(c)
		remoteID := conflictRemoteItemID(c)
		switch res {
		case ResolveKeepLocal:
			if err := e.uploadLocal(localID, local); err != nil {
				return applied, fmt.Errorf("keep_local %s: %w", localID, err)
			}
		case ResolveKeepRemote:
			if err := e.downloadRemote(remoteID, remote); err != nil {
				return applied, fmt.Errorf("keep_remote %s: %w", remoteID, err)
			}
		case ResolveKeepBoth:
			if err := e.uploadLocal(localID, local); err != nil {
				return applied, fmt.Errorf("keep_both upload %s: %w", localID, err)
			}
			if err := e.downloadRemote(remoteID, remote); err != nil {
				return applied, fmt.Errorf("keep_both download %s: %w", remoteID, err)
			}
		}
		applied[c.ItemID] = struct{}{}
		e.status.ActionDone()
	}
	return applied, nil
}

// hasConflictRecord reports whether the item has any conflict record
// (resolved or unresolved) in the conflict manager.
func hasConflictRecord(cm *ConflictManager, id model.ItemID) bool {
	for _, c := range cm.All() {
		if c.ItemID == id || c.LocalItemID == id || c.RemoteItemID == id {
			return true
		}
	}
	return false
}

func conflictLocalItemID(c planner.Conflict) model.ItemID {
	if c.LocalItemID != "" {
		return c.LocalItemID
	}
	if c.LocalVer.ItemID != "" {
		return c.LocalVer.ItemID
	}
	return c.ItemID
}

func conflictRemoteItemID(c planner.Conflict) model.ItemID {
	if c.RemoteItemID != "" {
		return c.RemoteItemID
	}
	if c.RemoteVer.ItemID != "" {
		return c.RemoteVer.ItemID
	}
	return c.ItemID
}

func unresolvedItemIDs(cm *ConflictManager) map[model.ItemID]struct{} {
	pending := cm.Pending()
	out := make(map[model.ItemID]struct{}, len(pending))
	for _, c := range pending {
		out[c.ItemID] = struct{}{}
		if c.LocalItemID != "" {
			out[c.LocalItemID] = struct{}{}
		}
		if c.RemoteItemID != "" {
			out[c.RemoteItemID] = struct{}{}
		}
	}
	return out
}

func safeCursorBeforeUnresolved(start int64, events []model.Event, unresolved map[model.ItemID]struct{}) int64 {
	safe := start
	for _, evt := range eventsByCursor(events) {
		if evt.CursorSeq <= safe {
			continue
		}
		if _, blocked := unresolved[evt.ItemID]; blocked {
			break
		}
		safe = evt.CursorSeq
	}
	return safe
}

func eventsByCursor(events []model.Event) []model.Event {
	out := append([]model.Event(nil), events...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].CursorSeq < out[j].CursorSeq
	})
	return out
}

func deltaUpToCursor(delta DeltaResponse, cursor int64) DeltaResponse {
	events := make([]model.Event, 0, len(delta.Events))
	for _, evt := range delta.Events {
		if evt.CursorSeq <= cursor {
			events = append(events, evt)
		}
	}
	return DeltaResponse{
		Events: events,
		Cursor: cursor,
		Head:   delta.Head,
	}
}

// executeAction performs a single reconciliation action against the local
// filesystem and/or the remote Base API.
func (e *SyncEngine) executeAction(a planner.Action, local, remote planner.Tree) error {
	switch a.Type {
	case planner.ActionUpload:
		return e.uploadLocal(a.ItemID, local)
	case planner.ActionUpdateRemote:
		return e.uploadLocal(a.ItemID, local)
	case planner.ActionMoveRemote:
		return e.uploadLocal(a.ItemID, local) // move is encoded in the item location
	case planner.ActionDownload:
		return e.downloadRemote(a.ItemID, remote)
	case planner.ActionUpdateLocal:
		return e.downloadRemote(a.ItemID, remote)
	case planner.ActionMoveLocal:
		return e.downloadRemote(a.ItemID, remote)
	case planner.ActionDeleteLocal:
		return e.deleteLocal(a.ItemID)
	case planner.ActionDeleteRemote:
		return e.deleteRemote(a.ItemID)
	default:
		return fmt.Errorf("unknown action type %s", a.Type)
	}
}

// uploadLocal uploads a local item (and its blob for files) to the remote.
func (e *SyncEngine) uploadLocal(id model.ItemID, local planner.Tree) error {
	item, ok := local.Items[id]
	if !ok {
		return fmt.Errorf("local item %s not found", id)
	}
	ver := local.Versions[id]
	owner := e.cfg.OwnerID

	var blobRef model.BlobRef
	if item.Kind == model.KindFile && ver.BlobRef != "" {
		// Read the file bytes and upload the blob.
		rel := RelPathFromID(local, id)
		if rel == "" {
			return fmt.Errorf("cannot resolve path for %s", id)
		}
		data, err := e.scan.ReadFile(rel)
		if err != nil {
			return fmt.Errorf("read local file: %w", err)
		}
		resp, err := e.client.PutBlob(data, ver.MediaType)
		if err != nil {
			return fmt.Errorf("upload blob: %w", err)
		}
		blobRef = resp.BlobRef
	}

	// Create/update the item via the Base API.
	evtType := model.EventCreate
	if _, exists := e.stateSnapshotItem(id); exists {
		evtType = model.EventUpdate
	}
	req := PutItemRequest{
		ItemID:       id,
		OwnerID:      owner,
		EventType:    evtType,
		Kind:         item.Kind,
		ParentItemID: item.ParentItemID,
		Name:         item.Name,
		BlobRef:      blobRef,
		VersionID:    ver.VersionID,
		MediaType:    ver.MediaType,
		ContentHash:  ver.ContentHash,
		DeviceID:     e.cfg.DeviceID,
	}
	if _, err := e.client.PutItem(req); err != nil {
		return fmt.Errorf("put item: %w", err)
	}
	return nil
}

// stateSnapshotItem reports whether an item was in the last synced state. It
// is used to decide create vs update event types. The server is the
// authority on item existence; this is a best-effort local hint.
func (e *SyncEngine) stateSnapshotItem(id model.ItemID) (model.Item, bool) {
	e.syncedMu.RLock()
	defer e.syncedMu.RUnlock()
	item, ok := e.lastSynced.Items[id]
	return item, ok
}

// downloadRemote downloads a remote item to the local filesystem.
func (e *SyncEngine) downloadRemote(id model.ItemID, remote planner.Tree) error {
	item, ok := remote.Items[id]
	if !ok {
		return fmt.Errorf("remote item %s not found", id)
	}
	// Reconstruct the local path from the remote tree's location chain.
	rel := RelPathFromID(remote, id)
	if rel == "" {
		return fmt.Errorf("cannot resolve remote path for %s", id)
	}
	abs := e.scan.AbsPath(rel)

	if item.Kind == model.KindFolder {
		return os.MkdirAll(abs, 0o755)
	}

	resp, err := e.client.GetItem(id)
	if err != nil {
		return fmt.Errorf("get item: %w", err)
	}
	if resp.Version.BlobRef == "" {
		return fmt.Errorf("remote item %s has no blob ref", id)
	}
	data, err := e.client.GetBlob(resp.Version.BlobRef)
	if err != nil {
		return fmt.Errorf("get blob: %w", err)
	}
	// Ensure the parent directory exists.
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return fmt.Errorf("write local file: %w", err)
	}
	return nil
}

// deleteLocal removes the local copy of an item (remote deleted it).
func (e *SyncEngine) deleteLocal(id model.ItemID) error {
	// We need the local path; the synced tree has it. Use the synced state
	// loaded in runCycle via the planner's synced tree. Since we don't have
	// direct access here, we look it up from the last known local scan by
	// reconstructing from the synced state snapshot stored on the engine.
	rel := e.syncedRelPath(id)
	if rel == "" {
		return nil // nothing to delete locally
	}
	abs := e.scan.AbsPath(rel)
	if err := os.RemoveAll(abs); err != nil {
		return fmt.Errorf("remove local %s: %w", abs, err)
	}
	return nil
}

// deleteRemote sends a delete event for the item to the Base API.
func (e *SyncEngine) deleteRemote(id model.ItemID) error {
	req := PutItemRequest{
		ItemID:    id,
		OwnerID:   e.cfg.OwnerID,
		EventType: model.EventDelete,
		Kind:      model.KindFile,
		DeviceID:  e.cfg.DeviceID,
	}
	if _, err := e.client.PutItem(req); err != nil {
		return fmt.Errorf("delete remote item: %w", err)
	}
	return nil
}

// syncedRelPath returns the relative path of an item in the last synced
// state, used by deleteLocal to locate the file to remove.
func (e *SyncEngine) syncedRelPath(id model.ItemID) string {
	e.syncedMu.RLock()
	defer e.syncedMu.RUnlock()
	if item, ok := e.lastSynced.Items[id]; ok {
		return relPathFromTreeItem(e.lastSynced, item)
	}
	return ""
}

// persistState writes the updated synced state (new cursor + the synced tree
// updated to reflect executed actions).
func (e *SyncEngine) persistState(prev SyncedState, delta DeltaResponse, executed map[model.ItemID]struct{}) error {
	// Rebuild the synced tree from the previous state + delta events, then
	// keep only items (the synced tree is now the remote tree at the new
	// cursor for executed items; unresolved conflicts retain their prior
	// synced version).
	syncedTree := applyDelta(snapshotToTree(prev), delta.Events)
	next := SyncedState{
		Cursor: delta.Cursor,
	}
	for id, it := range syncedTree.Items {
		next.Items = append(next.Items, it)
		if v, ok := syncedTree.Versions[id]; ok {
			next.Versions = append(next.Versions, v)
		}
	}
	return e.state.Save(next)
}

// --- tree conversion helpers --------------------------------------------

// snapshotToTree rebuilds a tree.Tree from a persisted SyncedState.
func snapshotToTree(s SyncedState) tree.Tree {
	t := tree.NewTree()
	for _, it := range s.Items {
		t.Items[it.ItemID] = it
	}
	for _, v := range s.Versions {
		t.Versions[v.ItemID] = v
	}
	return t
}

// snapshotToPlanner builds a planner.Tree from a persisted SyncedState.
func snapshotToPlanner(s SyncedState) planner.Tree {
	t := planner.NewTree()
	for _, it := range s.Items {
		t.Items[it.ItemID] = it
	}
	for _, v := range s.Versions {
		t.Versions[v.ItemID] = v
	}
	return t
}

// treeToPlanner converts a tree.Tree to a planner.Tree.
func treeToPlanner(t tree.Tree) planner.Tree {
	out := planner.NewTree()
	for id, it := range t.Items {
		out.Items[id] = it
	}
	for id, v := range t.Versions {
		out.Versions[id] = v
	}
	return out
}

// applyDelta folds a slice of delta events into an existing tree, returning
// the new tree at the updated cursor. The events are applied in CursorSeq
// order via tree.Derive over the combined event set (existing tree's events
// are not stored, so we re-derive from the snapshot items + new events by
// treating the snapshot as the base and applying only the delta).
//
// Because tree.Derive rebuilds from events, and the synced snapshot is a
// materialized tree (not events), we instead apply delta events directly to
// the materialized tree using a local apply pass that mirrors tree.applyEvent
// semantics. This keeps the remote tree consistent with the journal's view.
func applyDelta(base tree.Tree, events []model.Event) tree.Tree {
	out := tree.NewTree()
	for id, it := range base.Items {
		out.Items[id] = it
	}
	for id, v := range base.Versions {
		out.Versions[id] = v
	}
	// Apply delta events in CursorSeq order. We use tree.Derive over the
	// delta events only to get correctly-ordered application, then merge.
	// Simpler: derive a delta tree and merge non-deleted items over the base.
	deltaTree := tree.Derive(events)
	for id, it := range deltaTree.Items {
		out.Items[id] = it
		if v, ok := deltaTree.Versions[id]; ok {
			out.Versions[id] = v
		} else if it.CurrentVersion == "" {
			// A delete event: clear the version.
			delete(out.Versions, id)
		}
	}
	return out
}

// relPathFromTreeItem walks the parent chain in a tree.Tree to reconstruct the
// relative path of an item.
func relPathFromTreeItem(t tree.Tree, item model.Item) string {
	var parts []string
	cur := item
	for cur.ParentItemID != "" {
		parts = append([]string{cur.Name}, parts...)
		parent, ok := t.Items[cur.ParentItemID]
		if !ok {
			break
		}
		cur = parent
	}
	parts = append([]string{cur.Name}, parts...)
	return joinPath(parts)
}

func joinPath(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "/"
		}
		out += p
	}
	return out
}

// ErrAlreadyRunning is returned by Start when the loop is already active.
var ErrAlreadyRunning = errSentinel("sync engine already running")
