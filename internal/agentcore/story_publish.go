package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

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
// nothing). Without one the runtime publishes through its platform URL.
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
		if base := strings.TrimSpace(rt.cfg.CorpusdURL); base != "" {
			rt.stories.publisher = NewHTTPStoryPublisher(base)
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
func (rt *Runtime) AutoPublishRevision(ctx context.Context, ownerID string, doc types.Document, rev types.Revision) {
	if rt == nil || rt.store == nil {
		log.Printf("story publish: %s revision %s not considered: no runtime store", doc.DocID, rev.RevisionID)
		return
	}
	if strings.TrimSpace(doc.TrajectoryID) == "" {
		log.Printf("story publish: %s revision %s not published: the document has no trajectory, so it is no story", doc.DocID, rev.RevisionID)
		return
	}
	trajectory, err := rt.store.GetLifecycleTrajectory(ctx, ownerID, doc.ComputerID, doc.TrajectoryID)
	if err != nil {
		log.Printf("story publish: %s revision %s is a gap: read trajectory %s: %v", doc.DocID, rev.RevisionID, doc.TrajectoryID, err)
		return
	}
	if !types.StoryPublishesAuto(trajectory) {
		log.Printf("story publish: %s revision %s not published: trajectory %s is kind %q with publish %q, not a publish-auto story", doc.DocID, rev.RevisionID, doc.TrajectoryID, trajectory.Kind, trajectory.SubjectRefs[types.StoryRefPublish])
		return
	}
	if revisionRole(rev) == types.StorySeedRole {
		log.Printf("story publish: %s revision %s not published: it is the story's seed", doc.DocID, rev.RevisionID)
		return
	}
	v := StoryVersion{OwnerID: ownerID, ComputerID: doc.ComputerID, DocID: doc.DocID, RevisionID: rev.RevisionID, Revision: types.RevisionNumber(rev.VersionNumber)}
	publisher := rt.storyPublisher()
	if publisher == nil {
		log.Printf("story publish: %s v%d (revision %s) is a gap: no platform to publish to", v.DocID, v.Revision, v.RevisionID)
		return
	}
	rt.enqueueStoryPublish("doc:"+ownerID+":"+doc.DocID, func(ctx context.Context) {
		if err := publisher.PublishVersion(ctx, v); err != nil {
			log.Printf("story publish: %s v%d (revision %s) failed and stays a gap: %v", v.DocID, v.Revision, v.RevisionID, err)
			return
		}
		log.Printf("story publish: %s published v%d (revision %s)", v.DocID, v.Revision, v.RevisionID)
	})
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

// httpStoryPublisher publishes through the proxy's guest routes: the proxy
// binds the calling guest to its owner and computer, runs the owner's
// publish path for it, and writes verdicts to corpusd.
type httpStoryPublisher struct {
	baseURL string
	client  *http.Client
}

// NewHTTPStoryPublisher publishes to the platform at baseURL (the
// computer's CHOIR_PLATFORM_URL, the proxy).
func NewHTTPStoryPublisher(baseURL string) StoryPublisher {
	return &httpStoryPublisher{baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), client: &http.Client{Timeout: storyPublishTimeout}}
}

func (p *httpStoryPublisher) post(ctx context.Context, path string, body, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(raw))
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
	if err := p.post(ctx, types.StoryPublishPath, types.StoryPublishRequest{DocID: v.DocID, RevisionID: v.RevisionID}, &resp); err != nil {
		return err
	}
	if resp.Revision != v.Revision {
		return fmt.Errorf("published as v%d, want v%d", resp.Revision, v.Revision)
	}
	return nil
}

func (p *httpStoryPublisher) PublishVerdicts(ctx context.Context, req types.StoryVerdictsRequest) error {
	return p.post(ctx, types.StoryVerdictsPath, req, nil)
}
