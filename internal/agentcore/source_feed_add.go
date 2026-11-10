package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Add a source by asking (Lane F, handoff section 4.F, done item 7). The
// owner names a feed; the computer validates it with the poller's own guarded
// client before anything is written, and then starts an ordinary source
// schedule whose definition rides on the schedule record. A schedule is a
// standing owner intent, so boot recovery, the sweep and the crash bound
// already cover it; nothing here is a second scheduler.
//
// Adding a feed is an owner act: the URL can come from web-derived text, so a
// desk's in-process call never adds one.

const (
	// maxCustomFeedsPerOwner bounds the feeds one owner adds by asking.
	maxCustomFeedsPerOwner = 50
	// feedProbeTimeout bounds the validation fetch.
	feedProbeTimeout = 15 * time.Second
	// customFeedSourcePrefix starts every owner-added source id.
	customFeedSourcePrefix = "custom:"
)

// Feed kinds an owner can add by URL.
const (
	feedKindRSS      = "rss"
	feedKindTelegram = "telegram"
)

// Refusal codes, named so the prompt bar can say why.
const (
	feedRefusedURL          = "feed_url_refused"
	feedRefusedNotReachable = "feed_not_reachable"
	feedRefusedNotParsed    = "feed_not_parsed"
	feedRefusedEmpty        = "feed_empty"
	feedRefusedTooLarge     = "feed_too_large"
	feedRefusedCap          = "feed_cap"
	feedRefusedOwnerAct     = "feed_owner_act_required"
)

// feedRefusal is a named reason an add was refused. Nothing was written.
type feedRefusal struct {
	Status  int
	Code    string
	Message string
}

func (f *feedRefusal) Error() string { return f.Code + ": " + f.Message }

func refuseFeed(status int, code, format string, args ...any) *feedRefusal {
	return &feedRefusal{Status: status, Code: code, Message: fmt.Sprintf(format, args...)}
}

// feedKindOf is the source kind a URL names: a t.me channel is a Telegram
// source, everything else an RSS or Atom feed.
func feedKindOf(u *url.URL) string {
	switch strings.ToLower(u.Hostname()) {
	case "t.me", "telegram.me":
		return feedKindTelegram
	}
	return feedKindRSS
}

// customFeedSourceID is the id of an owner-added feed: a hash of its kind and
// URL, so adding the same feed again is the same schedule.
func customFeedSourceID(kind, feedURL string) string {
	sum := sha256.Sum256([]byte(kind + "\n" + feedURL))
	return customFeedSourcePrefix + kind + ":" + hex.EncodeToString(sum[:8])
}

// feedSource is the registry-shaped source an owner-added schedule polls.
func feedSource(sched store.SourceSchedule) sources.Source {
	src := sources.Source{
		ID: sched.SourceID, Type: sources.SourceType(sched.FeedKind), Name: sched.FeedName, URL: sched.FeedURL,
		Verticals: []string{"general"}, PollIntervalSeconds: int(sched.IntervalSeconds), MaxItemsPerPoll: sched.MaxItems,
		TOSClass: "owner_added_public_feed", RobotsPolicy: "respect", AuthPolicy: "none",
		StoreBodyPolicy: "bounded_text", RetentionDays: 30,
	}
	if sched.FeedKind == feedKindTelegram {
		src.ConditionalMode = "none"
	}
	return src
}

