package agentcore

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

type scriptedPublisher struct {
	StoryPublisher
	errs  []error
	calls int
}

func (p *scriptedPublisher) PublishVersion(context.Context, StoryVersion) error {
	err := p.errs[min(p.calls, len(p.errs)-1)]
	p.calls++
	return err
}

func withNoBackoff(t *testing.T) *[]time.Duration {
	t.Helper()
	var waits []time.Duration
	old := storyPublishSleep
	storyPublishSleep = func(d time.Duration) { waits = append(waits, d) }
	t.Cleanup(func() { storyPublishSleep = old })
	return &waits
}

func TestPublishVersionRetriesAProxyTimeoutThenSucceeds(t *testing.T) {
	waits := withNoBackoff(t)
	gateway := &platformStatusError{Path: "/p", Status: http.StatusBadGateway, Body: `{"error":"failed to load private texture revision"}`}
	p := &scriptedPublisher{errs: []error{gateway, gateway, nil}}
	attempts, err := publishVersionWithRetry(context.Background(), p, StoryVersion{DocID: "d", RevisionID: "r", Revision: 2})
	if err != nil || attempts != 3 || p.calls != 3 {
		t.Fatalf("attempts=%d calls=%d err=%v; want 3 attempts and success", attempts, p.calls, err)
	}
	if len(*waits) != 2 || (*waits)[0] != 5*time.Second || (*waits)[1] != 15*time.Second {
		t.Fatalf("backoff = %v, want 5s then 15s", *waits)
	}
}

func TestPublishVersionGivesUpAfterThreeAttempts(t *testing.T) {
	withNoBackoff(t)
	p := &scriptedPublisher{errs: []error{context.DeadlineExceeded}}
	attempts, err := publishVersionWithRetry(context.Background(), p, StoryVersion{})
	if err == nil || attempts != 3 || p.calls != 3 {
		t.Fatalf("attempts=%d calls=%d err=%v; want 3 attempts and the error", attempts, p.calls, err)
	}
}

func TestPublishVersionDoesNotRetryARefusal(t *testing.T) {
	waits := withNoBackoff(t)
	for _, err := range []error{
		&platformStatusError{Status: http.StatusConflict},
		&platformStatusError{Status: http.StatusForbidden},
		errors.New("published as v3, want v2"),
	} {
		p := &scriptedPublisher{errs: []error{err}}
		attempts, got := publishVersionWithRetry(context.Background(), p, StoryVersion{})
		if got == nil || attempts != 1 || p.calls != 1 {
			t.Fatalf("%v: attempts=%d calls=%d; want one attempt", err, attempts, p.calls)
		}
	}
	if len(*waits) != 0 {
		t.Fatalf("waited %v for a refusal", *waits)
	}
}

// firstTimesOutThenChecksItsContext is a publisher whose first attempt fails
// with a deadline and whose later attempts must be given a live context with a
// deadline, though the context the job started with has expired.
type firstTimesOutThenChecksItsContext struct {
	*fakeEntityPublisher
	t        *testing.T
	attempts int
}

func (p *firstTimesOutThenChecksItsContext) PublishVersion(ctx context.Context, _ StoryVersion) error {
	p.attempts++
	if p.attempts == 1 {
		return context.DeadlineExceeded
	}
	if err := ctx.Err(); err != nil {
		p.t.Errorf("attempt %d got a dead context: %v", p.attempts, err)
	}
	if _, ok := ctx.Deadline(); !ok {
		p.t.Errorf("attempt %d got a context with no deadline", p.attempts)
	}
	return nil
}

func expiredContext() (context.Context, context.CancelFunc) {
	return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
}

func TestPublishVersionRetryGetsALiveContextAfterTheJobContextExpired(t *testing.T) {
	withNoBackoff(t)
	ctx, cancel := expiredContext()
	defer cancel()
	p := &firstTimesOutThenChecksItsContext{fakeEntityPublisher: &fakeEntityPublisher{fakeRelationPublisher: &fakeRelationPublisher{}}, t: t}
	attempts, err := publishVersionWithRetry(ctx, p, StoryVersion{})
	if err != nil || attempts != 2 {
		t.Fatalf("attempts=%d err=%v; want a recovery on attempt 2", attempts, err)
	}
}

// A publish recovered by a retry still exports the story's entities: the
// export runs on its own context, not the job's expired one.
func TestRecoveredPublishStillExportsEntitiesAfterTheJobContextExpired(t *testing.T) {
	withNoBackoff(t)
	f := newAnalysisFixture(t)
	f.turnOn(t)
	a := f.analyse(t, agentprofile.Research, "run-retry-export", "doc-story")
	zelenskyy := a.Candidates[0]
	unpublished := &platformStatusError{Path: types.StoryEntitiesPath, Status: http.StatusConflict, Body: "not published"}
	f.epub.entityErrs = []error{unpublished}
	if _, err := f.relateEntities(t, "run-retry-export",
		yaegikernel.RelateEntity{EntityID: zelenskyy.EntityID, Role: types.EntityRoleCentral, JudgmentRef: zelenskyy.Role.ReceiptRef, QuestionID: zelenskyy.Role.QuestionID}); err != nil {
		t.Fatalf("relate: %v", err)
	}
	before := len(f.epub.entityCalls())

	ctx, cancel := expiredContext()
	defer cancel()
	p := &firstTimesOutThenChecksItsContext{fakeEntityPublisher: f.epub, t: t}
	f.rt.publishStoryVersion(ctx, p, StoryVersion{OwnerID: f.owner, ComputerID: f.computer, DocID: "doc-story", RevisionID: "rev-doc-story", Revision: 1})
	if p.attempts != 2 {
		t.Fatalf("attempts = %d, want 2", p.attempts)
	}
	if n := len(f.epub.entityCalls()) - before; n != 1 {
		t.Fatalf("entity exports after a recovered publish = %d, want 1", n)
	}
}
