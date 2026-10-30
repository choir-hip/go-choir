package platform

// Ways per-version permalinks can fail (Cut 2 unit 2b,
// docs/archive/definitions/choir-reeng-cut2-evidence-to-publication-2026-10-12.md),
// written before the code:
//
//  1. Every publish mints a new publication, version and route
//     (service.go ~199-210), so a story's link changes with each revision.
//     One publication per (owner, source doc): revisions 1, 2, 3 publish as
//     v1, v2, v3 under one bare route.
//  2. The version is hard-coded "v1" with no predecessor
//     (publication_graph.go:129, :138). v2 must name v1 as the version it
//     supersedes and chain v1's manifest hash; the manifest must not embed
//     every past body (service.go ~245-254).
//  3. Republishing a revision (a retry, a second click) mints a second
//     version. Publishing the same revision twice returns the same version;
//     a different revision under a published number is refused.
//  4. A failed publish is renumbered over or back-filled. v3 failing leaves
//     a gap nav skips; /v3 is not found; a late v3 after v4 is refused.
//  5. A version's permalink or payload changes when later versions publish.
//     v1's payload bytes and manifest hash are byte-identical after v2 and
//     v3; only its live nav changes (it gains next and latest).
//  6. The bundle read takes the last has_version edge
//     (service_publication_read.go:88-96) rather than the route: the bare
//     link serves the latest version in place, /v<n> serves version n.
//  7. The check overlay leaks into the manifest hash. Overlay results
//     change; the manifest hash and the payload bytes do not.
//  8. A citation keeps only its source entity's first selector
//     (source_metadata.go:152-158), so a version renders whatever its source
//     says later. Publish pins every cited and embedded capture block into
//     the version (resolved_blocks), writes one transclusion row per anchor,
//     and refuses a Ref whose block no longer hashes to what was cited.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/quotematch"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
)

func newVersionTestService(t *testing.T) (*Service, string) {
	t.Helper()
	store, root := openTestPlatformStore(t)
	artifactsRoot := filepath.Join(root, "artifacts")
	return NewService(store, artifactsRoot, filepath.Join(root, "signing.key")), artifactsRoot
}

func publishRevision(t *testing.T, svc *Service, n int, revisionID, content string) *PublishTextureResponse {
	t.Helper()
	resp, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID:          "owner-1",
		SourceDocID:      "doc-story",
		SourceRevisionID: revisionID,
		RevisionNumber:   n,
		RevisionHash:     "revhash-" + revisionID,
		Title:            "Harbor Story",
		Content:          content,
		RequestedBy:      "owner-1",
	})
	if err != nil {
		t.Fatalf("publish revision %d (%s): %v", n, revisionID, err)
	}
	return resp
}

func readBundle(t *testing.T, svc *Service, route string) *PublicationBundle {
	t.Helper()
	bundle, err := svc.GetPublicationBundleByRoute(context.Background(), route)
	if err != nil {
		t.Fatalf("read %s: %v", route, err)
	}
	return bundle
}

