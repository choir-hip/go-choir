package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/yusefmosiah/go-choir/internal/outlet"
	"github.com/yusefmosiah/go-choir/internal/quotematch"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Automatic publishing (Cut 2 2b, W3; defaults taken 4 and 6). Every
// committed revision of a story opened with publish auto publishes as its
// next version (version = Texture revision number) right after its commit,
// off the commit's path, through the owner's publish path run for this
// computer. No human step. A publish that fails leaves a gap the reader's
// nav skips; it is logged and never retried, and nothing renumbers.
//
// The queue lives in this process only. A crash takes it down with the
// process and no boot step refills it: a revision committed before a crash
// and not yet published stays a gap (CLAUDE.md, "Restarts End Work"; no
// boot-time resumer). The next committed revision publishes as its own
// number.
//
// Support verdicts on a story's citations go to Store B the same way, so
// the published check overlay updates without reading this computer.

// StoryVersion is one committed story revision to publish: Revision is its
// number on the revision chain (types.RevisionNumber), its version.
type StoryVersion struct {
	OwnerID    string
	ComputerID string
	DocID      string
	RevisionID string
	Revision   int
}

// StoryPublisher publishes story versions and support verdicts to the
// platform.
type StoryPublisher interface {
	PublishVersion(ctx context.Context, v StoryVersion) error
	PublishVerdicts(ctx context.Context, req types.StoryVerdictsRequest) error
}

// StoryRelationPublisher is the optional second face of a StoryPublisher: it
// exports the research desk's judged pair relations to the platform
// (choir.RelateStories). ValidateRelations runs every check the export runs
// and writes nothing; the verb calls it before the tape record, so the tape
// holds only pairs the platform would take. A publisher without these cannot
// check or export, and the verb records nothing.
type StoryRelationPublisher interface {
	ValidateRelations(ctx context.Context, req types.StoryRelationsRequest) error
	PublishRelations(ctx context.Context, req types.StoryRelationsRequest) error
}

// storyPublishTimeout bounds one publish; a publish that runs out is a
// gap like any other failure.
const storyPublishTimeout = 2 * time.Minute

// storyPublishing orders publishes per key (one story's versions, one
// owner's verdicts) so v<n> is attempted before v<n+1>.
type storyPublishing struct {
	mu        sync.Mutex
	publisher StoryPublisher
	set       bool
	queues    map[string][]func(context.Context)
	running   map[string]bool
}

// SetStoryPublisher installs the publisher (tests, or a nil to publish
// nothing). Without one the runtime publishes versions through its proxy
// URL and verdicts to its platform URL.
func (rt *Runtime) SetStoryPublisher(p StoryPublisher) {
	if rt == nil {
		return
	}
	rt.stories.mu.Lock()
	defer rt.stories.mu.Unlock()
	rt.stories.publisher, rt.stories.set = p, true
}

func (rt *Runtime) storyPublisher() StoryPublisher {
	rt.stories.mu.Lock()
	defer rt.stories.mu.Unlock()
	if !rt.stories.set {
		proxy, platform := strings.TrimSpace(rt.cfg.ProxyURL), strings.TrimSpace(rt.cfg.CorpusdURL)
		if proxy != "" || platform != "" {
			rt.stories.publisher = NewHTTPStoryPublisher(proxy, platform)
		}
		rt.stories.set = true
	}
	return rt.stories.publisher
}

// enqueueStoryPublish runs job after every earlier job under key, on one
// goroutine per busy key.
func (rt *Runtime) enqueueStoryPublish(key string, job func(context.Context)) {
	s := &rt.stories
	s.mu.Lock()
	if s.queues == nil {
		s.queues, s.running = map[string][]func(context.Context){}, map[string]bool{}
	}
	s.queues[key] = append(s.queues[key], job)
	if s.running[key] {
		s.mu.Unlock()
		return
	}
	s.running[key] = true
	s.mu.Unlock()
	go func() {
		for {
			s.mu.Lock()
			queue := s.queues[key]
			if len(queue) == 0 {
				delete(s.queues, key)
				delete(s.running, key)
				s.mu.Unlock()
				return
			}
			next := queue[0]
			s.queues[key] = queue[1:]
			s.mu.Unlock()
			ctx, cancel := context.WithTimeout(context.Background(), storyPublishTimeout)
			next(ctx)
			cancel()
		}
	}()
}

