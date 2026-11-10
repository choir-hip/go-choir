package agentcore

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The platform poll (autopaper contracts 2 and 3, amendment A3): a computer's
// poll of a corpusd feed, with no model call. oped_intake (the paper's, every
// minute) adopts the op-eds on its stories and runs the gate over them;
// notice_intake (every five minutes) adopts the owner's changelog notices.
//
// A poll is a standing intent; a tick is an occurrence. A restart leaves an
// active poll active and starts its loop one interval after boot, as new work.
// A crash closes only the poll in flight, and a poll that was in flight at
// two crash boots in a row is closed with that cause. Nothing here is resumed.
// Every item is committed with a deterministic record id, so a repeated or
// half-finished page mints nothing: "already committed" is done, and the
// cursor advances after the records.

const (
	opedIntakeInterval   = time.Minute
	noticeIntakeInterval = 5 * time.Minute
	pollPageLimit        = 100
	pollMaxPages         = 5
	pollTickTimeout      = 5 * time.Minute
	platformPollCrashWhy = "closed: its poll was in flight at 2 crash restarts in a row; an owner act restarts it"
)

// platformPolls is the runtime's poll state.
type platformPolls struct {
	mu      sync.Mutex
	feed    PlatformFeed
	feedSet bool
	loops   map[string]context.CancelFunc
	locks   map[string]*sync.Mutex
	wg      sync.WaitGroup
	// stopped is set by Stop: no loop starts after it. bootCancel ends the
	// boot arm's goroutine.
	stopped    bool
	bootCancel context.CancelFunc
}

func (p *platformPolls) lockFor(key string) *sync.Mutex {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.locks == nil {
		p.locks = map[string]*sync.Mutex{}
	}
	if p.locks[key] == nil {
		p.locks[key] = &sync.Mutex{}
	}
	return p.locks[key]
}

func platformPollInterval(name string) time.Duration {
	if name == store.PlatformPollOpedIntake {
		return opedIntakeInterval
	}
	return noticeIntakeInterval
}

func platformPollNames() []string {
	return []string{store.PlatformPollOpedIntake, store.PlatformPollNoticeIntake}
}

func (rt *Runtime) putPlatformPoll(ctx context.Context, poll store.PlatformPoll) error {
	return rt.store.PutPlatformPoll(withKindCustodian(ctx, "choir.platform_poll"), poll)
}

func (rt *Runtime) getPlatformPoll(ctx context.Context, owner, name string) (store.PlatformPoll, error) {
	return rt.store.GetPlatformPoll(objectgraph.WithCustodianReader(ctx, "choir.platform_poll"), owner, name)
}

