package agentcore

// Restore as an accepted act (docs/problems/restore-cannot-finish-inside-its-
// ten-minute-request-2026-11-14.md, "Repair design" and "Failure list"). Every
// test goes through the real router (NewAPIHandler(rt).HandleComputersRouter)
// or the boot function main calls; the tape, the store and the updater root
// are the ones the other restore tests use. The tests were written before the
// code and seen failing.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type restoreActFixture struct {
	t            *testing.T
	rt           *Runtime
	cas          *replayEventCAS
	handler      *APIHandler
	computerID   string
	storePath    string
	updaterRoot  string
	priorDigest  string
	targetDigest string
	checkpoint   selfdevprotocol.Checkpoint
	headBefore   *computerevent.Head
}

func newRestoreActFixture(t *testing.T, computerID string, tail int) *restoreActFixture {
	return newRestoreActFixtureIn(t, t.TempDir(), computerID, tail)
}

// newRestoreActFixtureIn builds a computer whose tape holds a genesis, a
// snapshot at sequence 1 and tail more events, a checkpoint bound at the head,
// and two pinned releases (the live one and the checkpoint's).
func newRestoreActFixtureIn(t *testing.T, base, computerID string, tail int) *restoreActFixture {
	t.Helper()
	dataDir := filepath.Join(base, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	storePath := filepath.Join(dataDir, "runtime.db")
	live, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	rt, cas, _ := rematerializeTapeRuntime(t, computerID, storePath, live)
	t.Cleanup(func() {
		if rt.store != nil {
			_ = rt.store.Close()
		}
	})
	updaterRoot := filepath.Join(base, "updater")
	priorDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>live</html>")
	targetDigest, targetIdentity := pinFrontendRelease(t, updaterRoot, computerID, "<html>checkpoint</html>")
	pointCurrent(t, updaterRoot, priorDigest)
	rt.selfdevUpdaterRoot = updaterRoot
	// What /health reads of a runtime (the other restore tests never serve it).
	rt.provider = healthStubProvider{}
	rt.health = types.HealthReady
	ctx := context.Background()
	seedRestoreBase(t, ctx, rt, cas, computerID, 1)
	for i := 0; i < tail; i++ {
		id, err := computerevent.NewEventID()
		if err != nil {
			t.Fatal(err)
		}
		event := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: id, ComputerID: computerID,
			EventKind: computerevent.EventArtifactProduced, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: fmt.Sprintf("restore-act-tail-%d", i), ActorProfile: "management", AuthorityRef: "owner", PrivacyClass: "owner",
			PayloadCommitment: strings.Repeat("c", 64), ReducerVersion: computerevent.ReducerVersionV1,
		}
		if _, err := rt.eventAppender.AppendNew(ctx, event, computerevent.TransitionInput{}, nil); err != nil {
			t.Fatal(err)
		}
	}
	head, err := rt.store.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("head: %v", err)
	}
	witness := boundWitness(t, rt, computerID)
	checkpoint := rematerializeTestCheckpoint(t, computerID, witness, targetDigest, targetIdentity, head.CanonicalEventHead)
	// Registered after the store's close, so it runs before it: an act a test
	// leaves running ends (or is waited for) before the directory goes.
	t.Cleanup(func() { drainRestoreActs(rt) })
	return &restoreActFixture{
		t: t, rt: rt, cas: cas, handler: NewAPIHandler(rt), computerID: computerID, storePath: storePath,
		updaterRoot: updaterRoot, priorDigest: priorDigest, targetDigest: targetDigest, checkpoint: checkpoint, headBefore: head,
	}
}

func (f *restoreActFixture) do(method, path string, body any) (int, map[string]any) {
	f.t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			f.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	request := httptest.NewRequest(method, "/api/computers/"+f.computerID+path, reader)
	request.Header.Set("X-Authenticated-User", "owner-restore")
	request.Header.Set("X-Authenticated-Computer", f.computerID)
	response := httptest.NewRecorder()
	f.handler.HandleComputersRouter(response, request)
	out := map[string]any{}
	if response.Body.Len() > 0 {
		if err := json.Unmarshal(response.Body.Bytes(), &out); err != nil {
			f.t.Fatalf("%s %s: answer is not JSON: %v (%s)", method, path, err, response.Body.String())
		}
	}
	return response.Code, out
}

