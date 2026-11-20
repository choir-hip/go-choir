package textureowner

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/agentcore"
	_ "modernc.org/sqlite"
)

// An armed digest the dispatcher's activation discarded is the failure: the
// handler that noted the change deferred, so its emission buffer died with
// the activation while the scope kept the nonce armed (for the stale-arm
// time), and every later commit coalesced into a digest that did not exist.
// The note sends outside the activation's buffer, so the digest is durable
// whatever the handler does after.
//
// The schedule stub routes as actorruntime.Adapter.dispatch does: into the
// activation's buffer when the context carries one, else out at once.
func TestTextureRootArmedDigestSurvivesAHandlerThatDefers(t *testing.T) {
	rh := newRootHarness(t)
	var mu sync.Mutex
	var sent, buffered int
	rh.core.SetScheduleActor(func(ctx context.Context, _, _, _, kind, _, _, _ string, _ time.Time) error {
		if kind != agentcore.TextureRootDigestKind {
			return nil
		}
		mu.Lock()
		defer mu.Unlock()
		if buf := actor.EmissionsFromCtx(ctx); buf != nil {
			buffered++
			return nil
		}
		sent++
		return nil
	})
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "actor.db")+"?_pragma=busy_timeout(60000)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	log, err := actor.NewSQLiteLog(db)
	if err != nil {
		t.Fatal(err)
	}
	called := make(chan struct{}, 1)
	handler := actor.HandlerFunc(func(ctx context.Context, _ string, _ actor.Update, memory []byte) ([]byte, error) {
		rh.h.NoteTextureRootChange(ctx, rootOwner, rootComputer, "doc-deferred")
		select {
		case called <- struct{}{}:
		default:
		}
		return memory, actor.ErrDeferUnprocessed
	})
	d := actor.NewDispatcher(log, handler, actor.DispatcherOptions{PollInterval: 15 * time.Millisecond})
	if _, err := log.Append(t.Context(), actor.Update{UpdateID: "u1", ToAgentID: "agent-x", Kind: "k", Content: "c", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	go d.Run(t.Context())
	select {
	case <-called:
	case <-time.After(10 * time.Second):
		t.Fatal("the handler never ran")
	}
	d.Stop()

	mu.Lock()
	defer mu.Unlock()
	rh.h.textureRootMu.Lock()
	armed := !rh.h.textureRootArmed[rootOwner+"\x00"+rootComputer].IsZero()
	rh.h.textureRootMu.Unlock()
	if !armed {
		t.Fatal("the note armed no digest: the test proves nothing")
	}
	if sent < 1 {
		t.Fatalf("the scope holds an armed digest but %d digests were sent (%d went into the discarded activation buffer)", sent, buffered)
	}
}

// A digest never delivered (dead-lettered) stops holding back the next one
// after the stale-arm time: a note inside it coalesces, one after arms again.
// Both digests are harmless to deliver: each recomputes from the store.
func TestTextureRootStaleArmIsReplaced(t *testing.T) {
	rh := newRootHarness(t)
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, "doc-b")
	rh.advance(textureRootBatchInterval + textureRootStaleArm - time.Second)
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, "doc-c")
	if n := len(rh.digests()); n != 1 {
		t.Fatalf("a note inside the stale-arm time armed %d digests, want 1", n)
	}
	rh.advance(2 * time.Second)
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, "doc-d")
	digests := rh.digests()
	if len(digests) != 2 || digests[0].content == digests[1].content {
		t.Fatalf("a note past the stale-arm time armed %d digests (%+v), want a second, different one", len(digests)-1, digests)
	}
}

