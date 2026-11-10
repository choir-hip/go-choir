package platform

// Review B2: the feed cursor never wedges. More than `limit` notices created in
// the same second are all delivered across pages, none twice, and the cursor
// advances on every page; a later write in the same second is not skipped.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func putTestNotice(t *testing.T, svc *Service, owner string, i int, at time.Time) string {
	t.Helper()
	svc.writeMu.Lock()
	seq := svc.nextNoticeSeq()
	svc.writeMu.Unlock()
	body := types.CommitmentChangelog{
		EventID: types.ChangelogEventID(types.ChangelogParentUsed, owner, fmt.Sprint(i)), Kind: types.ChangelogParentUsed, Level: types.ChangelogMajor,
		Relation: types.ChangelogRelationTakenInto, OccurredAt: at.Format(time.RFC3339), Summary: "x", Origin: types.ChangelogOriginCorpusd,
		Subject: types.ChangelogSubject{PublicationVersionID: fmt.Sprintf("child-%d", i)},
	}
	obj, err := noticeObjectFor(owner, body, noticeChildRef{PublicationVersionID: fmt.Sprintf("child-%d", i)}, nil, at)
	if err != nil {
		t.Fatal(err)
	}
	obj = withNoticeSeq(obj, seq)
	if err := svc.ogStore().PutBatch(context.Background(), objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		t.Fatal(err)
	}
	return body.EventID
}

func TestNoticeCursorDeliversEveryRowOfOneSecond(t *testing.T) {
	svc, _ := newOpinionService(t)
	at := time.Now().UTC().Truncate(time.Second)
	want := map[string]bool{}
	const n = 250
	for i := 0; i < n; i++ {
		want[putTestNotice(t, svc, "victim", i, at)] = true
	}
	got := map[string]int{}
	cursor := ""
	for page := 0; page < 20; page++ {
		feed, err := svc.ListNoticesForOwner(context.Background(), "victim", cursor, 100)
		if err != nil {
			t.Fatal(err)
		}
		if len(feed.Items) == 0 {
			break
		}
		if feed.NextCursor == cursor {
			t.Fatalf("page %d did not advance the cursor", page)
		}
		for _, it := range feed.Items {
			got[it.Notice.EventID]++
		}
		cursor = feed.NextCursor
	}
	if len(got) != n {
		t.Fatalf("delivered %d of %d notices", len(got), n)
	}
	for id, c := range got {
		if c != 1 || !want[id] {
			t.Fatalf("notice %s delivered %d times (known=%v)", id, c, want[id])
		}
	}
	// A notice written later in the same second, with an id that sorts first, is not skipped.
	late := putTestNotice(t, svc, "victim", -1, at)
	feed, err := svc.ListNoticesForOwner(context.Background(), "victim", cursor, 100)
	if err != nil || len(feed.Items) != 1 || feed.Items[0].Notice.EventID != late {
		t.Fatalf("late notice: %+v %v", feed.Items, err)
	}
	// And the op-ed feed, which reads the same rows, agrees.
	if of, err := svc.ListOpedsForOwner(context.Background(), "victim", "", 100); err != nil || len(of.Items) != 0 || of.NextCursor == "" {
		t.Fatalf("op-ed feed over non-op-ed rows: %+v %v", of, err)
	}
}