func (f *restoreActFixture) restoreBody(key string) map[string]any {
	body := map[string]any{
		"checkpoint":     f.checkpoint,
		"operand_scopes": []string{selfdevprotocol.RestoreScopeVMLocal, selfdevprotocol.RestoreScopeFrontend},
	}
	if key != "" {
		body["idempotency_key"] = key
	}
	return body
}

func (f *restoreActFixture) post(key string) (int, map[string]any) {
	f.t.Helper()
	return f.do(http.MethodPost, "/lifecycle/restore", f.restoreBody(key))
}

func (f *restoreActFixture) status(act string) (int, map[string]any) {
	f.t.Helper()
	path := "/lifecycle/restore-status"
	if act != "" {
		path += "?act=" + act
	}
	return f.do(http.MethodGet, path, nil)
}

func restoreActTerminal(state string) bool {
	return state == "succeeded" || state == "failed" || state == "interrupted"
}

func (f *restoreActFixture) waitTerminal(act string) map[string]any {
	f.t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		code, view := f.status(act)
		if code == http.StatusOK {
			if state, _ := view["state"].(string); restoreActTerminal(state) {
				return view
			}
		} else if time.Now().After(deadline) {
			f.t.Fatalf("status %s answered %d %v", act, code, view)
		}
		if time.Now().After(deadline) {
			f.t.Fatalf("act %s did not end: %v", act, view)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (f *restoreActFixture) waitState(act, want string) map[string]any {
	f.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		code, view := f.status(act)
		if code == http.StatusOK {
			if view["state"] == want {
				return view
			}
		}
		if time.Now().After(deadline) {
			f.t.Fatalf("act %s never reached %s: %d %v", act, want, code, view)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// actID is the act an acceptance answer names; a test whose answer names none
// fails there, not in a panic that hides the tests after it.
func (f *restoreActFixture) actID(view map[string]any) string {
	f.t.Helper()
	id, _ := view["act_id"].(string)
	if id == "" {
		f.t.Fatalf("the answer names no act: %v", view)
	}
	return id
}

func (f *restoreActFixture) intents() int {
	n := 0
	for _, record := range f.cas.events {
		if record.Request.Event.EventKind == computerevent.EventRestoreRequested {
			n++
		}
	}
	return n
}

func (f *restoreActFixture) served() string {
	f.t.Helper()
	raw, err := os.ReadFile(filepath.Join(f.updaterRoot, "current", "frontend", "index.html"))
	if err != nil {
		f.t.Fatal(err)
	}
	return string(raw)
}

// holdReplayAfter blocks the replay of a restore act after n applied events.
// reached closes when it is held; release lets it go (also at cleanup).
func holdReplayAfter(t *testing.T, n int) (reached <-chan struct{}, release func()) {
	t.Helper()
	hit, gate := make(chan struct{}), make(chan struct{})
	var once, free sync.Once
	restoreActApplyHook = func(applied int) {
		if applied == n {
			once.Do(func() { close(hit) })
			<-gate
		}
	}
	release = func() { free.Do(func() { close(gate) }) }
	t.Cleanup(func() { release(); restoreActApplyHook = nil })
	return hit, release
}

func waitChanClosed(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(30 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func num(t *testing.T, view map[string]any, key string) float64 {
	t.Helper()
	v, ok := view[key].(float64)
	if !ok {
		t.Fatalf("%s is %T %v in %v", key, view[key], view[key], view)
	}
	return v
}

// F1
func TestRestoreActAnswersAtOnceWhileTheReplayRuns(t *testing.T) {
	f := newRestoreActFixture(t, "computer-act-accept", 5)
	reached, release := holdReplayAfter(t, 2)
	answered := make(chan struct{})
	var code int
	var view map[string]any
	go func() {
		code, view = f.post("")
		close(answered)
	}()
	waitChanClosed(t, answered, "the acceptance answer (the replay is held, so an answer that waits for it never comes)")
	if code != http.StatusAccepted {
		t.Fatalf("restore answered %d %v, want 202", code, view)
	}
	act, _ := view["act_id"].(string)
	if act == "" || view["state"] != "accepted" {
		t.Fatalf("acceptance = %v, want an act id and state accepted", view)
	}
	if url, _ := view["status_url"].(string); !strings.Contains(url, "/lifecycle/restore-status") || !strings.Contains(url, act) {
		t.Fatalf("status_url = %q", url)
	}
	estimate, _ := view["estimate"].(map[string]any)
	if estimate == nil || num(t, estimate, "tail_events") <= 0 || num(t, estimate, "estimated_seconds") <= 0 || estimate["replay_rate_source"] != "assumed" {
		t.Fatalf("estimate = %v, want a tail, seconds and the assumed rate named", estimate)
	}
	waitChanClosed(t, reached, "the replay to reach the hold")
	release()
	if end := f.waitTerminal(act); end["state"] != "succeeded" {
		t.Fatalf("act ended %v", end)
	}
}

// F2
func TestRestoreActStatusReportsProgress(t *testing.T) {
	f := newRestoreActFixture(t, "computer-act-progress", 5)
	reached, release := holdReplayAfter(t, 2)
	_, accepted := f.post("")
	act := f.actID(accepted)
	waitChanClosed(t, reached, "the replay to reach the hold")
	code, view := f.status(act)
	if code != http.StatusOK || view["state"] != "replaying" {
		t.Fatalf("status = %d %v, want 200 replaying", code, view)
	}
	if num(t, view, "tail_events_total") != 5 || num(t, view, "tail_events_applied") != 2 {
		t.Fatalf("progress = %v of %v, want 2 of 5", view["tail_events_applied"], view["tail_events_total"])
	}
	if num(t, view, "replay_rate") <= 0 || view["replay_rate_source"] != "measured" {
		t.Fatalf("rate = %v (%v), want one measured in this act", view["replay_rate"], view["replay_rate_source"])
	}
	if num(t, view, "estimated_seconds_remaining") <= 0 {
		t.Fatalf("estimated_seconds_remaining = %v, want a positive figure with 3 events left", view["estimated_seconds_remaining"])
	}
	for _, key := range []string{"accepted_at", "started_at", "updated_at"} {
		if s, _ := view[key].(string); s == "" {
			t.Fatalf("%s missing in %v", key, view)
		}
	}
	// No act id: the running act.
	if code, running := f.status(""); code != http.StatusOK || running["act_id"] != act {
		t.Fatalf("status with no act = %d %v, want the running act", code, running)
	}
	release()
	f.waitTerminal(act)
}

// F3
func TestRestoreActSucceedsWithTheReport(t *testing.T) {
	f := newRestoreActFixture(t, "computer-act-success", 3)
	code, accepted := f.post("")
	if code != http.StatusAccepted {
		t.Fatalf("restore answered %d %v", code, accepted)
	}
	end := f.waitTerminal(f.actID(accepted))
	if end["state"] != "succeeded" {
		t.Fatalf("act = %v", end)
	}
	result, _ := end["result"].(map[string]any)
	if result == nil || result["witness_matched"] != true || result["frontend_restaged"] != true || result["original_denied"] != true {
		t.Fatalf("result = %v, want the report the synchronous route returned", result)
	}
	if result["checkpoint_digest"] != f.checkpoint.Digest || result["tail_events_applied"] != float64(3) {
		t.Fatalf("result = %v", result)
	}
	if got := f.served(); got != "<html>checkpoint</html>" {
		t.Fatalf("served SPA = %q", got)
	}
	if end["finished_at"] == "" || end["finished_at"] == nil {
		t.Fatalf("finished_at missing in %v", end)
	}
	if head, err := f.rt.store.Head(context.Background(), f.computerID); err != nil || head == nil || head.CanonicalEventHead != f.checkpoint.Request.AcceptedEventHead {
		t.Fatalf("restored head = %+v %v, want the checkpoint's", head, err)
	}
}

// F4
func TestRestoreActFailsByNameAndLeavesLiveState(t *testing.T) {
	f := newRestoreActFixture(t, "computer-act-refused", 3)
	altered := map[string]string{}
	for table := range f.checkpoint.Request.VMLocalContentWitness.Tables {
		altered[table] = strings.Repeat("0", 64)
		break
	}
	for table, digest := range f.checkpoint.Request.VMLocalContentWitness.Tables {
		if _, done := altered[table]; !done {
			altered[table] = digest
		}
	}
	w := f.checkpoint.Request.VMLocalContentWitness
	witness, err := selfdevprotocol.WitnessFromDigestsCanonical(computerversion.StateDigests{Database: w.Database, SchemaHash: w.SchemaHash, Tables: altered}, w.EventIndexTrimBoundary)
	if err != nil {
		t.Fatal(err)
	}
	f.checkpoint = rematerializeTestCheckpoint(t, f.computerID, witness, f.targetDigest, f.checkpoint.Request.FrontendIdentity, f.checkpoint.Request.AcceptedEventHead)
	code, accepted := f.post("")
	if code != http.StatusAccepted {
		t.Fatalf("a witness that does not match is found in the background; answered %d %v", code, accepted)
	}
	end := f.waitTerminal(f.actID(accepted))
	refusal, _ := end["refusal"].(map[string]any)
	if end["state"] != "failed" || refusal == nil || refusal["name"] != "witness_mismatch" || refusal["http_status"] != float64(http.StatusConflict) {
		t.Fatalf("act = %v, want failed naming witness_mismatch with status 409", end)
	}
	if end["result"] != nil {
		t.Fatalf("a failed act carries a result: %v", end["result"])
	}
	if f.rt.store == nil {
		t.Fatal("the refusal closed the live store")
	}
	if got := f.served(); got != "<html>live</html>" {
		t.Fatalf("a refused restore left the SPA %q", got)
	}
	if _, err := os.Stat(choirstore.StateDBPath(f.storePath)); err != nil {
		t.Fatalf("the live state file moved: %v", err)
	}
	if staged, _ := filepath.Glob(filepath.Join(filepath.Dir(f.storePath), "rematerialize-staging-*")); len(staged) != 0 {
		t.Fatalf("a failed restore left its staged store behind: %v", staged)
	}
}

// F5
func TestRestoreActOneAtATime(t *testing.T) {
	f := newRestoreActFixture(t, "computer-act-one", 4)
	reached, release := holdReplayAfter(t, 1)
	code, first := f.post("key-a")
	if code != http.StatusAccepted {
		t.Fatalf("first restore answered %d %v", code, first)
	}
	act := f.actID(first)
	waitChanClosed(t, reached, "the replay to reach the hold")
	code, other := f.post("key-b")
	if code != http.StatusConflict || other["running_act"] != act {
		t.Fatalf("a second request answered %d %v, want 409 naming %s", code, other, act)
	}
	if url, _ := other["status_url"].(string); !strings.Contains(url, act) {
		t.Fatalf("the refusal does not give the running act's status URL: %v", other)
	}
	code, same := f.post("key-a")
	if code != http.StatusAccepted || same["act_id"] != act {
		t.Fatalf("the same key answered %d %v, want the running act %s", code, same, act)
	}
	if n := f.intents(); n != 1 {
		t.Fatalf("%d restore intents on the tape, want 1", n)
	}
	release()
	f.waitTerminal(act)
	records, _ := filepath.Glob(filepath.Join(filepath.Dir(f.storePath), "restore-acts", "*.json"))
	if len(records) != 1 {
		t.Fatalf("act records = %v, want one", records)
	}
	// An act that has ended no longer blocks another.
	code, again := f.post("key-b")
	if code != http.StatusAccepted || again["act_id"] == act {
		t.Fatalf("a restore after the end answered %d %v, want a new act", code, again)
	}
	// The second act runs against the fixture's directory: it ends before the
	// test does.
	f.waitTerminal(f.actID(again))
}

// F6
func TestRestoreActEstimateNamesItsRate(t *testing.T) {
	got := estimateRestore(5400, 0, false)
	if got.ReplayRateSource != "assumed" || got.ReplayRate != snapshotAssumedReplayRate || got.EstimatedSeconds < 47 || got.EstimatedSeconds > 49 {
		t.Fatalf("assumed estimate = %+v, want 5400 events at 113/s (47.8s)", got)
	}
	got = estimateRestore(314364, 113, true)
	if got.ReplayRateSource != "measured" || got.EstimatedSeconds < 2700 || got.EstimatedSeconds > 2900 {
		t.Fatalf("measured estimate = %+v, want the paper's 46 minutes", got)
	}
	// A long tail is not refused.
	f := newRestoreActFixture(t, "computer-act-estimate", 2)
	if code, view := f.post(""); code != http.StatusAccepted {
		t.Fatalf("answered %d %v", code, view)
	}
}

// F7
func TestRestoreActRefusesBelowDiskFloorBeforeTheIntent(t *testing.T) {
	f := newRestoreActFixture(t, "computer-act-disk", 2)
	restoreActFreeBytes = func(string) (uint64, error) { return 1 << 20, nil }
	t.Cleanup(func() { restoreActFreeBytes = freeBytesOnVolume })
	code, view := f.post("")
	refusal, _ := view["refusal"].(map[string]any)
	if code != http.StatusInsufficientStorage || refusal == nil || refusal["name"] != "insufficient_disk" {
		t.Fatalf("answered %d %v, want 507 naming insufficient_disk", code, view)
	}
	if n := f.intents(); n != 0 {
		t.Fatalf("the refusal appended %d restore intents", n)
	}
	if records, _ := filepath.Glob(filepath.Join(filepath.Dir(f.storePath), "restore-acts", "*.json")); len(records) != 0 {
		t.Fatalf("the refusal left act records: %v", records)
	}
}

// F8
func TestRestoreActKeepsUpFrontRefusals(t *testing.T) {
	f := newRestoreActFixture(t, "computer-act-upfront", 2)
	body := f.restoreBody("")
	body["operand_scopes"] = []string{selfdevprotocol.RestoreScopeVMLocal, selfdevprotocol.RestoreScopeFrontend, selfdevprotocol.RestoreScopePlatform}
	if code, view := f.do(http.MethodPost, "/lifecycle/restore", body); code != http.StatusBadRequest {
		t.Fatalf("a platform operand answered %d %v, want 400", code, view)
	}
	f.checkpoint.Request.VMLocalContentWitness.WitnessVersion = 2
	if code, view := f.post(""); code != http.StatusConflict && code != http.StatusBadRequest {
		t.Fatalf("an unjudgeable witness answered %d %v, want a refusal at once", code, view)
	}
	if n := f.intents(); n != 0 {
		t.Fatalf("an up-front refusal appended %d restore intents", n)
	}
	if code, _ := f.status("no-such-act"); code != http.StatusNotFound {
		t.Fatalf("status of an unknown act = %d, want 404", code)
	}
}

// F13
func TestRestoreActDoesNotHoldTheReleaseLockDuringTheReplay(t *testing.T) {
	f := newRestoreActFixture(t, "computer-act-lock", 4)
	reached, release := holdReplayAfter(t, 1)
	_, accepted := f.post("")
	waitChanClosed(t, reached, "the replay to reach the hold")
	if !f.rt.currentReleaseMu.TryLock() {
		t.Fatal("the release lock is held across the replay: a checkpoint bind would wait for the whole restore")
	}
	f.rt.currentReleaseMu.Unlock()
	release()
	f.waitTerminal(f.actID(accepted))
}

// F12
func TestPlannedRestartIsRefusedMidRestore(t *testing.T) {
	f := newRestoreActFixture(t, "computer-act-planned", 4)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(f.storePath) })
	_ = ClearPlannedRestartMarker(f.storePath)
	reached, release := holdReplayAfter(t, 1)
	_, accepted := f.post("")
	waitChanClosed(t, reached, "the replay to reach the hold")
	for _, target := range []string{"/internal/runtime/planned-restart", "/internal/runtime/planned-restart?allow_busy=1"} {
		code, body := postPlannedRestartURL(t, &APIHandler{rt: f.rt}, target, `{"target":"761d94c5"}`)
		if code != http.StatusConflict || !strings.Contains(body, `"restore_in_flight":true`) {
			t.Fatalf("%s mid-restore: %d %s, want 409 naming restore_in_flight", target, code, body)
		}
	}
	if _, err := os.Stat(PlannedRestartMarkerPath(f.storePath)); err == nil {
		t.Fatal("a refused planned restart wrote its marker")
	}
	release()
	f.waitTerminal(f.actID(accepted))
}

func TestHealthReportsRestoreInFlight(t *testing.T) {
	f := newRestoreActFixture(t, "computer-act-health", 4)
	read := func() (bool, bool) {
		rec, _ := serveHealth(t, &APIHandler{rt: f.rt})
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("/health: %v %s", err, rec.Body.String())
		}
		v, present := body["restore_in_flight"].(bool)
		return v, present
	}
	if v, present := read(); present && v {
		t.Fatal("/health reports a restore before any started")
	}
	reached, release := holdReplayAfter(t, 1)
	_, accepted := f.post("")
	waitChanClosed(t, reached, "the replay to reach the hold")
	if v, _ := read(); !v {
		t.Fatal("/health does not report restore_in_flight mid-restore: vmctl would hibernate or restart the computer")
	}
	release()
	f.waitTerminal(f.actID(accepted))
	if v, _ := read(); v {
		t.Fatal("/health still reports restore_in_flight after the act ended")
	}
}

// F14: a probe (a checkpoint bind runs one every hour) and a restore both
// observe a replay. The restore's observation is its own, not the appender's:
// a probe that starts or ends mid-restore must not clear it.
func TestReplayProbeDuringARestoreDoesNotTouchItsObservation(t *testing.T) {
	f := newRestoreActFixture(t, "computer-act-probe", 5)
	reached, release := holdReplayAfter(t, 2)
	_, accepted := f.post("")
	waitChanClosed(t, reached, "the replay to reach the hold")
	if _, err := f.rt.ReplayCompleteness(context.Background(), f.computerID); err != nil {
		t.Fatalf("a probe during a restore: %v", err)
	}
	release()
	end := f.waitTerminal(f.actID(accepted))
	if end["state"] != "succeeded" {
		t.Fatalf("the probe broke the restore's observation: %v", end)
	}
	result, _ := end["result"].(map[string]any)
	if result == nil || result["tail_events_applied"] != float64(5) {
		t.Fatalf("result = %v, want 5 tail events observed", result)
	}
}

// F15
func TestRestoreActRecords(t *testing.T) {
	f := newRestoreActFixture(t, "computer-act-records", 1)
	dir := filepath.Join(filepath.Dir(f.storePath), "restore-acts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-48 * time.Hour)
	for i := 0; i < 30; i++ {
		stamp := old.Add(time.Duration(i) * time.Minute).Format(time.RFC3339Nano)
		record := map[string]any{"schema_version": 1, "act_id": fmt.Sprintf("ra-old-%02d", i), "computer_id": f.computerID, "state": "succeeded", "accepted_at": stamp, "updated_at": stamp, "finished_at": stamp}
		raw, _ := json.Marshal(record)
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("ra-old-%02d.json", i)), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	foreign, _ := json.Marshal(map[string]any{"schema_version": 1, "act_id": "ra-foreign", "computer_id": "computer-someone-else", "state": "succeeded"})
	if err := os.WriteFile(filepath.Join(dir, "ra-foreign.json"), foreign, 0o600); err != nil {
		t.Fatal(err)
	}
	if code, view := f.status("ra-old-29"); code != http.StatusOK || view["state"] != "succeeded" {
		t.Fatalf("an old act = %d %v", code, view)
	}
	if code, _ := f.status("ra-foreign"); code != http.StatusNotFound {
		t.Fatalf("another computer's record answered %d, want 404", code)
	}
	if code, _ := f.status("../../etc/passwd"); code != http.StatusNotFound && code != http.StatusBadRequest {
		t.Fatalf("a path as an act id answered %d", code)
	}
	_, accepted := f.post("")
	f.waitTerminal(f.actID(accepted))
	records, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(records) > 21 {
		t.Fatalf("%d act records kept, want the newest 20 (and the foreign one is not ours to delete)", len(records))
	}
	if _, err := os.Stat(filepath.Join(dir, "ra-foreign.json")); err != nil {
		t.Fatalf("another computer's record was pruned: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ra-old-00.json")); err == nil {
		t.Fatal("the oldest record was not pruned")
	}
}

// F16: the routes are on the production router and the act state is usable at
// its zero value: a runtime built by New (not a literal) with no event
// appender refuses a rematerialize by name, as an accepted-act refusal, and
// has no act to report.
func TestRestoreActWiring(t *testing.T) {
	rt, _ := testRuntime(t)
	rt.cfg.ComputerID = "computer-act-wiring"
	handler := NewAPIHandler(rt)
	serve := func(method, path string) (int, map[string]any) {
		request := httptest.NewRequest(method, "/api/computers/computer-act-wiring"+path, strings.NewReader("{}"))
		request.Header.Set("X-Authenticated-User", "owner-restore")
		request.Header.Set("X-Authenticated-Computer", "computer-act-wiring")
		response := httptest.NewRecorder()
		handler.HandleComputersRouter(response, request)
		out := map[string]any{}
		_ = json.Unmarshal(response.Body.Bytes(), &out)
		return response.Code, out
	}
	if code, body := serve(http.MethodGet, "/lifecycle/restore-status"); code != http.StatusNotFound {
		t.Fatalf("status with no act = %d %v, want 404", code, body)
	}
	code, body := serve(http.MethodPost, "/lifecycle/rematerialize-from-tape")
	refusal, _ := body["refusal"].(map[string]any)
	if code != http.StatusServiceUnavailable || refusal == nil || refusal["name"] != "unavailable" {
		t.Fatalf("rematerialize on a runtime with no appender = %d %v, want 503 with the refusal named", code, body)
	}
	// The observer the replay reports to is the one the exactly-once receipt reads.
	var _ computerevent.ReplayObserver = (*restoreReplayObserver)(nil)
}

// serveRestoreToEnd posts a restore or rematerialize request as the product
// does, and when the answer is an accepted act waits for its end and answers
// the way the synchronous route used to: 200 and the report, or the refusal's
// status and its message. Tests that care about the outcome of a restore, not
// about how it is waited for, use it.
func serveRestoreToEnd(t *testing.T, rt *Runtime, computerID, route string, body []byte) (int, []byte) {
	t.Helper()
	handler := NewAPIHandler(rt)
	serve := func(method, path string, payload []byte) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, "/api/computers/"+computerID+path, bytes.NewReader(payload))
		request.Header.Set("X-Authenticated-User", "owner-restore")
		request.Header.Set("X-Authenticated-Computer", computerID)
		response := httptest.NewRecorder()
		handler.HandleComputersRouter(response, request)
		return response
	}
	first := serve(http.MethodPost, route, body)
	if first.Code != http.StatusAccepted {
		return first.Code, first.Body.Bytes()
	}
	var accepted struct {
		ActID string `json:"act_id"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &accepted); err != nil || accepted.ActID == "" {
		t.Fatalf("accepted answer has no act: %s", first.Body.String())
	}
	deadline := time.Now().Add(90 * time.Second)
	for {
		response := serve(http.MethodGet, "/lifecycle/restore-status?act="+accepted.ActID, nil)
		var view struct {
			State   string          `json:"state"`
			Result  json.RawMessage `json:"result"`
			Refusal struct {
				HTTPStatus int    `json:"http_status"`
				Message    string `json:"message"`
			} `json:"refusal"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil {
			t.Fatalf("status answer: %v %s", err, response.Body.String())
		}
		switch view.State {
		case "succeeded":
			return http.StatusOK, view.Result
		case "failed":
			out, _ := json.Marshal(map[string]string{"error": view.Refusal.Message})
			return view.Refusal.HTTPStatus, out
		case "interrupted":
			return http.StatusInternalServerError, response.Body.Bytes()
		}
		if time.Now().After(deadline) {
			t.Fatalf("act %s did not end: %s", accepted.ActID, response.Body.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// drainRestoreActs waits, bounded, for the runtime's running restore act to
// let go of its slot.
func drainRestoreActs(rt *Runtime) {
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		rt.restoreActs.mu.Lock()
		running := rt.restoreActs.running
		rt.restoreActs.mu.Unlock()
		if running == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}
