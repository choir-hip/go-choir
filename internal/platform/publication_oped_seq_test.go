package platform

// Review round 2 on the feed sequence (B4, N7, N8, N13): the sequence is
// seeded from storage at start, so a restart with the clock behind cannot
// issue a number below a consumer's cursor; a cursor that is not ours means
// "from the start" and says so; a notice with no sequence gets one and is
// delivered; an undecodable row is a named skip, not a silent drop.

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

func restartedService(t *testing.T, svc *Service, clock func() int64) *Service {
	t.Helper()
	root := t.TempDir()
	fresh := NewService(svc.store, filepath.Join(root, "artifacts"), filepath.Join(root, "signing.key"))
	fresh.noticeClock = clock
	return fresh
}

func TestNoticeSequenceIsSeededFromStorageAtRestart(t *testing.T) {
	svc, _ := newOpinionService(t)
	at := time.Now().UTC().Truncate(time.Second)
	for i := 0; i < 3; i++ {
		putTestNotice(t, svc, "victim", i, at)
	}
	feed, err := svc.ListNoticesForOwner(context.Background(), "victim", "", 100)
	if err != nil || len(feed.Items) != 3 {
		t.Fatalf("feed = %+v %v", feed, err)
	}
	oldCursor := feed.NextCursor
	// A restart with the wall clock an hour behind what was already issued.
	behind := func() int64 { return time.Now().Add(-time.Hour).UnixNano() }
	fresh := restartedService(t, svc, behind)
	id := putTestNotice(t, fresh, "victim", 99, at)
	next, err := fresh.ListNoticesForOwner(context.Background(), "victim", oldCursor, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Items) != 1 || next.Items[0].Notice.EventID != id {
		t.Fatalf("a consumer at the old cursor got %+v; the new row sat below its cursor", next.Items)
	}
}

func TestMalformedCursorMeansFromTheStartAndSaysSo(t *testing.T) {
	svc, _ := newOpinionService(t)
	putTestNotice(t, svc, "victim", 1, time.Now().UTC())
	feed, err := svc.ListNoticesForOwner(context.Background(), "victim", "2026-11-10T00:00:00Z", 100)
	if err != nil {
		t.Fatalf("an unparseable cursor wedged the feed: %v", err)
	}
	if !feed.CursorReset || len(feed.Items) != 1 {
		t.Fatalf("feed = %+v, want the start of the feed with cursor_reset", feed)
	}
	if ok, _ := svc.ListNoticesForOwner(context.Background(), "victim", feed.NextCursor, 100); ok.CursorReset {
		t.Fatal("a good cursor was reported reset")
	}
}

func TestNoticeWithoutASequenceIsGivenOneAndDelivered(t *testing.T) {
	svc, _ := newOpinionService(t)
	at := time.Now().UTC()
	putTestNotice(t, svc, "victim", 1, at)
	feed, _ := svc.ListNoticesForOwner(context.Background(), "victim", "", 100)
	cursor := feed.NextCursor
	// A row written without a sequence (an older writer).
	body := putTestNotice(t, svc, "victim", 2, at)
	_ = body
	id, _ := objectgraph.BuildCanonicalID("choir.changelog_notice", "victim", objectgraph.StableSuffixFromKey(body))
	obj, err := svc.ogStore().GetObject(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	obj = withNoticeSeq(obj, 0)
	if err := svc.ogStore().PutObject(context.Background(), obj); err != nil {
		t.Fatal(err)
	}
	fresh := restartedService(t, svc, nil)
	next, err := fresh.ListNoticesForOwner(context.Background(), "victim", cursor, 100)
	if err != nil || len(next.Items) != 1 || next.Items[0].Notice.EventID != body {
		t.Fatalf("seq-less notice: %+v %v", next.Items, err)
	}
}

func TestUndecodableNoticeRowIsANamedSkip(t *testing.T) {
	svc, _ := newOpinionService(t)
	at := time.Now().UTC()
	putTestNotice(t, svc, "victim", 1, at)
	svc.writeMu.Lock()
	seq := svc.nextNoticeSeq()
	svc.writeMu.Unlock()
	id, _ := objectgraph.BuildCanonicalID("choir.changelog_notice", "victim", objectgraph.StableSuffixFromKey("garbage"))
	bad := objectgraph.Object{CanonicalID: id, ObjectKind: "choir.changelog_notice", OwnerID: "victim",
		Metadata: mustJSONRaw(map[string]any{"seq": seq, "notice": "not an object"}), CreatedAt: at, UpdatedAt: at}
	bad.ContentHash = objectgraph.ContentHash("choir.changelog_notice", nil, bad.Metadata)
	if err := svc.ogStore().PutObject(context.Background(), bad); err != nil {
		t.Fatal(err)
	}
	feed, err := svc.ListNoticesForOwner(context.Background(), "victim", "", 100)
	if err != nil || len(feed.Items) != 1 || len(feed.Skipped) != 1 || feed.Skipped[0].ID != id || feed.Skipped[0].Reason == "" {
		t.Fatalf("feed = %+v %v", feed, err)
	}
	if of, _ := svc.ListOpedsForOwner(context.Background(), "victim", "", 100); len(of.Skipped) != 1 {
		t.Fatalf("op-ed feed skipped = %+v", of.Skipped)
	}
}