func payloadBytes(t *testing.T, bundle *PublicationBundle) string {
	t.Helper()
	raw, err := json.Marshal(bundle.Payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return string(raw)
}

func wantNav(t *testing.T, label string, got PublicationNav, revision, prev, next, latest int) {
	t.Helper()
	want := PublicationNav{Revision: revision, Prev: prev, Next: next, Latest: latest}
	if got != want {
		t.Fatalf("%s nav = %+v, want %+v", label, got, want)
	}
}

// Failure modes 1, 2 and 6: three revisions are three versions of one
// publication, each chained to the previous manifest, under one bare link.
func TestRevisionsPublishAsVersionsOfOnePublication(t *testing.T) {
	svc, _ := newVersionTestService(t)
	v1 := publishRevision(t, svc, 1, "rev-a", "Harbor opens.")
	v2 := publishRevision(t, svc, 2, "rev-b", "Harbor opens at dawn.")
	v3 := publishRevision(t, svc, 3, "rev-c", "Harbor opens at dawn; ferries resume.")

	if v1.PublicationID == "" || v2.PublicationID != v1.PublicationID || v3.PublicationID != v1.PublicationID {
		t.Fatalf("publications = %q %q %q, want one", v1.PublicationID, v2.PublicationID, v3.PublicationID)
	}
	if v2.RoutePath != v1.RoutePath || v3.RoutePath != v1.RoutePath {
		t.Fatalf("bare routes = %q %q %q, want one", v1.RoutePath, v2.RoutePath, v3.RoutePath)
	}
	for i, resp := range []*PublishTextureResponse{v1, v2, v3} {
		n := i + 1
		if resp.Revision != n || resp.PermalinkPath != v1.RoutePath+"/v"+strconv.Itoa(n) {
			t.Fatalf("version %d: revision %d permalink %q", n, resp.Revision, resp.PermalinkPath)
		}
	}
	if v1.PreviousManifestHash != "" || v1.SupersedesVersionID != "" {
		t.Fatalf("v1 names a predecessor: %+v", v1)
	}
	if v2.PreviousManifestHash != v1.ManifestHash || v2.SupersedesVersionID != v1.PublicationVersionID {
		t.Fatalf("v2 chain = (%q, %q), want (%q, %q)", v2.PreviousManifestHash, v2.SupersedesVersionID, v1.ManifestHash, v1.PublicationVersionID)
	}
	if v3.PreviousManifestHash != v2.ManifestHash || v3.SupersedesVersionID != v2.PublicationVersionID {
		t.Fatalf("v3 chain = (%q, %q)", v3.PreviousManifestHash, v3.SupersedesVersionID)
	}

	b3 := readBundle(t, svc, v3.PermalinkPath)
	if b3.Payload.Text != "Harbor opens at dawn; ferries resume." || b3.Payload.Revision != 3 {
		t.Fatalf("/v3 payload = %+v", b3.Payload)
	}
	var manifest map[string]any
	if err := json.Unmarshal(b3.Payload.Manifest, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if manifest["previous_manifest_hash"] != v2.ManifestHash {
		t.Fatalf("manifest previous hash = %v, want %s", manifest["previous_manifest_hash"], v2.ManifestHash)
	}
	if _, embeds := manifest["version_history"]; embeds || strings.Contains(string(b3.Payload.Manifest), "Harbor opens.\"") {
		t.Fatalf("v3 manifest embeds past bodies: %s", b3.Payload.Manifest)
	}
	if sha256Hex(b3.Payload.Manifest) != b3.Payload.ManifestHash || b3.Payload.ManifestHash != v3.ManifestHash {
		t.Fatalf("manifest hash %s does not hash the manifest", b3.Payload.ManifestHash)
	}
	if len(b3.Payload.Signatures) != 1 || !VerifyManifestSignature(b3.Payload.Signatures[0].PublicKey, b3.Payload.ManifestHash, b3.Payload.Signatures[0].Signature) {
		t.Fatalf("v3 manifest signature does not verify: %+v", b3.Payload.Signatures)
	}
	wantNav(t, "/v3", b3.Nav, 3, 2, 0, 3)

	// Failure mode 6: the bare link serves the latest in place.
	bare := readBundle(t, svc, v1.RoutePath)
	if bare.Payload.Revision != 3 || bare.Route.Path != v1.RoutePath || bare.Route.Permalink != v3.PermalinkPath {
		t.Fatalf("bare link serves revision %d at %+v, want latest v3", bare.Payload.Revision, bare.Route)
	}
	if payloadBytes(t, bare) != payloadBytes(t, b3) {
		t.Fatal("bare link payload differs from /v3 payload")
	}
	b2 := readBundle(t, svc, v2.PermalinkPath)
	if b2.Payload.Text != "Harbor opens at dawn." || b2.Version.SupersedesVersionID != v1.PublicationVersionID {
		t.Fatalf("/v2 = %q supersedes %q", b2.Payload.Text, b2.Version.SupersedesVersionID)
	}
	wantNav(t, "/v2", b2.Nav, 2, 1, 3, 3)
}

// Failure mode 3: the same revision twice is one version.
func TestRepublishingARevisionIsIdempotent(t *testing.T) {
	svc, _ := newVersionTestService(t)
	publishRevision(t, svc, 1, "rev-a", "First.")
	v2 := publishRevision(t, svc, 2, "rev-b", "Second.")
	again := publishRevision(t, svc, 2, "rev-b", "Second.")
	if again.PublicationVersionID != v2.PublicationVersionID || again.ManifestHash != v2.ManifestHash || again.PermalinkPath != v2.PermalinkPath {
		t.Fatalf("republish = %+v, want version %+v", again, v2)
	}
	if !again.Replayed || v2.Replayed {
		t.Fatalf("replayed flags: first %v, again %v", v2.Replayed, again.Replayed)
	}
	og := svc.ogStore()
	edges, err := og.ListEdgesByKind(context.Background(), publicationCanonicalIDForTest(t, svc, v2.PublicationID), "has_version")
	if err != nil || len(edges) != 2 {
		t.Fatalf("has_version edges = %d (%v), want 2", len(edges), err)
	}
	if _, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID: "owner-1", SourceDocID: "doc-story", SourceRevisionID: "rev-other", RevisionNumber: 2,
		Title: "Harbor Story", Content: "Other.", RequestedBy: "owner-1",
	}); err == nil {
		t.Fatal("a different revision published under an existing version number")
	}
	if _, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID: "owner-1", SourceDocID: "doc-story", SourceRevisionID: "rev-zero",
		Title: "Harbor Story", Content: "No number.", RequestedBy: "owner-1",
	}); err == nil {
		t.Fatal("a revision without a revision number published")
	}
}

