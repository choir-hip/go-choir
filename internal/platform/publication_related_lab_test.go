package platform

// A lab for the related-stories tests (docs/reports/related-stories-graph-design-2026-10-19.md):
// real Store B captures, real published stories that cite them through
// Texture's own cite syntax, so the edges, the walk and the front rows run
// through the production paths and not through hand-built rows.

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
)

const labOwner = "paper-owner"

type relatedLab struct {
	t             *testing.T
	svc           *Service
	store         *Store
	artifactsRoot string
	ogSvc         *objectgraph.Service
	bodies        *CaptureBodyStore
	captures      *storeCaptures
	quotes        map[string]string // capture id -> a quotable sentence of its first block
	seq           int
}

func newRelatedLab(t *testing.T) *relatedLab {
	t.Helper()
	store, root := openTestPlatformStore(t)
	artifactsRoot := filepath.Join(root, "artifacts")
	svc := NewService(store, artifactsRoot, filepath.Join(root, "signing.key"))
	// A loaded -race runner can overrun the 400 ms production budget, and an
	// overrun answers empty (related-stories-test-overruns-its-budget-under-ci-2026-10-22.md).
	// The budget test sets its own values.
	svc.related.tuning.Budget = 30 * time.Second
	// The lab pins the strong signals; the floor (MinLinks) has its own
	// tests in publication_related_floor_test.go and turns it on there.
	svc.related.tuning.MinLinks = 0
	bodies := &CaptureBodyStore{root: CaptureBodiesRoot(artifactsRoot)}
	return &relatedLab{
		t: t, svc: svc, store: store, artifactsRoot: artifactsRoot,
		ogSvc:    objectgraph.NewService(objectgraph.Config{Durable: svc.ogStore()}),
		bodies:   bodies,
		captures: &storeCaptures{t: t, og: svc.ogStore(), bodies: bodies},
		quotes:   map[string]string{},
	}
}

// capture writes one web capture. canonical may be empty (then the URL is
// canonical). Two calls with the same url and different text are two
// captures of one page (a recapture).
func (l *relatedLab) capture(url, canonical, text string) string {
	return l.captureAs(labOwner, "computer-paper", url, canonical, text)
}

func (l *relatedLab) captureAs(owner, computer, url, canonical, text string) string {
	l.t.Helper()
	l.seq++
	item := sources.Item{
		ID: fmt.Sprintf("item-%d", l.seq), SourceID: "rss:lab", SourceType: sources.SourceTypeRSS,
		Title: "Capture " + text, URL: url, CanonicalURL: canonical,
		Body:      text + ".\n\nA second paragraph for " + text + ".",
		FetchedAt: time.Date(2026, 10, 18, 8, 0, l.seq, 0, time.UTC),
	}
	written, err := sourcegraph.WriteCaptures(context.Background(), l.ogSvc, l.bodies, []sourcegraph.CaptureItem{{Item: item}},
		sourcegraph.CaptureWriteConfig{OwnerID: owner, ComputerID: computer, Now: time.Now().UTC()})
	if err != nil || len(written.Captured) != 1 {
		l.t.Fatalf("write capture %q: %+v %v", text, written, err)
	}
	id := written.Captured[0].CaptureID
	l.quotes[id] = text
	return id
}

// publish publishes version n of doc as an Autopaper story that cites each
// capture once, in the order given, and returns the response.
func (l *relatedLab) publish(doc string, n int, title string, captureIDs ...string) *PublishTextureResponse {
	return l.publishAs(labOwner, PublishModeAuto, nil, doc, n, title, captureIDs...)
}

func (l *relatedLab) publishAs(owner, mode string, accessPolicy json.RawMessage, doc string, n int, title string, captureIDs ...string) *PublishTextureResponse {
	l.t.Helper()
	var bodyDoc, entities json.RawMessage
	if len(captureIDs) > 0 {
		var script strings.Builder
		script.WriteString("@insert end\n")
		for i, id := range captureIDs {
			if i > 0 {
				script.WriteString("\n\n")
			}
			fmt.Fprintf(&script, "Claim %d of %s {{cite %s \"%s\"}}.", i+1, title, id, l.quotes[id])
		}
		res, err := texturewrite.Apply(context.Background(), texturewrite.Input{
			OwnerID: owner, DocID: doc, Seed: fmt.Sprintf("%s-rev-%d", doc, n),
			Base:     texturewrite.PlainDoc(doc, fmt.Sprintf("%s-rev-%d-base", doc, n), "# "+title),
			Script:   script.String(),
			Resolver: l.captures,
		})
		if err != nil {
			l.t.Fatalf("write %s v%d: %v", doc, n, err)
		}
		bodyDoc, _ = json.Marshal(res.Doc)
		entities, _ = json.Marshal(res.Entities)
	}
	content := ""
	if bodyDoc == nil {
		content = "Story " + title
	}
	resp, err := l.svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID: owner, SourceDocID: doc, SourceRevisionID: fmt.Sprintf("%s-rev-%d", doc, n), RevisionNumber: n,
		Title: title, Content: content, BodyDoc: bodyDoc, SourceEntities: entities,
		AccessPolicy: accessPolicy, RequestedBy: owner, PublishMode: mode,
	})
	if err != nil {
		l.t.Fatalf("publish %s v%d: %v", doc, n, err)
	}
	return resp
}

// versionCanonicalID is the canonical id of the version a publish wrote.
func (l *relatedLab) versionCanonicalID(owner string, resp *PublishTextureResponse) string {
	l.t.Helper()
	id, err := publicationVersionCanonicalID(owner, resp.PublicationVersionID)
	if err != nil {
		l.t.Fatalf("version id: %v", err)
	}
	return id
}

// citesCapture lists the cites_capture edge targets of a version.
func (l *relatedLab) citesCapture(versionCanonicalID string) []string {
	l.t.Helper()
	edges, err := l.svc.ogStore().ListEdgesByKind(context.Background(), versionCanonicalID, edgeCitesCapture)
	if err != nil {
		l.t.Fatalf("list cites_capture: %v", err)
	}
	out := make([]string, 0, len(edges))
	for _, e := range edges {
		out = append(out, e.ToID)
	}
	return out
}

// fillers publishes n stories that cite nothing, so the paper has n more
// stories than the ones a test is about (the walk's idf reads the count).
func (l *relatedLab) fillers(n int) {
	l.t.Helper()
	for i := 0; i < n; i++ {
		l.publish(fmt.Sprintf("doc-filler-%d", i), 1, fmt.Sprintf("Filler story %d", i))
	}
}

// related asks the walk for a story's list; the cache is cleared first so
// a test reads what the graph says now unless it is testing the cache.
func (l *relatedLab) related(route string) *RelatedStories {
	l.t.Helper()
	l.svc.related.reset()
	out, err := l.svc.RelatedStories(context.Background(), route, 0)
	if err != nil {
		l.t.Fatalf("related %s: %v", route, err)
	}
	if out.Partial {
		l.t.Fatalf("related %s: over its %s budget (partial), not a result", route, l.svc.related.tuning.Budget)
	}
	return out
}

func relatedRoutes(r *RelatedStories) []string {
	out := make([]string, 0, len(r.Links))
	for _, l := range r.Links {
		out = append(out, l.Route.Bare)
	}
	return out
}

func hasRoute(routes []string, want string) bool {
	for _, r := range routes {
		if r == want {
			return true
		}
	}
	return false
}