// storyPublishesAuto reports whether docID is a story opened with publish
// auto.
func (rt *Runtime) storyPublishesAuto(ctx context.Context, ownerID, computerID, docID string) (bool, error) {
	doc, err := rt.store.GetLifecycleDocument(ctx, ownerID, computerID, docID)
	if err != nil || strings.TrimSpace(doc.TrajectoryID) == "" {
		return false, err
	}
	trajectory, err := rt.store.GetLifecycleTrajectory(ctx, ownerID, computerID, doc.TrajectoryID)
	if err != nil {
		return false, err
	}
	return types.StoryPublishesAuto(trajectory), nil
}

// AutoPublishRevision publishes a committed revision of a publish-auto
// story as its version, after the commit and off its path. The Texture
// owner calls it once per committed revision; it returns at once.
//
// Every outcome logs one "story publish:" line, the decisions not to
// publish included: a silent return is how B12 (every story revision
// inherited the seed's role and none published) hid on staging.
//
// A revision that cites no source is held: it stays committed on the
// computer and its version is a gap, and the next revision that cites a
// source publishes as its own number. With research failed, Texture
// wrote one-paragraph "the claims are unverified" revisions and they
// published as stories (docs/problems/free-model-429-opens-provider-
// circuit-stub-stories-2026-10-14.md). An owner's publish is another
// route and is not held.
//
// A revision that does not lead with a headline is held the same way. A
// story is titled by the headline Texture writes (the store retitles it in
// the revision's commit); without one the title is still the opening
// Cast's objective, and management's writing briefs ("Write the story of
// ...") went out as published titles, while V1 plans ("This document will
// explain ...") went out as deks (docs/problems/instruction-text-published-
// as-story-titles-2026-10-14.md).
//
// held is the reason when a publish-auto story's revision was held, so
// the caller can say so; it is empty otherwise.
func (rt *Runtime) AutoPublishRevision(ctx context.Context, ownerID string, doc types.Document, rev types.Revision) (held string) {
	if rt == nil || rt.store == nil {
		log.Printf("story publish: %s revision %s not considered: no runtime store", doc.DocID, rev.RevisionID)
		return ""
	}
	if strings.TrimSpace(doc.TrajectoryID) == "" {
		log.Printf("story publish: %s revision %s not published: the document has no trajectory, so it is no story", doc.DocID, rev.RevisionID)
		return ""
	}
	trajectory, err := rt.store.GetLifecycleTrajectory(ctx, ownerID, doc.ComputerID, doc.TrajectoryID)
	if err != nil {
		log.Printf("story publish: %s revision %s is a gap: read trajectory %s: %v", doc.DocID, rev.RevisionID, doc.TrajectoryID, err)
		return ""
	}
	if !types.StoryPublishesAuto(trajectory) {
		log.Printf("story publish: %s revision %s not published: trajectory %s is kind %q with publish %q, not a publish-auto story", doc.DocID, rev.RevisionID, doc.TrajectoryID, trajectory.Kind, trajectory.SubjectRefs[types.StoryRefPublish])
		return ""
	}
	if revisionRole(rev) == types.StorySeedRole {
		log.Printf("story publish: %s revision %s not published: it is the story's seed", doc.DocID, rev.RevisionID)
		return ""
	}
	v := StoryVersion{OwnerID: ownerID, ComputerID: doc.ComputerID, DocID: doc.DocID, RevisionID: rev.RevisionID, Revision: types.RevisionNumber(rev.VersionNumber)}
	// Station TE (E6): evidence is never published; the platform refuses it,
	// so it is held here with the reason instead of failing there.
	if private := texturedoc.PrivateEmbedsInBody(rev.BodyDoc); len(private) > 0 {
		held = fmt.Sprintf("it holds %d private embed(s) (capsule evidence is never published), so v%d stays a gap", len(private), v.Revision)
		log.Printf("story publish: %s revision %s not published: %s", v.DocID, v.RevisionID, held)
		return held
	}
	if cites, err := texturewrite.CitesSource(rev); err != nil || !cites {
		held = "it cites no captured source, so v" + strconv.Itoa(v.Revision) + " stays a gap; the next revision that cites one publishes"
		if err != nil {
			held = "its citations could not be read (" + err.Error() + "), so v" + strconv.Itoa(v.Revision) + " stays a gap"
		}
		log.Printf("story publish: %s revision %s not published: %s", v.DocID, v.RevisionID, held)
		return held
	}
	if outlets, linkOnly, err := rt.citedOutlets(ctx, ownerID, rev); err != nil || len(outlets) < MinPublishOutlets {
		held = fmt.Sprintf("it cites %d outlet%s (%s); a story publishes only when its citations span at least %d distinct outlets, so v%d stays a gap; the next revision that cites another outlet's capture publishes",
			len(outlets), plural(len(outlets)), strings.Join(outlets, ", "), MinPublishOutlets, v.Revision)
		if len(linkOnly) > 0 {
			held += fmt.Sprintf(" (not counted: %s, whose capture holds only links, not the outlet's own words)", strings.Join(linkOnly, ", "))
		}
		if err != nil {
			held = "its cited captures could not be read (" + err.Error() + "), so v" + strconv.Itoa(v.Revision) + " stays a gap"
		}
		log.Printf("story publish: %s revision %s not published: %s", v.DocID, v.RevisionID, held)
		return held
	}
	if texturedoc.Headline(rev.BodyDoc) == "" {
		held = "it does not lead with a headline (a heading as its first block, which titles the story), so v" + strconv.Itoa(v.Revision) + " stays a gap; the next revision that leads with one publishes"
		log.Printf("story publish: %s revision %s not published: %s", v.DocID, v.RevisionID, held)
		return held
	}
	publisher := rt.storyPublisher()
	if publisher == nil {
		log.Printf("story publish: %s v%d (revision %s) is a gap: no platform to publish to", v.DocID, v.Revision, v.RevisionID)
		return ""
	}
	rt.enqueueStoryPublish("doc:"+ownerID+":"+doc.DocID, func(ctx context.Context) {
		if err := publisher.PublishVersion(ctx, v); err != nil {
			log.Printf("story publish: %s v%d (revision %s) failed and stays a gap: %v", v.DocID, v.Revision, v.RevisionID, err)
			return
		}
		log.Printf("story publish: %s published v%d (revision %s)", v.DocID, v.Revision, v.RevisionID)
	})
	return ""
}