// Failure mode 4: a failed publish is a gap nav skips; nothing renumbers.
func TestFailedVersionIsAGapNavSkips(t *testing.T) {
	svc, _ := newVersionTestService(t)
	v1 := publishRevision(t, svc, 1, "rev-a", "One.")
	v2 := publishRevision(t, svc, 2, "rev-b", "Two.")
	// Revision 3 failed to publish; revision 4 publishes as v4.
	v4 := publishRevision(t, svc, 4, "rev-d", "Four.")
	if v4.PreviousManifestHash != v2.ManifestHash || v4.SupersedesVersionID != v2.PublicationVersionID {
		t.Fatalf("v4 chains to %q, want v2", v4.SupersedesVersionID)
	}
	wantNav(t, "/v2", readBundle(t, svc, v2.PermalinkPath).Nav, 2, 1, 4, 4)
	wantNav(t, "/v4", readBundle(t, svc, v4.PermalinkPath).Nav, 4, 2, 0, 4)
	if _, err := svc.GetPublicationBundleByRoute(context.Background(), v1.RoutePath+"/v3"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("/v3 (the gap) = %v, want not found", err)
	}
	for _, bad := range []string{"/v0", "/v03", "/vx", "/v-1"} {
		if _, err := svc.GetPublicationBundleByRoute(context.Background(), v1.RoutePath+bad); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("%s = %v, want not found", bad, err)
		}
	}
	if _, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID: "owner-1", SourceDocID: "doc-story", SourceRevisionID: "rev-c", RevisionNumber: 3,
		Title: "Harbor Story", Content: "Three, late.", RequestedBy: "owner-1",
	}); err == nil {
		t.Fatal("a late revision back-filled the gap behind v4")
	}
	wantNav(t, "/v2 after late v3", readBundle(t, svc, v2.PermalinkPath).Nav, 2, 1, 4, 4)
}