// ensurePlatformPollRows creates the owner's poll rows when absent.
func (rt *Runtime) ensurePlatformPollRows(ctx context.Context, owner, computer string, names ...string) error {
	if rt == nil || rt.store == nil {
		return fmt.Errorf("platform poll: store unavailable")
	}
	if len(names) == 0 {
		names = platformPollNames()
	}
	for _, name := range names {
		if _, err := rt.getPlatformPoll(ctx, owner, name); err == nil {
			continue
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		poll := store.PlatformPoll{OwnerID: owner, ComputerID: computer, Name: name,
			IntervalSeconds: int64(platformPollInterval(name) / time.Second), State: store.PlatformPollActive}
		if err := rt.putPlatformPoll(ctx, poll); err != nil {
			return err
		}
	}
	return nil
}

// EnsurePlatformPolls makes the owner's polls exist and run. It is idempotent
// and cheap: callers (a story publish, a fork) may call it on every use.
func (rt *Runtime) EnsurePlatformPolls(ctx context.Context, owner, computer string) {
	rt.ensurePlatformPollsNamed(ctx, owner, computer, platformPollNames()...)
}

// ensurePlatformPollsNamed is EnsurePlatformPolls for the named polls only.
func (rt *Runtime) ensurePlatformPollsNamed(ctx context.Context, owner, computer string, names ...string) {
	owner, computer = strings.TrimSpace(owner), strings.TrimSpace(computer)
	if rt == nil || rt.store == nil || owner == "" || computer == "" {
		return
	}
	if rt.platformFeed() == nil {
		return
	}
	if err := rt.ensurePlatformPollRows(ctx, owner, computer, names...); err != nil {
		log.Printf("platform poll: ensure rows for %s: %v", owner, err)
		return
	}
	for _, name := range names {
		if poll, err := rt.getPlatformPoll(ctx, owner, name); err == nil && poll.State == store.PlatformPollActive {
			rt.startPlatformPollLoop(poll)
		}
	}
}

// recoverPlatformPollsAtBoot carries every poll across this boot. A poll in
// flight at the boot is closed (it is not resumed) and counted; at two in a
// row the poll is closed with that cause. With start, each active poll's loop
// starts, one interval after boot. Only polls that already exist start: a poll is
// created by use (a story publish, a fork), so only publishers poll for op-eds.
func (rt *Runtime) recoverPlatformPollsAtBoot(ctx context.Context, start bool) {
	if rt == nil || rt.store == nil {
		return
	}
	polls, err := rt.store.ListPlatformPolls(objectgraph.WithCustodianReader(ctx, "choir.platform_poll"), "")
	if err != nil {
		log.Printf("platform poll: list at boot: %v", err)
		return
	}
	// A computer that published before publishing armed the polls has no row.
	// Once, off the boot path, it reads its notice feed; if the platform holds
	// anything for its owner, it arms. That is a read, not resumed work.
	if start && len(polls) == 0 && strings.TrimSpace(rt.selfdevRouteOwnerID) != "" {
		rt.polls.mu.Lock()
		if !rt.polls.stopped {
			armCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
			rt.polls.bootCancel = cancel
			rt.polls.wg.Add(1)
			go func() {
				defer rt.polls.wg.Done()
				defer cancel()
				rt.armPollsIfNoticesWaiting(armCtx)
			}()
		}
		rt.polls.mu.Unlock()
	}
	for _, poll := range polls {
		if poll.State != store.PlatformPollActive {
			continue
		}
		if !poll.PollInFlightSince.IsZero() {
			poll.CrashesWithPollInFlight++
			poll.PollInFlightSince = time.Time{}
			poll.LastError = "the poll in flight was interrupted by a restart"
			if poll.CrashesWithPollInFlight >= store.PlatformPollCrashLoopLimit {
				poll.State, poll.StateReason = store.PlatformPollClosed, platformPollCrashWhy
			}
		} else {
			poll.CrashesWithPollInFlight = 0
		}
		poll.UpdatedAt = time.Now().UTC()
		if err := rt.putPlatformPoll(ctx, poll); err != nil {
			log.Printf("platform poll: close at boot %s: %v", poll.Name, err)
			continue
		}
		if start && poll.State == store.PlatformPollActive {
			rt.startPlatformPollLoop(poll)
		}
	}
}

func platformPollKey(owner, name string) string { return owner + "\x00" + name }

// startPlatformPollLoop starts the poll's ticker once.
func (rt *Runtime) startPlatformPollLoop(poll store.PlatformPoll) {
	key := platformPollKey(poll.OwnerID, poll.Name)
	rt.polls.mu.Lock()
	if rt.polls.stopped {
		rt.polls.mu.Unlock()
		return
	}
	if rt.polls.loops == nil {
		rt.polls.loops = map[string]context.CancelFunc{}
	}
	if _, running := rt.polls.loops[key]; running {
		rt.polls.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	rt.polls.loops[key] = cancel
	rt.polls.wg.Add(1)
	rt.polls.mu.Unlock()
	interval := time.Duration(poll.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = platformPollInterval(poll.Name)
	}
	owner, name := poll.OwnerID, poll.Name
	go func() {
		defer rt.polls.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			tick, cancelTick := context.WithTimeout(ctx, pollTickTimeout)
			if err := rt.runPlatformPollOnce(tick, owner, name); err != nil {
				log.Printf("platform poll %s: %v", name, err)
			}
			cancelTick()
			if p, err := rt.getPlatformPoll(ctx, owner, name); err == nil && p.State != store.PlatformPollActive {
				return
			}
		}
	}()
}

// stopPlatformPolls ends every loop and waits for the ticks in flight.
func (rt *Runtime) stopPlatformPolls() {
	if rt == nil {
		return
	}
	rt.polls.mu.Lock()
	rt.polls.stopped = true
	if rt.polls.bootCancel != nil {
		rt.polls.bootCancel()
	}
	for key, cancel := range rt.polls.loops {
		cancel()
		delete(rt.polls.loops, key)
	}
	rt.polls.mu.Unlock()
	rt.polls.wg.Wait()
}

// runPlatformPollOnce runs one tick of one poll: mark it in flight, read the
// feed from the saved cursor, commit each item, advance the cursor after the
// records, and record the result. A failure leaves the cursor where it was.
func (rt *Runtime) runPlatformPollOnce(ctx context.Context, owner, name string) error {
	lock := rt.polls.lockFor(platformPollKey(owner, name))
	lock.Lock()
	defer lock.Unlock()
	poll, err := rt.getPlatformPoll(ctx, owner, name)
	if err != nil {
		return err
	}
	if poll.State != store.PlatformPollActive {
		return nil
	}
	feed := rt.platformFeed()
	if feed == nil {
		return fmt.Errorf("no platform URL (choir.platform_url) to poll")
	}
	computer := firstNonEmpty(poll.ComputerID, rt.cfg.ComputerID)
	now := time.Now().UTC()
	poll.PollInFlightSince = now
	if err := rt.putPlatformPoll(ctx, poll); err != nil {
		return err
	}
	var runErr error
	switch name {
	case store.PlatformPollOpedIntake:
		runErr = rt.pollOpeds(ctx, feed, &poll, owner, computer)
	case store.PlatformPollNoticeIntake:
		runErr = rt.pollNotices(ctx, feed, &poll, owner, computer)
	default:
		runErr = fmt.Errorf("unknown platform poll %q", name)
	}
	poll.PollInFlightSince, poll.CrashesWithPollInFlight = time.Time{}, 0
	poll.LastPollAt, poll.UpdatedAt = time.Now().UTC(), time.Now().UTC()
	if runErr != nil {
		poll.LastError, poll.ConsecutiveFailures = runErr.Error(), poll.ConsecutiveFailures+1
	} else {
		poll.LastError, poll.ConsecutiveFailures, poll.LastOKAt = "", 0, poll.LastPollAt
	}
	if err := rt.putPlatformPoll(ctx, poll); err != nil {
		return err
	}
	return runErr
}

// pollNotices adopts the owner's changelog notices.
func (rt *Runtime) pollNotices(ctx context.Context, feed PlatformFeed, poll *store.PlatformPoll, owner, computer string) error {
	for page := 0; page < pollMaxPages; page++ {
		got, err := feed.Notices(ctx, poll.Cursor, pollPageLimit)
		if err != nil {
			return err
		}
		for _, sk := range got.Skipped {
			skipPollItem(poll, sk.ID, sk.Reason)
		}
		for _, item := range got.Items {
			notice := item.Notice
			notice.RemoteRef = item.ID
			if notice.Origin == "" {
				notice.Origin = types.ChangelogOriginCorpusd
			}
			if _, verr := changelogRecordRequest(owner, computer, notice); verr != nil {
				skipPollItem(poll, item.ID, verr.Error())
				continue
			}
			if err := rt.EmitChangelog(ctx, owner, computer, notice); err != nil {
				return fmt.Errorf("adopt notice %s: %w", item.ID, err)
			}
		}
		advanced := got.NextCursor != "" && got.NextCursor != poll.Cursor
		if advanced {
			poll.Cursor = got.NextCursor
			if err := rt.putPlatformPoll(ctx, *poll); err != nil {
				return err
			}
		}
		if len(got.Items) < pollPageLimit || !advanced {
			return nil
		}
	}
	return nil
}

// pollOpeds adopts the op-eds on this computer's stories, then runs the gate
// over the ones without a verdict and decides the desks' wakes.
func (rt *Runtime) pollOpeds(ctx context.Context, feed PlatformFeed, poll *store.PlatformPoll, owner, computer string) error {
	var feedErr error
	for page := 0; page < pollMaxPages; page++ {
		got, err := feed.Opeds(ctx, poll.Cursor, pollPageLimit)
		if err != nil {
			feedErr = err
			break
		}
		for _, sk := range got.Skipped {
			skipPollItem(poll, sk.ID, sk.Reason)
		}
		for _, item := range got.Items {
			if why := opedItemInvalid(owner, computer, item); why != "" {
				skipPollItem(poll, item.VersionID, why)
				continue
			}
			// The feed is the owner's: a sibling computer of the same owner sees
			// op-eds on stories it does not hold. Only this computer's stories
			// are adopted and gated (a document is local when its lifecycle
			// document is in this computer's store).
			if !rt.storyDocLocal(ctx, owner, computer, item.StoryDocID) {
				skipPollItem(poll, item.VersionID, "the story document "+item.StoryDocID+" is not on this computer")
				continue
			}
			if err := rt.commitOpedArrival(ctx, owner, computer, item); err != nil {
				feedErr = fmt.Errorf("adopt op-ed %s: %w", item.VersionID, err)
				break
			}
			rt.queueOpedForGate(ctx, poll, owner, computer, item)
		}
		if feedErr != nil {
			break
		}
		advanced := got.NextCursor != "" && got.NextCursor != poll.Cursor
		if advanced {
			poll.Cursor = got.NextCursor
			if err := rt.putPlatformPoll(ctx, *poll); err != nil {
				return err
			}
		}
		if len(got.Items) < pollPageLimit || !advanced {
			break
		}
	}
	// The gate and the wakes run on what the tape holds, so they also carry
	// op-eds an earlier tick adopted and could not judge.
	rt.runOpedGates(ctx, feed, poll, owner, computer)
	rt.retryOpedCouriers(ctx, feed, poll, owner, computer)
	rt.evaluateAllOpedWakes(ctx, poll, owner, computer)
	return feedErr
}

// commitOpedArrival commits the oped_arrival record; an arrival already on
// the tape is done.
func (rt *Runtime) commitOpedArrival(ctx context.Context, owner, computer string, item OpedFeedItemWire) error {
	a := types.OpedArrival{VersionID: item.VersionID, RoutePath: item.RoutePath, AuthorLabel: item.AuthorLabel,
		RespondsTo: item.RespondsTo, TextHash: item.ManifestHash}
	subject := map[string]string{"oped_version_id": a.VersionID, "responds_to": a.RespondsTo.PublicationVersionID,
		"story_doc_id": item.StoryDocID}
	if err := rt.emitOpedWithSubject(ctx, owner, computer, types.CommitmentOped{Kind: types.OpedRecordArrival, Arrival: &a}, subject); err != nil {
		if rt.opedRecordExists(ctx, owner, computer, types.OpedArrivalRecordID(a.VersionID)) {
			return nil
		}
		return err
	}
	return nil
}

// opedRecordExists reports whether a record with the id is on the tape.
func (rt *Runtime) opedRecordExists(ctx context.Context, owner, computer, recordID string) bool {
	rec, err := rt.store.GetCommitmentRecord(ctx, owner, computer, recordID)
	return err == nil && rec != nil
}

// skipPollItem records a feed item the poll cannot adopt, by its id and why,
// so the cursor can move past it.
func skipPollItem(poll *store.PlatformPoll, id, reason string) {
	log.Printf("platform poll %s: skipped item %q: %s", poll.Name, id, reason)
	poll.Skipped = append(poll.Skipped, store.PollSkipped{ID: clipChars(id, 200), Reason: clipChars(reason, 300), At: time.Now().UTC()})
	if n := len(poll.Skipped); n > store.PollSkippedLimit {
		poll.Skipped = append([]store.PollSkipped(nil), poll.Skipped[n-store.PollSkippedLimit:]...)
	}
}

// opedItemInvalid is why an op-ed feed item cannot be adopted, or "".
func opedItemInvalid(owner, computer string, item OpedFeedItemWire) string {
	if strings.TrimSpace(item.StoryDocID) == "" {
		return "the item names no story document"
	}
	a := types.OpedArrival{VersionID: item.VersionID, RoutePath: item.RoutePath, AuthorLabel: item.AuthorLabel,
		RespondsTo: item.RespondsTo, TextHash: item.ManifestHash}
	if _, err := opedRecordRequest(owner, computer, types.CommitmentOped{Kind: types.OpedRecordArrival, Arrival: &a},
		map[string]string{"story_doc_id": item.StoryDocID}); err != nil {
		return err.Error()
	}
	return ""
}

// storyDocLocal reports whether the document is held by this computer: its
// lifecycle document is in this computer's store. A computer that cannot tell
// says no, so it gates nothing it cannot show it holds.
func (rt *Runtime) storyDocLocal(ctx context.Context, owner, computer, doc string) bool {
	if rt == nil || rt.store == nil || strings.TrimSpace(doc) == "" {
		return false
	}
	_, err := rt.store.GetLifecycleDocument(withKernelReader(ctx), owner, computer, doc)
	return err == nil
}

// armPollsIfNoticesWaiting arms this computer's polls from what the platform
// holds for its owner: one read of each feed from the start, no cursor kept,
// nothing consumed, nothing resumed. A notice waiting arms notice_intake (a
// publisher or forker has them; sibling computers each keep a copy by design).
// An op-ed on a story this computer holds arms oped_intake too. A quiet
// computer, or one whose op-eds are all on its siblings' stories, arms nothing
// for them. It is idempotent, and a fault in it ends in a log line, never a
// crash: it runs in its own goroutine at boot.
func (rt *Runtime) armPollsIfNoticesWaiting(ctx context.Context) {
	if rt == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("platform poll: boot arm: recovered: %v", r)
		}
	}()
	owner := strings.TrimSpace(rt.selfdevRouteOwnerID)
	if rt.store == nil || owner == "" {
		return
	}
	feed := rt.platformFeed()
	if feed == nil {
		return
	}
	computer := rt.cfg.ComputerID
	notices, err := feed.Notices(ctx, "", 1)
	switch {
	case err != nil:
		log.Printf("platform poll: boot arm: the first notice read failed (this computer will not hear notices until it publishes or forks): %v", err)
	case len(notices.Items) > 0:
		rt.ensurePlatformPollsNamed(ctx, owner, computer, store.PlatformPollNoticeIntake)
	case len(notices.Skipped) > 0:
		log.Printf("platform poll: boot arm: the first notice read returned only skipped rows (%d, first %q: %s)", len(notices.Skipped), notices.Skipped[0].ID, notices.Skipped[0].Reason)
	}
	opeds, err := feed.Opeds(ctx, "", 50)
	if err != nil {
		log.Printf("platform poll: boot arm: the first op-ed read failed: %v", err)
		return
	}
	for _, item := range opeds.Items {
		if rt.storyDocLocal(ctx, owner, computer, item.StoryDocID) {
			rt.ensurePlatformPollsNamed(ctx, owner, computer, store.PlatformPollOpedIntake, store.PlatformPollNoticeIntake)
			return
		}
	}
}

// RunPlatformPollNow runs one tick of a poll immediately: the same work its
// loop does each interval, for a caller that wants it now (a test that drives
// two computers in one process). It resumes nothing.
func (rt *Runtime) RunPlatformPollNow(ctx context.Context, owner, name string) error {
	return rt.runPlatformPollOnce(ctx, owner, name)
}