// A digest delivered with a context that fails the store reads (a graceful
// stop mid-handler, a transient error) returns an error, which the actor
// handler turns into a deferral; the dispatcher retries the SAME digest, and
// it still runs, because it cleared nothing durable: the watermark moves only
// with a committed root revision.
func TestTextureRootADigestRetriedAfterAFailureStillRuns(t *testing.T) {
	rh := newRootHarness(t)
	_, runB := rh.createDoc("retry-b", "Harbor")
	rh.write(rh.h, &runB, "harbor", "The harbor is open.", "")
	digest := rh.digests()[0]
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := rh.h.HandleTextureRootDigest(cancelled, rootOwner, rootComputer, digest.content); err == nil {
		t.Fatal("a digest handled on a cancelled context returned no error to defer on")
	}
	rh.deliver(digest)
	marker, ok := rh.marker()
	if !ok {
		t.Fatal("the retried digest made no master")
	}
	if _, live := rh.activeRun(marker.DocumentID); !live || rh.rootRuns(marker.DocumentID) != 1 {
		t.Fatalf("the retried digest started %d root runs, want 1", rh.rootRuns(marker.DocumentID))
	}
}

// A root run that does not commit (failed, cancelled, passivated) leaves the
// watermark where it was: the next digest covers the same documents.
func TestTextureRootAFailedRootRunLeavesTheWatermark(t *testing.T) {
	rh, master, root, b, c := rootStarted(t)
	passivateRun(t, rh.s, root)
	rh.advance(3 * time.Hour)
	rh.h.NoteTextureRootChange(t.Context(), rootOwner, rootComputer, b)
	rh.deliver(rh.digests()[len(rh.digests())-1])
	next, ok := rh.activeRun(master)
	if !ok || next.RunID == root.RunID {
		t.Fatalf("the next digest started no new root run: %+v %v", next, ok)
	}
	for _, doc := range []string{b, c} {
		if !strings.Contains(next.Prompt, "refs: doc="+doc) {
			t.Fatalf("the next digest's run lacks %s, which the failed run never digested:\n%s", doc, next.Prompt)
		}
	}
}

// K planned restarts that resume K digests, with nothing changed since the
// root's last commit, make no run at all.
func TestTextureRootRestartsWithNothingNewMakeNoRun(t *testing.T) {
	rh, master, root, _, _ := rootStarted(t)
	rh.write(rh.h, &root, "master", "Harbor: open. Bridge: closed.", "")
	passivateRun(t, rh.s, root)
	runs := rh.rootRuns(master)
	for i := 0; i < 4; i++ {
		rh.h.textureRootMu.Lock()
		rh.h.textureRootArmed = nil
		rh.h.textureRootMu.Unlock()
		rh.deliver(rootWake{content: fmt.Sprintf(`{"v":1,"due":"boot-%d"}`, i)})
	}
	if got := rh.rootRuns(master); got != runs {
		t.Fatalf("four resumed digests with nothing new made %d root runs", got-runs)
	}
}

// A crash drops the armed digest (the pre-boot digest is closed as
// interrupted), and loses nothing: a document changed after the root's last
// commit and before the crash is covered by the digest the next commit arms.
func TestTextureRootACrashLosesNothingTheNextCommitCovers(t *testing.T) {
	rh, master, root, _, _ := rootStarted(t)
	rh.write(rh.h, &root, "master", "Harbor: open. Bridge: closed.", "")
	passivateRun(t, rh.s, root)
	before, runBefore := rh.createDoc("crash-before", "Ferry")
	rh.write(rh.h, &runBefore, "ferry", "The ferry runs.", "")
	armed := len(rh.digests())
	// The crash: a new process, the pre-boot digest never delivered.
	booted := NewHandler(rh.core)
	rh.wire(booted)
	rh.advance(2 * time.Hour)
	rh.advance(time.Second)
	after, runAfter := rh.createDoc("crash-after", "Tunnel")
	rh.write(booted, &runAfter, "tunnel", "The tunnel is shut.", "")
	digests := rh.digests()
	if len(digests) != armed+1 {
		t.Fatalf("the first commit after boot armed %d digests, want 1", len(digests)-armed)
	}
	if err := booted.HandleTextureRootDigest(t.Context(), rootOwner, rootComputer, digests[len(digests)-1].content); err != nil {
		t.Fatal(err)
	}
	next, ok := rh.activeRun(master)
	if !ok {
		t.Fatal("the digest after the crash started no root run")
	}
	for _, doc := range []string{before, after} {
		if !strings.Contains(next.Prompt, "refs: doc="+doc) {
			t.Fatalf("the digest after the crash lacks %s:\n%s", doc, next.Prompt)
		}
	}
}