// Failure mode 5: v1's payload is fixed forever; only its nav moves.
func TestEarlierVersionPayloadIsByteIdenticalAfterLaterVersions(t *testing.T) {
	svc, _ := newVersionTestService(t)
	v1 := publishRevision(t, svc, 1, "rev-a", "Harbor opens.")
	before := readBundle(t, svc, v1.PermalinkPath)
	beforeBytes := payloadBytes(t, before)
	wantNav(t, "/v1 alone", before.Nav, 1, 0, 0, 1)

	publishRevision(t, svc, 2, "rev-b", "Harbor opens at dawn.")
	mid := readBundle(t, svc, v1.PermalinkPath)
	wantNav(t, "/v1 after v2", mid.Nav, 1, 0, 2, 2)
	publishRevision(t, svc, 3, "rev-c", "Harbor opens.") // a revert: same text as v1
	after := readBundle(t, svc, v1.PermalinkPath)
	if payloadBytes(t, after) != beforeBytes {
		t.Fatalf("v1 payload changed:\nbefore %s\nafter  %s", beforeBytes, payloadBytes(t, after))
	}
	if after.Payload.ManifestHash != v1.ManifestHash || after.Version.ManifestHash != v1.ManifestHash {
		t.Fatalf("v1 manifest hash %s, want %s", after.Payload.ManifestHash, v1.ManifestHash)
	}
	if after.Route.Permalink != v1.PermalinkPath || after.Version.ID != v1.PublicationVersionID {
		t.Fatalf("v1 permalink/id moved: %+v %s", after.Route, after.Version.ID)
	}
	wantNav(t, "/v1 after v3", after.Nav, 1, 0, 2, 3)
}

type fixedOverlay struct{ state string }

func (f fixedOverlay) CheckOverlay(_ context.Context, version CheckOverlayVersion, citations []CheckOverlayCitation) (PublicationCheckOverlay, error) {
	out := PublicationCheckOverlay{Source: "test", Citations: []CitationCheck{}}
	for _, c := range citations {
		out.Citations = append(out.Citations, CitationCheck{AnchorID: c.AnchorID, Ref: c.Ref, Quote: f.state, Judgement: f.state, Detail: "overlay-marker-" + version.PublicationVersionID})
	}
	return out, nil
}