// MinPublishOutlets is how many distinct outlets a story's citations must
// span before it publishes (owner §24: "We always should have two, ideally
// more"; docs/problems/capture-and-production-are-coupled-2026-10-16.md).
const MinPublishOutlets = 2

// citedOutletsTimeout bounds the capture reads on Texture's turn.
const citedOutletsTimeout = 20 * time.Second

// citedOutlets returns the distinct outlets of the captures rev cites,
// sorted, and the outlets of cited captures that hold only links. A cited
// capture the store cannot resolve, whose URL names no outlet, or that
// holds only links does not count.
// A store that is not configured is an error: the revision is held.
func (rt *Runtime) citedOutlets(ctx context.Context, ownerID string, rev types.Revision) ([]string, []string, error) {
	docs, err := texturewrite.CitedCaptures(rev)
	if err != nil {
		return nil, nil, err
	}
	captures := rt.TextureCaptures()
	if captures == nil {
		return nil, nil, fmt.Errorf("the capture store is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, citedOutletsTimeout)
	defer cancel()
	seen, seenLinkOnly := map[string]bool{}, map[string]bool{}
	var out, linkOnly []string
	for _, doc := range docs {
		capture, err := captures.CaptureBlocks(ctx, ownerID, doc)
		if err != nil {
			log.Printf("story publish: cited capture %s does not count toward outlets: %v", doc, err)
			continue
		}
		key := outlet.Of(capture.URL)
		if key == "" {
			continue
		}
		if captureHoldsOnlyLinks(capture.Blocks) {
			if !seenLinkOnly[key] {
				seenLinkOnly[key] = true
				linkOnly = append(linkOnly, key)
			}
			continue
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	sort.Strings(out)
	sort.Strings(linkOnly)
	return out, linkOnly, nil
}

// minOutletWords is how many letters of its own a cited capture must hold,
// once its links and link labels are removed, to speak for its outlet. A
// Hacker News feed item's capture is addressed by the linked site but
// holds only links, so it is not that site's report
// (docs/problems/link-only-capture-counts-as-an-outlet-2026-10-17.md).
// Twenty letters is under one short sentence in any script.
const minOutletWords = 20

var (
	captureLinkPattern = regexp.MustCompile(`https?://\S+`)
	captureLinkLabels  = strings.NewReplacer("Article URL:", " ", "Comments URL:", " ", "Points:", " ", "# Comments:", " ")
)

// captureHoldsOnlyLinks reports whether blocks hold fewer than
// minOutletWords letters outside their URLs and link labels.
func captureHoldsOnlyLinks(blocks []quotematch.Block) bool {
	letters := 0
	for _, b := range blocks {
		text := captureLinkLabels.Replace(captureLinkPattern.ReplaceAllString(b.Text, " "))
		for _, r := range text {
			if unicode.IsLetter(r) {
				letters++
				if letters >= minOutletWords {
					return false
				}
			}
		}
	}
	return true
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func revisionRole(rev types.Revision) string {
	var meta struct {
		Role string `json:"revision_role"`
	}
	_ = json.Unmarshal(rev.Metadata, &meta)
	return meta.Role
}

// publishStoryVerdicts sends the verdicts a management cell committed on
// publish-auto stories' citations to Store B, where the published check
// overlay reads them. Verdicts on other documents stay on this computer.
func (rt *Runtime) publishStoryVerdicts(ownerID, computerID string, verdicts []types.SupportVerdict) {
	if rt == nil || rt.store == nil || len(verdicts) == 0 {
		return
	}
	publisher := rt.storyPublisher()
	if publisher == nil {
		log.Printf("story verdicts: %d verdicts not published: no platform to publish to", len(verdicts))
		return
	}
	verdicts = append([]types.SupportVerdict(nil), verdicts...)
	rt.enqueueStoryPublish("verdicts:"+ownerID, func(ctx context.Context) {
		stories := map[string]bool{}
		req := types.StoryVerdictsRequest{OwnerID: ownerID, ComputerID: computerID}
		for _, v := range verdicts {
			if v.Pair == nil || strings.TrimSpace(v.Key) == "" {
				continue
			}
			story, seen := stories[v.Pair.DocID]
			if !seen {
				var err error
				if story, err = rt.storyPublishesAuto(ctx, ownerID, computerID, v.Pair.DocID); err != nil {
					log.Printf("story verdicts: %s: %v", v.Pair.DocID, err)
				}
				stories[v.Pair.DocID] = story
			}
			if story {
				req.Verdicts = append(req.Verdicts, types.StoryVerdict{Key: v.Key, Verdict: v.Verdict, Why: v.Why, Judge: v.Judge})
			}
		}
		if len(req.Verdicts) == 0 {
			log.Printf("story verdicts: none of %d verdicts judges a publish-auto story; nothing to publish", len(verdicts))
			return
		}
		if err := publisher.PublishVerdicts(ctx, req); err != nil {
			log.Printf("story verdicts: %d verdicts not published; their citations stay pending in the overlay: %v", len(req.Verdicts), err)
			return
		}
		log.Printf("story verdicts: published %d verdicts", len(req.Verdicts))
	})
}

// httpStoryPublisher publishes versions through the proxy's guest route
// (the proxy binds the calling guest to its owner and computer, reads the
// revision back from it and runs the owner's publish path) and pushes
// verdicts to the computer's platform URL, corpusd, which admits the bound
// guest under the same rule as its capture writes (B12).
type httpStoryPublisher struct {
	proxyURL    string
	platformURL string
	client      *http.Client
}

// NewHTTPStoryPublisher publishes versions to proxyURL (choir.proxy_url) and
// verdicts to platformURL (choir.platform_url, corpusd). A version with no
// proxy URL is a gap whose error says so.
func NewHTTPStoryPublisher(proxyURL, platformURL string) StoryPublisher {
	trim := func(s string) string { return strings.TrimRight(strings.TrimSpace(s), "/") }
	return &httpStoryPublisher{proxyURL: trim(proxyURL), platformURL: trim(platformURL), client: &http.Client{Timeout: storyPublishTimeout}}
}

func (p *httpStoryPublisher) post(ctx context.Context, baseURL, path string, body, out any) error {
	if baseURL == "" {
		return fmt.Errorf("%s: no host URL to send it to", path)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("%s: decode: %w", path, err)
		}
	}
	return nil
}

func (p *httpStoryPublisher) PublishVersion(ctx context.Context, v StoryVersion) error {
	var resp struct {
		Revision      int    `json:"revision"`
		PermalinkPath string `json:"permalink_path"`
	}
	if p.proxyURL == "" {
		return fmt.Errorf("no proxy URL (choir.proxy_url) to publish through; this computer booted without one")
	}
	if err := p.post(ctx, p.proxyURL, types.StoryPublishPath, types.StoryPublishRequest{DocID: v.DocID, RevisionID: v.RevisionID}, &resp); err != nil {
		return err
	}
	if resp.Revision != v.Revision {
		return fmt.Errorf("published as v%d, want v%d", resp.Revision, v.Revision)
	}
	return nil
}

// ValidateRelations posts the relations validate-only and requires the
// "validated" answer: a platform that does not know the flag answers
// "recorded" (and has written), and that is no check.
func (p *httpStoryPublisher) ValidateRelations(ctx context.Context, req types.StoryRelationsRequest) error {
	req.ValidateOnly = true
	var resp struct {
		Validated *int `json:"validated"`
	}
	if err := p.post(ctx, p.platformURL, types.StoryRelationsPath, req, &resp); err != nil {
		return err
	}
	if resp.Validated == nil {
		return fmt.Errorf("%s: the platform did not answer the validate-only call", types.StoryRelationsPath)
	}
	return nil
}

func (p *httpStoryPublisher) PublishRelations(ctx context.Context, req types.StoryRelationsRequest) error {
	return p.post(ctx, p.platformURL, types.StoryRelationsPath, req, nil)
}

// ValidateEntities posts the attachments validate-only and requires the
// "validated" answer, as ValidateRelations does.
func (p *httpStoryPublisher) ValidateEntities(ctx context.Context, req types.StoryEntitiesRequest) error {
	req.ValidateOnly = true
	var resp struct {
		Validated *int `json:"validated"`
	}
	if err := p.post(ctx, p.platformURL, types.StoryEntitiesPath, req, &resp); err != nil {
		return err
	}
	if resp.Validated == nil {
		return fmt.Errorf("%s: the platform did not answer the validate-only call", types.StoryEntitiesPath)
	}
	return nil
}

func (p *httpStoryPublisher) PublishEntities(ctx context.Context, req types.StoryEntitiesRequest) error {
	return p.post(ctx, p.platformURL, types.StoryEntitiesPath, req, nil)
}

// EntityCandidates reads a story's entity candidates from the platform
// (slice c0 §2); nothing is written.
func (p *httpStoryPublisher) EntityCandidates(ctx context.Context, req types.EntityCandidatesRequest) (types.EntityCandidatesResponse, error) {
	var resp types.EntityCandidatesResponse
	err := p.post(ctx, p.platformURL, types.StoryEntityCandidatesPath, req, &resp)
	return resp, err
}

func (p *httpStoryPublisher) PublishVerdicts(ctx context.Context, req types.StoryVerdictsRequest) error {
	return p.post(ctx, p.platformURL, types.StoryVerdictsPath, req, nil)
}
