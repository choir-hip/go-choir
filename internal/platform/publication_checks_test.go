package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/quotematch"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The check overlay in Store B (Cut 2 2b). Ways it can fail, written before
// the code:
//   - the overlay reads the paper computer, so a stopped computer blanks
//     every badge (public reads must depend on corpusd alone);
//   - a version's quote checks never reach the overlay, or reach it keyed
//     by something other than version and citation, so one version's
//     checks show on another;
//   - a tolerant match shows as exact;
//   - a verdict that arrives after publish is lost, or is written into the
//     manifest (moving its hash) instead of the live overlay;
//   - a failing verdict shows as anything but failed;
//   - the verdict route takes writes from outside the platform.

func publishCheckedVersion(t *testing.T) (*Service, *PublishTextureResponse, []texturewrite.QuoteCheck) {
	t.Helper()
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
		Script:   "@insert end\nThe harbor reopens {{cite " + captureID + " \"harbor reopens at dawn\"}}.\n\nFerries resume {{cite " + captureID + " \"FERRY  service resumes\"}}.",
		Resolver: captures,
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	checks, _ := texturewrite.Checks(res.Doc, "doc-story", "rev-a")
	if len(checks) != 2 || checks[0].Tier != "exact" || checks[1].Tier == "exact" || checks[0].Pair == "" || checks[1].Pair == "" {
		t.Fatalf("fixture checks = %+v; want one exact and one tolerant pair", checks)
	}
	bodyDoc, _ := json.Marshal(res.Doc)
	entities, _ := json.Marshal(res.Entities)
	meta, _ := json.Marshal(map[string]any{texturewrite.ChecksMetadataKey: checks})
	svc.SetCheckOverlayProvider(NewStoreCheckOverlay(svc))
	v1, err := svc.PublishTexture(ctx, PublishTextureRequest{
		OwnerID: "owner-1", SourceDocID: "doc-story", SourceRevisionID: "rev-a", RevisionNumber: 2,
		Title: "Harbor Story", BodyDoc: bodyDoc, SourceEntities: entities, Metadata: meta, RequestedBy: "owner-1",
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	return svc, v1, checks
}

func overlayByAnchor(bundle *PublicationBundle) map[string]CitationCheck {
	out := map[string]CitationCheck{}
	for _, c := range bundle.Checks.Citations {
		out[c.AnchorID] = c
	}
	return out
}

func TestStoreOverlayServesQuoteChecksAndLateVerdictsOutsideTheManifest(t *testing.T) {
	svc, v, checks := publishCheckedVersion(t)
	ctx := context.Background()
	exact, tolerant := checks[0], checks[1]

	before := readBundle(t, svc, v.PermalinkPath)
	payload := payloadBytes(t, before)
	got := overlayByAnchor(before)
	if before.Checks.Source != storeOverlaySource || len(got) != 2 {
		t.Fatalf("overlay = %+v; want the Store B overlay with both citations", before.Checks)
	}
	if c := got[exact.RefID]; c.Quote != CheckExact || c.Judgement != CheckPending || c.State != CheckPending {
		t.Fatalf("exact citation before judgement = %+v", c)
	}
	if c := got[tolerant.RefID]; c.Quote != CheckTolerant || c.State != CheckPending {
		t.Fatalf("tolerant citation before judgement = %+v; a tolerant match must never show exact", c)
	}

	// Verdicts arrive after publish: the overlay moves, the version does not.
	if err := svc.RecordSupportVerdicts(ctx, types.StoryVerdictsRequest{OwnerID: "owner-1", ComputerID: "computer-paper", Verdicts: []types.StoryVerdict{
		{Key: exact.Pair, Verdict: types.SupportSupported, Judge: "jev:test"},
		{Key: tolerant.Pair, Verdict: types.SupportUnsupported, Why: "the notice says inspection, not resumption", Judge: "management"},
	}}); err != nil {
		t.Fatalf("record verdicts: %v", err)
	}
	after := readBundle(t, svc, v.PermalinkPath)
	got = overlayByAnchor(after)
	if c := got[exact.RefID]; c.State != CheckExact || c.Judgement != types.SupportSupported {
		t.Fatalf("supported exact citation = %+v; want ✓ exact", c)
	}
	if c := got[tolerant.RefID]; c.State != CheckFailed || c.Judgement != CheckFailed || c.Detail == "" {
		t.Fatalf("unsupported citation = %+v; want ✗ failed with why", c)
	}
	if payloadBytes(t, after) != payload || after.Payload.ManifestHash != v.ManifestHash {
		t.Fatal("a verdict changed the version's payload or manifest hash")
	}
	// The bare link (latest) serves the same overlay.
	if bare := overlayByAnchor(readBundle(t, svc, v.RoutePath)); bare[exact.RefID].State != CheckExact {
		t.Fatalf("bare link overlay = %+v", bare)
	}
}

// A computer pushes its stories' verdicts straight to corpusd: its platform
// URL (choir.platform_url) is corpusd, not the proxy. So the route admits
// the bound guest that owns the verdicts by the same rule as a capture
// write, as well as a host-internal caller (B12: every guest verdict push
// was 403 "internal caller required").
func TestSupportVerdictRouteAdmitsItsBoundGuestAndInternalCallers(t *testing.T) {
	svc, _, checks := publishCheckedVersion(t)
	h := NewHandler(svc)
	if err := h.ConfigureGuestBinding(fakeGuestLookup{
		"10.200.44.2": {Found: true, UserID: "owner-1", ComputerID: "computer-paper"},
		"10.200.9.9":  {Found: true, UserID: "owner-1", ComputerID: "computer-other"},
	}); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(types.StoryVerdictsRequest{OwnerID: "owner-1", ComputerID: "computer-paper",
		Verdicts: []types.StoryVerdict{{Key: checks[0].Pair, Verdict: types.SupportSupported, Judge: "management"}}})
	push := func(from string, internal bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, types.StoryVerdictsPath, bytes.NewReader(body))
		req.RemoteAddr = from + ":4000"
		if internal {
			req.Header.Set("X-Internal-Caller", "true")
		}
		rec := httptest.NewRecorder()
		h.HandleSupportVerdicts(rec, req)
		return rec
	}
	for _, c := range []struct {
		label    string
		from     string
		internal bool
		want     int
	}{
		{"the bound guest that owns the verdicts", "10.200.44.2", false, http.StatusOK},
		{"a guest bound to another computer", "10.200.9.9", false, http.StatusForbidden},
		{"an unbound outside caller", "203.0.113.9", false, http.StatusForbidden},
		{"an outside caller claiming to be internal", "203.0.113.9", true, http.StatusForbidden},
		{"a loopback caller without the internal marker", "127.0.0.1", false, http.StatusForbidden},
	} {
		if rec := push(c.from, c.internal); rec.Code != c.want {
			t.Fatalf("%s = %d %s; want %d", c.label, rec.Code, rec.Body, c.want)
		}
	}
	rec := httptest.NewRecorder()
	inside := httptest.NewRequest(http.MethodPost, types.StoryVerdictsPath, bytes.NewReader(body))
	inside.RemoteAddr = "127.0.0.1:4000"
	inside.Header.Set("X-Internal-Caller", "true")
	rec = httptest.NewRecorder()
	h.HandleSupportVerdicts(rec, inside)
	if rec.Code != http.StatusOK {
		t.Fatalf("internal caller = %d %s", rec.Code, rec.Body)
	}
	bad, _ := json.Marshal(types.StoryVerdictsRequest{OwnerID: "owner-1", Verdicts: []types.StoryVerdict{{Key: "k", Verdict: "maybe", Judge: "management"}}})
	badReq := httptest.NewRequest(http.MethodPost, types.StoryVerdictsPath, bytes.NewReader(bad))
	badReq.RemoteAddr = "127.0.0.1:4000"
	badReq.Header.Set("X-Internal-Caller", "true")
	rec = httptest.NewRecorder()
	h.HandleSupportVerdicts(rec, badReq)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("an unknown verdict = %d; want 400", rec.Code)
	}
}