// Failure modes 7 and 8: cited and embedded capture blocks are pinned into
// the version, one transclusion row per anchor; the live overlay is keyed by
// version and citation and never moves the manifest hash.
func TestPublishPinsCitedBlocksAndOverlayStaysOutsideTheManifest(t *testing.T) {
	svc, artifactsRoot := newVersionTestService(t)
	ctx := context.Background()
	ogSvc := objectgraph.NewService(objectgraph.Config{Durable: svc.ogStore()})
	bodies := &CaptureBodyStore{root: CaptureBodiesRoot(artifactsRoot)}
	written, err := sourcegraph.WriteCaptures(ctx, ogSvc, bodies, []sourcegraph.CaptureItem{{Item: sources.Item{
		ID: "item-harbor", SourceID: "rss:port", SourceType: sources.SourceTypeRSS, Title: "Port notice",
		URL:       "https://port.example/notice",
		Body:      "The harbor reopens at dawn on Monday.\n\nFerry service resumes after inspection.",
		FetchedAt: time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC),
	}}}, sourcegraph.CaptureWriteConfig{OwnerID: "paper-owner", ComputerID: "computer-paper", Now: time.Now().UTC()})
	if err != nil || len(written.Captured) != 1 {
		t.Fatalf("write capture: %+v %v", written, err)
	}
	captureID := written.Captured[0].CaptureID
	captures := &storeCaptures{t: t, og: svc.ogStore(), bodies: bodies}

	res, err := texturewrite.Apply(ctx, texturewrite.Input{
		OwnerID: "owner-1", DocID: "doc-story", Seed: "rev-a",
		Base:     texturewrite.PlainDoc("doc-story", "rev-0", "# Harbor"),
		Script:   "@insert end\nThe harbor reopens {{cite " + captureID + " \"harbor reopens at dawn\"}}.\n\nAgain {{cite " + captureID + " \"reopens at dawn on Monday\"}}.\n\nFerries run {{cite " + captureID + " \"Ferry service resumes\"}}.",
		Resolver: captures,
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	markStoredEmbed(t, &res.Doc, "Ferry service resumes")
	bodyDoc, _ := json.Marshal(res.Doc)
	entities, _ := json.Marshal(res.Entities)
	publish := func(n int, revisionID string, doc, ents json.RawMessage) (*PublishTextureResponse, error) {
		return svc.PublishTexture(ctx, PublishTextureRequest{
			OwnerID: "owner-1", SourceDocID: "doc-story", SourceRevisionID: revisionID, RevisionNumber: n,
			Title: "Harbor Story", BodyDoc: doc, SourceEntities: ents, RequestedBy: "owner-1",
		})
	}
	v1, err := publish(1, "rev-a", bodyDoc, entities)
	if err != nil {
		t.Fatalf("publish cited revision: %v", err)
	}
	bundle := readBundle(t, svc, v1.PermalinkPath)

	blocks := captures.blocks(captureID)
	want := map[string]string{blocks[0].ID: blocks[0].Text, blocks[1].ID: blocks[1].Text}
	if len(bundle.Payload.ResolvedBlocks) != 2 {
		t.Fatalf("resolved blocks = %+v, want the two cited blocks once each", bundle.Payload.ResolvedBlocks)
	}
	for _, rb := range bundle.Payload.ResolvedBlocks {
		if rb.Doc != captureID || want[rb.CID] != rb.Text || rb.Hash != blockHashForTest(rb.Text) || rb.Version == "" {
			t.Fatalf("resolved block %+v does not pin the capture block", rb)
		}
	}
	var anchors []PublicationTransclusion
	for _, tr := range bundle.Transclusions {
		if tr.AnchorID != "" {
			anchors = append(anchors, tr)
		}
	}
	if len(anchors) != 3 {
		t.Fatalf("anchor transclusion rows = %d, want 3 (one per anchor): %+v", len(anchors), bundle.Transclusions)
	}
	modes := map[string]int{}
	for _, a := range anchors {
		modes[a.RelationType]++
		if a.Ref == nil || a.Ref.Doc != captureID || a.SnapshotText != want[a.Ref.CID] {
			t.Fatalf("anchor row %+v does not carry its Ref and pinned text", a)
		}
	}
	if modes["cites"] != 2 || modes["embeds"] != 1 {
		t.Fatalf("anchor modes = %v, want 2 cites and 1 embed", modes)
	}

	// The overlay is live: pending by default, keyed by anchor.
	if bundle.Checks.Source != "pending" || len(bundle.Checks.Citations) != 3 {
		t.Fatalf("default overlay = %+v", bundle.Checks)
	}
	for _, c := range bundle.Checks.Citations {
		if c.Quote != CheckPending || c.AnchorID == "" || c.Ref.Doc != captureID {
			t.Fatalf("default overlay citation = %+v", c)
		}
	}
	pendingPayload := payloadBytes(t, bundle)
	svc.SetCheckOverlayProvider(fixedOverlay{state: CheckExact})
	checked := readBundle(t, svc, v1.PermalinkPath)
	if checked.Checks.Source != "test" || checked.Checks.Citations[0].Quote != CheckExact {
		t.Fatalf("overlay provider not served: %+v", checked.Checks)
	}
	if payloadBytes(t, checked) != pendingPayload || checked.Payload.ManifestHash != v1.ManifestHash {
		t.Fatal("overlay results changed the payload or manifest hash")
	}
	if checked.Checks.Citations[0].Detail != "overlay-marker-"+v1.PublicationVersionID || strings.Contains(string(checked.Payload.Manifest), "overlay-marker-") {
		t.Fatalf("overlay state leaked into the manifest: %s", checked.Payload.Manifest)
	}

	// Pinned: the capture's bodies going away does not change the version.
	if err := os.RemoveAll(CaptureBodiesRoot(artifactsRoot)); err != nil {
		t.Fatal(err)
	}
	if payloadBytes(t, readBundle(t, svc, v1.PermalinkPath)) != pendingPayload {
		t.Fatal("version payload depends on the live capture store")
	}

	// A Ref whose block hash does not match the capture is refused.
	tampered := strings.Replace(string(bodyDoc), blockHashForTest(blocks[0].Text), blockHashForTest("something else"), 1)
	if tampered == string(bodyDoc) {
		t.Fatal("fixture: block hash not found in body_doc")
	}
	if _, err := publish(2, "rev-b", json.RawMessage(tampered), entities); err == nil {
		t.Fatal("a Ref whose block hash does not match was published")
	}
}

// TestVersionRouteParsing pins the /v<n> grammar: the last path segment,
// v and a positive decimal with no leading zero.
func TestVersionRouteParsing(t *testing.T) {
	cases := []struct {
		in   string
		bare string
		n    int
		ok   bool
	}{
		{"/pub/texture/harbor-pub1/v12", "/pub/texture/harbor-pub1", 12, true},
		{"/pub/texture/harbor-pub1/v1/", "/pub/texture/harbor-pub1", 1, true},
		{"/pub/texture/harbor-pub1", "/pub/texture/harbor-pub1", 0, true},
		{"/pub/texture/harbor-v2-pub1", "/pub/texture/harbor-v2-pub1", 0, true},
		{"/pub/texture/harbor-pub1/v0", "", 0, false},
		{"/pub/texture/harbor-pub1/v01", "", 0, false},
		{"/pub/texture/harbor-pub1/latest", "", 0, false},
	}
	for _, tc := range cases {
		bare, n, ok := parsePublicationRoute(tc.in)
		if ok != tc.ok || (ok && (bare != tc.bare || n != tc.n)) {
			t.Fatalf("parse %q = (%q, %d, %v), want (%q, %d, %v)", tc.in, bare, n, ok, tc.bare, tc.n, tc.ok)
		}
	}
}

// storeCaptures is a texturewrite.CaptureSource over the Store B capture the
// test wrote: the blocks Texture cites are the blocks publish pins.
type storeCaptures struct {
	t      *testing.T
	og     *ObjectGraphStore
	bodies *CaptureBodyStore
}

func (s *storeCaptures) blocks(captureID string) []quotematch.Block {
	s.t.Helper()
	c, err := s.CaptureBlocks(context.Background(), "", captureID)
	if err != nil {
		s.t.Fatalf("read capture blocks: %v", err)
	}
	return c.Blocks
}

func (s *storeCaptures) CaptureBlocks(ctx context.Context, _ string, handle string) (texturewrite.Capture, error) {
	obj, err := s.og.GetObject(ctx, handle)
	if err != nil {
		return texturewrite.Capture{}, texturewrite.ErrNotCaptured
	}
	meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
	if err != nil {
		return texturewrite.Capture{}, err
	}
	raw, err := s.bodies.Body(strings.TrimPrefix(meta.BlocksBlobID, "sha256:"))
	if err != nil {
		return texturewrite.Capture{}, err
	}
	var doc sourcegraph.CaptureBlocks
	if err := json.Unmarshal(raw, &doc); err != nil {
		return texturewrite.Capture{}, err
	}
	out := texturewrite.Capture{Doc: obj.CanonicalID, Version: meta.DocHash, URL: meta.URL, Title: meta.Title}
	for _, b := range doc.Blocks {
		out.Blocks = append(out.Blocks, quotematch.Block{ID: b.ID, Text: b.Text})
	}
	return out, nil
}

var _ texturewrite.CaptureSource = (*storeCaptures)(nil)

func blockHashForTest(text string) string { return "sha256:" + sha256Hex([]byte(text)) }

func publicationCanonicalIDForTest(t *testing.T, svc *Service, publicationID string) string {
	t.Helper()
	objs, err := svc.ogStore().ListObjectsByMetadata(context.Background(), "choir.publication", "$.publication_id", publicationID, 1)
	if err != nil || len(objs) != 1 {
		t.Fatalf("publication %s: %v", publicationID, err)
	}
	return objs[0].CanonicalID
}