// probeFeed validates a feed before it is accepted: the URL passes the fetch
// policy, the host answers 200 within the bound, and the body parses with at
// least one item. It returns the kind, the canonical URL and a name.
func (rt *Runtime) probeFeed(ctx context.Context, rawURL string) (kind, feedURL, name string, ref *feedRefusal) {
	feedURL = strings.TrimSpace(rawURL)
	if feedURL == "" {
		return "", "", "", refuseFeed(http.StatusBadRequest, feedRefusedURL, "feed_url is empty")
	}
	if err := sourcefetch.ValidateURL(feedURL); err != nil {
		return "", "", "", refuseFeed(http.StatusUnprocessableEntity, feedRefusedURL, "%v", err)
	}
	parsed, err := url.Parse(feedURL)
	if err != nil {
		return "", "", "", refuseFeed(http.StatusUnprocessableEntity, feedRefusedURL, "the URL does not parse")
	}
	kind = feedKindOf(parsed)
	userAgent := "ChoirAutopaper/0.1 (+https://choir.news)"
	if rt.researchDeps != nil && rt.researchDeps.Sources != nil && strings.TrimSpace(rt.researchDeps.Sources.UserAgent) != "" {
		userAgent = rt.researchDeps.Sources.UserAgent
	}
	ctx, cancel := context.WithTimeout(ctx, feedProbeTimeout)
	defer cancel()
	if kind == feedKindTelegram {
		src := sources.Source{ID: "probe", Type: sources.SourceTypeTelegram, URL: feedURL}
		result, err := sources.NewTelegramScraper(userAgent).Poll(ctx, &src)
		if err != nil {
			return "", "", "", refuseFeed(http.StatusUnprocessableEntity, feedRefusedNotReachable, "the channel did not answer: %v", err)
		}
		if len(result.Items) == 0 {
			return "", "", "", refuseFeed(http.StatusUnprocessableEntity, feedRefusedEmpty, "the channel shows no posts")
		}
		return kind, feedURL, strings.TrimPrefix(parsed.Path, "/s/"), nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return "", "", "", refuseFeed(http.StatusUnprocessableEntity, feedRefusedURL, "the URL cannot be requested")
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := sourcefetch.Client(feedProbeTimeout).Do(req)
	if err != nil {
		return "", "", "", refuseFeed(http.StatusUnprocessableEntity, feedRefusedNotReachable, "the feed did not answer: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", "", refuseFeed(http.StatusUnprocessableEntity, feedRefusedNotReachable, "the feed answered status %d", resp.StatusCode)
	}
	limit := int64(sources.DefaultMaxFeedBodyBytes)
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return "", "", "", refuseFeed(http.StatusUnprocessableEntity, feedRefusedNotReachable, "the feed could not be read: %v", err)
	}
	if int64(len(body)) > limit {
		return "", "", "", refuseFeed(http.StatusUnprocessableEntity, feedRefusedTooLarge, "the feed is larger than %d bytes", limit)
	}
	probe, err := sources.ProbeFeed(body)
	if err != nil {
		return "", "", "", refuseFeed(http.StatusUnprocessableEntity, feedRefusedNotParsed, "%v", err)
	}
	if probe.Items == 0 {
		return "", "", "", refuseFeed(http.StatusUnprocessableEntity, feedRefusedEmpty, "the feed has no items")
	}
	name = probe.Title
	if name == "" {
		name = parsed.Hostname()
	}
	return kind, feedURL, name, nil
}

// AddFeedSchedule validates a feed URL and starts a schedule on it. ownerAct
// is the owner request's id (empty is a desk, which is refused). Nothing is
// written unless every check passes.
func (rt *Runtime) AddFeedSchedule(ctx context.Context, ownerID, computerID, rawURL string, interval time.Duration, maxItems int, ownerAct string) (store.SourceSchedule, *feedRefusal, error) {
	if rt == nil || rt.store == nil {
		return store.SourceSchedule{}, nil, fmt.Errorf("source schedule: store unavailable")
	}
	if ownerAct == "" {
		return store.SourceSchedule{}, refuseFeed(http.StatusForbidden, feedRefusedOwnerAct, "adding a feed is an owner act; ask the owner to add it"), nil
	}
	kind, feedURL, name, ref := rt.probeFeed(ctx, rawURL)
	if ref != nil {
		return store.SourceSchedule{}, ref, nil
	}
	id := customFeedSourceID(kind, feedURL)
	existing, err := rt.store.ListSourceSchedules(scheduleCustodian(ctx), ownerID)
	if err != nil {
		return store.SourceSchedule{}, nil, err
	}
	custom, known := 0, false
	for _, s := range existing {
		if s.FeedURL != "" {
			custom++
		}
		if s.SourceID == id {
			known = true
		}
	}
	if !known && custom >= maxCustomFeedsPerOwner {
		return store.SourceSchedule{}, refuseFeed(http.StatusUnprocessableEntity, feedRefusedCap, "an owner can add at most %d feeds; stop one first", maxCustomFeedsPerOwner), nil
	}
	def := store.SourceSchedule{FeedURL: feedURL, FeedKind: kind, FeedName: name}
	src := feedSource(def)
	src.ID = id
	ctx = objectgraph.WithWriter(ctx, objectgraph.WriterOwner)
	sched, err := rt.startScheduleFor(ctx, ownerID, computerID, src, &def, interval, maxItems, false, ownerAct)
	return sched, nil, err
}