// A1 (Cut 2 pre-flight amendment): a citation whose source was never
// captured is "unread source", not "pending". Ways it can fail: a
// not-captured quote shows as pending forever (the reader cannot tell a
// check in progress from a source nobody read); an unread source hides a
// failed judgement; an unread source shows as checked.
func TestNotCapturedCitationIsUnreadSource(t *testing.T) {
	if got := quoteState(publicationCheck{Outcome: string(quotematch.NotCaptured)}); got != CheckUnread {
		t.Fatalf("quoteState(not captured) = %q; want %q", got, CheckUnread)
	}
	for _, c := range []struct{ quote, judgement, want string }{
		{CheckUnread, CheckPending, CheckUnread},
		{CheckUnread, types.SupportSupported, CheckUnread},
		{CheckUnread, CheckFailed, CheckFailed},
		{CheckExact, CheckPending, CheckPending},
	} {
		if got := badgeState(c.quote, c.judgement); got != c.want {
			t.Fatalf("badgeState(%q, %q) = %q; want %q", c.quote, c.judgement, got, c.want)
		}
	}
}

// The overlay scanned every check row in Store B by JSON path on each
// resolve: 890-2400 ms of a 1 s article open
// (docs/archive/problems/article-open-slow-store-b-scans-2026-10-16.md). It now
// reads a version's checks and judgements by their IDs. Ways it could fail:
// a resolve still scans; a check or verdict is missed because its ID is
// computed differently from the one publish wrote; a batch read returns an
// object nobody asked for, or fails on an empty list.
func TestOverlayReadsChecksByIDWithoutScanning(t *testing.T) {
	svc, v, checks := publishCheckedVersion(t)
	ctx := context.Background()
	if err := svc.RecordSupportVerdicts(ctx, types.StoryVerdictsRequest{OwnerID: "owner-1", ComputerID: "computer-paper", Verdicts: []types.StoryVerdict{
		{Key: checks[0].Pair, Verdict: types.SupportSupported, Judge: "jev:test"},
	}}); err != nil {
		t.Fatalf("record verdicts: %v", err)
	}
	before := overlayScans.Load()
	got := overlayByAnchor(readBundle(t, svc, v.PermalinkPath))
	if n := overlayScans.Load() - before; n != 0 {
		t.Fatalf("resolve scanned the check rows %d times", n)
	}
	if c := got[checks[0].RefID]; c.State != CheckExact || c.Judgement != types.SupportSupported {
		t.Fatalf("checked and judged citation = %+v", c)
	}
	if c := got[checks[1].RefID]; c.Quote != CheckTolerant || c.Judgement != CheckPending {
		t.Fatalf("checked, unjudged citation = %+v", c)
	}

	og := svc.ogStore()
	if objs, err := og.GetObjects(ctx, nil); err != nil || len(objs) != 0 {
		t.Fatalf("empty batch: %v %v", objs, err)
	}
	id := publicationCheckID("owner-1", v.PublicationVersionID, checks[0].RefID)
	objs, err := og.GetObjects(ctx, []string{id, "obj:choir.publication_check:owner-1:missing", id})
	if err != nil || len(objs) != 1 || objs[id].CanonicalID != id {
		t.Fatalf("batch read = %v, %v; want the one existing check", objs, err)
	}
}
